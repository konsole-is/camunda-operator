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

// Package externalurl reads the external URL of a Camunda app: the URL where
// browsers reach it. An app that runs under a path of its own has to be told
// that path, and a route to it takes that path.
package externalurl

import (
	"net/url"
	"strings"
)

// Parse reads an external URL of a spec. It yields the empty URL for a URL
// that does not parse, never nil. The CRDs validate every external URL as an
// http or https URL with a host, so such a URL cannot reach a renderer.
func Parse(raw string) *url.URL {
	parsed, err := url.Parse(raw)
	if err != nil {
		return &url.URL{}
	}

	return parsed
}

// ContextPath returns the path of a parsed external URL, without a trailing
// slash. It is empty for a URL that serves the root of its host.
func ContextPath(external *url.URL) string {
	return strings.TrimRight(external.Path, "/")
}

// RoutePrefix returns the path prefix of a route to an app with this external
// URL: its context path, or / for a URL at the root of its host or for no URL.
func RoutePrefix(raw string) string {
	if path := ContextPath(Parse(raw)); path != "" {
		return path
	}

	return "/"
}
