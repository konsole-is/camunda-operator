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

// Package environment reports whether the objects that an Environment names
// exist. It writes the status of the Environment only.
package environment

import (
	"context"
	"fmt"

	"github.com/sourcehawk/operator-component-framework/pkg/component"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	fleetv1alpha1 "github.com/konsole-is/camunda-operator/api/fleet/v1alpha1"
	pluginsv1alpha1 "github.com/konsole-is/camunda-operator/api/plugins/v1alpha1"
	v1 "github.com/konsole-is/camunda-operator/api/v1"
	"github.com/konsole-is/camunda-operator/internal/observability"
	"github.com/konsole-is/camunda-operator/pkg/conditions"
)

// controllerName is the name the controller registers with controller-runtime.
// It labels every metrics series it records.
const controllerName = "environment"

// lookupObjects builds an empty object of each kind that an Environment names,
// for the lookup of one name.
var lookupObjects = map[string]func() client.Object{
	kindPlatformConfig:       func() client.Object { return &v1.CamundaPlatformConfig{} },
	kindClusterPreset:        func() client.Object { return &v1.CamundaClusterPreset{} },
	kindElasticsearchPreset:  func() client.Object { return &v1.ElasticsearchClusterPreset{} },
	kindDatabaseServerPreset: func() client.Object { return &v1.DatabaseServerPreset{} },
	kindRelease:              func() client.Object { return &v1.CamundaRelease{} },
	kindSection:              func() client.Object { return &pluginsv1alpha1.PluginSection{} },
	kindRegistration:         func() client.Object { return &pluginsv1alpha1.PluginRegistration{} },
}

// Reconciler maintains the status of Environment resources.
type Reconciler struct {
	client.Client
	// APIReader reads the live Environment when a status write conflicts.
	APIReader client.Reader
	Scheme    *runtime.Scheme
	// Metrics records the condition gauge of the framework. SetupWithManager
	// sets it when it is nil.
	Metrics component.MetricsRecorder
}

// +kubebuilder:rbac:groups=fleet.camunda.io,resources=environments,verbs=get;list;watch
// +kubebuilder:rbac:groups=fleet.camunda.io,resources=environments/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=core.camunda.io,resources=camundareleases;camundaclusterpresets;elasticsearchclusterpresets;databaseserverpresets;camundaplatformconfigs,verbs=get;list;watch
// +kubebuilder:rbac:groups=plugins.camunda.io,resources=pluginsections;pluginregistrations,verbs=get;list;watch

// Reconcile sets Ready on the Environment: True with reason Healthy when every
// object that the spec names exists, otherwise False with reason
// InvalidReference and a message that names each missing object.
func (r *Reconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	var env fleetv1alpha1.Environment
	if err := r.Get(ctx, req.NamespacedName, &env); err != nil {
		if apierrors.IsNotFound(err) {
			observability.Forget(r.Metrics, new(fleetv1alpha1.Environment).GetKind(), req.NamespacedName)
			return ctrl.Result{}, nil
		}

		return ctrl.Result{}, err
	}

	missing, err := r.missing(ctx, references(&env.Spec))
	if err != nil {
		return ctrl.Result{}, err
	}

	ready := conditions.Ready(
		metav1.ConditionTrue, v1.ReasonHealthy, "Every object that the spec names exists", env.Generation,
	)
	if len(missing) > 0 {
		ready = conditions.Ready(
			metav1.ConditionFalse, v1.ReasonInvalidReference, invalidReferenceMessage(missing), env.Generation,
		)
	}
	conditions.Stage(&env, ready)

	// The controller owns no components, so its Ready condition follows the
	// server on a conflict and is staged again on the next reconcile.
	return ctrl.Result{}, component.FlushStatus(
		ctx,
		component.ReconcileContext{Client: r.Client, APIReader: r.APIReader, Metrics: r.Metrics, Owner: &env},
		nil,
	)
}

// missing returns the references whose object does not exist, in the order of
// refs.
func (r *Reconciler) missing(ctx context.Context, refs []reference) ([]reference, error) {
	var missing []reference
	for _, ref := range refs {
		err := r.Get(ctx, types.NamespacedName{Name: ref.Name}, lookupObjects[ref.Kind]())
		if apierrors.IsNotFound(err) {
			missing = append(missing, ref)
			continue
		}
		if err != nil {
			return nil, fmt.Errorf("reading %s %s: %w", ref.Kind, ref.Name, err)
		}
	}

	return missing, nil
}

// SetupWithManager registers the controller. Each watch of a kind that an
// Environment names enqueues every Environment, because a cluster holds few.
func (r *Reconciler) SetupWithManager(mgr ctrl.Manager) error {
	if r.Metrics == nil {
		r.Metrics = observability.Recorder(controllerName)
	}

	all := enqueueAll(mgr.GetClient())

	return ctrl.NewControllerManagedBy(mgr).
		For(&fleetv1alpha1.Environment{}).
		Watches(&v1.CamundaPlatformConfig{}, all).
		Watches(&v1.CamundaClusterPreset{}, all).
		Watches(&v1.ElasticsearchClusterPreset{}, all).
		Watches(&v1.DatabaseServerPreset{}, all).
		Watches(&v1.CamundaRelease{}, all).
		Watches(&pluginsv1alpha1.PluginSection{}, all).
		Watches(&pluginsv1alpha1.PluginRegistration{}, all).
		Named(controllerName).
		Complete(r)
}

// enqueueAll maps an event to a request for every Environment.
func enqueueAll(c client.Client) handler.EventHandler {
	return handler.EnqueueRequestsFromMapFunc(func(ctx context.Context, _ client.Object) []reconcile.Request {
		var envs fleetv1alpha1.EnvironmentList
		if err := c.List(ctx, &envs); err != nil {
			logf.FromContext(ctx).Error(err, "Could not list Environments for an enqueue")
			return nil
		}

		requests := make([]reconcile.Request, 0, len(envs.Items))
		for _, env := range envs.Items {
			requests = append(requests, reconcile.Request{NamespacedName: types.NamespacedName{Name: env.Name}})
		}
		return requests
	})
}
