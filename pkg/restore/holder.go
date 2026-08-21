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

	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
)

// unknownHolder names both causes of a broker volume that does not finish
// terminating, and the remedy for each. A lookup that finds no pod cannot
// tell the two apart.
const unknownHolder = "a pod still holds it. A cluster that is not suspended keeps its broker pods, " +
	"and a restore that failed keeps its Jobs. Suspend the cluster, or delete the restore that ran " +
	"before this one"

// terminatingMessage says why a broker data volume still terminates, and what
// frees it.
//
// A pod that mounts the volume keeps it alive under the
// kubernetes.io/pvc-protection finalizer, so the message names that pod and
// the resource that runs it. Two causes reach here. A cluster that nobody
// suspended still runs its broker pods, and a restore that failed still keeps
// its Jobs. The remedy follows the holder, and a pod of any other workload
// gets no remedy that the operator cannot promise.
//
// A lookup that finds no pod, and a lookup that fails, both report the two
// causes together. The wording of a wait is never worth a failed restore, so
// the error is logged and the restore goes on.
func terminatingMessage(
	ctx context.Context,
	reader client.Reader,
	target *Target,
	claim string,
) string {
	holder, err := claimHolder(ctx, reader, target, claim)
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
//
// The pod alone does not name a cause that a user can act on. A restore Job
// pod belongs to a Job, and that Job belongs to a restore, so the lookup
// follows the controller reference one step further when it finds a Job.
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
			return workloadHolder(pod.Name, target.StatefulSet.Name, controller), nil
		}

		return jobHolder(ctx, reader, namespace, pod.Name, controller.Name)
	}

	return "", nil
}

// workloadHolder names a pod that a workload runs.
//
// The broker StatefulSet of the target is the one workload whose remedy the
// operator knows: a suspended cluster runs no broker pod, so the volume goes
// free. Any other workload belongs to somebody else, and the message asks for
// the pod instead of naming a remedy the operator cannot promise.
func workloadHolder(pod, brokers string, controller *metav1.OwnerReference) string {
	if controller.Kind == "StatefulSet" && controller.Name == brokers {
		return fmt.Sprintf(
			"pod %s of StatefulSet %s holds it. Suspend the cluster to free the volume",
			pod, brokers,
		)
	}

	return fmt.Sprintf(
		"pod %s of %s %s holds it. Remove that pod, or the workload that runs it, to free the volume",
		pod, controller.Kind, controller.Name,
	)
}

// jobHolder names the resource that runs the pod, through the Job that owns
// it. A Job that no resource owns names itself.
func jobHolder(
	ctx context.Context,
	reader client.Reader,
	namespace, pod, name string,
) (string, error) {
	key := types.NamespacedName{Namespace: namespace, Name: name}

	var job batchv1.Job
	if err := reader.Get(ctx, key, &job); err != nil {
		return "", fmt.Errorf("reading the Job %s: %w", key, err)
	}

	owner := metav1.GetControllerOf(&job)
	if owner == nil {
		return fmt.Sprintf(
			"pod %s of Job %s holds it. Remove that Job to free the volume", pod, name,
		), nil
	}

	return fmt.Sprintf(
		"pod %s holds it. The pod belongs to a Job of the %s %s. Delete that %s to free the volume",
		pod, owner.Kind, owner.Name, owner.Kind,
	), nil
}

// mountsClaim reports whether pod uses the named claim as one of its volumes.
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
