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

package v1beta1

import (
	"context"

	admissionregistrationv1beta1 "k8s.io/api/admissionregistration/v1beta1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	runtime "k8s.io/apimachinery/pkg/runtime"
	internalinterfaces "k8s.io/client-go/informers/internalinterfaces"
	kubernetes "k8s.io/client-go/kubernetes"
	v1beta1 "k8s.io/client-go/listers/admissionregistration/v1beta1"
	watch "k8s.io/client-go/pkg/watch"
	cache "k8s.io/client-go/tools/cache"
)

// MutatingWebhookConfigurationInformer provides access to a shared informer and lister for
// MutatingWebhookConfigurations.
type MutatingWebhookConfigurationInformer interface {
	Informer() cache.SharedIndexInformer
	Lister() v1beta1.MutatingWebhookConfigurationLister
}

type mutatingWebhookConfigurationInformer struct {
	factory          internalinterfaces.SharedInformerFactory
	tweakListOptions internalinterfaces.TweakListOptionsFunc
}

// NewMutatingWebhookConfigurationInformer constructs a new informer for MutatingWebhookConfiguration type.
// Always prefer using an informer factory to get a shared informer instead of getting an independent
// one. This reduces memory footprint and number of connections to the server.
func NewMutatingWebhookConfigurationInformer(client kubernetes.Interface, indexers cache.Indexers) cache.SharedIndexInformer {
	return NewFilteredMutatingWebhookConfigurationInformer(client, indexers, nil)
}

// NewFilteredMutatingWebhookConfigurationInformer constructs a new informer for MutatingWebhookConfiguration type.
// Always prefer using an informer factory to get a shared informer instead of getting an independent
// one. This reduces memory footprint and number of connections to the server.
func NewFilteredMutatingWebhookConfigurationInformer(client kubernetes.Interface, indexers cache.Indexers, tweakListOptions internalinterfaces.TweakListOptionsFunc) cache.SharedIndexInformer {
	return cache.NewSharedIndexInformer(
		&cache.ListWatch{
			ListFunc: func(ctx context.Context, options v1.ListOptions) (runtime.Object, error) {
				if tweakListOptions != nil {
					tweakListOptions(&options)
				}
				return client.AdmissionregistrationV1beta1().MutatingWebhookConfigurations().List(ctx, options)
			},
			WatchFunc: func(ctx context.Context, options v1.ListOptions) (watch.Watcher, error) {
				if tweakListOptions != nil {
					tweakListOptions(&options)
				}
				return client.AdmissionregistrationV1beta1().MutatingWebhookConfigurations().Watch(ctx, options)
			},
		},
		&admissionregistrationv1beta1.MutatingWebhookConfiguration{},
		indexers,
	)
}

func (f *mutatingWebhookConfigurationInformer) defaultInformer(client kubernetes.Interface) cache.SharedIndexInformer {
	return NewFilteredMutatingWebhookConfigurationInformer(client, cache.Indexers{cache.NamespaceIndex: cache.MetaNamespaceIndexFunc}, f.tweakListOptions)
}

func (f *mutatingWebhookConfigurationInformer) Informer() cache.SharedIndexInformer {
	return f.factory.InformerFor(&admissionregistrationv1beta1.MutatingWebhookConfiguration{}, f.defaultInformer)
}

func (f *mutatingWebhookConfigurationInformer) Lister() v1beta1.MutatingWebhookConfigurationLister {
	return v1beta1.NewMutatingWebhookConfigurationLister(f.Informer().GetIndexer())
}
