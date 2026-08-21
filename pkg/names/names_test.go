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

package names

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"k8s.io/apimachinery/pkg/util/validation"
)

func TestBoundedKeepsANameThatFits(t *testing.T) {
	t.Parallel()

	assert.Equal(t, "my-cluster", Bounded("my-cluster", 63))
	assert.Equal(t, "abc", Bounded("abc", 3), "a name of exactly the limit is untouched")
	assert.Equal(t, "", Bounded("", 63))
}

func TestBoundedTruncatesALongNameToTheLimit(t *testing.T) {
	t.Parallel()

	for _, limit := range []int{12, 20, 63, 100} {
		got := Bounded(strings.Repeat("a", 253), limit)
		assert.LessOrEqual(t, len(got), limit, limit)
		assert.Equal(t, limit, len(got), "%d: the head fills the limit", limit)
	}
}

// The derived name is a function of the whole resource name, so two long
// names that agree on the truncated head still render apart.
func TestBoundedKeepsNamesWithASharedHeadApart(t *testing.T) {
	t.Parallel()

	head := strings.Repeat("a", 80)
	first := Bounded(head+"one", 63)
	second := Bounded(head+"two", 63)

	assert.NotEqual(t, first, second)
	assert.Equal(t, first, Bounded(head+"one", 63), "the name is deterministic")
}

// A truncation that lands on a dash or a dot would render a name that
// Kubernetes rejects, because a DNS label starts and ends alphanumeric.
func TestBoundedTrimsASeparatorOffTheHead(t *testing.T) {
	t.Parallel()

	// The head is cut at 13 characters, which lands on the dash.
	got := Bounded(strings.Repeat("a", 12)+"-"+strings.Repeat("b", 60), 24)

	assert.Equal(t, strings.Repeat("a", 12)+"-", got[:13], "the dash is the joiner, not the cut")
	assert.NotContains(t, got, "--")
	assert.Empty(t, validation.IsDNS1123Label(got), got)
}

// A limit with no room for a head and a hash still returns something inside
// the limit, rather than panicking on a negative slice bound.
func TestBoundedHandlesALimitSmallerThanTheHash(t *testing.T) {
	t.Parallel()

	for _, limit := range []int{0, 1, 5, HashLength + 1} {
		got := Bounded(strings.Repeat("a", 253), limit)
		assert.Len(t, got, limit, limit)
	}

	assert.NotEqual(
		t,
		Bounded(strings.Repeat("a", 253), 5),
		Bounded(strings.Repeat("b", 253), 5),
		"a short bound still separates two names",
	)
}
