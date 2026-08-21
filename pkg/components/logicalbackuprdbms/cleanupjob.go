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

package logicalbackuprdbms

import (
	"encoding/json"
	"fmt"

	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/validation"

	v1 "github.com/konsole-is/camunda-operator/api/v1"
	"github.com/konsole-is/camunda-operator/pkg/labels"
)

// cleanupNameSuffix ends every cleanup Job name.
const cleanupNameSuffix = "-cleanup"

// CleanupJobInput is everything the cleanup Job of one deleted backup renders
// from. The finalizer resolves it. The builder only shapes it.
type CleanupJobInput struct {
	// Backup is the LogicalBackupRDBMS whose dump object is removed.
	Backup *v1.LogicalBackupRDBMS
	// ClusterName identifies the backed-up cluster. The Job runs in the
	// namespace of the cluster, which is the namespace of the backup.
	ClusterName string
	// Dump shapes the pod the way it shaped the pod of the dump Job, so the
	// cleanup runs on the same pod identity surface: labels, annotations,
	// scheduling. Nil means defaults.
	Dump *v1.DumpPodSpec
	// BackupOwnsDump reports that Dump is the backup's own spec.dump. Then
	// no environment of it reaches the delete container, for the reason
	// that JobInput.BackupOwnsDump gives.
	BackupOwnsDump bool
	// Bucket is the backup bucket contract. It uses workload identity. The
	// manager cleans a credentials-mode bucket directly.
	Bucket *v1.ObjectStorageConfig
	// ServiceAccountName is the ServiceAccount that the cluster publishes.
	// The identity of the storage contract is bound to it, and the delete
	// runs as it, like the upload did. Empty means the default account of
	// the namespace.
	ServiceAccountName string
	// ObjectKey is the exact key of the dump to remove.
	ObjectKey string
	// CLIImage is the camunda-operator-cli image. Its delete subcommand
	// runs.
	CLIImage string
}

// CleanupJobName returns the name of the cleanup Job of one backup. It
// derives from the backup name alone, bounded like the name of the dump Job.
// A finalizer that re-enters therefore adopts the Job that it already
// created.
func CleanupJobName(backup *v1.LogicalBackupRDBMS) string {
	return labels.BoundedName(backup.Name, validation.DNS1123LabelMaxLength-len(cleanupNameSuffix)) +
		cleanupNameSuffix
}

// BuildCleanupJob renders the Job that removes the dump object of a deleted
// backup where the identity lives. The Job is one camunda-operator-cli
// delete container under the cluster ServiceAccount. It has the same pod
// identity surface as the dump Job: the workload-identity pod labels of the
// bucket and the pod settings of the resolved dump block. The delete is
// idempotent, so the Job can retry.
func BuildCleanupJob(in CleanupJobInput) (*batchv1.Job, error) {
	if in.CLIImage == "" {
		return nil, fmt.Errorf(
			"building the cleanup Job of %q: the camunda-operator-cli image is empty", in.Backup.Name,
		)
	}

	spec, err := json.Marshal(in.Bucket.Spec)
	if err != nil {
		return nil, fmt.Errorf("encoding the bucket spec of %q: %w", in.Bucket.Name, err)
	}

	dump := in.Dump
	if dump == nil {
		dump = &v1.DumpPodSpec{}
	}

	managed := labels.Managed(
		labels.LogicalBackupRDBMS(in.Backup.Name),
		"cleanup",
	)
	managed[labels.ClusterKey] = labels.OwnerName(in.ClusterName)
	managed[BackupUIDLabel] = string(in.Backup.UID)

	podManaged := labels.Merge(in.Bucket.WorkloadIdentityPodLabels(), managed)
	podLabels := labels.Merge(dump.PodLabels, podManaged)

	security := containerSecurity(operatorUID)
	security.ReadOnlyRootFilesystem = new(true)

	env := []corev1.EnvVar{
		{Name: EnvUploadKey, Value: in.ObjectKey},
		{Name: EnvUploadStorageName, Value: in.Bucket.Name},
		{Name: EnvUploadStorageSpec, Value: string(spec)},
	}
	container := corev1.Container{
		Name:            "delete",
		Image:           in.CLIImage,
		Args:            []string{"delete"},
		Env:             env,
		SecurityContext: security,
	}
	if !in.BackupOwnsDump {
		container.Env = mergeEnv(env, dump.ExtraEnv)
		container.EnvFrom = dump.ExtraEnvFrom
	}

	template := corev1.PodTemplateSpec{
		ObjectMeta: metav1.ObjectMeta{
			Labels:      podLabels,
			Annotations: dump.PodAnnotations,
		},
		Spec: corev1.PodSpec{
			RestartPolicy:      corev1.RestartPolicyNever,
			ServiceAccountName: in.ServiceAccountName,
			SecurityContext: &corev1.PodSecurityContext{
				RunAsNonRoot:   new(true),
				SeccompProfile: &corev1.SeccompProfile{Type: corev1.SeccompProfileTypeRuntimeDefault},
			},
			Containers: []corev1.Container{container},
		},
	}
	if dump.Scheduling != nil {
		template.Spec.Tolerations = dump.Scheduling.Tolerations
		if dump.Scheduling.NodeAffinity != nil || dump.Scheduling.PodAffinity != nil {
			template.Spec.Affinity = &corev1.Affinity{
				NodeAffinity: dump.Scheduling.NodeAffinity,
				PodAffinity:  dump.Scheduling.PodAffinity,
			}
		}
	}

	return &batchv1.Job{
		TypeMeta: metav1.TypeMeta{APIVersion: "batch/v1", Kind: "Job"},
		ObjectMeta: metav1.ObjectMeta{
			Name:      CleanupJobName(in.Backup),
			Namespace: in.Backup.Namespace,
			Labels:    managed,
		},
		Spec: batchv1.JobSpec{
			BackoffLimit:          new(defaultBackoffLimit),
			ActiveDeadlineSeconds: activeDeadline(dump),
			Template:              template,
		},
	}, nil
}
