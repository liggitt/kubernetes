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

package openapitest

import (
	"embed"
	"errors"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"k8s.io/client-go/openapi"
)

//go:embed testdata/*_openapi.json
var f embed.FS

func NewTestClient(t *testing.T) openapi.Client {
	if t == nil {
		panic("non-nil testing.T required; this package is only for use in tests")
	}
	return &testClient{t: t}
}

type testClient struct {
	t     *testing.T
	init  sync.Once
	paths map[string]openapi.GroupVersion
	err   error
}

func (t *testClient) Paths() (map[string]openapi.GroupVersion, error) {
	t.init.Do(func() {
		t.paths = map[string]openapi.GroupVersion{}
		entries, err := f.ReadDir("testdata")
		if err != nil {
			t.err = err
			t.t.Error(err)
		}
		for _, e := range entries {
			// this reverses the transformation done in hack/update-openapi-spec.sh
			path := strings.Replace(strings.TrimSuffix(e.Name(), "_openapi.json"), "__", "/", -1)
			t.paths[path] = &testGroupVersion{t: t.t, filename: filepath.Join("testdata", e.Name())}
		}
	})
	return t.paths, t.err
}

type testGroupVersion struct {
	t        *testing.T
	init     sync.Once
	filename string
	data     []byte
	err      error
}

func (t *testGroupVersion) Schema(contentType string) ([]byte, error) {
	if contentType != "application/json" {
		return nil, errors.New("openapitest only supports 'application/json' contentType")
	}
	t.init.Do(func() {
		t.data, t.err = f.ReadFile(t.filename)
		if t.err != nil {
			t.t.Error(t.err)
		}
	})
	return t.data, t.err
}
