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
	"slices"

	v1 "github.com/konsole-is/camunda-operator/api/v1"
	elasticsearch "github.com/konsole-is/camunda-operator/pkg/components/elasticsearchcluster"
	"github.com/konsole-is/camunda-operator/pkg/conditions"
	"github.com/konsole-is/camunda-operator/pkg/esadmin"
	"github.com/konsole-is/camunda-operator/pkg/logicalbackup"
	"github.com/konsole-is/camunda-operator/pkg/restore"
	"github.com/konsole-is/camunda-operator/pkg/wrappers/secondarystorageconfig"
)

// allIndicesOfSnapshot restores every index that a snapshot holds. Camunda
// splits the web-application indices over several snapshots, and a restore
// takes each snapshot whole (Camunda 8.9 restore guide, step 5).
var allIndicesOfSnapshot = []string{"*"}

// restoreSecondaryStorage writes the snapshots of the backup back into the
// Elasticsearch of the target. It takes several looks: call it again after
// Outcome.Wait until the phase moves on.
func (r *Reconciler) restoreSecondaryStorage(
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

	admin, failure, err := secondarystorageconfig.ElasticsearchAdmin(ctx, r.APIReader, resolved.storage)
	if err != nil {
		return restore.Outcome{}, err
	}
	if failure != nil {
		return r.holdStarted(lres, failure), nil
	}

	// The recorded names are the resume marker: a look that finds them never deletes an index again.
	if len(lres.Status.RestoredSnapshots) == 0 {
		return r.startRestore(ctx, lres, resolved, admin)
	}

	// Nothing in this phase clears the mid-run grace, because the target indices are already deleted.
	return r.trackRestore(ctx, lres, resolved, admin)
}

// startRestore registers the repository, empties the target, and asks
// Elasticsearch to restore every snapshot of the backup.
func (r *Reconciler) startRestore(
	ctx context.Context,
	lres *v1.LogicalRestoreElasticsearch,
	resolved *resolution,
	admin *esadmin.Client,
) (restore.Outcome, error) {
	// A failed or deleted restore holds its backend only once the repository
	// is in status, so it is recorded before the first index is deleted.
	if lres.Status.Repository == "" {
		repository, failure, err := r.ensureRepository(ctx, resolved, admin)
		if err != nil {
			return restore.Outcome{}, err
		}
		if failure != nil {
			return r.holdStarted(lres, failure), nil
		}
		lres.Status.Repository = repository

		return restore.Outcome{Wait: restore.Shortly}, nil
	}
	repository := lres.Status.Repository

	// The Optimize indices go only when the backup holds Optimize snapshots. A
	// backup without them cannot put them back, and deleting them would erase
	// Optimize data that no snapshot restores.
	patterns := restoredIndexPatterns(resolved.backup)
	if err := admin.DeleteIndices(ctx, patterns); err != nil {
		return r.holdStarted(lres, elasticsearchFailure(
			"deleting the Camunda indices of the target", err,
		)), nil
	}

	snapshots := restoredSnapshots(resolved.backup, lres.Status.BackupID)
	// A failure here leaves RestoredSnapshots unset, so the next look deletes and restores again.
	for _, snapshot := range snapshots {
		if err := admin.RestoreSnapshot(ctx, repository, snapshot, allIndicesOfSnapshot); err != nil {
			return r.holdStarted(lres, elasticsearchFailure(
				fmt.Sprintf("restoring the snapshot %s", snapshot), err,
			)), nil
		}
	}

	lres.Status.RestoredSnapshots = snapshots
	r.progressing(lres, "Elasticsearch restores the snapshots of the backup")

	return restore.Outcome{Wait: restore.Shortly}, nil
}

// ensureRepository makes sure that the Elasticsearch of the target holds the
// snapshot repository that the backup recorded, and returns its name.
//
// A repository that is already registered is used as it is. The restore never
// overwrites one, because it does not own every registration it finds: the
// Elasticsearch of a target can be a cluster that this operator does not
// manage, where an operator registered the repository by hand, and a PUT
// would point that registration at another prefix of another bucket. A
// registration that points elsewhere makes the restore fail on a snapshot
// that is missing, which names the repository and which a human can correct.
//
// Only an absent repository is registered. Its settings come from the bucket
// that the backup pinned, and its prefix is the prefix that the source
// cluster wrote under, read back out of the repository name. The snapshots
// lie under that prefix, whichever Elasticsearch server the target reads
// through. A name that this operator did not produce carries no prefix, so
// the restore reports it instead of guessing one.
func (r *Reconciler) ensureRepository(
	ctx context.Context,
	resolved *resolution,
	admin *esadmin.Client,
) (string, *conditions.PreCheckFailure, error) {
	repository := resolved.backup.Repository
	if repository == "" {
		return "", logicalbackup.InvalidReference(
			"the backup did not record the snapshot repository that holds its snapshots",
		), nil
	}

	registered, err := admin.SnapshotRepositoryExists(ctx, repository)
	if err != nil {
		return "", elasticsearchFailure(
			fmt.Sprintf("reading the snapshot repository %s", repository), err,
		), nil
	}
	if registered {
		return repository, nil, nil
	}

	bucket, failure, err := r.backupBucket(ctx, resolved.cluster.Namespace, resolved.backup.Bucket)
	if err != nil || failure != nil {
		return "", failure, err
	}
	if err := elasticsearch.ValidateSnapshotStorage(bucket); err != nil {
		return "", logicalbackup.InvalidReference("%s", err.Error()), nil
	}

	basePath, ok := elasticsearch.RepositoryBasePath(bucket.BasePath(), repository)
	if !ok {
		return "", logicalbackup.InvalidReference(
			"the Elasticsearch of the target holds no snapshot repository %q. The name is not one "+
				"that this operator registers. The restore therefore cannot tell which prefix of "+
				"ObjectStorageConfig %q holds the snapshots. Register the repository on the "+
				"target, over the prefix that holds the snapshots of the backup",
			repository, bucket.Name,
		), nil
	}

	// The credentials are left out on purpose. Elasticsearch reads them from
	// the node keystore, which the ElasticsearchCluster controller of the
	// target fills for the same bucket.
	storage := &elasticsearch.SnapshotStorage{Config: bucket}
	config := elasticsearch.RepositoryConfigAt(storage, basePath)

	if err := admin.EnsureSnapshotRepository(ctx, repository, config); err != nil {
		return "", elasticsearchFailure(
			fmt.Sprintf("registering the snapshot repository %s", repository), err,
		), nil
	}

	return repository, nil, nil
}

// restoredSnapshots names every snapshot that the restore asks Elasticsearch
// for: the web-application snapshots that the backup recorded, and the record
// snapshot that the backup id names.
func restoredSnapshots(source *backup, id int64) []string {
	snapshots := slices.Clone(source.HistorySnapshots)

	return append(snapshots, logicalbackup.RecordsSnapshotName(id))
}

// trackRestore polls the recovery of the restored indices. The restore of a
// snapshot is asynchronous, and the recovery is what says that the data
// arrived.
func (r *Reconciler) trackRestore(
	ctx context.Context,
	lres *v1.LogicalRestoreElasticsearch,
	resolved *resolution,
	admin *esadmin.Client,
) (restore.Outcome, error) {
	patterns := restoredIndexPatterns(resolved.backup)

	// The recovery answers for the indices that exist. Right after the restore
	// requests were accepted, Elasticsearch has not created them yet, and a
	// recovery of nothing reads as a recovery that finished. The indices are
	// therefore the first half of the answer.
	indices, err := admin.ResolveIndices(ctx, patterns)
	if err != nil {
		return r.holdStarted(lres, elasticsearchFailure(
			"reading the restored indices", err,
		)), nil
	}
	if len(indices) == 0 {
		r.progressing(lres, "Elasticsearch did not create the restored indices yet")

		return restore.Outcome{Wait: r.opts.PollInterval}, nil
	}

	state, err := admin.RestoreProgress(ctx, patterns)
	if err != nil {
		return r.holdStarted(lres, elasticsearchFailure(
			"reading the recovery of the restored indices", err,
		)), nil
	}

	if state == esadmin.RestoreInProgress {
		r.progressing(lres, "Elasticsearch is still recovering the restored indices")

		return restore.Outcome{Wait: r.opts.PollInterval}, nil
	}

	lres.Status.Phase = v1.LogicalRestoreRestoringPrimaryStorage
	r.progressing(lres, "the secondary storage is restored. The broker volumes come next")

	return restore.Outcome{Wait: restore.Shortly}, nil
}

// restoredIndexPatterns are the index patterns that the restore replaces on
// the target.
func restoredIndexPatterns(source *backup) []string {
	return logicalbackup.CamundaIndexPatterns(logicalbackup.HasOptimizeSnapshot(source.HistorySnapshots))
}

// elasticsearchFailure maps a client error to the failure a user sees. Both
// classes of the client are a connection failure: Elasticsearch that does not
// answer, and Elasticsearch that answers and refuses the call. That is the
// documented meaning of the reason, which api/v1 states as "a backing server
// is unreachable or rejects the configured credentials", and the CRD page
// sends the reader of it to the endpoint and the credentials.
//
// Both are held under the mid-run grace and fail the restore afterwards, so a
// refusal that no credential corrects still ends the restore. Telling a
// refused call apart from a rejected credential needs the status class of the
// answer, which the client does not carry today.
//
// Any other error is a fault of the operator itself, not of Elasticsearch.
func elasticsearchFailure(what string, err error) *conditions.PreCheckFailure {
	reason := v1.ReasonFailed
	if errors.Is(err, esadmin.ErrUnreachable) || errors.Is(err, esadmin.ErrRejected) {
		reason = v1.ReasonConnectionFailed
	}

	return &conditions.PreCheckFailure{
		Reason:  reason,
		Message: fmt.Sprintf("%s: %s", what, err),
	}
}
