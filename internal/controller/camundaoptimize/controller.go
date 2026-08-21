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

// Package camundaoptimize reconciles the CamundaOptimize CR. The controller
// attaches one Optimize instance to one CamundaCluster: it resolves the
// references of the cluster, turns on the Elasticsearch exporter that writes
// the records Optimize reads, and converges the webapp and importer
// Deployments. It never reads the internals of the cluster controller, and the
// cluster never knows that an Optimize is attached to it.
//
// One cluster carries one Optimize instance, see attachmentHolder.
package camundaoptimize

import (
	"context"
	"errors"
	"fmt"

	"github.com/sourcehawk/operator-component-framework/pkg/component"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/tools/events"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	v1 "github.com/konsole-is/camunda-operator/api/v1"
	components "github.com/konsole-is/camunda-operator/pkg/components/camundaoptimize"
	"github.com/konsole-is/camunda-operator/pkg/conditions"
)

// Finalizer keeps the CR alive until the exporter patch is withdrawn. Without
// it a deleted CamundaOptimize would leave the cluster exporting records that
// nothing reads.
const Finalizer = "core.camunda.io/camundaoptimize-exporter"

// The event vocabulary of this controller.
const (
	// eventActionFinalize is the action of the events that the controller
	// records while it deletes a CamundaOptimize.
	eventActionFinalize = "Finalize"
	// eventReasonExporterRemoved is recorded when the finalizer withdraws the
	// exporter patch.
	eventReasonExporterRemoved = "ExporterRemoved"
)

// Reconciler turns a CamundaOptimize into the Optimize workloads of one
// CamundaCluster: the webapp Deployment, the importer Deployment, their
// Services and ServiceMonitors, and the copies of referenced Secrets from
// other namespaces. It also patches the exporter settings onto the cluster.
type Reconciler struct {
	client.Client
	// APIReader reads without the cache. Secrets are watched metadata-only,
	// so their data and every referenced custom resource are read live.
	APIReader client.Reader
	Scheme    *runtime.Scheme
	// EventRecorder publishes the component lifecycle events and the events of
	// this controller. SetupWithManager sets it from the manager when it is
	// nil.
	EventRecorder events.EventRecorder

	// componentClient is the uncached client that the ocf components
	// reconcile through. The cached client of the manager must not be used
	// here: the typed Gets of ocf start a cluster-wide Secret informer, which
	// breaks the metadata-only Secret posture of the operator.
	// SetupWithManager sets it when it is nil.
	componentClient client.Client
	// restMapper resolves whether the cluster serves the ServiceMonitor kind.
	// SetupWithManager sets it from the manager.
	restMapper meta.RESTMapper
}

// +kubebuilder:rbac:groups=core.camunda.io,resources=camundaoptimizes,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=core.camunda.io,resources=camundaoptimizes/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=core.camunda.io,resources=camundaoptimizes/finalizers,verbs=update
// +kubebuilder:rbac:groups=core.camunda.io,resources=camundaclusters,verbs=get;list;watch;patch
// +kubebuilder:rbac:groups=core.camunda.io,resources=camundaclusterpresets,verbs=get;list;watch
// +kubebuilder:rbac:groups=core.camunda.io,resources=camundaplatformconfigs,verbs=get;list;watch
// +kubebuilder:rbac:groups=core.camunda.io,resources=managementauthconfigs,verbs=get;list;watch
// +kubebuilder:rbac:groups=core.camunda.io,resources=secondarystorageconfigs,verbs=get;list;watch
// +kubebuilder:rbac:groups="",resources=secrets,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="",resources=services,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="",resources=events,verbs=create;patch
// +kubebuilder:rbac:groups=apps,resources=deployments,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=monitoring.coreos.com,resources=servicemonitors,verbs=get;list;watch;create;update;patch;delete

// Reconcile converges a CamundaOptimize. A CR under deletion withdraws the
// exporter patch and releases the finalizer. Otherwise the finalizer is added
// before the first side effect, the pre-checks resolve every reference into
// the render input, and a failed pre-check reports its Ready reason and stops.
// Then the exporter patch turns the Elasticsearch exporter of the referenced
// cluster on, and the components converge: the copies of referenced Secrets,
// the webapp, the importer. Ready is True only when every component that takes
// part in it is True.
//
// Status is written once per reconcile: the components and conditions.Stage
// stage conditions on the in-memory CR, and the deferred FlushStatus persists
// them together.
func (r *Reconciler) Reconcile(ctx context.Context, req ctrl.Request) (_ ctrl.Result, err error) {
	var optimize v1.CamundaOptimize
	if err := r.APIReader.Get(ctx, req.NamespacedName, &optimize); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	if !optimize.DeletionTimestamp.IsZero() {
		return ctrl.Result{}, r.finalize(ctx, &optimize)
	}

	// The finalizer must exist before the exporter patch. A deletion between
	// the patch and the next write would leave the cluster exporting.
	if controllerutil.AddFinalizer(&optimize, Finalizer) {
		if err := r.Update(ctx, &optimize); err != nil {
			// A deletion that races this write is fine. The deletion path owns
			// the object from here.
			if apierrors.IsNotFound(err) {
				return ctrl.Result{}, nil
			}
			return ctrl.Result{}, fmt.Errorf("adding the finalizer: %w", err)
		}
	}

	rec := component.ReconcileContext{
		Client:        r.componentClient,
		Scheme:        r.Scheme,
		EventRecorder: r.EventRecorder,
		APIReader:     r.APIReader,
		Owner:         &optimize,
	}
	// Declared before the deferred flush, so the closure sees every component
	// that the reconcile builds below and FlushStatus owns their conditions.
	var comps []*component.Component
	defer func() {
		if flushErr := component.FlushStatus(ctx, rec, comps); flushErr != nil {
			err = errors.Join(err, flushErr)
		}
	}()

	res, err := r.preCheck(ctx, &optimize)
	var failure *conditions.PreCheckFailure
	if errors.As(err, &failure) {
		conditions.Stage(&optimize, conditions.Failed(&optimize, failure))
		// A CamundaOptimize that lost the attachment must not keep the
		// workloads it built while it held it, see releaseWorkloads.
		if failure.Reason == v1.ReasonClusterAlreadyAttached {
			// The component conditions describe workloads that the next call
			// deletes, and comps stays nil on this path, so the flush does not
			// own those types and would write the stale values back. A parked
			// CamundaOptimize renders nothing, so it reports nothing about
			// what it used to render.
			removeComponentConditions(&optimize)
			return ctrl.Result{}, r.releaseWorkloads(ctx, &optimize)
		}

		return ctrl.Result{}, nil
	}
	if err != nil {
		return ctrl.Result{}, err
	}

	res.Input.ServiceMonitorSupported = r.serviceMonitorSupported()
	// Before anything stages a new Ready, which is where the previous
	// suspension state is read from.
	r.recordSuspensionChange(&optimize, res.Input.Suspended)

	if err := r.patchExporter(ctx, res); err != nil {
		return ctrl.Result{}, err
	}

	built, err := r.buildComponents(res)
	if err != nil {
		return ctrl.Result{}, err
	}
	comps = built.all

	reconcileErr := reconcileComponents(ctx, rec, built.all)
	conditions.Stage(&optimize, conditions.Aggregate(&optimize, built.ready...))

	return ctrl.Result{}, reconcileErr
}

// optimizeComponents are the components of one CamundaOptimize: all of them
// are reconciled in order, and the ready ones make up Ready.
type optimizeComponents struct {
	all   []*component.Component
	ready []*component.Component
}

// buildComponents builds every component in reconcile order: the copies of
// referenced Secrets, then the two workloads. The copies component is always
// built, so a reference that moved into this namespace has its copy deleted,
// and it takes part in Ready only when there is a copy to make.
func (r *Reconciler) buildComponents(res resolved) (optimizeComponents, error) {
	mirrored, err := components.MirroredSecretComponent(res.Input.Optimize, res.Mirrors)
	if err != nil {
		return optimizeComponents{}, fmt.Errorf("building mirrored secrets component: %w", err)
	}

	workloads, err := components.Build(res.Input)
	if err != nil {
		return optimizeComponents{}, fmt.Errorf("building workload components: %w", err)
	}

	comps := optimizeComponents{all: append([]*component.Component{mirrored}, workloads...)}
	comps.ready = workloads
	if len(res.Mirrors) > 0 {
		comps.ready = append([]*component.Component{mirrored}, workloads...)
	}

	return comps, nil
}

// reconcileComponents reconciles comps in order. It continues past a failing
// component, so one failure does not stall the rest, and returns the first
// error.
func reconcileComponents(ctx context.Context, rec component.ReconcileContext, comps []*component.Component) error {
	var firstErr error
	for _, comp := range comps {
		if err := comp.Reconcile(ctx, rec); err != nil && firstErr == nil {
			firstErr = err
		}
	}

	return firstErr
}

// serviceMonitorSupported reports whether the Kubernetes cluster serves the
// ServiceMonitor kind. On a cluster without the prometheus-operator CRDs the
// components then omit the resource instead of failing every reconcile.
func (r *Reconciler) serviceMonitorSupported() bool {
	if r.restMapper == nil {
		return false
	}

	_, err := r.restMapper.RESTMapping(
		schema.GroupKind{Group: "monitoring.coreos.com", Kind: "ServiceMonitor"}, "v1",
	)

	return err == nil
}
