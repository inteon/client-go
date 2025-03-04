/*
Copyright 2018 The Kubernetes Authors.

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

package kubernetes_test

import (
	"bytes"
	"context"
	"io/ioutil"
	"net/http"
	"testing"

	"github.com/davecgh/go-spew/spew"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	manualfake "k8s.io/client-go/rest/fake"
)

func TestListTimeout(t *testing.T) {
	fakeClient := &manualfake.RESTClient{
		Client: manualfake.CreateHTTPClient(func(req *http.Request) (*http.Response, error) {
			if req.URL.Query().Get("timeout") != "21s" {
				t.Fatal(spew.Sdump(req.URL.Query()))
			}
			return &http.Response{StatusCode: http.StatusNotFound, Body: ioutil.NopCloser(&bytes.Buffer{})}, nil
		}),
	}

	restClient, err := rest.Config{}.Build()
	if err != nil {
		t.Fatal(err)
	}
	restClient.Client = fakeClient.Client
	realClient := kubernetes.New(restClient)

	timeout := int64(21)
	realClient.AppsV1().DaemonSets("").List(context.TODO(), metav1.ListOptions{TimeoutSeconds: &timeout})
	realClient.AppsV1().DaemonSets("").Watch(context.TODO(), metav1.ListOptions{TimeoutSeconds: &timeout})
}
