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
	"fmt"
	"net/url"
	"strings"

	commonv1 "github.com/elastic/cloud-on-k8s/v3/pkg/apis/common/v1"
	esv1 "github.com/elastic/cloud-on-k8s/v3/pkg/apis/elasticsearch/v1"
	"github.com/sourcehawk/operator-component-framework/pkg/component"
	"github.com/sourcehawk/operator-component-framework/pkg/feature"
	"github.com/sourcehawk/operator-component-framework/pkg/mutation/editors"
	"github.com/sourcehawk/operator-component-framework/pkg/primitives/secret"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	v1 "github.com/konsole-is/camunda-operator/api/v1"
	"github.com/konsole-is/camunda-operator/pkg/esadmin"
	"github.com/konsole-is/camunda-operator/pkg/labels"
	"github.com/konsole-is/camunda-operator/pkg/logicalbackup"
	"github.com/konsole-is/camunda-operator/pkg/objectstore"
	"github.com/konsole-is/camunda-operator/pkg/wrappers/eckelasticsearch"
)

// keystoreSecretSuffix appended to the CR name yields the name of the Secret
// that holds the bucket credentials of the node keystore.
const keystoreSecretSuffix = "-es-snapshot-keystore"

// The keystore entries that each repository client reads its credentials
// from. Elasticsearch accepts them nowhere else: the settings of a repository
// never carry credentials. The client name is shared with the repository
// settings through esadmin, so the two can never drift apart.
var (
	accessKeyKeystorePath       = "s3.client." + esadmin.DefaultClientName + ".access_key"
	secretKeyKeystorePath       = "s3.client." + esadmin.DefaultClientName + ".secret_key"
	credentialsFileKeystorePath = "gcs.client." + esadmin.DefaultClientName + ".credentials_file"
	accountKeystorePath         = "azure.client." + esadmin.DefaultClientName + ".account"
	accountKeyKeystorePath      = "azure.client." + esadmin.DefaultClientName + ".key"
)

// defaultAzureEndpointSuffix is the endpoint suffix that an azure client
// assumes, so a contract that names the public Azure endpoint needs no node
// configuration at all.
const defaultAzureEndpointSuffix = "core.windows.net"

// SnapshotStorage is the resolved snapshot bucket of a cluster: the contract
// that spec.snapshotStorageRef names, and the keys read from its Secret when
// it holds static credentials. A nil SnapshotStorage means the spec names no
// bucket, so the cluster takes no part in backups.
type SnapshotStorage struct {
	// Config is the referenced contract.
	Config *v1.ObjectStorageConfig
	// Credentials are the static keys of the bucket, or nil when the contract
	// uses workload identity.
	Credentials *objectstore.Credentials
}

// identityAnnotations returns the ServiceAccount annotations that bind the
// identity of the bucket, or nil when there is no bucket or no identity.
func (s *SnapshotStorage) identityAnnotations() map[string]string {
	if s == nil {
		return nil
	}

	return s.Config.WorkloadIdentityAnnotations()
}

// workloadIdentity reports whether the nodes authenticate against the bucket
// as their ServiceAccount. It is true for every workload-identity bucket,
// annotation or not: EKS Pod Identity and Workload Identity Federation bind
// the ServiceAccount by name on the cloud side, so the account must exist and
// be referenced even when there is nothing to annotate onto it.
func (s *SnapshotStorage) workloadIdentity() bool {
	return s != nil && s.Config != nil && s.Credentials == nil
}

// repository reports whether the cluster has a snapshot repository to
// register and to publish.
func (s *SnapshotStorage) repository() bool {
	return s != nil && s.Config != nil
}

// storageType returns the storage API of the bucket, or the empty string when
// there is no bucket. A contract whose declared type and block disagree also
// yields the empty string: it names no bucket, and every switch below is safe
// to read the block of whatever type this returns. The pre-check rejects such
// a contract long before the components are built.
func (s *SnapshotStorage) storageType() v1.ObjectStorageType {
	if s == nil || s.Config == nil {
		return ""
	}

	spec := s.Config.Spec
	switch spec.Type {
	case v1.ObjectStorageTypeS3:
		if spec.S3 == nil {
			return ""
		}

	case v1.ObjectStorageTypeGCS:
		if spec.GCS == nil {
			return ""
		}

	case v1.ObjectStorageTypeAzureBlob:
		if spec.AzureBlob == nil {
			return ""
		}
	}

	return spec.Type
}

// keystoreEntries returns the keystore entries that the repository client of
// the bucket needs, keyed by the setting each one carries.
//
// What lands here differs by storage type, and not only by whether the
// contract holds static credentials. An azure client takes its storage
// account from the keystore under every authentication choice: the account is
// the one mandatory setting of the client, and without it the repository
// plugin does not find the client at all. S3 and GCS name their bucket in the
// repository settings instead, so a workload-identity bucket of either type
// needs no keystore.
func (s *SnapshotStorage) keystoreEntries() map[string][]byte {
	entries := map[string][]byte{}
	switch s.storageType() {
	case v1.ObjectStorageTypeS3:
		if s.Credentials != nil {
			entries[accessKeyKeystorePath] = []byte(s.Credentials.AccessKeyID)
			entries[secretKeyKeystorePath] = []byte(s.Credentials.SecretAccessKey)
		}

	case v1.ObjectStorageTypeGCS:
		if s.Credentials != nil {
			entries[credentialsFileKeystorePath] = s.Credentials.ServiceAccountJSON
		}

	case v1.ObjectStorageTypeAzureBlob:
		entries[accountKeystorePath] = []byte(s.Config.Spec.AzureBlob.AccountName)
		if s.Credentials != nil {
			entries[accountKeyKeystorePath] = []byte(s.Credentials.AccountKey)
		}
	}

	return entries
}

// keystore reports whether the nodes need a keystore Secret.
func (s *SnapshotStorage) keystore() bool {
	return len(s.keystoreEntries()) > 0
}

// KeystoreSecretName returns the name of the Secret that carries the bucket
// credentials into the keystore of every node.
func KeystoreSecretName(cluster *v1.ElasticsearchCluster) string {
	return cluster.Name + keystoreSecretSuffix
}

// KeystoreComponent builds the keystore component: the Secret whose entries
// ECK loads into the keystore of every node, holding the client settings of
// the snapshot bucket that Elasticsearch accepts nowhere else. Elasticsearch
// reads the credentials of a repository from the keystore alone, never from
// the settings of the repository. The component is gated on the bucket
// needing entries at all; when it needs none the Secret is deleted.
func KeystoreComponent(
	cluster *v1.ElasticsearchCluster,
	storage *SnapshotStorage,
) (*component.Component, error) {
	keystoreSecret, err := secret.NewBuilder(&corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      KeystoreSecretName(cluster),
			Namespace: cluster.Namespace,
			Labels:    managedLabels(cluster),
		},
		Type: corev1.SecretTypeOpaque,
		Data: storage.keystoreEntries(),
	}).Build()
	if err != nil {
		return nil, err
	}

	return component.NewComponentBuilder().
		WithName("keystore").
		WithConditionType(ConditionKeystore).
		WithResource(keystoreSecret, component.GatedBy(feature.NewBooleanGate(storage.keystore()))).
		Build()
}

// RepositoryConfig returns the settings of the snapshot repository that the
// operator registers for the cluster in Elasticsearch. Every cluster writes
// under its own prefix of the shared bucket, so two clusters that reference
// the same contract never share a repository. The credentials are not part of
// it: they reach the nodes through the keystore.
//
// The result is meaningful only for a bucket that ValidateSnapshotStorage
// accepts.
func RepositoryConfig(
	cluster *v1.ElasticsearchCluster,
	storage *SnapshotStorage,
) esadmin.RepositoryConfig {
	storageType := storage.storageType()
	if storageType == "" {
		return esadmin.RepositoryConfig{}
	}

	config := storage.Config
	cfg := esadmin.RepositoryConfig{
		BasePath: logicalbackup.ClusterPrefix(config.BasePath(), cluster.Namespace, cluster.Name),
	}

	switch storageType {
	case v1.ObjectStorageTypeS3:
		cfg.Type = esadmin.RepositoryTypeS3
		cfg.Bucket = config.Spec.S3.BucketName
		cfg.Endpoint = config.Spec.S3.Endpoint
		cfg.PathStyleAccess = config.Spec.S3.ForcePathStyle

	case v1.ObjectStorageTypeGCS:
		cfg.Type = esadmin.RepositoryTypeGCS
		cfg.Bucket = config.Spec.GCS.BucketName

	case v1.ObjectStorageTypeAzureBlob:
		cfg.Type = esadmin.RepositoryTypeAzure
		cfg.Bucket = config.Spec.AzureBlob.Container
	}

	return cfg
}

// ValidateSnapshotStorage reports why a bucket cannot back the snapshot
// repository of a cluster, or nil when it can. The caller turns the message
// into a pre-check failure on the cluster.
//
// Every storage type of the contract is served. What can still fail is an
// azure container whose endpoint Elasticsearch cannot express: see
// azureEndpointSuffix.
func ValidateSnapshotStorage(config *v1.ObjectStorageConfig) error {
	switch config.Spec.Type {
	case v1.ObjectStorageTypeS3:
		if config.Spec.S3 != nil {
			return nil
		}

	case v1.ObjectStorageTypeGCS:
		if config.Spec.GCS != nil {
			return nil
		}

	case v1.ObjectStorageTypeAzureBlob:
		if config.Spec.AzureBlob == nil {
			break
		}
		if _, ok := azureEndpointSuffix(config.Spec.AzureBlob); !ok {
			return fmt.Errorf(
				"ObjectStorageConfig %q names endpoint %q, which Elasticsearch cannot address: "+
					"an azure repository takes an endpoint suffix of the form "+
					"https://<account>.blob.<suffix>, not an arbitrary URL",
				config.Name, config.Spec.AzureBlob.Endpoint,
			)
		}

		return nil
	}

	return fmt.Errorf(
		"ObjectStorageConfig %q declares type %s without the matching block",
		config.Name, config.Spec.Type,
	)
}

// azureEndpointSuffix reduces the service endpoint of an azure container to
// the endpoint suffix that Elasticsearch takes, and reports whether it could.
//
// Elasticsearch addresses an account as https://<account>.blob.<suffix> and
// configures only the suffix, so it serves the public Azure endpoint and the
// sovereign clouds, and nothing else. An emulator that addresses the account
// through a path, such as Azurite, has no suffix to give: it is rejected here
// rather than silently registered against the public endpoint of the account,
// which is a different store than the contract names.
func azureEndpointSuffix(storage *v1.AzureBlobStorage) (string, bool) {
	if storage.Endpoint == "" {
		return defaultAzureEndpointSuffix, true
	}

	endpoint, err := url.Parse(storage.ServiceEndpoint())
	if err != nil || endpoint.Scheme != "https" || endpoint.Path != "" || endpoint.Port() != "" {
		return "", false
	}

	suffix, found := strings.CutPrefix(endpoint.Host, storage.AccountName+".blob.")
	if !found || suffix == "" {
		return "", false
	}

	return suffix, true
}

// The federated-token plumbing of Azure workload identity. Elasticsearch
// reads files under its own configuration directory alone, so the projected
// token goes there rather than to the default path of the Azure webhook, and
// the SDK is pointed at it by name.
const (
	azureTokenVolume            = "azure-identity-token"
	azureTokenDir               = "/usr/share/elasticsearch/config/azure/tokens"
	azureTokenAudience          = "api://AzureADTokenExchange"
	azureTokenExpirationSeconds = 3600
	azureFederatedTokenFileEnv  = "AZURE_FEDERATED_TOKEN_FILE"
)

// azureEndpointSuffixSetting is the node setting that names the service
// endpoint of an azure client. It is the one client setting that the keystore
// does not take.
var azureEndpointSuffixSetting = "azure.client." + esadmin.DefaultClientName + ".endpoint_suffix"

// snapshotStorageMutations returns the mutations that the snapshot bucket adds
// to the ECK CR. Each one is gated on the storage type and the authentication
// choice that needs it; a cluster with no bucket gets none of them.
func snapshotStorageMutations(storage *SnapshotStorage) []eckelasticsearch.Mutation {
	azure := storage.storageType() == v1.ObjectStorageTypeAzureBlob

	endpointSuffix := ""
	if azure {
		if suffix, ok := azureEndpointSuffix(storage.Config.Spec.AzureBlob); ok {
			endpointSuffix = suffix
		}
	}

	return []eckelasticsearch.Mutation{
		{
			// ECK mounts no ServiceAccount token unless the pod template asks
			// for one, and both IRSA and Workload Identity Federation read the
			// projected token of the pod. Azure is not here: it projects a
			// token of its own below, with the audience that Entra ID expects.
			Name:    "ServiceAccountToken",
			Feature: feature.NewBooleanGate(storage.workloadIdentity() && !azure),
			Mutate: func(m *eckelasticsearch.Mutator) error {
				m.Edit(func(es *esv1.Elasticsearch) error {
					automount := true
					es.Spec.NodeSets[0].PodTemplate.Spec.AutomountServiceAccountToken = &automount
					return nil
				})
				return nil
			},
		},
		{
			Name:    "AzureWorkloadIdentity",
			Feature: feature.NewBooleanGate(azure && storage.workloadIdentity()),
			Mutate: func(m *eckelasticsearch.Mutator) error {
				m.Edit(func(es *esv1.Elasticsearch) error {
					template := &es.Spec.NodeSets[0].PodTemplate
					// Without the label the webhook of Azure injects neither
					// the client id nor the tenant id, whatever the
					// ServiceAccount is annotated with.
					template.Labels = labels.Merge(
						template.Labels, storage.Config.WorkloadIdentityPodLabels(),
					)
					expiration := int64(azureTokenExpirationSeconds)
					template.Spec.Volumes = append(template.Spec.Volumes, corev1.Volume{
						Name: azureTokenVolume,
						VolumeSource: corev1.VolumeSource{
							Projected: &corev1.ProjectedVolumeSource{
								Sources: []corev1.VolumeProjection{{
									ServiceAccountToken: &corev1.ServiceAccountTokenProjection{
										Audience:          azureTokenAudience,
										ExpirationSeconds: &expiration,
										Path:              azureTokenVolume,
									},
								}},
							},
						},
					})
					return nil
				})
				m.EditContainer(nodeSetName, func(c *editors.ContainerEditor) error {
					container := c.Raw()
					container.VolumeMounts = append(container.VolumeMounts, corev1.VolumeMount{
						Name:      azureTokenVolume,
						MountPath: azureTokenDir,
					})
					c.EnsureEnvVar(corev1.EnvVar{
						Name:  azureFederatedTokenFileEnv,
						Value: azureTokenDir + "/" + azureTokenVolume,
					})
					return nil
				})
				return nil
			},
		},
		{
			// The default suffix needs no setting, so a contract that names
			// the public Azure endpoint leaves the node configuration empty.
			Name:    "AzureEndpointSuffix",
			Feature: feature.NewBooleanGate(endpointSuffix != "" && endpointSuffix != defaultAzureEndpointSuffix),
			Mutate: func(m *eckelasticsearch.Mutator) error {
				m.Edit(func(es *esv1.Elasticsearch) error {
					nodeSet := &es.Spec.NodeSets[0]
					if nodeSet.Config == nil {
						nodeSet.Config = &commonv1.Config{Data: map[string]any{}}
					}
					nodeSet.Config.Data[azureEndpointSuffixSetting] = endpointSuffix
					return nil
				})
				return nil
			},
		},
	}
}
