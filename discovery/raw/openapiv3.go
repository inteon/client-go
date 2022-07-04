package raw

import (
	"context"
	"fmt"
	"sync"

	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/kube-openapi/pkg/util/proto"
)

type OpenApiV3Client struct {
	rawDiscoveryClient RawDiscoveryClient

	murw      sync.RWMutex
	endpoints map[schema.GroupVersion]string

	schemas sync.Map // map[schema.GroupVersion]map[schema.GroupVersionKind]proto.Schema
}

func NewOpenApiV3Client(rawDiscoveryClient RawDiscoveryClient) *OpenApiV3Client {
	return &OpenApiV3Client{
		rawDiscoveryClient: rawDiscoveryClient,
		endpoints:          map[schema.GroupVersion]string{},
	}
}

func (c *OpenApiV3Client) UnloadMappings() {
	c.murw.Lock()
	c.endpoints = nil
	c.murw.Unlock()
}

func (c *OpenApiV3Client) LoadMappings(ctx context.Context) error {
	discoMap, err := c.rawDiscoveryClient.OpenAPIV3Discovery(ctx)
	if err != nil {
		return err
	}

	// Create GroupVersions for each element of the result
	endpoints := map[schema.GroupVersion]string{}
	for gv, v := range discoMap.Paths {
		endpoints[parseGroupVersionEndpoint(gv)] = v.ServerRelativeURL
	}

	c.murw.Lock()
	c.endpoints = endpoints
	c.murw.Unlock()

	return nil
}

func (c *OpenApiV3Client) GetMapping(groupVersion schema.GroupVersion) (string, bool) {
	c.murw.RLock()
	defer c.murw.RUnlock()
	if c.endpoints == nil {
		return "", false
	}
	schema, ok := c.endpoints[groupVersion]
	return schema, ok
}

func (c *OpenApiV3Client) UnloadSchemas(groupVersion schema.GroupVersion) {
	_, _ = c.schemas.LoadAndDelete(groupVersion)
}

func (c *OpenApiV3Client) LoadSchemas(ctx context.Context, groupVersion schema.GroupVersion) error {
	endpoint, ok := c.GetMapping(groupVersion)
	if !ok {
		return fmt.Errorf("groupVersion not found")
	}

	doc, err := c.rawDiscoveryClient.OpenAPIV3Schema(ctx, endpoint)
	if err != nil {
		return err
	}

	models, err := proto.NewOpenAPIV3Data(doc)
	if err != nil {
		return err
	}

	schema := map[schema.GroupVersionKind]proto.Schema{}
	for _, modelName := range models.ListModels() {
		model := models.LookupModel(modelName)
		if model == nil {
			return fmt.Errorf("ListModels returns a model that can't be looked-up")
		}

		gvkList := parseGroupVersionKindExtension(model)
		for _, gvk := range gvkList {
			schema[gvk] = model
		}
	}

	c.schemas.Store(groupVersion, schema)

	return nil
}

func (c *OpenApiV3Client) GetSchema(gvk schema.GroupVersionKind) (proto.Schema, bool) {
	if schemas, ok := c.schemas.Load(gvk.GroupVersion()); !ok {
		return nil, false
	} else if schema, ok := schemas.(map[schema.GroupVersionKind]proto.Schema)[gvk]; !ok {
		return nil, false
	} else {
		return schema, true
	}
}
