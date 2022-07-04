package watch

import (
	"context"

	"k8s.io/apimachinery/pkg/watch"
)

type EmptyWatcher struct{}

func NewEmptyWatcher() Watcher {
	return EmptyWatcher(struct{}{})
}

func (w EmptyWatcher) Listen(ctx context.Context, outStream chan<- watch.Event) error {
	defer close(outStream)
	return nil
}
