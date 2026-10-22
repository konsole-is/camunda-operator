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
	"time"

	"github.com/sourcehawk/operator-component-framework/pkg/component"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
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

// cleanupRequeue is the wait before finalize runs again. The Owns watches
// enqueue the request only through a controller reference, so an object with
// a plain owner reference sends no event when it goes.
const cleanupRequeue = 5 * time.Second

// contractsField is the index on PluginRequest with one key for each
// contract that a built-in request reads. No other controller of the
// manager may register an index of this name on PluginRequest.
const contractsField = "pluginrequest.builtin.contracts"

// registrationField is the index on PluginRequest by spec.registration, for
// a request with a handler. No other controller of the manager may register
// an index of this name on PluginRequest.
const registrationField = "pluginrequest.builtin.registration"

// Reconciler serves every PluginRequest of a built-in registration that has a
// handler.
type Reconciler struct {
	client.Client
	// APIReader reads without the cache: the request, its registration, its
	// backing objects, and every read of a deletion.
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
	schemas  map[string]*parameterSchema
}

// +kubebuilder:rbac:groups=plugins.camunda.io,resources=pluginrequests,verbs=get;list;watch;update;patch
// +kubebuilder:rbac:groups=plugins.camunda.io,resources=pluginrequests/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=plugins.camunda.io,resources=pluginrequests/finalizers,verbs=update
// +kubebuilder:rbac:groups=plugins.camunda.io,resources=pluginregistrations,verbs=get;list;watch
// A new handler adds its registration here: REG-RBAC and the Grant admission policy check serve.
// +kubebuilder:rbac:groups=plugins.camunda.io,resources=pluginregistrations,verbs=serve,resourceNames=elasticsearch;existing-object-storage;existing-secondary-storage;existing-database-configs;management-database-server
// +kubebuilder:rbac:groups=core.camunda.io,resources=elasticsearchclusters,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=core.camunda.io,resources=objectstorageconfigs;secondarystorageconfigs;databaseconfigs,verbs=get;list;watch
// +kubebuilder:rbac:groups=core.camunda.io,resources=databaseservers;databases,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=events.k8s.io,resources=events,verbs=create;patch

// Reconcile serves a request whose registration label and
// spec.registration name the same built-in registration with a handler, and
// a request that holds the finalizer after its label changed. It leaves
// every other request alone. A request under deletion gets the
// deletion policy applied and loses the finalizer, whatever the state of its
// registration. Any other request waits, with no write, until its
// registration has Ready=True at its current generation. Then the finalizer
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

	h, ok := r.handlers[request.Spec.Registration]
	if !ok || !r.owns(&request) {
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
		waiting, err := r.finalize(ctx, rec, &request, h)
		if err != nil || !waiting {
			return ctrl.Result{}, err
		}
		return ctrl.Result{RequeueAfter: cleanupRequeue}, nil
	}

	ready, err := r.registrationReady(ctx, request.Spec.Registration)
	if err != nil {
		return ctrl.Result{}, err
	}
	if !ready {
		return ctrl.Result{}, nil
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
	// The message of OutputsReady can carry a contract name from the
	// parameters, whose length the schema does not bound.
	if outputs := meta.FindStatusCondition(
		request.Status.Conditions,
		pluginsv1alpha1.ConditionOutputsReady,
	); outputs != nil {
		outputs.Message = conditions.BoundMessage(outputs.Message)
	}

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

// registrationReady reports whether the registration name has Ready=True at
// its current generation. A missing registration is not ready.
func (r *Reconciler) registrationReady(ctx context.Context, name string) (bool, error) {
	var reg pluginsv1alpha1.PluginRegistration
	if err := r.APIReader.Get(ctx, client.ObjectKey{Name: name}, &reg); err != nil {
		if apierrors.IsNotFound(err) {
			return false, nil
		}
		return false, fmt.Errorf("reading the PluginRegistration %q: %w", name, err)
	}

	ready := meta.FindStatusCondition(reg.Status.Conditions, v1.ConditionReady)
	return ready != nil && ready.Status == metav1.ConditionTrue && ready.ObservedGeneration == reg.Generation, nil
}

// build checks the parameters of request and returns the components of h.
func (r *Reconciler) build(ctx context.Context, request *pluginsv1alpha1.PluginRequest, h handler) (served, error) {
	if err := r.schemas[request.Spec.Registration].check(ctx, request.Spec.Parameters); err != nil {
		return served{}, err
	}

	live, err := r.readBacking(ctx, request, h)
	if err != nil {
		return served{}, err
	}

	return h.build(request, live)
}

// readBacking reads each backing object of request that h names, without
// the cache. A missing object is absent from the result, not an error.
func (r *Reconciler) readBacking(
	ctx context.Context,
	request *pluginsv1alpha1.PluginRequest,
	h handler,
) (liveObjects, error) {
	live := liveObjects{}
	for _, ref := range h.backing(request) {
		key := client.ObjectKeyFromObject(ref.object)
		if err := r.APIReader.Get(ctx, key, ref.object); err != nil {
			if apierrors.IsNotFound(err) {
				continue
			}
			return nil, fmt.Errorf("reading the backing %s %s: %w", ref.kind, key, err)
		}
		live[liveKey{kind: ref.kind, name: key.Name}] = ref.object
	}

	return live, nil
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

// finalize applies the deletion policy of request stage by stage. It starts a
// stage only when every remaining object of the stage before it that the
// request owns is gone, and removes the finalizer after the last stage. It
// writes no status. waiting is true while an owned remaining object holds
// the finalizer.
func (r *Reconciler) finalize(
	ctx context.Context,
	rec component.ReconcileContext,
	request *pluginsv1alpha1.PluginRequest,
	h handler,
) (waiting bool, err error) {
	if !controllerutil.ContainsFinalizer(request, pluginsv1alpha1.FinalizerCleanup) {
		return false, nil
	}

	live, err := r.readBacking(ctx, request, h)
	if err != nil {
		return false, err
	}

	// An orphan removes the owner reference that its read finds. A cache that
	// has not seen the reference yet removes nothing, and the garbage
	// collector then deletes the object with the request.
	rec.Client = uncachedReads{Client: rec.Client, reader: r.APIReader}

	c, err := h.cleanup(request, request.Spec.DeletionPolicy, live)
	if err != nil {
		return false, fmt.Errorf("building the cleanup: %w", err)
	}

	for _, stage := range c.stages {
		for _, comp := range stage.components {
			if err := comp.Reconcile(ctx, rec); err != nil {
				return false, fmt.Errorf("applying the deletion policy: %w", err)
			}
		}

		// An object that the request does not own stays, so it cannot hold
		// the finalizer.
		for _, obj := range stage.remaining {
			err := r.APIReader.Get(ctx, client.ObjectKeyFromObject(obj), obj)
			if err == nil && ownedBy(obj, request.UID) {
				return true, nil
			}
			if err != nil && !apierrors.IsNotFound(err) {
				return false, fmt.Errorf("reading %s: %w", client.ObjectKeyFromObject(obj), err)
			}
		}
	}

	controllerutil.RemoveFinalizer(request, pluginsv1alpha1.FinalizerCleanup)
	if err := r.Update(ctx, request); err != nil && !apierrors.IsNotFound(err) {
		return false, fmt.Errorf("removing the finalizer: %w", err)
	}

	return false, nil
}

// uncachedReads reads through reader and writes through Client.
type uncachedReads struct {
	client.Client
	reader client.Reader
}

func (u uncachedReads) Get(
	ctx context.Context,
	key client.ObjectKey,
	obj client.Object,
	opts ...client.GetOption,
) error {
	return u.reader.Get(ctx, key, obj, opts...)
}

func (u uncachedReads) List(ctx context.Context, list client.ObjectList, opts ...client.ListOption) error {
	return u.reader.List(ctx, list, opts...)
}

func ownedBy(obj client.Object, uid types.UID) bool {
	return slices.ContainsFunc(obj.GetOwnerReferences(), func(ref metav1.OwnerReference) bool {
		return ref.UID == uid
	})
}

// SetupWithManager returns an error that names each handler without a
// registration in Registrations, or with a parameter schema that it cannot
// read. Then it registers the contractsField and registrationField indexes
// and the controller. The controller runs for a request whose registration
// label names a handler, for a request whose ElasticsearchCluster,
// DatabaseServer, or Database changed, for every request that reads a
// contract that changed, and for every request whose spec.registration
// names a registration that changed.
func (r *Reconciler) SetupWithManager(mgr ctrl.Manager) error {
	if r.Metrics == nil {
		r.Metrics = observability.Recorder(controllerName)
	}
	if r.EventRecorder == nil {
		r.EventRecorder = mgr.GetEventRecorder(controllerName)
	}

	r.handlers = handlers()
	parameters := map[string]*apiextensionsv1.JSON{}
	for _, reg := range r.Registrations {
		parameters[reg.Name] = reg.Spec.Parameters
	}
	r.schemas = map[string]*parameterSchema{}
	var errs []error
	for _, name := range slices.Sorted(maps.Keys(r.handlers)) {
		raw, ok := parameters[name]
		if !ok {
			errs = append(errs, fmt.Errorf("no built-in registration %q for its handler", name))
			continue
		}
		schema, err := newParameterSchema(raw)
		if err != nil {
			errs = append(errs, fmt.Errorf("built-in registration %q: %w", name, err))
			continue
		}
		r.schemas[name] = schema
	}
	if err := errors.Join(errs...); err != nil {
		return err
	}

	if err := mgr.GetFieldIndexer().IndexField(
		context.Background(), &pluginsv1alpha1.PluginRequest{},
		contractsField, r.contractKeys,
	); err != nil {
		return err
	}

	if err := mgr.GetFieldIndexer().IndexField(
		context.Background(), &pluginsv1alpha1.PluginRequest{},
		registrationField, r.registrationKeys,
	); err != nil {
		return err
	}

	b := ctrl.NewControllerManagedBy(mgr).
		For(&pluginsv1alpha1.PluginRequest{}, builder.WithPredicates(predicate.NewPredicateFuncs(r.serves))).
		Owns(&v1.ElasticsearchCluster{})
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

	return b.
		Owns(&v1.DatabaseServer{}).
		Owns(&v1.Database{}).
		Watches(
			&pluginsv1alpha1.PluginRegistration{},
			refindex.Enqueue(
				mgr.GetClient(),
				&pluginsv1alpha1.PluginRequestList{},
				registrationField,
				refindex.ObjectName,
			),
			builder.WithPredicates(predicate.NewPredicateFuncs(func(o client.Object) bool {
				_, ok := r.handlers[o.GetName()]
				return ok
			})),
		).
		Named(controllerName).
		Complete(r)
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

// registrationKeys returns the registrationField key of a request with a
// handler.
func (r *Reconciler) registrationKeys(o client.Object) []string {
	request, ok := o.(*pluginsv1alpha1.PluginRequest)
	if !ok {
		return nil
	}

	if _, ok := r.handlers[request.Spec.Registration]; !ok {
		return nil
	}

	return []string{request.Spec.Registration}
}

// serves reports whether the registration label of o names a handler, or
// whether o is a request that the controller owns.
func (r *Reconciler) serves(o client.Object) bool {
	if _, ok := r.handlers[o.GetLabels()[pluginsv1alpha1.LabelRegistration]]; ok {
		return true
	}

	request, ok := o.(*pluginsv1alpha1.PluginRequest)
	return ok && r.owns(request)
}

// A request that already holds the finalizer stays owned when its label
// changes later, so that its deletion still completes.
func (r *Reconciler) owns(request *pluginsv1alpha1.PluginRequest) bool {
	if _, ok := r.handlers[request.Spec.Registration]; !ok {
		return false
	}

	return request.Labels[pluginsv1alpha1.LabelRegistration] == request.Spec.Registration ||
		controllerutil.ContainsFinalizer(request, pluginsv1alpha1.FinalizerCleanup)
}

func keyOf(kind string) func(client.Object) string {
	return func(o client.Object) string { return contractKey(kind, o.GetNamespace(), o.GetName()) }
}

func contractKey(kind, namespace, name string) string {
	return kind + "/" + refindex.NamespacedKey(namespace, name)
}
