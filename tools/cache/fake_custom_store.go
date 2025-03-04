/*
Copyright 2016 The Kubernetes Authors.

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

import "context"

// FakeCustomStore lets you define custom functions for store operations.
type FakeCustomStore struct {
	AddFunc      func(ctx context.Context, obj interface{}) error
	UpdateFunc   func(ctx context.Context, obj interface{}) error
	DeleteFunc   func(ctx context.Context, obj interface{}) error
	ReplaceFunc  func(ctx context.Context, list []interface{}, resourceVersion string) error
	ListFunc     func() []interface{}
	ListKeysFunc func() []string
	GetFunc      func(obj interface{}) (item interface{}, exists bool, err error)
	GetByKeyFunc func(key string) (item interface{}, exists bool, err error)
}

// Add calls the custom Add function if defined
func (f *FakeCustomStore) Add(ctx context.Context, obj interface{}) error {
	if f.AddFunc != nil {
		return f.AddFunc(ctx, obj)
	}
	return nil
}

// Update calls the custom Update function if defined
func (f *FakeCustomStore) Update(ctx context.Context, obj interface{}) error {
	if f.UpdateFunc != nil {
		return f.UpdateFunc(ctx, obj)
	}
	return nil
}

// Delete calls the custom Delete function if defined
func (f *FakeCustomStore) Delete(ctx context.Context, obj interface{}) error {
	if f.DeleteFunc != nil {
		return f.DeleteFunc(ctx, obj)
	}
	return nil
}

// Replace calls the custom Replace function if defined
func (f *FakeCustomStore) Replace(ctx context.Context, list []interface{}, resourceVersion string) error {
	if f.ReplaceFunc != nil {
		return f.ReplaceFunc(ctx, list, resourceVersion)
	}
	return nil
}

// List calls the custom List function if defined
func (f *FakeCustomStore) List() []interface{} {
	if f.ListFunc != nil {
		return f.ListFunc()
	}
	return nil
}

// ListKeys calls the custom ListKeys function if defined
func (f *FakeCustomStore) ListKeys() []string {
	if f.ListKeysFunc != nil {
		return f.ListKeysFunc()
	}
	return nil
}

// Get calls the custom Get function if defined
func (f *FakeCustomStore) Get(obj interface{}) (item interface{}, exists bool, err error) {
	if f.GetFunc != nil {
		return f.GetFunc(obj)
	}
	return nil, false, nil
}

// GetByKey calls the custom GetByKey function if defined
func (f *FakeCustomStore) GetByKey(key string) (item interface{}, exists bool, err error) {
	if f.GetByKeyFunc != nil {
		return f.GetByKeyFunc(key)
	}
	return nil, false, nil
}
