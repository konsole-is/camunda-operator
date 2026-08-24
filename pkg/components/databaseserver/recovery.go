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
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	cnpgv1 "github.com/cloudnative-pg/api/pkg/api/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	v1 "github.com/konsole-is/camunda-operator/api/v1"
	"github.com/konsole-is/camunda-operator/pkg/wrappers/cnpgcluster"
)

// RecoveryFieldManager owns spec.pitr.lastRecovery on the published contract.
// The answer to a recovery request travels under a manager of its own,
// because the server has to answer a request it refuses while it is
// suspended, and a suspended server applies no contract at all.
const RecoveryFieldManager client.FieldOwner = "camunda-operator/databaseserver-recovery"

// recoveryNameSeparator sits between the name of the server and the recovery
// number of a recovered cluster.
const recoveryNameSeparator = "-r"

// ErrNoArchiveHolds reports that no archive of the server holds the requested
// point. It is the one refusal that means the request was never possible,
// rather than one that failed.
var ErrNoArchiveHolds = errors.New("no archive of the server holds the requested point")

// RecoveryClusterName returns the name of the CloudNativePG cluster that the
// next recovery of the server builds: the name of the server, the suffix -r,
// and the number of archives the server has written.
//
// The number of archives is what makes the name unique without a counter of
// its own: every recovery starts an archive, so a later recovery always
// counts more of them. A server that stopped and started its archive without
// a recovery in between counts those too, which skips a number and takes
// nothing away. The one name the count can land on that is already taken is
// the cluster that runs now, and the next number is used instead.
func RecoveryClusterName(server *v1.DatabaseServer) string {
	archives := 0
	if server.Status.Archive != nil {
		archives = len(server.Status.Archive.History)
	}

	name := server.Name + recoveryNameSeparator + strconv.Itoa(archives)
	if name == ClusterName(server) {
		return server.Name + recoveryNameSeparator + strconv.Itoa(archives+1)
	}

	return name
}

// SelectArchive returns the archive of history that a recovery to target
// starts from, or an error that names the intervals the server has. An
// interval holds its start and every point up to, but not including, its end,
// so a target on the boundary of two archives belongs to the later one. The
// archive the server writes now has no end and holds every point after its
// start.
//
// The error wraps ErrNoArchiveHolds. It is a refusal of the request, not a
// failure of the recovery: nothing the server does later brings the point
// back.
func SelectArchive(history []v1.ArchiveRecord, target time.Time) (v1.ArchiveRecord, error) {
	// Newest first: the intervals never overlap, and a record that a later
	// recovery superseded is the answer only when nothing newer holds the
	// point.
	for i := len(history) - 1; i >= 0; i-- {
		record := history[i]
		if target.Before(record.From.Time) {
			continue
		}
		if record.To == nil || target.Before(record.To.Time) {
			return record, nil
		}
	}

	return v1.ArchiveRecord{}, fmt.Errorf(
		"%w. It archived %s, and %s lies in none of those windows",
		ErrNoArchiveHolds, describeArchives(history), target.UTC().Format(time.RFC3339),
	)
}

// describeArchives names the windows of history for a refusal message, oldest
// first.
func describeArchives(history []v1.ArchiveRecord) string {
	if len(history) == 0 {
		return "nothing"
	}

	windows := make([]string, 0, len(history))
	for _, record := range history {
		end := "now"
		if record.To != nil {
			end = record.To.UTC().Format(time.RFC3339)
		}
		windows = append(windows, fmt.Sprintf(
			"%s from %s to %s", record.ServerName, record.From.UTC().Format(time.RFC3339), end,
		))
	}

	return strings.Join(windows, ", ")
}

// RecoveryCluster renders the CloudNativePG cluster that a recovery builds:
// the cluster of the server under the name of the recovery, bootstrapped from
// source and stopped at target.
//
// It is applied next to the cluster that runs now, and only the archive tells
// the two apart in the bucket. The recovered cluster writes its own archive
// under its own name, so it never writes over the archive it recovered from.
// target is RFC 3339 with a zone, as the contract carries it.
//
// The name comes from status.recovery.cluster, which the server records
// before it builds anything. Rendering it is an error while that is empty.
func RecoveryCluster(
	server *v1.DatabaseServer,
	merged v1.DatabaseServerSpec,
	archive *ArchiveStorage,
	platform *v1.CamundaPlatformConfigSpec,
	source v1.ArchiveRecord,
	target string,
) (*cnpgv1.Cluster, error) {
	if server.Status.Recovery == nil || server.Status.Recovery.Cluster == "" {
		return nil, errors.New("the server records no recovery cluster to build")
	}
	name := server.Status.Recovery.Cluster

	baseline := cluster(server, merged, platform)
	baseline.Name = name

	resource, err := cnpgcluster.NewBuilder(baseline).
		WithMutation(clusterMutations(server, merged, archive)...).
		WithMutation(recoveryMutation(server, name, source, target)).
		Build()
	if err != nil {
		return nil, fmt.Errorf("building the recovery cluster %q: %w", name, err)
	}

	rendered, err := resource.Preview()
	if err != nil {
		return nil, fmt.Errorf("rendering the recovery cluster %q: %w", name, err)
	}

	recovered := rendered.(*cnpgv1.Cluster)
	// A render carries no kind, and a server-side apply is refused without
	// one. The components apply through the framework, which fills it in;
	// this one is applied by the controller itself.
	recovered.TypeMeta = metav1.TypeMeta{
		APIVersion: cnpgv1.SchemeGroupVersion.String(),
		Kind:       "Cluster",
	}

	return recovered, nil
}

// recoveryMutation turns the baseline cluster into the recovery of source. It
// runs after the mutations of the running cluster, because it takes the
// archive plugin entry those wrote and repoints it at the archive of this
// cluster.
func recoveryMutation(
	server *v1.DatabaseServer,
	name string,
	source v1.ArchiveRecord,
	target string,
) cnpgcluster.Mutation {
	return cnpgcluster.Mutation{
		Name: "RecoverFromArchive",
		Mutate: func(m *cnpgcluster.Mutator) error {
			m.Edit(func(c *cnpgv1.Cluster) error {
				c.Spec.Bootstrap = &cnpgv1.BootstrapConfiguration{
					Recovery: &cnpgv1.BootstrapRecovery{
						Source:         source.ServerName,
						RecoveryTarget: &cnpgv1.RecoveryTarget{TargetTime: target},
					},
				}
				c.Spec.ExternalClusters = []cnpgv1.ExternalCluster{{
					Name: source.ServerName,
					PluginConfiguration: &cnpgv1.PluginConfiguration{
						Name: BarmanPluginName,
						Parameters: map[string]string{
							"barmanObjectName": ObjectStoreName(server),
							"serverName":       source.ServerName,
						},
					},
				}}
				for i := range c.Spec.Plugins {
					if c.Spec.Plugins[i].Name == BarmanPluginName {
						c.Spec.Plugins[i].Parameters["serverName"] = name
					}
				}

				return nil
			})

			return nil
		},
	}
}

// RecoveryOutcomePatch renders the server-side apply that publishes outcome on
// the contract. It carries spec.pitr.lastRecovery and nothing else, so the
// apply states the answer without touching a field that the contract
// component or the consumer owns.
func RecoveryOutcomePatch(
	contract types.NamespacedName,
	outcome v1.RecoveryOutcome,
) (*unstructured.Unstructured, error) {
	fields, err := runtime.DefaultUnstructuredConverter.ToUnstructured(&outcome)
	if err != nil {
		return nil, fmt.Errorf("rendering the recovery outcome of %s: %w", contract, err)
	}

	patch := &unstructured.Unstructured{}
	patch.SetGroupVersionKind(v1.GroupVersion.WithKind("DatabaseServerConfig"))
	patch.SetNamespace(contract.Namespace)
	patch.SetName(contract.Name)
	if err := unstructured.SetNestedMap(patch.Object, fields, "spec", "pitr", "lastRecovery"); err != nil {
		return nil, fmt.Errorf("rendering the recovery outcome of %s: %w", contract, err)
	}

	return patch, nil
}

// RecoveryOutcomeFor returns the outcome that answers request with result and
// message, completed at now.
func RecoveryOutcomeFor(
	request v1.RecoveryRequest,
	result v1.RecoveryResult,
	message string,
	now metav1.Time,
) v1.RecoveryOutcome {
	return v1.RecoveryOutcome{
		RequestedBy: request.RequestedBy,
		TargetTime:  request.TargetTime,
		CompletedAt: now,
		Result:      result,
		Message:     message,
	}
}
