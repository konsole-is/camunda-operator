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

package camundacluster

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"

	v1 "github.com/konsole-is/camunda-operator/api/v1"
)

// backupPreset returns a preset whose backup block sets every field, so the
// overlay tests can prove per-field inheritance and override.
func backupPreset() *v1.CamundaClusterPresetSpec {
	return &v1.CamundaClusterPresetSpec{Cluster: v1.CamundaClusterSpec{
		Backup: &v1.ClusterBackupSpec{
			PrimaryStorage: &v1.PrimaryStorageBackupSpec{
				Continuous:         new(true),
				Schedule:           "PT1H",
				CheckpointInterval: "PT15M",
				Retention: &v1.PrimaryStorageRetentionSpec{
					Window:          "P7D",
					CleanupSchedule: "PT1H",
				},
			},
			Dump: &v1.BackupDumpSpec{
				Resources: &corev1.ResourceRequirements{
					Requests: corev1.ResourceList{
						corev1.ResourceCPU:    resource.MustParse("1"),
						corev1.ResourceMemory: resource.MustParse("2Gi"),
					},
				},
				ExtraEnv: []corev1.EnvVar{{Name: "TZ", Value: "UTC"}, {Name: "KEEP", Value: "preset"}},
				ExtraEnvFrom: []corev1.EnvFromSource{{
					ConfigMapRef: &corev1.ConfigMapEnvSource{
						LocalObjectReference: corev1.LocalObjectReference{Name: "preset-cm"},
					},
				}},
				PodLabels:      map[string]string{"team": "preset", "shared": "preset"},
				PodAnnotations: map[string]string{"note": "preset", "shared": "preset"},
				Scheduling:     &v1.SchedulingSpec{Tolerations: []corev1.Toleration{{Key: "preset"}}},
				ScratchVolume:  &v1.ScratchVolumeSpec{SizeLimit: new(resource.MustParse("50Gi"))},
			},
		},
	}}
}

func TestMergePresetInheritsTheWholeBackupBlock(t *testing.T) {
	merged := MergePreset(v1.CamundaClusterSpec{}, backupPreset())

	require.NotNil(t, merged.Backup)
	require.NotNil(t, merged.Backup.PrimaryStorage)
	assert.Equal(t, "PT1H", merged.Backup.PrimaryStorage.Schedule)
	assert.Equal(t, "PT15M", merged.Backup.PrimaryStorage.CheckpointInterval)
	require.NotNil(t, merged.Backup.PrimaryStorage.Retention)
	assert.Equal(t, "P7D", merged.Backup.PrimaryStorage.Retention.Window)
	require.NotNil(t, merged.Backup.Dump)
	assert.Equal(t, "50Gi", merged.Backup.Dump.ScratchVolume.SizeLimit.String())
}

func TestMergePresetKeepsTheClusterBackupWhenThePresetHasNone(t *testing.T) {
	spec := v1.CamundaClusterSpec{Backup: &v1.ClusterBackupSpec{
		PrimaryStorage: &v1.PrimaryStorageBackupSpec{Schedule: "PT30M"},
	}}

	merged := MergePreset(spec, &v1.CamundaClusterPresetSpec{})

	require.NotNil(t, merged.Backup)
	require.NotNil(t, merged.Backup.PrimaryStorage)
	assert.Equal(t, "PT30M", merged.Backup.PrimaryStorage.Schedule)
}

func TestMergePresetOverridesPrimaryStoragePerField(t *testing.T) {
	spec := v1.CamundaClusterSpec{Backup: &v1.ClusterBackupSpec{
		PrimaryStorage: &v1.PrimaryStorageBackupSpec{
			Schedule:  "PT30M",
			Retention: &v1.PrimaryStorageRetentionSpec{Window: "P30D"},
		},
	}}

	merged := MergePreset(spec, backupPreset())

	ps := merged.Backup.PrimaryStorage
	require.NotNil(t, ps)
	assert.Equal(t, "PT30M", ps.Schedule, "the cluster wins on the field it sets")
	assert.Equal(t, "PT15M", ps.CheckpointInterval, "an unset field keeps the preset value")
	require.NotNil(t, ps.Retention)
	assert.Equal(t, "P30D", ps.Retention.Window)
	assert.Equal(t, "PT1H", ps.Retention.CleanupSchedule, "retention merges per field too")
}

// Continuous is a pointer so that a preset can enable it for a fleet and one
// cluster can still turn it off. A bare bool could not express the third
// state, and omitempty would drop the explicit false.
func TestMergePresetContinuousHasThreeStates(t *testing.T) {
	cases := []struct {
		name    string
		cluster *bool
		want    *bool
	}{
		{name: "unset inherits the preset", cluster: nil, want: new(true)},
		{name: "false overrides the preset", cluster: new(false), want: new(false)},
		{name: "true keeps the preset value", cluster: new(true), want: new(true)},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			spec := v1.CamundaClusterSpec{Backup: &v1.ClusterBackupSpec{
				PrimaryStorage: &v1.PrimaryStorageBackupSpec{Continuous: tc.cluster},
			}}

			merged := MergePreset(spec, backupPreset())

			require.NotNil(t, merged.Backup.PrimaryStorage.Continuous)
			assert.Equal(t, *tc.want, *merged.Backup.PrimaryStorage.Continuous)
		})
	}
}

func TestMergePresetMergesTheDumpBlockLikeAWorkload(t *testing.T) {
	spec := v1.CamundaClusterSpec{Backup: &v1.ClusterBackupSpec{
		Dump: &v1.BackupDumpSpec{
			Resources: &corev1.ResourceRequirements{
				Requests: corev1.ResourceList{corev1.ResourceMemory: resource.MustParse("8Gi")},
			},
			ExtraEnv: []corev1.EnvVar{{Name: "KEEP", Value: "cluster"}, {Name: "NEW", Value: "cluster"}},
			ExtraEnvFrom: []corev1.EnvFromSource{{
				ConfigMapRef: &corev1.ConfigMapEnvSource{
					LocalObjectReference: corev1.LocalObjectReference{Name: "cluster-cm"},
				},
			}},
			PodLabels:      map[string]string{"shared": "cluster", "own": "cluster"},
			PodAnnotations: map[string]string{"shared": "cluster"},
		},
	}}

	merged := MergePreset(spec, backupPreset())

	dump := merged.Backup.Dump
	require.NotNil(t, dump)
	assert.Equal(t, "8Gi", dump.Resources.Requests.Memory().String(), "requests merge by resource name")
	assert.Equal(t, "1", dump.Resources.Requests.Cpu().String(), "an unset request keeps the preset value")
	assert.Equal(
		t,
		[]corev1.EnvVar{{Name: "TZ", Value: "UTC"}, {Name: "KEEP", Value: "cluster"}, {Name: "NEW", Value: "cluster"}},
		dump.ExtraEnv,
		"extraEnv merges by name, the cluster wins, order of first appearance holds",
	)
	assert.Len(t, dump.ExtraEnvFrom, 2, "extraEnvFrom concatenates, preset first")
	assert.Equal(t, "preset-cm", dump.ExtraEnvFrom[0].ConfigMapRef.Name)
	assert.Equal(t, map[string]string{"team": "preset", "shared": "cluster", "own": "cluster"}, dump.PodLabels)
	assert.Equal(t, map[string]string{"note": "preset", "shared": "cluster"}, dump.PodAnnotations)
}

// Scheduling and the scratch volume replace as whole blocks: a half-merged
// affinity or a volume with a size from one source and a class from another
// is not a shape a user can predict.
func TestMergePresetReplacesTheDumpWholeBlocks(t *testing.T) {
	spec := v1.CamundaClusterSpec{Backup: &v1.ClusterBackupSpec{
		Dump: &v1.BackupDumpSpec{
			Scheduling:    &v1.SchedulingSpec{Tolerations: []corev1.Toleration{{Key: "cluster"}}},
			ScratchVolume: &v1.ScratchVolumeSpec{StorageClassName: new("fast")},
		},
	}}

	merged := MergePreset(spec, backupPreset())

	dump := merged.Backup.Dump
	require.Len(t, dump.Scheduling.Tolerations, 1)
	assert.Equal(t, "cluster", dump.Scheduling.Tolerations[0].Key)
	require.NotNil(t, dump.ScratchVolume)
	assert.Nil(t, dump.ScratchVolume.SizeLimit, "the preset size is gone with the block it belonged to")
	require.NotNil(t, dump.ScratchVolume.StorageClassName)
	assert.Equal(t, "fast", *dump.ScratchVolume.StorageClassName)
}

// The merge must not write through into the preset: a preset is shared by
// every cluster that references it, and the reconcile of one cluster would
// otherwise change what the next one reads.
func TestMergePresetLeavesTheBackupPresetUntouched(t *testing.T) {
	preset := backupPreset()
	spec := v1.CamundaClusterSpec{Backup: &v1.ClusterBackupSpec{
		PrimaryStorage: &v1.PrimaryStorageBackupSpec{Schedule: "PT5M"},
		Dump:           &v1.BackupDumpSpec{PodLabels: map[string]string{"team": "cluster"}},
	}}

	_ = MergePreset(spec, preset)

	assert.Equal(t, "PT1H", preset.Cluster.Backup.PrimaryStorage.Schedule)
	assert.Equal(t, "preset", preset.Cluster.Backup.Dump.PodLabels["team"])
}
