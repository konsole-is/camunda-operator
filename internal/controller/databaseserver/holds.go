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
	"fmt"

	"github.com/sourcehawk/operator-component-framework/pkg/component"
	"k8s.io/apimachinery/pkg/api/meta"

	v1 "github.com/konsole-is/camunda-operator/api/v1"
	"github.com/konsole-is/camunda-operator/pkg/conditions"
)

// recoveryHoldsSpec reports that the spec moved something the running recovery
// depends on, or nil when it did not.
//
// A recovery is a question that one contract asked, answered out of one
// archive. Publishing another contract while it runs leaves the cluster that
// is building with nobody to answer. Pointing spec.archive at another bucket
// takes away the copy it reads, removing spec.archive takes the archive with
// it, and a shorter retentionPeriodDays prunes the base backup it starts from.
// The server keeps the contract and the whole archive block until the request
// is answered.
func recoveryHoldsSpec(
	server *v1.DatabaseServer,
	merged v1.DatabaseServerSpec,
) *conditions.PreCheckFailure {
	running := server.Status.Recovery
	if running == nil || running.CompletedAt != nil {
		return nil
	}

	if running.Contract != "" && running.Contract != merged.DatabaseServerConfig {
		return &conditions.PreCheckFailure{
			Reason: v1.ReasonInvalidReference,
			Message: fmt.Sprintf(
				"A rollback that %s asked for on DatabaseServerConfig %q is still running. Set "+
					"spec.databaseServerConfig back to that name, or wait for the rollback to "+
					"finish, before the server publishes another contract",
				running.RequestedBy, running.Contract,
			),
		}
	}

	if running.Archive == nil {
		return nil
	}

	// A record that carries no retention was written before the settings were
	// recorded, and nothing else holds them. Holding the removal against it
	// would render "0d" on the ObjectStore and publish a retention the
	// contract refuses, so the removal applies and the rollback is refused
	// instead. That takes nothing out of the bucket.
	if merged.Archive == nil && running.Archive.RetentionPeriodDays > 0 {
		return &conditions.PreCheckFailure{
			Reason: v1.ReasonInvalidReference,
			Message: fmt.Sprintf(
				"A rollback that %s asked for is still running, and it reads the archive in "+
					"ObjectStorageConfig %q. The archive cannot be removed until that rollback "+
					"is answered. Put spec.archive back, or wait for the rollback to finish",
				running.RequestedBy, running.Archive.ObjectStorageRef,
			),
		}
	}

	if merged.Archive == nil {
		return nil
	}

	if running.Archive.ObjectStorageRef != merged.Archive.ObjectStorageRef {
		return &conditions.PreCheckFailure{
			Reason: v1.ReasonInvalidReference,
			Message: fmt.Sprintf(
				"A rollback that %s asked for reads the archive in ObjectStorageConfig %q, which "+
					"is still running. Set spec.archive.objectStorageRef back to that name, or "+
					"wait for the rollback to finish",
				running.RequestedBy, running.Archive.ObjectStorageRef,
			),
		}
	}

	// heldArchive is the rule: whatever it puts back is what the spec moved.
	// A preset carries these settings as readily as an inline block, and a
	// shrunk retention reaches the merged spec either way.
	if held := heldArchive(running.Archive, merged.Archive); *held != *merged.Archive {
		return &conditions.PreCheckFailure{
			Reason: v1.ReasonInvalidReference,
			Message: fmt.Sprintf(
				"A rollback that %s asked for is still running, and it reads the archive of this "+
					"server. A change to the retention or the schedule of the archive changes "+
					"what it holds, so the archive keeps its settings until the rollback is "+
					"answered. Set spec.archive back to retentionPeriodDays %d and "+
					"baseBackupSchedule %q, or wait for the rollback to finish",
				running.RequestedBy, held.RetentionPeriodDays, held.BaseBackupSchedule,
			),
		}
	}

	return nil
}

// heldArchive returns the archive block that a held spec renders: the bucket,
// the retention, and the schedule that status.recovery recorded. Every one of
// them comes from the record, so no edit of spec.archive reaches the archive a
// rollback reads. It is also what decides that the spec moved one of them, so
// the block the server renders and the edit it reports never disagree.
//
// A record written before the settings were recorded carries neither of them,
// and the spec fills what it still has. A removal is not held against such a
// record at all: see recoveryHoldsSpec.
func heldArchive(
	recorded *v1.RecoveryArchiveRef,
	spec *v1.DatabaseServerArchiveSpec,
) *v1.DatabaseServerArchiveSpec {
	block := v1.DatabaseServerArchiveSpec{
		ObjectStorageRef:    recorded.ObjectStorageRef,
		RetentionPeriodDays: recorded.RetentionPeriodDays,
		BaseBackupSchedule:  recorded.BaseBackupSchedule,
	}

	if spec != nil {
		if block.RetentionPeriodDays < 1 {
			block.RetentionPeriodDays = spec.RetentionPeriodDays
		}
		if block.BaseBackupSchedule == "" {
			block.BaseBackupSchedule = spec.BaseBackupSchedule
		}
	}

	return &block
}

// instancesAreDown reports whether CloudNativePG has taken the instances of the
// server down. The cluster component reports Suspended only once CloudNativePG
// has confirmed the hibernation, so a server that asked for a suspension and
// has not reached it yet is still running.
func instancesAreDown(server *v1.DatabaseServer) bool {
	condition := meta.FindStatusCondition(server.Status.Conditions, v1.ConditionClusterReady)

	return condition != nil && condition.Reason == string(component.Suspended)
}

// recoveryHoldsLocation reports that the archive of the server moved while a
// recovery still reads it, or nil when it did not.
//
// recoveryHoldsSpec pins the bucket contract by name, and an
// ObjectStorageConfig edited in place keeps its name, so only the location it
// resolves to shows this move.
func recoveryHoldsLocation(
	server *v1.DatabaseServer,
	location string,
) *conditions.PreCheckFailure {
	running := server.Status.Recovery
	if running == nil || running.CompletedAt != nil || running.Archive == nil {
		return nil
	}

	recorded := running.Archive.Location
	if recorded == "" || location == "" || recorded == location {
		return nil
	}

	return &conditions.PreCheckFailure{
		Reason: v1.ReasonInvalidReference,
		Message: fmt.Sprintf(
			"A rollback that %s asked for reads the archive at %q, and ObjectStorageConfig %q "+
				"names %q now. Point that ObjectStorageConfig back at the archive the rollback "+
				"reads, or wait for the rollback to finish, before the archive of the server "+
				"moves",
			running.RequestedBy, recorded, running.Archive.ObjectStorageRef, location,
		),
	}
}
