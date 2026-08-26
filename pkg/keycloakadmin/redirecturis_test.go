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

package keycloakadmin

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// callbackSuffix is the login path of Optimize, the suffix that marks an entry
// as one the operator can remove.
const callbackSuffix = "/api/authentication/callback"

func TestMergeRedirectURIs(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		current []string
		desired []string
		want    []string
	}{
		{
			name:    "adds a missing callback after the entries that are there",
			current: []string{"https://a.example.com" + callbackSuffix},
			desired: []string{
				"https://a.example.com" + callbackSuffix,
				"https://b.example.com" + callbackSuffix,
			},
			want: []string{
				"https://a.example.com" + callbackSuffix,
				"https://b.example.com" + callbackSuffix,
			},
		},
		{
			name: "removes a callback that is no longer wanted",
			current: []string{
				"https://a.example.com" + callbackSuffix,
				"https://gone.example.com" + callbackSuffix,
			},
			desired: []string{"https://a.example.com" + callbackSuffix},
			want:    []string{"https://a.example.com" + callbackSuffix},
		},
		{
			name: "keeps an entry that does not end in the callback path",
			current: []string{
				"https://a.example.com/*",
				"https://gone.example.com" + callbackSuffix,
			},
			desired: []string{"https://b.example.com" + callbackSuffix},
			want: []string{
				"https://a.example.com/*",
				"https://b.example.com" + callbackSuffix,
			},
		},
		{
			name:    "an empty list of wanted callbacks withdraws every callback",
			current: []string{"https://a.example.com" + callbackSuffix, "https://a.example.com/*"},
			want:    []string{"https://a.example.com/*"},
		},
		{
			name: "a list that needs no change comes back unchanged",
			current: []string{
				"https://a.example.com" + callbackSuffix,
				"https://b.example.com" + callbackSuffix,
			},
			desired: []string{
				"https://a.example.com" + callbackSuffix,
				"https://b.example.com" + callbackSuffix,
			},
			want: []string{
				"https://a.example.com" + callbackSuffix,
				"https://b.example.com" + callbackSuffix,
			},
		},
		{
			name: "a duplicate of the stored list is written once",
			current: []string{
				"https://a.example.com" + callbackSuffix,
				"https://a.example.com" + callbackSuffix,
			},
			desired: []string{"https://a.example.com" + callbackSuffix},
			want:    []string{"https://a.example.com" + callbackSuffix},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, tt.want, MergeRedirectURIs(tt.current, tt.desired, callbackSuffix))
		})
	}
}

func TestRepresentationAccessors(t *testing.T) {
	t.Parallel()

	rep := Representation{
		"id":           "6c4c0c5c",
		"clientId":     "optimize",
		"redirectUris": []any{"https://a.example.com" + callbackSuffix, 42},
	}

	assert.Equal(t, "6c4c0c5c", rep.ID())
	assert.Equal(t, []string{"https://a.example.com" + callbackSuffix}, rep.RedirectURIs())

	rep.SetRedirectURIs([]string{"https://b.example.com" + callbackSuffix})

	assert.Equal(t, []string{"https://b.example.com" + callbackSuffix}, rep.RedirectURIs())
	assert.Equal(t, "optimize", rep["clientId"])
}

func TestRepresentationWithoutFields(t *testing.T) {
	t.Parallel()

	rep := Representation{}

	assert.Empty(t, rep.ID())
	assert.Empty(t, rep.RedirectURIs())
}
