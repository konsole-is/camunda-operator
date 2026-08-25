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

package databaseserver

import (
	"context"
	"fmt"
	"strconv"

	cnpgv1 "github.com/cloudnative-pg/api/pkg/api/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/types"

	v1 "github.com/konsole-is/camunda-operator/api/v1"
	components "github.com/konsole-is/camunda-operator/pkg/components/databaseserver"
	"github.com/konsole-is/camunda-operator/pkg/conditions"
)

// The message names both majors, orders the remedies, and closes the door on
// an escape hatch. There is none: the epic defers the upgrade path, so a user
// who forces the change gets an outage and an archive nothing can restore.
const msgVersionChangeRefused = "the effective version %s is not the major version %d that the " +
	"server runs. The operator does not change the major version of a running server. " +
	"CloudNativePG stops every instance for an in-place upgrade of the data directory, no " +
	"point-in-time restore reaches across a major, and the archive of a new major needs a " +
	"directory of its own in the bucket. Set the version back to %d, on the server or on the " +
	"preset it reads. To run PostgreSQL %s, create a DatabaseServer on that version and move " +
	"the data to it. A supported upgrade path comes in a later release, and no annotation lets " +
	"this change through before then"

// eventActionUpgrade is the action of the events that the controller records
// about the PostgreSQL version of the server.
const eventActionUpgrade = "Upgrade"

// refuseMajorVersionChange returns the refusal for a merged version whose
// major is not the one the applied CloudNativePG cluster runs, or nil when the
// two agree. The caller applies nothing on a refusal, so the cluster keeps its
// image.
//
// A server with no applied cluster, and one whose cluster has not reported the
// major of its data directory, are both still bootstrapping and are never
// refused.
func (r *DatabaseServerReconciler) refuseMajorVersionChange(
	ctx context.Context,
	server *v1.DatabaseServer,
	merged v1.DatabaseServerSpec,
) (*conditions.PreCheckFailure, error) {
	var cluster cnpgv1.Cluster
	key := types.NamespacedName{Namespace: server.Namespace, Name: components.ClusterName(server)}
	if err := r.Get(ctx, key, &cluster); err != nil {
		if apierrors.IsNotFound(err) {
			return nil, nil
		}

		return nil, fmt.Errorf("reading the applied cluster %s: %w", key, err)
	}

	return refusedMajorChange(merged.Version, cluster.Status.PGDataImageInfo), nil
}

// refusedMajorChange compares version with the major that running reports for
// the data directory of the server. It refuses a change in either direction: a
// lower major is not an upgrade at all, and CloudNativePG has no path back.
// A version that does not parse is already reported by ValidateMerged.
func refusedMajorChange(version string, running *cnpgv1.ImageInfo) *conditions.PreCheckFailure {
	if running == nil || running.MajorVersion == 0 {
		return nil
	}

	major, err := strconv.Atoi(version)
	if err != nil || major == running.MajorVersion {
		return nil
	}

	return &conditions.PreCheckFailure{
		Reason: v1.ReasonVersionChangeRefused,
		Message: fmt.Sprintf(
			msgVersionChangeRefused,
			version, running.MajorVersion, running.MajorVersion, version,
		),
	}
}

// recordRefusedVersionChange records the Warning event for refused, unless the
// server already reports that refusal on Ready.
//
// The Ready condition answers this on its own. Every reconcile reads the
// server live rather than from the cache, so the condition it carries is what
// the last reconcile wrote. A memo of the recorded refusals, which the
// CamundaCluster downgrade guard keeps because it reads its cluster from the
// cache, would answer the same question from a second source.
func (r *DatabaseServerReconciler) recordRefusedVersionChange(
	server *v1.DatabaseServer,
	refused *conditions.PreCheckFailure,
) {
	ready := meta.FindStatusCondition(server.Status.Conditions, v1.ConditionReady)
	stands := ready != nil && ready.Reason == refused.Reason &&
		ready.Message == conditions.BoundMessage(refused.Message)
	if stands {
		return
	}

	r.EventRecorder.Eventf(
		server,
		nil,
		corev1.EventTypeWarning,
		v1.ReasonVersionChangeRefused,
		eventActionUpgrade,
		"%s",
		refused.Message,
	)
}
