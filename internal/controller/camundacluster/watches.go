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

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	v1 "github.com/konsole-is/camunda-operator/api/v1"
	"github.com/konsole-is/camunda-operator/internal/controller/camundaplatformconfig"
	"github.com/konsole-is/camunda-operator/internal/controller/databaseconfig"
	"github.com/konsole-is/camunda-operator/internal/controller/secondarystorageconfig"
	"github.com/konsole-is/camunda-operator/internal/observability"
	"github.com/konsole-is/camunda-operator/pkg/credentials"
	"github.com/konsole-is/camunda-operator/pkg/labels"
	"github.com/konsole-is/camunda-operator/pkg/refindex"
)

// The index fields of CamundaClusters, one per reference kind. Cluster-scoped
// referents are keyed by name, namespaced ones with refindex.NamespacedKey.
const (
	// PresetRefField is exported: an extension that renders from the effective
	// spec of a cluster finds the clusters bound to a preset through it.
	PresetRefField = "camundacluster.spec.presetRef"
	// ReleaseRefField is exported for the same reason: an extension that
	// renders from the effective spec of a cluster finds the clusters bound
	// to a release through it.
	ReleaseRefField = "camundacluster.spec.releaseRef"
	// PlatformConfigRefField is exported: an extension that reads the platform
	// defaults of a cluster finds the clusters bound to one through it.
	PlatformConfigRefField = "camundacluster.spec.platformConfigRef"
	// StorageRefField is exported: an extension that watches
	// SecondaryStorageConfigs finds the clusters bound to one through it.
	StorageRefField        = "camundacluster.spec.storageRef"
	objectStorageRefsField = "camundacluster.spec.objectStorageRefs"
	// secretRefsField lists the Secrets that the cluster references on its
	// own: spec.auth.clientSecretRef.
	secretRefsField = "camundacluster.spec.secretRefs"
	// presetSecretRefsField lists the Secret names that a preset references:
	// spec.cluster.auth.clientSecretRef. A preset is cluster-scoped and its
	// reference resolves in the namespace of each cluster that inherits it,
	// so the key is the name alone. No other controller reads presets, so
	// this controller owns the index.
	presetSecretRefsField = "camundaclusterpreset.spec.secretRefs"
)

// indexers are the index functions of the fields above.
var indexers = map[string]client.IndexerFunc{
	PresetRefField: func(o client.Object) []string {
		return nonEmpty(o.(*v1.CamundaCluster).Spec.PresetRef)
	},
	ReleaseRefField: func(o client.Object) []string {
		return nonEmpty(o.(*v1.CamundaCluster).Spec.ReleaseRef)
	},
	PlatformConfigRefField: func(o client.Object) []string {
		return nonEmpty(o.(*v1.CamundaCluster).Spec.PlatformConfigRef)
	},
	StorageRefField: func(o client.Object) []string {
		cluster := o.(*v1.CamundaCluster)
		if cluster.Spec.StorageRef == "" {
			return nil
		}
		return []string{refindex.NamespacedKey(cluster.Namespace, cluster.Spec.StorageRef)}
	},
	objectStorageRefsField: func(o client.Object) []string {
		cluster := o.(*v1.CamundaCluster)
		refs := nonEmpty(cluster.Spec.BackupStorageRef, cluster.Spec.DocumentStorageRef)
		for i, ref := range refs {
			refs[i] = refindex.NamespacedKey(cluster.Namespace, ref)
		}
		return refs
	},
	secretRefsField: func(o client.Object) []string {
		cluster := o.(*v1.CamundaCluster)
		if cluster.Spec.Auth == nil || cluster.Spec.Auth.ClientSecretRef == nil {
			return nil
		}
		return []string{refindex.NamespacedKey(cluster.Namespace, cluster.Spec.Auth.ClientSecretRef.Name)}
	},
}

// presetSecretRefs is the index function of presetSecretRefsField. A preset is
// cluster-scoped and its reference resolves in the namespace of each cluster
// that inherits it, so the key is the Secret name alone.
func presetSecretRefs(o client.Object) []string {
	auth := o.(*v1.CamundaClusterPreset).Spec.Cluster.Auth
	if auth == nil || auth.ClientSecretRef == nil {
		return nil
	}
	return []string{auth.ClientSecretRef.Name}
}

// nonEmpty returns the non-empty values.
func nonEmpty(values ...string) []string {
	var result []string
	for _, v := range values {
		if v != "" {
			result = append(result, v)
		}
	}
	return result
}

// enqueueForSecret maps a Secret event to every cluster that can reference
// it: every cluster of the Secret namespace (a same-namespace binding,
// DatabaseConfig, or auth Secret lives there), every cluster whose own auth
// reference names it, every cluster whose platform config or preset
// references it, every cluster whose binding references it, and every
// cluster in the namespace of a DatabaseConfig that references it. The reads
// go through the cached client; the Secret watch is metadata-only.
func (r *CamundaClusterReconciler) enqueueForSecret() handler.EventHandler {
	return handler.EnqueueRequestsFromMapFunc(func(ctx context.Context, o client.Object) []reconcile.Request {
		key := refindex.ObjectNamespacedName(o)
		set := requestSet{}

		set.addList(ctx, r.Client, client.InNamespace(o.GetNamespace()))
		set.addList(ctx, r.Client, client.MatchingFields{secretRefsField: key})

		for _, cfg := range listByIndex[v1.CamundaPlatformConfigList](
			ctx, r.Client, camundaplatformconfig.SecretRefsField, key,
		).Items {
			set.addList(ctx, r.Client, client.MatchingFields{PlatformConfigRefField: cfg.Name})
		}

		for _, preset := range listByIndex[v1.CamundaClusterPresetList](
			ctx, r.Client, presetSecretRefsField, o.GetName(),
		).Items {
			set.addList(
				ctx, r.Client,
				client.InNamespace(o.GetNamespace()),
				client.MatchingFields{PresetRefField: preset.Name},
			)
		}

		for _, binding := range listByIndex[v1.SecondaryStorageConfigList](
			ctx, r.Client, secondarystorageconfig.SecretRefsField, key,
		).Items {
			set.addList(ctx, r.Client, client.MatchingFields{
				StorageRefField: refindex.NamespacedKey(binding.Namespace, binding.Name),
			})
		}

		for _, dbConfig := range listByIndex[v1.DatabaseConfigList](
			ctx, r.Client, databaseconfig.SecretRefsField, key,
		).Items {
			set.addList(ctx, r.Client, client.InNamespace(dbConfig.Namespace))
		}

		for _, bucket := range listByIndex[v1.ObjectStorageConfigList](
			ctx, r.Client, refindex.ObjectStorageConfigSecretField, key,
		).Items {
			set.addList(ctx, r.Client, client.MatchingFields{
				objectStorageRefsField: refindex.NamespacedKey(bucket.Namespace, bucket.Name),
			})
		}

		return set.requests()
	})
}

// listByIndex lists the objects of L whose index field matches key. A list
// failure is logged and yields an empty list.
func listByIndex[L any, PL interface {
	*L
	client.ObjectList
}](ctx context.Context, c client.Client, field, key string) *L {
	list := PL(new(L))
	if err := c.List(ctx, list, client.MatchingFields{field: key}); err != nil {
		logf.FromContext(ctx).Error(err, "listing referrers for Secret enqueue", "field", field, "secret", key)
	}
	return list
}

// enqueueInNamespace maps an event to every cluster of the namespace of the
// event object.
func (r *CamundaClusterReconciler) enqueueInNamespace() handler.EventHandler {
	return handler.EnqueueRequestsFromMapFunc(func(ctx context.Context, o client.Object) []reconcile.Request {
		set := requestSet{}
		set.addList(ctx, r.Client, client.InNamespace(o.GetNamespace()))
		return set.requests()
	})
}

// enqueueAll maps an event to every cluster. DatabaseServerConfigs are few
// and rarely change, and any cluster on an rdbms binding can depend on one.
func (r *CamundaClusterReconciler) enqueueAll() handler.EventHandler {
	return handler.EnqueueRequestsFromMapFunc(func(ctx context.Context, _ client.Object) []reconcile.Request {
		set := requestSet{}
		set.addList(ctx, r.Client)
		return set.requests()
	})
}

// ClusterNameFromLabel returns the name of the CamundaCluster in the
// namespace whose rendered resources carry the given camunda.io/cluster label
// value, or the empty string when no cluster in the namespace does.
//
// The label carries labels.OwnerName of the cluster name, which is not the
// name once the name passes 63 characters. So a caller that maps a rendered
// resource back to its cluster resolves it here, by comparing forward over
// the clusters of the namespace. It never reads the label value as a name.
//
// It is exported because the Optimize controller maps the same label on the
// Deployments this operator renders.
func ClusterNameFromLabel(
	ctx context.Context,
	c client.Client,
	namespace, value string,
) (string, error) {
	var clusters v1.CamundaClusterList
	if err := c.List(ctx, &clusters, client.InNamespace(namespace)); err != nil {
		return "", fmt.Errorf("listing the clusters of namespace %q: %w", namespace, err)
	}

	for i := range clusters.Items {
		if labels.OwnerName(clusters.Items[i].Name) == value {
			return clusters.Items[i].Name, nil
		}
	}

	return "", nil
}

// enqueueForBrokerClaim maps a PersistentVolumeClaim event to the cluster
// that labels it, so a resize or a binding outside the spec updates
// status.volumes.
func (r *CamundaClusterReconciler) enqueueForBrokerClaim() handler.EventHandler {
	return handler.EnqueueRequestsFromMapFunc(func(ctx context.Context, o client.Object) []reconcile.Request {
		value, ok := o.GetLabels()[labels.ClusterKey]
		if !ok {
			return nil
		}

		name, err := ClusterNameFromLabel(ctx, r.Client, o.GetNamespace(), value)
		if err != nil {
			logf.FromContext(ctx).Error(err, "Resolving the cluster of a broker claim", "label", value)
			return nil
		}
		if name == "" {
			return nil
		}

		return []reconcile.Request{{NamespacedName: types.NamespacedName{Namespace: o.GetNamespace(), Name: name}}}
	})
}

// requestSet collects reconcile requests without duplicates.
type requestSet map[types.NamespacedName]struct{}

// addList adds every cluster that the list options select. A list failure is
// logged and drops those requests; the log line is the operational signal.
func (s requestSet) addList(ctx context.Context, c client.Client, opts ...client.ListOption) {
	var clusters v1.CamundaClusterList
	if err := c.List(ctx, &clusters, opts...); err != nil {
		logf.FromContext(ctx).Error(err, "listing clusters for enqueue")
		return
	}
	for _, cluster := range clusters.Items {
		s[client.ObjectKeyFromObject(&cluster)] = struct{}{}
	}
}

func (s requestSet) requests() []reconcile.Request {
	reqs := make([]reconcile.Request, 0, len(s))
	for key := range s {
		reqs = append(reqs, reconcile.Request{NamespacedName: key})
	}
	return reqs
}

// SetupWithManager registers the controller, the reference indexes of the
// clusters, the Secret index of the presets, and the watches. It owns the
// workloads, Services, ServiceAccounts, and Secrets (metadata only) it
// applies, and watches the broker claims by the camunda.io/cluster label.
// Every reference is watched: platform configs, presets, releases, bindings,
// and object storage configs through the indexes, DatabaseConfigs by namespace,
// DatabaseServerConfigs for every cluster, and Secrets (metadata only)
// through enqueueForSecret, which also follows the Secret indexes of the
// platform configs, the bindings, and the DatabaseConfigs. The pre-checks put the
// resource versions of the Secrets and the generations of the CRs they read
// into the config hash, so any of these events rolls the pods whose rendered
// configuration changed. It also sets EventRecorder, Metrics, and the uncached
// component client when they are nil.
func (r *CamundaClusterReconciler) SetupWithManager(mgr ctrl.Manager) error {
	if r.EventRecorder == nil {
		r.EventRecorder = mgr.GetEventRecorder(controllerName)
	}
	if r.Metrics == nil {
		r.Metrics = observability.Recorder(controllerName)
	}
	r.restMapper = mgr.GetRESTMapper()

	if r.componentClient == nil {
		componentClient, err := client.New(mgr.GetConfig(), client.Options{
			Scheme: mgr.GetScheme(),
			Mapper: mgr.GetRESTMapper(),
		})
		if err != nil {
			return fmt.Errorf("building the component client: %w", err)
		}
		// The apply wrapper enforces the precondition of a reused admin
		// password, so a delete of the admin Secret rotates it.
		r.componentClient = credentials.NewApplyClient(componentClient)
	}

	if err := refindex.EnsureObjectStorageConfigSecretIndex(mgr); err != nil {
		return fmt.Errorf("registering the bucket credentials index: %w", err)
	}

	for field, indexer := range indexers {
		if err := mgr.GetFieldIndexer().IndexField(
			context.Background(), &v1.CamundaCluster{}, field, indexer,
		); err != nil {
			return err
		}
	}
	if err := mgr.GetFieldIndexer().IndexField(
		context.Background(), &v1.CamundaClusterPreset{}, presetSecretRefsField, presetSecretRefs,
	); err != nil {
		return err
	}

	cached := mgr.GetClient()
	clusters := &v1.CamundaClusterList{}
	return ctrl.NewControllerManagedBy(mgr).
		WithOptions(controller.Options{MaxConcurrentReconciles: concurrentReconciles}).
		For(&v1.CamundaCluster{}).
		Owns(&appsv1.StatefulSet{}).
		Owns(&appsv1.Deployment{}).
		Owns(&corev1.Service{}).
		Owns(&corev1.ServiceAccount{}).
		Owns(&corev1.Secret{}, builder.OnlyMetadata).
		Watches(&corev1.PersistentVolumeClaim{}, r.enqueueForBrokerClaim()).
		Watches(
			&v1.CamundaPlatformConfig{},
			refindex.Enqueue(cached, clusters, PlatformConfigRefField, refindex.ObjectName),
		).
		Watches(
			&v1.CamundaClusterPreset{},
			refindex.Enqueue(cached, clusters, PresetRefField, refindex.ObjectName),
		).
		Watches(
			&v1.CamundaRelease{},
			refindex.Enqueue(cached, clusters, ReleaseRefField, refindex.ObjectName),
		).
		Watches(
			&v1.SecondaryStorageConfig{},
			refindex.Enqueue(cached, clusters, StorageRefField, refindex.ObjectNamespacedName),
		).
		Watches(
			&v1.ObjectStorageConfig{},
			refindex.Enqueue(cached, clusters, objectStorageRefsField, refindex.ObjectNamespacedName),
		).
		Watches(&v1.DatabaseConfig{}, r.enqueueInNamespace()).
		Watches(&v1.DatabaseServerConfig{}, r.enqueueAll()).
		Watches(&corev1.Secret{}, r.enqueueForSecret(), builder.OnlyMetadata).
		Named(controllerName).
		Complete(r)
}
