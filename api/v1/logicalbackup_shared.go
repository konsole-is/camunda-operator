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

package v1

import "k8s.io/apimachinery/pkg/api/resource"

// LogicalBackupStorageSizes are the effective restore sizes of the
// storage-bearing components, recorded when a backup starts so a restore can
// create right-sized volumes instead of guessing. Recording is best effort: a
// value that could not be computed stays unset. The RDBMS kind never sets
// Elasticsearch, whose data it does not back up.
type LogicalBackupStorageSizes struct {
	// Elasticsearch is the effective restore size of one Elasticsearch data
	// volume.
	// +optional
	Elasticsearch *resource.Quantity `json:"elasticsearch,omitempty"`
	// Zeebe is the effective restore size of one broker data volume.
	// +optional
	Zeebe *resource.Quantity `json:"zeebe,omitempty"`
}

// ClusterRef references a CamundaCluster by name and namespace.
type ClusterRef struct {
	// Name of the CamundaCluster.
	// +kubebuilder:validation:MinLength=1
	Name string `json:"name"`
	// Namespace of the CamundaCluster. Empty means the namespace of the
	// referencing object.
	// +optional
	Namespace string `json:"namespace,omitempty"`
}

// EffectiveClusterNamespace returns the namespace the reference points into:
// its own, or fallback — the namespace of the referencing object — when it
// names none. Every consumer of a ClusterRef resolves it through this one
// rule.
func (in ClusterRef) EffectiveClusterNamespace(fallback string) string {
	if in.Namespace != "" {
		return in.Namespace
	}

	return fallback
}

// LogicalBackupPhase tracks a one-shot backup operation. Completed and Failed
// are terminal; a retry is a new CR.
// +kubebuilder:validation:Enum=Pending;Running;Completed;Failed
type LogicalBackupPhase string

// The phases of a logical backup.
const (
	// LogicalBackupPending means the backup has not started real work: the
	// pre-checks have not all passed yet, or another backup of the same
	// cluster runs.
	LogicalBackupPending LogicalBackupPhase = "Pending"
	// LogicalBackupRunning means the backup procedure is in progress.
	LogicalBackupRunning LogicalBackupPhase = "Running"
	// LogicalBackupCompleted means the backup finished and is restorable.
	LogicalBackupCompleted LogicalBackupPhase = "Completed"
	// LogicalBackupFailed means the backup failed. The Ready condition
	// message names the failing step.
	LogicalBackupFailed LogicalBackupPhase = "Failed"
)

// The condition vocabulary that both logical backup kinds report. Reasons
// that only one kind reports are declared next to that kind.
const (
	// ReasonProgressing means the backup runs.
	ReasonProgressing = "Progressing"
	// ReasonCompleted means the backup finished and is restorable.
	ReasonCompleted = "Completed"
	// ReasonFailed means the backup failed. The message names the failing
	// step.
	ReasonFailed = "Failed"
	// ReasonClusterSuspended means the referenced cluster is suspended, so
	// its management API is unreachable.
	ReasonClusterSuspended = "ClusterSuspended"
	// ReasonBackupInProgress means another backup of the same cluster is not
	// terminal, so this one waits in Pending.
	ReasonBackupInProgress = "BackupInProgress"
	// ReasonStorageTypeMismatch means the backup kind does not match the
	// storage type of the cluster's SecondaryStorageConfig.
	ReasonStorageTypeMismatch = "StorageTypeMismatch"
)
