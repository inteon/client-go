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
	"fmt"
	"sync"

	"k8s.io/apimachinery/pkg/util/sets"
)

// Indexers maps a name to a IndexFunc
type Indexers map[string]IndexFunc

// ThreadSafeStore is an interface that allows concurrent indexed
// access to a storage backend.  It is like Indexer but does not
// (necessarily) know how to extract the Store key from a given
// object.
//
// TL;DR caveats: you must not modify anything returned by Get or List as it will break
// the indexing feature in addition to not being thread safe.
//
// The guarantees of thread safety provided by List/Get are only valid if the caller
// treats returned items as read-only. For example, a pointer inserted in the store
// through `Add` will be returned as is by `Get`. Multiple clients might invoke `Get`
// on the same key and modify the pointer in a non-thread-safe way. Also note that
// modifying objects stored by the indexers (if any) will *not* automatically lead
// to a re-index. So it's not a good idea to directly modify the objects returned by
// Get/List, in general.
type ThreadSafeStore interface {
	Add(key string, obj interface{}) error
	Update(key string, obj interface{}) error
	Delete(key string) error
	Replace(map[string]interface{}, string) error

	Rebuild() error
	GetIndexers() Indexers
	AddIndexers(newIndexers Indexers) error

	Get(key string) (item interface{}, exists bool)
	List() []interface{}
	ListKeys() []string
	Index(indexName string, obj interface{}) ([]interface{}, error)
	IndexKeys(indexName, indexKey string) ([]string, error)
	ListIndexFuncValues(name string) []string
	ByIndex(indexName, indexKey string) ([]interface{}, error)
}

// threadSafeMap implements ThreadSafeStore
type threadSafeMap struct {
	lock  sync.RWMutex
	items map[string]interface{}

	indices Indices
}

// Add item with key and value to safe map.
// An error means the action failed, the item was not added
// NOTE: on error, the indices are cleared, use .Rebuild() to try to rebuild
func (c *threadSafeMap) Add(key string, obj interface{}) error {
	return c.Update(key, obj)
}

// Update item with key and value in safe map.
// An error means the action failed, the item was not updated
// NOTE: on error, the indices are cleared, use .Rebuild() to try to rebuild
func (c *threadSafeMap) Update(key string, obj interface{}) error {
	c.lock.Lock()
	defer c.lock.Unlock()

	oldObject := c.items[key]
	if err := c.indices.update(oldObject, obj, key); err != nil {
		c.indices.clear()
		return err
	}
	c.items[key] = obj

	return nil
}

// Delete item with key from safe map.
// An error means the action failed, the item was not deleted.
// NOTE: on error, the indices are cleared, use .Rebuild() to try to rebuild
func (c *threadSafeMap) Delete(key string) error {
	c.lock.Lock()
	defer c.lock.Unlock()
	if obj, exists := c.items[key]; exists {
		if err := c.indices.update(obj, nil, key); err != nil {
			c.indices.clear()
			return err
		}
		delete(c.items, key)
	}
	return nil
}

// Replace all items in safe map.
// An error means the action failed, the safe map was not replaced
// NOTE: on error, the indices are cleared, use .Rebuild() to try to rebuild
func (c *threadSafeMap) Replace(items map[string]interface{}, resourceVersion string) error {
	c.lock.Lock()
	defer c.lock.Unlock()

	// rebuild any index
	if err := c.indices.rebuild(items); err != nil {
		c.indices.clear()
		return err
	}
	c.items = items

	return nil
}

// Try recalculating indices, usefull to recover from a failed action
// that invalidated the indices
func (c *threadSafeMap) Rebuild() error {
	return c.indices.rebuild(c.items)
}

func (c *threadSafeMap) GetIndexers() Indexers {
	indexers := make(Indexers, len(c.indices))
	for k, v := range c.indices {
		indexers[k] = v.indexFn
	}
	return indexers
}

// Add extra indexers
func (c *threadSafeMap) AddIndexers(newIndexers Indexers) error {
	c.lock.Lock()
	defer c.lock.Unlock()

	oldKeys := sets.StringKeySet(c.indices)
	newKeys := sets.StringKeySet(newIndexers)

	if oldKeys.HasAny(newKeys.List()...) {
		return fmt.Errorf("indexer conflict: %v", oldKeys.Intersection(newKeys))
	}

	for k, v := range newIndexers {
		index := &Index{indexFn: v}
		if err := index.rebuild(c.items); err != nil {
			return err
		}
		c.indices[k] = index
	}

	return nil
}

func (c *threadSafeMap) Get(key string) (item interface{}, exists bool) {
	c.lock.RLock()
	defer c.lock.RUnlock()
	item, exists = c.items[key]
	return item, exists
}

func (c *threadSafeMap) List() []interface{} {
	c.lock.RLock()
	defer c.lock.RUnlock()
	list := make([]interface{}, 0, len(c.items))
	for _, item := range c.items {
		list = append(list, item)
	}
	return list
}

// ListKeys returns a list of all the keys of the objects currently
// in the threadSafeMap.
func (c *threadSafeMap) ListKeys() []string {
	c.lock.RLock()
	defer c.lock.RUnlock()
	list := make([]string, 0, len(c.items))
	for key := range c.items {
		list = append(list, key)
	}
	return list
}

// Index returns a list of items that match the given object on the index function.
// Index is thread-safe so long as you treat all items as immutable.
func (c *threadSafeMap) Index(indexName string, obj interface{}) ([]interface{}, error) {
	c.lock.RLock()
	defer c.lock.RUnlock()

	if index, ok := c.indices[indexName]; !ok {
		return nil, fmt.Errorf("Index with name %s does not exist", indexName)
	} else if set, err := index.list(obj); err != nil {
		return nil, err
	} else {
		list := make([]interface{}, 0, set.Len())
		for key := range set {
			list = append(list, c.items[key])
		}

		return list, nil
	}
}

// ByIndex returns a list of the items whose indexed values in the given index include the given indexed value
func (c *threadSafeMap) ByIndex(indexName, indexedValue string) ([]interface{}, error) {
	c.lock.RLock()
	defer c.lock.RUnlock()

	if index, ok := c.indices[indexName]; !ok {
		return nil, fmt.Errorf("Index with name %s does not exist", indexName)
	} else if set, ok := index.listKeys(indexedValue); !ok {
		return nil, nil
	} else {
		list := make([]interface{}, 0, set.Len())
		for key := range set {
			list = append(list, c.items[key])
		}

		return list, nil
	}
}

// IndexKeys returns a list of the Store keys of the objects whose indexed values in the given index include the given indexed value.
// IndexKeys is thread-safe so long as you treat all items as immutable.
func (c *threadSafeMap) IndexKeys(indexName, indexedValue string) ([]string, error) {
	c.lock.RLock()
	defer c.lock.RUnlock()

	if index, ok := c.indices[indexName]; !ok {
		return nil, fmt.Errorf("Index with name %s does not exist", indexName)
	} else if set, ok := index.listKeys(indexedValue); !ok {
		return nil, fmt.Errorf("indexedValue %s does not exist in the index", indexedValue)
	} else {
		return set.UnsortedList(), nil
	}
}

func (c *threadSafeMap) ListIndexFuncValues(indexName string) []string {
	c.lock.RLock()
	defer c.lock.RUnlock()

	return c.indices[indexName].listValues()
}

// NewThreadSafeStore creates a new instance of ThreadSafeStore.
func NewThreadSafeStore(indexers Indexers) ThreadSafeStore {
	return &threadSafeMap{
		items:   map[string]interface{}{},
		indices: NewIndicesFromIndexers(indexers),
	}
}
