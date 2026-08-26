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

package logicalbackupelasticsearch

import (
	"context"
	"errors"
	"fmt"
	"strings"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	v1 "github.com/konsole-is/camunda-operator/api/v1"
	"github.com/konsole-is/camunda-operator/pkg/conditions"
	"github.com/konsole-is/camunda-operator/pkg/logicalbackup"
	"github.com/konsole-is/camunda-operator/pkg/management"
	"github.com/konsole-is/camunda-operator/pkg/wrappers/secondarystorageconfig"
)

// admit runs the full pre-checks and starts the backup. Admission ends when
// the backup ID is allocated. From then on the backup never returns here. A
// dependency that breaks mid-run is the state machine's to handle, not a
// reason to park.
//
// A backup that leaves admission without a start holds no claim. The claim
// is taken before the ID is allocated. A sibling-list read that fails, or a
// status flush that conflicts, can therefore leave the Lease held by a
// backup with no ID. If a pre-check then parks that backup, the held Lease blocks every
// sibling for as long as the park lasts. Every exit without a start
// therefore releases the claim. The release is a no-op read when the backup
// holds nothing.
func (r *Reconciler) admit(
	ctx context.Context,
	backup *v1.LogicalBackupElasticsearch,
) (result ctrl.Result, err error) {
	started := false
	defer func() {
		if !started {
			err = errors.Join(err, r.releaseClaim(ctx, backup))
		}
	}()

	res, err := logicalbackup.PreCheck(ctx, logicalbackup.PreCheckRequest{
		Reader:      r.APIReader,
		Ref:         backup.Spec.ClusterRef,
		Namespace:   backup.Namespace,
		StorageType: v1.SecondaryStorageTypeElasticsearch,
		InProgress:  r.inProgress(backup),
	})
	if err != nil {
		var failure *conditions.PreCheckFailure
		if !errors.As(err, &failure) {
			return ctrl.Result{}, err
		}

		backup.Status.Phase = v1.LogicalBackupPending
		conditions.Stage(backup, conditions.Failed(backup, failure))
		if logicalbackup.Waiting(err) {
			return ctrl.Result{RequeueAfter: r.poll()}, nil
		}

		// The cluster watch resolves a reference that appears later. The
		// timer covers the contracts that nothing here watches.
		return ctrl.Result{RequeueAfter: retryInterval}, nil
	}

	if result, done, err := r.admitBinding(ctx, backup, res.Cluster); done {
		return result, err
	}

	// The claim is the gate. The pre-checks above order the claimants and
	// verify the references. Only the Lease decides who holds the cluster,
	// and it is taken before the identity of the backup is written.
	blocked, err := r.claimCluster(ctx, backup)
	if err != nil {
		return ctrl.Result{}, err
	}
	if blocked != "" {
		backup.Status.Phase = v1.LogicalBackupPending
		conditions.Stage(backup, conditions.Failed(backup, &conditions.PreCheckFailure{
			Reason:  v1.ReasonBackupInProgress,
			Message: blocked,
		}))
		return ctrl.Result{RequeueAfter: r.poll()}, nil
	}

	highest, err := r.highestSiblingBackupID(ctx, backup)
	if err != nil {
		return ctrl.Result{}, err
	}

	r.start(ctx, backup, res, highest)
	started = true

	return ctrl.Result{RequeueAfter: r.poll()}, nil
}

// clusterReplaced reports whether the live cluster is a different resource
// than the one the backup pinned at its start.
func clusterReplaced(backup *v1.LogicalBackupElasticsearch, cluster *v1.CamundaCluster) bool {
	return backup.Status.ClusterUID != "" && string(cluster.UID) != backup.Status.ClusterUID
}

// failClusterLost ends the backup terminally with message. The cluster that
// the backup pinned no longer exists: it is gone, or a same-named cluster
// replaced it. Nothing of this backup is left to resume, so no management
// call is made. The terminal reason is Failed, never ResumeFailed. A
// ResumeFailed holder keeps its claim. If a claim is kept for a cluster
// that is gone, it blocks every backup of a cluster recreated later under
// the name.
func (r *Reconciler) failClusterLost(backup *v1.LogicalBackupElasticsearch, message string) {
	now := metav1.Now()
	backup.Status.Phase = v1.LogicalBackupFailed
	backup.Status.TerminalReason = v1.ReasonFailed
	backup.Status.FailureMessage = message
	backup.Status.CompletionTime = &now
	conditions.Stage(backup, terminalReady(backup))
	r.EventRecorder.Eventf(
		backup,
		nil,
		corev1.EventTypeWarning,
		eventReasonStepFailed,
		eventActionBackup,
		"%s",
		backup.Status.FailureMessage,
	)
}

// highestSiblingBackupID returns the highest backup ID among the other
// backups of this kind that name the same cluster, or zero. It arbitrates
// the ID allocation against the siblings that this controller can see. A
// clock that stepped backwards then cannot hand out an ID that one of them
// holds. The residual stays with the cluster: the IDs of the other backup
// kind and of deleted resources are arbitrated only by its own conflict
// answer.
func (r *Reconciler) highestSiblingBackupID(
	ctx context.Context,
	backup *v1.LogicalBackupElasticsearch,
) (int64, error) {
	var list v1.LogicalBackupElasticsearchList
	if err := r.APIReader.List(ctx, &list, client.InNamespace(backup.Namespace)); err != nil {
		return 0, fmt.Errorf("listing LogicalBackupElasticsearch: %w", err)
	}

	cluster := clusterKey(backup)
	var highest int64
	for i := range list.Items {
		other := &list.Items[i]
		if other.UID == backup.UID || clusterKey(other) != cluster {
			continue
		}
		highest = max(highest, other.Status.BackupID)
	}

	return highest, nil
}

// admitBinding checks the management binding of the cluster once, before the
// start. A broken binding blocks admission with its own reason. Without this
// check, the first step fails on it instead. done reports that the caller
// must return result and err.
func (r *Reconciler) admitBinding(
	ctx context.Context,
	backup *v1.LogicalBackupElasticsearch,
	cluster *v1.CamundaCluster,
) (result ctrl.Result, done bool, err error) {
	_, failure, err := management.NewClient(ctx, r.APIReader, cluster)
	if err != nil {
		return ctrl.Result{}, true, err
	}
	if failure != nil {
		backup.Status.Phase = v1.LogicalBackupPending
		conditions.Stage(backup, conditions.Failed(backup, failure))
		if failure.Reason == v1.ReasonProgressing {
			return ctrl.Result{RequeueAfter: r.poll()}, true, nil
		}
		return ctrl.Result{RequeueAfter: retryInterval}, true, nil
	}

	if cluster.Status.Management.BackupRepository == "" {
		backup.Status.Phase = v1.LogicalBackupPending
		conditions.Stage(backup, conditions.Failed(backup, &conditions.PreCheckFailure{
			Reason: v1.ReasonInvalidReference,
			Message: fmt.Sprintf(
				"CamundaCluster %s/%s publishes no backup repository; its storage contract carries no elasticsearch.snapshotRepository",
				cluster.Namespace,
				cluster.Name,
			),
		}))
		return ctrl.Result{RequeueAfter: retryInterval}, true, nil
	}

	return ctrl.Result{}, false, nil
}

// start records the identity of the procedure before the first management
// call: the backup ID, the pinned repository, the partition count, and the
// restore sizes. A crash after this point never loses the identity of work
// that already started.
func (r *Reconciler) start(
	ctx context.Context,
	backup *v1.LogicalBackupElasticsearch,
	res *logicalbackup.PreCheckResult,
	highestSiblingID int64,
) {
	binding := res.Cluster.Status.Management

	backup.Status.BackupID = logicalbackup.AllocateBackupIDAfter(metav1.Now(), highestSiblingID)
	backup.Status.ClusterUID = string(res.Cluster.UID)
	backup.Status.Version = binding.Version
	backup.Status.Repository = binding.BackupRepository
	// The endpoint is pinned in the form the client reaches: esadmin.New
	// trims trailing slashes, so a slash never reads as a retarget.
	backup.Status.Storage = &v1.PinnedStorage{
		SecondaryStorageConfig: res.Storage.Name,
		Endpoint:               strings.TrimRight(res.Storage.Spec.Elasticsearch.Endpoint, "/"),
		BucketRef:              res.Bucket.Name,
		BucketLocation:         res.Bucket.Location(),
	}
	backup.Status.Phase = v1.LogicalBackupRunning
	backup.Status.Step = v1.StepPauseExporting
	backup.Status.PartitionsCount = binding.Partitions
	backup.Status.History = v1.BackupPart{State: v1.BackupPartPending}
	backup.Status.Records = v1.BackupPart{State: v1.BackupPartPending}
	backup.Status.Runtime = v1.BackupPart{State: v1.BackupPartPending}

	computed := v1.LogicalBackupStorageSizes{Zeebe: logicalbackup.ZeebeSize(res.Cluster.Status.Volumes)}
	if size, err := r.elasticsearchSize(ctx, res.Storage); err == nil {
		computed.Elasticsearch = size
	}
	logicalbackup.RecordStorageSizes(&backup.Status.StorageSizes, computed)

	conditions.Stage(backup, conditions.Ready(
		metav1.ConditionFalse, v1.ReasonProgressing, "The backup procedure started", backup.Generation,
	))
	r.EventRecorder.Eventf(
		backup,
		nil,
		corev1.EventTypeNormal,
		eventReasonStarted,
		eventActionBackup,
		"Backup %d of CamundaCluster %s/%s started",
		backup.Status.BackupID,
		res.Cluster.Namespace,
		res.Cluster.Name,
	)
}

// run advances a started backup. The cluster is the only dependency that run
// resolves. Without its binding the procedure parks in place, with the same
// phase and the same step. A suspended cluster is not a failure. A cluster
// that is gone, or replaced under its name, ends the backup terminally
// without a management call. Nothing of this backup is left to resume.
func (r *Reconciler) run(
	ctx context.Context,
	backup *v1.LogicalBackupElasticsearch,
) (ctrl.Result, error) {
	// The read is live, so a NotFound is the cluster gone, not a cache
	// that lags. Its exporting state died with it: there is nothing to
	// resume, and no endpoint to resume it at. If the machine walks to
	// ResumeFailed instead, the claim stays with a cluster that does not
	// exist. That claim then blocks every backup of a cluster recreated
	// later under the name. The backup ends here, and the claim goes back
	// on the terminal reconcile.
	var cluster v1.CamundaCluster
	key := clusterKey(backup)
	if err := r.APIReader.Get(ctx, key, &cluster); err != nil {
		if apierrors.IsNotFound(err) {
			r.failClusterLost(backup, fmt.Sprintf(
				"CamundaCluster %s is gone mid-run; the backup ends without a management call", key,
			))
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, fmt.Errorf("reading CamundaCluster %s: %w", key, err)
	}

	// A cluster that was deleted and recreated under the same name is a
	// different cluster. Its exporting was never paused by this backup, so
	// there is nothing to resume. Every management call drives the
	// replacement instead: a resume mid-run of its own backup, or a pairing
	// of old snapshots with its runtime state. The backup ends here,
	// terminally and without a management call. The claim goes back on the
	// terminal reconcile, because nothing of this backup pauses the
	// replacement.
	if clusterReplaced(backup, &cluster) {
		r.failClusterLost(backup, fmt.Sprintf(
			"CamundaCluster %s/%s was replaced mid-run (UID %s, the backup started against %s); "+
				"the backup ends without touching the replacement",
			cluster.Namespace, cluster.Name, cluster.UID, backup.Status.ClusterUID,
		))
		return ctrl.Result{}, nil
	}

	binding := cluster.Status.Management
	if binding == nil || binding.Endpoint == "" {
		conditions.Stage(backup, conditions.Ready(
			metav1.ConditionFalse,
			v1.ReasonProgressing,
			fmt.Sprintf(
				"The procedure is parked at step %s: CamundaCluster %s/%s publishes no management binding (suspended?)",
				backup.Status.Step, cluster.Namespace, cluster.Name,
			),
			backup.Generation,
		))

		return ctrl.Result{RequeueAfter: r.poll()}, nil
	}

	// The sizes are optional metadata. They are backfilled only while
	// exporting still runs: before the pause, and again after the resume.
	// Between the two, a black-holed Elasticsearch must not add its client
	// timeout in front of every poll and every resume attempt.
	if backup.Status.Step == v1.StepPauseExporting {
		r.backfillStorageSizes(ctx, backup, &cluster)
	}

	return r.runStep(ctx, backup, &cluster)
}

// backfillStorageSizes fills the restore sizes that start did not compute. It
// is best effort. A transient blip at start must not leave them empty
// forever. It runs only while exporting runs: it is a metadata call, and it
// must never stand between the cluster and its resume.
func (r *Reconciler) backfillStorageSizes(
	ctx context.Context,
	backup *v1.LogicalBackupElasticsearch,
	cluster *v1.CamundaCluster,
) {
	sizes := &backup.Status.StorageSizes
	if sizes.Zeebe != nil && sizes.Elasticsearch != nil {
		return
	}

	computed := v1.LogicalBackupStorageSizes{Zeebe: logicalbackup.ZeebeSize(cluster.Status.Volumes)}
	if sizes.Elasticsearch == nil {
		if storage, err := r.resolveStorage(ctx, cluster); err == nil {
			if size, err := r.elasticsearchSize(ctx, storage); err == nil {
				computed.Elasticsearch = size
			}
		}
	}

	logicalbackup.RecordStorageSizes(sizes, computed)
}

// elasticsearchSize computes the effective Elasticsearch restore size from
// the node filesystem statistics.
func (r *Reconciler) elasticsearchSize(
	ctx context.Context,
	storage *v1.SecondaryStorageConfig,
) (*resource.Quantity, error) {
	es, failure, err := secondarystorageconfig.ElasticsearchAdmin(ctx, r.APIReader, storage)
	if err != nil {
		return nil, err
	}
	if failure != nil {
		return nil, errors.New(failure.Message)
	}

	total, used, err := es.MaxNodeFSTotalAndUsedBytes(ctx)
	if err != nil {
		return nil, err
	}

	return logicalbackup.ElasticsearchSize(total, used), nil
}

// resolveStorage reads the storage contract of the cluster.
// errNoStorage marks a cluster that names no storage contract.
var errNoStorage = errors.New("no storage contract")

// storageMissing reports whether a resolveStorage error means that the
// storage contract is gone: the cluster names none, or the named one does
// not exist. Every other error is a transient read.
func storageMissing(err error) bool {
	return errors.Is(err, errNoStorage) || apierrors.IsNotFound(err)
}

func (r *Reconciler) resolveStorage(
	ctx context.Context,
	cluster *v1.CamundaCluster,
) (*v1.SecondaryStorageConfig, error) {
	if cluster.Spec.StorageRef == "" {
		return nil, fmt.Errorf(
			"%w: CamundaCluster %s/%s no longer names a storage contract",
			errNoStorage,
			cluster.Namespace,
			cluster.Name,
		)
	}

	var storage v1.SecondaryStorageConfig
	key := types.NamespacedName{Namespace: cluster.Namespace, Name: cluster.Spec.StorageRef}
	if err := r.APIReader.Get(ctx, key, &storage); err != nil {
		return nil, fmt.Errorf("reading SecondaryStorageConfig %s: %w", key, err)
	}

	return &storage, nil
}

// errBucketMoved marks a cluster whose backup store is no longer the bucket
// the backup pinned, at the location it pinned.
var errBucketMoved = errors.New("the backup store moved")

// pinnedBucketCurrent verifies that the cluster still backs up through the
// ObjectStorageConfig that the backup pinned at its start. It also verifies
// that the contract still points at the pinned location. The snapshot
// repository and the runtime backup land in that bucket. A store that moved
// mid-run splits the set, and a delete aimed through the new store misses
// the artifacts. It returns nil when the pin holds. It returns an error
// that wraps errBucketMoved, naming the change, when the cluster names
// another contract or the contract points elsewhere. It returns a NotFound
// when the pinned contract is gone. Any other error is a transient read. A
// backup that has not started has no pin and passes.
func (r *Reconciler) pinnedBucketCurrent(
	ctx context.Context,
	backup *v1.LogicalBackupElasticsearch,
	cluster *v1.CamundaCluster,
) error {
	pinned := backup.Status.Storage
	if pinned == nil {
		return nil
	}
	if cluster.Spec.BackupStorageRef != pinned.BucketRef {
		return fmt.Errorf(
			"%w: CamundaCluster %s/%s now backs up through ObjectStorageConfig %q, but the backup "+
				"pinned %q; the set must land in one bucket",
			errBucketMoved, cluster.Namespace, cluster.Name, cluster.Spec.BackupStorageRef, pinned.BucketRef,
		)
	}

	return r.pinnedBucketLocationCurrent(ctx, backup.Namespace, pinned)
}

// pinnedBucketLocationCurrent verifies that the pinned ObjectStorageConfig
// still points at the pinned location. It is the part of pinnedBucketCurrent
// that needs no cluster, so the sweep of a gone cluster verifies it too. The
// contract is read from namespace, the namespace of the backup. It returns an
// error that wraps errBucketMoved when the contract points elsewhere. It
// returns a NotFound when the contract is gone. Any other read error comes
// back as it is.
func (r *Reconciler) pinnedBucketLocationCurrent(
	ctx context.Context,
	namespace string,
	pinned *v1.PinnedStorage,
) error {
	key := types.NamespacedName{Namespace: namespace, Name: pinned.BucketRef}

	var bucket v1.ObjectStorageConfig
	if err := r.APIReader.Get(ctx, key, &bucket); err != nil {
		return fmt.Errorf("reading the pinned ObjectStorageConfig %s: %w", key, err)
	}
	if location := bucket.Location(); location != pinned.BucketLocation {
		return fmt.Errorf(
			"%w: ObjectStorageConfig %q now points at %s, but the backup pinned %s; the set must land "+
				"in one bucket",
			errBucketMoved, bucket.Name, location, pinned.BucketLocation,
		)
	}

	return nil
}

// clusterKey returns the key of the referenced cluster.
func clusterKey(backup *v1.LogicalBackupElasticsearch) types.NamespacedName {
	return types.NamespacedName{Namespace: backup.Namespace, Name: backup.Spec.ClusterRef.Name}
}
