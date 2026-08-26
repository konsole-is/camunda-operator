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
	// Cluster is the CamundaCluster that the backup was taken from. The
	// restore application reads the partition backup under the prefix of the
	// cluster it runs as, so only that cluster is a target.
	Cluster string
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
// every one of them answers and no other operation holds the cluster. It pins
// what the restore reads, the backup id and the identity of the target, then
// claims the cluster and carries it to the state the restore needs:
// suspended, its brokers stopped, and running the Camunda version of the
// backup. From then on the restore never returns here, and a reference that
// breaks runs through the mid-run grace of its phase instead.
func (r *Reconciler) admit(
	ctx context.Context,
	lres *v1.LogicalRestoreElasticsearch,
) (restore.Outcome, error) {
	cluster, failure, err := restore.ResolveCluster(
		ctx,
		r.APIReader,
		types.NamespacedName{Namespace: lres.Namespace, Name: lres.Spec.TargetClusterRef.Name},
		lres.Status.TargetClusterUID,
	)
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
	if failure := pinnedBackup(lres.Status.BackupID, source); failure != nil {
		return r.waiting(lres, failure), nil
	}

	// The identities are pinned before the restore writes anything, and this
	// look returns here so the record is durable before that write.
	//
	// Preparation holds for as many looks as the brokers and the version
	// rollout need, and admission re-enters from the top on every one of
	// them. A cluster or a backup that somebody deletes and creates again
	// under one name during that hold would otherwise read as the original,
	// because both guards compare against a pin that is not there yet.
	if lres.Status.TargetClusterUID == "" || lres.Status.BackupID == 0 {
		lres.Status.TargetClusterUID = cluster.UID
		lres.Status.BackupID = source.ID
		r.progressing(lres, "the restore pinned the backup and the target it prepares")

		return restore.Outcome{Wait: restore.Shortly}, nil
	}

	// Every rule of this restore holds, so the cluster becomes this restore's
	// alone. The claim point is the same for all three restore kinds, and it
	// comes before every phase that touches storage, and before the restore
	// writes anything on the cluster spec. Two restores of one cluster
	// therefore never both pass validation.
	claimed, err := restore.Take(
		ctx, r.Client, r.APIReader, lres, lres.Spec.TargetClusterRef.Name,
	)
	if err != nil {
		return restore.Outcome{}, err
	}
	if claimed.Failure != nil {
		return r.waiting(lres, claimed.Failure), nil
	}

	target, failure, err := restore.ResolveTarget(ctx, r.APIReader, cluster)
	if err != nil {
		return restore.Outcome{}, err
	}
	if failure != nil {
		return r.waiting(lres, failure), nil
	}

	// The restore carries the cluster to the state it needs: suspended, its
	// brokers stopped, and running the Camunda version of the backup. It
	// holds here until the cluster is there, and it has destroyed nothing
	// yet, so the hold costs nothing.
	prepared, err := restore.Prepare(
		ctx, r.Client, &lres.Status.RestoreProgress, restore.PrepareInput{
			Owner:   lres,
			Cluster: cluster,
			Target:  target,
			Version: source.Version,
			Poll:    r.opts.PollInterval,
		},
	)
	if err != nil {
		return restore.Outcome{}, err
	}
	if prepared.Failure != nil {
		return r.waiting(lres, prepared.Failure), nil
	}
	if !prepared.Done {
		// The restore stays in Pending while it prepares its cluster. It has
		// erased nothing, and it recovers on its own once the cluster
		// converges.
		lres.Status.Phase = v1.LogicalRestorePending

		return prepared, nil
	}

	r.start(lres)

	return restore.Outcome{Wait: restore.Shortly}, nil
}

// start moves the restore into the validation phase. It pins nothing:
// admission pinned the backup id and the identity of the target earlier, on
// the look before it first wrote to the cluster.
func (r *Reconciler) start(lres *v1.LogicalRestoreElasticsearch) {
	lres.Status.Phase = v1.LogicalRestoreValidatingCompatibility
	r.progressing(lres, "the restore compares the backup against the target")
}

// pinnedBackup reports the backup that is not the backup the restore pinned.
// The pinned id is the identity of the backup, and the name alone is not: a
// backup that somebody deleted and created again under one name is another set
// of artifacts, whose snapshots lie under other names and pair with another
// record snapshot. A restore that has pinned nothing yet passes.
func pinnedBackup(pinned int64, source *backup) *conditions.PreCheckFailure {
	if pinned == 0 || source.ID == pinned {
		return nil
	}

	return logicalbackup.InvalidReference(
		"LogicalBackupElasticsearch %s/%s holds backup %d and the restore started against %d, "+
			"so it is another backup",
		source.Namespace, source.Name, source.ID, pinned,
	)
}

// notSuspended reports the target that started running again. A restore
// rewrites the storage of its target, so it only touches a cluster whose
// workloads are scaled down.
//
// Only a phase after admission reports it. Admission suspends the cluster
// itself, so a cluster that is not suspended there is one that the restore
// is about to suspend.
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
		Cluster:          source.Spec.ClusterRef.Name,
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
	cluster, failure, err := restore.ResolveCluster(
		ctx,
		r.APIReader,
		types.NamespacedName{Namespace: lres.Namespace, Name: lres.Spec.TargetClusterRef.Name},
		lres.Status.TargetClusterUID,
	)
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
	if failure := pinnedBackup(lres.Status.BackupID, source); failure != nil {
		return nil, failure, nil
	}

	target, failure, err := restore.ResolveTarget(ctx, r.APIReader, cluster)
	if err != nil || failure != nil {
		return nil, failure, err
	}
	// The version of the target is a standing condition too, for the same
	// reason its suspension is. The restore Jobs copy the broker image, so a
	// version that moves mid-run reaches the restore application itself.
	if failure := restore.MovedVersion(source.Version, target.Version); failure != nil {
		return nil, failure, nil
	}

	storage, failure, err := restore.ResolveStorage(ctx, r.APIReader, cluster)
	if err != nil || failure != nil {
		return nil, failure, err
	}

	return &resolution{cluster: cluster, backup: source, target: target, storage: storage}, nil, nil
}

// backupBucket reads the ObjectStorageConfig that the backup wrote its
// snapshots to, from namespace, the namespace of the target cluster. The
// compatibility check already proved that the target backs up through the same
// contract.
func (r *Reconciler) backupBucket(
	ctx context.Context,
	namespace, name string,
) (*v1.ObjectStorageConfig, *conditions.PreCheckFailure, error) {
	if name == "" {
		return nil, logicalbackup.InvalidReference(
			"the backup did not record the ObjectStorageConfig that holds its snapshots",
		), nil
	}

	key := types.NamespacedName{Namespace: namespace, Name: name}

	var bucket v1.ObjectStorageConfig
	if err := r.APIReader.Get(ctx, key, &bucket); err != nil {
		if apierrors.IsNotFound(err) {
			return nil, logicalbackup.InvalidReference("ObjectStorageConfig %s does not exist", key), nil
		}

		return nil, nil, fmt.Errorf("reading ObjectStorageConfig %s: %w", key, err)
	}

	return &bucket, nil, nil
}
