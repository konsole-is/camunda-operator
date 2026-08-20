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

package camundaoptimize

import (
	"context"
	"fmt"

	monitoringv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	v1 "github.com/konsole-is/camunda-operator/api/v1"
	components "github.com/konsole-is/camunda-operator/pkg/components/camundaoptimize"
	"github.com/konsole-is/camunda-operator/pkg/labels"
)

// attachmentHolder returns the CamundaOptimize that holds the attachment to
// the cluster that optimize names: the oldest one, and among equally old ones
// the one whose name sorts first. Every reconcile of every attached
// CamundaOptimize picks the same holder, so the two cannot take the
// attachment from each other.
//
// One cluster carries one Optimize instance. The Optimize index prefix is
// fixed, so two instances write the same analytics indices of the same
// Elasticsearch, and their pods would carry identical discovery labels, which
// makes each Service route to the pods of both. The holder does the work; the
// others report ClusterAlreadyAttached and touch nothing.
//
// A CamundaOptimize under deletion still counts. It keeps the attachment until
// its finalizer withdraws the exporter settings, so the next holder never
// applies them while the previous one is still removing them.
//
// The list is read live rather than from the cache. A cached list can miss a
// CamundaOptimize that was created moments ago, and both would then believe
// they hold the attachment and fight over the exporter settings.
func (r *Reconciler) attachmentHolder(
	ctx context.Context,
	optimize *v1.CamundaOptimize,
) (*v1.CamundaOptimize, error) {
	var list v1.CamundaOptimizeList
	if err := r.APIReader.List(ctx, &list, client.InNamespace(optimize.Namespace)); err != nil {
		return nil, fmt.Errorf("listing the CamundaOptimizes of namespace %q: %w", optimize.Namespace, err)
	}

	var holder *v1.CamundaOptimize
	for i := range list.Items {
		candidate := &list.Items[i]
		if candidate.Spec.ClusterRef.Name != optimize.Spec.ClusterRef.Name {
			continue
		}
		if holder == nil || olderThan(candidate, holder) {
			holder = candidate
		}
	}

	return holder, nil
}

// olderThan reports whether a was created before b, with the name breaking a
// tie. Two objects created in the same second are common, so the tie-break
// must be deterministic.
func olderThan(a, b *v1.CamundaOptimize) bool {
	if !a.CreationTimestamp.Equal(&b.CreationTimestamp) {
		return a.CreationTimestamp.Before(&b.CreationTimestamp)
	}

	return a.Name < b.Name
}

// holdsAttachment reports whether optimize is the CamundaOptimize that holds
// the attachment to the cluster it names.
func (r *Reconciler) holdsAttachment(ctx context.Context, optimize *v1.CamundaOptimize) (bool, error) {
	holder, err := r.attachmentHolder(ctx, optimize)
	if err != nil {
		return false, err
	}

	return holder != nil && holder.Name == optimize.Name, nil
}

// releaseWorkloads deletes the Deployment, the Service, and the ServiceMonitor
// of each component that optimize controls, and the copies of referenced
// Secrets that it made. A parked CamundaOptimize renders nothing, and a copy of
// a credential in a namespace that nothing reads is the part of "nothing" that
// is easiest to forget.
//
// The attachment can move to a CamundaOptimize that was created later. The
// election breaks a tie on the name, and a creationTimestamp carries whole
// seconds, so one created in the same second with a name that sorts earlier
// takes the attachment from one that already built its workloads. The deposed
// one reports ClusterAlreadyAttached and stops. Without this call its pods keep
// running, and they carry the discovery labels of the new holder, so the
// Services of both route to the pods of both and two importers write the same
// indices.
//
// The objects carry an owner reference, so deleting the CamundaOptimize
// collects them. This path is for the CamundaOptimize that stays.
func (r *Reconciler) releaseWorkloads(ctx context.Context, optimize *v1.CamundaOptimize) error {
	for _, comp := range []string{components.ComponentWebapp, components.ComponentImporter} {
		key := client.ObjectKey{
			Namespace: optimize.Namespace,
			Name:      components.WorkloadName(optimize, comp),
		}

		owned := []client.Object{&appsv1.Deployment{}, &corev1.Service{}}
		if r.serviceMonitorSupported() {
			owned = append(owned, &monitoringv1.ServiceMonitor{})
		}

		for _, obj := range owned {
			if err := r.deleteControlled(ctx, key, obj, optimize); err != nil {
				return err
			}
		}
	}

	for _, purpose := range components.MirrorPurposes {
		key := client.ObjectKey{
			Namespace: optimize.Namespace,
			Name:      components.MirroredSecretName(optimize, purpose),
		}
		if err := r.deleteControlled(ctx, key, &corev1.Secret{}, optimize); err != nil {
			return err
		}
	}

	return nil
}

// removeComponentConditions drops every condition that a component of this
// CamundaOptimize reports. A parked CamundaOptimize builds no components, so
// FlushStatus owns none of these types and leaves whatever the object already
// carries. Without this call a deposed holder keeps a WebappReady of True over
// a Deployment that releaseWorkloads has deleted.
//
// The conditions come back on their own when this CamundaOptimize takes the
// attachment again, because the components write them.
func removeComponentConditions(optimize *v1.CamundaOptimize) {
	for _, conditionType := range []string{
		v1.ConditionWebappReady,
		v1.ConditionImporterReady,
		v1.ConditionMirroredSecretsReady,
	} {
		meta.RemoveStatusCondition(optimize.GetStatusConditions(), conditionType)
	}
}

// deleteControlled deletes the object at key when optimize controls it. A
// missing object is already the wanted state. The ownership check matters
// because the managed labels of two CamundaOptimizes on one cluster are
// identical. Only the owner reference tells their objects apart.
func (r *Reconciler) deleteControlled(
	ctx context.Context,
	key client.ObjectKey,
	obj client.Object,
	optimize *v1.CamundaOptimize,
) error {
	if err := r.APIReader.Get(ctx, key, obj); err != nil {
		if apierrors.IsNotFound(err) || meta.IsNoMatchError(err) {
			return nil
		}
		return fmt.Errorf("reading %T %q: %w", obj, key, err)
	}

	if !metav1.IsControlledBy(obj, optimize) {
		return nil
	}

	if err := r.Delete(ctx, obj); err != nil && !apierrors.IsNotFound(err) {
		return fmt.Errorf("deleting %T %q: %w", obj, key, err)
	}

	return nil
}

// otherImporter returns the name of an importer Deployment of the same cluster
// that optimize does not control, or an empty string when there is none.
//
// The importer Deployments of every CamundaOptimize on one cluster carry the
// same managed labels, so one list finds them all and the owner reference tells
// them apart.
//
// The gate is the Deployment, not the pods behind it. Both handover paths
// delete the Deployment of the previous holder before the new holder renders,
// so the overlap shrinks to the termination of pods that are already ordered to
// stop. Closing it fully needs a pod-level check, which needs a label that
// names the owning CamundaOptimize on the pod template.
func (r *Reconciler) otherImporter(ctx context.Context, optimize *v1.CamundaOptimize) (string, error) {
	var list appsv1.DeploymentList
	err := r.APIReader.List(
		ctx,
		&list,
		client.InNamespace(optimize.Namespace),
		client.MatchingLabels(labels.Managed(
			labels.Cluster(optimize.Spec.ClusterRef.Name), components.ComponentImporter,
		)),
	)
	if err != nil {
		return "", fmt.Errorf("listing the importer Deployments of namespace %q: %w", optimize.Namespace, err)
	}

	for i := range list.Items {
		if !metav1.IsControlledBy(&list.Items[i], optimize) {
			return list.Items[i].Name, nil
		}
	}

	return "", nil
}
