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
	"fmt"

	"github.com/sourcehawk/operator-component-framework/pkg/component/concepts"
	ocfjob "github.com/sourcehawk/operator-component-framework/pkg/primitives/job"
	batchv1 "k8s.io/api/batch/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	v1 "github.com/konsole-is/camunda-operator/api/v1"
	camundacluster "github.com/konsole-is/camunda-operator/pkg/components/camundacluster"
	components "github.com/konsole-is/camunda-operator/pkg/components/logicalrestore"
	"github.com/konsole-is/camunda-operator/pkg/conditions"
	"github.com/konsole-is/camunda-operator/pkg/logicalbackup"
	"github.com/konsole-is/camunda-operator/pkg/podstate"
	restorepkg "github.com/konsole-is/camunda-operator/pkg/restore"
	"github.com/konsole-is/camunda-operator/pkg/secretref"
)

// whatSecondary names the work of the pg_restore Job in the messages of a pod
// that cannot start.
const whatSecondary = "the pg_restore Job"

// restoreDatabase writes the dump of the backup back into the logical
// database of the target. It applies one Job that downloads the dump from the
// backup bucket and runs pg_restore over it, and it tracks that Job to its
// end.
func (r *Reconciler) restoreDatabase(ctx context.Context, restore *v1.LogicalRestore) (hold, error) {
	if restore.Status.SecondaryJobName != "" {
		return r.trackDatabaseJob(ctx, restore)
	}

	resolved, failure, err := r.resolveTarget(ctx, restore)
	if err != nil {
		return settle, err
	}
	if failure != nil {
		return r.holdRunning(restore, failure), nil
	}

	database, failure, err := r.resolveDatabase(ctx, resolved)
	if err != nil {
		return settle, err
	}
	if failure != nil {
		return r.holdRunning(restore, failure), nil
	}

	job, err := components.BuildJob(components.JobInput{
		Restore:            restore,
		ClusterName:        resolved.cluster.Name,
		Pod:                database.pod,
		PostgresImage:      database.image,
		Bucket:             database.bucket,
		BucketSecretName:   database.bucketSecret,
		DBSecretName:       database.credentials.Name,
		DBUsernameKey:      database.credentials.UsernameKey,
		DBPasswordKey:      database.credentials.PasswordKey,
		ServiceAccountName: resolved.cluster.Status.ServiceAccountName,
		ServerVersion:      database.server.Status.ServerVersion,
		Host:               database.server.Spec.Host,
		Port:               database.server.Spec.Port,
		Database:           database.config.Spec.DatabaseName,
		ObjectKey:          resolved.backup.ObjectKey,
		CLIImage:           r.opts.CLIImage,
	})
	if err != nil {
		// The builder answers a pod block that it cannot render. No retry
		// changes that answer, and the user corrects it on the cluster.
		r.fail(restore, v1.ReasonFailed, err.Error())

		return settle, nil
	}
	if err := controllerutil.SetControllerReference(restore, job, r.Scheme); err != nil {
		return settle, fmt.Errorf("owning the pg_restore Job: %w", err)
	}

	return r.claimDatabaseJob(ctx, restore, job)
}

// claimDatabaseJob creates the Job as an identity claim: create-only, never a
// forced apply. The create still carries the field manager of this kind, so
// every resource of a restore names the same owner of its fields. A forced apply after a NotFound is not atomic. It overwrites
// the UID label and the owner reference of a same-named Job that lands in
// between, before the adoption check can look at them. A Create is atomic, so
// the API server decides who owns the name. This is the same reasoning the
// dump Job of a relational backup follows.
func (r *Reconciler) claimDatabaseJob(
	ctx context.Context,
	restore *v1.LogicalRestore,
	job *batchv1.Job,
) (hold, error) {
	err := r.Create(ctx, job, restorepkg.FieldManagerLogicalRestore)
	switch {
	case apierrors.IsAlreadyExists(err):
		var winner batchv1.Job
		key := types.NamespacedName{Namespace: job.Namespace, Name: job.Name}
		if err := r.APIReader.Get(ctx, key, &winner); err != nil {
			if apierrors.IsNotFound(err) {
				// The Job was deleted again in between. The requeue re-enters
				// the claim.
				return r.shortly(), nil
			}

			return settle, fmt.Errorf("reading the pg_restore Job that won the name: %w", err)
		}

		return r.adoptDatabaseJob(ctx, restore, &winner)
	case err != nil:
		return settle, fmt.Errorf("creating the pg_restore Job: %w", err)
	}

	recovered(restore)
	restore.Status.SecondaryJobName = job.Name
	conditions.Stage(restore, progressing(restore, "the pg_restore Job runs"))

	return hold{after: r.opts.PollInterval}, nil
}

// adoptDatabaseJob tracks a Job found under the Job name of this restore,
// after it proves that the Job belongs to it. A Job that carries another UID
// is a leftover of a same-named restore that was deleted and created again,
// or a foreign Job. Its completion would let this restore advance without a
// restore of its own database, so the restore fails instead.
func (r *Reconciler) adoptDatabaseJob(
	ctx context.Context,
	restore *v1.LogicalRestore,
	job *batchv1.Job,
) (hold, error) {
	if !components.JobBelongsTo(job, restore) {
		r.fail(restore, v1.ReasonFailed, fmt.Sprintf(
			"Job %s/%s exists but belongs to another restore (label %s=%q); this restore cannot track "+
				"it and will not create a second Job under the same name",
			job.Namespace, job.Name, components.RestoreUIDLabel, job.Labels[components.RestoreUIDLabel],
		))

		return settle, nil
	}
	restore.Status.SecondaryJobName = job.Name

	return r.trackDatabaseJob(ctx, restore)
}

// trackDatabaseJob follows the recorded Job to its end. A completed Job moves
// the restore to primary storage. A failed Job fails the restore and names
// the Job, because the logical database then holds a partial restore that
// only a new attempt repairs.
func (r *Reconciler) trackDatabaseJob(ctx context.Context, restore *v1.LogicalRestore) (hold, error) {
	name := restore.Status.SecondaryJobName

	job, gone, err := r.restoreJob(ctx, restore.Namespace, name)
	if err != nil {
		return settle, err
	}
	if gone {
		r.fail(restore, v1.ReasonFailed, fmt.Sprintf(
			"the pg_restore Job %s disappeared before it completed", name,
		))

		return settle, nil
	}

	status, err := ocfjob.DefaultConvergingStatusHandler(concepts.ConvergingOperationNone, job)
	if err != nil {
		return settle, err
	}

	switch status.Status {
	case concepts.CompletionStatusCompleted:
		recovered(restore)
		restore.Status.Phase = v1.LogicalRestoreRestoringPrimaryStorage
		conditions.Stage(restore, progressing(
			restore, "the logical database is restored; the broker volumes come next",
		))

		return r.shortly(), nil
	case concepts.CompletionStatusFailing:
		r.fail(restore, v1.ReasonFailed, fmt.Sprintf(
			"the pg_restore Job %s failed: %s", name, status.Reason,
		))

		return settle, nil
	}

	stuck, err := r.stuckDatabasePod(ctx, restore)
	if err != nil {
		return settle, err
	}
	if stuck != nil {
		return r.holdRunning(restore, stuck), nil
	}
	conditions.Stage(restore, progressing(restore, status.Reason))

	// The watch on the owned Jobs wakes the restore on progress. The poll also
	// re-checks the pods, because a Job does not report their waiting states.
	return hold{after: r.opts.PollInterval}, nil
}

// databaseResolution is what the pg_restore Job renders from, beside the
// facts that every phase resolves.
type databaseResolution struct {
	config       *v1.DatabaseConfig
	server       *v1.DatabaseServerConfig
	credentials  v1.CredentialsSecretRef
	bucket       *v1.ObjectStorageConfig
	bucketSecret string
	pod          *v1.DumpPodSpec
	image        string
}

// resolveDatabase resolves the logical database of the target, the server it
// runs on, the credentials the Job mounts, and the bucket that holds the
// dump. The database is reached with the backup user of the target, which is
// the user that wrote the dump on the source.
func (r *Reconciler) resolveDatabase(
	ctx context.Context,
	resolved *resolution,
) (*databaseResolution, *conditions.PreCheckFailure, error) {
	if resolved.storage.Spec.RDBMS == nil {
		return nil, logicalbackup.InvalidReference(
			"SecondaryStorageConfig %s/%s no longer describes a relational backend",
			resolved.storage.Namespace, resolved.storage.Name,
		), nil
	}

	config, failure, err := r.databaseConfig(ctx, resolved.storage)
	if err != nil || failure != nil {
		return nil, failure, err
	}

	server, failure, err := r.databaseServer(ctx, config)
	if err != nil || failure != nil {
		return nil, failure, err
	}

	bucket, failure, err := r.backupBucket(ctx, resolved.backup.Bucket)
	if err != nil || failure != nil {
		return nil, failure, err
	}

	credentials, bucketSecret, failure, err := r.jobCredentials(
		ctx, resolved.cluster, bucket, *config.Spec.BackupCredentialsSecretRef,
	)
	if err != nil || failure != nil {
		return nil, failure, err
	}

	pod, image, failure, err := r.dumpBlock(ctx, resolved.cluster)
	if err != nil || failure != nil {
		return nil, failure, err
	}

	return &databaseResolution{
		config:       config,
		server:       server,
		credentials:  credentials,
		bucket:       bucket,
		bucketSecret: bucketSecret,
		pod:          pod,
		image:        image,
	}, nil, nil
}

// databaseConfig reads the DatabaseConfig that the storage contract names and
// requires the backup user that a restore runs as.
func (r *Reconciler) databaseConfig(
	ctx context.Context,
	storage *v1.SecondaryStorageConfig,
) (*v1.DatabaseConfig, *conditions.PreCheckFailure, error) {
	key := types.NamespacedName{
		Namespace: storage.Namespace,
		Name:      storage.Spec.RDBMS.DatabaseConfigRef,
	}

	var config v1.DatabaseConfig
	if err := r.APIReader.Get(ctx, key, &config); err != nil {
		if apierrors.IsNotFound(err) {
			return nil, logicalbackup.InvalidReference("DatabaseConfig %s does not exist", key), nil
		}

		return nil, nil, fmt.Errorf("reading DatabaseConfig %s: %w", key, err)
	}

	if config.Spec.BackupCredentialsSecretRef == nil {
		return nil, &conditions.PreCheckFailure{
			Reason: v1.ReasonMissingSecret,
			Message: fmt.Sprintf(
				"DatabaseConfig %s has no backupCredentialsSecretRef, which a restore needs", key,
			),
		}, nil
	}

	return &config, nil, nil
}

// databaseServer reads the DatabaseServerConfig of the database and requires
// the major version that its controller probed. The Job runs client tools of
// that major, and a guessed major risks a pg_restore that is older than the
// server.
func (r *Reconciler) databaseServer(
	ctx context.Context,
	config *v1.DatabaseConfig,
) (*v1.DatabaseServerConfig, *conditions.PreCheckFailure, error) {
	key := types.NamespacedName{Name: config.Spec.ServerRef}

	var server v1.DatabaseServerConfig
	if err := r.APIReader.Get(ctx, key, &server); err != nil {
		if apierrors.IsNotFound(err) {
			return nil, logicalbackup.InvalidReference(
				"DatabaseServerConfig %q does not exist", key.Name,
			), nil
		}

		return nil, nil, fmt.Errorf("reading DatabaseServerConfig %q: %w", key.Name, err)
	}

	ready := meta.FindStatusCondition(server.Status.Conditions, v1.ConditionReady)
	probed := server.Status.ServerVersion != "" && ready != nil &&
		ready.Status == metav1.ConditionTrue && ready.ObservedGeneration == server.Generation
	if !probed {
		return nil, logicalbackup.InvalidReference(
			"DatabaseServerConfig %q has not been probed for its current spec: its controller "+
				"publishes status.serverVersion once it reaches the server as declared, and the "+
				"restore needs it to run matching client tools",
			key.Name,
		), nil
	}

	return &server, nil, nil
}

// backupBucket reads the ObjectStorageConfig that the backup wrote its dump
// to. The compatibility check already proved that the target backs up through
// the same contract.
func (r *Reconciler) backupBucket(
	ctx context.Context,
	name string,
) (*v1.ObjectStorageConfig, *conditions.PreCheckFailure, error) {
	if name == "" {
		return nil, logicalbackup.InvalidReference(
			"the backup did not record the ObjectStorageConfig that holds its dump",
		), nil
	}

	var bucket v1.ObjectStorageConfig
	if err := r.APIReader.Get(ctx, types.NamespacedName{Name: name}, &bucket); err != nil {
		if apierrors.IsNotFound(err) {
			return nil, logicalbackup.InvalidReference("ObjectStorageConfig %q does not exist", name), nil
		}

		return nil, nil, fmt.Errorf("reading ObjectStorageConfig %q: %w", name, err)
	}

	return &bucket, nil, nil
}

// jobCredentials locates the two Secrets that the Job mounts, as reachable
// from the namespace of the cluster. It obeys the rule of the CamundaCluster
// controller: a Secret in the namespace of the cluster is used where it is,
// and a Secret anywhere else is read through the local copy that the
// CamundaCluster controller maintains. The bucket name is empty for workload
// identity.
func (r *Reconciler) jobCredentials(
	ctx context.Context,
	cluster *v1.CamundaCluster,
	bucket *v1.ObjectStorageConfig,
	database v1.CredentialsSecretRef,
) (v1.CredentialsSecretRef, string, *conditions.PreCheckFailure, error) {
	local := localSecretName(
		cluster, database.Namespace, database.Name, camundacluster.MirrorPurposeDumpCredentials,
	)
	database.Name, database.Namespace = local, cluster.Namespace
	failure, err := r.checkSecret(
		ctx, cluster.Namespace, local, v1.ReasonMissingSecret, "database",
		database.UsernameKey, database.PasswordKey,
	)
	if err != nil || failure != nil {
		return database, "", failure, err
	}

	credentials := bucket.CredentialsSecret()
	if credentials == nil {
		return database, "", nil, nil
	}

	bucketSecret := localSecretName(
		cluster, credentials.Namespace, credentials.Name, camundacluster.MirrorPurposeBackupCredentials,
	)
	failure, err = r.checkSecret(
		ctx, cluster.Namespace, bucketSecret, v1.ReasonMissingCredentials, "bucket", credentials.Keys...,
	)
	if err != nil || failure != nil {
		return database, "", failure, err
	}

	return database, bucketSecret, nil, nil
}

// checkSecret checks that the Secret at namespace/name carries keys and maps
// a miss to a failure with the given reason. purpose names the credentials in
// the message.
func (r *Reconciler) checkSecret(
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
// namespace of the cluster. A source in that namespace is used where it is,
// and any other source through its purpose-named copy.
func localSecretName(cluster *v1.CamundaCluster, namespace, name, purpose string) string {
	if namespace == cluster.Namespace {
		return name
	}

	return camundacluster.MirroredSecretName(cluster, purpose)
}

// dumpBlock resolves the pod settings and the postgres image of the Job from
// the cluster, through its preset when it names one. Both belong to the owner
// of the cluster: the Job runs under the ServiceAccount of the cluster, so the
// executable and the pod shape stay their choice. A restore resource carries
// no pod block of its own.
func (r *Reconciler) dumpBlock(
	ctx context.Context,
	cluster *v1.CamundaCluster,
) (*v1.DumpPodSpec, string, *conditions.PreCheckFailure, error) {
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

	if merged.Backup == nil || merged.Backup.Dump == nil {
		return nil, "", nil, nil
	}

	return &merged.Backup.Dump.DumpPodSpec, merged.Backup.Dump.PostgresImage, nil, nil
}

// stuckDatabasePod reports the first pod of the pg_restore Job that cannot
// start. It selects the pods by the restore UID that the pod template
// carries, so a leftover pod of another restore of the same name never holds
// this one.
func (r *Reconciler) stuckDatabasePod(
	ctx context.Context,
	restore *v1.LogicalRestore,
) (*conditions.PreCheckFailure, error) {
	return podstate.Stuck(
		ctx,
		r.APIReader,
		restore.Namespace,
		map[string]string{components.RestoreUIDLabel: string(restore.UID)},
		whatSecondary,
	)
}
