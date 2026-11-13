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

	archiving := v1.DatabaseServerSpec{
		Archive: &v1.DatabaseServerArchiveSpec{RetentionPeriodDays: 30},
	}

	// No base backup of the archive has completed, so no restore can reach a
	// point yet.
	empty := pitrCapability(server, archiving, "", capabilityLocation, capabilityNow)
	require.NotNil(t, empty)
	assert.False(t, empty.Enabled)
	assert.Nil(t, empty.RetentionPeriodDays)
	assert.Empty(t, empty.Windows)
	assert.Equal(t, v1.RecoveryModeExternal, empty.Recovery)

	server.Status.Archive = &v1.DatabaseServerArchiveStatus{
		History: []v1.ArchiveRecord{{ServerName: "my-cluster-db", From: windowTime(0)}},
	}
	with := pitrCapability(server, archiving, "", capabilityLocation, capabilityNow)
	require.NotNil(t, with)
	assert.True(t, with.Enabled)
	assert.Equal(t, v1.RecoveryModeOperator, with.Recovery)
	require.NotNil(t, with.RetentionPeriodDays)
	assert.Equal(t, int32(30), *with.RetentionPeriodDays)

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
	assert.Empty(t, taken.Windows)
	assert.Equal(t, v1.RecoveryModeExternal, taken.Recovery)
}

// capabilityNow is the clock of the capability tests: ten days after
// windowTime(0).
var capabilityNow = windowTime(240).Time

// capabilityLocation is where the server of the capability tests archives to.
const capabilityLocation = "s3://camunda-backups/clusters/databaseserver/my-cluster-ns/a"

// windowTime is a fixed point plus the given number of hours.
func windowTime(hours int) metav1.Time {
	return metav1.NewTime(time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC).Add(time.Duration(hours) * time.Hour))
}

// The contract publishes each archive the server wrote as a window, and follows
// the history as a record gets unverifiedFrom, closes, and opens.
func TestPITRCapabilityPublishesTheArchiveHistory(t *testing.T) {
	t.Parallel()

	archiving := v1.DatabaseServerSpec{
		Archive: &v1.DatabaseServerArchiveSpec{RetentionPeriodDays: 30},
	}
	location := capabilityLocation
	lag := &metav1.Duration{Duration: ArchiveLag}
	server := archiveServer()
	server.Status.Archive = &v1.DatabaseServerArchiveStatus{
		History: []v1.ArchiveRecord{{ServerName: "my-cluster-db", Location: location, From: windowTime(0)}},
	}

	assert.Equal(
		t, []v1.RecoveryWindow{
			{Location: location, From: windowTime(0), Lag: lag},
		}, pitrCapability(server, archiving, "", capabilityLocation, capabilityNow).Windows,
		"an open record is an open window with the archive lag",
	)

	// The upload that failed at hour 2 carried the segment before it, so the
	// window ends ArchiveLag before the failure.
	server.Status.Archive.History[0].UnverifiedFrom = new(windowTime(2))
	cutoff := metav1.NewTime(windowTime(2).Add(-ArchiveLag))
	assert.Equal(
		t, []v1.RecoveryWindow{
			{Location: location, From: windowTime(0), UnverifiedFrom: &cutoff, Lag: lag},
		}, pitrCapability(server, archiving, "", capabilityLocation, capabilityNow).Windows,
		"unverifiedFrom moves back by the archive lag",
	)

	server.Status.Archive.History[0].UnverifiedFrom = nil
	server.Status.Archive.History[0].To = new(windowTime(5))
	server.Status.Archive.History = append(server.Status.Archive.History, v1.ArchiveRecord{
		ServerName: recoveredClusterName, Location: location, From: windowTime(6),
	})
	assert.Equal(
		t, []v1.RecoveryWindow{
			{Location: location, From: windowTime(0), To: new(windowTime(5))},
			{Location: location, From: windowTime(6), Lag: lag},
		}, pitrCapability(server, archiving, "", capabilityLocation, capabilityNow).Windows,
		"a closed record has no lag, and the new one is open",
	)
}

// A contract holds at most 256 windows. A server with a longer history
// publishes the newest 256 records.
func TestPITRCapabilityKeepsTheNewestWindows(t *testing.T) {
	t.Parallel()

	history := make([]v1.ArchiveRecord, 0, 300)
	for i := range 300 {
		history = append(history, v1.ArchiveRecord{From: windowTime(2 * i), To: new(windowTime(2*i + 1))})
	}
	history[299].To = nil
	server := archiveServer()
	server.Status.Archive = &v1.DatabaseServerArchiveStatus{History: history}

	windows := pitrCapability(
		server, v1.DatabaseServerSpec{
			Archive: &v1.DatabaseServerArchiveSpec{RetentionPeriodDays: 30},
		}, "", capabilityLocation, capabilityNow,
	).Windows

	require.Len(t, windows, 256)
	assert.Equal(t, windowTime(2*44), windows[0].From)
	assert.Equal(t, windowTime(2*299), windows[255].From)
	assert.Nil(t, windows[255].To)
}

// A rollback reads only the location that the server archives to now. A record
// at another location, and a record that names no location and another
// bucket, give no window. A record that names no location and the current
// bucket is at the current location.
func TestPITRCapabilityPublishesOnlyTheArchivesARollbackCanRead(t *testing.T) {
	t.Parallel()

	archiving := v1.DatabaseServerSpec{
		Archive: &v1.DatabaseServerArchiveSpec{ObjectStorageRef: "current", RetentionPeriodDays: 30},
	}
	server := archiveServer()
	server.Status.Archive = &v1.DatabaseServerArchiveStatus{
		History: []v1.ArchiveRecord{
			{ServerName: "my-cluster-db", ObjectStorageRef: "earlier", From: windowTime(0), To: new(windowTime(1))},
			{
				ServerName: "my-cluster-db", ObjectStorageRef: "current", Location: "s3://elsewhere/a",
				From: windowTime(2), To: new(windowTime(3)),
			},
			{ServerName: "my-cluster-db", ObjectStorageRef: "current", From: windowTime(4)},
		},
	}

	assert.Equal(
		t, []v1.RecoveryWindow{
			{Location: capabilityLocation, From: windowTime(4), Lag: &metav1.Duration{Duration: ArchiveLag}},
		}, pitrCapability(server, archiving, "", capabilityLocation, capabilityNow).Windows,
	)

	// The archive moved, and the new location has no base backup yet. No
	// restore can reach a point, as before the first base backup.
	server.Status.Archive.History[2].Location = capabilityLocation
	server.Status.Archive.History[2].To = new(windowTime(5))
	foreign := pitrCapability(server, archiving, "", "s3://new-bucket/a", capabilityNow)
	assert.False(t, foreign.Enabled)
	assert.Nil(t, foreign.RetentionPeriodDays)
	assert.Empty(t, foreign.Windows)
	assert.Equal(t, v1.RecoveryModeExternal, foreign.Recovery)
}

// A restore that asked for a running recovery waits for a contract that
// declares operator. The contract keeps that declaration while the recovery
// runs, also when no window is left.
func TestPITRCapabilityStaysOperatorWhileARecoveryRuns(t *testing.T) {
	t.Parallel()

	archiving := v1.DatabaseServerSpec{
		Archive: &v1.DatabaseServerArchiveSpec{RetentionPeriodDays: 30},
	}
	server := archiveServer()
	server.Status.Archive = &v1.DatabaseServerArchiveStatus{
		History: []v1.ArchiveRecord{{ServerName: "my-cluster-db", Location: "s3://elsewhere/a", From: windowTime(0)}},
	}
	server.Status.Recovery = &v1.DatabaseServerRecoveryStatus{RequestID: "a", Cluster: recoveredClusterName}

	running := pitrCapability(server, archiving, "", capabilityLocation, capabilityNow)
	assert.True(t, running.Enabled)
	assert.Equal(t, v1.RecoveryModeOperator, running.Recovery)
	assert.Empty(t, running.Windows)

	server.Status.Recovery.CompletedAt = new(windowTime(1))
	answered := pitrCapability(server, archiving, "", capabilityLocation, capabilityNow)
	assert.False(t, answered.Enabled)
	assert.Equal(t, v1.RecoveryModeExternal, answered.Recovery)
}

// After a shorter retention period pruned the bucket, a longer one does not
// bring the pruned points back. The windows start at the floor of those prunes
// until the longer period catches up with it.
func TestPITRCapabilityClipsTheWindowsToAPrunedFloor(t *testing.T) {
	t.Parallel()

	archiving := v1.DatabaseServerSpec{
		Archive: &v1.DatabaseServerArchiveSpec{RetentionPeriodDays: 3},
	}
	lag := &metav1.Duration{Duration: ArchiveLag}
	server := archiveServer()
	server.Status.Archive = &v1.DatabaseServerArchiveStatus{
		// Three days before capabilityNow is windowTime(168).
		ReachableFrom: new(windowTime(200)),
		History: []v1.ArchiveRecord{
			{ServerName: "my-cluster-db", From: windowTime(0), To: new(windowTime(150))},
			{ServerName: "my-cluster-db", From: windowTime(160), To: new(windowTime(200))},
			{ServerName: "my-cluster-db", From: windowTime(190), To: new(windowTime(220))},
			{ServerName: "my-cluster-db", From: windowTime(230)},
		},
	}

	assert.Equal(
		t, []v1.RecoveryWindow{
			{Location: capabilityLocation, From: windowTime(200), To: new(windowTime(220))},
			{Location: capabilityLocation, From: windowTime(230), Lag: lag},
		}, pitrCapability(server, archiving, "", capabilityLocation, capabilityNow).Windows,
		"a window starts at the floor, and a window that ends at or before it is left out",
	)

	// Once the retention period starts after the floor, admission refuses what
	// the floor refused, and the windows are the history again.
	caughtUp := windowTime(200 + 72 + 1).Time
	assert.Equal(
		t, windowTime(0), pitrCapability(server, archiving, "", capabilityLocation, caughtUp).Windows[0].From,
	)
}

// In steady state the floor trails the retention period, so the windows do
// not move with the clock. A contract that changed each minute would be
// applied again on each reconcile.
func TestPITRCapabilityIgnoresASteadyStateFloor(t *testing.T) {
	t.Parallel()

	archiving := v1.DatabaseServerSpec{
		Archive: &v1.DatabaseServerArchiveSpec{RetentionPeriodDays: 3},
	}
	server := archiveServer()
	server.Status.Archive = &v1.DatabaseServerArchiveStatus{
		History: []v1.ArchiveRecord{
			{ServerName: "my-cluster-db", From: windowTime(0), To: new(windowTime(150))},
			{ServerName: "my-cluster-db", From: windowTime(160)},
		},
	}
	unclipped := pitrCapability(server, archiving, "", capabilityLocation, capabilityNow).Windows

	// The floor that advanceArchiveFloor writes: the start of the retention
	// period, truncated to the minute.
	render := func(now time.Time) []v1.RecoveryWindow {
		floor := metav1.NewTime(now.Add(-3 * 24 * time.Hour).Truncate(time.Minute))
		server.Status.Archive.ReachableFrom = &floor

		return pitrCapability(server, archiving, "", capabilityLocation, now).Windows
	}

	assert.Equal(t, unclipped, render(capabilityNow), "a floor at the start of the retention period")

	now := capabilityNow.Add(30 * time.Second)
	assert.Equal(t, unclipped, render(now))
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
		windowTime(0).Time,
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
