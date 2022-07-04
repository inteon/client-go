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
	"sync"
)

// FIFO is a Queue in which (a) each accumulator is simply the most
// recently provided object and (b) the collection of keys to process
// is a FIFO.  The accumulators all start out empty, and deleting an
// object from its accumulator empties the accumulator.  The Resync
// operation is a no-op.
//
// Thus: if multiple adds/updates of a single object happen while that
// object's key is in the queue before it has been processed then it
// will only be processed once, and when it is processed the most
// recent version will be processed. This can't be done with a channel
//
// FIFO solves this use case:
//  * You want to process every object (exactly) once.
//  * You want to process the most recent version of the object when you process it.
//  * You do not want to process deleted objects, they should be removed from the queue.
//  * You do not want to periodically reprocess objects.
// Compare with DeltaFIFO for other use cases.
type FIFO struct {
	lock sync.RWMutex

	// We depend on the property that every key in `items` is also in `queue`
	items map[string]interface{}
	queue chan string

	// populated is true if the first batch of items inserted by Replace() has been populated
	// or Delete/Add/Update was called first.
	populated bool
	// initialPopulationCount is the number of items inserted by the first call of Replace()
	initialPopulationCount int
	// channel that is closed when the hasSynced condition completed
	hasSyncedChannel chan struct{}

	// keyFunc is used to make the key used for queued item insertion and retrieval, and
	// should be deterministic.
	keyFunc KeyFunc

	// Indication the queue is closed.
	// Used to indicate a queue is closed so a control loop can exit when a queue is empty.
	// Currently, not used to gate any of CRUD operations.
	closed bool
}

var _ Queue = &FIFO{} // FIFO is a Queue

// NewFIFO returns a Store which can be used to queue up items to
// process.
func NewFIFO(options ...QueueOption) *FIFO {
	queueOptions := NewQueueOptions(options...)

	return &FIFO{
		items:            map[string]interface{}{},
		queue:            make(chan string, queueOptions.QueueSize),
		keyFunc:          queueOptions.KeyFunction,
		hasSyncedChannel: nil,
	}
}

// Close the queue.
func (f *FIFO) Close() {
	f.lock.Lock()
	defer f.lock.Unlock()

	if !f.closed {
		close(f.queue)
	}

	f.closed = true
}

// HasSynced returns true if an Add/Update/Delete/AddIfNotPresent are called first,
// or the first batch of items inserted by Replace() has been popped.
func (f *FIFO) HasSynced() <-chan struct{} {
	f.lock.Lock()
	defer f.lock.Unlock()

	if f.populated && f.initialPopulationCount == 0 {
		return ClosedChannel
	}

	f.hasSyncedChannel = make(chan struct{})
	return f.hasSyncedChannel
}

// Add inserts an item, and puts it in the queue. The item is only enqueued
// if it doesn't already exist in the set.
func (f *FIFO) Add(ctx context.Context, obj interface{}) error {
	id, err := f.keyFunc(obj)
	if err != nil {
		return KeyError{obj, err}
	}

	done := func() bool {
		f.lock.Lock()
		defer f.lock.Unlock()

		f.populated = true
		_, exists := f.items[id]
		f.items[id] = obj

		return exists // if item exists already, we are done
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

// AddIfNotPresent inserts an item, and puts it in the queue. If the item is already
// present in the set, it is neither enqueued nor added to the set.
//
// This is useful in a single producer/consumer scenario so that the consumer can
// safely retry items without contending with the producer and potentially enqueueing
// stale items.
func (f *FIFO) AddIfNotPresent(ctx context.Context, obj interface{}) error {
	id, err := f.keyFunc(obj)
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
			f.items[id] = obj
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

// Update is the same as Add in this implementation.
func (f *FIFO) Update(ctx context.Context, obj interface{}) error {
	return f.Add(ctx, obj)
}

// Delete removes an item. It doesn't add it to the queue, because
// this implementation assumes the consumer only cares about the objects,
// not the order in which they were created/added.
func (f *FIFO) Delete(ctx context.Context, obj interface{}) error {
	id, err := f.keyFunc(obj)
	if err != nil {
		return KeyError{obj, err}
	}
	f.lock.Lock()
	defer f.lock.Unlock()
	f.populated = true
	delete(f.items, id)
	return err
}

// Replace will delete the contents of 'f', using instead the given map.
// 'f' takes ownership of the map, you should not reference the map again
// after calling this function. f's queue is reset, too; upon return, it
// will contain the items in the map, in no particular order.
func (f *FIFO) Replace(ctx context.Context, list []interface{}, resourceVersion string) error {
	items := make(map[string]interface{}, len(list))
	for _, item := range list {
		key, err := f.keyFunc(item)
		if err != nil {
			return KeyError{item, err}
		}
		items[key] = item
	}

	func() {
		f.lock.Lock()
		defer f.lock.Unlock()

		if !f.populated {
			f.populated = true
			f.initialPopulationCount = len(items)
		}

		f.items = items

		// clear queue
	loop:
		for {
			select {
			case <-f.queue:
			default:
				break loop
			}
		}
	}()

	// add unique items to queue
	for id := range items {
		select {
		case f.queue <- id:
		case <-ctx.Done():
			return ctx.Err()
		}
	}

	return nil
}

// List returns a list of all the items.
func (f *FIFO) List() []interface{} {
	f.lock.RLock()
	defer f.lock.RUnlock()
	list := make([]interface{}, 0, len(f.items))
	for _, item := range f.items {
		list = append(list, item)
	}
	return list
}

// ListKeys returns a list of all the keys of the objects currently
// in the FIFO.
func (f *FIFO) ListKeys() []string {
	f.lock.RLock()
	defer f.lock.RUnlock()
	list := make([]string, 0, len(f.items))
	for key := range f.items {
		list = append(list, key)
	}
	return list
}

// Get returns the requested item, or sets exists=false.
func (f *FIFO) Get(obj interface{}) (item interface{}, exists bool, err error) {
	key, err := f.keyFunc(obj)
	if err != nil {
		return nil, false, KeyError{obj, err}
	}
	return f.GetByKey(key)
}

// GetByKey returns the requested item, or sets exists=false.
func (f *FIFO) GetByKey(key string) (item interface{}, exists bool, err error) {
	f.lock.RLock()
	defer f.lock.RUnlock()
	item, exists = f.items[key]
	return item, exists, nil
}

// IsClosed checks if the queue is closed
func (f *FIFO) IsClosed() bool {
	f.lock.Lock()
	defer f.lock.Unlock()
	return f.closed
}

// Pop waits until an item is ready and processes it. If multiple items are
// ready, they are returned in the order in which they were added/updated.
// The item is removed from the queue (and the store) before it is processed,
// so if you don't successfully process it, it should be added back with
// AddIfNotPresent(). process function is called under lock, so it is safe
// update data structures in it that need to be in sync with the queue.
func (f *FIFO) PopAndProcess(ctx context.Context, process PopFunc) (interface{}, error) {
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

		if f.initialPopulationCount > 0 {
			f.initialPopulationCount--
			if f.initialPopulationCount == 0 && f.hasSyncedChannel != nil {
				close(f.hasSyncedChannel)
			}
		}

		var ok bool
		if item, ok = f.items[id]; !ok {
			// Item may have been deleted subsequently.
			// Re-try calling this function
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
