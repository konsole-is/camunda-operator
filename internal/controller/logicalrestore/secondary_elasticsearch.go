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
	"slices"

	v1 "github.com/konsole-is/camunda-operator/api/v1"
	elasticsearch "github.com/konsole-is/camunda-operator/pkg/components/elasticsearchcluster"
	"github.com/konsole-is/camunda-operator/pkg/conditions"
	"github.com/konsole-is/camunda-operator/pkg/esadmin"
	"github.com/konsole-is/camunda-operator/pkg/logicalbackup"
	"github.com/konsole-is/camunda-operator/pkg/wrappers/secondarystorageconfig"
)

// allIndicesOfSnapshot restores every index that a snapshot holds. Camunda
// splits the web-application indices over several snapshots, and a restore
// takes each snapshot whole (Camunda 8.9 restore guide, step 5).
var allIndicesOfSnapshot = []string{"*"}

// restoreElasticsearch writes the snapshots of the backup back into the
// Elasticsearch of the target. Camunda exposes no restore endpoint, so the
// operator talks to Elasticsearch itself, with the credentials of the storage
// contract of the target.
//
// The work runs in two looks. The first registers the snapshot repository,
// deletes the Camunda indices of the target, asks Elasticsearch to restore
// every snapshot, and records their names. Every later look polls the
// recovery. The recorded names are the resume marker: a look that finds them
// never deletes an index again.
//
// A failure between the delete and the restore leaves the secondary storage
// of the target empty. The backup itself stays whole, and the next look
// deletes what is there and asks for the restore again, so the retry
// converges.
func (r *Reconciler) restoreElasticsearch(ctx context.Context, restore *v1.LogicalRestore) (hold, error) {
	resolved, failure, err := r.resolveTarget(ctx, restore)
	if err != nil {
		return settle, err
	}
	if failure != nil {
		return r.holdRunning(restore, failure), nil
	}

	admin, failure, err := secondarystorageconfig.ElasticsearchAdmin(ctx, r.APIReader, resolved.storage)
	if err != nil {
		return settle, err
	}
	if failure != nil {
		return r.holdRunning(restore, failure), nil
	}

	if len(restore.Status.RestoredSnapshots) == 0 {
		return r.startElasticsearchRestore(ctx, restore, resolved, admin)
	}

	return r.trackElasticsearchRestore(ctx, restore, resolved, admin)
}

// startElasticsearchRestore registers the repository, empties the target, and
// asks Elasticsearch to restore every snapshot of the backup.
func (r *Reconciler) startElasticsearchRestore(
	ctx context.Context,
	restore *v1.LogicalRestore,
	resolved *resolution,
	admin *esadmin.Client,
) (hold, error) {
	repository, failure, err := r.ensureRepository(ctx, resolved, admin)
	if err != nil {
		return settle, err
	}
	if failure != nil {
		return r.holdRunning(restore, failure), nil
	}
	restore.Status.Repository = repository

	// The Optimize indices go only when the backup holds Optimize snapshots. A
	// backup without them cannot put them back, and deleting them would erase
	// Optimize data that no snapshot restores.
	patterns := restoredIndexPatterns(resolved.backup)
	if err := admin.DeleteIndices(ctx, patterns); err != nil {
		return r.holdRunning(restore, elasticsearchFailure(
			"deleting the Camunda indices of the target", err,
		)), nil
	}

	snapshots := restoredSnapshots(resolved.backup, restore.Status.BackupID)
	for _, snapshot := range snapshots {
		if err := admin.RestoreSnapshot(ctx, repository, snapshot, allIndicesOfSnapshot); err != nil {
			return r.holdRunning(restore, elasticsearchFailure(
				fmt.Sprintf("restoring the snapshot %s", snapshot), err,
			)), nil
		}
	}
	recovered(restore)

	restore.Status.RestoredSnapshots = snapshots
	conditions.Stage(restore, progressing(
		restore, "Elasticsearch restores the snapshots of the backup",
	))

	return shortly, nil
}

// trackElasticsearchRestore polls the recovery of the restored indices. The
// restore of a snapshot is asynchronous, and the recovery is what says that
// the data arrived.
func (r *Reconciler) trackElasticsearchRestore(
	ctx context.Context,
	restore *v1.LogicalRestore,
	resolved *resolution,
	admin *esadmin.Client,
) (hold, error) {
	patterns := restoredIndexPatterns(resolved.backup)

	// The recovery answers for the indices that exist. Right after the restore
	// requests were accepted, Elasticsearch has not created them yet, and a
	// recovery of nothing reads as a recovery that finished. The indices are
	// therefore the first half of the answer.
	indices, err := admin.ResolveIndices(ctx, patterns)
	if err != nil {
		return r.holdRunning(restore, elasticsearchFailure(
			"reading the restored indices", err,
		)), nil
	}
	if len(indices) == 0 {
		conditions.Stage(restore, progressing(
			restore, "Elasticsearch did not create the restored indices yet",
		))

		return hold{after: r.opts.PollInterval}, nil
	}

	state, err := admin.RestoreProgress(ctx, patterns)
	if err != nil {
		return r.holdRunning(restore, elasticsearchFailure(
			"reading the recovery of the restored indices", err,
		)), nil
	}

	if state == esadmin.RestoreInProgress {
		conditions.Stage(restore, progressing(
			restore, "Elasticsearch is still recovering the restored indices",
		))

		return hold{after: r.opts.PollInterval}, nil
	}

	recovered(restore)
	restore.Status.Phase = v1.LogicalRestoreRestoringPrimaryStorage
	conditions.Stage(restore, progressing(
		restore, "the secondary storage is restored; the broker volumes come next",
	))

	return shortly, nil
}

// ensureRepository registers the snapshot repository that holds the artifacts
// of the backup on the Elasticsearch of the target. Its settings come from the
// bucket that the backup pinned, and its prefix is the prefix that the source
// cluster wrote under. That is what lets a backup restore into a second
// cluster.
//
// The repository keeps the name that the backup recorded. On a restore into
// the source cluster, that registration is the one the ElasticsearchCluster
// controller already keeps, with the same settings, so the call converges on
// what is there. Two ElasticsearchCluster resources of the same name in
// different namespaces that share one Elasticsearch would point the same
// registration at another prefix, and the controller of the target puts its
// own back on its next look.
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

	bucket, failure, err := r.backupBucket(ctx, resolved.backup.Bucket)
	if err != nil || failure != nil {
		return "", failure, err
	}
	if err := elasticsearch.ValidateSnapshotStorage(bucket); err != nil {
		return "", logicalbackup.InvalidReference("%s", err.Error()), nil
	}

	// The credentials are left out on purpose. Elasticsearch reads them from
	// the node keystore, which the ElasticsearchCluster controller of the
	// target fills for the same bucket.
	storage := &elasticsearch.SnapshotStorage{Config: bucket}
	config := elasticsearch.RepositoryConfigAt(storage, logicalbackup.ClusterPrefix(
		bucket.BasePath(), resolved.backup.Namespace, repository,
	))

	if err := admin.EnsureSnapshotRepository(ctx, repository, config); err != nil {
		return "", elasticsearchFailure(
			fmt.Sprintf("registering the snapshot repository %s", repository), err,
		), nil
	}

	return repository, nil, nil
}

// restoredIndexPatterns are the index patterns that the restore replaces on
// the target.
func restoredIndexPatterns(source *backup) []string {
	return logicalbackup.CamundaIndexPatterns(logicalbackup.HasOptimizeSnapshot(source.HistorySnapshots))
}

// restoredSnapshots names every snapshot that the restore asks Elasticsearch
// for: the web-application snapshots that the backup recorded, and the record
// snapshot that the backup id names.
func restoredSnapshots(source *backup, id int64) []string {
	snapshots := slices.Clone(source.HistorySnapshots)

	return append(snapshots, logicalbackup.RecordsSnapshotName(id))
}

// elasticsearchFailure maps a client error to the failure a user sees.
// Elasticsearch that does not answer is a connection failure, and everything
// else is the answer of Elasticsearch itself.
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
