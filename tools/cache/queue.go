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
)

// PopFunc is passed to Pop() method of Queue interface.
// It is supposed to process the accumulator popped from the queue.
type PopFunc func(context.Context, interface{}) error

// ErrRequeue may be returned by a PopFunc to safely requeue
// the current item. The value of Err will be returned from Pop.
type ErrRequeue struct {
	// Err is returned by the Pop function
	Err error
}

// ErrFIFOClosed used when FIFO is closed
var ErrFIFOClosed = errors.New("DeltaFIFO: manipulating with closed queue")

func (e ErrRequeue) Error() string {
	if e.Err == nil {
		return "the popped item should be requeued without returning an error"
	}
	return e.Err.Error()
}

// Queue extends Store with a collection of Store keys to "process".
// Every Add, Update, or Delete may put the object's key in that collection.
// A Queue has a way to derive the corresponding key given an accumulator.
// A Queue can be accessed concurrently from multiple goroutines.
// A Queue can be "closed", after which Pop operations return an error.
type Queue interface {
	Store

	// Pop blocks until there is at least one key to process or the
	// Queue is closed.  In the latter case Pop returns with an error.
	// In the former case Pop atomically picks one key to process,
	// removes that (key, accumulator) association from the Store, and
	// processes the accumulator.  Pop returns the accumulator that
	// was processed and the result of processing.  The PopFunc
	// may return an ErrRequeue{inner} and in this case Pop will (a)
	// return that (key, accumulator) association to the Queue as part
	// of the atomic processing and (b) return the inner error from
	// Pop.
	PopAndProcess(context.Context, PopFunc) (interface{}, error)

	// AddIfNotPresent puts the given accumulator into the Queue (in
	// association with the accumulator's key) if and only if that key
	// is not already associated with a non-empty accumulator.
	AddIfNotPresent(context.Context, interface{}) error

	// HasSynced returns true if the first batch of keys have all been
	// popped.  The first batch of keys are those of the first Replace
	// operation if that happened before any Add, AddIfNotPresent,
	// Update, or Delete; otherwise the first batch is empty.
	HasSynced() <-chan struct{}

	// Close the queue
	Close()

	// Check if queue is closed
	IsClosed() bool
}

const DefaultQueueSize = 30

// CacheOptions is the configuration parameters for DeltaFIFO. All are
// optional.
type QueueOptions struct {

	// KeyFunction is used to figure out what key an object should have. (It's
	// exposed in the returned DeltaFIFO's KeyOf() method, with additional
	// handling around deleted objects and queue state).
	// Optional, the default is MetaNamespaceKeyFunc.
	KeyFunction KeyFunc

	// KnownObjects is expected to return a list of keys that the consumer of
	// this queue "knows about". It is used to decide which items are missing
	// when Replace() is called; 'Deleted' deltas are produced for the missing items.
	// KnownObjects may be nil if you can tolerate missing deletions on Replace().
	KnownObjects KeyListerGetter

	QueueSize int
}

func NewQueueOptions(options ...QueueOption) *QueueOptions {
	queueOptions := &QueueOptions{
		KeyFunction:  MetaNamespaceKeyFunc,
		KnownObjects: nil,
		QueueSize:    DefaultQueueSize,
	}

	for _, option := range options {
		option(queueOptions)
	}

	return queueOptions
}

type QueueOption func(options *QueueOptions)

func KeyFunctionQueueOption(keyFunction KeyFunc) QueueOption {
	return func(options *QueueOptions) {
		options.KeyFunction = keyFunction
	}
}

func KnownObjectsQueueOption(keyListerGetter KeyListerGetter) QueueOption {
	return func(options *QueueOptions) {
		options.KnownObjects = keyListerGetter
	}
}

func QueueSizeQueueOption(queueSize int) QueueOption {
	return func(options *QueueOptions) {
		options.QueueSize = queueSize
	}
}
