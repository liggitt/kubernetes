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

package validation

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	v1 "k8s.io/api/authorization/v1"
	"k8s.io/api/authorization/v1beta1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/apimachinery/pkg/util/validation/field"
	authorizationapi "k8s.io/apiserver/pkg/authorization/config"
)

// ValidateAuthorizationConfiguration validates a given AuthorizationConfiguration.
func ValidateAuthorizationConfiguration(fldPath *field.Path, c *authorizationapi.AuthorizationConfiguration, knownTypes sets.String, repeatableTypes sets.String) field.ErrorList {
	allErrs := field.ErrorList{}

	webhooks := 0
	for _, a := range c.Authorizers {
		if a.Type == authorizationapi.TypeWebhook {
			webhooks++
		}
	}

	seenAuthorizerTypes := sets.NewString()
	seenWebhookNames := sets.NewString()
	for i, a := range c.Authorizers {
		fldPath := fldPath.Child("authorizers").Index(i)
		aType := string(a.Type)
		if aType == "" {
			allErrs = append(allErrs, field.Required(fldPath.Child("type"), ""))
			continue
		}
		if !knownTypes.Has(aType) {
			allErrs = append(allErrs, field.NotSupported(fldPath.Child("type"), aType, knownTypes.List()))
			continue
		}
		if seenAuthorizerTypes.Has(aType) && !repeatableTypes.Has(aType) {
			allErrs = append(allErrs, field.Duplicate(fldPath.Child("type"), aType))
			continue
		}
		seenAuthorizerTypes.Insert(aType)

		switch a.Type {
		case authorizationapi.TypeWebhook:
			if a.Webhook == nil {
				allErrs = append(allErrs, field.Required(fldPath.Child("webhook"), "required when type=Webhook"))
				continue
			}
			allErrs = append(allErrs, ValidateWebhookConfiguration(fldPath, a.Webhook, webhooks > 0, seenWebhookNames)...)
		default:
			if a.Webhook != nil {
				allErrs = append(allErrs, field.Invalid(fldPath.Child("webhook"), "non-null", "may only be specified when type=Webhook"))
			}
		}
	}

	return allErrs
}

func ValidateWebhookConfiguration(fldPath *field.Path, c *authorizationapi.WebhookConfiguration, requireName bool, seenNames sets.String) field.ErrorList {
	allErrs := field.ErrorList{}
	if len(c.Name) == 0 {
		if requireName {
			allErrs = append(allErrs, field.Required(fldPath.Child("name"), ""))
		}
	} else if seenNames.Has(c.Name) {
		allErrs = append(allErrs, field.Duplicate(fldPath.Child("name"), c.Name))
	} else {
		// TODO: check format? dns label or subdomain?
	}
	seenNames.Insert(c.Name)

	if c.AuthorizedTTL.Duration == 0 {
		allErrs = append(allErrs, field.Required(fldPath.Child("authorizedTTL"), ""))
	}

	if c.UnauthorizedTTL.Duration == 0 {
		allErrs = append(allErrs, field.Required(fldPath.Child("unauthorizedTTL"), ""))
	}

	if c.Timeout.Duration == 0 {
		allErrs = append(allErrs, field.Required(fldPath.Child("timeout"), ""))
	} else if c.Timeout.Duration > 30*time.Minute {
		allErrs = append(allErrs, field.Invalid(fldPath.Child("timeout"), c.Timeout.Duration.String(), "must be <= 30s"))
	}

	var sampleSAR runtime.Object
	switch c.SubjectAccessReviewVersion {
	case "":
		allErrs = append(allErrs, field.Required(fldPath.Child("subjectAccessReviewVersion"), ""))
	case "v1":
		sampleSAR = &v1.SubjectAccessReview{}
	case "v1beta1":
		sampleSAR = &v1beta1.SubjectAccessReview{}
	default:
		allErrs = append(allErrs, field.NotSupported(fldPath.Child("subjectAccessReviewVersion"), c.SubjectAccessReviewVersion, []string{"v1", "v1beta1"}))
	}

	switch c.FailurePolicy {
	case "":
		allErrs = append(allErrs, field.Required(fldPath.Child("failurePolicy"), ""))
	case "NoOpinion", "Deny":
	default:
		allErrs = append(allErrs, field.NotSupported(fldPath.Child("failurePolicy"), c.FailurePolicy, []string{"NoOpinion", "Deny"}))
	}

	switch c.ConnectionInfo.Type {
	case "":
		allErrs = append(allErrs, field.Required(fldPath.Child("connectionInfo", "type"), ""))
	case "InClusterConfig":
		if c.ConnectionInfo.KubeConfigFile != nil {
			allErrs = append(allErrs, field.Invalid(fldPath.Child("connectionInfo", "kubeConfigFile"), *c.ConnectionInfo.KubeConfigFile, "can only be set when type=KubeConfigFile"))
		}
	case "KubeConfigFile":
		if c.ConnectionInfo.KubeConfigFile == nil || *c.ConnectionInfo.KubeConfigFile == "" {
			allErrs = append(allErrs, field.Required(fldPath.Child("connectionInfo", "kubeConfigFile"), ""))
		} else if !filepath.IsAbs(*c.ConnectionInfo.KubeConfigFile) {
			allErrs = append(allErrs, field.Invalid(fldPath.Child("connectionInfo", "kubeConfigFile"), *c.ConnectionInfo.KubeConfigFile, "must be an absolute path"))
		} else if info, err := os.Stat(*c.ConnectionInfo.KubeConfigFile); err != nil {
			allErrs = append(allErrs, field.Invalid(fldPath.Child("connectionInfo", "kubeConfigFile"), *c.ConnectionInfo.KubeConfigFile, fmt.Sprintf("error loading file: %v", err)))
		} else if !info.Mode().IsRegular() {
			allErrs = append(allErrs, field.Invalid(fldPath.Child("connectionInfo", "kubeConfigFile"), *c.ConnectionInfo.KubeConfigFile, "must be a regular file"))
		}
	default:
		allErrs = append(allErrs, field.NotSupported(fldPath.Child("connectionInfo", "type"), c.FailurePolicy, []string{"InClusterConfig", "KubeConfigFile"}))
	}

	for i, condition := range c.MatchConditions {
		fldPath := fldPath.Child("matchConditions").Index(i).Child("expression")
		if len(strings.TrimSpace(condition.Expression)) == 0 {
			allErrs = append(allErrs, field.Required(fldPath, ""))
		} else {
			allErrs = append(allErrs, ValidateWebhookMatchCondition(fldPath, sampleSAR, condition.Expression)...)
		}
	}

	return allErrs
}

func ValidateWebhookMatchCondition(fldPath *field.Path, sampleSAR runtime.Object, expression string) field.ErrorList {
	allErrs := field.ErrorList{}
	// TODO: typecheck CEL expression
	return allErrs
}
