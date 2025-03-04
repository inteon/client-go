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

package cache

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"k8s.io/klog/v2"
	utiltrace "k8s.io/utils/trace"
)

// DeltaFIFO is like FIFO, but differs in two ways.  One is that the
// accumulator associated with a given object's key is not that object
// but rather a Deltas, which is a slice of Delta values for that
// object.  Applying an object to a Deltas means to append a Delta
// except when the potentially appended Delta is a Deleted and the
// Deltas already ends with a Deleted.  In that case the Deltas does
// not grow, although the terminal Deleted will be replaced by the new
// Deleted if the older Deleted's object is a
// DeletedFinalStateUnknown.
//
// DeltaFIFO is a producer-consumer queue, where a Reflector is
// intended to be the producer, and the consumer is whatever calls
// the Pop() method.
//
// DeltaFIFO solves this use case:
//  * You want to process every object change (delta) at most once.
//  * When you process an object, you want to see everything
//    that's happened to it since you last processed it.
//  * You want to process the deletion of some of the objects.
//  * You might want to periodically reprocess objects.
//
// DeltaFIFO's Pop(), Get(), and GetByKey() methods return
// interface{} to satisfy the Store/Queue interfaces, but they
// will always return an object of type Deltas. List() returns
// the newest object from each accumulator in the FIFO.
//
// A DeltaFIFO's knownObjects KeyListerGetter provides the abilities
// to list Store keys and to get objects by Store key.  The objects in
// question are called "known objects" and this set of objects
// modifies the behavior of the Delete, Replace, and Resync methods
// (each in a different way).
//
// A note on threading: If you call Pop() in parallel from multiple
// threads, you could end up with multiple threads processing slightly
// different versions of the same object.
type DeltaFIFO struct {
	// lock/cond protects access to 'items' and 'queue'.
	lock sync.RWMutex

	// `items` maps a key to a Deltas.
	// Each such Deltas has at least one Delta.
	items map[string]Deltas

	// `queue` maintains FIFO order of keys for consumption in Pop().
	// There are no duplicates in `queue`.
	// A key is in `queue` if and only if it is in `items`.
	queue chan string

	// populated is true if the first batch of items inserted by Replace() has been populated
	// or Delete/Add/Update/AddIfNotPresent was called first.
	populated bool
	// initialPopulationCount is the number of items inserted by the first call of Replace()
	initialPopulationCount int
	// channel that is closed when the hasSynced condition completed
	hasSyncedChannel chan struct{}

	// keyFunc is used to make the key used for queued item
	// insertion and retrieval, and should be deterministic.
	keyFunc KeyFunc

	// knownObjects list keys that are "known" --- affecting Delete(),
	// Replace(), and Resync()
	knownObjects KeyListerGetter

	// Used to indicate a queue is closed so a control loop can exit when a queue is empty.
	// Currently, not used to gate any of CRUD operations.
	closed bool
}

// DeltaType is the type of a change (addition, deletion, etc)
type DeltaType string

// Change type definition
const (
	Added   DeltaType = "Added"
	Updated DeltaType = "Updated"
	Deleted DeltaType = "Deleted"
	// Replaced is emitted when we encountered watch errors and had to do a
	// relist. We don't know if the replaced object has changed.
	Replaced DeltaType = "Replaced"
	// Sync is for synthetic events during a periodic resync.
	Sync DeltaType = "Sync"
)

// Delta is a member of Deltas (a list of Delta objects) which
// in its turn is the type stored by a DeltaFIFO. It tells you what
// change happened, and the object's state after* that change.
//
// [*] Unless the change is a deletion, and then you'll get the final
// state of the object before it was deleted.
type Delta struct {
	Type   DeltaType
	Object interface{}
}

// Deltas is a list of one or more 'Delta's to an individual object.
// The oldest delta is at index 0, the newest delta is the last one.
type Deltas []Delta

// NewDeltaFIFO returns a Queue which can be used to process changes to items.
//
// keyFunc is used to figure out what key an object should have. (It is
// exposed in the returned DeltaFIFO's KeyOf() method, with additional handling
// around deleted objects and queue state).
//
// 'knownObjects' may be supplied to modify the behavior of Delete,
// Replace, and Resync.  It may be nil if you do not need those
// modifications.
//
// TODO: consider merging keyLister with this object, tracking a list of
// "known" keys when Pop() is called. Have to think about how that
// affects error retrying.
//
//       NOTE: It is possible to misuse this and cause a race when using an
//       external known object source.
//       Whether there is a potential race depends on how the consumer
//       modifies knownObjects. In Pop(), process function is called under
//       lock, so it is safe to update data structures in it that need to be
//       in sync with the queue (e.g. knownObjects).
//
//       Example:
//       In case of sharedIndexInformer being a consumer
//       (https://github.com/kubernetes/kubernetes/blob/0cdd940f/staging/src/k8s.io/client-go/tools/cache/shared_informer.go#L192),
//       there is no race as knownObjects (s.indexer) is modified safely
//       under DeltaFIFO's lock. The only exceptions are GetStore() and
//       GetIndexer() methods, which expose ways to modify the underlying
//       storage. Currently these two methods are used for creating Lister
//       and internal tests.
//
// Also see the comment on DeltaFIFO.
//
// NewDeltaFIFO returns a Queue which can be used to process changes to
// items. See also the comment on DeltaFIFO.
func NewDeltaFIFO(options ...QueueOption) *DeltaFIFO {
	queueOptions := NewQueueOptions(options...)

	return &DeltaFIFO{
		items:            map[string]Deltas{},
		queue:            make(chan string, queueOptions.QueueSize),
		keyFunc:          queueOptions.KeyFunction,
		knownObjects:     queueOptions.KnownObjects,
		hasSyncedChannel: nil,
	}
}

var _ Queue = &DeltaFIFO{} // DeltaFIFO is a Queue

var (
	// ErrZeroLengthDeltasObject is returned in a KeyError if a Deltas
	// object with zero length is encountered (should be impossible,
	// but included for completeness).
	ErrZeroLengthDeltasObject = errors.New("0 length Deltas object; can't get key")
)

// Close the queue.
func (f *DeltaFIFO) Close() {
	f.lock.Lock()
	defer f.lock.Unlock()

	if !f.closed {
		close(f.queue)
	}

	f.closed = true
}

// KeyOf exposes f's keyFunc, but also detects the key of a Deltas object or
// DeletedFinalStateUnknown objects.
func (f *DeltaFIFO) KeyOf(obj interface{}) (string, error) {
	if d, ok := obj.(Deltas); ok {
		if len(d) == 0 {
			return "", KeyError{obj, ErrZeroLengthDeltasObject}
		}
		obj = d.Newest().Object
	}
	if d, ok := obj.(DeletedFinalStateUnknown); ok {
		return d.Key, nil
	}
	return f.keyFunc(obj)
}

// HasSynced returns true if an Add/Update/Delete/AddIfNotPresent are called first,
// or the first batch of items inserted by Replace() has been popped.
func (f *DeltaFIFO) HasSynced() <-chan struct{} {
	f.lock.Lock()
	defer f.lock.Unlock()

	if f.populated && f.initialPopulationCount == 0 {
		return ClosedChannel
	}

	f.hasSyncedChannel = make(chan struct{})
	return f.hasSyncedChannel
}

func (f *DeltaFIFO) updateItemLocked(id string, actionType DeltaType, obj interface{}) error {
	oldDeltas := f.items[id]
	newDeltas := append(oldDeltas, Delta{actionType, obj})
	newDeltas = dedupDeltas(newDeltas)
	if len(newDeltas) == 0 {
		// This never happens, because dedupDeltas never returns an empty list
		// when given a non-empty list (as it is here).
		// If somehow it happens anyway, deal with it but complain.
		if oldDeltas == nil {
			klog.Errorf("impossible dedupDeltas for id=%q: oldDeltas=%#+v, obj=%#+v; ignoring", id, oldDeltas, obj)
			return nil
		}
		klog.Errorf("impossible dedupDeltas for id=%q: oldDeltas=%#+v, obj=%#+v; breaking invariant by storing empty Deltas", id, oldDeltas, obj)
		f.items[id] = newDeltas
		return fmt.Errorf("impossible dedupDeltas for id=%q: oldDeltas=%#+v, obj=%#+v; broke DeltaFIFO invariant by storing empty Deltas", id, oldDeltas, obj)
	}
	f.items[id] = newDeltas

	return nil
}

// queueAction appends to the delta list for the object.
// Caller must lock first.
func (f *DeltaFIFO) queueAction(ctx context.Context, actionType DeltaType, obj interface{}) error {
	id, err := f.KeyOf(obj)
	if err != nil {
		return KeyError{obj, err}
	}

	done, err := func() (bool, error) {
		f.lock.Lock()
		defer f.lock.Unlock()

		f.populated = true
		_, exists := f.items[id]
		return exists, f.updateItemLocked(id, actionType, obj)
	}()

	if done || err != nil {
		return err
	}

	select {
	case f.queue <- id:
	case <-ctx.Done():
		return ctx.Err()
	}

	return nil
}

// Add inserts an item, and puts it in the queue. The item is only enqueued
// if it doesn't already exist in the set.
func (f *DeltaFIFO) Add(ctx context.Context, obj interface{}) error {
	return f.queueAction(ctx, Added, obj)
}

// AddIfNotPresent inserts an item, and puts it in the queue. If the item is already
// present in the set, it is neither enqueued nor added to the set.
//
// This is useful in a single producer/consumer scenario so that the consumer can
// safely retry items without contending with the producer and potentially enqueueing
// stale items.
//
// Important: obj must be a Deltas (the output of the Pop() function). Yes, this is
// different from the Add/Update/Delete functions.
func (f *DeltaFIFO) AddIfNotPresent(ctx context.Context, obj interface{}) error {
	deltas, ok := obj.(Deltas)
	if !ok {
		return fmt.Errorf("object must be of type deltas, but got: %#v", obj)
	}

	id, err := f.KeyOf(deltas)
	if err != nil {
		return KeyError{obj, err}
	}

	done := func() bool {
		f.lock.Lock()
		defer f.lock.Unlock()

		f.populated = true
		if _, exists := f.items[id]; exists {
			return true // don't update queue or itemset
		} else {
			f.items[id] = deltas
			return false
		}
	}()

	if done {
		return nil
	}

	select {
	case f.queue <- id:
	case <-ctx.Done():
		return ctx.Err()
	}

	return nil
}

// Update is just like Add, but makes an Updated Delta.
func (f *DeltaFIFO) Update(ctx context.Context, obj interface{}) error {
	return f.queueAction(ctx, Updated, obj)
}

// Delete is just like Add, but makes a Deleted Delta. If the given
// object does not already exist, it will be ignored. (It may have
// already been deleted by a Replace (re-list), for example.)  In this
// method `f.knownObjects`, if not nil, provides (via GetByKey)
// _additional_ objects that are considered to already exist.
func (f *DeltaFIFO) Delete(ctx context.Context, obj interface{}) error {
	id, err := f.KeyOf(obj)
	if err != nil {
		return KeyError{obj, err}
	}

	done, err := func() (bool, error) {
		f.lock.Lock()
		defer f.lock.Unlock()

		f.populated = true
		_, exists := f.items[id]
		existsInKnown := false

		if f.knownObjects != nil {
			_, found, err := f.knownObjects.GetByKey(id)
			existsInKnown = found && err == nil
		}

		if !exists && !existsInKnown {
			return true, nil
		}

		if err := f.updateItemLocked(id, Deleted, obj); err != nil {
			return true, err
		}

		return false, nil
	}()

	if done || err != nil {
		return nil
	}

	select {
	case f.queue <- id:
	case <-ctx.Done():
		return ctx.Err()
	}

	return nil
}

// Replace atomically does two things: (1) it adds the given objects
// using the Sync or Replace DeltaType and then (2) it does some deletions.
// In particular: for every pre-existing key K that is not the key of
// an object in `list` there is the effect of
// `Delete(DeletedFinalStateUnknown{K, O})` where O is current object
// of K.  If `f.knownObjects == nil` then the pre-existing keys are
// those in `f.items` and the current object of K is the `.Newest()`
// of the Deltas associated with K.  Otherwise the pre-existing keys
// are those listed by `f.knownObjects` and the current object of K is
// what `f.knownObjects.GetByKey(K)` returns.
func (f *DeltaFIFO) Replace(ctx context.Context, list []interface{}, _ string) (err error) {
	items := make(map[string]interface{}, len(list))
	for _, item := range list {
		key, err := f.KeyOf(item)
		if err != nil {
			return KeyError{item, err}
		}
		items[key] = item
	}

	newEnqueues := []string{}
	defer func() {
		for _, id := range newEnqueues {
			select {
			case f.queue <- id:
			case <-ctx.Done():
				err = ctx.Err()
				return
			}
		}
	}()

	f.lock.Lock()
	defer f.lock.Unlock()

	// Add replace delta to each of the provided items
	for id, item := range items {
		_, exists := f.items[id]

		if err := f.updateItemLocked(id, Replaced, item); err != nil {
			return err
		}

		if !exists {
			newEnqueues = append(newEnqueues, id)
		}
	}

	// Add delete delta to each of the existing items that
	// are not in the provided set.
	queuedDeletions := 0
	if f.knownObjects == nil {
		// Do deletion detection against our own list.
		for id, oldItem := range f.items {
			if _, exists := items[id]; exists {
				continue
			}

			// Delete pre-existing items not in the new list.
			// This could happen if watch deletion event was missed while
			// disconnected from apiserver.
			var deletedObj interface{}
			if n := oldItem.Newest(); n != nil {
				deletedObj = n.Object
			}
			queuedDeletions++
			if err := f.updateItemLocked(id, Deleted, DeletedFinalStateUnknown{id, deletedObj}); err != nil {
				return err
			}
			newEnqueues = append(newEnqueues, id)
		}
	} else {
		// Detect deletions not already in the queue.
		for _, id := range f.knownObjects.ListKeys() {
			if _, exists := items[id]; exists {
				continue
			}

			deletedObj, exists, err := f.knownObjects.GetByKey(id)
			if err != nil {
				deletedObj = nil
				klog.Errorf("Unexpected error %w during lookup of key %v, placing DeleteFinalStateUnknown marker without object", err, id)
			} else if !exists {
				deletedObj = nil
				klog.Infof("Key %v does not exist in known objects store, placing DeleteFinalStateUnknown marker without object", id)
			}
			queuedDeletions++
			if err := f.updateItemLocked(id, Deleted, DeletedFinalStateUnknown{id, deletedObj}); err != nil {
				return err
			}
			newEnqueues = append(newEnqueues, id)
		}
	}

	if !f.populated {
		f.populated = true
		f.initialPopulationCount = len(items) + queuedDeletions
		if f.initialPopulationCount == 0 && f.hasSyncedChannel != nil {
			close(f.hasSyncedChannel)
		}
	}

	return err
}

// re-listing and watching can deliver the same update multiple times in any
// order. This will combine the most recent two deltas if they are the same.
func dedupDeltas(deltas Deltas) Deltas {
	n := len(deltas)
	if n < 2 {
		return deltas
	}
	a := &deltas[n-1]
	b := &deltas[n-2]
	if out := isDup(a, b); out != nil {
		deltas[n-2] = *out
		return deltas[:n-1]
	}
	return deltas
}

// If a & b represent the same event, returns the delta that ought to be kept.
// Otherwise, returns nil.
// TODO: is there anything other than deletions that need deduping?
func isDup(a, b *Delta) *Delta {
	if out := isDeletionDup(a, b); out != nil {
		return out
	}
	// TODO: Detect other duplicate situations? Are there any?
	return nil
}

// keep the one with the most information if both are deletions.
func isDeletionDup(a, b *Delta) *Delta {
	if b.Type != Deleted || a.Type != Deleted {
		return nil
	}
	// Do more sophisticated checks, or is this sufficient?
	if _, ok := b.Object.(DeletedFinalStateUnknown); ok {
		return a
	}
	return b
}

// List returns a list of all the items; it returns the object
// from the most recent Delta.
// You should treat the items returned inside the deltas as immutable.
func (f *DeltaFIFO) List() []interface{} {
	f.lock.RLock()
	defer f.lock.RUnlock()
	list := make([]interface{}, 0, len(f.items))
	for _, item := range f.items {
		list = append(list, item.Newest().Object)
	}
	return list
}

// ListKeys returns a list of all the keys of the objects currently
// in the FIFO.
func (f *DeltaFIFO) ListKeys() []string {
	f.lock.RLock()
	defer f.lock.RUnlock()
	list := make([]string, 0, len(f.queue))
	for key := range f.items {
		list = append(list, key)
	}
	return list
}

// Get returns the complete list of deltas for the requested item,
// or sets exists=false.
// You should treat the items returned inside the deltas as immutable.
func (f *DeltaFIFO) Get(obj interface{}) (item interface{}, exists bool, err error) {
	key, err := f.KeyOf(obj)
	if err != nil {
		return nil, false, KeyError{obj, err}
	}
	return f.GetByKey(key)
}

// GetByKey returns the complete list of deltas for the requested item,
// setting exists=false if that list is empty.
// You should treat the items returned inside the deltas as immutable.
func (f *DeltaFIFO) GetByKey(key string) (item interface{}, exists bool, err error) {
	f.lock.RLock()
	defer f.lock.RUnlock()
	d, exists := f.items[key]
	if exists {
		// Copy item's slice so operations on this slice
		// won't interfere with the object we return.
		d = copyDeltas(d)
	}
	return d, exists, nil
}

// IsClosed checks if the queue is closed
func (f *DeltaFIFO) IsClosed() bool {
	f.lock.Lock()
	defer f.lock.Unlock()
	return f.closed
}

// Pop blocks until the queue has some items, and then returns one.  If
// multiple items are ready, they are returned in the order in which they were
// added/updated. The item is removed from the queue (and the store) before it
// is returned, so if you don't successfully process it, you need to add it back
// with AddIfNotPresent().
// process function is called under lock, so it is safe to update data structures
// in it that need to be in sync with the queue (e.g. knownKeys). The PopFunc
// may return an instance of ErrRequeue with a nested error to indicate the current
// item should be requeued (equivalent to calling AddIfNotPresent under the lock).
// process should avoid expensive I/O operation so that other queue operations, i.e.
// Add() and Get(), won't be blocked for too long.
//
// Pop returns a 'Deltas', which has a complete list of all the things
// that happened to the object (deltas) while it was sitting in the queue.
func (f *DeltaFIFO) PopAndProcess(ctx context.Context, process PopFunc) (interface{}, error) {
	var id string
	var item interface{}

	select {
	case id = <-f.queue:
	case <-ctx.Done():
		return nil, ctx.Err()
	}

	ok, err := func() (bool, error) {
		f.lock.Lock()
		defer f.lock.Unlock()

		// When Close() is called, the f.closed is set
		if f.closed {
			return false, ErrFIFOClosed
		}

		// Only log traces if the queue depth is greater than 10 and it takes more than
		// 100 milliseconds to process one item from the queue.
		// Queue depth never goes high because processing an item is locking the queue,
		// and new items can't be added until processing finish.
		// https://github.com/kubernetes/kubernetes/issues/103789
		depth := len(f.queue)
		if depth > 10 {
			trace := utiltrace.New("DeltaFIFO Pop Process",
				utiltrace.Field{Key: "ID", Value: id},
				utiltrace.Field{Key: "Depth", Value: depth},
				utiltrace.Field{Key: "Reason", Value: "slow event handlers blocking the queue"})
			defer trace.LogIfLong(100 * time.Millisecond)
		}

		if f.initialPopulationCount > 0 {
			f.initialPopulationCount--
			if f.initialPopulationCount == 0 && f.hasSyncedChannel != nil {
				close(f.hasSyncedChannel)
			}
		}

		var ok bool
		if item, ok = f.items[id]; !ok {
			// This should never happen
			klog.Errorf("Inconceivable! %q was in f.queue but not f.items; ignoring.", id)
			return false, nil
		}

		delete(f.items, id)

		return true, nil
	}()

	if err != nil {
		return nil, err
	} else if !ok {
		return f.PopAndProcess(ctx, process)
	} else {
		err := process(ctx, item)
		if e, ok := err.(ErrRequeue); ok {
			f.AddIfNotPresent(ctx, item)
			err = e.Err
		}

		return item, err
	}
}

// A KeyListerGetter is anything that knows how to list its keys and look up by key.
type KeyListerGetter interface {
	KeyLister
	KeyGetter
}

// A KeyLister is anything that knows how to list its keys.
type KeyLister interface {
	ListKeys() []string
}

// A KeyGetter is anything that knows how to get the value stored under a given key.
type KeyGetter interface {
	// GetByKey returns the value associated with the key, or sets exists=false.
	GetByKey(key string) (value interface{}, exists bool, err error)
}

// Oldest is a convenience function that returns the oldest delta, or
// nil if there are no deltas.
func (d Deltas) Oldest() *Delta {
	if len(d) > 0 {
		return &d[0]
	}
	return nil
}

// Newest is a convenience function that returns the newest delta, or
// nil if there are no deltas.
func (d Deltas) Newest() *Delta {
	if n := len(d); n > 0 {
		return &d[n-1]
	}
	return nil
}

// copyDeltas returns a shallow copy of d; that is, it copies the slice but not
// the objects in the slice. This allows Get/List to return an object that we
// know won't be clobbered by a subsequent modifications.
func copyDeltas(d Deltas) Deltas {
	d2 := make(Deltas, len(d))
	copy(d2, d)
	return d2
}

// DeletedFinalStateUnknown is placed into a DeltaFIFO in the case where an object
// was deleted but the watch deletion event was missed while disconnected from
// apiserver. In this case we don't know the final "resting" state of the object, so
// there's a chance the included `Obj` is stale.
type DeletedFinalStateUnknown struct {
	Key string
	Obj interface{}
}
