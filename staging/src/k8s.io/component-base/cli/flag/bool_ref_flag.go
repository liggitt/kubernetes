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

import (
	"strconv"

	"github.com/spf13/pflag"
)

// BoolRefFlag is a bool flag compatible with pflag.Value
type BoolRefFlag struct {
	value *bool
}

var _ pflag.Value = &BoolRefFlag{}

// NewBoolRef returns a value which stores into the provided bool pointer.
func NewBoolRef(value *bool) *BoolRefFlag {
	return &BoolRefFlag{value: value}
}

func (f *BoolRefFlag) String() string {
	return strconv.FormatBool(*f.value)
}

func (f *BoolRefFlag) Set(value string) error {
	b, err := strconv.ParseBool(value)
	*f.value = b
	return err
}

func (f *BoolRefFlag) Type() string {
	return "bool"
}

func (f *BoolRefFlag) IsBoolFlag() bool {
	return true
}
