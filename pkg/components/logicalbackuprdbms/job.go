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

// Package logicalbackuprdbms renders the Job that backs up the logical
// database of a relational orchestration cluster. The Job runs pg_dump into
// a scratch volume, then uploads the archive to the backup bucket. The
// package is pure: spec in, resources out, no API calls.
package logicalbackuprdbms

import (
	"encoding/json"
	"fmt"
	"slices"
	"strconv"
	"strings"

	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/validation"

	v1 "github.com/konsole-is/camunda-operator/api/v1"
	"github.com/konsole-is/camunda-operator/pkg/labels"
	"github.com/konsole-is/camunda-operator/pkg/logicalbackup"
)

const (
	// componentName is the camunda.io/component of everything the backup
	// renders.
	componentName = "dump"
	// scratchVolumeName is the volume that holds the dump between the two
	// containers.
	scratchVolumeName = "scratch"
	// scratchMountPath is where both containers see the scratch volume.
	scratchMountPath = "/scratch"
	// DumpFileName is the file that pg_dump writes and that the upload reads.
	// It is also the last segment of the object key in the bucket.
	// DumpObjectKey builds the whole key.
	DumpFileName = "camunda.dump"
	// defaultBackoffLimit bounds the pod retries of the Job. A dump that
	// fails three times needs a human, not a fourth pod.
	defaultBackoffLimit = int32(3)
	// DefaultActiveDeadlineSeconds is the deadline of a Job whose dump block
	// sets none: 24 hours. The value is large so that a very large dump
	// completes. It is not unbounded because a pod that cannot start uses no
	// backoff. Examples are an image that does not pull and a volume that
	// does not bind. Without a deadline, such a Job stays active as long as
	// the backup lives.
	DefaultActiveDeadlineSeconds = int64(24 * 60 * 60)
	// postgresUID is the uid of the postgres user in the upstream image. It
	// is also the fsGroup of the pod. A storage class commonly hands over a
	// PVC-backed scratch volume root-owned, and the fsGroup lets pg_dump
	// write to it.
	postgresUID = int64(999)
	// operatorUID is the uid of the distroless nonroot user that the
	// operator image runs as.
	operatorUID = int64(65532)
	// BackupUIDLabel carries the UID of the LogicalBackupRDBMS that a Job
	// works for. A reconcile checks it before it adopts a Job by name. A
	// leftover Job of a deleted and recreated backup with the same name is
	// therefore never tracked as the Job of this backup.
	BackupUIDLabel = "camunda.io/logical-backup-rdbms-uid"
	// jobNameSuffix ends every dump Job name.
	jobNameSuffix = "-dump"
)

// reservedEnvPrefixes start the environment variables that a per-backup
// dump block must not supply: everything that libpq reads (PG*) and the
// whole upload contract (UPLOAD_*). The rule is prefix-based and
// case-sensitive like libpq, because a finite list cannot cover libpq.
// PGHOSTADDR redirects the connection even when PGHOST is set. PGSERVICE
// and PGPASSFILE swap the identity. PGOPTIONS injects server options. Any
// PG* name is connection policy. The cluster's own spec.backup.dump is not
// restricted. Its owner sets policy inside their own boundary, and no
// boundary is crossed there.
var reservedEnvPrefixes = []string{"PG", "UPLOAD_"}

// The environment contract of the upload subcommand. The Job renders these
// variables and internal/cli/upload reads them. They are the only interface
// between the two.
const (
	// EnvUploadFile is the path of the file to upload.
	EnvUploadFile = "UPLOAD_FILE"
	// EnvUploadKey is the object key the file is uploaded to.
	EnvUploadKey = "UPLOAD_KEY"
	// EnvUploadStorageName is the name of the ObjectStorageConfig. The
	// subcommand uses it in error messages.
	EnvUploadStorageName = "UPLOAD_STORAGE_NAME"
	// EnvUploadStorageSpec is the ObjectStorageConfigSpec as JSON. The spec
	// of the contract is the wire format, so the Job and the subcommand can
	// never disagree on a field.
	EnvUploadStorageSpec = "UPLOAD_STORAGE_SPEC"
	// EnvUploadCredentialKeys names the Secret keys of the contract's static
	// credentials, comma-separated in the contract's own order. Each listed
	// key arrives as its own EnvUploadCredentialPrefix<index> variable. The
	// subcommand rebuilds the Secret data from the pair, so the one mapping
	// in objectstore.CredentialsFrom serves the upload too. It is unset for
	// workload identity.
	EnvUploadCredentialKeys = "UPLOAD_CREDENTIAL_KEYS"
	// EnvUploadCredentialPrefix prefixes the indexed credential values.
	EnvUploadCredentialPrefix = "UPLOAD_CREDENTIAL_"
)

// JobInput is everything the Job of one backup renders from. The controller
// resolves it. The builder only shapes it.
type JobInput struct {
	// Backup is the LogicalBackupRDBMS the Job works for.
	Backup *v1.LogicalBackupRDBMS
	// ClusterName identifies the backed-up cluster. The Job runs in the
	// namespace of the backup, which is the namespace of the cluster. The
	// ServiceAccount and the credential copies live there.
	ClusterName string
	// Dump shapes the pod. It is the backup's spec.dump when that is set,
	// otherwise the pod settings of the cluster's spec.backup.dump. Nil
	// means defaults.
	Dump *v1.DumpPodSpec
	// BackupOwnsDump reports that Dump is the backup's own spec.dump. Then
	// its extraEnv and extraEnvFrom reach the dump container only. The
	// upload container never takes environment from a backup. Cloud SDKs
	// read endpoint, proxy, and configuration variables from the
	// environment, so a backup author with no access to the bucket must not
	// steer where the dump goes. The block of the cluster reaches every
	// container: it is the policy of the cluster owner.
	BackupOwnsDump bool
	// PostgresImage is the image of the dump container. It is the image of
	// the cluster block, or empty for the default postgres:<ServerVersion>.
	// It never comes from the backup. The Job runs under the cluster's
	// ServiceAccount, so the executable is the choice of the cluster owner.
	PostgresImage string
	// Bucket is the backup bucket contract.
	Bucket *v1.ObjectStorageConfig
	// BucketSecretName is the credentials Secret of the bucket as reachable
	// from the cluster namespace. It is the source Secret itself when that
	// lives there, otherwise its local copy. Empty means workload identity.
	BucketSecretName string
	// DBSecretName is the Secret of the backup user of the database,
	// reachable the same way. DBUsernameKey and DBPasswordKey name its keys.
	DBSecretName  string
	DBUsernameKey string
	DBPasswordKey string
	// ServiceAccountName is the ServiceAccount of the pod. Empty means the
	// default account of the namespace.
	ServiceAccountName string
	// ServerVersion is the major version of the database server. The dump
	// container runs client tools of that major.
	ServerVersion string
	// Host, Port, and Database locate the logical database to dump.
	Host     string
	Port     int32
	Database string
	// ObjectKey is the full key of the dump in the bucket.
	ObjectKey string
	// CLIImage is the camunda-operator-cli image. Its upload subcommand
	// streams the dump to the bucket. The image ships separately from the
	// manager, and the manager receives it as --camunda-operator-cli-image.
	CLIImage string
}

// BuildJob renders the Job that dumps the logical database and uploads the
// archive. An initContainer runs pg_dump into the scratch volume. The main
// container streams the file to the bucket, so the Job succeeds only when
// the upload succeeded. The two containers run in turn, and one resource
// block sizes both. The effective request of the pod is the maximum, not
// the sum.
func BuildJob(in JobInput) (*batchv1.Job, error) {
	if in.CLIImage == "" {
		return nil, fmt.Errorf("building the dump Job of %q: the camunda-operator-cli image is empty", in.Backup.Name)
	}

	dump := in.Dump
	if dump == nil {
		dump = &v1.DumpPodSpec{}
	}

	if dump.ScratchVolume != nil &&
		dump.ScratchVolume.StorageClassName != nil &&
		dump.ScratchVolume.SizeLimit == nil {
		return nil, fmt.Errorf(
			"building the dump Job of %q: a scratch volume with a storage class needs a sizeLimit",
			in.Backup.Name,
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
		labels.LogicalBackupRDBMS(in.Backup.Name),
		componentName,
	)
	managed[labels.ClusterKey] = labels.OwnerName(in.ClusterName)
	managed[BackupUIDLabel] = string(in.Backup.UID)

	// The workload-identity pod label is operator-required. Without it, the
	// Azure webhook injects no token, whatever the ServiceAccount carries.
	podManaged := labels.Merge(in.Bucket.WorkloadIdentityPodLabels(), managed)
	podLabels := labels.Merge(dump.PodLabels, podManaged)

	template := corev1.PodTemplateSpec{
		ObjectMeta: metav1.ObjectMeta{
			Labels:      podLabels,
			Annotations: dump.PodAnnotations,
		},
		Spec: corev1.PodSpec{
			RestartPolicy:      corev1.RestartPolicyNever,
			ServiceAccountName: in.ServiceAccountName,
			SecurityContext: &corev1.PodSecurityContext{
				RunAsNonRoot:   new(true),
				FSGroup:        new(postgresUID),
				SeccompProfile: &corev1.SeccompProfile{Type: corev1.SeccompProfileTypeRuntimeDefault},
			},
			InitContainers: []corev1.Container{dumpContainer(in, dump)},
			Containers:     []corev1.Container{uploadContainer(in, dump, string(spec))},
			Volumes:        []corev1.Volume{scratchVolume(dump)},
		},
	}

	if dump.Scheduling != nil {
		template.Spec.Tolerations = dump.Scheduling.Tolerations
		if dump.Scheduling.NodeAffinity != nil || dump.Scheduling.PodAffinity != nil {
			template.Spec.Affinity = &corev1.Affinity{
				NodeAffinity: dump.Scheduling.NodeAffinity,
				PodAffinity:  dump.Scheduling.PodAffinity,
			}
		}
	}

	return &batchv1.Job{
		TypeMeta: metav1.TypeMeta{APIVersion: "batch/v1", Kind: "Job"},
		ObjectMeta: metav1.ObjectMeta{
			Name:      JobName(in.Backup),
			Namespace: in.Backup.Namespace,
			Labels:    managed,
		},
		Spec: batchv1.JobSpec{
			BackoffLimit:          new(defaultBackoffLimit),
			ActiveDeadlineSeconds: activeDeadline(dump),
			Template:              template,
		},
	}, nil
}

// dumpContainer runs pg_dump of the entire logical database into the scratch
// volume. The archive format (-Fc) restores with pg_restore and compresses
// by default.
func dumpContainer(in JobInput, dump *v1.DumpPodSpec) corev1.Container {
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
		Name:  "dump",
		Image: image,
		Command: []string{
			"pg_dump",
			"--format=custom",
			"--no-password",
			"--file=" + scratchMountPath + "/" + DumpFileName,
		},
		Env:             mergeEnv(env, dump.ExtraEnv),
		EnvFrom:         dump.ExtraEnvFrom,
		SecurityContext: containerSecurity(postgresUID),
		VolumeMounts:    []corev1.VolumeMount{{Name: scratchVolumeName, MountPath: scratchMountPath}},
	}
	if dump.Resources != nil {
		container.Resources = *dump.Resources
	}

	return container
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

// mergeEnv combines the variables that the Job sets with the extras of the
// dump block, by name. The Job's own values always win, and a name appears
// once. A duplicate can therefore neither redirect the dump nor make the
// apply fail on a duplicate list-map key. Admission already rejects reserved
// names. This function is the second layer.
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

// uploadContainer streams the archive to the bucket through the upload
// subcommand of camunda-operator-cli.
func uploadContainer(in JobInput, dump *v1.DumpPodSpec, spec string) corev1.Container {
	env := make([]corev1.EnvVar, 0, 6)
	env = append(
		env,
		corev1.EnvVar{Name: EnvUploadFile, Value: scratchMountPath + "/" + DumpFileName},
		corev1.EnvVar{Name: EnvUploadKey, Value: in.ObjectKey},
		corev1.EnvVar{Name: EnvUploadStorageName, Value: in.Bucket.Name},
		corev1.EnvVar{Name: EnvUploadStorageSpec, Value: spec},
	)
	env = append(env, credentialEnv(in)...)

	security := containerSecurity(operatorUID)
	security.ReadOnlyRootFilesystem = new(true)

	container := corev1.Container{
		Name:            "upload",
		Image:           in.CLIImage,
		Args:            []string{"upload"},
		Env:             env,
		SecurityContext: security,
		VolumeMounts:    []corev1.VolumeMount{{Name: scratchVolumeName, MountPath: scratchMountPath}},
	}
	// Only the environment of the cluster block reaches the CLI. See
	// JobInput.BackupOwnsDump.
	if !in.BackupOwnsDump {
		container.Env = mergeEnv(env, dump.ExtraEnv)
		container.EnvFrom = dump.ExtraEnvFrom
	}
	if dump.Resources != nil {
		container.Resources = *dump.Resources
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
		Name:  EnvUploadCredentialKeys,
		Value: strings.Join(credentials.Keys, ","),
	})
	for i, key := range credentials.Keys {
		env = append(env, secretEnv(
			EnvUploadCredentialPrefix+strconv.Itoa(i), in.BucketSecretName, key,
		))
	}

	return env
}

// scratchVolume returns the volume that holds the dump. It is an emptyDir
// bounded by sizeLimit, or a generic ephemeral PersistentVolumeClaim when a
// storage class is set. The claim lets a dump larger than the node's
// ephemeral storage fit.
func scratchVolume(dump *v1.DumpPodSpec) corev1.Volume {
	scratch := corev1.Volume{Name: scratchVolumeName}

	switch {
	case dump.ScratchVolume != nil && dump.ScratchVolume.StorageClassName != nil:
		scratch.Ephemeral = &corev1.EphemeralVolumeSource{
			VolumeClaimTemplate: &corev1.PersistentVolumeClaimTemplate{
				Spec: corev1.PersistentVolumeClaimSpec{
					AccessModes:      []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce},
					StorageClassName: dump.ScratchVolume.StorageClassName,
					Resources: corev1.VolumeResourceRequirements{
						Requests: corev1.ResourceList{
							corev1.ResourceStorage: *dump.ScratchVolume.SizeLimit,
						},
					},
				},
			},
		}
	case dump.ScratchVolume != nil && dump.ScratchVolume.SizeLimit != nil:
		scratch.EmptyDir = &corev1.EmptyDirVolumeSource{SizeLimit: dump.ScratchVolume.SizeLimit}
	default:
		scratch.EmptyDir = &corev1.EmptyDirVolumeSource{}
	}

	return scratch
}

// JobName returns the name of the Job of one backup. It derives from the
// backup name alone. The Job lives in the backup's own namespace, where that
// name is unique. A reconcile that re-enters after a crash therefore adopts
// the Job that it already created instead of a second one. A backup name can
// be a full DNS subdomain, but a Job name is a DNS label. BoundedName
// therefore truncates a long name deterministically and keeps it unique with
// a hash of the whole name.
func JobName(backup *v1.LogicalBackupRDBMS) string {
	return labels.BoundedName(backup.Name, validation.DNS1123LabelMaxLength-len(jobNameSuffix)) + jobNameSuffix
}

// activeDeadline returns the deadline of the dump block, or the production
// default when it sets none.
func activeDeadline(dump *v1.DumpPodSpec) *int64 {
	if dump.ActiveDeadlineSeconds != nil {
		return dump.ActiveDeadlineSeconds
	}

	return new(DefaultActiveDeadlineSeconds)
}

// DumpObjectKey returns the key of the dump of one backup in the bucket:
// <basePath>/<namespace>/<cluster>/<id>/<uid>/camunda.dump. The backup id
// groups the dump with the Zeebe backup that it pairs with. The UID of the
// backup resource makes the key unique to this backup. Nothing arbitrates
// the id of a dump against the ids of deleted backups, because the Zeebe
// request carries no id. A clock that stepped backwards can therefore
// allocate an id again. A key of the id alone then overwrites the dump that
// a deleted backup left behind, and the finalizer deletes it as its own.
// With the UID in the key, a backup writes only its own object, and the
// finalizer deletes only that object.
func DumpObjectKey(basePath, namespace, cluster string, id int64, uid types.UID) string {
	return logicalbackup.ObjectKeyPrefix(basePath, namespace, cluster, id) + "/" + string(uid) + "/" + DumpFileName
}

// JobBelongsTo reports whether job carries the identity of backup, that is
// the UID label that BuildJob stamps. A Job found by name with another UID
// is a leftover of a deleted and recreated backup of the same name, or a
// foreign Job. The reconcile must not adopt or delete it for this backup.
func JobBelongsTo(job *batchv1.Job, backup *v1.LogicalBackupRDBMS) bool {
	return job.Labels[BackupUIDLabel] == string(backup.UID)
}

// ReservedEnv returns the names in the extraEnv of a per-backup dump block
// that the Job reserves for itself, in the order they appear. A reserved
// name is any name under a reserved prefix. It returns nothing when the
// block is clean. The controller rejects a per-backup block that names one
// at admission, with the names in the message. Callers run it on the
// backup's own spec.dump, never on the cluster's block.
func ReservedEnv(dump *v1.DumpPodSpec) []string {
	if dump == nil {
		return nil
	}
	var reserved []string
	for _, env := range dump.ExtraEnv {
		if isReservedEnv(env.Name) && !slices.Contains(reserved, env.Name) {
			reserved = append(reserved, env.Name)
		}
	}

	return reserved
}

func isReservedEnv(name string) bool {
	for _, prefix := range reservedEnvPrefixes {
		if strings.HasPrefix(name, prefix) {
			return true
		}
	}

	return false
}

// UnsafeEnvFrom returns the extraEnvFrom sources of a per-backup dump block
// whose prefix does not neutralize the keys of the referenced Secret or
// ConfigMap. It describes each source as "source <i>". The writer of the
// referenced object chooses the envFrom keys. Without a safe prefix, a
// source can therefore supply PGHOSTADDR, which libpq prefers over the Job's
// own PGHOST, and redirect the dump with the injected credentials. The CRD
// schema enforces the same rule. This function is the second layer. Callers
// run it on the backup's own spec.dump, never on the cluster's block.
func UnsafeEnvFrom(dump *v1.DumpPodSpec) []string {
	if dump == nil {
		return nil
	}
	var unsafe []string
	for i, source := range dump.ExtraEnvFrom {
		if !SafeEnvFromPrefix(source.Prefix) {
			unsafe = append(unsafe, fmt.Sprintf("source %d (prefix %q)", i, source.Prefix))
		}
	}

	return unsafe
}

// SafeEnvFromPrefix reports whether prefix guarantees that no key of an
// envFrom source can land on a reserved name. A safe prefix is non-empty,
// starts with no reserved prefix, and is no head of one. For example, the
// prefix "P" plus the key "GHOST" spells PGHOST.
func SafeEnvFromPrefix(prefix string) bool {
	if prefix == "" {
		return false
	}
	for _, reserved := range reservedEnvPrefixes {
		if strings.HasPrefix(prefix, reserved) || strings.HasPrefix(reserved, prefix) {
			return false
		}
	}

	return true
}
