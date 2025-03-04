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
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"sync"
	"testing"
	"time"

	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	k8sruntime "k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/strategicpatch"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/pkg/concurrent"
	restclient "k8s.io/client-go/rest"
	ref "k8s.io/client-go/tools/reference"
	"k8s.io/utils/clock"
	testclocks "k8s.io/utils/clock/testing"
)

func newKubeScheme(t testing.TB) *runtime.Scheme {
	kubeScheme := runtime.NewScheme()
	if err := scheme.AddToScheme(kubeScheme); err != nil {
		t.Fatal(err)
	}
	return kubeScheme
}

type testEventSink struct {
	OnCreate func(e *v1.Event) (*v1.Event, error)
	OnUpdate func(e *v1.Event) (*v1.Event, error)
	OnPatch  func(e *v1.Event, p []byte) (*v1.Event, error)
}

// CreateEvent records the event for testing.
func (t *testEventSink) Create(e *v1.Event) (*v1.Event, error) {
	if t.OnCreate != nil {
		return t.OnCreate(e)
	}
	return e, nil
}

// UpdateEvent records the event for testing.
func (t *testEventSink) Update(e *v1.Event) (*v1.Event, error) {
	if t.OnUpdate != nil {
		return t.OnUpdate(e)
	}
	return e, nil
}

// PatchEvent records the event for testing.
func (t *testEventSink) Patch(e *v1.Event, p []byte) (*v1.Event, error) {
	if t.OnPatch != nil {
		return t.OnPatch(e, p)
	}
	return e, nil
}

type OnCreateFunc func(*v1.Event) (*v1.Event, error)

func OnCreateFactory(testCache map[string]*v1.Event, createEvent chan<- *v1.Event) OnCreateFunc {
	return func(event *v1.Event) (*v1.Event, error) {
		testCache[getEventKey(event)] = event
		createEvent <- event
		return event, nil
	}
}

type OnPatchFunc func(*v1.Event, []byte) (*v1.Event, error)

func OnPatchFactory(testCache map[string]*v1.Event, patchEvent chan<- *v1.Event) OnPatchFunc {
	return func(event *v1.Event, patch []byte) (*v1.Event, error) {
		cachedEvent, found := testCache[getEventKey(event)]
		if !found {
			return nil, fmt.Errorf("unexpected error: couldn't find Event in testCache.")
		}
		originalData, err := json.Marshal(cachedEvent)
		if err != nil {
			return nil, fmt.Errorf("unexpected error: %v", err)
		}
		patched, err := strategicpatch.StrategicMergePatch(originalData, patch, event)
		if err != nil {
			return nil, fmt.Errorf("unexpected error: %v", err)
		}
		patchedObj := &v1.Event{}
		err = json.Unmarshal(patched, patchedObj)
		if err != nil {
			return nil, fmt.Errorf("unexpected error: %v", err)
		}
		patchEvent <- patchedObj
		return patchedObj, nil
	}
}

func TestNonRacyShutdown(t *testing.T) {
	// Attempt to simulate previously racy conditions, and ensure that no race
	// occurs: Nominally, calling "Eventf" *followed by* shutdown from the same
	// thread should be a safe operation, but it's not if we launch recorder.Action
	// in a goroutine.

	caster := NewBroadcasterForTests(0)
	clock := testclocks.NewFakeClock(time.Now())
	recorder := recorderWithFakeClock(t, v1.EventSource{Component: "eventTest"}, caster, clock)

	var wg sync.WaitGroup
	wg.Add(100)
	for i := 0; i < 100; i++ {
		go func() {
			defer wg.Done()
			recorder.Eventf(&v1.ObjectReference{}, v1.EventTypeNormal, "Started", "blah")
		}()
	}

	wg.Wait()
	caster.Shutdown()
}

func TestEventf(t *testing.T) {
	testPod := &v1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "foo",
			Namespace: "baz",
			UID:       "bar",
		},
	}
	testPod2 := &v1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "foo",
			Namespace: "baz",
			UID:       "differentUid",
		},
	}
	testRef, err := ref.GetPartialReference(newKubeScheme(t), testPod, "spec.containers[2]")
	if err != nil {
		t.Fatal(err)
	}
	testRef2, err := ref.GetPartialReference(newKubeScheme(t), testPod2, "spec.containers[3]")
	if err != nil {
		t.Fatal(err)
	}
	table := []struct {
		obj          k8sruntime.Object
		eventtype    string
		reason       string
		messageFmt   string
		elements     []interface{}
		expect       *v1.Event
		expectLog    string
		expectUpdate bool
	}{
		{
			obj:        testRef,
			eventtype:  v1.EventTypeNormal,
			reason:     "Started",
			messageFmt: "some verbose message: %v",
			elements:   []interface{}{1},
			expect: &v1.Event{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "foo",
					Namespace: "baz",
				},
				InvolvedObject: v1.ObjectReference{
					Kind:       "Pod",
					Name:       "foo",
					Namespace:  "baz",
					UID:        "bar",
					APIVersion: "v1",
					FieldPath:  "spec.containers[2]",
				},
				Reason:  "Started",
				Message: "some verbose message: 1",
				Source:  v1.EventSource{Component: "eventTest"},
				Count:   1,
				Type:    v1.EventTypeNormal,
			},
			expectLog:    `Event(v1.ObjectReference{Kind:"Pod", Namespace:"baz", Name:"foo", UID:"bar", APIVersion:"v1", ResourceVersion:"", FieldPath:"spec.containers[2]"}): type: 'Normal' reason: 'Started' some verbose message: 1`,
			expectUpdate: false,
		},
		{
			obj:        testPod,
			eventtype:  v1.EventTypeNormal,
			reason:     "Killed",
			messageFmt: "some other verbose message: %v",
			elements:   []interface{}{1},
			expect: &v1.Event{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "foo",
					Namespace: "baz",
				},
				InvolvedObject: v1.ObjectReference{
					Kind:       "Pod",
					Name:       "foo",
					Namespace:  "baz",
					UID:        "bar",
					APIVersion: "v1",
				},
				Reason:  "Killed",
				Message: "some other verbose message: 1",
				Source:  v1.EventSource{Component: "eventTest"},
				Count:   1,
				Type:    v1.EventTypeNormal,
			},
			expectLog:    `Event(v1.ObjectReference{Kind:"Pod", Namespace:"baz", Name:"foo", UID:"bar", APIVersion:"v1", ResourceVersion:"", FieldPath:""}): type: 'Normal' reason: 'Killed' some other verbose message: 1`,
			expectUpdate: false,
		},
		{
			obj:        testRef,
			eventtype:  v1.EventTypeNormal,
			reason:     "Started",
			messageFmt: "some verbose message: %v",
			elements:   []interface{}{1},
			expect: &v1.Event{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "foo",
					Namespace: "baz",
				},
				InvolvedObject: v1.ObjectReference{
					Kind:       "Pod",
					Name:       "foo",
					Namespace:  "baz",
					UID:        "bar",
					APIVersion: "v1",
					FieldPath:  "spec.containers[2]",
				},
				Reason:  "Started",
				Message: "some verbose message: 1",
				Source:  v1.EventSource{Component: "eventTest"},
				Count:   2,
				Type:    v1.EventTypeNormal,
			},
			expectLog:    `Event(v1.ObjectReference{Kind:"Pod", Namespace:"baz", Name:"foo", UID:"bar", APIVersion:"v1", ResourceVersion:"", FieldPath:"spec.containers[2]"}): type: 'Normal' reason: 'Started' some verbose message: 1`,
			expectUpdate: true,
		},
		{
			obj:        testRef2,
			eventtype:  v1.EventTypeNormal,
			reason:     "Started",
			messageFmt: "some verbose message: %v",
			elements:   []interface{}{1},
			expect: &v1.Event{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "foo",
					Namespace: "baz",
				},
				InvolvedObject: v1.ObjectReference{
					Kind:       "Pod",
					Name:       "foo",
					Namespace:  "baz",
					UID:        "differentUid",
					APIVersion: "v1",
					FieldPath:  "spec.containers[3]",
				},
				Reason:  "Started",
				Message: "some verbose message: 1",
				Source:  v1.EventSource{Component: "eventTest"},
				Count:   1,
				Type:    v1.EventTypeNormal,
			},
			expectLog:    `Event(v1.ObjectReference{Kind:"Pod", Namespace:"baz", Name:"foo", UID:"differentUid", APIVersion:"v1", ResourceVersion:"", FieldPath:"spec.containers[3]"}): type: 'Normal' reason: 'Started' some verbose message: 1`,
			expectUpdate: false,
		},
		{
			obj:        testRef,
			eventtype:  v1.EventTypeNormal,
			reason:     "Started",
			messageFmt: "some verbose message: %v",
			elements:   []interface{}{1},
			expect: &v1.Event{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "foo",
					Namespace: "baz",
				},
				InvolvedObject: v1.ObjectReference{
					Kind:       "Pod",
					Name:       "foo",
					Namespace:  "baz",
					UID:        "bar",
					APIVersion: "v1",
					FieldPath:  "spec.containers[2]",
				},
				Reason:  "Started",
				Message: "some verbose message: 1",
				Source:  v1.EventSource{Component: "eventTest"},
				Count:   3,
				Type:    v1.EventTypeNormal,
			},
			expectLog:    `Event(v1.ObjectReference{Kind:"Pod", Namespace:"baz", Name:"foo", UID:"bar", APIVersion:"v1", ResourceVersion:"", FieldPath:"spec.containers[2]"}): type: 'Normal' reason: 'Started' some verbose message: 1`,
			expectUpdate: true,
		},
		{
			obj:        testRef2,
			eventtype:  v1.EventTypeNormal,
			reason:     "Stopped",
			messageFmt: "some verbose message: %v",
			elements:   []interface{}{1},
			expect: &v1.Event{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "foo",
					Namespace: "baz",
				},
				InvolvedObject: v1.ObjectReference{
					Kind:       "Pod",
					Name:       "foo",
					Namespace:  "baz",
					UID:        "differentUid",
					APIVersion: "v1",
					FieldPath:  "spec.containers[3]",
				},
				Reason:  "Stopped",
				Message: "some verbose message: 1",
				Source:  v1.EventSource{Component: "eventTest"},
				Count:   1,
				Type:    v1.EventTypeNormal,
			},
			expectLog:    `Event(v1.ObjectReference{Kind:"Pod", Namespace:"baz", Name:"foo", UID:"differentUid", APIVersion:"v1", ResourceVersion:"", FieldPath:"spec.containers[3]"}): type: 'Normal' reason: 'Stopped' some verbose message: 1`,
			expectUpdate: false,
		},
		{
			obj:        testRef2,
			eventtype:  v1.EventTypeNormal,
			reason:     "Stopped",
			messageFmt: "some verbose message: %v",
			elements:   []interface{}{1},
			expect: &v1.Event{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "foo",
					Namespace: "baz",
				},
				InvolvedObject: v1.ObjectReference{
					Kind:       "Pod",
					Name:       "foo",
					Namespace:  "baz",
					UID:        "differentUid",
					APIVersion: "v1",
					FieldPath:  "spec.containers[3]",
				},
				Reason:  "Stopped",
				Message: "some verbose message: 1",
				Source:  v1.EventSource{Component: "eventTest"},
				Count:   2,
				Type:    v1.EventTypeNormal,
			},
			expectLog:    `Event(v1.ObjectReference{Kind:"Pod", Namespace:"baz", Name:"foo", UID:"differentUid", APIVersion:"v1", ResourceVersion:"", FieldPath:"spec.containers[3]"}): type: 'Normal' reason: 'Stopped' some verbose message: 1`,
			expectUpdate: true,
		},
	}

	testCache := map[string]*v1.Event{}
	logCalled := make(chan struct{})
	createEvent := make(chan *v1.Event)
	updateEvent := make(chan *v1.Event)
	patchEvent := make(chan *v1.Event)
	testEvents := testEventSink{
		OnCreate: OnCreateFactory(testCache, createEvent),
		OnUpdate: func(event *v1.Event) (*v1.Event, error) {
			updateEvent <- event
			return event, nil
		},
		OnPatch: OnPatchFactory(testCache, patchEvent),
	}
	eventBroadcaster := NewBroadcasterForTests(0)
	ctx, cancelRecording := concurrent.CompleteWithError(context.Background(), func(ctx context.Context) error {
		return eventBroadcaster.StartRecordingToSink(ctx, &testEvents)
	})

	clock := testclocks.NewFakeClock(time.Now())
	recorder := recorderWithFakeClock(t, v1.EventSource{Component: "eventTest"}, eventBroadcaster, clock)
	for index, item := range table {
		clock.Step(1 * time.Second)
		_, cancelLogging := concurrent.CompleteWithError(ctx, func(ctx context.Context) error {
			return eventBroadcaster.StartLogging(ctx, func(formatter string, args ...interface{}) {
				if e, a := item.expectLog, fmt.Sprintf(formatter, args...); e != a {
					t.Errorf("Expected '%v', got '%v'", e, a)
				}
				logCalled <- struct{}{}
			})
		})

		recorder.Eventf(item.obj, item.eventtype, item.reason, item.messageFmt, item.elements...)

		<-logCalled

		// validate event
		if item.expectUpdate {
			actualEvent := <-patchEvent
			validateEvent(strconv.Itoa(index), actualEvent, item.expect, t)
		} else {
			actualEvent := <-createEvent
			validateEvent(strconv.Itoa(index), actualEvent, item.expect, t)
		}
		cancelLogging()
	}
	cancelRecording()
}

func recorderWithFakeClock(t testing.TB, eventSource v1.EventSource, eventBroadcaster EventBroadcaster, clock clock.Clock) EventRecorder {
	return &recorderImpl{newKubeScheme(t), eventSource, eventBroadcaster.(*eventBroadcasterImpl).BoundedWatcher, clock}
}

func TestWriteEventError(t *testing.T) {
	type entry struct {
		timesToSendError int
		attemptsWanted   int
		err              error
	}
	table := map[string]*entry{
		"giveUp1": {
			timesToSendError: 1000,
			attemptsWanted:   1,
			err:              &restclient.RequestConstructionError{},
		},
		"giveUp2": {
			timesToSendError: 1000,
			attemptsWanted:   1,
			err:              &errors.StatusError{},
		},
		"retry1": {
			timesToSendError: 1000,
			attemptsWanted:   12,
			err:              &errors.UnexpectedObjectError{},
		},
		"retry2": {
			timesToSendError: 1000,
			attemptsWanted:   12,
			err:              fmt.Errorf("A weird error"),
		},
		"succeedEventually": {
			timesToSendError: 2,
			attemptsWanted:   2,
			err:              fmt.Errorf("A weird error"),
		},
	}

	clock := testclocks.SimpleIntervalClock{Time: time.Now(), Duration: time.Second}
	eventCorrelator := NewEventCorrelator(&clock)

	for caseName, ent := range table {
		attempts := 0
		sink := &testEventSink{
			OnCreate: func(event *v1.Event) (*v1.Event, error) {
				attempts++
				if attempts < ent.timesToSendError {
					return nil, ent.err
				}
				return event, nil
			},
		}
		ev := &v1.Event{}
		recordToSink(sink, ev, eventCorrelator, 0)
		if attempts != ent.attemptsWanted {
			t.Errorf("case %v: wanted %d, got %d attempts", caseName, ent.attemptsWanted, attempts)
		}
	}
}

func TestUpdateExpiredEvent(t *testing.T) {
	clock := testclocks.SimpleIntervalClock{Time: time.Now(), Duration: time.Second}
	eventCorrelator := NewEventCorrelator(&clock)

	var createdEvent *v1.Event

	sink := &testEventSink{
		OnPatch: func(*v1.Event, []byte) (*v1.Event, error) {
			return nil, &errors.StatusError{
				ErrStatus: metav1.Status{
					Code:   http.StatusNotFound,
					Reason: metav1.StatusReasonNotFound,
				}}
		},
		OnCreate: func(event *v1.Event) (*v1.Event, error) {
			createdEvent = event
			return event, nil
		},
	}

	ev := &v1.Event{}
	ev.ResourceVersion = "updated-resource-version"
	ev.Count = 2
	recordToSink(sink, ev, eventCorrelator, 0)

	if createdEvent == nil {
		t.Error("Event did not get created after patch failed")
		return
	}

	if createdEvent.ResourceVersion != "" {
		t.Errorf("Event did not have its resource version cleared, was %s", createdEvent.ResourceVersion)
	}
}

func TestLotsOfEvents(t *testing.T) {
	recorderCalled := make(chan struct{})
	loggerCalled := make(chan struct{})

	// Fail each event a few times to ensure there's some load on the tested code.
	var counts [1000]int
	testEvents := testEventSink{
		OnCreate: func(event *v1.Event) (*v1.Event, error) {
			num, err := strconv.Atoi(event.Message)
			if err != nil {
				t.Error(err)
				return event, nil
			}
			counts[num]++
			if counts[num] < 5 {
				return nil, fmt.Errorf("fake error")
			}
			recorderCalled <- struct{}{}
			return event, nil
		},
	}

	eventBroadcaster := NewBroadcasterForTests(0)
	_, cancelRecording := concurrent.CompleteWithError(context.Background(), func(ctx context.Context) error {
		return eventBroadcaster.StartRecordingToSink(ctx, &testEvents)
	})
	_, cancelLogging := concurrent.CompleteWithError(context.Background(), func(ctx context.Context) error {
		return eventBroadcaster.StartLogging(ctx, func(formatter string, args ...interface{}) {
			loggerCalled <- struct{}{}
		})
	})
	recorder := eventBroadcaster.NewRecorder(newKubeScheme(t), v1.EventSource{Component: "eventTest"})
	for i := 0; i < maxQueuedEvents; i++ {
		// we want a unique object to stop spam filtering
		ref := &v1.ObjectReference{
			Kind:       "Pod",
			Name:       fmt.Sprintf("foo-%v", i),
			Namespace:  "baz",
			UID:        "bar",
			APIVersion: "version",
		}
		// we need to vary the reason to prevent aggregation
		go recorder.Eventf(ref, v1.EventTypeNormal, "Reason-"+strconv.Itoa(i), strconv.Itoa(i))
	}
	// Make sure no events were dropped by either of the listeners.
	for i := 0; i < maxQueuedEvents; i++ {
		<-recorderCalled
		<-loggerCalled
	}
	// Make sure that every event was attempted 5 times
	for i := 0; i < maxQueuedEvents; i++ {
		if counts[i] < 5 {
			t.Errorf("Only attempted to record event '%d' %d times.", i, counts[i])
		}
	}
	cancelRecording()
	cancelLogging()
}

func TestEventfNoNamespace(t *testing.T) {
	testPod := &v1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name: "foo",
			UID:  "bar",
		},
	}
	testRef, err := ref.GetPartialReference(newKubeScheme(t), testPod, "spec.containers[2]")
	if err != nil {
		t.Fatal(err)
	}
	table := []struct {
		obj          k8sruntime.Object
		eventtype    string
		reason       string
		messageFmt   string
		elements     []interface{}
		expect       *v1.Event
		expectLog    string
		expectUpdate bool
	}{
		{
			obj:        testRef,
			eventtype:  v1.EventTypeNormal,
			reason:     "Started",
			messageFmt: "some verbose message: %v",
			elements:   []interface{}{1},
			expect: &v1.Event{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "foo",
					Namespace: "default",
				},
				InvolvedObject: v1.ObjectReference{
					Kind:       "Pod",
					Name:       "foo",
					Namespace:  "",
					UID:        "bar",
					APIVersion: "v1",
					FieldPath:  "spec.containers[2]",
				},
				Reason:  "Started",
				Message: "some verbose message: 1",
				Source:  v1.EventSource{Component: "eventTest"},
				Count:   1,
				Type:    v1.EventTypeNormal,
			},
			expectLog:    `Event(v1.ObjectReference{Kind:"Pod", Namespace:"", Name:"foo", UID:"bar", APIVersion:"v1", ResourceVersion:"", FieldPath:"spec.containers[2]"}): type: 'Normal' reason: 'Started' some verbose message: 1`,
			expectUpdate: false,
		},
	}

	testCache := map[string]*v1.Event{}
	logCalled := make(chan struct{})
	createEvent := make(chan *v1.Event)
	updateEvent := make(chan *v1.Event)
	patchEvent := make(chan *v1.Event)
	testEvents := testEventSink{
		OnCreate: OnCreateFactory(testCache, createEvent),
		OnUpdate: func(event *v1.Event) (*v1.Event, error) {
			updateEvent <- event
			return event, nil
		},
		OnPatch: OnPatchFactory(testCache, patchEvent),
	}
	eventBroadcaster := NewBroadcasterForTests(0)
	ctx, cancelRecording := concurrent.CompleteWithError(context.Background(), func(ctx context.Context) error {
		return eventBroadcaster.StartRecordingToSink(ctx, &testEvents)
	})

	clock := testclocks.NewFakeClock(time.Now())
	recorder := recorderWithFakeClock(t, v1.EventSource{Component: "eventTest"}, eventBroadcaster, clock)

	for index, item := range table {
		clock.Step(1 * time.Second)
		_, cancelLogging := concurrent.CompleteWithError(ctx, func(ctx context.Context) error {
			return eventBroadcaster.StartLogging(ctx, func(formatter string, args ...interface{}) {
				if e, a := item.expectLog, fmt.Sprintf(formatter, args...); e != a {
					t.Errorf("Expected '%v', got '%v'", e, a)
				}
				logCalled <- struct{}{}
			})
		})
		recorder.Eventf(item.obj, item.eventtype, item.reason, item.messageFmt, item.elements...)

		<-logCalled

		// validate event
		if item.expectUpdate {
			actualEvent := <-patchEvent
			validateEvent(strconv.Itoa(index), actualEvent, item.expect, t)
		} else {
			actualEvent := <-createEvent
			validateEvent(strconv.Itoa(index), actualEvent, item.expect, t)
		}

		cancelLogging()
	}
	cancelRecording()
}

func TestMultiSinkCache(t *testing.T) {
	testPod := &v1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "foo",
			Namespace: "baz",
			UID:       "bar",
		},
	}
	testPod2 := &v1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "foo",
			Namespace: "baz",
			UID:       "differentUid",
		},
	}
	testRef, err := ref.GetPartialReference(newKubeScheme(t), testPod, "spec.containers[2]")
	if err != nil {
		t.Fatal(err)
	}
	testRef2, err := ref.GetPartialReference(newKubeScheme(t), testPod2, "spec.containers[3]")
	if err != nil {
		t.Fatal(err)
	}
	table := []struct {
		obj          k8sruntime.Object
		eventtype    string
		reason       string
		messageFmt   string
		elements     []interface{}
		expect       *v1.Event
		expectLog    string
		expectUpdate bool
	}{
		{
			obj:        testRef,
			eventtype:  v1.EventTypeNormal,
			reason:     "Started",
			messageFmt: "some verbose message: %v",
			elements:   []interface{}{1},
			expect: &v1.Event{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "foo",
					Namespace: "baz",
				},
				InvolvedObject: v1.ObjectReference{
					Kind:       "Pod",
					Name:       "foo",
					Namespace:  "baz",
					UID:        "bar",
					APIVersion: "v1",
					FieldPath:  "spec.containers[2]",
				},
				Reason:  "Started",
				Message: "some verbose message: 1",
				Source:  v1.EventSource{Component: "eventTest"},
				Count:   1,
				Type:    v1.EventTypeNormal,
			},
			expectLog:    `Event(v1.ObjectReference{Kind:"Pod", Namespace:"baz", Name:"foo", UID:"bar", APIVersion:"v1", ResourceVersion:"", FieldPath:"spec.containers[2]"}): type: 'Normal' reason: 'Started' some verbose message: 1`,
			expectUpdate: false,
		},
		{
			obj:        testPod,
			eventtype:  v1.EventTypeNormal,
			reason:     "Killed",
			messageFmt: "some other verbose message: %v",
			elements:   []interface{}{1},
			expect: &v1.Event{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "foo",
					Namespace: "baz",
				},
				InvolvedObject: v1.ObjectReference{
					Kind:       "Pod",
					Name:       "foo",
					Namespace:  "baz",
					UID:        "bar",
					APIVersion: "v1",
				},
				Reason:  "Killed",
				Message: "some other verbose message: 1",
				Source:  v1.EventSource{Component: "eventTest"},
				Count:   1,
				Type:    v1.EventTypeNormal,
			},
			expectLog:    `Event(v1.ObjectReference{Kind:"Pod", Namespace:"baz", Name:"foo", UID:"bar", APIVersion:"v1", ResourceVersion:"", FieldPath:""}): type: 'Normal' reason: 'Killed' some other verbose message: 1`,
			expectUpdate: false,
		},
		{
			obj:        testRef,
			eventtype:  v1.EventTypeNormal,
			reason:     "Started",
			messageFmt: "some verbose message: %v",
			elements:   []interface{}{1},
			expect: &v1.Event{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "foo",
					Namespace: "baz",
				},
				InvolvedObject: v1.ObjectReference{
					Kind:       "Pod",
					Name:       "foo",
					Namespace:  "baz",
					UID:        "bar",
					APIVersion: "v1",
					FieldPath:  "spec.containers[2]",
				},
				Reason:  "Started",
				Message: "some verbose message: 1",
				Source:  v1.EventSource{Component: "eventTest"},
				Count:   2,
				Type:    v1.EventTypeNormal,
			},
			expectLog:    `Event(v1.ObjectReference{Kind:"Pod", Namespace:"baz", Name:"foo", UID:"bar", APIVersion:"v1", ResourceVersion:"", FieldPath:"spec.containers[2]"}): type: 'Normal' reason: 'Started' some verbose message: 1`,
			expectUpdate: true,
		},
		{
			obj:        testRef2,
			eventtype:  v1.EventTypeNormal,
			reason:     "Started",
			messageFmt: "some verbose message: %v",
			elements:   []interface{}{1},
			expect: &v1.Event{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "foo",
					Namespace: "baz",
				},
				InvolvedObject: v1.ObjectReference{
					Kind:       "Pod",
					Name:       "foo",
					Namespace:  "baz",
					UID:        "differentUid",
					APIVersion: "v1",
					FieldPath:  "spec.containers[3]",
				},
				Reason:  "Started",
				Message: "some verbose message: 1",
				Source:  v1.EventSource{Component: "eventTest"},
				Count:   1,
				Type:    v1.EventTypeNormal,
			},
			expectLog:    `Event(v1.ObjectReference{Kind:"Pod", Namespace:"baz", Name:"foo", UID:"differentUid", APIVersion:"v1", ResourceVersion:"", FieldPath:"spec.containers[3]"}): type: 'Normal' reason: 'Started' some verbose message: 1`,
			expectUpdate: false,
		},
		{
			obj:        testRef,
			eventtype:  v1.EventTypeNormal,
			reason:     "Started",
			messageFmt: "some verbose message: %v",
			elements:   []interface{}{1},
			expect: &v1.Event{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "foo",
					Namespace: "baz",
				},
				InvolvedObject: v1.ObjectReference{
					Kind:       "Pod",
					Name:       "foo",
					Namespace:  "baz",
					UID:        "bar",
					APIVersion: "v1",
					FieldPath:  "spec.containers[2]",
				},
				Reason:  "Started",
				Message: "some verbose message: 1",
				Source:  v1.EventSource{Component: "eventTest"},
				Count:   3,
				Type:    v1.EventTypeNormal,
			},
			expectLog:    `Event(v1.ObjectReference{Kind:"Pod", Namespace:"baz", Name:"foo", UID:"bar", APIVersion:"v1", ResourceVersion:"", FieldPath:"spec.containers[2]"}): type: 'Normal' reason: 'Started' some verbose message: 1`,
			expectUpdate: true,
		},
		{
			obj:        testRef2,
			eventtype:  v1.EventTypeNormal,
			reason:     "Stopped",
			messageFmt: "some verbose message: %v",
			elements:   []interface{}{1},
			expect: &v1.Event{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "foo",
					Namespace: "baz",
				},
				InvolvedObject: v1.ObjectReference{
					Kind:       "Pod",
					Name:       "foo",
					Namespace:  "baz",
					UID:        "differentUid",
					APIVersion: "v1",
					FieldPath:  "spec.containers[3]",
				},
				Reason:  "Stopped",
				Message: "some verbose message: 1",
				Source:  v1.EventSource{Component: "eventTest"},
				Count:   1,
				Type:    v1.EventTypeNormal,
			},
			expectLog:    `Event(v1.ObjectReference{Kind:"Pod", Namespace:"baz", Name:"foo", UID:"differentUid", APIVersion:"v1", ResourceVersion:"", FieldPath:"spec.containers[3]"}): type: 'Normal' reason: 'Stopped' some verbose message: 1`,
			expectUpdate: false,
		},
		{
			obj:        testRef2,
			eventtype:  v1.EventTypeNormal,
			reason:     "Stopped",
			messageFmt: "some verbose message: %v",
			elements:   []interface{}{1},
			expect: &v1.Event{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "foo",
					Namespace: "baz",
				},
				InvolvedObject: v1.ObjectReference{
					Kind:       "Pod",
					Name:       "foo",
					Namespace:  "baz",
					UID:        "differentUid",
					APIVersion: "v1",
					FieldPath:  "spec.containers[3]",
				},
				Reason:  "Stopped",
				Message: "some verbose message: 1",
				Source:  v1.EventSource{Component: "eventTest"},
				Count:   2,
				Type:    v1.EventTypeNormal,
			},
			expectLog:    `Event(v1.ObjectReference{Kind:"Pod", Namespace:"baz", Name:"foo", UID:"differentUid", APIVersion:"v1", ResourceVersion:"", FieldPath:"spec.containers[3]"}): type: 'Normal' reason: 'Stopped' some verbose message: 1`,
			expectUpdate: true,
		},
	}

	testCache := map[string]*v1.Event{}
	createEvent := make(chan *v1.Event)
	updateEvent := make(chan *v1.Event)
	patchEvent := make(chan *v1.Event)
	testEvents := testEventSink{
		OnCreate: OnCreateFactory(testCache, createEvent),
		OnUpdate: func(event *v1.Event) (*v1.Event, error) {
			updateEvent <- event
			return event, nil
		},
		OnPatch: OnPatchFactory(testCache, patchEvent),
	}

	testCache2 := map[string]*v1.Event{}
	createEvent2 := make(chan *v1.Event)
	updateEvent2 := make(chan *v1.Event)
	patchEvent2 := make(chan *v1.Event)
	testEvents2 := testEventSink{
		OnCreate: OnCreateFactory(testCache2, createEvent2),
		OnUpdate: func(event *v1.Event) (*v1.Event, error) {
			updateEvent2 <- event
			return event, nil
		},
		OnPatch: OnPatchFactory(testCache2, patchEvent2),
	}

	eventBroadcaster := NewBroadcasterForTests(0)
	clock := testclocks.NewFakeClock(time.Now())
	recorder := recorderWithFakeClock(t, v1.EventSource{Component: "eventTest"}, eventBroadcaster, clock)

	_, cancelRecording := concurrent.CompleteWithError(context.Background(), func(ctx context.Context) error {
		return eventBroadcaster.StartRecordingToSink(ctx, &testEvents)
	})
	for index, item := range table {
		clock.Step(1 * time.Second)
		recorder.Eventf(item.obj, item.eventtype, item.reason, item.messageFmt, item.elements...)

		// validate event
		if item.expectUpdate {
			actualEvent := <-patchEvent
			validateEvent(strconv.Itoa(index), actualEvent, item.expect, t)
		} else {
			actualEvent := <-createEvent
			validateEvent(strconv.Itoa(index), actualEvent, item.expect, t)
		}
	}

	// Another StartRecordingToSink call should start to record events with new clean cache.
	_, cancelRecording2 := concurrent.CompleteWithError(context.Background(), func(ctx context.Context) error {
		return eventBroadcaster.StartRecordingToSink(ctx, &testEvents2)
	})
	for index, item := range table {
		clock.Step(1 * time.Second)
		recorder.Eventf(item.obj, item.eventtype, item.reason, item.messageFmt, item.elements...)

		// validate event
		if item.expectUpdate {
			actualEvent := <-patchEvent2
			validateEvent(strconv.Itoa(index), actualEvent, item.expect, t)
		} else {
			actualEvent := <-createEvent2
			validateEvent(strconv.Itoa(index), actualEvent, item.expect, t)
		}
	}

	cancelRecording()
	cancelRecording2()
}
