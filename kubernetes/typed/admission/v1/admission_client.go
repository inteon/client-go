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

type AdmissionV1Interface interface {
	RESTClient() rest.Interface
}

// AdmissionV1Client is used to interact with features provided by the admission.k8s.io group.
type AdmissionV1Client struct {
	restClient rest.Interface
}

// New creates a new AdmissionV1Client for the given RESTClient.
func New(c rest.Interface) *AdmissionV1Client {
	return &AdmissionV1Client{c}
}

// RESTClient returns a RESTClient that is used to communicate
// with API server by this client implementation.
func (c *AdmissionV1Client) RESTClient() rest.Interface {
	if c == nil {
		return nil
	}
	return c.restClient
}
