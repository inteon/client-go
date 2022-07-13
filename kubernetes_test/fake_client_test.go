/*
Copyright 2020 The Kubernetes Authors.

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

package kubernetes

import (
	"context"
	"strings"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/kubernetes/fake"
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

// This test proves that the kube fake client does not return GVKs.  This is consistent with actual client (see tests below)
// and should not be changed unless the decoding behavior and somehow literal creation (`&corev1.ConfigMap{}`) behavior change.
func Test_ConfigMapFakeClient(t *testing.T) {
	fakeKubeClient := fake.NewSimpleClientset(&corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Namespace: "foo-ns", Name: "foo-name"}})
	cm, err := fakeKubeClient.CoreV1().ConfigMaps("foo-ns").Get(context.TODO(), "foo-name", metav1.GetOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if cm.GetObjectKind().GroupVersionKind() != (schema.GroupVersionKind{Version: "v1", Kind: "ConfigMap"}) {
		t.Fatal(cm.GetObjectKind().GroupVersionKind())
	}
	cmList, err := fakeKubeClient.CoreV1().ConfigMaps("foo-ns").List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if cmList.GetObjectKind().GroupVersionKind() != (schema.GroupVersionKind{Version: "v1", Kind: "ConfigMapList"}) {
		t.Fatal(cmList.GetObjectKind().GroupVersionKind())
	}
}

// This test checks decoding behavior for the actual client to ensure the fake client (tested above) is consistent.
func TestGetDecoding(t *testing.T) {
	negotiator := newNegotiator(t)

	// this the duplication of logic from the real Get API for configmaps.  This will prove that the generated client will not return a GVK
	decoder, err := negotiator.Decoder(string(rest.JsonMediaType), nil)
	if err != nil {
		t.Fatal(err)
	}
	//	decoder := scheme.Codecs.WithoutConversion().DecoderToVersion(info.Serializer, corev1.SchemeGroupVersion)

	body := []byte(`{"apiVersion": "v1", "kind": "ConfigMap", "metadata":{"Namespace":"foo","Name":"bar"}}`)
	obj := &corev1.ConfigMap{}
	out, _, err := decoder.Decode(body, nil, obj)
	if err != nil || out != obj {
		t.Fatal(err)
	}

	if obj.GetObjectKind().GroupVersionKind() != (schema.GroupVersionKind{Version: "v1", Kind: "ConfigMap"}) {
		t.Fatal(obj.GetObjectKind().GroupVersionKind())
	}
}

// This test checks decoding behavior for the actual client to ensure the fake client (tested above) is consistent.
func TestListDecoding(t *testing.T) {
	negotiator := newNegotiator(t)

	// this the duplication of logic from the real Get API for configmaps.  This will prove that the generated client will not return a GVK
	mediaTypes := negotiator.AcceptContentTypes(schema.GroupVersionKind{Version: "v1", Kind: "Configmap"})
	mediaType, _, _ := strings.Cut(mediaTypes, ",")
	decoder, err := negotiator.Decoder(mediaType, nil)
	if err != nil {
		t.Fatal(err)
	}

	body := []byte(`{"apiVersion": "v1", "kind": "ConfigMapList", "items":[]}`)
	obj := &corev1.ConfigMapList{}
	out, _, err := decoder.Decode(body, nil, obj)
	if err != nil || out != obj {
		t.Fatal(err)
	}

	if obj.GetObjectKind().GroupVersionKind() != (schema.GroupVersionKind{Version: "v1", Kind: "ConfigMapList"}) {
		t.Fatal(obj.GetObjectKind().GroupVersionKind())
	}
}
