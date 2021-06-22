/*
Copyright 2021 The Kubernetes Authors.

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

package policy

import (
	"strings"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type Check interface {
	ID() string
	// CheckPod determines if the pod is allowed.
	CheckPod(podMetadata *metav1.ObjectMeta, podSpec *corev1.PodSpec) CheckResult
}

// CheckResult contains the result of checking a pod and indicates whether the pod is allowed,
// and if not, why it was forbidden.
//
// Example output for (false, "host ports", "8080, 9090"):
//   When checking all pods in a namespace:
//     disallowed by policy "baseline": host ports, privileged containers, non-default capabilities
//   When checking an individual pod:
//     disallowed by policy "baseline": host ports (8080, 9090), privileged containers, non-default capabilities (CAP_NET_RAW)
type CheckResult struct {
	// Allowed indicates if the check allowed the pod.
	Allowed bool
	// ForbiddenReason should only be set if Allowed is false.
	// ForbiddenReason should be as succinct as possible and is always output.
	ForbiddenReason string
	// ForbiddenDetail should only be set if Allowed is false.
	// ForbiddenDetail can include specific values that were disallowed and is used when checking an individual object.
	ForbiddenDetail string
}

// CheckDocumentation is used to generate documentation for checks.
type CheckDocumentation interface {
	// Name returns a short human-readable string, used for the left column of the docs
	Name() string
	// Description returns markdown, used for the body of docs
	Description() string
	// Delta describes changes since the previous version of this check.
	// Optional, should be empty if there was no previous version of this check
	Delta() string
}

type doc struct {
	name        string
	description string
	delta       string
}

func (d doc) Name() string        { return d.name }
func (d doc) Description() string { return d.description }
func (d doc) Delta() string       { return d.delta }

type check struct {
	id string
	doc
	checkPod func(podMetadata *metav1.ObjectMeta, podSpec *corev1.PodSpec) CheckResult
}

func (c *check) CheckPod(podMetadata *metav1.ObjectMeta, podSpec *corev1.PodSpec) CheckResult {
	return c.checkPod(podMetadata, podSpec)
}
func (c *check) ID() string {
	return c.id
}

// AggergateCheckResult holds the aggregate result of running CheckPod across multiple checks.
type AggregateCheckResult struct {
	// Allowed indicates if all checks allowed the pod.
	Allowed bool
	// ForbiddenReasons is a slice of the forbidden reasons from all the forbidden checks. It should not include empty strings.
	// ForbiddenReasons and ForbiddenDetails must have the same number of elements, and the indexes are for the same check.
	ForbiddenReasons []string
	// ForbiddenDetails is a slice of the forbidden details from all the forbidden checks. It may include empty strings.
	// ForbiddenReasons and ForbiddenDetails must have the same number of elements, and the indexes are for the same check.
	ForbiddenDetails []string
}

// ForbiddenReason returns a comma-separated string of of the forbidden reasons.
// Example: host ports, privileged containers, non-default capabilities
func (a *AggregateCheckResult) ForbiddenReason() string {
	return strings.Join(a.ForbiddenReasons, ", ")
}

// ForbiddenDetail returns a detailed forbidden message, with non-empty details formatted in
// parentheses with the associated reason.
// Example: host ports (8080, 9090), privileged containers, non-default capabilities (NET_RAW)
func (a *AggregateCheckResult) ForbiddenDetail() string {
	var b strings.Builder
	for i := 0; i < len(a.ForbiddenReasons); i++ {
		b.WriteString(a.ForbiddenReasons[i])
		if a.ForbiddenDetails[i] != "" {
			b.WriteString(" (")
			b.WriteString(a.ForbiddenDetails[i])
			b.WriteString(")")
		}
		if i != len(a.ForbiddenReasons)-1 {
			b.WriteString(", ")
		}
	}
	return b.String()
}

// AggregateCheckPod runs all the checks and aggregates the forbidden results into a single CheckResult.
// The aggregated reason is a comma-separated
func AggregateCheckPod(checks []Check, podMetadata *metav1.ObjectMeta, podSpec *corev1.PodSpec) AggregateCheckResult {
	var (
		reasons []string
		details []string
	)
	for _, check := range checks {
		r := check.CheckPod(podMetadata, podSpec)
		if !r.Allowed {
			reasons = append(reasons, r.ForbiddenReason)
			details = append(details, r.ForbiddenDetail)
		}
	}
	return AggregateCheckResult{
		Allowed:          len(reasons) == 0,
		ForbiddenReasons: reasons,
		ForbiddenDetails: details,
	}
}
