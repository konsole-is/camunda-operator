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

// Package camundaclusterclaim expands a CamundaClusterClaim into one
// PluginRequest for each selection and, once every request with a slot has
// its outputs, a CamundaCluster named after the claim. It refuses a claim
// that its Environment does not allow, and records the policy of the
// Environment on the claim before it writes anything. A deleted claim writes
// its deletion policy on its requests, then deletes what it generated.
package camundaclusterclaim

import (
	"context"
	"errors"
	"fmt"

	"github.com/sourcehawk/operator-component-framework/pkg/component"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/events"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	fleetv1alpha1 "github.com/konsole-is/camunda-operator/api/fleet/v1alpha1"
	v1 "github.com/konsole-is/camunda-operator/api/v1"
	"github.com/konsole-is/camunda-operator/internal/observability"
	"github.com/konsole-is/camunda-operator/internal/ownerstatus"
	"github.com/konsole-is/camunda-operator/internal/pluginstartup"
	components "github.com/konsole-is/camunda-operator/pkg/components/camundaclusterclaim"
	"github.com/konsole-is/camunda-operator/pkg/components/claimrequests"
	"github.com/konsole-is/camunda-operator/pkg/conditions"
)

// controllerName is the name the controller registers with controller-runtime.
// It labels its events and every metrics series it records.
const controllerName = "camundaclusterclaim"

// builtinStorage names the built-in storage whose ServiceAccounts the cloud
// identity of a claim trusts.
var builtinStorage = components.BuiltinStorage{
	ElasticsearchRegistration:  pluginstartup.RegistrationElasticsearch,
	ElasticsearchSuffix:        pluginstartup.BackingSuffixElasticsearch,
	DatabaseServerRegistration: pluginstartup.RegistrationDatabaseServer,
	DatabaseServerSuffix:       pluginstartup.BackingSuffixDatabaseServer,
}

// Reconciler turns a CamundaClusterClaim into its PluginRequests and its
// CamundaCluster.
type Reconciler struct {
	client.Client
	// APIReader reads the claim and the namespace without the cache.
	APIReader client.Reader
	Scheme    *runtime.Scheme
	// EventRecorder publishes the component lifecycle events.
	// SetupWithManager sets it from the manager when it is nil.
	EventRecorder events.EventRecorder
	// Metrics records the condition gauge and the apply counters of the
	// framework. SetupWithManager sets it when it is nil.
	Metrics component.MetricsRecorder

	// componentClient is the uncached client that the components apply
	// through, wrapped so that an apply sends only the fields that the claim
	// and the outputs set. SetupWithManager sets it when it is nil.
	componentClient client.Client
}

// +kubebuilder:rbac:groups=fleet.camunda.io,resources=camundaclusterclaims,verbs=get;list;watch;update;patch
// +kubebuilder:rbac:groups=fleet.camunda.io,resources=camundaclusterclaims/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=fleet.camunda.io,resources=camundaclusterclaims/finalizers,verbs=update
// +kubebuilder:rbac:groups=fleet.camunda.io,resources=environments,verbs=get;list;watch
// +kubebuilder:rbac:groups=plugins.camunda.io,resources=pluginrequests,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=plugins.camunda.io,resources=pluginsections;pluginregistrations,verbs=get;list;watch
// +kubebuilder:rbac:groups=core.camunda.io,resources=camundaclusters,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=core.camunda.io,resources=camundaoptimizes;backupschedules,verbs=get;list;watch;delete
// +kubebuilder:rbac:groups=core.camunda.io,resources=camundaclusterpresets;camundareleases;camundaplatformconfigs,verbs=get;list;watch
// +kubebuilder:rbac:groups="",resources=namespaces,verbs=get;list;watch
// +kubebuilder:rbac:groups=events.k8s.io,resources=events,verbs=create;patch

// Reconcile converges a CamundaClusterClaim. A failed precheck reports its
// reason on Ready and writes nothing else. The first pass that passes the
// precheck adds the finalizer, records status.environment and
// status.resolvedPolicy, and writes no resource, so no request exists before
// the policy of the claim does. Later passes apply the requests, then the
// cluster, and set Ready from RequestsReady and ClusterReady. A deleted claim
// reports Ready=False with reason Deleting until nothing that it generated is
// left, or with reason NotAllowed while a selected registration does not
// support its deletion policy. Status is written at most once per
// reconcile.
func (r *Reconciler) Reconcile(ctx context.Context, req ctrl.Request) (result ctrl.Result, err error) {
	var claim fleetv1alpha1.CamundaClusterClaim
	if err := r.APIReader.Get(ctx, req.NamespacedName, &claim); err != nil {
		if apierrors.IsNotFound(err) {
			observability.Forget(r.Metrics, claim.GetKind(), req.NamespacedName)
			return ctrl.Result{}, nil
		}

		return ctrl.Result{}, err
	}

	rec := component.ReconcileContext{
		Client:        r.componentClient,
		Scheme:        r.Scheme,
		EventRecorder: r.EventRecorder,
		Metrics:       r.Metrics,
		APIReader:     r.APIReader,
		Owner:         &claim,
	}
	// A claim whose finalizer this pass removed is gone, and so is its
	// status.
	finalized := false
	defer func() {
		if finalized {
			return
		}
		result, err = ownerstatus.Flush(ctx, rec, result, err)
	}()

	if !claim.DeletionTimestamp.IsZero() {
		remaining, err := r.finalize(ctx, &claim)
		var failure *conditions.PreCheckFailure
		if errors.As(err, &failure) {
			conditions.Stage(&claim, conditions.Failed(&claim, failure))
			return ctrl.Result{}, nil
		}
		if err != nil {
			return ctrl.Result{}, err
		}
		if len(remaining) == 0 {
			finalized = true
			return ctrl.Result{}, nil
		}

		conditions.Stage(&claim, conditions.Ready(
			metav1.ConditionFalse,
			fleetv1alpha1.ReasonDeleting,
			claimrequests.WaitingMessage(remaining),
			claim.Generation,
		))
		return ctrl.Result{}, nil
	}

	res, err := r.preCheck(ctx, &claim)
	var failure *conditions.PreCheckFailure
	if errors.As(err, &failure) {
		conditions.Stage(&claim, conditions.Failed(&claim, failure))
		return ctrl.Result{}, nil
	}
	if err != nil {
		return ctrl.Result{}, err
	}

	// Record, then act: the flush of this pass writes the policy, and the
	// next pass reads it back before it writes a request.
	if claim.Status.ResolvedPolicy == nil {
		return ctrl.Result{}, r.recordSnapshot(ctx, &claim, res)
	}

	in := components.Input{
		Claim:             &claim,
		ReleaseRef:        res.release,
		PlatformConfigRef: res.platformConfig,
		Order:             res.order,
		Parameters:        res.resolution.Requests,
		Sections:          res.sections,
		Registrations:     res.registrations,
		Requests:          res.requests,
		Live:              res.live,
		Storage:           builtinStorage,
	}
	comps, err := components.Build(in)
	if err != nil {
		return ctrl.Result{}, err
	}

	var reconcileErr error
	for _, comp := range []*component.Component{comps.Requests, comps.Cluster} {
		if err := comp.Reconcile(ctx, rec); err != nil && reconcileErr == nil {
			reconcileErr = err
		}
	}

	// A component joins the messages of its requests, and the claim schema
	// caps the message of a condition.
	conditions.BoundMessages(claim.Status.Conditions)

	// The cluster goes first: on a tie of priority the first component
	// gives the reason, so a claim that waits for outputs reports
	// PrerequisiteNotMet with the message of RequestsReady.
	conditions.Stage(&claim, conditions.Aggregate(&claim, comps.Cluster, comps.Requests))
	claim.Status.Requests = claimrequests.Statuses(&claim, components.RequestsInput(in))
	claim.Status.Cluster = nil
	if res.live != nil && metav1.IsControlledBy(res.live, &claim) {
		claim.Status.Cluster = &v1.ClusterRef{Name: claim.Name}
	}

	return ctrl.Result{}, reconcileErr
}

// recordSnapshot adds the finalizer to claim with an update, then stages
// status.environment and status.resolvedPolicy from res for the flush.
func (r *Reconciler) recordSnapshot(ctx context.Context, claim *fleetv1alpha1.CamundaClusterClaim, res resolved) error {
	// The response of the update replaces the claim in memory, so the
	// finalizer goes on before the policy is staged.
	if controllerutil.AddFinalizer(claim, fleetv1alpha1.FinalizerCleanup) {
		if err := r.Update(ctx, claim); err != nil {
			return fmt.Errorf("adding the finalizer: %w", err)
		}
	}

	claim.Status.Environment = ""
	if res.environment != nil {
		claim.Status.Environment = res.environment.Name
	}
	claim.Status.ResolvedPolicy = &res.resolution.Policy

	return nil
}
