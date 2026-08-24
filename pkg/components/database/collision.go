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

package database

import (
	"strings"

	v1 "github.com/konsole-is/camunda-operator/api/v1"
)

// CollisionKey returns the index key of a logical database claim:
// "<systemIdentifier>/<databaseName>". The caller passes the identifier that
// the contract published in status.systemIdentifier, and holds the claim back
// while that is empty.
func CollisionKey(systemIdentifier, databaseName string) string {
	return systemIdentifier + "/" + databaseName
}

// CollisionIdentity returns the system identifier that key was built from, or
// the empty string for an empty key. It is the inverse of CollisionKey, for a
// caller that holds a recorded claim and needs the server behind it.
func CollisionIdentity(key string) string {
	identifier, _, _ := strings.Cut(key, "/")

	return identifier
}

// CollisionWinner picks the Database that owns a contested claim. The oldest
// creationTimestamp wins. The lexicographically smaller "<namespace>/<name>"
// is the tiebreaker. It returns nil for an empty claimant list.
func CollisionWinner(items []v1.Database) *v1.Database {
	var winner *v1.Database
	for i := range items {
		candidate := &items[i]
		if winner == nil {
			winner = candidate
			continue
		}

		switch {
		case candidate.CreationTimestamp.Time.Before(winner.CreationTimestamp.Time):
			winner = candidate
		case winner.CreationTimestamp.Time.Before(candidate.CreationTimestamp.Time):
		case candidate.Namespace+"/"+candidate.Name < winner.Namespace+"/"+winner.Name:
			winner = candidate
		}
	}

	return winner
}
