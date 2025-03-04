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

	batchv1 "k8s.io/api/batch/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	labels "k8s.io/apimachinery/pkg/labels"
	schema "k8s.io/apimachinery/pkg/runtime/schema"
	types "k8s.io/apimachinery/pkg/types"
	applyconfigurationsbatchv1 "k8s.io/client-go/applyconfigurations/batch/v1"
	watch "k8s.io/client-go/pkg/watch"
	testing "k8s.io/client-go/testing"
)

// FakeJobs implements JobInterface
type FakeJobs struct {
	Fake *FakeBatchV1
	ns   string
}

var jobsResource = schema.GroupVersionResource{Group: "batch", Version: "v1", Resource: "jobs"}

var jobsKind = schema.GroupVersionKind{Group: "batch", Version: "v1", Kind: "Job"}

// Get takes name of the job, and returns the corresponding job object, and an error if there is any.
func (c *FakeJobs) Get(ctx context.Context, name string, options v1.GetOptions) (result *batchv1.Job, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewGetAction(jobsResource, c.ns, name), &batchv1.Job{})

	if obj == nil {
		return nil, err
	}
	return obj.(*batchv1.Job), err
}

// List takes label and field selectors, and returns the list of Jobs that match those selectors.
func (c *FakeJobs) List(ctx context.Context, opts v1.ListOptions) (result *batchv1.JobList, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewListAction(jobsResource, jobsKind, c.ns, opts), &batchv1.JobList{})

	if obj == nil {
		return nil, err
	}

	label, _, _ := testing.ExtractFromListOptions(opts)
	if label == nil {
		label = labels.Everything()
	}
	list := &batchv1.JobList{
		TypeMeta: obj.(*batchv1.JobList).TypeMeta,
		ListMeta: obj.(*batchv1.JobList).ListMeta,
	}
	for _, item := range obj.(*batchv1.JobList).Items {
		if label.Matches(labels.Set(item.Labels)) {
			list.Items = append(list.Items, item)
		}
	}
	return list, err
}

// Watch returns a watch.Watcher that watches the requested jobs.
func (c *FakeJobs) Watch(ctx context.Context, opts v1.ListOptions) (watch.Watcher, error) {
	return c.Fake.
		InvokesWatch(testing.NewWatchAction(jobsResource, c.ns, opts))

}

// Create takes the representation of a job and creates it.  Returns the server's representation of the job, and an error, if there is any.
func (c *FakeJobs) Create(ctx context.Context, job *batchv1.Job, opts v1.CreateOptions) (result *batchv1.Job, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewCreateAction(jobsResource, c.ns, job), &batchv1.Job{})

	if obj == nil {
		return nil, err
	}
	return obj.(*batchv1.Job), err
}

// Update takes the representation of a job and updates it. Returns the server's representation of the job, and an error, if there is any.
func (c *FakeJobs) Update(ctx context.Context, job *batchv1.Job, opts v1.UpdateOptions) (result *batchv1.Job, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewUpdateAction(jobsResource, c.ns, job), &batchv1.Job{})

	if obj == nil {
		return nil, err
	}
	return obj.(*batchv1.Job), err
}

// UpdateStatus was generated because the type contains a Status member.
// Add a +genclient:noStatus comment above the type to avoid generating UpdateStatus().
func (c *FakeJobs) UpdateStatus(ctx context.Context, job *batchv1.Job, opts v1.UpdateOptions) (*batchv1.Job, error) {
	obj, err := c.Fake.
		Invokes(testing.NewUpdateSubresourceAction(jobsResource, "status", c.ns, job), &batchv1.Job{})

	if obj == nil {
		return nil, err
	}
	return obj.(*batchv1.Job), err
}

// Delete takes name of the job and deletes it. Returns an error if one occurs.
func (c *FakeJobs) Delete(ctx context.Context, name string, opts v1.DeleteOptions) error {
	_, err := c.Fake.
		Invokes(testing.NewDeleteActionWithOptions(jobsResource, c.ns, name, opts), &batchv1.Job{})

	return err
}

// DeleteCollection deletes a collection of objects.
func (c *FakeJobs) DeleteCollection(ctx context.Context, opts v1.DeleteOptions, listOpts v1.ListOptions) error {
	action := testing.NewDeleteCollectionAction(jobsResource, c.ns, listOpts)

	_, err := c.Fake.Invokes(action, &batchv1.JobList{})
	return err
}

// Patch applies the patch and returns the patched job.
func (c *FakeJobs) Patch(ctx context.Context, name string, pt types.PatchType, data []byte, opts v1.PatchOptions, subresources ...string) (result *batchv1.Job, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewPatchSubresourceAction(jobsResource, c.ns, name, pt, data, subresources...), &batchv1.Job{})

	if obj == nil {
		return nil, err
	}
	return obj.(*batchv1.Job), err
}

// Apply takes the given apply declarative configuration, applies it and returns the applied job.
func (c *FakeJobs) Apply(ctx context.Context, job *applyconfigurationsbatchv1.JobApplyConfiguration, opts v1.ApplyOptions) (result *batchv1.Job, err error) {
	if job == nil {
		return nil, fmt.Errorf("job provided to Apply must not be nil")
	}
	data, err := json.Marshal(job)
	if err != nil {
		return nil, err
	}
	name := job.Name
	if name == nil {
		return nil, fmt.Errorf("job.Name must be provided to Apply")
	}
	obj, err := c.Fake.
		Invokes(testing.NewPatchSubresourceAction(jobsResource, c.ns, *name, types.ApplyPatchType, data), &batchv1.Job{})

	if obj == nil {
		return nil, err
	}
	return obj.(*batchv1.Job), err
}

// ApplyStatus was generated because the type contains a Status member.
// Add a +genclient:noStatus comment above the type to avoid generating ApplyStatus().
func (c *FakeJobs) ApplyStatus(ctx context.Context, job *applyconfigurationsbatchv1.JobApplyConfiguration, opts v1.ApplyOptions) (result *batchv1.Job, err error) {
	if job == nil {
		return nil, fmt.Errorf("job provided to Apply must not be nil")
	}
	data, err := json.Marshal(job)
	if err != nil {
		return nil, err
	}
	name := job.Name
	if name == nil {
		return nil, fmt.Errorf("job.Name must be provided to Apply")
	}
	obj, err := c.Fake.
		Invokes(testing.NewPatchSubresourceAction(jobsResource, c.ns, *name, types.ApplyPatchType, data, "status"), &batchv1.Job{})

	if obj == nil {
		return nil, err
	}
	return obj.(*batchv1.Job), err
}
