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

package camundamanagementcluster

import (
	"context"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	k8slabels "k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	v1 "github.com/konsole-is/camunda-operator/api/v1"
	"github.com/konsole-is/camunda-operator/internal/controller/camundaplatformconfig"
	"github.com/konsole-is/camunda-operator/internal/controller/databaseconfig"
	components "github.com/konsole-is/camunda-operator/pkg/components/camundamanagementcluster"
	"github.com/konsole-is/camunda-operator/pkg/refindex"
	"github.com/konsole-is/camunda-operator/pkg/wrappers/keycloak"
)

// The index fields of CamundaManagementClusters, one per reference kind. The
// cluster-scoped referents are keyed by name, the namespaced ones with
// refindex.NamespacedKey.
const (
	// PlatformConfigRefField lists management clusters by the
	// CamundaPlatformConfig they name.
	PlatformConfigRefField = "camundamanagementcluster.spec.platformConfigRef"
	// DatabaseConfigRefsField lists management clusters by every
	// DatabaseConfig they name.
	DatabaseConfigRefsField = "camundamanagementcluster.spec.databaseConfigRefs"
	// ContractNameField lists management clusters by the
	// ManagementAuthConfig they write.
	ContractNameField = "camundamanagementcluster.spec.managementAuthConfigName"
	// SecretRefsField lists management clusters by every Secret they name
	// themselves. The Secrets they reach through a platform config or a
	// DatabaseConfig are indexed by those resources instead.
	SecretRefsField = "camundamanagementcluster.spec.secretRefs"
)

// indexers are the index functions of the fields above.
var indexers = map[string]client.IndexerFunc{
	PlatformConfigRefField: func(o client.Object) []string {
		mc := o.(*v1.CamundaManagementCluster)
		if mc.Spec.PlatformConfigRef == "" {
			return nil
		}
		return []string{mc.Spec.PlatformConfigRef}
	},
	DatabaseConfigRefsField: func(o client.Object) []string {
		mc := o.(*v1.CamundaManagementCluster)
		refs := databaseConfigRefs(mc)
		keys := make([]string, 0, len(refs))
		for _, ref := range refs {
			keys = append(keys, refindex.NamespacedKey(mc.Namespace, ref))
		}
		return keys
	},
	ContractNameField: func(o client.Object) []string {
		return []string{components.ContractName(o.(*v1.CamundaManagementCluster))}
	},
	SecretRefsField: func(o client.Object) []string {
		return secretRefs(o.(*v1.CamundaManagementCluster))
	},
}

// setupWatches registers the controller, the reference indexes, and the
// watches. It owns the Deployments, the Services, and the Secrets it applies
// (Secrets metadata only), and the Keycloak custom resource where the
// Kubernetes cluster serves that kind. Every reference is watched: the
// platform config, the DatabaseConfigs, and the contract through the indexes
// above, the database servers through the DatabaseConfigs that name them, and
// Secrets (metadata only) through the namespace and the contracts that reach
// them. The orchestration clusters are watched too, because the claim follows
// their labels.
func (r *Reconciler) setupWatches(mgr ctrl.Manager) error {
	for field, indexer := range indexers {
		if err := mgr.GetFieldIndexer().IndexField(
			context.Background(), &v1.CamundaManagementCluster{}, field, indexer,
		); err != nil {
			return err
		}
	}

	cached := mgr.GetClient()
	list := &v1.CamundaManagementClusterList{}

	controller := ctrl.NewControllerManagedBy(mgr).
		For(&v1.CamundaManagementCluster{}).
		Owns(&appsv1.Deployment{}).
		Owns(&corev1.Service{}).
		Owns(&corev1.Secret{}, builder.OnlyMetadata)
	// A watch on a kind the Kubernetes cluster does not serve fails the
	// manager at start, so the Keycloak is only watched where the Keycloak
	// Operator is installed. Without it the pre-check reports
	// KeycloakOperatorNotInstalled and the keycloak mode renders nothing.
	if r.keycloakServed {
		controller = controller.Owns(&keycloak.Keycloak{})
	} else {
		mgr.GetLogger().Info(
			"Keycloak CRD not found, every CamundaManagementCluster in the keycloak mode reports " +
				"KeycloakOperatorNotInstalled until the Keycloak Operator is installed and the " +
				"operator restarts",
		)
	}

	return controller.
		Watches(
			&v1.CamundaPlatformConfig{},
			refindex.Enqueue(cached, list, PlatformConfigRefField, refindex.ObjectName),
		).
		Watches(
			&v1.DatabaseConfig{},
			refindex.Enqueue(cached, list, DatabaseConfigRefsField, refindex.ObjectNamespacedName),
		).
		Watches(
			&v1.ManagementAuthConfig{},
			refindex.Enqueue(cached, list, ContractNameField, refindex.ObjectName),
		).
		Watches(&v1.DatabaseServerConfig{}, r.enqueueForDatabaseServer()).
		Watches(&v1.CamundaCluster{}, r.enqueueForCluster()).
		Watches(&corev1.Namespace{}, r.enqueueForNamespace(), builder.OnlyMetadata).
		Watches(&corev1.Secret{}, r.enqueueForSecret(), builder.OnlyMetadata).
		Named(controllerName).
		Complete(r)
}

// secretRefs returns every Secret that a management cluster names itself, as
// namespaced index keys. Each one lives in the management namespace, so an
// event on it has to reach the management cluster that reads it.
func secretRefs(mc *v1.CamundaManagementCluster) []string {
	var keys []string
	if external := mc.Spec.IdentityProvider.ExternalKeycloak; external != nil {
		keys = append(
			keys,
			refindex.NamespacedKey(mc.Namespace, external.AdminCredentialsSecretRef.Name),
		)
	}
	if ref := mc.Spec.Identity.Admin.PasswordSecretRef; ref != nil {
		keys = append(keys, refindex.NamespacedKey(mc.Namespace, ref.Name))
	}
	if webModeler := mc.Spec.WebModeler; webModeler != nil {
		if ref := webModeler.Mail.CredentialsSecretRef; ref != nil {
			keys = append(keys, refindex.NamespacedKey(mc.Namespace, ref.Name))
		}
	}

	return keys
}

// databaseConfigRefs returns every DatabaseConfig that a management cluster
// names, in the namespace of that management cluster.
func databaseConfigRefs(mc *v1.CamundaManagementCluster) []string {
	refs := []string{mc.Spec.Identity.DatabaseConfigRef}
	if managed := mc.Spec.IdentityProvider.Keycloak; managed != nil {
		refs = append(refs, managed.DatabaseConfigRef)
	}
	if webModeler := mc.Spec.WebModeler; webModeler != nil {
		refs = append(refs, webModeler.DatabaseConfigRef)
	}

	return refs
}

// enqueueForCluster maps an event on a CamundaCluster to every management
// cluster that the event concerns: the ones whose selector matches its labels,
// and the one that holds its claim. The holder is enqueued as well, because a
// cluster that is relabeled out of a selector no longer matches the management
// cluster that has to withdraw the claim.
func (r *Reconciler) enqueueForCluster() handler.EventHandler {
	return handler.EnqueueRequestsFromMapFunc(func(ctx context.Context, o client.Object) []reconcile.Request {
		set := requestSet{}
		for _, mc := range managementClusters(ctx, r.Client) {
			selector, err := metav1.LabelSelectorAsSelector(mc.Spec.ClusterSelector)
			if err != nil {
				continue
			}
			if selector.Matches(k8slabels.Set(o.GetLabels())) || holdsClaim(o, &mc) {
				set[client.ObjectKeyFromObject(&mc)] = struct{}{}
			}
		}

		return set.requests()
	})
}

// enqueueForNamespace reconciles every management cluster whose
// namespaceSelector puts a bound on the namespace: a changed namespace label
// can move clusters into or out of that bound.
func (r *Reconciler) enqueueForNamespace() handler.EventHandler {
	return handler.EnqueueRequestsFromMapFunc(func(ctx context.Context, _ client.Object) []reconcile.Request {
		set := requestSet{}
		for _, mc := range managementClusters(ctx, r.Client) {
			spec := mc.Spec.NamespaceSelector
			if spec == nil || (len(spec.MatchLabels) == 0 && len(spec.MatchExpressions) == 0) {
				continue
			}
			set[client.ObjectKeyFromObject(&mc)] = struct{}{}
		}

		return set.requests()
	})
}

// holdsClaim reports whether mc is the management cluster that the claim
// annotation of an orchestration cluster names.
func holdsClaim(cluster client.Object, mc *v1.CamundaManagementCluster) bool {
	return cluster.GetAnnotations()[components.ClaimAnnotation] == ClaimValue(mc)
}

// enqueueForDatabaseServer maps an event on a DatabaseServerConfig to every
// management cluster that reaches it. The reference is two hops away: a
// DatabaseConfig names the server, and a management cluster names the
// DatabaseConfig. The host and the port of the server reach the containers, so
// a change to them must roll the pods.
func (r *Reconciler) enqueueForDatabaseServer() handler.EventHandler {
	return handler.EnqueueRequestsFromMapFunc(func(ctx context.Context, o client.Object) []reconcile.Request {
		var configs v1.DatabaseConfigList
		if err := r.List(ctx, &configs); err != nil {
			logf.FromContext(ctx).Error(err, "Listing DatabaseConfigs for a server enqueue")
			return nil
		}

		set := requestSet{}
		for _, cfg := range configs.Items {
			if cfg.Namespace != o.GetNamespace() || cfg.Spec.ServerRef != o.GetName() {
				continue
			}
			set.addList(ctx, r.Client, client.MatchingFields{
				DatabaseConfigRefsField: refindex.NamespacedKey(cfg.Namespace, cfg.Name),
			})
		}

		return set.requests()
	})
}

// enqueueForSecret maps a Secret event to every management cluster that can
// reference it: every one of the Secret namespace, every one that names it in
// its own spec, and every one that reaches it through a platform config or a
// DatabaseConfig. A Secret in another namespace is copied into the management
// namespace, so an event on it must refresh that copy. The reads go through
// the cached client; the Secret watch is metadata-only.
func (r *Reconciler) enqueueForSecret() handler.EventHandler {
	return handler.EnqueueRequestsFromMapFunc(func(ctx context.Context, o client.Object) []reconcile.Request {
		key := refindex.ObjectNamespacedName(o)
		set := requestSet{}

		set.addList(ctx, r.Client, client.InNamespace(o.GetNamespace()))
		set.addList(ctx, r.Client, client.MatchingFields{SecretRefsField: key})

		var configs v1.CamundaPlatformConfigList
		if err := r.List(
			ctx, &configs, client.MatchingFields{camundaplatformconfig.SecretRefsField: key},
		); err != nil {
			logf.FromContext(ctx).Error(err, "Listing platform configs for a Secret enqueue", "secret", key)
		}
		for _, cfg := range configs.Items {
			set.addList(ctx, r.Client, client.MatchingFields{PlatformConfigRefField: cfg.Name})
		}

		var databases v1.DatabaseConfigList
		if err := r.List(
			ctx, &databases, client.MatchingFields{databaseconfig.SecretRefsField: key},
		); err != nil {
			logf.FromContext(ctx).Error(err, "Listing DatabaseConfigs for a Secret enqueue", "secret", key)
		}
		for _, cfg := range databases.Items {
			set.addList(ctx, r.Client, client.MatchingFields{
				DatabaseConfigRefsField: refindex.NamespacedKey(cfg.Namespace, cfg.Name),
			})
		}

		return set.requests()
	})
}

// managementClusters lists every CamundaManagementCluster. A list failure is
// logged and yields none; the log line is the operational signal.
func managementClusters(ctx context.Context, c client.Client) []v1.CamundaManagementCluster {
	var list v1.CamundaManagementClusterList
	if err := c.List(ctx, &list); err != nil {
		logf.FromContext(ctx).Error(err, "Listing CamundaManagementClusters for enqueue")
		return nil
	}

	return list.Items
}

// requestSet collects reconcile requests without duplicates.
type requestSet map[types.NamespacedName]struct{}

// addList adds every management cluster that the list options select. A list
// failure is logged and drops those requests.
func (s requestSet) addList(ctx context.Context, c client.Client, opts ...client.ListOption) {
	var list v1.CamundaManagementClusterList
	if err := c.List(ctx, &list, opts...); err != nil {
		logf.FromContext(ctx).Error(err, "Listing CamundaManagementClusters for enqueue")
		return
	}
	for i := range list.Items {
		s[client.ObjectKeyFromObject(&list.Items[i])] = struct{}{}
	}
}

func (s requestSet) requests() []reconcile.Request {
	reqs := make([]reconcile.Request, 0, len(s))
	for key := range s {
		reqs = append(reqs, reconcile.Request{NamespacedName: key})
	}

	return reqs
}
