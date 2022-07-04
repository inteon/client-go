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

type AdmissionregistrationV1Interface interface {
	RESTClient() rest.Interface
	MutatingWebhookConfigurationsGetter
	ValidatingWebhookConfigurationsGetter
}

// AdmissionregistrationV1Client is used to interact with features provided by the admissionregistration.k8s.io group.
type AdmissionregistrationV1Client struct {
	restClient rest.Interface
}

func (c *AdmissionregistrationV1Client) MutatingWebhookConfigurations() MutatingWebhookConfigurationInterface {
	return newMutatingWebhookConfigurations(c)
}

func (c *AdmissionregistrationV1Client) ValidatingWebhookConfigurations() ValidatingWebhookConfigurationInterface {
	return newValidatingWebhookConfigurations(c)
}

// New creates a new AdmissionregistrationV1Client for the given RESTClient.
func New(c rest.Interface) *AdmissionregistrationV1Client {
	return &AdmissionregistrationV1Client{c}
}

// RESTClient returns a RESTClient that is used to communicate
// with API server by this client implementation.
func (c *AdmissionregistrationV1Client) RESTClient() rest.Interface {
	if c == nil {
		return nil
	}
	return c.restClient
}
