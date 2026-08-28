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
	"slices"
	"strings"
)

// The fields of a client representation that this package reads by name.
const (
	fieldID           = "id"
	fieldRedirectURIs = "redirectUris"
)

// ID returns the internal id of the client, which every other call addresses
// it by. It differs from the client id that FindClient looks the client up by.
func (r Representation) ID() string {
	id, _ := r[fieldID].(string)

	return id
}

// RedirectURIs returns the redirect URIs of the client, in the order the
// server holds them.
func (r Representation) RedirectURIs() []string {
	raw, ok := r[fieldRedirectURIs].([]any)
	if !ok {
		return nil
	}

	uris := make([]string, 0, len(raw))
	for _, entry := range raw {
		if uri, ok := entry.(string); ok {
			uris = append(uris, uri)
		}
	}

	return uris
}

// SetRedirectURIs replaces the redirect URIs of the client. Every other field
// stays as the server returned it.
func (r Representation) SetRedirectURIs(uris []string) {
	entries := make([]any, 0, len(uris))
	for _, uri := range uris {
		entries = append(entries, uri)
	}
	r[fieldRedirectURIs] = entries
}

// MergeRedirectURIs returns the redirect URI list that carries every entry of
// desired and keeps every entry of current that this operator does not own.
//
// An entry of current is dropped only when it ends with suffix and desired
// does not hold it. Everything else survives, so a redirect URI of another
// shape stays whoever wrote it. An entry that does end with suffix is treated
// as one of the caller's, so a caller that shares suffix with another writer
// takes that writer's entries away.
//
// The kept entries stay in the order of current, and the new ones follow in
// the order of desired, so a list that needs no change compares equal to
// current. An entry that does not end with suffix passes through exactly as it
// is, a repeat of another writer's entry included, so this never rewrites a
// list for a duplicate that is not the caller's.
func MergeRedirectURIs(current, desired []string, suffix string) []string {
	merged := make([]string, 0, len(current)+len(desired))
	for _, uri := range current {
		if !strings.HasSuffix(uri, suffix) {
			// Not the caller's to touch, not even to collapse a repeat of.
			merged = append(merged, uri)

			continue
		}
		if slices.Contains(desired, uri) && !slices.Contains(merged, uri) {
			merged = append(merged, uri)
		}
	}

	for _, uri := range desired {
		if !slices.Contains(merged, uri) {
			merged = append(merged, uri)
		}
	}

	return merged
}
