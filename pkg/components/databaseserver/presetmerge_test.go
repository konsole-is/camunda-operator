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
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"

	v1 "github.com/konsole-is/camunda-operator/api/v1"
)

// presetBaseline is the preset that the merge cases resolve against.
func presetBaseline() *v1.DatabaseServerPresetSpec {
	return &v1.DatabaseServerPresetSpec{
		Server: v1.DatabaseServerSpec{
			Version:          "17",
			Instances:        new(int32(3)),
			StorageSize:      new(resource.MustParse("64Gi")),
			StorageClassName: new("preset-class"),
			WALStorageSize:   new(resource.MustParse("8Gi")),
			PodLabels:        map[string]string{"from": "preset"},
			PodAnnotations:   map[string]string{"from": "preset"},
			Scheduling: &v1.SchedulingSpec{
				Tolerations: []corev1.Toleration{{Key: "preset"}},
			},
			Monitoring: &v1.DatabaseServerMonitoringSpec{
				PodMonitor: &v1.PodMonitorSpec{Enabled: true},
			},
			ServiceAccount: &v1.DatabaseServerServiceAccountSpec{
				Annotations: map[string]string{"from": "preset"},
			},
			Resources: &corev1.ResourceRequirements{
				Requests: corev1.ResourceList{corev1.ResourceCPU: resource.MustParse("1")},
			},
			Archive: &v1.DatabaseServerArchiveSpec{
				ObjectStorageRef:    "preset-bucket",
				RetentionPeriodDays: 7,
			},
		},
	}
}

func TestMergePresetInheritsEveryUnsetField(t *testing.T) {
	t.Parallel()

	preset := presetBaseline()
	merged := MergePreset(v1.DatabaseServerSpec{
		PresetRef:            "standard",
		DatabaseServerConfig: "my-database-server",
	}, preset)

	assert.Equal(t, preset.Server.Version, merged.Version)
	assert.Equal(t, preset.Server.Instances, merged.Instances)
	assert.Equal(t, preset.Server.StorageSize, merged.StorageSize)
	assert.Equal(t, preset.Server.StorageClassName, merged.StorageClassName)
	assert.Equal(t, preset.Server.WALStorageSize, merged.WALStorageSize)
	assert.Equal(t, preset.Server.PodLabels, merged.PodLabels)
	assert.Equal(t, preset.Server.PodAnnotations, merged.PodAnnotations)
	assert.Equal(t, preset.Server.Scheduling, merged.Scheduling)
	assert.Equal(t, preset.Server.Monitoring, merged.Monitoring)
	assert.Equal(t, preset.Server.ServiceAccount, merged.ServiceAccount)
	assert.Equal(t, preset.Server.Resources, merged.Resources)
	assert.Equal(t, preset.Server.Archive, merged.Archive)

	assert.Equal(t, "standard", merged.PresetRef)
	assert.Equal(t, "my-database-server", merged.DatabaseServerConfig)
}

func TestMergePresetOverridesEverySetField(t *testing.T) {
	t.Parallel()

	spec := v1.DatabaseServerSpec{
		PresetRef:            "standard",
		DatabaseServerConfig: "my-database-server",
		Suspend:              true,
		Version:              "18",
		Instances:            new(int32(1)),
		StorageSize:          new(resource.MustParse("128Gi")),
		StorageClassName:     new("spec-class"),
		WALStorageSize:       new(resource.MustParse("16Gi")),
		PodLabels:            map[string]string{"from": "spec"},
		PodAnnotations:       map[string]string{"from": "spec"},
		Scheduling:           &v1.SchedulingSpec{Tolerations: []corev1.Toleration{{Key: "spec"}}},
		Monitoring: &v1.DatabaseServerMonitoringSpec{
			PodMonitor: &v1.PodMonitorSpec{Enabled: false},
		},
		ServiceAccount: &v1.DatabaseServerServiceAccountSpec{
			Annotations: map[string]string{"from": "spec"},
		},
		Resources: &corev1.ResourceRequirements{
			Requests: corev1.ResourceList{corev1.ResourceCPU: resource.MustParse("2")},
		},
		Archive: &v1.DatabaseServerArchiveSpec{
			ObjectStorageRef:    "spec-bucket",
			RetentionPeriodDays: 30,
		},
	}

	merged := MergePreset(spec, presetBaseline())

	assert.Equal(t, spec, merged)
}

// A preset is a shared baseline. Merging must never hand a caller a value that
// aliases it, or one server's edit would reach every other server.
func TestMergePresetCopiesThePresetBaseline(t *testing.T) {
	t.Parallel()

	preset := presetBaseline()
	merged := MergePreset(v1.DatabaseServerSpec{DatabaseServerConfig: "c"}, preset)

	merged.PodLabels["from"] = "merged"
	merged.Archive.RetentionPeriodDays = 99

	assert.Equal(t, "preset", preset.Server.PodLabels["from"])
	assert.Equal(t, int32(7), preset.Server.Archive.RetentionPeriodDays)
}

func TestMergePresetWithoutAPresetReturnsTheSpec(t *testing.T) {
	t.Parallel()

	spec := v1.DatabaseServerSpec{Version: "17", DatabaseServerConfig: "c"}

	assert.Equal(t, spec, MergePreset(spec, nil))
}

func TestValidateMerged(t *testing.T) {
	t.Parallel()

	complete := v1.DatabaseServerSpec{
		Version:              "17",
		StorageSize:          new(resource.MustParse("20Gi")),
		DatabaseServerConfig: "my-database-server",
	}

	tests := []struct {
		name    string
		mutate  func(*v1.DatabaseServerSpec)
		wantErr string
	}{
		{name: "complete spec", mutate: func(*v1.DatabaseServerSpec) {}},
		{
			name:    "no version",
			mutate:  func(s *v1.DatabaseServerSpec) { s.Version = "" },
			wantErr: "missing required fields after preset merge: version",
		},
		{
			name:    "no storage size",
			mutate:  func(s *v1.DatabaseServerSpec) { s.StorageSize = nil },
			wantErr: "missing required fields after preset merge: storageSize",
		},
		{
			name:    "no contract name",
			mutate:  func(s *v1.DatabaseServerSpec) { s.DatabaseServerConfig = "" },
			wantErr: "missing required fields after preset merge: databaseServerConfig",
		},
		{
			name: "every field missing is named",
			mutate: func(s *v1.DatabaseServerSpec) {
				s.Version = ""
				s.StorageSize = nil
				s.DatabaseServerConfig = ""
			},
			wantErr: "version, storageSize, databaseServerConfig",
		},
		{
			name:    "a major below the floor",
			mutate:  func(s *v1.DatabaseServerSpec) { s.Version = "13" },
			wantErr: `version "13" is not supported`,
		},
		{name: "the oldest supported major", mutate: func(s *v1.DatabaseServerSpec) { s.Version = "14" }},
		{name: "a major newer than this operator knows", mutate: func(s *v1.DatabaseServerSpec) { s.Version = "19" }},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			spec := *complete.DeepCopy()
			tt.mutate(&spec)

			err := ValidateMerged(spec)
			if tt.wantErr == "" {
				require.NoError(t, err)
				return
			}
			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.wantErr)
		})
	}
}
