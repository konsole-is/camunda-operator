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
	"time"

	esv1 "github.com/elastic/cloud-on-k8s/v3/pkg/apis/elasticsearch/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	v1 "github.com/konsole-is/camunda-operator/api/v1"
	components "github.com/konsole-is/camunda-operator/pkg/components/elasticsearchcluster"
	"github.com/konsole-is/camunda-operator/pkg/conditions"
	"github.com/konsole-is/camunda-operator/pkg/esadmin"
	"github.com/konsole-is/camunda-operator/pkg/logicalbackup"
	"github.com/konsole-is/camunda-operator/pkg/objectstore"
	"github.com/konsole-is/camunda-operator/pkg/refindex"
	"github.com/konsole-is/camunda-operator/pkg/secretref"
)

const (
	// elasticUsername is the superuser that ECK provisions. The operator
	// registers the snapshot repository as this user, because registering one
	// needs cluster:admin/repository, which the Camunda user deliberately
	// lacks.
	elasticUsername = "elastic"
	// elasticPasswordKey is the key of that user's password inside the Secret
	// that ECK publishes for it. ECK names the key after the user.
	elasticPasswordKey = "elastic"
	// registrationTimeout bounds one registration attempt. Elasticsearch
	// verifies a repository on every registration, with every data node
	// writing a test blob, and a black-holed endpoint would otherwise hold
	// the one worker for the full HTTP timeout.
	registrationTimeout = 5 * time.Second
)

// unwatchedPreCheck marks a pre-check failure that no watch resolves: nothing
// enqueues the cluster when the missing object appears, so the reconcile must
// come back on its own.
type unwatchedPreCheck struct {
	failure *conditions.PreCheckFailure
}

// Error returns the message of the wrapped failure.
func (u *unwatchedPreCheck) Error() string { return u.failure.Error() }

// Unwrap exposes the wrapped failure, so errors.As finds both types.
func (u *unwatchedPreCheck) Unwrap() error { return u.failure }

// resolveSnapshotStorage resolves the snapshot bucket of the merged spec into
// the contract and, for a contract with static credentials, the keys of its
// Secret. It returns nil when the spec names no bucket, which means the
// cluster takes no part in backups.
//
// A reference that does not resolve is a pre-check failure, not an error: the
// contract, or the Secret it names, can appear later, and both are watched.
func (r *ElasticsearchClusterReconciler) resolveSnapshotStorage(
	ctx context.Context,
	merged v1.ElasticsearchClusterSpec,
) (*components.SnapshotStorage, error) {
	if merged.SnapshotStorageRef == "" {
		return nil, nil
	}

	// The cached client: the type is watched, so the cache is current, and
	// every bucket event lands here again anyway.
	var config v1.ObjectStorageConfig
	if err := r.Get(ctx, types.NamespacedName{Name: merged.SnapshotStorageRef}, &config); err != nil {
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
	if config.Spec.Type != v1.ObjectStorageTypeS3 || config.Spec.S3 == nil {
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

	// The Secret is read live: the watch on it is metadata-only.
	key := types.NamespacedName{Namespace: credentialsSecret.Namespace, Name: credentialsSecret.Name}
	secret, msg, err := secretref.Get(ctx, r.APIReader, key, credentialsSecret.Keys...)
	if err != nil {
		return nil, fmt.Errorf("reading credentials of snapshot storage %q: %w", config.Name, err)
	}
	if msg != "" {
		return nil, &conditions.PreCheckFailure{Reason: v1.ReasonMissingSecret, Message: msg}
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
// reference that reports itself. The failure is unwatched: a foreign
// ServiceAccount carries no reference back to the cluster, so only a requeue
// notices its creation.
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
			return &unwatchedPreCheck{failure: &conditions.PreCheckFailure{
				Reason: v1.ReasonInvalidReference,
				Message: fmt.Sprintf(
					"ServiceAccount %s not found, and serviceAccount.create is false", key,
				),
			}}
		}

		return fmt.Errorf("reading ServiceAccount %s: %w", key, err)
	}

	return nil
}

// registerSnapshotRepository converges the snapshot repository of the cluster
// in Elasticsearch and returns the condition that reports the outcome. It
// returns a zero condition when the cluster references no bucket, so nothing
// is reported for a cluster that takes no part in backups. The caller skips
// it entirely while the cluster is suspended.
//
// A repository that already converged is not re-registered while its settings
// are unchanged: Elasticsearch verifies a repository on every registration,
// with every data node writing a test blob. The fingerprints live in memory,
// so an operator restart re-verifies each repository once.
//
// The operator registers the repository itself, with the elastic user of ECK,
// because registering one needs cluster:admin/repository. Giving that to the
// Camunda user would defeat the narrow role it holds.
func (r *ElasticsearchClusterReconciler) registerSnapshotRepository(
	ctx context.Context,
	cluster *v1.ElasticsearchCluster,
	storage *components.SnapshotStorage,
) metav1.Condition {
	if storage == nil || storage.Config == nil {
		r.registeredRepositories.Delete(client.ObjectKeyFromObject(cluster))
		return metav1.Condition{}
	}

	name := components.RepositoryName(cluster)
	config := esadmin.S3RepositoryConfig{
		Bucket:          storage.Config.Spec.S3.BucketName,
		BasePath:        logicalbackup.ClusterPrefix(storage.Config.BasePath(), cluster.Namespace, cluster.Name),
		Endpoint:        storage.Config.Spec.S3.Endpoint,
		PathStyleAccess: storage.Config.Spec.S3.ForcePathStyle,
	}

	fingerprint := fmt.Sprintf("%s|%+v", name, config)
	key := client.ObjectKeyFromObject(cluster)
	if previous, ok := r.registeredRepositories.Load(key); ok &&
		previous == fingerprint &&
		meta.IsStatusConditionTrue(cluster.Status.Conditions, components.ConditionSnapshotRepository) {
		return r.repositoryCondition(
			cluster, metav1.ConditionTrue, v1.ReasonHealthy,
			fmt.Sprintf("snapshot repository %q is registered", name),
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

	registerCtx, cancel := context.WithTimeout(ctx, registrationTimeout)
	defer cancel()
	if err := admin.EnsureSnapshotRepository(registerCtx, name, config); err != nil {
		r.registeredRepositories.Delete(key)
		return r.repositoryCondition(
			cluster, metav1.ConditionFalse, v1.ReasonConnectionFailed,
			fmt.Sprintf("registering snapshot repository %q: %v", name, err),
		)
	}
	r.registeredRepositories.Store(key, fingerprint)

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
// missing one is a pre-check failure and the reconcile retries.
func (r *ElasticsearchClusterReconciler) elasticsearchAdmin(
	ctx context.Context,
	cluster *v1.ElasticsearchCluster,
) (*esadmin.Client, error) {
	password, err := r.eckSecretValue(
		ctx,
		types.NamespacedName{Namespace: cluster.Namespace, Name: esv1.ElasticUserSecret(cluster.Name)},
		elasticPasswordKey,
	)
	if err != nil {
		return nil, err
	}

	ca, err := r.eckSecretValue(
		ctx,
		types.NamespacedName{Namespace: cluster.Namespace, Name: components.CACertSecretName(cluster)},
		components.CACertKey,
	)
	if err != nil {
		return nil, err
	}

	endpoint := components.HTTPEndpoint(cluster)
	if r.EndpointFor != nil {
		endpoint = r.EndpointFor(cluster)
	}

	admin, err := esadmin.New(endpoint, elasticUsername, string(password), ca)
	if err != nil {
		return nil, fmt.Errorf("building the Elasticsearch client of %q: %w", cluster.Name, err)
	}

	return admin, nil
}

// eckSecretValue reads one key of a Secret that ECK publishes, without the
// cache. A Secret or key that is absent is a pre-check failure: it appears
// once ECK has created the cluster, and nothing but a requeue notices when.
func (r *ElasticsearchClusterReconciler) eckSecretValue(
	ctx context.Context,
	key types.NamespacedName,
	dataKey string,
) ([]byte, error) {
	secret, msg, err := secretref.Get(ctx, r.APIReader, key, dataKey)
	if err != nil {
		return nil, fmt.Errorf("reading Secret %s: %w", key, err)
	}
	if msg != "" {
		return nil, &conditions.PreCheckFailure{Reason: v1.ReasonMissingSecret, Message: msg}
	}

	return secret.Data[dataKey], nil
}

// effectiveSnapshotStorageRef resolves the bucket reference of cluster the
// way the reconcile does: the inline value, or the preset's when the inline
// one is unset. The watches must see through the preset, or a fleet cluster
// that inherits its bucket never hears about a credential rotation.
func (r *ElasticsearchClusterReconciler) effectiveSnapshotStorageRef(
	ctx context.Context,
	cluster *v1.ElasticsearchCluster,
) string {
	if cluster.Spec.SnapshotStorageRef != "" {
		return cluster.Spec.SnapshotStorageRef
	}
	if cluster.Spec.PresetRef == "" {
		return ""
	}

	var preset v1.ElasticsearchClusterPreset
	if err := r.Get(ctx, types.NamespacedName{Name: cluster.Spec.PresetRef}, &preset); err != nil {
		return ""
	}

	return preset.Spec.Cluster.SnapshotStorageRef
}

// enqueueForSnapshotStorage maps a bucket event to every cluster whose
// effective reference names it, preset-provided references included.
func (r *ElasticsearchClusterReconciler) enqueueForSnapshotStorage() handler.EventHandler {
	return handler.EnqueueRequestsFromMapFunc(func(ctx context.Context, o client.Object) []reconcile.Request {
		return r.clustersReferencingBuckets(ctx, map[string]bool{o.GetName(): true})
	})
}

// enqueueForBucketSecret maps a Secret event to every cluster whose effective
// bucket holds its static credentials in that Secret. The bucket's Secret
// carries no owner reference to any cluster, so without this watch a rotated
// credential reaches the node keystore only when something unrelated triggers
// a reconcile.
func (r *ElasticsearchClusterReconciler) enqueueForBucketSecret() handler.EventHandler {
	return handler.EnqueueRequestsFromMapFunc(func(ctx context.Context, o client.Object) []reconcile.Request {
		key := refindex.ObjectNamespacedName(o)

		var buckets v1.ObjectStorageConfigList
		if err := r.List(
			ctx, &buckets,
			client.MatchingFields{refindex.ObjectStorageConfigSecretField: key},
		); err != nil {
			logf.FromContext(ctx).Error(err, "Could not list buckets for a Secret event", "secret", key)
			return nil
		}
		if len(buckets.Items) == 0 {
			return nil
		}

		names := make(map[string]bool, len(buckets.Items))
		for i := range buckets.Items {
			names[buckets.Items[i].Name] = true
		}

		return r.clustersReferencingBuckets(ctx, names)
	})
}

// clustersReferencingBuckets returns a request for every cluster whose
// effective bucket reference is one of buckets. The clusters are listed in
// full, because the effective reference can come from a preset and a field
// index cannot resolve one; fleets are small enough for that.
func (r *ElasticsearchClusterReconciler) clustersReferencingBuckets(
	ctx context.Context,
	buckets map[string]bool,
) []reconcile.Request {
	var clusters v1.ElasticsearchClusterList
	if err := r.List(ctx, &clusters); err != nil {
		logf.FromContext(ctx).Error(err, "Could not list clusters for a bucket event")
		return nil
	}

	var requests []reconcile.Request
	for i := range clusters.Items {
		cluster := &clusters.Items[i]
		if buckets[r.effectiveSnapshotStorageRef(ctx, cluster)] {
			requests = append(requests, reconcile.Request{
				NamespacedName: client.ObjectKeyFromObject(cluster),
			})
		}
	}

	return requests
}
