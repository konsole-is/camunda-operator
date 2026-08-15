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

package pgbootstrap

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestValidateIdentifier(t *testing.T) {
	tests := []struct {
		name       string
		identifier string
		wantErr    bool
	}{
		{"simple lowercase", "camunda", false},
		{"leading underscore", "_camunda", false},
		{"digits and underscores", "a1_b2", false},
		{"single letter", "a", false},
		{"single underscore", "_", false},
		{"63 characters", "d" + strings.Repeat("b", 62), false},
		{"empty", "", true},
		{"uppercase", "Camunda", true},
		{"leading digit", "1camunda", true},
		{"64 characters", "d" + strings.Repeat("b", 63), true},
		{"hyphen", "a-b", true},
		{"space", "a b", true},
		{"sql injection", `a";drop table x;--`, true},
		{"embedded quote", `a"b`, true},
		{"unicode", "café", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateIdentifier(tt.identifier)
			if tt.wantErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), "identifier")
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestQuoteIdentifier(t *testing.T) {
	assert.Equal(t, `"camunda"`, quoteIdentifier("camunda"))
	assert.Equal(t, `"a1_b2"`, quoteIdentifier("a1_b2"))
}

func TestQuoteLiteral(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{"plain", "abc", "'abc'"},
		{"empty", "", "''"},
		{"single quote doubled", "a'b", "'a''b'"},
		{"only quotes", "''", "''''''"},
		{"backslash preserved", `a\b`, ` E'a\\b'`},
		{"quote and backslash", `a'\b`, ` E'a''\\b'`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, quoteLiteral(tt.in))
		})
	}
}
