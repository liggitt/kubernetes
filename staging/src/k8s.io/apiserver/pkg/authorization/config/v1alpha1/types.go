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

package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

type AuthorizationConfiguration struct {
	metav1.TypeMeta

	Authorizers []AuthorizerConfiguration `json:"authorizers"`
}

const (
	TypeWebhook AuthorizerType = "Webhook"
)

type AuthorizerType string

type AuthorizerConfiguration struct {
	Type string `json:"type"`

	Webhook *WebhookConfiguration `json:"webhook,omitempty"`
}

type WebhookConfiguration struct {
	// Name used to describe the webhook
	// This is explicitly used in monitoring machinery for metrics
	// Note:
	//   - Do exercise caution when setting the value
	//   - If not specified, the default would be set to ""
	//   - If there are multiple webhooks in the authorizer chain,
	//     this field is required
	Name string `json:"name"`
	// The duration to cache 'authorized' responses from the webhook
	// authorizer.
	// Same as setting `--authorization-webhook-cache-authorized-ttl` flag
	// Default: 5m0s
	AuthorizedTTL metav1.Duration `json:"authorizedTTL"`
	// The duration to cache 'unauthorized' responses from the webhook
	// authorizer.
	// Same as setting `--authorization-webhook-cache-unauthorized-ttl` flag
	// Default: 30s
	UnauthorizedTTL metav1.Duration `json:"unauthorizedTTL"`
	// Timeout for the webhook request
	// Default: 30s
	Timeout metav1.Duration `json:"timeout"`
	// The API version of the authorization.k8s.io SubjectAccessReview to
	// send to and expect from the # webhook.
	// Same as setting `--authorization-webhook-version` flag
	// Valid values: v1beta1, v1
	// Required, no default value
	SubjectAccessReviewVersion string `json:"subjectAccessReviewVersion"`
	// Controls the authorization decision when a webhook request fails to
	// complete or returns a malformed response.
	// Valid values:
	//   - NoOpinion: continue to subsequent authorizers to see if one of
	//     them allows the request
	//   - Deny: reject the request without consulting subsequent authorizers
	// Default: NoOpinion
	FailurePolicy string `json:"failurePolicy"`

	ConnectionInfo WebhookConnectionInfo `json:"connectionInfo"`

	MatchConditions []WebhookMatchCondition `json:"matchConditions"`
}

type WebhookConnectionInfo struct {
	Type string `json:"type"`

	KubeConfigFile *string `json:"kubeConfigFile"`
}

type WebhookMatchCondition struct {
	Expression string `json:"expression"`
}
