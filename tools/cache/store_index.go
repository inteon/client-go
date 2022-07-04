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

	"k8s.io/apimachinery/pkg/util/sets"
)

// Index maps the indexed value to a set of keys in the store that match on that value
type Index struct {
	indexFn IndexFunc
	values  map[string]sets.String
}

func (index *Index) addKey(key, indexValue string) {
	set, ok := index.values[indexValue]
	if !ok || set == nil {
		set = sets.String{}
		index.values[indexValue] = set
	}
	set.Insert(key)
}

func (index *Index) deleteKey(key, indexValue string) {
	set, ok := index.values[indexValue]
	if !ok || set == nil {
		return
	}
	set.Delete(key)
	// If we don't delete the set when zero, indices with high cardinality
	// short lived resources can cause memory to increase over time from
	// unused empty sets. See `kubernetes/kubernetes/issues/84959`.
	if len(set) == 0 {
		delete(index.values, indexValue)
	}
}

func (index *Index) listKeys(indexedValues ...string) (sets.String, bool) {
	var storeKeySet sets.String

	if len(indexedValues) == 1 {
		// In majority of cases, there is exactly one value matching.
		// Optimize the most common path - deduping is not needed here.
		if set, ok := index.values[indexedValues[0]]; !ok {
			return nil, false
		} else {
			return set, true
		}
	} else {
		// Need to de-dupe the return list.
		// Since multiple keys are allowed, this can happen.
		storeKeySet = sets.String{}
		for _, indexedValue := range indexedValues {
			if set, ok := index.values[indexedValue]; !ok {
				return nil, false
			} else {
				for key := range set {
					storeKeySet.Insert(key)
				}
			}
		}
		return storeKeySet, true
	}
}

func (index *Index) listValues() []string {
	keys := make([]string, 0, len(index.values))
	for key := range index.values {
		keys = append(keys, key)
	}
	return keys
}

func (index *Index) list(obj interface{}) (sets.String, error) {
	if indexedValues, err := index.indexFn(obj); err != nil {
		return nil, err
	} else if keys, ok := index.listKeys(indexedValues...); !ok {
		return nil, fmt.Errorf("indexedValues %v do not all exist in the index", indexedValues)
	} else {
		return keys, nil
	}
}

// updateIndices modifies the objects location in the managed indexes:
// - for create you must provide only the newObj
// - for update you must provide both the oldObj and the newObj
// - for delete you must provide only the oldObj
// updateIndices must be called from a function that already has a lock on the cache
func (index *Index) update(oldObj interface{}, newObj interface{}, key string) error {
	var err error
	var oldIndexValues, newIndexValues []string

	if oldObj != nil {
		oldIndexValues, err = index.indexFn(oldObj)
	} else {
		oldIndexValues = nil
	}
	if err != nil {
		return fmt.Errorf("unable to calculate an index entry for key %q: %w", key, err)
	}

	if newObj != nil {
		newIndexValues, err = index.indexFn(newObj)
	} else {
		newIndexValues = nil
	}
	if err != nil {
		return fmt.Errorf("unable to calculate an index entry for key %q: %w", key, err)
	}

	if index.values == nil {
		index.values = map[string]sets.String{}
	}

	if len(newIndexValues) == 1 && len(oldIndexValues) == 1 && newIndexValues[0] == oldIndexValues[0] {
		// We optimize for the most common case where indexFunc returns a single value which has not been changed
		return nil
	}

	for _, value := range oldIndexValues {
		index.deleteKey(key, value)
	}
	for _, value := range newIndexValues {
		index.addKey(key, value)
	}

	return nil
}

func (index *Index) clear() {
	index.values = map[string]sets.String{}
}

func (index *Index) rebuild(items map[string]interface{}) error {
	index.clear()

	for key, newObj := range items {
		if err := index.update(nil, newObj, key); err != nil {
			return err
		}
	}
	return nil
}

// Indices maps a name to an Index
type Indices map[string]*Index

func NewIndicesFromIndexers(indexers map[string]IndexFunc) Indices {
	indices := make(Indices, len(indexers))

	for k, v := range indexers {
		indices[k] = &Index{indexFn: v}
	}

	return indices
}

func (c Indices) update(oldObj interface{}, newObj interface{}, key string) error {
	for name, index := range c {
		if err := index.update(oldObj, newObj, key); err != nil {
			return fmt.Errorf("on index %s: %w", name, err)
		}
	}
	return nil
}

func (c Indices) clear() {
	for _, index := range c {
		index.clear()
	}
}

func (c Indices) rebuild(items map[string]interface{}) error {
	for name, index := range c {
		if err := index.rebuild(items); err != nil {
			return fmt.Errorf("on index %s: %w", name, err)
		}
	}
	return nil
}
