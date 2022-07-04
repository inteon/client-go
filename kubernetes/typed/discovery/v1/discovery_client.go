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

package v1

import (
	rest "k8s.io/client-go/rest"
)

type DiscoveryV1Interface interface {
	RESTClient() rest.Interface
	EndpointSlicesGetter
}

// DiscoveryV1Client is used to interact with features provided by the discovery.k8s.io group.
type DiscoveryV1Client struct {
	restClient rest.Interface
}

func (c *DiscoveryV1Client) EndpointSlices(namespace string) EndpointSliceInterface {
	return newEndpointSlices(c, namespace)
}

// New creates a new DiscoveryV1Client for the given RESTClient.
func New(c rest.Interface) *DiscoveryV1Client {
	return &DiscoveryV1Client{c}
}

// RESTClient returns a RESTClient that is used to communicate
// with API server by this client implementation.
func (c *DiscoveryV1Client) RESTClient() rest.Interface {
	if c == nil {
		return nil
	}
	return c.restClient
}
