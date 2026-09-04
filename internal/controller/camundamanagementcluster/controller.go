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

// Package camundamanagementcluster reconciles the CamundaManagementCluster CR.
// The controller runs one management plane: it resolves the platform config
// and the identity provider, converges Management Identity and the components
// that authenticate through it, writes the ManagementAuthConfig that Optimize
// reads, and claims the orchestration clusters that spec.clusterSelector
// matches.
//
// A CamundaCluster never knows that a management plane serves it. The claim
// annotation and the Console ping settings are applied by this controller,
// each under a field manager of its own, and both are withdrawn when the
// cluster leaves the selector or the management cluster is deleted.
package camundamanagementcluster

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/sourcehawk/operator-component-framework/pkg/component"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/events"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	v1 "github.com/konsole-is/camunda-operator/api/v1"
	"github.com/konsole-is/camunda-operator/internal/observability"
	components "github.com/konsole-is/camunda-operator/pkg/components/camundamanagementcluster"
	"github.com/konsole-is/camunda-operator/pkg/conditions"
	"github.com/konsole-is/camunda-operator/pkg/credentials"
	"github.com/konsole-is/camunda-operator/pkg/wrappers/keycloak"
)

// controllerName is the name the controller registers with controller-runtime.
// It labels its events and every metrics series it records.
const controllerName = "camundamanagementcluster"

// Finalizer keeps the CR alive until the ManagementAuthConfig is deleted and
// every claim is withdrawn. Neither is collected by Kubernetes: the contract
// is cluster-scoped and its owner is namespaced, and a claim is an annotation
// on an object of another owner.
const Finalizer = "core.camunda.io/camundamanagementcluster-attachment"

// keycloakKind is the kind of the Keycloak custom resource, which the
// RESTMapper probe asks the Kubernetes cluster for.
const keycloakKind = "Keycloak"

// defaultRetryInterval is how long the controller waits before it calls a
// cluster again whose user API refused the Web Modeler user, and before a
// management plane parked on a realm that another one holds looks at the
// claim again.
const defaultRetryInterval = 30 * time.Second

// retryInterval returns RetryInterval, or defaultRetryInterval when unset.
func (r *Reconciler) retryInterval() time.Duration {
	if r.RetryInterval > 0 {
		return r.RetryInterval
	}

	return defaultRetryInterval
}

// Reconciler turns a CamundaManagementCluster into one management plane: the
// Management Identity Deployment and Service, the copies of referenced
// Secrets, the ManagementAuthConfig, and the claims on the orchestration
// clusters it serves.
type Reconciler struct {
	client.Client
	// APIReader reads without the cache. Secrets are watched metadata-only,
	// so their data and every referenced custom resource are read live, and so
	// is every claim decision.
	APIReader client.Reader
	Scheme    *runtime.Scheme
	// EventRecorder publishes the component lifecycle events and the events of
	// this controller. SetupWithManager sets it from the manager when it is
	// nil.
	EventRecorder events.EventRecorder
	// Metrics records the condition gauge and the apply counters of the
	// framework. SetupWithManager sets it when it is nil.
	Metrics component.MetricsRecorder
	// RetryInterval overrides how long the controller waits before it calls a
	// cluster again whose user API refused it, and before a management plane
	// parked on a realm that another one holds looks at the claim again. No
	// watch reports the recovery of that API, and no plane watches another.
	// Zero means defaultRetryInterval; tests shorten it.
	RetryInterval time.Duration
	// ConvergeInterval overrides how long a cluster that holds the Web Modeler
	// user is left alone before the controller reads it again. Zero means
	// defaultConvergeInterval; tests shorten it.
	ConvergeInterval time.Duration
	// ClaimNamespace holds the claim Leases of every Keycloak realm. One
	// realm can be named from any namespace, so every claimant meets on a
	// Lease of this one namespace. It is the namespace of the operator, and
	// SetupWithManager refuses an empty one.
	ClaimNamespace string

	// componentClient is the uncached client that the ocf components
	// reconcile through. The cached client of the manager must not be used
	// here: the typed Gets of ocf start a cluster-wide Secret informer, which
	// breaks the metadata-only Secret posture of the operator.
	// SetupWithManager sets it when it is nil, wrapped so that a generated
	// credential cannot be republished into a Secret that was deleted, and so
	// that an apply of the Keycloak carries only the fields its schema
	// declares.
	componentClient client.Client
	// keycloakServed reports whether the Kubernetes cluster serves the
	// Keycloak kind of the Keycloak Operator. SetupWithManager probes the
	// RESTMapper once and sets it.
	keycloakServed bool
}

// New builds the reconciler of the management plane. SetupWithManager fills
// what the manager supplies.
func New(c client.Client, apiReader client.Reader, scheme *runtime.Scheme) *Reconciler {
	return &Reconciler{Client: c, APIReader: apiReader, Scheme: scheme}
}

// +kubebuilder:rbac:groups=core.camunda.io,resources=camundamanagementclusters,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=core.camunda.io,resources=camundamanagementclusters/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=core.camunda.io,resources=camundamanagementclusters/finalizers,verbs=update
// +kubebuilder:rbac:groups=core.camunda.io,resources=camundaclusters,verbs=get;list;watch;patch
// +kubebuilder:rbac:groups=core.camunda.io,resources=camundaoptimizes,verbs=get;list;watch
// +kubebuilder:rbac:groups="",resources=namespaces,verbs=get;list;watch
// +kubebuilder:rbac:groups=core.camunda.io,resources=camundaclusters/status,verbs=get
// +kubebuilder:rbac:groups=core.camunda.io,resources=camundaplatformconfigs,verbs=get;list;watch
// +kubebuilder:rbac:groups=core.camunda.io,resources=databaseconfigs,verbs=get;list;watch
// +kubebuilder:rbac:groups=core.camunda.io,resources=databaseserverconfigs,verbs=get;list;watch
// +kubebuilder:rbac:groups=core.camunda.io,resources=managementauthconfigs,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=k8s.keycloak.org,resources=keycloaks,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=k8s.keycloak.org,resources=keycloaks/status,verbs=get
// +kubebuilder:rbac:groups="",resources=configmaps,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="",resources=secrets,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="",resources=services,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="",resources=pods,verbs=list
// +kubebuilder:rbac:groups="",resources=events,verbs=create;patch
// +kubebuilder:rbac:groups=apps,resources=deployments,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=apps,resources=replicasets,verbs=list;delete
// +kubebuilder:rbac:groups=coordination.k8s.io,resources=leases,verbs=get;list;watch;create;update;patch;delete

// Reconcile converges one management plane. A CR under deletion withdraws its
// claims, deletes its contract, and releases the finalizer. Otherwise the
// finalizer is added before the first side effect, the pre-checks resolve every
// reference into the render input, and a failed pre-check reports its Ready
// reason, lets go of the clusters that the selectors no longer match,
// withdraws from a realm that status.callbackRealm names and the spec does
// not, and stops. The Keycloak realm that the spec names is claimed next, in
// both Keycloak modes, and a plane whose realm another plane holds parks the
// same way, under RealmClaimedElsewhere, and looks again on the retry
// interval. Then the
// orchestration clusters are selected and claimed, the CamundaOptimizes
// behind the contract are discovered, the login callbacks are withdrawn from
// a realm that status.callbackRealm names and the spec does not (a
// Keycloak-to-Keycloak move that cannot finish that withdrawal stops here, so
// the components never move Management Identity to a new realm while the old
// one still signs people in), the components converge, every attached cluster
// is pointed at Console, the ManagementAuthConfig is applied, and the login
// callback of every discovered Optimize is registered in the realm. A plane
// that serves at least one Optimize also waits for the login callbacks of the
// realm, and one that serves none reads Ready whatever the realm says.
//
// Ready is True only when every component that takes part in it is True and
// every step of the pass ran. A step is one of the calls above, which the
// controller makes outside the components. A failed step reports StepFailed,
// or WriteFailed for the contract, with the name of the step in the message,
// and the first step that failed is the one Ready names. Every other condition
// keeps the value of the last pass that ran to the end.
//
// Status is written once per reconcile: the components and conditions.Stage
// stage conditions on the in-memory CR, and the deferred FlushStatus persists
// them together. The pass that first records the realm of the login callbacks
// writes it once more, before the components apply, because Management
// Identity registers those callbacks as it starts. A pass that deletes the
// resource writes the status once, to clear that record before it gives the
// realm back.
func (r *Reconciler) Reconcile(ctx context.Context, req ctrl.Request) (_ ctrl.Result, err error) {
	var mc v1.CamundaManagementCluster
	if err := r.APIReader.Get(ctx, req.NamespacedName, &mc); err != nil {
		if apierrors.IsNotFound(err) {
			observability.Forget(r.Metrics, new(v1.CamundaManagementCluster).GetKind(), req.NamespacedName)
			return ctrl.Result{}, nil
		}

		return ctrl.Result{}, err
	}

	if !mc.DeletionTimestamp.IsZero() {
		return ctrl.Result{}, r.finalize(ctx, &mc)
	}

	// The finalizer must exist before the first claim. A deletion between the
	// claim and the next write would leave a cluster claimed by an owner that
	// is gone.
	if controllerutil.AddFinalizer(&mc, Finalizer) {
		if err := r.Update(ctx, &mc); err != nil {
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
		Metrics:       r.Metrics,
		APIReader:     r.APIReader,
		Owner:         &mc,
	}
	// Declared before the deferred flush, so the closure sees every component
	// that the reconcile builds below and FlushStatus owns their conditions.
	var comps []*component.Component
	defer func() {
		if flushErr := component.FlushStatus(ctx, rec, comps); flushErr != nil {
			err = errors.Join(err, flushErr)
		}
	}()

	res, err := r.preCheck(ctx, &mc)
	var failure *conditions.PreCheckFailure
	if errors.As(err, &failure) {
		return r.reconcileUnresolved(ctx, &mc, failure)
	}
	if err != nil {
		return ctrl.Result{}, stepResolveReferences.stop(&mc, err)
	}

	// The realm goes before anything that would touch it: the components
	// render the Management Identity that bootstraps it, and the callback
	// step writes its Optimize client.
	parked, heldRealm, err := r.claimRealm(ctx, &mc, res)
	if err != nil {
		return ctrl.Result{}, stepClaimRealm.stop(&mc, err)
	}
	if parked != nil {
		return r.reconcileParked(ctx, &mc, parked)
	}

	clusters, err := r.listClusters(ctx)
	if err != nil {
		return ctrl.Result{}, stepFindClusters.stop(&mc, err)
	}

	namespaces, err := r.selectedNamespaces(ctx, &mc)
	if err != nil {
		return ctrl.Result{}, stepSelectNamespaces.stop(&mc, err)
	}

	attached, rows, err := r.attachedClusters(ctx, &mc, clusters, namespaces, res.Input.Provider)
	if err != nil {
		return ctrl.Result{}, stepClaimClusters.stop(&mc, err)
	}
	res.Input.Clusters = attached
	mc.Status.Clusters = rows

	if err := r.discoverOptimizes(ctx, &mc, &res); err != nil {
		return ctrl.Result{}, stepDiscoverOptimize.stop(&mc, err)
	}

	target := components.RealmTarget(res.Input.Provider)
	withdrawal, stop, err := r.leaveOldRealm(ctx, rec, &mc, res, target)
	if err != nil {
		return ctrl.Result{}, err
	}
	if stop {
		return ctrl.Result{RequeueAfter: r.retryInterval()}, nil
	}

	// A renamed contract leaves the old name behind until the new contract is
	// written, so the status keeps naming the old one until then.
	previousContract := mc.Status.ManagementAuthConfig
	mc.Status.ManagementAuthConfig = res.ContractName

	built, err := components.Build(res.Input)
	if err != nil {
		return ctrl.Result{}, stepBuildComponents.stop(&mc, err)
	}
	comps = built.Components

	reconcileErr := reconcileComponents(ctx, rec, built.Components)
	// recordInitialClaim overwrites IdentityReady, and readyCondition
	// aggregates that condition. The refusal must therefore reach the CR
	// before the aggregate, or Ready reads True beside it.
	claimErr := stepRecordClaim.wrap(r.recordInitialClaim(ctx, &mc, res.Input.Provider.Mode))
	userErr := stepWebModelerUsers.wrap(r.syncWebModelerUsers(ctx, &mc, clusters, attached, rows))
	pingErr := stepPing.wrap(r.syncPing(ctx, &mc, clusters, attached))
	// The claim of a cluster that left the selector goes last, once its user
	// and its ping are gone, so that no other management plane adopts a user
	// this one still has to remove.
	var releaseErr error
	if userErr == nil && pingErr == nil {
		releaseErr = stepReleaseClaims.wrap(r.releaseClaims(ctx, &mc, clusters, namespaces))
	}
	contractErr := r.writeContract(ctx, &mc, res)
	if contractErr == nil && previousContract != "" && previousContract != res.ContractName {
		// The new contract is written, so the old one goes. A failed
		// withdrawal keeps the old name on the status, which is what makes
		// the next reconcile withdraw it again.
		if err := r.withdrawContractNamed(ctx, &mc, previousContract); err != nil {
			mc.Status.ManagementAuthConfig = previousContract
			contractErr = err
		}
	}
	callbackFailure, callbackRetry, callbackErr := r.syncOptimizeCallbacks(
		ctx, &mc, res, contractErr, withdrawal,
	)
	contractErr = stepWriteContract.wrapAs(v1.ReasonWriteFailed, contractErr)
	callbackErr = stepOptimizeCallbacks.wrap(callbackErr)
	conditions.Stage(&mc, readyCondition(
		&mc,
		built.Ready,
		firstStep(claimErr, userErr, pingErr, releaseErr, contractErr, callbackErr),
		callbackFailure,
	))

	// Nothing watches the user API of a cluster or the Optimize client of the
	// realm, so the controller comes back on its own: soon when a cluster or
	// Keycloak refused the call, and on the converge interval to find a user
	// or a login callback that somebody removed there.
	var result ctrl.Result
	switch {
	case anyRow(rows, v1.ReasonBasicAuthUserFailed), callbackRetry, heldRealm:
		result.RequeueAfter = r.retryInterval()
	case convergesUsers(&mc, attached), len(res.Input.OptimizeURLs) > 0:
		result.RequeueAfter = r.convergeInterval()
	}

	return result, errors.Join(
		reconcileErr, claimErr, userErr, pingErr, releaseErr, contractErr, callbackErr,
	)
}

// leaveOldRealm tidies the realm that status.callbackRealm names and the spec
// does not, and records the realm that the components are about to point
// Management Identity at. It runs before the components move Identity to the
// new Keycloak: a pod of the new revision registers the login callbacks in the
// new realm as it starts, and they must not appear there while the old realm
// still refuses to let go of its own. A move to the oidc mode is not held:
// Management Identity writes no realm there, so there is no second
// registration to hold back.
//
// target is the realm of the spec, nil in the oidc mode. It returns the
// withdrawal that is still pending, for the caller to fold into Ready, and
// whether the pass ends here and comes back on the retry interval.
func (r *Reconciler) leaveOldRealm(
	ctx context.Context,
	rec component.ReconcileContext,
	mc *v1.CamundaManagementCluster,
	res resolved,
	target *v1.KeycloakRealmTarget,
) (*conditions.PreCheckFailure, bool, error) {
	withdrawal, consumed, err := r.withdrawRetargeted(ctx, mc, target, res.Input.Suspended)
	if err != nil {
		return nil, false, stepWithdrawCallbacks.stop(mc, err)
	}
	// A record that the forget annotation consumed goes out with this pass's
	// status flush, and the annotation is removed on the next pass, once the
	// record it consumed is gone from the API server. Registering in a new
	// realm now would replace the record first, and the next pass would then
	// report the spent annotation as one that names a foreign realm.
	if consumed {
		return nil, true, nil
	}
	if withdrawal != nil && target != nil && len(res.Input.OptimizeURLs) > 0 {
		// The plane stays where it is until the old realm and its writers are
		// gone, so the callbacks never fill the new realm beside a realm that
		// still signs people in. A plane that serves no Optimize is not held,
		// and neither is a move to the oidc mode: neither fills the new realm
		// with anything, so there is no second registration to hold back, and
		// only the condition reports the realm still to be emptied.
		stageCallbacks(mc, metav1.ConditionFalse, withdrawal.Reason, withdrawal.Message)
		conditions.Stage(mc, conditions.Failed(mc, withdrawal))

		return nil, true, nil
	}
	if err := r.recordCallbackRealm(ctx, rec, mc, res, target); err != nil {
		return nil, false, err
	}

	return withdrawal, false, nil
}

// reconcileUnresolved is the pass of a management plane whose pre-check
// failed. It reports the failure and runs the three halves that need nothing
// the pre-check resolves: the release of the realm claims that nothing of the
// plane names any more, the withdrawal from a recorded realm that the spec no
// longer names, and the release of the clusters that left the selector. The
// old realm would otherwise keep the login callbacks, and the claims of the
// realms this plane left would keep every later claimant out, for as long as
// the failure stands.
func (r *Reconciler) reconcileUnresolved(
	ctx context.Context,
	mc *v1.CamundaManagementCluster,
	failure *conditions.PreCheckFailure,
) (ctrl.Result, error) {
	conditions.Stage(mc, conditions.Failed(mc, failure))

	// The sweep runs before the withdrawal, where the claim step runs before
	// it on the resolved path, so that it still reads the recorded realm.
	heldRealm, sweepErr := r.releaseUnusedRealms(ctx, r.realmClaims(), mc)
	sweepErr = stepClaimRealm.wrap(sweepErr)
	retry, withdrawErr := r.withdrawStopped(
		ctx, mc, "the identity provider of the spec is not resolved",
	)
	callbackErr := stepWithdrawCallbacks.wrap(withdrawErr)
	releaseErr := stepReleaseClaims.wrap(r.withdrawFromDeselected(ctx, mc))
	// A step that failed says more than the pre-check it ran beside: the
	// pre-check names something of the spec, and the step names a call that
	// the operator could not make at all.
	if failed := firstStep(sweepErr, callbackErr, releaseErr); failed != nil {
		conditions.Stage(mc, failed.condition(mc))

		// A result beside a non-nil error is dropped by controller-runtime,
		// so the retry is dropped here too. A failure requeues with backoff
		// on its own.
		return ctrl.Result{}, errors.Join(sweepErr, callbackErr, releaseErr)
	}
	if retry || heldRealm {
		return ctrl.Result{RequeueAfter: r.retryInterval()}, nil
	}

	return ctrl.Result{}, nil
}

// reconcileParked is the pass of a management plane whose realm another plane
// holds. It renders nothing, so the plane stays where it is, and it runs the
// two halves that a park does not stop: the withdrawal from a recorded realm
// that the spec no longer names, and the release of the clusters that left the
// selector. The old realm would otherwise keep signing people in for as long
// as the park stands. OptimizeCallbacksReady reports the park too, because a
// plane that lost the claim registers nothing in that realm any more.
func (r *Reconciler) reconcileParked(
	ctx context.Context,
	mc *v1.CamundaManagementCluster,
	parked *conditions.PreCheckFailure,
) (ctrl.Result, error) {
	conditions.Stage(mc, conditions.Failed(mc, parked))
	// The plane registers nothing in a realm it does not hold, so the callbacks
	// of the last pass that ran must not keep reading Healthy over a realm that
	// another plane converges now. The withdrawal below overwrites this with
	// what it found, where it has something more exact to say.
	stageCallbacks(mc, metav1.ConditionFalse, parked.Reason, parked.Message)

	_, withdrawErr := r.withdrawStopped(
		ctx, mc, "the realm of the spec answers to another management plane",
	)
	callbackErr := stepWithdrawCallbacks.wrap(withdrawErr)
	releaseErr := stepReleaseClaims.wrap(r.withdrawFromDeselected(ctx, mc))
	if failed := firstStep(callbackErr, releaseErr); failed != nil {
		conditions.Stage(mc, failed.condition(mc))

		// A result beside a non-nil error is dropped by controller-runtime, so
		// the interval is dropped here too. A failure requeues with backoff.
		return ctrl.Result{}, errors.Join(callbackErr, releaseErr)
	}

	// Nothing watches the Lease, and a plane never watches another plane, so a
	// parked plane finds a released realm on its own.
	return ctrl.Result{RequeueAfter: r.retryInterval()}, nil
}

// withdrawFromDeselected takes back what the management plane put on the
// orchestration clusters that spec.clusterSelector and spec.namespaceSelector
// no longer match: the Web Modeler user, the Console ping settings, and the
// claim.
//
// It is the half of the reconcile that a failed pre-check does not stop.
// Nothing here reads a resolved reference, and a claim that waits for a broken
// one keeps another management plane from taking the cluster.
//
// The order is the order of the attached path: the user and the ping first,
// the claim last and only once both are gone, so that no other management
// plane adopts a cluster whose user this one still has to remove.
func (r *Reconciler) withdrawFromDeselected(
	ctx context.Context,
	mc *v1.CamundaManagementCluster,
) error {
	clusters, err := r.listClusters(ctx)
	if err != nil {
		return err
	}

	namespaces, err := r.selectedNamespaces(ctx, mc)
	if err != nil {
		return err
	}

	selected, err := selectedClusters(mc, clusters, namespaces)
	if err != nil {
		return err
	}

	users := make(map[types.UID]bool, len(selected))
	pings := make(map[client.ObjectKey]bool, len(selected))
	for _, cluster := range selected {
		users[cluster.UID] = true
		pings[client.ObjectKeyFromObject(cluster)] = true
	}

	userErr := r.withdrawUnservedUsers(ctx, mc, clusters, users, false)
	pingErr := r.withdrawPingUnserved(ctx, mc, clusters, pings)
	if err := errors.Join(userErr, pingErr); err != nil {
		return err
	}

	return r.releaseClaims(ctx, mc, clusters, namespaces)
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

// recordInitialClaim records the initial administrator claim that Management
// Identity started with, and reports a later change to it.
//
// Identity reads the claim as it boots and stores the result in its database.
// The annotation is what keeps the rendered environment on the value that
// Identity actually holds; without it a later edit of spec.identity.admin
// would roll the pods into a claim that changes nothing.
//
// The recorded claim comes from the Identity pod that started first, not from
// the spec and not from the readiness of the Deployment. Identity writes the
// administrator to its database before the pod is ready, so an edit of
// spec.identity.admin in that window would otherwise record a claim that the
// database never held.
//
// Only the oidc mode has a claim. The two Keycloak modes name a user instead,
// and Identity creates that user on its first start, so a later change to
// spec.identity.admin.username creates a second one.
func (r *Reconciler) recordInitialClaim(
	ctx context.Context,
	mc *v1.CamundaManagementCluster,
	mode components.ProviderMode,
) error {
	if mode != components.ModeOIDC {
		return nil
	}

	recorded := components.RecordedInitialClaim(mc)
	if recorded == "" {
		started, err := r.startedInitialClaim(ctx, mc)
		if err != nil {
			return err
		}
		// A management plane whose Identity never ran holds no administrator
		// anywhere, so a claim the user corrects before the first start
		// leaves no record behind.
		if started == "" {
			return nil
		}

		return r.recordAnnotation(ctx, mc, started)
	}
	if recorded == components.SpecInitialClaim(mc) {
		return nil
	}

	admin := mc.Spec.Identity.Admin
	meta.SetStatusCondition(mc.GetStatusConditions(), metav1.Condition{
		Type:   v1.ConditionIdentityReady,
		Status: metav1.ConditionFalse,
		Reason: v1.ReasonImmutableAfterStart,
		Message: fmt.Sprintf(
			"Management Identity started with the administrator claim %q and stores it in its database; "+
				"spec.identity.admin now asks for %q, which only a change in the database can do",
			recorded, admin.ClaimName+"="+admin.ClaimValue,
		),
		ObservedGeneration: mc.GetGeneration(),
	})

	return nil
}

// startedInitialClaim returns the administrator claim that the Management
// Identity pod that started first carries, or empty while none of them has
// started.
//
// The read goes through APIReader. The pod cache of the operator holds only
// the pods that carry its managed-by label, and the Identity pods carry the
// discovery labels alone. No watch reports a started pod either, so the record
// lands on the next reconcile that another event brings. The edit of
// spec.identity.admin that opens the window is one of them, and the pod that
// started is still running then.
func (r *Reconciler) startedInitialClaim(
	ctx context.Context,
	mc *v1.CamundaManagementCluster,
) (string, error) {
	var pods corev1.PodList
	if err := r.APIReader.List(
		ctx,
		&pods,
		client.InNamespace(mc.Namespace),
		client.MatchingLabels(components.IdentityPodLabels(mc)),
	); err != nil {
		return "", fmt.Errorf("listing the Management Identity pods: %w", err)
	}

	return components.StartedInitialClaim(pods.Items), nil
}

// recordAnnotation writes the recorded claim onto the CR. It is a patch of one
// annotation rather than an update of the whole object, so it cannot write
// back a field that another writer changed in the meantime.
func (r *Reconciler) recordAnnotation(
	ctx context.Context,
	mc *v1.CamundaManagementCluster,
	claim string,
) error {
	body, err := json.Marshal(map[string]any{
		"metadata": map[string]any{
			"annotations": map[string]string{components.InitialClaimAnnotation: claim},
		},
	})
	if err != nil {
		return fmt.Errorf("building the initial administrator claim patch: %w", err)
	}

	// The patch goes through a copy. A write decodes the answer of the API
	// server into the object it was given, and that answer carries the stored
	// status, which would drop every condition the reconcile has staged so far.
	patched := mc.DeepCopy()
	if err := r.Patch(ctx, patched, client.RawPatch(types.MergePatchType, body)); err != nil {
		if apierrors.IsNotFound(err) {
			return nil
		}
		return fmt.Errorf("recording the initial administrator claim: %w", err)
	}
	mc.ObjectMeta = patched.ObjectMeta

	return nil
}

// removeAnnotation takes one annotation off the CR, through a copy for the
// reason recordAnnotation gives. The patch carries the resource version, so a
// concurrent write conflicts and the reconcile retries instead of writing
// back an annotation somebody just set.
func (r *Reconciler) removeAnnotation(
	ctx context.Context,
	mc *v1.CamundaManagementCluster,
	key string,
) error {
	patched := mc.DeepCopy()
	delete(patched.Annotations, key)
	patch := client.MergeFromWithOptions(mc, client.MergeFromWithOptimisticLock{})
	if err := r.Patch(ctx, patched, patch); err != nil {
		if apierrors.IsNotFound(err) {
			return nil
		}
		return fmt.Errorf("removing the %s annotation: %w", key, err)
	}
	mc.ObjectMeta = patched.ObjectMeta

	return nil
}

// writeContract applies the ManagementAuthConfig and reports the write on
// ManagementAuthReady.
func (r *Reconciler) writeContract(
	ctx context.Context,
	mc *v1.CamundaManagementCluster,
	res resolved,
) error {
	err := r.applyContract(ctx, mc, components.ManagementAuthSpec(res.Input))

	condition := metav1.Condition{
		Type:               v1.ConditionManagementAuthReady,
		Status:             metav1.ConditionTrue,
		Reason:             v1.ReasonHealthy,
		Message:            fmt.Sprintf("ManagementAuthConfig %q is up to date", res.ContractName),
		ObservedGeneration: mc.GetGeneration(),
	}
	if err != nil {
		condition.Status = metav1.ConditionFalse
		condition.Reason = v1.ReasonWriteFailed
		condition.Message = conditions.BoundMessage(err.Error())
	}
	meta.SetStatusCondition(mc.GetStatusConditions(), condition)

	return err
}

// anyRow reports whether a row of status.clusters carries reason.
func anyRow(rows []v1.AttachedClusterStatus, reason string) bool {
	for _, row := range rows {
		if row.Reason == reason {
			return true
		}
	}

	return false
}

// readyCondition derives Ready from the steps of the reconcile, the
// components, and the registration of the Optimize callbacks. Neither a step
// nor the callbacks are a component, and neither is a ready management plane
// when it fails: a step that did not run left a claim, a user, a ping, or the
// contract behind, and an Optimize whose callback is missing from the realm
// cannot complete a login.
//
// A failed step decides Ready first, because it reports nowhere else. failed
// is the first step that failed in reconcile order, and it carries the reason
// it reports: the contract write keeps the documented WriteFailed, every other
// step reports StepFailed.
//
// A component that is not True yet decides Ready before the callbacks do. The
// realm is bootstrapped by Management Identity against Keycloak, so a plane
// that is still starting cannot register anything, and reporting that instead
// of the component that is starting would name the symptom over the cause.
func readyCondition(
	mc *v1.CamundaManagementCluster,
	comps []*component.Component,
	failed *stepError,
	callbackFailure *conditions.PreCheckFailure,
) metav1.Condition {
	if failed != nil {
		return failed.condition(mc)
	}

	ready := conditions.Aggregate(mc, comps...)
	if ready.Status == metav1.ConditionTrue && callbackFailure != nil {
		return conditions.Failed(mc, callbackFailure)
	}

	return ready
}

// SetupWithManager registers the controller, the reference indexes, and the
// watches. It refuses a reconciler without a namespace for the realm claim
// Leases. It also sets EventRecorder and Metrics when they are nil, builds
// the uncached component client, and probes once whether the Kubernetes
// cluster serves the Keycloak kind.
func (r *Reconciler) SetupWithManager(mgr ctrl.Manager) error {
	if r.ClaimNamespace == "" {
		return errors.New("the namespace of the realm claim Leases is required")
	}

	if r.EventRecorder == nil {
		r.EventRecorder = mgr.GetEventRecorder(controllerName)
	}
	if r.Metrics == nil {
		r.Metrics = observability.Recorder(controllerName)
	}
	if r.componentClient == nil {
		componentClient, err := client.New(mgr.GetConfig(), client.Options{
			Scheme: mgr.GetScheme(),
			Mapper: mgr.GetRESTMapper(),
		})
		if err != nil {
			return fmt.Errorf("building the component client: %w", err)
		}
		// The credentials wrapper enforces the precondition of a reused
		// generated credential, so a delete of its Secret rotates it. The
		// Keycloak wrapper sanitizes the apply of the Keycloak custom
		// resource, whose CRD schema refuses what the typed struct serializes.
		r.componentClient = keycloak.NewApplyClient(credentials.NewApplyClient(componentClient))
	}
	r.keycloakServed = keycloakKindServed(mgr.GetRESTMapper())

	return r.setupWatches(mgr)
}

// keycloakKindServed reports whether the Kubernetes cluster serves the
// Keycloak kind. On a cluster without the Keycloak Operator the pre-check
// reports KeycloakOperatorNotInstalled instead of failing every reconcile.
func keycloakKindServed(mapper meta.RESTMapper) bool {
	_, err := mapper.RESTMapping(
		schema.GroupKind{Group: keycloak.GroupVersion.Group, Kind: keycloakKind},
		keycloak.GroupVersion.Version,
	)

	return err == nil
}
