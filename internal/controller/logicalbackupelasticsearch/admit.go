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

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"

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
func (r *Reconciler) admit(
	ctx context.Context,
	backup *v1.LogicalBackupElasticsearch,
) (ctrl.Result, error) {
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

	r.start(ctx, backup, res)

	return ctrl.Result{RequeueAfter: r.poll()}, nil
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
) {
	binding := res.Cluster.Status.Management

	backup.Status.BackupID = logicalbackup.AllocateBackupID(metav1.Now())
	backup.Status.Repository = binding.BackupRepository
	backup.Status.Storage = &v1.PinnedStorage{
		SecondaryStorageConfig: res.Storage.Name,
		Endpoint:               res.Storage.Spec.Elasticsearch.Endpoint,
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
// that is gone for good routes through the machine, so the resume deadline
// still bounds the end.
func (r *Reconciler) run(
	ctx context.Context,
	backup *v1.LogicalBackupElasticsearch,
) (ctrl.Result, error) {
	var cluster v1.CamundaCluster
	key := clusterKey(backup)
	if err := r.APIReader.Get(ctx, key, &cluster); err != nil {
		if apierrors.IsNotFound(err) {
			// Nothing is addressable anymore. The machine walks to its
			// terminal phase through the resume deadline.
			if backup.Status.Step != v1.StepResumeExporting {
				r.failStep(
					backup,
					string(backup.Status.Step),
					partOf(backup, backup.Status.Step),
					fmt.Errorf("CamundaCluster %s is gone", key),
				)
			}
			return r.runStep(ctx, backup, &cluster)
		}
		return ctrl.Result{}, fmt.Errorf("reading CamundaCluster %s: %w", key, err)
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

	r.backfillStorageSizes(ctx, backup, &cluster)

	return r.runStep(ctx, backup, &cluster)
}

// backfillStorageSizes fills the restore sizes that start did not compute. It
// is best effort. A transient blip at start must not leave them empty
// forever.
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

// clusterKey returns the key of the referenced cluster.
func clusterKey(backup *v1.LogicalBackupElasticsearch) types.NamespacedName {
	return types.NamespacedName{Namespace: backup.Namespace, Name: backup.Spec.ClusterRef.Name}
}
