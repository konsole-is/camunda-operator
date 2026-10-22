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

package camundamanagementclusterclaim

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
	"github.com/konsole-is/camunda-operator/pkg/components/claimrequests"
	"github.com/konsole-is/camunda-operator/pkg/refindex"
	"github.com/konsole-is/camunda-operator/pkg/wrappers/camundamanagementcluster"
	"github.com/konsole-is/camunda-operator/pkg/wrappers/pluginrequest"
)

// The field indexes of CamundaManagementClusterClaim: the distinct
// registration names of spec.plugins, and the <namespace>/<name> of each
// PluginRequest that the claim can write. The index holds, for each item,
// the request name for a One section and for a Many section.
const (
	pluginRegistrationsField = "spec.plugins.registration"
	requestNamesField        = "spec.plugins.request"
)

// SetupWithManager registers the controller, its field indexes, and its
// watches. It also sets EventRecorder and Metrics when they are nil, and
// builds the uncached component client.
//
// A change of any Environment enqueues every claim, because whether an
// Environment exists at all decides NoEnvironment for each claim in a
// namespace without the label, and a claim serves Environments by name. A
// namespace enqueues its claims, so a claim leaves NoEnvironment when the
// admin labels the namespace.
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
		r.componentClient = pluginrequest.NewApplyClient(camundamanagementcluster.NewApplyClient(uncached))
	}

	indexers := map[string]func(*fleetv1alpha1.CamundaManagementClusterClaim, fleetv1alpha1.PluginSelection) []string{
		pluginRegistrationsField: func(_ *fleetv1alpha1.CamundaManagementClusterClaim, s fleetv1alpha1.PluginSelection) []string {
			return []string{s.Registration}
		},
		requestNamesField: func(c *fleetv1alpha1.CamundaManagementClusterClaim, s fleetv1alpha1.PluginSelection) []string {
			return []string{
				refindex.NamespacedKey(c.Namespace, claimrequests.Name(c.Name, s, pluginsv1alpha1.SelectionOne)),
				refindex.NamespacedKey(c.Namespace, claimrequests.Name(c.Name, s, pluginsv1alpha1.SelectionMany)),
			}
		},
	}
	for field, name := range indexers {
		if err := mgr.GetFieldIndexer().IndexField(
			context.Background(),
			&fleetv1alpha1.CamundaManagementClusterClaim{},
			field,
			func(o client.Object) []string { return distinctNames(o, name) },
		); err != nil {
			return fmt.Errorf("indexing %s: %w", field, err)
		}
	}

	cached := mgr.GetClient()
	list := &fleetv1alpha1.CamundaManagementClusterClaimList{}
	all := enqueueAllClaims(cached)

	return ctrl.NewControllerManagedBy(mgr).
		For(&fleetv1alpha1.CamundaManagementClusterClaim{}).
		// Not Owns: an object that another owner controls must also wake the
		// claim that it blocks, so the claim moves on when the object goes.
		Watches(
			&pluginsv1alpha1.PluginRequest{},
			refindex.Enqueue(cached, list, requestNamesField, refindex.ObjectNamespacedName),
		).
		Watches(&v1.CamundaManagementCluster{}, &handler.EnqueueRequestForObject{}).
		// A required section that the claim does not select decides NotAllowed.
		Watches(&pluginsv1alpha1.PluginSection{}, all).
		Watches(
			&pluginsv1alpha1.PluginRegistration{},
			refindex.Enqueue(cached, list, pluginRegistrationsField, refindex.ObjectName),
		).
		Watches(&fleetv1alpha1.Environment{}, all).
		Watches(&v1.CamundaPlatformConfig{}, all).
		Watches(&corev1.Namespace{}, enqueueClaimsInNamespace(cached), builder.OnlyMetadata).
		Named(controllerName).
		Complete(r)
}

// distinctNames returns the distinct names that name gives for the items of
// spec.plugins of the claim o.
func distinctNames(
	o client.Object,
	name func(*fleetv1alpha1.CamundaManagementClusterClaim, fleetv1alpha1.PluginSelection) []string,
) []string {
	claim, ok := o.(*fleetv1alpha1.CamundaManagementClusterClaim)
	if !ok {
		return nil
	}

	var names []string
	for _, s := range claim.Spec.Plugins {
		names = append(names, name(claim, s)...)
	}
	slices.Sort(names)

	return slices.Compact(names)
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
	var claims fleetv1alpha1.CamundaManagementClusterClaimList
	if err := c.List(ctx, &claims, opts...); err != nil {
		logf.FromContext(ctx).Error(err, "Could not list CamundaManagementClusterClaims for an enqueue")
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
