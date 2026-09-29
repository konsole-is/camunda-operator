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

package restore

import (
	"context"
	"fmt"
	"slices"
	"strings"

	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
)

// unknownHolder names both causes of a broker volume that does not finish
// terminating, because a lookup that finds no pod cannot tell them apart.
const unknownHolder = "a pod still holds it. A cluster that is not suspended keeps its broker pods, " +
	"and a restore that failed keeps its Jobs. Suspend the cluster, or delete the restore that ran " +
	"before this one"

// terminatingMessage says why a broker data volume still terminates, and what
// frees it.
func terminatingMessage(
	ctx context.Context,
	reader client.Reader,
	target *Target,
	claim string,
) string {
	holder, err := claimHolder(ctx, reader, target, claim)
	// The wording of a wait is not worth a failed restore, so the error is only logged.
	if err != nil {
		logf.FromContext(ctx).Error(
			err, "Could not read the pod that holds a terminating broker volume", "claim", claim,
		)
	}
	if holder == "" {
		holder = unknownHolder
	}

	return fmt.Sprintf("the broker volume %s is still terminating because %s", claim, holder)
}

// claimHolder returns the phrase that names the pod which still holds claim,
// together with the remedy for it. It returns the empty string when no pod of
// the namespace mounts the claim.
func claimHolder(
	ctx context.Context,
	reader client.Reader,
	target *Target,
	claim string,
) (string, error) {
	namespace := target.StatefulSet.Namespace

	var pods corev1.PodList
	if err := reader.List(ctx, &pods, client.InNamespace(namespace)); err != nil {
		return "", fmt.Errorf("listing the pods of namespace %s: %w", namespace, err)
	}

	// The order of a list is not part of its contract. Without the sort, two
	// pods of one claim take turns in the message on every look.
	slices.SortFunc(pods.Items, func(a, b corev1.Pod) int {
		return strings.Compare(a.Name, b.Name)
	})

	for i := range pods.Items {
		pod := &pods.Items[i]
		if !mountsClaim(pod, claim) {
			continue
		}

		controller := metav1.GetControllerOf(pod)
		switch {
		case controller == nil:
			return fmt.Sprintf("pod %s holds it. Remove that pod to free the volume", pod.Name), nil
		case !isJob(controller):
			return workloadHolder(pod.Name, target.StatefulSet, controller), nil
		}

		return jobHolder(ctx, reader, namespace, pod.Name, controller)
	}

	return "", nil
}

func mountsClaim(pod *corev1.Pod, claim string) bool {
	return slices.ContainsFunc(pod.Spec.Volumes, func(volume corev1.Volume) bool {
		return volume.PersistentVolumeClaim != nil &&
			volume.PersistentVolumeClaim.ClaimName == claim
	})
}

// isJob reports whether the reference names a batch/v1 Job. The kind alone is
// not enough: another API group can declare a kind of the same name.
func isJob(ref *metav1.OwnerReference) bool {
	return ref.Kind == "Job" && ref.APIVersion == batchv1.SchemeGroupVersion.String()
}

// workloadHolder names a pod that a workload runs. Only the broker StatefulSet
// of the target gets the remedy to suspend the cluster: a suspended cluster
// runs no broker pod.
func workloadHolder(pod string, brokers *appsv1.StatefulSet, controller *metav1.OwnerReference) string {
	// The UID decides: a StatefulSet created again under one name leaves the
	// pods of its predecessor, and a suspend removes none of them.
	if brokers.UID != "" && controller.UID == brokers.UID {
		return fmt.Sprintf(
			"pod %s of StatefulSet %s holds it. Suspend the cluster to free the volume",
			pod, brokers.Name,
		)
	}

	return fmt.Sprintf(
		"pod %s of %s %s holds it. Remove that pod, or the workload that runs it, to free the volume",
		pod, controller.Kind, controller.Name,
	)
}

// jobHolder names the resource that runs the pod, through the Job that owns
// it.
func jobHolder(
	ctx context.Context,
	reader client.Reader,
	namespace, pod string,
	ref *metav1.OwnerReference,
) (string, error) {
	key := types.NamespacedName{Namespace: namespace, Name: ref.Name}

	var job batchv1.Job
	err := reader.Get(ctx, key, &job)
	switch {
	case apierrors.IsNotFound(err):
		return podOfGoneJob(pod, ref.Name), nil
	case err != nil:
		return "", fmt.Errorf("reading the Job %s: %w", key, err)
	}

	owner := metav1.GetControllerOf(&job)
	switch {
	// A restore created again under one name reuses the Job names of its
	// predecessor, so only the UID tells which Job ran the pod.
	case job.UID != ref.UID:
		return podOfGoneJob(pod, ref.Name), nil
	case owner == nil:
		return fmt.Sprintf(
			"pod %s of Job %s holds it. Remove that Job to free the volume", pod, ref.Name,
		), nil
	}

	return fmt.Sprintf(
		"pod %s holds it. The pod belongs to a Job of the %s %s. Delete that %s to free the volume",
		pod, owner.Kind, owner.Name, owner.Kind,
	), nil
}

// podOfGoneJob names a pod whose Job the operator cannot read any more: the
// Job is gone, or another Job holds its name now.
func podOfGoneJob(pod, name string) string {
	return fmt.Sprintf(
		"pod %s holds it. The Job %s that ran it is gone. Remove that pod to free the volume",
		pod, name,
	)
}
