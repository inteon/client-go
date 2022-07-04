package raw

import (
	"context"
	"strings"
	"sync"

	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

type apiResourceSet struct {
	apiResources     []v1.APIResource
	gvkToAPIResource map[schema.GroupVersionKind]*v1.APIResource
	gvrToAPIResource map[schema.GroupVersionResource]*v1.APIResource
}

type ApiResourceClient struct {
	rawDiscoveryClient RawDiscoveryClient
	infos              sync.Map // map[schema.GroupVersion]apiResourceSet
}

func NewApiResourceClient(rawDiscoveryClient RawDiscoveryClient) *ApiResourceClient {
	return &ApiResourceClient{
		rawDiscoveryClient: rawDiscoveryClient,
	}
}

func (c *ApiResourceClient) UnloadResources(groupVersion schema.GroupVersion) {
	_, _ = c.infos.LoadAndDelete(groupVersion)
}

func (c *ApiResourceClient) LoadResources(ctx context.Context, groupVersion schema.GroupVersion) error {
	resourceList, err := c.rawDiscoveryClient.ApiResources(ctx, groupVersion.String())
	if err != nil {
		return err
	}

	gv, gverr := schema.ParseGroupVersion(resourceList.GroupVersion)

	infos := apiResourceSet{
		apiResources:     resourceList.APIResources,
		gvkToAPIResource: make(map[schema.GroupVersionKind]*v1.APIResource, len(resourceList.APIResources)/2),
		gvrToAPIResource: make(map[schema.GroupVersionResource]*v1.APIResource, len(resourceList.APIResources)),
	}
	for i := range infos.apiResources {
		res := &infos.apiResources[i]

		if len(res.Group) == 0 && len(res.Version) == 0 && gverr == nil {
			res.Group = gv.Group
			res.Version = gv.Version
		}

		infos.gvrToAPIResource[groupVersion.WithResource(res.Name)] = res

		// skip subresources
		if strings.Index(res.Name, "/") == 0 {
			infos.gvkToAPIResource[groupVersion.WithKind(res.Kind)] = res
		}
	}

	c.infos.Store(groupVersion, infos)

	return nil
}

func (c *ApiResourceClient) HasGroupVersion(gv schema.GroupVersion) bool {
	_, ok := c.infos.Load(gv)
	return ok
}

func (c *ApiResourceClient) GetGroupVersion(gv schema.GroupVersion) ([]v1.APIResource, bool) {
	if infos, ok := c.infos.Load(gv); !ok {
		return nil, false
	} else {
		return infos.(apiResourceSet).apiResources, true
	}
}

func (c *ApiResourceClient) GetGroupVersionKind(gvk schema.GroupVersionKind) (*v1.APIResource, bool) {
	if infos, ok := c.infos.Load(gvk.GroupVersion()); !ok {
		return nil, false
	} else if info, ok := infos.(apiResourceSet).gvkToAPIResource[gvk]; !ok {
		return nil, false
	} else {
		return info, true
	}
}

func (c *ApiResourceClient) GetGroupVersionResource(gvr schema.GroupVersionResource) (*v1.APIResource, bool) {
	if infos, ok := c.infos.Load(gvr.GroupVersion()); !ok {
		return nil, false
	} else if info, ok := infos.(apiResourceSet).gvrToAPIResource[gvr]; !ok {
		return nil, false
	} else {
		return info, true
	}
}
