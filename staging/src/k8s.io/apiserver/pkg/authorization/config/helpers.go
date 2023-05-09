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

package config

import (
	"fmt"
)

var requiredErr = fmt.Errorf("required")

// appendErr is a helper function to collect field-specific errors.
func appendErr(errs []error, err error, field string) []error {
	if err != nil {
		return append(errs, fmt.Errorf("%s: %s", field, err.Error()))
	}
	return errs
}
