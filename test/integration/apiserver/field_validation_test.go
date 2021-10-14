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
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	apiextensionsclient "k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset"
	"k8s.io/apiextensions-apiserver/test/integration/fixtures"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apiserver/pkg/features"
	utilfeature "k8s.io/apiserver/pkg/util/feature"
	"k8s.io/client-go/dynamic"
	clientset "k8s.io/client-go/kubernetes"
	restclient "k8s.io/client-go/rest"
	featuregatetesting "k8s.io/component-base/featuregate/testing"
	kubeapiservertesting "k8s.io/kubernetes/cmd/kube-apiserver/app/testing"

	"k8s.io/kubernetes/test/integration/framework"
)

// TestFieldValidationPut tests PUT requests containing unknown fields with
// strict and non-strict field validation.
func TestFieldValidationPut(t *testing.T) {
	defer featuregatetesting.SetFeatureGateDuringTest(t, utilfeature.DefaultFeatureGate, features.ServerSideApply, true)()

	_, client, closeFn := setup(t)
	defer closeFn()

	deployName := `"test-deployment"`
	postBytes, err := os.ReadFile("./testdata/deploy-small.json")
	if err != nil {
		t.Fatalf("failed to read file: %v", err)
	}
	postBody := []byte(fmt.Sprintf(string(postBytes), deployName))

	if _, err := client.CoreV1().RESTClient().Post().
		AbsPath("/apis/apps/v1").
		Namespace("default").
		Resource("deployments").
		Body(postBody).
		DoRaw(context.TODO()); err != nil {
		t.Fatalf("failed to create initial deployment: %v", err)
	}

	putBytes, err := os.ReadFile("./testdata/deploy-small-unknown-field.json")
	if err != nil {
		t.Fatalf("failed to read file: %v", err)
	}
	putBody := []byte(fmt.Sprintf(string(putBytes), deployName))
	var testcases = []struct {
		name string
		// TODO: use PostOptions for fieldValidation param instead of raw strings.
		params      map[string]string
		errContains string
	}{
		{
			name:        "put-strict-validation",
			params:      map[string]string{"fieldValidation": "Strict"},
			errContains: "unknown field",
		},
		{
			name:        "put-default-ignore-validation",
			params:      map[string]string{},
			errContains: "",
		},
		{
			name:        "put-ignore-validation",
			params:      map[string]string{"fieldValidation": "Ignore"},
			errContains: "",
		},
	}

	for _, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {
			req := client.CoreV1().RESTClient().Put().
				AbsPath("/apis/apps/v1").
				Namespace("default").
				Resource("deployments").
				Name("test-dep")
			for k, v := range tc.params {
				req.Param(k, v)

			}
			result, err := req.Body(putBody).DoRaw(context.TODO())
			if err == nil && tc.errContains != "" {
				t.Fatalf("unexpected post succeeded")

			}
			if err != nil && !strings.Contains(string(result), tc.errContains) {
				t.Fatalf("unexpected response: %v", string(result))

			}
		})

	}

}

// TestFieldValidationPost tests POST requests containing unknown fields with
// strict and non-strict field validation.
func TestFieldValidationPost(t *testing.T) {
	defer featuregatetesting.SetFeatureGateDuringTest(t, utilfeature.DefaultFeatureGate, features.ServerSideApply, true)()

	_, client, closeFn := setup(t)
	defer closeFn()

	bodyBytes, err := os.ReadFile("./testdata/deploy-small-unknown-field.json")
	if err != nil {
		t.Fatalf("failed to read file: %v", err)
	}
	body := []byte(fmt.Sprintf(string(bodyBytes), `"test-deployment"`))

	var testcases = []struct {
		name string
		// TODO: use PostOptions for fieldValidation param instead of raw strings.
		params      map[string]string
		errContains string
	}{
		{
			name:        "post-strict-validation",
			params:      map[string]string{"fieldValidation": "Strict"},
			errContains: "unknown field",
		},
		{
			name:        "post-default-ignore-validation",
			params:      map[string]string{},
			errContains: "",
		},
		{
			name:        "post-ignore-validation",
			params:      map[string]string{"fieldValidation": "Ignore"},
			errContains: "",
		},
	}

	for _, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {
			req := client.CoreV1().RESTClient().Post().
				AbsPath("/apis/apps/v1").
				Namespace("default").
				Resource("deployments")
			for k, v := range tc.params {
				req.Param(k, v)

			}
			result, err := req.Body([]byte(body)).DoRaw(context.TODO())
			if err == nil && tc.errContains != "" {
				t.Fatalf("unexpected post succeeded")
			}
			if err != nil && !strings.Contains(string(result), tc.errContains) {
				t.Fatalf("unexpected response: %v", string(result))
			}
		})
	}
}

// Benchmark field validation for strict vs non-strict
func BenchmarkFieldValidationPostPut(b *testing.B) {
	_, client, closeFn := setup(b)
	defer closeFn()

	flag.Lookup("v").Value.Set("0")
	corePath := "/api/v1"
	appsPath := "/apis/apps/v1"
	// TODO: split POST and PUT into their own test-cases.
	// TODO: add test for "Warn" validation once it is implemented.
	benchmarks := []struct {
		name     string
		params   map[string]string
		bodyFile string
		resource string
		absPath  string
	}{
		{
			name:     "ignore-validation-deployment",
			params:   map[string]string{"fieldValidation": "Ignore"},
			bodyFile: "./testdata/deploy-small.json",
			resource: "deployments",
			absPath:  appsPath,
		},
		{
			name:     "strict-validation-deployment",
			params:   map[string]string{"fieldValidation": "Strict"},
			bodyFile: "./testdata/deploy-small.json",
			resource: "deployments",
			absPath:  appsPath,
		},
		{
			name:     "ignore-validation-pod",
			params:   map[string]string{"fieldValidation": "Ignore"},
			bodyFile: "./testdata/pod-medium.json",
			resource: "pods",
			absPath:  corePath,
		},
		{
			name:     "strict-validation-pod",
			params:   map[string]string{"fieldValidation": "Strict"},
			bodyFile: "./testdata/pod-medium.json",
			resource: "pods",
			absPath:  corePath,
		},
		{
			name:     "ignore-validation-big-pod",
			params:   map[string]string{"fieldValidation": "Ignore"},
			bodyFile: "./testdata/pod-large.json",
			resource: "pods",
			absPath:  corePath,
		},
		{
			name:     "strict-validation-big-pod",
			params:   map[string]string{"fieldValidation": "Strict"},
			bodyFile: "./testdata/pod-large.json",
			resource: "pods",
			absPath:  corePath,
		},
	}

	for _, bm := range benchmarks {
		b.Run(bm.name, func(b *testing.B) {
			b.ResetTimer()
			b.ReportAllocs()
			for n := 0; n < b.N; n++ {
				// append the timestamp to the name so that we don't hit conflicts when running the test multiple times
				// (i.e. without it -count=n for n>1 will fail, this might be from not tearing stuff down properly).
				bodyBase, err := os.ReadFile(bm.bodyFile)
				if err != nil {
					b.Fatal(err)
				}

				objName := fmt.Sprintf("obj-%s-%d-%d-%d", bm.name, n, b.N, time.Now().UnixNano())
				objString := fmt.Sprintf(string(bodyBase), fmt.Sprintf(`"%s"`, objName))
				body := []byte(objString)

				postReq := client.CoreV1().RESTClient().Post().
					AbsPath(bm.absPath).
					Namespace("default").
					Resource(bm.resource)
				for k, v := range bm.params {
					postReq = postReq.Param(k, v)
				}

				_, err = postReq.Body(body).
					DoRaw(context.TODO())
				if err != nil {
					b.Fatal(err)
				}

				// TODO: put PUT in a different bench case than POST (ie. have a baseReq) be a part of the test case.
				putReq := client.CoreV1().RESTClient().Put().
					AbsPath(bm.absPath).
					Namespace("default").
					Resource(bm.resource).
					Name(objName)
				for k, v := range bm.params {
					putReq = putReq.Param(k, v)
				}

				_, err = putReq.Body(body).
					DoRaw(context.TODO())
				if err != nil {
					b.Fatal(err)
				}
			}
		})

	}
}

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
			errContains: "unknown field",
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

	// TODO: add more benchmarks to test bigger objects
	var benchmarks = []smpTestCase{
		{
			name:        "smp-strict-validation",
			params:      map[string]string{"fieldValidation": "Strict"},
			errContains: "unknown field",
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

func patchCRDTestSetup(t testing.TB, server kubeapiservertesting.TestServer, name string) (restclient.Interface, *apiextensionsv1.CustomResourceDefinition) {
	config := server.ClientConfig

	apiExtensionClient, err := apiextensionsclient.NewForConfig(config)
	if err != nil {
		t.Fatal(err)
	}
	dynamicClient, err := dynamic.NewForConfig(config)
	if err != nil {
		t.Fatal(err)
	}
	crdSchema, err := os.ReadFile("./testdata/crd-schema.json")
	if err != nil {
		t.Fatalf("failed to read file: %v", err)
	}
	patchYAMLBody, err := os.ReadFile("./testdata/noxu-cr-shell.yaml")
	if err != nil {
		t.Fatalf("failed to read file: %v", err)
	}

	// create the CRD
	noxuDefinition := fixtures.NewNoxuV1CustomResourceDefinition(apiextensionsv1.ClusterScoped)
	var c apiextensionsv1.CustomResourceValidation
	err = json.Unmarshal(crdSchema, &c)
	if err != nil {
		t.Fatal(err)
	}
	// set the CRD schema
	noxuDefinition.Spec.PreserveUnknownFields = false
	for i := range noxuDefinition.Spec.Versions {
		noxuDefinition.Spec.Versions[i].Schema = &c
	}
	// install the CRD
	noxuDefinition, err = fixtures.CreateNewV1CustomResourceDefinition(noxuDefinition, apiExtensionClient, dynamicClient)
	if err != nil {
		t.Fatal(err)
	}

	kind := noxuDefinition.Spec.Names.Kind
	apiVersion := noxuDefinition.Spec.Group + "/" + noxuDefinition.Spec.Versions[0].Name

	// create a CR
	rest := apiExtensionClient.Discovery().RESTClient()
	yamlBody := []byte(fmt.Sprintf(string(patchYAMLBody), apiVersion, kind, name))
	result, err := rest.Patch(types.ApplyPatchType).
		AbsPath("/apis", noxuDefinition.Spec.Group, noxuDefinition.Spec.Versions[0].Name, noxuDefinition.Spec.Names.Plural).
		Name(name).
		Param("fieldManager", "apply_test").
		Body(yamlBody).
		DoRaw(context.TODO())
	if err != nil {
		t.Fatalf("failed to create custom resource with apply: %v:\n%v", err, string(result))
	}

	return rest, noxuDefinition
}

// TestFieldValidationPatchCRD tests that server-side schema validation
// works for jsonpatch and mergepatch requests.
func TestFieldValidationPatchCRD(t *testing.T) {
	var testcases = []struct {
		name        string
		patchType   types.PatchType
		params      map[string]string
		body        string
		errContains string
	}{
		{
			name:        "merge-patch-strict-validation",
			patchType:   types.MergePatchType,
			params:      map[string]string{"fieldValidation": "Strict"},
			body:        `{"metadata":{"finalizers":["test-finalizer","another-one"]}, "spec":{"foo": "bar"}}`,
			errContains: "unknown field",
		},
		{
			name:        "merge-patch-no-validation",
			patchType:   types.MergePatchType,
			params:      map[string]string{},
			body:        `{"metadata":{"finalizers":["test-finalizer","another-one"]}, "spec":{"foo": "bar"}}`,
			errContains: "",
		},
		// TODO: figure out how to test JSONPatch
		//{
		//	name:        "jsonPatchStrictValidation",
		//	patchType:   types.JSONPatchType,
		//	params:      map[string]string{"validate": "strict"},
		//	body:        // TODO
		//	errContains: "failed with unknown fields",
		//},
		//{
		//	name:        "jsonPatchNoValidation",
		//	patchType:   types.JSONPatchType,
		//	params:      map[string]string{},
		//	body:        // TODO
		//	errContains: "",
		//},
	}
	for _, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {
			// setup the testerver and install the CRD
			server, err := kubeapiservertesting.StartTestServer(t, kubeapiservertesting.NewDefaultTestServerOptions(), nil, framework.SharedEtcd())
			if err != nil {
				t.Fatal(err)
			}
			defer server.TearDownFn()
			rest, noxuDefinition := patchCRDTestSetup(t, server, tc.name)

			// patch the CR as specified by the test case
			req := rest.Patch(tc.patchType).
				AbsPath("/apis", noxuDefinition.Spec.Group, noxuDefinition.Spec.Versions[0].Name, noxuDefinition.Spec.Names.Plural).
				Name(tc.name)
			for k, v := range tc.params {
				req = req.Param(k, v)
			}
			result, err := req.
				Body([]byte(tc.body)).
				DoRaw(context.TODO())
			if err == nil && tc.errContains != "" {
				t.Fatalf("unexpected patch succeeded, expected %s", tc.errContains)
			}
			if err != nil && !strings.Contains(string(result), tc.errContains) {
				t.Errorf("bad err: %v", err)
				t.Fatalf("unexpected response: %v", string(result))
			}
		})
	}
}

// Benchmark patch CRD for strict vs non-strict
func BenchmarkFieldValidationPatchCRD(b *testing.B) {
	benchmarks := []struct {
		name        string
		patchType   types.PatchType
		params      map[string]string
		bodyBase    string
		errContains string
	}{
		{
			name:        "ignore-validation-crd-patch",
			patchType:   types.MergePatchType,
			params:      map[string]string{},
			bodyBase:    `{"metadata":{"finalizers":["test-finalizer","finalizer-ignore-%d"]}}`,
			errContains: "",
		},
		{
			name:        "strict-validation-crd-patch",
			patchType:   types.MergePatchType,
			params:      map[string]string{"fieldValidation": "Strict"},
			bodyBase:    `{"metadata":{"finalizers":["test-finalizer","finalizer-strict-%d"]}}`,
			errContains: "",
		},
		{
			name:        "ignore-validation-crd-patch-unknown-field",
			patchType:   types.MergePatchType,
			params:      map[string]string{},
			bodyBase:    `{"metadata":{"finalizers":["test-finalizer","finalizer-ignore-unknown-%d"]}, "spec":{"foo": "bar"}}`,
			errContains: "",
		},
		{
			name:        "strict-validation-crd-patch-unknown-field",
			patchType:   types.MergePatchType,
			params:      map[string]string{"fieldValidation": "Strict"},
			bodyBase:    `{"metadata":{"finalizers":["test-finalizer","finalizer-strict-unknown-%d"]}, "spec":{"foo": "bar"}}`,
			errContains: "unknown field",
		},
	}
	for _, bm := range benchmarks {
		b.Run(bm.name, func(b *testing.B) {
			b.ResetTimer()
			b.ReportAllocs()
			for n := 0; n < b.N; n++ {

				// setup the testerver and install the CRD
				server, err := kubeapiservertesting.StartTestServer(b, kubeapiservertesting.NewDefaultTestServerOptions(), nil, framework.SharedEtcd())
				if err != nil {
					b.Fatal(err)
				}
				defer server.TearDownFn()
				rest, noxuDefinition := patchCRDTestSetup(b, server, bm.name)

				body := fmt.Sprintf(bm.bodyBase, n)
				// patch the CR as specified by the test case
				req := rest.Patch(bm.patchType).
					AbsPath("/apis", noxuDefinition.Spec.Group, noxuDefinition.Spec.Versions[0].Name, noxuDefinition.Spec.Names.Plural).
					Name(bm.name)
				for k, v := range bm.params {
					req = req.Param(k, v)
				}
				result, err := req.
					Body([]byte(body)).
					DoRaw(context.TODO())
				if err == nil && bm.errContains != "" {
					b.Fatalf("unexpected patch succeeded, expected %s", bm.errContains)
				}
				if err != nil && !strings.Contains(string(result), bm.errContains) {
					b.Fatal(err)
				}
			}
		})
	}
}
