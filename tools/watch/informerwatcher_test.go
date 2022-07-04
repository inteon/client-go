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

package watch

import (
	"context"
	"reflect"
	"sort"
	"testing"
	"time"

	"github.com/davecgh/go-spew/spew"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/diff"
	fakeclientset "k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/pkg/concurrent"
	"k8s.io/client-go/pkg/watch"
	testcore "k8s.io/client-go/testing"
	"k8s.io/client-go/tools/cache"
)

type apiInt int

func (apiInt) GetObjectKind() schema.ObjectKind { return nil }
func (apiInt) DeepCopyObject() runtime.Object   { return nil }

type byEventTypeAndName []watch.Event

func (a byEventTypeAndName) Len() int      { return len(a) }
func (a byEventTypeAndName) Swap(i, j int) { a[i], a[j] = a[j], a[i] }
func (a byEventTypeAndName) Less(i, j int) bool {
	if a[i].Type < a[j].Type {
		return true
	}

	if a[i].Type > a[j].Type {
		return false
	}

	return a[i].Object.(*corev1.Secret).Name < a[j].Object.(*corev1.Secret).Name
}

func TestNewInformerWatcher(t *testing.T) {
	// Make sure there are no 2 same types of events on a secret with the same name or that might be flaky.
	tt := []struct {
		name    string
		objects []runtime.Object
		events  []watch.Event
	}{
		{
			name: "basic test",
			objects: []runtime.Object{
				&corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{
						Name: "pod-1",
					},
					StringData: map[string]string{
						"foo-1": "initial",
					},
				},
				&corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{
						Name: "pod-2",
					},
					StringData: map[string]string{
						"foo-2": "initial",
					},
				},
				&corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{
						Name: "pod-3",
					},
					StringData: map[string]string{
						"foo-3": "initial",
					},
				},
			},
			events: []watch.Event{
				{
					Type: watch.Added,
					Object: &corev1.Secret{
						ObjectMeta: metav1.ObjectMeta{
							Name: "pod-4",
						},
						StringData: map[string]string{
							"foo-4": "initial",
						},
					},
				},
				{
					Type: watch.Modified,
					Object: &corev1.Secret{
						ObjectMeta: metav1.ObjectMeta{
							Name: "pod-2",
						},
						StringData: map[string]string{
							"foo-2": "new",
						},
					},
				},
				{
					Type: watch.Deleted,
					Object: &corev1.Secret{
						ObjectMeta: metav1.ObjectMeta{
							Name: "pod-3",
						},
					},
				},
			},
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()

			var expected []watch.Event
			for _, o := range tc.objects {
				expected = append(expected, watch.Event{
					Type:   watch.Added,
					Object: o.DeepCopyObject(),
				})
			}
			for _, e := range tc.events {
				expected = append(expected, *e.DeepCopy())
			}

			fake := fakeclientset.NewSimpleClientset(tc.objects...)
			fakeWatch := watch.NewBoundedWatcherWithSize(len(tc.events))
			fake.PrependWatchReactor("secrets", testcore.DefaultWatchReactor(fakeWatch, nil))

			for _, e := range tc.events {
				fakeWatch.TryAction(e.Type, e.Object)
			}

			lw := &cache.ListWatch{
				ListFunc: func(ctx context.Context, options metav1.ListOptions) (runtime.Object, error) {
					return fake.CoreV1().Secrets("").List(ctx, options)
				},
				WatchFunc: func(ctx context.Context, options metav1.ListOptions) (watch.Watcher, error) {
					return fake.CoreV1().Secrets("").Watch(ctx, options)
				},
			}
			_, _, w := NewIndexerInformerWatcher(lw, &corev1.Secret{})
			eventCh, ctx, complete := concurrent.Watch(ctx, w)
			defer complete()

			var result []watch.Event
		loop:
			for {
				var event watch.Event
				var ok bool
				select {
				case event, ok = <-eventCh:
					if !ok {
						t.Errorf("Failed to read event: channel is already closed!")
						return
					}

					result = append(result, *event.DeepCopy())
				case <-time.After(time.Second * 1):
					// All the events are buffered -> this means we are done
					// Also the one sec will make sure that we would detect RetryWatcher's incorrect behaviour after last event
					break loop
				}
			}

			// Informers don't guarantee event order so we need to sort these arrays to compare them
			sort.Sort(byEventTypeAndName(expected))
			sort.Sort(byEventTypeAndName(result))

			if !reflect.DeepEqual(expected, result) {
				t.Error(spew.Errorf("\nexpected: %#v,\ngot:      %#v,\ndiff: %s", expected, result, diff.ObjectReflectDiff(expected, result)))
				return
			}

			// Fill in some data to test watch closing while there are some events to be read
			for _, e := range tc.events {
				fakeWatch.Action(ctx, e.Type, e.Object)
			}
		})
	}

}

// TestInformerWatcherDeletedFinalStateUnknown tests the code path when `DeleteFunc`
// in `NewIndexerInformerWatcher` receives a `cache.DeletedFinalStateUnknown`
// object from the underlying `DeltaFIFO`. The triggering condition is described
// at https://github.com/kubernetes/kubernetes/blob/dc39ab2417bfddcec37be4011131c59921fdbe98/staging/src/k8s.io/client-go/tools/cache/delta_fifo.go#L736-L739.
//
// Code from @liggitt
func TestInformerWatcherDeletedFinalStateUnknown(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	listCalls := 0
	watchCalls := 0
	lw := &cache.ListWatch{
		ListFunc: func(_ context.Context, options metav1.ListOptions) (runtime.Object, error) {
			retval := &corev1.SecretList{}
			if listCalls == 0 {
				// Return a list with items in it
				retval.ResourceVersion = "1"
				retval.Items = []corev1.Secret{{ObjectMeta: metav1.ObjectMeta{Name: "secret1", Namespace: "ns1", ResourceVersion: "123"}}}
			} else {
				// Return empty lists after the first call
				retval.ResourceVersion = "2"
			}
			listCalls++
			return retval, nil
		},
		WatchFunc: func(ctx context.Context, options metav1.ListOptions) (watch.Watcher, error) {
			w := watch.NewBoundedWatcher()
			if options.ResourceVersion == "1" {
				go func() {
					// Close with a "Gone" error when trying to start a watch from the first list
					w.Error(ctx, &apierrors.NewGone("gone").ErrStatus)
				}()
			}
			watchCalls++
			return w, nil
		},
	}
	_, _, w := NewIndexerInformerWatcher(lw, &corev1.Secret{})
	eventCh, _, complete := concurrent.Watch(ctx, w)
	defer complete()

	// Expect secret add
	select {
	case event, ok := <-eventCh:
		if !ok {
			t.Fatal("unexpected close")
		}
		if event.Type != watch.Added {
			t.Fatalf("expected Added event, got %#v", event)
		}
		if event.Object.(*corev1.Secret).ResourceVersion != "123" {
			t.Fatalf("expected added Secret with rv=123, got %#v", event.Object)
		}
	case <-time.After(time.Second * 10):
		t.Fatal("timeout: add")
	}

	// Expect secret delete because the relist was missing the secret
	select {
	case event, ok := <-eventCh:
		if !ok {
			t.Fatal("unexpected close")
		}
		if event.Type != watch.Deleted {
			t.Fatalf("expected Deleted event, got %#v", event)
		}
		if event.Object.(*corev1.Secret).ResourceVersion != "123" {
			t.Fatalf("expected deleted Secret with rv=123, got %#v", event.Object)
		}
	case <-time.After(time.Second * 10):
		t.Fatal("timeout: delete")
	}

	complete()

	if listCalls < 2 {
		t.Fatalf("expected at least 2 list calls, got %d", listCalls)
	}
	if watchCalls < 1 {
		t.Fatalf("expected at least 1 watch call, got %d", watchCalls)
	}
}
