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

package elasticsearchcluster

import (
	"testing"

	esv1 "github.com/elastic/cloud-on-k8s/v3/pkg/apis/elasticsearch/v1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	v1 "github.com/konsole-is/camunda-operator/api/v1"
	"github.com/konsole-is/camunda-operator/pkg/esadmin"
	"github.com/konsole-is/camunda-operator/pkg/objectstore"
)

// keystoreData renders the keystore component of storage and returns the data
// of its Secret, or nil when the component renders none.
func keystoreData(t *testing.T, storage *SnapshotStorage) map[string][]byte {
	t.Helper()

	cluster := &v1.ElasticsearchCluster{
		ObjectMeta: metav1.ObjectMeta{Name: "my-cluster", Namespace: "my-ns"},
	}
	comp, err := KeystoreComponent(cluster, storage)
	require.NoError(t, err)

	objects, err := comp.Preview()
	require.NoError(t, err)

	for _, obj := range objects {
		if secret, ok := obj.(*corev1.Secret); ok {
			return secret.Data
		}
	}

	return nil
}

func s3Bucket(auth v1.S3StorageAuth) *v1.ObjectStorageConfig {
	return &v1.ObjectStorageConfig{
		ObjectMeta: metav1.ObjectMeta{Name: "bucket"},
		Spec: v1.ObjectStorageConfigSpec{
			Type: v1.ObjectStorageTypeS3,
			S3:   &v1.S3Storage{BucketName: "camunda-backups", BasePath: "clusters", Region: "eu-west-1", Auth: auth},
		},
	}
}

func gcsBucket(auth v1.GCSStorageAuth) *v1.ObjectStorageConfig {
	return &v1.ObjectStorageConfig{
		ObjectMeta: metav1.ObjectMeta{Name: "bucket"},
		Spec: v1.ObjectStorageConfigSpec{
			Type: v1.ObjectStorageTypeGCS,
			GCS:  &v1.GCSStorage{BucketName: "camunda-backups", BasePath: "clusters", Auth: auth},
		},
	}
}

func azureBucket(auth v1.AzureBlobStorageAuth, endpoint string) *v1.ObjectStorageConfig {
	return &v1.ObjectStorageConfig{
		ObjectMeta: metav1.ObjectMeta{Name: "bucket"},
		Spec: v1.ObjectStorageConfigSpec{
			Type: v1.ObjectStorageTypeAzureBlob,
			AzureBlob: &v1.AzureBlobStorage{
				AccountName: "camundabackups",
				Container:   "backups",
				BasePath:    "clusters",
				Endpoint:    endpoint,
				Auth:        auth,
			},
		},
	}
}

func TestKeystoreHoldsTheAccessKeyPairOfAnS3Bucket(t *testing.T) {
	t.Parallel()

	data := keystoreData(t, &SnapshotStorage{
		Config:      s3Bucket(v1.S3StorageAuth{Type: v1.ObjectStorageAuthTypeCredentials}),
		Credentials: &objectstore.Credentials{AccessKeyID: "id", SecretAccessKey: "secret"},
	})

	assert.Equal(
		t, map[string][]byte{
			"s3.client.default.access_key": []byte("id"),
			"s3.client.default.secret_key": []byte("secret"),
		}, data,
	)
}

func TestKeystoreHoldsTheServiceAccountKeyOfAGCSBucket(t *testing.T) {
	t.Parallel()

	data := keystoreData(t, &SnapshotStorage{
		Config:      gcsBucket(v1.GCSStorageAuth{Type: v1.ObjectStorageAuthTypeCredentials}),
		Credentials: &objectstore.Credentials{ServiceAccountJSON: []byte(`{"type":"service_account"}`)},
	})

	assert.Equal(
		t, map[string][]byte{
			"gcs.client.default.credentials_file": []byte(`{"type":"service_account"}`),
		}, data,
	)
}

func TestKeystoreHoldsTheAccountAndKeyOfAnAzureContainer(t *testing.T) {
	t.Parallel()

	data := keystoreData(t, &SnapshotStorage{
		Config:      azureBucket(v1.AzureBlobStorageAuth{Type: v1.ObjectStorageAuthTypeCredentials}, ""),
		Credentials: &objectstore.Credentials{AccountKey: "account-key"},
	})

	assert.Equal(
		t, map[string][]byte{
			"azure.client.default.account": []byte("camundabackups"),
			"azure.client.default.key":     []byte("account-key"),
		}, data,
	)
}

// The account is the one mandatory setting of an azure client, so an azure
// container needs a keystore even under workload identity. Without it the
// repository plugin fails to find the client at all. The key is what workload
// identity replaces: left out, the Azure SDK takes the identity of the pod.
func TestKeystoreHoldsTheAccountOfAnAzureContainerUnderWorkloadIdentity(t *testing.T) {
	t.Parallel()

	data := keystoreData(t, &SnapshotStorage{
		Config: azureBucket(v1.AzureBlobStorageAuth{Type: v1.ObjectStorageAuthTypeWorkloadIdentity}, ""),
	})

	assert.Equal(
		t, map[string][]byte{
			"azure.client.default.account": []byte("camundabackups"),
		}, data,
	)
}

// S3 and GCS carry the identity of the pod itself, so a workload-identity
// bucket of either type needs no keystore at all.
func TestNoKeystoreForAnS3OrGCSBucketUnderWorkloadIdentity(t *testing.T) {
	t.Parallel()

	s3 := keystoreData(t, &SnapshotStorage{
		Config: s3Bucket(v1.S3StorageAuth{Type: v1.ObjectStorageAuthTypeWorkloadIdentity}),
	})
	assert.Nil(t, s3)

	gcs := keystoreData(t, &SnapshotStorage{
		Config: gcsBucket(v1.GCSStorageAuth{Type: v1.ObjectStorageAuthTypeWorkloadIdentity}),
	})
	assert.Nil(t, gcs)
}

func TestRepositoryConfigOfEachStorageType(t *testing.T) {
	t.Parallel()

	cluster := &v1.ElasticsearchCluster{
		ObjectMeta: metav1.ObjectMeta{Name: "my-cluster", Namespace: "my-ns"},
	}

	s3 := RepositoryConfig(cluster, &SnapshotStorage{
		Config: s3Bucket(v1.S3StorageAuth{Type: v1.ObjectStorageAuthTypeWorkloadIdentity}),
	})
	assert.Equal(
		t, esadmin.RepositoryConfig{
			Type:     esadmin.RepositoryTypeS3,
			Bucket:   "camunda-backups",
			BasePath: "clusters/my-ns/my-cluster",
		}, s3,
	)

	gcs := RepositoryConfig(cluster, &SnapshotStorage{
		Config: gcsBucket(v1.GCSStorageAuth{Type: v1.ObjectStorageAuthTypeWorkloadIdentity}),
	})
	assert.Equal(
		t, esadmin.RepositoryConfig{
			Type:     esadmin.RepositoryTypeGCS,
			Bucket:   "camunda-backups",
			BasePath: "clusters/my-ns/my-cluster",
		}, gcs,
	)

	// The blob container, never the storage account, is what an azure
	// repository addresses.
	azure := RepositoryConfig(cluster, &SnapshotStorage{
		Config: azureBucket(v1.AzureBlobStorageAuth{Type: v1.ObjectStorageAuthTypeWorkloadIdentity}, ""),
	})
	assert.Equal(
		t, esadmin.RepositoryConfig{
			Type:     esadmin.RepositoryTypeAzure,
			Bucket:   "backups",
			BasePath: "clusters/my-ns/my-cluster",
		}, azure,
	)
}

// Elasticsearch addresses an azure account as https://<account>.blob.<suffix>
// and takes only the suffix, so an endpoint that does not reduce to one
// cannot be served. Rejecting it is better than registering a repository
// against the public Azure endpoint of the account, which is a different
// store than the contract names.
func TestValidateSnapshotStorageOnAzureEndpoints(t *testing.T) {
	t.Parallel()

	for name, endpoint := range map[string]string{
		"the public endpoint of the account": "",
		"a sovereign cloud":                  "https://camundabackups.blob.core.chinacloudapi.cn",
		"a trailing slash":                   "https://camundabackups.blob.core.windows.net/",
	} {
		t.Run("accepts "+name, func(t *testing.T) {
			auth := v1.AzureBlobStorageAuth{Type: v1.ObjectStorageAuthTypeWorkloadIdentity}
			assert.NoError(t, ValidateSnapshotStorage(azureBucket(auth, endpoint)))
		})
	}

	for name, endpoint := range map[string]string{
		"a path-addressed emulator": "http://azurite.azurite.svc:10000/devstoreaccount1",
		"a plain host":              "https://blob.example.com",
		"another account":           "https://other.blob.core.windows.net",
		"an explicit port":          "https://camundabackups.blob.core.windows.net:8443",
		"a query string":            "https://camundabackups.blob.core.windows.net?sv=2021-08-06&sig=abc",
		"a fragment":                "https://camundabackups.blob.core.windows.net#frag",
		"credentials in the URL":    "https://user:pass@camundabackups.blob.core.windows.net",
	} {
		t.Run("rejects "+name, func(t *testing.T) {
			auth := v1.AzureBlobStorageAuth{Type: v1.ObjectStorageAuthTypeWorkloadIdentity}
			err := ValidateSnapshotStorage(azureBucket(auth, endpoint))
			require.Error(t, err)
			// The message reaches a status condition, which every reader of
			// the cluster can see. An endpoint carries a shared access
			// signature or a password often enough that none of it may be
			// echoed back. The contract names the endpoint; the message names
			// the contract.
			assert.NotContains(t, err.Error(), endpoint)
			assert.Contains(t, err.Error(), "bucket")
		})
	}
}

// A contract whose declared type and block disagree names no bucket at all.
func TestValidateSnapshotStorageRejectsAMismatchedBlock(t *testing.T) {
	t.Parallel()

	config := &v1.ObjectStorageConfig{
		ObjectMeta: metav1.ObjectMeta{Name: "bucket"},
		Spec:       v1.ObjectStorageConfigSpec{Type: v1.ObjectStorageTypeGCS},
	}

	require.Error(t, ValidateSnapshotStorage(config))
}

// Every other type is accepted whole: the epic proved the s3 path end to end,
// and gcs and azure register through the same call.
func TestValidateSnapshotStorageAcceptsEveryStorageType(t *testing.T) {
	t.Parallel()

	assert.NoError(t, ValidateSnapshotStorage(s3Bucket(v1.S3StorageAuth{
		Type: v1.ObjectStorageAuthTypeWorkloadIdentity,
	})))
	assert.NoError(t, ValidateSnapshotStorage(gcsBucket(v1.GCSStorageAuth{
		Type: v1.ObjectStorageAuthTypeWorkloadIdentity,
	})))
	assert.NoError(t, ValidateSnapshotStorage(azureBucket(v1.AzureBlobStorageAuth{
		Type: v1.ObjectStorageAuthTypeWorkloadIdentity,
	}, "")))
}

// elasticsearchOf renders the elasticsearch component for storage and returns
// the ECK CR it declares.
func elasticsearchOf(t *testing.T, storage *SnapshotStorage) *esv1.Elasticsearch {
	t.Helper()

	cluster, preset := goldenMinimalElasticsearchCluster()
	cluster.Spec.SnapshotStorageRef = "bucket"

	comp, err := ElasticsearchComponent(cluster, MergePreset(cluster.Spec, preset), storage)
	require.NoError(t, err)

	objects, err := comp.Preview()
	require.NoError(t, err)

	for _, obj := range objects {
		if es, ok := obj.(*esv1.Elasticsearch); ok {
			return es
		}
	}

	require.Fail(t, "the component rendered no Elasticsearch")

	return nil
}

// ECK mounts no ServiceAccount token unless the pod template asks for one, and
// both IRSA and Workload Identity Federation read the projected token of the
// pod. Without this the nodes hold the annotated ServiceAccount in name only.
func TestWorkloadIdentityMountsTheServiceAccountToken(t *testing.T) {
	t.Parallel()

	for name, storage := range map[string]*SnapshotStorage{
		"s3":  {Config: s3Bucket(v1.S3StorageAuth{Type: v1.ObjectStorageAuthTypeWorkloadIdentity})},
		"gcs": {Config: gcsBucket(v1.GCSStorageAuth{Type: v1.ObjectStorageAuthTypeWorkloadIdentity})},
	} {
		t.Run(name, func(t *testing.T) {
			pod := elasticsearchOf(t, storage).Spec.NodeSets[0].PodTemplate.Spec

			require.NotNil(t, pod.AutomountServiceAccountToken)
			assert.True(t, *pod.AutomountServiceAccountToken)
		})
	}
}

// A bucket with static credentials authenticates from the keystore, so the
// nodes need no token of their own.
func TestStaticCredentialsMountNoServiceAccountToken(t *testing.T) {
	t.Parallel()

	pod := elasticsearchOf(t, &SnapshotStorage{
		Config:      s3Bucket(v1.S3StorageAuth{Type: v1.ObjectStorageAuthTypeCredentials}),
		Credentials: &objectstore.Credentials{AccessKeyID: "id", SecretAccessKey: "secret"},
	}).Spec.NodeSets[0].PodTemplate.Spec

	assert.Nil(t, pod.AutomountServiceAccountToken)
}

// Azure workload identity is the one that needs more than a ServiceAccount.
// The webhook of Azure injects nothing without the pod label, and
// Elasticsearch reads the federated token only from under its own config
// directory, so the operator projects the token there itself and points the
// Azure SDK at it.
func TestAzureWorkloadIdentityProjectsTheFederatedToken(t *testing.T) {
	t.Parallel()

	es := elasticsearchOf(t, &SnapshotStorage{
		Config: azureBucket(v1.AzureBlobStorageAuth{
			Type:             v1.ObjectStorageAuthTypeWorkloadIdentity,
			WorkloadIdentity: &v1.AzureBlobWorkloadIdentity{ClientID: "00000000-0000-0000-0000-000000000000"},
		}, ""),
	})
	template := es.Spec.NodeSets[0].PodTemplate

	assert.Equal(t, "true", template.Labels["azure.workload.identity/use"])

	require.Len(t, template.Spec.Volumes, 1)
	volume := template.Spec.Volumes[0]
	assert.Equal(t, "azure-identity-token", volume.Name)
	require.NotNil(t, volume.Projected)
	require.Len(t, volume.Projected.Sources, 1)
	token := volume.Projected.Sources[0].ServiceAccountToken
	require.NotNil(t, token)
	assert.Equal(t, "api://AzureADTokenExchange", token.Audience)
	assert.Equal(t, "azure-identity-token", token.Path)

	require.Len(t, template.Spec.Containers, 1)
	container := template.Spec.Containers[0]
	require.Len(t, container.VolumeMounts, 1)
	assert.Equal(t, "azure-identity-token", container.VolumeMounts[0].Name)
	assert.Equal(
		t, "/usr/share/elasticsearch/config/azure/tokens", container.VolumeMounts[0].MountPath,
	)
	assert.Contains(t, container.Env, corev1.EnvVar{
		Name:  "AZURE_FEDERATED_TOKEN_FILE",
		Value: "/usr/share/elasticsearch/config/azure/tokens/azure-identity-token",
	})
}

// An azure container with static credentials reads the account key from the
// keystore, so none of the workload-identity plumbing applies.
func TestAzureCredentialsProjectNoFederatedToken(t *testing.T) {
	t.Parallel()

	template := elasticsearchOf(t, &SnapshotStorage{
		Config:      azureBucket(v1.AzureBlobStorageAuth{Type: v1.ObjectStorageAuthTypeCredentials}, ""),
		Credentials: &objectstore.Credentials{AccountKey: "account-key"},
	}).Spec.NodeSets[0].PodTemplate

	assert.NotContains(t, template.Labels, "azure.workload.identity/use")
	assert.Empty(t, template.Spec.Volumes)
}

// The endpoint suffix of an azure client is node configuration: the keystore
// does not take it, and neither do the repository settings.
func TestAzureSovereignEndpointConfiguresTheNodes(t *testing.T) {
	t.Parallel()

	es := elasticsearchOf(t, &SnapshotStorage{
		Config: azureBucket(
			v1.AzureBlobStorageAuth{Type: v1.ObjectStorageAuthTypeWorkloadIdentity},
			"https://camundabackups.blob.core.chinacloudapi.cn",
		),
	})

	config := es.Spec.NodeSets[0].Config
	require.NotNil(t, config)
	assert.Equal(t, "core.chinacloudapi.cn", config.Data["azure.client.default.endpoint_suffix"])
}

// The public endpoint of the account is what an azure client assumes, so it
// leaves the node configuration alone.
func TestAzurePublicEndpointConfiguresNothing(t *testing.T) {
	t.Parallel()

	es := elasticsearchOf(t, &SnapshotStorage{
		Config: azureBucket(v1.AzureBlobStorageAuth{Type: v1.ObjectStorageAuthTypeWorkloadIdentity}, ""),
	})

	assert.Nil(t, es.Spec.NodeSets[0].Config)
}

// A contract whose declared type and block disagree names no bucket, so every
// renderer must treat it as one with nothing to render. The pre-check rejects
// such a contract before the components are built, but a panic here would
// take the whole manager down if it ever did reach them.
func TestAMismatchedContractRendersNothing(t *testing.T) {
	t.Parallel()

	cluster := &v1.ElasticsearchCluster{
		ObjectMeta: metav1.ObjectMeta{Name: "my-cluster", Namespace: "my-ns"},
	}
	storage := &SnapshotStorage{Config: &v1.ObjectStorageConfig{
		ObjectMeta: metav1.ObjectMeta{Name: "bucket"},
		Spec:       v1.ObjectStorageConfigSpec{Type: v1.ObjectStorageTypeAzureBlob},
	}}

	assert.Nil(t, keystoreData(t, storage))
	assert.Equal(t, esadmin.RepositoryConfig{}, RepositoryConfig(cluster, storage))
	assert.NotNil(t, elasticsearchOf(t, storage))
}
