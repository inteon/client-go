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

	v1beta1 "k8s.io/api/admissionregistration/v1beta1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	labels "k8s.io/apimachinery/pkg/labels"
	schema "k8s.io/apimachinery/pkg/runtime/schema"
	types "k8s.io/apimachinery/pkg/types"
	admissionregistrationv1beta1 "k8s.io/client-go/applyconfigurations/admissionregistration/v1beta1"
	watch "k8s.io/client-go/pkg/watch"
	testing "k8s.io/client-go/testing"
)

// FakeValidatingWebhookConfigurations implements ValidatingWebhookConfigurationInterface
type FakeValidatingWebhookConfigurations struct {
	Fake *FakeAdmissionregistrationV1beta1
}

var validatingwebhookconfigurationsResource = schema.GroupVersionResource{Group: "admissionregistration.k8s.io", Version: "v1beta1", Resource: "validatingwebhookconfigurations"}

var validatingwebhookconfigurationsKind = schema.GroupVersionKind{Group: "admissionregistration.k8s.io", Version: "v1beta1", Kind: "ValidatingWebhookConfiguration"}

// Get takes name of the validatingWebhookConfiguration, and returns the corresponding validatingWebhookConfiguration object, and an error if there is any.
func (c *FakeValidatingWebhookConfigurations) Get(ctx context.Context, name string, options v1.GetOptions) (result *v1beta1.ValidatingWebhookConfiguration, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewRootGetAction(validatingwebhookconfigurationsResource, name), &v1beta1.ValidatingWebhookConfiguration{})
	if obj == nil {
		return nil, err
	}
	return obj.(*v1beta1.ValidatingWebhookConfiguration), err
}

// List takes label and field selectors, and returns the list of ValidatingWebhookConfigurations that match those selectors.
func (c *FakeValidatingWebhookConfigurations) List(ctx context.Context, opts v1.ListOptions) (result *v1beta1.ValidatingWebhookConfigurationList, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewRootListAction(validatingwebhookconfigurationsResource, validatingwebhookconfigurationsKind, opts), &v1beta1.ValidatingWebhookConfigurationList{})
	if obj == nil {
		return nil, err
	}

	label, _, _ := testing.ExtractFromListOptions(opts)
	if label == nil {
		label = labels.Everything()
	}
	list := &v1beta1.ValidatingWebhookConfigurationList{
		TypeMeta: obj.(*v1beta1.ValidatingWebhookConfigurationList).TypeMeta,
		ListMeta: obj.(*v1beta1.ValidatingWebhookConfigurationList).ListMeta,
	}
	for _, item := range obj.(*v1beta1.ValidatingWebhookConfigurationList).Items {
		if label.Matches(labels.Set(item.Labels)) {
			list.Items = append(list.Items, item)
		}
	}
	return list, err
}

// Watch returns a watch.Watcher that watches the requested validatingWebhookConfigurations.
func (c *FakeValidatingWebhookConfigurations) Watch(ctx context.Context, opts v1.ListOptions) (watch.Watcher, error) {
	return c.Fake.
		InvokesWatch(testing.NewRootWatchAction(validatingwebhookconfigurationsResource, opts))
}

// Create takes the representation of a validatingWebhookConfiguration and creates it.  Returns the server's representation of the validatingWebhookConfiguration, and an error, if there is any.
func (c *FakeValidatingWebhookConfigurations) Create(ctx context.Context, validatingWebhookConfiguration *v1beta1.ValidatingWebhookConfiguration, opts v1.CreateOptions) (result *v1beta1.ValidatingWebhookConfiguration, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewRootCreateAction(validatingwebhookconfigurationsResource, validatingWebhookConfiguration), &v1beta1.ValidatingWebhookConfiguration{})
	if obj == nil {
		return nil, err
	}
	return obj.(*v1beta1.ValidatingWebhookConfiguration), err
}

// Update takes the representation of a validatingWebhookConfiguration and updates it. Returns the server's representation of the validatingWebhookConfiguration, and an error, if there is any.
func (c *FakeValidatingWebhookConfigurations) Update(ctx context.Context, validatingWebhookConfiguration *v1beta1.ValidatingWebhookConfiguration, opts v1.UpdateOptions) (result *v1beta1.ValidatingWebhookConfiguration, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewRootUpdateAction(validatingwebhookconfigurationsResource, validatingWebhookConfiguration), &v1beta1.ValidatingWebhookConfiguration{})
	if obj == nil {
		return nil, err
	}
	return obj.(*v1beta1.ValidatingWebhookConfiguration), err
}

// Delete takes name of the validatingWebhookConfiguration and deletes it. Returns an error if one occurs.
func (c *FakeValidatingWebhookConfigurations) Delete(ctx context.Context, name string, opts v1.DeleteOptions) error {
	_, err := c.Fake.
		Invokes(testing.NewRootDeleteActionWithOptions(validatingwebhookconfigurationsResource, name, opts), &v1beta1.ValidatingWebhookConfiguration{})
	return err
}

// DeleteCollection deletes a collection of objects.
func (c *FakeValidatingWebhookConfigurations) DeleteCollection(ctx context.Context, opts v1.DeleteOptions, listOpts v1.ListOptions) error {
	action := testing.NewRootDeleteCollectionAction(validatingwebhookconfigurationsResource, listOpts)

	_, err := c.Fake.Invokes(action, &v1beta1.ValidatingWebhookConfigurationList{})
	return err
}

// Patch applies the patch and returns the patched validatingWebhookConfiguration.
func (c *FakeValidatingWebhookConfigurations) Patch(ctx context.Context, name string, pt types.PatchType, data []byte, opts v1.PatchOptions, subresources ...string) (result *v1beta1.ValidatingWebhookConfiguration, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewRootPatchSubresourceAction(validatingwebhookconfigurationsResource, name, pt, data, subresources...), &v1beta1.ValidatingWebhookConfiguration{})
	if obj == nil {
		return nil, err
	}
	return obj.(*v1beta1.ValidatingWebhookConfiguration), err
}

// Apply takes the given apply declarative configuration, applies it and returns the applied validatingWebhookConfiguration.
func (c *FakeValidatingWebhookConfigurations) Apply(ctx context.Context, validatingWebhookConfiguration *admissionregistrationv1beta1.ValidatingWebhookConfigurationApplyConfiguration, opts v1.ApplyOptions) (result *v1beta1.ValidatingWebhookConfiguration, err error) {
	if validatingWebhookConfiguration == nil {
		return nil, fmt.Errorf("validatingWebhookConfiguration provided to Apply must not be nil")
	}
	data, err := json.Marshal(validatingWebhookConfiguration)
	if err != nil {
		return nil, err
	}
	name := validatingWebhookConfiguration.Name
	if name == nil {
		return nil, fmt.Errorf("validatingWebhookConfiguration.Name must be provided to Apply")
	}
	obj, err := c.Fake.
		Invokes(testing.NewRootPatchSubresourceAction(validatingwebhookconfigurationsResource, *name, types.ApplyPatchType, data), &v1beta1.ValidatingWebhookConfiguration{})
	if obj == nil {
		return nil, err
	}
	return obj.(*v1beta1.ValidatingWebhookConfiguration), err
}
