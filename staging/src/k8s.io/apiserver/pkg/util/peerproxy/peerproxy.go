/*
Copyright 2023 The Kubernetes Authors.

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

package peerproxy

import (
	"net/http"
	"sync"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apiserver/pkg/reconcilers"
	"k8s.io/apiserver/pkg/storageversion"
	kubeinformers "k8s.io/client-go/informers"
	"k8s.io/client-go/tools/cache"
)

// Interface defines how the Unknown Version Proxy filter interacts with the underlying system.
type Interface interface {
	Handle(handler http.Handler, serverId string, s runtime.NegotiatedSerializer, reconciler reconcilers.PeerEndpointLeaseReconciler) http.Handler
	WaitForCacheSync(stopCh <-chan struct{}) error
	HasFinishedSync() bool
}

// New creates a new instance to implement unknown version proxy
func NewPeerProxyHandler(informerFactory kubeinformers.SharedInformerFactory,
	svm storageversion.Manager,
	proxyClientCertFile string,
	proxyClientKeyFile string,
	peerCAFile string) *peerProxyHandler {
	h := &peerProxyHandler{
		name:                  "PeerProxyHandler",
		storageversionManager: svm,
		proxyClientCertFile:   proxyClientCertFile,
		proxyClientKeyFile:    proxyClientKeyFile,
		peerCAFile:            peerCAFile,
		svMap:                 sync.Map{},
	}
	svi := informerFactory.Internal().V1alpha1().StorageVersions()
	h.storageversionInformer = svi.Informer()

	svi.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			h.addSV(obj)
		},
		UpdateFunc: func(oldObj, newObj interface{}) {
			h.updateSV(oldObj, newObj)
		},
		DeleteFunc: func(obj interface{}) {
			h.deleteSV(obj)
		}})
	return h
}
