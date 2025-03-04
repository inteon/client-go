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

	v1beta1 "k8s.io/api/policy/v1beta1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	labels "k8s.io/apimachinery/pkg/labels"
	schema "k8s.io/apimachinery/pkg/runtime/schema"
	types "k8s.io/apimachinery/pkg/types"
	policyv1beta1 "k8s.io/client-go/applyconfigurations/policy/v1beta1"
	watch "k8s.io/client-go/pkg/watch"
	testing "k8s.io/client-go/testing"
)

// FakePodSecurityPolicies implements PodSecurityPolicyInterface
type FakePodSecurityPolicies struct {
	Fake *FakePolicyV1beta1
}

var podsecuritypoliciesResource = schema.GroupVersionResource{Group: "policy", Version: "v1beta1", Resource: "podsecuritypolicies"}

var podsecuritypoliciesKind = schema.GroupVersionKind{Group: "policy", Version: "v1beta1", Kind: "PodSecurityPolicy"}

// Get takes name of the podSecurityPolicy, and returns the corresponding podSecurityPolicy object, and an error if there is any.
func (c *FakePodSecurityPolicies) Get(ctx context.Context, name string, options v1.GetOptions) (result *v1beta1.PodSecurityPolicy, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewRootGetAction(podsecuritypoliciesResource, name), &v1beta1.PodSecurityPolicy{})
	if obj == nil {
		return nil, err
	}
	return obj.(*v1beta1.PodSecurityPolicy), err
}

// List takes label and field selectors, and returns the list of PodSecurityPolicies that match those selectors.
func (c *FakePodSecurityPolicies) List(ctx context.Context, opts v1.ListOptions) (result *v1beta1.PodSecurityPolicyList, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewRootListAction(podsecuritypoliciesResource, podsecuritypoliciesKind, opts), &v1beta1.PodSecurityPolicyList{})
	if obj == nil {
		return nil, err
	}

	label, _, _ := testing.ExtractFromListOptions(opts)
	if label == nil {
		label = labels.Everything()
	}
	list := &v1beta1.PodSecurityPolicyList{
		TypeMeta: obj.(*v1beta1.PodSecurityPolicyList).TypeMeta,
		ListMeta: obj.(*v1beta1.PodSecurityPolicyList).ListMeta,
	}
	for _, item := range obj.(*v1beta1.PodSecurityPolicyList).Items {
		if label.Matches(labels.Set(item.Labels)) {
			list.Items = append(list.Items, item)
		}
	}
	return list, err
}

// Watch returns a watch.Watcher that watches the requested podSecurityPolicies.
func (c *FakePodSecurityPolicies) Watch(ctx context.Context, opts v1.ListOptions) (watch.Watcher, error) {
	return c.Fake.
		InvokesWatch(testing.NewRootWatchAction(podsecuritypoliciesResource, opts))
}

// Create takes the representation of a podSecurityPolicy and creates it.  Returns the server's representation of the podSecurityPolicy, and an error, if there is any.
func (c *FakePodSecurityPolicies) Create(ctx context.Context, podSecurityPolicy *v1beta1.PodSecurityPolicy, opts v1.CreateOptions) (result *v1beta1.PodSecurityPolicy, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewRootCreateAction(podsecuritypoliciesResource, podSecurityPolicy), &v1beta1.PodSecurityPolicy{})
	if obj == nil {
		return nil, err
	}
	return obj.(*v1beta1.PodSecurityPolicy), err
}

// Update takes the representation of a podSecurityPolicy and updates it. Returns the server's representation of the podSecurityPolicy, and an error, if there is any.
func (c *FakePodSecurityPolicies) Update(ctx context.Context, podSecurityPolicy *v1beta1.PodSecurityPolicy, opts v1.UpdateOptions) (result *v1beta1.PodSecurityPolicy, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewRootUpdateAction(podsecuritypoliciesResource, podSecurityPolicy), &v1beta1.PodSecurityPolicy{})
	if obj == nil {
		return nil, err
	}
	return obj.(*v1beta1.PodSecurityPolicy), err
}

// Delete takes name of the podSecurityPolicy and deletes it. Returns an error if one occurs.
func (c *FakePodSecurityPolicies) Delete(ctx context.Context, name string, opts v1.DeleteOptions) error {
	_, err := c.Fake.
		Invokes(testing.NewRootDeleteActionWithOptions(podsecuritypoliciesResource, name, opts), &v1beta1.PodSecurityPolicy{})
	return err
}

// DeleteCollection deletes a collection of objects.
func (c *FakePodSecurityPolicies) DeleteCollection(ctx context.Context, opts v1.DeleteOptions, listOpts v1.ListOptions) error {
	action := testing.NewRootDeleteCollectionAction(podsecuritypoliciesResource, listOpts)

	_, err := c.Fake.Invokes(action, &v1beta1.PodSecurityPolicyList{})
	return err
}

// Patch applies the patch and returns the patched podSecurityPolicy.
func (c *FakePodSecurityPolicies) Patch(ctx context.Context, name string, pt types.PatchType, data []byte, opts v1.PatchOptions, subresources ...string) (result *v1beta1.PodSecurityPolicy, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewRootPatchSubresourceAction(podsecuritypoliciesResource, name, pt, data, subresources...), &v1beta1.PodSecurityPolicy{})
	if obj == nil {
		return nil, err
	}
	return obj.(*v1beta1.PodSecurityPolicy), err
}

// Apply takes the given apply declarative configuration, applies it and returns the applied podSecurityPolicy.
func (c *FakePodSecurityPolicies) Apply(ctx context.Context, podSecurityPolicy *policyv1beta1.PodSecurityPolicyApplyConfiguration, opts v1.ApplyOptions) (result *v1beta1.PodSecurityPolicy, err error) {
	if podSecurityPolicy == nil {
		return nil, fmt.Errorf("podSecurityPolicy provided to Apply must not be nil")
	}
	data, err := json.Marshal(podSecurityPolicy)
	if err != nil {
		return nil, err
	}
	name := podSecurityPolicy.Name
	if name == nil {
		return nil, fmt.Errorf("podSecurityPolicy.Name must be provided to Apply")
	}
	obj, err := c.Fake.
		Invokes(testing.NewRootPatchSubresourceAction(podsecuritypoliciesResource, *name, types.ApplyPatchType, data), &v1beta1.PodSecurityPolicy{})
	if obj == nil {
		return nil, err
	}
	return obj.(*v1beta1.PodSecurityPolicy), err
}
