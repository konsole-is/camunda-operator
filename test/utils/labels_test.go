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

package utils

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLabelFilter(t *testing.T) {
	known := []string{"a", "b", "c"}

	tests := []struct {
		name string
		list string
		want string
	}{
		{name: "empty list selects every spec", list: "", want: ""},
		{name: "blank entries are ignored", list: " , ", want: ""},
		{name: "one label", list: "a", want: "(a)"},
		{name: "two labels", list: "a, b", want: "(a || b)"},
		{name: "exclusion alone", list: "!c", want: "!c"},
		{name: "selection with exclusion", list: "a,b,!c", want: "(a || b) && !c"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := LabelFilter(tt.list, known)
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}

	t.Run("unknown label is an error", func(t *testing.T) {
		_, err := LabelFilter("a,d", known)
		require.Error(t, err)
		assert.Contains(t, err.Error(), `"d"`)
	})

	t.Run("unknown excluded label is an error", func(t *testing.T) {
		_, err := LabelFilter("!d", known)
		require.Error(t, err)
	})
}
