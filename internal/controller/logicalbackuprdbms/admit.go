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

package logicalbackuprdbms

import (
	"context"
	"errors"
	"fmt"
	"strings"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	v1 "github.com/konsole-is/camunda-operator/api/v1"
	camundacluster "github.com/konsole-is/camunda-operator/pkg/components/camundacluster"
	components "github.com/konsole-is/camunda-operator/pkg/components/logicalbackuprdbms"
	"github.com/konsole-is/camunda-operator/pkg/conditions"
	"github.com/konsole-is/camunda-operator/pkg/logicalbackup"
	"github.com/konsole-is/camunda-operator/pkg/management"
	"github.com/konsole-is/camunda-operator/pkg/secretref"
)

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

	if _, failure, err := r.resolveDump(ctx, backup, precheck); err != nil || failure != nil {
		if err != nil {
			return settle, err
		}

		return r.parkPending(backup, failure), nil
	}
	if failure := r.checkManagement(ctx, precheck.Cluster); failure != nil {
		return r.parkPending(backup, failure), nil
	}

	r.start(backup, precheck)

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

	return hold{after: r.opts.RetryInterval}
}

// checkManagement verifies the management binding is usable at admission, so
// a backup never dumps gigabytes it cannot pair with a Zeebe backup
// afterwards. The client is rebuilt when the step needs it.
func (r *LogicalBackupRDBMSReconciler) checkManagement(
	ctx context.Context,
	cluster *v1.CamundaCluster,
) *conditions.PreCheckFailure {
	_, failure, err := management.NewClient(ctx, r.APIReader, cluster)
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

// resolveDump resolves everything the dump Job renders from, one concern per
// helper: the database chain, the server it runs on, where the credentials
// are reachable, and the pod settings. Each helper reports a failure the
// user must see or an error to retry; resolveDump composes them.
func (r *LogicalBackupRDBMSReconciler) resolveDump(
	ctx context.Context,
	backup *v1.LogicalBackupRDBMS,
	precheck *logicalbackup.PreCheckResult,
) (*dumpResolution, *conditions.PreCheckFailure, error) {
	dbConfig, failure, err := r.resolveDatabaseConfig(ctx, precheck.Storage)
	if err != nil || failure != nil {
		return nil, failure, err
	}

	server, failure, err := r.resolveServer(ctx, dbConfig)
	if err != nil || failure != nil {
		return nil, failure, err
	}

	dbSecret, bucketSecret, failure, err := r.resolveCredentials(
		ctx, precheck.Cluster, precheck.Bucket, *dbConfig.Spec.BackupCredentialsSecretRef,
	)
	if err != nil || failure != nil {
		return nil, failure, err
	}

	dump, account, failure, err := r.resolvePod(ctx, precheck.Cluster, backup)
	if err != nil || failure != nil {
		return nil, failure, err
	}

	return &dumpResolution{
		cluster:      precheck.Cluster,
		bucket:       precheck.Bucket,
		dump:         dump,
		account:      account,
		bucketSecret: bucketSecret,
		dbSecret:     dbSecret,
		server:       server,
		dbConfig:     dbConfig,
	}, nil, nil
}

// resolveDatabaseConfig reads the DatabaseConfig the storage binding names
// and requires the backup user a dump runs as.
func (r *LogicalBackupRDBMSReconciler) resolveDatabaseConfig(
	ctx context.Context,
	storage *v1.SecondaryStorageConfig,
) (*v1.DatabaseConfig, *conditions.PreCheckFailure, error) {
	var dbConfig v1.DatabaseConfig
	key := types.NamespacedName{
		Namespace: storage.Namespace,
		Name:      storage.Spec.RDBMS.DatabaseConfigRef,
	}
	if err := r.APIReader.Get(ctx, key, &dbConfig); err != nil {
		if apierrors.IsNotFound(err) {
			return nil, logicalbackup.InvalidReference("DatabaseConfig %s does not exist", key), nil
		}

		return nil, nil, fmt.Errorf("reading DatabaseConfig %s: %w", key, err)
	}

	if dbConfig.Spec.BackupCredentialsSecretRef == nil {
		return nil, &conditions.PreCheckFailure{
			Reason: v1.ReasonMissingSecret,
			Message: fmt.Sprintf(
				"DatabaseConfig %s has no backupCredentialsSecretRef, which a dump needs", key,
			),
		}, nil
	}

	return &dbConfig, nil, nil
}

// resolveServer reads the DatabaseServerConfig of the database and requires
// the major version its controller probed: the dump runs client tools of that
// major, and guessing one risks a pg_dump older than the server.
func (r *LogicalBackupRDBMSReconciler) resolveServer(
	ctx context.Context,
	dbConfig *v1.DatabaseConfig,
) (*v1.DatabaseServerConfig, *conditions.PreCheckFailure, error) {
	var server v1.DatabaseServerConfig
	key := types.NamespacedName{Name: dbConfig.Spec.ServerRef}
	if err := r.APIReader.Get(ctx, key, &server); err != nil {
		if apierrors.IsNotFound(err) {
			return nil, logicalbackup.InvalidReference(
				"DatabaseServerConfig %q does not exist", key.Name,
			), nil
		}

		return nil, nil, fmt.Errorf("reading DatabaseServerConfig %q: %w", key.Name, err)
	}

	if !serverProbedForCurrentSpec(&server) {
		return nil, logicalbackup.InvalidReference(
			"DatabaseServerConfig %q has not been probed for its current spec: its controller "+
				"publishes status.serverVersion once it reaches the server as declared, and the "+
				"dump needs it to run matching client tools",
			key.Name,
		), nil
	}

	return &server, nil, nil
}

// serverProbedForCurrentSpec reports whether the version the server publishes
// belongs to the spec it has now: Ready is True for the current generation
// and a version is recorded. The controller keeps the last version while a
// retargeted server is unreachable, so the version alone could be the old
// server's; only a current Ready proves it is this one's.
func serverProbedForCurrentSpec(server *v1.DatabaseServerConfig) bool {
	if server.Status.ServerVersion == "" {
		return false
	}
	ready := meta.FindStatusCondition(server.Status.Conditions, v1.ConditionReady)

	return ready != nil &&
		ready.Status == metav1.ConditionTrue &&
		ready.ObservedGeneration == server.Generation
}

// resolveCredentials locates the two Secrets the Job mounts as reachable from
// the cluster namespace, following the CamundaCluster controller's rule: a
// Secret in the cluster namespace is used where it is, one anywhere else
// through the local copy that controller maintains. It returns the dump
// credentials reference rewritten to the local location and the local name of
// the bucket credentials — empty for workload identity.
func (r *LogicalBackupRDBMSReconciler) resolveCredentials(
	ctx context.Context,
	cluster *v1.CamundaCluster,
	bucket *v1.ObjectStorageConfig,
	dbSecret v1.CredentialsSecretRef,
) (v1.CredentialsSecretRef, string, *conditions.PreCheckFailure, error) {
	local := localSecretName(
		cluster, dbSecret.Namespace, dbSecret.Name, camundacluster.MirrorPurposeDumpCredentials,
	)
	dbSecret.Name, dbSecret.Namespace = local, cluster.Namespace
	failure, err := r.checkLocalSecret(
		ctx, cluster.Namespace, local, v1.ReasonMissingSecret, "dump",
		dbSecret.UsernameKey, dbSecret.PasswordKey,
	)
	if err != nil || failure != nil {
		return dbSecret, "", failure, err
	}

	credentials := bucket.CredentialsSecret()
	if credentials == nil {
		return dbSecret, "", nil, nil
	}
	bucketSecret := localSecretName(
		cluster, credentials.Namespace, credentials.Name, camundacluster.MirrorPurposeBackupCredentials,
	)
	failure, err = r.checkLocalSecret(
		ctx, cluster.Namespace, bucketSecret, v1.ReasonMissingCredentials, "bucket",
		credentials.Keys...,
	)
	if err != nil || failure != nil {
		return dbSecret, "", failure, err
	}

	return dbSecret, bucketSecret, nil, nil
}

// checkLocalSecret verifies that the Secret at namespace/name carries keys,
// mapping a miss to a pre-check failure with the given reason. purpose names
// the credentials in the message, which also says who keeps the copy when the
// Secret is one.
func (r *LogicalBackupRDBMSReconciler) checkLocalSecret(
	ctx context.Context,
	namespace, name, reason, purpose string,
	keys ...string,
) (*conditions.PreCheckFailure, error) {
	message, err := secretref.CheckKeys(
		ctx, r.APIReader, types.NamespacedName{Namespace: namespace, Name: name}, keys...,
	)
	if err != nil {
		return nil, fmt.Errorf("checking the %s credentials: %w", purpose, err)
	}
	if message == "" {
		return nil, nil
	}

	return &conditions.PreCheckFailure{
		Reason: reason,
		Message: fmt.Sprintf(
			"%s; the CamundaCluster controller keeps the local copy of %s credentials that live "+
				"outside the cluster namespace",
			message, purpose,
		),
	}, nil
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

// resolvePod resolves the pod settings of the Job: the effective dump block
// and the ServiceAccount, both through the cluster's preset when it names
// one.
func (r *LogicalBackupRDBMSReconciler) resolvePod(
	ctx context.Context,
	cluster *v1.CamundaCluster,
	backup *v1.LogicalBackupRDBMS,
) (*v1.BackupDumpSpec, string, *conditions.PreCheckFailure, error) {
	merged := cluster.Spec
	if cluster.Spec.PresetRef != "" {
		var preset v1.CamundaClusterPreset
		if err := r.APIReader.Get(
			ctx, types.NamespacedName{Name: cluster.Spec.PresetRef}, &preset,
		); err != nil {
			if apierrors.IsNotFound(err) {
				return nil, "", logicalbackup.InvalidReference(
					"CamundaClusterPreset %q does not exist", cluster.Spec.PresetRef,
				), nil
			}

			return nil, "", nil, fmt.Errorf(
				"reading CamundaClusterPreset %q: %w", cluster.Spec.PresetRef, err,
			)
		}
		merged = camundacluster.MergePreset(cluster.Spec, &preset.Spec)
	}

	account := camundacluster.ServiceAccountName(cluster, camundacluster.NewEffective(merged))

	dump := dumpBlock(merged, backup)
	if reserved := components.ReservedEnv(dump); len(reserved) > 0 {
		return nil, "", logicalbackup.InvalidReference(
			"the dump block sets %s in extraEnv; the Job reserves the connection variables "+
				"(PGHOST, PGPORT, PGDATABASE, PGUSER, PGPASSWORD, ...) and every UPLOAD_* variable "+
				"for itself, so a dump cannot be redirected or run as someone else",
			strings.Join(reserved, ", "),
		), nil
	}

	return dump, account, nil, nil
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
func (r *LogicalBackupRDBMSReconciler) start(
	backup *v1.LogicalBackupRDBMS,
	precheck *logicalbackup.PreCheckResult,
) {
	id := logicalbackup.AllocateBackupID(metav1.Now())
	cluster := precheck.Cluster

	backup.Status.BackupID = id
	backup.Status.ObjectKey = logicalbackup.ObjectKeyPrefix(
		precheck.Bucket.BasePath(), cluster.Namespace, cluster.Name, id,
	) + "/" + components.DumpFileName
	backup.Status.BucketRef = precheck.Bucket.Name
	backup.Status.BucketGeneration = precheck.Bucket.Generation
	backup.Status.BucketLocation = precheck.Bucket.Location()
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
