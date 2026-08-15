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
	v1 "github.com/konsole-is/camunda-operator/api/v1"
)

// CollisionKey returns the index key of the logical database that db claims:
// "<serverRef>/<databaseName>". Database names are unique per server, so the
// pair identifies the claim.
func CollisionKey(db *v1.Database) string {
	return db.Spec.ServerRef + "/" + db.Spec.DatabaseName
}

// CollisionWinner picks the Database that owns a contested serverRef and
// databaseName claim. The oldest creationTimestamp wins. The lexicographically
// smaller name is the tiebreaker. It returns nil for an empty claimant list.
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
		case candidate.Name < winner.Name:
			winner = candidate
		}
	}

	return winner
}
