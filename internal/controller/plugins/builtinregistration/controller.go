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

// Package builtinregistration reports the Ready condition of each built-in
// PluginRegistration whose requests the operator serves. For these
// registrations the operator is the plugin controller.
package builtinregistration

import (
	"context"
	"slices"

	"github.com/sourcehawk/operator-component-framework/pkg/component"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	pluginsv1alpha1 "github.com/konsole-is/camunda-operator/api/plugins/v1alpha1"
	v1 "github.com/konsole-is/camunda-operator/api/v1"
	"github.com/konsole-is/camunda-operator/internal/observability"
	"github.com/konsole-is/camunda-operator/internal/ownerstatus"
	"github.com/konsole-is/camunda-operator/pkg/conditions"
)

// controllerName is the name the controller registers with controller-runtime.
// It labels every metrics series it records.
const controllerName = "builtinregistration"

// Reconciler maintains the Ready condition of the built-in registrations
// whose requests the operator serves.
type Reconciler struct {
	client.Client
	// Names are the built-in registrations whose requests the operator
	// serves. The controller writes no status on any other registration.
	Names []string
	// Metrics records the condition gauge of the framework. SetupWithManager
	// sets it when it is nil.
	Metrics component.MetricsRecorder
}

// +kubebuilder:rbac:groups=plugins.camunda.io,resources=pluginregistrations,verbs=get;list;watch
// +kubebuilder:rbac:groups=plugins.camunda.io,resources=pluginregistrations/status,verbs=get;update;patch

// Reconcile sets Ready to True with reason Healthy, and
// status.observedGeneration, on a registration in Names. The operator serves
// the requests of these registrations in the manager, so they are ready
// while the manager runs. It leaves every other condition as it finds it,
// and it ignores a registration that is not in Names. It writes nothing
// while Accepted is missing.
func (r *Reconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	if !slices.Contains(r.Names, req.Name) {
		return ctrl.Result{}, nil
	}

	var reg pluginsv1alpha1.PluginRegistration
	if err := r.Get(ctx, req.NamespacedName, &reg); err != nil {
		if apierrors.IsNotFound(err) {
			observability.Forget(r.Metrics, new(pluginsv1alpha1.PluginRegistration).GetKind(), req.NamespacedName)
			return ctrl.Result{}, nil
		}

		return ctrl.Result{}, err
	}

	// The write of Accepted starts the next reconcile.
	if meta.FindStatusCondition(reg.Status.Conditions, pluginsv1alpha1.ConditionAccepted) == nil {
		return ctrl.Result{}, nil
	}

	conditions.Stage(&reg, conditions.Ready(
		metav1.ConditionTrue,
		v1.ReasonHealthy,
		"The operator serves the requests of this registration",
		reg.Generation,
	))

	return ownerstatus.Flush(
		ctx,
		component.ReconcileContext{Client: r.Client, Metrics: r.Metrics, Owner: &reg},
		ctrl.Result{},
		nil,
	)
}

// SetupWithManager registers the controller. It runs for each change of a
// registration in Names, its status included, so a removed Ready comes back.
func (r *Reconciler) SetupWithManager(mgr ctrl.Manager) error {
	if r.Metrics == nil {
		r.Metrics = observability.Recorder(controllerName)
	}

	return ctrl.NewControllerManagedBy(mgr).
		For(
			&pluginsv1alpha1.PluginRegistration{},
			builder.WithPredicates(predicate.NewPredicateFuncs(func(o client.Object) bool {
				return slices.Contains(r.Names, o.GetName())
			})),
		).
		Named(controllerName).
		Complete(r)
}
