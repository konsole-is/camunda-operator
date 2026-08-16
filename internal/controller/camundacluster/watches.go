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
	"sigs.k8s.io/controller-runtime/pkg/handler"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	v1 "github.com/konsole-is/camunda-operator/api/v1"
	"github.com/konsole-is/camunda-operator/internal/controller/camundaplatformconfig"
	"github.com/konsole-is/camunda-operator/pkg/labels"
	"github.com/konsole-is/camunda-operator/pkg/refindex"
)

// The index fields of CamundaClusters, one per reference kind. Cluster-scoped
// referents are keyed by name, namespaced ones with refindex.NamespacedKey.
const (
	presetRefField         = "camundacluster.spec.presetRef"
	platformConfigRefField = "camundacluster.spec.platformConfigRef"
	storageRefField        = "camundacluster.spec.storageRef"
	objectStorageRefsField = "camundacluster.spec.objectStorageRefs"
	// secretRefsField lists the Secrets that the cluster references on its
	// own: spec.auth.clientSecretRef.
	secretRefsField = "camundacluster.spec.secretRefs"
)

// indexers are the index functions of the fields above.
var indexers = map[string]client.IndexerFunc{
	presetRefField: func(o client.Object) []string {
		return nonEmpty(o.(*v1.CamundaCluster).Spec.PresetRef)
	},
	platformConfigRefField: func(o client.Object) []string {
		return nonEmpty(o.(*v1.CamundaCluster).Spec.PlatformConfigRef)
	},
	storageRefField: func(o client.Object) []string {
		cluster := o.(*v1.CamundaCluster)
		if cluster.Spec.StorageRef == "" {
			return nil
		}
		return []string{refindex.NamespacedKey(cluster.Namespace, cluster.Spec.StorageRef)}
	},
	objectStorageRefsField: func(o client.Object) []string {
		cluster := o.(*v1.CamundaCluster)
		return nonEmpty(cluster.Spec.BackupStorageRef, cluster.Spec.DocumentStorageRef)
	},
	secretRefsField: func(o client.Object) []string {
		cluster := o.(*v1.CamundaCluster)
		if cluster.Spec.Auth == nil || cluster.Spec.Auth.ClientSecretRef == nil {
			return nil
		}
		ref := cluster.Spec.Auth.ClientSecretRef
		return []string{refindex.NamespacedKey(ref.Namespace, ref.Name)}
	},
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
// it: every cluster of the Secret namespace (the binding, its DatabaseConfig,
// and a same-namespace auth Secret live there), every cluster whose own auth
// reference names it, and every cluster whose platform config references it.
// The reads go through the cached client; the Secret watch is metadata-only.
func (r *CamundaClusterReconciler) enqueueForSecret() handler.EventHandler {
	return handler.EnqueueRequestsFromMapFunc(func(ctx context.Context, o client.Object) []reconcile.Request {
		key := refindex.ObjectNamespacedName(o)
		set := requestSet{}

		set.addList(ctx, r.Client, client.InNamespace(o.GetNamespace()))
		set.addList(ctx, r.Client, client.MatchingFields{secretRefsField: key})

		var cfgs v1.CamundaPlatformConfigList
		if err := r.List(ctx, &cfgs, client.MatchingFields{camundaplatformconfig.SecretRefsField: key}); err != nil {
			logf.FromContext(ctx).Error(err, "listing platform configs for Secret enqueue", "secret", key)
		}
		for _, cfg := range cfgs.Items {
			set.addList(ctx, r.Client, client.MatchingFields{platformConfigRefField: cfg.Name})
		}

		return set.requests()
	})
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

// enqueueForBrokerClaim maps a PersistentVolumeClaim event to the cluster
// that labels it, so a resize or a binding outside the spec updates
// status.storageSize.
func enqueueForBrokerClaim() handler.EventHandler {
	return handler.EnqueueRequestsFromMapFunc(func(_ context.Context, o client.Object) []reconcile.Request {
		name, ok := o.GetLabels()[labels.ClusterKey]
		if !ok {
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

// SetupWithManager registers the controller, the reference indexes, and the
// watches. It owns the workloads, Services, ServiceAccounts, and Secrets
// (metadata only) it applies, and watches the broker claims by the
// camunda.io/cluster label. Every reference is watched: platform configs,
// presets, bindings, and object storage configs through the indexes,
// DatabaseConfigs by namespace, DatabaseServerConfigs for every cluster, and
// Secrets (metadata only) through enqueueForSecret. The pre-checks put the
// resource versions of the Secrets and the generations of the CRs they read
// into the config hash, so any of these events rolls the pods whose rendered
// configuration changed. It also sets Recorder to the recorder of the manager
// and builds the uncached component client when they are nil.
func (r *CamundaClusterReconciler) SetupWithManager(mgr ctrl.Manager) error {
	if r.Recorder == nil {
		// The ReconcileContext of the component framework takes the legacy
		// record.EventRecorder, so the deprecated accessor is required here.
		r.Recorder = mgr.GetEventRecorderFor("camundacluster") //nolint:staticcheck
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
		r.componentClient = componentClient
	}

	for field, indexer := range indexers {
		if err := mgr.GetFieldIndexer().IndexField(
			context.Background(), &v1.CamundaCluster{}, field, indexer,
		); err != nil {
			return err
		}
	}

	cached := mgr.GetClient()
	clusters := &v1.CamundaClusterList{}
	return ctrl.NewControllerManagedBy(mgr).
		For(&v1.CamundaCluster{}).
		Owns(&appsv1.StatefulSet{}).
		Owns(&appsv1.Deployment{}).
		Owns(&corev1.Service{}).
		Owns(&corev1.ServiceAccount{}).
		Owns(&corev1.Secret{}, builder.OnlyMetadata).
		Watches(&corev1.PersistentVolumeClaim{}, enqueueForBrokerClaim()).
		Watches(
			&v1.CamundaPlatformConfig{},
			refindex.Enqueue(cached, clusters, platformConfigRefField, refindex.ObjectName),
		).
		Watches(
			&v1.CamundaClusterPreset{},
			refindex.Enqueue(cached, clusters, presetRefField, refindex.ObjectName),
		).
		Watches(
			&v1.SecondaryStorageConfig{},
			refindex.Enqueue(cached, clusters, storageRefField, refindex.ObjectNamespacedName),
		).
		Watches(
			&v1.ObjectStorageConfig{},
			refindex.Enqueue(cached, clusters, objectStorageRefsField, refindex.ObjectName),
		).
		Watches(&v1.DatabaseConfig{}, r.enqueueInNamespace()).
		Watches(&v1.DatabaseServerConfig{}, r.enqueueAll()).
		Watches(&corev1.Secret{}, r.enqueueForSecret(), builder.OnlyMetadata).
		Named("camundacluster").
		Complete(r)
}
