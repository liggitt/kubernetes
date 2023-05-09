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
	"testing"

	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/apimachinery/pkg/util/validation/field"
	api "k8s.io/apiserver/pkg/authorization/config"
)

type (
	test struct {
		configuration   api.AuthorizationConfiguration
		expectedErrList field.ErrorList
	}
)

const (
	invalidValueUppercase = "TEST"
	invalidValueChars     = "_&$%#"
	invalidValueTooLong   = "testtesttesttesttesttesttesttesttesttesttesttesttesttesttesttest"
	invalidValueEmpty     = ""
	validValue            = "testing"
)

var (
	knownTypes      = sets.NewString()
	repeatableTypes = sets.NewString()
)

func TestValidateAuthorizationConfiguration(t *testing.T) {
	tests := []test{}

	for _, test := range tests {
		errList := ValidateAuthorizationConfiguration(nil, &test.configuration, knownTypes, repeatableTypes)
		if len(errList) != len(test.expectedErrList) {
			t.Errorf("expected %d errs, got %d", len(test.expectedErrList), len(errList))
		}

		for i, expected := range test.expectedErrList {
			if expected.Type.String() != errList[i].Type.String() {
				t.Errorf("expected err type %s, got %s",
					expected.Type.String(),
					errList[i].Type.String())
			}
			if expected.BadValue != errList[i].BadValue {
				t.Errorf("expected bad value '%s', got '%s'",
					expected.BadValue,
					errList[i].BadValue)
			}
		}
	}
}
