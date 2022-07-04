/*
Copyright 2014 The Kubernetes Authors.

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
	"encoding/json"
	"fmt"
	"mime"
	"net/http"
	"net/http/httptest"
	"reflect"
	"sort"
	"testing"
	"time"

	gogoproto "github.com/gogo/protobuf/proto"
	openapi_v2 "github.com/google/gnostic/openapiv2"
	openapi_v3 "github.com/google/gnostic/openapiv3"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/diff"
	"k8s.io/apimachinery/pkg/version"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	restclient "k8s.io/client-go/rest"
	testutil "k8s.io/client-go/util/testing"
	"k8s.io/kube-openapi/pkg/util/proto"
)

func newKubeScheme(t testing.TB) *runtime.Scheme {
	kubeScheme := runtime.NewScheme()
	if err := scheme.AddToScheme(kubeScheme); err != nil {
		t.Fatal(err)
	}
	return kubeScheme
}

func newNegotiator(t testing.TB) rest.SerializerNegotiator {
	return rest.NewSerializerNegotiator(newKubeScheme(t), false)
}

func discoveryClient(t testing.TB, config *restclient.Config) *DiscoveryClient {
	config.Negotiator = newNegotiator(t)
	restClient, err := restclient.RESTClientFor(config)
	if err != nil {
		t.Fatal(err)
	}
	return NewDiscoveryClient(restClient)
}

func TestGetServerVersion(t *testing.T) {
	expect := version.Info{
		Major:     "foo",
		Minor:     "bar",
		GitCommit: "baz",
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		output, err := json.Marshal(expect)
		if err != nil {
			t.Errorf("unexpected encoding error: %v", err)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write(output)
	}))
	defer server.Close()

	client := discoveryClient(t, &restclient.Config{Host: server.URL})

	got, err := client.ServerVersion(context.TODO())
	if err != nil {
		t.Fatalf("unexpected encoding error: %v", err)
	}
	if e, a := expect, *got; !reflect.DeepEqual(e, a) {
		t.Errorf("expected %v, got %v", e, a)
	}
}

func TestGetServerGroupsWithV1Server(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		var obj interface{}
		switch req.URL.Path {
		case "/api":
			obj = &metav1.APIVersions{
				Versions: []string{
					"v1",
				},
			}
		case "/apis":
			obj = &metav1.APIGroupList{
				Groups: []metav1.APIGroup{
					{
						Name: "extensions",
						Versions: []metav1.GroupVersionForDiscovery{
							{GroupVersion: "extensions/v1beta1"},
						},
					},
				},
			}
		default:
			w.WriteHeader(http.StatusNotFound)
			return
		}
		output, err := json.Marshal(obj)
		if err != nil {
			t.Fatalf("unexpected encoding error: %v", err)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write(output)
	}))
	defer server.Close()
	client := discoveryClient(t, &restclient.Config{Host: server.URL})
	// ServerGroups should not return an error even if server returns error at /api and /apis
	groupVersions, err := client.ApiGroupVersions(context.TODO())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !reflect.DeepEqual(GroupVersions(groupVersions), []schema.GroupVersion{{Version: "v1"}, {Group: "extensions", Version: "v1beta1"}}) {
		t.Errorf("expected: %q, got: %q", []string{"v1", "extensions/v1beta1"}, groupVersions)
	}
}

func TestGetServerGroupsWithBrokenServer(t *testing.T) {
	for _, statusCode := range []int{http.StatusNotFound, http.StatusForbidden} {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			w.WriteHeader(statusCode)
		}))
		defer server.Close()
		client := discoveryClient(t, &restclient.Config{Host: server.URL})
		// ServerGroups should not return an error even if server returns Not Found or Forbidden error at all end points
		groupVersions, err := client.ApiGroupVersions(context.TODO())
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(groupVersions) != 0 {
			t.Errorf("expected empty list, got: %q", groupVersions)
		}
	}
}

func TestGetServerResourcesWithV1Server(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		var obj interface{}
		switch req.URL.Path {
		case "/api":
			obj = &metav1.APIVersions{
				Versions: []string{
					"v1",
				},
			}
		default:
			w.WriteHeader(http.StatusNotFound)
			return
		}
		output, err := json.Marshal(obj)
		if err != nil {
			t.Errorf("unexpected encoding error: %v", err)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write(output)
	}))
	defer server.Close()
	client := discoveryClient(t, &restclient.Config{Host: server.URL})

	groupVersions, err := client.ApiGroupVersions(context.TODO())
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if !reflect.DeepEqual(GroupVersions(groupVersions), []schema.GroupVersion{{Version: "v1"}}) {
		t.Errorf("missing v1 in resource list: %v", groupVersions)
	}
}

func TestGetServerResourcesForGroupVersion(t *testing.T) {
	stable := metav1.APIResourceList{
		GroupVersion: "v1",
		APIResources: []metav1.APIResource{
			{Name: "pods", Namespaced: true, Kind: "Pod"},
			{Name: "services", Namespaced: true, Kind: "Service"},
			{Name: "namespaces", Namespaced: false, Kind: "Namespace"},
		},
	}
	beta := metav1.APIResourceList{
		GroupVersion: "extensions/v1beta1",
		APIResources: []metav1.APIResource{
			{Name: "deployments", Namespaced: true, Kind: "Deployment"},
			{Name: "ingresses", Namespaced: true, Kind: "Ingress"},
			{Name: "jobs", Namespaced: true, Kind: "Job"},
		},
	}
	beta2 := metav1.APIResourceList{
		GroupVersion: "extensions/v1beta2",
		APIResources: []metav1.APIResource{
			{Name: "deployments", Namespaced: true, Kind: "Deployment"},
			{Name: "ingresses", Namespaced: true, Kind: "Ingress"},
			{Name: "jobs", Namespaced: true, Kind: "Job"},
		},
	}
	extensionsbeta3 := metav1.APIResourceList{GroupVersion: "extensions/v1beta3", APIResources: []metav1.APIResource{{Name: "deployments", Namespaced: true, Kind: "Deployment"}}}
	extensionsbeta4 := metav1.APIResourceList{GroupVersion: "extensions/v1beta4", APIResources: []metav1.APIResource{{Name: "deployments", Namespaced: true, Kind: "Deployment"}}}
	extensionsbeta5 := metav1.APIResourceList{GroupVersion: "extensions/v1beta5", APIResources: []metav1.APIResource{{Name: "deployments", Namespaced: true, Kind: "Deployment"}}}
	extensionsbeta6 := metav1.APIResourceList{GroupVersion: "extensions/v1beta6", APIResources: []metav1.APIResource{{Name: "deployments", Namespaced: true, Kind: "Deployment"}}}
	extensionsbeta7 := metav1.APIResourceList{GroupVersion: "extensions/v1beta7", APIResources: []metav1.APIResource{{Name: "deployments", Namespaced: true, Kind: "Deployment"}}}
	extensionsbeta8 := metav1.APIResourceList{GroupVersion: "extensions/v1beta8", APIResources: []metav1.APIResource{{Name: "deployments", Namespaced: true, Kind: "Deployment"}}}
	extensionsbeta9 := metav1.APIResourceList{GroupVersion: "extensions/v1beta9", APIResources: []metav1.APIResource{{Name: "deployments", Namespaced: true, Kind: "Deployment"}}}
	extensionsbeta10 := metav1.APIResourceList{GroupVersion: "extensions/v1beta10", APIResources: []metav1.APIResource{{Name: "deployments", Namespaced: true, Kind: "Deployment"}}}

	appsbeta1 := metav1.APIResourceList{GroupVersion: "apps/v1beta1", APIResources: []metav1.APIResource{{Name: "deployments", Namespaced: true, Kind: "Deployment"}}}
	appsbeta2 := metav1.APIResourceList{GroupVersion: "apps/v1beta2", APIResources: []metav1.APIResource{{Name: "deployments", Namespaced: true, Kind: "Deployment"}}}
	appsbeta3 := metav1.APIResourceList{GroupVersion: "apps/v1beta3", APIResources: []metav1.APIResource{{Name: "deployments", Namespaced: true, Kind: "Deployment"}}}
	appsbeta4 := metav1.APIResourceList{GroupVersion: "apps/v1beta4", APIResources: []metav1.APIResource{{Name: "deployments", Namespaced: true, Kind: "Deployment"}}}
	appsbeta5 := metav1.APIResourceList{GroupVersion: "apps/v1beta5", APIResources: []metav1.APIResource{{Name: "deployments", Namespaced: true, Kind: "Deployment"}}}
	appsbeta6 := metav1.APIResourceList{GroupVersion: "apps/v1beta6", APIResources: []metav1.APIResource{{Name: "deployments", Namespaced: true, Kind: "Deployment"}}}
	appsbeta7 := metav1.APIResourceList{GroupVersion: "apps/v1beta7", APIResources: []metav1.APIResource{{Name: "deployments", Namespaced: true, Kind: "Deployment"}}}
	appsbeta8 := metav1.APIResourceList{GroupVersion: "apps/v1beta8", APIResources: []metav1.APIResource{{Name: "deployments", Namespaced: true, Kind: "Deployment"}}}
	appsbeta9 := metav1.APIResourceList{GroupVersion: "apps/v1beta9", APIResources: []metav1.APIResource{{Name: "deployments", Namespaced: true, Kind: "Deployment"}}}
	appsbeta10 := metav1.APIResourceList{GroupVersion: "apps/v1beta10", APIResources: []metav1.APIResource{{Name: "deployments", Namespaced: true, Kind: "Deployment"}}}

	tests := []struct {
		resourcesList *metav1.APIResourceList
		path          string
		request       schema.GroupVersion
		expectErr     bool
	}{
		{
			resourcesList: &stable,
			path:          "/api/v1",
			request:       schema.GroupVersion{Version: "v1"},
			expectErr:     false,
		},
		{
			resourcesList: &beta,
			path:          "/apis/extensions/v1beta1",
			request:       schema.GroupVersion{Version: "v1beta1", Group: "extensions"},
			expectErr:     false,
		},
		{
			resourcesList: &stable,
			path:          "/api/v1",
			request:       schema.GroupVersion{Group: "foobar"},
			expectErr:     true,
		},
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		var list interface{}
		switch req.URL.Path {
		case "/api/v1":
			list = &stable
		case "/apis/extensions/v1beta1":
			list = &beta
		case "/apis/extensions/v1beta2":
			list = &beta2
		case "/apis/extensions/v1beta3":
			list = &extensionsbeta3
		case "/apis/extensions/v1beta4":
			list = &extensionsbeta4
		case "/apis/extensions/v1beta5":
			list = &extensionsbeta5
		case "/apis/extensions/v1beta6":
			list = &extensionsbeta6
		case "/apis/extensions/v1beta7":
			list = &extensionsbeta7
		case "/apis/extensions/v1beta8":
			list = &extensionsbeta8
		case "/apis/extensions/v1beta9":
			list = &extensionsbeta9
		case "/apis/extensions/v1beta10":
			list = &extensionsbeta10
		case "/apis/apps/v1beta1":
			list = &appsbeta1
		case "/apis/apps/v1beta2":
			list = &appsbeta2
		case "/apis/apps/v1beta3":
			list = &appsbeta3
		case "/apis/apps/v1beta4":
			list = &appsbeta4
		case "/apis/apps/v1beta5":
			list = &appsbeta5
		case "/apis/apps/v1beta6":
			list = &appsbeta6
		case "/apis/apps/v1beta7":
			list = &appsbeta7
		case "/apis/apps/v1beta8":
			list = &appsbeta8
		case "/apis/apps/v1beta9":
			list = &appsbeta9
		case "/apis/apps/v1beta10":
			list = &appsbeta10
		case "/api":
			list = &metav1.APIVersions{
				Versions: []string{
					"v1",
				},
			}
		case "/apis":
			list = &metav1.APIGroupList{
				Groups: []metav1.APIGroup{
					{
						Name: "apps",
						Versions: []metav1.GroupVersionForDiscovery{
							{GroupVersion: "apps/v1beta1", Version: "v1beta1"},
							{GroupVersion: "apps/v1beta2", Version: "v1beta2"},
							{GroupVersion: "apps/v1beta3", Version: "v1beta3"},
							{GroupVersion: "apps/v1beta4", Version: "v1beta4"},
							{GroupVersion: "apps/v1beta5", Version: "v1beta5"},
							{GroupVersion: "apps/v1beta6", Version: "v1beta6"},
							{GroupVersion: "apps/v1beta7", Version: "v1beta7"},
							{GroupVersion: "apps/v1beta8", Version: "v1beta8"},
							{GroupVersion: "apps/v1beta9", Version: "v1beta9"},
							{GroupVersion: "apps/v1beta10", Version: "v1beta10"},
						},
					},
					{
						Name: "extensions",
						Versions: []metav1.GroupVersionForDiscovery{
							{GroupVersion: "extensions/v1beta1", Version: "v1beta1"},
							{GroupVersion: "extensions/v1beta2", Version: "v1beta2"},
							{GroupVersion: "extensions/v1beta3", Version: "v1beta3"},
							{GroupVersion: "extensions/v1beta4", Version: "v1beta4"},
							{GroupVersion: "extensions/v1beta5", Version: "v1beta5"},
							{GroupVersion: "extensions/v1beta6", Version: "v1beta6"},
							{GroupVersion: "extensions/v1beta7", Version: "v1beta7"},
							{GroupVersion: "extensions/v1beta8", Version: "v1beta8"},
							{GroupVersion: "extensions/v1beta9", Version: "v1beta9"},
							{GroupVersion: "extensions/v1beta10", Version: "v1beta10"},
						},
					},
				},
			}
		default:
			t.Logf("unexpected request: %s", req.URL.Path)
			w.WriteHeader(http.StatusNotFound)
			return
		}
		output, err := json.Marshal(list)
		if err != nil {
			t.Errorf("unexpected encoding error: %v", err)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write(output)
	}))
	defer server.Close()
	for _, test := range tests {
		client := discoveryClient(t, &restclient.Config{Host: server.URL})
		got, err := client.ApiResources(context.TODO(), test.request)
		if test.expectErr {
			if err == nil {
				t.Error("unexpected non-error")
			}
			continue
		}
		if err != nil {
			t.Errorf("unexpected error: %v", err)
			continue
		}
		if !reflect.DeepEqual(got, UnpackAPIResourceLists(test.resourcesList)) {
			t.Errorf("expected:\n%v\ngot:\n%v\n", UnpackAPIResourceLists(test.resourcesList), got)
		}
	}

	client := discoveryClient(t, &restclient.Config{Host: server.URL})
	start := time.Now()

	serverGroups, err := client.ApiGroupVersions(context.TODO())
	if err != nil {
		t.Fatal(err)
	}

	end := time.Now()
	if d := end.Sub(start); d > time.Second {
		t.Errorf("took too long to perform discovery: %s", d)
	}
	expectedGroupVersions := []schema.GroupVersion{
		{Group: "", Version: "v1"},
		{Group: "apps", Version: "v1beta1"},
		{Group: "apps", Version: "v1beta2"},
		{Group: "apps", Version: "v1beta3"},
		{Group: "apps", Version: "v1beta4"},
		{Group: "apps", Version: "v1beta5"},
		{Group: "apps", Version: "v1beta6"},
		{Group: "apps", Version: "v1beta7"},
		{Group: "apps", Version: "v1beta8"},
		{Group: "apps", Version: "v1beta9"},
		{Group: "apps", Version: "v1beta10"},
		{Group: "extensions", Version: "v1beta1"},
		{Group: "extensions", Version: "v1beta2"},
		{Group: "extensions", Version: "v1beta3"},
		{Group: "extensions", Version: "v1beta4"},
		{Group: "extensions", Version: "v1beta5"},
		{Group: "extensions", Version: "v1beta6"},
		{Group: "extensions", Version: "v1beta7"},
		{Group: "extensions", Version: "v1beta8"},
		{Group: "extensions", Version: "v1beta9"},
		{Group: "extensions", Version: "v1beta10"},
	}
	if !reflect.DeepEqual(expectedGroupVersions, GroupVersions(serverGroups)) {
		t.Errorf("unexpected group versions: %v", diff.ObjectReflectDiff(expectedGroupVersions, serverGroups))
	}
}

func returnedOpenAPI() *openapi_v2.Document {
	return &openapi_v2.Document{
		Definitions: &openapi_v2.Definitions{
			AdditionalProperties: []*openapi_v2.NamedSchema{
				{
					Name: "fake.type.1",
					Value: &openapi_v2.Schema{
						VendorExtension: []*openapi_v2.NamedAny{
							{
								Name: "x-kubernetes-group-version-kind",
								Value: &openapi_v2.Any{
									Yaml: "[{\"group\":\"fake\",\"kind\":\"Type\",\"version\":\"1\"}]",
								},
							},
						},
						Properties: &openapi_v2.Properties{
							AdditionalProperties: []*openapi_v2.NamedSchema{
								{
									Name: "count",
									Value: &openapi_v2.Schema{
										Type: &openapi_v2.TypeItem{
											Value: []string{"integer"},
										},
									},
								},
							},
						},
					},
				},
				{
					Name: "fake.type.2",
					Value: &openapi_v2.Schema{
						VendorExtension: []*openapi_v2.NamedAny{
							{
								Name: "x-kubernetes-group-version-kind",
								Value: &openapi_v2.Any{
									Yaml: "[{\"group\":\"fake\",\"kind\":\"Type\",\"version\":\"2\"}]",
								},
							},
						},
						Properties: &openapi_v2.Properties{
							AdditionalProperties: []*openapi_v2.NamedSchema{
								{
									Name: "count",
									Value: &openapi_v2.Schema{
										Type: &openapi_v2.TypeItem{
											Value: []string{"array"},
										},
										Items: &openapi_v2.ItemsItem{
											Schema: []*openapi_v2.Schema{
												{
													Type: &openapi_v2.TypeItem{
														Value: []string{"string"},
													},
												},
											},
										},
									},
								},
							},
						},
					},
				},
			},
		},
	}
}

func returnedOpenApiSchema(t testing.TB, gvk schema.GroupVersionKind) proto.Schema {
	models, err := proto.NewOpenAPIData(returnedOpenAPI())
	if err != nil {
		t.Fatal("test openapi is not valid")
	}

	modelName := ""
	if gvk.Group == "fake" && gvk.Version == "1" && gvk.Kind == "Type" {
		modelName = "fake.type.1"
	} else if gvk.Group == "fake" && gvk.Version == "2" && gvk.Kind == "Type" {
		modelName = "fake.type.2"
	}

	model := models.LookupModel(modelName)
	if model == nil {
		t.Fatal("ListModels returns a model that can't be looked-up")
	}

	return model
}

func openapiSchemaFakeServer(t *testing.T) (*httptest.Server, error) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		if req.URL.Path != "/openapi/v2" {
			errMsg := fmt.Sprintf("Unexpected url %v", req.URL)
			w.WriteHeader(http.StatusNotFound)
			w.Write([]byte(errMsg))
			return
		}
		if req.Method != "GET" {
			errMsg := fmt.Sprintf("Unexpected method %v", req.Method)
			w.WriteHeader(http.StatusMethodNotAllowed)
			w.Write([]byte(errMsg))
			t.Errorf("testing should fail as %s", errMsg)
			return
		}
		decipherableFormat := req.Header.Get("Accept")
		if decipherableFormat != "application/com.github.proto-openapi.spec.v2@v1.0+protobuf" {
			errMsg := fmt.Sprintf("Unexpected accept mime type %v", decipherableFormat)
			w.WriteHeader(http.StatusUnsupportedMediaType)
			w.Write([]byte(errMsg))
			t.Errorf("testing should fail as %s", errMsg)
			return
		}

		mime.AddExtensionType(".pb-v1", "application/com.github.googleapis.gnostic.OpenAPIv2@68f4ded+protobuf")

		output, err := gogoproto.Marshal(returnedOpenAPI())
		if err != nil {
			errMsg := fmt.Sprintf("Unexpected marshal error: %v", err)
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte(errMsg))
			t.Errorf("testing should fail as %s", errMsg)
			return
		}
		w.WriteHeader(http.StatusOK)
		w.Write(output)
	}))

	return server, nil
}

func openapiV3SchemaFakeServer(t *testing.T) (*httptest.Server, map[string]*openapi_v3.Document, error) {
	res, err := testutil.NewFakeOpenAPIV3Server("testdata")
	if err != nil {
		return nil, nil, err
	}
	return res.HttpServer, res.ServedDocuments, nil
}

func TestGetOpenAPISchema(t *testing.T) {
	server, err := openapiSchemaFakeServer(t)
	if err != nil {
		t.Errorf("unexpected error starting fake server: %v", err)
	}
	defer server.Close()

	client := discoveryClient(t, &restclient.Config{Host: server.URL})
	gvk := schema.GroupVersionKind{Group: "fake", Version: "2", Kind: "Type"}
	got, err := client.OpenApiSchema(context.TODO(), gvk)
	if err != nil {
		t.Fatalf("unexpected error getting openapi: %v", err)
	}
	if e, a := returnedOpenApiSchema(t, gvk), got; !reflect.DeepEqual(e, a) {
		t.Errorf("expected \n%v, got \n%v", e, a)
	}
}

func TestGetOpenAPISchemaV3(t *testing.T) {
	server, testV3Specs, err := openapiV3SchemaFakeServer(t)
	if err != nil {
		t.Errorf("unexpected error starting fake server: %v", err)
	}
	defer server.Close()

	client := discoveryClient(t, &restclient.Config{Host: server.URL})
	gvk := schema.GroupVersionKind{Group: "batch", Version: "v1", Kind: "CronJob"}
	got, err := client.OpenApiSchema(context.TODO(), gvk)
	if err != nil {
		t.Fatal(err)
	}

	models, err := proto.NewOpenAPIV3Data(testV3Specs["apis/batch/v1"])
	if err != nil {
		t.Fatal(err)
	}

	expected := models.LookupModel("io.k8s.api.batch.v1.CronJob")

	if err != nil {
		t.Fatalf("unexpected error getting openapi: %v", err)
	}
	if e, a := expected, got; !reflect.DeepEqual(e, a) {
		t.Errorf("expected \n%v, got \n%v", e, a)
	}
}

func TestServerPreferredResources(t *testing.T) {
	stable := metav1.APIResourceList{
		GroupVersion: "v1",
		APIResources: []metav1.APIResource{
			{Name: "pods", Namespaced: true, Kind: "Pod"},
			{Name: "services", Namespaced: true, Kind: "Service"},
			{Name: "namespaces", Namespaced: false, Kind: "Namespace"},
		},
	}
	tests := []struct {
		resourcesList []*metav1.APIResourceList
		response      func(w http.ResponseWriter, req *http.Request)
		expectErr     func(err error) bool
	}{
		{
			resourcesList: []*metav1.APIResourceList{&stable},
			expectErr:     IsGroupDiscoveryFailedError,
			response: func(w http.ResponseWriter, req *http.Request) {
				var list interface{}
				switch req.URL.Path {
				case "/apis/extensions/v1beta1":
					w.WriteHeader(http.StatusInternalServerError)
					return
				case "/api/v1":
					list = &stable
				case "/api":
					list = &metav1.APIVersions{
						Versions: []string{
							"v1",
						},
					}
				case "/apis":
					list = &metav1.APIGroupList{
						Groups: []metav1.APIGroup{
							{
								Versions: []metav1.GroupVersionForDiscovery{
									{GroupVersion: "extensions/v1beta1"},
								},
							},
						},
					}
				default:
					t.Logf("unexpected request: %s", req.URL.Path)
					w.WriteHeader(http.StatusNotFound)
					return
				}
				output, err := json.Marshal(list)
				if err != nil {
					t.Errorf("unexpected encoding error: %v", err)
					return
				}
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusOK)
				w.Write(output)
			},
		},
		{
			resourcesList: nil,
			expectErr:     IsGroupDiscoveryFailedError,
			response: func(w http.ResponseWriter, req *http.Request) {
				var list interface{}
				switch req.URL.Path {
				case "/apis/extensions/v1beta1":
					w.WriteHeader(http.StatusInternalServerError)
					return
				case "/api/v1":
					w.WriteHeader(http.StatusInternalServerError)
					return
				case "/api":
					list = &metav1.APIVersions{
						Versions: []string{
							"v1",
						},
					}
				case "/apis":
					list = &metav1.APIGroupList{
						Groups: []metav1.APIGroup{
							{
								Versions: []metav1.GroupVersionForDiscovery{
									{GroupVersion: "extensions/v1beta1"},
								},
							},
						},
					}
				default:
					t.Logf("unexpected request: %s", req.URL.Path)
					w.WriteHeader(http.StatusNotFound)
					return
				}
				output, err := json.Marshal(list)
				if err != nil {
					t.Errorf("unexpected encoding error: %v", err)
					return
				}
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusOK)
				w.Write(output)
			},
		},
	}
	for _, test := range tests {
		server := httptest.NewServer(http.HandlerFunc(test.response))
		defer server.Close()

		client := discoveryClient(t, &restclient.Config{Host: server.URL})

		resources, err := PreferredServerResources(context.TODO(), client)

		if test.expectErr != nil {
			if err == nil {
				t.Error("unexpected non-error")
			}

			continue
		}
		if err != nil {
			t.Errorf("unexpected error: %v", err)
			continue
		}
		got := GroupVersionResources(resources)
		expected := GroupVersionResources(UnpackAPIResourceLists(test.resourcesList...))
		if !reflect.DeepEqual(got, expected) {
			t.Errorf("expected:\n%v\ngot:\n%v\n", test.resourcesList, got)
		}
		server.Close()
	}
}

func TestServerPreferredResourcesRetries(t *testing.T) {
	stable := metav1.APIResourceList{
		GroupVersion: "v1",
		APIResources: []metav1.APIResource{
			{Name: "pods", Namespaced: true, Kind: "Pod"},
		},
	}
	beta := metav1.APIResourceList{
		GroupVersion: "extensions/v1",
		APIResources: []metav1.APIResource{
			{Name: "deployments", Namespaced: true, Kind: "Deployment"},
		},
	}

	response := func(numErrors int) http.HandlerFunc {
		var i = 0
		return func(w http.ResponseWriter, req *http.Request) {
			var list interface{}
			switch req.URL.Path {
			case "/apis/extensions/v1beta1":
				if i < numErrors {
					i++
					w.Header().Add("Retry-After", "0")
					w.WriteHeader(http.StatusInternalServerError)
					return
				}
				list = &beta
			case "/api/v1":
				list = &stable
			case "/api":
				list = &metav1.APIVersions{
					Versions: []string{
						"v1",
					},
				}
			case "/apis":
				list = &metav1.APIGroupList{
					Groups: []metav1.APIGroup{
						{
							Name: "extensions",
							Versions: []metav1.GroupVersionForDiscovery{
								{GroupVersion: "extensions/v1beta1", Version: "v1beta1"},
							},
							PreferredVersion: metav1.GroupVersionForDiscovery{
								GroupVersion: "extensions/v1beta1",
								Version:      "v1beta1",
							},
						},
					},
				}
			default:
				t.Logf("unexpected request: %s", req.URL.Path)
				w.WriteHeader(http.StatusNotFound)
				return
			}
			output, err := json.Marshal(list)
			if err != nil {
				t.Errorf("unexpected encoding error: %v", err)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			w.Write(output)
		}
	}
	tests := []struct {
		responseErrors  int
		expectResources int
		expectedError   func(err error) bool
	}{
		{
			responseErrors:  1,
			expectResources: 2,
			expectedError: func(err error) bool {
				return err == nil
			},
		},
		{
			responseErrors:  2,
			expectResources: 2,
			expectedError: func(err error) bool {
				return err == nil
			},
		},
	}

	for i, tc := range tests {
		server := httptest.NewServer(http.HandlerFunc(response(tc.responseErrors)))
		defer server.Close()

		client := discoveryClient(t, &restclient.Config{Host: server.URL})
		resources, err := PreferredServerResources(context.TODO(), client)
		if !tc.expectedError(err) {
			t.Errorf("case %d: unexpected error: %v", i, err)
		}
		got := GroupVersionResources(resources)
		if len(got) != tc.expectResources {
			t.Errorf("case %d: expect %d resources, got %#v", i, tc.expectResources, got)
		}
		server.Close()
	}
}

func TestServerPreferredNamespacedResources(t *testing.T) {
	stable := metav1.APIResourceList{
		GroupVersion: "v1",
		APIResources: []metav1.APIResource{
			{Name: "pods", Namespaced: true, Kind: "Pod"},
			{Name: "services", Namespaced: true, Kind: "Service"},
			{Name: "namespaces", Namespaced: false, Kind: "Namespace"},
		},
	}
	batchv1 := metav1.APIResourceList{
		GroupVersion: "batch/v1",
		APIResources: []metav1.APIResource{
			{Name: "jobs", Namespaced: true, Kind: "Job"},
		},
	}
	batchv2alpha1 := metav1.APIResourceList{
		GroupVersion: "batch/v2alpha1",
		APIResources: []metav1.APIResource{
			{Name: "jobs", Namespaced: true, Kind: "Job"},
			{Name: "cronjobs", Namespaced: true, Kind: "CronJob"},
		},
	}
	batchv3alpha1 := metav1.APIResourceList{
		GroupVersion: "batch/v3alpha1",
		APIResources: []metav1.APIResource{
			{Name: "jobs", Namespaced: true, Kind: "Job"},
			{Name: "cronjobs", Namespaced: true, Kind: "CronJob"},
		},
	}
	tests := []struct {
		response func(w http.ResponseWriter, req *http.Request)
		expected []schema.GroupVersionResource
	}{
		{
			response: func(w http.ResponseWriter, req *http.Request) {
				var list interface{}
				switch req.URL.Path {
				case "/api/v1":
					list = &stable
				case "/api":
					list = &metav1.APIVersions{
						Versions: []string{
							"v1",
						},
					}
				default:
					t.Logf("unexpected request: %s", req.URL.Path)
					w.WriteHeader(http.StatusNotFound)
					return
				}
				output, err := json.Marshal(list)
				if err != nil {
					t.Errorf("unexpected encoding error: %v", err)
					return
				}
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusOK)
				w.Write(output)
			},
			expected: []schema.GroupVersionResource{
				{Group: "", Version: "v1", Resource: "pods"},
				{Group: "", Version: "v1", Resource: "services"},
			},
		},
		{
			response: func(w http.ResponseWriter, req *http.Request) {
				var list interface{}
				switch req.URL.Path {
				case "/apis":
					list = &metav1.APIGroupList{
						Groups: []metav1.APIGroup{
							{
								Name: "batch",
								Versions: []metav1.GroupVersionForDiscovery{
									{GroupVersion: "batch/v1", Version: "v1"},
									{GroupVersion: "batch/v2alpha1", Version: "v2alpha1"},
									{GroupVersion: "batch/v3alpha1", Version: "v3alpha1"},
								},
								PreferredVersion: metav1.GroupVersionForDiscovery{GroupVersion: "batch/v1", Version: "v1"},
							},
						},
					}
				case "/apis/batch/v1":
					list = &batchv1
				case "/apis/batch/v2alpha1":
					list = &batchv2alpha1
				case "/apis/batch/v3alpha1":
					list = &batchv3alpha1
				default:
					t.Logf("unexpected request: %s", req.URL.Path)
					w.WriteHeader(http.StatusNotFound)
					return
				}
				output, err := json.Marshal(list)
				if err != nil {
					t.Errorf("unexpected encoding error: %v", err)
					return
				}
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusOK)
				w.Write(output)
			},
			expected: []schema.GroupVersionResource{
				{Group: "batch", Version: "v1", Resource: "jobs"},
				{Group: "batch", Version: "v2alpha1", Resource: "cronjobs"},
			},
		},
		{
			response: func(w http.ResponseWriter, req *http.Request) {
				var list interface{}
				switch req.URL.Path {
				case "/apis":
					list = &metav1.APIGroupList{
						Groups: []metav1.APIGroup{
							{
								Name: "batch",
								Versions: []metav1.GroupVersionForDiscovery{
									{GroupVersion: "batch/v1", Version: "v1"},
									{GroupVersion: "batch/v2alpha1", Version: "v2alpha1"},
									{GroupVersion: "batch/v3alpha1", Version: "v3alpha1"},
								},
								PreferredVersion: metav1.GroupVersionForDiscovery{GroupVersion: "batch/v2alpha", Version: "v2alpha1"},
							},
						},
					}
				case "/apis/batch/v1":
					list = &batchv1
				case "/apis/batch/v2alpha1":
					list = &batchv2alpha1
				case "/apis/batch/v3alpha1":
					list = &batchv3alpha1
				default:
					t.Logf("unexpected request: %s", req.URL.Path)
					w.WriteHeader(http.StatusNotFound)
					return
				}
				output, err := json.Marshal(list)
				if err != nil {
					t.Errorf("unexpected encoding error: %v", err)
					return
				}
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusOK)
				w.Write(output)
			},
			expected: []schema.GroupVersionResource{
				{Group: "batch", Version: "v2alpha1", Resource: "jobs"},
				{Group: "batch", Version: "v2alpha1", Resource: "cronjobs"},
			},
		},
	}
	for i, test := range tests {
		server := httptest.NewServer(http.HandlerFunc(test.response))
		defer server.Close()

		client := discoveryClient(t, &restclient.Config{Host: server.URL})
		resources, err := PreferredServerResources(context.TODO(), client)
		if err != nil {
			t.Errorf("[%d] unexpected error: %v", i, err)
			continue
		}

		resources = FilteredBy(ResourcePredicateFunc(func(resource *metav1.APIResource) bool {
			return resource.Namespaced
		}), resources)

		got := GroupVersionResources(resources)
		if !equalGroupVersionResourceLists(got, test.expected) {
			t.Errorf("[%d] expected:\n%v\ngot:\n%v\n", i, test.expected, got)
		}
		server.Close()
	}
}

func equalGroupVersionResourceLists(listA, listB []schema.GroupVersionResource) bool {
	sortFn := func(list []schema.GroupVersionResource) func(i, j int) bool {
		return func(i, j int) bool {
			if list[i].Group != list[j].Group {
				return list[i].Group < list[j].Group
			} else if list[i].Version != list[j].Version {
				return list[i].Version < list[j].Version
			}

			return list[i].Resource < list[j].Resource
		}
	}

	sort.Slice(listA, sortFn(listA))
	sort.Slice(listB, sortFn(listB))

	return reflect.DeepEqual(listA, listB)
}
