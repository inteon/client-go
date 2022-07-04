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

	rbacv1 "k8s.io/api/rbac/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	labels "k8s.io/apimachinery/pkg/labels"
	schema "k8s.io/apimachinery/pkg/runtime/schema"
	types "k8s.io/apimachinery/pkg/types"
	applyconfigurationsrbacv1 "k8s.io/client-go/applyconfigurations/rbac/v1"
	watch "k8s.io/client-go/pkg/watch"
	testing "k8s.io/client-go/testing"
)

// FakeClusterRoleBindings implements ClusterRoleBindingInterface
type FakeClusterRoleBindings struct {
	Fake *FakeRbacV1
}

var clusterrolebindingsResource = schema.GroupVersionResource{Group: "rbac.authorization.k8s.io", Version: "v1", Resource: "clusterrolebindings"}

var clusterrolebindingsKind = schema.GroupVersionKind{Group: "rbac.authorization.k8s.io", Version: "v1", Kind: "ClusterRoleBinding"}

// Get takes name of the clusterRoleBinding, and returns the corresponding clusterRoleBinding object, and an error if there is any.
func (c *FakeClusterRoleBindings) Get(ctx context.Context, name string, options v1.GetOptions) (result *rbacv1.ClusterRoleBinding, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewRootGetAction(clusterrolebindingsResource, name), &rbacv1.ClusterRoleBinding{})
	if obj == nil {
		return nil, err
	}
	return obj.(*rbacv1.ClusterRoleBinding), err
}

// List takes label and field selectors, and returns the list of ClusterRoleBindings that match those selectors.
func (c *FakeClusterRoleBindings) List(ctx context.Context, opts v1.ListOptions) (result *rbacv1.ClusterRoleBindingList, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewRootListAction(clusterrolebindingsResource, clusterrolebindingsKind, opts), &rbacv1.ClusterRoleBindingList{})
	if obj == nil {
		return nil, err
	}

	label, _, _ := testing.ExtractFromListOptions(opts)
	if label == nil {
		label = labels.Everything()
	}
	list := &rbacv1.ClusterRoleBindingList{
		TypeMeta: obj.(*rbacv1.ClusterRoleBindingList).TypeMeta,
		ListMeta: obj.(*rbacv1.ClusterRoleBindingList).ListMeta,
	}
	for _, item := range obj.(*rbacv1.ClusterRoleBindingList).Items {
		if label.Matches(labels.Set(item.Labels)) {
			list.Items = append(list.Items, item)
		}
	}
	return list, err
}

// Watch returns a watch.Watcher that watches the requested clusterRoleBindings.
func (c *FakeClusterRoleBindings) Watch(ctx context.Context, opts v1.ListOptions) (watch.Watcher, error) {
	return c.Fake.
		InvokesWatch(testing.NewRootWatchAction(clusterrolebindingsResource, opts))
}

// Create takes the representation of a clusterRoleBinding and creates it.  Returns the server's representation of the clusterRoleBinding, and an error, if there is any.
func (c *FakeClusterRoleBindings) Create(ctx context.Context, clusterRoleBinding *rbacv1.ClusterRoleBinding, opts v1.CreateOptions) (result *rbacv1.ClusterRoleBinding, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewRootCreateAction(clusterrolebindingsResource, clusterRoleBinding), &rbacv1.ClusterRoleBinding{})
	if obj == nil {
		return nil, err
	}
	return obj.(*rbacv1.ClusterRoleBinding), err
}

// Update takes the representation of a clusterRoleBinding and updates it. Returns the server's representation of the clusterRoleBinding, and an error, if there is any.
func (c *FakeClusterRoleBindings) Update(ctx context.Context, clusterRoleBinding *rbacv1.ClusterRoleBinding, opts v1.UpdateOptions) (result *rbacv1.ClusterRoleBinding, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewRootUpdateAction(clusterrolebindingsResource, clusterRoleBinding), &rbacv1.ClusterRoleBinding{})
	if obj == nil {
		return nil, err
	}
	return obj.(*rbacv1.ClusterRoleBinding), err
}

// Delete takes name of the clusterRoleBinding and deletes it. Returns an error if one occurs.
func (c *FakeClusterRoleBindings) Delete(ctx context.Context, name string, opts v1.DeleteOptions) error {
	_, err := c.Fake.
		Invokes(testing.NewRootDeleteActionWithOptions(clusterrolebindingsResource, name, opts), &rbacv1.ClusterRoleBinding{})
	return err
}

// DeleteCollection deletes a collection of objects.
func (c *FakeClusterRoleBindings) DeleteCollection(ctx context.Context, opts v1.DeleteOptions, listOpts v1.ListOptions) error {
	action := testing.NewRootDeleteCollectionAction(clusterrolebindingsResource, listOpts)

	_, err := c.Fake.Invokes(action, &rbacv1.ClusterRoleBindingList{})
	return err
}

// Patch applies the patch and returns the patched clusterRoleBinding.
func (c *FakeClusterRoleBindings) Patch(ctx context.Context, name string, pt types.PatchType, data []byte, opts v1.PatchOptions, subresources ...string) (result *rbacv1.ClusterRoleBinding, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewRootPatchSubresourceAction(clusterrolebindingsResource, name, pt, data, subresources...), &rbacv1.ClusterRoleBinding{})
	if obj == nil {
		return nil, err
	}
	return obj.(*rbacv1.ClusterRoleBinding), err
}

// Apply takes the given apply declarative configuration, applies it and returns the applied clusterRoleBinding.
func (c *FakeClusterRoleBindings) Apply(ctx context.Context, clusterRoleBinding *applyconfigurationsrbacv1.ClusterRoleBindingApplyConfiguration, opts v1.ApplyOptions) (result *rbacv1.ClusterRoleBinding, err error) {
	if clusterRoleBinding == nil {
		return nil, fmt.Errorf("clusterRoleBinding provided to Apply must not be nil")
	}
	data, err := json.Marshal(clusterRoleBinding)
	if err != nil {
		return nil, err
	}
	name := clusterRoleBinding.Name
	if name == nil {
		return nil, fmt.Errorf("clusterRoleBinding.Name must be provided to Apply")
	}
	obj, err := c.Fake.
		Invokes(testing.NewRootPatchSubresourceAction(clusterrolebindingsResource, *name, types.ApplyPatchType, data), &rbacv1.ClusterRoleBinding{})
	if obj == nil {
		return nil, err
	}
	return obj.(*rbacv1.ClusterRoleBinding), err
}
