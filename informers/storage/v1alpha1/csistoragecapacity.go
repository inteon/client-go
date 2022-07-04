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

	storagev1alpha1 "k8s.io/api/storage/v1alpha1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	runtime "k8s.io/apimachinery/pkg/runtime"
	internalinterfaces "k8s.io/client-go/informers/internalinterfaces"
	kubernetes "k8s.io/client-go/kubernetes"
	v1alpha1 "k8s.io/client-go/listers/storage/v1alpha1"
	watch "k8s.io/client-go/pkg/watch"
	cache "k8s.io/client-go/tools/cache"
)

// CSIStorageCapacityInformer provides access to a shared informer and lister for
// CSIStorageCapacities.
type CSIStorageCapacityInformer interface {
	Informer() cache.SharedIndexInformer
	Lister() v1alpha1.CSIStorageCapacityLister
}

type cSIStorageCapacityInformer struct {
	factory          internalinterfaces.SharedInformerFactory
	tweakListOptions internalinterfaces.TweakListOptionsFunc
	namespace        string
}

// NewCSIStorageCapacityInformer constructs a new informer for CSIStorageCapacity type.
// Always prefer using an informer factory to get a shared informer instead of getting an independent
// one. This reduces memory footprint and number of connections to the server.
func NewCSIStorageCapacityInformer(client kubernetes.Interface, namespace string, indexers cache.Indexers) cache.SharedIndexInformer {
	return NewFilteredCSIStorageCapacityInformer(client, namespace, indexers, nil)
}

// NewFilteredCSIStorageCapacityInformer constructs a new informer for CSIStorageCapacity type.
// Always prefer using an informer factory to get a shared informer instead of getting an independent
// one. This reduces memory footprint and number of connections to the server.
func NewFilteredCSIStorageCapacityInformer(client kubernetes.Interface, namespace string, indexers cache.Indexers, tweakListOptions internalinterfaces.TweakListOptionsFunc) cache.SharedIndexInformer {
	return cache.NewSharedIndexInformer(
		&cache.ListWatch{
			ListFunc: func(ctx context.Context, options v1.ListOptions) (runtime.Object, error) {
				if tweakListOptions != nil {
					tweakListOptions(&options)
				}
				return client.StorageV1alpha1().CSIStorageCapacities(namespace).List(ctx, options)
			},
			WatchFunc: func(ctx context.Context, options v1.ListOptions) (watch.Watcher, error) {
				if tweakListOptions != nil {
					tweakListOptions(&options)
				}
				return client.StorageV1alpha1().CSIStorageCapacities(namespace).Watch(ctx, options)
			},
		},
		&storagev1alpha1.CSIStorageCapacity{},
		indexers,
	)
}

func (f *cSIStorageCapacityInformer) defaultInformer(client kubernetes.Interface) cache.SharedIndexInformer {
	return NewFilteredCSIStorageCapacityInformer(client, f.namespace, cache.Indexers{cache.NamespaceIndex: cache.MetaNamespaceIndexFunc}, f.tweakListOptions)
}

func (f *cSIStorageCapacityInformer) Informer() cache.SharedIndexInformer {
	return f.factory.InformerFor(&storagev1alpha1.CSIStorageCapacity{}, f.defaultInformer)
}

func (f *cSIStorageCapacityInformer) Lister() v1alpha1.CSIStorageCapacityLister {
	return v1alpha1.NewCSIStorageCapacityLister(f.Informer().GetIndexer())
}
