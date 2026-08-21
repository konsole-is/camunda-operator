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

// Package names bounds the names that the operator derives from the name of a
// custom resource.
//
// A custom resource name is a DNS subdomain of up to 253 characters, but most
// of the names derived from it are bounded tighter: a Service name is a DNS
// label of 63, and so is a label value. A component that appends a suffix to
// the resource name therefore renders a name that the API server rejects, and
// the apply fails on every reconcile with nothing to tell the user why.
package names

import (
	"crypto/sha256"
	"encoding/hex"
	"strings"
)

// HashLength is the hex length of the hash that Bounded appends to a
// truncated name.
const HashLength = 10

// Bounded returns name when it fits limit, or its head followed by a hash of
// the whole name otherwise. The result is deterministic, so every render of
// one resource agrees, and two names that share the head differ in the hash.
//
// A limit under HashLength+2 has no room for a head and a hash, so Bounded
// returns the first limit characters of the hash instead. That keeps the
// result inside limit for every caller, and it stays unique to name.
func Bounded(name string, limit int) string {
	if len(name) <= limit {
		return name
	}

	sum := sha256.Sum256([]byte(name))
	hash := hex.EncodeToString(sum[:])

	if limit < HashLength+2 {
		return hash[:max(limit, 0)]
	}

	return strings.TrimRight(name[:limit-1-HashLength], "-.") + "-" + hash[:HashLength]
}
