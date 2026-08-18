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

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	v1 "github.com/konsole-is/camunda-operator/api/v1"
	"github.com/konsole-is/camunda-operator/pkg/camundaconfig"
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

	if failure := clusterConverged(precheck.Cluster); failure != nil {
		return r.parkPending(backup, failure), nil
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

	// The claim is the gate. The pre-checks above order the claimants and
	// verify the references. Only the Lease decides who holds the cluster,
	// and it is taken before the identity of the backup is written.
	holder, err := r.claimCluster(ctx, backup)
	if err != nil {
		return settle, err
	}
	if holder != "" {
		return r.parkPending(backup, &conditions.PreCheckFailure{
			Reason: v1.ReasonBackupInProgress,
			Message: fmt.Sprintf(
				"backup %s of CamundaCluster %s/%s holds the cluster; backups of one cluster run one at a time",
				holder, precheck.Cluster.Namespace, precheck.Cluster.Name,
			),
		}), nil
	}

	hash, failure, err := r.zeebeConfigHash(ctx, precheck.Cluster)
	if err != nil {
		return settle, err
	}
	if failure != nil {
		return r.parkPending(backup, failure), nil
	}

	highest, err := r.highestSiblingBackupID(ctx, backup)
	if err != nil {
		return settle, err
	}
	r.start(backup, precheck, highest, hash)

	// The identity must be persisted before the Job exists: a crash between
	// the two would otherwise allocate a second id against an immutable Job
	// template. The deferred flush writes it; the requeue re-enters with it
	// recorded.
	return shortly, nil
}

// highestSiblingBackupID returns the highest backup ID among the other
// backups of this kind that name the same cluster, terminal ones included, or
// zero. It arbitrates the ID allocation against the siblings that this
// controller can see. A clock that stepped backwards then cannot hand out an
// ID that one of them holds. Nothing arbitrates the residual — the IDs of
// deleted resources — for the dump: the Zeebe request carries no id, so the
// cluster answers no conflict for it. That is why the dump key carries the
// UID of the backup (components.DumpObjectKey): a reused id can never name
// another backup's object.
func (r *LogicalBackupRDBMSReconciler) highestSiblingBackupID(
	ctx context.Context,
	backup *v1.LogicalBackupRDBMS,
) (int64, error) {
	var list v1.LogicalBackupRDBMSList
	if err := r.APIReader.List(ctx, &list, client.InNamespace(backup.Namespace)); err != nil {
		return 0, fmt.Errorf("listing LogicalBackupRDBMS: %w", err)
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

// clusterConverged requires the cluster to run the spec it declares: the
// operator has reconciled the current generation and reports Ready for it.
// A backup admitted against a desired spec that Zeebe does not run yet would
// dump to the new backup store while the Zeebe backup, requested before the
// rollout finishes, lands in the old one — a completed status for a split
// restore point. This is a wait, not a user error, so it reports
// Progressing.
func clusterConverged(cluster *v1.CamundaCluster) *conditions.PreCheckFailure {
	ready := meta.FindStatusCondition(cluster.Status.Conditions, v1.ConditionReady)
	observed := cluster.Status.ObservedGeneration
	if observed == cluster.Generation && ready != nil &&
		ready.Status == metav1.ConditionTrue && ready.ObservedGeneration == cluster.Generation {
		return nil
	}

	readyState := "absent"
	if ready != nil {
		readyState = fmt.Sprintf("%s/%s at generation %d", ready.Status, ready.Reason, ready.ObservedGeneration)
	}

	return &conditions.PreCheckFailure{
		Reason: v1.ReasonProgressing,
		Message: fmt.Sprintf(
			"CamundaCluster %s/%s has not converged on its current spec (generation %d, observed %d, "+
				"Ready %s); a backup taken now could pair a dump with a Zeebe backup of the previous "+
				"configuration",
			cluster.Namespace, cluster.Name, cluster.Generation, observed, readyState,
		),
	}
}

// zeebeWorkload reads the live Zeebe StatefulSet of the cluster: the pod
// template that says what Zeebe actually runs. A workload that is not
// rendered yet is a wait, reported as Progressing.
func (r *LogicalBackupRDBMSReconciler) zeebeWorkload(
	ctx context.Context,
	cluster *v1.CamundaCluster,
) (*appsv1.StatefulSet, *conditions.PreCheckFailure, error) {
	var workload appsv1.StatefulSet
	key := types.NamespacedName{
		Namespace: cluster.Namespace,
		Name:      camundacluster.WorkloadName(cluster, camundacluster.ComponentZeebe),
	}
	if err := r.APIReader.Get(ctx, key, &workload); err != nil {
		if apierrors.IsNotFound(err) {
			return nil, &conditions.PreCheckFailure{
				Reason: v1.ReasonProgressing,
				Message: fmt.Sprintf(
					"the Zeebe workload %s is not rendered yet; the backup needs the configuration "+
						"it runs to pin", key,
				),
			}, nil
		}

		return nil, nil, fmt.Errorf("reading the Zeebe workload %s: %w", key, err)
	}

	return &workload, nil, nil
}

// zeebeConfigHash reads the config hash the live Zeebe pod template carries:
// the strongest observable identity of the configuration Zeebe actually
// runs. Mutable referents — the DatabaseConfig, the DatabaseServerConfig —
// enter that hash without bumping the cluster's generation, so the converged
// generation alone cannot prove that Zeebe still runs the database that a
// dump captures. The hash is pinned at start and required unchanged before
// the Job and before the Zeebe request.
func (r *LogicalBackupRDBMSReconciler) zeebeConfigHash(
	ctx context.Context,
	cluster *v1.CamundaCluster,
) (string, *conditions.PreCheckFailure, error) {
	workload, failure, err := r.zeebeWorkload(ctx, cluster)
	if err != nil || failure != nil {
		return "", failure, err
	}

	hash := workload.Spec.Template.Annotations[camundacluster.ConfigHashAnnotation]
	if hash == "" {
		return "", &conditions.PreCheckFailure{
			Reason: v1.ReasonProgressing,
			Message: fmt.Sprintf(
				"the Zeebe workload %s/%s carries no config hash yet; the backup needs the "+
					"configuration it runs to pin", workload.Namespace, workload.Name,
			),
		}, nil
	}

	return hash, nil, nil
}

// zeebeRunsDatabase requires the database that a dump would capture — the
// DatabaseServerConfig host and port and the DatabaseConfig database name,
// as resolved now — to be the one the live Zeebe pod template is configured
// with. The pinned config hash proves only that Zeebe did not roll since
// the start; it cannot tell that the referents changed and the cluster
// controller has not rendered them yet. In that window the dump would read
// the new referents while Zeebe still runs the old database, and a dump and
// a Zeebe backup of two databases would report one restore point. The
// template carries the URL Zeebe runs, so the two are compared directly.
// A mismatch is a wait: the cluster rolls to the referenced database, or
// the referents go back, and until then no dump starts.
func (r *LogicalBackupRDBMSReconciler) zeebeRunsDatabase(
	ctx context.Context,
	cluster *v1.CamundaCluster,
	server *v1.DatabaseServerConfig,
	dbConfig *v1.DatabaseConfig,
) (*conditions.PreCheckFailure, error) {
	workload, failure, err := r.zeebeWorkload(ctx, cluster)
	if err != nil || failure != nil {
		return failure, err
	}

	running, ok := templateEnvValue(&workload.Spec.Template, camundaconfig.KeyRDBMSURL.Env())
	if !ok {
		return &conditions.PreCheckFailure{
			Reason: v1.ReasonProgressing,
			Message: fmt.Sprintf(
				"the Zeebe workload %s/%s carries no relational storage URL yet; the backup needs "+
					"the database it runs to compare against", workload.Namespace, workload.Name,
			),
		}, nil
	}

	wanted := camundacluster.RDBMSURL(server.Spec.Host, server.Spec.Port, dbConfig.Spec.DatabaseName)
	if running != wanted {
		return &conditions.PreCheckFailure{
			Reason: v1.ReasonProgressing,
			Message: fmt.Sprintf(
				"Zeebe of CamundaCluster %s/%s runs %s, but DatabaseConfig %s/%s and "+
					"DatabaseServerConfig %s now resolve to %s; the dump waits until Zeebe runs "+
					"the database it would capture",
				cluster.Namespace, cluster.Name, running,
				dbConfig.Namespace, dbConfig.Name, server.Name, wanted,
			),
		}, nil
	}

	return nil, nil
}

// templateEnvValue returns the plain value of the environment variable name
// on any container of the template, and whether one carries it as a value.
func templateEnvValue(template *corev1.PodTemplateSpec, name string) (string, bool) {
	for i := range template.Spec.Containers {
		for _, env := range template.Spec.Containers[i].Env {
			if env.Name == name && env.ValueFrom == nil {
				return env.Value, true
			}
		}
	}

	return "", false
}

// workloadUnchanged requires the live Zeebe workload to still carry the
// config hash pinned at start. A changed hash means Zeebe rolled to another
// configuration — for example a swapped database — and pairing the dump with
// a Zeebe backup taken now would report an unusable restore point as
// complete.
func (r *LogicalBackupRDBMSReconciler) workloadUnchanged(
	ctx context.Context,
	backup *v1.LogicalBackupRDBMS,
	cluster *v1.CamundaCluster,
) (*conditions.PreCheckFailure, error) {
	hash, failure, err := r.zeebeConfigHash(ctx, cluster)
	if err != nil || failure != nil {
		return failure, err
	}
	if hash != backup.Status.WorkloadConfigHash {
		return logicalbackup.InvalidReference(
			"the Zeebe workload of CamundaCluster %s/%s now runs config hash %s, but the backup "+
				"pinned %s at start; its configuration — for example the database — changed in "+
				"between, so the dump and a Zeebe backup taken now would not be one restore point",
			cluster.Namespace, cluster.Name, hash, backup.Status.WorkloadConfigHash,
		), nil
	}

	return nil, nil
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
	cluster *v1.CamundaCluster
	bucket  *v1.ObjectStorageConfig
	// pod shapes the Job's pod; image runs its dump container. They come
	// from different owners: the pod settings may be the backup's, the image
	// is always the cluster's.
	pod          *v1.DumpPodSpec
	image        string
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

	if failure, err := r.zeebeRunsDatabase(ctx, precheck.Cluster, server, dbConfig); err != nil || failure != nil {
		return nil, failure, err
	}

	dbSecret, bucketSecret, failure, err := r.resolveCredentials(
		ctx, precheck.Cluster, precheck.Bucket, *dbConfig.Spec.BackupCredentialsSecretRef,
	)
	if err != nil || failure != nil {
		return nil, failure, err
	}

	pod, failure, err := r.resolvePod(ctx, precheck.Cluster, backup)
	if err != nil || failure != nil {
		return nil, failure, err
	}

	return &dumpResolution{
		cluster:      precheck.Cluster,
		bucket:       precheck.Bucket,
		pod:          pod.settings,
		image:        pod.image,
		account:      pod.account,
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

// podResolution is what resolvePod produces: the pod settings, the image, and
// the ServiceAccount of the Job.
type podResolution struct {
	settings *v1.DumpPodSpec
	image    string
	account  string
}

// resolvePod resolves the pod of the Job through the cluster's preset when it
// names one: the pod settings — the backup's own block replacing the
// cluster's as a whole, or the cluster's — the ServiceAccount, and the image,
// which is always the cluster's: the Job runs under the cluster's
// ServiceAccount, so the executable is the cluster owner's choice.
func (r *LogicalBackupRDBMSReconciler) resolvePod(
	ctx context.Context,
	cluster *v1.CamundaCluster,
	backup *v1.LogicalBackupRDBMS,
) (*podResolution, *conditions.PreCheckFailure, error) {
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

	settings, image := dumpBlock(merged, backup)
	// The environment bound applies to the backup's own block only: the
	// cluster's spec.backup.dump is its owner's policy inside their own
	// boundary. The CRD schema enforces the envFrom half too; this is the
	// second layer.
	if backup != nil && backup.Spec.Dump != nil {
		if reserved := components.ReservedEnv(backup.Spec.Dump); len(reserved) > 0 {
			return nil, logicalbackup.InvalidReference(
				"the backup's dump block sets %s in extraEnv; every name under PG or UPLOAD_ is "+
					"reserved, so a dump cannot be redirected or run as someone else",
				strings.Join(reserved, ", "),
			), nil
		}
		if unsafe := components.UnsafeEnvFrom(backup.Spec.Dump); len(unsafe) > 0 {
			return nil, logicalbackup.InvalidReference(
				"the backup's dump block has extraEnvFrom %s without a safe prefix; a source "+
					"could otherwise supply PGHOSTADDR and redirect the dump",
				strings.Join(unsafe, ", "),
			), nil
		}
	}

	return &podResolution{
		settings: settings,
		image:    image,
		account:  camundacluster.ServiceAccountName(cluster, camundacluster.NewEffective(merged)),
	}, nil, nil
}

// dumpBlock returns the pod settings and the image of one backup's Job. The
// settings are the backup's own block replacing the cluster's as a whole, or
// the cluster's; the image is the cluster's or empty for the default.
func dumpBlock(merged v1.CamundaClusterSpec, backup *v1.LogicalBackupRDBMS) (*v1.DumpPodSpec, string) {
	var cluster *v1.BackupDumpSpec
	if merged.Backup != nil {
		cluster = merged.Backup.Dump
	}
	image := ""
	if cluster != nil {
		image = cluster.PostgresImage
	}
	if backup != nil && backup.Spec.Dump != nil {
		return backup.Spec.Dump, image
	}
	if cluster != nil {
		return &cluster.DumpPodSpec, image
	}

	return nil, image
}

// start allocates the identity of the backup — after the highest id a
// visible sibling holds, so a clock that stepped backwards cannot reuse one
// — pins the bucket it writes through, and records the effective restore
// size of the brokers. It only mutates status; the caller persists.
func (r *LogicalBackupRDBMSReconciler) start(
	backup *v1.LogicalBackupRDBMS,
	precheck *logicalbackup.PreCheckResult,
	highestSiblingID int64,
	workloadConfigHash string,
) {
	id := logicalbackup.AllocateBackupIDAfter(metav1.Now(), highestSiblingID)
	cluster := precheck.Cluster

	backup.Status.BackupID = id
	backup.Status.ObjectKey = components.DumpObjectKey(
		precheck.Bucket.BasePath(), cluster.Namespace, cluster.Name, id, backup.UID,
	)
	backup.Status.BucketRef = precheck.Bucket.Name
	backup.Status.BucketGeneration = precheck.Bucket.Generation
	backup.Status.BucketLocation = precheck.Bucket.Location()
	backup.Status.WorkloadConfigHash = workloadConfigHash
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
