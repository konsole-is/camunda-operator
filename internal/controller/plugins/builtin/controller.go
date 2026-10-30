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

	"github.com/go-logr/logr"
	"github.com/sourcehawk/operator-component-framework/pkg/component"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/events"
	"k8s.io/klog/v2"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	pluginsv1alpha1 "github.com/konsole-is/camunda-operator/api/plugins/v1alpha1"
	v1 "github.com/konsole-is/camunda-operator/api/v1"
	"github.com/konsole-is/camunda-operator/internal/controller/plugins/builtin/adapter"
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

// presetsField is the index on PluginRequest with one key for each preset
// that a built-in request reads. No other controller of the manager may
// register an index of this name on PluginRequest.
const presetsField = "pluginrequest.builtin.presets"

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

	handlers map[string]adapter.Handler
	schemas  map[string]*parameterSchema
}

// +kubebuilder:rbac:groups=plugins.camunda.io,resources=pluginrequests,verbs=get;list;watch;update;patch
// +kubebuilder:rbac:groups=plugins.camunda.io,resources=pluginrequests/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=plugins.camunda.io,resources=pluginrequests/finalizers,verbs=update
// +kubebuilder:rbac:groups=plugins.camunda.io,resources=pluginregistrations,verbs=get;list;watch
// A new handler adds its registration here: REG-RBAC and the Grant admission policy check serve.
// +kubebuilder:rbac:groups=plugins.camunda.io,resources=pluginregistrations,verbs=serve,resourceNames=database-server;elasticsearch;existing-object-storage;existing-secondary-storage;existing-database-configs;management-database-server
// +kubebuilder:rbac:groups=core.camunda.io,resources=elasticsearchclusters,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=core.camunda.io,resources=objectstorageconfigs;secondarystorageconfigs;databaseconfigs,verbs=get;list;watch
// +kubebuilder:rbac:groups=core.camunda.io,resources=databaseservers;databases,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=core.camunda.io,resources=databaseserverpresets,verbs=get;list;watch
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

	// The staging below changes the conditions in place, so before is a copy.
	var before metav1.Condition
	if ready := meta.FindStatusCondition(request.Status.Conditions, v1.ConditionReady); ready != nil {
		before = *ready
	}
	defer func() {
		result, err = flush(ctx, rec, before, result, err)
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
	if s.Backing != nil {
		errs = append(errs, s.Backing.Reconcile(ctx, rec))
	}
	errs = append(errs, s.Outputs.Reconcile(ctx, rec))
	// The message of OutputsReady can carry a contract name from the
	// parameters, whose length the schema does not bound.
	if outputs := meta.FindStatusCondition(
		request.Status.Conditions,
		pluginsv1alpha1.ConditionOutputsReady,
	); outputs != nil {
		outputs.Message = conditions.BoundMessage(outputs.Message)
	}

	conditions.Stage(&request, conditions.Aggregate(&request, s.Readiness()))

	rep := s.Report()
	request.Status.Outputs = nil
	if s.Outputs.GetCondition(&request).Status == metav1.ConditionTrue {
		request.Status.Outputs = rep.Outputs
	}
	setSummary(&request.Status, rep.Summary, metav1.Now())
	request.Status.Resources = rep.Resources
	request.Status.PodSelector = rep.PodSelector

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

// flush writes the status of rec.Owner, and logs a change of its Ready
// condition since before only when the write landed. It returns the result
// and the error as ownerstatus.Settle decides them.
func flush(
	ctx context.Context,
	rec component.ReconcileContext,
	before metav1.Condition,
	res ctrl.Result,
	err error,
) (ctrl.Result, error) {
	flushErr := component.FlushStatus(ctx, rec)
	if flushErr == nil {
		after := meta.FindStatusCondition(*rec.Owner.GetStatusConditions(), v1.ConditionReady)
		logReadyChange(logf.FromContext(ctx), before, after)
	}

	return ownerstatus.Settle(res, err, flushErr)
}

// logReadyChange writes one line when the status or the reason of Ready
// changed, so that each request has a line from its first reconcile on.
func logReadyChange(log logr.Logger, before metav1.Condition, after *metav1.Condition) {
	if after == nil || before.Status == after.Status && before.Reason == after.Reason {
		return
	}
	log.Info("Set Ready of PluginRequest", "status", after.Status, "reason", after.Reason)
}

// build checks the parameters of request and returns the components of h.
func (r *Reconciler) build(
	ctx context.Context,
	request *pluginsv1alpha1.PluginRequest,
	h adapter.Handler,
) (adapter.Served, error) {
	if err := r.schemas[request.Spec.Registration].check(ctx, request.Spec.Parameters); err != nil {
		return adapter.Served{}, err
	}

	live, err := r.readBacking(ctx, request, h)
	if err != nil {
		return adapter.Served{}, err
	}

	return h.Build(request, live)
}

// readBacking reads each backing object of request that h names, without
// the cache. A missing object is absent from the result, not an error.
func (r *Reconciler) readBacking(
	ctx context.Context,
	request *pluginsv1alpha1.PluginRequest,
	h adapter.Handler,
) (adapter.LiveObjects, error) {
	live := adapter.LiveObjects{}
	for _, ref := range h.Backing(request) {
		key := client.ObjectKeyFromObject(ref.Object)
		if err := r.APIReader.Get(ctx, key, ref.Object); err != nil {
			if apierrors.IsNotFound(err) {
				continue
			}
			return nil, fmt.Errorf("reading the backing %s %s: %w", ref.Kind, key, err)
		}
		live[adapter.LiveKey{Kind: ref.Kind, Name: key.Name}] = ref.Object
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

// finalize applies the deletion policy of request, and removes the finalizer
// once every remaining object that the request owns is gone. It writes no
// status. waiting is true while an owned remaining object holds the finalizer.
func (r *Reconciler) finalize(
	ctx context.Context,
	rec component.ReconcileContext,
	request *pluginsv1alpha1.PluginRequest,
	h adapter.Handler,
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

	c, err := h.Cleanup(request, request.Spec.DeletionPolicy, live)
	if err != nil {
		return false, fmt.Errorf("building the cleanup: %w", err)
	}

	if c.Backing != nil {
		if err := c.Backing.Reconcile(ctx, rec); err != nil {
			return false, fmt.Errorf("applying the deletion policy: %w", err)
		}
	}

	// An object that the request does not own stays, so it cannot hold the
	// finalizer.
	for _, obj := range c.Remaining {
		err := r.APIReader.Get(ctx, client.ObjectKeyFromObject(obj), obj)
		if err == nil && adapter.OwnedBy(obj, request.UID) {
			return true, nil
		}
		if err != nil && !apierrors.IsNotFound(err) {
			return false, fmt.Errorf("reading %s: %w", client.ObjectKeyFromObject(obj), err)
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

// SetupWithManager returns an error that names each handler without a
// registration in Registrations, or with a parameter schema that it cannot
// read. Then it registers the contractsField, presetsField, and
// registrationField indexes and the controller. The controller runs for a
// request whose registration label names a handler, for a request whose
// ElasticsearchCluster, DatabaseServer, or Database changed, for every
// request that reads a contract or a preset that changed, and for every
// request whose spec.registration names a registration that changed.
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
		presetsField, r.presetKeys,
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
		adapter.KindObjectStorageConfig:    &v1.ObjectStorageConfig{},
		adapter.KindSecondaryStorageConfig: &v1.SecondaryStorageConfig{},
		adapter.KindDatabaseConfig:         &v1.DatabaseConfig{},
	} {
		b = b.Watches(
			obj,
			refindex.Enqueue(mgr.GetClient(), &pluginsv1alpha1.PluginRequestList{}, contractsField, keyOf(kind)),
		)
	}
	for kind, obj := range presetKinds() {
		b = b.Watches(
			obj,
			refindex.Enqueue(mgr.GetClient(), &pluginsv1alpha1.PluginRequestList{}, presetsField, keyOf(kind)),
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
		WithLogConstructor(requestLogger(mgr.GetLogger())).
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
	for _, ref := range h.Contracts(request) {
		keys = append(keys, contractKey(ref.Kind, request.Namespace, ref.Name))
	}

	return keys
}

// presetKeys returns the presetsField keys of a request with a handler.
func (r *Reconciler) presetKeys(o client.Object) []string {
	request, ok := o.(*pluginsv1alpha1.PluginRequest)
	if !ok {
		return nil
	}

	h, ok := r.handlers[request.Spec.Registration]
	if !ok {
		return nil
	}

	presets := presetKinds()
	var keys []string
	for _, ref := range h.Backing(request) {
		if _, ok := presets[ref.Kind]; ok {
			keys = append(keys, contractKey(ref.Kind, ref.Object.GetNamespace(), ref.Object.GetName()))
		}
	}

	return keys
}

// presetKinds returns an empty object of each preset kind that a handler
// reads as a backing object, by kind. A preset kind that a handler only
// names in a backing object is not here: the controller of that object
// reads the preset.
func presetKinds() map[string]client.Object {
	return map[string]client.Object{adapter.KindDatabaseServerPreset: &v1.DatabaseServerPreset{}}
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

// requestLogger returns the logger constructor of the controller. Each line
// about a request, including the reconcile errors of controller-runtime,
// carries the key request with its namespace and name.
func requestLogger(base logr.Logger) func(*reconcile.Request) logr.Logger {
	log := base.WithValues(
		"controller", controllerName,
		"controllerGroup", pluginsv1alpha1.GroupVersion.Group,
		"controllerKind", "PluginRequest",
	)

	return func(req *reconcile.Request) logr.Logger {
		if req == nil {
			return log
		}

		ref := klog.KRef(req.Namespace, req.Name)
		return log.WithValues("PluginRequest", ref, "namespace", req.Namespace, "name", req.Name, "request", ref)
	}
}
