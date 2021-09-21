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
	"fmt"
	"strings"
	"sync"
	"unicode"
	"unicode/utf8"

	"github.com/google/cel-go/cel"
	"github.com/google/cel-go/checker/decls"
	"github.com/google/cel-go/common/types/ref"

	v1 "k8s.io/api/admission/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"k8s.io/apimachinery/pkg/util/sets"
)

var (
	admissionRequestExprType = decls.NewObjectType("k8s.io.api.admission.v1.AdmissionRequest")
	kubeTypes                = NewKubeTypeProvider()

	kubeEnv *cel.Env
	envInit sync.Once
)

type Program struct {
	Expression string
	Program    cel.Program
}

func NewProgram(expr string) (*Program, error) {
	envInit.Do(func() {
		var err error
		kubeEnv, err = cel.NewEnv(
			cel.CustomTypeAdapter(kubeTypes),
			cel.CustomTypeProvider(kubeTypes),
			cel.Declarations(
				decls.NewVar("request", admissionRequestExprType),
			),
			cel.Container("k8s.io.api.admission.v1"),
		)
		if err != nil {
			panic(fmt.Errorf("env error: %w", err))
		}
	})

	ast, issues := kubeEnv.Compile(expr)
	if issues != nil && issues.Err() != nil {
		return nil, fmt.Errorf("compile error: %w", issues.Err())
	}

	prg, err := kubeEnv.Program(ast)
	if err != nil {
		return nil, fmt.Errorf("program error: %w", err)
	}
	return &Program{Expression: expr, Program: prg}, nil
}

func (p *Program) Evaluate(ctx context.Context, request *v1.AdmissionRequest) (*v1.AdmissionResponse, error) {
	outValue, _, err := p.Program.Eval(map[string]interface{}{
		"request": request,
	})
	if err != nil {
		return nil, fmt.Errorf("eval error: %w", err)
	}
	return celValueToAdmissionResponse(outValue)
}

func celValueToAdmissionResponse(v ref.Val) (*v1.AdmissionResponse, error) {
	switch value := v.Value().(type) {

	case []ref.Val:
		// an empty list disallows, a non-empty list starts with Allow=true and requires all items to be Allow=true
		response := &v1.AdmissionResponse{Allowed: len(value) > 0}
		seenWarnings := sets.NewString()
		for _, sub := range value {
			// FIXME: handle null items in lists?

			subresponse, err := celValueToAdmissionResponse(sub)
			if err != nil {
				return nil, err
			}

			// allowed if both allow
			response.Allowed = response.Allowed && subresponse.Allowed
			// aggregate warnings
			for _, w := range subresponse.Warnings {
				if !seenWarnings.Has(w) {
					seenWarnings.Insert(w)
					response.Warnings = append(response.Warnings, w)
				}
			}
			// aggregate audit
			for k, v := range subresponse.AuditAnnotations {
				if response.AuditAnnotations == nil {
					response.AuditAnnotations = map[string]string{}
				}
				response.AuditAnnotations[k] = v
			}
			// aggregate status for forbidden results
			if !subresponse.Allowed && subresponse.Result != nil {
				if response.Result == nil {
					response.Result = subresponse.Result
				} else {
					// first non-empty code/status/reason sticks
					if response.Result.Code == 0 && len(response.Result.Status) == 0 && len(response.Result.Reason) == 0 {
						response.Result.Code = subresponse.Result.Code
						response.Result.Status = subresponse.Result.Status
						response.Result.Reason = subresponse.Result.Reason
					}
					// first non-empty details stick
					if response.Result.Details == nil {
						response.Result.Details = subresponse.Result.Details
					}
					// messages are aggregated
					if msg2 := strings.TrimSpace(subresponse.Result.Message); len(msg2) > 0 {
						if msg1 := strings.TrimSpace(response.Result.Message); len(msg1) > 0 {
							if lastRune, _ := utf8.DecodeLastRuneInString(msg1); unicode.IsPunct(lastRune) {
								response.Result.Message = msg1 + " " + msg2
							} else {
								response.Result.Message = msg1 + "; " + msg2
							}
						} else {
							response.Result.Message = msg2
						}
					}
				}
			}
			// FIXME: aggregate patch
		}
		return response, nil

	case bool:
		// boolean results indicate allowed/forbidden
		return &v1.AdmissionResponse{Allowed: value}, nil

	case string:
		// empty string results indicate allowed
		if len(value) == 0 {
			return &v1.AdmissionResponse{Allowed: true}, nil
		}

		// non-empty string results indicate forbidden with a reason
		return &v1.AdmissionResponse{Allowed: false, Result: &metav1.Status{Message: value}}, nil

	case *v1.AdmissionResponse:
		return value, nil

	default:
		// fmt.Printf("%#v\n", value)
		// FIXME: surface information from evaluation errors
		return nil, fmt.Errorf("unexpected result type %v", v.Type())
	}
}
