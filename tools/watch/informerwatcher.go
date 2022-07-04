/*
Copyright 2017 The Kubernetes Authors.

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

package watch

import (
	"context"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/pkg/concurrent"
	"k8s.io/client-go/pkg/watch"
	"k8s.io/client-go/tools/cache"
)

// NewIndexerInformerWatcher will create an IndexerInformer and wrap it into watch.Interface
// so you can use it anywhere where you'd have used a regular Watcher returned from Watch method.
// it also returns a channel you can use to wait for the informers to fully shutdown.
func NewIndexerInformerWatcher(lw cache.ListerWatcher, objType runtime.Object) (cache.Indexer, cache.Controller, watch.Watcher) {
	w := watch.NewBoundedWatcher()

	indexer, informer := cache.NewIndexerInformer(lw, objType, cache.ResourceEventHandlerFuncs{
		AddFunc: func(ctx context.Context, obj interface{}) {
			w.Add(ctx, obj.(runtime.Object))
		},
		UpdateFunc: func(ctx context.Context, old, new interface{}) {
			w.Modify(ctx, new.(runtime.Object))
		},
		DeleteFunc: func(ctx context.Context, obj interface{}) {
			staleObj, stale := obj.(cache.DeletedFinalStateUnknown)
			if stale {
				// We have no means of passing the additional information down using
				// watch API based on watch.Event but the caller can filter such
				// objects by checking if metadata.deletionTimestamp is set
				obj = staleObj.Obj
			}

			w.Delete(ctx, obj.(runtime.Object))
		},
	}, cache.Indexers{})

	return indexer, informer, watch.FuncWatcher(func(ctx context.Context, outStream chan<- watch.Event) error {
		ctx, cancel := concurrent.Complete(ctx, func(ctx context.Context) {
			informer.Run(ctx)
		})
		defer cancel()

		return w.Listen(ctx, outStream)
	})
}
