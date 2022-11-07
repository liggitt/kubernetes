/*
Copyright 2022 The Kubernetes Authors.

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

package storage

import (
	"context"
	"fmt"

	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apiserver/pkg/authorization/authorizer"
	genericapirequest "k8s.io/apiserver/pkg/endpoints/request"
	genericregistry "k8s.io/apiserver/pkg/registry/generic/registry"
	"k8s.io/kubernetes/pkg/apis/admissionregistration"
	rbacregistry "k8s.io/kubernetes/pkg/registry/rbac"
)

func (r *REST) beginCreate(ctx context.Context, obj runtime.Object, options *metav1.CreateOptions) (genericregistry.FinishFunc, error) {
	// for superuser, skip all checks
	if rbacregistry.EscalationAllowed(ctx) {
		return noop, nil
	}

	policy := obj.(*admissionregistration.ValidatingAdmissionPolicy)
	if err := r.authorize(ctx, policy); err != nil {
		return nil, errors.NewForbidden(groupResource, policy.Name, err)
	}
	return noop, nil
}

func (r *REST) beginUpdate(ctx context.Context, obj, old runtime.Object, options *metav1.UpdateOptions) (genericregistry.FinishFunc, error) {
	// for superuser, skip all checks
	if rbacregistry.EscalationAllowed(ctx) {
		return noop, nil
	}

	policy := obj.(*admissionregistration.ValidatingAdmissionPolicy)
	oldPolicy := old.(*admissionregistration.ValidatingAdmissionPolicy)

	// if the policy has no paramKind, no extra authorization is required
	if policy.Spec.ParamKind == nil {
		return noop, nil
	}

	// if the new policy has the same paramKind as the old policy, no extra authorization is required
	if oldPolicy.Spec.ParamKind != nil && *oldPolicy.Spec.ParamKind == *policy.Spec.ParamKind {
		return noop, nil
	}

	// changed, authorize
	if err := r.authorize(ctx, policy); err != nil {
		return nil, errors.NewForbidden(groupResource, policy.Name, err)
	}
	return noop, nil
}

func (r *REST) authorize(ctx context.Context, policy *admissionregistration.ValidatingAdmissionPolicy) error {
	if r.authorizer == nil {
		return nil
	}
	if policy.Spec.ParamKind == nil {
		return nil
	}

	user, ok := genericapirequest.UserFrom(ctx)
	if !ok {
		return fmt.Errorf("cannot identify user to authorize read access to kind=%s, apiVersion=%s", policy.Spec.ParamKind.Kind, policy.Spec.ParamKind.APIVersion)
	}

	paramKind := policy.Spec.ParamKind
	// default to requiring permissions on all group/version/resources
	resource, apiGroup, apiVersion := "*", "*", "*"
	if gv, err := schema.ParseGroupVersion(paramKind.APIVersion); err == nil {
		// we only need to authorize the parsed group/version
		apiGroup = gv.Group
		apiVersion = gv.Version
		if gvr, err := r.resourceResolver.Resolve(gv.WithKind(paramKind.Kind)); err == nil {
			// we only need to authorize the resolved resource
			resource = gvr.Resource
		}
	}

	// require that the user can read (verb "get") the referred kind.
	attrs := authorizer.AttributesRecord{
		User:            user,
		Verb:            "get",
		ResourceRequest: true,
		Name:            "*",
		Namespace:       "*",
		APIGroup:        apiGroup,
		APIVersion:      apiVersion,
		Resource:        resource,
	}

	d, _, err := r.authorizer.Authorize(ctx, attrs)
	if err != nil {
		return err
	}
	if d != authorizer.DecisionAllow {
		return fmt.Errorf(`user %v must have "get" permission on all objects of the referenced paramKind (kind=%s, apiVersion=%s)`, user, paramKind.Kind, paramKind.APIVersion)
	}
	return nil
}

func noop(context.Context, bool) {}

var _ genericregistry.FinishFunc = noop
