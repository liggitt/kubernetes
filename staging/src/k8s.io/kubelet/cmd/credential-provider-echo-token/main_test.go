/*
Copyright 2025 The Kubernetes Authors.

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

package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	v1 "k8s.io/kubelet/pkg/apis/credentialprovider/v1"
)

func TestRequest(t *testing.T) {
	files, err := os.ReadDir("testdata")
	if err != nil {
		t.Fatal(err)
	}
	for _, file := range files {
		t.Run(file.Name(), func(t *testing.T) {
			f, err := os.Open(filepath.Join("testdata", file.Name()))
			if err != nil {
				t.Fatal(err)
			}
			defer f.Close()

			out := bytes.NewBuffer(nil)
			err = handle("myuser", f, out)
			if out.Len() > 0 {
				t.Logf("got output:\n%v", out.String())
			}
			switch {
			case strings.HasPrefix(file.Name(), "invalid"):
				if err == nil {
					t.Fatal("expected error, got none")
				}
				t.Logf("expected error, got:\n%v", err)
			case strings.HasPrefix(file.Name(), "valid"):
				if err != nil {
					t.Fatalf("expected no error, got:\n%v", err)
				}
				result := &v1.CredentialProviderResponse{}
				if err := json.NewDecoder(out).Decode(result); err != nil {
					t.Fatalf("error decoding response: %v", err)
				}
				if want, got := "myuser", result.Auth["*"].Username; want != got {
					t.Errorf("unexpected user, want=%v, got=%v", want, got)
				}
				if want, got := "mytoken", result.Auth["*"].Password; want != got {
					t.Errorf("unexpected password, want=%v, got=%v", want, got)
				}
			default:
				t.Fatal("test file must start with 'invalid' or 'valid'")
			}
		})
	}
}
