package raw

import (
	"context"
	"fmt"
	"sync"

	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/kube-openapi/pkg/util/proto"
)

type OpenApiV2Client struct {
	rawDiscoveryClient RawDiscoveryClient
	murw               sync.RWMutex
	schemas            map[schema.GroupVersionKind]proto.Schema
}

func NewOpenApiV2Client(rawDiscoveryClient RawDiscoveryClient) *OpenApiV2Client {
	return &OpenApiV2Client{
		rawDiscoveryClient: rawDiscoveryClient,
	}
}

func (c *OpenApiV2Client) UnloadSchemas() {
	c.murw.Lock()
	c.schemas = nil
	c.murw.Unlock()
}

func (c *OpenApiV2Client) LoadSchemas(ctx context.Context) error {
	doc, err := c.rawDiscoveryClient.OpenAPISchema(ctx)
	if err != nil {
		return err
	}

	models, err := proto.NewOpenAPIData(doc)
	if err != nil {
		return err
	}

	schemas := map[schema.GroupVersionKind]proto.Schema{}
	for _, modelName := range models.ListModels() {
		model := models.LookupModel(modelName)
		if model == nil {
			return fmt.Errorf("ListModels returns a model that can't be looked-up")
		}

		gvkList := parseGroupVersionKindExtension(model)
		for _, gvk := range gvkList {
			schemas[gvk] = model
		}
	}

	c.murw.Lock()
	c.schemas = schemas
	c.murw.Unlock()

	return nil
}

func (c *OpenApiV2Client) GetSchema(gvk schema.GroupVersionKind) (proto.Schema, bool) {
	c.murw.RLock()
	defer c.murw.RUnlock()
	if c.schemas == nil {
		return nil, false
	}
	schema, ok := c.schemas[gvk]
	return schema, ok
}
