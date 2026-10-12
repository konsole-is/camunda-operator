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

// Package accounthash hashes the annotations of a ServiceAccount into the
// value of a pod template annotation, so that a change of the annotations
// rolls the pods. Workload identity reads those annotations when a pod
// starts, so a pod that started before a change keeps the old identity.
package accounthash

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
)

// Annotation is the pod template annotation that carries the hash of the
// annotations that the operator renders on the ServiceAccount of the pods. A
// change to it rolls the pods. It belongs to the operator: a user pod
// annotation with this key must not reach the template.
const Annotation = "camunda.io/service-account-hash"

// hashLength is the number of hex characters of the hash: the first 64 bits
// of the SHA-256 digest.
const hashLength = 16

// Of returns the hash of a set of annotations, independent of the map order.
// An empty set returns "".
func Of(annotations map[string]string) string {
	if len(annotations) == 0 {
		return ""
	}

	// encoding/json writes the keys sorted and quotes every string, so two
	// different sets never encode the same. A map of strings always encodes.
	encoded, _ := json.Marshal(annotations)
	sum := sha256.Sum256(encoded)
	return hex.EncodeToString(sum[:])[:hashLength]
}
