/*
Copyright The Kubernetes Authors.

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

// Code generated by client-gen. DO NOT EDIT.

package fake

import (
	"context"
	json "encoding/json"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	labels "k8s.io/apimachinery/pkg/labels"
	schema "k8s.io/apimachinery/pkg/runtime/schema"
	types "k8s.io/apimachinery/pkg/types"
	applyconfigurationscorev1 "k8s.io/client-go/applyconfigurations/core/v1"
	watch "k8s.io/client-go/pkg/watch"
	testing "k8s.io/client-go/testing"
)

// FakeComponentStatuses implements ComponentStatusInterface
type FakeComponentStatuses struct {
	Fake *FakeCoreV1
}

var componentstatusesResource = schema.GroupVersionResource{Group: "", Version: "v1", Resource: "componentstatuses"}

var componentstatusesKind = schema.GroupVersionKind{Group: "", Version: "v1", Kind: "ComponentStatus"}

// Get takes name of the componentStatus, and returns the corresponding componentStatus object, and an error if there is any.
func (c *FakeComponentStatuses) Get(ctx context.Context, name string, options v1.GetOptions) (result *corev1.ComponentStatus, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewRootGetAction(componentstatusesResource, name), &corev1.ComponentStatus{})
	if obj == nil {
		return nil, err
	}
	return obj.(*corev1.ComponentStatus), err
}

// List takes label and field selectors, and returns the list of ComponentStatuses that match those selectors.
func (c *FakeComponentStatuses) List(ctx context.Context, opts v1.ListOptions) (result *corev1.ComponentStatusList, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewRootListAction(componentstatusesResource, componentstatusesKind, opts), &corev1.ComponentStatusList{})
	if obj == nil {
		return nil, err
	}

	label, _, _ := testing.ExtractFromListOptions(opts)
	if label == nil {
		label = labels.Everything()
	}
	list := &corev1.ComponentStatusList{
		TypeMeta: obj.(*corev1.ComponentStatusList).TypeMeta,
		ListMeta: obj.(*corev1.ComponentStatusList).ListMeta,
	}
	for _, item := range obj.(*corev1.ComponentStatusList).Items {
		if label.Matches(labels.Set(item.Labels)) {
			list.Items = append(list.Items, item)
		}
	}
	return list, err
}

// Watch returns a watch.Watcher that watches the requested componentStatuses.
func (c *FakeComponentStatuses) Watch(ctx context.Context, opts v1.ListOptions) (watch.Watcher, error) {
	return c.Fake.
		InvokesWatch(testing.NewRootWatchAction(componentstatusesResource, opts))
}

// Create takes the representation of a componentStatus and creates it.  Returns the server's representation of the componentStatus, and an error, if there is any.
func (c *FakeComponentStatuses) Create(ctx context.Context, componentStatus *corev1.ComponentStatus, opts v1.CreateOptions) (result *corev1.ComponentStatus, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewRootCreateAction(componentstatusesResource, componentStatus), &corev1.ComponentStatus{})
	if obj == nil {
		return nil, err
	}
	return obj.(*corev1.ComponentStatus), err
}

// Update takes the representation of a componentStatus and updates it. Returns the server's representation of the componentStatus, and an error, if there is any.
func (c *FakeComponentStatuses) Update(ctx context.Context, componentStatus *corev1.ComponentStatus, opts v1.UpdateOptions) (result *corev1.ComponentStatus, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewRootUpdateAction(componentstatusesResource, componentStatus), &corev1.ComponentStatus{})
	if obj == nil {
		return nil, err
	}
	return obj.(*corev1.ComponentStatus), err
}

// Delete takes name of the componentStatus and deletes it. Returns an error if one occurs.
func (c *FakeComponentStatuses) Delete(ctx context.Context, name string, opts v1.DeleteOptions) error {
	_, err := c.Fake.
		Invokes(testing.NewRootDeleteActionWithOptions(componentstatusesResource, name, opts), &corev1.ComponentStatus{})
	return err
}

// DeleteCollection deletes a collection of objects.
func (c *FakeComponentStatuses) DeleteCollection(ctx context.Context, opts v1.DeleteOptions, listOpts v1.ListOptions) error {
	action := testing.NewRootDeleteCollectionAction(componentstatusesResource, listOpts)

	_, err := c.Fake.Invokes(action, &corev1.ComponentStatusList{})
	return err
}

// Patch applies the patch and returns the patched componentStatus.
func (c *FakeComponentStatuses) Patch(ctx context.Context, name string, pt types.PatchType, data []byte, opts v1.PatchOptions, subresources ...string) (result *corev1.ComponentStatus, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewRootPatchSubresourceAction(componentstatusesResource, name, pt, data, subresources...), &corev1.ComponentStatus{})
	if obj == nil {
		return nil, err
	}
	return obj.(*corev1.ComponentStatus), err
}

// Apply takes the given apply declarative configuration, applies it and returns the applied componentStatus.
func (c *FakeComponentStatuses) Apply(ctx context.Context, componentStatus *applyconfigurationscorev1.ComponentStatusApplyConfiguration, opts v1.ApplyOptions) (result *corev1.ComponentStatus, err error) {
	if componentStatus == nil {
		return nil, fmt.Errorf("componentStatus provided to Apply must not be nil")
	}
	data, err := json.Marshal(componentStatus)
	if err != nil {
		return nil, err
	}
	name := componentStatus.Name
	if name == nil {
		return nil, fmt.Errorf("componentStatus.Name must be provided to Apply")
	}
	obj, err := c.Fake.
		Invokes(testing.NewRootPatchSubresourceAction(componentstatusesResource, *name, types.ApplyPatchType, data), &corev1.ComponentStatus{})
	if obj == nil {
		return nil, err
	}
	return obj.(*corev1.ComponentStatus), err
}
