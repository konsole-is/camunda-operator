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
// database of a relational orchestration cluster: a pg_dump into a scratch
// volume, then an upload of the archive to the backup bucket. The package is
// pure: spec in, resources out, no API calls.
package logicalbackuprdbms

import (
	"crypto/sha256"
	"encoding/hex"
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
	"github.com/konsole-is/camunda-operator/pkg/labels"
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
	// DumpFileName is the file the dump is written to and uploaded from, and
	// the last segment of the object key in the bucket.
	DumpFileName = "camunda.dump"
	// defaultBackoffLimit bounds the pod retries of the Job. A dump that
	// fails three times needs a human, not a fourth pod.
	defaultBackoffLimit = int32(3)
	// postgresUID is the uid of the postgres user in the upstream image.
	postgresUID = int64(999)
	// operatorUID is the uid of the distroless nonroot user that the
	// operator image runs as.
	operatorUID = int64(65532)
	// BackupUIDLabel carries the UID of the LogicalBackupRDBMS a Job works
	// for. The Job lives in the cluster namespace, where the backup name
	// alone is not unique across backup namespaces; the UID is what a
	// reconcile checks before it adopts a Job by name.
	BackupUIDLabel = "camunda.io/logical-backup-rdbms-uid"
	// jobNameSuffix ends every dump Job name.
	jobNameSuffix = "-dump"
	// jobNameHashLength is the hex length of the hash that keeps a long
	// name unique once it is truncated.
	jobNameHashLength = 10
)

// reservedEnvNames are the environment variables the Job sets to reach the
// right database with the right identity. A dump block may not override
// them: a user-set PGHOST or PGPASSWORD would redirect the dump or run it as
// someone else. Everything else libpq reads, for example PGSSLMODE, stays
// open to extraEnv.
var reservedEnvNames = []string{
	"PGHOST", "PGHOSTADDR", "PGPORT", "PGDATABASE", "PGUSER", "PGPASSWORD",
	"PGPASSFILE", "PGSERVICE", "PGSERVICEFILE",
}

// reservedEnvPrefix is the prefix of the upload contract; every variable
// under it belongs to the operator.
const reservedEnvPrefix = "UPLOAD_"

// The environment contract of the upload subcommand. The Job renders these
// variables and cmd/upload reads them; they are the only interface between
// the two.
const (
	// EnvUploadFile is the path of the file to upload.
	EnvUploadFile = "UPLOAD_FILE"
	// EnvUploadKey is the object key the file is uploaded to.
	EnvUploadKey = "UPLOAD_KEY"
	// EnvUploadStorageName is the name of the ObjectStorageConfig, used in
	// error messages.
	EnvUploadStorageName = "UPLOAD_STORAGE_NAME"
	// EnvUploadStorageSpec is the ObjectStorageConfigSpec as JSON. The spec
	// of the contract is the wire format, so the Job and the subcommand can
	// never disagree on a field.
	EnvUploadStorageSpec = "UPLOAD_STORAGE_SPEC"
	// EnvUploadCredentialKeys names the Secret keys of the contract's static
	// credentials, comma-separated in the contract's own order. Each listed
	// key arrives as its own EnvUploadCredentialPrefix<index> variable, and
	// the subcommand rebuilds the Secret data from the pair — so the one
	// mapping in objectstore.CredentialsFrom serves the upload too. Unset
	// for workload identity.
	EnvUploadCredentialKeys = "UPLOAD_CREDENTIAL_KEYS"
	// EnvUploadCredentialPrefix prefixes the indexed credential values.
	EnvUploadCredentialPrefix = "UPLOAD_CREDENTIAL_"
)

// JobInput is everything the Job of one backup renders from. The controller
// resolves it; the builder only shapes it.
type JobInput struct {
	// Backup is the LogicalBackupRDBMS the Job works for.
	Backup *v1.LogicalBackupRDBMS
	// ClusterName and ClusterNamespace identify the backed-up cluster. The
	// Job runs in the cluster namespace: the ServiceAccount and the
	// credential copies live there.
	ClusterName      string
	ClusterNamespace string
	// Dump is the effective dump block: the cluster's spec.backup.dump, or
	// the backup's spec.dump replacing it as a whole. Nil means defaults.
	Dump *v1.BackupDumpSpec
	// Bucket is the backup bucket contract.
	Bucket *v1.ObjectStorageConfig
	// BucketSecretName is the credentials Secret of the bucket as reachable
	// from the cluster namespace: the source Secret itself when it lives
	// there, or its local copy otherwise. Empty means workload identity.
	BucketSecretName string
	// DBSecretName is the backup user of the database, reachable the same
	// way, with DBUsernameKey and DBPasswordKey naming its keys.
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
	// CLIImage is the camunda-operator-cli image, whose upload subcommand
	// streams the dump to the bucket. It is shipped separately from the
	// manager; the manager receives it as --camunda-operator-cli-image.
	CLIImage string
}

// JobName returns the name of the Job of one backup. It derives from the
// backup namespace and name, so a reconcile that re-enters after a crash
// adopts the Job it already created instead of creating a second one, and
// two backups of one cluster from different namespaces never share a Job.
// The name is a DNS label: a long namespace/name pair is truncated and kept
// unique by a hash of the pair.
func JobName(backup *v1.LogicalBackupRDBMS) string {
	base := backup.Namespace + "-" + backup.Name
	limit := validation.DNS1123LabelMaxLength - len(jobNameSuffix)
	if len(base) > limit {
		sum := sha256.Sum256([]byte(backup.Namespace + "/" + backup.Name))
		hash := hex.EncodeToString(sum[:])[:jobNameHashLength]
		base = strings.TrimRight(base[:limit-1-jobNameHashLength], "-.") + "-" + hash
	}

	return base + jobNameSuffix
}

// JobBelongsTo reports whether job carries the identity of backup: the UID
// label BuildJob stamps. A Job found by name without it, or with another UID,
// belongs to another backup and must not be adopted or deleted for this one.
func JobBelongsTo(job *batchv1.Job, backup *v1.LogicalBackupRDBMS) bool {
	return job.Labels[BackupUIDLabel] == string(backup.UID)
}

// ReservedEnv returns the names in dump.extraEnv that the Job reserves for
// itself, in the order they appear, or nothing when the block is clean. The
// controller rejects a block that names one at admission, with the names in
// the message.
func ReservedEnv(dump *v1.BackupDumpSpec) []string {
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
	return slices.Contains(reservedEnvNames, name) || strings.HasPrefix(name, reservedEnvPrefix)
}

// BuildJob renders the Job that dumps the logical database and uploads the
// archive. An initContainer runs pg_dump into the scratch volume; the main
// container streams the file to the bucket, so the Job succeeds only when
// the upload did. The two containers run in turn, and one resource block
// sizes both: the pod's effective request is the maximum, not the sum.
func BuildJob(in JobInput) (*batchv1.Job, error) {
	if in.CLIImage == "" {
		return nil, fmt.Errorf("building the dump Job of %q: the camunda-operator-cli image is empty", in.Backup.Name)
	}

	dump := in.Dump
	if dump == nil {
		dump = &v1.BackupDumpSpec{}
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

	managed := labels.Managed(labels.LogicalBackupRDBMS(in.Backup.Name), componentName)
	managed[labels.ClusterKey] = in.ClusterName
	managed[BackupUIDLabel] = string(in.Backup.UID)

	// The workload-identity pod label is operator-required: without it the
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
		// Server-Side Apply requires the type to be stated on the patch.
		TypeMeta: metav1.TypeMeta{APIVersion: "batch/v1", Kind: "Job"},
		ObjectMeta: metav1.ObjectMeta{
			Name:      JobName(in.Backup),
			Namespace: in.ClusterNamespace,
			Labels:    managed,
		},
		Spec: batchv1.JobSpec{
			BackoffLimit:          new(defaultBackoffLimit),
			ActiveDeadlineSeconds: dump.ActiveDeadlineSeconds,
			Template:              template,
		},
	}, nil
}

// dumpContainer runs pg_dump of the entire logical database into the scratch
// volume. The archive format (-Fc) restores with pg_restore and compresses
// by default.
func dumpContainer(in JobInput, dump *v1.BackupDumpSpec) corev1.Container {
	env := []corev1.EnvVar{
		{Name: "PGHOST", Value: in.Host},
		{Name: "PGPORT", Value: strconv.FormatInt(int64(in.Port), 10)},
		{Name: "PGDATABASE", Value: in.Database},
		secretEnv("PGUSER", in.DBSecretName, in.DBUsernameKey),
		secretEnv("PGPASSWORD", in.DBSecretName, in.DBPasswordKey),
	}

	image := dump.PostgresImage
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

// uploadContainer streams the archive to the bucket through the upload
// subcommand of camunda-operator-cli.
func uploadContainer(in JobInput, dump *v1.BackupDumpSpec, spec string) corev1.Container {
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
		Env:             mergeEnv(env, dump.ExtraEnv),
		EnvFrom:         dump.ExtraEnvFrom,
		SecurityContext: security,
		VolumeMounts:    []corev1.VolumeMount{{Name: scratchVolumeName, MountPath: scratchMountPath}},
	}
	if dump.Resources != nil {
		container.Resources = *dump.Resources
	}

	return container
}

// credentialEnv projects the static credentials of the bucket, key by key in
// the contract's own order, so the subcommand rebuilds the Secret data and
// hands it to the one mapping in objectstore.CredentialsFrom. Workload
// identity projects nothing: the provider chain authenticates as the
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

// mergeEnv combines the variables the Job sets with the extras of the dump
// block, by name: the Job's own values always win, and a name appears once,
// so a duplicate can neither redirect the dump nor make the apply fail on a
// duplicate list-map key. Admission already rejects reserved names; this is
// the second layer.
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

// scratchVolume returns the volume that holds the dump: an emptyDir bounded
// by sizeLimit, or a generic ephemeral PersistentVolumeClaim when a storage
// class is set, so a dump larger than the node's ephemeral storage still
// fits.
func scratchVolume(dump *v1.BackupDumpSpec) corev1.Volume {
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

// containerSecurity is the restricted-profile security context of one
// container, running as the given non-root uid.
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
