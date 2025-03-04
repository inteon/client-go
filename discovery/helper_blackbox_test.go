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

package discovery_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"io/ioutil"
	"net/http"
	"reflect"
	"strings"
	"testing"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	restclient "k8s.io/client-go/rest"
	"k8s.io/client-go/rest/fake"
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

func discoveryClient(t testing.TB, config *restclient.Config) *discovery.DiscoveryClient {
	config.Negotiator = newNegotiator(t)
	restClient, err := config.Build()
	if err != nil {
		t.Fatal(err)
	}
	return discovery.NewDiscoveryClient(restClient)
}

func objBody(object interface{}) io.ReadCloser {
	output, err := json.MarshalIndent(object, "", "")
	if err != nil {
		panic(err)
	}
	return ioutil.NopCloser(bytes.NewReader([]byte(output)))
}

func TestServerSupportsVersion(t *testing.T) {
	tests := []struct {
		name            string
		requiredVersion schema.GroupVersion
		serverVersions  []string
		expectErr       func(err error) bool
		sendErr         error
		statusCode      int
	}{
		{
			name:            "explicit version supported",
			requiredVersion: schema.GroupVersion{Version: "v1"},
			serverVersions:  []string{"/version1", v1.SchemeGroupVersion.String()},
			statusCode:      http.StatusOK,
		},
		{
			name:            "explicit version not supported on server",
			requiredVersion: schema.GroupVersion{Version: "v1"},
			serverVersions:  []string{"version1"},
			expectErr:       func(err error) bool { return strings.Contains(err.Error(), `server does not support API version "v1"`) },
			statusCode:      http.StatusOK,
		},
		{
			name:           "connection refused error",
			serverVersions: []string{"version1"},
			sendErr:        errors.New("connection refused"),
			expectErr:      func(err error) bool { return strings.Contains(err.Error(), "connection refused") },
			statusCode:     http.StatusOK,
		},
		{
			name:            "discovery fails due to 404 Not Found errors and thus serverVersions is empty, use requested GroupVersion",
			requiredVersion: schema.GroupVersion{Version: "version1"},
			statusCode:      http.StatusNotFound,
		},
	}

	for _, test := range tests {
		fakeClient := fake.CreateHTTPClient(func(req *http.Request) (*http.Response, error) {
			if test.sendErr != nil {
				return nil, test.sendErr
			}
			header := http.Header{}
			header.Set("Content-Type", runtime.ContentTypeJSON)
			return &http.Response{StatusCode: test.statusCode, Header: header, Body: objBody(&metav1.APIVersions{Versions: test.serverVersions})}, nil
		})
		c := discoveryClient(t, &restclient.Config{Negotiator: newNegotiator(t)})
		c.RESTClient().(*restclient.RESTClient).Client = fakeClient
		err := discovery.ServerSupportsVersion(context.TODO(), c, test.requiredVersion)
		if err == nil && test.expectErr != nil {
			t.Errorf("expected error, got nil for [%s].", test.name)
		}
		if err != nil {
			if test.expectErr == nil || !test.expectErr(err) {
				t.Errorf("unexpected error for [%s]: %v.", test.name, err)
			}
			continue
		}
	}
}

func TestFilteredBy(t *testing.T) {
	all := discovery.ResourcePredicateFunc(func(resource *metav1.APIResource) bool {
		return true
	})
	none := discovery.ResourcePredicateFunc(func(resource *metav1.APIResource) bool {
		return false
	})
	onlyV2 := discovery.ResourcePredicateFunc(func(resource *metav1.APIResource) bool {
		return resource.Version == "v2"
	})
	onlyBar := discovery.ResourcePredicateFunc(func(resource *metav1.APIResource) bool {
		return resource.Kind == "Bar"
	})

	foo1 := []*metav1.APIResourceList{{GroupVersion: "foo/v1"}}
	foo2 := []*metav1.APIResourceList{
		{
			GroupVersion: "foo/v1",
			APIResources: []metav1.APIResource{
				{Name: "bar", Kind: "Bar"},
				{Name: "test", Kind: "Test"},
			},
		},
		{
			GroupVersion: "foo/v2",
			APIResources: []metav1.APIResource{
				{Name: "bar", Kind: "Bar"},
				{Name: "test", Kind: "Test"},
			},
		},
		{
			GroupVersion: "foo/v3",
			APIResources: []metav1.APIResource{},
		},
	}

	tests := []struct {
		input             []*metav1.APIResourceList
		pred              discovery.ResourcePredicate
		expectedResources []schema.GroupVersionResource
	}{
		{nil, all, []schema.GroupVersionResource{}},
		{foo1, all, []schema.GroupVersionResource{}},
		{foo2, all, []schema.GroupVersionResource{
			{Group: "foo", Version: "v1", Resource: "bar"},
			{Group: "foo", Version: "v1", Resource: "test"},
			{Group: "foo", Version: "v2", Resource: "bar"},
			{Group: "foo", Version: "v2", Resource: "test"},
		}},
		{foo2, onlyV2, []schema.GroupVersionResource{
			{Group: "foo", Version: "v2", Resource: "bar"},
			{Group: "foo", Version: "v2", Resource: "test"},
		}},
		{foo2, onlyBar, []schema.GroupVersionResource{
			{Group: "foo", Version: "v1", Resource: "bar"},
			{Group: "foo", Version: "v2", Resource: "bar"},
		}},
		{foo2, none, []schema.GroupVersionResource{}},
	}

	for i, test := range tests {
		filtered := discovery.FilteredBy(test.pred, discovery.UnpackAPIResourceLists(test.input...))

		t.Log(filtered)

		if !reflect.DeepEqual(test.expectedResources, discovery.GroupVersionResources(filtered)) {
			t.Errorf("[%d] unexpected group versions: expected=%v, got=%v", i, test.expectedResources, filtered)
		}
	}
}

func stringify(rls []*metav1.APIResourceList) []string {
	result := []string{}
	for _, rl := range rls {
		for _, r := range rl.APIResources {
			result = append(result, rl.GroupVersion+"."+r.Name)
		}
		if len(rl.APIResources) == 0 {
			result = append(result, rl.GroupVersion)
		}
	}
	return result
}
