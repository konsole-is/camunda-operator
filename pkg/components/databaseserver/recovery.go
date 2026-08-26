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
	"sigs.k8s.io/controller-runtime/pkg/client"

	v1 "github.com/konsole-is/camunda-operator/api/v1"
	"github.com/konsole-is/camunda-operator/pkg/labels"
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

// cnpgClusterNameMaxLength is the longest name that CloudNativePG accepts for
// a cluster. Admission on the DatabaseServer bounds the name of the server to
// this length less the four characters of "-r99", so the name of a server the
// API server accepted reaches a recovery cluster whole while the recovery
// index stays below 100.
const cnpgClusterNameMaxLength = 50

// ErrNoArchiveHolds reports that no archive of the server holds the requested
// point. It is the one refusal that means the request was never possible,
// rather than one that failed.
var ErrNoArchiveHolds = errors.New("no archive of the server holds the requested point")

// ErrArchiveInAnotherLocation reports that the archive that holds the
// requested point is somewhere the spec no longer archives to, or somewhere
// the operator cannot place at all. The recovered cluster reads one
// ObjectStore, and that ObjectStore describes where the server archives to
// now.
var ErrArchiveInAnotherLocation = errors.New(
	"the archive that holds the requested point is in another location",
)

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

	name := recoveryName(server.Name, archives)
	if name == ClusterName(server) {
		return recoveryName(server.Name, archives+1)
	}

	return name
}

// recoveryName renders the name of the n-th recovery of a server, bounded so
// that CloudNativePG accepts the name.
//
// The suffix of the recovery comes off the bound, and the name of the server
// is shortened to what is left, the same way every other derived name in this
// operator is shortened: the head of the name, and a hash of the whole of it.
// Admission leaves room for a two-digit n, so a server named at the bound is
// shortened only once n reaches 100. The Services that CloudNativePG
// derives need no budget of their own: this bound plus the longest of their
// suffixes is well inside a DNS label of 63 characters.
func recoveryName(server string, n int) string {
	suffix := recoveryNameSeparator + strconv.Itoa(n)

	return labels.BoundedName(server, cnpgClusterNameMaxLength-len(suffix)) + suffix
}

// SelectArchive returns the archive of history that a recovery to target
// starts from, or an error that says why no archive answers. An interval
// holds its start and every point up to, but not including, its end, so a
// target on the boundary of two archives belongs to the later one. The
// archive the server writes now has no end and holds every point after its
// start.
//
// location is where the server archives to now, and bucket is the
// ObjectStorageConfig that names it. A record of another location holds its
// interval, and a recovery still cannot read it: the recovered cluster is
// given one ObjectStore, and that one describes location. The comparison is on
// the location rather than on the name, because an ObjectStorageConfig can be
// edited in place, and removed and created again, without its name changing.
// A record that names no location was written before the field existed and is
// taken as the current one.
//
// A record written before the location was recorded carries only the contract
// that named it. It is the archive the server writes now when that contract is
// the one it writes through, and it is unplaceable when it is not: the
// contract has moved since, and nothing says where its objects went.
//
// The error wraps ErrNoArchiveHolds when no interval holds target, and
// ErrArchiveInAnotherLocation when one holds it somewhere else, or somewhere
// that cannot be placed. Both are refusals of the request rather than failures
// of the recovery.
func SelectArchive(
	history []v1.ArchiveRecord,
	target time.Time,
	location, bucket string,
) (v1.ArchiveRecord, error) {
	// Newest first: the intervals never overlap, and a record that a later
	// recovery superseded is the answer only when nothing newer holds the
	// point.
	for i := len(history) - 1; i >= 0; i-- {
		record := history[i]
		if target.Before(record.From.Time) {
			continue
		}
		if record.To == nil || target.Before(record.To.Time) {
			if record.ObjectStorageRef == "" {
				record.ObjectStorageRef = bucket
			}
			if record.Location == "" && record.ObjectStorageRef == bucket {
				record.Location = location
			}
			if record.Location == "" {
				return v1.ArchiveRecord{}, fmt.Errorf(
					"%w. It was written through ObjectStorageConfig %q, which the server no "+
						"longer archives through, and its location was not recorded. Nothing "+
						"says where those objects are",
					ErrArchiveInAnotherLocation, record.ObjectStorageRef,
				)
			}
			if record.Location != location {
				return v1.ArchiveRecord{}, fmt.Errorf(
					"%w. It is at %q (ObjectStorageConfig %q), and the server archives to %q "+
						"(ObjectStorageConfig %q) now. Reading the archive of an earlier "+
						"location is not supported yet",
					ErrArchiveInAnotherLocation,
					record.Location, record.ObjectStorageRef, location, bucket,
				)
			}

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
	// A render carries no kind. The components go through the framework,
	// which fills the kind in on the way out; the controller writes this one
	// itself, and every reader of it reads a whole object.
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
							// The one ObjectStore of the server, which
							// describes the bucket spec.archive names now.
							// SelectArchive refuses a source of any other
							// bucket, so this is the bucket source is in.
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
// contract. It carries spec.pitr.lastRecovery, no other spec field, and the UID of the contract it was read from, so the apply
// states the answer without touching a field that the contract component or
// the consumer owns.
//
// It also carries the uid of contract. A name is taken again by whatever is
// created under it next, and the API server refuses an apply whose uid is not
// the uid of the object that holds the name. The answer therefore reaches the
// object it was read from, or no object at all.
func RecoveryOutcomePatch(
	contract *v1.DatabaseServerConfig,
	outcome v1.RecoveryOutcome,
) (*unstructured.Unstructured, error) {
	key := client.ObjectKeyFromObject(contract)

	fields, err := runtime.DefaultUnstructuredConverter.ToUnstructured(&outcome)
	if err != nil {
		return nil, fmt.Errorf("rendering the recovery outcome of %s: %w", key, err)
	}

	patch := &unstructured.Unstructured{}
	patch.SetGroupVersionKind(v1.GroupVersion.WithKind("DatabaseServerConfig"))
	patch.SetNamespace(key.Namespace)
	patch.SetName(key.Name)
	patch.SetUID(contract.UID)
	if err := unstructured.SetNestedMap(patch.Object, fields, "spec", "pitr", "lastRecovery"); err != nil {
		return nil, fmt.Errorf("rendering the recovery outcome of %s: %w", key, err)
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
		RequestID:   request.RequestID,
		RequestedBy: request.RequestedBy,
		TargetTime:  request.TargetTime,
		CompletedAt: now,
		Result:      result,
		Message:     message,
	}
}
