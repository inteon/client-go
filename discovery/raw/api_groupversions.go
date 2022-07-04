package raw

import (
	"context"
	"fmt"
	"sync"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

type ApiGroupVersion struct {
	Preferred bool
	schema.GroupVersion
}

type ApiGroupVersionClient struct {
	rawDiscoveryClient RawDiscoveryClient
	rwmu               sync.RWMutex
	apiGroupVersions   []ApiGroupVersion
}

func NewApiGroupVersionClient(rawDiscoveryClient RawDiscoveryClient) *ApiGroupVersionClient {
	return &ApiGroupVersionClient{
		rawDiscoveryClient: rawDiscoveryClient,
	}
}

func createGroupVersion(group string, groupVersion metav1.GroupVersionForDiscovery) (schema.GroupVersion, error) {
	var err error
	gv := schema.GroupVersion{
		Group:   group,
		Version: groupVersion.Version,
	}

	if len(gv.Version) == 0 {
		if gv, err = schema.ParseGroupVersion(groupVersion.GroupVersion); err != nil {
			return gv, err
		}

		if len(gv.Version) > 0 && gv.Group != group {
			return gv, fmt.Errorf("group '%s' in GroupVersion property does not match expected group '%s'", gv.Group, group)
		}
	}

	return gv, nil
}

func (c *ApiGroupVersionClient) UnloadGroupVersions(groupVersion schema.GroupVersion) {
	c.rwmu.Lock()
	c.apiGroupVersions = nil
	c.rwmu.Unlock()
}

func (c *ApiGroupVersionClient) LoadGroupVersions(ctx context.Context) error {
	var apiGroupVersions []ApiGroupVersion

	// core API: /api
	{
		apiVersions, err := c.rawDiscoveryClient.ApiCoreGroupVersions(ctx)
		if err != nil {
			return err
		}

		for i, version := range apiVersions.Versions {
			apiGroupVersions = append(apiGroupVersions, ApiGroupVersion{
				Preferred: i == len(apiVersions.Versions)-1,
				GroupVersion: schema.GroupVersion{
					Version: version,
				},
			})
		}
	}

	// new API: /apis
	{
		apiGroupList, err := c.rawDiscoveryClient.ApiGroupVersions(ctx)
		if err != nil {
			return err
		}

		for _, apiGroup := range apiGroupList.Groups {
			pgv, err := createGroupVersion(apiGroup.Name, apiGroup.PreferredVersion)
			if err != nil {
				return err
			}

			foundPreferred := false
			for i, version := range apiGroup.Versions {
				gv, err := createGroupVersion(apiGroup.Name, version)
				if err != nil {
					return err
				}

				isPreferred := (gv.Version == pgv.Version) || (!foundPreferred && i == len(apiGroup.Versions)-1)
				apiGroupVersions = append(apiGroupVersions, ApiGroupVersion{
					Preferred:    isPreferred,
					GroupVersion: gv,
				})
				foundPreferred = foundPreferred || isPreferred
			}
		}
	}

	c.rwmu.Lock()
	c.apiGroupVersions = apiGroupVersions
	c.rwmu.Unlock()

	return nil
}

func (c *ApiGroupVersionClient) GetGroupVersions() ([]ApiGroupVersion, bool) {
	c.rwmu.RLock()
	defer c.rwmu.RUnlock()
	return c.apiGroupVersions, c.apiGroupVersions != nil
}
