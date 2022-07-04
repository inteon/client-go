package watch

import (
	"context"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/watch"
)

// BoundedWatcher lets you create a simple Watcher impl; threadsafe.

type BoundedWatcher struct {
	uniqueWatcher

	result chan watch.Event
}

var _ Watcher = &BoundedWatcher{}

func NewBoundedWatcher() *BoundedWatcher {
	return &BoundedWatcher{
		result: make(chan watch.Event),
	}
}

func NewBoundedWatcherWithSize(size int) *BoundedWatcher {
	return &BoundedWatcher{
		result: make(chan watch.Event, size),
	}
}

func NewBoundedWatcherFromChannel(ch chan watch.Event) *BoundedWatcher {
	return &BoundedWatcher{
		result: ch,
	}
}

func (f *BoundedWatcher) Listen(ctx context.Context, outStream chan<- watch.Event) error {
	if err := f.start(); err != nil {
		return err
	}
	defer close(outStream)
	defer f.stop()

	for {
		select {
		case val, ok := <-f.result:
			if !ok {
				return nil
			}
			select {
			case outStream <- val:
			case <-ctx.Done():
				return nil
			}
		case <-ctx.Done():
			return nil
		}
	}
}

func (f *BoundedWatcher) Close() {
	close(f.result)
}

func (f *BoundedWatcher) Stopped() bool {
	return f.stopped()
}

// Add sends an add watch.Event.
func (f *BoundedWatcher) Add(ctx context.Context, obj runtime.Object) {
	f.Action(ctx, watch.Added, obj)
}

// Modify sends a modify watch.Event.
func (f *BoundedWatcher) Modify(ctx context.Context, obj runtime.Object) {
	f.Action(ctx, watch.Modified, obj)
}

// Delete sends a delete watch.Event.
func (f *BoundedWatcher) Delete(ctx context.Context, lastValue runtime.Object) {
	f.Action(ctx, watch.Deleted, lastValue)
}

// Error sends an Error watch.Event.
func (f *BoundedWatcher) Error(ctx context.Context, errValue runtime.Object) {
	f.Action(ctx, watch.Error, errValue)
}

// Action sends an watch.Event of the requested type, for table-based testing.
func (f *BoundedWatcher) Action(ctx context.Context, action watch.EventType, obj runtime.Object) {
	f.Event(ctx, watch.Event{Type: action, Object: obj})
}

// Event sends an watch.Event
func (f *BoundedWatcher) Event(ctx context.Context, event watch.Event) {
	select {
	case f.result <- event:
	case <-ctx.Done():
	}
}

// Add sends an add watch.Event.
func (f *BoundedWatcher) TryAdd(obj runtime.Object) bool {
	return f.TryAction(watch.Added, obj)
}

// Modify sends a modify watch.Event.
func (f *BoundedWatcher) TryModify(obj runtime.Object) bool {
	return f.TryAction(watch.Modified, obj)
}

// Delete sends a delete watch.Event.
func (f *BoundedWatcher) TryDelete(lastValue runtime.Object) bool {
	return f.TryAction(watch.Deleted, lastValue)
}

// Error sends an Error watch.Event.
func (f *BoundedWatcher) TryError(errValue runtime.Object) bool {
	return f.TryAction(watch.Error, errValue)
}

// Action sends an watch.Event of the requested type, for table-based testing.
func (f *BoundedWatcher) TryAction(action watch.EventType, obj runtime.Object) bool {
	return f.TryEvent(watch.Event{Type: action, Object: obj})
}

// Event sends an watch.Event
func (f *BoundedWatcher) TryEvent(event watch.Event) bool {
	select {
	case f.result <- event:
		return true
	default:
		return false
	}
}
