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

package strict

import (
	"context"
	"crypto/x509"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	admissionv1 "k8s.io/api/admission/v1"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	apiextensionsclient "k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset"
	"k8s.io/apiextensions-apiserver/test/integration/fixtures"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apiserver/pkg/features"
	utilfeature "k8s.io/apiserver/pkg/util/feature"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/util/cert"
	featuregatetesting "k8s.io/component-base/featuregate/testing"
	kubeapiservertesting "k8s.io/kubernetes/cmd/kube-apiserver/app/testing"
	"k8s.io/kubernetes/test/integration/framework"
)

func TestFieldValidation(t *testing.T) {
	defer featuregatetesting.SetFeatureGateDuringTest(t, utilfeature.DefaultFeatureGate, features.ServerSideFieldValidation, true)()

	server, err := kubeapiservertesting.StartTestServer(t, kubeapiservertesting.NewDefaultTestServerOptions(), nil, framework.SharedEtcd())
	if err != nil {
		t.Fatal(err)
	}
	config := server.ClientConfig
	defer server.TearDownFn()

	// don't log warnings, tests inspect them in the responses directly
	config.WarningHandler = rest.NoWarnings{}

	apiExtensionClient, err := apiextensionsclient.NewForConfig(config)
	if err != nil {
		t.Fatal(err)
	}
	dynamicClient, err := dynamic.NewForConfig(config)
	if err != nil {
		t.Fatal(err)
	}
	client, err := kubernetes.NewForConfig(config)
	if err != nil {
		t.Fatal(err)
	}

	webhookServer := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Log("webhook called")
		review := &admissionv1.AdmissionReview{}
		if err := json.NewDecoder(r.Body).Decode(review); err != nil {
			t.Error("webhook called, could not decode", err)
			return
		}
		s, _ := json.MarshalIndent(review, "", "  ")
		t.Logf("webhook called with:\n%s", string(s))
		patchType := admissionv1.PatchTypeJSONPatch
		review.Response = &admissionv1.AdmissionResponse{
			UID:       review.Request.UID,
			Allowed:   true,
			Patch:     []byte(`[{"op":"replace","path":"/field","value":"800m"}]`),
			PatchType: &patchType,
		}
		json.NewEncoder(w).Encode(review)
	}))
	ca, err := x509.ParseCertificate(webhookServer.TLS.Certificates[0].Certificate[0])
	if err != nil {
		t.Fatal(err)
	}
	caPEM, err := cert.EncodeCertificates(ca)
	if err != nil {
		t.Fatal(err)
	}
	defer webhookServer.Close()

	// register a mutating webhook to intecept the CRD
	none := admissionregistrationv1.SideEffectClassNone
	_, err = client.AdmissionregistrationV1().MutatingWebhookConfigurations().Create(context.TODO(), &admissionregistrationv1.MutatingWebhookConfiguration{
		ObjectMeta: metav1.ObjectMeta{Name: "test.example.com"},
		Webhooks: []admissionregistrationv1.MutatingWebhook{{
			Name: "test.example.com",
			ClientConfig: admissionregistrationv1.WebhookClientConfig{
				URL:      &webhookServer.URL,
				CABundle: caPEM,
			},
			SideEffects:             &none,
			AdmissionReviewVersions: []string{"v1"},
			ObjectSelector:          &metav1.LabelSelector{MatchLabels: map[string]string{"strict": "true"}},
			Rules: []admissionregistrationv1.RuleWithOperations{{
				Operations: []admissionregistrationv1.OperationType{admissionregistrationv1.OperationAll},
				Rule:       admissionregistrationv1.Rule{APIGroups: []string{"mygroup.example.com"}, APIVersions: []string{"*"}, Resources: []string{"noxus"}},
			}},
		}},
	}, metav1.CreateOptions{})
	if err != nil {
		t.Fatal(err)
	}

	// create the CRD
	crd := &apiextensionsv1.CustomResourceDefinition{
		ObjectMeta: metav1.ObjectMeta{Name: "noxus.mygroup.example.com"},
		Spec: apiextensionsv1.CustomResourceDefinitionSpec{
			Group: "mygroup.example.com",
			Versions: []apiextensionsv1.CustomResourceDefinitionVersion{{
				Name:    "v1beta1",
				Served:  true,
				Storage: true,
				Schema: &apiextensionsv1.CustomResourceValidation{
					OpenAPIV3Schema: &apiextensionsv1.JSONSchemaProps{
						Type: "object",
						Properties: map[string]apiextensionsv1.JSONSchemaProps{
							"field": {
								XIntOrString: true,
								AnyOf: []apiextensionsv1.JSONSchemaProps{
									{Type: "integer"},
									{Type: "string"},
								},
							},
						},
					},
				},
			}},
			Names: apiextensionsv1.CustomResourceDefinitionNames{
				Plural:     "noxus",
				Singular:   "noxu",
				Kind:       "WishIHadChosenNoxu",
				ShortNames: []string{"foo", "bar", "abc", "def"},
				ListKind:   "NoxuItemList",
				Categories: []string{"all"},
			},
			Scope: apiextensionsv1.ClusterScoped,
		},
	}

	// install the CRD
	schemaCRD, err := fixtures.CreateNewV1CustomResourceDefinition(crd, apiExtensionClient, dynamicClient)
	if err != nil {
		t.Fatal(err)
	}

	schemaGVR := schema.GroupVersionResource{
		Group:    schemaCRD.Spec.Group,
		Version:  schemaCRD.Spec.Versions[0].Name,
		Resource: schemaCRD.Spec.Names.Plural,
	}
	schemaGVK := schema.GroupVersionKind{
		Group:   schemaCRD.Spec.Group,
		Version: schemaCRD.Spec.Versions[0].Name,
		Kind:    schemaCRD.Spec.Names.Kind,
	}

	result, err := dynamicClient.Resource(schemaGVR).Create(
		context.TODO(),
		&unstructured.Unstructured{Object: map[string]interface{}{
			"apiVersion": schemaGVK.GroupVersion().String(),
			"kind":       schemaGVK.Kind,
			"metadata": map[string]interface{}{
				"name":   "test",
				"labels": map[string]interface{}{"strict": "true"},
			},
			"field": 0.8,
		}},
		metav1.CreateOptions{FieldValidation: "Strict"},
	)
	if err != nil {
		t.Log("error creating object", err)
	} else {
		s, _ := json.MarshalIndent(result, "", "  ")
		t.Logf("object created as:\n%s", string(s))
	}
}
