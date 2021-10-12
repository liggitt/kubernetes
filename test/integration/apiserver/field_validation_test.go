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

package apiserver

import (
	"context"
	"fmt"
	"os"
	"strings"
	"testing"

	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apiserver/pkg/features"
	utilfeature "k8s.io/apiserver/pkg/util/feature"
	clientset "k8s.io/client-go/kubernetes"
	featuregatetesting "k8s.io/component-base/featuregate/testing"
)

// smpTestSetup applies an object that will later be patched
// in the actual test/benchmark.
func smpTestSetup(t testing.TB, client clientset.Interface) {
	bodyBase, err := os.ReadFile("./testdata/deploy-small.json")
	if err != nil {
		t.Fatalf("failed to read file: %v", err)
	}
	_, err = client.CoreV1().RESTClient().Patch(types.ApplyPatchType).
		AbsPath("/apis/apps/v1").
		Namespace("default").
		Resource("deployments").
		Name("test-deployment").
		Param("fieldManager", "apply_test").
		Body([]byte(fmt.Sprintf(string(bodyBase), "test-deployment"))).
		Do(context.TODO()).
		Get()
	if err != nil {
		t.Fatalf("Failed to create object using Apply patch: %v", err)
	}
}

// smpRunTest attempts to patch an object via strategic-merge-patch
// with params given from the testcase.
func smpRunTest(t testing.TB, client clientset.Interface, tc smpTestCase) {
	req := client.CoreV1().RESTClient().Patch(types.StrategicMergePatchType).
		AbsPath("/apis/apps/v1").
		Namespace("default").
		Resource("deployments").
		Name("test-deployment")
	for k, v := range tc.params {
		req.Param(k, v)
	}
	result, err := req.Body([]byte(`{"metadata":{"labels":{"label1": "val1"}},"spec":{"foo":"bar"}}`)).DoRaw(context.TODO())
	if err == nil && tc.errContains != "" {
		t.Fatalf("unexpected patch succeeded")
	}
	if err != nil && !strings.Contains(string(result), tc.errContains) {
		t.Fatalf("unexpected response: %v", string(result))
	}
}

type smpTestCase struct {
	name        string
	params      map[string]string
	errContains string
}

// TestFieldValidationSMP tests that attempting a strategic-merge-patch
// with unknown fields errors out when fieldValidation is strict,
// but succeeds when fieldValidation is ignored.
func TestFieldValidationSMP(t *testing.T) {
	defer featuregatetesting.SetFeatureGateDuringTest(t, utilfeature.DefaultFeatureGate, features.ServerSideApply, true)()

	_, client, closeFn := setup(t)
	defer closeFn()

	smpTestSetup(t, client)

	var testcases = []smpTestCase{
		{
			name:        "smp-strict-validation",
			params:      map[string]string{"fieldValidation": "Strict"},
			errContains: "unknown fields when converting from unstructured",
		},
		{
			name:        "smp-ignore-validation",
			params:      map[string]string{},
			errContains: "",
		},
	}

	for _, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {
			smpRunTest(t, client, tc)
		})
	}
}

// Benchmark strategic-merge-patch field validation for strict vs non-strict
func BenchmarkFieldValidationSMP(b *testing.B) {
	defer featuregatetesting.SetFeatureGateDuringTest(b, utilfeature.DefaultFeatureGate, features.ServerSideApply, true)()

	_, client, closeFn := setup(b)
	defer closeFn()

	smpTestSetup(b, client)

	var benchmarks = []smpTestCase{
		{
			name:        "smp-strict-validation",
			params:      map[string]string{"fieldValidation": "Strict"},
			errContains: "unknown fields when converting from unstructured",
		},
		{
			name:        "smp-ignore-validation",
			params:      map[string]string{},
			errContains: "",
		},
	}

	for _, bm := range benchmarks {
		b.Run(bm.name, func(b *testing.B) {
			b.ResetTimer()
			b.ReportAllocs()
			for n := 0; n < b.N; n++ {
				smpRunTest(b, client, bm)
			}

		})
	}
}
