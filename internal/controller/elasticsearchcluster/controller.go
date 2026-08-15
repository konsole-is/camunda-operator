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

package controller

import (
	"context"
	"fmt"

	esv1 "github.com/elastic/cloud-on-k8s/v3/pkg/apis/elasticsearch/v1"
	"github.com/sourcehawk/operator-component-framework/pkg/component"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	v1 "github.com/konsole-is/camunda-operator/api/v1"
	"github.com/konsole-is/camunda-operator/pkg/conditions"
	"github.com/konsole-is/camunda-operator/pkg/credentials"
	"github.com/konsole-is/camunda-operator/pkg/refindex"
	"github.com/konsole-is/camunda-operator/pkg/wrappers/eckelasticsearch"
)

// elasticsearchClusterPresetRefField indexes ElasticsearchClusters by their
// spec.presetRef, so preset edits enqueue every referencing cluster.
const elasticsearchClusterPresetRefField = "elasticsearchcluster.spec.presetRef"

// ElasticsearchClusterReconciler provisions an Elasticsearch cluster through
// the external ECK operator: it renders an ECK Elasticsearch CR, generates
// file-realm credentials, and publishes a SecondaryStorageConfig binding in
// the CR's namespace.
type ElasticsearchClusterReconciler struct {
	client.Client
	// APIReader reads uncached, for credential Secrets whose data must be
	// read live.
	APIReader client.Reader
	Scheme    *runtime.Scheme
	// Recorder publishes the component lifecycle events. SetupWithManager
	// sets it from the manager.
	Recorder record.EventRecorder

	// componentClient is the uncached client the ocf components reconcile
	// through, wrapped for ECK apply sanitization. SetupWithManager builds
	// it; the manager's cached client must not be used here because ocf's
	// typed Gets would spin up a cluster-wide Secret informer, breaking the
	// operator's metadata-only Secret posture.
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
// +kubebuilder:rbac:groups="",resources=events,verbs=create;patch
// +kubebuilder:rbac:groups=monitoring.coreos.com,resources=servicemonitors,verbs=get;list;watch;create;update;patch;delete

// Reconcile converges an ElasticsearchCluster: it resolves the preset, runs
// the pre-checks, reconciles the credentials, elasticsearch, and
// storage-contract components in dependency order, and derives the CR-level
// Ready and Suspended conditions. Component conditions are persisted through
// ocf's FlushStatus; the CR-level conditions and status.observedGeneration
// through pkg/conditions SSA.
func (r *ElasticsearchClusterReconciler) Reconcile(ctx context.Context, req ctrl.Request) (_ ctrl.Result, err error) {
	var cluster v1.ElasticsearchCluster
	if err := r.Get(ctx, req.NamespacedName, &cluster); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	recCtx := component.ReconcileContext{
		Client:   r.componentClient,
		Scheme:   r.Scheme,
		Recorder: r.Recorder,
		Owner:    &cluster,
	}

	pre, merged, err := r.preCheck(ctx, &cluster)
	if err != nil {
		return ctrl.Result{}, err
	}

	if pre == nil {
		componentErr := r.reconcileComponents(ctx, recCtx, &cluster, merged)

		// Flush even when a component errored: conditions staged on error
		// paths must still be persisted.
		if flushErr := component.FlushStatus(ctx, recCtx); flushErr != nil && componentErr == nil {
			componentErr = flushErr
		}
		if componentErr != nil {
			return ctrl.Result{}, componentErr
		}
	}

	return ctrl.Result{}, r.patchCRConditions(ctx, &cluster, pre)
}

// preCheck resolves the preset and validates the merged spec, mapping every
// failure admission cannot catch to its documented Ready reason: a dangling
// presetRef, an incomplete merge, a below-floor version, and a storageSize
// shrink relative to the applied ECK CR all report InvalidReference. A
// non-nil failure short-circuits component reconciliation for the cycle. The
// returned spec is the preset-merged configuration.
func (r *ElasticsearchClusterReconciler) preCheck(
	ctx context.Context,
	cluster *v1.ElasticsearchCluster,
) (*conditions.PreCheckFailure, v1.ElasticsearchClusterSpec, error) {
	merged := cluster.Spec

	if cluster.Spec.PresetRef != "" {
		var preset v1.ElasticsearchClusterPreset
		if err := r.APIReader.Get(ctx, types.NamespacedName{Name: cluster.Spec.PresetRef}, &preset); err != nil {
			if apierrors.IsNotFound(err) {
				return &conditions.PreCheckFailure{
					Reason:  conditions.ReasonInvalidReference,
					Message: fmt.Sprintf("ElasticsearchClusterPreset %q not found", cluster.Spec.PresetRef),
				}, merged, nil
			}
			return nil, merged, fmt.Errorf("resolving preset %q: %w", cluster.Spec.PresetRef, err)
		}
		merged = mergePreset(cluster.Spec, &preset.Spec)
	}

	if err := validateMerged(merged); err != nil {
		return &conditions.PreCheckFailure{
			Reason:  conditions.ReasonInvalidReference,
			Message: err.Error(),
		}, merged, nil
	}

	shrink, err := r.checkStorageShrink(ctx, cluster, merged)
	if err != nil {
		return nil, merged, err
	}

	return shrink, merged, nil
}

// checkStorageShrink guards against the shrinks admission cannot see — a
// preset baseline lowered under a referencing cluster, or an inline
// storageSize set below a previously preset-provided size — by comparing the
// merged size against the applied ECK CR's data volume claim. A shrink is
// reported instead of applied: Elasticsearch data volumes cannot be reduced
// in place.
func (r *ElasticsearchClusterReconciler) checkStorageShrink(
	ctx context.Context,
	cluster *v1.ElasticsearchCluster,
	merged v1.ElasticsearchClusterSpec,
) (*conditions.PreCheckFailure, error) {
	var es esv1.Elasticsearch
	if err := r.Get(ctx, client.ObjectKeyFromObject(cluster), &es); err != nil {
		if apierrors.IsNotFound(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("reading applied Elasticsearch %q: %w", cluster.Name, err)
	}

	applied := appliedDataClaimSize(&es)
	if applied == nil {
		return nil, nil
	}

	if merged.StorageSize.Cmp(*applied) < 0 {
		return &conditions.PreCheckFailure{
			Reason: conditions.ReasonInvalidReference,
			Message: fmt.Sprintf(
				"storageSize %s would shrink the applied data volume size %s; Elasticsearch data volumes cannot be reduced",
				merged.StorageSize, applied,
			),
		}, nil
	}

	return nil, nil
}

// appliedDataClaimSize returns the applied ECK CR's data volume claim size,
// or nil when the CR carries no such claim.
func appliedDataClaimSize(es *esv1.Elasticsearch) *resource.Quantity {
	for _, nodeSet := range es.Spec.NodeSets {
		for _, claim := range nodeSet.VolumeClaimTemplates {
			if claim.Name != escDataVolumeClaimName {
				continue
			}
			if size, ok := claim.Spec.Resources.Requests[corev1.ResourceStorage]; ok {
				return &size
			}
		}
	}

	return nil
}

// reconcileComponents builds and reconciles the three components in
// dependency order, continuing past a failing component so one failure does
// not stall the rest, and returning the first error. The password is read
// from the existing user Secret uncached so it stays stable once created;
// deleting the Secret is the rotation mechanism.
func (r *ElasticsearchClusterReconciler) reconcileComponents(
	ctx context.Context,
	recCtx component.ReconcileContext,
	cluster *v1.ElasticsearchCluster,
	merged v1.ElasticsearchClusterSpec,
) error {
	password, found, err := credentials.Lookup(ctx, r.APIReader, client.ObjectKey{
		Namespace: cluster.Namespace, Name: escUserSecretName(cluster),
	}, escPasswordKey)
	if err != nil {
		return err
	}
	if !found {
		if password, err = credentials.NewPassword(); err != nil {
			return err
		}
	}

	credentialsComp, err := escCredentialsComponent(cluster, password)
	if err != nil {
		return fmt.Errorf("building credentials component: %w", err)
	}

	elasticsearchComp, err := escElasticsearchComponent(cluster, merged, r.serviceMonitorSupported())
	if err != nil {
		return fmt.Errorf("building elasticsearch component: %w", err)
	}

	storageContractComp, err := escStorageContractComponent(cluster, merged)
	if err != nil {
		return fmt.Errorf("building storage-contract component: %w", err)
	}

	var firstErr error
	for _, comp := range []*component.Component{credentialsComp, elasticsearchComp, storageContractComp} {
		if err := comp.Reconcile(ctx, recCtx); err != nil && firstErr == nil {
			firstErr = err
		}
	}

	return firstErr
}

// patchCRConditions derives and server-side-applies the CR-level Ready and
// Suspended conditions plus status.observedGeneration, from the pre-check
// result and the in-memory component conditions the reconcile staged.
func (r *ElasticsearchClusterReconciler) patchCRConditions(
	ctx context.Context,
	cluster *v1.ElasticsearchCluster,
	pre *conditions.PreCheckFailure,
) error {
	componentConds := make([]metav1.Condition, 0, 3)
	for _, condType := range []string{escConditionCredentials, escConditionElasticsearch, escConditionStorageContract} {
		if cond := meta.FindStatusCondition(cluster.Status.Conditions, condType); cond != nil {
			componentConds = append(componentConds, *cond)
		}
	}

	readyReason, readyMessage := conditions.DeriveReady(pre, componentConds, cluster.Spec.Suspend)
	readyStatus := metav1.ConditionFalse
	if readyReason == conditions.ReasonHealthy {
		readyStatus = metav1.ConditionTrue
	}

	suspendedStatus := metav1.ConditionFalse
	suspendedMessage := "Suspension not requested"
	if cluster.Spec.Suspend {
		elasticsearchCond := meta.FindStatusCondition(cluster.Status.Conditions, escConditionElasticsearch)
		if elasticsearchCond != nil && elasticsearchCond.Reason == string(component.Suspended) {
			suspendedStatus = metav1.ConditionTrue
			suspendedMessage = "Node set scaled to zero by spec.suspend"
		} else {
			suspendedMessage = "Suspension in progress"
		}
	}

	return conditions.PatchConditions(ctx, r.Client, cluster, cluster.Generation,
		conditions.Ready(readyStatus, readyReason, readyMessage, cluster.Generation),
		conditions.Suspended(suspendedStatus, suspendedMessage, cluster.Generation),
	)
}

// serviceMonitorSupported reports whether the cluster serves the
// ServiceMonitor kind, so the elasticsearch component omits the resource on
// clusters without the prometheus-operator CRDs instead of failing every
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

// SetupWithManager sets up the controller with the Manager: ownership
// watches on the ECK CR, the user Secret, and the SecondaryStorageConfig,
// plus a preset watch through a field index on spec.presetRef.
func (r *ElasticsearchClusterReconciler) SetupWithManager(mgr ctrl.Manager) error {
	// ocf's ReconcileContext.Recorder is the old events API; the manager
	// only vends it through the deprecated accessor.
	r.Recorder = mgr.GetEventRecorderFor("elasticsearchcluster") //nolint:staticcheck
	r.restMapper = mgr.GetRESTMapper()

	// Uncached: see the componentClient field GoDoc. The apply wrapper
	// sanitizes typed Elasticsearch patches down to the fields the ECK CRD
	// schema declares.
	componentClient, err := client.New(mgr.GetConfig(), client.Options{
		Scheme: mgr.GetScheme(), Mapper: mgr.GetRESTMapper(),
	})
	if err != nil {
		return fmt.Errorf("building the component client: %w", err)
	}
	r.componentClient = eckelasticsearch.NewApplyClient(componentClient)

	if err := mgr.GetFieldIndexer().IndexField(context.Background(), &v1.ElasticsearchCluster{},
		elasticsearchClusterPresetRefField, func(o client.Object) []string {
			if ref := o.(*v1.ElasticsearchCluster).Spec.PresetRef; ref != "" {
				return []string{ref}
			}
			return nil
		}); err != nil {
		return err
	}

	return ctrl.NewControllerManagedBy(mgr).
		For(&v1.ElasticsearchCluster{}).
		Owns(&esv1.Elasticsearch{}).
		Owns(&corev1.Secret{}).
		Owns(&v1.SecondaryStorageConfig{}).
		Watches(&v1.ElasticsearchClusterPreset{},
			refindex.Enqueue(mgr.GetClient(), &v1.ElasticsearchClusterList{},
				elasticsearchClusterPresetRefField, refindex.ObjectName)).
		Named("elasticsearchcluster").
		Complete(r)
}
