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

package hostfold

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestFoldHost(t *testing.T) {
	cases := map[string]struct {
		host   string
		folded string
	}{
		"a name in upper case":             {host: "ES.Data.SVC", folded: "es.data.svc"},
		"the trailing dot of the DNS root": {host: "es.data.svc.", folded: "es.data.svc"},
		"an internationalized name":        {host: "BÜCHER.example", folded: "xn--bcher-kva.example"},
		"the punycode of that name":        {host: "xn--bcher-kva.example", folded: "xn--bcher-kva.example"},
		"a long IPv6 spelling":             {host: "0:0:0:0:0:0:0:1", folded: "::1"},
		"an IPv6 literal":                  {host: "::1", folded: "::1"},
		"an IPv4 literal":                  {host: "10.0.0.1", folded: "10.0.0.1"},
		// The dot alone is the DNS root. Trimming it would leave an empty host,
		// which names nothing at all, so it keeps the spelling it came with.
		"the DNS root alone": {host: ".", folded: "."},
		"an empty host":      {host: "", folded: ""},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			assert.Equal(t, tc.folded, FoldHost(tc.host))
		})
	}
}
