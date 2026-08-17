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
	"context"
	"fmt"

	"sigs.k8s.io/controller-runtime/pkg/client"

	v1 "github.com/konsole-is/camunda-operator/api/v1"
	components "github.com/konsole-is/camunda-operator/pkg/components/camundacluster"
	"github.com/konsole-is/camunda-operator/pkg/conditions"
)

// resolveObjectStorage reads the ObjectStorageConfigs that
// spec.backupStorageRef and spec.documentStorageRef name. The backup bucket
// enters the input with its credentials reference pointed at the copy in the
// cluster namespace, so the renderer only ever names local Secrets. Both
// buckets contribute their workload identity to the ServiceAccount
// annotations; two identities of one cloud are rejected, because a pod has
// one ServiceAccount.
//
// An Elasticsearch cluster that takes backups also needs the snapshot
// repository of its storage contract: without it the web applications have
// nowhere to write, so the reference is incomplete.
func (res *resolver) resolveObjectStorage(ctx context.Context, in *components.Input) error {
	backup, err := res.objectStorage(ctx, res.cluster.Spec.BackupStorageRef)
	if err != nil {
		return err
	}
	documents, err := res.objectStorage(ctx, res.cluster.Spec.DocumentStorageRef)
	if err != nil {
		return err
	}

	annotations, err := components.DerivedServiceAccountAnnotations(backup, documents)
	if err != nil {
		return &conditions.PreCheckFailure{Reason: v1.ReasonInvalidReference, Message: err.Error()}
	}
	in.ServiceAccountAnnotations = annotations

	if backup == nil {
		return nil
	}
	if err := res.rejectSharedAzureContainer(ctx, backup); err != nil {
		return err
	}
	if err := res.localizeBucketCredentials(ctx, backup); err != nil {
		return err
	}
	in.Backup = backup

	return res.requireSnapshotRepository(in)
}

// rejectSharedAzureContainer rejects an Azure backup bucket that an older
// cluster already backs up to. The azure store has no container field: its
// base-path IS the container, so the per-cluster prefix that isolates two
// clusters on S3 and GCS does not exist there, and two clusters on one
// contract would write into one container. The oldest cluster keeps the
// contract (ties break by name), so creating a second cluster never breaks
// the first.
func (res *resolver) rejectSharedAzureContainer(ctx context.Context, bucket *v1.ObjectStorageConfig) error {
	if bucket.Spec.AzureBlob == nil {
		return nil
	}

	var clusters v1.CamundaClusterList
	if err := res.reader.List(ctx, &clusters); err != nil {
		return fmt.Errorf("listing clusters that share ObjectStorageConfig %q: %w", bucket.Name, err)
	}

	for i := range clusters.Items {
		other := &clusters.Items[i]
		if other.UID == res.cluster.UID || other.Spec.BackupStorageRef != bucket.Name {
			continue
		}
		if olderThan(other, res.cluster) {
			return &conditions.PreCheckFailure{
				Reason: v1.ReasonInvalidReference,
				Message: fmt.Sprintf(
					"ObjectStorageConfig %q is an Azure container that CamundaCluster %q already backs up "+
						"to; the azure store writes into one container per contract, so every cluster needs "+
						"a contract with its own container",
					bucket.Name, objectPath(client.ObjectKeyFromObject(other)),
				),
			}
		}
	}

	return nil
}

// olderThan reports whether a was created before b, with the name as the
// tie-break, so exactly one of two clusters ever yields.
func olderThan(a, b *v1.CamundaCluster) bool {
	if !a.CreationTimestamp.Equal(&b.CreationTimestamp) {
		return a.CreationTimestamp.Before(&b.CreationTimestamp)
	}
	return a.Name < b.Name
}

// objectStorage reads the ObjectStorageConfig that ref names, or returns nil
// when the reference is empty.
func (res *resolver) objectStorage(ctx context.Context, ref string) (*v1.ObjectStorageConfig, error) {
	if ref == "" {
		return nil, nil
	}

	var config v1.ObjectStorageConfig
	if err := res.get(ctx, client.ObjectKey{Name: ref}, &config); err != nil {
		return nil, err
	}

	return &config, nil
}

// localizeBucketCredentials checks the static credentials of bucket and
// rewrites its reference to the copy in the cluster namespace. A bucket that
// authenticates with workload identity references no Secret and is left
// alone.
func (res *resolver) localizeBucketCredentials(ctx context.Context, bucket *v1.ObjectStorageConfig) error {
	creds := bucket.CredentialsSecret()
	if creds == nil {
		return nil
	}

	local, err := res.secret(
		ctx,
		client.ObjectKey{Namespace: creds.Namespace, Name: creds.Name},
		components.MirrorPurposeBackupCredentials,
		creds.Keys...,
	)
	if err != nil {
		return err
	}

	switch spec := &bucket.Spec; {
	case spec.S3 != nil && spec.S3.Auth.Credentials != nil:
		spec.S3.Auth.Credentials.SecretRef.Name = local.Name
		spec.S3.Auth.Credentials.SecretRef.Namespace = local.Namespace
	case spec.GCS != nil && spec.GCS.Auth.Credentials != nil:
		spec.GCS.Auth.Credentials.SecretRef.Name = local.Name
		spec.GCS.Auth.Credentials.SecretRef.Namespace = local.Namespace
	case spec.AzureBlob != nil && spec.AzureBlob.Auth.Credentials != nil:
		spec.AzureBlob.Auth.Credentials.SecretRef.Name = local.Name
		spec.AzureBlob.Auth.Credentials.SecretRef.Namespace = local.Namespace
	}

	return nil
}

// requireSnapshotRepository rejects an Elasticsearch cluster that takes
// backups whose storage contract carries no snapshot repository. The
// ElasticsearchCluster controller fills the field when it manages the
// cluster; a hand-written contract needs it set by hand, after the repository
// is registered.
func (res *resolver) requireSnapshotRepository(in *components.Input) error {
	if in.Storage.Type != v1.SecondaryStorageTypeElasticsearch || in.Storage.Elasticsearch == nil {
		return nil
	}
	if in.Storage.Elasticsearch.SnapshotRepository != "" {
		return nil
	}

	return &conditions.PreCheckFailure{
		Reason: v1.ReasonInvalidReference,
		Message: fmt.Sprintf(
			"SecondaryStorageConfig %q has no elasticsearch.snapshotRepository, which a cluster with a "+
				"backupStorageRef needs to write backups to",
			objectPath(client.ObjectKey{Namespace: res.cluster.Namespace, Name: res.cluster.Spec.StorageRef}),
		),
	}
}
