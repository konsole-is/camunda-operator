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

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/types"

	v1 "github.com/konsole-is/camunda-operator/api/v1"
	"github.com/konsole-is/camunda-operator/pkg/conditions"
	"github.com/konsole-is/camunda-operator/pkg/logicalbackup"
	restorepkg "github.com/konsole-is/camunda-operator/pkg/restore"
)

// backup is what a restore reads off the backup it restores, whichever kind
// that is. The two kinds record the same facts under different fields, and
// every phase after admission reads them through this one shape.
type backup struct {
	// Namespace and Name locate the backup resource. It lives in the
	// namespace of the restore.
	Namespace string
	Name      string
	// StorageType is the secondary storage type of the backup kind. It
	// decides which restore procedure runs.
	StorageType v1.SecondaryStorageType
	// ID is the backup id that keys every artifact of the backup.
	ID int64
	// Partitions is the partition count that the backup recorded, and zero
	// when the kind records none.
	Partitions int32
	// Version is the Camunda version that the cluster ran when the backup was
	// taken. It is empty for a backup that recorded none.
	Version string
	// Bucket is the ObjectStorageConfig that the backup wrote its artifacts
	// to. The target must back up through the same one.
	Bucket string
	// Repository is the Elasticsearch snapshot repository that holds the
	// snapshots. It is empty on the relational path.
	Repository string
	// HistorySnapshots names the snapshots of the web-application indices. It
	// is empty on the relational path.
	HistorySnapshots []string
	// ObjectKey is the key of the dump in the bucket. It is empty on the
	// Elasticsearch path.
	ObjectKey string
	// ZeebeSize is the effective restore size that the backup recorded for
	// the broker volumes, or nil when it recorded none.
	ZeebeSize *resource.Quantity
}

// admit resolves the references of the restore and holds it in Pending until
// every one of them answers. It ends by pinning what the restore reads: the
// backup id, the storage type, and the identity of the target. From then on
// the restore never returns here, and a reference that breaks runs through
// the mid-run grace of its phase instead.
func (r *Reconciler) admit(ctx context.Context, restore *v1.LogicalRestore) (hold, error) {
	cluster, failure, err := r.targetCluster(ctx, restore)
	if err != nil {
		return settle, err
	}
	if failure == nil {
		var source *backup
		source, failure, err = r.readBackup(ctx, restore)
		if err != nil {
			return settle, err
		}
		if failure == nil {
			// The controller reads spec.suspend and never writes it. Suspending
			// the target and unsuspending it afterwards belongs to whoever owns
			// the cluster.
			if !cluster.Spec.Suspend {
				failure = &conditions.PreCheckFailure{
					Reason: v1.ReasonClusterNotSuspended,
					Message: fmt.Sprintf(
						"CamundaCluster %s/%s is not suspended; a restore rewrites its storage, so it "+
							"waits until spec.suspend is true",
						cluster.Namespace, cluster.Name,
					),
				}
			} else {
				r.start(restore, cluster, source)

				return r.shortly(), nil
			}
		}
	}

	restore.Status.Phase = v1.LogicalRestorePending
	conditions.Stage(restore, conditions.Failed(restore, failure))

	// The watches on the cluster and on both backup kinds resolve every hold
	// that admission reports. The timer is the safety net.
	return hold{after: r.opts.RetryInterval}, nil
}

// start pins what the restore reads and moves it into the validation phase.
// The pins are written before the first side effect, so a backup that is
// deleted afterwards cannot move the restore to another set of artifacts.
func (r *Reconciler) start(restore *v1.LogicalRestore, cluster *v1.CamundaCluster, source *backup) {
	restore.Status.Phase = v1.LogicalRestoreValidatingCompatibility
	restore.Status.BackupID = source.ID
	restore.Status.StorageType = source.StorageType
	restore.Status.TargetClusterUID = cluster.UID
	conditions.Stage(restore, progressing(restore, "the restore compares the backup against the target"))
	r.EventRecorder.Eventf(
		restore,
		nil,
		corev1.EventTypeNormal,
		eventReasonStarted,
		eventActionRestore,
		"Restore of backup %d into CamundaCluster %s/%s started",
		source.ID,
		cluster.Namespace,
		cluster.Name,
	)
}

// targetCluster reads the target of the restore. A cluster that does not
// exist is a failure the user corrects. After admission the pinned UID must
// match too: a cluster that was deleted and created again under the same name
// is another cluster, and this restore is not its restore.
func (r *Reconciler) targetCluster(
	ctx context.Context,
	restore *v1.LogicalRestore,
) (*v1.CamundaCluster, *conditions.PreCheckFailure, error) {
	key := types.NamespacedName{
		Namespace: restore.Namespace,
		Name:      restore.Spec.TargetClusterRef.Name,
	}

	var cluster v1.CamundaCluster
	if err := r.APIReader.Get(ctx, key, &cluster); err != nil {
		if apierrors.IsNotFound(err) {
			return nil, logicalbackup.InvalidReference("CamundaCluster %s does not exist", key), nil
		}

		return nil, nil, fmt.Errorf("reading CamundaCluster %s: %w", key, err)
	}

	if restore.Status.TargetClusterUID != "" && cluster.UID != restore.Status.TargetClusterUID {
		return nil, logicalbackup.InvalidReference(
			"CamundaCluster %s has UID %s and the restore started against %s, so it is another cluster",
			key, cluster.UID, restore.Status.TargetClusterUID,
		), nil
	}

	return &cluster, nil, nil
}

// readBackup reads the backup that the restore names, whichever kind it is,
// and reports a backup that is not completed as a failure the user corrects.
// A backup that is deleted after the restore started is a failure too: the
// restore keeps its pinned id, and the phases that still read the backup
// cannot run without it.
func (r *Reconciler) readBackup(
	ctx context.Context,
	restore *v1.LogicalRestore,
) (*backup, *conditions.PreCheckFailure, error) {
	ref := restore.Spec.BackupRef
	key := types.NamespacedName{Namespace: restore.Namespace, Name: ref.Name}

	switch ref.Kind {
	case v1.LogicalBackupKindElasticsearch:
		var source v1.LogicalBackupElasticsearch
		if err := r.APIReader.Get(ctx, key, &source); err != nil {
			failure, readErr := backupUnreadable(ref.Kind, key, err)

			return nil, failure, readErr
		}
		if source.Status.Phase != v1.LogicalBackupCompleted {
			return nil, backupNotCompleted(ref.Kind, key, source.Status.Phase), nil
		}

		return elasticsearchBackup(&source), nil, nil
	case v1.LogicalBackupKindRDBMS:
		var source v1.LogicalBackupRDBMS
		if err := r.APIReader.Get(ctx, key, &source); err != nil {
			failure, readErr := backupUnreadable(ref.Kind, key, err)

			return nil, failure, readErr
		}
		if source.Status.Phase != v1.LogicalBackupCompleted {
			return nil, backupNotCompleted(ref.Kind, key, source.Status.Phase), nil
		}

		return relationalBackup(&source), nil, nil
	default:
		return nil, nil, fmt.Errorf("unknown backup kind %q", ref.Kind)
	}
}

// elasticsearchBackup reads the facts of an Elasticsearch backup. A backup
// that never started carries no pinned storage, and the compatibility check
// then reports the empty bucket.
func elasticsearchBackup(source *v1.LogicalBackupElasticsearch) *backup {
	facts := &backup{
		Namespace:        source.Namespace,
		Name:             source.Name,
		StorageType:      v1.SecondaryStorageTypeElasticsearch,
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

// relationalBackup reads the facts of a relational backup. It records no
// partition count: the restore application reads the exporter position from
// the restored database and aligns the partitions itself.
func relationalBackup(source *v1.LogicalBackupRDBMS) *backup {
	return &backup{
		Namespace:   source.Namespace,
		Name:        source.Name,
		StorageType: v1.SecondaryStorageTypeRDBMS,
		ID:          source.Status.BackupID,
		Version:     source.Status.Version,
		Bucket:      source.Status.BucketRef,
		ObjectKey:   source.Status.ObjectKey,
		ZeebeSize:   source.Status.StorageSizes.Zeebe,
	}
}

func backupUnreadable(
	kind v1.LogicalBackupKind,
	key types.NamespacedName,
	err error,
) (*conditions.PreCheckFailure, error) {
	if apierrors.IsNotFound(err) {
		return logicalbackup.InvalidReference("%s %s does not exist", kind, key), nil
	}

	return nil, fmt.Errorf("reading %s %s: %w", kind, key, err)
}

func backupNotCompleted(
	kind v1.LogicalBackupKind,
	key types.NamespacedName,
	phase v1.LogicalBackupPhase,
) *conditions.PreCheckFailure {
	reported := string(phase)
	if reported == "" {
		reported = "no phase yet"
	}

	return logicalbackup.InvalidReference(
		"%s %s reports %s; a restore reads a Completed backup only", kind, key, reported,
	)
}

// resolution is everything a phase after admission reads again: the target
// cluster and its identity, the backup it restores, the facts of the live
// broker StatefulSet, and the storage contract of the target. A phase
// resolves it on every look, because a reference that breaks mid-run has to
// reach the Ready condition and the mid-run grace.
type resolution struct {
	cluster *v1.CamundaCluster
	backup  *backup
	target  *restorepkg.Target
	storage *v1.SecondaryStorageConfig
}

// resolveTarget resolves everything a running phase needs and reports the
// first failure that the user must see.
func (r *Reconciler) resolveTarget(
	ctx context.Context,
	restore *v1.LogicalRestore,
) (*resolution, *conditions.PreCheckFailure, error) {
	cluster, failure, err := r.targetCluster(ctx, restore)
	if err != nil || failure != nil {
		return nil, failure, err
	}

	source, failure, err := r.readBackup(ctx, restore)
	if err != nil || failure != nil {
		return nil, failure, err
	}

	resolved, failure, err := r.target(ctx, cluster)
	if err != nil || failure != nil {
		return nil, failure, err
	}

	storage, failure, err := r.targetStorage(ctx, cluster)
	if err != nil || failure != nil {
		return nil, failure, err
	}

	return &resolution{cluster: cluster, backup: source, target: resolved, storage: storage}, nil, nil
}

// targetStorage reads the secondary storage contract of the target. The
// restore reaches Elasticsearch or the logical database through it, with the
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
			return nil, logicalbackup.InvalidReference("SecondaryStorageConfig %s does not exist", key), nil
		}

		return nil, nil, fmt.Errorf("reading SecondaryStorageConfig %s: %w", key, err)
	}

	return &storage, nil, nil
}

// target reads the live broker StatefulSet of the target cluster. Every phase
// after admission needs it: the broker count, the partition count, the
// Camunda version, and the claim template all come from there, because the
// management binding of a suspended cluster is unset.
func (r *Reconciler) target(
	ctx context.Context,
	cluster *v1.CamundaCluster,
) (*restorepkg.Target, *conditions.PreCheckFailure, error) {
	resolved, err := restorepkg.ReadTarget(ctx, r.APIReader, cluster)
	if err != nil {
		var failure *conditions.PreCheckFailure
		if errors.As(err, &failure) {
			return nil, failure, nil
		}

		return nil, nil, err
	}

	return resolved, nil, nil
}
