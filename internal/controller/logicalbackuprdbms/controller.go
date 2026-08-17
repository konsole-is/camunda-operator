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
//
// Admission and runtime are split. The full pre-checks — references, storage
// type, serialization against other backups — run only until the backup
// starts. A running backup re-resolves only what its current step needs, so
// a reference that breaks mid-run cannot park it: it either finishes on what
// it already holds, or terminalizes after a bounded grace.
package logicalbackuprdbms

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/sourcehawk/operator-component-framework/pkg/component"
	"github.com/sourcehawk/operator-component-framework/pkg/component/concepts"
	ocfjob "github.com/sourcehawk/operator-component-framework/pkg/primitives/job"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/events"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	v1 "github.com/konsole-is/camunda-operator/api/v1"
	"github.com/konsole-is/camunda-operator/pkg/camundaadmin"
	camundacluster "github.com/konsole-is/camunda-operator/pkg/components/camundacluster"
	components "github.com/konsole-is/camunda-operator/pkg/components/logicalbackuprdbms"
	"github.com/konsole-is/camunda-operator/pkg/conditions"
	"github.com/konsole-is/camunda-operator/pkg/logicalbackup"
	"github.com/konsole-is/camunda-operator/pkg/objectstore"
	"github.com/konsole-is/camunda-operator/pkg/refindex"
	"github.com/konsole-is/camunda-operator/pkg/secretref"
)

const (
	// fieldManager owns the Job of a backup in Server-Side Apply.
	fieldManager = "camunda-operator/backup"
	// clusterKeyIndex indexes backups by the namespace/name of the cluster
	// they reference, for the serialization pre-check and the cluster watch.
	clusterKeyIndex = "logicalbackuprdbms.spec.clusterRef"
	// defaultRetryInterval paces the requeues of waiting states: an
	// unreachable management API, a running sibling backup, or a reference
	// that nothing watches.
	defaultRetryInterval = 30 * time.Second
	// defaultMidRunGrace bounds how long a running backup waits on a
	// dependency that stopped resolving before it fails. A broken reference
	// parks one backup for this long at most, never forever — a parked
	// non-terminal backup blocks every later backup of the cluster.
	defaultMidRunGrace = 10 * time.Minute
	// defaultRegistrationGrace bounds how long the primary-storage poll
	// tolerates a backup the cluster does not report yet: the partitions
	// register their parts asynchronously after the 202.
	defaultRegistrationGrace = 2 * time.Minute

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

// hold is the domain result of one reconcile step: how long to wait before
// the next look, or nothing when watches carry the wake-up. Only Reconcile
// turns it into a ctrl.Result.
type hold struct {
	after time.Duration
}

var (
	// settle waits on watches alone.
	settle = hold{}
	// shortly re-enters to persist staged status before acting on it.
	shortly = hold{after: time.Second}
)

// LogicalBackupRDBMSReconciler reconciles a LogicalBackupRDBMS.
type LogicalBackupRDBMSReconciler struct {
	client.Client
	// APIReader reads without the cache. Admission decides on live state:
	// a stale suspend flag or a stale sibling list must not start a backup.
	APIReader client.Reader
	Scheme    *runtime.Scheme
	// EventRecorder publishes the backup lifecycle events. SetupWithManager
	// sets it from the manager.
	EventRecorder events.EventRecorder
	// OperatorImage runs the upload container of the dump Job. Empty means
	// the image is resolved from the operator's own Pod on first use;
	// --operator-image overrides it.
	OperatorImage string
	// OpenBucket opens the backup bucket for the finalizer. Nil means
	// pkg/objectstore; tests point it at a local fake.
	OpenBucket func(
		ctx context.Context, cfg *v1.ObjectStorageConfig, creds *objectstore.Credentials,
	) (ArtifactBucket, error)
	// SiblingInProgress reports a non-terminal backup of the other kind for
	// the same cluster. The manager wires the LogicalBackupElasticsearch
	// controller in here; nil means no other kind is checked.
	SiblingInProgress logicalbackup.SiblingInProgress
	// RetryInterval overrides defaultRetryInterval. Zero means the default.
	RetryInterval time.Duration
	// MidRunGrace overrides defaultMidRunGrace. Zero means the default.
	MidRunGrace time.Duration
	// RegistrationGrace overrides defaultRegistrationGrace. Zero means the
	// default.
	RegistrationGrace time.Duration
}

// +kubebuilder:rbac:groups=core.camunda.io,resources=logicalbackuprdbmses,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=core.camunda.io,resources=logicalbackuprdbmses/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=core.camunda.io,resources=logicalbackuprdbmses/finalizers,verbs=update
// +kubebuilder:rbac:groups=core.camunda.io,resources=camundaclusters;secondarystorageconfigs;databaseconfigs;databaseserverconfigs;objectstorageconfigs;camundaclusterpresets,verbs=get;list;watch
// +kubebuilder:rbac:groups=batch,resources=jobs,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="",resources=secrets,verbs=get;list;watch
// +kubebuilder:rbac:groups="",resources=pods,verbs=get
// +kubebuilder:rbac:groups=events.k8s.io,resources=events,verbs=create;patch

// Reconcile drives one backup to a terminal phase: admission, the dump Job,
// then the primary-storage backup of the cluster. It is the only function
// that builds a ctrl.Result.
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

	if backup.Terminal() {
		// A conflict on the terminal flush can restore a stale Ready from
		// the server; re-staging the terminal condition is idempotent and
		// heals it on the next look.
		r.stageTerminal(&backup)

		return ctrl.Result{}, nil
	}

	var wait hold
	switch {
	case backup.Status.BackupID == 0:
		wait, err = r.admit(ctx, &backup)
	case backup.Status.Step == v1.StepDumping:
		wait, err = r.dump(ctx, &backup)
	case backup.Status.Step == v1.StepPrimaryBackup:
		wait, err = r.primaryBackup(ctx, &backup)
	default:
		return ctrl.Result{}, fmt.Errorf("unknown step %q", backup.Status.Step)
	}
	if err != nil {
		return ctrl.Result{}, err
	}

	return ctrl.Result{RequeueAfter: wait.after}, nil
}

// admit runs the full pre-checks and starts the backup when they pass. They
// run only here: a backup that started already owns its resolved identity,
// and re-checking mid-run would let a broken reference park it forever.
func (r *LogicalBackupRDBMSReconciler) admit(
	ctx context.Context,
	backup *v1.LogicalBackupRDBMS,
) (hold, error) {
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
			return r.parkPending(backup, failure), nil
		}

		return settle, err
	}

	res, failure, err := r.resolveStart(ctx, backup, precheck)
	if err != nil {
		return settle, err
	}
	if failure != nil {
		return r.parkPending(backup, failure), nil
	}

	r.start(backup, res)

	// The identity must be persisted before the Job exists: a crash between
	// the two would otherwise allocate a second id against an immutable Job
	// template. The deferred flush writes it; the requeue re-enters with it
	// recorded.
	return shortly, nil
}

// parkPending records a pre-check failure: the documented Pending phase and
// the Ready condition carrying the reason. Nothing watches most of the
// checked references from here, so the reconcile comes back on a timer.
func (r *LogicalBackupRDBMSReconciler) parkPending(
	backup *v1.LogicalBackupRDBMS,
	failure *conditions.PreCheckFailure,
) hold {
	backup.Status.Phase = v1.LogicalBackupPending
	conditions.Stage(backup, conditions.Failed(backup, failure))

	return hold{after: r.retryInterval()}
}

// startResolution is everything admission resolves beyond the shared
// pre-checks, consumed once by start.
type startResolution struct {
	precheck *logicalbackup.PreCheckResult
	image    string
}

// resolveStart runs the RDBMS-specific admission checks: the database chain
// is dumpable, the credentials are reachable from the cluster namespace, and
// the upload container has an image.
func (r *LogicalBackupRDBMSReconciler) resolveStart(
	ctx context.Context,
	backup *v1.LogicalBackupRDBMS,
	precheck *logicalbackup.PreCheckResult,
) (*startResolution, *conditions.PreCheckFailure, error) {
	if _, failure, err := r.resolveDump(ctx, backup, precheck); err != nil || failure != nil {
		return nil, failure, err
	}

	if failure := r.checkManagement(ctx, precheck.Cluster); failure != nil {
		return nil, failure, nil
	}

	image, err := r.operatorImage(ctx)
	if err != nil {
		return nil, logicalbackup.InvalidReference(
			"the operator image is unknown: %v; set --operator-image on the manager", err,
		), nil
	}

	return &startResolution{precheck: precheck, image: image}, nil, nil
}

// checkManagement verifies the management binding is usable at admission, so
// a backup never dumps gigabytes it cannot pair with a primary-storage
// backup afterwards. The client is rebuilt when the step needs it.
func (r *LogicalBackupRDBMSReconciler) checkManagement(
	ctx context.Context,
	cluster *v1.CamundaCluster,
) *conditions.PreCheckFailure {
	_, failure, err := logicalbackup.ManagementClient(ctx, r.APIReader, cluster)
	if err != nil {
		return &conditions.PreCheckFailure{Reason: v1.ReasonConnectionFailed, Message: err.Error()}
	}

	return failure
}

// dumpResolution is what the Dumping step needs to render its Job.
type dumpResolution struct {
	cluster      *v1.CamundaCluster
	bucket       *v1.ObjectStorageConfig
	dump         *v1.BackupDumpSpec
	account      string
	bucketSecret string
	dbSecret     v1.CredentialsSecretRef
	server       *v1.DatabaseServerConfig
	dbConfig     *v1.DatabaseConfig
}

// resolveDump resolves the database chain and the credential locations. The
// credentials follow the CamundaCluster controller's rule: a Secret in the
// cluster namespace is used where it is, one anywhere else through the local
// copy that controller maintains.
func (r *LogicalBackupRDBMSReconciler) resolveDump(
	ctx context.Context,
	backup *v1.LogicalBackupRDBMS,
	precheck *logicalbackup.PreCheckResult,
) (*dumpResolution, *conditions.PreCheckFailure, error) {
	cluster := precheck.Cluster

	var dbConfig v1.DatabaseConfig
	dbKey := types.NamespacedName{
		Namespace: precheck.Storage.Namespace,
		Name:      precheck.Storage.Spec.RDBMS.DatabaseConfigRef,
	}
	if err := r.APIReader.Get(ctx, dbKey, &dbConfig); err != nil {
		if apierrors.IsNotFound(err) {
			return nil, logicalbackup.InvalidReference("DatabaseConfig %s does not exist", dbKey), nil
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
			return nil, logicalbackup.InvalidReference(
				"DatabaseServerConfig %q does not exist", serverKey.Name,
			), nil
		}

		return nil, nil, fmt.Errorf("reading DatabaseServerConfig %q: %w", serverKey.Name, err)
	}

	if server.Spec.Version == "" {
		return nil, logicalbackup.InvalidReference(
			"DatabaseServerConfig %q sets no version; the dump needs it to run matching client tools",
			serverKey.Name,
		), nil
	}

	merged := cluster.Spec
	if cluster.Spec.PresetRef != "" {
		var preset v1.CamundaClusterPreset
		if err := r.APIReader.Get(
			ctx, types.NamespacedName{Name: cluster.Spec.PresetRef}, &preset,
		); err != nil {
			if apierrors.IsNotFound(err) {
				return nil, logicalbackup.InvalidReference(
					"CamundaClusterPreset %q does not exist", cluster.Spec.PresetRef,
				), nil
			}

			return nil, nil, fmt.Errorf(
				"reading CamundaClusterPreset %q: %w", cluster.Spec.PresetRef, err,
			)
		}
		merged = camundacluster.MergePreset(cluster.Spec, &preset.Spec)
	}

	dbSecret := *dbConfig.Spec.BackupCredentialsSecretRef
	local := localSecretName(
		cluster, dbSecret.Namespace, dbSecret.Name, camundacluster.MirrorPurposeDumpCredentials,
	)
	dbSecret.Name, dbSecret.Namespace = local, cluster.Namespace
	if message, err := secretref.CheckKeys(
		ctx,
		r.APIReader,
		types.NamespacedName{Namespace: cluster.Namespace, Name: local},
		dbSecret.UsernameKey,
		dbSecret.PasswordKey,
	); err != nil {
		return nil, nil, fmt.Errorf("checking the dump credentials: %w", err)
	} else if message != "" {
		return nil, &conditions.PreCheckFailure{
			Reason: v1.ReasonMissingSecret,
			Message: message + "; the CamundaCluster controller keeps the local copy of the " +
				"dump credentials",
		}, nil
	}

	bucketSecret := ""
	if credentials := precheck.Bucket.CredentialsSecret(); credentials != nil {
		bucketSecret = localSecretName(
			cluster,
			credentials.Namespace,
			credentials.Name,
			camundacluster.MirrorPurposeBackupCredentials,
		)
		if message, err := secretref.CheckKeys(
			ctx,
			r.APIReader,
			types.NamespacedName{Namespace: cluster.Namespace, Name: bucketSecret},
			credentials.Keys...,
		); err != nil {
			return nil, nil, fmt.Errorf("checking the bucket credentials: %w", err)
		} else if message != "" {
			return nil, &conditions.PreCheckFailure{
				Reason: v1.ReasonMissingCredentials,
				Message: message + "; the CamundaCluster controller keeps the local copy of the " +
					"bucket credentials",
			}, nil
		}
	}

	return &dumpResolution{
		cluster:      cluster,
		bucket:       precheck.Bucket,
		dump:         dumpBlock(merged, backup),
		account:      camundacluster.ServiceAccountName(cluster, camundacluster.NewEffective(merged)),
		bucketSecret: bucketSecret,
		dbSecret:     dbSecret,
		server:       &server,
		dbConfig:     &dbConfig,
	}, nil, nil
}

// localSecretName resolves where a referenced Secret is reachable from the
// cluster namespace, mirroring the rule of the CamundaCluster controller: the
// source itself when it already lives there, its purpose-named copy
// otherwise.
func localSecretName(cluster *v1.CamundaCluster, namespace, name, purpose string) string {
	if namespace == cluster.Namespace {
		return name
	}

	return camundacluster.MirroredSecretName(cluster, purpose)
}

// dumpBlock returns the dump settings of one backup: the backup's own block
// replacing the cluster's as a whole, or the cluster's.
func dumpBlock(merged v1.CamundaClusterSpec, backup *v1.LogicalBackupRDBMS) *v1.BackupDumpSpec {
	if backup != nil && backup.Spec.Dump != nil {
		return backup.Spec.Dump
	}
	if merged.Backup != nil {
		return merged.Backup.Dump
	}

	return nil
}

// start allocates the identity of the backup, pins the bucket it writes
// through, and records the effective restore size of the brokers. It only
// mutates status; the caller persists.
func (r *LogicalBackupRDBMSReconciler) start(backup *v1.LogicalBackupRDBMS, res *startResolution) {
	id := logicalbackup.AllocateBackupID(metav1.Now())
	cluster := res.precheck.Cluster

	backup.Status.BackupID = id
	backup.Status.ObjectKey = logicalbackup.ObjectKeyPrefix(
		res.precheck.Bucket.BasePath(), cluster.Namespace, cluster.Name, id,
	) + "/" + components.DumpFileName
	backup.Status.BucketRef = res.precheck.Bucket.Name
	backup.Status.BucketGeneration = res.precheck.Bucket.Generation
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

// dump applies the Job once and tracks it to completion. A dependency that
// stopped resolving holds the backup for the mid-run grace, then fails it:
// a Running backup must either finish or terminalize, never park.
func (r *LogicalBackupRDBMSReconciler) dump(
	ctx context.Context,
	backup *v1.LogicalBackupRDBMS,
) (hold, error) {
	namespace := backup.Spec.ClusterRef.EffectiveClusterNamespace(backup.Namespace)
	key := types.NamespacedName{Namespace: namespace, Name: components.JobName(backup)}

	var current batchv1.Job
	if backup.Status.JobName != "" {
		// The Job was applied; the cache is enough to track it, and the
		// watch (same-namespace) or the poll below (cross-namespace) wakes
		// the backup on progress.
		if err := r.Get(ctx, key, &current); err != nil {
			if apierrors.IsNotFound(err) {
				// The recorded Job is gone before it reported: deleted by
				// hand. The dump cannot be trusted to have uploaded.
				r.fail(backup, "the dump Job disappeared before it completed")

				return settle, nil
			}

			return settle, err
		}

		return r.trackJob(backup, &current)
	}

	// The Job does not exist yet; the read must be live, because a stale
	// cache after the apply would re-apply against the server-stamped
	// immutable template and be rejected.
	err := r.APIReader.Get(ctx, key, &current)
	switch {
	case err == nil:
		backup.Status.JobName = current.Name

		return r.trackJob(backup, &current)
	case !apierrors.IsNotFound(err):
		return settle, err
	}

	return r.createJob(ctx, backup)
}

// createJob re-resolves the dump dependencies and applies the Job. It runs
// once per backup; afterwards the recorded name is tracked, never re-applied.
func (r *LogicalBackupRDBMSReconciler) createJob(
	ctx context.Context,
	backup *v1.LogicalBackupRDBMS,
) (hold, error) {
	res, failure, err := r.resolveRunning(ctx, backup)
	if err != nil {
		return settle, err
	}
	if failure != nil {
		return r.holdRunning(backup, failure)
	}

	image, err := r.operatorImage(ctx)
	if err != nil {
		return r.holdRunning(backup, logicalbackup.InvalidReference(
			"the operator image is unknown: %v", err,
		))
	}

	creds := res.dbSecret
	job, err := components.BuildJob(components.JobInput{
		Backup:             backup,
		ClusterName:        res.cluster.Name,
		ClusterNamespace:   res.cluster.Namespace,
		Dump:               res.dump,
		Bucket:             res.bucket,
		BucketSecretName:   res.bucketSecret,
		DBSecretName:       creds.Name,
		DBUsernameKey:      creds.UsernameKey,
		DBPasswordKey:      creds.PasswordKey,
		ServiceAccountName: res.account,
		ServerVersion:      res.server.Spec.Version,
		Host:               res.server.Spec.Host,
		Port:               res.server.Spec.Port,
		Database:           res.dbConfig.Spec.DatabaseName,
		ObjectKey:          backup.Status.ObjectKey,
		OperatorImage:      image,
	})
	if err != nil {
		return settle, err
	}

	if job.Namespace == backup.Namespace {
		if err := controllerutil.SetControllerReference(backup, job, r.Scheme); err != nil {
			return settle, fmt.Errorf("owning the dump Job: %w", err)
		}
	}
	//nolint:staticcheck // the repo applies through the deprecated client.Apply patch
	if err := r.Patch(
		ctx, job, client.Apply, client.FieldOwner(fieldManager), client.ForceOwnership,
	); err != nil {
		return settle, fmt.Errorf("applying the dump Job: %w", err)
	}
	backup.Status.JobName = job.Name
	conditions.Stage(backup, progressing(backup, "the dump Job runs"))

	// The watch only covers the backup's own namespace; a cross-namespace
	// cluster needs the poll.
	return hold{after: r.retryInterval()}, nil
}

// resolveRunning re-resolves what the Dumping step needs, reusing the
// admission resolution against the pinned bucket. It reports a failure the
// user must see; holdRunning bounds how long that failure may hold the
// backup.
func (r *LogicalBackupRDBMSReconciler) resolveRunning(
	ctx context.Context,
	backup *v1.LogicalBackupRDBMS,
) (*dumpResolution, *conditions.PreCheckFailure, error) {
	namespace := backup.Spec.ClusterRef.EffectiveClusterNamespace(backup.Namespace)

	var cluster v1.CamundaCluster
	if err := r.APIReader.Get(
		ctx,
		types.NamespacedName{Namespace: namespace, Name: backup.Spec.ClusterRef.Name},
		&cluster,
	); err != nil {
		if apierrors.IsNotFound(err) {
			return nil, logicalbackup.InvalidReference(
				"CamundaCluster %s/%s does not exist", namespace, backup.Spec.ClusterRef.Name,
			), nil
		}

		return nil, nil, fmt.Errorf("reading the cluster: %w", err)
	}

	var storage v1.SecondaryStorageConfig
	if cluster.Spec.StorageRef == "" {
		return nil, logicalbackup.InvalidReference(
			"CamundaCluster %s/%s has no spec.storageRef", namespace, cluster.Name,
		), nil
	}
	storageKey := types.NamespacedName{Namespace: namespace, Name: cluster.Spec.StorageRef}
	if err := r.APIReader.Get(ctx, storageKey, &storage); err != nil {
		if apierrors.IsNotFound(err) {
			return nil, logicalbackup.InvalidReference(
				"SecondaryStorageConfig %s does not exist", storageKey,
			), nil
		}

		return nil, nil, fmt.Errorf("reading SecondaryStorageConfig %s: %w", storageKey, err)
	}
	if storage.Spec.RDBMS == nil {
		return nil, logicalbackup.InvalidReference(
			"SecondaryStorageConfig %s no longer describes a relational backend", storageKey,
		), nil
	}

	// The pinned bucket, not the cluster's current backupStorageRef: the
	// object key was written through the pinned one.
	var bucket v1.ObjectStorageConfig
	if err := r.APIReader.Get(
		ctx, types.NamespacedName{Name: backup.Status.BucketRef}, &bucket,
	); err != nil {
		if apierrors.IsNotFound(err) {
			return nil, logicalbackup.InvalidReference(
				"the pinned ObjectStorageConfig %q does not exist", backup.Status.BucketRef,
			), nil
		}

		return nil, nil, fmt.Errorf("reading ObjectStorageConfig %q: %w", backup.Status.BucketRef, err)
	}

	return r.resolveDump(ctx, backup, &logicalbackup.PreCheckResult{
		Cluster: &cluster,
		Storage: &storage,
		Bucket:  &bucket,
	})
}

// holdRunning stages a mid-run failure and decides its fate: within the
// grace it holds the backup on a timer, past it the backup fails. The grace
// is measured from the start of the backup — its id is the start timestamp.
func (r *LogicalBackupRDBMSReconciler) holdRunning(
	backup *v1.LogicalBackupRDBMS,
	failure *conditions.PreCheckFailure,
) (hold, error) {
	started := time.UnixMilli(backup.Status.BackupID)
	if time.Since(started) > r.midRunGrace() {
		r.fail(backup, fmt.Sprintf(
			"a dependency stopped resolving and did not recover: %s", failure.Message,
		))

		return settle, nil
	}

	conditions.Stage(backup, conditions.Failed(backup, failure))

	return hold{after: r.retryInterval()}, nil
}

// trackJob maps the observed Job onto the backup through the same status
// handler the ocf job primitive uses.
func (r *LogicalBackupRDBMSReconciler) trackJob(
	backup *v1.LogicalBackupRDBMS,
	job *batchv1.Job,
) (hold, error) {
	status, err := ocfjob.DefaultConvergingStatusHandler(concepts.ConvergingOperationNone, job)
	if err != nil {
		return settle, err
	}

	switch status.Status {
	case concepts.CompletionStatusCompleted:
		backup.Status.Step = v1.StepPrimaryBackup
		conditions.Stage(backup, progressing(
			backup, "the dump uploaded; the primary-storage backup starts",
		))

		return shortly, nil
	case concepts.CompletionStatusFailing:
		r.fail(backup, fmt.Sprintf("the dump Job failed: %s", status.Reason))

		return settle, nil
	}

	conditions.Stage(backup, progressing(backup, status.Reason))

	// The watch wakes same-namespace backups instantly; the poll covers a
	// cross-namespace cluster, whose Job the watch cannot map back.
	return hold{after: r.retryInterval()}, nil
}

// primaryBackup requests one cluster-generated primary-storage backup and
// polls it to completion.
func (r *LogicalBackupRDBMSReconciler) primaryBackup(
	ctx context.Context,
	backup *v1.LogicalBackupRDBMS,
) (hold, error) {
	namespace := backup.Spec.ClusterRef.EffectiveClusterNamespace(backup.Namespace)

	var cluster v1.CamundaCluster
	if err := r.APIReader.Get(
		ctx,
		types.NamespacedName{Namespace: namespace, Name: backup.Spec.ClusterRef.Name},
		&cluster,
	); err != nil {
		if apierrors.IsNotFound(err) {
			return r.holdRunning(backup, logicalbackup.InvalidReference(
				"CamundaCluster %s/%s does not exist", namespace, backup.Spec.ClusterRef.Name,
			))
		}

		return settle, err
	}

	admin, failure, err := logicalbackup.ManagementClient(ctx, r.APIReader, &cluster)
	if err != nil {
		return settle, err
	}
	if failure != nil {
		return r.holdRunning(backup, failure)
	}

	if backup.Status.PrimaryBackupID == nil {
		id, err := admin.StartRuntimeBackup(ctx, nil)
		switch {
		case errors.Is(err, camundaadmin.ErrUnreachable):
			conditions.Stage(backup, unreachable(backup, err))

			return hold{after: r.retryInterval()}, nil
		case err != nil:
			// A rejected call — a 503 through a restarting gateway, or even
			// a conflict on the generated id — is retried with backoff. The
			// dump already succeeded; discarding it over one bad answer
			// would be the real loss. A conflict is never resolved by
			// adopting the backup that holds the id.
			return settle, fmt.Errorf("requesting the primary-storage backup: %w", err)
		}

		now := metav1.Now()
		backup.Status.PrimaryBackupID = &id
		backup.Status.PrimaryBackupRequestedAt = &now
		conditions.Stage(backup, progressing(backup, "the primary-storage backup runs"))

		// Persist the generated id before polling it: a crash here must
		// re-enter polling, never request a second backup.
		return shortly, nil
	}

	status, err := admin.RuntimeBackupStatus(ctx, *backup.Status.PrimaryBackupID)
	if errors.Is(err, camundaadmin.ErrUnreachable) {
		conditions.Stage(backup, unreachable(backup, err))

		return hold{after: r.retryInterval()}, nil
	}
	if err != nil {
		return settle, fmt.Errorf("reading the primary-storage backup: %w", err)
	}

	switch status.State {
	case camundaadmin.StateCompleted:
		r.complete(backup)

		return settle, nil
	case camundaadmin.StateInProgress:
		conditions.Stage(backup, progressing(backup, "the primary-storage backup runs"))

		return hold{after: r.retryInterval()}, nil
	case camundaadmin.StateDoesNotExist, camundaadmin.StateIncomplete:
		// The partitions register their parts asynchronously after the 202,
		// so both states are normal right after the request — and fatal
		// once the grace is over.
		if requested := backup.Status.PrimaryBackupRequestedAt; requested != nil &&
			time.Since(requested.Time) < r.registrationGrace() {
			conditions.Stage(backup, progressing(
				backup, "the primary-storage backup is registering its partitions",
			))

			return hold{after: r.retryInterval()}, nil
		}
	}

	r.fail(backup, fmt.Sprintf(
		"primary-storage backup %d reports %s: %s",
		*backup.Status.PrimaryBackupID, status.State, status.FailureReason,
	))

	return settle, nil
}

func (r *LogicalBackupRDBMSReconciler) complete(backup *v1.LogicalBackupRDBMS) {
	now := metav1.Now()
	backup.Status.Phase = v1.LogicalBackupCompleted
	backup.Status.CompletionTime = &now
	r.stageTerminal(backup)
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
	backup.Status.FailureMessage = message
	r.stageTerminal(backup)
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

// stageTerminal stages the Ready condition of a terminal phase. It is
// idempotent, so a terminal backup re-stages it on every look and heals a
// conflict that restored a stale condition.
func (r *LogicalBackupRDBMSReconciler) stageTerminal(backup *v1.LogicalBackupRDBMS) {
	switch backup.Status.Phase {
	case v1.LogicalBackupCompleted:
		conditions.Stage(backup, conditions.Ready(
			metav1.ConditionTrue,
			v1.ReasonCompleted,
			"the backup finished and is restorable",
			backup.Generation,
		))
	case v1.LogicalBackupFailed:
		conditions.Stage(backup, conditions.Ready(
			metav1.ConditionFalse, v1.ReasonFailed, backup.Status.FailureMessage, backup.Generation,
		))
	}
}

// inProgress serializes the backups of one cluster with a deterministic
// entry gate: an already-running backup blocks everything else, and among
// the pending ones only the oldest (creation time, then name) may start.
// Both halves read live state, so two backups admitted from a stale cache
// cannot both start.
func (r *LogicalBackupRDBMSReconciler) inProgress(backup *v1.LogicalBackupRDBMS) logicalbackup.InProgress {
	return func(ctx context.Context) (string, error) {
		cluster := types.NamespacedName{
			Namespace: backup.Spec.ClusterRef.EffectiveClusterNamespace(backup.Namespace),
			Name:      backup.Spec.ClusterRef.Name,
		}

		var list v1.LogicalBackupRDBMSList
		if err := r.APIReader.List(ctx, &list); err != nil {
			return "", err
		}
		for i := range list.Items {
			other := &list.Items[i]
			if other.UID == backup.UID || other.Terminal() {
				continue
			}
			otherCluster := types.NamespacedName{
				Namespace: other.Spec.ClusterRef.EffectiveClusterNamespace(other.Namespace),
				Name:      other.Spec.ClusterRef.Name,
			}
			if otherCluster != cluster {
				continue
			}

			if other.Status.BackupID != 0 || olderBackup(other, backup) {
				return other.Name, nil
			}
		}

		if r.SiblingInProgress == nil {
			return "", nil
		}

		return r.SiblingInProgress(ctx, cluster)
	}
}

// olderBackup reports whether a was created before b, with the name as the
// tie-break, so exactly one of two pending backups ever starts first.
func olderBackup(a, b *v1.LogicalBackupRDBMS) bool {
	if !a.CreationTimestamp.Equal(&b.CreationTimestamp) {
		return a.CreationTimestamp.Before(&b.CreationTimestamp)
	}

	return a.Name < b.Name
}

func (r *LogicalBackupRDBMSReconciler) retryInterval() time.Duration {
	if r.RetryInterval > 0 {
		return r.RetryInterval
	}

	return defaultRetryInterval
}

func (r *LogicalBackupRDBMSReconciler) midRunGrace() time.Duration {
	if r.MidRunGrace > 0 {
		return r.MidRunGrace
	}

	return defaultMidRunGrace
}

func (r *LogicalBackupRDBMSReconciler) registrationGrace() time.Duration {
	if r.RegistrationGrace > 0 {
		return r.RegistrationGrace
	}

	return defaultRegistrationGrace
}

func progressing(backup *v1.LogicalBackupRDBMS, message string) metav1.Condition {
	return conditions.Ready(metav1.ConditionFalse, v1.ReasonProgressing, message, backup.Generation)
}

func unreachable(backup *v1.LogicalBackupRDBMS, err error) metav1.Condition {
	return conditions.Ready(
		metav1.ConditionFalse, v1.ReasonConnectionFailed, err.Error(), backup.Generation,
	)
}

// clusterKey is the index value of one backup: the namespace and name of the
// cluster it references.
func clusterKey(backup *v1.LogicalBackupRDBMS) string {
	return refindex.NamespacedKey(
		backup.Spec.ClusterRef.EffectiveClusterNamespace(backup.Namespace),
		backup.Spec.ClusterRef.Name,
	)
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
		Watches(
			&v1.CamundaCluster{},
			r.enqueueForCluster(),
			builder.WithPredicates(clusterChanged()),
		).
		Named("logicalbackuprdbms").
		Complete(r)
}

// clusterChanged passes the cluster events a waiting backup cares about: a
// spec change (suspend, references) or a change of the published management
// binding. Bare status noise wakes nothing.
func clusterChanged() predicate.Predicate {
	changed := predicate.Funcs{
		UpdateFunc: func(e event.UpdateEvent) bool {
			older, okOld := e.ObjectOld.(*v1.CamundaCluster)
			newer, okNew := e.ObjectNew.(*v1.CamundaCluster)
			if !okOld || !okNew {
				return false
			}
			if older.Generation != newer.Generation {
				return true
			}

			return !managementEqual(older.Status.Management, newer.Status.Management)
		},
	}

	return changed
}

func managementEqual(a, b *v1.ManagementBinding) bool {
	if (a == nil) != (b == nil) {
		return false
	}
	if a == nil {
		return true
	}

	return *a == *b
}

// enqueueForCluster maps a cluster event to every non-terminal backup that
// references it, so a suspend flip or a published binding wakes the waiting
// ones.
func (r *LogicalBackupRDBMSReconciler) enqueueForCluster() handler.EventHandler {
	return handler.EnqueueRequestsFromMapFunc(func(ctx context.Context, obj client.Object) []ctrl.Request {
		var list v1.LogicalBackupRDBMSList
		if err := r.List(ctx, &list, client.MatchingFields{
			clusterKeyIndex: refindex.NamespacedKey(obj.GetNamespace(), obj.GetName()),
		}); err != nil {
			return nil
		}

		requests := make([]ctrl.Request, 0, len(list.Items))
		for i := range list.Items {
			if list.Items[i].Terminal() {
				continue
			}
			requests = append(requests, ctrl.Request{
				NamespacedName: client.ObjectKeyFromObject(&list.Items[i]),
			})
		}

		return requests
	})
}
