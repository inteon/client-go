/*
Copyright 2015 The Kubernetes Authors.

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

package cache

import (
	"context"
	"errors"
	"sync"
	"time"

	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/utils/clock"
)

// This file implements a low-level controller that is used in
// sharedIndexInformer, which is an implementation of
// SharedIndexInformer.  Such informers, in turn, are key components
// in the high level controllers that form the backbone of the
// Kubernetes control plane.  Look at those for examples, or the
// example in
// https://github.com/kubernetes/client-go/tree/master/examples/workqueue
// .

// Config contains all the settings for one of these low-level controllers.
type Config struct {
	// The queue for your objects - has to be a DeltaFIFO due to
	// assumptions in the implementation. Your Process() function
	// should accept the output of this Queue's Pop() method.
	Queue

	// Something that can list and watch your objects.
	ListerWatcher

	// Something that can process a popped Deltas.
	Process ProcessFunc

	// ObjectType is an example object of the type this controller is
	// expected to handle.  Only the type needs to be right, except
	// that when that is `unstructured.Unstructured` the object's
	// `"apiVersion"` and `"kind"` must also be right.
	ObjectType runtime.Object

	// Called whenever the ListAndWatch drops the connection with an error.
	WatchErrorHandler WatchErrorHandler

	// WatchListPageSize is the requested chunk size of initial and relist watch lists.
	WatchListPageSize int64
}

// ProcessFunc processes a single object.
type ProcessFunc func(ctx context.Context, obj interface{}) error

// `*controller` implements Controller
type controller struct {
	config         Config
	reflector      *Reflector
	reflectorMutex sync.RWMutex
	clock          clock.Clock
}

// Controller is a low-level controller that is parameterized by a
// Config and used in sharedIndexInformer.
type Controller interface {
	// Run does two things.  One is to construct and run a Reflector
	// to pump objects/notifications from the Config's ListerWatcher
	// to the Config's Queue and possibly invoke the occasional Resync
	// on that Queue.  The other is to repeatedly Pop from the Queue
	// and process with the Config's ProcessFunc.  Both of these
	// continue until `stopCh` is closed.
	Run(ctx context.Context)

	// HasSynced delegates to the Config's Queue
	HasSynced() <-chan struct{}

	// LastSyncResourceVersion delegates to the Reflector when there
	// is one, otherwise returns the empty string
	LastSyncResourceVersion() string
}

// New makes a new Controller from the given Config.
func New(c *Config) Controller {
	ctlr := &controller{
		config: *c,
		clock:  &clock.RealClock{},
	}
	return ctlr
}

// Run begins processing items, and will continue until a value is sent down stopCh or it is closed.
// It's an error to call Run more than once.
// Run blocks; call via go.
func (c *controller) Run(ctx context.Context) {
	defer utilruntime.HandleCrash()
	go func() {
		<-ctx.Done()
		c.config.Queue.Close()
	}()

	r := NewReflector(
		c.config.ListerWatcher,
		c.config.ObjectType,
		c.config.Queue,
	)
	r.WatchListPageSize = c.config.WatchListPageSize
	r.clock = c.clock
	if c.config.WatchErrorHandler != nil {
		r.watchErrorHandler = c.config.WatchErrorHandler
	}

	c.reflectorMutex.Lock()
	c.reflector = r
	c.reflectorMutex.Unlock()

	var wg wait.Group

	wg.StartWithContext(ctx, r.Run)

	wait.UntilWithContext(ctx, c.processLoop, time.Second)
	wg.Wait()
}

// Returns true once this controller has completed an initial resource listing
func (c *controller) HasSynced() <-chan struct{} {
	return c.config.Queue.HasSynced()
}

func (c *controller) LastSyncResourceVersion() string {
	c.reflectorMutex.RLock()
	defer c.reflectorMutex.RUnlock()
	if c.reflector == nil {
		return ""
	}
	return c.reflector.LastSyncResourceVersion()
}

// processLoop drains the work queue.
// TODO: Consider doing the processing in parallel. This will require a little thought
// to make sure that we don't end up processing the same object multiple times
// concurrently.
func (c *controller) processLoop(ctx context.Context) {
	for {
		_, err := c.config.Queue.PopAndProcess(ctx, PopFunc(c.config.Process))
		if err != nil {
			if err == ErrFIFOClosed {
				return
			}
		}
	}
}

// ResourceEventHandler can handle notifications for events that
// happen to a resource. The events are informational only, so you
// can't return an error.  The handlers MUST NOT modify the objects
// received; this concerns not only the top level of structure but all
// the data structures reachable from it.
//  * OnAdd is called when an object is added.
//  * OnUpdate is called when an object is modified. Note that oldObj is the
//      last known state of the object-- it is possible that several changes
//      were combined together, so you can't use this to see every single
//      change. OnUpdate is also called when a re-list happens, and it will
//      get called even if nothing changed. This is useful for periodically
//      evaluating or syncing something.
//  * OnDelete will get the final state of the item if it is known, otherwise
//      it will get an object of type DeletedFinalStateUnknown. This can
//      happen if the watch is closed and misses the delete event and we don't
//      notice the deletion until the subsequent re-list.
type ResourceEventHandler interface {
	OnAdd(ctx context.Context, obj interface{})
	OnUpdate(ctx context.Context, oldObj, newObj interface{})
	OnDelete(ctx context.Context, obj interface{})
}

// ResourceEventHandlerFuncs is an adaptor to let you easily specify as many or
// as few of the notification functions as you want while still implementing
// ResourceEventHandler.  This adapter does not remove the prohibition against
// modifying the objects.
type ResourceEventHandlerFuncs struct {
	AddFunc    func(ctx context.Context, obj interface{})
	UpdateFunc func(ctx context.Context, oldObj, newObj interface{})
	DeleteFunc func(ctx context.Context, obj interface{})
}

// OnAdd calls AddFunc if it's not nil.
func (r ResourceEventHandlerFuncs) OnAdd(ctx context.Context, obj interface{}) {
	if r.AddFunc != nil {
		r.AddFunc(ctx, obj)
	}
}

// OnUpdate calls UpdateFunc if it's not nil.
func (r ResourceEventHandlerFuncs) OnUpdate(ctx context.Context, oldObj, newObj interface{}) {
	if r.UpdateFunc != nil {
		r.UpdateFunc(ctx, oldObj, newObj)
	}
}

// OnDelete calls DeleteFunc if it's not nil.
func (r ResourceEventHandlerFuncs) OnDelete(ctx context.Context, obj interface{}) {
	if r.DeleteFunc != nil {
		r.DeleteFunc(ctx, obj)
	}
}

// FilteringResourceEventHandler applies the provided filter to all events coming
// in, ensuring the appropriate nested handler method is invoked. An object
// that starts passing the filter after an update is considered an add, and an
// object that stops passing the filter after an update is considered a delete.
// Like the handlers, the filter MUST NOT modify the objects it is given.
type FilteringResourceEventHandler struct {
	FilterFunc func(obj interface{}) bool
	Handler    ResourceEventHandler
}

// OnAdd calls the nested handler only if the filter succeeds
func (r FilteringResourceEventHandler) OnAdd(ctx context.Context, obj interface{}) {
	if !r.FilterFunc(obj) {
		return
	}
	r.Handler.OnAdd(ctx, obj)
}

// OnUpdate ensures the proper handler is called depending on whether the filter matches
func (r FilteringResourceEventHandler) OnUpdate(ctx context.Context, oldObj, newObj interface{}) {
	newer := r.FilterFunc(newObj)
	older := r.FilterFunc(oldObj)
	switch {
	case newer && older:
		r.Handler.OnUpdate(ctx, oldObj, newObj)
	case newer && !older:
		r.Handler.OnAdd(ctx, newObj)
	case !newer && older:
		r.Handler.OnDelete(ctx, oldObj)
	default:
		// do nothing
	}
}

// OnDelete calls the nested handler only if the filter succeeds
func (r FilteringResourceEventHandler) OnDelete(ctx context.Context, obj interface{}) {
	if !r.FilterFunc(obj) {
		return
	}
	r.Handler.OnDelete(ctx, obj)
}

// DeletionHandlingMetaNamespaceKeyFunc checks for
// DeletedFinalStateUnknown objects before calling
// MetaNamespaceKeyFunc.
func DeletionHandlingMetaNamespaceKeyFunc(obj interface{}) (string, error) {
	if d, ok := obj.(DeletedFinalStateUnknown); ok {
		return d.Key, nil
	}
	return MetaNamespaceKeyFunc(obj)
}

// NewInformer returns a Store and a controller for populating the store
// while also providing event notifications. You should only used the returned
// Store for Get/List operations; Add/Modify/Deletes will cause the event
// notifications to be faulty.
//
// Parameters:
//  * lw is list and watch functions for the source of the resource you want to
//    be informed of.
//  * objType is an object of the type that you expect to receive.
//  * resyncPeriod: if non-zero, will re-list this often (you will get OnUpdate
//    calls, even if nothing changed). Otherwise, re-list will be delayed as
//    long as possible (until the upstream source closes the watch or times out,
//    or you stop the controller).
//  * h is the object you want notifications sent to.
//
func NewInformer(
	lw ListerWatcher,
	objType runtime.Object,
	h ResourceEventHandler,
) (Store, Controller) {
	// This will hold the client state, as we know it.
	clientState := NewStore(DeletionHandlingMetaNamespaceKeyFunc)

	return clientState, newInformer(lw, objType, h, clientState, nil)
}

// NewIndexerInformer returns an Indexer and a Controller for populating the index
// while also providing event notifications. You should only used the returned
// Index for Get/List operations; Add/Modify/Deletes will cause the event
// notifications to be faulty.
//
// Parameters:
//  * lw is list and watch functions for the source of the resource you want to
//    be informed of.
//  * objType is an object of the type that you expect to receive.
//  * resyncPeriod: if non-zero, will re-list this often (you will get OnUpdate
//    calls, even if nothing changed). Otherwise, re-list will be delayed as
//    long as possible (until the upstream source closes the watch or times out,
//    or you stop the controller).
//  * h is the object you want notifications sent to.
//  * indexers is the indexer for the received object type.
//
func NewIndexerInformer(
	lw ListerWatcher,
	objType runtime.Object,
	h ResourceEventHandler,
	indexers Indexers,
) (Indexer, Controller) {
	// This will hold the client state, as we know it.
	clientState := NewIndexer(DeletionHandlingMetaNamespaceKeyFunc, indexers)

	return clientState, newInformer(lw, objType, h, clientState, nil)
}

// TransformFunc allows for transforming an object before it will be processed
// and put into the controller cache and before the corresponding handlers will
// be called on it.
// TransformFunc (similarly to ResourceEventHandler functions) should be able
// to correctly handle the tombstone of type cache.DeletedFinalStateUnknown
//
// The most common usage pattern is to clean-up some parts of the object to
// reduce component memory usage if a given component doesn't care about them.
// given controller doesn't care for them
type TransformFunc func(interface{}) (interface{}, error)

// NewTransformingInformer returns a Store and a controller for populating
// the store while also providing event notifications. You should only used
// the returned Store for Get/List operations; Add/Modify/Deletes will cause
// the event notifications to be faulty.
// The given transform function will be called on all objects before they will
// put into the Store and corresponding Add/Modify/Delete handlers will
// be invoked for them.
func NewTransformingInformer(
	lw ListerWatcher,
	objType runtime.Object,
	h ResourceEventHandler,
	transformer TransformFunc,
) (Store, Controller) {
	// This will hold the client state, as we know it.
	clientState := NewStore(DeletionHandlingMetaNamespaceKeyFunc)

	return clientState, newInformer(lw, objType, h, clientState, transformer)
}

// NewTransformingIndexerInformer returns an Indexer and a controller for
// populating the index while also providing event notifications. You should
// only used the returned Index for Get/List operations; Add/Modify/Deletes
// will cause the event notifications to be faulty.
// The given transform function will be called on all objects before they will
// be put into the Index and corresponding Add/Modify/Delete handlers will
// be invoked for them.
func NewTransformingIndexerInformer(
	lw ListerWatcher,
	objType runtime.Object,
	h ResourceEventHandler,
	indexers Indexers,
	transformer TransformFunc,
) (Indexer, Controller) {
	// This will hold the client state, as we know it.
	clientState := NewIndexer(DeletionHandlingMetaNamespaceKeyFunc, indexers)

	return clientState, newInformer(lw, objType, h, clientState, transformer)
}

// Multiplexes updates in the form of a list of Deltas into a Store, and informs
// a given handler of events OnUpdate, OnAdd, OnDelete
func processDeltas(
	ctx context.Context,
	// Object which receives event notifications from the given deltas
	handler ResourceEventHandler,
	clientState Store,
	transformer TransformFunc,
	deltas Deltas,
) error {
	// from oldest to newest
	for _, d := range deltas {
		obj := d.Object
		if transformer != nil {
			var err error
			obj, err = transformer(obj)
			if err != nil {
				return err
			}
		}

		switch d.Type {
		case Sync, Replaced, Added, Updated:
			if old, exists, err := clientState.Get(obj); err == nil && exists {
				if err := clientState.Update(ctx, obj); err != nil {
					return err
				}
				handler.OnUpdate(ctx, old, obj)
			} else {
				if err := clientState.Add(ctx, obj); err != nil {
					return err
				}
				handler.OnAdd(ctx, obj)
			}
		case Deleted:
			if err := clientState.Delete(ctx, obj); err != nil {
				return err
			}
			handler.OnDelete(ctx, obj)
		}
	}
	return nil
}

// newInformer returns a controller for populating the store while also
// providing event notifications.
//
// Parameters
//  * lw is list and watch functions for the source of the resource you want to
//    be informed of.
//  * objType is an object of the type that you expect to receive.
//  * resyncPeriod: if non-zero, will re-list this often (you will get OnUpdate
//    calls, even if nothing changed). Otherwise, re-list will be delayed as
//    long as possible (until the upstream source closes the watch or times out,
//    or you stop the controller).
//  * h is the object you want notifications sent to.
//  * clientState is the store you want to populate
//
func newInformer(
	lw ListerWatcher,
	objType runtime.Object,
	h ResourceEventHandler,
	clientState Store,
	transformer TransformFunc,
) Controller {
	// This will hold incoming changes. Note how we pass clientState in as a
	// KeyLister, that way resync operations will result in the correct set
	// of update/delete deltas.
	fifo := NewDeltaFIFO(KnownObjectsQueueOption(clientState))

	cfg := &Config{
		Queue:         fifo,
		ListerWatcher: lw,
		ObjectType:    objType,

		Process: func(ctx context.Context, obj interface{}) error {
			if deltas, ok := obj.(Deltas); ok {
				return processDeltas(ctx, h, clientState, transformer, deltas)
			}
			return errors.New("object given as Process argument is not Deltas")
		},
	}
	return New(cfg)
}
