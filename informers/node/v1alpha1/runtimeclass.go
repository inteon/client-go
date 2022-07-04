/*
Copyright The Kubernetes Authors.

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

// Code generated by informer-gen. DO NOT EDIT.

package v1alpha1

import (
	"context"

	nodev1alpha1 "k8s.io/api/node/v1alpha1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	runtime "k8s.io/apimachinery/pkg/runtime"
	internalinterfaces "k8s.io/client-go/informers/internalinterfaces"
	kubernetes "k8s.io/client-go/kubernetes"
	v1alpha1 "k8s.io/client-go/listers/node/v1alpha1"
	watch "k8s.io/client-go/pkg/watch"
	cache "k8s.io/client-go/tools/cache"
)

// RuntimeClassInformer provides access to a shared informer and lister for
// RuntimeClasses.
type RuntimeClassInformer interface {
	Informer() cache.SharedIndexInformer
	Lister() v1alpha1.RuntimeClassLister
}

type runtimeClassInformer struct {
	factory          internalinterfaces.SharedInformerFactory
	tweakListOptions internalinterfaces.TweakListOptionsFunc
}

// NewRuntimeClassInformer constructs a new informer for RuntimeClass type.
// Always prefer using an informer factory to get a shared informer instead of getting an independent
// one. This reduces memory footprint and number of connections to the server.
func NewRuntimeClassInformer(client kubernetes.Interface, indexers cache.Indexers) cache.SharedIndexInformer {
	return NewFilteredRuntimeClassInformer(client, indexers, nil)
}

// NewFilteredRuntimeClassInformer constructs a new informer for RuntimeClass type.
// Always prefer using an informer factory to get a shared informer instead of getting an independent
// one. This reduces memory footprint and number of connections to the server.
func NewFilteredRuntimeClassInformer(client kubernetes.Interface, indexers cache.Indexers, tweakListOptions internalinterfaces.TweakListOptionsFunc) cache.SharedIndexInformer {
	return cache.NewSharedIndexInformer(
		&cache.ListWatch{
			ListFunc: func(ctx context.Context, options v1.ListOptions) (runtime.Object, error) {
				if tweakListOptions != nil {
					tweakListOptions(&options)
				}
				return client.NodeV1alpha1().RuntimeClasses().List(ctx, options)
			},
			WatchFunc: func(ctx context.Context, options v1.ListOptions) (watch.Watcher, error) {
				if tweakListOptions != nil {
					tweakListOptions(&options)
				}
				return client.NodeV1alpha1().RuntimeClasses().Watch(ctx, options)
			},
		},
		&nodev1alpha1.RuntimeClass{},
		indexers,
	)
}

func (f *runtimeClassInformer) defaultInformer(client kubernetes.Interface) cache.SharedIndexInformer {
	return NewFilteredRuntimeClassInformer(client, cache.Indexers{cache.NamespaceIndex: cache.MetaNamespaceIndexFunc}, f.tweakListOptions)
}

func (f *runtimeClassInformer) Informer() cache.SharedIndexInformer {
	return f.factory.InformerFor(&nodev1alpha1.RuntimeClass{}, f.defaultInformer)
}

func (f *runtimeClassInformer) Lister() v1alpha1.RuntimeClassLister {
	return v1alpha1.NewRuntimeClassLister(f.Informer().GetIndexer())
}
