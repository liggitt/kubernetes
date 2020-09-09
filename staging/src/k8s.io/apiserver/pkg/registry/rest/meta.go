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

package rest

import (
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/uuid"
)

// FillObjectMetaSystemFields populates fields that are managed by the system on ObjectMeta.
func FillObjectMetaSystemFields(meta metav1.Object) {
	meta.SetCreationTimestamp(metav1.Now())
	meta.SetUID(uuid.NewUUID())
	meta.SetSelfLink("")
}

func EnsureObjectNamespaceMatchesRequestNamespace(namespacedType bool, requestNamespace string, obj metav1.Object) error {
	if !namespacedType {
		// cluster-scoped, clear any namespace
		obj.SetNamespace(metav1.NamespaceNone)
		return nil
	}

	switch obj.GetNamespace() {
	case requestNamespace:
		// already matches, no-op
		return nil

	case metav1.NamespaceNone:
		// unset, default to request namespace
		obj.SetNamespace(requestNamespace)
		return nil

	default:
		// mismatch, error
		return errors.NewBadRequest("the namespace of the provided object does not match the namespace sent on the request")
	}
}
