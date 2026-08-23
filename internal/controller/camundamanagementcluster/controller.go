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
	components "github.com/konsole-is/camunda-operator/pkg/components/camundamanagementcluster"
	"github.com/konsole-is/camunda-operator/pkg/conditions"
	"github.com/konsole-is/camunda-operator/pkg/credentials"
	"github.com/konsole-is/camunda-operator/pkg/wrappers/keycloak"
)

// Finalizer keeps the CR alive until the ManagementAuthConfig is deleted and
// every claim is withdrawn. Neither is collected by Kubernetes: the contract
// is cluster-scoped and its owner is namespaced, and a claim is an annotation
// on an object of another owner.
const Finalizer = "core.camunda.io/camundamanagementcluster-attachment"

// keycloakKind is the kind of the Keycloak custom resource, which the
// RESTMapper probe asks the Kubernetes cluster for.
const keycloakKind = "Keycloak"

// defaultRetryInterval is how long the controller waits before it calls a
// cluster again whose user API refused the Web Modeler user.
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
	// RetryInterval overrides how long the controller waits before it calls a
	// cluster again whose user API refused it. No watch reports the recovery
	// of that API. Zero means defaultRetryInterval; tests shorten it.
	RetryInterval time.Duration
	// ConvergeInterval overrides how long a cluster that holds the Web Modeler
	// user is left alone before the controller reads it again. Zero means
	// defaultConvergeInterval; tests shorten it.
	ConvergeInterval time.Duration

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
// +kubebuilder:rbac:groups="",resources=namespaces,verbs=get;list;watch
// +kubebuilder:rbac:groups=core.camunda.io,resources=camundaclusters/status,verbs=get
// +kubebuilder:rbac:groups=core.camunda.io,resources=camundaplatformconfigs,verbs=get;list;watch
// +kubebuilder:rbac:groups=core.camunda.io,resources=databaseconfigs,verbs=get;list;watch
// +kubebuilder:rbac:groups=core.camunda.io,resources=databaseserverconfigs,verbs=get;list;watch
// +kubebuilder:rbac:groups=core.camunda.io,resources=managementauthconfigs,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=k8s.keycloak.org,resources=keycloaks,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=k8s.keycloak.org,resources=keycloaks/status,verbs=get
// +kubebuilder:rbac:groups="",resources=secrets,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="",resources=services,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="",resources=events,verbs=create;patch
// +kubebuilder:rbac:groups=apps,resources=deployments,verbs=get;list;watch;create;update;patch;delete

// Reconcile converges one management plane. A CR under deletion withdraws its
// claims, deletes its contract, and releases the finalizer. Otherwise the
// finalizer is added before the first side effect, the pre-checks resolve every
// reference into the render input, and a failed pre-check reports its Ready
// reason and stops. Then the orchestration clusters are selected and claimed,
// the components converge, every attached cluster is pointed at Console, and
// the ManagementAuthConfig is applied. Ready is
// True only when every component that takes part in it is True and the
// contract is written; a failed write reports WriteFailed.
//
// Status is written once per reconcile: the components and conditions.Stage
// stage conditions on the in-memory CR, and the deferred FlushStatus persists
// them together.
func (r *Reconciler) Reconcile(ctx context.Context, req ctrl.Request) (_ ctrl.Result, err error) {
	var mc v1.CamundaManagementCluster
	if err := r.APIReader.Get(ctx, req.NamespacedName, &mc); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
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
		conditions.Stage(&mc, conditions.Failed(&mc, failure))
		return ctrl.Result{}, nil
	}
	if err != nil {
		return ctrl.Result{}, err
	}

	clusters, err := r.listClusters(ctx)
	if err != nil {
		return ctrl.Result{}, err
	}

	namespaces, err := r.selectedNamespaces(ctx, &mc)
	if err != nil {
		return ctrl.Result{}, err
	}

	attached, rows, err := r.attachedClusters(ctx, &mc, clusters, namespaces)
	if err != nil {
		return ctrl.Result{}, err
	}
	res.Input.Clusters = attached
	mc.Status.Clusters = rows
	// A renamed contract leaves the old name behind until the new contract is
	// written, so the status keeps naming the old one until then.
	previousContract := mc.Status.ManagementAuthConfig
	mc.Status.ManagementAuthConfig = res.ContractName

	built, err := components.Build(res.Input)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("building the components: %w", err)
	}
	comps = built.Components

	reconcileErr := reconcileComponents(ctx, rec, built.Components)
	// recordInitialClaim overwrites IdentityReady, and readyCondition
	// aggregates that condition. The refusal must therefore reach the CR
	// before the aggregate, or Ready reads True beside it.
	claimErr := r.recordInitialClaim(ctx, &mc, res.Input.Provider.Mode)
	userErr := r.syncWebModelerUsers(ctx, &mc, clusters, attached, rows)
	pingErr := r.syncPing(ctx, &mc, clusters, attached)
	// The claim of a cluster that left the selector goes last, once its user
	// and its ping are gone, so that no other management plane adopts a user
	// this one still has to remove.
	var releaseErr error
	if userErr == nil && pingErr == nil {
		releaseErr = r.releaseClaims(ctx, &mc, clusters, namespaces)
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
	conditions.Stage(&mc, readyCondition(&mc, built.Ready, contractErr))

	// Nothing watches the user API of a cluster, so the controller comes back
	// on its own: soon when a cluster refused the call, and on the converge
	// interval to find a user that somebody removed on the cluster.
	var result ctrl.Result
	switch {
	case anyRow(rows, v1.ReasonBasicAuthUserFailed):
		result.RequeueAfter = r.retryInterval()
	case convergesUsers(&mc, attached):
		result.RequeueAfter = r.convergeInterval()
	}

	return result, errors.Join(reconcileErr, claimErr, userErr, pingErr, releaseErr, contractErr)
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
// Identity reads the claim on its first start only and stores the result in
// its database. The annotation is what keeps the rendered environment on the
// value that Identity actually holds; without it a later edit of
// spec.identity.admin would roll the pods into a claim that changes nothing.
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

	ready := meta.FindStatusCondition(mc.Status.Conditions, v1.ConditionIdentityReady)
	if ready == nil || ready.Status != metav1.ConditionTrue {
		return nil
	}

	recorded := components.RecordedInitialClaim(mc)
	if recorded == "" {
		return r.recordAnnotation(ctx, mc, components.SpecInitialClaim(mc))
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

// readyCondition derives Ready from the components and the contract write. A
// failed write is never a ready management plane: the ManagementAuthConfig is
// the only thing a CamundaOptimize reads from this resource, and the
// components know nothing about it.
func readyCondition(
	mc *v1.CamundaManagementCluster,
	comps []*component.Component,
	contractErr error,
) metav1.Condition {
	if contractErr != nil {
		return conditions.Failed(mc, &conditions.PreCheckFailure{
			Reason:  v1.ReasonWriteFailed,
			Message: contractErr.Error(),
		})
	}

	return conditions.Aggregate(mc, comps...)
}

// SetupWithManager registers the controller, the reference indexes, and the
// watches. It also sets EventRecorder to the recorder of the manager, builds
// the uncached component client, and probes once whether the Kubernetes
// cluster serves the Keycloak kind.
func (r *Reconciler) SetupWithManager(mgr ctrl.Manager) error {
	if r.EventRecorder == nil {
		r.EventRecorder = mgr.GetEventRecorder("camundamanagementcluster")
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
