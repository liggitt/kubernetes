/*
Copyright 2025 The Kubernetes Authors.

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

package proxy

import (
	"fmt"

	discoveryv1 "k8s.io/api/discovery/v1"
	"k8s.io/apimachinery/pkg/labels"
	discoveryv1informer "k8s.io/client-go/informers/discovery/v1"
	discoveryv1lister "k8s.io/client-go/listers/discovery/v1"
	"k8s.io/client-go/tools/cache"
)

type EndpointSliceGetter interface {
	GetEndpointSlices(namespaceName, serviceName string) ([]*discoveryv1.EndpointSlice, error)
}

const indexKey = "namespaceName_serviceName"

// NewEndpointSliceIndexerGetter ensures a namespace/service-name indexer is added to the sliceInformer, and returns a slice getter function.
// The sliceInformer must not yet be started when this is called.
func NewEndpointSliceIndexerGetter(sliceInformer discoveryv1informer.EndpointSliceInformer) (EndpointSliceGetter, error) {
	// Install the indexer if not yet installed
	if _, exists := sliceInformer.Informer().GetIndexer().GetIndexers()[indexKey]; !exists {
		if err := sliceInformer.Informer().AddIndexers(map[string]cache.IndexFunc{indexKey: func(obj any) ([]string, error) {
			ep, ok := obj.(*discoveryv1.EndpointSlice)
			if !ok {
				return nil, fmt.Errorf("expected *discoveryv1.EndpointSlice, got %T", obj)
			}
			return []string{ep.Namespace + "/" + ep.Labels[discoveryv1.LabelServiceName]}, nil
		}}); err != nil {
			return nil, err
		}
	}

	return &endpointSliceIndexerGetter{indexer: sliceInformer.Informer().GetIndexer()}, nil
}

type endpointSliceIndexerGetter struct {
	indexer cache.Indexer
}

func (e *endpointSliceIndexerGetter) GetEndpointSlices(namespaceName, serviceName string) ([]*discoveryv1.EndpointSlice, error) {
	objs, err := e.indexer.ByIndex(indexKey, namespaceName+"/"+serviceName)
	if err != nil {
		return nil, err
	}
	eps := make([]*discoveryv1.EndpointSlice, 0, len(objs))
	for _, obj := range objs {
		ep, ok := obj.(*discoveryv1.EndpointSlice)
		if !ok {
			return nil, fmt.Errorf("expected *discoveryv1.EndpointSlice, got %T", obj)
		}
		eps = append(eps, ep)
	}
	return eps, nil
}

// NewEndpointSliceListerGetter returns an EndpointSliceGetter that uses a lister to do a full selection on every lookup.
func NewEndpointSliceListerGetter(sliceLister discoveryv1lister.EndpointSliceLister) (EndpointSliceGetter, error) {
	return &endpointSliceListerGetter{lister: sliceLister}, nil
}

type endpointSliceListerGetter struct {
	lister discoveryv1lister.EndpointSliceLister
}

func (e *endpointSliceListerGetter) GetEndpointSlices(namespaceName, serviceName string) ([]*discoveryv1.EndpointSlice, error) {
	return e.lister.EndpointSlices(namespaceName).List(labels.SelectorFromSet(labels.Set{discoveryv1.LabelServiceName: serviceName}))
}
