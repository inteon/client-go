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
	"fmt"
	"reflect"
	"strconv"
	"syscall"
	"testing"
	"time"

	v1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/pkg/concurrent"
	"k8s.io/client-go/pkg/watch"
	"k8s.io/utils/clock"
	testingclock "k8s.io/utils/clock/testing"
)

type testLW struct {
	ListFunc  func(ctx context.Context, options metav1.ListOptions) (runtime.Object, error)
	WatchFunc func(ctx context.Context, options metav1.ListOptions) (watch.Watcher, error)
}

func (t *testLW) List(ctx context.Context, options metav1.ListOptions) (runtime.Object, error) {
	return t.ListFunc(ctx, options)
}
func (t *testLW) Watch(ctx context.Context, options metav1.ListOptions) (watch.Watcher, error) {
	return t.WatchFunc(ctx, options)
}

func TestCloseWatchChannelOnError(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	r := NewReflector(&testLW{}, &v1.Pod{}, NewStore(MetaNamespaceKeyFunc))
	pod := &v1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "bar"}}
	fw := watch.NewBoundedWatcher()
	r.listerWatcher = &testLW{
		WatchFunc: func(_ context.Context, options metav1.ListOptions) (watch.Watcher, error) {
			return fw, nil
		},
		ListFunc: func(_ context.Context, options metav1.ListOptions) (runtime.Object, error) {
			return &v1.PodList{ListMeta: metav1.ListMeta{ResourceVersion: "1"}}, nil
		},
	}

	ctx, complete := concurrent.CompleteWithError(ctx, func(ctx context.Context) error {
		return r.ListAndWatch(ctx)
	})
	defer complete()

	fw.Error(ctx, pod)

	complete()

	if !fw.Stopped() {
		t.Errorf("Watch channel left open after stopping the watch")
	}
}

func TestRunUntil(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	store := NewStore(MetaNamespaceKeyFunc)
	r := NewReflector(&testLW{}, &v1.Pod{}, store)
	fw := watch.NewBoundedWatcher()
	r.listerWatcher = &testLW{
		WatchFunc: func(_ context.Context, options metav1.ListOptions) (watch.Watcher, error) {
			return fw, nil
		},
		ListFunc: func(_ context.Context, options metav1.ListOptions) (runtime.Object, error) {
			return &v1.PodList{ListMeta: metav1.ListMeta{ResourceVersion: "1"}}, nil
		},
	}
	go r.Run(ctx)
	// Synchronously add a dummy pod into the watch channel so we
	// know the RunUntil go routine is in the watch handler.
	fw.Add(ctx, &v1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "bar"}})
	cancel()

	if ok := fw.Stopped(); ok {
		t.Errorf("Watch channel left open after stopping the watch")
	}
}

/*
func TestReflectorResyncChan(t *testing.T) {
	s := NewStore(MetaNamespaceKeyFunc)
	g := NewReflector(&testLW{}, &v1.Pod{}, s)
	a, _ := g.resyncChan()
	b := time.After(wait.ForeverTestTimeout)
	select {
	case <-a:
		t.Logf("got timeout as expected")
	case <-b:
		t.Errorf("resyncChan() is at least 99 milliseconds late??")
	}
}
*/

/*
func BenchmarkReflectorResyncChanMany(b *testing.B) {
	s := NewStore(MetaNamespaceKeyFunc)
	g := NewReflector(&testLW{}, &v1.Pod{}, s)
	// The improvement to this (calling the timer's Stop() method) makes
	// this benchmark about 40% faster.
	for i := 0; i < b.N; i++ {
		g.resyncPeriod = time.Duration(rand.Float64() * float64(time.Millisecond) * 25)
		_, stop := g.resyncChan()
		stop()
	}
}
*/

func TestReflectorWatchHandlerError(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	s := NewStore(MetaNamespaceKeyFunc)
	g := NewReflector(&testLW{}, &v1.Pod{}, s)
	fw := watch.NewBoundedWatcher()
	go func() {
		fw.Close()
	}()
	var resumeRV string
	err := g.watchHandler(ctx, time.Now(), fw, &resumeRV)
	if err == nil {
		t.Errorf("unexpected non-error")
	}
}

func TestReflectorWatchHandler(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	s := NewStore(MetaNamespaceKeyFunc)
	g := NewReflector(&testLW{}, &v1.Pod{}, s)
	fw := watch.NewBoundedWatcher()
	s.Add(ctx, &v1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "foo"}})
	s.Add(ctx, &v1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "bar"}})
	go func() {
		fw.Add(ctx, &v1.Service{ObjectMeta: metav1.ObjectMeta{Name: "rejected"}})
		fw.Delete(ctx, &v1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "foo"}})
		fw.Modify(ctx, &v1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "bar", ResourceVersion: "55"}})
		fw.Add(ctx, &v1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "baz", ResourceVersion: "32"}})
		fw.Close()
	}()
	var resumeRV string
	err := g.watchHandler(ctx, time.Now(), fw, &resumeRV)
	if err != nil {
		t.Errorf("unexpected error %v", err)
	}

	mkPod := func(id string, rv string) *v1.Pod {
		return &v1.Pod{ObjectMeta: metav1.ObjectMeta{Name: id, ResourceVersion: rv}}
	}

	table := []struct {
		Pod    *v1.Pod
		exists bool
	}{
		{mkPod("foo", ""), false},
		{mkPod("rejected", ""), false},
		{mkPod("bar", "55"), true},
		{mkPod("baz", "32"), true},
	}
	for _, item := range table {
		obj, exists, _ := s.Get(item.Pod)
		if e, a := item.exists, exists; e != a {
			t.Errorf("%v: expected %v, got %v", item.Pod, e, a)
		}
		if !exists {
			continue
		}
		if e, a := item.Pod.ResourceVersion, obj.(*v1.Pod).ResourceVersion; e != a {
			t.Errorf("%v: expected %v, got %v", item.Pod, e, a)
		}
	}

	// RV should send the last version we see.
	if e, a := "32", resumeRV; e != a {
		t.Errorf("expected %v, got %v", e, a)
	}

	// last sync resource version should be the last version synced with store
	if e, a := "32", g.LastSyncResourceVersion(); e != a {
		t.Errorf("expected %v, got %v", e, a)
	}
}

func TestReflectorStopWatch(t *testing.T) {
	s := NewStore(MetaNamespaceKeyFunc)
	g := NewReflector(&testLW{}, &v1.Pod{}, s)
	fw := watch.NewBoundedWatcher()
	var resumeRV string
	ctx, cancel := context.WithCancel(context.TODO())
	cancel()
	err := g.watchHandler(ctx, time.Now(), fw, &resumeRV)
	if err != errorStopRequested {
		t.Errorf("expected stop error, got %q", err)
	}
}

func TestReflectorListAndWatch(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	createdFakes := make(chan *watch.BoundedWatcher)

	// The ListFunc says that it's at revision 1. Therefore, we expect our WatchFunc
	// to get called at the beginning of the watch with 1, and again with 3 when we
	// inject an error.
	expectedRVs := []string{"1", "3"}
	lw := &testLW{
		WatchFunc: func(_ context.Context, options metav1.ListOptions) (watch.Watcher, error) {
			rv := options.ResourceVersion
			fw := watch.NewBoundedWatcher()
			if e, a := expectedRVs[0], rv; e != a {
				t.Errorf("Expected rv %v, but got %v", e, a)
			}
			expectedRVs = expectedRVs[1:]
			// channel is not buffered because the for loop below needs to block. But
			// we don't want to block here, so report the new fake via a go routine.
			go func() { createdFakes <- fw }()
			return fw, nil
		},
		ListFunc: func(_ context.Context, options metav1.ListOptions) (runtime.Object, error) {
			return &v1.PodList{ListMeta: metav1.ListMeta{ResourceVersion: "1"}}, nil
		},
	}
	s := NewFIFO(KeyFunctionQueueOption(MetaNamespaceKeyFunc))
	r := NewReflector(lw, &v1.Pod{}, s)
	go r.ListAndWatch(ctx)

	ids := []string{"foo", "bar", "baz", "qux", "zoo"}
	var fw *watch.BoundedWatcher
	for i, id := range ids {
		if fw == nil {
			fw = <-createdFakes
		}
		sendingRV := strconv.FormatUint(uint64(i+2), 10)
		fw.Add(ctx, &v1.Pod{ObjectMeta: metav1.ObjectMeta{Name: id, ResourceVersion: sendingRV}})
		if sendingRV == "3" {
			// Inject a failure.
			fw.Close()
			fw = nil
		}
	}

	// Verify we received the right ids with the right resource versions.
	for i, id := range ids {
		var pod *v1.Pod
		if val, ok := popFromQueue(ctx, s); !ok {
			t.Fatalf("cannot pop from queue: context has expired")
		} else {
			pod = val.(*v1.Pod)
		}

		if e, a := id, pod.Name; e != a {
			t.Errorf("%v: Expected %v, got %v", i, e, a)
		}
		if e, a := strconv.FormatUint(uint64(i+2), 10), pod.ResourceVersion; e != a {
			t.Errorf("%v: Expected %v, got %v", i, e, a)
		}
	}

	if len(expectedRVs) != 0 {
		t.Error("called watchStarter an unexpected number of times")
	}
}

func TestReflectorListAndWatchWithErrors(t *testing.T) {
	mkPod := func(id string, rv string) *v1.Pod {
		return &v1.Pod{ObjectMeta: metav1.ObjectMeta{Name: id, ResourceVersion: rv}}
	}
	mkList := func(rv string, pods ...*v1.Pod) *v1.PodList {
		list := &v1.PodList{ListMeta: metav1.ListMeta{ResourceVersion: rv}}
		for _, pod := range pods {
			list.Items = append(list.Items, *pod)
		}
		return list
	}
	table := []struct {
		list     *v1.PodList
		listErr  error
		events   []watch.Event
		watchErr error
	}{
		{
			list: mkList("1"),
			events: []watch.Event{
				{Type: watch.Added, Object: mkPod("foo", "2")},
				{Type: watch.Added, Object: mkPod("bar", "3")},
			},
		}, {
			list: mkList("3", mkPod("foo", "2"), mkPod("bar", "3")),
			events: []watch.Event{
				{Type: watch.Deleted, Object: mkPod("foo", "4")},
				{Type: watch.Added, Object: mkPod("qux", "5")},
			},
		}, {
			listErr: fmt.Errorf("a list error"),
		}, {
			list:     mkList("5", mkPod("bar", "3"), mkPod("qux", "5")),
			watchErr: fmt.Errorf("a watch error"),
		}, {
			list: mkList("5", mkPod("bar", "3"), mkPod("qux", "5")),
			events: []watch.Event{
				{Type: watch.Added, Object: mkPod("baz", "6")},
			},
		}, {
			list: mkList("6", mkPod("bar", "3"), mkPod("qux", "5"), mkPod("baz", "6")),
		},
	}

	s := NewFIFO(KeyFunctionQueueOption(MetaNamespaceKeyFunc))
	for line, item := range table {
		if item.list != nil {
			// Test that the list is what currently exists in the store.
			current := s.List()
			checkMap := map[string]string{}
			for _, item := range current {
				pod := item.(*v1.Pod)
				checkMap[pod.Name] = pod.ResourceVersion
			}
			for _, pod := range item.list.Items {
				if e, a := pod.ResourceVersion, checkMap[pod.Name]; e != a {
					t.Errorf("%v: expected %v, got %v for pod %v", line, e, a, pod.Name)
				}
			}
			if e, a := len(item.list.Items), len(checkMap); e != a {
				t.Errorf("%v: expected %v, got %v", line, e, a)
			}
		}
		watchRet, watchErr := item.events, item.watchErr
		lw := &testLW{
			WatchFunc: func(ctx context.Context, options metav1.ListOptions) (watch.Watcher, error) {
				if watchErr != nil {
					return nil, watchErr
				}
				watchErr = fmt.Errorf("second watch")
				fw := watch.NewBoundedWatcher()
				go func() {
					for _, e := range watchRet {
						fw.Action(ctx, e.Type, e.Object)
					}
					fw.Close()
				}()
				return fw, nil
			},
			ListFunc: func(ctx context.Context, options metav1.ListOptions) (runtime.Object, error) {
				return item.list, item.listErr
			},
		}
		r := NewReflector(lw, &v1.Pod{}, s)
		r.ListAndWatch(context.TODO())
	}
}

func TestReflectorListAndWatchInitConnBackoff(t *testing.T) {
	maxBackoff := 50 * time.Millisecond
	table := []struct {
		numConnFails  int
		expLowerBound time.Duration
		expUpperBound time.Duration
	}{
		{5, 32 * time.Millisecond, 64 * time.Millisecond}, // case where maxBackoff is not hit, time should grow exponentially
		{40, 35 * 2 * maxBackoff, 40 * 2 * maxBackoff},    // case where maxBoff is hit, backoff time should flatten

	}
	for _, test := range table {
		t.Run(fmt.Sprintf("%d connection failures takes at least %d ms", test.numConnFails, 1<<test.numConnFails),
			func(t *testing.T) {
				ctx, cancel := context.WithCancel(context.TODO())
				connFails := test.numConnFails
				fakeClock := testingclock.NewFakeClock(time.Unix(0, 0))
				bm := wait.NewExponentialBackoffManager(time.Millisecond, maxBackoff, 100*time.Millisecond, 2.0, 1.0, fakeClock)
				done := make(chan struct{})
				defer close(done)
				go func() {
					i := 0
					for {
						select {
						case <-done:
							return
						default:
						}
						if fakeClock.HasWaiters() {
							step := (1 << (i + 1)) * time.Millisecond
							if step > maxBackoff*2 {
								step = maxBackoff * 2
							}
							fakeClock.Step(step)
							i++
						}
						time.Sleep(100 * time.Microsecond)
					}
				}()
				lw := &testLW{
					WatchFunc: func(_ context.Context, options metav1.ListOptions) (watch.Watcher, error) {
						if connFails > 0 {
							connFails--
							return nil, syscall.ECONNREFUSED
						}
						cancel()
						return watch.NewBoundedWatcher(), nil
					},
					ListFunc: func(_ context.Context, options metav1.ListOptions) (runtime.Object, error) {
						return &v1.PodList{ListMeta: metav1.ListMeta{ResourceVersion: "1"}}, nil
					},
				}
				r := &Reflector{
					name:                   "test-reflector",
					listerWatcher:          lw,
					store:                  NewFIFO(KeyFunctionQueueOption(MetaNamespaceKeyFunc)),
					initConnBackoffManager: bm,
					clock:                  fakeClock,
					watchErrorHandler:      WatchErrorHandler(DefaultWatchErrorHandler),
				}
				start := fakeClock.Now()
				err := r.ListAndWatch(ctx)
				elapsed := fakeClock.Since(start)
				if err != nil {
					t.Errorf("unexpected error %v", err)
				}
				if elapsed < (test.expLowerBound) {
					t.Errorf("expected lower bound of ListAndWatch: %v, got %v", test.expLowerBound, elapsed)
				}
				if elapsed > (test.expUpperBound) {
					t.Errorf("expected upper bound of ListAndWatch: %v, got %v", test.expUpperBound, elapsed)
				}
			})
	}
}

type fakeBackoff struct {
	clock clock.Clock
	calls int
}

func (f *fakeBackoff) Backoff() clock.Timer {
	f.calls++
	return f.clock.NewTimer(time.Duration(0))
}

func TestBackoffOnTooManyRequests(t *testing.T) {
	err := apierrors.NewTooManyRequests("too many requests", 1)
	clock := &clock.RealClock{}
	bm := &fakeBackoff{clock: clock}

	lw := &testLW{
		ListFunc: func(_ context.Context, options metav1.ListOptions) (runtime.Object, error) {
			return &v1.PodList{ListMeta: metav1.ListMeta{ResourceVersion: "1"}}, nil
		},
		WatchFunc: func(ctx context.Context, options metav1.ListOptions) (watch.Watcher, error) {
			switch bm.calls {
			case 0:
				return nil, err
			case 1:
				w := watch.NewBoundedWatcherWithSize(1)
				status := err.Status()
				w.Error(ctx, &status)
				return w, nil
			default:
				w := watch.NewBoundedWatcher()
				w.Close()
				return w, nil
			}
		},
	}

	r := &Reflector{
		name:                   "test-reflector",
		listerWatcher:          lw,
		store:                  NewFIFO(KeyFunctionQueueOption(MetaNamespaceKeyFunc)),
		initConnBackoffManager: bm,
		clock:                  clock,
		watchErrorHandler:      WatchErrorHandler(DefaultWatchErrorHandler),
	}

	ctx, cancel := context.WithCancel(context.TODO())
	r.ListAndWatch(ctx)
	cancel()
	if bm.calls != 2 {
		t.Errorf("unexpected watch backoff calls: %d", bm.calls)
	}
}

/*
func TestReflectorResync(t *testing.T) {
	iteration := 0
	ctx, cancel := context.WithCancel(context.TODO())
	defer cancel()
	rerr := errors.New("expected resync reached")
	s := &FakeCustomStore{
		ResyncFunc: func() error {
			iteration++
			if iteration == 2 {
				return rerr
			}
			return nil
		},
	}

	lw := &testLW{
		WatchFunc: func(_ context.Context, options metav1.ListOptions) (watch.Watcher, error) {
			fw := watch.NewBoundedWatcher()
			return fw, nil
		},
		ListFunc: func(_ context.Context, options metav1.ListOptions) (runtime.Object, error) {
			return &v1.PodList{ListMeta: metav1.ListMeta{ResourceVersion: "0"}}, nil
		},
	}
	r := NewReflector(lw, &v1.Pod{}, s)
	if err := r.ListAndWatch(ctx); err != nil {
		// error from Resync is not propaged up to here.
		t.Errorf("expected error %v", err)
	}
	if iteration != 2 {
		t.Errorf("exactly 2 iterations were expected, got: %v", iteration)
	}
}
*/

func TestReflectorWatchListPageSize(t *testing.T) {
	ctx, cancel := context.WithCancel(context.TODO())
	s := NewStore(MetaNamespaceKeyFunc)

	lw := &testLW{
		WatchFunc: func(_ context.Context, options metav1.ListOptions) (watch.Watcher, error) {
			// Stop once the reflector begins watching since we're only interested in the list.
			cancel()
			fw := watch.NewBoundedWatcher()
			return fw, nil
		},
		ListFunc: func(_ context.Context, options metav1.ListOptions) (runtime.Object, error) {
			if options.Limit != 4 {
				t.Fatalf("Expected list Limit of 4 but got %d", options.Limit)
			}
			pods := make([]v1.Pod, 10)
			for i := 0; i < 10; i++ {
				pods[i] = v1.Pod{ObjectMeta: metav1.ObjectMeta{Name: fmt.Sprintf("pod-%d", i), ResourceVersion: fmt.Sprintf("%d", i)}}
			}
			switch options.Continue {
			case "":
				return &v1.PodList{ListMeta: metav1.ListMeta{ResourceVersion: "10", Continue: "C1"}, Items: pods[0:4]}, nil
			case "C1":
				return &v1.PodList{ListMeta: metav1.ListMeta{ResourceVersion: "10", Continue: "C2"}, Items: pods[4:8]}, nil
			case "C2":
				return &v1.PodList{ListMeta: metav1.ListMeta{ResourceVersion: "10"}, Items: pods[8:10]}, nil
			default:
				t.Fatalf("Unrecognized continue: %s", options.Continue)
			}
			return nil, nil
		},
	}
	r := NewReflector(lw, &v1.Pod{}, s)
	// Set resource version to test pagination also for not consistent reads.
	r.setLastSyncResourceVersion("10")
	// Set the reflector to paginate the list request in 4 item chunks.
	r.WatchListPageSize = 4
	r.ListAndWatch(ctx)

	results := s.List()
	if len(results) != 10 {
		t.Errorf("Expected 10 results, got %d", len(results))
	}
}

func TestReflectorNotPaginatingNotConsistentReads(t *testing.T) {
	ctx, cancel := context.WithCancel(context.TODO())
	s := NewStore(MetaNamespaceKeyFunc)

	lw := &testLW{
		WatchFunc: func(_ context.Context, options metav1.ListOptions) (watch.Watcher, error) {
			// Stop once the reflector begins watching since we're only interested in the list.
			cancel()
			fw := watch.NewBoundedWatcher()
			return fw, nil
		},
		ListFunc: func(_ context.Context, options metav1.ListOptions) (runtime.Object, error) {
			if options.ResourceVersion != "10" {
				t.Fatalf("Expected ResourceVersion: \"10\", got: %s", options.ResourceVersion)
			}
			if options.Limit != 0 {
				t.Fatalf("Expected list Limit of 0 but got %d", options.Limit)
			}
			pods := make([]v1.Pod, 10)
			for i := 0; i < 10; i++ {
				pods[i] = v1.Pod{ObjectMeta: metav1.ObjectMeta{Name: fmt.Sprintf("pod-%d", i), ResourceVersion: fmt.Sprintf("%d", i)}}
			}
			return &v1.PodList{ListMeta: metav1.ListMeta{ResourceVersion: "10"}, Items: pods}, nil
		},
	}
	r := NewReflector(lw, &v1.Pod{}, s)
	r.setLastSyncResourceVersion("10")
	r.ListAndWatch(ctx)

	results := s.List()
	if len(results) != 10 {
		t.Errorf("Expected 10 results, got %d", len(results))
	}
}

func TestReflectorPaginatingNonConsistentReadsIfWatchCacheDisabled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.TODO())
	s := NewStore(MetaNamespaceKeyFunc)

	lw := &testLW{
		WatchFunc: func(_ context.Context, options metav1.ListOptions) (watch.Watcher, error) {
			// Stop once the reflector begins watching since we're only interested in the list.
			cancel()
			fw := watch.NewBoundedWatcher()
			return fw, nil
		},
		ListFunc: func(_ context.Context, options metav1.ListOptions) (runtime.Object, error) {
			// Check that default pager limit is set.
			if options.Limit != 500 {
				t.Fatalf("Expected list Limit of 500 but got %d", options.Limit)
			}
			pods := make([]v1.Pod, 10)
			for i := 0; i < 10; i++ {
				pods[i] = v1.Pod{ObjectMeta: metav1.ObjectMeta{Name: fmt.Sprintf("pod-%d", i), ResourceVersion: fmt.Sprintf("%d", i)}}
			}
			switch options.Continue {
			case "":
				return &v1.PodList{ListMeta: metav1.ListMeta{ResourceVersion: "10", Continue: "C1"}, Items: pods[0:4]}, nil
			case "C1":
				return &v1.PodList{ListMeta: metav1.ListMeta{ResourceVersion: "10", Continue: "C2"}, Items: pods[4:8]}, nil
			case "C2":
				return &v1.PodList{ListMeta: metav1.ListMeta{ResourceVersion: "10"}, Items: pods[8:10]}, nil
			default:
				t.Fatalf("Unrecognized continue: %s", options.Continue)
			}
			return nil, nil
		},
	}
	r := NewReflector(lw, &v1.Pod{}, s)

	// Initial list should initialize paginatedResult in the reflector.
	r.ListAndWatch(ctx)
	if results := s.List(); len(results) != 10 {
		t.Errorf("Expected 10 results, got %d", len(results))
	}

	// Since initial list for ResourceVersion="0" was paginated, the subsequent
	// ones should also be paginated.
	r.ListAndWatch(ctx)
	if results := s.List(); len(results) != 10 {
		t.Errorf("Expected 10 results, got %d", len(results))
	}
}

// TestReflectorResyncWithResourceVersion ensures that a reflector keeps track of the ResourceVersion and sends
// it in relist requests to prevent the reflector from traveling back in time if the relist is to a api-server or
// etcd that is partitioned and serving older data than the reflector has already processed.
func TestReflectorResyncWithResourceVersion(t *testing.T) {
	s := NewStore(MetaNamespaceKeyFunc)
	listCallRVs := []string{}

	var cancel func()
	var ctx context.Context
	lw := &testLW{
		WatchFunc: func(_ context.Context, options metav1.ListOptions) (watch.Watcher, error) {
			// Stop once the reflector begins watching since we're only interested in the list.
			cancel()
			fw := watch.NewBoundedWatcher()
			return fw, nil
		},
		ListFunc: func(_ context.Context, options metav1.ListOptions) (runtime.Object, error) {
			listCallRVs = append(listCallRVs, options.ResourceVersion)
			pods := make([]v1.Pod, 8)
			for i := 0; i < 8; i++ {
				pods[i] = v1.Pod{ObjectMeta: metav1.ObjectMeta{Name: fmt.Sprintf("pod-%d", i), ResourceVersion: fmt.Sprintf("%d", i)}}
			}
			switch options.ResourceVersion {
			case "0":
				return &v1.PodList{ListMeta: metav1.ListMeta{ResourceVersion: "10"}, Items: pods[0:4]}, nil
			case "10":
				return &v1.PodList{ListMeta: metav1.ListMeta{ResourceVersion: "11"}, Items: pods[0:8]}, nil
			default:
				t.Fatalf("Unrecognized ResourceVersion: %s", options.ResourceVersion)
			}
			return nil, nil
		},
	}
	r := NewReflector(lw, &v1.Pod{}, s)

	// Initial list should use RV=0
	ctx, cancel = context.WithCancel(context.TODO())
	r.ListAndWatch(ctx)

	results := s.List()
	if len(results) != 4 {
		t.Errorf("Expected 4 results, got %d", len(results))
	}

	// relist should use lastSyncResourceVersions (RV=10)
	ctx, cancel = context.WithCancel(context.TODO())
	defer cancel()
	r.ListAndWatch(ctx)

	results = s.List()
	if len(results) != 8 {
		t.Errorf("Expected 8 results, got %d", len(results))
	}

	expectedRVs := []string{"0", "10"}
	if !reflect.DeepEqual(listCallRVs, expectedRVs) {
		t.Errorf("Expected series of list calls with resource versions of %v but got: %v", expectedRVs, listCallRVs)
	}
}

// TestReflectorExpiredExactResourceVersion tests that a reflector handles the behavior of kubernetes 1.16 an earlier
// where if the exact ResourceVersion requested is not available for a List request for a non-zero ResourceVersion,
// an "Expired" error is returned if the ResourceVersion has expired (etcd has compacted it).
// (In kubernetes 1.17, or when the watch cache is enabled, the List will instead return the list that is no older than
// the requested ResourceVersion).
func TestReflectorExpiredExactResourceVersion(t *testing.T) {
	s := NewStore(MetaNamespaceKeyFunc)
	listCallRVs := []string{}

	var cancel func()
	var ctx context.Context
	lw := &testLW{
		WatchFunc: func(_ context.Context, options metav1.ListOptions) (watch.Watcher, error) {
			// Stop once the reflector begins watching since we're only interested in the list.
			cancel()
			fw := watch.NewBoundedWatcher()
			return fw, nil
		},
		ListFunc: func(_ context.Context, options metav1.ListOptions) (runtime.Object, error) {
			listCallRVs = append(listCallRVs, options.ResourceVersion)
			pods := make([]v1.Pod, 8)
			for i := 0; i < 8; i++ {
				pods[i] = v1.Pod{ObjectMeta: metav1.ObjectMeta{Name: fmt.Sprintf("pod-%d", i), ResourceVersion: fmt.Sprintf("%d", i)}}
			}
			switch options.ResourceVersion {
			case "0":
				return &v1.PodList{ListMeta: metav1.ListMeta{ResourceVersion: "10"}, Items: pods[0:4]}, nil
			case "10":
				// When watch cache is disabled, if the exact ResourceVersion requested is not available, a "Expired" error is returned.
				return nil, apierrors.NewResourceExpired("The resourceVersion for the provided watch is too old.")
			case "":
				return &v1.PodList{ListMeta: metav1.ListMeta{ResourceVersion: "11"}, Items: pods[0:8]}, nil
			default:
				t.Fatalf("Unrecognized ResourceVersion: %s", options.ResourceVersion)
			}
			return nil, nil
		},
	}
	r := NewReflector(lw, &v1.Pod{}, s)

	// Initial list should use RV=0
	ctx, cancel = context.WithCancel(context.TODO())
	r.ListAndWatch(ctx)

	results := s.List()
	if len(results) != 4 {
		t.Errorf("Expected 4 results, got %d", len(results))
	}

	// relist should use lastSyncResourceVersions (RV=10) and since RV=10 is expired, it should retry with RV="".
	ctx, cancel = context.WithCancel(context.TODO())
	defer cancel()
	r.ListAndWatch(ctx)

	results = s.List()
	if len(results) != 8 {
		t.Errorf("Expected 8 results, got %d", len(results))
	}

	expectedRVs := []string{"0", "10", ""}
	if !reflect.DeepEqual(listCallRVs, expectedRVs) {
		t.Errorf("Expected series of list calls with resource versions of %v but got: %v", expectedRVs, listCallRVs)
	}
}

func TestReflectorFullListIfExpired(t *testing.T) {
	s := NewStore(MetaNamespaceKeyFunc)
	listCallRVs := []string{}

	var cancel func()
	var ctx context.Context
	lw := &testLW{
		WatchFunc: func(_ context.Context, options metav1.ListOptions) (watch.Watcher, error) {
			// Stop once the reflector begins watching since we're only interested in the list.
			cancel()
			fw := watch.NewBoundedWatcher()
			return fw, nil
		},
		ListFunc: func(_ context.Context, options metav1.ListOptions) (runtime.Object, error) {
			listCallRVs = append(listCallRVs, options.ResourceVersion)
			pods := make([]v1.Pod, 8)
			for i := 0; i < 8; i++ {
				pods[i] = v1.Pod{ObjectMeta: metav1.ObjectMeta{Name: fmt.Sprintf("pod-%d", i), ResourceVersion: fmt.Sprintf("%d", i)}}
			}
			rvContinueLimit := func(rv, c string, l int64) metav1.ListOptions {
				return metav1.ListOptions{ResourceVersion: rv, Continue: c, Limit: l}
			}
			switch rvContinueLimit(options.ResourceVersion, options.Continue, options.Limit) {
			// initial limited list
			case rvContinueLimit("0", "", 4):
				return &v1.PodList{ListMeta: metav1.ListMeta{ResourceVersion: "10"}, Items: pods[0:4]}, nil
			// first page of the rv=10 list
			case rvContinueLimit("10", "", 4):
				return &v1.PodList{ListMeta: metav1.ListMeta{Continue: "C1", ResourceVersion: "11"}, Items: pods[0:4]}, nil
			// second page of the above list
			case rvContinueLimit("", "C1", 4):
				return nil, apierrors.NewResourceExpired("The resourceVersion for the provided watch is too old.")
			// rv=10 unlimited list
			case rvContinueLimit("10", "", 0):
				return &v1.PodList{ListMeta: metav1.ListMeta{ResourceVersion: "11"}, Items: pods[0:8]}, nil
			default:
				err := fmt.Errorf("unexpected list options: %#v", options)
				t.Error(err)
				return nil, err
			}
		},
	}
	r := NewReflector(lw, &v1.Pod{}, s)
	r.WatchListPageSize = 4

	// Initial list should use RV=0
	ctx, cancel = context.WithCancel(context.TODO())
	if err := r.ListAndWatch(ctx); err != nil {
		t.Fatal(err)
	}

	results := s.List()
	if len(results) != 4 {
		t.Errorf("Expected 4 results, got %d", len(results))
	}

	// relist should use lastSyncResourceVersions (RV=10) and since second page of that expired, it should full list with RV=10
	ctx, cancel = context.WithCancel(context.TODO())
	defer cancel()
	if err := r.ListAndWatch(ctx); err != nil {
		t.Fatal(err)
	}

	results = s.List()
	if len(results) != 8 {
		t.Errorf("Expected 8 results, got %d", len(results))
	}

	expectedRVs := []string{"0", "10", "", "10"}
	if !reflect.DeepEqual(listCallRVs, expectedRVs) {
		t.Errorf("Expected series of list calls with resource versions of %#v but got: %#v", expectedRVs, listCallRVs)
	}
}

func TestReflectorFullListIfTooLarge(t *testing.T) {
	s := NewStore(MetaNamespaceKeyFunc)
	listCallRVs := []string{}

	var cancel func()
	var ctx context.Context
	lw := &testLW{
		WatchFunc: func(_ context.Context, options metav1.ListOptions) (watch.Watcher, error) {
			// Stop once the reflector begins watching since we're only interested in the list.
			cancel()
			fw := watch.NewBoundedWatcher()
			return fw, nil
		},
		ListFunc: func(_ context.Context, options metav1.ListOptions) (runtime.Object, error) {
			listCallRVs = append(listCallRVs, options.ResourceVersion)

			switch options.ResourceVersion {
			// initial list
			case "0":
				return &v1.PodList{ListMeta: metav1.ListMeta{ResourceVersion: "20"}}, nil
			// relist after the initial list
			case "20":
				err := apierrors.NewTimeoutError("too large resource version", 1)
				err.ErrStatus.Details.Causes = []metav1.StatusCause{{Type: metav1.CauseTypeResourceVersionTooLarge}}
				return nil, err
			// relist after the initial list (covers the error format used in api server 1.17.0-1.18.5)
			case "30":
				err := apierrors.NewTimeoutError("too large resource version", 1)
				err.ErrStatus.Details.Causes = []metav1.StatusCause{{Message: "Too large resource version"}}
				return nil, err
			// relist from etcd after "too large" error
			case "":
				return &v1.PodList{ListMeta: metav1.ListMeta{ResourceVersion: "30"}}, nil
			default:
				return nil, fmt.Errorf("unexpected List call: %s", options.ResourceVersion)
			}
		},
	}
	r := NewReflector(lw, &v1.Pod{}, s)

	// Initial list should use RV=0
	ctx, cancel = context.WithCancel(context.TODO())
	defer cancel()
	if err := r.ListAndWatch(ctx); err != nil {
		t.Fatal(err)
	}

	// Relist from the future version.
	// This may happen, as watchcache is initialized from "current global etcd resource version"
	// when kube-apiserver is starting and if no objects are changing after that each kube-apiserver
	// may be synced to a different version and they will never converge.
	// TODO: We should use etcd progress-notify feature to avoid this behavior but until this is
	// done we simply try to relist from now to avoid continuous errors on relists.
	for i := 1; i <= 2; i++ {
		// relist twice to cover the two variants of TooLargeResourceVersion api errors
		ctx, cancel = context.WithCancel(context.TODO())
		defer cancel()
		if err := r.ListAndWatch(ctx); err != nil {
			t.Fatal(err)
		}
	}

	expectedRVs := []string{"0", "20", "", "30", ""}
	if !reflect.DeepEqual(listCallRVs, expectedRVs) {
		t.Errorf("Expected series of list calls with resource version of %#v but got: %#v", expectedRVs, listCallRVs)
	}
}

func TestReflectorSetExpectedType(t *testing.T) {
	obj := &unstructured.Unstructured{}
	gvk := schema.GroupVersionKind{
		Group:   "mygroup",
		Version: "v1",
		Kind:    "MyKind",
	}
	obj.SetGroupVersionKind(gvk)
	testCases := map[string]struct {
		inputType        interface{}
		expectedTypeName string
		expectedType     reflect.Type
		expectedGVK      *schema.GroupVersionKind
	}{
		"Nil type": {
			expectedTypeName: defaultExpectedTypeName,
		},
		"Normal type": {
			inputType:        &v1.Pod{},
			expectedTypeName: "*v1.Pod",
			expectedType:     reflect.TypeOf(&v1.Pod{}),
		},
		"Unstructured type without GVK": {
			inputType:        &unstructured.Unstructured{},
			expectedTypeName: "*unstructured.Unstructured",
			expectedType:     reflect.TypeOf(&unstructured.Unstructured{}),
		},
		"Unstructured type with GVK": {
			inputType:        obj,
			expectedTypeName: gvk.String(),
			expectedType:     reflect.TypeOf(&unstructured.Unstructured{}),
			expectedGVK:      &gvk,
		},
	}
	for testName, tc := range testCases {
		t.Run(testName, func(t *testing.T) {
			r := &Reflector{}
			r.setExpectedType(tc.inputType)
			if tc.expectedType != r.expectedType {
				t.Fatalf("Expected expectedType %v, got %v", tc.expectedType, r.expectedType)
			}
			if tc.expectedTypeName != r.expectedTypeName {
				t.Fatalf("Expected expectedTypeName %v, got %v", tc.expectedTypeName, r.expectedTypeName)
			}
			gvkNotEqual := (tc.expectedGVK == nil) != (r.expectedGVK == nil)
			if tc.expectedGVK != nil && r.expectedGVK != nil {
				gvkNotEqual = *tc.expectedGVK != *r.expectedGVK
			}
			if gvkNotEqual {
				t.Fatalf("Expected expectedGVK %v, got %v", tc.expectedGVK, r.expectedGVK)
			}
		})
	}
}

type storeWithRV struct {
	Store

	// resourceVersions tracks values passed by UpdateResourceVersion
	resourceVersions []string
}

func (s *storeWithRV) UpdateResourceVersion(resourceVersion string) {
	s.resourceVersions = append(s.resourceVersions, resourceVersion)
}

func newStoreWithRV() *storeWithRV {
	return &storeWithRV{
		Store: NewStore(MetaNamespaceKeyFunc),
	}
}

func TestReflectorResourceVersionUpdate(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	s := newStoreWithRV()
	fw := watch.NewBoundedWatcher()

	lw := &testLW{
		WatchFunc: func(_ context.Context, options metav1.ListOptions) (watch.Watcher, error) {
			return fw, nil
		},
		ListFunc: func(_ context.Context, options metav1.ListOptions) (runtime.Object, error) {
			return &v1.PodList{ListMeta: metav1.ListMeta{ResourceVersion: "10"}}, nil
		},
	}
	r := NewReflector(lw, &v1.Pod{}, s)

	makePod := func(rv string) *v1.Pod {
		return &v1.Pod{ObjectMeta: metav1.ObjectMeta{ResourceVersion: rv}}
	}

	go func() {
		fw.Action(ctx, watch.Added, makePod("10"))
		fw.Action(ctx, watch.Modified, makePod("20"))
		fw.Action(ctx, watch.Bookmark, makePod("30"))
		fw.Action(ctx, watch.Deleted, makePod("40"))
		fw.Close()
	}()

	// Initial list should use RV=0
	if err := r.ListAndWatch(ctx); err != nil {
		t.Fatal(err)
	}

	expectedRVs := []string{"10", "20", "30", "40"}
	if !reflect.DeepEqual(s.resourceVersions, expectedRVs) {
		t.Errorf("Expected series of resource version updates of %#v but got: %#v", expectedRVs, s.resourceVersions)
	}
}
