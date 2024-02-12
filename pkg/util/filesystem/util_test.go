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

package filesystem

import (
	"net"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestIsUnixDomainSocket(t *testing.T) {
	tests := []struct {
		label          string
		listenOnSocket bool
		expectSocket   bool
		expectError    bool
		invalidFile    bool
	}{
		{
			label:          "Domain Socket file",
			listenOnSocket: true,
			expectSocket:   true,
			expectError:    false,
		},
		{
			label:       "Non Existent file",
			invalidFile: true,
			expectError: true,
		},
		{
			label:          "Regular file",
			listenOnSocket: false,
			expectSocket:   false,
			expectError:    false,
		},
	}
	for _, test := range tests {
		f, err := os.CreateTemp("", "test-domain-socket")
		require.NoErrorf(t, err, "Failed to create file for test purposes: %v while setting up: %s", err, test.label)
		addr := f.Name()
		f.Close()
		var ln *net.UnixListener
		if test.listenOnSocket {
			os.Remove(addr)
			ta, err := net.ResolveUnixAddr("unix", addr)
			require.NoErrorf(t, err, "Failed to ResolveUnixAddr: %v while setting up: %s", err, test.label)
			ln, err = net.ListenUnix("unix", ta)
			require.NoErrorf(t, err, "Failed to ListenUnix: %v while setting up: %s", err, test.label)
		}
		fileToTest := addr
		if test.invalidFile {
			fileToTest = fileToTest + ".invalid"
		}
		result, err := IsUnixDomainSocket(fileToTest)
		if test.listenOnSocket {
			// this takes care of removing the file associated with the domain socket
			ln.Close()
		} else {
			// explicitly remove regular file
			os.Remove(addr)
		}
		if test.expectError {
			assert.Errorf(t, err, "Unexpected nil error from IsUnixDomainSocket for %s", test.label)
		} else {
			assert.NoErrorf(t, err, "Unexpected error invoking IsUnixDomainSocket for %s", test.label)
		}
		assert.Equal(t, result, test.expectSocket, "Unexpected result from IsUnixDomainSocket: %v for %s", result, test.label)
	}
}

func TestSymbolicLinkDir(t *testing.T) {
	dir := t.TempDir()

	if err := os.Mkdir(filepath.Join(dir, "v1"), os.FileMode(0755)); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "v1", "file"), []byte(`v1`), os.FileMode(0644)); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(dir, "v2"), os.FileMode(0755)); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "v2", "file"), []byte(`v2`), os.FileMode(0644)); err != nil {
		t.Fatal(err)
	}

	if err := os.Symlink(filepath.Join(dir, "v1"), filepath.Join(dir, "v")); err != nil {
		t.Fatal(err)
	}

	fileWatcher := NewFsnotifyWatcher()
	fileWatcher.Init(
		func(event fsnotify.Event) {
			t.Log("fileWatcher", event)
			data, err := os.ReadFile(filepath.Join(dir, "v", "file"))
			if err != nil {
				t.Error("fileWatcher", err)
			}
			t.Log("fileWatcher", string(data))
		},
		func(err error) {
			t.Error("fileWatcher", err)
		},
	)
	if err := fileWatcher.AddWatch(filepath.Join(dir, "v", "file")); err != nil {
		t.Fatal(err)
	}

	dirWatcher := NewFsnotifyWatcher()
	dirWatcher.Init(
		func(event fsnotify.Event) {
			t.Log("dirWatcher", event)
			data, err := os.ReadFile(filepath.Join(dir, "v", "file"))
			if err != nil {
				t.Error("dirWatcher", err)
			}
			t.Log("dirWatcher", string(data))
		},
		func(err error) {
			t.Error("dirWatcher", err)
		},
	)
	if err := dirWatcher.AddWatch(filepath.Join(dir, "v")); err != nil {
		t.Fatal(err)
	}

	if err := os.Symlink(filepath.Join(dir, "v2"), filepath.Join(dir, "v.tmp")); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(filepath.Join(dir, "v.tmp"), filepath.Join(dir, "v")); err != nil {
		t.Fatal(err)
	}

	time.Sleep(time.Second)
}
