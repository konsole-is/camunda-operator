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
	"slices"
	"strconv"

	cnpgv1 "github.com/cloudnative-pg/api/pkg/api/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	v1 "github.com/konsole-is/camunda-operator/api/v1"
	components "github.com/konsole-is/camunda-operator/pkg/components/databaseserver"
	"github.com/konsole-is/camunda-operator/pkg/conditions"
	"github.com/konsole-is/camunda-operator/pkg/labels"
)

const msgVersionChangeRefused = "The version %s that the spec asks for is not the major version %d that " +
	"the server runs. The operator does not change the major version of a running server. " +
	"CloudNativePG stops every instance to upgrade the data directory in place. No " +
	"point-in-time restore reaches across a major. The archive of a new major needs a directory " +
	"of its own in the bucket. The server keeps running %d until the version names that major " +
	"again, on the server or on the preset it reads. To run PostgreSQL %s, create a " +
	"DatabaseServer on that version and move the data to it. A supported upgrade path comes in a " +
	"later release, and no annotation lets this change through before then"

// eventActionUpgrade is the action of the events that the controller records
// about the PostgreSQL version of the server.
const eventActionUpgrade = "Upgrade"

// keepRunningVersion pins merged.Version to the PostgreSQL major that the data
// directory of the server runs, when merged names another one, and returns the
// refusal that Ready reports for it. It returns nil when the two agree.
//
// The server keeps running rather than stopping. Everything the reconcile
// renders takes the pinned version, so a rollback in flight finishes on the
// major it started from, and the contract, the archive, and the monitoring
// stay maintained while the refusal stands.
//
// The applied cluster is read live. A cached copy that predates the major
// CloudNativePG reported lets the change through once, and once is all
// CloudNativePG needs to start the upgrade.
func (r *DatabaseServerReconciler) keepRunningVersion(
	ctx context.Context,
	server *v1.DatabaseServer,
	merged *v1.DatabaseServerSpec,
) (*conditions.PreCheckFailure, error) {
	cluster, err := r.runningCluster(ctx, server, merged.DatabaseServerConfig)
	if err != nil || cluster == nil {
		return nil, err
	}

	running := cluster.Status.PGDataImageInfo
	refused := refusedMajorChange(merged.Version, running)
	if refused == nil {
		return nil, nil
	}

	merged.Version = strconv.Itoa(running.MajorVersion)

	return refused, nil
}

// runningCluster returns the CloudNativePG cluster whose data directory the
// server runs on, or nil when it runs on none.
//
// status.cluster names it, and Reconcile fills an empty one with the name of
// the server. Neither is certain: a rollback whose status write was lost leaves
// that field on the cluster the rollback already removed, and the name of the
// server is that same cluster. The contracts the server owns are the second
// record of which cluster it runs from, and reconcileRecovery repairs
// status.cluster from one of them a step further on.
//
// Every owned contract is read, not the one the spec names now: a rename that
// lands before the repair leaves the name of the spec unpublished, and the
// cluster is named by the contract of the name before it. They are read live,
// because a guard that finds nothing lets the change through.
func (r *DatabaseServerReconciler) runningCluster(
	ctx context.Context,
	server *v1.DatabaseServer,
	contractName string,
) (*cnpgv1.Cluster, error) {
	cluster, err := r.clusterOrNil(ctx, server.Namespace, components.ClusterName(server))
	if err != nil || cluster != nil {
		return cluster, err
	}

	var contracts v1.DatabaseServerConfigList
	if err := r.APIReader.List(
		ctx, &contracts,
		client.InNamespace(server.Namespace),
		client.MatchingLabels{labels.DatabaseServerKey: labels.OwnerName(server.Name)},
	); err != nil {
		return nil, fmt.Errorf("listing the contracts of %q: %w", server.Name, err)
	}

	for _, published := range contractsByPreference(server, contractName, contracts.Items) {
		if !ownedByServer(server, &published) {
			continue
		}

		name, err := r.recoveredClusterOf(ctx, server, &published)
		if err != nil {
			return nil, err
		}
		if name == "" {
			continue
		}

		recovered, err := r.clusterOrNil(ctx, server.Namespace, name)
		if err != nil || recovered != nil {
			return recovered, err
		}
	}

	return nil, nil
}

// contractsByPreference orders published so that the contract a rollback
// answers on comes first, and the one the spec names now comes next. The
// rollback contract names the cluster the rollback moved to, and a contract
// the server published under an earlier name can still name a cluster it has
// replaced since.
func contractsByPreference(
	server *v1.DatabaseServer,
	contractName string,
	published []v1.DatabaseServerConfig,
) []v1.DatabaseServerConfig {
	answering := ""
	if running := server.Status.Recovery; running != nil {
		answering = running.Contract
	}

	rank := func(name string) int {
		switch name {
		case answering:
			return 0
		case contractName:
			return 1
		default:
			return 2
		}
	}

	ordered := slices.Clone(published)
	slices.SortStableFunc(ordered, func(a, b v1.DatabaseServerConfig) int {
		return rank(a.Name) - rank(b.Name)
	})

	return ordered
}

// clusterOrNil reads one CloudNativePG cluster of the server, or nil when it
// does not exist.
func (r *DatabaseServerReconciler) clusterOrNil(
	ctx context.Context,
	namespace, name string,
) (*cnpgv1.Cluster, error) {
	key := types.NamespacedName{Namespace: namespace, Name: name}

	var cluster cnpgv1.Cluster
	if err := r.APIReader.Get(ctx, key, &cluster); err != nil {
		if apierrors.IsNotFound(err) {
			return nil, nil
		}

		return nil, fmt.Errorf("reading the applied cluster %s: %w", key, err)
	}

	return &cluster, nil
}

// refusedMajorChange compares version with the major that running reports for
// the data directory of the server. It refuses a change in either direction: a
// lower major is not an upgrade at all, and CloudNativePG has no path back.
//
// A nil running means CloudNativePG has not written the data directory yet, so
// the server is still bootstrapping and is never refused. Every CloudNativePG
// release this operator supports reports the field: it arrived in 1.26, which
// is the floor that the installation docs name.
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

// recordRefusedVersionChange records the Warning event for refused, unless
// standing already reports it. standing is the Ready condition as the last
// reconcile left it, because this one stages over it before the event is
// recorded.
func (r *DatabaseServerReconciler) recordRefusedVersionChange(
	server *v1.DatabaseServer,
	standing *metav1.Condition,
	refused *conditions.PreCheckFailure,
) {
	if standing != nil && standing.Reason == refused.Reason &&
		standing.Message == conditions.BoundMessage(refused.Message) {
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
