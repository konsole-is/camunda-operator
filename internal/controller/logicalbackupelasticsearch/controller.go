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
package logicalbackupelasticsearch

import (
	"context"
	"errors"
	"fmt"
	"time"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/events"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	"github.com/sourcehawk/operator-component-framework/pkg/component"

	v1 "github.com/konsole-is/camunda-operator/api/v1"
	"github.com/konsole-is/camunda-operator/pkg/camundaadmin"
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

	// defaultResumeDeadline bounds how long the controller retries resuming
	// exporting before it gives the cluster to a human.
	defaultResumeDeadline = 30 * time.Minute
	// defaultPollInterval paces the polling of the running procedure and the
	// waits on pre-checks that resolve on their own.
	defaultPollInterval = 5 * time.Second
	// retryInterval paces pre-check failures that no watch resolves.
	retryInterval = 30 * time.Second
)

const (
	eventReasonStarted      = "BackupStarted"
	eventReasonCompleted    = "BackupCompleted"
	eventReasonStepFailed   = "BackupStepFailed"
	eventReasonResumeFailed = "ResumeFailed"
	eventReasonReleased     = "ArtifactsUnreachable"
	eventActionBackup       = "Backup"
	eventActionFinalize     = "Finalize"
)

// Reconciler drives a LogicalBackupElasticsearch to a terminal phase.
type Reconciler struct {
	client.Client
	// APIReader reads referenced resources without the cache: a pre-check
	// that decides a backup may start must not act on a stale suspend flag
	// or storage reference.
	APIReader client.Reader
	Scheme    *runtime.Scheme
	// EventRecorder publishes the lifecycle events of the backup.
	// SetupWithManager sets it from the manager when it is nil.
	EventRecorder events.EventRecorder

	// ResumeDeadline bounds the retries of resume-exporting before the phase
	// goes Failed with reason ResumeFailed. Zero means the default of 30
	// minutes.
	ResumeDeadline time.Duration
	// PollInterval paces the polling of a running backup. Zero means the
	// default of five seconds.
	PollInterval time.Duration
	// SiblingInProgress reports a non-terminal backup of the same cluster
	// held by the other backup kind, so backups of one cluster run one at a
	// time across kinds. Nil means no other kind is checked; the manager
	// wires it once both kinds are registered.
	SiblingInProgress func(ctx context.Context, clusterNamespace, clusterName string) (string, error)
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

	if backup.Terminal() {
		return ctrl.Result{}, nil
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

	res, err := logicalbackup.PreCheck(ctx, logicalbackup.PreCheckRequest{
		Reader:      r.APIReader,
		Ref:         backup.Spec.ClusterRef,
		Namespace:   backup.Namespace,
		StorageType: v1.SecondaryStorageTypeElasticsearch,
		InProgress:  r.inProgress(&backup),
	})
	if err != nil {
		var failure *conditions.PreCheckFailure
		if !errors.As(err, &failure) {
			return ctrl.Result{}, err
		}

		if backup.Status.Phase == "" {
			backup.Status.Phase = v1.LogicalBackupPending
		}
		conditions.Stage(&backup, conditions.Failed(&backup, failure))
		if logicalbackup.Waiting(err) {
			return ctrl.Result{RequeueAfter: r.poll()}, nil
		}

		// The cluster watch resolves a reference that appears later; the
		// timer covers the contracts nothing here watches.
		return ctrl.Result{RequeueAfter: retryInterval}, nil
	}

	binding := res.Cluster.Status.Management
	if binding == nil || binding.Endpoint == "" {
		backup.Status.Phase = v1.LogicalBackupPending
		conditions.Stage(&backup, conditions.Ready(
			metav1.ConditionFalse,
			v1.ReasonProgressing,
			fmt.Sprintf(
				"CamundaCluster %s/%s has not published its management binding yet",
				res.Cluster.Namespace, res.Cluster.Name,
			),
			backup.Generation,
		))

		return ctrl.Result{RequeueAfter: r.poll()}, nil
	}

	if binding.BackupRepository == "" {
		backup.Status.Phase = v1.LogicalBackupPending
		conditions.Stage(&backup, conditions.Failed(&backup, &conditions.PreCheckFailure{
			Reason: v1.ReasonInvalidReference,
			Message: fmt.Sprintf(
				"CamundaCluster %s/%s publishes no backup repository; its storage contract carries no elasticsearch.snapshotRepository",
				res.Cluster.Namespace,
				res.Cluster.Name,
			),
		}))

		return ctrl.Result{RequeueAfter: retryInterval}, nil
	}

	if backup.Status.Phase == "" || backup.Status.Phase == v1.LogicalBackupPending {
		r.start(ctx, &backup, res, binding)
		return ctrl.Result{RequeueAfter: r.poll()}, nil
	}

	return r.runStep(ctx, &backup, res, binding)
}

// start records everything the procedure is keyed by — the backup ID, the
// partition count, the restore sizes — before the first management call, so a
// crash never loses the identity of work already started.
func (r *Reconciler) start(
	ctx context.Context,
	backup *v1.LogicalBackupElasticsearch,
	res *logicalbackup.PreCheckResult,
	binding *v1.ManagementBinding,
) {
	if backup.Status.BackupID == 0 {
		backup.Status.BackupID = logicalbackup.AllocateBackupID(metav1.Now())
	}

	backup.Status.Phase = v1.LogicalBackupRunning
	backup.Status.Step = v1.StepPauseExporting
	backup.Status.PartitionsCount = binding.Partitions
	backup.Status.History = v1.BackupPart{State: v1.BackupPartPending}
	backup.Status.Records = v1.BackupPart{State: v1.BackupPartPending}
	backup.Status.Runtime = v1.BackupPart{State: v1.BackupPartPending}

	r.recordStorageSizes(ctx, backup, res)
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

// recordStorageSizes fills the unset restore sizes, best effort: a size that
// cannot be computed stays unset and never fails the backup.
func (r *Reconciler) recordStorageSizes(
	ctx context.Context,
	backup *v1.LogicalBackupElasticsearch,
	res *logicalbackup.PreCheckResult,
) {
	computed := v1.LogicalBackupStorageSizes{Zeebe: logicalbackup.ZeebeSize(res.Cluster.Status.Volumes)}

	if es, err := r.elasticsearchAdmin(ctx, res.Storage); err == nil {
		if total, used, err := es.MaxNodeFSTotalAndUsedBytes(ctx); err == nil {
			computed.Elasticsearch = logicalbackup.ElasticsearchSize(total, used)
		}
	}

	logicalbackup.RecordStorageSizes(&backup.Status.StorageSizes, computed)
}

// inProgress reports another non-terminal backup of the same cluster, of this
// kind or, once the manager wires it, of the sibling kind.
func (r *Reconciler) inProgress(backup *v1.LogicalBackupElasticsearch) logicalbackup.InProgress {
	return func(ctx context.Context) (string, error) {
		clusterNamespace := backup.EffectiveClusterNamespace()

		var list v1.LogicalBackupElasticsearchList
		if err := r.APIReader.List(ctx, &list); err != nil {
			return "", fmt.Errorf("listing LogicalBackupElasticsearch: %w", err)
		}

		for i := range list.Items {
			other := &list.Items[i]
			if other.UID == backup.UID || other.Terminal() {
				continue
			}
			if other.Spec.ClusterRef.Name != backup.Spec.ClusterRef.Name ||
				other.EffectiveClusterNamespace() != clusterNamespace {
				continue
			}
			// Two waiting backups must not block each other, or neither ever
			// starts. Only one that holds an allocated id has begun.
			if other.Status.BackupID == 0 {
				continue
			}

			return other.Name, nil
		}

		if r.SiblingInProgress == nil {
			return "", nil
		}

		return r.SiblingInProgress(ctx, clusterNamespace, backup.Spec.ClusterRef.Name)
	}
}

// managementClient builds the client of the cluster's management API from the
// published binding.
func (r *Reconciler) managementClient(
	ctx context.Context,
	binding *v1.ManagementBinding,
) (*camundaadmin.Client, error) {
	auth := camundaadmin.Auth{}
	if binding.Auth.Method == v1.ManagementAuthMethodBasic && binding.Auth.CredentialsSecretRef != nil {
		ref := binding.Auth.CredentialsSecretRef
		secret, msg, err := secretref.Get(
			ctx,
			r.APIReader,
			types.NamespacedName{Namespace: ref.Namespace, Name: ref.Name},
			ref.UsernameKey,
			ref.PasswordKey,
		)
		if err != nil {
			return nil, fmt.Errorf("reading the management credentials: %w", err)
		}
		if msg != "" {
			return nil, &conditions.PreCheckFailure{Reason: v1.ReasonMissingSecret, Message: msg}
		}

		auth.Username = string(secret.Data[ref.UsernameKey])
		auth.Password = string(secret.Data[ref.PasswordKey])
	}

	return camundaadmin.New(camundaadmin.Binding{
		Endpoint: binding.Endpoint,
		Version:  binding.Version,
		Auth:     auth,
	})
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
