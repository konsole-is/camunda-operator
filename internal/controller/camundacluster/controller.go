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

// Package camundacluster reconciles the CamundaCluster CR. The controller
// resolves every reference, mirrors the Secrets that live outside the cluster
// namespace, converges one ocf component per process, and manages the broker
// volumes.
package camundacluster

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/sourcehawk/operator-component-framework/pkg/component"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/tools/events"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	v1 "github.com/konsole-is/camunda-operator/api/v1"
	components "github.com/konsole-is/camunda-operator/pkg/components/camundacluster"
	"github.com/konsole-is/camunda-operator/pkg/conditions"
)

// eventReasonPaused is recorded on every reconcile of a cluster with
// spec.pause set. Nothing else happens on such a reconcile.
const eventReasonPaused = "Paused"

// eventActionReconcile is the action of the events that the controller records
// about the reconcile of the cluster itself.
const eventActionReconcile = "Reconcile"

// CamundaClusterReconciler turns a CamundaCluster into the workloads of a
// Camunda orchestration cluster: the broker StatefulSet, one Deployment per
// standalone process, their Services, the admin Secret of a basic-auth
// cluster, and the copies of referenced Secrets from other namespaces.
type CamundaClusterReconciler struct {
	client.Client
	// APIReader reads without the cache. Secrets are watched metadata-only,
	// so their data and every referenced CR are read live.
	APIReader client.Reader
	Scheme    *runtime.Scheme
	// EventRecorder publishes the component lifecycle events and the events of
	// this controller. SetupWithManager sets it from the manager when it is
	// nil.
	EventRecorder events.EventRecorder

	// componentClient is the uncached client that the ocf components
	// reconcile through. The cached client of the manager must not be used
	// here: the typed Gets of ocf start a cluster-wide Secret informer,
	// which breaks the metadata-only Secret posture of the operator.
	// SetupWithManager sets it when it is nil.
	componentClient client.Client
	// restMapper resolves whether the cluster serves the ServiceMonitor
	// kind. SetupWithManager sets it from the manager.
	restMapper meta.RESTMapper
	// RetryInterval overrides how long the controller waits on something no
	// watch reports. Zero means defaultRetryInterval; tests shorten it.
	RetryInterval time.Duration
	// RESTEndpoint overrides where the user API of a cluster is called
	// during a password rotation. Nil means components.RESTEndpoint; tests
	// point it at a fake.
	RESTEndpoint func(cluster *v1.CamundaCluster, e components.Effective) string

	// refusals remembers the downgrade refusal that the controller recorded
	// for each cluster, so it records the Warning once and not once per
	// reconcile.
	refusals refusalMemo
}

// concurrentReconciles bounds the parallel reconciles. A rotation and an
// admin profile update are synchronous HTTP calls against the gateway of one
// cluster, and adminhttp bounds a call at 30 seconds. One black-holed
// gateway must not hold the only worker and delay every other cluster.
const concurrentReconciles = 4

// defaultRetryInterval is how long the controller waits before it looks again
// at an unwatched dependency, such as a pre-existing ServiceAccount.
const defaultRetryInterval = 30 * time.Second

// retryInterval returns the wait before an unwatched dependency is looked at
// again.
func (r *CamundaClusterReconciler) retryInterval() time.Duration {
	if r.RetryInterval > 0 {
		return r.RetryInterval
	}

	return defaultRetryInterval
}

// +kubebuilder:rbac:groups=core.camunda.io,resources=camundaclusters,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=core.camunda.io,resources=camundaclusters/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=core.camunda.io,resources=camundaclusters/finalizers,verbs=update
// +kubebuilder:rbac:groups=core.camunda.io,resources=camundaclusterpresets,verbs=get;list;watch
// +kubebuilder:rbac:groups=core.camunda.io,resources=camundaplatformconfigs,verbs=get;list;watch
// +kubebuilder:rbac:groups=core.camunda.io,resources=secondarystorageconfigs,verbs=get;list;watch;patch
// +kubebuilder:rbac:groups=core.camunda.io,resources=databaseconfigs,verbs=get;list;watch
// +kubebuilder:rbac:groups=core.camunda.io,resources=databaseserverconfigs,verbs=get;list;watch
// +kubebuilder:rbac:groups=core.camunda.io,resources=objectstorageconfigs,verbs=get;list;watch
// +kubebuilder:rbac:groups="",resources=secrets,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="",resources=services,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="",resources=serviceaccounts,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="",resources=persistentvolumeclaims,verbs=get;list;watch;patch
// +kubebuilder:rbac:groups="",resources=events,verbs=create;patch
// +kubebuilder:rbac:groups=apps,resources=statefulsets,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=apps,resources=deployments,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=monitoring.coreos.com,resources=servicemonitors,verbs=get;list;watch;create;update;patch;delete

// Reconcile converges a CamundaCluster. A paused cluster records one Paused
// event and returns before anything is read or written, status included.
// Otherwise the pre-checks resolve every reference into the render input; a
// failed pre-check reports its Ready reason and stops. A cluster whose
// storage contract another cluster holds renders suspended, reports
// StorageAlreadyAttached instead of the aggregate, and looks again on a
// timer. An effective version below the one the brokers run is refused
// before anything is applied, unless the annotation
// camunda.io/allow-version-downgrade names it. The annotation is removed once
// the brokers carry that version, and as soon as it names a version the
// cluster is not asked to run. The running version is stamped on the bound
// broker claims, so it survives a delete with retained volumes and the rule
// holds for a cluster recreated on them. The broker storage lifecycle then grows the
// bound broker claims in place and records an ignored shrink; the claim
// template keeps its applied size, so the StatefulSet is never recreated.
// The admin credential resolves next, and a requested password rotation runs
// there; a failed user API call surfaces on AdminSecretReady and retries on
// a timer. Then the components are reconciled in order: the admin Secret,
// the mirrored Secrets, then every process, each gated on whether the
// cluster needs it. The management binding is published with the status, and
// cleared while the cluster is suspended. The ServiceAccount the pods run
// under is published with it and is never cleared: the account outlives a
// suspension. Ready is True only when every component the cluster needs is
// True. Its reason and message come from the governing component, which is
// the highest-priority component that is not True, or the highest-priority
// of all of them when they all are.
//
// Status is written once per reconcile: the components and conditions.Stage
// stage conditions on the in-memory cluster, and the deferred FlushStatus
// persists them together.
func (r *CamundaClusterReconciler) Reconcile(ctx context.Context, req ctrl.Request) (_ ctrl.Result, err error) {
	var cluster v1.CamundaCluster
	// A reconcile that does not refuse drops the cluster out of the memo of
	// the recorded refusals. A refusal that comes back is then recorded
	// again, and a deleted cluster leaves nothing behind. The staged Ready
	// condition cannot stand in for this flag: a flush that conflicts drops
	// it.
	refusing := false
	defer func() {
		if !refusing {
			r.refusals.forget(req.NamespacedName)
		}
	}()

	if err := r.Get(ctx, req.NamespacedName, &cluster); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	if cluster.Spec.Pause {
		r.EventRecorder.Eventf(
			&cluster,
			nil,
			corev1.EventTypeNormal,
			eventReasonPaused,
			eventActionReconcile,
			"Reconcile paused by spec.pause",
		)
		return ctrl.Result{}, nil
	}

	rec := component.ReconcileContext{
		Client:        r.componentClient,
		Scheme:        r.Scheme,
		EventRecorder: r.EventRecorder,
		APIReader:     r.APIReader,
		Owner:         &cluster,
	}
	// Declared before the deferred flush, so the closure sees every component
	// that the reconcile builds below and FlushStatus owns their conditions.
	var comps []*component.Component
	defer func() {
		if flushErr := component.FlushStatus(ctx, rec, comps); flushErr != nil {
			err = errors.Join(err, flushErr)
		}
	}()

	in, mirrors, err := r.preCheck(ctx, &cluster)
	var failure *conditions.PreCheckFailure
	if errors.As(err, &failure) {
		conditions.Stage(&cluster, conditions.Failed(&cluster, failure))

		// Only an unwatched failure needs a timer; everything else the
		// pre-checks resolve re-enqueues through a watch.
		var unwatched *conditions.UnwatchedPreCheckFailure
		if errors.As(err, &unwatched) {
			return ctrl.Result{RequeueAfter: r.retryInterval()}, nil
		}
		return ctrl.Result{}, nil
	}
	if err != nil {
		return ctrl.Result{}, err
	}

	// A cluster whose storage contract another cluster holds renders
	// suspended: every workload at zero and the volumes kept, until the
	// holder releases it.
	// The suspension also idles the admin rotation and clears the management
	// binding, as a user suspension does.
	if in.Storage.Holder != nil {
		in.Effective.Suspend = true
	}

	in.ServiceMonitorSupported = r.serviceMonitorSupported()

	storage, err := r.readBrokerStorage(ctx, &cluster)
	if err != nil {
		return ctrl.Result{}, err
	}

	if err := r.stampBrokerVersion(ctx, storage); err != nil {
		return ctrl.Result{}, err
	}

	if err := r.consumeDowngradeSanction(ctx, &cluster, in, storage); err != nil {
		return ctrl.Result{}, err
	}

	// A refused downgrade re-enqueues through the watches on the cluster, the
	// preset, and the owned StatefulSet, so no timer is needed.
	if failure := refuseDowngrade(&cluster, in, storage); failure != nil {
		refusing = true
		refused := conditions.Failed(&cluster, failure)
		r.recordRefusedDowngrade(&cluster, refused)
		conditions.Stage(&cluster, refused)

		return ctrl.Result{}, nil
	}

	in.VolumeClaimSize = storage.volumeClaimSize()

	if err := r.growBrokerClaims(ctx, storage, in.Effective.StorageSize()); err != nil {
		return ctrl.Result{}, err
	}
	r.recordIgnoredShrink(&cluster, storage, in.Effective.StorageSize())

	cred, err := r.resolveAdminCredential(ctx, &cluster, in, storage)
	if err != nil {
		return ctrl.Result{}, err
	}
	in.AdminPasswordHash = components.PasswordHash(cred.published)

	built, err := r.buildComponents(&cluster, in, mirrors, cred)
	if err != nil {
		return ctrl.Result{}, err
	}
	comps = built.all

	// Read before the components stage their own conditions: stageFailure
	// needs the AdminSecretReady that the server holds, not the healthy one
	// that the Secret component is about to write over it.
	priorAdminSecret := meta.FindStatusCondition(cluster.Status.Conditions, v1.ConditionAdminSecretReady).
		DeepCopy()

	reconcileErr := reconcileComponents(ctx, rec, built.all)

	cred.stageFailure(&cluster, priorAdminSecret)
	if in.Storage.Holder != nil {
		// The parked reason is the gate that Suspended reports to extensions and
		// backups, so it stays while the apply fails. The message carries the
		// error, and the component conditions carry the state of each workload.
		conditions.Stage(&cluster, storageHeld(&cluster, in.Storage.Holder, reconcileErr))
	} else {
		conditions.Stage(&cluster, conditions.Aggregate(&cluster, built.ready...))
	}
	cluster.Status.Volumes = storage.volumes()
	cluster.Status.Management = managementBinding(&cluster, in)
	cluster.Status.Gateway = gatewayBinding(&cluster, in)
	cluster.Status.ServiceAccountName = components.PodServiceAccountName(in)
	cred.recordRotation(&cluster)

	// A failed rotation retries on a timer: no watch fires when the user API
	// recovers or accepts the credentials again.
	if cred.failure != nil && reconcileErr == nil {
		return ctrl.Result{RequeueAfter: r.retryInterval()}, nil
	}

	// A parked cluster takes a stale claim on a timer: nothing watches its
	// holder for it, and nothing should.
	if in.Storage.Holder != nil && reconcileErr == nil {
		return ctrl.Result{RequeueAfter: r.retryInterval()}, nil
	}

	return ctrl.Result{}, reconcileErr
}

// clusterComponents are the components of one cluster: all of them are
// reconciled in order, and the ready ones make up Ready.
type clusterComponents struct {
	all   []*component.Component
	ready []*component.Component
}

// buildComponents builds every component of the cluster in reconcile order:
// the admin Secret, the mirrored Secrets, then one component per process.
// Every component is always built, so a switch of the authentication method,
// a reference that went away, or a topology change deletes what is no longer
// needed through its gate. Only the admin Secret of a basic-auth cluster, the
// mirrored Secrets when a referenced Secret lives outside the cluster
// namespace, and the enabled processes take part in Ready, so Ready never
// reports Disabled. The admin credential comes from resolveAdminCredential:
// the password stays stable after creation, a delete of the Secret replaces
// it, and spec.auth.basic.passwordRotation rotates it through the user API.
func (r *CamundaClusterReconciler) buildComponents(
	cluster *v1.CamundaCluster,
	in components.Input,
	mirrors mirroredSecrets,
	cred adminCredential,
) (clusterComponents, error) {
	var comps clusterComponents
	add := func(comp *component.Component, ready bool) {
		comps.all = append(comps.all, comp)
		if ready {
			comps.ready = append(comps.ready, comp)
		}
	}

	basic := components.ResolveAuth(in).Method == v1.AuthenticationMethodBasic
	admin, err := components.AdminSecretComponent(cluster, basic, components.AdminSecretState{
		Password:        cred.password,
		PendingPassword: cred.pending,
		PendingRotation: cred.pendingRotation,
		Rotation:        cred.rotation,
		Email:           cred.email,
		AppliedEmail:    cred.appliedEmail,
	})
	if err != nil {
		return clusterComponents{}, fmt.Errorf("building admin secret component: %w", err)
	}
	add(admin, basic)

	mirrored, err := components.MirroredSecretComponent(cluster, mirrors)
	if err != nil {
		return clusterComponents{}, fmt.Errorf("building mirrored secrets component: %w", err)
	}
	add(mirrored, len(mirrors) > 0)

	processes, err := components.Build(in)
	if err != nil {
		return clusterComponents{}, fmt.Errorf("building process components: %w", err)
	}
	for _, pc := range processes {
		add(pc.Component, pc.Process.Enabled)
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

// serviceMonitorSupported reports whether the cluster serves the
// ServiceMonitor kind. On a cluster without the prometheus-operator CRDs the
// components then omit the resource instead of failing every reconcile.
func (r *CamundaClusterReconciler) serviceMonitorSupported() bool {
	if r.restMapper == nil {
		return false
	}

	_, err := r.restMapper.RESTMapping(
		schema.GroupKind{Group: "monitoring.coreos.com", Kind: "ServiceMonitor"}, "v1",
	)
	return err == nil
}
