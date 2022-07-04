/*
Copyright 2017 The Kubernetes Authors.

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
	"strings"
	"sync"
	"testing"
	"time"

	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/apimachinery/pkg/util/wait"
	fcache "k8s.io/client-go/tools/cache/testing"
)

type testListener struct {
	lock              sync.RWMutex
	expectedItemNames sets.String
	receivedItemNames []string
	name              string
}

func newTestListener(name string, expected ...string) *testListener {
	l := &testListener{
		expectedItemNames: sets.NewString(expected...),
		name:              name,
	}
	return l
}

func (l *testListener) OnAdd(ctx context.Context, obj interface{}) {
	l.handle(ctx, obj)
}

func (l *testListener) OnUpdate(ctx context.Context, old, new interface{}) {
	l.handle(ctx, new)
}

func (l *testListener) OnDelete(ctx context.Context, obj interface{}) {
}

func (l *testListener) handle(ctx context.Context, obj interface{}) {
	key, _ := MetaNamespaceKeyFunc(obj)
	fmt.Printf("%s: handle: %v\n", l.name, key)
	l.lock.Lock()
	defer l.lock.Unlock()

	objectMeta, _ := meta.Accessor(obj)
	l.receivedItemNames = append(l.receivedItemNames, objectMeta.GetName())
}

func (l *testListener) ok() bool {
	fmt.Println("polling")
	err := wait.PollImmediate(100*time.Millisecond, 2*time.Second, func() (bool, error) {
		if l.satisfiedExpectations() {
			return true, nil
		}
		return false, nil
	})
	if err != nil {
		return false
	}

	// wait just a bit to allow any unexpected stragglers to come in
	fmt.Println("sleeping")
	time.Sleep(1 * time.Second)
	fmt.Println("final check")
	return l.satisfiedExpectations()
}

func (l *testListener) satisfiedExpectations() bool {
	l.lock.RLock()
	defer l.lock.RUnlock()

	return sets.NewString(l.receivedItemNames...).Equal(l.expectedItemNames)
}

func TestListenerResyncPeriods(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// source simulates an apiserver object endpoint.
	source := fcache.NewFakeControllerSource()
	source.Add(ctx, &v1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "pod1"}})
	source.Add(ctx, &v1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "pod2"}})

	defer cancel()

	// create the shared informer and resync every 1s
	informer := NewSharedInformer(source, &v1.Pod{}).(*sharedIndexInformer)

	// listener 1, never resync
	listener1 := newTestListener("listener1", "pod1", "pod2")
	informer.AddEventHandler(ctx, listener1)

	// listener 2, never resync
	listener2 := newTestListener("listener2", "pod1", "pod2")
	informer.AddEventHandler(ctx, listener2)

	// listener 3, never resync
	listener3 := newTestListener("listener3", "pod1", "pod2")
	informer.AddEventHandler(ctx, listener3)
	listeners := []*testListener{listener1, listener2, listener3}

	go informer.Run(ctx)

	// ensure all listeners got the initial List
	for _, listener := range listeners {
		if !listener.ok() {
			t.Errorf("%s: expected %v, got %v", listener.name, listener.expectedItemNames, listener.receivedItemNames)
		}
	}
}

// verify that https://github.com/kubernetes/kubernetes/issues/59822 is fixed
func TestSharedInformerInitializationRace(t *testing.T) {
	source := fcache.NewFakeControllerSource()
	informer := NewSharedInformer(source, &v1.Pod{}).(*sharedIndexInformer)
	listener := newTestListener("raceListener")

	ctx, cancel := context.WithCancel(context.TODO())
	go informer.AddEventHandler(ctx, listener)
	go informer.Run(ctx)
	cancel()
}

// TestSharedInformerWatchDisruption simulates a watch that was closed
// with updates to the store during that time. We ensure that handlers with
// resync and no resync see the expected state.
func TestSharedInformerWatchDisruption(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// source simulates an apiserver object endpoint.
	source := fcache.NewFakeControllerSource()

	source.Add(ctx, &v1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "pod1", UID: "pod1", ResourceVersion: "1"}})
	source.Add(ctx, &v1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "pod2", UID: "pod2", ResourceVersion: "2"}})

	// create the shared informer and resync every 1s
	informer := NewSharedInformer(source, &v1.Pod{}).(*sharedIndexInformer)

	// listener, never resync
	listenerNoResync1 := newTestListener("listenerNoResync1", "pod1", "pod2")
	informer.AddEventHandler(ctx, listenerNoResync1)

	listenerNoResync2 := newTestListener("listenerNoResync2", "pod1", "pod2")
	informer.AddEventHandler(ctx, listenerNoResync2)
	listeners := []*testListener{listenerNoResync1, listenerNoResync2}

	go informer.Run(ctx)

	for _, listener := range listeners {
		if !listener.ok() {
			t.Errorf("%s: expected %v, got %v", listener.name, listener.expectedItemNames, listener.receivedItemNames)
		}
	}

	// Add pod3, bump pod2 but don't broadcast it, so that the change will be seen only on relist
	source.AddDropWatch(ctx, &v1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "pod3", UID: "pod3", ResourceVersion: "3"}})
	source.ModifyDropWatch(ctx, &v1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "pod2", UID: "pod2", ResourceVersion: "4"}})

	// Ensure that nobody saw any changes
	for _, listener := range listeners {
		if !listener.ok() {
			t.Errorf("%s: expected %v, got %v", listener.name, listener.expectedItemNames, listener.receivedItemNames)
		}
	}

	for _, listener := range listeners {
		listener.receivedItemNames = []string{}
	}

	listenerNoResync1.expectedItemNames = sets.NewString("pod2", "pod3")
	listenerNoResync2.expectedItemNames = sets.NewString("pod2", "pod3")

	// Simulate a connection loss (or even just a too-old-watch)
	source.ResetWatch()

	for _, listener := range listeners {
		if !listener.ok() {
			t.Errorf("%s: expected %v, got %v", listener.name, listener.expectedItemNames, listener.receivedItemNames)
		}
	}
}

func TestSharedInformerErrorHandling(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	source := fcache.NewFakeControllerSource()
	source.Add(ctx, &v1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "pod1"}})
	source.ListError = fmt.Errorf("Access Denied")

	informer := NewSharedInformer(source, &v1.Pod{}).(*sharedIndexInformer)

	errCh := make(chan error)
	_ = informer.SetWatchErrorHandler(func(_ *Reflector, err error) {
		errCh <- err
	})

	go informer.Run(ctx)

	select {
	case err := <-errCh:
		if !strings.Contains(err.Error(), "Access Denied") {
			t.Errorf("Expected 'Access Denied' error. Actual: %v", err)
		}
	case <-time.After(time.Second):
		t.Errorf("Timeout waiting for error handler call")
	}
}

func TestSharedInformerTransformer(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// source simulates an apiserver object endpoint.
	source := fcache.NewFakeControllerSource()

	source.Add(ctx, &v1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "pod1", UID: "pod1", ResourceVersion: "1"}})
	source.Add(ctx, &v1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "pod2", UID: "pod2", ResourceVersion: "2"}})

	informer := NewSharedInformer(source, &v1.Pod{}).(*sharedIndexInformer)
	informer.SetTransform(func(obj interface{}) (interface{}, error) {
		if pod, ok := obj.(*v1.Pod); ok {
			name := pod.GetName()

			if upper := strings.ToUpper(name); upper != name {
				copied := pod.DeepCopyObject().(*v1.Pod)
				copied.SetName(upper)
				return copied, nil
			}
		}
		return obj, nil
	})

	listenerTransformer := newTestListener("listenerTransformer", "POD1", "POD2")
	informer.AddEventHandler(ctx, listenerTransformer)

	go informer.Run(ctx)

	if !listenerTransformer.ok() {
		t.Errorf("%s: expected %v, got %v", listenerTransformer.name, listenerTransformer.expectedItemNames, listenerTransformer.receivedItemNames)
	}
}
