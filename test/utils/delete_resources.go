/*
Copyright 2018 The Kubernetes Authors.

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

// TODO: Refactor common part of functions in this file for generic object kinds.

package utils

import (
	"context"
	"fmt"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	clientset "k8s.io/client-go/kubernetes"
	appsinternal "k8s.io/kubernetes/pkg/apis/apps"
	batchinternal "k8s.io/kubernetes/pkg/apis/batch"
	api "k8s.io/kubernetes/pkg/apis/core"
	extensionsinternal "k8s.io/kubernetes/pkg/apis/extensions"
)

func deleteResource(c clientset.Interface, resourceClient dynamic.ResourceInterface, kind schema.GroupKind, namespace, name string, options metav1.DeleteOptions) error {
	switch kind {
	case api.Kind("Pod"):
		return c.CoreV1().Pods(namespace).Delete(context.TODO(), name, options)
	case api.Kind("ReplicationController"):
		return c.CoreV1().ReplicationControllers(namespace).Delete(context.TODO(), name, options)
	case extensionsinternal.Kind("ReplicaSet"), appsinternal.Kind("ReplicaSet"):
		return c.AppsV1().ReplicaSets(namespace).Delete(context.TODO(), name, options)
	case extensionsinternal.Kind("Deployment"), appsinternal.Kind("Deployment"):
		return c.AppsV1().Deployments(namespace).Delete(context.TODO(), name, options)
	case extensionsinternal.Kind("DaemonSet"):
		return c.AppsV1().DaemonSets(namespace).Delete(context.TODO(), name, options)
	case batchinternal.Kind("Job"):
		return c.BatchV1().Jobs(namespace).Delete(context.TODO(), name, options)
	case api.Kind("Secret"):
		return c.CoreV1().Secrets(namespace).Delete(context.TODO(), name, options)
	case api.Kind("ConfigMap"):
		return c.CoreV1().ConfigMaps(namespace).Delete(context.TODO(), name, options)
	case api.Kind("Service"):
		return c.CoreV1().Services(namespace).Delete(context.TODO(), name, options)
	default:
		if resourceClient != nil {
			return resourceClient.Delete(context.TODO(), name, options)
		} else {
			return fmt.Errorf("Invalid resource client when deleting object of kind: %v with name: %s", kind, name)
		}
	}
}

func DeleteResourceWithRetries(c clientset.Interface, kind schema.GroupKind, namespace, name string, options metav1.DeleteOptions) error {
	return DeleteResourceWithRetriesWithDynamicClient(c, nil, schema.GroupVersionResource{}, kind, namespace, name, options)
}

func DeleteResourceWithRetriesWithDynamicClient(c clientset.Interface, dynamicClient dynamic.Interface, gvr schema.GroupVersionResource, kind schema.GroupKind, namespace, name string, options metav1.DeleteOptions) error {
	var resourceClient dynamic.ResourceInterface
	if dynamicClient != nil && !gvr.Empty() {
		resourceClient = dynamicClient.Resource(gvr).Namespace(namespace)
	}
	deleteFunc := func() (bool, error) {
		err := deleteResource(c, resourceClient, kind, namespace, name, options)
		if err == nil || apierrors.IsNotFound(err) {
			return true, nil
		}
		return false, fmt.Errorf("failed to delete object with non-retriable error: %v", err)
	}
	return RetryWithExponentialBackOff(deleteFunc)
}
