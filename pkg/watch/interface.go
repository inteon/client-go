package watch

import (
	"context"
	"fmt"
	"sync"

	"k8s.io/apimachinery/pkg/watch"
)

type EventType = watch.EventType

const (
	Added    EventType = watch.Added
	Modified EventType = watch.Modified
	Deleted  EventType = watch.Deleted
	Bookmark EventType = watch.Bookmark
	Error    EventType = watch.Error
)

var (
	DefaultChanSize int32 = 100
)

type Event = watch.Event

// Interface can be implemented by anything that knows how to watch and report changes.
type Watcher interface {
	// Listen will block until ctx expires
	// NOTE: outStream WILL be closed on completion
	Listen(ctx context.Context, outStream chan<- watch.Event) error
}

type FuncWatcher func(ctx context.Context, outStream chan<- watch.Event) error

func (f FuncWatcher) Listen(ctx context.Context, outStream chan<- watch.Event) error {
	return f(ctx, outStream)
}

type uniqueWatcher struct {
	mu      sync.Mutex
	started bool
}

func (f *uniqueWatcher) start() error {
	f.mu.Lock()
	defer f.mu.Unlock()

	if f.started {
		return fmt.Errorf("a listener was already registered")
	}

	f.started = true

	return nil
}

func (f *uniqueWatcher) stopped() bool {
	f.mu.Lock()
	defer f.mu.Unlock()

	return !f.started
}

func (f *uniqueWatcher) stop() {
	f.mu.Lock()
	defer f.mu.Unlock()

	f.started = false
}
