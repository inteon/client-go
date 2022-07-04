package watch

import (
	"context"
	"fmt"
	"io"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/net"
	"k8s.io/apimachinery/pkg/watch"
)

// Decoder allows StreamWatcher to watch any stream for which a Decoder can be written.
type Decoder interface {
	// Decode should return the type of event, the decoded object, or an error.
	// An error will cause StreamWatcher to call Close(). Decode should block until
	// it has data or an error occurs.
	Decode() (action EventType, object runtime.Object, err error)

	// Close should close the underlying io.Reader, signalling to the source of
	// the stream that it is no longer being watched. Close() must cause any
	// outstanding call to Decode() to return with an error of some sort.
	Close()
}

// Reporter hides the details of how an error is turned into a runtime.Object for
// reporting on a watch stream since this package may not import a higher level report.
type Reporter interface {
	// AsObject must convert err into a valid runtime.Object for the watch stream.
	AsObject(err error) runtime.Object
}

// StreamWatcher turns any stream for which you can write a Decoder interface
// into a watch.Interface.
type StreamWatcher struct {
	uniqueWatcher

	source   Decoder
	reporter Reporter
}

var _ Watcher = &StreamWatcher{}

// NewStreamWatcher creates a StreamWatcher from the given decoder.
func NewStreamWatcher(d Decoder, r Reporter) *StreamWatcher {
	sw := &StreamWatcher{
		source:   d,
		reporter: r,
	}
	return sw
}

func (sw *StreamWatcher) stop() {
	sw.mu.Lock()
	defer sw.mu.Unlock()

	sw.started = false
	sw.source.Close()
}

// ResultChan implements Interface.
func (sw *StreamWatcher) Listen(ctx context.Context, outStream chan<- watch.Event) error {
	defer close(outStream)
	if err := sw.start(); err != nil {
		return err
	}
	defer sw.stop()

	for {
		action, obj, err := sw.source.Decode()

		if err != nil {
			switch err {
			case io.EOF:
				// watch closed normally
				return nil
			case io.ErrUnexpectedEOF:
				return fmt.Errorf("unexpected EOF during watch stream event decoding: %w", err)
			default:
				if net.IsProbableEOF(err) || net.IsTimeout(err) {
					return fmt.Errorf("unable to decode an event from the watch stream: %w", err)
				} else {
					select {
					case outStream <- Event{
						Type:   Error,
						Object: sw.reporter.AsObject(fmt.Errorf("unable to decode an event from the watch stream: %w", err)),
					}:
						continue // continue since context has not expired yet
					case <-ctx.Done():
						return nil
					}
				}
			}
		}

		select {
		case outStream <- Event{
			Type:   action,
			Object: obj,
		}:
		case <-ctx.Done():
			return nil
		}
	}
}
