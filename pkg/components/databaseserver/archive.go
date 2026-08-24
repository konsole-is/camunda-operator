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

package databaseserver

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	cnpgv1 "github.com/cloudnative-pg/api/pkg/api/v1"
	"github.com/sourcehawk/operator-component-framework/pkg/component"
	"github.com/sourcehawk/operator-component-framework/pkg/component/concepts"
	"github.com/sourcehawk/operator-component-framework/pkg/feature"
	"github.com/sourcehawk/operator-component-framework/pkg/primitives/secret"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	v1 "github.com/konsole-is/camunda-operator/api/v1"
	"github.com/konsole-is/camunda-operator/pkg/objectstore"
	"github.com/konsole-is/camunda-operator/pkg/wrappers/barmanobjectstore"
	"github.com/konsole-is/camunda-operator/pkg/wrappers/cnpgcluster"
	"github.com/konsole-is/camunda-operator/pkg/wrappers/cnpgscheduledbackup"
)

const (
	// DefaultBaseBackupSchedule is the schedule of a server whose archive
	// names none: daily at 02:00 UTC. The schema defaults the field to the
	// same value.
	DefaultBaseBackupSchedule = "0 0 2 * * *"

	// archivePathSegment separates the archive of a DatabaseServer from every
	// other layout in the bucket, so a bucket can hold backups and archives
	// side by side.
	archivePathSegment = "databaseserver"
	// archiveSecretSuffix appended to the server name yields the Secret that
	// carries the bucket settings the plugin reads.
	archiveSecretSuffix = "-archive"
)

// The keys of the archive Secret. The Barman Cloud plugin reads every bucket
// setting from a Secret key, so each one it needs has a key here.
const (
	regionKey          = "region"
	accessKeyIDKey     = "accessKeyId"
	secretAccessKeyKey = "secretAccessKey"
	credentialsJSONKey = "credentials.json"
	storageAccountKey  = "storageAccount"
	storageKeyKey      = "storageKey"
)

// ArchiveStorage is the resolved archive bucket of a server: the
// ObjectStorageConfig that spec.archive names, and the keys read from its
// Secret when it holds static credentials. A nil ArchiveStorage means the spec
// names no bucket, so the server has no archive and no point-in-time restore
// can reach it.
type ArchiveStorage struct {
	// Config is the referenced contract.
	Config *v1.ObjectStorageConfig
	// Credentials are the static keys of the bucket, or nil when the contract
	// uses workload identity.
	Credentials *objectstore.Credentials
}

// ObjectStoreName returns the name of the ObjectStore that describes the
// archive of the server. It stays the name of the server across a recovery:
// one bucket location holds every archive the server has written, and the
// serverName parameter of the cluster is what separates them.
func ObjectStoreName(server *v1.DatabaseServer) string { return server.Name }

// BaseBackupName returns the name of the ScheduledBackup that takes the base
// backups of the current cluster. It follows the cluster, so a recovery gets
// a schedule of its own.
func BaseBackupName(server *v1.DatabaseServer) string { return ClusterName(server) }

// ArchiveSecretName returns the Secret that carries the bucket settings of the
// archive into the Barman Cloud plugin.
func ArchiveSecretName(server *v1.DatabaseServer) string {
	return server.Name + archiveSecretSuffix
}

// ValidateArchiveStorage reports why a bucket cannot hold the archive of a
// server, or nil when it can. The caller turns the message into a pre-check
// failure on the server.
//
// Every storage type of the contract is served. What can still fail is a
// contract whose declared type and block disagree. It answers with the same
// resolution the renderers use, so a bucket the pre-check accepts is one every
// renderer can render.
func ValidateArchiveStorage(config *v1.ObjectStorageConfig) error {
	// The server only names the archive Secret and the object prefix, and
	// neither can make a bucket unservable, so a nameless one answers the
	// question.
	_, err := (&ArchiveStorage{Config: config}).resolve(&v1.DatabaseServer{})

	return err
}

// ArchiveComponent builds the archive component: the Secret with the bucket
// settings, the ObjectStore that describes the archive, and the ScheduledBackup
// that takes the base backups. The component is feature-gated on the server
// having an archive at all; without one it deletes its resources and reports
// Disabled.
//
// The component is never suspended. A suspended component applies no baseline,
// so the suspension of the base backup schedule could never reach the cluster,
// and the archive is not deactivated by a suspension anyway: the write-ahead
// log of the last moments before the instances go still has to arrive. What
// suspension reaches is the schedule alone, through its own spec field.
//
// archiveStart is when the earliest base backup of the archive the server
// writes now completed, or nil when none has. Until then the component reports
// Blocked: an archive that holds write-ahead log but no base backup cannot be
// recovered to any point, so it is not ready however well the uploads run. An
// archive the server re-enabled starts again from nil, because the backups of
// the archive it wrote before reach no point in the new one.
func ArchiveComponent(
	server *v1.DatabaseServer,
	merged v1.DatabaseServerSpec,
	archive *ArchiveStorage,
	archiveStart *metav1.Time,
) (*component.Component, error) {
	resolved := archive.resolveOrNil(server)

	settings, err := secret.NewBuilder(archiveSecret(server, resolved)).Build()
	if err != nil {
		return nil, err
	}

	store, err := barmanobjectstore.NewBuilder(objectStore(server, merged, resolved)).Build()
	if err != nil {
		return nil, err
	}

	baseBackup, err := cnpgscheduledbackup.NewBuilder(scheduledBackup(server, merged)).Build()
	if err != nil {
		return nil, err
	}

	// The cluster is read here, never applied: it is the object the archive
	// belongs to, and the guard on it is what holds ArchiveReady False until
	// the archive can be recovered from. Its own health belongs to
	// ClusterReady, which is why the registration is auxiliary.
	recoverable, err := cnpgcluster.NewBuilder(&cnpgv1.Cluster{
		ObjectMeta: metav1.ObjectMeta{Name: ClusterName(server), Namespace: server.Namespace},
	}).
		WithGuard(baseBackupGuard(server, archiveStart, merged.Suspend)).
		Build()
	if err != nil {
		return nil, err
	}

	return component.NewComponentBuilder().
		WithName("archive").
		WithConditionType(ConditionArchive).
		WithFeatureGate(feature.NewBooleanGate(merged.Archive != nil)).
		WithResource(settings, component.GatedBy(feature.NewBooleanGate(len(archiveSecretData(resolved)) > 0))).
		WithResource(store).
		WithResource(baseBackup).
		WithResource(recoverable, component.ReadOnly(), component.Auxiliary()).
		Build()
}

// baseBackupGuard blocks the archive until a base backup of the archive the
// server writes now has completed. archiveStart is when the earliest such
// backup completed, or nil when none has.
//
// A suspended server is never blocked. Its schedule is suspended with it, so
// no base backup can complete and there is nothing to wait on. Blocking there
// would hold the archive of a server that was suspended before its first
// backup at False for as long as the suspension lasts. The guard engages
// again when the server comes back.
func baseBackupGuard(
	server *v1.DatabaseServer,
	archiveStart *metav1.Time,
	suspended bool,
) func(cnpgv1.Cluster) (concepts.GuardStatusWithReason, error) {
	return func(cnpgv1.Cluster) (concepts.GuardStatusWithReason, error) {
		switch {
		case archiveStart != nil:
			return concepts.GuardStatusWithReason{
				Status: concepts.GuardStatusUnblocked,
				Reason: fmt.Sprintf(
					"the archive holds a base backup taken at %s",
					archiveStart.UTC().Format(time.RFC3339),
				),
			}, nil

		case suspended:
			return concepts.GuardStatusWithReason{
				Status: concepts.GuardStatusUnblocked,
				Reason: "the server is suspended, so no base backup can complete",
			}, nil

		default:
			return concepts.GuardStatusWithReason{
				Status: concepts.GuardStatusBlocked,
				Reason: fmt.Sprintf("the first base backup of %q is not complete yet", ClusterName(server)),
			}, nil
		}
	}
}

// archiveSecret renders the Secret that carries the bucket settings of the
// archive. The Barman Cloud plugin reads every one of them from a Secret key,
// including the region of an S3 bucket, so the Secret exists for a bucket that
// authenticates through workload identity too.
func archiveSecret(server *v1.DatabaseServer, resolved *barmanArchive) *corev1.Secret {
	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      ArchiveSecretName(server),
			Namespace: server.Namespace,
			Labels:    managedLabels(server),
		},
		Type: corev1.SecretTypeOpaque,
		Data: archiveSecretData(resolved),
	}
}

// archiveSecretData returns the keys of the archive Secret, or nil when the
// bucket needs none.
func archiveSecretData(resolved *barmanArchive) map[string][]byte {
	if resolved == nil {
		return nil
	}

	return resolved.secretData
}

// objectStore renders the ObjectStore that describes the archive: where it
// lives, how the plugin authenticates, how long a base backup and the
// write-ahead log that follows it are kept, and that both are compressed.
func objectStore(
	server *v1.DatabaseServer,
	merged v1.DatabaseServerSpec,
	resolved *barmanArchive,
) *barmanobjectstore.ObjectStore {
	store := &barmanobjectstore.ObjectStore{
		ObjectMeta: metav1.ObjectMeta{
			Name:      ObjectStoreName(server),
			Namespace: server.Namespace,
			Labels:    managedLabels(server),
		},
		Spec: barmanobjectstore.ObjectStoreSpec{
			Configuration: barmanobjectstore.BarmanObjectStoreConfiguration{
				DestinationPath: destinationPath(resolved),
				Wal: &barmanobjectstore.WalBackupConfiguration{
					Compression: barmanobjectstore.CompressionTypeGzip,
				},
				Data: &barmanobjectstore.DataBackupConfiguration{
					Compression: barmanobjectstore.CompressionTypeGzip,
				},
			},
			RetentionPolicy: retentionPolicy(merged),
		},
	}

	if resolved != nil {
		store.Spec.Configuration.EndpointURL = resolved.endpointURL
		store.Spec.Configuration.S3Credentials = resolved.s3Credentials
		store.Spec.Configuration.AzureCredentials = resolved.azureCredentials
		store.Spec.Configuration.GoogleCredentials = resolved.googleCredentials
	}

	return store
}

// destinationPath returns the bucket URL that holds the archives of the
// server, in the form the Barman Cloud plugin addresses the provider, or the
// empty string when the bucket did not resolve.
func destinationPath(resolved *barmanArchive) string {
	if resolved == nil {
		return ""
	}

	return resolved.destinationPath
}

// retentionPolicy renders spec.archive.retentionPeriodDays as the barman
// retention policy the plugin enforces. It is the same number the contract of
// the server publishes, so what the operator declares is what it enforces.
func retentionPolicy(merged v1.DatabaseServerSpec) string {
	if merged.Archive == nil {
		return ""
	}

	return strconv.Itoa(int(merged.Archive.RetentionPeriodDays)) + "d"
}

// scheduledBackup renders the ScheduledBackup that takes the base backups of
// the current cluster. The first one runs at once, whatever the schedule says:
// the archive can be recovered from only after a base backup completes, so
// waiting for the next scheduled slot would leave a window with no recovery
// point at all.
//
// A suspended server carries a suspended schedule. Its instances are gone, so
// every slot the schedule reaches would otherwise start a backup that cannot
// run and fail it.
func scheduledBackup(server *v1.DatabaseServer, merged v1.DatabaseServerSpec) *cnpgv1.ScheduledBackup {
	schedule := DefaultBaseBackupSchedule
	if merged.Archive != nil && merged.Archive.BaseBackupSchedule != "" {
		schedule = merged.Archive.BaseBackupSchedule
	}

	return &cnpgv1.ScheduledBackup{
		ObjectMeta: metav1.ObjectMeta{
			Name:      BaseBackupName(server),
			Namespace: server.Namespace,
			Labels:    managedLabels(server),
		},
		Spec: cnpgv1.ScheduledBackupSpec{
			Schedule:            schedule,
			Suspend:             new(merged.Suspend),
			Immediate:           new(true),
			Method:              cnpgv1.BackupMethodPlugin,
			PluginConfiguration: &cnpgv1.BackupPluginConfiguration{Name: BarmanPluginName},
			Cluster:             cnpgv1.LocalObjectReference{Name: ClusterName(server)},
		},
	}
}

// archivePlugin returns the plugin entry that makes the cluster archive its
// write-ahead log. serverName is the cluster name, so a recovered cluster
// writes its own archive instead of overwriting the one it recovered from.
func archivePlugin(server *v1.DatabaseServer) cnpgv1.PluginConfiguration {
	return cnpgv1.PluginConfiguration{
		Name:          BarmanPluginName,
		IsWALArchiver: new(true),
		Parameters: map[string]string{
			"barmanObjectName": ObjectStoreName(server),
			"serverName":       ClusterName(server),
		},
	}
}

// identityAnnotations returns the ServiceAccount annotations that bind the
// identity of the bucket, or nil when there is no bucket or no identity.
func (a *ArchiveStorage) identityAnnotations() map[string]string {
	if a == nil || a.Config == nil {
		return nil
	}

	return a.Config.WorkloadIdentityAnnotations()
}

// podLabels returns the labels the instance pods need for the identity of the
// bucket, or nil when they need none. Only Azure has one.
func (a *ArchiveStorage) podLabels() map[string]string {
	if a == nil || a.Config == nil {
		return nil
	}

	return a.Config.WorkloadIdentityPodLabels()
}

// barmanArchive is the Barman Cloud side of the active storage block, reduced
// to what every renderer in this file reads. One switch builds it, in resolve;
// nothing else here dispatches on the storage type. Adding a storage type is a
// case in that switch and nothing more.
//
// This mirrors the activeBlock of ObjectStorageConfig, which reduces the same
// three blocks one layer down for the same reason.
type barmanArchive struct {
	// destinationRoot is the provider-addressing prefix of the archive URL,
	// without the base path of the contract.
	destinationRoot string
	// destinationPath is the bucket URL that holds the archives of one
	// server: destinationRoot, the base path of the contract, and the prefix
	// of the server.
	destinationPath string
	// endpointURL addresses an S3-compatible store. It is empty for every
	// other provider and for AWS S3 itself.
	endpointURL string
	// secretData holds the bucket settings that the plugin reads from a
	// Secret, keyed by the key each one takes.
	secretData map[string][]byte
	// Exactly one credentials block is set, the one of the declared storage
	// type.
	s3Credentials     *barmanobjectstore.S3Credentials
	azureCredentials  *barmanobjectstore.AzureCredentials
	googleCredentials *barmanobjectstore.GoogleCredentials
}

// resolve reduces the contract and its credentials to the archive that the
// plugin writes for server, or reports why it cannot.
//
// It is the one place that dispatches on the storage type, and it is also what
// ValidateArchiveStorage answers with. A bucket the pre-check accepts is
// therefore a bucket every renderer here can render, by construction rather
// than by two switches agreeing. Every renderer of one build shares one call,
// so no two of them can disagree about the bucket either.
func (a *ArchiveStorage) resolve(server *v1.DatabaseServer) (*barmanArchive, error) {
	if a == nil || a.Config == nil {
		return nil, errors.New("the server references no bucket")
	}

	spec := a.Config.Spec
	resolved := &barmanArchive{secretData: map[string][]byte{}}
	secretName := ArchiveSecretName(server)
	ref := func(key string) *barmanobjectstore.SecretKeySelector {
		return &barmanobjectstore.SecretKeySelector{Name: secretName, Key: key}
	}

	served := false
	switch spec.Type {
	case v1.ObjectStorageTypeS3:
		if spec.S3 == nil {
			break
		}
		served = true
		resolved.destinationRoot = "s3://" + spec.S3.BucketName
		resolved.endpointURL = strings.TrimRight(spec.S3.Endpoint, "/")
		resolved.s3Credentials = &barmanobjectstore.S3Credentials{}
		if region := spec.S3.SigningRegion(); region != "" {
			resolved.secretData[regionKey] = []byte(region)
			resolved.s3Credentials.Region = ref(regionKey)
		}
		if a.Credentials != nil {
			resolved.secretData[accessKeyIDKey] = []byte(a.Credentials.AccessKeyID)
			resolved.secretData[secretAccessKeyKey] = []byte(a.Credentials.SecretAccessKey)
			resolved.s3Credentials.AccessKeyID = ref(accessKeyIDKey)
			resolved.s3Credentials.SecretAccessKey = ref(secretAccessKeyKey)
		} else {
			resolved.s3Credentials.InheritFromIAMRole = true
		}

	case v1.ObjectStorageTypeGCS:
		if spec.GCS == nil {
			break
		}
		served = true
		resolved.destinationRoot = "gs://" + spec.GCS.BucketName
		resolved.googleCredentials = &barmanobjectstore.GoogleCredentials{}
		if a.Credentials != nil {
			resolved.secretData[credentialsJSONKey] = a.Credentials.ServiceAccountJSON
			resolved.googleCredentials.ApplicationCredentials = ref(credentialsJSONKey)
		} else {
			resolved.googleCredentials.GKEEnvironment = true
		}

	case v1.ObjectStorageTypeAzureBlob:
		if spec.AzureBlob == nil {
			break
		}
		served = true
		// barman-cloud addresses a container through the service endpoint of
		// the account, so a sovereign cloud and an emulator need no setting
		// beyond the endpoint the contract already carries.
		resolved.destinationRoot = spec.AzureBlob.ServiceEndpoint() + "/" + spec.AzureBlob.Container
		resolved.azureCredentials = &barmanobjectstore.AzureCredentials{}
		if a.Credentials != nil {
			resolved.secretData[storageAccountKey] = []byte(spec.AzureBlob.AccountName)
			resolved.secretData[storageKeyKey] = []byte(a.Credentials.AccountKey)
			resolved.azureCredentials.StorageAccount = ref(storageAccountKey)
			resolved.azureCredentials.StorageKey = ref(storageKeyKey)
		} else {
			resolved.azureCredentials.InheritFromAzureAD = true
		}
	}

	if !served {
		return nil, fmt.Errorf(
			"ObjectStorageConfig %q declares type %s without the matching block",
			a.Config.Name, spec.Type,
		)
	}

	a.setDestinationPath(resolved, server)

	return resolved, nil
}

// setDestinationPath sets the bucket URL of the archive of server on resolved:
// the provider-addressing root of the resolved block, the base path of the
// contract, and the prefix that holds this server alone.
func (a *ArchiveStorage) setDestinationPath(
	resolved *barmanArchive,
	server *v1.DatabaseServer,
) {
	segments := []string{resolved.destinationRoot}
	if base := a.Config.BasePath(); base != "" {
		segments = append(segments, base)
	}
	segments = append(segments, archivePathSegment, server.Namespace, server.Name)
	resolved.destinationPath = strings.Join(segments, "/")
}

// resolveOrNil resolves the bucket and drops the reason it could not. The
// renderers use it: a bucket that does not resolve renders nothing, and the
// pre-check has already reported why to the user.
func (a *ArchiveStorage) resolveOrNil(server *v1.DatabaseServer) *barmanArchive {
	resolved, err := a.resolve(server)
	if err != nil {
		return nil
	}

	return resolved
}
