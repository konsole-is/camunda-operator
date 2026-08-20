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

// Package labels is the one place that knows which labels the operator puts
// on the resources it renders. Every component builds its labels here, so
// every resource carries the same keys and an extension can discover a
// workload by them.
//
// Each owning kind has its own key: camunda.io/cluster names a
// CamundaCluster, camunda.io/elasticsearch-cluster an ElasticsearchCluster,
// camunda.io/database a Database. One key per kind keeps two owners of
// different kinds with the same name apart, and lets an extension select the
// resources of one kind without knowing the component vocabulary of the other.
package labels

import "maps"

const (
	// ClusterKey names the owning CamundaCluster.
	ClusterKey = "camunda.io/cluster"
	// ElasticsearchClusterKey names the owning ElasticsearchCluster.
	ElasticsearchClusterKey = "camunda.io/elasticsearch-cluster"
	// DatabaseKey names the owning Database.
	DatabaseKey = "camunda.io/database"
	// LogicalBackupElasticsearchKey names the owning
	// LogicalBackupElasticsearch.
	LogicalBackupElasticsearchKey = "camunda.io/logical-backup-elasticsearch"
	// LogicalBackupRDBMSKey names the owning LogicalBackupRDBMS.
	LogicalBackupRDBMSKey = "camunda.io/logical-backup-rdbms"
	// BackupScheduleKey names the owning BackupSchedule.
	BackupScheduleKey = "camunda.io/backup-schedule"
	// PointInTimeRestoreKey names the owning PointInTimeRestore.
	PointInTimeRestoreKey = "camunda.io/point-in-time-restore"
	// ComponentKey names the role of a resource inside its owner, for example
	// "elasticsearch" or "elasticsearch-exporter".
	ComponentKey = "camunda.io/component"
	// ManagedByKey is the Kubernetes recommended label that names the tool
	// that manages a resource.
	ManagedByKey = "app.kubernetes.io/managed-by"
	// ManagedBy is the ManagedByKey value of every resource the operator
	// applies.
	ManagedBy = "camunda-operator"
)

// Owner identifies the custom resource that a rendered resource belongs to:
// the label key of its kind and its name.
type Owner struct {
	// Key is the label key of the owning kind, one of ClusterKey,
	// ElasticsearchClusterKey, or DatabaseKey.
	Key string
	// Name is the name of the owning custom resource.
	Name string
}

// Cluster returns the Owner of resources that a CamundaCluster with the given
// name renders.
func Cluster(name string) Owner { return Owner{Key: ClusterKey, Name: name} }

// ElasticsearchCluster returns the Owner of resources that an
// ElasticsearchCluster with the given name renders.
func ElasticsearchCluster(name string) Owner { return Owner{Key: ElasticsearchClusterKey, Name: name} }

// Database returns the Owner of resources that a Database with the given name
// renders.
func Database(name string) Owner { return Owner{Key: DatabaseKey, Name: name} }

// LogicalBackupElasticsearch returns the Owner of resources that a
// LogicalBackupElasticsearch with the given name renders.
func LogicalBackupElasticsearch(name string) Owner {
	return Owner{Key: LogicalBackupElasticsearchKey, Name: name}
}

// LogicalBackupRDBMS returns the Owner of resources that a LogicalBackupRDBMS
// with the given name renders.
func LogicalBackupRDBMS(name string) Owner { return Owner{Key: LogicalBackupRDBMSKey, Name: name} }

// BackupSchedule returns the Owner of resources that a BackupSchedule with
// the given name renders.
func BackupSchedule(name string) Owner { return Owner{Key: BackupScheduleKey, Name: name} }

// PointInTimeRestore returns the Owner of resources that a PointInTimeRestore
// with the given name renders.
func PointInTimeRestore(name string) Owner {
	return Owner{Key: PointInTimeRestoreKey, Name: name}
}

// Managed returns the labels of a resource that the operator applies: the
// owner, the component, and the operator as manager.
func Managed(owner Owner, component string) map[string]string {
	return map[string]string{
		owner.Key:    owner.Name,
		ComponentKey: component,
		ManagedByKey: ManagedBy,
	}
}

// Discovery returns the labels that let an extension find a workload by owner
// and component, without the manager label. Use it on pod templates and
// volume claims that another operator creates and manages, for example the
// Elasticsearch pods that ECK runs from the operator's template, and on
// selectors, which are immutable.
func Discovery(owner Owner, component string) map[string]string {
	return map[string]string{
		owner.Key:    owner.Name,
		ComponentKey: component,
	}
}

// Merge returns the user labels with the operator labels applied over them.
// A user label never overrides an operator label, because extensions and
// selectors depend on the operator labels. The result is a new map.
func Merge(user, operator map[string]string) map[string]string {
	merged := make(map[string]string, len(user)+len(operator))
	maps.Copy(merged, user)
	maps.Copy(merged, operator)
	return merged
}
