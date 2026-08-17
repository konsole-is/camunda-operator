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
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	v1 "github.com/konsole-is/camunda-operator/api/v1"
	"github.com/konsole-is/camunda-operator/pkg/camundaconfig"
)

// s3Bucket returns an S3 contract with workload identity, the shape that a
// cloud cluster uses.
// fixtureSnapshotRepository is the repository name of the storage contract
// in the fixtures of this package.
const fixtureSnapshotRepository = "my-cluster"

func s3Bucket() *v1.ObjectStorageConfig {
	return &v1.ObjectStorageConfig{
		ObjectMeta: metav1.ObjectMeta{Name: "my-backup-config"},
		Spec: v1.ObjectStorageConfigSpec{
			Type: v1.ObjectStorageTypeS3,
			S3: &v1.S3Storage{
				BucketName: "camunda-backups",
				BasePath:   "clusters",
				Region:     "eu-west-1",
				Auth: v1.S3StorageAuth{
					Type: v1.ObjectStorageAuthTypeWorkloadIdentity,
					WorkloadIdentity: &v1.S3WorkloadIdentity{
						RoleARN: "arn:aws:iam::123456789012:role/camunda",
					},
				},
			},
		},
	}
}

// minioBucket returns the S3-compatible contract that the kind suite uses:
// an endpoint, path-style addressing, and static keys already pointed at the
// copy in the cluster namespace.
func minioBucket() *v1.ObjectStorageConfig {
	bucket := s3Bucket()
	bucket.Spec.S3.Region = ""
	bucket.Spec.S3.Endpoint = "http://minio.minio.svc:9000"
	bucket.Spec.S3.ForcePathStyle = true
	bucket.Spec.S3.Auth = v1.S3StorageAuth{
		Type: v1.ObjectStorageAuthTypeCredentials,
		Credentials: &v1.S3Credentials{SecretRef: v1.S3CredentialsSecretRef{
			Name:               "my-cluster-camunda-backup-credentials",
			Namespace:          "my-cluster-ns",
			AccessKeyIDKey:     "accessKeyId",
			SecretAccessKeyKey: "secretAccessKey",
		}},
	}
	return bucket
}

func TestBackupEnvIsAbsentWithoutABucket(t *testing.T) {
	in := newInput(t, nil)

	env := render(in, Process{Component: ComponentZeebe}).env

	for _, name := range []string{
		camundaconfig.KeyPrimaryBackupStore.Env(),
		camundaconfig.KeyBackupRepositoryName.Env(),
	} {
		_, ok := envByName(env, name)
		assert.False(t, ok, "%s must not be set without a backupStorageRef", name)
	}
}

func TestBackupEnvRendersTheS3StoreAndTheRepositoryName(t *testing.T) {
	in := newInput(t, func(in *Input) {
		in.Backup = s3Bucket()
		in.Storage.Elasticsearch.SnapshotRepository = fixtureSnapshotRepository
	})

	env := render(in, Process{Component: ComponentZeebe}).env

	assertEnv(t, env, camundaconfig.KeyBackupRepositoryName.Env(), fixtureSnapshotRepository)
	assertEnv(t, env, camundaconfig.KeyPrimaryBackupStore.Env(), "S3")
	assertEnv(t, env, camundaconfig.KeyPrimaryBackupS3BucketName.Env(), "camunda-backups")
	assertEnv(t, env, camundaconfig.KeyPrimaryBackupS3Region.Env(), "eu-west-1")

	// The prefix carries the namespace and the name, so two clusters never
	// share one, which the Zeebe backup store requires.
	assertEnv(t, env, camundaconfig.KeyPrimaryBackupS3BasePath.Env(), "clusters/my-cluster-ns/my-cluster")

	// Workload identity renders no keys at all: the AWS credential chain
	// resolves the identity from the ServiceAccount of the pod.
	for _, name := range []string{
		camundaconfig.KeyPrimaryBackupS3AccessKey.Env(),
		camundaconfig.KeyPrimaryBackupS3SecretKey.Env(),
	} {
		_, ok := envByName(env, name)
		assert.False(t, ok, "%s must not be set under workload identity", name)
	}
}

func TestBackupEnvRendersStaticKeysAndTheChecksumWorkaround(t *testing.T) {
	in := newInput(t, func(in *Input) { in.Backup = minioBucket() })

	env := render(in, Process{Component: ComponentZeebe}).env

	assertEnv(t, env, camundaconfig.KeyPrimaryBackupS3Endpoint.Env(), "http://minio.minio.svc:9000")
	assertEnv(t, env, camundaconfig.KeyPrimaryBackupS3ForcePathStyleAccess.Env(), "true")
	assertSecretEnv(
		t, env,
		camundaconfig.KeyPrimaryBackupS3AccessKey.Env(),
		"my-cluster-camunda-backup-credentials", "accessKeyId",
	)
	assertSecretEnv(
		t, env,
		camundaconfig.KeyPrimaryBackupS3SecretKey.Env(),
		"my-cluster-camunda-backup-credentials", "secretAccessKey",
	)

	// An S3-compatible store needs the chunked-encoding workaround, or the
	// AWS SDK writes its chunk framing into the backup manifest.
	assertEnv(t, env, camundaconfig.EnvAWSRequestChecksumCalculation, camundaconfig.ChecksumCalculationWhenRequired)
	assertEnv(t, env, camundaconfig.EnvAWSResponseChecksumCalculation, camundaconfig.ChecksumCalculationWhenRequired)
}

// A bucket without an endpoint is AWS S3 itself, which handles the newer
// checksums, so the workaround must not be rendered there.
func TestBackupEnvOmitsTheChecksumWorkaroundOnAWS(t *testing.T) {
	in := newInput(t, func(in *Input) { in.Backup = s3Bucket() })

	env := render(in, Process{Component: ComponentZeebe}).env

	_, ok := envByName(env, camundaconfig.EnvAWSRequestChecksumCalculation)
	assert.False(t, ok, "the checksum workaround belongs to S3-compatible stores only")
}

// The GCS store takes no key as a property: auth is auto or none, and auto
// reads the application default credentials. A static key must therefore be a
// file that GOOGLE_APPLICATION_CREDENTIALS names.
func TestBackupEnvMountsTheGoogleKeyAsAFile(t *testing.T) {
	in := newInput(t, func(in *Input) {
		in.Backup = &v1.ObjectStorageConfig{
			ObjectMeta: metav1.ObjectMeta{Name: "my-backup-config"},
			Spec: v1.ObjectStorageConfigSpec{
				Type: v1.ObjectStorageTypeGCS,
				GCS: &v1.GCSStorage{
					BucketName: "camunda-backups",
					BasePath:   "clusters",
					Auth: v1.GCSStorageAuth{
						Type: v1.ObjectStorageAuthTypeCredentials,
						Credentials: &v1.GCSCredentials{SecretRef: v1.SecretKeyRef{
							Name:      "my-cluster-camunda-backup-credentials",
							Namespace: "my-cluster-ns",
							Key:       "key.json",
						}},
					},
				},
			},
		}
	})

	r := render(in, Process{Component: ComponentZeebe})

	assertEnv(t, r.env, camundaconfig.KeyPrimaryBackupStore.Env(), "GCS")
	assertEnv(t, r.env, camundaconfig.KeyPrimaryBackupGCSBucketName.Env(), "camunda-backups")
	assertEnv(t, r.env, camundaconfig.KeyPrimaryBackupGCSBasePath.Env(), "clusters/my-cluster-ns/my-cluster")
	assertEnv(t, r.env, camundaconfig.KeyPrimaryBackupGCSAuth.Env(), "auto")
	assertEnv(t, r.env, camundaconfig.EnvGoogleApplicationCredentials, gcsKeyMountPath+"/key.json")

	require.Len(t, r.volumes, 1)
	assert.Equal(t, gcsKeyVolumeName, r.volumes[0].Name)
	assert.Equal(t, "my-cluster-camunda-backup-credentials", r.volumes[0].Secret.SecretName)
	require.Len(t, r.mounts, 1)
	assert.Equal(t, gcsKeyMountPath, r.mounts[0].MountPath)
	assert.True(t, r.mounts[0].ReadOnly)
}

// The azure block carries no container field: its base-path IS the container
// name. Concatenating the prefix of the contract into it would write the
// backups into a container named after the prefix.
func TestBackupEnvUsesTheAzureBasePathAsTheContainer(t *testing.T) {
	azure := &v1.ObjectStorageConfig{
		ObjectMeta: metav1.ObjectMeta{Name: "my-backup-config"},
		Spec: v1.ObjectStorageConfigSpec{
			Type: v1.ObjectStorageTypeAzureBlob,
			AzureBlob: &v1.AzureBlobStorage{
				AccountName: "camundabackups",
				Container:   "backups",
				BasePath:    "clusters",
			},
		},
	}
	in := newInput(t, func(in *Input) { in.Backup = azure })

	env := render(in, Process{Component: ComponentZeebe}).env

	assertEnv(t, env, camundaconfig.KeyPrimaryBackupStore.Env(), "AZURE")
	assertEnv(t, env, camundaconfig.KeyPrimaryBackupAzureBasePath.Env(), "backups")

	// The endpoint is required without a connection string, and the contract
	// carries none, so it is derived from the account.
	assertEnv(
		t, env,
		camundaconfig.KeyPrimaryBackupAzureEndpoint.Env(),
		"https://camundabackups.blob.core.windows.net",
	)
}

func TestBackupEnvKeepsAnExplicitAzureEndpoint(t *testing.T) {
	azure := &v1.ObjectStorageConfig{
		ObjectMeta: metav1.ObjectMeta{Name: "my-backup-config"},
		Spec: v1.ObjectStorageConfigSpec{
			Type: v1.ObjectStorageTypeAzureBlob,
			AzureBlob: &v1.AzureBlobStorage{
				AccountName: "camundabackups",
				Container:   "backups",
				Endpoint:    "http://azurite:10000/camundabackups",
			},
		},
	}
	in := newInput(t, func(in *Input) { in.Backup = azure })

	env := render(in, Process{Component: ComponentZeebe}).env

	assertEnv(t, env, camundaconfig.KeyPrimaryBackupAzureEndpoint.Env(), "http://azurite:10000/camundabackups")
}

// The backup scheduler is a relational capability: on the Elasticsearch path
// Camunda coordinates a backup through the management API instead.
func TestPrimaryStorageScheduleIsRelationalOnly(t *testing.T) {
	in := newInput(t, func(in *Input) { in.Backup = s3Bucket() })

	env := render(in, Process{Component: ComponentZeebe}).env

	_, ok := envByName(env, camundaconfig.KeyPrimaryBackupContinuous.Env())
	assert.False(t, ok, "the scheduler must not be configured on an Elasticsearch cluster")
}

func TestPrimaryStorageScheduleDefaults(t *testing.T) {
	in := rdbmsBackupInput(t, nil)

	env := render(in, Process{Component: ComponentZeebe}).env

	assertEnv(t, env, camundaconfig.KeyPrimaryBackupContinuous.Env(), "true")
	assertEnv(t, env, camundaconfig.KeyPrimaryBackupSchedule.Env(), "PT1H")
	assertEnv(t, env, camundaconfig.KeyPrimaryBackupCheckpointInterval.Env(), "PT15M")
	assertEnv(t, env, camundaconfig.KeyPrimaryBackupRetentionWindow.Env(), "P7D")
	assertEnv(t, env, camundaconfig.KeyPrimaryBackupRetentionCleanupSchedule.Env(), "PT1H")
}

func TestPrimaryStorageScheduleHonoursTheSpec(t *testing.T) {
	in := rdbmsBackupInput(t, &v1.ClusterBackupSpec{
		PrimaryStorage: &v1.PrimaryStorageBackupSpec{
			Continuous:         new(false),
			Schedule:           "PT10M",
			CheckpointInterval: "PT1M",
			Retention:          &v1.PrimaryStorageRetentionSpec{Window: "P7D", CleanupSchedule: "none"},
		},
	})

	env := render(in, Process{Component: ComponentZeebe}).env

	assertEnv(t, env, camundaconfig.KeyPrimaryBackupContinuous.Env(), "false")
	assertEnv(t, env, camundaconfig.KeyPrimaryBackupSchedule.Env(), "PT10M")
	assertEnv(t, env, camundaconfig.KeyPrimaryBackupCheckpointInterval.Env(), "PT1M")
	assertEnv(t, env, camundaconfig.KeyPrimaryBackupRetentionWindow.Env(), "P7D")
	assertEnv(t, env, camundaconfig.KeyPrimaryBackupRetentionCleanupSchedule.Env(), "none")
}

// rdbmsBackupInput returns a relational cluster that backs up to S3, with an
// optional backup policy.
func rdbmsBackupInput(t *testing.T, backup *v1.ClusterBackupSpec) Input {
	t.Helper()

	return newInput(t, func(in *Input) {
		in.Backup = s3Bucket()
		in.Storage = Storage{
			Type: v1.SecondaryStorageTypeRDBMS,
			RDBMS: &RDBMSStorage{
				Host: "postgres.my-cluster-ns.svc", Port: 5432, Database: "camunda",
				Credentials: v1.CredentialsSecretRef{
					Name: "db-user", Namespace: "my-cluster-ns", UsernameKey: "username", PasswordKey: "password",
				},
			},
		}
		in.Cluster.Spec.Backup = backup
		in.Effective = NewEffective(MergePreset(in.Cluster.Spec, nil))
	})
}

func TestDerivedServiceAccountAnnotationsPerCloud(t *testing.T) {
	gcs := &v1.ObjectStorageConfig{Spec: v1.ObjectStorageConfigSpec{
		Type: v1.ObjectStorageTypeGCS,
		GCS: &v1.GCSStorage{BucketName: "b", Auth: v1.GCSStorageAuth{
			WorkloadIdentity: &v1.GCSWorkloadIdentity{ServiceAccountEmail: "camunda@p.iam.gserviceaccount.com"},
		}},
	}}
	azure := &v1.ObjectStorageConfig{Spec: v1.ObjectStorageConfigSpec{
		Type: v1.ObjectStorageTypeAzureBlob,
		AzureBlob: &v1.AzureBlobStorage{AccountName: "a", Container: "c", Auth: v1.AzureBlobStorageAuth{
			WorkloadIdentity: &v1.AzureBlobWorkloadIdentity{ClientID: "11111111-2222-3333-4444-555555555555"},
		}},
	}}

	cases := []struct {
		name   string
		bucket *v1.ObjectStorageConfig
		want   map[string]string
	}{
		{
			name:   "aws",
			bucket: s3Bucket(),
			want:   map[string]string{v1.IRSARoleARNAnnotation: "arn:aws:iam::123456789012:role/camunda"},
		},
		{
			name:   "gcp",
			bucket: gcs,
			want:   map[string]string{v1.GKEServiceAccountAnnotation: "camunda@p.iam.gserviceaccount.com"},
		},
		{
			name:   "azure",
			bucket: azure,
			want:   map[string]string{v1.AzureClientIDAnnotation: "11111111-2222-3333-4444-555555555555"},
		},
		{
			name:   "static credentials carry no identity",
			bucket: minioBucket(),
			want:   nil,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := DerivedServiceAccountAnnotations(tc.bucket)

			require.NoError(t, err)
			assert.Equal(t, tc.want, got)
		})
	}
}

// An empty workload-identity block is the Pod Identity shape: the binding
// lives on the cloud side and names the ServiceAccount, so the operator adds
// no annotation of its own.
func TestDerivedServiceAccountAnnotationsAreEmptyForPodIdentity(t *testing.T) {
	bucket := s3Bucket()
	bucket.Spec.S3.Auth.WorkloadIdentity = &v1.S3WorkloadIdentity{}

	got, err := DerivedServiceAccountAnnotations(bucket)

	require.NoError(t, err)
	assert.Nil(t, got)
}

func TestDerivedServiceAccountAnnotationsRejectTwoIdentities(t *testing.T) {
	documents := s3Bucket()
	documents.Name = "my-document-config"
	documents.Spec.S3.Auth.WorkloadIdentity.RoleARN = "arn:aws:iam::123456789012:role/other"

	_, err := DerivedServiceAccountAnnotations(s3Bucket(), documents)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "one cluster has one ServiceAccount")
}

func TestDerivedServiceAccountAnnotationsAcceptTheSameIdentityTwice(t *testing.T) {
	documents := s3Bucket()
	documents.Name = "my-document-config"

	got, err := DerivedServiceAccountAnnotations(s3Bucket(), documents)

	require.NoError(t, err)
	assert.Equal(t, map[string]string{v1.IRSARoleARNAnnotation: "arn:aws:iam::123456789012:role/camunda"}, got)
}

// The Azure webhook injects the projected token only into a pod that carries
// the opt-in label, so the annotation alone would leave the identity unused.
// The label must also survive an identity block with no client id: the
// binding then lives on the cloud side, but the webhook still needs the
// opt-in.
func TestAzureWorkloadIdentityAddsThePodLabel(t *testing.T) {
	azure := &v1.ObjectStorageConfig{
		ObjectMeta: metav1.ObjectMeta{Name: "my-backup-config"},
		Spec: v1.ObjectStorageConfigSpec{
			Type: v1.ObjectStorageTypeAzureBlob,
			AzureBlob: &v1.AzureBlobStorage{
				AccountName: "camundabackups",
				Container:   "backups",
				Auth: v1.AzureBlobStorageAuth{
					Type:             v1.ObjectStorageAuthTypeWorkloadIdentity,
					WorkloadIdentity: &v1.AzureBlobWorkloadIdentity{},
				},
			},
		},
	}
	in := newInput(t, func(in *Input) { in.Backup = azure })

	labels := podTemplate(in, Process{Component: ComponentZeebe}).Labels

	assert.Equal(t, "true", labels[v1.AzureWorkloadIdentityUseLabel])
}

func TestOtherCloudsAddNoPodLabel(t *testing.T) {
	in := newInput(t, func(in *Input) { in.Backup = s3Bucket() })

	labels := podTemplate(in, Process{Component: ComponentZeebe}).Labels

	_, ok := labels[v1.AzureWorkloadIdentityUseLabel]
	assert.False(t, ok)
}

func TestServiceAccountCarriesTheDerivedAnnotation(t *testing.T) {
	in := newInput(t, func(in *Input) {
		in.Backup = s3Bucket()
		in.ServiceAccountAnnotations = map[string]string{v1.IRSARoleARNAnnotation: "arn:derived"}
	})

	assert.True(t, rendersServiceAccount(in), "a derived identity needs a ServiceAccount to live on")
	assert.Equal(t, "arn:derived", serviceAccountFor(in).Annotations[v1.IRSARoleARNAnnotation])
}

// An identity block with nothing in it is still an identity: EKS Pod
// Identity and Workload Identity Federation bind the ServiceAccount by name
// on the cloud side, so the account must exist and the pods must reference
// it even though there is nothing to annotate onto it.
func TestEmptyIdentityBlockStillBindsTheServiceAccount(t *testing.T) {
	t.Parallel()

	bucket := s3Bucket()
	bucket.Spec.S3.Auth.WorkloadIdentity = &v1.S3WorkloadIdentity{}
	in := newInput(t, func(in *Input) { in.Backup = bucket })

	assert.True(t, usesServiceAccount(in))
	assert.True(t, rendersServiceAccount(in))

	comps, err := Build(in)
	require.NoError(t, err)
	template := previewedPodTemplate(t, previewObjects(t, comps[0].Component))
	assert.Equal(t, ServiceAccountName(in.Cluster, in.Effective), template.Spec.ServiceAccountName)
}

// Static credentials carry no identity, so without spec.serviceAccount there
// is no ServiceAccount to render or reference.
func TestCredentialsBucketBindsNoServiceAccount(t *testing.T) {
	t.Parallel()

	in := newInput(t, func(in *Input) { in.Backup = minioBucket() })

	assert.False(t, usesServiceAccount(in))
	assert.False(t, rendersServiceAccount(in))
}

// A named ServiceAccount overrides the derived default everywhere it is
// used: the rendered account, the pod reference, and the documented
// principal are one name.
func TestServiceAccountNameIsOverridable(t *testing.T) {
	t.Parallel()

	in := newInput(t, func(in *Input) {
		in.Cluster.Spec.ServiceAccount = &v1.ServiceAccountSpec{Name: "camunda-prod"}
		in.Effective = NewEffective(MergePreset(in.Cluster.Spec, nil))
	})

	assert.Equal(t, "camunda-prod", ServiceAccountName(in.Cluster, in.Effective))
	assert.Equal(t, "camunda-prod", serviceAccountFor(in).Name)

	comps, err := Build(in)
	require.NoError(t, err)
	template := previewedPodTemplate(t, previewObjects(t, comps[0].Component))
	assert.Equal(t, "camunda-prod", template.Spec.ServiceAccountName)
}

// create: false names a pre-existing ServiceAccount: the pods reference it,
// and the operator renders nothing it would own and delete with the cluster.
func TestForeignServiceAccountIsReferencedNotRendered(t *testing.T) {
	t.Parallel()

	in := newInput(t, func(in *Input) {
		in.Cluster.Spec.ServiceAccount = &v1.ServiceAccountSpec{
			Name:   "platform-sa",
			Create: new(false),
		}
		in.Effective = NewEffective(MergePreset(in.Cluster.Spec, nil))
	})

	assert.True(t, usesServiceAccount(in))
	assert.False(t, rendersServiceAccount(in))

	comps, err := Build(in)
	require.NoError(t, err)
	template := previewedPodTemplate(t, previewObjects(t, comps[0].Component))
	assert.Equal(t, "platform-sa", template.Spec.ServiceAccountName)
}

// A user who writes the annotation by hand has the last word: the operator
// must not overwrite an explicit value with the one it derived.
func TestUserAnnotationsWinOverTheDerivedOne(t *testing.T) {
	in := newInput(t, func(in *Input) {
		in.ServiceAccountAnnotations = map[string]string{v1.IRSARoleARNAnnotation: "arn:derived"}
		in.Cluster.Spec.ServiceAccount = &v1.ServiceAccountSpec{
			Annotations: map[string]string{v1.IRSARoleARNAnnotation: "arn:explicit", "extra": "kept"},
		}
		in.Effective = NewEffective(MergePreset(in.Cluster.Spec, nil))
	})

	annotations := serviceAccountFor(in).Annotations

	assert.Equal(t, "arn:explicit", annotations[v1.IRSARoleARNAnnotation])
	assert.Equal(t, "kept", annotations["extra"])
}

// The derived identity is worthless unless the pods run under the annotated
// ServiceAccount: the annotation binds the principal, the pod reference uses
// it. This covers every cloud, because the binding mechanism is the same.
func TestDerivedIdentityBindsThePods(t *testing.T) {
	t.Parallel()

	in := newInput(t, func(in *Input) {
		in.Backup = s3Bucket()
		in.ServiceAccountAnnotations = map[string]string{v1.IRSARoleARNAnnotation: "arn:derived"}
	})

	comps, err := Build(in)
	require.NoError(t, err)

	for _, pc := range comps {
		if !pc.Process.Enabled {
			continue
		}
		template := previewedPodTemplate(t, previewObjects(t, pc.Component))
		assert.Equal(
			t,
			ServiceAccountName(in.Cluster, in.Effective),
			template.Spec.ServiceAccountName,
			pc.Process.Component,
		)
	}
}

// Camunda rejects an account name without an account key, and only falls back
// to the credential chain of the runtime when name, key, connection string,
// and SAS token are all absent. Workload identity must therefore render no
// account fields at all; the endpoint alone addresses the store.
func TestAzureWorkloadIdentityRendersNoAccountName(t *testing.T) {
	t.Parallel()

	azure := &v1.ObjectStorageConfig{
		ObjectMeta: metav1.ObjectMeta{Name: "my-backup-config"},
		Spec: v1.ObjectStorageConfigSpec{
			Type: v1.ObjectStorageTypeAzureBlob,
			AzureBlob: &v1.AzureBlobStorage{
				AccountName: "camundabackups",
				Container:   "backups",
				Auth: v1.AzureBlobStorageAuth{
					Type:             v1.ObjectStorageAuthTypeWorkloadIdentity,
					WorkloadIdentity: &v1.AzureBlobWorkloadIdentity{ClientID: "client"},
				},
			},
		},
	}
	in := newInput(t, func(in *Input) { in.Backup = azure })

	env := render(in, Process{Component: ComponentZeebe}).env

	_, ok := envByName(env, camundaconfig.KeyPrimaryBackupAzureAccountName.Env())
	assert.False(t, ok, "an account name without an account key crash-loops the broker")
	_, ok = envByName(env, camundaconfig.KeyPrimaryBackupAzureAccountKey.Env())
	assert.False(t, ok)

	// The endpoint is still required: the account name of the contract
	// derives it, it is just never rendered as a credential.
	assertEnv(
		t, env,
		camundaconfig.KeyPrimaryBackupAzureEndpoint.Env(),
		"https://camundabackups.blob.core.windows.net",
	)
}

func TestAzureCredentialsRenderTheAccountPair(t *testing.T) {
	t.Parallel()

	azure := &v1.ObjectStorageConfig{
		ObjectMeta: metav1.ObjectMeta{Name: "my-backup-config"},
		Spec: v1.ObjectStorageConfigSpec{
			Type: v1.ObjectStorageTypeAzureBlob,
			AzureBlob: &v1.AzureBlobStorage{
				AccountName: "camundabackups",
				Container:   "backups",
				Auth: v1.AzureBlobStorageAuth{
					Type: v1.ObjectStorageAuthTypeCredentials,
					Credentials: &v1.AzureBlobCredentials{SecretRef: v1.SecretKeyRef{
						Name: "azure-key", Namespace: "my-cluster-ns", Key: "accountKey",
					}},
				},
			},
		},
	}
	in := newInput(t, func(in *Input) { in.Backup = azure })

	env := render(in, Process{Component: ComponentZeebe}).env

	assertEnv(t, env, camundaconfig.KeyPrimaryBackupAzureAccountName.Env(), "camundabackups")
	assertSecretEnv(t, env, camundaconfig.KeyPrimaryBackupAzureAccountKey.Env(), "azure-key", "accountKey")
}

// Only the brokers take primary-storage backups, so only they get the store
// and its credentials. The repository name configures the history backups of
// the web applications, which any unified process can host, so it stays on
// all of them.
func TestBackupStoreEnvIsBrokerOnly(t *testing.T) {
	t.Parallel()

	in := newInput(t, func(in *Input) {
		in.Backup = minioBucket()
		in.Storage.Elasticsearch.SnapshotRepository = fixtureSnapshotRepository
	})

	gateway := render(in, Process{Component: ComponentGateway, Kind: ProcessDeployment})
	assertEnv(t, gateway.env, camundaconfig.KeyBackupRepositoryName.Env(), fixtureSnapshotRepository)
	for _, name := range []string{
		camundaconfig.KeyPrimaryBackupStore.Env(),
		camundaconfig.KeyPrimaryBackupS3AccessKey.Env(),
		camundaconfig.EnvAWSRequestChecksumCalculation,
	} {
		_, ok := envByName(gateway.env, name)
		assert.False(t, ok, "%s belongs to the brokers only", name)
	}

	zeebe := render(in, Process{Component: ComponentZeebe})
	assertEnv(t, zeebe.env, camundaconfig.KeyPrimaryBackupStore.Env(), "S3")
}

// The Google key file follows the store: only the broker pods mount it.
func TestGoogleKeyMountIsBrokerOnly(t *testing.T) {
	t.Parallel()

	in := newInput(t, func(in *Input) {
		in.Backup = &v1.ObjectStorageConfig{
			ObjectMeta: metav1.ObjectMeta{Name: "my-backup-config"},
			Spec: v1.ObjectStorageConfigSpec{
				Type: v1.ObjectStorageTypeGCS,
				GCS: &v1.GCSStorage{
					BucketName: "camunda-backups",
					Auth: v1.GCSStorageAuth{
						Type: v1.ObjectStorageAuthTypeCredentials,
						Credentials: &v1.GCSCredentials{SecretRef: v1.SecretKeyRef{
							Name: "gcs-key", Namespace: "my-cluster-ns", Key: "key.json",
						}},
					},
				},
			},
		}
	})

	gateway := render(in, Process{Component: ComponentGateway, Kind: ProcessDeployment})
	assert.Empty(t, gateway.volumes)
	assert.Empty(t, gateway.mounts)

	zeebe := render(in, Process{Component: ComponentZeebe})
	require.Len(t, zeebe.volumes, 1)
	assert.Equal(t, gcsKeyVolumeName, zeebe.volumes[0].Name)
}

// Continuous mode holds the log until a backup runs. A schedule of none takes
// no backups, so defaulting continuous to true there would fill the disks of
// a cluster that asked for nothing.
func TestContinuousDefaultFollowsTheSchedule(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name   string
		backup *v1.ClusterBackupSpec
		want   bool
	}{
		{name: "defaults on with the default schedule", backup: nil, want: true},
		{
			name: "defaults on with an explicit schedule",
			backup: &v1.ClusterBackupSpec{
				PrimaryStorage: &v1.PrimaryStorageBackupSpec{Schedule: "PT30M"},
			},
			want: true,
		},
		{
			name: "defaults off when the schedule is none",
			backup: &v1.ClusterBackupSpec{
				PrimaryStorage: &v1.PrimaryStorageBackupSpec{Schedule: "none"},
			},
			want: false,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			in := rdbmsBackupInput(t, tc.backup)

			env := render(in, Process{Component: ComponentZeebe}).env

			assertEnv(t, env, camundaconfig.KeyPrimaryBackupContinuous.Env(), strconv.FormatBool(tc.want))
		})
	}
}

// An explicit continuous with no schedule is a contradiction the user must
// resolve, not a state the operator quietly runs into a full disk.
func TestValidateMergedRejectsContinuousWithoutASchedule(t *testing.T) {
	t.Parallel()

	spec := v1.CamundaClusterSpec{
		Version: "8.9.9",
		Backup: &v1.ClusterBackupSpec{
			PrimaryStorage: &v1.PrimaryStorageBackupSpec{
				Continuous: new(true),
				Schedule:   "none",
			},
		},
	}

	err := ValidateMerged(spec)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "continuous")
	assert.Contains(t, err.Error(), "schedule")
}
