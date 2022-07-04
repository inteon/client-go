/*
Copyright 2016 The Kubernetes Authors.

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
	"fmt"

	"golang.org/x/exp/maps"
	"golang.org/x/sync/errgroup"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/sets"
	apimachineryversion "k8s.io/apimachinery/pkg/version"
)

// IsResourceEnabled queries the server to determine if the resource specified is present on the server.
// This is particularly helpful when writing a controller or an e2e test that requires a particular resource to function.
func IsResourceEnabled(ctx context.Context, client DiscoveryInterface, resourceToCheck schema.GroupVersionResource) (bool, error) {
	if _, ok := client.GvrToAPIResourceFromCache(resourceToCheck); ok {
		return true, nil
	}

	if _, err := client.GvrToAPIResource(ctx, resourceToCheck); err != nil {
		return false, err
	}

	return true, nil
}

// MatchesServerVersion queries the server to compares the build version
// (git hash) of the client with the server's build version. It returns an error
// if it failed to contact the server or if the versions are not an exact match.
func MatchesServerVersion(ctx context.Context, clientVersion apimachineryversion.Info, client DiscoveryInterface) error {
	sVer, err := client.ServerVersion(ctx)
	if err != nil {
		return fmt.Errorf("couldn't read version from server: %v", err)
	}

	// GitVersion includes GitCommit and GitTreeState, but best to be safe?
	if clientVersion.GitVersion != sVer.GitVersion || clientVersion.GitCommit != sVer.GitCommit || clientVersion.GitTreeState != sVer.GitTreeState {
		return fmt.Errorf("server version (%#v) differs from client version (%#v)", sVer, clientVersion)
	}

	return nil
}

// ServerSupportsVersion returns an error if the server doesn't have the required version
func ServerSupportsVersion(ctx context.Context, client DiscoveryInterface, requiredGV schema.GroupVersion) error {
	groups, err := client.ApiGroupVersions(ctx)
	if err != nil {
		// This is almost always a connection error, and higher level code should treat this as a generic error,
		// not a negotiation specific error.
		return err
	}

	for _, v := range groups {
		if v.GroupVersion == requiredGV {
			return nil
		}
	}

	// If the server supports no versions, then we should pretend it has the version because of old servers.
	// This can happen because discovery fails due to 403 Forbidden errors
	if len(groups) == 0 {
		return nil
	}

	return fmt.Errorf("server does not support API version %q", requiredGV)
}

// GroupVersionResources converts APIResources to the GroupVersionResources.
func GroupVersionResources(apiResources []metav1.APIResource) []schema.GroupVersionResource {
	gvrs := make([]schema.GroupVersionResource, 0, len(apiResources))
	for _, rl := range apiResources {
		gvrs = append(gvrs, schema.GroupVersionResource{
			Group:    rl.Group,
			Version:  rl.Version,
			Resource: rl.Name,
		})
	}
	return gvrs
}

// FilteredBy filters by the given predicate. Empty APIResources are dropped.
func FilteredBy(pred ResourcePredicate, apiResources []metav1.APIResource) []metav1.APIResource {
	result := []metav1.APIResource{}
	for _, rl := range apiResources {
		if pred.Match(&rl) {
			result = append(result, rl)
		}
	}
	return result
}

// ResourcePredicate has a method to check if a resource matches a given condition.
type ResourcePredicate interface {
	Match(resource *metav1.APIResource) bool
}

// ResourcePredicateFunc returns true if it matches a resource based on a custom condition.
type ResourcePredicateFunc func(resource *metav1.APIResource) bool

// Match is a wrapper around ResourcePredicateFunc.
func (fn ResourcePredicateFunc) Match(resource *metav1.APIResource) bool {
	return fn(resource)
}

// SupportsAllVerbs is a predicate matching a resource iff all given verbs are supported.
type SupportsAllVerbs struct {
	Verbs []string
}

// Match checks if a resource contains all the given verbs.
func (p SupportsAllVerbs) Match(resource *metav1.APIResource) bool {
	return sets.NewString([]string(resource.Verbs)...).HasAll(p.Verbs...)
}

func UnpackAPIResourceLists(lists ...*metav1.APIResourceList) []metav1.APIResource {
	count := 0
	for _, list := range lists {
		count += len(list.APIResources)
	}

	resources := make([]metav1.APIResource, 0, count)
	for _, list := range lists {
		gv, gverr := schema.ParseGroupVersion(list.GroupVersion)

		for _, res := range list.APIResources {
			if len(res.Group) == 0 && len(res.Version) == 0 && gverr == nil {
				res.Group = gv.Group
				res.Version = gv.Version
			}

			resources = append(resources, res)
		}
	}
	return resources
}

func PreferredServerResources(ctx context.Context, client *DiscoveryClient) ([]metav1.APIResource, error) {
	var err error
	groupVersions, ok := client.ApiGroupVersionsFromCache()
	if !ok {
		groupVersions, err = client.ApiGroupVersions(ctx)
		if err != nil {
			return nil, err
		}
	}

	resourceLists := make([][]metav1.APIResource, len(groupVersions))
	group, gctx := errgroup.WithContext(ctx)

	for i, gv := range groupVersions {
		localIndex := i
		localGv := gv

		if resources, ok := client.ApiResourcesFromCache(localGv.GroupVersion); ok {
			resourceLists[localIndex] = resources
		} else {
			group.Go(func() error {
				if resources, err := client.ApiResources(gctx, localGv.GroupVersion); err != nil {
					return err
				} else {
					resourceLists[localIndex] = resources
				}
				return nil
			})
		}
	}

	if err := group.Wait(); err != nil {
		return nil, err
	}

	preferred := map[schema.GroupKind]metav1.APIResource{}
	for i, gv := range groupVersions {
		for _, resource := range resourceLists[i] {
			gk := schema.GroupKind{
				Group: resource.Group,
				Kind:  resource.Kind,
			}

			if _, ok := preferred[gk]; !ok || gv.Preferred {
				preferred[gk] = resource
			}
		}
	}

	return maps.Values(preferred), nil
}

func GroupVersions(apiGroupVersions []ApiGroupVersion) []schema.GroupVersion {
	groupVersions := make([]schema.GroupVersion, 0, len(apiGroupVersions))
	for _, apiGv := range apiGroupVersions {
		groupVersions = append(groupVersions, apiGv.GroupVersion)
	}
	return groupVersions
}
