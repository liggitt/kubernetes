// +build linux

/*
Copyright 2020 The Kubernetes Authors.

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

package iptables

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"k8s.io/apimachinery/pkg/util/wait"
)

func TestMain(m *testing.M) {
	rc := 1
	defer os.Exit(rc)

	// the tests in this package mutate global state, ensure we only run one set of them at once
	lockfile := filepath.Join(os.TempDir(), "kubernetes_iptables_test.lock")
	err := wait.PollImmediate(time.Second, wait.ForeverTestTimeout, func() (bool, error) {
		f, err := os.OpenFile(lockfile, os.O_CREATE|os.O_EXCL, 0)
		if err != nil {
			fmt.Printf("error locking %s, will retry: %v\n", lockfile, err)
			return false, nil
		}
		f.Close()
		return true, nil
	})
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
	defer os.Remove(lockfile)

	rc = m.Run()
}
