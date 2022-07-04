package fake

import (
	"context"
	"fmt"
	"net/http"

	openapi_v2 "github.com/google/gnostic/openapiv2"
	openapi_v3 "github.com/google/gnostic/openapiv3"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/version"
	"k8s.io/client-go/discovery/raw"
	kubeversion "k8s.io/client-go/pkg/version"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/testing"
	"k8s.io/kube-openapi/pkg/handler3"
)

type FakeRawDiscoveryClient struct {
	*testing.Fake
	FakedServerVersion *version.Info
}

var _ raw.RawDiscoveryClient = &FakeRawDiscoveryClient{}

func (rc *FakeRawDiscoveryClient) RESTClient() rest.Interface {
	return nil
}

// ServerVersion retrieves and parses the server's version (git version).
func (rc *FakeRawDiscoveryClient) ServerVersion(ctx context.Context) (*version.Info, error) {
	action := testing.ActionImpl{}
	action.Verb = "get"
	action.Resource = schema.GroupVersionResource{Resource: "version"}
	rc.Invokes(action, nil)

	if rc.FakedServerVersion != nil {
		return rc.FakedServerVersion, nil
	}

	versionInfo := kubeversion.Get()
	return &versionInfo, nil
}

func (rc *FakeRawDiscoveryClient) ApiGroupVersions(ctx context.Context) (*metav1.APIGroupList, error) {
	action := testing.ActionImpl{
		Verb:     "get",
		Resource: schema.GroupVersionResource{Resource: "group"},
	}
	rc.Invokes(action, nil)

	groups := map[string]*metav1.APIGroup{}

	for _, res := range rc.Resources {
		gv, err := schema.ParseGroupVersion(res.GroupVersion)
		if err != nil {
			return nil, err
		}
		group := groups[gv.Group]
		if group == nil {
			group = &metav1.APIGroup{
				Name: gv.Group,
				PreferredVersion: metav1.GroupVersionForDiscovery{
					GroupVersion: res.GroupVersion,
					Version:      gv.Version,
				},
			}
			groups[gv.Group] = group
		}

		group.Versions = append(group.Versions, metav1.GroupVersionForDiscovery{
			GroupVersion: res.GroupVersion,
			Version:      gv.Version,
		})
	}

	list := &metav1.APIGroupList{}
	for _, apiGroup := range groups {
		list.Groups = append(list.Groups, *apiGroup)
	}

	return list, nil
}

func (rc *FakeRawDiscoveryClient) ApiCoreGroupVersions(ctx context.Context) (*metav1.APIVersions, error) {
	// TODO
	return nil, nil
}

func (rc *FakeRawDiscoveryClient) ApiResources(ctx context.Context, groupVersion string) (*metav1.APIResourceList, error) {
	action := testing.ActionImpl{
		Verb:     "get",
		Resource: schema.GroupVersionResource{Resource: "resource"},
	}
	rc.Invokes(action, nil)

	for _, resourceList := range rc.Resources {
		if resourceList.GroupVersion == groupVersion {
			return resourceList, nil
		}
	}

	return nil, &errors.StatusError{
		ErrStatus: metav1.Status{
			Status:  metav1.StatusFailure,
			Code:    http.StatusNotFound,
			Reason:  metav1.StatusReasonNotFound,
			Message: fmt.Sprintf("the server could not find the requested resource, GroupVersion %q not found", groupVersion),
		}}
}

func (rc *FakeRawDiscoveryClient) OpenAPISchema(ctx context.Context) (*openapi_v2.Document, error) {
	// TODO
	return nil, nil
}

func (rc *FakeRawDiscoveryClient) OpenAPIV3Discovery(ctx context.Context) (*handler3.OpenAPIV3Discovery, error) {
	// TODO
	return nil, nil
}

func (rc *FakeRawDiscoveryClient) OpenAPIV3Schema(ctx context.Context, endpoint string) (*openapi_v3.Document, error) {
	// TODO
	return nil, nil
}
