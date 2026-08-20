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

package logicalrestore

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"

	v1 "github.com/konsole-is/camunda-operator/api/v1"
	"github.com/konsole-is/camunda-operator/pkg/conditions"
)

// validate compares the backup against the target and fails the restore when
// the target cannot hold it. It runs before the first destructive step, so a
// mismatch costs nothing but the resource.
func (r *Reconciler) validate(ctx context.Context, restore *v1.LogicalRestore) (hold, error) {
	resolved, failure, err := r.resolveTarget(ctx, restore)
	if err != nil {
		return settle, err
	}
	if failure != nil {
		return r.holdRunning(restore, failure), nil
	}

	if failure := check(compatibility{
		BackupStorageType: resolved.backup.StorageType,
		TargetStorageType: resolved.storage.Spec.Type,
		BackupPartitions:  resolved.backup.Partitions,
		TargetPartitions:  resolved.target.Partitions,
		BackupBucket:      resolved.backup.Bucket,
		TargetBucket:      resolved.cluster.Spec.BackupStorageRef,
		BackupVersion:     resolved.backup.Version,
		TargetVersion:     resolved.target.Version,
	}); failure != nil {
		r.fail(restore, failure.Reason, fmt.Sprintf(
			"the target cannot hold backup %s/%s: %s",
			resolved.backup.Namespace, resolved.backup.Name, failure.Message,
		))

		return settle, nil
	}

	recovered(restore)
	restore.Status.Phase = v1.LogicalRestoreRestoringSecondaryStorage
	conditions.Stage(restore, progressing(
		restore, "the restore writes the backup into the secondary storage of the target",
	))

	return r.shortly(), nil
}

// compatibility is what the ValidatingCompatibility phase compares: the facts
// of the backup against the facts of the target. The target facts come from
// the live broker StatefulSet and the storage contract of the target, because
// the management binding of a suspended cluster is unset.
type compatibility struct {
	// BackupStorageType is the secondary storage type of the backup kind.
	BackupStorageType v1.SecondaryStorageType
	// TargetStorageType is the type of the storage contract of the target.
	TargetStorageType v1.SecondaryStorageType
	// BackupPartitions is the partition count that the backup recorded, and
	// zero when it recorded none. Only the Elasticsearch kind records one.
	BackupPartitions int32
	// TargetPartitions is CAMUNDA_CLUSTER_PARTITIONCOUNT of the target.
	TargetPartitions int32
	// BackupBucket is the ObjectStorageConfig that the backup wrote to.
	BackupBucket string
	// TargetBucket is the spec.backupStorageRef of the target.
	TargetBucket string
	// BackupVersion is the Camunda version that the backup recorded.
	BackupVersion string
	// TargetVersion is the tag of the broker image of the target.
	TargetVersion string
}

// check reports the first reason why the target cannot hold the backup, or
// nil when it can. Every answer carries v1.ReasonIncompatibleTarget: the
// restore fails on it, because no change to the restore resource resolves it.
func check(in compatibility) *conditions.PreCheckFailure {
	if in.BackupStorageType != in.TargetStorageType {
		return incompatible(
			"the backup holds %s data and the target cluster stores its data in %s",
			in.BackupStorageType, in.TargetStorageType,
		)
	}

	// A relational backup records no partition count. The restore application
	// then reads the exporter position from the restored database, and Camunda
	// aligns the partitions itself.
	if in.BackupPartitions > 0 && in.BackupPartitions != in.TargetPartitions {
		return incompatible(
			"the backup was taken from %d partitions and the target cluster runs %d",
			in.BackupPartitions, in.TargetPartitions,
		)
	}

	if in.BackupBucket != in.TargetBucket {
		return incompatible(
			"the backup wrote to ObjectStorageConfig %q and the target cluster backs up through %q; "+
				"the restore reads the bucket of the backup with the credentials that the "+
				"CamundaCluster controller copies into the namespace, and it copies them for the "+
				"bucket of the target alone",
			in.BackupBucket, in.TargetBucket,
		)
	}

	return checkVersions(in)
}

// checkVersions applies the version rule of the storage type. An
// Elasticsearch backup restores only with the exact version it was taken
// with, because that version is part of every snapshot name. A relational
// backup restores with the same version or one minor version newer.
func checkVersions(in compatibility) *conditions.PreCheckFailure {
	if in.BackupVersion == "" {
		return incompatible(
			"the backup did not record the Camunda version it was taken with, so the restore cannot " +
				"prove that the target can read it",
		)
	}

	backup, err := parseVersion(in.BackupVersion)
	if err != nil {
		return incompatible("the backup recorded the Camunda version %q, which %s", in.BackupVersion, err)
	}

	target, err := parseVersion(in.TargetVersion)
	if err != nil {
		return incompatible("the target cluster runs the Camunda version %q, which %s", in.TargetVersion, err)
	}

	if in.BackupStorageType == v1.SecondaryStorageTypeElasticsearch {
		if in.BackupVersion != in.TargetVersion {
			return incompatible(
				"the backup was taken with Camunda %s and the target cluster runs %s; an Elasticsearch "+
					"backup restores only with the exact version it was taken with, because that "+
					"version is part of every snapshot name",
				in.BackupVersion, in.TargetVersion,
			)
		}

		return nil
	}

	if target.major != backup.major || target.minor < backup.minor || target.minor > backup.minor+1 {
		return incompatible(
			"the backup was taken with Camunda %s and the target cluster runs %s; a relational backup "+
				"restores with the same version or with one minor version newer",
			in.BackupVersion, in.TargetVersion,
		)
	}

	return nil
}

// version is a Camunda version as the rule reads it. The patch level is
// parsed so that a version that is not of the form x.y.z is refused, and it
// takes part in no comparison: the rule is written on the major and the minor.
type version struct {
	major int
	minor int
	patch int
}

// errNotVersion says that a value is not a Camunda version. It reads as the
// tail of "the version %q, which ...".
var errNotVersion = errors.New("is not a version of the form x.y.z")

// parseVersion reads a version of the form x.y.z. The error reads as the tail
// of "the version %q, which ...", so the caller names the version once.
func parseVersion(value string) (version, error) {
	parts := strings.Split(value, ".")
	if len(parts) != 3 {
		return version{}, errNotVersion
	}

	numbers := make([]int, 0, len(parts))
	for _, part := range parts {
		number, err := strconv.Atoi(part)
		if err != nil || number < 0 {
			return version{}, errNotVersion
		}
		numbers = append(numbers, number)
	}

	return version{major: numbers[0], minor: numbers[1], patch: numbers[2]}, nil
}

// incompatible builds the failure of a target that cannot hold the backup.
func incompatible(format string, args ...any) *conditions.PreCheckFailure {
	return &conditions.PreCheckFailure{
		Reason:  v1.ReasonIncompatibleTarget,
		Message: fmt.Sprintf(format, args...),
	}
}
