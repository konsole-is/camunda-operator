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

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/types"

	v1 "github.com/konsole-is/camunda-operator/api/v1"
	"github.com/konsole-is/camunda-operator/pkg/conditions"
	"github.com/konsole-is/camunda-operator/pkg/logicalbackup"
	"github.com/konsole-is/camunda-operator/pkg/restore"
)

// backup is what a restore reads off the LogicalBackupElasticsearch it
// restores. Every phase after admission reads the backup through this shape,
// so no phase reaches into the status of another kind.
type backup struct {
	// Namespace and Name locate the backup resource. It lives in the
	// namespace of the restore.
	Namespace string
	Name      string
	// ID is the backup id that keys every artifact of the backup.
	ID int64
	// Partitions is the partition count that the backup recorded.
	Partitions int32
	// Version is the Camunda version that the cluster ran when the backup was
	// taken. It is empty for a backup that recorded none.
	Version string
	// Bucket is the ObjectStorageConfig that the backup wrote its artifacts
	// to. The target must back up through the same one.
	Bucket string
	// Repository is the Elasticsearch snapshot repository that holds the
	// snapshots.
	Repository string
	// HistorySnapshots names the snapshots of the web-application indices.
	HistorySnapshots []string
	// ZeebeSize is the effective restore size that the backup recorded for
	// the broker volumes, or nil when it recorded none.
	ZeebeSize *resource.Quantity
}

// admit resolves the references of the restore and holds it in Pending until
// every one of them answers, the target is suspended, and no other operation
// holds the cluster. It ends by claiming the cluster and pinning what the
// restore reads: the backup id and the identity of the target. From then on
// the restore never returns here, and a reference that breaks runs through
// the mid-run grace of its phase instead.
func (r *Reconciler) admit(
	ctx context.Context,
	lres *v1.LogicalRestoreElasticsearch,
) (restore.Outcome, error) {
	cluster, failure, err := r.targetCluster(ctx, lres)
	if err != nil {
		return restore.Outcome{}, err
	}
	if failure != nil {
		return r.waiting(lres, failure), nil
	}

	source, failure, err := r.readBackup(ctx, lres)
	if err != nil {
		return restore.Outcome{}, err
	}
	if failure != nil {
		return r.waiting(lres, failure), nil
	}

	// The controller reads spec.suspend and never writes it. Suspending the
	// target and unsuspending it afterwards belongs to whoever owns the
	// cluster.
	if failure := notSuspended(cluster); failure != nil {
		return r.waiting(lres, failure), nil
	}

	// Every rule of this restore holds, so the cluster becomes this restore's
	// alone. The claim point is the same for all three restore kinds, and it
	// comes before every phase that touches storage. Two restores of one
	// cluster therefore never both pass validation.
	claimed, err := restore.Take(
		ctx, r.Client, r.APIReader, lres.Namespace, lres.Spec.TargetClusterRef.Name, claimant(lres),
	)
	if err != nil {
		return restore.Outcome{}, err
	}
	if claimed.Failure != nil {
		return r.waiting(lres, claimed.Failure), nil
	}

	r.start(lres, cluster, source)

	return restore.Outcome{Wait: restore.Shortly}, nil
}

// start pins what the restore reads and moves it into the validation phase.
// The pins are written before the first side effect, so a backup that is
// deleted afterwards cannot move the restore to another set of artifacts.
func (r *Reconciler) start(
	lres *v1.LogicalRestoreElasticsearch,
	cluster *v1.CamundaCluster,
	source *backup,
) {
	lres.Status.Phase = v1.LogicalRestoreValidatingCompatibility
	lres.Status.BackupID = source.ID
	lres.Status.TargetClusterUID = cluster.UID
	r.progressing(lres, "the restore compares the backup against the target")
}

// notSuspended reports the target that still runs. A restore rewrites the
// storage of its target, so it only touches a cluster whose workloads are
// scaled down. The controller reads spec.suspend and never writes it.
func notSuspended(cluster *v1.CamundaCluster) *conditions.PreCheckFailure {
	if cluster.Spec.Suspend {
		return nil
	}

	return &conditions.PreCheckFailure{
		Reason: v1.ReasonClusterNotSuspended,
		Message: fmt.Sprintf(
			"CamundaCluster %s/%s is not suspended. Set spec.suspend to true, so that no workload "+
				"writes while the restore runs",
			cluster.Namespace, cluster.Name,
		),
	}
}

// targetCluster reads the target of the restore. A cluster that does not
// exist is a failure the user corrects. After admission the pinned UID must
// match too: a cluster that was deleted and created again under the same name
// is another cluster, and this restore is not its restore.
func (r *Reconciler) targetCluster(
	ctx context.Context,
	lres *v1.LogicalRestoreElasticsearch,
) (*v1.CamundaCluster, *conditions.PreCheckFailure, error) {
	key := types.NamespacedName{
		Namespace: lres.Namespace,
		Name:      lres.Spec.TargetClusterRef.Name,
	}

	var cluster v1.CamundaCluster
	if err := r.APIReader.Get(ctx, key, &cluster); err != nil {
		if apierrors.IsNotFound(err) {
			return nil, logicalbackup.InvalidReference("CamundaCluster %s does not exist", key), nil
		}

		return nil, nil, fmt.Errorf("reading CamundaCluster %s: %w", key, err)
	}

	if lres.Status.TargetClusterUID != "" && cluster.UID != lres.Status.TargetClusterUID {
		return nil, logicalbackup.InvalidReference(
			"CamundaCluster %s has UID %s and the restore started against %s, so it is another cluster",
			key, cluster.UID, lres.Status.TargetClusterUID,
		), nil
	}

	return &cluster, nil, nil
}

// readBackup reads the LogicalBackupElasticsearch that the restore names. The
// kind of the restore says which backup kind it reads, so the reference
// carries a name alone. A backup that is not completed is a failure the user
// corrects, and so is a backup that is deleted after the restore started: the
// restore keeps its pinned id, and the phases that still read the backup
// cannot run without it.
func (r *Reconciler) readBackup(
	ctx context.Context,
	lres *v1.LogicalRestoreElasticsearch,
) (*backup, *conditions.PreCheckFailure, error) {
	key := types.NamespacedName{Namespace: lres.Namespace, Name: lres.Spec.BackupRef.Name}

	var source v1.LogicalBackupElasticsearch
	if err := r.APIReader.Get(ctx, key, &source); err != nil {
		if apierrors.IsNotFound(err) {
			return nil, logicalbackup.InvalidReference(
				"LogicalBackupElasticsearch %s does not exist", key,
			), nil
		}

		return nil, nil, fmt.Errorf("reading LogicalBackupElasticsearch %s: %w", key, err)
	}
	if source.Status.Phase != v1.LogicalBackupCompleted {
		reported := string(source.Status.Phase)
		if reported == "" {
			reported = "no phase yet"
		}

		return nil, logicalbackup.InvalidReference(
			"LogicalBackupElasticsearch %s reports %s. A restore reads a Completed backup only",
			key, reported,
		), nil
	}

	return backupFacts(&source), nil, nil
}

// backupFacts reads the facts of an Elasticsearch backup. A backup that never
// started carries no pinned storage, and the compatibility check then reports
// the empty bucket.
func backupFacts(source *v1.LogicalBackupElasticsearch) *backup {
	facts := &backup{
		Namespace:        source.Namespace,
		Name:             source.Name,
		ID:               source.Status.BackupID,
		Partitions:       source.Status.PartitionsCount,
		Version:          source.Status.Version,
		Repository:       source.Status.Repository,
		HistorySnapshots: source.Status.HistorySnapshots,
		ZeebeSize:        source.Status.StorageSizes.Zeebe,
	}
	if source.Status.Storage != nil {
		facts.Bucket = source.Status.Storage.BucketRef
	}

	return facts
}

// resolution is everything a phase after admission reads again: the target
// cluster, the backup it restores, the facts of the live broker StatefulSet,
// and the storage contract of the target. A phase resolves it on every look,
// because a reference that breaks mid-run has to reach the Ready condition
// and the mid-run grace.
type resolution struct {
	cluster *v1.CamundaCluster
	backup  *backup
	target  *restore.Target
	storage *v1.SecondaryStorageConfig
}

// resolve resolves everything a running phase needs and reports the first
// failure that the user must see. The reads are live: a stale suspend flag or
// a stale storage reference lets the restore delete under a cluster that
// moved on.
func (r *Reconciler) resolve(
	ctx context.Context,
	lres *v1.LogicalRestoreElasticsearch,
) (*resolution, *conditions.PreCheckFailure, error) {
	cluster, failure, err := r.targetCluster(ctx, lres)
	if err != nil || failure != nil {
		return nil, failure, err
	}
	// Suspension is a standing condition of a restore, not a gate that
	// admission passes once. Every phase after admission deletes something of
	// the target: the indices of its Elasticsearch, and the data volumes of
	// its brokers. A cluster that is unsuspended mid-run starts its workloads
	// again, and the restore would delete under them. The failure holds the
	// restore for the mid-run grace and then ends it.
	if failure := notSuspended(cluster); failure != nil {
		return nil, failure, nil
	}

	source, failure, err := r.readBackup(ctx, lres)
	if err != nil || failure != nil {
		return nil, failure, err
	}
	// The pinned id is the identity of the backup, and the name alone is not.
	// A backup that somebody deleted and created again under the same name is
	// another set of artifacts: its snapshots lie under other names and pair
	// with another record snapshot.
	if lres.Status.BackupID != 0 && source.ID != lres.Status.BackupID {
		return nil, logicalbackup.InvalidReference(
			"LogicalBackupElasticsearch %s/%s holds backup %d and the restore started against %d, "+
				"so it is another backup",
			source.Namespace, source.Name, source.ID, lres.Status.BackupID,
		), nil
	}

	target, failure, err := r.readTarget(ctx, cluster)
	if err != nil || failure != nil {
		return nil, failure, err
	}

	storage, failure, err := r.targetStorage(ctx, cluster)
	if err != nil || failure != nil {
		return nil, failure, err
	}

	return &resolution{cluster: cluster, backup: source, target: target, storage: storage}, nil, nil
}

// targetStorage reads the secondary storage contract of the target. The
// restore reaches the Elasticsearch of the target through it, with the
// credentials the target's own workloads use.
func (r *Reconciler) targetStorage(
	ctx context.Context,
	cluster *v1.CamundaCluster,
) (*v1.SecondaryStorageConfig, *conditions.PreCheckFailure, error) {
	// A Get with an empty name is an invalid request, not a NotFound, so an
	// unset reference would loop as a transient error instead of reporting
	// itself.
	if cluster.Spec.StorageRef == "" {
		return nil, logicalbackup.InvalidReference(
			"CamundaCluster %s/%s has no spec.storageRef", cluster.Namespace, cluster.Name,
		), nil
	}

	key := types.NamespacedName{Namespace: cluster.Namespace, Name: cluster.Spec.StorageRef}

	var storage v1.SecondaryStorageConfig
	if err := r.APIReader.Get(ctx, key, &storage); err != nil {
		if apierrors.IsNotFound(err) {
			return nil, logicalbackup.InvalidReference(
				"SecondaryStorageConfig %s does not exist", key,
			), nil
		}

		return nil, nil, fmt.Errorf("reading SecondaryStorageConfig %s: %w", key, err)
	}

	return &storage, nil, nil
}

// readTarget reads the live broker StatefulSet of the target cluster. Every
// phase after admission needs it: the broker count, the partition count, the
// Camunda version, and the claim template all come from there, because the
// management binding of a suspended cluster is unset.
func (r *Reconciler) readTarget(
	ctx context.Context,
	cluster *v1.CamundaCluster,
) (*restore.Target, *conditions.PreCheckFailure, error) {
	target, err := restore.ReadTarget(ctx, r.APIReader, cluster)
	if err != nil {
		var failure *conditions.PreCheckFailure
		if errors.As(err, &failure) {
			return nil, failure, nil
		}

		return nil, nil, err
	}

	return target, nil, nil
}

// backupBucket reads the ObjectStorageConfig that the backup wrote its
// snapshots to. The compatibility check already proved that the target backs
// up through the same contract.
func (r *Reconciler) backupBucket(
	ctx context.Context,
	name string,
) (*v1.ObjectStorageConfig, *conditions.PreCheckFailure, error) {
	if name == "" {
		return nil, logicalbackup.InvalidReference(
			"the backup did not record the ObjectStorageConfig that holds its snapshots",
		), nil
	}

	var bucket v1.ObjectStorageConfig
	if err := r.APIReader.Get(ctx, types.NamespacedName{Name: name}, &bucket); err != nil {
		if apierrors.IsNotFound(err) {
			return nil, logicalbackup.InvalidReference("ObjectStorageConfig %q does not exist", name), nil
		}

		return nil, nil, fmt.Errorf("reading ObjectStorageConfig %q: %w", name, err)
	}

	return &bucket, nil, nil
}
