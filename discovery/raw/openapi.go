package raw

import (
	"strings"

	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/kube-openapi/pkg/util/proto"
)

// groupVersionKindExtensionKey is the key used to lookup the
// GroupVersionKind value for an object definition from the
// definition's "extensions" map.
const groupVersionKindExtensionKey = "x-kubernetes-group-version-kind"

func parseGroupVersionEndpoint(endpoint string) schema.GroupVersion {
	gvObj := schema.GroupVersion{}

	if strings.HasPrefix(endpoint, "api/") {
		gvObj.Version = strings.TrimPrefix(endpoint, "api/")
	} else {
		endpoint = strings.TrimPrefix(endpoint, "apis/")

		if i := strings.Index(endpoint, "/"); i < 0 {
			gvObj.Group = endpoint
		} else {
			gvObj.Group = endpoint[:i]
			gvObj.Version = endpoint[i+1:]
		}
	}

	return gvObj
}

// Get and parse GroupVersionKind from the extension. Returns empty if it doesn't have one.
func parseGroupVersionKindExtension(s proto.Schema) (gvkListResult []schema.GroupVersionKind) {
	extensions := s.GetExtensions()

	// Get the extensions
	gvkExtension, ok := extensions[groupVersionKindExtensionKey]
	if !ok {
		return gvkListResult
	}

	// gvk extension must be a list of at least 1 element.
	gvkList, ok := gvkExtension.([]interface{})
	if !ok {
		return gvkListResult
	}

	for _, gvk := range gvkList {
		var group, version, kind string

		// gvk extension list must be a map with group, version, and
		// kind fields
		if gvkMap, ok := gvk.(map[string]interface{}); ok {
			if group, ok = gvkMap["group"].(string); !ok {
				continue
			}
			if version, ok = gvkMap["version"].(string); !ok {
				continue
			}
			if kind, ok = gvkMap["kind"].(string); !ok {
				continue
			}
		}

		if gvkMap, ok := gvk.(map[interface{}]interface{}); ok {
			if group, ok = gvkMap["group"].(string); !ok {
				continue
			}
			if version, ok = gvkMap["version"].(string); !ok {
				continue
			}
			if kind, ok = gvkMap["kind"].(string); !ok {
				continue
			}
		}

		gvkListResult = append(gvkListResult, schema.GroupVersionKind{
			Group:   group,
			Version: version,
			Kind:    kind,
		})
	}

	return gvkListResult
}
