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

package rest

import (
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

type SerializerNegotiator interface {
	// The content types we want the API server to return for the given resource kind.
	AcceptContentTypes(gvk schema.GroupVersionKind) string
	// The content type to use to encode a resource when sending to the API server.
	ContentType(gvk schema.GroupVersionKind) string

	ParameterCodec() runtime.ParameterCodec

	Encoder(contentType string, params map[string]string) (runtime.Encoder, error)
	Decoder(contentType string, params map[string]string) (runtime.Decoder, error)
	StreamEncoder(contentType string, params map[string]string) (runtime.Encoder, runtime.Serializer, runtime.Framer, error)
	StreamDecoder(contentType string, params map[string]string) (runtime.Decoder, runtime.Serializer, runtime.Framer, error)
}
