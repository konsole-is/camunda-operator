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
	"time"

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

	server := archiveServer()
	without := pitrCapability(server, v1.DatabaseServerSpec{}, "", capabilityLocation, capabilityNow)
	require.NotNil(t, without)
	assert.False(t, without.Enabled)
	assert.Nil(t, without.RetentionPeriodDays)
	assert.Equal(t, v1.RecoveryModeExternal, without.Recovery)

	archiving := v1.DatabaseServerSpec{
		Archive: &v1.DatabaseServerArchiveSpec{RetentionPeriodDays: 30},
	}

	// No base backup of the archive has completed, so no restore can reach a
	// point yet.
	empty := pitrCapability(server, archiving, "", capabilityLocation, capabilityNow)
	require.NotNil(t, empty)
	assert.False(t, empty.Enabled)
	assert.Nil(t, empty.RetentionPeriodDays)
	assert.Nil(t, empty.RecoverableFrom)
	assert.Equal(t, v1.RecoveryModeExternal, empty.Recovery)

	server.Status.Archive = &v1.DatabaseServerArchiveStatus{
		History: []v1.ArchiveRecord{{ServerName: "my-cluster-db", From: historyTime(0)}},
	}
	with := pitrCapability(server, archiving, "", capabilityLocation, capabilityNow)
	require.NotNil(t, with)
	assert.True(t, with.Enabled)
	assert.Equal(t, v1.RecoveryModeOperator, with.Recovery)
	require.NotNil(t, with.RetentionPeriodDays)
	assert.Equal(t, int32(30), *with.RetentionPeriodDays)
	assert.Equal(t, new(historyTime(0)), with.RecoverableFrom)

	// A server whose ObjectStore another owner holds archives nothing, so it
	// declares nothing a restore can reach. A capability here sends a restore
	// to the archive that the other owner writes.
	taken := pitrCapability(
		server, archiving, ArchiveTakenMessage("my-cluster-db", metav1.OwnerReference{
			Kind: "DatabaseServer", Name: "other",
		}), capabilityLocation, capabilityNow,
	)
	require.NotNil(t, taken)
	assert.False(t, taken.Enabled)
	assert.Nil(t, taken.RetentionPeriodDays)
	assert.Nil(t, taken.RecoverableFrom)
	assert.Equal(t, v1.RecoveryModeExternal, taken.Recovery)
}

// capabilityNow is the clock of the capability tests: ten days after
// historyTime(0).
var capabilityNow = historyTime(240).Time

// capabilityLocation is where the server of the capability tests archives to.
const capabilityLocation = "s3://camunda-backups/clusters/databaseserver/my-cluster-ns/a"

// historyTime is a fixed point plus the given number of hours.
func historyTime(hours int) metav1.Time {
	return metav1.NewTime(time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC).Add(time.Duration(hours) * time.Hour))
}

// The oldest record of the history is where a restore can start. A record
// that closes, or a record that a rollback opens, does not move it.
func TestPITRCapabilityPublishesTheStartOfTheOldestRecord(t *testing.T) {
	t.Parallel()

	archiving := v1.DatabaseServerSpec{
		Archive: &v1.DatabaseServerArchiveSpec{RetentionPeriodDays: 30},
	}
	server := archiveServer()
	server.Status.Archive = &v1.DatabaseServerArchiveStatus{
		History: []v1.ArchiveRecord{
			{ServerName: "my-cluster-db", Location: capabilityLocation, From: historyTime(0), To: new(historyTime(5))},
			{ServerName: recoveredClusterName, Location: capabilityLocation, From: historyTime(6)},
		},
	}

	assert.Equal(
		t,
		new(historyTime(0)),
		pitrCapability(server, archiving, "", capabilityLocation, capabilityNow).RecoverableFrom,
	)
}

// A rollback reads only the location that the server archives to now. A record
// at another location, and a record that names no location and another
// bucket, do not count. A record that names no location and the current
// bucket is at the current location.
func TestPITRCapabilityCountsOnlyTheArchivesARollbackCanRead(t *testing.T) {
	t.Parallel()

	archiving := v1.DatabaseServerSpec{
		Archive: &v1.DatabaseServerArchiveSpec{ObjectStorageRef: "current", RetentionPeriodDays: 30},
	}
	server := archiveServer()
	server.Status.Archive = &v1.DatabaseServerArchiveStatus{
		History: []v1.ArchiveRecord{
			{ServerName: "my-cluster-db", ObjectStorageRef: "earlier", From: historyTime(0), To: new(historyTime(1))},
			{
				ServerName: "my-cluster-db", ObjectStorageRef: "current", Location: "s3://elsewhere/a",
				From: historyTime(2), To: new(historyTime(3)),
			},
			{ServerName: "my-cluster-db", ObjectStorageRef: "current", From: historyTime(4)},
		},
	}

	assert.Equal(
		t,
		new(historyTime(4)),
		pitrCapability(server, archiving, "", capabilityLocation, capabilityNow).RecoverableFrom,
	)

	// The archive moved, and the new location has no base backup yet. No
	// restore can reach a point, as before the first base backup.
	server.Status.Archive.History[2].Location = capabilityLocation
	server.Status.Archive.History[2].To = new(historyTime(5))
	foreign := pitrCapability(server, archiving, "", "s3://new-bucket/a", capabilityNow)
	assert.False(t, foreign.Enabled)
	assert.Nil(t, foreign.RetentionPeriodDays)
	assert.Nil(t, foreign.RecoverableFrom)
	assert.Equal(t, v1.RecoveryModeExternal, foreign.Recovery)
}

// A restore that asked for a running recovery waits for a contract that
// declares operator. The contract keeps that declaration while the recovery
// runs, also when no archive record is left to publish.
func TestPITRCapabilityStaysOperatorWhileARecoveryRuns(t *testing.T) {
	t.Parallel()

	archiving := v1.DatabaseServerSpec{
		Archive: &v1.DatabaseServerArchiveSpec{RetentionPeriodDays: 30},
	}
	server := archiveServer()
	server.Status.Archive = &v1.DatabaseServerArchiveStatus{
		History: []v1.ArchiveRecord{{ServerName: "my-cluster-db", Location: "s3://elsewhere/a", From: historyTime(0)}},
	}
	server.Status.Recovery = &v1.DatabaseServerRecoveryStatus{RequestID: "a", Cluster: recoveredClusterName}

	running := pitrCapability(server, archiving, "", capabilityLocation, capabilityNow)
	assert.True(t, running.Enabled)
	assert.Equal(t, v1.RecoveryModeOperator, running.Recovery)
	assert.Nil(t, running.RecoverableFrom)

	server.Status.Recovery.CompletedAt = new(historyTime(1))
	answered := pitrCapability(server, archiving, "", capabilityLocation, capabilityNow)
	assert.False(t, answered.Enabled)
	assert.Equal(t, v1.RecoveryModeExternal, answered.Recovery)
}

// After a shorter retention period pruned the bucket, a longer one does not
// bring the pruned points back. recoverableFrom starts at the floor of those
// prunes until the longer period catches up with it.
func TestPITRCapabilityClipsToAPrunedFloor(t *testing.T) {
	t.Parallel()

	archiving := v1.DatabaseServerSpec{
		Archive: &v1.DatabaseServerArchiveSpec{RetentionPeriodDays: 3},
	}
	server := archiveServer()
	server.Status.Archive = &v1.DatabaseServerArchiveStatus{
		// Three days before capabilityNow is historyTime(168).
		ReachableFrom: new(historyTime(200)),
		History: []v1.ArchiveRecord{
			{ServerName: "my-cluster-db", From: historyTime(0), To: new(historyTime(150))},
			{ServerName: "my-cluster-db", From: historyTime(160), To: new(historyTime(200))},
			{ServerName: "my-cluster-db", From: historyTime(190), To: new(historyTime(220))},
			{ServerName: "my-cluster-db", From: historyTime(230)},
		},
	}

	assert.Equal(
		t,
		new(historyTime(200)),
		pitrCapability(server, archiving, "", capabilityLocation, capabilityNow).RecoverableFrom,
		"a record that reaches past the floor starts at the floor",
	)

	server.Status.Archive.ReachableFrom = new(historyTime(225))
	assert.Equal(
		t,
		new(historyTime(230)),
		pitrCapability(server, archiving, "", capabilityLocation, capabilityNow).RecoverableFrom,
		"a record that ends at or before the floor does not count",
	)

	// Once the retention period starts after the floor, admission refuses what
	// the floor refused, and recoverableFrom is the start of the history again.
	caughtUp := historyTime(225 + 72 + 1).Time
	assert.Equal(
		t, new(historyTime(0)), pitrCapability(server, archiving, "", capabilityLocation, caughtUp).RecoverableFrom,
	)
}

// In steady state the floor trails the retention period, so recoverableFrom
// does not move with the clock. A contract that changed each minute would be
// applied again on each reconcile.
func TestPITRCapabilityIgnoresASteadyStateFloor(t *testing.T) {
	t.Parallel()

	archiving := v1.DatabaseServerSpec{
		Archive: &v1.DatabaseServerArchiveSpec{RetentionPeriodDays: 3},
	}
	server := archiveServer()
	server.Status.Archive = &v1.DatabaseServerArchiveStatus{
		History: []v1.ArchiveRecord{
			{ServerName: "my-cluster-db", From: historyTime(0), To: new(historyTime(150))},
			{ServerName: "my-cluster-db", From: historyTime(160)},
		},
	}

	// The floor that advanceArchiveFloor writes: the start of the retention
	// period, truncated to the minute.
	render := func(now time.Time) *metav1.Time {
		floor := metav1.NewTime(now.Add(-3 * 24 * time.Hour).Truncate(time.Minute))
		server.Status.Archive.ReachableFrom = &floor

		return pitrCapability(server, archiving, "", capabilityLocation, now).RecoverableFrom
	}

	assert.Equal(t, new(historyTime(0)), render(capabilityNow), "a floor at the start of the retention period")

	now := capabilityNow.Add(30 * time.Second)
	assert.Equal(t, new(historyTime(0)), render(now))
	assert.Equal(t, render(now), render(now.Add(time.Minute)))
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
		"",
		capabilityLocation,
		historyTime(0).Time,
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
