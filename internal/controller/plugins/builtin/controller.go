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

// Package builtin serves the PluginRequests of the built-in
// PluginRegistrations. Each registration that it serves has a handler.
package builtin

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"slices"

	"github.com/sourcehawk/operator-component-framework/pkg/component"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/events"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	pluginsv1alpha1 "github.com/konsole-is/camunda-operator/api/plugins/v1alpha1"
	v1 "github.com/konsole-is/camunda-operator/api/v1"
	"github.com/konsole-is/camunda-operator/internal/observability"
	"github.com/konsole-is/camunda-operator/internal/ownerstatus"
	"github.com/konsole-is/camunda-operator/pkg/conditions"
	"github.com/konsole-is/camunda-operator/pkg/refindex"
)

// controllerName is the name the controller registers with controller-runtime.
// It labels every metrics series it records.
const controllerName = "builtin"

// contractsField is the index on PluginRequest with one key for each
// contract that a built-in request reads. No other controller of the
// manager may register an index of this name on PluginRequest.
const contractsField = "pluginrequest.builtin.contracts"

// Reconciler serves every PluginRequest of a built-in registration that has a
// handler.
type Reconciler struct {
	client.Client
	// APIReader reads the request at the top of each reconcile, and the
	// remaining objects of a deletion, without the cache.
	APIReader client.Reader
	Scheme    *runtime.Scheme
	// Registrations are the built-in registrations. A request is checked
	// against the parameter schema of its registration in this list.
	// SetupWithManager refuses a list without a registration that a handler
	// serves.
	Registrations []pluginsv1alpha1.PluginRegistration
	// EventRecorder publishes the events of the components. SetupWithManager
	// sets it when it is nil.
	EventRecorder events.EventRecorder
	// Metrics records the metrics of the framework. SetupWithManager sets it
	// when it is nil.
	Metrics component.MetricsRecorder

	handlers map[string]handler
	schemas  map[string]*apiextensionsv1.JSON
}

// +kubebuilder:rbac:groups=plugins.camunda.io,resources=pluginrequests,verbs=get;list;watch;update;patch
// +kubebuilder:rbac:groups=plugins.camunda.io,resources=pluginrequests/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=plugins.camunda.io,resources=pluginrequests/finalizers,verbs=update
// +kubebuilder:rbac:groups=core.camunda.io,resources=objectstorageconfigs;secondarystorageconfigs;databaseconfigs,verbs=get;list;watch
// +kubebuilder:rbac:groups=events.k8s.io,resources=events,verbs=create;patch

// Reconcile serves a request whose registration label and
// spec.registration name the same built-in registration with a handler. It
// leaves every other request alone. A request under deletion gets the
// deletion policy applied and loses the finalizer. Otherwise the finalizer
// is added first, the parameters are checked against the schema and the
// rules of the registration, and the components of the handler converge.
// OutputsReady comes from the outputs component, and Ready from the backing
// component, or from the outputs when the handler writes nothing. The
// outputs are published only while OutputsReady is True.
//
// Status is written once per reconcile by the deferred flush. The deletion
// path writes no status.
func (r *Reconciler) Reconcile(ctx context.Context, req ctrl.Request) (result ctrl.Result, err error) {
	var request pluginsv1alpha1.PluginRequest
	if err := r.APIReader.Get(ctx, req.NamespacedName, &request); err != nil {
		if apierrors.IsNotFound(err) {
			observability.Forget(r.Metrics, new(pluginsv1alpha1.PluginRequest).GetKind(), req.NamespacedName)
			return ctrl.Result{}, nil
		}

		return ctrl.Result{}, err
	}

	// The label selects the request for the watch, and the spec says which
	// registration serves it. A request where the two differ is not ours.
	h, ok := r.handlers[request.Spec.Registration]
	if !ok || request.Labels[pluginsv1alpha1.LabelRegistration] != request.Spec.Registration {
		return ctrl.Result{}, nil
	}

	rec := component.ReconcileContext{
		Client:        r.Client,
		Scheme:        r.Scheme,
		EventRecorder: r.EventRecorder,
		Metrics:       r.Metrics,
		APIReader:     r.APIReader,
		Owner:         &request,
	}

	if !request.DeletionTimestamp.IsZero() {
		return ctrl.Result{}, r.finalize(ctx, rec, &request, h)
	}

	if controllerutil.AddFinalizer(&request, pluginsv1alpha1.FinalizerCleanup) {
		if err := r.Update(ctx, &request); err != nil {
			if apierrors.IsNotFound(err) {
				return ctrl.Result{}, nil
			}
			return ctrl.Result{}, fmt.Errorf("adding the finalizer: %w", err)
		}
	}

	defer func() {
		result, err = ownerstatus.Flush(ctx, rec, result, err)
	}()

	s, err := r.build(ctx, &request, h)
	var failure *conditions.PreCheckFailure
	if errors.As(err, &failure) {
		refuse(&request, failure)
		return ctrl.Result{}, nil
	}
	if err != nil {
		return ctrl.Result{}, err
	}

	var errs []error
	if s.backing != nil {
		errs = append(errs, s.backing.Reconcile(ctx, rec))
	}
	errs = append(errs, s.outputs.Reconcile(ctx, rec))

	conditions.Stage(&request, conditions.Aggregate(&request, s.readiness()))

	rep := s.report()
	request.Status.Outputs = nil
	if s.outputs.GetCondition(&request).Status == metav1.ConditionTrue {
		request.Status.Outputs = rep.outputs
	}
	setSummary(&request.Status, rep.summary, metav1.Now())
	request.Status.Resources = rep.resources
	request.Status.PodSelector = rep.podSelector

	return ctrl.Result{}, errors.Join(errs...)
}

// build checks the parameters of request and returns the components of h.
func (r *Reconciler) build(ctx context.Context, request *pluginsv1alpha1.PluginRequest, h handler) (served, error) {
	if err := checkParameters(ctx, r.schemas[request.Spec.Registration], request.Spec.Parameters); err != nil {
		return served{}, err
	}

	return h.build(request)
}

// refuse sets OutputsReady and Ready of request to False with the reason
// and the message of failure, and clears the outputs and the display
// fields, in memory.
func refuse(request *pluginsv1alpha1.PluginRequest, failure *conditions.PreCheckFailure) {
	meta.SetStatusCondition(request.GetStatusConditions(), metav1.Condition{
		Type:               pluginsv1alpha1.ConditionOutputsReady,
		Status:             metav1.ConditionFalse,
		Reason:             failure.Reason,
		Message:            conditions.BoundMessage(failure.Message),
		ObservedGeneration: request.Generation,
	})
	conditions.Stage(request, conditions.Failed(request, failure))

	request.Status.Outputs = nil
	request.Status.Resources = nil
	request.Status.PodSelector = nil
	setSummary(&request.Status, nil, metav1.Now())
}

// finalize applies the deletion policy of request and removes the finalizer
// once every remaining object of the handler is gone. It writes no status.
func (r *Reconciler) finalize(
	ctx context.Context,
	rec component.ReconcileContext,
	request *pluginsv1alpha1.PluginRequest,
	h handler,
) error {
	if !controllerutil.ContainsFinalizer(request, pluginsv1alpha1.FinalizerCleanup) {
		return nil
	}

	c, err := h.cleanup(request, request.Spec.DeletionPolicy)
	if err != nil {
		return fmt.Errorf("building the cleanup: %w", err)
	}

	for _, comp := range c.components {
		if err := comp.Reconcile(ctx, rec); err != nil {
			return fmt.Errorf("applying the deletion policy: %w", err)
		}
	}

	// The Owns watch of the kind of a remaining object reconciles the
	// request again when the object goes.
	for _, obj := range c.remaining {
		err := r.APIReader.Get(ctx, client.ObjectKeyFromObject(obj), obj)
		if err == nil {
			return nil
		}
		if !apierrors.IsNotFound(err) {
			return fmt.Errorf("reading %s: %w", client.ObjectKeyFromObject(obj), err)
		}
	}

	controllerutil.RemoveFinalizer(request, pluginsv1alpha1.FinalizerCleanup)
	if err := r.Update(ctx, request); err != nil && !apierrors.IsNotFound(err) {
		return fmt.Errorf("removing the finalizer: %w", err)
	}

	return nil
}

// SetupWithManager returns an error that names each handler without a
// registration in Registrations. Then it registers the contractsField index and the
// controller. The controller runs for a request whose registration label
// names a handler, and for every request that reads a contract that changed.
func (r *Reconciler) SetupWithManager(mgr ctrl.Manager) error {
	if r.Metrics == nil {
		r.Metrics = observability.Recorder(controllerName)
	}
	if r.EventRecorder == nil {
		r.EventRecorder = mgr.GetEventRecorder(controllerName)
	}

	r.handlers = handlers()
	r.schemas = map[string]*apiextensionsv1.JSON{}
	for _, reg := range r.Registrations {
		if _, ok := r.handlers[reg.Name]; ok {
			r.schemas[reg.Name] = reg.Spec.Parameters
		}
	}
	var missing []error
	for _, name := range slices.Sorted(maps.Keys(r.handlers)) {
		if _, ok := r.schemas[name]; !ok {
			missing = append(missing, fmt.Errorf("no built-in registration %q for its handler", name))
		}
	}
	if err := errors.Join(missing...); err != nil {
		return err
	}

	if err := mgr.GetFieldIndexer().IndexField(
		context.Background(), &pluginsv1alpha1.PluginRequest{},
		contractsField, r.contractKeys,
	); err != nil {
		return err
	}

	b := ctrl.NewControllerManagedBy(mgr).
		For(&pluginsv1alpha1.PluginRequest{}, builder.WithPredicates(predicate.NewPredicateFuncs(r.serves)))
	for kind, obj := range map[string]client.Object{
		kindObjectStorageConfig:    &v1.ObjectStorageConfig{},
		kindSecondaryStorageConfig: &v1.SecondaryStorageConfig{},
		kindDatabaseConfig:         &v1.DatabaseConfig{},
	} {
		b = b.Watches(
			obj,
			refindex.Enqueue(mgr.GetClient(), &pluginsv1alpha1.PluginRequestList{}, contractsField, keyOf(kind)),
		)
	}

	return b.Named(controllerName).Complete(r)
}

// contractKeys returns the contractsField keys of a request with a handler.
func (r *Reconciler) contractKeys(o client.Object) []string {
	request, ok := o.(*pluginsv1alpha1.PluginRequest)
	if !ok {
		return nil
	}

	h, ok := r.handlers[request.Spec.Registration]
	if !ok {
		return nil
	}

	var keys []string
	for _, ref := range h.contracts(request) {
		keys = append(keys, contractKey(ref.kind, request.Namespace, ref.name))
	}

	return keys
}

// serves reports whether the registration label of o names a handler.
func (r *Reconciler) serves(o client.Object) bool {
	_, ok := r.handlers[o.GetLabels()[pluginsv1alpha1.LabelRegistration]]
	return ok
}

func keyOf(kind string) func(client.Object) string {
	return func(o client.Object) string { return contractKey(kind, o.GetNamespace(), o.GetName()) }
}

func contractKey(kind, namespace, name string) string {
	return kind + "/" + refindex.NamespacedKey(namespace, name)
}
