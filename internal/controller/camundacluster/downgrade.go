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
	"sync"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	v1 "github.com/konsole-is/camunda-operator/api/v1"
	components "github.com/konsole-is/camunda-operator/pkg/components/camundacluster"
	"github.com/konsole-is/camunda-operator/pkg/conditions"
)

// A restore cannot serve as the remedy while the refusal stands: it suspends
// the cluster first, and a refused cluster applies nothing, so the brokers
// never stop. The message therefore orders the remedies.
const msgVersionDowngradeRefused = "the effective version %s is below the running version %s. " +
	"Camunda does not support a downgrade of a running cluster: a broker that starts on data that " +
	"a newer version wrote marks itself unhealthy. Set the version to %s or later. To run %s on " +
	"the data of a backup taken with it, restore that backup after that: the restore sets the " +
	"version itself, and it cannot start while this refusal stands. To downgrade on purpose over " +
	"the data the brokers have, set the annotation %s=%q on the cluster"

// refusalMemo remembers the downgrade refusal that the controller last
// recorded an event for, one entry per cluster. The cluster that a reconcile
// receives cannot answer that on its own. Its Ready condition comes from a
// cache that can be older than the last status write.
type refusalMemo struct {
	mu      sync.Mutex
	entries map[client.ObjectKey]refusalEntry
}

// refusalEntry is what the memo holds for one cluster. The UID is part of the
// entry. A cluster recreated under the name of a deleted one therefore starts
// with no recorded refusal.
type refusalEntry struct {
	uid     types.UID
	message string
}

// consumeDowngradeSanction removes the downgrade annotation from cluster once
// the brokers carry the version it names, and as soon as it names a version
// that the cluster is not asked to run. The sanction covers one move to the
// effective version, whoever set it. A merge patch removes the key from every
// manager that declared it, which is what a spent sanction needs. The patch
// carries the resource version, so a concurrent write conflicts and the
// reconcile retries.
func (r *CamundaClusterReconciler) consumeDowngradeSanction(
	ctx context.Context,
	cluster *v1.CamundaCluster,
	in components.Input,
	storage brokerStorage,
) error {
	sanctioned, ok := cluster.Annotations[components.AllowVersionDowngradeAnnotation]
	if !ok || (sanctioned == in.Effective.Version && sanctioned != storage.runningVersion()) {
		return nil
	}

	before := cluster.DeepCopy()
	delete(cluster.Annotations, components.AllowVersionDowngradeAnnotation)
	patch := client.MergeFromWithOptions(before, client.MergeFromWithOptimisticLock{})
	if err := r.Patch(ctx, cluster, patch); err != nil {
		return fmt.Errorf(
			"removing the %s annotation of CamundaCluster %q: %w",
			components.AllowVersionDowngradeAnnotation, cluster.Name, err,
		)
	}

	return nil
}

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

// recordRefusedDowngrade records the Warning event for refused, unless this
// controller already recorded that refusal for cluster. The memo of the
// recorded refusals answers that for one process. The Ready condition of
// cluster answers it for the first reconcile after a restart, which begins
// with an empty memo.
func (r *CamundaClusterReconciler) recordRefusedDowngrade(cluster *v1.CamundaCluster, refused metav1.Condition) {
	recorded := r.refusals.recorded(cluster, refused.Message)

	ready := meta.FindStatusCondition(cluster.Status.Conditions, v1.ConditionReady)
	stands := ready != nil && ready.Reason == refused.Reason && ready.Message == refused.Message

	if recorded || stands {
		return
	}

	r.EventRecorder.Eventf(
		cluster,
		nil,
		corev1.EventTypeWarning,
		v1.ReasonVersionDowngradeRefused,
		eventActionReconcile,
		"%s",
		refused.Message,
	)
}

// recorded stores message as the refusal of cluster and reports whether the
// memo already held that exact message for it. The entry carries the UID of
// the cluster, so a cluster created again under the same name counts as new.
func (m *refusalMemo) recorded(cluster *v1.CamundaCluster, message string) bool {
	key := client.ObjectKeyFromObject(cluster)

	m.mu.Lock()
	defer m.mu.Unlock()

	if m.entries == nil {
		m.entries = map[client.ObjectKey]refusalEntry{}
	}

	held := m.entries[key]
	m.entries[key] = refusalEntry{uid: cluster.UID, message: message}

	return held.uid == cluster.UID && held.message == message
}

// forget drops what the memo holds for key.
func (m *refusalMemo) forget(key client.ObjectKey) {
	m.mu.Lock()
	defer m.mu.Unlock()

	delete(m.entries, key)
}
