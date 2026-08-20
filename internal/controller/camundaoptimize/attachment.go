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
// of each component that optimize controls.
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

	return nil
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
