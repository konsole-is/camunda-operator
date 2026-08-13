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

package controller

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"

	v1 "github.com/konsole-is/camunda-operator/api/v1"
)

// versionBelowFloor is an Elasticsearch version below the Camunda 8.9 floor.
const versionBelowFloor = "8.18.0"

// fullPresetSpec returns a preset baseline with every inheritable field set,
// so overlay tests can prove per-field inheritance and override.
func fullPresetSpec() *v1.ElasticsearchClusterPresetSpec {
	return &v1.ElasticsearchClusterPresetSpec{
		Cluster: v1.ElasticsearchClusterSpec{
			Version:  "9.2.4",
			Replicas: new(int32(3)),
			Resources: &corev1.ResourceRequirements{
				Requests: corev1.ResourceList{
					corev1.ResourceCPU:    resource.MustParse("1"),
					corev1.ResourceMemory: resource.MustParse("2Gi"),
				},
			},
			StorageSize:      new(resource.MustParse("64Gi")),
			StorageClassName: new("standard"),
			ServiceAccount: &v1.ServiceAccountSpec{
				Annotations: map[string]string{"preset/annotation": "a"},
			},
			ExtraEnv:       []corev1.EnvVar{{Name: "PRESET_ENV", Value: "1"}},
			ExtraEnvFrom:   []corev1.EnvFromSource{{Prefix: "PRESET_"}},
			PodLabels:      map[string]string{"preset": "label"},
			PodAnnotations: map[string]string{"preset": "annotation"},
			Scheduling: &v1.SchedulingSpec{
				NodeAffinity: &corev1.NodeAffinity{
					RequiredDuringSchedulingIgnoredDuringExecution: &corev1.NodeSelector{
						NodeSelectorTerms: []corev1.NodeSelectorTerm{{
							MatchExpressions: []corev1.NodeSelectorRequirement{{
								Key: "preset", Operator: corev1.NodeSelectorOpExists,
							}},
						}},
					},
				},
				Tolerations: []corev1.Toleration{{
					Key: "preset", Operator: corev1.TolerationOpExists,
				}},
			},
		},
	}
}

func TestMergePresetNilPresetPassesSpecThrough(t *testing.T) {
	t.Parallel()

	spec := realisticElasticsearchCluster().Spec

	merged := mergePreset(spec, nil)

	assert.Equal(t, spec, merged)
}

func TestMergePresetInheritsUnsetFields(t *testing.T) {
	t.Parallel()

	spec := v1.ElasticsearchClusterSpec{
		PresetRef:              "standard",
		SecondaryStorageConfig: "my-storage-config",
	}
	preset := fullPresetSpec()

	merged := mergePreset(spec, preset)

	assert.Equal(t, preset.Cluster.Version, merged.Version)
	assert.Equal(t, preset.Cluster.Replicas, merged.Replicas)
	assert.Equal(t, preset.Cluster.Resources, merged.Resources)
	assert.Equal(t, preset.Cluster.StorageSize, merged.StorageSize)
	assert.Equal(t, preset.Cluster.StorageClassName, merged.StorageClassName)
	assert.Equal(t, preset.Cluster.ServiceAccount, merged.ServiceAccount)
	assert.Equal(t, preset.Cluster.ExtraEnv, merged.ExtraEnv)
	assert.Equal(t, preset.Cluster.ExtraEnvFrom, merged.ExtraEnvFrom)
	assert.Equal(t, preset.Cluster.PodLabels, merged.PodLabels)
	assert.Equal(t, preset.Cluster.PodAnnotations, merged.PodAnnotations)
	assert.Equal(t, preset.Cluster.Scheduling, merged.Scheduling)
}

func TestMergePresetOverridesInlineFieldsWholesale(t *testing.T) {
	t.Parallel()

	spec := v1.ElasticsearchClusterSpec{
		PresetRef: "standard",
		Version:   "9.2.5",
		Replicas:  new(int32(5)),
		Resources: &corev1.ResourceRequirements{
			Limits: corev1.ResourceList{corev1.ResourceMemory: resource.MustParse("8Gi")},
		},
		StorageSize:      new(resource.MustParse("128Gi")),
		StorageClassName: new("ssd"),
		ServiceAccount: &v1.ServiceAccountSpec{
			Annotations: map[string]string{"inline/annotation": "b"},
		},
		ExtraEnv:               []corev1.EnvVar{{Name: "INLINE_ENV", Value: "2"}},
		ExtraEnvFrom:           []corev1.EnvFromSource{{Prefix: "INLINE_"}},
		PodLabels:              map[string]string{"inline": "label"},
		PodAnnotations:         map[string]string{"inline": "annotation"},
		SecondaryStorageConfig: "my-storage-config",
	}
	preset := fullPresetSpec()

	merged := mergePreset(spec, preset)

	assert.Equal(t, spec.Version, merged.Version)
	assert.Equal(t, spec.Replicas, merged.Replicas)
	assert.Equal(t, spec.Resources, merged.Resources)
	assert.Equal(t, spec.StorageSize, merged.StorageSize)
	assert.Equal(t, spec.StorageClassName, merged.StorageClassName)
	assert.Equal(t, spec.ServiceAccount, merged.ServiceAccount)
	assert.Equal(t, spec.ExtraEnv, merged.ExtraEnv)
	assert.Equal(t, spec.ExtraEnvFrom, merged.ExtraEnvFrom)
	assert.Equal(t, spec.PodLabels, merged.PodLabels)
	assert.Equal(t, spec.PodAnnotations, merged.PodAnnotations)
}

func TestMergePresetReplacesSchedulingBlockEntirely(t *testing.T) {
	t.Parallel()

	inline := &v1.SchedulingSpec{
		NodeAffinity: &corev1.NodeAffinity{
			RequiredDuringSchedulingIgnoredDuringExecution: &corev1.NodeSelector{
				NodeSelectorTerms: []corev1.NodeSelectorTerm{{
					MatchExpressions: []corev1.NodeSelectorRequirement{{
						Key: "inline", Operator: corev1.NodeSelectorOpExists,
					}},
				}},
			},
		},
	}
	spec := v1.ElasticsearchClusterSpec{
		PresetRef:              "standard",
		Scheduling:             inline,
		SecondaryStorageConfig: "my-storage-config",
	}

	merged := mergePreset(spec, fullPresetSpec())

	require.NotNil(t, merged.Scheduling)
	assert.Equal(t, inline, merged.Scheduling)
	assert.Empty(t, merged.Scheduling.Tolerations,
		"an inline scheduling block must drop the preset's tolerations, not merge them")
}

func TestMergePresetKeepsInstanceBoundFieldsFromSpec(t *testing.T) {
	t.Parallel()

	spec := v1.ElasticsearchClusterSpec{
		PresetRef:              "standard",
		SecondaryStorageConfig: "my-storage-config",
		Suspend:                true,
		Monitoring: &v1.MonitoringSpec{
			ServiceMonitor: &v1.ServiceMonitorSpec{Enabled: true},
		},
	}

	merged := mergePreset(spec, fullPresetSpec())

	assert.Equal(t, spec.PresetRef, merged.PresetRef)
	assert.Equal(t, spec.SecondaryStorageConfig, merged.SecondaryStorageConfig)
	assert.Equal(t, spec.Suspend, merged.Suspend)
	assert.Equal(t, spec.Monitoring, merged.Monitoring)
}

func TestMergePresetDoesNotAliasThePresetBaseline(t *testing.T) {
	t.Parallel()

	preset := fullPresetSpec()

	merged := mergePreset(v1.ElasticsearchClusterSpec{
		SecondaryStorageConfig: "my-storage-config",
	}, preset)
	merged.PodLabels["mutated"] = "yes"

	assert.NotContains(t, preset.Cluster.PodLabels, "mutated",
		"mutating the merged spec must not write through to the preset")
}

func TestValidateMerged(t *testing.T) {
	t.Parallel()

	complete := func() v1.ElasticsearchClusterSpec {
		return v1.ElasticsearchClusterSpec{
			Version:                "9.2.4",
			Replicas:               new(int32(3)),
			StorageSize:            new(resource.MustParse("64Gi")),
			SecondaryStorageConfig: "my-storage-config",
		}
	}

	tests := []struct {
		name    string
		mutate  func(*v1.ElasticsearchClusterSpec)
		wantErr []string
	}{
		{
			name:   "complete spec at the 9.x floor passes",
			mutate: func(*v1.ElasticsearchClusterSpec) {},
		},
		{
			name:   "8.19.0 is at the 8.x floor",
			mutate: func(s *v1.ElasticsearchClusterSpec) { s.Version = "8.19.0" },
		},
		{
			name:   "9.2.0 is at the 9.x floor",
			mutate: func(s *v1.ElasticsearchClusterSpec) { s.Version = "9.2.0" },
		},
		{
			name:   "a later major is above the floor",
			mutate: func(s *v1.ElasticsearchClusterSpec) { s.Version = "10.0.0" },
		},
		{
			name: "all required fields missing are each named",
			mutate: func(s *v1.ElasticsearchClusterSpec) {
				s.Version = ""
				s.Replicas = nil
				s.StorageSize = nil
			},
			wantErr: []string{"version", "replicas", "storageSize"},
		},
		{
			name:    "a single missing field is named",
			mutate:  func(s *v1.ElasticsearchClusterSpec) { s.StorageSize = nil },
			wantErr: []string{"storageSize"},
		},
		{
			name:    "8.18.0 is below the 8.x floor",
			mutate:  func(s *v1.ElasticsearchClusterSpec) { s.Version = versionBelowFloor },
			wantErr: []string{versionBelowFloor, "8.19+ or 9.2+"},
		},
		{
			name:    "9.1.9 is below the 9.x floor",
			mutate:  func(s *v1.ElasticsearchClusterSpec) { s.Version = "9.1.9" },
			wantErr: []string{"9.1.9", "8.19+ or 9.2+"},
		},
		{
			name:    "7.17.0 is below both floors",
			mutate:  func(s *v1.ElasticsearchClusterSpec) { s.Version = "7.17.0" },
			wantErr: []string{"7.17.0"},
		},
		{
			name:    "an unparsable version is rejected",
			mutate:  func(s *v1.ElasticsearchClusterSpec) { s.Version = "nine.two.four" },
			wantErr: []string{"nine.two.four"},
		},
		{
			name: "a missing field and a below-floor version are both reported",
			mutate: func(s *v1.ElasticsearchClusterSpec) {
				s.Version = versionBelowFloor
				s.StorageSize = nil
			},
			wantErr: []string{"storageSize", versionBelowFloor},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			spec := complete()
			tt.mutate(&spec)

			err := validateMerged(spec)
			if len(tt.wantErr) == 0 {
				assert.NoError(t, err)
				return
			}
			require.Error(t, err)
			for _, want := range tt.wantErr {
				assert.ErrorContains(t, err, want)
			}
		})
	}
}
