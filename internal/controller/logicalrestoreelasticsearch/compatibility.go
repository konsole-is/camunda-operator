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

package logicalrestoreelasticsearch

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

// validate compares the backup against the target and fails the restore when
// the target cannot hold it. It runs before the first destructive step, so a
// mismatch costs nothing but the resource.
func (r *Reconciler) validate(
	ctx context.Context,
	lres *v1.LogicalRestoreElasticsearch,
) (restore.Outcome, error) {
	resolved, failure, err := r.resolve(ctx, lres)
	if err != nil {
		return restore.Outcome{}, err
	}
	if failure != nil {
		return r.holdStarted(lres, failure), nil
	}

	if failure := check(compatibility{
		TargetStorageType: resolved.storage.Spec.Type,
		BackupPartitions:  resolved.backup.Partitions,
		TargetPartitions:  resolved.target.Partitions,
		BackupBucket:      resolved.backup.Bucket,
		TargetBucket:      resolved.cluster.Spec.BackupStorageRef,
		BackupVersion:     resolved.backup.Version,
		TargetVersion:     resolved.target.Version,
	}); failure != nil {
		r.fail(lres, failure.Reason, fmt.Sprintf(
			"the target cannot hold backup %s/%s: %s",
			resolved.backup.Namespace, resolved.backup.Name, failure.Message,
		))

		return restore.Outcome{}, nil
	}

	restore.Recovered(&lres.Status.RestoreProgress)
	lres.Status.Phase = v1.LogicalRestoreRestoringSecondaryStorage
	r.progressing(lres, "the restore writes the backup into the secondary storage of the target")

	return restore.Outcome{Wait: restore.Shortly}, nil
}

// compatibility is what the ValidatingCompatibility phase compares: the facts
// of the backup against the facts of the target. The target facts come from
// the live broker StatefulSet and the storage contract of the target, because
// the management binding of a suspended cluster is unset.
type compatibility struct {
	// TargetStorageType is the type of the storage contract of the target. An
	// Elasticsearch backup restores into Elasticsearch alone.
	TargetStorageType v1.SecondaryStorageType
	// BackupPartitions is the partition count that the backup recorded.
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
	if in.TargetStorageType != v1.SecondaryStorageTypeElasticsearch {
		return incompatible(
			"the backup holds %s data and the target cluster stores its data in %s",
			v1.SecondaryStorageTypeElasticsearch, in.TargetStorageType,
		)
	}

	if in.BackupPartitions != in.TargetPartitions {
		return incompatible(
			"the backup was taken from %d partitions and the target cluster runs %d",
			in.BackupPartitions, in.TargetPartitions,
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

// checkVersions applies the version rule of an Elasticsearch backup. Such a
// backup restores only with the exact version it was taken with, because that
// version is part of every snapshot name. A backup that recorded no version
// fails closed.
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
			"the backup was taken with Camunda %s and the target cluster runs %s. An Elasticsearch "+
				"backup restores only with the exact version it was taken with, because that version "+
				"is part of every snapshot name",
			in.BackupVersion, in.TargetVersion,
		)
	}

	return nil
}

// errNotVersion says that a value is not a Camunda version. It reads as the
// tail of "the version %q, which ...".
var errNotVersion = errors.New("is not a version of the form x.y.z")

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
