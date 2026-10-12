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

package accounthash_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/konsole-is/camunda-operator/pkg/accounthash"
)

func TestOfAnEmptySetIsEmpty(t *testing.T) {
	t.Parallel()

	assert.Empty(t, accounthash.Of(nil))
	assert.Empty(t, accounthash.Of(map[string]string{}))
}

// Go randomizes the iteration order of a map, so many keys make an order
// dependence show up across the calls.
func TestOfIsStable(t *testing.T) {
	t.Parallel()

	annotations := map[string]string{}
	for _, key := range []string{"a", "b", "c", "d", "e", "f", "g", "h", "i", "j"} {
		annotations["example.com/"+key] = key
	}

	first := accounthash.Of(annotations)
	assert.NotEmpty(t, first)
	for range 20 {
		assert.Equal(t, first, accounthash.Of(annotations))
	}
}

// A value that holds the characters that could separate two entries must not
// give the hash of those two entries.
func TestOfTellsAnnotationSetsApart(t *testing.T) {
	t.Parallel()

	assert.NotEqual(
		t,
		accounthash.Of(map[string]string{"a": "x\nb=y"}),
		accounthash.Of(map[string]string{"a": "x", "b": "y"}),
	)
	assert.NotEqual(
		t,
		accounthash.Of(map[string]string{"a": "x"}),
		accounthash.Of(map[string]string{"a": "y"}),
	)
}
