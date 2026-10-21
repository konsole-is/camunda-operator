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

package camundaclusterclaim

import (
	"context"
	"fmt"
	"slices"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	fleetv1alpha1 "github.com/konsole-is/camunda-operator/api/fleet/v1alpha1"
	pluginsv1alpha1 "github.com/konsole-is/camunda-operator/api/plugins/v1alpha1"
	v1 "github.com/konsole-is/camunda-operator/api/v1"
	"github.com/konsole-is/camunda-operator/internal/observability"
	"github.com/konsole-is/camunda-operator/pkg/refindex"
	"github.com/konsole-is/camunda-operator/pkg/wrappers/camundacluster"
	"github.com/konsole-is/camunda-operator/pkg/wrappers/pluginrequest"
)

// The field indexes of CamundaClusterClaim: the distinct section and
// registration names of spec.plugins.
const (
	pluginSectionsField      = "spec.plugins.section"
	pluginRegistrationsField = "spec.plugins.registration"
)

// SetupWithManager registers the controller, its field indexes, and its
// watches. It also sets EventRecorder and Metrics when they are nil, and
// builds the uncached component client.
//
// A change of any Environment enqueues every claim, because whether an
// Environment exists at all decides NoEnvironment for each claim in a
// namespace without the label. A namespace enqueues its claims, so a claim
// leaves NoEnvironment when the admin labels the namespace.
func (r *Reconciler) SetupWithManager(mgr ctrl.Manager) error {
	if r.EventRecorder == nil {
		r.EventRecorder = mgr.GetEventRecorder(controllerName)
	}
	if r.Metrics == nil {
		r.Metrics = observability.Recorder(controllerName)
	}
	if r.componentClient == nil {
		uncached, err := client.New(mgr.GetConfig(), client.Options{
			Scheme: mgr.GetScheme(),
			Mapper: mgr.GetRESTMapper(),
		})
		if err != nil {
			return fmt.Errorf("building the component client: %w", err)
		}
		r.componentClient = pluginrequest.NewApplyClient(camundacluster.NewApplyClient(uncached))
	}

	indexers := map[string]func(fleetv1alpha1.PluginSelection) string{
		pluginSectionsField:      func(s fleetv1alpha1.PluginSelection) string { return s.Section },
		pluginRegistrationsField: func(s fleetv1alpha1.PluginSelection) string { return s.Registration },
	}
	for field, name := range indexers {
		if err := mgr.GetFieldIndexer().IndexField(
			context.Background(),
			&fleetv1alpha1.CamundaClusterClaim{},
			field,
			func(o client.Object) []string { return distinctNames(o, name) },
		); err != nil {
			return fmt.Errorf("indexing %s: %w", field, err)
		}
	}

	cached := mgr.GetClient()
	list := &fleetv1alpha1.CamundaClusterClaimList{}
	all := enqueueAllClaims(cached)

	return ctrl.NewControllerManagedBy(mgr).
		For(&fleetv1alpha1.CamundaClusterClaim{}).
		// Not Owns: an object that another owner controls must also wake the
		// claim that it blocks, so the claim moves on when the object goes.
		Watches(&pluginsv1alpha1.PluginRequest{}, enqueueClaimOfRequest()).
		Watches(&v1.CamundaCluster{}, &handler.EnqueueRequestForObject{}).
		Watches(
			&pluginsv1alpha1.PluginSection{},
			refindex.Enqueue(cached, list, pluginSectionsField, refindex.ObjectName),
		).
		Watches(
			&pluginsv1alpha1.PluginRegistration{},
			refindex.Enqueue(cached, list, pluginRegistrationsField, refindex.ObjectName),
		).
		Watches(&fleetv1alpha1.Environment{}, all).
		Watches(&v1.CamundaRelease{}, all).
		Watches(&v1.CamundaClusterPreset{}, all).
		Watches(&v1.CamundaPlatformConfig{}, all).
		Watches(&corev1.Namespace{}, enqueueClaimsInNamespace(cached), builder.OnlyMetadata).
		Named(controllerName).
		Complete(r)
}

// distinctNames returns the distinct names that name gives for the items of
// spec.plugins of the claim o.
func distinctNames(o client.Object, name func(fleetv1alpha1.PluginSelection) string) []string {
	claim, ok := o.(*fleetv1alpha1.CamundaClusterClaim)
	if !ok {
		return nil
	}

	names := make([]string, 0, len(claim.Spec.Plugins))
	for _, s := range claim.Spec.Plugins {
		names = append(names, name(s))
	}
	slices.Sort(names)

	return slices.Compact(names)
}

// enqueueClaimOfRequest maps the event of a PluginRequest to the claim of
// its label fleet.camunda.io/claim and to the claim of spec.claimRef.
func enqueueClaimOfRequest() handler.EventHandler {
	return handler.EnqueueRequestsFromMapFunc(func(_ context.Context, o client.Object) []reconcile.Request {
		request, ok := o.(*pluginsv1alpha1.PluginRequest)
		if !ok {
			return nil
		}

		names := []string{request.Labels[fleetv1alpha1.LabelClaim], request.Spec.ClaimRef.Name}
		slices.Sort(names)

		var requests []reconcile.Request
		for _, name := range slices.Compact(names) {
			if name != "" {
				requests = append(requests, reconcile.Request{
					NamespacedName: types.NamespacedName{Namespace: request.Namespace, Name: name},
				})
			}
		}

		return requests
	})
}

// enqueueAllClaims maps an event to a request for every claim.
func enqueueAllClaims(c client.Client) handler.EventHandler {
	return handler.EnqueueRequestsFromMapFunc(func(ctx context.Context, _ client.Object) []reconcile.Request {
		return listClaims(ctx, c)
	})
}

// enqueueClaimsInNamespace maps the event of a namespace to a request for
// every claim in it.
func enqueueClaimsInNamespace(c client.Client) handler.EventHandler {
	return handler.EnqueueRequestsFromMapFunc(func(ctx context.Context, o client.Object) []reconcile.Request {
		return listClaims(ctx, c, client.InNamespace(o.GetName()))
	})
}

func listClaims(ctx context.Context, c client.Client, opts ...client.ListOption) []reconcile.Request {
	var claims fleetv1alpha1.CamundaClusterClaimList
	if err := c.List(ctx, &claims, opts...); err != nil {
		logf.FromContext(ctx).Error(err, "Could not list CamundaClusterClaims for an enqueue")
		return nil
	}

	requests := make([]reconcile.Request, 0, len(claims.Items))
	for _, claim := range claims.Items {
		requests = append(requests, reconcile.Request{NamespacedName: types.NamespacedName{
			Namespace: claim.Namespace, Name: claim.Name,
		}})
	}

	return requests
}
