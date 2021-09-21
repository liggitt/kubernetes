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

package celpoc

import (
	"context"
	"reflect"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"

	v1 "k8s.io/api/admission/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
)

// testcases used for functional and benchmark testing.
// testcases that expect compile errors are skipped for benchmarks.
var testcases = []struct {
	name string

	expression       string
	expectCompileErr string

	input *v1.AdmissionRequest

	expectEvalErr string
	expectEvalOut *v1.AdmissionResponse
}{
	{
		name:          "true",
		input:         &v1.AdmissionRequest{Operation: v1.Delete},
		expression:    `true`,
		expectEvalOut: &v1.AdmissionResponse{Allowed: true},
	},
	{
		name:          "false",
		input:         &v1.AdmissionRequest{Operation: v1.Delete},
		expression:    `false`,
		expectEvalOut: &v1.AdmissionResponse{Allowed: false},
	},
	{
		name:       "error",
		input:      &v1.AdmissionRequest{Operation: v1.Delete},
		expression: `"some error"`,
		expectEvalOut: &v1.AdmissionResponse{
			Allowed: false,
			Result:  &metav1.Status{Message: "some error"},
		},
	},
	{
		name:  "warnings and audits",
		input: &v1.AdmissionRequest{Operation: v1.Delete},
		expression: `k8s.io.api.admission.v1.AdmissionResponse{
			allowed: true,
			warnings: ["warning1","warning2"],
			auditAnnotations: {"key1":"value1","key2":"value2"}
		}`,
		expectEvalOut: &v1.AdmissionResponse{
			Allowed:          true,
			Warnings:         []string{"warning1", "warning2"},
			AuditAnnotations: map[string]string{"key1": "value1", "key2": "value2"},
		},
	},
	{
		name:          "evaluate typed admission fields",
		input:         &v1.AdmissionRequest{Operation: v1.Delete},
		expression:    `request.operation == "DELETE"`,
		expectEvalOut: &v1.AdmissionResponse{Allowed: true},
	},
	{
		name:          "evaluate typed admission fields with message success",
		input:         &v1.AdmissionRequest{Operation: v1.Delete},
		expression:    `request.operation == "DELETE" ? "" : "only delete is allowed"`,
		expectEvalOut: &v1.AdmissionResponse{Allowed: true},
	},
	{
		name:       "evaluate typed admission fields with message",
		input:      &v1.AdmissionRequest{Operation: v1.Delete},
		expression: `request.operation == "CREATE" ? "" : "only create is allowed"`,
		expectEvalOut: &v1.AdmissionResponse{
			Allowed: false,
			Result:  &metav1.Status{Message: "only create is allowed"},
		},
	},
	{
		name: "evaluate typed object fields",
		// FIXME: map paths under "request.object" to input object go fields based on json tags
		input: &v1.AdmissionRequest{
			Object: runtime.RawExtension{
				Object: &corev1.Pod{
					TypeMeta:   metav1.TypeMeta{APIVersion: "v1", Kind: "Pod"},
					ObjectMeta: metav1.ObjectMeta{Namespace: "testns", Name: "testname"},
					Spec:       corev1.PodSpec{ServiceAccountName: "default"},
				},
			},
		},
		expression:    `request.object.spec.serviceAccountName == "default"`,
		expectEvalOut: &v1.AdmissionResponse{Allowed: true},
	},
	{
		name: "evaluate untyped object fields",
		// FIXME: map paths under "request.object" to input object unstructured data based on traversing unstructured map
		input: &v1.AdmissionRequest{
			Object: runtime.RawExtension{
				Object: &unstructured.Unstructured{
					Object: map[string]interface{}{
						"apiVersion": "foo/v1",
						"kind":       "Bar",
						"metadata": map[string]interface{}{
							"namespace": "testns",
							"name":      "testname",
						},
						"spec": map[string]interface{}{
							"replicas": int64(1),
						},
					},
				},
			},
		},
		expression:    `request.object.spec.replicas == 1`,
		expectEvalOut: &v1.AdmissionResponse{Allowed: true},
	},
	{
		name:  "aggregate list items",
		input: &v1.AdmissionRequest{},
		expression: `[
			true,
			false,
			"error #1",
			"error #2.",
			" error #3 ",
			"error #4",
			AdmissionResponse{allowed:true, warnings:["a","b","c"]},
			AdmissionResponse{allowed:true, warnings:["c","d","e","f"]},
			AdmissionResponse{allowed:true, auditAnnotations:{"a":"a1","b":"b1"}},
			AdmissionResponse{allowed:true, auditAnnotations:{"b":"b2","c":"c2"}},
			AdmissionResponse{allowed:true},
			AdmissionResponse{allowed:false}
		]`,
		expectEvalOut: &v1.AdmissionResponse{
			Allowed:          false,
			Warnings:         []string{"a", "b", "c", "d", "e", "f"},
			AuditAnnotations: map[string]string{"a": "a1", "b": "b2", "c": "c2"},
			Result:           &metav1.Status{Message: "error #1; error #2. error #3; error #4"},
		},
	},
	/*
		TODO: testcases/examples
		- require specific label (handle unset labels, empty labels, existing labels cases)
		- forbid specific label (handle unset labels, empty labels, existing labels cases)
		- mutate to set label (handle unset labels, empty labels, existing labels cases)
		- check for a specific field value in all container fields (all containers/initContainers)
		- mutate to set a specific nested field value in all container fields (all containers/initContainers)
		- check for a specific field value in a container with a particular name
		- mutate to set a specific field value in a container with a particular name
	*/
}

func TestEval(t *testing.T) {
	for _, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {
			p, err := NewProgram(tc.expression)
			if err != nil {
				if len(tc.expectCompileErr) == 0 {
					t.Fatalf("no compile error expected, got %v", err)
				}
				if !strings.Contains(err.Error(), tc.expectCompileErr) {
					t.Fatalf("expected compile error '%s', got '%s'", tc.expectCompileErr, err.Error())
				}
				return
			} else if len(tc.expectCompileErr) > 0 {
				t.Fatalf("expected compile error '%s', got none", tc.expectCompileErr)
			}

			output, err := p.Evaluate(context.Background(), tc.input)
			if err != nil {
				if len(tc.expectEvalErr) == 0 {
					t.Fatalf("no eval error expected, got %v", err)
				}
				if !strings.Contains(err.Error(), tc.expectEvalErr) {
					t.Fatalf("expected eval error '%s', got '%s'", tc.expectEvalErr, err.Error())
				}
				return
			} else if len(tc.expectEvalErr) > 0 {
				t.Fatalf("expected eval error '%s', got none", tc.expectEvalErr)
			}

			if !reflect.DeepEqual(output, tc.expectEvalOut) {
				t.Fatalf("unexpected output:\n%s", cmp.Diff(tc.expectEvalOut, output))
			}
		})
	}
}

func BenchmarkEval(b *testing.B) {
	ctx := context.Background()
	for _, tc := range testcases {
		b.Run("compile_"+tc.name, func(b *testing.B) {
			b.RunParallel(func(pb *testing.PB) {
				for pb.Next() {
					NewProgram(tc.expression)
				}
			})
		})
		if len(tc.expectCompileErr) > 0 {
			continue
		}
		b.Run("eval_"+tc.name, func(b *testing.B) {
			p, err := NewProgram(tc.expression)
			if err != nil {
				b.Fatal(err)
			}
			b.ResetTimer()
			b.RunParallel(func(pb *testing.PB) {
				for pb.Next() {
					p.Evaluate(ctx, tc.input)
				}
			})
		})
	}
}
