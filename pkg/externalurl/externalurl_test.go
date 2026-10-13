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

package externalurl

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestParse(t *testing.T) {
	t.Parallel()

	parsed := Parse("https://camunda.example.com/optimize")
	assert.Equal(t, "camunda.example.com", parsed.Host)
	assert.Equal(t, "/optimize", parsed.Path)

	// A URL that does not parse yields the empty URL, not nil, so a caller
	// can read its fields.
	assert.Equal(t, "", Parse("http://[::1").Path)
	assert.Equal(t, "", Parse("").Path)
}

func TestContextPathAndRoutePrefix(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		raw         string
		contextPath string
		routePrefix string
	}{
		"no URL":             {raw: "", contextPath: "", routePrefix: "/"},
		"the root":           {raw: "https://optimize.example.com", contextPath: "", routePrefix: "/"},
		"the root and slash": {raw: "https://optimize.example.com/", contextPath: "", routePrefix: "/"},
		"a port":             {raw: "http://optimize.example.com:8080", contextPath: "", routePrefix: "/"},
		"a path": {
			raw: "https://camunda.example.com/optimize", contextPath: "/optimize", routePrefix: "/optimize",
		},
		"a path and slash": {
			raw: "https://camunda.example.com/optimize/", contextPath: "/optimize", routePrefix: "/optimize",
		},
		"a nested path": {
			raw: "https://camunda.example.com/a/optimize", contextPath: "/a/optimize", routePrefix: "/a/optimize",
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, tc.contextPath, ContextPath(Parse(tc.raw)))
			assert.Equal(t, tc.routePrefix, RoutePrefix(tc.raw))
		})
	}
}
