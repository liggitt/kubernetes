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

	binding := obj.(*admissionregistration.ValidatingAdmissionPolicyBinding)
	if err := r.authorize(ctx, binding); err != nil {
		return nil, errors.NewForbidden(groupResource, binding.Name, err)
	}
	return noop, nil
}

func (r *REST) beginUpdate(ctx context.Context, obj, old runtime.Object, options *metav1.UpdateOptions) (genericregistry.FinishFunc, error) {
	// for superuser, skip all checks
	if rbacregistry.EscalationAllowed(ctx) {
		return noop, nil
	}

	binding := obj.(*admissionregistration.ValidatingAdmissionPolicyBinding)
	oldBinding := old.(*admissionregistration.ValidatingAdmissionPolicyBinding)

	// both nil, no change
	if binding.Spec.ParamRef == nil && oldBinding.Spec.ParamRef == nil {
		return noop, nil
	}

	// both non-nil but equivalent
	if binding.Spec.ParamRef != nil && oldBinding.Spec.ParamRef != nil &&
		binding.Spec.ParamRef.Name == oldBinding.Spec.ParamRef.Name &&
		binding.Spec.ParamRef.Namespace == oldBinding.Spec.ParamRef.Namespace {
		return noop, nil
	}

	// if the binding has no ParamRef, no EXTRA permissions are checked
	if binding.Spec.ParamRef == nil {
		return noop, nil
	}

	// changed, authorize
	if err := r.authorize(ctx, binding); err != nil {
		return nil, errors.NewForbidden(groupResource, binding.Name, err)
	}
	return noop, nil
}

func (r *REST) authorize(ctx context.Context, binding *admissionregistration.ValidatingAdmissionPolicyBinding) error {
	if r.authorizer == nil {
		return nil
	}
	if binding.Spec.ParamRef == nil {
		return nil
	}

	user, ok := genericapirequest.UserFrom(ctx)
	if !ok {
		return fmt.Errorf("cannot identify user to authorize read access to paramRef object")
	}

	// default to requiring permissions on all group/version/resources
	resource, apiGroup, apiVersion := "*", "*", "*"

	if policy, err := r.policyGetter.GetValidatingAdmissionPolicy(ctx, binding.Spec.PolicyName); err == nil && policy.Spec.ParamKind != nil {
		paramKind := policy.Spec.ParamKind
		if gv, err := schema.ParseGroupVersion(paramKind.APIVersion); err == nil {
			// we only need to authorize the parsed group/version
			apiGroup = gv.Group
			apiVersion = gv.Version
			if gvr, err := r.resourceResolver.Resolve(gv.WithKind(paramKind.Kind)); err == nil {
				// we only need to authorize the resolved resource
				resource = gvr.Resource
			}
		}
	}

	paramRef := binding.Spec.ParamRef

	// require that the user can read (verb "get") the referred resource.
	attrs := authorizer.AttributesRecord{
		User:            user,
		Verb:            "get",
		ResourceRequest: true,
		Name:            paramRef.Name,
		Namespace:       paramRef.Namespace,
		APIGroup:        apiGroup,
		APIVersion:      apiVersion,
		Resource:        resource,
	}

	d, _, err := r.authorizer.Authorize(ctx, attrs)
	if err != nil {
		return err
	}
	if d != authorizer.DecisionAllow {
		return fmt.Errorf(`user %v must have "get" permission on object of the referenced paramRef %s`, user, paramRef)
	}
	return nil
}

func noop(context.Context, bool) {}

var _ genericregistry.FinishFunc = noop
