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

package camundacluster

import (
	"context"
	"fmt"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	v1 "github.com/konsole-is/camunda-operator/api/v1"
	"github.com/konsole-is/camunda-operator/pkg/conditions"
)

// LiveZeebeWorkload reads the Zeebe StatefulSet of cluster through reader. Its
// pod template is what Zeebe runs, or rolls to. A workload that does not
// exist yet returns a failure with reason Progressing. Any other read error
// comes back as an error.
func LiveZeebeWorkload(
	ctx context.Context,
	reader client.Reader,
	cluster *v1.CamundaCluster,
) (*appsv1.StatefulSet, *conditions.PreCheckFailure, error) {
	var workload appsv1.StatefulSet
	key := types.NamespacedName{
		Namespace: cluster.Namespace,
		Name:      WorkloadName(cluster, ComponentZeebe),
	}
	if err := reader.Get(ctx, key, &workload); err != nil {
		if apierrors.IsNotFound(err) {
			return nil, &conditions.PreCheckFailure{
				Reason:  v1.ReasonProgressing,
				Message: fmt.Sprintf("the Zeebe workload %s is not rendered yet", key),
			}, nil
		}

		return nil, nil, fmt.Errorf("reading the Zeebe workload %s: %w", key, err)
	}

	return &workload, nil, nil
}

// RunningConfigHash returns the config hash that the pod template of
// workload carries. Mutable referents, for example a SecondaryStorageConfig,
// change this hash without a change of the cluster generation. A template
// without a hash returns a failure with reason Progressing.
func RunningConfigHash(workload *appsv1.StatefulSet) (string, *conditions.PreCheckFailure) {
	hash := workload.Spec.Template.Annotations[ConfigHashAnnotation]
	if hash == "" {
		return "", &conditions.PreCheckFailure{
			Reason: v1.ReasonProgressing,
			Message: fmt.Sprintf(
				"the Zeebe workload %s/%s carries no config hash yet", workload.Namespace, workload.Name,
			),
		}
	}

	return hash, nil
}

// RolledOut returns nil when every replica of workload runs its current pod
// template. A workload that still rolls returns a failure with reason
// Progressing. The pod template changes before the first pod restarts, so
// only this check shows that the pods run what the template says.
func RolledOut(workload *appsv1.StatefulSet) *conditions.PreCheckFailure {
	replicas := int32(1)
	if workload.Spec.Replicas != nil {
		replicas = *workload.Spec.Replicas
	}

	status := workload.Status
	if status.ObservedGeneration == workload.Generation && status.UpdatedReplicas == replicas &&
		status.CurrentRevision == status.UpdateRevision {
		return nil
	}

	return &conditions.PreCheckFailure{
		Reason: v1.ReasonProgressing,
		Message: fmt.Sprintf(
			"the Zeebe workload %s/%s has not rolled out its current pod template yet (generation %d, "+
				"observed %d, %d of %d replicas updated)",
			workload.Namespace, workload.Name, workload.Generation, status.ObservedGeneration,
			status.UpdatedReplicas, replicas,
		),
	}
}

// TemplateEnvValue returns the plain value of the environment variable name
// on any container of template, and whether one carries it as a plain value.
// A variable that takes its value from a reference does not count.
func TemplateEnvValue(template *corev1.PodTemplateSpec, name string) (string, bool) {
	for i := range template.Spec.Containers {
		for _, env := range template.Spec.Containers[i].Env {
			if env.Name == name && env.ValueFrom == nil {
				return env.Value, true
			}
		}
	}

	return "", false
}
