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
	"errors"
	"fmt"
	"maps"
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

// identityMechanism is how the nodes prove who they are to the bucket. It is
// a property of the cloud, not of the storage type: S3 and GCS both read the
// projected ServiceAccount token of the pod, while Azure reads a federated
// token that this operator projects itself.
type identityMechanism int

const (
	// identityFromKeystore means the nodes authenticate with static
	// credentials, which Elasticsearch reads from the keystore.
	identityFromKeystore identityMechanism = iota
	// identityFromServiceAccountToken means the nodes authenticate as their
	// ServiceAccount, through the token that ECK must be told to mount.
	identityFromServiceAccountToken
	// identityFromFederatedToken means the nodes authenticate with a token
	// projected under the configuration directory of Elasticsearch.
	identityFromFederatedToken
)

// snapshotRepository is the Elasticsearch side of the active storage block,
// reduced to what every renderer in this file reads. One switch builds it, in
// resolve; nothing else here dispatches on the storage type. Adding a storage
// type is a case in that switch and nothing more.
//
// This mirrors the activeBlock of ObjectStorageConfig, which reduces the same
// three blocks one layer down for the same reason.
type snapshotRepository struct {
	// repositoryType is the type of the Elasticsearch repository.
	repositoryType esadmin.RepositoryType
	// bucket is the bucket of the repository, or the blob container of an
	// azure one.
	bucket string
	// endpoint, region, and pathStyle are repository settings of the s3 type
	// alone.
	endpoint  string
	region    string
	pathStyle bool
	// keystore holds the entries that the repository client reads, keyed by
	// the setting each one carries.
	keystore map[string][]byte
	// nodeConfig holds the client settings that the keystore does not take,
	// for elasticsearch.yml. Only azure has any.
	nodeConfig map[string]any
	// identity is how the nodes prove who they are to the bucket.
	identity identityMechanism
}

// resolve reduces the contract and its credentials to the repository that
// this cluster registers, or reports why it cannot.
//
// It is the one place that dispatches on the storage type, and it is also
// what ValidateSnapshotStorage answers with. A bucket the pre-check accepts
// is therefore a bucket every renderer here can render, by construction
// rather than by two switches agreeing.
func (s *SnapshotStorage) resolve() (*snapshotRepository, error) {
	if s == nil || s.Config == nil {
		return nil, errors.New("the cluster references no bucket")
	}

	spec := s.Config.Spec
	repo := &snapshotRepository{
		keystore: map[string][]byte{},
		identity: identityFromServiceAccountToken,
	}
	if s.Credentials != nil {
		repo.identity = identityFromKeystore
	}

	switch spec.Type {
	case v1.ObjectStorageTypeS3:
		if spec.S3 == nil {
			break
		}
		repo.repositoryType = esadmin.RepositoryTypeS3
		repo.bucket = spec.S3.BucketName
		repo.endpoint = spec.S3.Endpoint
		repo.region = spec.S3.SigningRegion()
		repo.pathStyle = spec.S3.ForcePathStyle
		if s.Credentials != nil {
			repo.keystore[accessKeyKeystorePath] = []byte(s.Credentials.AccessKeyID)
			repo.keystore[secretKeyKeystorePath] = []byte(s.Credentials.SecretAccessKey)
		}

		return repo, nil

	case v1.ObjectStorageTypeGCS:
		if spec.GCS == nil {
			break
		}
		repo.repositoryType = esadmin.RepositoryTypeGCS
		repo.bucket = spec.GCS.BucketName
		if s.Credentials != nil {
			repo.keystore[credentialsFileKeystorePath] = s.Credentials.ServiceAccountJSON
		}

		return repo, nil

	case v1.ObjectStorageTypeAzureBlob:
		if spec.AzureBlob == nil {
			break
		}
		repo.repositoryType = esadmin.RepositoryTypeAzure
		repo.bucket = spec.AzureBlob.Container

		// The account is the one mandatory setting of an azure client, and
		// the keystore is the only place that takes it, so it is there under
		// every authentication choice. The key is what workload identity
		// replaces: left out, the Azure SDK takes the identity of the pod.
		repo.keystore[accountKeystorePath] = []byte(spec.AzureBlob.AccountName)
		if s.Credentials != nil {
			repo.keystore[accountKeyKeystorePath] = []byte(s.Credentials.AccountKey)
		} else {
			repo.identity = identityFromFederatedToken
		}

		suffix, ok := azureEndpointSuffix(spec.AzureBlob)
		if !ok {
			// The endpoint itself is never echoed back. This message reaches
			// a status condition of the cluster, which every reader of the
			// cluster can see, and an endpoint carries a shared access
			// signature or a password often enough that none of it may go
			// there. Naming the contract is enough to find the field.
			return nil, fmt.Errorf(
				"the endpoint of ObjectStorageConfig %q is one that Elasticsearch cannot address: "+
					"an azure repository takes an endpoint suffix of the form "+
					"https://<account>.blob.<suffix>, with no port, path, query, fragment, "+
					"or credentials",
				s.Config.Name,
			)
		}
		// The default suffix needs no setting at all.
		if suffix != defaultAzureEndpointSuffix {
			repo.nodeConfig = map[string]any{azureEndpointSuffixSetting: suffix}
		}

		return repo, nil
	}

	return nil, fmt.Errorf(
		"ObjectStorageConfig %q declares type %s without the matching block",
		s.Config.Name, spec.Type,
	)
}

// repositoryOrNil resolves the bucket and drops the reason it could not. The
// renderers use it: a bucket that does not resolve renders nothing, and the
// pre-check has already reported why to the user.
func (s *SnapshotStorage) repositoryOrNil() *snapshotRepository {
	repo, err := s.resolve()
	if err != nil {
		return nil
	}

	return repo
}

// keystoreEntries returns the keystore entries that the repository client of
// the bucket needs, keyed by the setting each one carries.
func (s *SnapshotStorage) keystoreEntries() map[string][]byte {
	repo := s.repositoryOrNil()
	if repo == nil {
		return map[string][]byte{}
	}

	return repo.keystore
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
// An s3 repository carries the region of the contract, so the nodes sign
// their requests for the region the other consumers of the bucket sign for
// rather than for one they resolved on their own.
//
// A bucket that does not resolve yields the zero value, which esadmin
// rejects before it reaches Elasticsearch.
func RepositoryConfig(
	cluster *v1.ElasticsearchCluster,
	storage *SnapshotStorage,
) esadmin.RepositoryConfig {
	repo := storage.repositoryOrNil()
	if repo == nil {
		return esadmin.RepositoryConfig{}
	}

	return esadmin.RepositoryConfig{
		Type:            repo.repositoryType,
		Bucket:          repo.bucket,
		BasePath:        logicalbackup.ClusterPrefix(storage.Config.BasePath(), cluster.Namespace, cluster.Name),
		Endpoint:        repo.endpoint,
		Region:          repo.region,
		PathStyleAccess: repo.pathStyle,
	}
}

// ValidateSnapshotStorage reports why a bucket cannot back the snapshot
// repository of a cluster, or nil when it can. The caller turns the message
// into a pre-check failure on the cluster.
//
// Every storage type of the contract is served. What can still fail is a
// contract whose declared type and block disagree, and an azure container
// whose endpoint Elasticsearch cannot express: see azureEndpointSuffix.
//
// It answers with resolve, so the pre-check can never accept a bucket that
// the renderers cannot render.
func ValidateSnapshotStorage(config *v1.ObjectStorageConfig) error {
	_, err := (&SnapshotStorage{Config: config}).resolve()

	return err
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

	// Only the scheme and the host survive into the suffix, so an endpoint
	// that carries anything else says something the suffix cannot. Dropping a
	// port, a path, a query, a fragment, or credentials silently would leave
	// the repository addressing a store the contract did not name.
	endpoint, err := url.Parse(storage.ServiceEndpoint())
	if err != nil ||
		endpoint.Scheme != "https" ||
		endpoint.Path != "" ||
		endpoint.Port() != "" ||
		endpoint.RawQuery != "" ||
		endpoint.Fragment != "" ||
		endpoint.User != nil {
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
// to the ECK CR. Each one is gated on what the resolved repository needs; a
// cluster with no bucket, or one whose bucket does not resolve, gets none of
// them.
//
// The gates read the identity mechanism, not the storage type. S3 and GCS
// share a branch here because they share a mechanism, which is the reason,
// rather than because neither of them is Azure.
func snapshotStorageMutations(storage *SnapshotStorage) []eckelasticsearch.Mutation {
	repo := storage.repositoryOrNil()
	if repo == nil {
		return nil
	}

	return []eckelasticsearch.Mutation{
		{
			// ECK mounts no ServiceAccount token unless the pod template asks
			// for one, and both IRSA and Workload Identity Federation read the
			// projected token of the pod. Azure is not here: it projects a
			// token of its own below, with the audience that Entra ID expects.
			Name:    "ServiceAccountToken",
			Feature: feature.NewBooleanGate(repo.identity == identityFromServiceAccountToken),
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
			Feature: feature.NewBooleanGate(repo.identity == identityFromFederatedToken),
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
			// Only the settings that the keystore does not take land here, and
			// a client whose settings are all default needs none of them.
			Name:    "NodeConfig",
			Feature: feature.NewBooleanGate(len(repo.nodeConfig) > 0),
			Mutate: func(m *eckelasticsearch.Mutator) error {
				m.Edit(func(es *esv1.Elasticsearch) error {
					nodeSet := &es.Spec.NodeSets[0]
					if nodeSet.Config == nil {
						nodeSet.Config = &commonv1.Config{Data: map[string]any{}}
					}
					maps.Copy(nodeSet.Config.Data, repo.nodeConfig)
					return nil
				})
				return nil
			},
		},
	}
}
