package concurrent

import (
	"context"

	"k8s.io/client-go/pkg/watch"
)

type DeferFunc func()

func Wait(ctx context.Context, f func(ctx context.Context)) DeferFunc {
	completedCh := make(chan struct{})
	go func() {
		defer close(completedCh)
		f(ctx)
	}()
	return func() { <-completedCh }
}

func Complete(ctx context.Context, f func(ctx context.Context)) (context.Context, DeferFunc) {
	ctx, cancel := context.WithCancel(ctx)
	completedCh := make(chan struct{})
	go func() {
		defer close(completedCh)
		f(ctx)
	}()
	return ctx, func() {
		cancel()
		<-completedCh
	}
}

type DeferErrFunc func() error

func WaitWithError(ctx context.Context, f func(ctx context.Context) error) DeferErrFunc {
	completedCh := make(chan error)
	var err error
	go func() {
		defer func() {
			completedCh <- err
			close(completedCh)
		}()
		err = f(ctx)
	}()
	return func() error { return <-completedCh }
}

func CompleteWithError(ctx context.Context, f func(ctx context.Context) error) (context.Context, DeferErrFunc) {
	ctx, cancel := context.WithCancel(ctx)
	completedCh := make(chan error)
	var err error
	go func() {
		defer func() {
			completedCh <- err
			close(completedCh)
		}()
		err = f(ctx)
	}()
	return ctx, func() error {
		cancel()
		return <-completedCh
	}
}

func Watch(ctx context.Context, watcher watch.Watcher) (<-chan watch.Event, context.Context, DeferErrFunc) {
	outStream := make(chan watch.Event)
	ctx, deferFn := CompleteWithError(ctx, func(ctx context.Context) error {
		return watcher.Listen(ctx, outStream)
	})
	return outStream, ctx, deferFn
}
