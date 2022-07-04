package raw

import (
	"context"
	"encoding/json"
	"fmt"

	openapi_v2 "github.com/google/gnostic/openapiv2"
	openapi_v3 "github.com/google/gnostic/openapiv3"
	"google.golang.org/protobuf/proto"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/version"
	"k8s.io/client-go/rest"
	"k8s.io/kube-openapi/pkg/handler3"
)

const (
	apiPrefix     = "/apis"
	apiCorePrefix = "/api"
	// TODO: support using json here too (in case scheme does not include the types??)
	// this could maybe cause errors? try with different schemes to check this!
	apiMime = "application/vnd.kubernetes.protobuf"

	versionEndpoint = "/version"
	versionMime     = "application/json"

	openAPIV2Endpoint   = "/openapi/v2"
	openAPIV2SchemaMime = "application/com.github.proto-openapi.spec.v2@v1.0+protobuf"

	openAPIV3Endpoint      = "/openapi/v3"
	openAPIV3DiscoveryMime = "application/json"
	openAPIV3SchemaMime    = "application/com.github.proto-openapi.spec.v3@v1.0+protobuf"
)

type RawDiscoveryClient interface {
	RESTClient() rest.Interface

	ServerVersion(ctx context.Context) (*version.Info, error)
	ApiGroupVersions(ctx context.Context) (*metav1.APIGroupList, error)
	ApiCoreGroupVersions(ctx context.Context) (*metav1.APIVersions, error)
	ApiResources(ctx context.Context, groupVersion string) (*metav1.APIResourceList, error)
	OpenAPISchema(ctx context.Context) (*openapi_v2.Document, error)
	OpenAPIV3Discovery(ctx context.Context) (*handler3.OpenAPIV3Discovery, error)
	OpenAPIV3Schema(ctx context.Context, endpoint string) (*openapi_v3.Document, error)
}

type implRawDiscoveryClient struct {
	rest.Interface
}

var _ RawDiscoveryClient = &implRawDiscoveryClient{}

func NewRawDiscoveryClient(rest rest.Interface) RawDiscoveryClient {
	return implRawDiscoveryClient{rest}
}

func (rc implRawDiscoveryClient) RESTClient() rest.Interface {
	return rc.Interface
}

// ServerVersion retrieves and parses the server's version (git version).
func (rc implRawDiscoveryClient) ServerVersion(ctx context.Context) (*version.Info, error) {
	body, err := rc.Get().AbsPath(versionEndpoint).SetHeader("Accept", versionMime).Do(ctx).Raw()
	if err != nil {
		return nil, err
	}

	var info version.Info
	if err = json.Unmarshal(body, &info); err != nil {
		return nil, fmt.Errorf("unable to parse the server version: %w", err)
	}

	return &info, nil
}

func (rc implRawDiscoveryClient) ApiGroupVersions(ctx context.Context) (*metav1.APIGroupList, error) {
	var apiGroupList metav1.APIGroupList
	err := rc.Get().AbsPath(apiPrefix).SetHeader("Accept", apiMime).Do(ctx).Into(&apiGroupList)

	if err != nil && !errors.IsNotFound(err) && !errors.IsForbidden(err) {
		return nil, err
	}

	return &apiGroupList, nil
}

func (rc implRawDiscoveryClient) ApiCoreGroupVersions(ctx context.Context) (*metav1.APIVersions, error) {
	var apiVersions metav1.APIVersions
	err := rc.Get().AbsPath(apiCorePrefix).SetHeader("Accept", apiMime).Do(ctx).Into(&apiVersions)

	if err != nil && !errors.IsNotFound(err) && !errors.IsForbidden(err) {
		return nil, err
	}

	return &apiVersions, nil
}

func (rc implRawDiscoveryClient) ApiResources(ctx context.Context, groupVersion string) (*metav1.APIResourceList, error) {
	var urlPath string
	if groupVersion == "v1" {
		urlPath = apiCorePrefix + "/" + groupVersion
	} else {
		urlPath = apiPrefix + "/" + groupVersion
	}

	resources := &metav1.APIResourceList{GroupVersion: groupVersion}
	if err := rc.Get().AbsPath(urlPath).SetHeader("Accept", apiMime).Do(ctx).Into(resources); err != nil {
		return nil, err
	}

	return resources, nil
}

func (rc implRawDiscoveryClient) OpenAPISchema(ctx context.Context) (*openapi_v2.Document, error) {
	data, err := rc.Get().AbsPath(openAPIV2Endpoint).SetHeader("Accept", openAPIV2SchemaMime).Do(ctx).Raw()
	if err != nil {
		return nil, err
	}

	document := &openapi_v2.Document{}
	if err = proto.Unmarshal(data, document); err != nil {
		return nil, err
	}

	return document, nil
}

func (rc implRawDiscoveryClient) OpenAPIV3Discovery(ctx context.Context) (*handler3.OpenAPIV3Discovery, error) {
	data, err := rc.Get().AbsPath(openAPIV3Endpoint).SetHeader("Accept", openAPIV3DiscoveryMime).Do(ctx).Raw()
	if err != nil {
		return nil, err
	}

	discoMap := &handler3.OpenAPIV3Discovery{}
	if err = json.Unmarshal(data, discoMap); err != nil {
		return nil, err
	}

	return discoMap, nil
}

func (rc implRawDiscoveryClient) OpenAPIV3Schema(ctx context.Context, endpoint string) (*openapi_v3.Document, error) {
	data, err := rc.Get().RequestURI(endpoint).SetHeader("Accept", openAPIV3SchemaMime).Do(ctx).Raw()
	if err != nil {
		return nil, err
	}

	document := &openapi_v3.Document{}
	if err := proto.Unmarshal(data, document); err != nil {
		return nil, err
	}

	return document, nil
}
