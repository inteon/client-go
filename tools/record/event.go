/*
Copyright 2014 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package record

import (
	"context"
	"fmt"
	"math/rand"
	"time"

	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/pkg/concurrent"
	"k8s.io/client-go/pkg/watch"
	restclient "k8s.io/client-go/rest"
	"k8s.io/client-go/tools/record/util"
	ref "k8s.io/client-go/tools/reference"
	"k8s.io/klog/v2"
	"k8s.io/utils/clock"
)

const maxTriesPerEvent = 12

var defaultSleepDuration = 10 * time.Second

const maxQueuedEvents = 1000

// CorrelatorOptions allows you to change the default of the EventSourceObjectSpamFilter
// and EventAggregator in EventCorrelator
type CorrelatorOptions struct {
	// The lru cache size used for both EventSourceObjectSpamFilter and the EventAggregator
	// If not specified (zero value), the default specified in events_cache.go will be picked
	// This means that the LRUCacheSize has to be greater than 0.
	LRUCacheSize int
	// The burst size used by the token bucket rate filtering in EventSourceObjectSpamFilter
	// If not specified (zero value), the default specified in events_cache.go will be picked
	// This means that the BurstSize has to be greater than 0.
	BurstSize int
	// The fill rate of the token bucket in queries per second in EventSourceObjectSpamFilter
	// If not specified (zero value), the default specified in events_cache.go will be picked
	// This means that the QPS has to be greater than 0.
	QPS float32
	// The func used by the EventAggregator to group event keys for aggregation
	// If not specified (zero value), EventAggregatorByReasonFunc will be used
	KeyFunc EventAggregatorKeyFunc
	// The func used by the EventAggregator to produced aggregated message
	// If not specified (zero value), EventAggregatorByReasonMessageFunc will be used
	MessageFunc EventAggregatorMessageFunc
	// The number of events in an interval before aggregation happens by the EventAggregator
	// If not specified (zero value), the default specified in events_cache.go will be picked
	// This means that the MaxEvents has to be greater than 0
	MaxEvents int
	// The amount of time in seconds that must transpire since the last occurrence of a similar event before it is considered new by the EventAggregator
	// If not specified (zero value), the default specified in events_cache.go will be picked
	// This means that the MaxIntervalInSeconds has to be greater than 0
	MaxIntervalInSeconds int
	// The clock used by the EventAggregator to allow for testing
	// If not specified (zero value), clock.RealClock{} will be used
	Clock clock.PassiveClock
	// The func used by EventFilterFunc, which returns a key for given event, based on which filtering will take place
	// If not specified (zero value), getSpamKey will be used
	SpamKeyFunc EventSpamKeyFunc
}

// EventRecorderAdapter is a wrapper around a "k8s.io/client-go/tools/record".EventRecorder
// implementing the new "k8s.io/client-go/tools/events".EventRecorder interface.
type EventRecorderAdapter struct {
	recorder EventRecorder
}

// NewEventRecorderAdapter returns an adapter implementing the new
// "k8s.io/client-go/tools/events".EventRecorder interface.
func NewEventRecorderAdapter(recorder EventRecorder) *EventRecorderAdapter {
	return &EventRecorderAdapter{
		recorder: recorder,
	}
}

// Eventf is a wrapper around v1 Eventf
func (a *EventRecorderAdapter) Eventf(regarding, _ runtime.Object, eventtype, reason, action, note string, args ...interface{}) {
	a.recorder.Eventf(regarding, eventtype, reason, note, args...)
}

// Creates a new event broadcaster.
func NewBroadcaster() EventBroadcaster {
	return &eventBroadcasterImpl{
		BoundedWatcher: watch.NewBoundedWatcherWithSize(maxQueuedEvents),
		sleepDuration:  defaultSleepDuration,
	}
}

func NewBroadcasterForTests(sleepDuration time.Duration) EventBroadcaster {
	return &eventBroadcasterImpl{
		BoundedWatcher: watch.NewBoundedWatcherWithSize(maxQueuedEvents),
		sleepDuration:  sleepDuration,
	}
}

func NewBroadcasterWithCorrelatorOptions(options CorrelatorOptions) EventBroadcaster {
	return &eventBroadcasterImpl{
		BoundedWatcher: watch.NewBoundedWatcherWithSize(maxQueuedEvents),
		sleepDuration:  defaultSleepDuration,
		options:        options,
	}
}

type eventBroadcasterImpl struct {
	*watch.BoundedWatcher
	sleepDuration time.Duration
	options       CorrelatorOptions
}

// StartRecordingToSink starts sending events received from the specified eventBroadcaster to the given sink.
// The return value can be ignored or used to stop recording, if desired.
// TODO: make me an object with parameterizable queue length and retry interval
func (e *eventBroadcasterImpl) StartRecordingToSink(ctx context.Context, sink EventSink) error {
	eventCorrelator := NewEventCorrelatorWithOptions(e.options)
	return e.StartEventWatcher(
		ctx,
		func(event *v1.Event) {
			recordToSink(sink, event, eventCorrelator, e.sleepDuration)
		})
}

func recordToSink(sink EventSink, event *v1.Event, eventCorrelator *EventCorrelator, sleepDuration time.Duration) {
	// Make a copy before modification, because there could be multiple listeners.
	// Events are safe to copy like this.
	eventCopy := *event
	event = &eventCopy
	result, err := eventCorrelator.EventCorrelate(event)
	if err != nil {
		utilruntime.HandleError(err)
	}
	if result.Skip {
		return
	}
	tries := 0
	for {
		if recordEvent(sink, result.Event, result.Patch, result.Event.Count > 1, eventCorrelator) {
			break
		}
		tries++
		if tries >= maxTriesPerEvent {
			klog.Errorf("Unable to write event '%#v' (retry limit exceeded!)", event)
			break
		}
		// Randomize the first sleep so that various clients won't all be
		// synced up if the master goes down.
		if tries == 1 {
			time.Sleep(time.Duration(float64(sleepDuration) * rand.Float64()))
		} else {
			time.Sleep(sleepDuration)
		}
	}
}

// recordEvent attempts to write event to a sink. It returns true if the event
// was successfully recorded or discarded, false if it should be retried.
// If updateExistingEvent is false, it creates a new event, otherwise it updates
// existing event.
func recordEvent(sink EventSink, event *v1.Event, patch []byte, updateExistingEvent bool, eventCorrelator *EventCorrelator) bool {
	var newEvent *v1.Event
	var err error
	if updateExistingEvent {
		newEvent, err = sink.Patch(event, patch)
	}
	// Update can fail because the event may have been removed and it no longer exists.
	if !updateExistingEvent || (updateExistingEvent && util.IsKeyNotFoundError(err)) {
		// Making sure that ResourceVersion is empty on creation
		event.ResourceVersion = ""
		newEvent, err = sink.Create(event)
	}
	if err == nil {
		// we need to update our event correlator with the server returned state to handle name/resourceversion
		eventCorrelator.UpdateState(newEvent)
		return true
	}

	// If we can't contact the server, then hold everything while we keep trying.
	// Otherwise, something about the event is malformed and we should abandon it.
	switch err.(type) {
	case *restclient.RequestConstructionError:
		// We will construct the request the same next time, so don't keep trying.
		klog.Errorf("Unable to construct event '%#v': '%v' (will not retry!)", event, err)
		return true
	case *errors.StatusError:
		if errors.IsAlreadyExists(err) {
			klog.V(5).Infof("Server rejected event '%#v': '%v' (will not retry!)", event, err)
		} else {
			klog.Errorf("Server rejected event '%#v': '%v' (will not retry!)", event, err)
		}
		return true
	case *errors.UnexpectedObjectError:
		// We don't expect this; it implies the server's response didn't match a
		// known pattern. Go ahead and retry.
	default:
		// This case includes actual http transport errors. Go ahead and retry.
	}
	klog.Errorf("Unable to write event: '%#v': '%v'(may retry after sleeping)", event, err)
	return false
}

// StartLogging starts sending events received from this EventBroadcaster to the given logging function.
// The return value can be ignored or used to stop recording, if desired.
func (e *eventBroadcasterImpl) StartLogging(ctx context.Context, logf func(format string, args ...interface{})) error {
	return e.StartEventWatcher(
		ctx,
		func(e *v1.Event) {
			logf("Event(%#v): type: '%v' reason: '%v' %v", e.InvolvedObject, e.Type, e.Reason, e.Message)
		})
}

// StartStructuredLogging starts sending events received from this EventBroadcaster to the structured logging function.
// The return value can be ignored or used to stop recording, if desired.
func (e *eventBroadcasterImpl) StartStructuredLogging(ctx context.Context, verbosity klog.Level) error {
	return e.StartEventWatcher(
		ctx,
		func(e *v1.Event) {
			klog.V(verbosity).InfoS("Event occurred", "object", klog.KRef(e.InvolvedObject.Namespace, e.InvolvedObject.Name), "fieldPath", e.InvolvedObject.FieldPath, "kind", e.InvolvedObject.Kind, "apiVersion", e.InvolvedObject.APIVersion, "type", e.Type, "reason", e.Reason, "message", e.Message)
		})
}

// StartEventWatcher starts sending events received from this EventBroadcaster to the given event handler function.
// The return value can be ignored or used to stop recording, if desired.
func (e *eventBroadcasterImpl) StartEventWatcher(ctx context.Context, eventHandler func(*v1.Event)) error {
	outStream, ctx, cancel := concurrent.Watch(ctx, e)
	defer cancel()

	func() {
		defer utilruntime.HandleCrash()
	Loop:
		for {
			select {
			case watchEvent := <-outStream:
				event, ok := watchEvent.Object.(*v1.Event)
				if !ok {
					// This is all local, so there's no reason this should
					// ever happen.
					continue
				}
				eventHandler(event)
			case <-ctx.Done():
				break Loop
			}
		}
	}()

	return nil
}

// NewRecorder returns an EventRecorder that records events with the given event source.
func (e *eventBroadcasterImpl) NewRecorder(scheme *runtime.Scheme, source v1.EventSource) EventRecorder {
	return &recorderImpl{scheme, source, e.BoundedWatcher, clock.RealClock{}}
}

type recorderImpl struct {
	scheme *runtime.Scheme
	source v1.EventSource
	*watch.BoundedWatcher
	clock clock.PassiveClock
}

func (recorder *recorderImpl) generateEvent(object runtime.Object, annotations map[string]string, eventtype, reason, message string) {
	ref, err := ref.GetReference(recorder.scheme, object)
	if err != nil {
		klog.Errorf("Could not construct reference to: '%#v' due to: '%v'. Will not report event: '%v' '%v' '%v'", object, err, eventtype, reason, message)
		return
	}

	if !util.ValidateEventType(eventtype) {
		klog.Errorf("Unsupported event type: '%v'", eventtype)
		return
	}

	event := recorder.makeEvent(ref, annotations, eventtype, reason, message)
	event.Source = recorder.source

	// NOTE: events should be a non-blocking operation, but we also need to not
	// put this in a goroutine, otherwise we'll race to write to a closed channel
	// when we go to shut down this broadcaster.  Just drop events if we get overloaded,
	// and log an error if that happens (we've configured the broadcaster to drop
	// outgoing events anyway).
	sent := recorder.TryAction(watch.Added, event)
	if !sent {
		klog.Errorf("unable to record event: too many queued events, dropped event %#v", event)
	}
}

func (recorder *recorderImpl) Event(object runtime.Object, eventtype, reason, message string) {
	recorder.generateEvent(object, nil, eventtype, reason, message)
}

func (recorder *recorderImpl) Eventf(object runtime.Object, eventtype, reason, messageFmt string, args ...interface{}) {
	recorder.Event(object, eventtype, reason, fmt.Sprintf(messageFmt, args...))
}

func (recorder *recorderImpl) AnnotatedEventf(object runtime.Object, annotations map[string]string, eventtype, reason, messageFmt string, args ...interface{}) {
	recorder.generateEvent(object, annotations, eventtype, reason, fmt.Sprintf(messageFmt, args...))
}

func (recorder *recorderImpl) makeEvent(ref *v1.ObjectReference, annotations map[string]string, eventtype, reason, message string) *v1.Event {
	t := metav1.Time{Time: recorder.clock.Now()}
	namespace := ref.Namespace
	if namespace == "" {
		namespace = metav1.NamespaceDefault
	}
	return &v1.Event{
		ObjectMeta: metav1.ObjectMeta{
			Name:        fmt.Sprintf("%v.%x", ref.Name, t.UnixNano()),
			Namespace:   namespace,
			Annotations: annotations,
		},
		InvolvedObject: *ref,
		Reason:         reason,
		Message:        message,
		FirstTimestamp: t,
		LastTimestamp:  t,
		Count:          1,
		Type:           eventtype,
	}
}
