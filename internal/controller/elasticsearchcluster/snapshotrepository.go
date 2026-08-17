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
	"context"
	"errors"
	"fmt"
	"path"

	esv1 "github.com/elastic/cloud-on-k8s/v3/pkg/apis/elasticsearch/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	v1 "github.com/konsole-is/camunda-operator/api/v1"
	components "github.com/konsole-is/camunda-operator/pkg/components/elasticsearchcluster"
	"github.com/konsole-is/camunda-operator/pkg/conditions"
	"github.com/konsole-is/camunda-operator/pkg/esadmin"
	"github.com/konsole-is/camunda-operator/pkg/objectstore"
)

// elasticUserKey is the key of the password of the elastic user inside the
// Secret that ECK publishes for it.
const elasticUserKey = "elastic"

// resolveSnapshotStorage resolves spec.snapshotStorageRef into the bucket
// contract and, for a contract with static credentials, the keys of its
// Secret. It returns nil when the spec names no bucket, which means the
// cluster takes no part in backups.
//
// A reference that does not resolve is a pre-check failure, not an error: the
// contract, or the Secret it names, can appear later.
func (r *ElasticsearchClusterReconciler) resolveSnapshotStorage(
	ctx context.Context,
	merged v1.ElasticsearchClusterSpec,
) (*components.SnapshotStorage, error) {
	if merged.SnapshotStorageRef == "" {
		return nil, nil
	}

	var config v1.ObjectStorageConfig
	if err := r.APIReader.Get(ctx, types.NamespacedName{Name: merged.SnapshotStorageRef}, &config); err != nil {
		if apierrors.IsNotFound(err) {
			return nil, &conditions.PreCheckFailure{
				Reason:  v1.ReasonInvalidReference,
				Message: fmt.Sprintf("ObjectStorageConfig %q not found", merged.SnapshotStorageRef),
			}
		}

		return nil, fmt.Errorf("resolving snapshot storage %q: %w", merged.SnapshotStorageRef, err)
	}

	// Elasticsearch registers a repository per storage type, and the operator
	// implements the s3 one. A GCS or Azure bucket is a valid contract that
	// this path cannot use, so it fails the reference rather than registering
	// something that does not match the bucket.
	if config.Spec.S3 == nil {
		return nil, &conditions.PreCheckFailure{
			Reason: v1.ReasonInvalidReference,
			Message: fmt.Sprintf(
				"ObjectStorageConfig %q is of type %s; a snapshot repository needs type %s",
				config.Name, config.Spec.Type, v1.ObjectStorageTypeS3,
			),
		}
	}

	storage := &components.SnapshotStorage{Config: &config}

	credentialsSecret := config.CredentialsSecret()
	if credentialsSecret == nil {
		return storage, nil
	}

	var secret corev1.Secret
	key := types.NamespacedName{Namespace: credentialsSecret.Namespace, Name: credentialsSecret.Name}
	if err := r.APIReader.Get(ctx, key, &secret); err != nil {
		if apierrors.IsNotFound(err) {
			return nil, &conditions.PreCheckFailure{
				Reason:  v1.ReasonMissingSecret,
				Message: fmt.Sprintf("Secret %s of ObjectStorageConfig %q not found", key, config.Name),
			}
		}

		return nil, fmt.Errorf("reading credentials of snapshot storage %q: %w", config.Name, err)
	}

	credentials, err := objectstore.CredentialsFrom(&config, secret.Data)
	if err != nil {
		return nil, &conditions.PreCheckFailure{Reason: v1.ReasonMissingSecret, Message: err.Error()}
	}
	storage.Credentials = credentials

	return storage, nil
}

// checkServiceAccount fails the pre-check when the spec names a ServiceAccount
// that the operator does not create and that does not exist. Without it every
// pod stays unschedulable, which is a slower and less obvious failure than a
// reference that reports itself.
func (r *ElasticsearchClusterReconciler) checkServiceAccount(
	ctx context.Context,
	cluster *v1.ElasticsearchCluster,
	merged v1.ElasticsearchClusterSpec,
) error {
	if merged.ServiceAccount.Creates() {
		return nil
	}

	name := components.ServiceAccountName(cluster, merged)
	key := types.NamespacedName{Namespace: cluster.Namespace, Name: name}
	if err := r.APIReader.Get(ctx, key, &corev1.ServiceAccount{}); err != nil {
		if apierrors.IsNotFound(err) {
			return &conditions.PreCheckFailure{
				Reason: v1.ReasonInvalidReference,
				Message: fmt.Sprintf(
					"ServiceAccount %s not found, and serviceAccount.create is false", key,
				),
			}
		}

		return fmt.Errorf("reading ServiceAccount %s: %w", key, err)
	}

	return nil
}

// registerSnapshotRepository converges the snapshot repository of the cluster
// in Elasticsearch with an idempotent PUT, and returns the condition that
// reports the outcome. It returns a zero condition when the cluster references
// no bucket, so nothing is reported for a cluster that takes no part in
// backups.
//
// The operator registers the repository itself, with the elastic user of ECK,
// because registering one needs cluster:admin/repository. Giving that to the
// Camunda user would defeat the narrow role it holds.
func (r *ElasticsearchClusterReconciler) registerSnapshotRepository(
	ctx context.Context,
	cluster *v1.ElasticsearchCluster,
	merged v1.ElasticsearchClusterSpec,
	storage *components.SnapshotStorage,
) metav1.Condition {
	if storage == nil || storage.Config == nil {
		return metav1.Condition{}
	}

	name := components.RepositoryName(cluster)

	if merged.Suspend {
		return r.repositoryCondition(
			cluster, metav1.ConditionFalse, v1.ReasonConnectionFailed,
			"the cluster is suspended, so the snapshot repository cannot be registered",
		)
	}

	admin, err := r.elasticsearchAdmin(ctx, cluster)
	if err != nil {
		var failure *conditions.PreCheckFailure
		if errors.As(err, &failure) {
			return r.repositoryCondition(cluster, metav1.ConditionFalse, failure.Reason, failure.Message)
		}

		return r.repositoryCondition(
			cluster, metav1.ConditionFalse, v1.ReasonConnectionFailed, err.Error(),
		)
	}

	config := esadmin.S3RepositoryConfig{
		Bucket:          storage.Config.Spec.S3.BucketName,
		BasePath:        path.Join(storage.Config.BasePath(), cluster.Namespace, cluster.Name),
		Endpoint:        storage.Config.Spec.S3.Endpoint,
		PathStyleAccess: storage.Config.Spec.S3.ForcePathStyle,
	}
	if err := admin.EnsureSnapshotRepository(ctx, name, config); err != nil {
		return r.repositoryCondition(
			cluster, metav1.ConditionFalse, v1.ReasonConnectionFailed,
			fmt.Sprintf("registering snapshot repository %q: %v", name, err),
		)
	}

	return r.repositoryCondition(
		cluster, metav1.ConditionTrue, v1.ReasonHealthy,
		fmt.Sprintf("snapshot repository %q is registered", name),
	)
}

// repositoryCondition builds the SnapshotRepositoryReady condition of cluster.
func (r *ElasticsearchClusterReconciler) repositoryCondition(
	cluster *v1.ElasticsearchCluster,
	status metav1.ConditionStatus,
	reason, message string,
) metav1.Condition {
	return metav1.Condition{
		Type:               components.ConditionSnapshotRepository,
		Status:             status,
		Reason:             reason,
		Message:            message,
		ObservedGeneration: cluster.Generation,
	}
}

// elasticsearchAdmin builds a client for the cluster, authenticated as the
// elastic user that ECK provisions and verified against the CA that ECK
// publishes. Both Secrets appear only once ECK has created the cluster, so a
// missing one is a pre-check failure and the next reconcile retries.
func (r *ElasticsearchClusterReconciler) elasticsearchAdmin(
	ctx context.Context,
	cluster *v1.ElasticsearchCluster,
) (*esadmin.Client, error) {
	password, err := r.secretValue(
		ctx,
		types.NamespacedName{Namespace: cluster.Namespace, Name: esv1.ElasticUserSecret(cluster.Name)},
		elasticUserKey,
	)
	if err != nil {
		return nil, err
	}

	ca, err := r.secretValue(
		ctx,
		types.NamespacedName{Namespace: cluster.Namespace, Name: components.CACertSecretName(cluster)},
		components.CACertKey,
	)
	if err != nil {
		return nil, err
	}

	admin, err := esadmin.New(components.HTTPEndpoint(cluster), elasticUserKey, string(password), ca)
	if err != nil {
		return nil, fmt.Errorf("building the Elasticsearch client of %q: %w", cluster.Name, err)
	}

	return admin, nil
}

// secretValue reads one key of a Secret without the cache. A Secret or key
// that is absent is a pre-check failure, so the caller reports it on the
// resource instead of failing the reconcile.
func (r *ElasticsearchClusterReconciler) secretValue(
	ctx context.Context,
	key types.NamespacedName,
	dataKey string,
) ([]byte, error) {
	var secret corev1.Secret
	if err := r.APIReader.Get(ctx, key, &secret); err != nil {
		if apierrors.IsNotFound(err) {
			return nil, &conditions.PreCheckFailure{
				Reason:  v1.ReasonMissingSecret,
				Message: fmt.Sprintf("Secret %s not found", key),
			}
		}

		return nil, fmt.Errorf("reading Secret %s: %w", key, err)
	}

	value, ok := secret.Data[dataKey]
	if !ok {
		return nil, &conditions.PreCheckFailure{
			Reason:  v1.ReasonMissingSecret,
			Message: fmt.Sprintf("Secret %s is missing key %q", key, dataKey),
		}
	}

	return value, nil
}
