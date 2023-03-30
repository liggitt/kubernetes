/*
Copyright 2019 The Kubernetes Authors.

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

package unknownversionproxy

import (
	"errors"
	"fmt"
	"net/http"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apiserver/pkg/endpoints/handlers/responsewriters"
	apirequest "k8s.io/apiserver/pkg/endpoints/request"
	kubeinformers "k8s.io/client-go/informers"
	apiserverinternallister "k8s.io/client-go/listers/apiserverinternal/v1alpha1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/klog/v2"
)

const ConfigConsumerAsFieldManager = "api-server-proxy-v1"

// Interface defines how the API Priority and Fairness filter interacts with the underlying system.
type Interface interface {
	Handle(handler http.Handler, localAPIServerId string, s runtime.NegotiatedSerializer )  http.Handler
}

// New creates a new instance to implement API server proxy
func New(
	informerFactory kubeinformers.SharedInformerFactory,
) Interface {
	return NewTestable(TestableConfig{
		Name:                   "Controller",
		InformerFactory:        informerFactory,
	})
}

// TestableConfig carries the parameters to an implementation that is testable
type TestableConfig struct {
	// Name of the controller
	Name string

	// InformerFactory to use in building the controller
	InformerFactory kubeinformers.SharedInformerFactory
}

// NewTestable is extra flexible to facilitate testing
func NewTestable(config TestableConfig) Interface {
	return newTestableController(config)
}

func (cfgCtlr *configController) Handle(handler http.Handler, localAPIServerId string, s runtime.NegotiatedSerializer ) http.Handler {
	if cfgCtlr.svLister == nil {
		klog.Warningf("api server interoperability proxy support not found, skipping")
		return handler
	}
	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		gv := schema.GroupVersion{Group: "unknown", Version: "unknown"}
		requestInfo, ok := apirequest.RequestInfoFrom(req.Context())
		if ok {
			gv.Group = requestInfo.APIGroup
			gv.Version = requestInfo.APIVersion
		}

		storageVersions, err := cfgCtlr.svLister.Get(fmt.Sprintf("%s.%s", requestInfo.APIGroup, requestInfo.Resource))
		if err != nil {
			utilruntime.HandleError(fmt.Errorf("failed to serve request: No StorageVersion found for the requested GVR: %v", err))
			responsewriters.InternalError(w, req, errors.New("failed to create audit event"))
			return
		}

		var serviceableBy []string

		for _, sv := range storageVersions.Status.StorageVersions {
			for _, version := range sv.DecodableVersions {
				if version == requestInfo.APIVersion {
					// found the gvr locally, pass handler as it is
					if sv.APIServerID == localAPIServerId {
						handler.ServeHTTP(w, req)
						return
					}
					serviceableBy = append(serviceableBy, sv.APIServerID)
				}
			}
		}

		if len(serviceableBy) == 0 {
			utilruntime.HandleError(fmt.Errorf("failed to serve request: No StorageVersion found for the requested GVR: %v", err))
			responsewriters.ErrorNegotiated(apierrors.NewServiceUnavailable(fmt.Sprintf("wait for storage version registration to complete for resource: %v", gv)), s, gv, w, req)
		}

		// proxy the request to one of the serviceableBy's

	})
}

type configController struct {
	name              string // varies in tests of fighting controllers
	svInformerSynced cache.InformerSynced
	svLister         apiserverinternallister.StorageVersionLister
}

func newTestableController(config TestableConfig) *configController {
	cfgCtlr := &configController{
		name:                   config.Name,
	}
	cfgCtlr.svLister = config.InformerFactory.Internal().V1alpha1().StorageVersions().Lister()
	return cfgCtlr
}
