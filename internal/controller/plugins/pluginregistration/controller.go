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

// Package pluginregistration reports whether the operator accepts each
// PluginRegistration, in its Accepted condition. It writes no other condition
// of the registration and creates no resource.
package pluginregistration

import (
	"context"
	"fmt"

	"github.com/Masterminds/semver/v3"
	"github.com/sourcehawk/operator-component-framework/pkg/component"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	pluginsv1alpha1 "github.com/konsole-is/camunda-operator/api/plugins/v1alpha1"
	"github.com/konsole-is/camunda-operator/internal/observability"
	"github.com/konsole-is/camunda-operator/pkg/refindex"
)

// controllerName is the name the controller registers with controller-runtime.
// It labels every metrics series it records.
const controllerName = "pluginregistration"

// SectionsField is the index field that lists registrations by the names in
// their spec.sections. Other controllers in the same manager list the
// registrations of a section by it.
const SectionsField = "pluginregistration.spec.sections"

// Reconciler maintains the Accepted condition of every PluginRegistration.
type Reconciler struct {
	client.Client
	// APIReader reads the live registration when a status write conflicts.
	APIReader client.Reader
	Scheme    *runtime.Scheme
	// OperatorVersion is the version of this operator build. The controller
	// checks it against spec.operatorVersions. version.Dev is inside every
	// valid range.
	OperatorVersion string
	// Metrics records the condition gauge of the framework. SetupWithManager
	// sets it when it is nil.
	Metrics component.MetricsRecorder
}

// +kubebuilder:rbac:groups=plugins.camunda.io,resources=pluginregistrations,verbs=get;list;watch
// +kubebuilder:rbac:groups=plugins.camunda.io,resources=pluginregistrations/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=plugins.camunda.io,resources=pluginsections,verbs=get;list;watch

// Reconcile sets the Accepted condition of the registration. It writes no
// other condition and leaves status.observedGeneration as the server has it,
// because the plugin controller owns Ready and that field.
func (r *Reconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	var reg pluginsv1alpha1.PluginRegistration
	if err := r.Get(ctx, req.NamespacedName, &reg); err != nil {
		if apierrors.IsNotFound(err) {
			observability.Forget(r.Metrics, new(pluginsv1alpha1.PluginRegistration).GetKind(), req.NamespacedName)
			return ctrl.Result{}, nil
		}

		return ctrl.Result{}, err
	}

	f, err := r.observe(ctx, &reg)
	if err != nil {
		return ctrl.Result{}, err
	}

	// The component owns no resources. Passing it makes FlushStatus own
	// Accepted, so on a conflict the live Ready of the plugin controller stays.
	owner, err := component.NewComponentBuilder().
		WithName("accepted").
		WithConditionType(pluginsv1alpha1.ConditionAccepted).
		Build()
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("building the accepted component: %w", err)
	}

	meta.SetStatusCondition(&reg.Status.Conditions, accepted(&reg, r.OperatorVersion, f))

	return ctrl.Result{}, component.FlushStatus(
		ctx,
		component.ReconcileContext{Client: r.Client, APIReader: r.APIReader, Metrics: r.Metrics, Owner: &reg},
		[]*component.Component{owner},
	)
}

// observe reads the sections that reg names.
func (r *Reconciler) observe(ctx context.Context, reg *pluginsv1alpha1.PluginRegistration) (facts, error) {
	var f facts
	for _, name := range reg.Spec.Sections {
		var section pluginsv1alpha1.PluginSection
		err := r.Get(ctx, client.ObjectKey{Name: name}, &section)
		switch {
		case apierrors.IsNotFound(err):
			f.missingSections = append(f.missingSections, name)
		case err != nil:
			return facts{}, fmt.Errorf("reading PluginSection %q: %w", name, err)
		}
	}

	return f, nil
}

// SetupWithManager checks OperatorVersion first and returns an error when it
// is not strict semver. Then it registers the SectionsField index and the
// controller, which also runs when a section is created or deleted.
func (r *Reconciler) SetupWithManager(mgr ctrl.Manager) error {
	if _, err := semver.StrictNewVersion(r.OperatorVersion); err != nil {
		return fmt.Errorf("operator version %q is not valid semver: %w", r.OperatorVersion, err)
	}

	if r.Metrics == nil {
		r.Metrics = observability.Recorder(controllerName)
	}

	if err := mgr.GetFieldIndexer().IndexField(
		context.Background(), &pluginsv1alpha1.PluginRegistration{},
		SectionsField, func(o client.Object) []string {
			return o.(*pluginsv1alpha1.PluginRegistration).Spec.Sections
		},
	); err != nil {
		return err
	}

	return ctrl.NewControllerManagedBy(mgr).
		For(&pluginsv1alpha1.PluginRegistration{}).
		Watches(
			&pluginsv1alpha1.PluginSection{},
			refindex.Enqueue(
				mgr.GetClient(), &pluginsv1alpha1.PluginRegistrationList{},
				SectionsField, refindex.ObjectName,
			),
		).
		Named(controllerName).
		Complete(r)
}
