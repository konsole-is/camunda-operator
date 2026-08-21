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

import (
	"crypto/sha256"
	"encoding/hex"
	"maps"
	"strings"

	"k8s.io/apimachinery/pkg/util/validation"
)

const (
	// nameHashLength is the hex length of the hash that keeps a long name
	// unique after BoundedName truncates it.
	nameHashLength = 10

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
	// LogicalRestoreElasticsearchKey names the owning
	// LogicalRestoreElasticsearch.
	LogicalRestoreElasticsearchKey = "camunda.io/logical-restore-elasticsearch"
	// BackupScheduleKey names the owning BackupSchedule.
	BackupScheduleKey = "camunda.io/backup-schedule"
	// LogicalRestoreRDBMSKey names the owning LogicalRestoreRDBMS.
	LogicalRestoreRDBMSKey = "camunda.io/logical-restore-rdbms"
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
	// Name is the name of the owning custom resource, bounded to what a
	// label value admits. Build an Owner through the constructor of its
	// kind, which applies the bound.
	Name string
}

// OwnerName bounds the name of an owning custom resource to what a label
// value admits. A custom resource name is a DNS subdomain of up to 253
// characters, but a label value stops at 63. The owner label is also part of
// every selector, which the API server rejects whole when one value is too
// long, so the resources of an owner with a long name never apply.
//
// The Owner constructors below apply it, so a caller that builds its labels
// through Managed or Discovery never calls it. Call it directly in the two
// places that step outside those constructors:
//
//   - to add a second owner label to a map that Managed built, for example
//     the cluster label on a backup Job.
//   - to read an owner label back. The value is not the name of the custom
//     resource once the name passes 63 characters, so a reader that maps a
//     rendered resource to its owner compares OwnerName(candidate) against
//     the value. It never reads the value as a name.
func OwnerName(name string) string {
	return BoundedName(name, validation.LabelValueMaxLength)
}

// Cluster returns the Owner of resources that a CamundaCluster with the given
// name renders.
func Cluster(name string) Owner { return Owner{Key: ClusterKey, Name: OwnerName(name)} }

// ElasticsearchCluster returns the Owner of resources that an
// ElasticsearchCluster with the given name renders.
func ElasticsearchCluster(name string) Owner {
	return Owner{Key: ElasticsearchClusterKey, Name: OwnerName(name)}
}

// Database returns the Owner of resources that a Database with the given name
// renders.
func Database(name string) Owner { return Owner{Key: DatabaseKey, Name: OwnerName(name)} }

// LogicalBackupElasticsearch returns the Owner of resources that a
// LogicalBackupElasticsearch with the given name renders.
func LogicalBackupElasticsearch(name string) Owner {
	return Owner{Key: LogicalBackupElasticsearchKey, Name: OwnerName(name)}
}

// LogicalBackupRDBMS returns the Owner of resources that a LogicalBackupRDBMS
// with the given name renders.
func LogicalBackupRDBMS(name string) Owner {
	return Owner{Key: LogicalBackupRDBMSKey, Name: OwnerName(name)}
}

// LogicalRestoreElasticsearch returns the Owner of resources that a
// LogicalRestoreElasticsearch with the given name renders.
func LogicalRestoreElasticsearch(name string) Owner {
	return Owner{Key: LogicalRestoreElasticsearchKey, Name: OwnerName(name)}
}

// BackupSchedule returns the Owner of resources that a BackupSchedule with
// the given name renders.
func BackupSchedule(name string) Owner { return Owner{Key: BackupScheduleKey, Name: OwnerName(name)} }

// LogicalRestoreRDBMS returns the Owner of resources that a
// LogicalRestoreRDBMS with the given name renders.
func LogicalRestoreRDBMS(name string) Owner {
	return Owner{Key: LogicalRestoreRDBMSKey, Name: OwnerName(name)}
}

// PointInTimeRestore returns the Owner of resources that a PointInTimeRestore
// with the given name renders.
func PointInTimeRestore(name string) Owner {
	return Owner{Key: PointInTimeRestoreKey, Name: OwnerName(name)}
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

// BoundedName returns name when it fits limit, or its head followed by a hash
// of the whole name otherwise. The result is deterministic, so every render of
// one resource agrees, and two names that share the head differ in the hash.
//
// The name of a custom resource can be a full DNS subdomain, and a label value
// and a Job name are both bounded like a DNS label. Pass
// validation.LabelValueMaxLength or validation.DNS1123LabelMaxLength as limit,
// less the length of any suffix the caller appends.
func BoundedName(name string, limit int) string {
	if len(name) <= limit {
		return name
	}
	sum := sha256.Sum256([]byte(name))
	hash := hex.EncodeToString(sum[:])

	// A limit with no room for a head and a hash would index name
	// negatively. No caller passes one, and a panic here would take the whole
	// manager down rather than fail one reconcile.
	if limit < nameHashLength+2 {
		return hash[:max(limit, 0)]
	}

	return strings.TrimRight(name[:limit-1-nameHashLength], "-.") + "-" + hash[:nameHashLength]
}
