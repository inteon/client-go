/*
Copyright 2022 The Kubernetes Authors.

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
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	http_client "github.com/go418/http-client"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
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

func TestClientUserAgent(t *testing.T) {
	tests := []struct {
		name      string
		userAgent string
		expect    string
	}{
		{
			name:   "empty",
			expect: rest.DefaultKubernetesUserAgent(),
		},
		{
			name:      "custom",
			userAgent: "test-agent",
			expect:    "test-agent",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ts := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				userAgent := r.Header.Get("User-Agent")
				if userAgent != tc.expect {
					t.Errorf("User Agent expected: %s got: %s", tc.expect, userAgent)
					http.Error(w, "Unexpected user agent", http.StatusBadRequest)
					return
				}
				w.Header().Set("Content-Type", "application/json")
				w.Write([]byte("{}"))
			}))
			ts.Start()
			defer ts.Close()

			restClient, err := rest.Config{
				Host:       ts.URL,
				Negotiator: newNegotiator(t),
			}.Build(
				http_client.UserAgent(tc.userAgent),
			)
			if err != nil {
				t.Fatalf("failed to create REST client: %v", err)
			}
			client := kubernetes.New(restClient)

			_, err = client.CoreV1().Pods("").List(context.TODO(), metav1.ListOptions{})
			if err != nil {
				t.Error(err)
			}
			_, err = client.CoreV1().Secrets("").List(context.TODO(), metav1.ListOptions{})
			if err != nil {
				t.Error(err)
			}
		})
	}

}
