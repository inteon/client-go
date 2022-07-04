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

package v1beta1

import (
	rest "k8s.io/client-go/rest"
)

type RbacV1beta1Interface interface {
	RESTClient() rest.Interface
	ClusterRolesGetter
	ClusterRoleBindingsGetter
	RolesGetter
	RoleBindingsGetter
}

// RbacV1beta1Client is used to interact with features provided by the rbac.authorization.k8s.io group.
type RbacV1beta1Client struct {
	restClient rest.Interface
}

func (c *RbacV1beta1Client) ClusterRoles() ClusterRoleInterface {
	return newClusterRoles(c)
}

func (c *RbacV1beta1Client) ClusterRoleBindings() ClusterRoleBindingInterface {
	return newClusterRoleBindings(c)
}

func (c *RbacV1beta1Client) Roles(namespace string) RoleInterface {
	return newRoles(c, namespace)
}

func (c *RbacV1beta1Client) RoleBindings(namespace string) RoleBindingInterface {
	return newRoleBindings(c, namespace)
}

// New creates a new RbacV1beta1Client for the given RESTClient.
func New(c rest.Interface) *RbacV1beta1Client {
	return &RbacV1beta1Client{c}
}

// RESTClient returns a RESTClient that is used to communicate
// with API server by this client implementation.
func (c *RbacV1beta1Client) RESTClient() rest.Interface {
	if c == nil {
		return nil
	}
	return c.restClient
}
