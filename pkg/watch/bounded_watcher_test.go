package watch_test

import (
	"context"
	goruntime "runtime"
	"testing"
	"time"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/pkg/watch"
)

// TestBoundedWatcherExit is expected to timeout if the event processor fails
// to exit when stopped.
func TestBoundedWatcherExit(t *testing.T) {
	event := watch.Event{}

	tests := []struct {
		name  string
		write func(e *watch.BoundedWatcher)
	}{
		{
			name: "exit on blocked read",
			write: func(e *watch.BoundedWatcher) {
				e.TryEvent(event)
			},
		},
		{
			name: "exit on blocked write",
			write: func(e *watch.BoundedWatcher) {
				e.TryEvent(event)
				e.TryEvent(event)
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			e := watch.NewBoundedWatcherWithSize(10)

			test.write(e)

			ctx, cancel := context.WithCancel(context.Background())
			exited := make(chan struct{})
			eventStream := make(chan watch.Event)
			go func() {
				e.Listen(ctx, eventStream)
				close(exited)
			}()

			<-eventStream
			cancel()
			goruntime.Gosched()
			<-exited
		})
	}
}

type apiInt int

func (apiInt) GetObjectKind() schema.ObjectKind { return nil }
func (apiInt) DeepCopyObject() runtime.Object   { return nil }

func TestBoundedWatcherOrdersEvents(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)

	e := watch.NewBoundedWatcherWithSize(1000)
	eventStream := make(chan watch.Event)
	go e.Listen(ctx, eventStream)

	numProcessed := 0
	go func() {
		for i := 0; i < 1000; i++ {
			e := <-eventStream
			if got, want := int(e.Object.(apiInt)), i; got != want {
				t.Errorf("unexpected event: got=%d, want=%d", got, want)
			}
			numProcessed++
		}
		cancel()
	}()

	for i := 0; i < 1000; i++ {
		e.TryEvent(watch.Event{Object: apiInt(i)})
	}

	<-ctx.Done()

	if numProcessed != 1000 {
		t.Errorf("unexpected number of events processed: %d", numProcessed)
	}

}
