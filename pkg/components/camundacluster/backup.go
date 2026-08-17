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

package camundacluster

import (
	"fmt"
	"path"
	"strconv"

	corev1 "k8s.io/api/core/v1"

	v1 "github.com/konsole-is/camunda-operator/api/v1"
	"github.com/konsole-is/camunda-operator/pkg/camundaconfig"
)

// The documented defaults of the primary-storage backup scheduler. They are
// the values that the Camunda backup guide recommends, and they apply to a
// relational cluster that names a backup bucket.
const (
	defaultBackupSchedule           = "PT1H"
	defaultBackupCheckpointInterval = "PT15M"
	defaultBackupRetentionWindow    = "P7D"
	defaultBackupCleanupSchedule    = "PT1H"
	// scheduleNone is the schedule value that takes no backups. Continuous
	// mode must not default on with it, and an explicit pairing is rejected.
	scheduleNone = "none"
)

// The literal values of the backup store that come from Camunda, not from
// the user.
const (
	// backupStoreS3, backupStoreGCS, and backupStoreAzure are the values of
	// camunda.data.primary-storage.backup.store.
	backupStoreS3    = "S3"
	backupStoreGCS   = "GCS"
	backupStoreAzure = "AZURE"
	// gcsAuthAuto makes the GCS client read the application default
	// credentials of the runtime. It is the only value that authenticates;
	// the alternative, none, is for an emulator.
	gcsAuthAuto = "auto"
	// azurePublicSuffix builds the blob service endpoint of an account in
	// the public Azure cloud. The azure block needs an endpoint unless a
	// connection string carries one, and the contract has none.
	azurePublicSuffix = ".blob.core.windows.net"
	// gcsKeyVolumeName is the volume that carries a static Google
	// service-account key, and gcsKeyMountPath is where it is mounted. The
	// GCS backup store takes no key as a property, so the key must be a file.
	gcsKeyVolumeName = "gcs-backup-key"
	gcsKeyMountPath  = "/etc/camunda/gcs-backup"
)

// The workload-identity annotations of each cloud, and the pod label that
// the Azure webhook requires. Each one binds a cloud principal to the
// ServiceAccount of the pods.
const (
	// AWSRoleARNAnnotation is the IRSA annotation of a ServiceAccount.
	AWSRoleARNAnnotation = "eks.amazonaws.com/role-arn"
	// GCPServiceAccountAnnotation is the Workload Identity annotation of a
	// ServiceAccount on GKE.
	GCPServiceAccountAnnotation = "iam.gke.io/gcp-service-account"
	// AzureClientIDAnnotation is the Workload Identity annotation of a
	// ServiceAccount on AKS.
	AzureClientIDAnnotation = "azure.workload.identity/client-id"
	// AzureWorkloadIdentityPodLabel opts a pod into the Azure Workload
	// Identity webhook, which injects the projected token. Without it the
	// annotation alone does nothing.
	AzureWorkloadIdentityPodLabel = "azure.workload.identity/use"
)

// BackupBasePath returns the prefix that this cluster writes its backups
// under: the prefix of the contract, then the namespace and the name of the
// cluster. Two clusters that share a bucket therefore never share a prefix,
// which the Zeebe backup store requires. Azure has no prefix of its own; see
// azureEnv.
func BackupBasePath(in Input) string {
	return path.Join(in.Backup.BasePath(), in.Cluster.Namespace, in.Cluster.Name)
}

// DerivedServiceAccountAnnotations returns the workload-identity annotations
// of every bucket that cluster references, keyed by annotation. A bucket that
// carries no identity contributes nothing: the binding then lives on the
// cloud side and names the ServiceAccount itself, as EKS Pod Identity and
// Workload Identity Federation for GKE do.
//
// Two buckets of one storage type that name different identities are an
// error: a pod has one ServiceAccount, so the operator cannot honor both.
func DerivedServiceAccountAnnotations(buckets ...*v1.ObjectStorageConfig) (map[string]string, error) {
	annotations := map[string]string{}
	sources := map[string]string{}

	for _, bucket := range buckets {
		if bucket == nil {
			continue
		}
		key, value := workloadIdentityOf(bucket)
		if key == "" || value == "" {
			continue
		}
		if previous, seen := annotations[key]; seen && previous != value {
			return nil, fmt.Errorf(
				"ObjectStorageConfig %q wants %s=%q but %q wants %q; one cluster has one ServiceAccount",
				bucket.Name, key, value, sources[key], previous,
			)
		}
		annotations[key] = value
		sources[key] = bucket.Name
	}

	if len(annotations) == 0 {
		return nil, nil
	}
	return annotations, nil
}

// workloadIdentityOf returns the annotation key and value of the workload
// identity of bucket, or two empty strings when it authenticates with static
// credentials or carries no identity.
func workloadIdentityOf(bucket *v1.ObjectStorageConfig) (string, string) {
	switch spec := bucket.Spec; {
	case spec.S3 != nil && spec.S3.Auth.WorkloadIdentity != nil:
		return AWSRoleARNAnnotation, spec.S3.Auth.WorkloadIdentity.RoleARN
	case spec.GCS != nil && spec.GCS.Auth.WorkloadIdentity != nil:
		return GCPServiceAccountAnnotation, spec.GCS.Auth.WorkloadIdentity.ServiceAccountEmail
	case spec.AzureBlob != nil && spec.AzureBlob.Auth.WorkloadIdentity != nil:
		return AzureClientIDAnnotation, spec.AzureBlob.Auth.WorkloadIdentity.ClientID
	}

	return "", ""
}

// derivedPodLabels returns the labels that a derived workload identity needs
// on the pods. Only Azure has one: its webhook injects the projected token
// only into a pod that carries the opt-in label.
func derivedPodLabels(in Input) map[string]string {
	if _, ok := in.ServiceAccountAnnotations[AzureClientIDAnnotation]; !ok {
		return nil
	}

	return map[string]string{AzureWorkloadIdentityPodLabel: "true"}
}

// backupEnv is the backup layer of the configuration. Every unified process
// gets the snapshot repository name on the Elasticsearch path, because any of
// them can host the web applications that take the history backups. Only the
// brokers get the store of the referenced bucket, its credentials, and the
// primary-storage scheduler: no other process takes primary-storage backups,
// so no other pod carries the keys. The returned volume and mount hold a
// static Google service-account key, the one credential that the store cannot
// take as a property.
//
// Without a backupStorageRef nothing is rendered, so the store stays NONE and
// the cluster takes no backups.
func backupEnv(in Input, p Process) rendered {
	var r rendered
	if in.Backup == nil {
		return r
	}

	if in.Storage.Type == v1.SecondaryStorageTypeElasticsearch && in.Storage.Elasticsearch != nil {
		r.env = append(r.env, camundaconfig.Var(
			camundaconfig.KeyBackupRepositoryName,
			in.Storage.Elasticsearch.SnapshotRepository,
		))
	}

	if p.Component != ComponentZeebe {
		return r
	}

	switch spec := in.Backup.Spec; {
	case spec.S3 != nil:
		r.env = append(r.env, s3Env(in, spec.S3)...)
	case spec.GCS != nil:
		gcs := gcsEnv(in, spec.GCS)
		r.env = append(r.env, gcs.env...)
		r.volumes, r.mounts = gcs.volumes, gcs.mounts
	case spec.AzureBlob != nil:
		r.env = append(r.env, azureEnv(spec.AzureBlob)...)
	}

	if in.Storage.Type == v1.SecondaryStorageTypeRDBMS {
		r.env = append(r.env, primaryStorageScheduleEnv(in)...)
	}

	return r
}

// s3Env renders the S3 backup store. Without static keys nothing
// authenticates here and the AWS credential chain resolves the identity of
// the pod ServiceAccount. An endpoint marks S3-compatible storage, which also
// needs the checksum variables: several such stores write the chunk framing
// of AWS SDK 2.30 into the object and corrupt the backup manifest.
func s3Env(in Input, s3 *v1.S3Storage) []corev1.EnvVar {
	env := []corev1.EnvVar{
		camundaconfig.Var(camundaconfig.KeyPrimaryBackupStore, backupStoreS3),
		camundaconfig.Var(camundaconfig.KeyPrimaryBackupS3BucketName, s3.BucketName),
		camundaconfig.Var(camundaconfig.KeyPrimaryBackupS3BasePath, BackupBasePath(in)),
	}

	if s3.Region != "" {
		env = append(env, camundaconfig.Var(camundaconfig.KeyPrimaryBackupS3Region, s3.Region))
	}
	if s3.Endpoint != "" {
		env = append(
			env,
			camundaconfig.Var(camundaconfig.KeyPrimaryBackupS3Endpoint, s3.Endpoint),
			corev1.EnvVar{
				Name:  camundaconfig.EnvAWSRequestChecksumCalculation,
				Value: camundaconfig.ChecksumCalculationWhenRequired,
			},
			corev1.EnvVar{
				Name:  camundaconfig.EnvAWSResponseChecksumCalculation,
				Value: camundaconfig.ChecksumCalculationWhenRequired,
			},
		)
	}
	if s3.ForcePathStyle {
		env = append(env, camundaconfig.Var(
			camundaconfig.KeyPrimaryBackupS3ForcePathStyleAccess,
			strconv.FormatBool(true),
		))
	}
	if creds := s3.Auth.Credentials; creds != nil {
		env = append(
			env,
			camundaconfig.VarFrom(
				camundaconfig.KeyPrimaryBackupS3AccessKey,
				secretSource(creds.SecretRef.Name, creds.SecretRef.AccessKeyIDKey),
			),
			camundaconfig.VarFrom(
				camundaconfig.KeyPrimaryBackupS3SecretKey,
				secretSource(creds.SecretRef.Name, creds.SecretRef.SecretAccessKeyKey),
			),
		)
	}

	return env
}

// gcsEnv renders the GCS backup store. The store takes no key as a property:
// its auth is auto or none, and auto reads the application default
// credentials. A static key is therefore mounted as a file and named by
// GOOGLE_APPLICATION_CREDENTIALS, which the default credentials read first.
// Under workload identity the same auto setting picks the identity up from
// the ServiceAccount.
func gcsEnv(in Input, gcs *v1.GCSStorage) rendered {
	r := rendered{env: []corev1.EnvVar{
		camundaconfig.Var(camundaconfig.KeyPrimaryBackupStore, backupStoreGCS),
		camundaconfig.Var(camundaconfig.KeyPrimaryBackupGCSBucketName, gcs.BucketName),
		camundaconfig.Var(camundaconfig.KeyPrimaryBackupGCSBasePath, BackupBasePath(in)),
		camundaconfig.Var(camundaconfig.KeyPrimaryBackupGCSAuth, gcsAuthAuto),
	}}

	creds := gcs.Auth.Credentials
	if creds == nil {
		return r
	}

	r.env = append(r.env, corev1.EnvVar{
		Name:  camundaconfig.EnvGoogleApplicationCredentials,
		Value: gcsKeyMountPath + "/" + creds.SecretRef.Key,
	})
	r.volumes = []corev1.Volume{{
		Name: gcsKeyVolumeName,
		VolumeSource: corev1.VolumeSource{Secret: &corev1.SecretVolumeSource{
			SecretName: creds.SecretRef.Name,
			Items:      []corev1.KeyToPath{{Key: creds.SecretRef.Key, Path: creds.SecretRef.Key}},
		}},
	}}
	r.mounts = []corev1.VolumeMount{{Name: gcsKeyVolumeName, MountPath: gcsKeyMountPath, ReadOnly: true}}

	return r
}

// azureEnv renders the Azure backup store. Its base-path is the container
// name, not a prefix: the azure block carries no container field of its own.
// The contract's own prefix therefore cannot scope this store, so two
// clusters that back up to Azure need two containers, and the pre-check
// rejects a second cluster on one contract. The endpoint is required without
// a connection string, which the contract does not carry, so it is derived
// from the account when unset.
//
// The account name and the account key render together or not at all: Camunda
// rejects a name without a key, and the credential chain of the runtime (the
// workload identity) only takes over when name, key, connection string, and
// SAS token are all absent. Under workload identity the account name of the
// contract therefore only derives the endpoint.
func azureEnv(azure *v1.AzureBlobStorage) []corev1.EnvVar {
	endpoint := azure.Endpoint
	if endpoint == "" {
		endpoint = "https://" + azure.AccountName + azurePublicSuffix
	}

	env := []corev1.EnvVar{
		camundaconfig.Var(camundaconfig.KeyPrimaryBackupStore, backupStoreAzure),
		camundaconfig.Var(camundaconfig.KeyPrimaryBackupAzureEndpoint, endpoint),
		camundaconfig.Var(camundaconfig.KeyPrimaryBackupAzureBasePath, azure.Container),
	}

	if creds := azure.Auth.Credentials; creds != nil {
		env = append(
			env,
			camundaconfig.Var(camundaconfig.KeyPrimaryBackupAzureAccountName, azure.AccountName),
			camundaconfig.VarFrom(
				camundaconfig.KeyPrimaryBackupAzureAccountKey,
				secretSource(creds.SecretRef.Name, creds.SecretRef.Key),
			),
		)
	}

	return env
}

// primaryStorageScheduleEnv renders the backup scheduler of Zeebe. It runs on
// the relational path, where Zeebe backs up its own primary storage and a
// restore pairs those backups with a database dump through the exported
// position. Continuous mode holds the log until it is backed up, so the
// schedule must always accompany it.
func primaryStorageScheduleEnv(in Input) []corev1.EnvVar {
	backup := in.Effective.primaryStorageBackup()

	return []corev1.EnvVar{
		camundaconfig.Var(
			camundaconfig.KeyPrimaryBackupContinuous,
			strconv.FormatBool(backup.continuous),
		),
		camundaconfig.Var(camundaconfig.KeyPrimaryBackupSchedule, backup.schedule),
		camundaconfig.Var(camundaconfig.KeyPrimaryBackupCheckpointInterval, backup.checkpointInterval),
		camundaconfig.Var(camundaconfig.KeyPrimaryBackupRetentionWindow, backup.retentionWindow),
		camundaconfig.Var(camundaconfig.KeyPrimaryBackupRetentionCleanupSchedule, backup.cleanupSchedule),
	}
}
