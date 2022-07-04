package rest

import (
	"strings"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/runtime/serializer/json"
	"k8s.io/apimachinery/pkg/runtime/serializer/protobuf"
	"k8s.io/apimachinery/pkg/runtime/serializer/versioning"
)

type MediaType string

const (
	ProtobufMediaType MediaType = "application/vnd.kubernetes.protobuf"
	YamlMediaType     MediaType = "application/yaml"
	JsonMediaType     MediaType = "application/json"
)

type recognizer interface {
	Recognizes(gvk schema.GroupVersionKind) bool
}

type SerializerInfo struct {
	recognizer
	MediaType  string
	Serializer runtime.Serializer
	Framer     runtime.Framer
}

type resourceSerializerNegotiator struct {
	mediaTypePreference []string
	supportedMediaTypes map[string]SerializerInfo
	parameterCodec      runtime.ParameterCodec
}

var _ SerializerNegotiator = &resourceSerializerNegotiator{}

type schemeGroupVersioner struct {
	scheme interface {
		recognizer
		Name() string
	}
}

var _ runtime.GroupVersioner = schemeGroupVersioner{}

// KindForGroupVersionKinds returns false for any input.
func (s schemeGroupVersioner) KindForGroupVersionKinds(kinds []schema.GroupVersionKind) (schema.GroupVersionKind, bool) {
	for _, kind := range kinds {
		if s.scheme.Recognizes(kind) {
			return kind, true
		}
	}
	return schema.GroupVersionKind{}, false
}

// Identifier implements GroupVersioner interface.
func (s schemeGroupVersioner) Identifier() string {
	return "schemeGroupVersioner(" + s.scheme.Name() + ")"
}

func NewSerializerNegotiator(scheme *runtime.Scheme, conversion bool) SerializerNegotiator {
	if conversion {
		panic("conversion is not supported!")
	}

	universalScheme := newUniversalScheme(scheme)

	jsonSerializer := json.NewSerializerWithOptions(json.DefaultMetaFactory, universalScheme, universalScheme, json.SerializerOptions{
		Yaml:   false,
		Pretty: false,
		Strict: false,
	})
	jsonCodec := versioning.NewCodec(jsonSerializer, jsonSerializer, universalScheme, universalScheme, universalScheme, universalScheme, schemeGroupVersioner{scheme: universalScheme}, schemeGroupVersioner{scheme: scheme}, universalScheme.Name()+":"+string(JsonMediaType))

	yamlSerializer := json.NewSerializerWithOptions(json.DefaultMetaFactory, universalScheme, universalScheme, json.SerializerOptions{
		Yaml:   true,
		Pretty: false,
		Strict: false,
	})
	yamlCodec := versioning.NewCodec(yamlSerializer, yamlSerializer, universalScheme, universalScheme, universalScheme, universalScheme, schemeGroupVersioner{scheme: universalScheme}, schemeGroupVersioner{scheme: scheme}, universalScheme.Name()+":"+string(YamlMediaType))

	protoSerializer := protobuf.NewSerializer(scheme, scheme)
	protoCodec := versioning.NewCodec(protoSerializer, protoSerializer, scheme, scheme, scheme, scheme, schemeGroupVersioner{scheme: scheme}, schemeGroupVersioner{scheme: scheme}, scheme.Name()+":"+string(ProtobufMediaType))

	return &resourceSerializerNegotiator{
		parameterCodec: runtime.NewParameterCodec(scheme),
		mediaTypePreference: []string{
			string(ProtobufMediaType),
			string(JsonMediaType),
			// string(YamlMediaType),
		},
		supportedMediaTypes: map[string]SerializerInfo{
			string(ProtobufMediaType): {
				recognizer: scheme,
				MediaType:  string(ProtobufMediaType),
				Serializer: protoCodec,
				Framer:     protobuf.LengthDelimitedFramer,
			},
			string(JsonMediaType): {
				recognizer: universalScheme,
				MediaType:  string(JsonMediaType),
				Serializer: jsonCodec,
				Framer:     json.Framer,
			},
			string(YamlMediaType): {
				recognizer: universalScheme,
				MediaType:  string(YamlMediaType),
				Serializer: yamlCodec,
				Framer:     json.YAMLFramer,
			},
		},
	}
}

func (s *resourceSerializerNegotiator) AcceptContentTypes(gvk schema.GroupVersionKind) string {
	// select all content types that supports decoding/ recognizes the given gvk
	contentTypes := []string{}
	for _, mediaType := range s.mediaTypePreference {
		if s.supportedMediaTypes[mediaType].Recognizes(gvk) {
			contentTypes = append(contentTypes, mediaType)
		}
	}
	if len(contentTypes) == 0 {
		return string(JsonMediaType) // use JSON as default content encoding
	} else {
		return strings.Join(contentTypes, ", ")
	}
}

func (s *resourceSerializerNegotiator) ContentType(gvk schema.GroupVersionKind) string {
	// select first (following preference order) content type
	// that supports encoding/ recognizes the given gvk
	for _, mediaType := range s.mediaTypePreference {
		if s.supportedMediaTypes[mediaType].Recognizes(gvk) {
			return mediaType
		}
	}
	return string(JsonMediaType) // use JSON as default content encoding
}

func (s *resourceSerializerNegotiator) Encoder(contentType string, params map[string]string) (runtime.Encoder, error) {
	if info, ok := s.supportedMediaTypes[contentType]; !ok {
		return nil, runtime.NegotiateError{ContentType: contentType}
	} else {
		return info.Serializer, nil
	}
}

func (s *resourceSerializerNegotiator) Decoder(contentType string, params map[string]string) (runtime.Decoder, error) {
	if info, ok := s.supportedMediaTypes[contentType]; !ok {
		return nil, runtime.NegotiateError{ContentType: contentType}
	} else {
		return info.Serializer, nil
	}
}

func (s *resourceSerializerNegotiator) StreamEncoder(contentType string, params map[string]string) (runtime.Encoder, runtime.Serializer, runtime.Framer, error) {
	if info, ok := s.supportedMediaTypes[contentType]; !ok {
		return nil, nil, nil, runtime.NegotiateError{ContentType: contentType, Stream: true}
	} else if info.Framer == nil {
		return nil, nil, nil, runtime.NegotiateError{ContentType: info.MediaType, Stream: true}
	} else {
		return info.Serializer, info.Serializer, info.Framer, nil
	}
}

func (s *resourceSerializerNegotiator) StreamDecoder(contentType string, params map[string]string) (runtime.Decoder, runtime.Serializer, runtime.Framer, error) {
	if info, ok := s.supportedMediaTypes[contentType]; !ok {
		return nil, nil, nil, runtime.NegotiateError{ContentType: contentType, Stream: true}
	} else if info.Framer == nil {
		return nil, nil, nil, runtime.NegotiateError{ContentType: info.MediaType, Stream: true}
	} else {
		return info.Serializer, info.Serializer, info.Framer, nil
	}
}

func (s *resourceSerializerNegotiator) ParameterCodec() runtime.ParameterCodec {
	return s.parameterCodec
}
