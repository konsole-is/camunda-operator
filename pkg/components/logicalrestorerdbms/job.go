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

// Package logicalrestorerdbms renders the Job that restores a logical backup
// into the relational database of a suspended orchestration cluster. The Job is
// the mirror image of the dump Job in pkg/components/logicalbackuprdbms: an
// init container downloads the archive from the backup bucket into a scratch
// volume, then the main container runs pg_restore against the target. The
// package is pure: spec in, resources out, no API calls.
package logicalrestorerdbms

import (
	"encoding/json"
	"fmt"
	"slices"
	"strconv"
	"strings"

	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/validation"

	v1 "github.com/konsole-is/camunda-operator/api/v1"
	backup "github.com/konsole-is/camunda-operator/pkg/components/logicalbackuprdbms"
	"github.com/konsole-is/camunda-operator/pkg/labels"
)

const (
	// ComponentName is the camunda.io/component of the Job that this package
	// renders.
	ComponentName = "pg-restore"
	// RestoreUIDLabel carries the UID of the LogicalRestoreRDBMS that a Job
	// works for. A reconcile checks it before it adopts a Job by name. A
	// leftover Job of a deleted and recreated restore with the same name is
	// therefore never tracked as the Job of this restore.
	RestoreUIDLabel = "camunda.io/logical-restore-rdbms-uid"
	// DefaultActiveDeadlineSeconds is the deadline of a Job whose pod block
	// sets none: 24 hours. It is the default of the dump Job, because a
	// restore takes about as long as the dump it reads. It is not unbounded
	// because a pod that cannot start uses no backoff. Examples are an image
	// that does not pull and a volume that does not bind.
	DefaultActiveDeadlineSeconds = int64(24 * 60 * 60)
	// jobNameSuffix ends every restore Job name.
	jobNameSuffix = "-" + ComponentName
	// scratchVolumeName is the volume that holds the archive between the two
	// containers.
	scratchVolumeName = "scratch"
	// scratchMountPath is where both containers see the scratch volume.
	scratchMountPath = "/scratch"
	// defaultBackoffLimit bounds the pod retries of the Job. It is the limit
	// of the dump Job. A retry is safe: pg_restore --clean --if-exists drops
	// what it restores before it restores it, so a second pod starts from the
	// same state as the first. A restore that fails three times needs a human,
	// not a fourth pod.
	defaultBackoffLimit = int32(3)
	// postgresUID is the uid of the postgres user in the upstream image. It is
	// also the fsGroup of the pod. A storage class commonly hands over a
	// PVC-backed scratch volume root-owned, and the fsGroup lets both
	// containers write to it.
	postgresUID = int64(999)
	// operatorUID is the uid of the distroless nonroot user that the operator
	// image runs as.
	operatorUID = int64(65532)
)

// The environment contract of the download subcommand. The Job renders these
// variables and internal/cli/download reads them. The bucket half of the
// contract is the UPLOAD_* environment of the dump Job, which
// internal/cli/storageenv reads for every subcommand.
const (
	// EnvDownloadFile is the path that the downloaded object is written to.
	EnvDownloadFile = "DOWNLOAD_FILE"
	// EnvDownloadKey is the object key to download.
	EnvDownloadKey = "DOWNLOAD_KEY"
)

// JobInput is everything the Job of one restore renders from. The controller
// resolves it. The builder only shapes it.
type JobInput struct {
	// Restore is the LogicalRestoreRDBMS the Job works for. It gives the Job its
	// namespace and its name.
	Restore *v1.LogicalRestoreRDBMS
	// ClusterName is the target CamundaCluster. The Job runs in the namespace
	// of the restore, which is the namespace of that cluster. The
	// ServiceAccount and the credential copies live there.
	ClusterName string
	// Pod shapes the pod. It is the spec.backup.dump of the target cluster.
	// Nil means defaults. Its extraEnv, extraEnvFrom, resources, scratch
	// volume, scheduling, pod labels, and pod annotations all apply, and they
	// reach both containers. The block is the policy of the cluster owner,
	// and the restore writes into that owner's own database.
	Pod *v1.DumpPodSpec
	// PostgresImage is the image of the restore container. It is the image of
	// the cluster block, or empty for the default postgres:<ServerVersion>.
	PostgresImage string
	// Bucket is the backup bucket contract.
	Bucket *v1.ObjectStorageConfig
	// BucketSecretName is the credentials Secret of the bucket as reachable
	// from the cluster namespace. Empty means workload identity.
	BucketSecretName string
	// DBSecretName is the Secret of the role that owns the target database,
	// which is its application role, reachable the same way. DBUsernameKey
	// and DBPasswordKey name its keys.
	//
	// pg_restore --clean drops every object before it recreates it, and
	// PostgreSQL lets only the owner of an object drop it. The backup role
	// that wrote the archive owns nothing of what it dumped, so a restore
	// that connected as it would fail every DROP with "must be owner of
	// table" and would restore no data.
	DBSecretName  string
	DBUsernameKey string
	DBPasswordKey string
	// ServiceAccountName is the ServiceAccount of the pod. Empty means the
	// default account of the namespace.
	ServiceAccountName string
	// ServerVersion is the major version of the target database server. The
	// restore container runs client tools of that major.
	ServerVersion string
	// Host, Port, and Database locate the logical database to restore into.
	Host     string
	Port     int32
	Database string
	// ObjectKey is the full key of the dump in the bucket, as the backup
	// recorded it.
	ObjectKey string
	// CLIImage is the camunda-operator-cli image. Its download subcommand
	// streams the archive from the bucket. The image ships separately from the
	// manager, and the manager receives it as --camunda-operator-cli-image.
	CLIImage string
}

// JobName returns the name of the Job of one restore. It derives from the
// restore name alone. The Job lives in the restore's own namespace, where that
// name is unique. A reconcile that re-enters after a crash therefore adopts
// the Job that it already created instead of a second one. A restore name can
// be a full DNS subdomain, but a Job name is a DNS label. BoundedName
// therefore truncates a long name deterministically and keeps it unique with a
// hash of the whole name.
func JobName(restore *v1.LogicalRestoreRDBMS) string {
	return labels.BoundedName(restore.Name, validation.DNS1123LabelMaxLength-len(jobNameSuffix)) + jobNameSuffix
}

// JobBelongsTo reports whether job carries the identity of restore, that is
// the UID label that BuildJob stamps. A Job found by name with another UID is
// a leftover of a deleted and recreated restore of the same name, or a foreign
// Job. The reconcile must not adopt or delete it for this restore.
func JobBelongsTo(job *batchv1.Job, restore *v1.LogicalRestoreRDBMS) bool {
	return job.Labels[RestoreUIDLabel] == string(restore.UID)
}

// BuildJob renders the Job that downloads the dump and restores it into the
// target database. An initContainer streams the object into the scratch
// volume. The main container runs pg_restore from that file, so the Job
// succeeds only when the restore succeeded. The two containers run in turn,
// and one resource block sizes both. The effective request of the pod is the
// maximum, not the sum.
//
// The Job carries no owner reference. The caller sets the controller reference
// before it applies the Job.
func BuildJob(in JobInput) (*batchv1.Job, error) {
	if in.CLIImage == "" {
		return nil, fmt.Errorf(
			"building the restore Job of %q: the camunda-operator-cli image is empty", in.Restore.Name,
		)
	}

	pod := in.Pod
	if pod == nil {
		pod = &v1.DumpPodSpec{}
	}

	if pod.ScratchVolume != nil &&
		pod.ScratchVolume.StorageClassName != nil &&
		pod.ScratchVolume.SizeLimit == nil {
		return nil, fmt.Errorf(
			"building the restore Job of %q: a scratch volume with a storage class needs a sizeLimit",
			in.Restore.Name,
		)
	}

	spec, err := json.Marshal(in.Bucket.Spec)
	if err != nil {
		return nil, fmt.Errorf("encoding the bucket spec of %q: %w", in.Bucket.Name, err)
	}

	// Label values are bounded like DNS labels. The owner label carries the
	// bounded name, and the UID label carries the identity that never
	// truncates.
	managed := labels.Managed(
		labels.LogicalRestoreRDBMS(labels.BoundedName(in.Restore.Name, validation.LabelValueMaxLength)),
		ComponentName,
	)
	managed[labels.ClusterKey] = in.ClusterName
	managed[RestoreUIDLabel] = string(in.Restore.UID)

	// The workload-identity pod label is operator-required. Without it, the
	// Azure webhook injects no token, whatever the ServiceAccount carries.
	podManaged := labels.Merge(in.Bucket.WorkloadIdentityPodLabels(), managed)
	podLabels := labels.Merge(pod.PodLabels, podManaged)

	template := corev1.PodTemplateSpec{
		ObjectMeta: metav1.ObjectMeta{
			Labels:      podLabels,
			Annotations: pod.PodAnnotations,
		},
		Spec: corev1.PodSpec{
			RestartPolicy:      corev1.RestartPolicyNever,
			ServiceAccountName: in.ServiceAccountName,
			SecurityContext: &corev1.PodSecurityContext{
				RunAsNonRoot:   new(true),
				FSGroup:        new(postgresUID),
				SeccompProfile: &corev1.SeccompProfile{Type: corev1.SeccompProfileTypeRuntimeDefault},
			},
			InitContainers: []corev1.Container{downloadContainer(in, pod, string(spec))},
			Containers:     []corev1.Container{restoreContainer(in, pod)},
			Volumes:        []corev1.Volume{scratchVolume(pod)},
		},
	}

	if pod.Scheduling != nil {
		template.Spec.Tolerations = pod.Scheduling.Tolerations
		if pod.Scheduling.NodeAffinity != nil || pod.Scheduling.PodAffinity != nil {
			template.Spec.Affinity = &corev1.Affinity{
				NodeAffinity: pod.Scheduling.NodeAffinity,
				PodAffinity:  pod.Scheduling.PodAffinity,
			}
		}
	}

	return &batchv1.Job{
		TypeMeta: metav1.TypeMeta{APIVersion: "batch/v1", Kind: "Job"},
		ObjectMeta: metav1.ObjectMeta{
			Name:      JobName(in.Restore),
			Namespace: in.Restore.Namespace,
			Labels:    managed,
		},
		Spec: batchv1.JobSpec{
			BackoffLimit:          new(defaultBackoffLimit),
			ActiveDeadlineSeconds: activeDeadline(pod),
			Template:              template,
		},
	}, nil
}

// activeDeadline returns the deadline of the pod block, or the production
// default when it sets none.
func activeDeadline(pod *v1.DumpPodSpec) *int64 {
	if pod.ActiveDeadlineSeconds != nil {
		return pod.ActiveDeadlineSeconds
	}

	return new(DefaultActiveDeadlineSeconds)
}

// downloadContainer streams the archive from the bucket into the scratch
// volume through the download subcommand of camunda-operator-cli. It reads the
// same file name that the dump Job wrote, so the two can never disagree.
func downloadContainer(in JobInput, pod *v1.DumpPodSpec, spec string) corev1.Container {
	env := make([]corev1.EnvVar, 0, 6)
	env = append(
		env,
		corev1.EnvVar{Name: EnvDownloadFile, Value: scratchMountPath + "/" + backup.DumpFileName},
		corev1.EnvVar{Name: EnvDownloadKey, Value: in.ObjectKey},
		corev1.EnvVar{Name: backup.EnvUploadStorageName, Value: in.Bucket.Name},
		corev1.EnvVar{Name: backup.EnvUploadStorageSpec, Value: spec},
	)
	env = append(env, credentialEnv(in)...)

	security := containerSecurity(operatorUID)
	// The container writes only into the scratch mount, which is a volume and
	// stays writable with a read-only root. The root filesystem of the CLI
	// image therefore carries the same read-only setting as the upload
	// container of the dump Job.
	security.ReadOnlyRootFilesystem = new(true)

	container := corev1.Container{
		Name:            "download",
		Image:           in.CLIImage,
		Args:            []string{"download"},
		Env:             mergeEnv(env, pod.ExtraEnv),
		EnvFrom:         pod.ExtraEnvFrom,
		SecurityContext: security,
		VolumeMounts:    []corev1.VolumeMount{{Name: scratchVolumeName, MountPath: scratchMountPath}},
	}
	if pod.Resources != nil {
		container.Resources = *pod.Resources
	}

	return container
}

// restoreContainer runs pg_restore of the downloaded archive into the target
// database. The --clean and --if-exists pair drops each object before it
// restores it, so a retried pod starts from the same state as the first one.
// The --no-owner flag restores every object as the connected user, because the
// target cluster does not have to hold the roles of the source.
func restoreContainer(in JobInput, pod *v1.DumpPodSpec) corev1.Container {
	env := []corev1.EnvVar{
		{Name: "PGHOST", Value: in.Host},
		{Name: "PGPORT", Value: strconv.FormatInt(int64(in.Port), 10)},
		{Name: "PGDATABASE", Value: in.Database},
		secretEnv("PGUSER", in.DBSecretName, in.DBUsernameKey),
		secretEnv("PGPASSWORD", in.DBSecretName, in.DBPasswordKey),
	}

	image := in.PostgresImage
	if image == "" {
		image = "postgres:" + in.ServerVersion
	}

	container := corev1.Container{
		Name:  "restore",
		Image: image,
		Command: []string{
			"pg_restore",
			"--clean",
			"--if-exists",
			"--no-owner",
			"--dbname=" + in.Database,
			scratchMountPath + "/" + backup.DumpFileName,
		},
		Env:             mergeEnv(env, pod.ExtraEnv),
		EnvFrom:         pod.ExtraEnvFrom,
		SecurityContext: containerSecurity(postgresUID),
		VolumeMounts:    []corev1.VolumeMount{{Name: scratchVolumeName, MountPath: scratchMountPath}},
	}
	if pod.Resources != nil {
		container.Resources = *pod.Resources
	}

	return container
}

// credentialEnv projects the static credentials of the bucket, key by key in
// the contract's own order. The subcommand rebuilds the Secret data from them
// and hands it to the one mapping in objectstore.CredentialsFrom. Workload
// identity projects nothing. The provider chain then authenticates as the
// ServiceAccount of the pod.
func credentialEnv(in JobInput) []corev1.EnvVar {
	credentials := in.Bucket.CredentialsSecret()
	if in.BucketSecretName == "" || credentials == nil {
		return nil
	}

	env := make([]corev1.EnvVar, 0, len(credentials.Keys)+1)
	env = append(env, corev1.EnvVar{
		Name:  backup.EnvUploadCredentialKeys,
		Value: strings.Join(credentials.Keys, ","),
	})
	for i, key := range credentials.Keys {
		env = append(env, secretEnv(
			backup.EnvUploadCredentialPrefix+strconv.Itoa(i), in.BucketSecretName, key,
		))
	}

	return env
}

// mergeEnv combines the variables that the Job sets with the extras of the pod
// block, by name. The Job's own values always win, and a name appears once. A
// duplicate can therefore neither redirect the restore nor make the apply fail
// on a duplicate list-map key.
func mergeEnv(own, extra []corev1.EnvVar) []corev1.EnvVar {
	merged := make([]corev1.EnvVar, 0, len(own)+len(extra))
	merged = append(merged, own...)
	for _, env := range extra {
		if slices.ContainsFunc(own, func(o corev1.EnvVar) bool { return o.Name == env.Name }) {
			continue
		}
		merged = append(merged, env)
	}

	return merged
}

// scratchVolume returns the volume that holds the archive. It is an emptyDir
// bounded by sizeLimit, or a generic ephemeral PersistentVolumeClaim when a
// storage class is set. The claim lets an archive larger than the node's
// ephemeral storage fit.
func scratchVolume(pod *v1.DumpPodSpec) corev1.Volume {
	scratch := corev1.Volume{Name: scratchVolumeName}

	switch {
	case pod.ScratchVolume != nil && pod.ScratchVolume.StorageClassName != nil:
		scratch.Ephemeral = &corev1.EphemeralVolumeSource{
			VolumeClaimTemplate: &corev1.PersistentVolumeClaimTemplate{
				Spec: corev1.PersistentVolumeClaimSpec{
					AccessModes:      []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce},
					StorageClassName: pod.ScratchVolume.StorageClassName,
					Resources: corev1.VolumeResourceRequirements{
						Requests: corev1.ResourceList{
							corev1.ResourceStorage: *pod.ScratchVolume.SizeLimit,
						},
					},
				},
			},
		}
	case pod.ScratchVolume != nil && pod.ScratchVolume.SizeLimit != nil:
		scratch.EmptyDir = &corev1.EmptyDirVolumeSource{SizeLimit: pod.ScratchVolume.SizeLimit}
	default:
		scratch.EmptyDir = &corev1.EmptyDirVolumeSource{}
	}

	return scratch
}

// containerSecurity is the restricted-profile security context of one
// container that runs as the given non-root uid.
func containerSecurity(uid int64) *corev1.SecurityContext {
	return &corev1.SecurityContext{
		RunAsUser:                new(uid),
		RunAsGroup:               new(uid),
		RunAsNonRoot:             new(true),
		AllowPrivilegeEscalation: new(false),
		Capabilities:             &corev1.Capabilities{Drop: []corev1.Capability{"ALL"}},
	}
}

func secretEnv(name, secret, key string) corev1.EnvVar {
	return corev1.EnvVar{
		Name: name,
		ValueFrom: &corev1.EnvVarSource{
			SecretKeyRef: &corev1.SecretKeySelector{
				LocalObjectReference: corev1.LocalObjectReference{Name: secret},
				Key:                  key,
			},
		},
	}
}
