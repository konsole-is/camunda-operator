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

package logicalbackup

import (
	"fmt"

	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	v1 "github.com/konsole-is/camunda-operator/api/v1"
	"github.com/konsole-is/camunda-operator/pkg/conditions"
)

// ClusterConverged returns nil when the cluster runs the spec that it
// declares: the operator observed the current generation and reports Ready
// for it. Otherwise it returns a failure with reason Progressing, because a
// rollout ends on its own. A backup that starts during a rollout can write
// its parts under two configurations, and a completed status then describes
// a split restore point.
func ClusterConverged(cluster *v1.CamundaCluster) *conditions.PreCheckFailure {
	ready := meta.FindStatusCondition(cluster.Status.Conditions, v1.ConditionReady)
	observed := cluster.Status.ObservedGeneration
	if observed == cluster.Generation && ready != nil &&
		ready.Status == metav1.ConditionTrue && ready.ObservedGeneration == cluster.Generation {
		return nil
	}

	readyState := "absent"
	if ready != nil {
		readyState = fmt.Sprintf("%s/%s at generation %d", ready.Status, ready.Reason, ready.ObservedGeneration)
	}

	return &conditions.PreCheckFailure{
		Reason: v1.ReasonProgressing,
		Message: fmt.Sprintf(
			"CamundaCluster %s/%s has not converged on its current spec (generation %d, observed %d, "+
				"Ready %s); a backup taken now could hold parts of two configurations",
			cluster.Namespace, cluster.Name, cluster.Generation, observed, readyState,
		),
	}
}
