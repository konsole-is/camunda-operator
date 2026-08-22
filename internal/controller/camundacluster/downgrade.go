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

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	v1 "github.com/konsole-is/camunda-operator/api/v1"
	components "github.com/konsole-is/camunda-operator/pkg/components/camundacluster"
	"github.com/konsole-is/camunda-operator/pkg/conditions"
)

// eventReasonVersionDowngradeRefused is recorded once per distinct refusal of
// an effective version below the running one. The Ready condition carries the
// same reason for as long as the refusal stands.
const eventReasonVersionDowngradeRefused = v1.ReasonVersionDowngradeRefused

const msgVersionDowngradeRefused = "the effective version %s is below the running version %s. " +
	"Camunda does not support a downgrade of a running cluster: a broker that starts over data of a " +
	"newer version marks itself unhealthy. Set the version to %s or later, restore a backup taken " +
	"with %s, or set the annotation %s=%q on the cluster to downgrade on purpose"

// refuseDowngrade returns the pre-check failure for an effective version
// below the version that the brokers run, or nil when the move is not a
// downgrade or the annotation sanctions it. The caller applies nothing on a
// failure, so the brokers keep the version they have.
func refuseDowngrade(
	cluster *v1.CamundaCluster,
	in components.Input,
	storage brokerStorage,
) *conditions.PreCheckFailure {
	effective := in.Effective.Version
	running := storage.runningVersion()
	if !components.VersionDowngrade(effective, running) ||
		cluster.Annotations[components.AllowVersionDowngradeAnnotation] == effective {
		return nil
	}

	return &conditions.PreCheckFailure{
		Reason: v1.ReasonVersionDowngradeRefused,
		Message: fmt.Sprintf(
			msgVersionDowngradeRefused,
			effective, running, running, effective,
			components.AllowVersionDowngradeAnnotation, effective,
		),
	}
}

// recordRefusedDowngrade records the Warning event for refused unless the
// Ready condition that the server holds already carries this exact refusal.
// The caller stages refused afterwards, so the next reconcile finds it and
// records nothing.
func (r *CamundaClusterReconciler) recordRefusedDowngrade(cluster *v1.CamundaCluster, refused metav1.Condition) {
	ready := meta.FindStatusCondition(cluster.Status.Conditions, v1.ConditionReady)
	if ready != nil && ready.Reason == refused.Reason && ready.Message == refused.Message {
		return
	}

	r.EventRecorder.Eventf(
		cluster,
		nil,
		corev1.EventTypeWarning,
		eventReasonVersionDowngradeRefused,
		eventActionReconcile,
		"%s",
		refused.Message,
	)
}

// consumeDowngradeSanction removes the downgrade annotation from cluster once
// the brokers carry the version it names. The sanction is spent at that
// point, whoever set it. A merge patch removes the key from every manager
// that declared it, which is what a spent sanction needs.
func (r *CamundaClusterReconciler) consumeDowngradeSanction(
	ctx context.Context,
	cluster *v1.CamundaCluster,
	storage brokerStorage,
) error {
	sanctioned, ok := cluster.Annotations[components.AllowVersionDowngradeAnnotation]
	if !ok || sanctioned != storage.runningVersion() {
		return nil
	}

	before := cluster.DeepCopy()
	delete(cluster.Annotations, components.AllowVersionDowngradeAnnotation)
	if err := r.Patch(ctx, cluster, client.MergeFrom(before)); err != nil {
		return fmt.Errorf(
			"removing the %s annotation of CamundaCluster %q: %w",
			components.AllowVersionDowngradeAnnotation, cluster.Name, err,
		)
	}

	return nil
}
