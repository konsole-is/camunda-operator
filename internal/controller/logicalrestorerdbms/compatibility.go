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

package logicalrestorerdbms

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"

	v1 "github.com/konsole-is/camunda-operator/api/v1"
	"github.com/konsole-is/camunda-operator/pkg/conditions"
	"github.com/konsole-is/camunda-operator/pkg/restore"
)

// compatibility is what the ValidatingCompatibility phase compares: the facts
// of the backup against the facts of the target. The target facts come from
// the live broker StatefulSet and the storage contract of the target, because
// the management binding of a suspended cluster is unset.
//
// A relational backup records no partition count, so the rules compare none.
// The restore application reads the exporter position from the restored
// database and aligns the partitions itself.
type compatibility struct {
	// TargetStorageType is the type of the storage contract of the target.
	// Only a relational target can hold a relational backup.
	TargetStorageType v1.SecondaryStorageType
	// BackupCluster is the name of the CamundaCluster the backup was taken
	// from.
	BackupCluster string
	// TargetCluster is the name of the CamundaCluster the restore writes into.
	TargetCluster string
	// BackupBucket is the ObjectStorageConfig that the backup wrote to.
	BackupBucket string
	// TargetBucket is the spec.backupStorageRef of the target.
	TargetBucket string
	// BackupVersion is the Camunda version that the backup recorded.
	BackupVersion string
	// TargetVersion is the tag of the broker image of the target.
	TargetVersion string
}

// errNotVersion says that a value is not a Camunda version. It reads as the
// tail of "the version %q, which ...".
var errNotVersion = errors.New("is not a version of the form x.y.z")

// validate compares the backup against the target and fails the restore when
// the target cannot hold it. It runs before the first destructive step, so a
// mismatch costs nothing but the resource.
func (r *Reconciler) validate(
	ctx context.Context,
	lrr *v1.LogicalRestoreRDBMS,
) (restore.Outcome, error) {
	resolved, failure, err := r.resolve(ctx, lrr)
	if err != nil {
		return restore.Outcome{}, err
	}
	if failure != nil {
		return r.holdStarted(lrr, failure), nil
	}

	if failure := check(compatibility{
		TargetStorageType: resolved.storage.Spec.Type,
		BackupCluster:     resolved.backup.Cluster,
		TargetCluster:     resolved.cluster.Name,
		BackupBucket:      resolved.backup.Bucket,
		TargetBucket:      resolved.cluster.Spec.BackupStorageRef,
		BackupVersion:     resolved.backup.Version,
		TargetVersion:     resolved.target.Version,
	}); failure != nil {
		r.fail(lrr, failure.Reason, fmt.Sprintf(
			"the target cannot hold backup %s/%s: %s",
			resolved.backup.Namespace, resolved.backup.Name, failure.Message,
		))

		return restore.Outcome{}, nil
	}

	restore.Recovered(&lrr.Status.RestoreProgress)
	lrr.Status.Phase = v1.LogicalRestoreRestoringSecondaryStorage
	r.progressing(lrr, "the restore writes the dump into the logical database of the target")

	return restore.Outcome{Wait: restore.Shortly}, nil
}

// check reports the first reason why the target cannot hold the backup, or
// nil when it can. Every answer carries v1.ReasonIncompatibleTarget: the
// restore fails on it, because no change to the restore resource resolves it.
func check(in compatibility) *conditions.PreCheckFailure {
	// This rule goes first. The others say that one fact of the target does
	// not match the backup. This one says that the restore cannot work at all.
	if in.BackupCluster != in.TargetCluster {
		return incompatible(
			"the backup was taken from CamundaCluster %q and the target cluster is %q. A restore "+
				"into a different cluster is not supported. The restore application reads the "+
				"primary-storage backup under the prefix of the cluster it runs as",
			in.BackupCluster, in.TargetCluster,
		)
	}

	if in.TargetStorageType != v1.SecondaryStorageTypeRDBMS {
		return incompatible(
			"the backup holds %s data and the target cluster stores its data in %s",
			v1.SecondaryStorageTypeRDBMS, in.TargetStorageType,
		)
	}

	if in.BackupBucket != in.TargetBucket {
		return incompatible(
			"the backup wrote to ObjectStorageConfig %q and the target cluster backs up through %q. "+
				"The restore reads the bucket of the backup with the credentials that the "+
				"CamundaCluster controller copies into the namespace, and it copies them for the "+
				"bucket of the target alone",
			in.BackupBucket, in.TargetBucket,
		)
	}

	return checkVersions(in)
}

// checkVersions applies the version rule of a relational backup. Such a backup
// restores only with the exact version it was taken with, because the restore
// application runs with the version of the broker image. A backup that
// recorded no version fails closed.
func checkVersions(in compatibility) *conditions.PreCheckFailure {
	if in.BackupVersion == "" {
		return incompatible(
			"the backup did not record the Camunda version it was taken with, so the restore cannot " +
				"prove that the target can read it",
		)
	}

	if _, err := parseVersion(in.BackupVersion); err != nil {
		return incompatible("the backup recorded the Camunda version %q, which %s", in.BackupVersion, err)
	}

	if _, err := parseVersion(in.TargetVersion); err != nil {
		return incompatible("the target cluster runs the Camunda version %q, which %s", in.TargetVersion, err)
	}

	if in.BackupVersion != in.TargetVersion {
		return incompatible(
			"the backup was taken with Camunda %s and the target cluster runs %s. A relational backup "+
				"restores only with the exact version it was taken with, because the restore application "+
				"runs with the version of the broker image",
			in.BackupVersion, in.TargetVersion,
		)
	}

	return nil
}

// parseVersion reads a version of the form x.y.z and returns its three
// numbers. The exact-version rule compares the recorded strings, so the parse
// exists to refuse a value that names no version at all.
func parseVersion(value string) ([3]int, error) {
	parts := strings.Split(value, ".")
	if len(parts) != 3 {
		return [3]int{}, errNotVersion
	}

	var numbers [3]int
	for i, part := range parts {
		number, err := strconv.Atoi(part)
		if err != nil || number < 0 {
			return [3]int{}, errNotVersion
		}
		numbers[i] = number
	}

	return numbers, nil
}

// incompatible builds the failure of a target that cannot hold the backup.
func incompatible(format string, args ...any) *conditions.PreCheckFailure {
	return &conditions.PreCheckFailure{
		Reason:  v1.ReasonIncompatibleTarget,
		Message: fmt.Sprintf(format, args...),
	}
}
