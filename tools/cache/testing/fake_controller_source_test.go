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

package framework

import (
	"sync"
	"testing"
	"time"

	"golang.org/x/net/context"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/pkg/concurrent"
	"k8s.io/client-go/pkg/watch"
)

// ensure the watch delivers the requested and only the requested items.
func consume(t *testing.T, w watch.Watcher, rvs []string, done *sync.WaitGroup) {
	defer done.Done()
	channel, _, cancel := concurrent.Watch(context.Background(), w)
	defer cancel()
	for _, rv := range rvs {
		select {
		case got, ok := <-channel:
			if !ok {
				t.Errorf("%#v: unexpected channel close, wanted %v", rvs, rv)
				return
			}
			gotRV := got.Object.(*v1.Pod).ObjectMeta.ResourceVersion
			if e, a := rv, gotRV; e != a {
				t.Errorf("wanted %v, got %v", e, a)
			} else {
				t.Logf("Got %v as expected", gotRV)
			}
		case <-time.After(10 * time.Second):
			t.Fatalf("receive timed out")
		}
	}
	// We should not get anything else.
	select {
	case got, open := <-channel:
		if open {
			t.Errorf("%#v: unwanted object %#v", rvs, got)
		}
	default:
	}
}

func TestRCNumber(t *testing.T) {
	ctx, cancel := context.WithCancel(context.TODO())
	defer cancel()

	pod := func(name string) *v1.Pod {
		return &v1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Name: name,
			},
		}
	}

	wg := &sync.WaitGroup{}
	wg.Add(3)

	source := NewFakeControllerSource()
	source.Add(ctx, pod("foo"))
	source.Modify(ctx, pod("foo"))
	source.Modify(ctx, pod("foo"))

	w, err := source.Watch(ctx, metav1.ListOptions{ResourceVersion: "1"})
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	go consume(t, w, []string{"2", "3"}, wg)

	list, err := source.List(ctx, metav1.ListOptions{})
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if e, a := "3", list.(*v1.List).ResourceVersion; e != a {
		t.Errorf("wanted %v, got %v", e, a)
	}

	w2, err := source.Watch(ctx, metav1.ListOptions{ResourceVersion: "2"})
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	go consume(t, w2, []string{"3"}, wg)

	w3, err := source.Watch(ctx, metav1.ListOptions{ResourceVersion: "3"})
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	go consume(t, w3, []string{}, wg)

	wg.Wait()
}

// TestResetWatch validates that the FakeController correctly mocks a watch
// falling behind and ResourceVersions aging out.
func TestResetWatch(t *testing.T) {
	ctx, cancel := context.WithCancel(context.TODO())
	defer cancel()

	pod := func(name string) *v1.Pod {
		return &v1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Name: name,
			},
		}
	}

	wg := &sync.WaitGroup{}
	wg.Add(1)

	source := NewFakeControllerSource()
	source.Add(ctx, pod("foo"))    // RV = 1
	source.Modify(ctx, pod("foo")) // RV = 2
	source.Modify(ctx, pod("foo")) // RV = 3

	// Kill watch, delete change history
	source.ResetWatch()

	// This should fail, RV=1 was lost with ResetWatch
	_, err := source.Watch(ctx, metav1.ListOptions{ResourceVersion: "1"})
	if err == nil {
		t.Fatalf("Unexpected non-error")
	}

	// This should succeed, RV=3 is current
	w, err := source.Watch(ctx, metav1.ListOptions{ResourceVersion: "3"})
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	// Modify again, ensure the watch is still working
	source.Modify(ctx, pod("foo"))
	go consume(t, w, []string{"4"}, wg)

	wg.Wait()
}
