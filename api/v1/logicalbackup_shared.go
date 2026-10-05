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
// components that hold storage. The operator records them when a backup
// starts, so that a restore can create volumes of the correct size. A value
// that the operator cannot compute stays unset. An Elasticsearch backup can
// add a missing value later, while exporting runs. The RDBMS kind never sets
// Elasticsearch, because it does not back up Elasticsearch data.
type LogicalBackupStorageSizes struct {
	// Elasticsearch is the effective restore size of one Elasticsearch data
	// volume.
	// +optional
	Elasticsearch *resource.Quantity `json:"elasticsearch,omitempty"`
	// Zeebe is the effective restore size of one broker data volume.
	// +optional
	Zeebe *resource.Quantity `json:"zeebe,omitempty"`
}

// ClusterRef references a CamundaCluster by name, in the namespace of the
// referencing object. The reference cannot cross namespaces. For a backup,
// the operator reads the Secrets of the cluster, runs a Job in its
// namespace, and calls its management API. Thus the reference stays inside
// the RBAC limits of the namespace of the CR. A user who can create the CR
// in a namespace can back up the clusters of that namespace, and no others.
type ClusterRef struct {
	// Name is the name of the CamundaCluster, in the namespace of this
	// object.
	// +kubebuilder:validation:MinLength=1
	Name string `json:"name"`
}

// LogicalBackupPhase is the phase of a backup that runs one time. Completed
// and Failed are final. To try again, create a new CR.
// +kubebuilder:validation:Enum=Pending;Running;Completed;Failed
type LogicalBackupPhase string

// The phases of a logical backup.
const (
	// LogicalBackupPending means that the backup did not start its work. A
	// pre-check did not pass yet, or another backup of the same cluster runs.
	LogicalBackupPending LogicalBackupPhase = "Pending"
	// LogicalBackupRunning means that the backup runs.
	LogicalBackupRunning LogicalBackupPhase = "Running"
	// LogicalBackupCompleted means that the backup finished and a restore can
	// use it.
	LogicalBackupCompleted LogicalBackupPhase = "Completed"
	// LogicalBackupFailed means that the backup failed. The message of the
	// Ready condition names the failed step.
	LogicalBackupFailed LogicalBackupPhase = "Failed"
)

// The condition vocabulary that both logical backup kinds report. Reasons
// that only one kind reports are declared next to that kind.
const (
	// ReasonProgressing means that the backup runs, or that it waits, for
	// example for the cluster to run its current spec.
	ReasonProgressing = "Progressing"
	// ReasonCompleted means that the backup finished and a restore can use
	// it.
	ReasonCompleted = "Completed"
	// ReasonFailed means that the backup failed. The message names the
	// failed step.
	ReasonFailed = "Failed"
	// ReasonClusterSuspended means that the referenced cluster is suspended,
	// so its management API is not available.
	ReasonClusterSuspended = "ClusterSuspended"
	// ReasonBackupInProgress means that the cluster is claimed, and this
	// backup waits in Pending. The message names the holder, or a Lease to
	// delete.
	ReasonBackupInProgress = "BackupInProgress"
)
