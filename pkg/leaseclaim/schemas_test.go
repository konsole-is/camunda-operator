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

package leaseclaim_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	managementcomponents "github.com/konsole-is/camunda-operator/pkg/components/camundamanagementcluster"
	databasecomponents "github.com/konsole-is/camunda-operator/pkg/components/database"
)

// claimNaming is what tells the claim Leases of one claim from those of
// another: the prefix of their names and the annotation keys that carry
// ownership.
type claimNaming struct {
	claim       string
	prefix      string
	annotations []string
}

// TestTheShippedSchemasNameSeparateClaims pins the invariant that Validate
// cannot check on its own. Two claims that shared a Lease name prefix or an
// annotation key would read each other's Leases as their own, and one claimant
// would take a key another one holds.
func TestTheShippedSchemasNameSeparateClaims(t *testing.T) {
	t.Parallel()

	database := databasecomponents.ClaimSchema()
	realm := managementcomponents.RealmClaimSchema()
	require.NoError(t, database.Validate())
	require.NoError(t, realm.Validate())

	shipped := []claimNaming{
		{
			claim:  "the logical database claim",
			prefix: database.Prefix,
			annotations: []string{
				database.HolderNamespaceAnnotation,
				database.HolderNameAnnotation,
				database.HolderUIDAnnotation,
				database.KeyAnnotation,
			},
		},
		{
			claim:  "the Keycloak realm claim",
			prefix: realm.Prefix,
			annotations: []string{
				realm.HolderNamespaceAnnotation,
				realm.HolderNameAnnotation,
				realm.HolderUIDAnnotation,
				realm.KeyAnnotation,
			},
		},
	}

	prefixes := make(map[string]string, len(shipped))
	annotations := make(map[string]string, 4*len(shipped))
	for _, naming := range shipped {
		first, taken := prefixes[naming.prefix]
		assert.Falsef(t, taken, "%s and %s share the prefix %q", first, naming.claim, naming.prefix)
		prefixes[naming.prefix] = naming.claim

		for _, annotation := range naming.annotations {
			first, taken := annotations[annotation]
			assert.Falsef(
				t, taken, "%s and %s share the annotation %q", first, naming.claim, annotation,
			)
			annotations[annotation] = naming.claim
		}
	}
}
