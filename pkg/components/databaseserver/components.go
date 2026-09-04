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

// Package databaseserver renders the resources that a DatabaseServer CR
// publishes. It merges the preset into the spec, validates the merged spec,
// and assembles the ocf components: the CloudNativePG cluster, the continuous
// archive, the published DatabaseServerConfig, and the optional PodMonitor.
// Everything here is pure: spec in, resources out, no API calls. The
// controller in internal/controller drives it.
package databaseserver

import (
	"fmt"
	"maps"
	"strings"

	cnpgv1 "github.com/cloudnative-pg/api/pkg/api/v1"
	"github.com/sourcehawk/operator-component-framework/pkg/component"
	"github.com/sourcehawk/operator-component-framework/pkg/component/concepts"
	"github.com/sourcehawk/operator-component-framework/pkg/feature"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	v1 "github.com/konsole-is/camunda-operator/api/v1"
	"github.com/konsole-is/camunda-operator/pkg/images"
	"github.com/konsole-is/camunda-operator/pkg/labels"
	"github.com/konsole-is/camunda-operator/pkg/wrappers/cnpgcluster"
)

const (
	// BarmanPluginName is the CNPG-I plugin that writes the archive. The
	// in-tree spec.backup.barmanObjectStore is deprecated and goes away in
	// CloudNativePG 1.31, so the operator never uses it.
	BarmanPluginName = "barman-cloud.cloudnative-pg.io"
	// CNPGClusterNameLabel is the label that CloudNativePG puts on every
	// object of a cluster, with the cluster name as its value. The PodMonitor
	// selects the instance pods by it, and the controller finds the data
	// volumes by it.
	CNPGClusterNameLabel = "cnpg.io/cluster"
	// PostgresPort is the port every PostgreSQL instance listens on.
	PostgresPort = 5432
	// MetricsPortName is the port of an instance pod that serves the
	// Prometheus metrics of CloudNativePG.
	MetricsPortName = "metrics"
	// SuperuserUsernameKey and SuperuserPasswordKey are the keys of the
	// superuser Secret that CloudNativePG writes.
	SuperuserUsernameKey = "username"
	SuperuserPasswordKey = "password"
)

const (
	// componentLabel is the labels.ComponentKey value on everything that a
	// DatabaseServer manages.
	componentLabel = "postgres"
	// readWriteServiceSuffix appended to the cluster name yields the Service
	// that CloudNativePG points at the primary instance.
	readWriteServiceSuffix = "-rw"
	// superuserSecretSuffix appended to the cluster name yields the Secret
	// that CloudNativePG writes the superuser credentials to.
	superuserSecretSuffix = "-superuser"
	// defaultInstances is the instance count of a server whose merged spec
	// leaves it unset.
	defaultInstances = 1
)

// ClusterComponent builds the cluster component from the preset-merged spec:
// the CloudNativePG cluster that runs the PostgreSQL instances. spec.suspend
// suspends the component, which hibernates the cluster and keeps its volumes.
//
// The returned data cell holds the PostgreSQL system identifier that
// CloudNativePG reports, empty until it has detected one. It is set after the
// component reconciles, and the controller mirrors it to status.
//
// blocked is why a cluster of that name is not this component's to apply, from
// ClusterTakenMessage or RecoveryHoldsClusterMessage, and it is empty when the
// name is the component's. A guard blocks the apply while it is set, and a
// taken name is reported by the caller as v1.ReasonClusterTaken.
//
// A blocked component is never suspended, whatever spec.suspend says. ocf
// decides suspension before it reaches any guard, and a suspended component
// applies the hibernation of every resource it holds, so a server suspended on
// a name it does not own would stop the database behind that name. Staying on
// the running path is what puts the guard in front of the apply, and
// ClusterReady then names the holder rather than reporting Suspended, which is
// the truth: the server suspended nothing, because the cluster is not its.
//
// BlockOnForeignController covers the same ground on the suspension path, but
// only for a cluster another owner controls. The guard is what covers a
// cluster that nothing controls, and a rollback whose cluster is gone.
//
// archiveTaken is why the ObjectStore that describes the archive is not this
// server's, from ArchiveTakenMessage, and it is empty when that name is the
// server's. The cluster then carries no archive plugin: the entry names that
// ObjectStore, and a cluster that keeps it writes its write-ahead log into the
// bucket of whoever holds the name.
func ClusterComponent(
	server *v1.DatabaseServer,
	merged v1.DatabaseServerSpec,
	archive *ArchiveStorage,
	archiveTaken string,
	platform *v1.CamundaPlatformConfigSpec,
	blocked string,
) (*component.Component, *concepts.Data[string], error) {
	systemIdentifier := concepts.NewData[string]("postgres-system-identifier")

	builder := cnpgcluster.NewBuilder(cluster(server, merged, platform)).
		WithMutation(clusterMutations(server, merged, archive, archiveTaken)...).
		WithGuard(takenGuard[cnpgv1.Cluster](blocked))
	cnpgcluster.ExtractInto(builder, systemIdentifier, func(c cnpgv1.Cluster) (string, error) {
		return c.Status.SystemID, nil
	})

	postgres, err := builder.Build()
	if err != nil {
		return nil, nil, err
	}

	comp, err := component.NewComponentBuilder().
		WithName("cluster").
		WithConditionType(v1.ConditionClusterReady).
		WithResource(postgres, component.BlockOnForeignController()).
		Suspend(merged.Suspend && blocked == "").
		Build()
	if err != nil {
		return nil, nil, err
	}

	return comp, systemIdentifier, nil
}

// cluster renders the baseline CloudNativePG cluster: the instance count, the
// image, the data volume, the superuser access that the contract publishes,
// and the metadata that every object of the cluster inherits.
// clusterMutations layers the optional concerns on top.
func cluster(
	server *v1.DatabaseServer,
	merged v1.DatabaseServerSpec,
	platform *v1.CamundaPlatformConfigSpec,
) *cnpgv1.Cluster {
	return &cnpgv1.Cluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      ClusterName(server),
			Namespace: server.Namespace,
			Labels:    managedLabels(server),
		},
		Spec: cnpgv1.ClusterSpec{
			Instances: instances(merged),
			ImageName: images.Resolve(platform, images.Postgres, merged.Version),
			// CloudNativePG writes <cluster>-superuser and keeps the password
			// of the postgres user in step with it. Without this it blanks
			// that password, and the contract would name a Secret that grants
			// nothing.
			EnableSuperuserAccess: new(true),
			StorageConfiguration: cnpgv1.StorageConfiguration{
				Size:         merged.StorageSize.String(),
				StorageClass: merged.StorageClassName,
			},
			// CloudNativePG copies this onto every object it creates for the
			// cluster: the instance pods, the volume claims, and the
			// services. It is what lets an extension discover them.
			InheritedMetadata: &cnpgv1.EmbeddedObjectMetadata{
				Labels:      labels.Merge(merged.PodLabels, discoveryLabels(server)),
				Annotations: merged.PodAnnotations,
			},
		},
	}
}

// ClusterName returns the name of the CloudNativePG cluster that backs the
// server: the one status records, or the name of the server before the first
// reconcile. A recovery replaces the cluster with one under a new name, so
// every caller derives names from this rather than from server.Name.
func ClusterName(server *v1.DatabaseServer) string {
	if server.Status.Cluster != "" {
		return server.Status.Cluster
	}

	return server.Name
}

// managedLabels returns the labels of a resource that the operator applies for
// server.
func managedLabels(server *v1.DatabaseServer) map[string]string {
	return labels.Managed(labels.DatabaseServer(server.Name), componentLabel)
}

// instances returns the instance count of the merged spec, or the documented
// default when it names none.
func instances(merged v1.DatabaseServerSpec) int {
	if merged.Instances == nil {
		return defaultInstances
	}

	return int(*merged.Instances)
}

// discoveryLabels returns the labels of the pods, volumes, and services that
// CloudNativePG creates for the cluster. They carry the owner and component,
// so an extension can discover them, but not the manager label: CloudNativePG
// manages those objects.
func discoveryLabels(server *v1.DatabaseServer) map[string]string {
	return labels.Discovery(labels.DatabaseServer(server.Name), componentLabel)
}

// clusterMutations layers the optional concerns of the merged spec onto the
// baseline cluster. Each mutation is gated on its field.
func clusterMutations(
	server *v1.DatabaseServer,
	merged v1.DatabaseServerSpec,
	archive *ArchiveStorage,
	archiveTaken string,
) []cnpgcluster.Mutation {
	serviceAccountAnnotations := serviceAccountAnnotations(merged, archive)

	return []cnpgcluster.Mutation{
		{
			Name:    "InstanceResources",
			Feature: feature.NewBooleanGate(merged.Resources != nil),
			Mutate: func(m *cnpgcluster.Mutator) error {
				m.Edit(func(c *cnpgv1.Cluster) error {
					c.Spec.Resources = *merged.Resources
					return nil
				})
				return nil
			},
		},
		{
			Name:    "WriteAheadLogVolume",
			Feature: feature.NewBooleanGate(merged.WALStorageSize != nil),
			Mutate: func(m *cnpgcluster.Mutator) error {
				m.Edit(func(c *cnpgv1.Cluster) error {
					c.Spec.WalStorage = &cnpgv1.StorageConfiguration{
						Size:         merged.WALStorageSize.String(),
						StorageClass: merged.StorageClassName,
					}
					return nil
				})
				return nil
			},
		},
		{
			Name:    "SchedulingConstraints",
			Feature: feature.NewBooleanGate(merged.Scheduling != nil),
			Mutate: func(m *cnpgcluster.Mutator) error {
				m.Edit(func(c *cnpgv1.Cluster) error {
					c.Spec.Affinity.NodeAffinity = merged.Scheduling.NodeAffinity
					c.Spec.Affinity.AdditionalPodAffinity = merged.Scheduling.PodAffinity
					c.Spec.Affinity.Tolerations = merged.Scheduling.Tolerations
					return nil
				})
				return nil
			},
		},
		{
			Name:    "ServiceAccountAnnotations",
			Feature: feature.NewBooleanGate(len(serviceAccountAnnotations) > 0),
			Mutate: func(m *cnpgcluster.Mutator) error {
				m.Edit(func(c *cnpgv1.Cluster) error {
					c.Spec.ServiceAccountTemplate = &cnpgv1.ServiceAccountTemplate{
						Metadata: cnpgv1.Metadata{Annotations: serviceAccountAnnotations},
					}
					return nil
				})
				return nil
			},
		},
		{
			// The gate reads the spec, not the resolved bucket. The entry
			// needs neither, and a bucket that stops resolving under a
			// suspended server must not take the archive off the cluster.
			//
			// An ObjectStore of that name that another owner controls does
			// take it off. The entry carries that name, so a cluster that
			// keeps it archives into the bucket, and under the credentials,
			// of whoever holds the name.
			Name:    "ContinuousArchive",
			Feature: feature.NewBooleanGate(merged.Archive != nil && archiveTaken == ""),
			Mutate: func(m *cnpgcluster.Mutator) error {
				m.Edit(func(c *cnpgv1.Cluster) error {
					c.Spec.Plugins = []cnpgv1.PluginConfiguration{archivePlugin(server)}
					return nil
				})
				return nil
			},
		},
		{
			Name:    "AzureWorkloadIdentity",
			Feature: feature.NewBooleanGate(len(archive.podLabels()) > 0),
			Mutate: func(m *cnpgcluster.Mutator) error {
				m.Edit(func(c *cnpgv1.Cluster) error {
					c.Spec.InheritedMetadata.Labels = labels.Merge(
						c.Spec.InheritedMetadata.Labels, archive.podLabels(),
					)
					return nil
				})
				return nil
			},
		},
	}
}

// serviceAccountAnnotations returns the annotations of the ServiceAccount that
// CloudNativePG creates for the instance pods: the identity of the archive
// bucket with the annotations of spec.serviceAccount layered over it, so a
// value the user states wins over the derived one on the same key.
func serviceAccountAnnotations(
	merged v1.DatabaseServerSpec,
	archive *ArchiveStorage,
) map[string]string {
	var user map[string]string
	if merged.ServiceAccount != nil {
		user = merged.ServiceAccount.Annotations
	}

	derived := archive.identityAnnotations()
	if len(derived) == 0 && len(user) == 0 {
		return nil
	}

	// Not labels.Merge: that helper lets the operator win, because selectors
	// depend on operator labels. Here the user wins, so an identity stated on
	// the server overrides the one derived from the bucket.
	annotations := make(map[string]string, len(derived)+len(user))
	maps.Copy(annotations, derived)
	maps.Copy(annotations, user)

	return annotations
}

// takenGuard blocks a resource while the object of the name the server derives
// is not this component's to apply. reason says why, and it is empty when the
// name is the component's.
//
// component.BlockOnForeignController covers an object that another owner
// controls. This covers the ones it does not: an object that nothing controls
// at all, which the apply would otherwise rewrite and adopt, and a cluster
// that a running rollback has cut over to and that is no longer there.
func takenGuard[T any](reason string) func(T) (concepts.GuardStatusWithReason, error) {
	return func(T) (concepts.GuardStatusWithReason, error) {
		if reason == "" {
			return concepts.GuardStatusWithReason{Status: concepts.GuardStatusUnblocked}, nil
		}

		return concepts.GuardStatusWithReason{
			Status: concepts.GuardStatusBlocked,
			Reason: reason,
		}, nil
	}
}

// ReadWriteHost returns the in-cluster address of the primary instance, the
// host that the published contract carries.
func ReadWriteHost(server *v1.DatabaseServer) string {
	return ClusterName(server) + readWriteServiceSuffix + "." + server.Namespace + ".svc"
}

// ClusterFromReadWriteHost returns the CloudNativePG cluster that host names,
// or the empty string when host is not an address of this server. It is the
// inverse of ReadWriteHost: the published contract is the record of which
// cluster the server runs from, so a server whose status was lost reads it
// back from the contract it published.
func ClusterFromReadWriteHost(server *v1.DatabaseServer, host string) string {
	suffix := readWriteServiceSuffix + "." + server.Namespace + ".svc"
	name, found := strings.CutSuffix(host, suffix)
	if !found {
		return ""
	}

	return name
}

// ClusterTakenMessage says that a CloudNativePG cluster of the name the server
// derives is not the server's to write, and what to do about it. holder is the
// owner that controls that cluster, and it is nil when nothing controls it.
// The guard of the cluster and the ClusterReady condition of the server both
// read it, so the reason a user acts on is written once.
//
// A cluster with no controller is refused, the way a contract of the same
// shape is. The cluster holds a database, the apply rewrites its spec and
// makes it a child of this server, and deleting the server then deletes data
// the server never built.
func ClusterTakenMessage(name string, holder *metav1.OwnerReference) string {
	controller := "no owner controls it"
	if holder != nil {
		controller = fmt.Sprintf("%s %q controls it", holder.Kind, holder.Name)
	}

	return fmt.Sprintf(
		"CloudNativePG cluster %q already exists and %s. It holds a database this server did "+
			"not build, so the server writes nothing on it, publishes no contract, and takes "+
			"no base backup of it. The archive the server wrote before and its history stay. "+
			"Remove that cluster, or give this server a name of its own.",
		name, controller,
	)
}

// RecoveryHoldsClusterMessage says that the cluster the server cut over to is
// gone and that the rollback, not this component, decides what happens next.
//
// The component renders the same name once status.cluster carries it. An apply
// that creates the object again makes the removal invisible: the next look
// reads the cluster the component just built, waits for a probe of a database
// nothing recovered, and the rollback never ends. The object it builds carries
// no bootstrap either, so it is an empty database under the recovered name.
func RecoveryHoldsClusterMessage(name string) string {
	return fmt.Sprintf(
		"CloudNativePG cluster %q is gone and a rollback of this server was running on it. "+
			"The rollback is abandoned on the next look, and the server goes back to the "+
			"cluster it came from.",
		name,
	)
}

// superuserSecretRef returns a read-only reference to the Secret that
// CloudNativePG writes the superuser credentials to, for a component that must
// wait until that Secret exists.
func superuserSecretRef(server *v1.DatabaseServer) *corev1.Secret {
	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      SuperuserSecretName(server),
			Namespace: server.Namespace,
		},
	}
}

// SuperuserSecretName returns the Secret that CloudNativePG writes the
// superuser credentials to. The published contract points at it, so no
// password passes through the operator.
func SuperuserSecretName(server *v1.DatabaseServer) string {
	return ClusterName(server) + superuserSecretSuffix
}
