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
	"runtime"
	"testing"
	"time"
)

func testPopTestFifoObject(t testing.TB, ctx context.Context, f *FIFO) testFifoObject {
	if val, ok := popFromQueue(ctx, f); !ok {
		t.Fatalf("cannot pop from queue: context has expired")
		return testFifoObject{}
	} else {
		return val.(testFifoObject)
	}
}

// Pop is helper function for popping from Queue.
func popFromQueue(ctx context.Context, queue Queue) (interface{}, bool) {
	item, err := queue.PopAndProcess(ctx, func(ctx context.Context, obj interface{}) error {
		return nil
	})
	if err == context.Canceled {
		return nil, false
	} else if err != nil {
		panic(err)
	}
	return item, true
}

func testFifoObjectKeyFunc(obj interface{}) (string, error) {
	return obj.(testFifoObject).name, nil
}

type testFifoObject struct {
	name string
	val  interface{}
}

func mkFifoObj(name string, val interface{}) testFifoObject {
	return testFifoObject{name: name, val: val}
}

func TestFIFO_basic(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	f := NewFIFO(KeyFunctionQueueOption(testFifoObjectKeyFunc))
	const amount = 500
	go func() {
		for i := 0; i < amount; i++ {
			f.Add(ctx, mkFifoObj(string([]rune{'a', rune(i)}), i+1))
		}
	}()
	go func() {
		for u := uint64(0); u < amount; u++ {
			f.Add(ctx, mkFifoObj(string([]rune{'b', rune(u)}), u+1))
		}
	}()

	lastInt := int(0)
	lastUint := uint64(0)
	for i := 0; i < amount*2; i++ {
		switch obj := testPopTestFifoObject(t, ctx, f).val.(type) {
		case int:
			if obj <= lastInt {
				t.Errorf("got %v (int) out of order, last was %v", obj, lastInt)
			}
			lastInt = obj
		case uint64:
			if obj <= lastUint {
				t.Errorf("got %v (uint) out of order, last was %v", obj, lastUint)
			} else {
				lastUint = obj
			}
		default:
			t.Fatalf("unexpected type %#v", obj)
		}
	}
}

func TestFIFO_requeueOnPop(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	f := NewFIFO(KeyFunctionQueueOption(testFifoObjectKeyFunc))

	f.Add(ctx, mkFifoObj("foo", 10))
	_, err := f.PopAndProcess(ctx, func(ctx context.Context, obj interface{}) error {
		if obj.(testFifoObject).name != "foo" {
			t.Fatalf("unexpected object: %#v", obj)
		}
		return ErrRequeue{Err: nil}
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, ok, err := f.GetByKey("foo"); !ok || err != nil {
		t.Fatalf("object should have been requeued: %t %v", ok, err)
	}

	_, err = f.PopAndProcess(ctx, func(ctx context.Context, obj interface{}) error {
		if obj.(testFifoObject).name != "foo" {
			t.Fatalf("unexpected object: %#v", obj)
		}
		return ErrRequeue{Err: fmt.Errorf("test error")}
	})
	if err == nil || err.Error() != "test error" {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, ok, err := f.GetByKey("foo"); !ok || err != nil {
		t.Fatalf("object should have been requeued: %t %v", ok, err)
	}

	_, err = f.PopAndProcess(ctx, func(ctx context.Context, obj interface{}) error {
		if obj.(testFifoObject).name != "foo" {
			t.Fatalf("unexpected object: %#v", obj)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, ok, err := f.GetByKey("foo"); ok || err != nil {
		t.Fatalf("object should have been removed: %t %v", ok, err)
	}
}

func TestFIFO_addUpdate(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	f := NewFIFO(KeyFunctionQueueOption(testFifoObjectKeyFunc))
	f.Add(ctx, mkFifoObj("foo", 10))
	f.Update(ctx, mkFifoObj("foo", 15))

	if e, a := []interface{}{mkFifoObj("foo", 15)}, f.List(); !reflect.DeepEqual(e, a) {
		t.Errorf("Expected %+v, got %+v", e, a)
	}
	if e, a := []string{"foo"}, f.ListKeys(); !reflect.DeepEqual(e, a) {
		t.Errorf("Expected %+v, got %+v", e, a)
	}

	got := make(chan testFifoObject, 2)
	go func() {
		for {
			if val, ok := popFromQueue(ctx, f); !ok {
				break
			} else {
				got <- val.(testFifoObject)
			}
		}
	}()

	first := <-got
	if e, a := 15, first.val; e != a {
		t.Errorf("Didn't get updated value (%v), got %v", e, a)
	}
	select {
	case unexpected := <-got:
		t.Errorf("Got second value %v", unexpected.val)
	case <-time.After(50 * time.Millisecond):
	}
	_, exists, _ := f.Get(mkFifoObj("foo", ""))
	if exists {
		t.Errorf("item did not get removed")
	}
}

func TestFIFO_addReplace(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	f := NewFIFO(KeyFunctionQueueOption(testFifoObjectKeyFunc))
	f.Add(ctx, mkFifoObj("foo", 10))
	f.Replace(ctx, []interface{}{mkFifoObj("foo", 15)}, "15")
	got := make(chan testFifoObject, 2)
	go func() {
		for {
			if val, ok := popFromQueue(ctx, f); !ok {
				break
			} else {
				got <- val.(testFifoObject)
			}
		}
	}()

	first := <-got
	if e, a := 15, first.val; e != a {
		t.Errorf("Didn't get updated value (%v), got %v", e, a)
	}
	select {
	case unexpected := <-got:
		t.Errorf("Got second value %v", unexpected.val)
	case <-time.After(50 * time.Millisecond):
	}
	_, exists, _ := f.Get(mkFifoObj("foo", ""))
	if exists {
		t.Errorf("item did not get removed")
	}
}

func TestFIFO_detectLineJumpers(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	f := NewFIFO(KeyFunctionQueueOption(testFifoObjectKeyFunc))

	f.Add(ctx, mkFifoObj("foo", 10))
	f.Add(ctx, mkFifoObj("bar", 1))
	f.Add(ctx, mkFifoObj("foo", 11))
	f.Add(ctx, mkFifoObj("foo", 13))
	f.Add(ctx, mkFifoObj("zab", 30))

	if e, a := 13, testPopTestFifoObject(t, ctx, f).val; a != e {
		t.Fatalf("expected %d, got %d", e, a)
	}

	f.Add(ctx, mkFifoObj("foo", 14)) // ensure foo doesn't jump back in line

	if e, a := 1, testPopTestFifoObject(t, ctx, f).val; a != e {
		t.Fatalf("expected %d, got %d", e, a)
	}

	if e, a := 30, testPopTestFifoObject(t, ctx, f).val; a != e {
		t.Fatalf("expected %d, got %d", e, a)
	}

	if e, a := 14, testPopTestFifoObject(t, ctx, f).val; a != e {
		t.Fatalf("expected %d, got %d", e, a)
	}
}

func TestFIFO_addIfNotPresent(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	f := NewFIFO(KeyFunctionQueueOption(testFifoObjectKeyFunc))

	f.Add(ctx, mkFifoObj("a", 1))
	f.Add(ctx, mkFifoObj("b", 2))
	f.AddIfNotPresent(ctx, mkFifoObj("b", 3))
	f.AddIfNotPresent(ctx, mkFifoObj("c", 4))

	if e, a := 3, len(f.items); a != e {
		t.Fatalf("expected queue length %d, got %d", e, a)
	}

	expectedValues := []int{1, 2, 4}
	for _, expected := range expectedValues {
		if actual := testPopTestFifoObject(t, ctx, f).val; actual != expected {
			t.Fatalf("expected value %d, got %d", expected, actual)
		}
	}
}

func TestFIFO_HasSynced(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	tests := []struct {
		actions        []func(f *FIFO)
		expectedSynced bool
	}{
		{
			actions:        []func(f *FIFO){},
			expectedSynced: false,
		},
		{
			actions: []func(f *FIFO){
				func(f *FIFO) { f.Add(ctx, mkFifoObj("a", 1)) },
			},
			expectedSynced: true,
		},
		{
			actions: []func(f *FIFO){
				func(f *FIFO) { f.Replace(ctx, []interface{}{}, "0") },
			},
			expectedSynced: true,
		},
		{
			actions: []func(f *FIFO){
				func(f *FIFO) { f.Replace(ctx, []interface{}{mkFifoObj("a", 1), mkFifoObj("b", 2)}, "0") },
			},
			expectedSynced: false,
		},
		{
			actions: []func(f *FIFO){
				func(f *FIFO) { f.Replace(ctx, []interface{}{mkFifoObj("a", 1), mkFifoObj("b", 2)}, "0") },
				func(f *FIFO) { testPopTestFifoObject(t, ctx, f) },
			},
			expectedSynced: false,
		},
		{
			actions: []func(f *FIFO){
				func(f *FIFO) { f.Replace(ctx, []interface{}{mkFifoObj("a", 1), mkFifoObj("b", 2)}, "0") },
				func(f *FIFO) { testPopTestFifoObject(t, ctx, f) },
				func(f *FIFO) { testPopTestFifoObject(t, ctx, f) },
			},
			expectedSynced: true,
		},
	}

	for i, test := range tests {
		f := NewFIFO(KeyFunctionQueueOption(testFifoObjectKeyFunc))

		for _, action := range test.actions {
			action(f)
		}

		var hasSynced bool
		select {
		case <-f.HasSynced():
			hasSynced = true
		default:
			hasSynced = false
		}

		if e, a := test.expectedSynced, hasSynced; a != e {
			t.Errorf("test case %v failed, expected: %v , got %v", i, e, a)
		}
	}
}

// TestFIFO_PopShouldUnblockWhenClosed checks that any blocking Pop on an empty queue
// should unblock and return after Close is called.
func TestFIFO_PopShouldUnblockWhenClosed(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	f := NewFIFO(KeyFunctionQueueOption(testFifoObjectKeyFunc))

	c := make(chan struct{})
	const jobs = 10
	for i := 0; i < jobs; i++ {
		go func() {
			f.PopAndProcess(ctx, func(ctx context.Context, obj interface{}) error {
				return nil
			})
			c <- struct{}{}
		}()
	}

	runtime.Gosched()
	f.Close()

	for i := 0; i < jobs; i++ {
		select {
		case <-c:
		case <-time.After(500 * time.Millisecond):
			t.Fatalf("timed out waiting for Pop to return after Close")
		}
	}
}
