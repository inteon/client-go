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

package v1alpha1

import (
	"context"
	json "encoding/json"
	"fmt"
	"time"

	v1alpha1 "k8s.io/api/storage/v1alpha1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	types "k8s.io/apimachinery/pkg/types"
	storagev1alpha1 "k8s.io/client-go/applyconfigurations/storage/v1alpha1"
	watch "k8s.io/client-go/pkg/watch"
	rest "k8s.io/client-go/rest"
)

// VolumeAttachmentsGetter has a method to return a VolumeAttachmentInterface.
// A group's client should implement this interface.
type VolumeAttachmentsGetter interface {
	VolumeAttachments() VolumeAttachmentInterface
}

// VolumeAttachmentInterface has methods to work with VolumeAttachment resources.
type VolumeAttachmentInterface interface {
	Create(ctx context.Context, volumeAttachment *v1alpha1.VolumeAttachment, options v1.CreateOptions) (*v1alpha1.VolumeAttachment, error)
	Update(ctx context.Context, volumeAttachment *v1alpha1.VolumeAttachment, options v1.UpdateOptions) (*v1alpha1.VolumeAttachment, error)
	UpdateStatus(ctx context.Context, volumeAttachment *v1alpha1.VolumeAttachment, options v1.UpdateOptions) (*v1alpha1.VolumeAttachment, error)
	Delete(ctx context.Context, name string, options v1.DeleteOptions) error
	DeleteCollection(ctx context.Context, options v1.DeleteOptions, listOptions v1.ListOptions) error
	Get(ctx context.Context, name string, options v1.GetOptions) (*v1alpha1.VolumeAttachment, error)
	List(ctx context.Context, options v1.ListOptions) (*v1alpha1.VolumeAttachmentList, error)
	Watch(ctx context.Context, options v1.ListOptions) (watch.Watcher, error)
	Patch(ctx context.Context, name string, pt types.PatchType, data []byte, options v1.PatchOptions, subresources ...string) (result *v1alpha1.VolumeAttachment, err error)
	Apply(ctx context.Context, volumeAttachment *storagev1alpha1.VolumeAttachmentApplyConfiguration, options v1.ApplyOptions) (result *v1alpha1.VolumeAttachment, err error)
	ApplyStatus(ctx context.Context, volumeAttachment *storagev1alpha1.VolumeAttachmentApplyConfiguration, options v1.ApplyOptions) (result *v1alpha1.VolumeAttachment, err error)
	VolumeAttachmentExpansion
}

// volumeAttachments implements VolumeAttachmentInterface
type volumeAttachments struct {
	client rest.Interface
}

// newVolumeAttachments returns a VolumeAttachments
func newVolumeAttachments(c *StorageV1alpha1Client) *volumeAttachments {
	return &volumeAttachments{
		client: c.RESTClient(),
	}
}

// Get takes name of the volumeAttachment, and returns the corresponding volumeAttachment object, and an error if there is any.
func (c *volumeAttachments) Get(ctx context.Context, name string, options v1.GetOptions) (result *v1alpha1.VolumeAttachment, err error) {
	result = &v1alpha1.VolumeAttachment{}
	err = c.client.Get().
		ApiPath("/apis").
		GroupVersion(v1alpha1.SchemeGroupVersion).
		Resource("volumeattachments").
		Name(name).
		VersionedParams(&options).
		ExpectKind("VolumeAttachment").
		Do(ctx).
		Into(result)
	return
}

// List takes label and field selectors, and returns the list of VolumeAttachments that match those selectors.
func (c *volumeAttachments) List(ctx context.Context, options v1.ListOptions) (result *v1alpha1.VolumeAttachmentList, err error) {
	var timeout time.Duration
	if options.TimeoutSeconds != nil {
		timeout = time.Duration(*options.TimeoutSeconds) * time.Second
	}
	result = &v1alpha1.VolumeAttachmentList{}
	err = c.client.Get().
		ApiPath("/apis").
		GroupVersion(v1alpha1.SchemeGroupVersion).
		Resource("volumeattachments").
		VersionedParams(&options).
		Timeout(timeout).
		ExpectKind("VolumeAttachment").
		Do(ctx).
		Into(result)
	return
}

// Watch returns a watch.Watcher that watches the requested volumeAttachments.
func (c *volumeAttachments) Watch(ctx context.Context, options v1.ListOptions) (watch.Watcher, error) {
	var timeout time.Duration
	if options.TimeoutSeconds != nil {
		timeout = time.Duration(*options.TimeoutSeconds) * time.Second
	}
	options.Watch = true
	return c.client.Get().
		ApiPath("/apis").
		GroupVersion(v1alpha1.SchemeGroupVersion).
		Resource("volumeattachments").
		VersionedParams(&options).
		Timeout(timeout).
		ExpectKind("VolumeAttachment").
		Watch(ctx)
}

// Create takes the representation of a volumeAttachment and creates it.  Returns the server's representation of the volumeAttachment, and an error, if there is any.
func (c *volumeAttachments) Create(ctx context.Context, volumeAttachment *v1alpha1.VolumeAttachment, options v1.CreateOptions) (result *v1alpha1.VolumeAttachment, err error) {
	result = &v1alpha1.VolumeAttachment{}
	err = c.client.Post().
		ApiPath("/apis").
		GroupVersion(v1alpha1.SchemeGroupVersion).
		Resource("volumeattachments").
		VersionedParams(&options).
		Body(volumeAttachment).
		ExpectKind("VolumeAttachment").
		Do(ctx).
		Into(result)
	return
}

// Update takes the representation of a volumeAttachment and updates it. Returns the server's representation of the volumeAttachment, and an error, if there is any.
func (c *volumeAttachments) Update(ctx context.Context, volumeAttachment *v1alpha1.VolumeAttachment, options v1.UpdateOptions) (result *v1alpha1.VolumeAttachment, err error) {
	result = &v1alpha1.VolumeAttachment{}
	err = c.client.Put().
		ApiPath("/apis").
		GroupVersion(v1alpha1.SchemeGroupVersion).
		Resource("volumeattachments").
		Name(volumeAttachment.Name).
		VersionedParams(&options).
		Body(volumeAttachment).
		ExpectKind("VolumeAttachment").
		Do(ctx).
		Into(result)
	return
}

// UpdateStatus was generated because the type contains a Status member.
// Add a +genclient:noStatus comment above the type to avoid generating UpdateStatus().
func (c *volumeAttachments) UpdateStatus(ctx context.Context, volumeAttachment *v1alpha1.VolumeAttachment, options v1.UpdateOptions) (result *v1alpha1.VolumeAttachment, err error) {
	result = &v1alpha1.VolumeAttachment{}
	err = c.client.Put().
		ApiPath("/apis").
		GroupVersion(v1alpha1.SchemeGroupVersion).
		Resource("volumeattachments").
		Name(volumeAttachment.Name).
		SubResource("status").
		VersionedParams(&options).
		Body(volumeAttachment).
		ExpectKind("VolumeAttachment").
		Do(ctx).
		Into(result)
	return
}

// Delete takes name of the volumeAttachment and deletes it. Returns an error if one occurs.
func (c *volumeAttachments) Delete(ctx context.Context, name string, options v1.DeleteOptions) error {
	return c.client.Delete().
		ApiPath("/apis").
		GroupVersion(v1alpha1.SchemeGroupVersion).
		Resource("volumeattachments").
		Name(name).
		Body(&options).
		ExpectKind("VolumeAttachment").
		Do(ctx).
		Error()
}

// DeleteCollection deletes a collection of objects.
func (c *volumeAttachments) DeleteCollection(ctx context.Context, options v1.DeleteOptions, listOptions v1.ListOptions) error {
	var timeout time.Duration
	if listOptions.TimeoutSeconds != nil {
		timeout = time.Duration(*listOptions.TimeoutSeconds) * time.Second
	}
	return c.client.Delete().
		ApiPath("/apis").
		GroupVersion(v1alpha1.SchemeGroupVersion).
		Resource("volumeattachments").
		VersionedParams(&listOptions).
		Timeout(timeout).
		Body(&options).
		ExpectKind("VolumeAttachment").
		Do(ctx).
		Error()
}

// Patch applies the patch and returns the patched volumeAttachment.
func (c *volumeAttachments) Patch(ctx context.Context, name string, pt types.PatchType, data []byte, options v1.PatchOptions, subresources ...string) (result *v1alpha1.VolumeAttachment, err error) {
	result = &v1alpha1.VolumeAttachment{}
	err = c.client.Patch(pt).
		ApiPath("/apis").
		GroupVersion(v1alpha1.SchemeGroupVersion).
		Resource("volumeattachments").
		Name(name).
		SubResource(subresources...).
		VersionedParams(&options).
		Body(data).
		ExpectKind("VolumeAttachment").
		Do(ctx).
		Into(result)
	return
}

// Apply takes the given apply declarative configuration, applies it and returns the applied volumeAttachment.
func (c *volumeAttachments) Apply(ctx context.Context, volumeAttachment *storagev1alpha1.VolumeAttachmentApplyConfiguration, options v1.ApplyOptions) (result *v1alpha1.VolumeAttachment, err error) {
	if volumeAttachment == nil {
		return nil, fmt.Errorf("volumeAttachment provided to Apply must not be nil")
	}
	patchOpts := options.ToPatchOptions()
	data, err := json.Marshal(volumeAttachment)
	if err != nil {
		return nil, err
	}
	name := volumeAttachment.Name
	if name == nil {
		return nil, fmt.Errorf("volumeAttachment.Name must be provided to Apply")
	}
	result = &v1alpha1.VolumeAttachment{}
	err = c.client.Patch(types.ApplyPatchType).
		ApiPath("/apis").
		GroupVersion(v1alpha1.SchemeGroupVersion).
		Resource("volumeattachments").
		Name(*name).
		VersionedParams(&patchOpts).
		Body(data).
		ExpectKind("VolumeAttachment").
		Do(ctx).
		Into(result)
	return
}

// ApplyStatus was generated because the type contains a Status member.
// Add a +genclient:noStatus comment above the type to avoid generating ApplyStatus().
func (c *volumeAttachments) ApplyStatus(ctx context.Context, volumeAttachment *storagev1alpha1.VolumeAttachmentApplyConfiguration, options v1.ApplyOptions) (result *v1alpha1.VolumeAttachment, err error) {
	if volumeAttachment == nil {
		return nil, fmt.Errorf("volumeAttachment provided to Apply must not be nil")
	}
	patchOpts := options.ToPatchOptions()
	data, err := json.Marshal(volumeAttachment)
	if err != nil {
		return nil, err
	}

	name := volumeAttachment.Name
	if name == nil {
		return nil, fmt.Errorf("volumeAttachment.Name must be provided to Apply")
	}

	result = &v1alpha1.VolumeAttachment{}
	err = c.client.Patch(types.ApplyPatchType).
		ApiPath("/apis").
		GroupVersion(v1alpha1.SchemeGroupVersion).
		Resource("volumeattachments").
		Name(*name).
		SubResource("status").
		VersionedParams(&patchOpts).
		Body(data).
		ExpectKind("VolumeAttachment").
		Do(ctx).
		Into(result)
	return
}
