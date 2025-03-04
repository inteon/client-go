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

	eventsv1 "k8s.io/api/events/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	labels "k8s.io/apimachinery/pkg/labels"
	schema "k8s.io/apimachinery/pkg/runtime/schema"
	types "k8s.io/apimachinery/pkg/types"
	applyconfigurationseventsv1 "k8s.io/client-go/applyconfigurations/events/v1"
	watch "k8s.io/client-go/pkg/watch"
	testing "k8s.io/client-go/testing"
)

// FakeEvents implements EventInterface
type FakeEvents struct {
	Fake *FakeEventsV1
	ns   string
}

var eventsResource = schema.GroupVersionResource{Group: "events.k8s.io", Version: "v1", Resource: "events"}

var eventsKind = schema.GroupVersionKind{Group: "events.k8s.io", Version: "v1", Kind: "Event"}

// Get takes name of the event, and returns the corresponding event object, and an error if there is any.
func (c *FakeEvents) Get(ctx context.Context, name string, options v1.GetOptions) (result *eventsv1.Event, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewGetAction(eventsResource, c.ns, name), &eventsv1.Event{})

	if obj == nil {
		return nil, err
	}
	return obj.(*eventsv1.Event), err
}

// List takes label and field selectors, and returns the list of Events that match those selectors.
func (c *FakeEvents) List(ctx context.Context, opts v1.ListOptions) (result *eventsv1.EventList, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewListAction(eventsResource, eventsKind, c.ns, opts), &eventsv1.EventList{})

	if obj == nil {
		return nil, err
	}

	label, _, _ := testing.ExtractFromListOptions(opts)
	if label == nil {
		label = labels.Everything()
	}
	list := &eventsv1.EventList{
		TypeMeta: obj.(*eventsv1.EventList).TypeMeta,
		ListMeta: obj.(*eventsv1.EventList).ListMeta,
	}
	for _, item := range obj.(*eventsv1.EventList).Items {
		if label.Matches(labels.Set(item.Labels)) {
			list.Items = append(list.Items, item)
		}
	}
	return list, err
}

// Watch returns a watch.Watcher that watches the requested events.
func (c *FakeEvents) Watch(ctx context.Context, opts v1.ListOptions) (watch.Watcher, error) {
	return c.Fake.
		InvokesWatch(testing.NewWatchAction(eventsResource, c.ns, opts))

}

// Create takes the representation of a event and creates it.  Returns the server's representation of the event, and an error, if there is any.
func (c *FakeEvents) Create(ctx context.Context, event *eventsv1.Event, opts v1.CreateOptions) (result *eventsv1.Event, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewCreateAction(eventsResource, c.ns, event), &eventsv1.Event{})

	if obj == nil {
		return nil, err
	}
	return obj.(*eventsv1.Event), err
}

// Update takes the representation of a event and updates it. Returns the server's representation of the event, and an error, if there is any.
func (c *FakeEvents) Update(ctx context.Context, event *eventsv1.Event, opts v1.UpdateOptions) (result *eventsv1.Event, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewUpdateAction(eventsResource, c.ns, event), &eventsv1.Event{})

	if obj == nil {
		return nil, err
	}
	return obj.(*eventsv1.Event), err
}

// Delete takes name of the event and deletes it. Returns an error if one occurs.
func (c *FakeEvents) Delete(ctx context.Context, name string, opts v1.DeleteOptions) error {
	_, err := c.Fake.
		Invokes(testing.NewDeleteActionWithOptions(eventsResource, c.ns, name, opts), &eventsv1.Event{})

	return err
}

// DeleteCollection deletes a collection of objects.
func (c *FakeEvents) DeleteCollection(ctx context.Context, opts v1.DeleteOptions, listOpts v1.ListOptions) error {
	action := testing.NewDeleteCollectionAction(eventsResource, c.ns, listOpts)

	_, err := c.Fake.Invokes(action, &eventsv1.EventList{})
	return err
}

// Patch applies the patch and returns the patched event.
func (c *FakeEvents) Patch(ctx context.Context, name string, pt types.PatchType, data []byte, opts v1.PatchOptions, subresources ...string) (result *eventsv1.Event, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewPatchSubresourceAction(eventsResource, c.ns, name, pt, data, subresources...), &eventsv1.Event{})

	if obj == nil {
		return nil, err
	}
	return obj.(*eventsv1.Event), err
}

// Apply takes the given apply declarative configuration, applies it and returns the applied event.
func (c *FakeEvents) Apply(ctx context.Context, event *applyconfigurationseventsv1.EventApplyConfiguration, opts v1.ApplyOptions) (result *eventsv1.Event, err error) {
	if event == nil {
		return nil, fmt.Errorf("event provided to Apply must not be nil")
	}
	data, err := json.Marshal(event)
	if err != nil {
		return nil, err
	}
	name := event.Name
	if name == nil {
		return nil, fmt.Errorf("event.Name must be provided to Apply")
	}
	obj, err := c.Fake.
		Invokes(testing.NewPatchSubresourceAction(eventsResource, c.ns, *name, types.ApplyPatchType, data), &eventsv1.Event{})

	if obj == nil {
		return nil, err
	}
	return obj.(*eventsv1.Event), err
}
