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

package podsecuritystandards

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

const (
	AllowLabel = "podsecurity.kubernetes.io/allow"
	AuditLabel = "podsecurity.kubernetes.io/audit"
	WarnLabel  = "podsecurity.kubernetes.io/warn"

	AllowVersionLabel = "podsecurity.kubernetes.io/allow.version"
	AuditVersionLabel = "podsecurity.kubernetes.io/audit.version"
	WarnVersionLabel  = "podsecurity.kubernetes.io/warn.version"

	LevelPrivileged = "privileged"
	LevelBaseline   = "baseline"
	LevelRestricted = "restricted"

	LatestVersionString = "latest"
)

var levelKeys = []string{AllowLabel, AuditLabel, WarnLabel}
var levelToVersionKeys = map[string]string{
	AllowLabel: AllowVersionLabel,
	AuditLabel: AuditVersionLabel,
	WarnLabel:  WarnVersionLabel,
}

var validLevels = []string{LevelPrivileged, LevelBaseline, LevelRestricted}

var versionRegexp = regexp.MustCompile(`^v1\.([0-9]|[1-9][0-9]*)$`)

// levelToEvaluate returns the level that should be evaluated.
// level must be "privileged", "baseline", or "restricted".
// if level does not match one of those strings, "restricted" and an error is returned.
func LevelToEvaluate(level string) (string, error) {
	switch level {
	case LevelPrivileged, LevelBaseline, LevelRestricted:
		return level, nil
	default:
		return LevelRestricted, fmt.Errorf(`must be one of %s`, strings.Join(validLevels, ", "))
	}
}

type Version struct {
	minor  int
	latest bool
}

func (v Version) Empty() bool {
	return v.minor == 0 && !v.latest
}

// Returns true if minor is set in both and v.minor > v2.minor.
func (v Version) GT(v2 Version) bool {
	return v.minor != 0 && v2.minor != 0 && v.minor > v2.minor
}

// Returns true if minor is set in both and v.minor < v2.minor.
func (v Version) LT(v2 Version) bool {
	return v.minor != 0 && v2.minor != 0 && v.minor < v2.minor
}

// versionToEvaluate returns the policy version that should be evaluated.
// version must be "latest" or "v1.x".
// If version does not match one of those patterns, the latest version and an error is returned.
func VersionToEvaluate(version string) (Version, error) {
	if version == "latest" {
		return Version{latest: true}, nil
	}
	match := versionRegexp.FindStringSubmatch(version)
	if len(match) != 2 {
		return Version{latest: true}, fmt.Errorf(`must be "latest" or "v1.x"`)
	}
	versionNumber, err := strconv.Atoi(match[1])
	if err != nil || versionNumber <= 0 {
		return Version{latest: true}, fmt.Errorf(`must be "latest" or "v1.x"`)
	}
	return Version{minor: versionNumber}, nil
}
