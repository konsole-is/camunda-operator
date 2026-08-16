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

// Package elasticsearchcluster reconciles the ElasticsearchCluster CR. The
// controller merges the preset into the spec, drives the ECK operator through
// a rendered Elasticsearch CR, generates the file-realm credentials, and
// publishes the SecondaryStorageConfig binding.
package elasticsearchcluster

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"

	esv1 "github.com/elastic/cloud-on-k8s/v3/pkg/apis/elasticsearch/v1"
	"github.com/sourcehawk/operator-component-framework/pkg/component"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/events"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	v1 "github.com/konsole-is/camunda-operator/api/v1"
	components "github.com/konsole-is/camunda-operator/pkg/components/elasticsearchcluster"
	"github.com/konsole-is/camunda-operator/pkg/conditions"
	"github.com/konsole-is/camunda-operator/pkg/credentials"
	"github.com/konsole-is/camunda-operator/pkg/refindex"
	"github.com/konsole-is/camunda-operator/pkg/wrappers/eckelasticsearch"
)

// elasticsearchClusterPresetRefField indexes ElasticsearchClusters by
// spec.presetRef, so a preset edit enqueues every cluster that references it.
const elasticsearchClusterPresetRefField = "elasticsearchcluster.spec.presetRef"

// eventReasonStorageShrinkIgnored is the Warning event that the controller
// records when the merged storageSize is below the applied data volume size.
// It keeps the applied size, because Elasticsearch data volumes cannot be
// reduced in place.
const eventReasonStorageShrinkIgnored = "StorageShrinkIgnored"

// eventActionResize is the action of the events that the controller records
// about the size of the data volumes.
const eventActionResize = "Resize"

// ElasticsearchClusterReconciler provisions an Elasticsearch cluster through
// the external ECK operator. It renders an ECK Elasticsearch CR, generates the
// file-realm credentials, and publishes a SecondaryStorageConfig binding in
// the namespace of the CR.
type ElasticsearchClusterReconciler struct {
	client.Client
	// APIReader reads without the cache. The credential Secrets must be read
	// live.
	APIReader client.Reader
	Scheme    *runtime.Scheme
	// EventRecorder publishes the component lifecycle events. SetupWithManager
	// sets it from the manager.
	EventRecorder events.EventRecorder

	// componentClient is the uncached client that the ocf components
	// reconcile through, wrapped for ECK apply sanitization. SetupWithManager
	// builds it. The cached client of the manager must not be used here: the
	// typed Gets of ocf start a cluster-wide Secret informer, which breaks
	// the metadata-only Secret posture of the operator.
	componentClient client.Client
	// restMapper resolves whether the cluster serves the ServiceMonitor
	// kind. SetupWithManager sets it from the manager.
	restMapper meta.RESTMapper
}

// +kubebuilder:rbac:groups=core.camunda.io,resources=elasticsearchclusters,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=core.camunda.io,resources=elasticsearchclusters/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=core.camunda.io,resources=elasticsearchclusters/finalizers,verbs=update
// +kubebuilder:rbac:groups=core.camunda.io,resources=elasticsearchclusterpresets,verbs=get;list;watch
// +kubebuilder:rbac:groups=core.camunda.io,resources=secondarystorageconfigs,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=elasticsearch.k8s.elastic.co,resources=elasticsearches,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="",resources=secrets,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="",resources=serviceaccounts,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="",resources=services,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=apps,resources=deployments,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="",resources=persistentvolumeclaims,verbs=get;list;watch
// +kubebuilder:rbac:groups="",resources=events,verbs=create;patch
// +kubebuilder:rbac:groups=monitoring.coreos.com,resources=servicemonitors,verbs=get;list;watch;create;update;patch;delete

// Reconcile converges an ElasticsearchCluster. It resolves the preset, runs
// the pre-checks, reconciles the credentials, elasticsearch, storage-contract,
// and metrics components in dependency order, and derives the CR-level Ready
// condition.
//
// Status is written once per reconcile. The components and conditions.Stage
// stage conditions on the in-memory cluster, and the deferred FlushStatus
// persists them together.
func (r *ElasticsearchClusterReconciler) Reconcile(ctx context.Context, req ctrl.Request) (_ ctrl.Result, err error) {
	var cluster v1.ElasticsearchCluster
	if err := r.Get(ctx, req.NamespacedName, &cluster); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	recCtx := component.ReconcileContext{
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
		if flushErr := component.FlushStatus(ctx, recCtx, comps); flushErr != nil {
			err = errors.Join(err, flushErr)
		}
	}()

	merged, err := r.preCheck(ctx, &cluster)
	var failure *conditions.PreCheckFailure
	if errors.As(err, &failure) {
		conditions.Stage(&cluster, conditions.Failed(&cluster, failure))
		return ctrl.Result{}, nil
	}
	if err != nil {
		return ctrl.Result{}, err
	}

	if err := r.keepAppliedStorageSize(ctx, &cluster, &merged); err != nil {
		return ctrl.Result{}, err
	}

	core, metrics, err := r.buildComponents(ctx, &cluster, merged)
	if err != nil {
		return ctrl.Result{}, err
	}
	comps = append(core, metrics)

	// The metrics exporter is auxiliary: it keeps its own MetricsReady
	// condition and stays out of Ready, so a broken exporter never marks the
	// cluster not ready and a disabled one never shows up on it.
	reconcileErr := reconcileComponents(ctx, recCtx, comps)
	conditions.Stage(&cluster, conditions.Aggregate(&cluster, core...))

	volumes, err := r.dataVolumes(ctx, &cluster)
	if err != nil {
		return ctrl.Result{}, errors.Join(reconcileErr, err)
	}
	cluster.Status.Volumes = volumes.volumes

	return ctrl.Result{}, reconcileErr
}

// preCheck resolves the preset and validates the merged spec. It returns the
// preset-merged spec. A failed check returns a *conditions.PreCheckFailure
// that carries its Ready reason. A dangling presetRef, an incomplete merge,
// and a version below the floor all report InvalidReference. Any other error
// is a transient API failure.
func (r *ElasticsearchClusterReconciler) preCheck(
	ctx context.Context,
	cluster *v1.ElasticsearchCluster,
) (v1.ElasticsearchClusterSpec, error) {
	merged := cluster.Spec

	if cluster.Spec.PresetRef != "" {
		var preset v1.ElasticsearchClusterPreset
		if err := r.APIReader.Get(ctx, types.NamespacedName{Name: cluster.Spec.PresetRef}, &preset); err != nil {
			if apierrors.IsNotFound(err) {
				return merged, &conditions.PreCheckFailure{
					Reason:  v1.ReasonInvalidReference,
					Message: fmt.Sprintf("ElasticsearchClusterPreset %q not found", cluster.Spec.PresetRef),
				}
			}
			return merged, fmt.Errorf("resolving preset %q: %w", cluster.Spec.PresetRef, err)
		}
		merged = components.MergePreset(cluster.Spec, &preset.Spec)
	}

	if err := components.ValidateMerged(merged); err != nil {
		return merged, &conditions.PreCheckFailure{
			Reason:  v1.ReasonInvalidReference,
			Message: err.Error(),
		}
	}

	return merged, nil
}

// keepAppliedStorageSize guards against the shrinks that admission cannot
// see: a preset baseline lowered under a cluster, or an inline storageSize set
// below a size that a preset provided before. It compares the merged size
// against the largest data volume that exists: the claim of the applied ECK
// CR, and the data PersistentVolumeClaims themselves. The claims matter on
// their own during suspension, when the ECK CR is deleted and the volumes
// stay. If the merged size is smaller, it keeps the existing size in merged
// and records a Warning event, because Elasticsearch data volumes cannot be
// reduced in place. The reconcile continues with that size.
func (r *ElasticsearchClusterReconciler) keepAppliedStorageSize(
	ctx context.Context,
	cluster *v1.ElasticsearchCluster,
	merged *v1.ElasticsearchClusterSpec,
) error {
	volumes, err := r.dataVolumes(ctx, cluster)
	if err != nil {
		return err
	}

	largest := volumes.largest()
	if largest == nil || merged.StorageSize == nil || merged.StorageSize.Cmp(*largest) >= 0 {
		return nil
	}

	r.EventRecorder.Eventf(
		cluster,
		nil,
		corev1.EventTypeWarning,
		eventReasonStorageShrinkIgnored,
		eventActionResize,
		"storageSize %s is below the existing data volume size %s; keeping %s, Elasticsearch data volumes cannot be reduced",
		merged.StorageSize,
		largest,
		largest,
	)
	merged.StorageSize = largest

	return nil
}

// dataVolumes are the data volumes that the cluster has: one entry per data
// PersistentVolumeClaim that reports a capacity, sorted by name, and the
// claim size of the applied ECK CR when that CR exists.
type dataVolumes struct {
	volumes []v1.VolumeStatus
	applied *resource.Quantity
}

// largest returns the largest of the claim capacities and the applied claim
// size, or nil when neither exists. It is the size that a rendered claim must
// not go below.
func (d dataVolumes) largest() *resource.Quantity {
	largest := d.applied
	for i := range d.volumes {
		if largest == nil || d.volumes[i].Capacity.Cmp(*largest) > 0 {
			largest = &d.volumes[i].Capacity
		}
	}
	return largest
}

// dataVolumes lists the data volumes of cluster. ECK labels the claims with
// the cluster name, and the data claims carry the data volume claim name as
// prefix.
func (r *ElasticsearchClusterReconciler) dataVolumes(
	ctx context.Context,
	cluster *v1.ElasticsearchCluster,
) (dataVolumes, error) {
	var claims corev1.PersistentVolumeClaimList
	if err := r.List(
		ctx, &claims,
		client.InNamespace(cluster.Namespace),
		client.MatchingLabels{components.ECKClusterNameLabel: cluster.Name},
	); err != nil {
		return dataVolumes{}, fmt.Errorf("listing data volume claims of %q: %w", cluster.Name, err)
	}

	var volumes dataVolumes
	for i := range claims.Items {
		claim := &claims.Items[i]
		if !strings.HasPrefix(claim.Name, components.DataVolumeClaimName+"-") {
			continue
		}
		if capacity, ok := claim.Status.Capacity[corev1.ResourceStorage]; ok {
			volumes.volumes = append(volumes.volumes, v1.VolumeStatus{Name: claim.Name, Capacity: capacity})
		}
	}
	slices.SortFunc(volumes.volumes, func(a, b v1.VolumeStatus) int { return strings.Compare(a.Name, b.Name) })

	var es esv1.Elasticsearch
	if err := r.Get(ctx, client.ObjectKeyFromObject(cluster), &es); err != nil {
		if apierrors.IsNotFound(err) {
			return volumes, nil
		}
		return dataVolumes{}, fmt.Errorf("reading applied Elasticsearch %q: %w", cluster.Name, err)
	}
	volumes.applied = appliedDataClaimSize(&es)

	return volumes, nil
}

// enqueueForDataClaim maps a PersistentVolumeClaim event to the cluster that
// ECK labels it with, so a resize outside the spec updates status.volumes.
func enqueueForDataClaim() handler.EventHandler {
	return handler.EnqueueRequestsFromMapFunc(func(_ context.Context, o client.Object) []reconcile.Request {
		name, ok := o.GetLabels()[components.ECKClusterNameLabel]
		if !ok {
			return nil
		}
		return []reconcile.Request{{NamespacedName: types.NamespacedName{Namespace: o.GetNamespace(), Name: name}}}
	})
}

// appliedDataClaimSize returns the data volume claim size of the applied ECK
// CR, or nil when the CR carries no such claim.
func appliedDataClaimSize(es *esv1.Elasticsearch) *resource.Quantity {
	for _, nodeSet := range es.Spec.NodeSets {
		for _, claim := range nodeSet.VolumeClaimTemplates {
			if claim.Name != components.DataVolumeClaimName {
				continue
			}
			if size, ok := claim.Spec.Resources.Requests[corev1.ResourceStorage]; ok {
				return &size
			}
		}
	}

	return nil
}

// buildComponents builds the components in dependency order: the three that
// make up Ready (credentials, elasticsearch, storage-contract), and the
// metrics component apart. It reads the password from the existing user
// Secret without the cache, so the password stays stable after creation. To
// rotate it, delete the Secret.
func (r *ElasticsearchClusterReconciler) buildComponents(
	ctx context.Context,
	cluster *v1.ElasticsearchCluster,
	merged v1.ElasticsearchClusterSpec,
) (core []*component.Component, metrics *component.Component, err error) {
	password, err := credentials.LookupOrNew(
		ctx, r.APIReader, client.ObjectKey{
			Namespace: cluster.Namespace, Name: components.UserSecretName(cluster),
		}, components.PasswordKey,
	)
	if err != nil {
		return nil, nil, err
	}

	credentialsComp, err := components.CredentialsComponent(cluster, password)
	if err != nil {
		return nil, nil, fmt.Errorf("building credentials component: %w", err)
	}

	elasticsearchComp, err := components.ElasticsearchComponent(cluster, merged)
	if err != nil {
		return nil, nil, fmt.Errorf("building elasticsearch component: %w", err)
	}

	storageContractComp, err := components.StorageContractComponent(cluster, merged)
	if err != nil {
		return nil, nil, fmt.Errorf("building storage-contract component: %w", err)
	}

	metricsComp, err := components.MetricsComponent(cluster, merged, r.serviceMonitorSupported())
	if err != nil {
		return nil, nil, fmt.Errorf("building metrics component: %w", err)
	}

	return []*component.Component{credentialsComp, elasticsearchComp, storageContractComp}, metricsComp, nil
}

// reconcileComponents reconciles comps in order. It continues past a failing
// component, so one failure does not stall the rest, and returns the first
// error.
func reconcileComponents(ctx context.Context, recCtx component.ReconcileContext, comps []*component.Component) error {
	var firstErr error
	for _, comp := range comps {
		if err := comp.Reconcile(ctx, recCtx); err != nil && firstErr == nil {
			firstErr = err
		}
	}

	return firstErr
}

// serviceMonitorSupported reports whether the cluster serves the
// ServiceMonitor kind. On a cluster without the prometheus-operator CRDs the
// metrics component then omits the resource instead of failing every
// reconcile.
func (r *ElasticsearchClusterReconciler) serviceMonitorSupported() bool {
	if r.restMapper == nil {
		return false
	}

	_, err := r.restMapper.RESTMapping(
		schema.GroupKind{Group: "monitoring.coreos.com", Kind: "ServiceMonitor"}, "v1",
	)
	return err == nil
}

// SetupWithManager registers the controller, ownership watches on the ECK CR,
// the user Secret, and the SecondaryStorageConfig, a preset watch through a
// field index on spec.presetRef, and a watch on the data volume claims that
// ECK labels with the cluster name.
func (r *ElasticsearchClusterReconciler) SetupWithManager(mgr ctrl.Manager) error {
	if r.EventRecorder == nil {
		r.EventRecorder = mgr.GetEventRecorder("elasticsearchcluster")
	}
	r.restMapper = mgr.GetRESTMapper()

	if r.componentClient == nil {
		// Uncached: see the componentClient field doc. The apply wrapper
		// sanitizes typed Elasticsearch patches down to the fields that the
		// ECK CRD schema declares.
		componentClient, err := client.New(mgr.GetConfig(), client.Options{
			Scheme: mgr.GetScheme(),
			Mapper: mgr.GetRESTMapper(),
		})
		if err != nil {
			return fmt.Errorf("building the component client: %w", err)
		}
		r.componentClient = eckelasticsearch.NewApplyClient(componentClient)
	}

	if err := mgr.GetFieldIndexer().IndexField(
		context.Background(), &v1.ElasticsearchCluster{},
		elasticsearchClusterPresetRefField, func(o client.Object) []string {
			if ref := o.(*v1.ElasticsearchCluster).Spec.PresetRef; ref != "" {
				return []string{ref}
			}
			return nil
		},
	); err != nil {
		return err
	}

	return ctrl.NewControllerManagedBy(mgr).
		For(&v1.ElasticsearchCluster{}).
		Owns(&esv1.Elasticsearch{}).
		Owns(&corev1.Secret{}, builder.OnlyMetadata).
		Owns(&v1.SecondaryStorageConfig{}).
		Owns(&appsv1.Deployment{}).
		Owns(&corev1.Service{}).
		Watches(
			&v1.ElasticsearchClusterPreset{},
			refindex.Enqueue(
				mgr.GetClient(), &v1.ElasticsearchClusterList{},
				elasticsearchClusterPresetRefField, refindex.ObjectName,
			),
		).
		Watches(&corev1.PersistentVolumeClaim{}, enqueueForDataClaim()).
		Named("elasticsearchcluster").
		Complete(r)
}
