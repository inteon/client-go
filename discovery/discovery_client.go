/*
Copyright 2015 The Kubernetes Authors.

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

package discovery

import (
	"context"
	"fmt"

	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/version"
	"k8s.io/client-go/discovery/raw"
	restclient "k8s.io/client-go/rest"
	"k8s.io/kube-openapi/pkg/util/proto"
)

// ErrGroupDiscoveryFailed is returned if an API group fails to load.
type ErrGroupDiscoveryFailed struct {
	GroupVersion schema.GroupVersion
	ApiError     error
}

// Error implements the error interface
func (e *ErrGroupDiscoveryFailed) Error() string {
	return fmt.Sprintf("unable to retrieve the server APIs for %s: %s", e.GroupVersion, e.ApiError)
}

// IsGroupDiscoveryFailedError returns true if the provided error indicates the server was unable to discover
// a complete list of APIs for the client to use.
func IsGroupDiscoveryFailedError(err error) bool {
	_, ok := err.(*ErrGroupDiscoveryFailed)
	return err != nil && ok
}

type ApiGroupVersion = raw.ApiGroupVersion

// DiscoveryInterface holds the methods that discover server-supported API groups,
// versions and resources.
type DiscoveryInterface interface {
	RESTClient() restclient.Interface

	ServerVersion(ctx context.Context) (*version.Info, error)

	UnloadGroupVersion(gv schema.GroupVersion) error

	ApiGroupVersionsFromCache() ([]ApiGroupVersion, bool)
	ApiGroupVersions(ctx context.Context) ([]ApiGroupVersion, error)

	ApiResourcesFromCache(gv schema.GroupVersion) ([]v1.APIResource, bool)
	ApiResources(ctx context.Context, gv schema.GroupVersion) ([]v1.APIResource, error)

	GvkToAPIResourceFromCache(gvk schema.GroupVersionKind) (*v1.APIResource, bool)
	GvkToAPIResource(ctx context.Context, gvk schema.GroupVersionKind) (*v1.APIResource, error)

	GvrToAPIResourceFromCache(gvk schema.GroupVersionResource) (*v1.APIResource, bool)
	GvrToAPIResource(ctx context.Context, gvk schema.GroupVersionResource) (*v1.APIResource, error)

	OpenApiSchemaFromCache(gvk schema.GroupVersionKind) (proto.Schema, bool)
	OpenApiSchema(ctx context.Context, gvk schema.GroupVersionKind) (proto.Schema, error)
}

// DiscoveryClient implements the functions that discover server-supported API groups,
// versions and resources.
type DiscoveryClient struct {
	rawDiscoveryClient raw.RawDiscoveryClient

	ApiGroupVersionClient *raw.ApiGroupVersionClient
	ApiResourceClient     *raw.ApiResourceClient

	OpenapiV2 *raw.OpenApiV2Client
	OpenapiV3 *raw.OpenApiV3Client
}

// NewDiscoveryClient returns a new DiscoveryClient for the given RESTClient.
func NewDiscoveryClient(c restclient.Interface) *DiscoveryClient {
	return NewDiscoveryClientFromRaw(raw.NewRawDiscoveryClient(c))
}

// NewDiscoveryClient returns a new DiscoveryClient for the given RawDiscoveryClient.
func NewDiscoveryClientFromRaw(rawDiscoveryClient raw.RawDiscoveryClient) *DiscoveryClient {
	return &DiscoveryClient{rawDiscoveryClient: rawDiscoveryClient}
}

// RESTClient returns a RESTClient that is used to communicate
// with API server by this client implementation.
func (d *DiscoveryClient) RESTClient() restclient.Interface {
	if d == nil || d.rawDiscoveryClient == nil {
		return nil
	}
	return d.rawDiscoveryClient.RESTClient()
}

func (dc *DiscoveryClient) ServerVersion(ctx context.Context) (*version.Info, error) {
	return dc.rawDiscoveryClient.ServerVersion(ctx)
}

func (dc *DiscoveryClient) UnloadGroupVersion(gv schema.GroupVersion) error {
	if dc.OpenapiV3 != nil {
		dc.OpenapiV3.UnloadMappings()
		dc.OpenapiV3.UnloadSchemas(gv)
	} else if dc.OpenapiV2 != nil {
		dc.OpenapiV2.UnloadSchemas()
	}

	if dc.ApiGroupVersionClient != nil {
		dc.ApiGroupVersionClient.UnloadGroupVersions(gv)
	}

	if dc.ApiResourceClient != nil {
		dc.ApiResourceClient.UnloadResources(gv)
	}

	return nil
}

func (dc *DiscoveryClient) ApiGroupVersionsFromCache() ([]ApiGroupVersion, bool) {
	if dc.ApiGroupVersionClient == nil {
		dc.ApiGroupVersionClient = raw.NewApiGroupVersionClient(dc.rawDiscoveryClient)
	}

	return dc.ApiGroupVersionClient.GetGroupVersions()
}

func (dc *DiscoveryClient) ApiGroupVersions(ctx context.Context) ([]ApiGroupVersion, error) {
	if dc.ApiGroupVersionClient == nil {
		dc.ApiGroupVersionClient = raw.NewApiGroupVersionClient(dc.rawDiscoveryClient)
	}

	if err := dc.ApiGroupVersionClient.LoadGroupVersions(ctx); err != nil {
		return nil, err
	}

	if groupVersions, ok := dc.ApiGroupVersionClient.GetGroupVersions(); !ok {
		return []ApiGroupVersion{}, nil
	} else {
		return groupVersions, nil
	}
}

// APIResourcesFromCache checks if our caches contain a resolution for the gvk
func (dc *DiscoveryClient) ApiResourcesFromCache(gv schema.GroupVersion) ([]v1.APIResource, bool) {
	if dc.ApiResourceClient == nil {
		dc.ApiResourceClient = raw.NewApiResourceClient(dc.rawDiscoveryClient)
	}

	return dc.ApiResourceClient.GetGroupVersion(gv)
}

// APIResources fetches the result from the kube API and saves the result to cache
func (dc *DiscoveryClient) ApiResources(ctx context.Context, gv schema.GroupVersion) ([]v1.APIResource, error) {
	if dc.ApiResourceClient == nil {
		dc.ApiResourceClient = raw.NewApiResourceClient(dc.rawDiscoveryClient)
	}

	if err := dc.ApiResourceClient.LoadResources(ctx, gv); err != nil {
		return nil, &ErrGroupDiscoveryFailed{GroupVersion: gv, ApiError: err}
	}

	if info, ok := dc.ApiResourceClient.GetGroupVersion(gv); ok {
		return info, nil
	}

	return nil, &ErrGroupDiscoveryFailed{GroupVersion: gv, ApiError: fmt.Errorf("GroupVersionKind not found")}
}

// GvkToAPIResourceFromCache checks if our caches contain a resolution for the gvk
func (dc *DiscoveryClient) GvkToAPIResourceFromCache(gvk schema.GroupVersionKind) (*v1.APIResource, bool) {
	if dc.ApiResourceClient == nil {
		dc.ApiResourceClient = raw.NewApiResourceClient(dc.rawDiscoveryClient)
	}

	return dc.ApiResourceClient.GetGroupVersionKind(gvk)
}

// GvkToAPIResource fetches the result from the kube API and saves the result to cache
func (dc *DiscoveryClient) GvkToAPIResource(ctx context.Context, gvk schema.GroupVersionKind) (*v1.APIResource, error) {
	if dc.ApiResourceClient == nil {
		dc.ApiResourceClient = raw.NewApiResourceClient(dc.rawDiscoveryClient)
	}

	if err := dc.ApiResourceClient.LoadResources(ctx, gvk.GroupVersion()); err != nil {
		return nil, &ErrGroupDiscoveryFailed{GroupVersion: gvk.GroupVersion(), ApiError: err}
	}

	if info, ok := dc.ApiResourceClient.GetGroupVersionKind(gvk); ok {
		return info, nil
	}

	return nil, &ErrGroupDiscoveryFailed{GroupVersion: gvk.GroupVersion(), ApiError: fmt.Errorf("GroupVersionKind not found")}
}

// GvkToAPIResourceFromCache checks if our caches contain a resolution for the gvr
func (dc *DiscoveryClient) GvrToAPIResourceFromCache(gvr schema.GroupVersionResource) (*v1.APIResource, bool) {
	if dc.ApiResourceClient == nil {
		dc.ApiResourceClient = raw.NewApiResourceClient(dc.rawDiscoveryClient)
	}

	return dc.ApiResourceClient.GetGroupVersionResource(gvr)
}

// GvkToAPIResource fetches the result from the kube API and saves the result to cache
func (dc *DiscoveryClient) GvrToAPIResource(ctx context.Context, gvr schema.GroupVersionResource) (*v1.APIResource, error) {
	if dc.ApiResourceClient == nil {
		dc.ApiResourceClient = raw.NewApiResourceClient(dc.rawDiscoveryClient)
	}

	if err := dc.ApiResourceClient.LoadResources(ctx, gvr.GroupVersion()); err != nil {
		return nil, &ErrGroupDiscoveryFailed{GroupVersion: gvr.GroupVersion(), ApiError: err}
	}

	if info, ok := dc.ApiResourceClient.GetGroupVersionResource(gvr); ok {
		return info, nil
	}

	return nil, &ErrGroupDiscoveryFailed{GroupVersion: gvr.GroupVersion(), ApiError: fmt.Errorf("GroupVersionKind not found")}
}

func (dc *DiscoveryClient) OpenApiSchemaFromCache(gvk schema.GroupVersionKind) (proto.Schema, bool) {
	if dc.OpenapiV3 != nil {
		return dc.OpenapiV3.GetSchema(gvk)
	} else if dc.OpenapiV2 != nil {
		return dc.OpenapiV2.GetSchema(gvk)
	}

	return nil, false
}

func (dc *DiscoveryClient) OpenApiSchema(ctx context.Context, gvk schema.GroupVersionKind) (proto.Schema, error) {
	if dc.OpenapiV3 != nil {
		if _, ok := dc.OpenapiV3.GetMapping(gvk.GroupVersion()); !ok {
			if err := dc.OpenapiV3.LoadMappings(ctx); err != nil {
				return nil, err
			}
		}

		if err := dc.OpenapiV3.LoadSchemas(ctx, gvk.GroupVersion()); err != nil {
			return nil, err
		}

		if schema, ok := dc.OpenapiV3.GetSchema(gvk); !ok {
			return nil, fmt.Errorf("no schema for %s found", gvk)
		} else {
			return schema, nil
		}
	} else if dc.OpenapiV2 != nil {
		if err := dc.OpenapiV2.LoadSchemas(ctx); err != nil {
			return nil, err
		}

		if schema, ok := dc.OpenapiV2.GetSchema(gvk); !ok {
			return nil, fmt.Errorf("no schema for %s found", gvk)
		} else {
			return schema, nil
		}
	} else {
		dc.OpenapiV3 = raw.NewOpenApiV3Client(dc.rawDiscoveryClient)
		if err := dc.OpenapiV3.LoadMappings(ctx); err != nil {
			// use v2 instead of v3
			dc.OpenapiV3 = nil
			dc.OpenapiV2 = raw.NewOpenApiV2Client(dc.rawDiscoveryClient)
		}

		return dc.OpenApiSchema(ctx, gvk)
	}
}
