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
	components "github.com/konsole-is/camunda-operator/pkg/components/elasticsearchcluster"
	"github.com/konsole-is/camunda-operator/pkg/conditions"
	"github.com/konsole-is/camunda-operator/pkg/credentials"
	"github.com/konsole-is/camunda-operator/pkg/refindex"
	"github.com/konsole-is/camunda-operator/pkg/wrappers/eckelasticsearch"
)

// elasticsearchClusterPresetRefField indexes ElasticsearchClusters by
// spec.presetRef, so a preset edit enqueues every cluster that references it.
const elasticsearchClusterPresetRefField = "elasticsearchcluster.spec.presetRef"

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
	// Recorder publishes the component lifecycle events. SetupWithManager
	// sets it from the manager.
	Recorder record.EventRecorder

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
// +kubebuilder:rbac:groups="",resources=events,verbs=create;patch
// +kubebuilder:rbac:groups=monitoring.coreos.com,resources=servicemonitors,verbs=get;list;watch;create;update;patch;delete

// Reconcile converges an ElasticsearchCluster. It resolves the preset, runs
// the pre-checks, reconciles the credentials, elasticsearch, and
// storage-contract components in dependency order, and derives the CR-level
// Ready and Suspended conditions.
//
// Status is written once per reconcile. The components and stageConditions
// stage conditions on the in-memory cluster, and the deferred FlushStatus
// persists them together.
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
	defer func() {
		if flushErr := component.FlushStatus(ctx, recCtx); flushErr != nil {
			err = errors.Join(err, flushErr)
		}
	}()

	merged, err := r.preCheck(ctx, &cluster)
	var failure *conditions.PreCheckFailure
	if errors.As(err, &failure) {
		stageConditions(&cluster, failure)
		return ctrl.Result{}, nil
	}
	if err != nil {
		return ctrl.Result{}, err
	}

	reconcileErr := r.reconcileComponents(ctx, recCtx, &cluster, merged)
	stageConditions(&cluster, nil)

	return ctrl.Result{}, reconcileErr
}

// preCheck resolves the preset and validates the merged spec. It returns the
// preset-merged spec. A failed check returns a *conditions.PreCheckFailure
// that carries its Ready reason. A dangling presetRef, an incomplete merge, a
// version below the floor, and a storageSize shrink relative to the applied
// ECK CR all report InvalidReference. Any other error is a transient API
// failure.
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

	return merged, r.checkStorageShrink(ctx, cluster, merged)
}

// checkStorageShrink guards against the shrinks that admission cannot see: a
// preset baseline lowered under a referencing cluster, or an inline
// storageSize set below a size that a preset provided before. It compares the
// merged size against the data volume claim of the applied ECK CR. A shrink
// is reported and not applied, because Elasticsearch data volumes cannot be
// reduced in place.
func (r *ElasticsearchClusterReconciler) checkStorageShrink(
	ctx context.Context,
	cluster *v1.ElasticsearchCluster,
	merged v1.ElasticsearchClusterSpec,
) error {
	var es esv1.Elasticsearch
	if err := r.Get(ctx, client.ObjectKeyFromObject(cluster), &es); err != nil {
		if apierrors.IsNotFound(err) {
			return nil
		}
		return fmt.Errorf("reading applied Elasticsearch %q: %w", cluster.Name, err)
	}

	applied := appliedDataClaimSize(&es)
	if applied == nil {
		return nil
	}

	if merged.StorageSize.Cmp(*applied) < 0 {
		return &conditions.PreCheckFailure{
			Reason: v1.ReasonInvalidReference,
			Message: fmt.Sprintf(
				"storageSize %s would shrink the applied data volume size %s; Elasticsearch data volumes cannot be reduced",
				merged.StorageSize,
				applied,
			),
		}
	}

	return nil
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

// reconcileComponents builds and reconciles the three components in
// dependency order. It continues past a failing component, so one failure
// does not stall the rest, and returns the first error. It reads the password
// from the existing user Secret without the cache, so the password stays
// stable after creation. To rotate it, delete the Secret.
func (r *ElasticsearchClusterReconciler) reconcileComponents(
	ctx context.Context,
	recCtx component.ReconcileContext,
	cluster *v1.ElasticsearchCluster,
	merged v1.ElasticsearchClusterSpec,
) error {
	password, err := credentials.LookupOrNew(
		ctx, r.APIReader, client.ObjectKey{
			Namespace: cluster.Namespace, Name: components.UserSecretName(cluster),
		}, components.PasswordKey,
	)
	if err != nil {
		return err
	}

	credentialsComp, err := components.CredentialsComponent(cluster, password)
	if err != nil {
		return fmt.Errorf("building credentials component: %w", err)
	}

	elasticsearchComp, err := components.ElasticsearchComponent(cluster, merged, r.serviceMonitorSupported())
	if err != nil {
		return fmt.Errorf("building elasticsearch component: %w", err)
	}

	storageContractComp, err := components.StorageContractComponent(cluster, merged)
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

// stageConditions sets the CR-level Ready and Suspended conditions and
// observedGeneration on the in-memory cluster. A pre-check failure becomes
// Ready directly. Otherwise Ready mirrors the representative component
// condition. Suspended is True while that representative reports the ocf
// Suspended status. FlushStatus persists them.
func stageConditions(cluster *v1.ElasticsearchCluster, pre *conditions.PreCheckFailure) {
	ready := conditions.Ready(metav1.ConditionFalse, "", "", cluster.Generation)
	if pre != nil {
		ready.Reason, ready.Message = pre.Reason, pre.Message
	} else {
		ready = conditions.Aggregate(componentConditions(cluster), cluster.Generation)
	}

	suspendedStatus := metav1.ConditionFalse
	suspendedMessage := "Suspension not requested"
	switch {
	case ready.Reason == string(component.Suspended):
		suspendedStatus = metav1.ConditionTrue
		suspendedMessage = "Node set scaled to zero by spec.suspend"
	case cluster.Spec.Suspend:
		suspendedMessage = "Suspension in progress"
	}

	meta.SetStatusCondition(&cluster.Status.Conditions, ready)
	meta.SetStatusCondition(
		&cluster.Status.Conditions,
		components.SuspendedCondition(suspendedStatus, suspendedMessage, cluster.Generation),
	)
	cluster.Status.ObservedGeneration = cluster.Generation
}

// componentConditions returns the ocf component conditions on cluster, in
// component registration order.
func componentConditions(cluster *v1.ElasticsearchCluster) []metav1.Condition {
	conds := make([]metav1.Condition, 0, 3)
	for _, condType := range []string{
		components.ConditionCredentials, components.ConditionElasticsearch, components.ConditionStorageContract,
	} {
		if cond := meta.FindStatusCondition(cluster.Status.Conditions, condType); cond != nil {
			conds = append(conds, *cond)
		}
	}
	return conds
}

// serviceMonitorSupported reports whether the cluster serves the
// ServiceMonitor kind. On a cluster without the prometheus-operator CRDs the
// elasticsearch component then omits the resource instead of failing every
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
// the user Secret, and the SecondaryStorageConfig, and a preset watch through
// a field index on spec.presetRef.
func (r *ElasticsearchClusterReconciler) SetupWithManager(mgr ctrl.Manager) error {
	// The ReconcileContext.Recorder of ocf is the old events API, and the
	// manager only vends it through the deprecated accessor.
	r.Recorder = mgr.GetEventRecorderFor("elasticsearchcluster") //nolint:staticcheck
	r.restMapper = mgr.GetRESTMapper()

	// Uncached: see the componentClient field doc. The apply wrapper
	// sanitizes typed Elasticsearch patches down to the fields that the ECK
	// CRD schema declares.
	componentClient, err := client.New(mgr.GetConfig(), client.Options{
		Scheme: mgr.GetScheme(), Mapper: mgr.GetRESTMapper(),
	})
	if err != nil {
		return fmt.Errorf("building the component client: %w", err)
	}
	r.componentClient = eckelasticsearch.NewApplyClient(componentClient)

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
		Owns(&corev1.Secret{}).
		Owns(&v1.SecondaryStorageConfig{}).
		Watches(
			&v1.ElasticsearchClusterPreset{},
			refindex.Enqueue(
				mgr.GetClient(), &v1.ElasticsearchClusterList{},
				elasticsearchClusterPresetRefField, refindex.ObjectName,
			),
		).
		Named("elasticsearchcluster").
		Complete(r)
}
