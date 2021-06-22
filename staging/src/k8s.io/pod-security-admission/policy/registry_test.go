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

package policy

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/pod-security-admission/api"
)

func TestCheckRegistry_empty(t *testing.T) {
	reg := NewCheckRegistry()

	_, err := reg.CheckForIDAndVersion("noexist", api.LatestVersion())
	assert.Error(t, err, "nonexistant ID")

	_, err = reg.ChecksForLevelAndVersion("foo-bar", api.LatestVersion())
	assert.Error(t, err, "invalid level")

	emptyCases := []struct {
		level   api.Level
		version string
	}{
		{api.LevelPrivileged, "latest"},
		{api.LevelPrivileged, "v1.0"},
		{api.LevelBaseline, "latest"},
		{api.LevelBaseline, "v1.10"},
		{api.LevelRestricted, "latest"},
		{api.LevelRestricted, "v1.20"},
	}
	for _, test := range emptyCases {
		checks, err := reg.ChecksForLevelAndVersion(test.level, versionOrPanic(test.version))
		assert.Emptyf(t, checks, "%s:%s", test.level, test.version)
		assert.NoError(t, err, "%s:%s", test.level, test.version)
	}
}

func TestCheckRegistry(t *testing.T) {
	reg := NewCheckRegistry()

	reg.AddCheck(api.LevelBaseline, checksForIDAndVersions("a", []string{"v1.0"}))
	reg.AddCheck(api.LevelBaseline, checksForIDAndVersions("b", []string{"v1.10"}))
	reg.AddCheck(api.LevelBaseline, checksForIDAndVersions("c", []string{"v1.0", "v1.5", "v1.10"}))
	reg.AddCheck(api.LevelBaseline, checksForIDAndVersions("d", []string{"v1.11", "v1.15", "v1.20"}))

	reg.AddCheck(api.LevelRestricted, checksForIDAndVersions("e", []string{"v1.0"}))
	reg.AddCheck(api.LevelRestricted, checksForIDAndVersions("f", []string{"v1.12", "v1.16", "v1.21"}))

	// Test ChecksForLevelAndVersion
	levelCases := []struct {
		level    api.Level
		version  string
		expected []string
	}{
		{api.LevelPrivileged, "v1.0", nil},
		{api.LevelPrivileged, "latest", nil},
		{api.LevelBaseline, "v1.0", []string{"a:v1.0", "c:v1.0"}},
		{api.LevelBaseline, "v1.4", []string{"a:v1.0", "c:v1.0"}},
		{api.LevelBaseline, "v1.5", []string{"a:v1.0", "c:v1.5"}},
		{api.LevelBaseline, "v1.10", []string{"a:v1.0", "b:v1.10", "c:v1.10"}},
		{api.LevelBaseline, "v1.11", []string{"a:v1.0", "b:v1.10", "c:v1.10", "d:v1.11"}},
		{api.LevelBaseline, "latest", []string{"a:v1.0", "b:v1.10", "c:v1.10", "d:v1.20"}},
		{api.LevelRestricted, "v1.0", []string{"a:v1.0", "c:v1.0", "e:v1.0"}},
		{api.LevelRestricted, "v1.4", []string{"a:v1.0", "c:v1.0", "e:v1.0"}},
		{api.LevelRestricted, "v1.5", []string{"a:v1.0", "c:v1.5", "e:v1.0"}},
		{api.LevelRestricted, "v1.10", []string{"a:v1.0", "b:v1.10", "c:v1.10", "e:v1.0"}},
		{api.LevelRestricted, "v1.11", []string{"a:v1.0", "b:v1.10", "c:v1.10", "d:v1.11", "e:v1.0"}},
		{api.LevelRestricted, "latest", []string{"a:v1.0", "b:v1.10", "c:v1.10", "d:v1.20", "e:v1.0", "f:v1.21"}},
		{api.LevelRestricted, "v1.10000", []string{"a:v1.0", "b:v1.10", "c:v1.10", "d:v1.20", "e:v1.0", "f:v1.21"}},
	}
	for _, test := range levelCases {
		t.Run(fmt.Sprintf("ChecksForLevelAndVersion(%s,%s)", test.level, test.version), func(t *testing.T) {
			checks, err := reg.ChecksForLevelAndVersion(test.level, versionOrPanic(test.version))
			require.NoError(t, err)

			// Set up checks returned in {id:version} format
			var actual []string
			for _, c := range checks {
				cc := c.(versionedCheck)
				actual = append(actual, fmt.Sprintf("%s:%s", cc.ID(), cc.firstVersion.String()))
			}
			assert.ElementsMatch(t, test.expected, actual)
		})
	}

	// Test CheckForIDAndVersion
	idCases := []struct {
		id, version, expected string
	}{
		{"foo", "latest", ""},
		{"d", "v1.10", ""},
		{"d", "v1.11", "v1.11"},
		{"d", "v1.12", "v1.11"},
		{"d", "v1.14", "v1.11"},
		{"d", "v1.15", "v1.15"},
		{"d", "latest", "v1.20"},
		{"e", "latest", "v1.0"},
	}
	for _, test := range idCases {
		t.Run(fmt.Sprintf("CheckForIDAndVersion(%s,%s)", test.id, test.version), func(t *testing.T) {
			c, err := reg.CheckForIDAndVersion(test.id, versionOrPanic(test.version))
			if test.expected != "" {
				require.NoError(t, err)
				cc := c.(versionedCheck)
				assert.Equal(t, test.id, cc.ID())
				assert.Equal(t, test.expected, cc.firstVersion.String())
			} else {
				assert.Error(t, err)
			}
		})
	}

}

func checksForIDAndVersions(id string, versions []string) map[string]Check {
	checks := map[string]Check{}
	for _, v := range versions {
		checks[v] = &check{id: id}
	}
	return checks
}

func versionOrPanic(v string) api.Version {
	ver, err := api.VersionToEvaluate(v)
	if err != nil {
		panic(err)
	}
	return ver
}
