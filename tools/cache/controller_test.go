/*
Copyright 2015 The Kubernetes Authors.

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
	"math/rand"
	"sync"
	"testing"
	"time"

	v1 "k8s.io/api/core/v1"
	apiequality "k8s.io/apimachinery/pkg/api/equality"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/pkg/watch"
	fcache "k8s.io/client-go/tools/cache/testing"

	fuzz "github.com/google/gofuzz"
)

func Example() {
	// source simulates an apiserver object endpoint.
	source := fcache.NewFakeControllerSource()

	// This will hold the downstream state, as we know it.
	downstream := NewStore(DeletionHandlingMetaNamespaceKeyFunc)

	// This will hold incoming changes. Note how we pass downstream in as a
	// KeyLister, that way resync operations will result in the correct set
	// of update/delete deltas.
	fifo := NewDeltaFIFO(
		KeyFunctionQueueOption(MetaNamespaceKeyFunc),
		KnownObjectsQueueOption(downstream),
	)

	// Let's do threadsafe output to get predictable test results.
	deletionCounter := make(chan string, 1000)

	cfg := &Config{
		Queue:         fifo,
		ListerWatcher: source,
		ObjectType:    &v1.Pod{},

		// Let's implement a simple controller that just deletes
		// everything that comes in.
		Process: func(ctx context.Context, obj interface{}) error {
			// Obj is from the Pop method of the Queue we make above.
			newest := obj.(Deltas).Newest()

			if newest.Type != Deleted {
				// Update our downstream store.
				err := downstream.Add(ctx, newest.Object)
				if err != nil {
					return err
				}

				// Delete this object.
				source.Delete(ctx, newest.Object.(runtime.Object))
			} else {
				// Update our downstream store.
				err := downstream.Delete(ctx, newest.Object)
				if err != nil {
					return err
				}

				// fifo's KeyOf is easiest, because it handles
				// DeletedFinalStateUnknown markers.
				key, err := fifo.KeyOf(newest.Object)
				if err != nil {
					return err
				}

				// Report this deletion.
				deletionCounter <- key
			}
			return nil
		},
	}

	// Create the controller and run it until we close stop.
	ctx, cancel := context.WithCancel(context.TODO())
	defer cancel()
	go New(cfg).Run(ctx)

	// Let's add a few objects to the source.
	testIDs := []string{"a-hello", "b-controller", "c-framework"}
	for _, name := range testIDs {
		// Note that these pods are not valid-- the fake source doesn't
		// call validation or anything.
		source.Add(ctx, &v1.Pod{ObjectMeta: metav1.ObjectMeta{Name: name}})
	}

	// Let's wait for the controller to process the things we just added.
	outputSet := sets.String{}
	for i := 0; i < len(testIDs); i++ {
		outputSet.Insert(<-deletionCounter)
	}

	for _, key := range outputSet.List() {
		fmt.Println(key)
	}
	// Output:
	// a-hello
	// b-controller
	// c-framework
}

func ExampleNewInformer() {
	// source simulates an apiserver object endpoint.
	source := fcache.NewFakeControllerSource()

	// Let's do threadsafe output to get predictable test results.
	deletionCounter := make(chan string, 1000)

	// Make a controller that immediately deletes anything added to it, and
	// logs anything deleted.
	_, controller := NewInformer(
		source,
		&v1.Pod{},
		ResourceEventHandlerFuncs{
			AddFunc: func(ctx context.Context, obj interface{}) {
				source.Delete(ctx, obj.(runtime.Object))
			},
			DeleteFunc: func(ctx context.Context, obj interface{}) {
				key, err := DeletionHandlingMetaNamespaceKeyFunc(obj)
				if err != nil {
					key = "oops something went wrong with the key"
				}

				// Report this deletion.
				deletionCounter <- key
			},
		},
	)

	// Run the controller and run it until we close stop.
	ctx, cancel := context.WithCancel(context.TODO())
	defer cancel()
	go controller.Run(ctx)

	// Let's add a few objects to the source.
	testIDs := []string{"a-hello", "b-controller", "c-framework"}
	for _, name := range testIDs {
		// Note that these pods are not valid-- the fake source doesn't
		// call validation or anything.
		source.Add(ctx, &v1.Pod{ObjectMeta: metav1.ObjectMeta{Name: name}})
	}

	// Let's wait for the controller to process the things we just added.
	outputSet := sets.String{}
	for i := 0; i < len(testIDs); i++ {
		outputSet.Insert(<-deletionCounter)
	}

	for _, key := range outputSet.List() {
		fmt.Println(key)
	}
	// Output:
	// a-hello
	// b-controller
	// c-framework
}

func TestHammerController(t *testing.T) {
	// This test executes a bunch of requests through the fake source and
	// controller framework to make sure there's no locking/threading
	// errors. If an error happens, it should hang forever or trigger the
	// race detector.

	// source simulates an apiserver object endpoint.
	source := fcache.NewFakeControllerSource()

	// Let's do threadsafe output to get predictable test results.
	outputSetLock := sync.Mutex{}
	// map of key to operations done on the key
	outputSet := map[string][]string{}

	recordFunc := func(eventType string, obj interface{}) {
		key, err := DeletionHandlingMetaNamespaceKeyFunc(obj)
		if err != nil {
			t.Errorf("something wrong with key: %v", err)
			key = "oops something went wrong with the key"
		}

		// Record some output when items are deleted.
		outputSetLock.Lock()
		defer outputSetLock.Unlock()
		outputSet[key] = append(outputSet[key], eventType)
	}

	// Make a controller which just logs all the changes it gets.
	_, controller := NewInformer(
		source,
		&v1.Pod{},
		ResourceEventHandlerFuncs{
			AddFunc:    func(ctx context.Context, obj interface{}) { recordFunc("add", obj) },
			UpdateFunc: func(ctx context.Context, oldObj, newObj interface{}) { recordFunc("update", newObj) },
			DeleteFunc: func(ctx context.Context, obj interface{}) { recordFunc("delete", obj) },
		},
	)

	select {
	case <-controller.HasSynced():
		t.Errorf("Expected HasSynced() to return false before we started the controller")
	default:
	}

	// Run the controller and run it until we close stop.
	ctx, cancel := context.WithCancel(context.TODO())
	go controller.Run(ctx)

	// Let's wait for the controller to do its initial sync
	<-controller.HasSynced()

	wg := sync.WaitGroup{}
	const threads = 3
	wg.Add(threads)
	for i := 0; i < threads; i++ {
		go func() {
			defer wg.Done()
			// Let's add a few objects to the source.
			currentNames := sets.String{}
			rs := rand.NewSource(rand.Int63())
			f := fuzz.New().NilChance(.5).NumElements(0, 2).RandSource(rs)
			for i := 0; i < 100; i++ {
				var name string
				var isNew bool
				if currentNames.Len() == 0 || rand.Intn(3) == 1 {
					f.Fuzz(&name)
					isNew = true
				} else {
					l := currentNames.List()
					name = l[rand.Intn(len(l))]
				}

				pod := &v1.Pod{}
				f.Fuzz(pod)
				pod.ObjectMeta.Name = name
				pod.ObjectMeta.Namespace = "default"
				// Add, update, or delete randomly.
				// Note that these pods are not valid-- the fake source doesn't
				// call validation or perform any other checking.
				if isNew {
					currentNames.Insert(name)
					source.Add(ctx, pod)
					continue
				}
				switch rand.Intn(2) {
				case 0:
					currentNames.Insert(name)
					source.Modify(ctx, pod)
				case 1:
					currentNames.Delete(name)
					source.Delete(ctx, pod)
				}
			}
		}()
	}
	wg.Wait()

	// Let's wait for the controller to finish processing the things we just added.
	// TODO: look in the queue to see how many items need to be processed.
	time.Sleep(100 * time.Millisecond)
	cancel()

	// TODO: Verify that no goroutines were leaked here and that everything shut
	// down cleanly.

	outputSetLock.Lock()
	t.Logf("got: %#v", outputSet)
}

func TestUpdate(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// This test is going to exercise the various paths that result in a
	// call to update.

	// source simulates an apiserver object endpoint.
	source := fcache.NewFakeControllerSource()

	const (
		FROM = "from"
		TO   = "to"
	)

	// These are the transitions we expect to see; because this is
	// asynchronous, there are a lot of valid possibilities.
	type pair struct{ from, to string }
	allowedTransitions := map[pair]bool{
		{FROM, TO}: true,

		// Because a resync can happen when we've already observed one
		// of the above but before the item is deleted.
		{TO, TO}: true,
		// Because a resync could happen before we observe an update.
		{FROM, FROM}: true,
	}

	pod := func(name, check string, final bool) *v1.Pod {
		p := &v1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Name:   name,
				Labels: map[string]string{"check": check},
			},
		}
		if final {
			p.Labels["final"] = "true"
		}
		return p
	}
	deletePod := func(p *v1.Pod) bool {
		return p.Labels["final"] == "true"
	}

	tests := []func(string){
		func(name string) {
			name = "a-" + name
			source.Add(ctx, pod(name, FROM, false))
			source.Modify(ctx, pod(name, TO, true))
		},
	}

	const threads = 3

	var testDoneWG sync.WaitGroup
	testDoneWG.Add(threads * len(tests))

	// Make a controller that deletes things once it observes an update.
	// It calls Done() on the wait group on deletions so we can tell when
	// everything we've added has been deleted.
	watchCh := make(chan struct{})
	_, controller := NewInformer(
		&testLW{
			WatchFunc: func(ctx context.Context, options metav1.ListOptions) (watch.Watcher, error) {
				watch, err := source.Watch(ctx, options)
				close(watchCh)
				return watch, err
			},
			ListFunc: func(ctx context.Context, options metav1.ListOptions) (runtime.Object, error) {
				return source.List(ctx, options)
			},
		},
		&v1.Pod{},
		ResourceEventHandlerFuncs{
			UpdateFunc: func(ctx context.Context, oldObj, newObj interface{}) {
				o, n := oldObj.(*v1.Pod), newObj.(*v1.Pod)
				from, to := o.Labels["check"], n.Labels["check"]
				if !allowedTransitions[pair{from, to}] {
					t.Errorf("observed transition %q -> %q for %v", from, to, n.Name)
				}
				if deletePod(n) {
					source.Delete(ctx, n)
				}
			},
			DeleteFunc: func(ctx context.Context, obj interface{}) {
				testDoneWG.Done()
			},
		},
	)

	// Run the controller and run it until we close stop.
	// Once Run() is called, calls to testDoneWG.Done() might start, so
	// all testDoneWG.Add() calls must happen before this point
	go controller.Run(ctx)
	<-watchCh

	// run every test a few times, in parallel
	var wg sync.WaitGroup
	wg.Add(threads * len(tests))
	for i := 0; i < threads; i++ {
		for j, f := range tests {
			go func(name string, f func(string)) {
				defer wg.Done()
				f(name)
			}(fmt.Sprintf("%v-%v", i, j), f)
		}
	}
	wg.Wait()

	// Let's wait for the controller to process the things we just added.
	testDoneWG.Wait()
}

func TestPanicPropagated(t *testing.T) {
	// source simulates an apiserver object endpoint.
	source := fcache.NewFakeControllerSource()

	// Make a controller that just panic if the AddFunc is called.
	_, controller := NewInformer(
		source,
		&v1.Pod{},
		ResourceEventHandlerFuncs{
			AddFunc: func(ctx context.Context, obj interface{}) {
				// Create a panic.
				panic("Just panic.")
			},
		},
	)

	// Run the controller and run it until we close stop.
	ctx, cancel := context.WithCancel(context.TODO())
	defer cancel()

	propagated := make(chan interface{})
	go func() {
		defer func() {
			if r := recover(); r != nil {
				propagated <- r
			}
		}()
		controller.Run(ctx)
	}()
	// Let's add a object to the source. It will trigger a panic.
	source.Add(ctx, &v1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "test"}})

	// Check if the panic propagated up.
	select {
	case p := <-propagated:
		if p == "Just panic." {
			t.Logf("Test Passed")
		} else {
			t.Errorf("unrecognized panic in controller run: %v", p)
		}
	case <-time.After(wait.ForeverTestTimeout):
		t.Errorf("timeout: the panic failed to propagate from the controller run method!")
	}
}

func TestTransformingInformer(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// source simulates an apiserver object endpoint.
	source := fcache.NewFakeControllerSource()

	makePod := func(name, generation string) *v1.Pod {
		return &v1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Name:      name,
				Namespace: "namespace",
				Labels:    map[string]string{"generation": generation},
			},
			Spec: v1.PodSpec{
				Hostname:  "hostname",
				Subdomain: "subdomain",
			},
		}
	}
	expectedPod := func(name, generation string) *v1.Pod {
		pod := makePod(name, generation)
		pod.Spec.Hostname = "new-hostname"
		pod.Spec.Subdomain = ""
		pod.Spec.NodeName = "nodename"
		return pod
	}

	source.Add(ctx, makePod("pod1", "1"))
	source.Modify(ctx, makePod("pod1", "2"))

	type event struct {
		eventType watch.EventType
		previous  interface{}
		current   interface{}
	}
	events := make(chan event, 10)
	recordEvent := func(eventType watch.EventType, previous, current interface{}) {
		events <- event{eventType: eventType, previous: previous, current: current}
	}
	verifyEvent := func(eventType watch.EventType, previous, current interface{}) {
		select {
		case event := <-events:
			if event.eventType != eventType {
				t.Errorf("expected type %v, got %v", eventType, event.eventType)
			}
			if !apiequality.Semantic.DeepEqual(event.previous, previous) {
				t.Errorf("expected previous object %#v, got %#v", previous, event.previous)
			}
			if !apiequality.Semantic.DeepEqual(event.current, current) {
				t.Errorf("expected object %#v, got %#v", current, event.current)
			}
		case <-time.After(wait.ForeverTestTimeout):
			t.Errorf("failed to get event")
		}
	}

	podTransformer := func(obj interface{}) (interface{}, error) {
		pod, ok := obj.(*v1.Pod)
		if !ok {
			return nil, fmt.Errorf("unexpected object type: %T", obj)
		}
		pod.Spec.Hostname = "new-hostname"
		pod.Spec.Subdomain = ""
		pod.Spec.NodeName = "nodename"

		// Clear out ResourceVersion to simplify comparisons.
		pod.ResourceVersion = ""

		return pod, nil
	}

	store, controller := NewTransformingInformer(
		source,
		&v1.Pod{},
		ResourceEventHandlerFuncs{
			AddFunc:    func(ctx context.Context, obj interface{}) { recordEvent(watch.Added, nil, obj) },
			UpdateFunc: func(ctx context.Context, oldObj, newObj interface{}) { recordEvent(watch.Modified, oldObj, newObj) },
			DeleteFunc: func(ctx context.Context, obj interface{}) { recordEvent(watch.Deleted, obj, nil) },
		},
		podTransformer,
	)

	verifyStore := func(expectedItems []interface{}) {
		items := store.List()
		if len(items) != len(expectedItems) {
			t.Errorf("unexpected items %v, expected %v", items, expectedItems)
		}
		for _, expectedItem := range expectedItems {
			found := false
			for _, item := range items {
				if apiequality.Semantic.DeepEqual(item, expectedItem) {
					found = true
				}
			}
			if !found {
				t.Errorf("expected item %v not found in %v", expectedItem, items)
			}
		}
	}

	go controller.Run(ctx)

	verifyEvent(watch.Added, nil, expectedPod("pod1", "2"))
	verifyStore([]interface{}{expectedPod("pod1", "2")})

	source.Add(ctx, makePod("pod2", "1"))
	verifyEvent(watch.Added, nil, expectedPod("pod2", "1"))
	verifyStore([]interface{}{expectedPod("pod1", "2"), expectedPod("pod2", "1")})

	source.Add(ctx, makePod("pod3", "1"))
	verifyEvent(watch.Added, nil, expectedPod("pod3", "1"))

	source.Modify(ctx, makePod("pod2", "2"))
	verifyEvent(watch.Modified, expectedPod("pod2", "1"), expectedPod("pod2", "2"))

	source.Delete(ctx, makePod("pod1", "2"))
	verifyEvent(watch.Deleted, expectedPod("pod1", "2"), nil)
	verifyStore([]interface{}{expectedPod("pod2", "2"), expectedPod("pod3", "1")})
}
