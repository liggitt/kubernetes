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

package flag

import "github.com/spf13/pflag"

// StringRefFlag is a string flag compatible with pflag.Value
type StringRefFlag struct {
	value *string
}

var _ pflag.Value = &StringRefFlag{}

// NewStringRef returns a string value which stores into the provided string pointer.
func NewStringRef(value *string) *StringRefFlag {
	return &StringRefFlag{value: value}
}

func (f *StringRefFlag) String() string {
	return *f.value
}

func (f *StringRefFlag) Set(value string) error {
	*f.value = value
	return nil
}

func (f *StringRefFlag) Type() string {
	return "string"
}
