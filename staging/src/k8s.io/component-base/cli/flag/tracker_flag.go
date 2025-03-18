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

// TrackerValue wraps a value and stores true in the provided boolean when it is set
type TrackerValue struct {
	value    pflag.Value
	provided *bool
}

var _ pflag.Value = &TrackerValue{}

// NewTracker returns a Value wrapping the given value which stores true in the provided boolean when it is set
func NewTracker(value pflag.Value, provided *bool) *TrackerValue {
	return &TrackerValue{value: value, provided: provided}
}

func (f *TrackerValue) String() string {
	return f.value.String()
}

func (f *TrackerValue) Set(value string) error {
	err := f.value.Set(value)
	if err == nil {
		*f.provided = true
	}
	return err
}

func (f *TrackerValue) Type() string {
	return f.value.Type()
}

type boolFlag interface {
	IsBoolFlag() bool
}

func (f *TrackerValue) IsBoolFlag() bool {
	if b, ok := f.value.(boolFlag); ok {
		return b.IsBoolFlag()
	}
	return false
}
