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

package filters

import (
	"net/http"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apiserver/pkg/reconcilers"
	"k8s.io/apiserver/pkg/util/peerproxy"
)

// WithPeerProxy creates a handler to proxy request to peer kube-apiservers if the request can not be
// served locally due to version skew or if the requested API is disabled in local apiserver
func WithPeerProxy(handler http.Handler, serverId string, ppi peerproxy.Interface, s runtime.NegotiatedSerializer, r reconcilers.PeerEndpointLeaseReconciler) http.Handler {
	if ppi == nil {
		return handler
	}
	return ppi.Handle(handler, serverId, s, r)
}
