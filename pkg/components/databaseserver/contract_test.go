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

package databaseserver

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	v1 "github.com/konsole-is/camunda-operator/api/v1"
)

// A recovery replaces the cluster, and every published name follows it: a
// consumer that read the contract reaches the recovered server, not the one it
// replaced.
func TestPublishedNamesFollowTheCluster(t *testing.T) {
	t.Parallel()

	server := archiveServer()
	assert.Equal(t, "my-cluster-db", ClusterName(server))
	assert.Equal(t, "my-cluster-db-rw.my-cluster-ns.svc", ReadWriteHost(server))
	assert.Equal(t, "my-cluster-db-superuser", SuperuserSecretName(server))

	server.Status.Cluster = recoveredClusterName
	assert.Equal(t, recoveredClusterName, ClusterName(server))
	assert.Equal(t, "my-cluster-db-r1-rw.my-cluster-ns.svc", ReadWriteHost(server))
	assert.Equal(t, "my-cluster-db-r1-superuser", SuperuserSecretName(server))
}

// The contract declares the point-in-time-recovery capability the archive
// gives the server, and the retention it publishes is the one the bucket
// enforces.
func TestPITRCapabilityFollowsTheArchive(t *testing.T) {
	t.Parallel()

	without := pitrCapability(v1.DatabaseServerSpec{})
	require.NotNil(t, without)
	assert.False(t, without.Enabled)
	assert.Nil(t, without.RetentionPeriodDays)

	with := pitrCapability(v1.DatabaseServerSpec{
		Archive: &v1.DatabaseServerArchiveSpec{RetentionPeriodDays: 30},
	})
	require.NotNil(t, with)
	assert.True(t, with.Enabled)
	require.NotNil(t, with.RetentionPeriodDays)
	assert.Equal(t, int32(30), *with.RetentionPeriodDays)
}

// The contract must not be published before CloudNativePG has written the
// superuser Secret, or every consumer is sent to credentials that do not
// exist.
func TestContractWaitsForTheSuperuserSecret(t *testing.T) {
	t.Parallel()

	comp, err := ContractComponent(
		archiveServer(),
		v1.DatabaseServerSpec{DatabaseServerConfig: "my-database-server"},
		"",
		"",
	)
	require.NoError(t, err)

	objects, err := comp.Preview()
	require.NoError(t, err)

	// The read-only Secret is a precondition, not desired state, so only the
	// contract is rendered.
	require.Len(t, objects, 1)
	contract, ok := objects[0].(*v1.DatabaseServerConfig)
	require.True(t, ok)
	assert.Equal(t, "my-database-server", contract.Name)
	assert.Equal(t, "my-cluster-db-superuser", contract.Spec.AdminCredentialsSecretRef.Name)
	assert.Equal(t, SuperuserUsernameKey, contract.Spec.AdminCredentialsSecretRef.UsernameKey)
	assert.Equal(t, SuperuserPasswordKey, contract.Spec.AdminCredentialsSecretRef.PasswordKey)
	assert.Equal(t, v1.DatabaseEnginePostgres, contract.Spec.Engine)
	assert.Equal(t, int32(PostgresPort), contract.Spec.Port)
}

// A contract name that already carries a contract is not this server's to
// write. Two servers that both publish it rewrite the endpoint in turn, and a
// consumer reads one that moves under it. A contract nobody controls is the
// bring-your-own-server API, and rewriting that one sends every consumer of an
// external server somewhere else. The message names both cases and the remedy.
func TestContractTakenMessageNamesTheHolderAndTheRemedy(t *testing.T) {
	t.Parallel()

	message := ContractTakenMessage("my-database-server", &metav1.OwnerReference{
		Kind: "DatabaseServer", Name: "other",
	})
	assert.Contains(t, message, `DatabaseServerConfig "my-database-server"`)
	assert.Contains(t, message, `DatabaseServer "other"`)
	assert.Contains(t, message, "spec.databaseServerConfig")

	unowned := ContractTakenMessage("my-database-server", nil)
	assert.Contains(t, unowned, `DatabaseServerConfig "my-database-server"`)
	assert.Contains(t, unowned, "no owner controls it")
	assert.Contains(t, unowned, "spec.databaseServerConfig")
}
