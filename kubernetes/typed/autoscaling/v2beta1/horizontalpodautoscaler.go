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

package v2beta1

import (
	"context"
	json "encoding/json"
	"fmt"
	"time"

	v2beta1 "k8s.io/api/autoscaling/v2beta1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	types "k8s.io/apimachinery/pkg/types"
	autoscalingv2beta1 "k8s.io/client-go/applyconfigurations/autoscaling/v2beta1"
	watch "k8s.io/client-go/pkg/watch"
	rest "k8s.io/client-go/rest"
)

// HorizontalPodAutoscalersGetter has a method to return a HorizontalPodAutoscalerInterface.
// A group's client should implement this interface.
type HorizontalPodAutoscalersGetter interface {
	HorizontalPodAutoscalers(namespace string) HorizontalPodAutoscalerInterface
}

// HorizontalPodAutoscalerInterface has methods to work with HorizontalPodAutoscaler resources.
type HorizontalPodAutoscalerInterface interface {
	Create(ctx context.Context, horizontalPodAutoscaler *v2beta1.HorizontalPodAutoscaler, options v1.CreateOptions) (*v2beta1.HorizontalPodAutoscaler, error)
	Update(ctx context.Context, horizontalPodAutoscaler *v2beta1.HorizontalPodAutoscaler, options v1.UpdateOptions) (*v2beta1.HorizontalPodAutoscaler, error)
	UpdateStatus(ctx context.Context, horizontalPodAutoscaler *v2beta1.HorizontalPodAutoscaler, options v1.UpdateOptions) (*v2beta1.HorizontalPodAutoscaler, error)
	Delete(ctx context.Context, name string, options v1.DeleteOptions) error
	DeleteCollection(ctx context.Context, options v1.DeleteOptions, listOptions v1.ListOptions) error
	Get(ctx context.Context, name string, options v1.GetOptions) (*v2beta1.HorizontalPodAutoscaler, error)
	List(ctx context.Context, options v1.ListOptions) (*v2beta1.HorizontalPodAutoscalerList, error)
	Watch(ctx context.Context, options v1.ListOptions) (watch.Watcher, error)
	Patch(ctx context.Context, name string, pt types.PatchType, data []byte, options v1.PatchOptions, subresources ...string) (result *v2beta1.HorizontalPodAutoscaler, err error)
	Apply(ctx context.Context, horizontalPodAutoscaler *autoscalingv2beta1.HorizontalPodAutoscalerApplyConfiguration, options v1.ApplyOptions) (result *v2beta1.HorizontalPodAutoscaler, err error)
	ApplyStatus(ctx context.Context, horizontalPodAutoscaler *autoscalingv2beta1.HorizontalPodAutoscalerApplyConfiguration, options v1.ApplyOptions) (result *v2beta1.HorizontalPodAutoscaler, err error)
	HorizontalPodAutoscalerExpansion
}

// horizontalPodAutoscalers implements HorizontalPodAutoscalerInterface
type horizontalPodAutoscalers struct {
	client rest.Interface
	ns     string
}

// newHorizontalPodAutoscalers returns a HorizontalPodAutoscalers
func newHorizontalPodAutoscalers(c *AutoscalingV2beta1Client, namespace string) *horizontalPodAutoscalers {
	return &horizontalPodAutoscalers{
		client: c.RESTClient(),
		ns:     namespace,
	}
}

// Get takes name of the horizontalPodAutoscaler, and returns the corresponding horizontalPodAutoscaler object, and an error if there is any.
func (c *horizontalPodAutoscalers) Get(ctx context.Context, name string, options v1.GetOptions) (result *v2beta1.HorizontalPodAutoscaler, err error) {
	result = &v2beta1.HorizontalPodAutoscaler{}
	err = c.client.Get().
		ApiPath("/apis").
		GroupVersion(v2beta1.SchemeGroupVersion).
		Namespace(c.ns).
		Resource("horizontalpodautoscalers").
		Name(name).
		VersionedParams(&options).
		ExpectKind("HorizontalPodAutoscaler").
		Do(ctx).
		Into(result)
	return
}

// List takes label and field selectors, and returns the list of HorizontalPodAutoscalers that match those selectors.
func (c *horizontalPodAutoscalers) List(ctx context.Context, options v1.ListOptions) (result *v2beta1.HorizontalPodAutoscalerList, err error) {
	var timeout time.Duration
	if options.TimeoutSeconds != nil {
		timeout = time.Duration(*options.TimeoutSeconds) * time.Second
	}
	result = &v2beta1.HorizontalPodAutoscalerList{}
	err = c.client.Get().
		ApiPath("/apis").
		GroupVersion(v2beta1.SchemeGroupVersion).
		Namespace(c.ns).
		Resource("horizontalpodautoscalers").
		VersionedParams(&options).
		Timeout(timeout).
		ExpectKind("HorizontalPodAutoscaler").
		Do(ctx).
		Into(result)
	return
}

// Watch returns a watch.Watcher that watches the requested horizontalPodAutoscalers.
func (c *horizontalPodAutoscalers) Watch(ctx context.Context, options v1.ListOptions) (watch.Watcher, error) {
	var timeout time.Duration
	if options.TimeoutSeconds != nil {
		timeout = time.Duration(*options.TimeoutSeconds) * time.Second
	}
	options.Watch = true
	return c.client.Get().
		ApiPath("/apis").
		GroupVersion(v2beta1.SchemeGroupVersion).
		Namespace(c.ns).
		Resource("horizontalpodautoscalers").
		VersionedParams(&options).
		Timeout(timeout).
		ExpectKind("HorizontalPodAutoscaler").
		Watch(ctx)
}

// Create takes the representation of a horizontalPodAutoscaler and creates it.  Returns the server's representation of the horizontalPodAutoscaler, and an error, if there is any.
func (c *horizontalPodAutoscalers) Create(ctx context.Context, horizontalPodAutoscaler *v2beta1.HorizontalPodAutoscaler, options v1.CreateOptions) (result *v2beta1.HorizontalPodAutoscaler, err error) {
	result = &v2beta1.HorizontalPodAutoscaler{}
	err = c.client.Post().
		ApiPath("/apis").
		GroupVersion(v2beta1.SchemeGroupVersion).
		Namespace(c.ns).
		Resource("horizontalpodautoscalers").
		VersionedParams(&options).
		Body(horizontalPodAutoscaler).
		ExpectKind("HorizontalPodAutoscaler").
		Do(ctx).
		Into(result)
	return
}

// Update takes the representation of a horizontalPodAutoscaler and updates it. Returns the server's representation of the horizontalPodAutoscaler, and an error, if there is any.
func (c *horizontalPodAutoscalers) Update(ctx context.Context, horizontalPodAutoscaler *v2beta1.HorizontalPodAutoscaler, options v1.UpdateOptions) (result *v2beta1.HorizontalPodAutoscaler, err error) {
	result = &v2beta1.HorizontalPodAutoscaler{}
	err = c.client.Put().
		ApiPath("/apis").
		GroupVersion(v2beta1.SchemeGroupVersion).
		Namespace(c.ns).
		Resource("horizontalpodautoscalers").
		Name(horizontalPodAutoscaler.Name).
		VersionedParams(&options).
		Body(horizontalPodAutoscaler).
		ExpectKind("HorizontalPodAutoscaler").
		Do(ctx).
		Into(result)
	return
}

// UpdateStatus was generated because the type contains a Status member.
// Add a +genclient:noStatus comment above the type to avoid generating UpdateStatus().
func (c *horizontalPodAutoscalers) UpdateStatus(ctx context.Context, horizontalPodAutoscaler *v2beta1.HorizontalPodAutoscaler, options v1.UpdateOptions) (result *v2beta1.HorizontalPodAutoscaler, err error) {
	result = &v2beta1.HorizontalPodAutoscaler{}
	err = c.client.Put().
		ApiPath("/apis").
		GroupVersion(v2beta1.SchemeGroupVersion).
		Namespace(c.ns).
		Resource("horizontalpodautoscalers").
		Name(horizontalPodAutoscaler.Name).
		SubResource("status").
		VersionedParams(&options).
		Body(horizontalPodAutoscaler).
		ExpectKind("HorizontalPodAutoscaler").
		Do(ctx).
		Into(result)
	return
}

// Delete takes name of the horizontalPodAutoscaler and deletes it. Returns an error if one occurs.
func (c *horizontalPodAutoscalers) Delete(ctx context.Context, name string, options v1.DeleteOptions) error {
	return c.client.Delete().
		ApiPath("/apis").
		GroupVersion(v2beta1.SchemeGroupVersion).
		Namespace(c.ns).
		Resource("horizontalpodautoscalers").
		Name(name).
		Body(&options).
		ExpectKind("HorizontalPodAutoscaler").
		Do(ctx).
		Error()
}

// DeleteCollection deletes a collection of objects.
func (c *horizontalPodAutoscalers) DeleteCollection(ctx context.Context, options v1.DeleteOptions, listOptions v1.ListOptions) error {
	var timeout time.Duration
	if listOptions.TimeoutSeconds != nil {
		timeout = time.Duration(*listOptions.TimeoutSeconds) * time.Second
	}
	return c.client.Delete().
		ApiPath("/apis").
		GroupVersion(v2beta1.SchemeGroupVersion).
		Namespace(c.ns).
		Resource("horizontalpodautoscalers").
		VersionedParams(&listOptions).
		Timeout(timeout).
		Body(&options).
		ExpectKind("HorizontalPodAutoscaler").
		Do(ctx).
		Error()
}

// Patch applies the patch and returns the patched horizontalPodAutoscaler.
func (c *horizontalPodAutoscalers) Patch(ctx context.Context, name string, pt types.PatchType, data []byte, options v1.PatchOptions, subresources ...string) (result *v2beta1.HorizontalPodAutoscaler, err error) {
	result = &v2beta1.HorizontalPodAutoscaler{}
	err = c.client.Patch(pt).
		ApiPath("/apis").
		GroupVersion(v2beta1.SchemeGroupVersion).
		Namespace(c.ns).
		Resource("horizontalpodautoscalers").
		Name(name).
		SubResource(subresources...).
		VersionedParams(&options).
		Body(data).
		ExpectKind("HorizontalPodAutoscaler").
		Do(ctx).
		Into(result)
	return
}

// Apply takes the given apply declarative configuration, applies it and returns the applied horizontalPodAutoscaler.
func (c *horizontalPodAutoscalers) Apply(ctx context.Context, horizontalPodAutoscaler *autoscalingv2beta1.HorizontalPodAutoscalerApplyConfiguration, options v1.ApplyOptions) (result *v2beta1.HorizontalPodAutoscaler, err error) {
	if horizontalPodAutoscaler == nil {
		return nil, fmt.Errorf("horizontalPodAutoscaler provided to Apply must not be nil")
	}
	patchOpts := options.ToPatchOptions()
	data, err := json.Marshal(horizontalPodAutoscaler)
	if err != nil {
		return nil, err
	}
	name := horizontalPodAutoscaler.Name
	if name == nil {
		return nil, fmt.Errorf("horizontalPodAutoscaler.Name must be provided to Apply")
	}
	result = &v2beta1.HorizontalPodAutoscaler{}
	err = c.client.Patch(types.ApplyPatchType).
		ApiPath("/apis").
		GroupVersion(v2beta1.SchemeGroupVersion).
		Namespace(c.ns).
		Resource("horizontalpodautoscalers").
		Name(*name).
		VersionedParams(&patchOpts).
		Body(data).
		ExpectKind("HorizontalPodAutoscaler").
		Do(ctx).
		Into(result)
	return
}

// ApplyStatus was generated because the type contains a Status member.
// Add a +genclient:noStatus comment above the type to avoid generating ApplyStatus().
func (c *horizontalPodAutoscalers) ApplyStatus(ctx context.Context, horizontalPodAutoscaler *autoscalingv2beta1.HorizontalPodAutoscalerApplyConfiguration, options v1.ApplyOptions) (result *v2beta1.HorizontalPodAutoscaler, err error) {
	if horizontalPodAutoscaler == nil {
		return nil, fmt.Errorf("horizontalPodAutoscaler provided to Apply must not be nil")
	}
	patchOpts := options.ToPatchOptions()
	data, err := json.Marshal(horizontalPodAutoscaler)
	if err != nil {
		return nil, err
	}

	name := horizontalPodAutoscaler.Name
	if name == nil {
		return nil, fmt.Errorf("horizontalPodAutoscaler.Name must be provided to Apply")
	}

	result = &v2beta1.HorizontalPodAutoscaler{}
	err = c.client.Patch(types.ApplyPatchType).
		ApiPath("/apis").
		GroupVersion(v2beta1.SchemeGroupVersion).
		Namespace(c.ns).
		Resource("horizontalpodautoscalers").
		Name(*name).
		SubResource("status").
		VersionedParams(&patchOpts).
		Body(data).
		ExpectKind("HorizontalPodAutoscaler").
		Do(ctx).
		Into(result)
	return
}
