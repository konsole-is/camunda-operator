/*
Copyright 2026.

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

package version

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestVersionWithoutStampIsDev(t *testing.T) {
	assert.Equal(t, "0.0.0-dev", Version())
}

// TestVersionReportsStampedTag runs only from make test-version-stamp, which
// builds the test with the ldflags of a release build.
func TestVersionReportsStampedTag(t *testing.T) {
	want := os.Getenv("WANT_OPERATOR_VERSION")
	if want == "" {
		t.Skip("WANT_OPERATOR_VERSION is not set")
	}

	assert.Equal(t, want, Version())
}

func TestInRangeAcceptsEveryRangeOnADevBuild(t *testing.T) {
	for _, rangeExpr := range []string{">=0.3.0 <0.5.0", "^0.3", "0.3.x", ">=99.0.0", "<0.0.1"} {
		ok, err := InRange(rangeExpr)

		require.NoError(t, err, rangeExpr)
		assert.True(t, ok, rangeExpr)
	}
}

func TestSatisfies(t *testing.T) {
	tests := []struct {
		name      string
		version   string
		rangeExpr string
		want      bool
	}{
		{name: "inside a bounded range", version: "0.4.1", rangeExpr: ">=0.3.0 <0.5.0", want: true},
		{name: "on the lower bound", version: "0.3.0", rangeExpr: ">=0.3.0 <0.5.0", want: true},
		{name: "on the excluded upper bound", version: "0.5.0", rangeExpr: ">=0.3.0 <0.5.0", want: false},
		{name: "below the range", version: "0.2.9", rangeExpr: ">=0.3.0 <0.5.0", want: false},
		{name: "comma form", version: "0.4.0", rangeExpr: ">= 0.3.0, < 1.0.0", want: true},
		{name: "caret on a zero major locks the minor", version: "0.4.1", rangeExpr: "^0.3", want: false},
		{name: "dev is inside a range it would fail", version: "0.0.0-dev", rangeExpr: ">=99.0.0", want: true},
		{name: "a prerelease is outside a plain range", version: "0.3.0-rc.1", rangeExpr: ">=0.3.0", want: false},
		{name: "a prerelease is inside a prerelease range", version: "0.3.0-rc.1", rangeExpr: ">=0.3.0-0", want: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Satisfies(tt.version, tt.rangeExpr)

			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestSatisfiesRefusesAMalformedRange(t *testing.T) {
	for _, v := range []string{"0.4.1", Dev} {
		for _, rangeExpr := range []string{"", "bogus"} {
			_, err := Satisfies(v, rangeExpr)

			assert.Error(t, err, "version %q, range %q", v, rangeExpr)
		}
	}
}

func TestSatisfiesRefusesAVersionThatIsNotSemver(t *testing.T) {
	for _, v := range []string{"main", "1", "0.3", "v0.3.0"} {
		_, err := Satisfies(v, ">=0.3.0")

		require.Error(t, err, v)
		assert.Contains(t, err.Error(), `"`+v+`"`)
	}
}
