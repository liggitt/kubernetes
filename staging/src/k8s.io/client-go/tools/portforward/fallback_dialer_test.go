/*
Copyright 2024 The Kubernetes Authors.

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

package portforward

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestFallbackDialer(t *testing.T) {
	// If "shouldFallback" is false, then only primary should be dialed.
	primary := &fakeDialer{dialed: false}
	secondary := &fakeDialer{dialed: false}
	fallbackDialer := NewFallbackDialer(primary, secondary, alwaysFalse)
	_, _, _ = fallbackDialer.Dial("unused")
	assert.True(t, primary.dialed, "no fallback; primary should have dialed")
	assert.False(t, secondary.dialed, "no fallback; secondary should *not* have dialed")
	// If "shouldFallback" is true, then primary AND secondary should be dialed.
	primary.dialed = false   // reset dialed field
	secondary.dialed = false // reset dialed field
	fallbackDialer = NewFallbackDialer(primary, secondary, alwaysTrue)
	_, _, _ = fallbackDialer.Dial("unused")
	assert.True(t, primary.dialed, "fallback; primary should have dialed (first)")
	assert.True(t, secondary.dialed, "fallback; secondary should have dialed")
}

func alwaysTrue(err error) bool { return true }

func alwaysFalse(err error) bool { return false }
