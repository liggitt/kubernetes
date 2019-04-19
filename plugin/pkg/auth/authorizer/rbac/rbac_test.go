/*
Copyright 2016 The Kubernetes Authors.

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

package authorizer

import (
	"testing"

	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apiserver/pkg/authentication/user"
	"k8s.io/apiserver/pkg/authorization/authorizer"
	"k8s.io/kubernetes/plugin/pkg/auth/authorizer/rbac/bootstrappolicy"
	rbacauthorizer "k8s.io/rbac/authorizer"
	rbacregistryvalidation "k8s.io/rbac/validation"
)

func BenchmarkAuthorize(b *testing.B) {
	bootstrapRoles := []rbacv1.ClusterRole{}
	bootstrapRoles = append(bootstrapRoles, bootstrappolicy.ControllerRoles()...)
	bootstrapRoles = append(bootstrapRoles, bootstrappolicy.ClusterRoles()...)

	bootstrapBindings := []rbacv1.ClusterRoleBinding{}
	bootstrapBindings = append(bootstrapBindings, bootstrappolicy.ClusterRoleBindings()...)
	bootstrapBindings = append(bootstrapBindings, bootstrappolicy.ControllerRoleBindings()...)

	clusterRoles := []*rbacv1.ClusterRole{}
	for i := range bootstrapRoles {
		clusterRoles = append(clusterRoles, &bootstrapRoles[i])
	}
	clusterRoleBindings := []*rbacv1.ClusterRoleBinding{}
	for i := range bootstrapBindings {
		clusterRoleBindings = append(clusterRoleBindings, &bootstrapBindings[i])
	}

	_, resolver := rbacregistryvalidation.NewTestRuleResolver(nil, nil, clusterRoles, clusterRoleBindings)

	authz := rbacauthorizer.New(resolver, resolver, resolver, resolver)

	nodeUser := &user.DefaultInfo{Name: "system:node:node1", Groups: []string{"system:nodes", "system:authenticated"}}
	requests := []struct {
		name  string
		attrs authorizer.Attributes
	}{
		{
			"allow list pods",
			authorizer.AttributesRecord{
				ResourceRequest: true,
				User:            nodeUser,
				Verb:            "list",
				Resource:        "pods",
				Subresource:     "",
				Name:            "",
				Namespace:       "",
				APIGroup:        "",
				APIVersion:      "v1",
			},
		},
		{
			"allow update pods/status",
			authorizer.AttributesRecord{
				ResourceRequest: true,
				User:            nodeUser,
				Verb:            "update",
				Resource:        "pods",
				Subresource:     "status",
				Name:            "mypods",
				Namespace:       "myns",
				APIGroup:        "",
				APIVersion:      "v1",
			},
		},
		{
			"forbid educate dolphins",
			authorizer.AttributesRecord{
				ResourceRequest: true,
				User:            nodeUser,
				Verb:            "educate",
				Resource:        "dolphins",
				Subresource:     "",
				Name:            "",
				Namespace:       "",
				APIGroup:        "",
				APIVersion:      "v1",
			},
		},
	}

	b.ResetTimer()
	for _, request := range requests {
		b.Run(request.name, func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				authz.Authorize(request.attrs)
			}
		})
	}
}
