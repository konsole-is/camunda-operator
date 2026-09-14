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

// Package hostfold renders one host name for the spellings that reach one
// host. Every claim key that names an address folds its host here: two
// spellings that a client resolves to one server must never take a claim each.
package hostfold

import (
	"net"
	"strings"

	"golang.org/x/net/idna"
)

// FoldHost renders host in one spelling, in this order: lower case, without
// the trailing dot of the DNS root, then the IDNA form its client resolves,
// and last an IP literal in the form net.IP writes.
//
// The IDNA profile refuses an IP literal, which keeps the spelling it came
// with through that step and is canonical after the last one. A host of "."
// alone keeps its dot, because an empty host names nothing at all.
func FoldHost(host string) string {
	folded := strings.ToLower(host)
	if trimmed := strings.TrimSuffix(folded, "."); trimmed != "" {
		folded = trimmed
	}
	if ascii, err := idna.Lookup.ToASCII(folded); err == nil && ascii != "" {
		folded = ascii
	}
	if ip := net.ParseIP(folded); ip != nil {
		return ip.String()
	}

	return folded
}
