/*
Copyright 2020 The Kubernetes Authors.

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

package storageversion

import (
	"context"
	"testing"
	"time"

	apiextensionsclientset "k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset"
	apiextensionsv1informer "k8s.io/apiextensions-apiserver/pkg/client/informers/externalversions/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	kubeapiservertesting "k8s.io/kubernetes/cmd/kube-apiserver/app/testing"
	"k8s.io/kubernetes/test/integration/etcd"
	"k8s.io/kubernetes/test/integration/framework"
)

func TestUnavailable(t *testing.T) {
	etcdConfig := framework.SharedEtcd()
	crd := etcd.GetCustomResourceDefinitionData()[0]
	gvr := schema.GroupVersionResource{
		Group:    crd.Spec.Group,
		Version:  crd.Spec.Versions[0].Name,
		Resource: crd.Spec.Names.Plural,
	}
	{
		server := kubeapiservertesting.StartTestServerOrDie(t, nil, nil, etcdConfig)
		etcd.CreateTestCRDs(t, apiextensionsclientset.NewForConfigOrDie(server.ClientConfig), false, crd)
		obj, err := dynamic.NewForConfigOrDie(server.ClientConfig).Resource(gvr).Namespace("default").Create(context.TODO(), &unstructured.Unstructured{Object: map[string]interface{}{
			"apiVersion": gvr.GroupVersion().String(),
			"kind":       crd.Spec.Names.Kind,
			"metadata": map[string]interface{}{
				"name": "test",
			},
		}}, metav1.CreateOptions{})
		if err != nil {
			t.Fatal(err)
		}
		t.Log(obj.GetName())
		server.TearDownFn()
	}

	apiextensionsv1informer.Delay.Store(true)
	{
		options := kubeapiservertesting.NewDefaultTestServerOptions()
		options.SkipHealthCheck = true
		t.Log(time.Now(), "JTL", 63)
		server := kubeapiservertesting.StartTestServerOrDie(t, options, nil, etcdConfig)
		t.Log(time.Now(), "JTL", 65)
		_, err := dynamic.NewForConfigOrDie(server.ClientConfig).Resource(gvr).Namespace("default").Get(context.TODO(), "test", metav1.GetOptions{})
		t.Log(time.Now(), "JTL", err)
		server.TearDownFn()
	}
}
