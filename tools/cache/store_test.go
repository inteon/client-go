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
	"errors"
	"testing"
	"time"

	"k8s.io/apimachinery/pkg/util/sets"
)

// Test public interface
func doTestStore(t *testing.T, store Store) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	mkObj := func(id string, val string) testStoreObject {
		return testStoreObject{id: id, val: val}
	}

	store.Add(ctx, mkObj("foo", "bar"))
	if item, ok, _ := store.Get(mkObj("foo", "")); !ok {
		t.Errorf("didn't find inserted item")
	} else {
		if e, a := "bar", item.(testStoreObject).val; e != a {
			t.Errorf("expected %v, got %v", e, a)
		}
	}
	store.Update(ctx, mkObj("foo", "baz"))
	if item, ok, _ := store.Get(mkObj("foo", "")); !ok {
		t.Errorf("didn't find inserted item")
	} else {
		if e, a := "baz", item.(testStoreObject).val; e != a {
			t.Errorf("expected %v, got %v", e, a)
		}
	}
	store.Delete(ctx, mkObj("foo", ""))
	if _, ok, _ := store.Get(mkObj("foo", "")); ok {
		t.Errorf("found deleted item??")
	}

	// Test List.
	store.Add(ctx, mkObj("a", "b"))
	store.Add(ctx, mkObj("c", "d"))
	store.Add(ctx, mkObj("e", "e"))
	{
		found := sets.String{}
		for _, item := range store.List() {
			found.Insert(item.(testStoreObject).val)
		}
		if !found.HasAll("b", "d", "e") {
			t.Errorf("missing items, found: %v", found)
		}
		if len(found) != 3 {
			t.Errorf("extra items")
		}
	}

	// Test Replace.
	store.Replace(ctx, []interface{}{
		mkObj("foo", "foo"),
		mkObj("bar", "bar"),
	}, "0")

	{
		found := sets.String{}
		for _, item := range store.List() {
			found.Insert(item.(testStoreObject).val)
		}
		if !found.HasAll("foo", "bar") {
			t.Errorf("missing items")
		}
		if len(found) != 2 {
			t.Errorf("extra items")
		}
	}
}

// Test public interface
func doTestIndex(t *testing.T, indexer Indexer) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	mkObj := func(id string, val string) testStoreObject {
		return testStoreObject{id: id, val: val}
	}

	// Test Index
	expected := map[string]sets.String{}
	expected["b"] = sets.NewString("a", "c")
	expected["f"] = sets.NewString("e")
	expected["h"] = sets.NewString("g")
	indexer.Add(ctx, mkObj("a", "b"))
	indexer.Add(ctx, mkObj("c", "b"))
	indexer.Add(ctx, mkObj("e", "f"))
	indexer.Add(ctx, mkObj("g", "h"))
	{
		for k, v := range expected {
			found := sets.String{}
			indexResults, err := indexer.Index("by_val", mkObj("", k))
			if err != nil {
				t.Errorf("Unexpected error %v", err)
			}
			for _, item := range indexResults {
				found.Insert(item.(testStoreObject).id)
			}
			items := v.List()
			if !found.HasAll(items...) {
				t.Errorf("missing items, index %s, expected %v but found %v", k, items, found.List())
			}
		}
	}
}

func testStoreKeyFunc(obj interface{}) (string, error) {
	return obj.(testStoreObject).id, nil
}

func testStoreIndexFunc(obj interface{}) ([]string, error) {
	return []string{obj.(testStoreObject).val}, nil
}

func testStoreIndexers() Indexers {
	indexers := Indexers{}
	indexers["by_val"] = testStoreIndexFunc
	return indexers
}

type testStoreObject struct {
	id  string
	val string
}

func TestCache(t *testing.T) {
	doTestStore(t, NewStore(testStoreKeyFunc))
}

func TestFIFOCache(t *testing.T) {
	doTestStore(t, NewFIFO(KeyFunctionQueueOption(testStoreKeyFunc)))
}

func TestUndeltaStore(t *testing.T) {
	nop := func([]interface{}) {}
	doTestStore(t, NewUndeltaStore(nop, testStoreKeyFunc))
}

func TestIndex(t *testing.T) {
	doTestIndex(t, NewIndexer(testStoreKeyFunc, testStoreIndexers()))
}

func TestKeyError(t *testing.T) {
	obj := 100
	err := errors.New("error")
	keyErr := KeyError{obj, err}

	if errors.Unwrap(keyErr) != err {
		t.Errorf("expected unwrap error: %v", err)
	}

	nestedKeyErr := KeyError{obj, keyErr}
	if !errors.Is(keyErr, err) || !errors.Is(nestedKeyErr, err) {
		t.Errorf("not match target error: %v", err)
	}
}
