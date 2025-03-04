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

package v1

import (
	"context"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	runtime "k8s.io/apimachinery/pkg/runtime"
	internalinterfaces "k8s.io/client-go/informers/internalinterfaces"
	kubernetes "k8s.io/client-go/kubernetes"
	v1 "k8s.io/client-go/listers/core/v1"
	watch "k8s.io/client-go/pkg/watch"
	cache "k8s.io/client-go/tools/cache"
)

// ReplicationControllerInformer provides access to a shared informer and lister for
// ReplicationControllers.
type ReplicationControllerInformer interface {
	Informer() cache.SharedIndexInformer
	Lister() v1.ReplicationControllerLister
}

type replicationControllerInformer struct {
	factory          internalinterfaces.SharedInformerFactory
	tweakListOptions internalinterfaces.TweakListOptionsFunc
	namespace        string
}

// NewReplicationControllerInformer constructs a new informer for ReplicationController type.
// Always prefer using an informer factory to get a shared informer instead of getting an independent
// one. This reduces memory footprint and number of connections to the server.
func NewReplicationControllerInformer(client kubernetes.Interface, namespace string, indexers cache.Indexers) cache.SharedIndexInformer {
	return NewFilteredReplicationControllerInformer(client, namespace, indexers, nil)
}

// NewFilteredReplicationControllerInformer constructs a new informer for ReplicationController type.
// Always prefer using an informer factory to get a shared informer instead of getting an independent
// one. This reduces memory footprint and number of connections to the server.
func NewFilteredReplicationControllerInformer(client kubernetes.Interface, namespace string, indexers cache.Indexers, tweakListOptions internalinterfaces.TweakListOptionsFunc) cache.SharedIndexInformer {
	return cache.NewSharedIndexInformer(
		&cache.ListWatch{
			ListFunc: func(ctx context.Context, options metav1.ListOptions) (runtime.Object, error) {
				if tweakListOptions != nil {
					tweakListOptions(&options)
				}
				return client.CoreV1().ReplicationControllers(namespace).List(ctx, options)
			},
			WatchFunc: func(ctx context.Context, options metav1.ListOptions) (watch.Watcher, error) {
				if tweakListOptions != nil {
					tweakListOptions(&options)
				}
				return client.CoreV1().ReplicationControllers(namespace).Watch(ctx, options)
			},
		},
		&corev1.ReplicationController{},
		indexers,
	)
}

func (f *replicationControllerInformer) defaultInformer(client kubernetes.Interface) cache.SharedIndexInformer {
	return NewFilteredReplicationControllerInformer(client, f.namespace, cache.Indexers{cache.NamespaceIndex: cache.MetaNamespaceIndexFunc}, f.tweakListOptions)
}

func (f *replicationControllerInformer) Informer() cache.SharedIndexInformer {
	return f.factory.InformerFor(&corev1.ReplicationController{}, f.defaultInformer)
}

func (f *replicationControllerInformer) Lister() v1.ReplicationControllerLister {
	return v1.NewReplicationControllerLister(f.Informer().GetIndexer())
}
