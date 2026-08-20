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

package camundaoptimize

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
	"github.com/konsole-is/camunda-operator/internal/controller/camundacluster"
	"github.com/konsole-is/camunda-operator/internal/controller/camundaplatformconfig"
	"github.com/konsole-is/camunda-operator/internal/controller/managementauthconfig"
	"github.com/konsole-is/camunda-operator/internal/controller/secondarystorageconfig"
	"github.com/konsole-is/camunda-operator/pkg/refindex"
)

// The index fields of CamundaOptimizes, one per reference kind. The
// cluster-scoped referent is keyed by name, the namespaced one with
// refindex.NamespacedKey.
const (
	clusterRefField        = "camundaoptimize.spec.clusterRef"
	managementAuthRefField = "camundaoptimize.spec.managementAuthRef"
)

// indexers are the index functions of the fields above.
var indexers = map[string]client.IndexerFunc{
	clusterRefField: func(o client.Object) []string {
		optimize := o.(*v1.CamundaOptimize)
		if optimize.Spec.ClusterRef.Name == "" {
			return nil
		}
		return []string{refindex.NamespacedKey(optimize.Namespace, optimize.Spec.ClusterRef.Name)}
	},
	managementAuthRefField: func(o client.Object) []string {
		optimize := o.(*v1.CamundaOptimize)
		if optimize.Spec.ManagementAuthRef == "" {
			return nil
		}
		return []string{optimize.Spec.ManagementAuthRef}
	},
}

// enqueueForStorage maps a SecondaryStorageConfig event to every Optimize that
// reads it. The reference is two hops away: a cluster names the contract, and
// an Optimize names the cluster. The first hop goes through the storage index
// of the cluster controller.
func (r *Reconciler) enqueueForStorage() handler.EventHandler {
	return handler.EnqueueRequestsFromMapFunc(func(ctx context.Context, o client.Object) []reconcile.Request {
		key := refindex.NamespacedKey(o.GetNamespace(), o.GetName())

		var clusters v1.CamundaClusterList
		if err := r.List(ctx, &clusters, client.MatchingFields{camundacluster.StorageRefField: key}); err != nil {
			logf.FromContext(ctx).Error(err, "Listing clusters for a storage enqueue", "storage", key)
			return nil
		}

		set := requestSet{}
		for _, cluster := range clusters.Items {
			set.addOptimizesOfCluster(ctx, r.Client, &cluster)
		}

		return set.requests()
	})
}

// enqueueForSecret maps a Secret event to every Optimize that can reference
// it: every Optimize of the Secret namespace, and every Optimize that reaches
// it through a contract. Each contract is followed by its own Secret index,
// then back through the cluster index and the Optimize indexes. A Secret in
// another namespace is copied into the Optimize namespace, so an event on it
// must refresh that copy. The reads go through the cached client; the Secret
// watch is metadata-only.
func (r *Reconciler) enqueueForSecret() handler.EventHandler {
	return handler.EnqueueRequestsFromMapFunc(func(ctx context.Context, o client.Object) []reconcile.Request {
		key := refindex.ObjectNamespacedName(o)
		set := requestSet{}

		set.addList(ctx, r.Client, client.InNamespace(o.GetNamespace()))

		for _, auth := range listByIndex[v1.ManagementAuthConfigList](
			ctx, r.Client, managementauthconfig.SecretRefsField, key,
		).Items {
			set.addList(ctx, r.Client, client.MatchingFields{managementAuthRefField: auth.Name})
		}

		for _, binding := range listByIndex[v1.SecondaryStorageConfigList](
			ctx, r.Client, secondarystorageconfig.SecretRefsField, key,
		).Items {
			set.addClustersByIndex(
				ctx, r.Client, camundacluster.StorageRefField,
				refindex.NamespacedKey(binding.Namespace, binding.Name),
			)
		}

		for _, cfg := range listByIndex[v1.CamundaPlatformConfigList](
			ctx, r.Client, camundaplatformconfig.SecretRefsField, key,
		).Items {
			set.addClustersByIndex(ctx, r.Client, camundacluster.PlatformConfigRefField, cfg.Name)
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
		logf.FromContext(ctx).Error(err, "Listing referrers for enqueue", "field", field, "key", key)
	}

	return list
}

// enqueueSiblings maps an event on one CamundaOptimize to the others attached
// to the same cluster. One cluster carries one Optimize instance, so the
// others wait for the holder of the attachment to go. Nothing else tells them
// that it went: their own watch reports events on themselves only.
func (r *Reconciler) enqueueSiblings() handler.EventHandler {
	return handler.EnqueueRequestsFromMapFunc(func(ctx context.Context, o client.Object) []reconcile.Request {
		optimize, ok := o.(*v1.CamundaOptimize)
		if !ok {
			return nil
		}

		set := requestSet{}
		set.addList(ctx, r.Client, client.InNamespace(optimize.Namespace), client.MatchingFields{
			clusterRefField: refindex.NamespacedKey(optimize.Namespace, optimize.Spec.ClusterRef.Name),
		})
		delete(set, client.ObjectKeyFromObject(optimize))

		return set.requests()
	})
}

// requestSet collects reconcile requests without duplicates.
type requestSet map[types.NamespacedName]struct{}

// addList adds every Optimize that the list options select. A list failure is
// logged and drops those requests; the log line is the operational signal.
func (s requestSet) addList(ctx context.Context, c client.Client, opts ...client.ListOption) {
	var list v1.CamundaOptimizeList
	if err := c.List(ctx, &list, opts...); err != nil {
		logf.FromContext(ctx).Error(err, "Listing CamundaOptimizes for enqueue")
		return
	}
	for _, optimize := range list.Items {
		s[client.ObjectKeyFromObject(&optimize)] = struct{}{}
	}
}

// addOptimizesOfCluster adds every Optimize attached to a cluster.
func (s requestSet) addOptimizesOfCluster(ctx context.Context, c client.Client, cluster *v1.CamundaCluster) {
	s.addList(ctx, c, client.MatchingFields{
		clusterRefField: refindex.NamespacedKey(cluster.Namespace, cluster.Name),
	})
}

// addClustersByIndex adds every Optimize attached to a cluster that the given
// index of the cluster controller selects.
func (s requestSet) addClustersByIndex(ctx context.Context, c client.Client, field, key string) {
	for _, cluster := range listByIndex[v1.CamundaClusterList](ctx, c, field, key).Items {
		s.addOptimizesOfCluster(ctx, c, &cluster)
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
// watches. It owns the Deployments and Services it applies, and the Secrets it
// copies (metadata only). Every reference is watched: the cluster and the
// Management Identity contract through the indexes, the storage contract
// through the storage index of the cluster controller, and Secrets (metadata
// only) by namespace. It also watches its own kind a second time, so the
// CamundaOptimizes that wait for an attached cluster hear that the holder of
// the attachment went. ServiceMonitors are not watched: the operator only
// checks whether the Kubernetes cluster serves the kind. It also sets
// EventRecorder to the recorder of the manager and builds the uncached
// component client when they are nil.
func (r *Reconciler) SetupWithManager(mgr ctrl.Manager) error {
	if r.EventRecorder == nil {
		r.EventRecorder = mgr.GetEventRecorder("camundaoptimize")
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
			context.Background(), &v1.CamundaOptimize{}, field, indexer,
		); err != nil {
			return err
		}
	}

	cached := mgr.GetClient()
	list := &v1.CamundaOptimizeList{}

	return ctrl.NewControllerManagedBy(mgr).
		For(&v1.CamundaOptimize{}).
		Owns(&appsv1.Deployment{}).
		Owns(&corev1.Service{}).
		Owns(&corev1.Secret{}, builder.OnlyMetadata).
		Watches(
			&v1.CamundaCluster{},
			refindex.Enqueue(cached, list, clusterRefField, refindex.ObjectNamespacedName),
		).
		Watches(
			&v1.ManagementAuthConfig{},
			refindex.Enqueue(cached, list, managementAuthRefField, refindex.ObjectName),
		).
		Watches(&v1.CamundaOptimize{}, r.enqueueSiblings()).
		Watches(&v1.SecondaryStorageConfig{}, r.enqueueForStorage()).
		Watches(&corev1.Secret{}, r.enqueueForSecret(), builder.OnlyMetadata).
		Named("camundaoptimize").
		Complete(r)
}
