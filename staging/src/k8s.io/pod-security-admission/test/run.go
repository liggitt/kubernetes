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

package test

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/pod-security-admission/api"
	"k8s.io/pod-security-admission/policy"
)

// Options hold configuration for running integration tests against an existing server.
type Options struct {
	// Client is a client interface with sufficient permission to create, update, and delete
	// namespaces, pods, and pod-template-containing objects.
	// Required.
	Client kubernetes.Interface

	// CreateNamespace is an optional stub for creating a namespace with the given name and labels.
	// Returning an error fails the test.
	// If nil, DefaultCreateNamespace is used.
	CreateNamespace func(client kubernetes.Interface, name string, labels map[string]string) (*corev1.Namespace, error)

	// ExemptClient is an optional client interface to exercise behavior of an exempt client.
	ExemptClient kubernetes.Interface
	// ExemptNamespaces are optional namespaces not expected to have PodSecurity controls enforced.
	ExemptNamespaces []string
	// ExemptRuntimeClasses are optional runtimeclasses not expected to have PodSecurity controls enforced.
	ExemptRuntimeClasses []string
}

func toJSON(pod *corev1.Pod) string {
	data, _ := json.Marshal(pod)
	return string(data)
}

// Run creates namespaces with different policy levels/versions,
// and ensures pod fixtures expected to pass and fail against that level/version work as expected.
func Run(t *testing.T, opts Options) {
	if opts.Client == nil {
		t.Fatal("Client is required")
	}

	if opts.CreateNamespace == nil {
		opts.CreateNamespace = DefaultCreateNamespace
	}

	for _, level := range []api.Level{api.LevelBaseline, api.LevelRestricted} {
		// TODO: derive from registered levels
		// TODO: test "latest" and "no explicit version" are compatible with latest concrete policy
		for version := 0; version <= 22; version++ {
			ns := fmt.Sprintf("podsecurity-%s-1-%d", level, version)
			_, err := opts.CreateNamespace(opts.Client, ns, map[string]string{
				api.EnforceLevelLabel:   string(level),
				api.EnforceVersionLabel: fmt.Sprintf("v1.%d", version),
			})
			if err != nil {
				t.Errorf("failed creating namespace %s: %v", ns, err)
				continue
			}
			t.Cleanup(func() {
				opts.Client.CoreV1().Namespaces().Delete(context.Background(), ns, metav1.DeleteOptions{})
			})

			sa, err := opts.Client.CoreV1().ServiceAccounts(ns).Create(
				context.Background(),
				&corev1.ServiceAccount{ObjectMeta: metav1.ObjectMeta{Name: "default"}},
				metav1.CreateOptions{},
			)
			if err != nil && !apierrors.IsAlreadyExists(err) {
				t.Errorf("failed creating serviceaccount %s: %v", ns, err)
				continue
			}
			t.Cleanup(func() {
				opts.Client.CoreV1().ServiceAccounts(ns).Delete(context.Background(), sa.Name, metav1.DeleteOptions{})
			})

			createPod := func(t *testing.T, i int, pod *corev1.Pod, expectSuccess bool) {
				t.Helper()
				pod = pod.DeepCopy()
				pod.Name = "test"
				pod.Spec.ServiceAccountName = "default"
				_, err := opts.Client.CoreV1().Pods(ns).Create(context.Background(), pod, metav1.CreateOptions{DryRun: []string{metav1.DryRunAll}})
				if !expectSuccess && err == nil {
					t.Errorf("%d: expected error creating %s, got none", i, toJSON(pod))
				}
				if expectSuccess && err != nil {
					t.Errorf("%d: unexpected error creating %s: %v", i, toJSON(pod), err)
				}
			}

			minimalValidPod, err := getMinimalValidPod(level, api.MajorMinorVersion(1, version))
			if err != nil {
				t.Fatal(err)
			}
			t.Run(ns+"_pass_base", func(t *testing.T) {
				createPod(t, 0, minimalValidPod.DeepCopy(), true)
			})

			checks, err := policy.ChecksForLevelAndVersion(level, api.MajorMinorVersion(1, version))
			if err != nil {
				t.Fatal(err)
			}
			if len(checks) == 0 {
				t.Fatal(fmt.Errorf("no checks registered for %s/1.%d", level, version))
			}
			for _, check := range checks {
				checkData, err := getFixtures(fixtureKey{level: level, version: api.MajorMinorVersion(1, version), check: check.ID()})
				if err != nil {
					t.Fatal(err)
				}

				t.Run(ns+"_pass_"+check.ID(), func(t *testing.T) {
					for i, pod := range checkData.pass {
						createPod(t, i, pod, true)
					}
				})
				t.Run(ns+"_fail_"+check.ID(), func(t *testing.T) {
					for i, pod := range checkData.fail {
						createPod(t, i, pod, false)
					}
				})
			}
		}
	}
}

func DefaultCreateNamespace(client kubernetes.Interface, name string, labels map[string]string) (*corev1.Namespace, error) {
	return client.CoreV1().Namespaces().Create(
		context.Background(),
		&corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{Name: name, Labels: labels},
		},
		metav1.CreateOptions{},
	)
}
