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

// fullPreset returns a preset baseline with every merge rule represented, so
// the overlay tests can prove per-field inheritance and override.
func fullPreset() *v1.CamundaClusterPresetSpec {
	return &v1.CamundaClusterPresetSpec{Cluster: v1.CamundaClusterSpec{
		Version: "8.9.0",
		Auth: &v1.ClusterAuthSpec{
			ClientID:        "preset-client",
			Audience:        "preset-audience",
			ClientSecretRef: &v1.LocalSecretKeyRef{Name: "p", Key: "s"},
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
		Zeebe: &v1.ZeebeSpec{
			WorkloadSpec: v1.WorkloadSpec{
				Replicas: new(int32(3)),
				Resources: &corev1.ResourceRequirements{
					Requests: corev1.ResourceList{
						corev1.ResourceCPU:    resource.MustParse("1"),
						corev1.ResourceMemory: resource.MustParse("2Gi"),
					},
				},
				ExtraEnv:   []corev1.EnvVar{{Name: "JAVA_OPTS", Value: "-Xmx4g"}},
				PodLabels:  map[string]string{"tier": "preset"},
				Scheduling: &v1.SchedulingSpec{Tolerations: []corev1.Toleration{{Key: "preset-zeebe"}}},
			},
			Partitions:        new(int32(3)),
			ReplicationFactor: new(int32(3)),
			StorageClassName:  new("ssd"),
			StorageSize:       new(resource.MustParse("32Gi")),
			PersistentVolumeClaimRetentionPolicy: &v1.PersistentVolumeClaimRetentionPolicy{
				WhenDeleted: v1.RetainPersistentVolumeClaimRetentionPolicyType,
			},
		},
		Gateway: &v1.GatewaySpec{
			Mode:         v1.ComponentModeStandalone,
			WorkloadSpec: v1.WorkloadSpec{Replicas: new(int32(2))},
		},
		Operate:    &v1.WebAppSpec{Mode: v1.ComponentModeStandalone},
		Connectors: &v1.ConnectorsSpec{Enabled: new(true), Version: "8.9.7"},
	}}
}

func TestMergeSpec(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		spec v1.CamundaClusterSpec
		want func(t *testing.T, got v1.CamundaClusterSpec)
	}{
		{
			"scalar override: version",
			v1.CamundaClusterSpec{Version: "8.9.1"},
			func(t *testing.T, got v1.CamundaClusterSpec) { assert.Equal(t, "8.9.1", got.Version) },
		},
		{
			"scalar inherit: version",
			v1.CamundaClusterSpec{},
			func(t *testing.T, got v1.CamundaClusterSpec) { assert.Equal(t, "8.9.0", got.Version) },
		},
		{
			"auth fields override individually",
			v1.CamundaClusterSpec{Auth: &v1.ClusterAuthSpec{ClientID: "mine"}},
			func(t *testing.T, got v1.CamundaClusterSpec) {
				require.NotNil(t, got.Auth)
				assert.Equal(t, "mine", got.Auth.ClientID)
				assert.Equal(t, "preset-audience", got.Auth.Audience)
				require.NotNil(t, got.Auth.ClientSecretRef)
				assert.Equal(t, "p", got.Auth.ClientSecretRef.Name)
			},
		},
		{
			"pointer override: zeebe.replicas, partitions inherited",
			v1.CamundaClusterSpec{Zeebe: &v1.ZeebeSpec{WorkloadSpec: v1.WorkloadSpec{Replicas: new(int32(5))}}},
			func(t *testing.T, got v1.CamundaClusterSpec) {
				require.NotNil(t, got.Zeebe)
				assert.Equal(t, int32(5), *got.Zeebe.Replicas)
				assert.Equal(t, int32(3), *got.Zeebe.Partitions)
				assert.Equal(t, int32(3), *got.Zeebe.ReplicationFactor)
				assert.Equal(t, "ssd", *got.Zeebe.StorageClassName)
				assert.Equal(t, resource.MustParse("32Gi"), *got.Zeebe.StorageSize)
				assert.Equal(
					t,
					v1.RetainPersistentVolumeClaimRetentionPolicyType,
					got.Zeebe.PersistentVolumeClaimRetentionPolicy.WhenDeleted,
				)
			},
		},
		{
			"zeebe storage fields override individually",
			v1.CamundaClusterSpec{Zeebe: &v1.ZeebeSpec{
				StorageSize: new(resource.MustParse("64Gi")),
				PersistentVolumeClaimRetentionPolicy: &v1.PersistentVolumeClaimRetentionPolicy{
					WhenDeleted: v1.DeletePersistentVolumeClaimRetentionPolicyType,
				},
			}},
			func(t *testing.T, got v1.CamundaClusterSpec) {
				assert.Equal(t, resource.MustParse("64Gi"), *got.Zeebe.StorageSize)
				assert.Equal(t, "ssd", *got.Zeebe.StorageClassName)
				assert.Equal(
					t,
					v1.DeletePersistentVolumeClaimRetentionPolicyType,
					got.Zeebe.PersistentVolumeClaimRetentionPolicy.WhenDeleted,
				)
			},
		},
		{
			"resources merge per entry",
			v1.CamundaClusterSpec{Zeebe: &v1.ZeebeSpec{WorkloadSpec: v1.WorkloadSpec{
				Resources: &corev1.ResourceRequirements{
					Requests: corev1.ResourceList{corev1.ResourceMemory: resource.MustParse("4Gi")},
					Limits:   corev1.ResourceList{corev1.ResourceCPU: resource.MustParse("2")},
				},
			}}},
			func(t *testing.T, got v1.CamundaClusterSpec) {
				require.NotNil(t, got.Zeebe.Resources)
				assert.Equal(t, resource.MustParse("4Gi"), got.Zeebe.Resources.Requests[corev1.ResourceMemory])
				assert.Equal(t, resource.MustParse("1"), got.Zeebe.Resources.Requests[corev1.ResourceCPU])
				assert.Equal(t, resource.MustParse("2"), got.Zeebe.Resources.Limits[corev1.ResourceCPU])
			},
		},
		{
			"extraEnv by name, cluster wins, preset first",
			v1.CamundaClusterSpec{
				ExtraEnv: []corev1.EnvVar{{Name: "KEEP", Value: "cluster"}, {Name: "NEW", Value: "x"}},
			},
			func(t *testing.T, got v1.CamundaClusterSpec) {
				assert.Equal(
					t, []corev1.EnvVar{
						{Name: "TZ", Value: "UTC"},
						{Name: "KEEP", Value: "cluster"},
						{Name: "NEW", Value: "x"},
					}, got.ExtraEnv,
				)
			},
		},
		{
			"component extraEnv by name",
			v1.CamundaClusterSpec{Zeebe: &v1.ZeebeSpec{WorkloadSpec: v1.WorkloadSpec{
				ExtraEnv: []corev1.EnvVar{{Name: "JAVA_OPTS", Value: "-Xmx6g"}},
			}}},
			func(t *testing.T, got v1.CamundaClusterSpec) {
				assert.Equal(t, []corev1.EnvVar{{Name: "JAVA_OPTS", Value: "-Xmx6g"}}, got.Zeebe.ExtraEnv)
			},
		},
		{
			"extraEnvFrom concatenates preset first",
			v1.CamundaClusterSpec{ExtraEnvFrom: []corev1.EnvFromSource{{
				SecretRef: &corev1.SecretEnvSource{
					LocalObjectReference: corev1.LocalObjectReference{Name: "cluster-secret"},
				},
			}}},
			func(t *testing.T, got v1.CamundaClusterSpec) {
				require.Len(t, got.ExtraEnvFrom, 2)
				assert.Equal(t, "preset-cm", got.ExtraEnvFrom[0].ConfigMapRef.Name)
				assert.Equal(t, "cluster-secret", got.ExtraEnvFrom[1].SecretRef.Name)
			},
		},
		{
			"podLabels and podAnnotations merge by key, cluster wins",
			v1.CamundaClusterSpec{
				PodLabels:      map[string]string{"shared": "cluster", "own": "cluster"},
				PodAnnotations: map[string]string{"shared": "cluster"},
			},
			func(t *testing.T, got v1.CamundaClusterSpec) {
				assert.Equal(
					t,
					map[string]string{"team": "preset", "shared": "cluster", "own": "cluster"},
					got.PodLabels,
				)
				assert.Equal(t, map[string]string{"note": "preset", "shared": "cluster"}, got.PodAnnotations)
			},
		},
		{
			"component podLabels merge by key",
			v1.CamundaClusterSpec{Zeebe: &v1.ZeebeSpec{WorkloadSpec: v1.WorkloadSpec{
				PodLabels: map[string]string{"own": "cluster"},
			}}},
			func(t *testing.T, got v1.CamundaClusterSpec) {
				assert.Equal(t, map[string]string{"tier": "preset", "own": "cluster"}, got.Zeebe.PodLabels)
			},
		},
		{
			"scheduling replaced entirely at top level",
			v1.CamundaClusterSpec{Scheduling: &v1.SchedulingSpec{NodeAffinity: &corev1.NodeAffinity{}}},
			func(t *testing.T, got v1.CamundaClusterSpec) {
				require.NotNil(t, got.Scheduling)
				assert.NotNil(t, got.Scheduling.NodeAffinity)
				assert.Empty(
					t,
					got.Scheduling.Tolerations,
					"an inline scheduling block must drop the preset's tolerations",
				)
				assert.Equal(t, []corev1.Toleration{{Key: "preset-zeebe"}}, got.Zeebe.Scheduling.Tolerations)
			},
		},
		{
			"scheduling at component level replaced only there",
			v1.CamundaClusterSpec{Zeebe: &v1.ZeebeSpec{WorkloadSpec: v1.WorkloadSpec{
				Scheduling: &v1.SchedulingSpec{NodeAffinity: &corev1.NodeAffinity{}},
			}}},
			func(t *testing.T, got v1.CamundaClusterSpec) {
				assert.Empty(t, got.Zeebe.Scheduling.Tolerations)
				assert.Equal(t, []corev1.Toleration{{Key: "preset"}}, got.Scheduling.Tolerations)
			},
		},
		{
			"mode overrides individually",
			v1.CamundaClusterSpec{
				Gateway: &v1.GatewaySpec{Mode: v1.ComponentModeEmbedded},
				Operate: &v1.WebAppSpec{WorkloadSpec: v1.WorkloadSpec{Replicas: new(int32(2))}},
			},
			func(t *testing.T, got v1.CamundaClusterSpec) {
				assert.Equal(t, v1.ComponentModeEmbedded, got.Gateway.Mode)
				assert.Equal(t, int32(2), *got.Gateway.Replicas)
				assert.Equal(t, v1.ComponentModeStandalone, got.Operate.Mode)
				assert.Equal(t, int32(2), *got.Operate.Replicas)
			},
		},
		{
			"component block absent in the preset comes from the cluster",
			v1.CamundaClusterSpec{Tasklist: &v1.WebAppSpec{Mode: v1.ComponentModeStandalone}},
			func(t *testing.T, got v1.CamundaClusterSpec) {
				require.NotNil(t, got.Tasklist)
				assert.Equal(t, v1.ComponentModeStandalone, got.Tasklist.Mode)
				assert.Nil(t, got.Admin)
			},
		},
		{
			"connectors.enabled pointer override, version inherited",
			v1.CamundaClusterSpec{Connectors: &v1.ConnectorsSpec{Enabled: new(false)}},
			func(t *testing.T, got v1.CamundaClusterSpec) {
				require.NotNil(t, got.Connectors)
				assert.False(t, *got.Connectors.Enabled)
				assert.Equal(t, "8.9.7", got.Connectors.Version)
			},
		},
		{
			"instance-bound fields come from the cluster",
			v1.CamundaClusterSpec{
				PlatformConfigRef:  "pc",
				PresetRef:          "medium",
				ExternalURL:        "https://example.com",
				ServiceAccount:     &v1.ServiceAccountSpec{Annotations: map[string]string{"a": "b"}},
				StorageRef:         "storage",
				BackupStorageRef:   "backup",
				DocumentStorageRef: "documents",
				Monitoring:         &v1.ClusterMonitoringSpec{ServiceMonitor: &v1.ServiceMonitorSpec{Enabled: true}},
				Suspend:            true,
				Pause:              true,
			},
			func(t *testing.T, got v1.CamundaClusterSpec) {
				assert.Equal(t, "pc", got.PlatformConfigRef)
				assert.Equal(t, "medium", got.PresetRef)
				assert.Equal(t, "https://example.com", got.ExternalURL)
				assert.Equal(t, map[string]string{"a": "b"}, got.ServiceAccount.Annotations)
				assert.Equal(t, "storage", got.StorageRef)
				assert.Equal(t, "backup", got.BackupStorageRef)
				assert.Equal(t, "documents", got.DocumentStorageRef)
				assert.True(t, got.Monitoring.ServiceMonitor.Enabled)
				assert.True(t, got.Suspend)
				assert.True(t, got.Pause)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			tt.want(t, MergeSpec(tt.spec, fullPreset(), nil))
		})
	}
}

// mergeEnv is a pure function and states its own contract, so it holds that
// contract whatever the caller passes. The schema of the CRD rejects a
// manifest that repeats a name inside one list, so a repeat cannot arrive
// through the API today, but the merge must still leave one entry per name.
func TestMergeEnvKeepsEachNameOnce(t *testing.T) {
	t.Parallel()

	merged := mergeEnv(
		[]corev1.EnvVar{{Name: "A", Value: "base"}, {Name: "B", Value: "base"}},
		[]corev1.EnvVar{
			{Name: "B", Value: "over"},
			{Name: "C", Value: "first"},
			{Name: "C", Value: "second"},
		},
	)

	assert.Equal(
		t,
		[]corev1.EnvVar{
			{Name: "A", Value: "base"},
			{Name: "B", Value: "over"},
			{Name: "C", Value: "second"},
		},
		merged,
	)
}

func TestMergeSpecNilPresetReturnsSpecUnchanged(t *testing.T) {
	t.Parallel()

	spec := v1.CamundaClusterSpec{Version: "8.9.1", StorageRef: "storage"}

	assert.Equal(t, spec, MergeSpec(spec, nil, nil))
}

func TestMergeSpecSharesNoMemoryWithSpecOrPreset(t *testing.T) {
	t.Parallel()

	preset := fullPreset()
	spec := v1.CamundaClusterSpec{
		Tasklist:       &v1.WebAppSpec{WorkloadSpec: v1.WorkloadSpec{PodLabels: map[string]string{"own": "cluster"}}},
		ServiceAccount: &v1.ServiceAccountSpec{Annotations: map[string]string{"a": "b"}},
	}
	merged := MergeSpec(spec, preset, nil)

	const changed = "changed"
	merged.PodLabels["team"] = changed
	merged.Zeebe.ExtraEnv[0].Value = changed
	merged.Tasklist.PodLabels["own"] = changed
	merged.ServiceAccount.Annotations["a"] = changed

	assert.Equal(t, "preset", preset.Cluster.PodLabels["team"])
	assert.Equal(t, "-Xmx4g", preset.Cluster.Zeebe.ExtraEnv[0].Value)
	assert.Equal(t, "cluster", spec.Tasklist.PodLabels["own"])
	assert.Equal(t, "b", spec.ServiceAccount.Annotations["a"])
}

func TestMergeSpecNilPresetSharesNoMemoryWithSpec(t *testing.T) {
	t.Parallel()

	spec := v1.CamundaClusterSpec{PodLabels: map[string]string{"own": "cluster"}}
	merged := MergeSpec(spec, nil, nil)

	merged.PodLabels["own"] = "changed"

	assert.Equal(t, "cluster", spec.PodLabels["own"])
}

func TestValidateMerged(t *testing.T) {
	t.Parallel()

	valid := v1.CamundaClusterSpec{
		Version: "8.9.0",
		Zeebe: &v1.ZeebeSpec{
			WorkloadSpec:      v1.WorkloadSpec{Replicas: new(int32(3))},
			ReplicationFactor: new(int32(3)),
		},
		Connectors: &v1.ConnectorsSpec{Enabled: new(true), Version: "8.9.7"},
	}

	tests := []struct {
		name    string
		spec    v1.CamundaClusterSpec
		wantErr string
	}{
		{"valid spec", valid, ""},
		{"missing version", v1.CamundaClusterSpec{}, "missing required fields after preset merge: version"},
		{
			"version below floor",
			v1.CamundaClusterSpec{Version: "8.8.0"},
			"version 8.8.0 is below the supported floor 8.9.0",
		},
		{"malformed version", v1.CamundaClusterSpec{Version: "8.x.0"}, "version 8.x.0 is not of the form x.y.z"},
		{"later minor passes", v1.CamundaClusterSpec{Version: "8.10.0"}, ""},
		{"later major passes", v1.CamundaClusterSpec{Version: "9.0.0"}, ""},
		{
			"replicationFactor above default replicas",
			v1.CamundaClusterSpec{Version: "8.9.0", Zeebe: &v1.ZeebeSpec{ReplicationFactor: new(int32(3))}},
			"zeebe.replicationFactor 3 exceeds zeebe.replicas 1",
		},
		{
			"partitions below one",
			v1.CamundaClusterSpec{Version: "8.9.0", Zeebe: &v1.ZeebeSpec{Partitions: new(int32(0))}},
			"zeebe.partitions 0 is below the minimum 1",
		},
		{
			"connectors enabled without version",
			v1.CamundaClusterSpec{Version: "8.9.0", Connectors: &v1.ConnectorsSpec{Enabled: new(true)}},
			"connectors.version is required when connectors are enabled",
		},
		{
			"connectors disabled without version",
			v1.CamundaClusterSpec{Version: "8.9.0", Connectors: &v1.ConnectorsSpec{}},
			"",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := ValidateMerged(tt.spec)
			if tt.wantErr == "" {
				assert.NoError(t, err)
				return
			}
			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.wantErr)
		})
	}
}

func TestValidateMergedJoinsEveryProblem(t *testing.T) {
	t.Parallel()

	spec := v1.CamundaClusterSpec{
		Zeebe:      &v1.ZeebeSpec{ReplicationFactor: new(int32(3))},
		Connectors: &v1.ConnectorsSpec{Enabled: new(true)},
	}

	err := ValidateMerged(spec)

	require.Error(t, err)
	assert.Equal(
		t,
		"missing required fields after preset merge: version; "+
			"zeebe.replicationFactor 3 exceeds zeebe.replicas 1; "+
			"connectors.version is required when connectors are enabled",
		err.Error(),
	)
}

// The admin block never merges per field: a cluster that sets it replaces the
// block of the preset entirely, so one manifest names every administrator.
func TestMergeSpecAdminBlockReplacesWholesale(t *testing.T) {
	t.Parallel()

	preset := &v1.CamundaClusterPresetSpec{Cluster: v1.CamundaClusterSpec{
		Auth: &v1.ClusterAuthSpec{
			ClientID: "preset-client",
			Admin: &v1.ClusterAdminSpec{
				Users:   []string{"platform-ops@example.com"},
				Clients: []string{"platform-ops"},
			},
		},
	}}

	inherited := MergeSpec(v1.CamundaClusterSpec{}, preset, nil)
	require.NotNil(t, inherited.Auth)
	require.NotNil(t, inherited.Auth.Admin)
	assert.Equal(t, []string{"platform-ops@example.com"}, inherited.Auth.Admin.Users)
	assert.Equal(t, []string{"platform-ops"}, inherited.Auth.Admin.Clients)

	replaced := MergeSpec(v1.CamundaClusterSpec{
		Auth: &v1.ClusterAuthSpec{Admin: &v1.ClusterAdminSpec{Users: []string{"team-a@example.com"}}},
	}, preset, nil)
	require.NotNil(t, replaced.Auth.Admin)
	assert.Equal(t, []string{"team-a@example.com"}, replaced.Auth.Admin.Users)
	assert.Empty(t, replaced.Auth.Admin.Clients, "the clients of the preset do not survive a cluster block")
	assert.Equal(t, "preset-client", replaced.Auth.ClientID, "the other auth fields still merge per field")
}

// The basic block never merges per field: a cluster that sets it replaces
// the block of the preset entirely. A preset block still applies to every
// cluster that sets none, so a fleet can rotate on one preset change.
func TestMergeSpecBasicBlockReplacesWholesale(t *testing.T) {
	t.Parallel()

	preset := &v1.CamundaClusterPresetSpec{Cluster: v1.CamundaClusterSpec{
		Auth: &v1.ClusterAuthSpec{
			ClientID: "preset-client",
			Basic:    &v1.BasicAuthSpec{PasswordRotation: "fleet-2026-08"},
		},
	}}

	inherited := MergeSpec(v1.CamundaClusterSpec{}, preset, nil)
	require.NotNil(t, inherited.Auth)
	require.NotNil(t, inherited.Auth.Basic)
	assert.Equal(t, "fleet-2026-08", inherited.Auth.Basic.PasswordRotation)

	replaced := MergeSpec(v1.CamundaClusterSpec{
		Auth: &v1.ClusterAuthSpec{Basic: &v1.BasicAuthSpec{PasswordRotation: "mine"}},
	}, preset, nil)
	require.NotNil(t, replaced.Auth.Basic)
	assert.Equal(t, "mine", replaced.Auth.Basic.PasswordRotation)
	assert.Equal(t, "preset-client", replaced.Auth.ClientID, "the other auth fields still merge per field")
}
