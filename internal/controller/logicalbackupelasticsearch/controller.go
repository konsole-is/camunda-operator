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

// Package logicalbackupelasticsearch reconciles LogicalBackupElasticsearch:
// one hot backup of an Elasticsearch-backed CamundaCluster, taken as a
// coordinated set under one backup ID and tracked to a terminal phase. The
// controller talks to the cluster through the management binding it
// publishes, and to Elasticsearch through the SecondaryStorageConfig — never
// through the internals of another controller.
//
// Admission and runtime are split. The full pre-checks run only before the
// backup starts; once it runs, a missing dependency routes through the state
// machine to resume-exporting and a terminal phase, and a cluster that is
// momentarily unaddressable parks the procedure in place. A running backup
// never regresses to Pending and never re-starts: its identity — the backup
// id, the pinned repository — is written before the first side effect and
// only read afterwards.
package logicalbackupelasticsearch

import (
	"context"
	"errors"
	"fmt"
	"time"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/events"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	"github.com/sourcehawk/operator-component-framework/pkg/component"

	v1 "github.com/konsole-is/camunda-operator/api/v1"
	"github.com/konsole-is/camunda-operator/pkg/conditions"
	"github.com/konsole-is/camunda-operator/pkg/esadmin"
	"github.com/konsole-is/camunda-operator/pkg/logicalbackup"
	"github.com/konsole-is/camunda-operator/pkg/refindex"
	"github.com/konsole-is/camunda-operator/pkg/secretref"
)

const (
	// clusterRefField indexes backups by the effective namespace/name of
	// their clusterRef, so a cluster event enqueues its backups.
	clusterRefField = "logicalbackupelasticsearch.spec.clusterRef"

	// defaultResumeDeadline bounds the accumulated time of active resume
	// attempts before the controller gives the cluster to a human.
	defaultResumeDeadline = 30 * time.Minute
	// defaultPollInterval paces the polling of the running procedure and the
	// waits on pre-checks that resolve on their own.
	defaultPollInterval = 5 * time.Second
	// retryInterval paces admission failures that no watch resolves.
	retryInterval = 30 * time.Second
	// concurrentReconciles bounds the parallel reconciles. Every step is a
	// synchronous HTTP call, so one black-holed endpoint must not head-of-line
	// block the polling and the finalizers of every other backup.
	concurrentReconciles = 4
)

const (
	eventReasonStarted      = "BackupStarted"
	eventReasonCompleted    = "BackupCompleted"
	eventReasonStepFailed   = "BackupStepFailed"
	eventReasonResumeFailed = "ResumeFailed"
	eventReasonReleased     = "ArtifactsUnreachable"
	eventReasonDeleteHeld   = "DeletionWaiting"
	eventActionBackup       = "Backup"
	eventActionFinalize     = "Finalize"
)

// Reconciler drives a LogicalBackupElasticsearch to a terminal phase.
type Reconciler struct {
	client.Client
	// APIReader reads referenced resources, and the backup itself, without
	// the cache: a stale status would re-run a side effect, and a stale
	// suspend flag or storage reference would admit a backup that must wait.
	APIReader client.Reader
	Scheme    *runtime.Scheme
	// EventRecorder publishes the lifecycle events of the backup.
	// SetupWithManager sets it from the manager when it is nil.
	EventRecorder events.EventRecorder

	// ResumeDeadline bounds the accumulated time of active resume attempts
	// before the phase goes Failed with reason ResumeFailed. Zero means the
	// default of 30 minutes.
	ResumeDeadline time.Duration
	// PollInterval paces the polling of a running backup. Zero means the
	// default of five seconds.
	PollInterval time.Duration
	// SiblingInProgress reports a non-terminal backup of the same cluster
	// held by the other backup kind, so backups of one cluster run one at a
	// time across kinds. Nil means no other kind is checked; the manager
	// wires it once both kinds are registered.
	SiblingInProgress logicalbackup.SiblingInProgress
}

// +kubebuilder:rbac:groups=core.camunda.io,resources=logicalbackupelasticsearches,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=core.camunda.io,resources=logicalbackupelasticsearches/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=core.camunda.io,resources=logicalbackupelasticsearches/finalizers,verbs=update
// +kubebuilder:rbac:groups=core.camunda.io,resources=camundaclusters,verbs=get;list;watch
// +kubebuilder:rbac:groups=core.camunda.io,resources=secondarystorageconfigs,verbs=get;list;watch
// +kubebuilder:rbac:groups=core.camunda.io,resources=objectstorageconfigs,verbs=get;list;watch
// +kubebuilder:rbac:groups="",resources=secrets,verbs=get
// +kubebuilder:rbac:groups=events.k8s.io,resources=events,verbs=create;patch

// Reconcile advances the backup by at most one step, so the recorded step is
// always persisted before the next side effect and a crash re-enters where it
// left off.
func (r *Reconciler) Reconcile(ctx context.Context, req ctrl.Request) (_ ctrl.Result, err error) {
	// The backup is its own state machine: a stale cached read of its status
	// would re-enter a step whose side effect already ran, or re-allocate the
	// backup id and orphan artifacts. Its own object is therefore always read
	// live.
	var backup v1.LogicalBackupElasticsearch
	if err := r.APIReader.Get(ctx, req.NamespacedName, &backup); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	if !backup.DeletionTimestamp.IsZero() {
		return r.finalize(ctx, &backup)
	}

	// The finalizer must exist before the first external side effect, or a
	// deletion between the side effect and the next write would leak the
	// artifacts.
	if controllerutil.AddFinalizer(&backup, logicalbackup.Finalizer) {
		if err := r.Update(ctx, &backup); err != nil {
			// A deletion racing this write is fine: the deletion path owns
			// the object from here.
			if apierrors.IsNotFound(err) {
				return ctrl.Result{}, nil
			}
			return ctrl.Result{}, fmt.Errorf("adding the finalizer: %w", err)
		}
	}

	rec := component.ReconcileContext{
		Client:        r.Client,
		Scheme:        r.Scheme,
		EventRecorder: r.EventRecorder,
		APIReader:     r.APIReader,
		Owner:         &backup,
	}
	defer func() {
		if flushErr := component.FlushStatus(ctx, rec, nil); flushErr != nil {
			err = errors.Join(err, flushErr)
		}
	}()

	// A write conflict at the terminal transition can restore an older Ready
	// from the server, so the terminal condition is re-staged from the
	// recorded outcome; SetStatusCondition makes the repeat a no-op.
	if backup.Terminal() {
		conditions.Stage(&backup, r.terminalReady(&backup))
		return ctrl.Result{}, nil
	}

	if backup.Status.BackupID == 0 {
		return r.admit(ctx, &backup)
	}

	return r.run(ctx, &backup)
}

// admit runs the full pre-checks and starts the backup. Admission ends when
// the backup id is allocated: from then on the backup never returns here, so
// a dependency that breaks mid-run is the state machine's to handle, not a
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

		// The cluster watch resolves a reference that appears later; the
		// timer covers the contracts nothing here watches.
		return ctrl.Result{RequeueAfter: retryInterval}, nil
	}

	// The management client is built once here so a broken binding — no
	// binding yet, basic auth without a Secret, an unsupported version —
	// blocks admission with its own reason instead of failing the first step.
	if _, failure, err := logicalbackup.ManagementClient(ctx, r.APIReader, res.Cluster); err != nil {
		return ctrl.Result{}, err
	} else if failure != nil {
		backup.Status.Phase = v1.LogicalBackupPending
		conditions.Stage(backup, conditions.Failed(backup, failure))
		if failure.Reason == v1.ReasonProgressing {
			return ctrl.Result{RequeueAfter: r.poll()}, nil
		}
		return ctrl.Result{RequeueAfter: retryInterval}, nil
	}

	binding := res.Cluster.Status.Management
	if binding.BackupRepository == "" {
		backup.Status.Phase = v1.LogicalBackupPending
		conditions.Stage(backup, conditions.Failed(backup, &conditions.PreCheckFailure{
			Reason: v1.ReasonInvalidReference,
			Message: fmt.Sprintf(
				"CamundaCluster %s/%s publishes no backup repository; its storage contract carries no elasticsearch.snapshotRepository",
				res.Cluster.Namespace,
				res.Cluster.Name,
			),
		}))

		return ctrl.Result{RequeueAfter: retryInterval}, nil
	}

	r.start(ctx, backup, res, binding)

	return ctrl.Result{RequeueAfter: r.poll()}, nil
}

// run advances a started backup. The cluster is the only dependency resolved
// here: without its binding the procedure parks in place — same phase, same
// step — because a suspended or momentarily unaddressed cluster is not a
// failure. A cluster that is gone for good routes through the machine, so the
// resume deadline still bounds the end.
func (r *Reconciler) run(
	ctx context.Context,
	backup *v1.LogicalBackupElasticsearch,
) (ctrl.Result, error) {
	var cluster v1.CamundaCluster
	key := types.NamespacedName{
		Namespace: backup.EffectiveClusterNamespace(),
		Name:      backup.Spec.ClusterRef.Name,
	}
	if err := r.APIReader.Get(ctx, key, &cluster); err != nil {
		if apierrors.IsNotFound(err) {
			// Nothing is addressable anymore; the machine walks to its
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

// start records everything the procedure is keyed by — the backup id, the
// pinned repository, the partition count, the restore sizes — before the
// first management call, so a crash never loses the identity of work already
// started.
func (r *Reconciler) start(
	ctx context.Context,
	backup *v1.LogicalBackupElasticsearch,
	res *logicalbackup.PreCheckResult,
	binding *v1.ManagementBinding,
) {
	backup.Status.BackupID = logicalbackup.AllocateBackupID(metav1.Now())
	backup.Status.Repository = binding.BackupRepository
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

// backfillStorageSizes fills the restore sizes that start could not compute,
// best effort: a transient blip at start must not leave them empty forever.
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
	es, err := r.elasticsearchAdmin(ctx, storage)
	if err != nil {
		return nil, err
	}

	total, used, err := es.MaxNodeFSTotalAndUsedBytes(ctx)
	if err != nil {
		return nil, err
	}

	return logicalbackup.ElasticsearchSize(total, used), nil
}

// resolveStorage reads the storage contract of the cluster.
func (r *Reconciler) resolveStorage(
	ctx context.Context,
	cluster *v1.CamundaCluster,
) (*v1.SecondaryStorageConfig, error) {
	if cluster.Spec.StorageRef == "" {
		return nil, fmt.Errorf(
			"CamundaCluster %s/%s no longer names a storage contract",
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

// inProgress reports another non-terminal backup of the same cluster, of this
// kind or, once the manager wires it, of the sibling kind.
func (r *Reconciler) inProgress(backup *v1.LogicalBackupElasticsearch) logicalbackup.InProgress {
	return func(ctx context.Context) (string, error) {
		cluster := types.NamespacedName{
			Namespace: backup.EffectiveClusterNamespace(),
			Name:      backup.Spec.ClusterRef.Name,
		}

		var list v1.LogicalBackupElasticsearchList
		if err := r.APIReader.List(ctx, &list); err != nil {
			return "", fmt.Errorf("listing LogicalBackupElasticsearch: %w", err)
		}

		for i := range list.Items {
			other := &list.Items[i]
			if other.UID == backup.UID || other.Terminal() {
				continue
			}
			if other.Spec.ClusterRef.Name != cluster.Name ||
				other.EffectiveClusterNamespace() != cluster.Namespace {
				continue
			}
			// A sibling that holds an id has begun work worth waiting for.
			// Between two unstarted backups the older one (by creation time,
			// then name) goes first — a deterministic order, so two waiting
			// backups can never deadlock on each other.
			if other.Status.BackupID == 0 && !starts(other, backup) {
				continue
			}

			return other.Name, nil
		}

		if r.SiblingInProgress == nil {
			return "", nil
		}

		return r.SiblingInProgress(ctx, cluster)
	}
}

// partOf returns the backup part that the step drives, or nil for the steps
// that own no part.
func partOf(backup *v1.LogicalBackupElasticsearch, step v1.LogicalBackupElasticsearchStep) *v1.BackupPart {
	switch step {
	case v1.StepBackupHistory:
		return &backup.Status.History
	case v1.StepSnapshotRecords:
		return &backup.Status.Records
	case v1.StepBackupRuntime:
		return &backup.Status.Runtime
	default:
		return nil
	}
}

// starts reports whether a goes before b when neither has started: the older
// creation time wins, the lexically smaller name breaks a tie.
func starts(a, b *v1.LogicalBackupElasticsearch) bool {
	if !a.CreationTimestamp.Equal(&b.CreationTimestamp) {
		return a.CreationTimestamp.Before(&b.CreationTimestamp)
	}
	return a.Name < b.Name
}

// terminalReady rebuilds the Ready condition of a terminal backup from the
// recorded outcome.
func (r *Reconciler) terminalReady(backup *v1.LogicalBackupElasticsearch) metav1.Condition {
	if backup.Status.Phase == v1.LogicalBackupCompleted {
		return conditions.Ready(
			metav1.ConditionTrue,
			v1.ReasonCompleted,
			"The backup finished and is restorable",
			backup.Generation,
		)
	}

	reason := backup.Status.TerminalReason
	if reason == "" {
		reason = v1.ReasonFailed
	}

	return conditions.Ready(metav1.ConditionFalse, reason, backup.Status.FailureMessage, backup.Generation)
}

// elasticsearchAdmin builds the Elasticsearch client from the published
// storage contract: endpoint, the camunda user, and the CA when the contract
// names one.
func (r *Reconciler) elasticsearchAdmin(
	ctx context.Context,
	storage *v1.SecondaryStorageConfig,
) (*esadmin.Client, error) {
	es := storage.Spec.Elasticsearch
	if es == nil {
		return nil, fmt.Errorf(
			"SecondaryStorageConfig %s/%s has no elasticsearch block",
			storage.Namespace,
			storage.Name,
		)
	}

	creds := es.CredentialsSecretRef
	secret, msg, err := secretref.Get(
		ctx,
		r.APIReader,
		types.NamespacedName{Namespace: creds.Namespace, Name: creds.Name},
		creds.UsernameKey,
		creds.PasswordKey,
	)
	if err != nil {
		return nil, fmt.Errorf("reading the Elasticsearch credentials: %w", err)
	}
	if msg != "" {
		return nil, &conditions.PreCheckFailure{Reason: v1.ReasonMissingSecret, Message: msg}
	}

	var ca []byte
	if es.CASecretRef != nil {
		caSecret, msg, err := secretref.Get(
			ctx,
			r.APIReader,
			types.NamespacedName{Namespace: es.CASecretRef.Namespace, Name: es.CASecretRef.Name},
			es.CASecretRef.Key,
		)
		if err != nil {
			return nil, fmt.Errorf("reading the Elasticsearch CA: %w", err)
		}
		if msg != "" {
			return nil, &conditions.PreCheckFailure{Reason: v1.ReasonMissingSecret, Message: msg}
		}
		ca = caSecret.Data[es.CASecretRef.Key]
	}

	admin, err := esadmin.New(
		es.Endpoint,
		string(secret.Data[creds.UsernameKey]),
		string(secret.Data[creds.PasswordKey]),
		ca,
	)
	if err != nil {
		return nil, fmt.Errorf("building the Elasticsearch client: %w", err)
	}

	return admin, nil
}

func (r *Reconciler) poll() time.Duration {
	if r.PollInterval > 0 {
		return r.PollInterval
	}
	return defaultPollInterval
}

func (r *Reconciler) resumeDeadline() time.Duration {
	if r.ResumeDeadline > 0 {
		return r.ResumeDeadline
	}
	return defaultResumeDeadline
}

// SetupWithManager registers the controller, the clusterRef index, and the
// cluster watch that wakes waiting backups when the binding appears. It sets
// EventRecorder from the manager when it is nil.
func (r *Reconciler) SetupWithManager(mgr ctrl.Manager) error {
	if r.EventRecorder == nil {
		r.EventRecorder = mgr.GetEventRecorder("logicalbackupelasticsearch")
	}

	if err := mgr.GetFieldIndexer().IndexField(
		context.Background(),
		&v1.LogicalBackupElasticsearch{},
		clusterRefField,
		func(o client.Object) []string {
			backup := o.(*v1.LogicalBackupElasticsearch)
			return []string{refindex.NamespacedKey(backup.EffectiveClusterNamespace(), backup.Spec.ClusterRef.Name)}
		},
	); err != nil {
		return fmt.Errorf("indexing LogicalBackupElasticsearch by clusterRef: %w", err)
	}

	return ctrl.NewControllerManagedBy(mgr).
		For(&v1.LogicalBackupElasticsearch{}).
		Named("logicalbackupelasticsearch").
		WithOptions(controller.Options{MaxConcurrentReconciles: concurrentReconciles}).
		Watches(
			&v1.CamundaCluster{},
			refindex.Enqueue(
				mgr.GetClient(),
				&v1.LogicalBackupElasticsearchList{},
				clusterRefField,
				refindex.ObjectNamespacedName,
			),
		).
		Complete(r)
}
