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

// Package podstate reports a pod that cannot start on its own. The kubelet
// retries such a pod without end, the Job that owns it stays active, and the
// Job consumes no backoff. A controller that waits on a Job therefore never
// learns about it from the Job alone.
package podstate

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	v1 "github.com/konsole-is/camunda-operator/api/v1"
	"github.com/konsole-is/camunda-operator/pkg/conditions"
)

// stuckWaitingReasons are the container waiting states that mean the pod will
// not start on its own. Either the configuration that it mounts does not
// resolve, or its image does not pull. Each maps to the Ready reason that the
// caller reports.
var stuckWaitingReasons = map[string]string{
	"CreateContainerConfigError": v1.ReasonMissingSecret,
	"CreateContainerError":       v1.ReasonMissingSecret,
	"ErrImagePull":               v1.ReasonInvalidReference,
	"ImagePullBackOff":           v1.ReasonInvalidReference,
	"InvalidImageName":           v1.ReasonInvalidReference,
}

// Stuck reports the first pod under selector that cannot start, as a
// pre-check failure that names the pod, the container, and the waiting
// reason. Such a pod has a container in a non-progressing waiting state, or
// the scheduler cannot place it, for example on a volume that never binds.
// The what argument names the work that the pod does, for example "the dump
// Job". It goes into the message. Stuck returns nil when every pod
// progresses.
//
// The reader must be uncached. A pod that just entered a waiting state is the
// reason to call this at all.
func Stuck(
	ctx context.Context,
	reader client.Reader,
	namespace string,
	selector map[string]string,
	what string,
) (*conditions.PreCheckFailure, error) {
	var pods corev1.PodList
	if err := reader.List(
		ctx, &pods, client.InNamespace(namespace), client.MatchingLabels(selector),
	); err != nil {
		return nil, fmt.Errorf("listing the pods of %s: %w", what, err)
	}

	for i := range pods.Items {
		if failure := stuckPod(&pods.Items[i], what); failure != nil {
			return failure, nil
		}
	}

	return nil, nil
}

// stuckPod classifies one pod. A pod that already reached a terminal phase
// tells nothing about starting, whatever its last container status says.
func stuckPod(pod *corev1.Pod, what string) *conditions.PreCheckFailure {
	if pod.Status.Phase != corev1.PodPending && pod.Status.Phase != corev1.PodRunning {
		return nil
	}

	statuses := append(
		append([]corev1.ContainerStatus{}, pod.Status.InitContainerStatuses...),
		pod.Status.ContainerStatuses...,
	)
	for _, cs := range statuses {
		waiting := cs.State.Waiting
		if waiting == nil {
			continue
		}
		if reason, stuck := stuckWaitingReasons[waiting.Reason]; stuck {
			return &conditions.PreCheckFailure{
				Reason: reason,
				Message: fmt.Sprintf(
					"pod %s of %s cannot start: container %s reports %s: %s",
					pod.Name, what, cs.Name, waiting.Reason, waiting.Message,
				),
			}
		}
	}

	for _, cond := range pod.Status.Conditions {
		if cond.Type == corev1.PodScheduled && cond.Status == corev1.ConditionFalse &&
			cond.Reason == corev1.PodReasonUnschedulable {
			return &conditions.PreCheckFailure{
				Reason: v1.ReasonProgressing,
				Message: fmt.Sprintf(
					"pod %s of %s cannot be scheduled: %s", pod.Name, what, cond.Message,
				),
			}
		}
	}

	return nil
}
