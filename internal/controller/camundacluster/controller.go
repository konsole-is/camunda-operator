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

	"github.com/sourcehawk/operator-component-framework/pkg/component"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	v1 "github.com/konsole-is/camunda-operator/api/v1"
	components "github.com/konsole-is/camunda-operator/pkg/components/camundacluster"
	"github.com/konsole-is/camunda-operator/pkg/conditions"
	"github.com/konsole-is/camunda-operator/pkg/credentials"
)

// eventReasonPaused is recorded on every reconcile of a cluster with
// spec.pause set. Nothing else happens on such a reconcile.
const eventReasonPaused = "Paused"

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
	// Recorder publishes the component lifecycle events and the events of
	// this controller. SetupWithManager sets it from the manager when it is
	// nil.
	Recorder record.EventRecorder

	// componentClient is the uncached client that the ocf components
	// reconcile through. The cached client of the manager must not be used
	// here: the typed Gets of ocf start a cluster-wide Secret informer,
	// which breaks the metadata-only Secret posture of the operator.
	// SetupWithManager sets it when it is nil.
	componentClient client.Client
	// restMapper resolves whether the cluster serves the ServiceMonitor
	// kind. SetupWithManager sets it from the manager.
	restMapper meta.RESTMapper
}

// +kubebuilder:rbac:groups=core.camunda.io,resources=camundaclusters,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=core.camunda.io,resources=camundaclusters/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=core.camunda.io,resources=camundaclusters/finalizers,verbs=update
// +kubebuilder:rbac:groups=core.camunda.io,resources=camundaclusterpresets,verbs=get;list;watch
// +kubebuilder:rbac:groups=core.camunda.io,resources=camundaplatformconfigs,verbs=get;list;watch
// +kubebuilder:rbac:groups=core.camunda.io,resources=secondarystorageconfigs,verbs=get;list;watch
// +kubebuilder:rbac:groups=core.camunda.io,resources=databaseconfigs,verbs=get;list;watch
// +kubebuilder:rbac:groups=core.camunda.io,resources=databaseserverconfigs,verbs=get;list;watch
// +kubebuilder:rbac:groups=core.camunda.io,resources=objectstorageconfigs,verbs=get;list;watch
// +kubebuilder:rbac:groups="",resources=secrets,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="",resources=services,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="",resources=serviceaccounts,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="",resources=persistentvolumeclaims,verbs=get;list;watch;update;patch
// +kubebuilder:rbac:groups="",resources=events,verbs=create;patch
// +kubebuilder:rbac:groups=apps,resources=statefulsets,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=apps,resources=deployments,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=monitoring.coreos.com,resources=servicemonitors,verbs=get;list;watch;create;update;patch;delete

// Reconcile converges a CamundaCluster. A paused cluster records one Paused
// event and returns before anything is read or written, status included.
// Otherwise the pre-checks resolve every reference into the render input; a
// failed pre-check reports its Ready reason and stops. The broker storage
// lifecycle then clamps and grows the volumes, and the components are
// reconciled in order: the admin Secret, the mirrored Secrets, then every
// process. Ready mirrors the highest-priority component condition.
//
// Status is written once per reconcile: the components and conditions.Stage
// stage conditions on the in-memory cluster, and the deferred FlushStatus
// persists them together.
func (r *CamundaClusterReconciler) Reconcile(ctx context.Context, req ctrl.Request) (_ ctrl.Result, err error) {
	var cluster v1.CamundaCluster
	if err := r.Get(ctx, req.NamespacedName, &cluster); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	if cluster.Spec.Pause {
		r.Recorder.Event(&cluster, corev1.EventTypeNormal, eventReasonPaused, "reconcile paused by spec.pause")
		return ctrl.Result{}, nil
	}

	rec := component.ReconcileContext{
		Client:   r.componentClient,
		Scheme:   r.Scheme,
		Recorder: r.Recorder,
		Owner:    &cluster,
	}
	defer func() {
		if flushErr := component.FlushStatus(ctx, rec); flushErr != nil {
			err = errors.Join(err, flushErr)
		}
	}()

	in, mirrors, err := r.preCheck(ctx, &cluster)
	var failure *conditions.PreCheckFailure
	if errors.As(err, &failure) {
		conditions.Stage(&cluster, conditions.Failed(&cluster, failure))
		return ctrl.Result{}, nil
	}
	if err != nil {
		return ctrl.Result{}, err
	}
	in.ServiceMonitorSupported = r.serviceMonitorSupported()

	sizes, err := r.keepAppliedStorageSize(ctx, &cluster, &in)
	if err != nil {
		return ctrl.Result{}, err
	}
	if err := r.growBrokerClaims(ctx, &cluster, in.Effective.StorageSize()); err != nil {
		return ctrl.Result{}, err
	}
	if err := r.recreateStatefulSetOnClaimChange(ctx, &cluster, in); err != nil {
		return ctrl.Result{}, err
	}

	comps, err := r.buildComponents(ctx, &cluster, in, mirrors)
	if err != nil {
		return ctrl.Result{}, err
	}

	reconcileErr := reconcileComponents(ctx, rec, comps)
	conditions.Stage(&cluster, conditions.Aggregate(&cluster, comps...))
	cluster.Status.StorageSize = sizes.smallest()

	return ctrl.Result{}, reconcileErr
}

// buildComponents builds every component that makes up Ready, in reconcile
// order: the admin Secret of a basic-auth cluster, the mirrored Secrets when
// a referenced Secret lives outside the cluster namespace, then one component
// per process. It reads the admin password from the existing admin Secret
// without the cache, so the password stays stable after creation. To rotate
// it, delete the Secret.
func (r *CamundaClusterReconciler) buildComponents(
	ctx context.Context,
	cluster *v1.CamundaCluster,
	in components.Input,
	mirrors map[string]map[string][]byte,
) ([]*component.Component, error) {
	var comps []*component.Component

	if components.ResolveAuth(in).Method == v1.AuthenticationMethodBasic {
		password, err := credentials.LookupOrNew(
			ctx, r.APIReader,
			client.ObjectKey{Namespace: cluster.Namespace, Name: components.AdminSecretName(cluster)},
			components.AdminPasswordKey,
		)
		if err != nil {
			return nil, err
		}

		admin, err := components.AdminSecretComponent(cluster, password)
		if err != nil {
			return nil, err
		}
		comps = append(comps, admin)
	}

	if len(mirrors) > 0 {
		mirrored, err := components.MirroredSecretComponent(cluster, mirrors)
		if err != nil {
			return nil, err
		}
		comps = append(comps, mirrored)
	}

	processes, err := components.Build(in)
	if err != nil {
		return nil, err
	}

	return append(comps, processes...), nil
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
