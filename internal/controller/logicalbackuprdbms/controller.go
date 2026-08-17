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

// Package logicalbackuprdbms reconciles a LogicalBackupRDBMS: it runs the Job
// that dumps the logical database of a relational cluster to the backup
// bucket, then requests one cluster-generated primary-storage backup, so the
// pair is a complete restore point.
package logicalbackuprdbms

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/sourcehawk/operator-component-framework/pkg/component"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/events"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/handler"

	v1 "github.com/konsole-is/camunda-operator/api/v1"
	"github.com/konsole-is/camunda-operator/pkg/camundaadmin"
	camundacluster "github.com/konsole-is/camunda-operator/pkg/components/camundacluster"
	components "github.com/konsole-is/camunda-operator/pkg/components/logicalbackuprdbms"
	"github.com/konsole-is/camunda-operator/pkg/conditions"
	"github.com/konsole-is/camunda-operator/pkg/labels"
	"github.com/konsole-is/camunda-operator/pkg/logicalbackup"
	"github.com/konsole-is/camunda-operator/pkg/objectstore"
	"github.com/konsole-is/camunda-operator/pkg/secretref"
)

const (
	// fieldManager owns the Job of a backup in Server-Side Apply.
	fieldManager = "camunda-operator/backup"
	// clusterKeyIndex indexes backups by the namespace/name of the cluster
	// they reference, for the serialization pre-check and the cluster watch.
	clusterKeyIndex = "spec.clusterKey"
	// defaultRetryInterval paces the requeues of waiting states: an
	// unreachable management API, a running sibling backup, or a reference
	// that nothing watches.
	defaultRetryInterval = 30 * time.Second

	eventReasonStarted   = "BackupStarted"
	eventReasonCompleted = "BackupCompleted"
	eventReasonFailed    = "BackupFailed"
	eventReasonCleanup   = "ArtifactCleanupFailed"
	eventActionBackup    = "Backup"
	eventActionFinalize  = "Finalize"
)

// ArtifactBucket is the slice of the backup bucket that the finalizer needs.
type ArtifactBucket interface {
	Delete(ctx context.Context, key string) error
	Close()
}

// LogicalBackupRDBMSReconciler reconciles a LogicalBackupRDBMS.
type LogicalBackupRDBMSReconciler struct {
	client.Client
	// APIReader reads without the cache. A pre-check that decides a backup
	// may start must not act on a stale suspend flag or reference.
	APIReader client.Reader
	Scheme    *runtime.Scheme
	// EventRecorder publishes the backup lifecycle events. SetupWithManager
	// sets it from the manager.
	EventRecorder events.EventRecorder
	// OperatorImage runs the upload container of the dump Job. The manager
	// deployment passes its own image through --operator-image.
	OperatorImage string
	// OpenBucket opens the backup bucket for the finalizer. Nil means
	// pkg/objectstore; tests point it at a local fake.
	OpenBucket func(
		ctx context.Context, cfg *v1.ObjectStorageConfig, creds *objectstore.Credentials,
	) (ArtifactBucket, error)
	// SiblingInProgress reports a non-terminal backup of another kind for
	// the same cluster. The LogicalBackupElasticsearch kind plugs in here
	// once both exist in one manager; nil means no other kind is checked.
	SiblingInProgress func(ctx context.Context, cluster types.NamespacedName) (string, error)
	// RetryInterval overrides defaultRetryInterval. Zero means the default.
	RetryInterval time.Duration
}

// +kubebuilder:rbac:groups=core.camunda.io,resources=logicalbackuprdbmses,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=core.camunda.io,resources=logicalbackuprdbmses/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=core.camunda.io,resources=logicalbackuprdbmses/finalizers,verbs=update
// +kubebuilder:rbac:groups=core.camunda.io,resources=camundaclusters;secondarystorageconfigs;databaseconfigs;databaseserverconfigs;objectstorageconfigs;camundaclusterpresets,verbs=get;list;watch
// +kubebuilder:rbac:groups=batch,resources=jobs,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="",resources=secrets,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=events.k8s.io,resources=events,verbs=create;patch

// Reconcile drives one backup to a terminal phase: pre-checks, the dump Job,
// then the primary-storage backup of the cluster.
func (r *LogicalBackupRDBMSReconciler) Reconcile(
	ctx context.Context,
	req ctrl.Request,
) (_ ctrl.Result, err error) {
	var backup v1.LogicalBackupRDBMS
	if err := r.Get(ctx, req.NamespacedName, &backup); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	if !backup.DeletionTimestamp.IsZero() {
		return ctrl.Result{}, r.finalize(ctx, &backup)
	}

	if !controllerutil.ContainsFinalizer(&backup, logicalbackup.Finalizer) {
		controllerutil.AddFinalizer(&backup, logicalbackup.Finalizer)
		if err := r.Update(ctx, &backup); err != nil {
			return ctrl.Result{}, fmt.Errorf("adding the finalizer: %w", err)
		}
	}

	if backup.Terminal() {
		return ctrl.Result{}, nil
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

	resolved, failure, err := r.resolve(ctx, &backup)
	if err != nil {
		return ctrl.Result{}, err
	}
	if failure != nil {
		conditions.Stage(&backup, conditions.Failed(&backup, failure))

		return ctrl.Result{RequeueAfter: r.retryInterval()}, nil
	}

	return r.advance(ctx, &backup, resolved)
}

// resolved is everything one reconcile of a running backup works with.
type resolved struct {
	precheck *logicalbackup.PreCheckResult
	dump     *v1.BackupDumpSpec
	// serviceAccount is the account of the dump pod, honoring the cluster's
	// serviceAccount.name override.
	serviceAccount string
	// bucketSecret is the local copy of the bucket's static credentials in
	// the cluster namespace; empty for workload identity.
	bucketSecret string
	server       *v1.DatabaseServerConfig
	dbConfig     *v1.DatabaseConfig
}

// resolve runs the shared pre-checks and the RDBMS-specific ones. A
// *conditions.PreCheckFailure describes a state the user must see on the
// Ready condition; an error is transient.
func (r *LogicalBackupRDBMSReconciler) resolve(
	ctx context.Context,
	backup *v1.LogicalBackupRDBMS,
) (*resolved, *conditions.PreCheckFailure, error) {
	precheck, err := logicalbackup.PreCheck(ctx, logicalbackup.PreCheckRequest{
		Reader:      r.APIReader,
		Ref:         backup.Spec.ClusterRef,
		Namespace:   backup.Namespace,
		StorageType: v1.SecondaryStorageTypeRDBMS,
		InProgress:  r.inProgress(backup),
	})
	if err != nil {
		var failure *conditions.PreCheckFailure
		if errors.As(err, &failure) {
			return nil, failure, nil
		}

		return nil, nil, err
	}

	cluster := precheck.Cluster

	var dbConfig v1.DatabaseConfig
	dbKey := types.NamespacedName{
		Namespace: precheck.Storage.Namespace,
		Name:      precheck.Storage.Spec.RDBMS.DatabaseConfigRef,
	}
	if err := r.APIReader.Get(ctx, dbKey, &dbConfig); err != nil {
		if apierrors.IsNotFound(err) {
			return nil, invalidReference("DatabaseConfig %s does not exist", dbKey), nil
		}

		return nil, nil, fmt.Errorf("reading DatabaseConfig %s: %w", dbKey, err)
	}

	if dbConfig.Spec.BackupCredentialsSecretRef == nil {
		return nil, &conditions.PreCheckFailure{
			Reason: v1.ReasonMissingSecret,
			Message: fmt.Sprintf(
				"DatabaseConfig %s has no backupCredentialsSecretRef, which a dump needs", dbKey,
			),
		}, nil
	}

	var server v1.DatabaseServerConfig
	serverKey := types.NamespacedName{Name: dbConfig.Spec.ServerRef}
	if err := r.APIReader.Get(ctx, serverKey, &server); err != nil {
		if apierrors.IsNotFound(err) {
			return nil, invalidReference("DatabaseServerConfig %q does not exist", serverKey.Name), nil
		}

		return nil, nil, fmt.Errorf("reading DatabaseServerConfig %q: %w", serverKey.Name, err)
	}

	if server.Spec.Version == "" {
		return nil, invalidReference(
			"DatabaseServerConfig %q sets no version; the dump needs it to run matching client tools",
			serverKey.Name,
		), nil
	}

	if r.OperatorImage == "" {
		return nil, invalidReference(
			"the manager runs without --operator-image, so the upload container has no image",
		), nil
	}

	merged := cluster.Spec
	if cluster.Spec.PresetRef != "" {
		var preset v1.CamundaClusterPreset
		if err := r.APIReader.Get(
			ctx, types.NamespacedName{Name: cluster.Spec.PresetRef}, &preset,
		); err != nil {
			if apierrors.IsNotFound(err) {
				return nil, invalidReference(
					"CamundaClusterPreset %q does not exist", cluster.Spec.PresetRef,
				), nil
			}

			return nil, nil, fmt.Errorf("reading CamundaClusterPreset %q: %w", cluster.Spec.PresetRef, err)
		}
		merged = camundacluster.MergePreset(cluster.Spec, &preset.Spec)
	}

	dump := dumpBlock(merged, backup)

	bucketSecret := ""
	if precheck.Bucket.CredentialsSecret() != nil {
		bucketSecret = camundacluster.MirroredSecretName(
			cluster, camundacluster.MirrorPurposeBackupCredentials,
		)
		keys := precheck.Bucket.CredentialsSecret().Keys
		message, err := secretref.CheckKeys(
			ctx,
			r.APIReader,
			types.NamespacedName{Namespace: cluster.Namespace, Name: bucketSecret},
			keys...,
		)
		if err != nil {
			return nil, nil, fmt.Errorf("checking the bucket credentials copy: %w", err)
		}
		if message != "" {
			return nil, &conditions.PreCheckFailure{
				Reason: v1.ReasonMissingCredentials,
				Message: message + "; the CamundaCluster controller copies the bucket " +
					"credentials into the cluster namespace",
			}, nil
		}
	}

	return &resolved{
		precheck:       precheck,
		dump:           dump,
		serviceAccount: camundacluster.ServiceAccountName(cluster, camundacluster.NewEffective(merged)),
		bucketSecret:   bucketSecret,
		server:         &server,
		dbConfig:       &dbConfig,
	}, nil, nil
}

// dumpBlock returns the dump settings of one backup: the backup's own block
// replacing the cluster's as a whole, or the cluster's.
func dumpBlock(merged v1.CamundaClusterSpec, backup *v1.LogicalBackupRDBMS) *v1.BackupDumpSpec {
	if backup.Spec.Dump != nil {
		return backup.Spec.Dump
	}
	if merged.Backup != nil {
		return merged.Backup.Dump
	}

	return nil
}

// advance moves the backup one step: allocate its identity, run the dump Job,
// then the primary-storage backup.
func (r *LogicalBackupRDBMSReconciler) advance(
	ctx context.Context,
	backup *v1.LogicalBackupRDBMS,
	res *resolved,
) (ctrl.Result, error) {
	if backup.Status.BackupID == 0 {
		r.start(backup, res)

		// The identity must be persisted before the Job exists: a crash
		// between the two would otherwise allocate a second id against an
		// immutable Job template. The deferred flush writes it; the requeue
		// re-enters with it recorded.
		return ctrl.Result{RequeueAfter: time.Second}, nil
	}

	switch backup.Status.Step {
	case v1.StepDumping:
		return r.dump(ctx, backup, res)
	case v1.StepPrimaryBackup:
		return r.primaryBackup(ctx, backup, res)
	}

	return ctrl.Result{}, fmt.Errorf("unknown step %q", backup.Status.Step)
}

// start allocates the identity of the backup and records the effective
// restore size of the brokers. It only mutates status; the caller persists.
func (r *LogicalBackupRDBMSReconciler) start(backup *v1.LogicalBackupRDBMS, res *resolved) {
	id := logicalbackup.AllocateBackupID(metav1.Now())
	cluster := res.precheck.Cluster

	backup.Status.BackupID = id
	backup.Status.ObjectKey = logicalbackup.ObjectKeyPrefix(
		res.precheck.Bucket.BasePath(), cluster.Namespace, cluster.Name, id,
	) + "/" + components.DumpFileName
	backup.Status.Step = v1.StepDumping
	backup.Status.Phase = v1.LogicalBackupRunning

	logicalbackup.RecordStorageSizes(&backup.Status.StorageSizes, v1.LogicalBackupStorageSizes{
		Zeebe: logicalbackup.ZeebeSize(cluster.Status.Volumes),
	})

	conditions.Stage(backup, progressing(backup, "the dump Job starts"))
	r.EventRecorder.Eventf(
		backup,
		nil,
		corev1.EventTypeNormal,
		eventReasonStarted,
		eventActionBackup,
		"Backup %d of CamundaCluster %s/%s started",
		id,
		cluster.Namespace,
		cluster.Name,
	)
}

// dump applies the Job and tracks it to completion.
func (r *LogicalBackupRDBMSReconciler) dump(
	ctx context.Context,
	backup *v1.LogicalBackupRDBMS,
	res *resolved,
) (ctrl.Result, error) {
	if err := r.mirrorDBCredentials(ctx, backup, res); err != nil {
		var failure *conditions.PreCheckFailure
		if errors.As(err, &failure) {
			conditions.Stage(backup, conditions.Failed(backup, failure))

			return ctrl.Result{RequeueAfter: r.retryInterval()}, nil
		}

		return ctrl.Result{}, err
	}

	cluster := res.precheck.Cluster
	creds := res.dbConfig.Spec.BackupCredentialsSecretRef
	job, err := components.BuildJob(components.JobInput{
		Backup:             backup,
		ClusterName:        cluster.Name,
		ClusterNamespace:   cluster.Namespace,
		Dump:               res.dump,
		Bucket:             res.precheck.Bucket,
		BucketSecretName:   res.bucketSecret,
		DBSecretName:       dbSecretName(backup),
		DBUsernameKey:      creds.UsernameKey,
		DBPasswordKey:      creds.PasswordKey,
		ServiceAccountName: res.serviceAccount,
		ServerVersion:      res.server.Spec.Version,
		Host:               res.server.Spec.Host,
		Port:               res.server.Spec.Port,
		Database:           res.dbConfig.Spec.DatabaseName,
		ObjectKey:          backup.Status.ObjectKey,
		OperatorImage:      r.OperatorImage,
	})
	if err != nil {
		return ctrl.Result{}, err
	}

	// The Job is applied once and adopted afterwards: the API server stamps
	// controller labels into the immutable template, so a byte-identical
	// re-apply would still be rejected as a template change.
	var current batchv1.Job
	err = r.Get(ctx, client.ObjectKeyFromObject(job), &current)
	switch {
	case apierrors.IsNotFound(err):
		r.ownWhenLocal(backup, job)
		if err := r.Patch(
			ctx,
			job,
			client.Apply, //nolint:staticcheck // the repo applies through the deprecated client.Apply patch
			client.FieldOwner(fieldManager),
			client.ForceOwnership,
		); err != nil {
			return ctrl.Result{}, fmt.Errorf("applying the dump Job: %w", err)
		}
		backup.Status.JobName = job.Name
		conditions.Stage(backup, progressing(backup, "the dump Job runs"))

		return ctrl.Result{}, nil
	case err != nil:
		return ctrl.Result{}, fmt.Errorf("reading the dump Job: %w", err)
	}
	backup.Status.JobName = current.Name

	switch {
	case jobCondition(&current, batchv1.JobComplete):
		backup.Status.Step = v1.StepPrimaryBackup
		conditions.Stage(backup, progressing(backup, "the dump uploaded; the primary-storage backup starts"))

		return ctrl.Result{RequeueAfter: time.Second}, nil
	case jobCondition(&current, batchv1.JobFailed):
		r.fail(backup, fmt.Sprintf("the dump Job failed: %s", jobFailureMessage(&current)))

		return ctrl.Result{}, nil
	}

	conditions.Stage(backup, progressing(backup, "the dump Job runs"))

	// The Job is owned and watched; its completion re-enqueues the backup.
	return ctrl.Result{}, nil
}

// primaryBackup requests one cluster-generated primary-storage backup and
// polls it to completion.
func (r *LogicalBackupRDBMSReconciler) primaryBackup(
	ctx context.Context,
	backup *v1.LogicalBackupRDBMS,
	res *resolved,
) (ctrl.Result, error) {
	admin, failure, err := r.admin(ctx, res.precheck.Cluster)
	if err != nil {
		return ctrl.Result{}, err
	}
	if failure != nil {
		conditions.Stage(backup, conditions.Failed(backup, failure))

		return ctrl.Result{RequeueAfter: r.retryInterval()}, nil
	}

	if backup.Status.PrimaryBackupID == nil {
		id, err := admin.StartRuntimeBackup(ctx, nil)
		switch {
		case errors.Is(err, camundaadmin.ErrConflict):
			// No id was supplied, so a conflict on the generated one is a
			// cluster fault, not re-entry.
			r.fail(backup, fmt.Sprintf("the cluster rejected its own generated backup id: %v", err))

			return ctrl.Result{}, nil
		case errors.Is(err, camundaadmin.ErrUnreachable):
			conditions.Stage(backup, unreachable(backup, err))

			return ctrl.Result{RequeueAfter: r.retryInterval()}, nil
		case err != nil:
			r.fail(backup, fmt.Sprintf("requesting the primary-storage backup: %v", err))

			return ctrl.Result{}, nil
		}

		backup.Status.PrimaryBackupID = &id
		conditions.Stage(backup, progressing(backup, "the primary-storage backup runs"))

		// Persist the generated id before polling it: a crash here must
		// re-enter polling, never request a second backup.
		return ctrl.Result{RequeueAfter: time.Second}, nil
	}

	status, err := admin.RuntimeBackupStatus(ctx, *backup.Status.PrimaryBackupID)
	if errors.Is(err, camundaadmin.ErrUnreachable) {
		conditions.Stage(backup, unreachable(backup, err))

		return ctrl.Result{RequeueAfter: r.retryInterval()}, nil
	}
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("reading the primary-storage backup: %w", err)
	}

	switch status.State {
	case camundaadmin.StateCompleted:
		r.complete(backup)

		return ctrl.Result{}, nil
	case camundaadmin.StateInProgress:
		conditions.Stage(backup, progressing(backup, "the primary-storage backup runs"))

		return ctrl.Result{RequeueAfter: r.retryInterval()}, nil
	}

	r.fail(backup, fmt.Sprintf(
		"primary-storage backup %d reports %s: %s",
		*backup.Status.PrimaryBackupID, status.State, status.FailureReason,
	))

	return ctrl.Result{}, nil
}

// admin builds the management client of the cluster from its published
// binding. A missing binding is a waiting state, not an error.
func (r *LogicalBackupRDBMSReconciler) admin(
	ctx context.Context,
	cluster *v1.CamundaCluster,
) (*camundaadmin.Client, *conditions.PreCheckFailure, error) {
	binding := cluster.Status.Management
	if binding == nil || binding.Endpoint == "" {
		return nil, &conditions.PreCheckFailure{
			Reason: v1.ReasonProgressing,
			Message: fmt.Sprintf(
				"CamundaCluster %s/%s has not published its management binding yet",
				cluster.Namespace, cluster.Name,
			),
		}, nil
	}

	auth := camundaadmin.Auth{}
	if binding.Auth.Method == v1.ManagementAuthMethodBasic && binding.Auth.CredentialsSecretRef != nil {
		ref := binding.Auth.CredentialsSecretRef
		secret, message, err := secretref.Get(
			ctx,
			r.APIReader,
			types.NamespacedName{Namespace: ref.Namespace, Name: ref.Name},
			ref.UsernameKey,
			ref.PasswordKey,
		)
		if err != nil {
			return nil, nil, fmt.Errorf("reading the management credentials: %w", err)
		}
		if message != "" {
			return nil, &conditions.PreCheckFailure{Reason: v1.ReasonMissingSecret, Message: message}, nil
		}
		auth.Username = string(secret.Data[ref.UsernameKey])
		auth.Password = string(secret.Data[ref.PasswordKey])
	}

	admin, err := camundaadmin.New(camundaadmin.Binding{
		Endpoint: binding.Endpoint,
		Version:  binding.Version,
		Auth:     auth,
	})
	if err != nil {
		return nil, &conditions.PreCheckFailure{
			Reason:  v1.ReasonInvalidReference,
			Message: fmt.Sprintf("the management binding is unusable: %v", err),
		}, nil
	}

	return admin, nil, nil
}

// mirrorDBCredentials copies the backup credentials of the database into the
// cluster namespace, where the Job pod can mount them.
func (r *LogicalBackupRDBMSReconciler) mirrorDBCredentials(
	ctx context.Context,
	backup *v1.LogicalBackupRDBMS,
	res *resolved,
) error {
	ref := res.dbConfig.Spec.BackupCredentialsSecretRef
	source, message, err := secretref.Get(
		ctx,
		r.APIReader,
		types.NamespacedName{Namespace: ref.Namespace, Name: ref.Name},
		ref.UsernameKey,
		ref.PasswordKey,
	)
	if err != nil {
		return fmt.Errorf("reading the backup credentials: %w", err)
	}
	if message != "" {
		return &conditions.PreCheckFailure{Reason: v1.ReasonMissingSecret, Message: message}
	}

	cluster := res.precheck.Cluster
	managed := labels.Managed(labels.LogicalBackupRDBMS(backup.Name), "dump")
	managed[labels.ClusterKey] = cluster.Name

	local := &corev1.Secret{
		TypeMeta: metav1.TypeMeta{APIVersion: "v1", Kind: "Secret"},
		ObjectMeta: metav1.ObjectMeta{
			Name:      dbSecretName(backup),
			Namespace: cluster.Namespace,
			Labels:    managed,
		},
		Type: corev1.SecretTypeOpaque,
		Data: map[string][]byte{
			ref.UsernameKey: source.Data[ref.UsernameKey],
			ref.PasswordKey: source.Data[ref.PasswordKey],
		},
	}
	r.ownWhenLocal(backup, local)

	if err := r.Patch(
		ctx,
		local,
		client.Apply, //nolint:staticcheck // the repo applies through the deprecated client.Apply patch
		client.FieldOwner(fieldManager),
		client.ForceOwnership,
	); err != nil {
		return fmt.Errorf("copying the backup credentials: %w", err)
	}

	return nil
}

// ownWhenLocal sets the backup as the owner of obj when both share a
// namespace, so deleting the backup collects it. A cross-namespace reference
// cannot carry an owner; the finalizer deletes those explicitly.
func (r *LogicalBackupRDBMSReconciler) ownWhenLocal(backup *v1.LogicalBackupRDBMS, obj client.Object) {
	if obj.GetNamespace() != backup.Namespace {
		return
	}
	_ = controllerutil.SetControllerReference(backup, obj, r.Scheme)
}

func (r *LogicalBackupRDBMSReconciler) complete(backup *v1.LogicalBackupRDBMS) {
	now := metav1.Now()
	backup.Status.Phase = v1.LogicalBackupCompleted
	backup.Status.CompletionTime = &now
	conditions.Stage(backup, conditions.Ready(
		metav1.ConditionTrue, v1.ReasonCompleted, "the backup finished and is restorable", backup.Generation,
	))
	r.EventRecorder.Eventf(
		backup,
		nil,
		corev1.EventTypeNormal,
		eventReasonCompleted,
		eventActionBackup,
		"Backup %d completed",
		backup.Status.BackupID,
	)
}

func (r *LogicalBackupRDBMSReconciler) fail(backup *v1.LogicalBackupRDBMS, message string) {
	now := metav1.Now()
	backup.Status.Phase = v1.LogicalBackupFailed
	backup.Status.CompletionTime = &now
	conditions.Stage(backup, conditions.Ready(
		metav1.ConditionFalse, v1.ReasonFailed, message, backup.Generation,
	))
	r.EventRecorder.Eventf(
		backup,
		nil,
		corev1.EventTypeWarning,
		eventReasonFailed,
		eventActionBackup,
		"Backup %d failed: %s",
		backup.Status.BackupID,
		message,
	)
}

// inProgress reports a non-terminal backup of the same cluster, of this kind
// and, when wired, of the sibling kind.
func (r *LogicalBackupRDBMSReconciler) inProgress(backup *v1.LogicalBackupRDBMS) logicalbackup.InProgress {
	return func(ctx context.Context) (string, error) {
		key := clusterKey(backup)

		var list v1.LogicalBackupRDBMSList
		if err := r.List(ctx, &list, client.MatchingFields{clusterKeyIndex: key}); err != nil {
			return "", err
		}
		for i := range list.Items {
			other := &list.Items[i]
			if other.UID == backup.UID || other.Terminal() {
				continue
			}

			return other.Name, nil
		}

		if r.SiblingInProgress == nil {
			return "", nil
		}
		namespace, name, _ := splitKey(key)

		return r.SiblingInProgress(ctx, types.NamespacedName{Namespace: namespace, Name: name})
	}
}

func (r *LogicalBackupRDBMSReconciler) retryInterval() time.Duration {
	if r.RetryInterval > 0 {
		return r.RetryInterval
	}

	return defaultRetryInterval
}

func progressing(backup *v1.LogicalBackupRDBMS, message string) metav1.Condition {
	return conditions.Ready(metav1.ConditionFalse, v1.ReasonProgressing, message, backup.Generation)
}

func unreachable(backup *v1.LogicalBackupRDBMS, err error) metav1.Condition {
	return conditions.Ready(
		metav1.ConditionFalse, v1.ReasonConnectionFailed, err.Error(), backup.Generation,
	)
}

func invalidReference(format string, args ...any) *conditions.PreCheckFailure {
	return &conditions.PreCheckFailure{
		Reason:  v1.ReasonInvalidReference,
		Message: fmt.Sprintf(format, args...),
	}
}

func dbSecretName(backup *v1.LogicalBackupRDBMS) string {
	return backup.Name + "-dump-credentials"
}

// clusterKey is the index value of one backup: the namespace and name of the
// cluster it references, with the namespace defaulted to the backup's own.
func clusterKey(backup *v1.LogicalBackupRDBMS) string {
	namespace := backup.Spec.ClusterRef.Namespace
	if namespace == "" {
		namespace = backup.Namespace
	}

	return namespace + "/" + backup.Spec.ClusterRef.Name
}

func splitKey(key string) (namespace, name string, ok bool) {
	for i := range key {
		if key[i] == '/' {
			return key[:i], key[i+1:], true
		}
	}

	return "", key, false
}

func jobCondition(job *batchv1.Job, kind batchv1.JobConditionType) bool {
	for _, cond := range job.Status.Conditions {
		if cond.Type == kind && cond.Status == corev1.ConditionTrue {
			return true
		}
	}

	return false
}

func jobFailureMessage(job *batchv1.Job) string {
	for _, cond := range job.Status.Conditions {
		if cond.Type == batchv1.JobFailed && cond.Status == corev1.ConditionTrue {
			return cond.Reason + ": " + cond.Message
		}
	}

	return "unknown reason"
}

// SetupWithManager registers the controller, the cluster-key index, and the
// watches: the backups, the Jobs they own, and the referenced clusters.
func (r *LogicalBackupRDBMSReconciler) SetupWithManager(mgr ctrl.Manager) error {
	if r.EventRecorder == nil {
		r.EventRecorder = mgr.GetEventRecorder("logicalbackuprdbms")
	}
	if r.OpenBucket == nil {
		r.OpenBucket = func(
			ctx context.Context, cfg *v1.ObjectStorageConfig, creds *objectstore.Credentials,
		) (ArtifactBucket, error) {
			return objectstore.Open(ctx, cfg, creds)
		}
	}

	if err := mgr.GetFieldIndexer().IndexField(
		context.Background(),
		&v1.LogicalBackupRDBMS{},
		clusterKeyIndex,
		func(obj client.Object) []string {
			return []string{clusterKey(obj.(*v1.LogicalBackupRDBMS))}
		},
	); err != nil {
		return fmt.Errorf("indexing backups by cluster: %w", err)
	}

	return ctrl.NewControllerManagedBy(mgr).
		For(&v1.LogicalBackupRDBMS{}).
		Owns(&batchv1.Job{}).
		Watches(&v1.CamundaCluster{}, r.enqueueForCluster()).
		Named("logicalbackuprdbms").
		Complete(r)
}

// enqueueForCluster maps a cluster event to every backup that references it,
// so a suspend flip or a published binding wakes the waiting backups.
func (r *LogicalBackupRDBMSReconciler) enqueueForCluster() handler.EventHandler {
	return handler.EnqueueRequestsFromMapFunc(func(ctx context.Context, obj client.Object) []ctrl.Request {
		var list v1.LogicalBackupRDBMSList
		if err := r.List(ctx, &list, client.MatchingFields{
			clusterKeyIndex: obj.GetNamespace() + "/" + obj.GetName(),
		}); err != nil {
			return nil
		}

		requests := make([]ctrl.Request, 0, len(list.Items))
		for i := range list.Items {
			requests = append(requests, ctrl.Request{
				NamespacedName: client.ObjectKeyFromObject(&list.Items[i]),
			})
		}

		return requests
	})
}
