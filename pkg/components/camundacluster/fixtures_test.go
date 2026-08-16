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

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"

	v1 "github.com/konsole-is/camunda-operator/api/v1"
)

// The golden fixtures are named after the state they encode. Each one is an
// Input, so the goldens and the declared-keys test share them.

// mediumPreset is the "medium" preset of the CamundaClusterPreset doc.
func mediumPreset() *v1.CamundaClusterPresetSpec {
	return &v1.CamundaClusterPresetSpec{Cluster: v1.CamundaClusterSpec{
		Version: "8.9.0",
		Auth: &v1.ClusterAuthSpec{
			ClientID: "medium-clusters",
			Audience: "medium-clusters",
			ClientSecretRef: &v1.SecretKeyRef{
				Name: "medium-clusters-oidc-secret", Namespace: "camunda-system", Key: "client-secret",
			},
		},
		ExtraEnv:       []corev1.EnvVar{{Name: "TZ", Value: "UTC"}},
		PodLabels:      map[string]string{"company.com/team": "automation-ops"},
		PodAnnotations: map[string]string{"company.com/cluster-preset": "medium"},
		Zeebe: &v1.ZeebeSpec{
			WorkloadSpec: v1.WorkloadSpec{
				Replicas: new(int32(3)),
				Resources: &corev1.ResourceRequirements{Requests: corev1.ResourceList{
					corev1.ResourceCPU:    resource.MustParse("1"),
					corev1.ResourceMemory: resource.MustParse("2Gi"),
				}},
				ExtraEnv: []corev1.EnvVar{{Name: "JAVA_OPTS", Value: "-Xmx4g"}},
			},
			Partitions:        new(int32(3)),
			ReplicationFactor: new(int32(3)),
			StorageClassName:  new("ssd"),
			StorageSize:       new(resource.MustParse("32Gi")),
		},
		Gateway: &v1.GatewaySpec{
			Mode: v1.ComponentModeStandalone,
			WorkloadSpec: v1.WorkloadSpec{
				Replicas: new(int32(2)),
				Resources: &corev1.ResourceRequirements{Requests: corev1.ResourceList{
					corev1.ResourceCPU:    resource.MustParse("500m"),
					corev1.ResourceMemory: resource.MustParse("1Gi"),
				}},
			},
		},
		Operate:    &v1.WebAppSpec{Mode: v1.ComponentModeEmbedded},
		Tasklist:   &v1.WebAppSpec{Mode: v1.ComponentModeEmbedded},
		Identity:   &v1.WebAppSpec{Mode: v1.ComponentModeEmbedded},
		Connectors: &v1.ConnectorsSpec{Enabled: new(true), Version: "8.9.7"},
	}}
}

// fixtureMinimal is the doc's minimal cluster: an Elasticsearch binding
// without CA, basic auth, and a platform config without license or registry.
func fixtureMinimal(t *testing.T) Input {
	t.Helper()
	return newInput(t, nil)
}

// fixtureDefault is the doc's realistic cluster on the medium preset: three
// brokers, resources, extra env, connectors, monitoring, a service account,
// a license, and a registry.
func fixtureDefault(t *testing.T) Input {
	t.Helper()
	return newInput(t, func(in *Input) {
		in.Cluster.Spec = v1.CamundaClusterSpec{
			PlatformConfigRef: "my-platform-config",
			PresetRef:         "medium",
			Version:           "8.9.9",
			ExternalURL:       "https://my-cluster.camunda.example.com",
			ServiceAccount: &v1.ServiceAccountSpec{Annotations: map[string]string{
				"eks.amazonaws.com/role-arn": "arn:aws:iam::123456789012:role/my-cluster-role",
			}},
			Zeebe: &v1.ZeebeSpec{
				WorkloadSpec: v1.WorkloadSpec{
					Replicas: new(int32(3)),
					Resources: &corev1.ResourceRequirements{
						Requests: corev1.ResourceList{corev1.ResourceMemory: resource.MustParse("8Gi")},
					},
					ExtraEnv: []corev1.EnvVar{{Name: "JAVA_TOOL_OPTIONS", Value: "-XX:+ExitOnOutOfMemoryError -Xmx6g"}},
					PodAnnotations: map[string]string{
						"prometheus.io/scrape": "true",
					},
					Scheduling: &v1.SchedulingSpec{Tolerations: []corev1.Toleration{{
						Key:      "dedicated",
						Operator: corev1.TolerationOpEqual,
						Value:    "camunda",
						Effect:   corev1.TaintEffectNoSchedule,
					}}},
				},
			},
			Connectors: &v1.ConnectorsSpec{
				WorkloadSpec: v1.WorkloadSpec{Replicas: new(int32(2))},
			},
			ExtraEnvFrom: []corev1.EnvFromSource{{
				ConfigMapRef: &corev1.ConfigMapEnvSource{
					LocalObjectReference: corev1.LocalObjectReference{Name: "overrides"},
				},
			}},
			PodLabels:  map[string]string{"team": "platform"},
			StorageRef: "my-storage-config",
			Monitoring: &v1.ClusterMonitoringSpec{ServiceMonitor: &v1.ServiceMonitorSpec{
				Enabled: true,
				Labels:  map[string]string{"release": "prometheus"},
			}},
		}
		in.Effective = NewEffective(MergePreset(in.Cluster.Spec, mediumPreset()))
		in.Platform = v1.CamundaPlatformConfigSpec{
			LicenseSecretRef: &v1.SecretKeyRef{Name: "camunda-license", Namespace: "camunda-system", Key: "key"},
			ImageRegistry:    "registry.example.com",
		}
		in.Storage.Elasticsearch.CASecretRef = &v1.SecretKeyRef{
			Name: "my-cluster-es-es-http-certs-public", Namespace: "my-cluster-ns", Key: "ca.crt",
		}
		in.HashInputs = []string{
			"Secret/my-cluster-ns/es-user=12",
			"CamundaPlatformConfig//my-platform-config=3",
			"SecondaryStorageConfig/my-cluster-ns/my-storage-config=2",
		}
		in.ServiceMonitorSupported = true
	})
}

// fixtureAllInOne embeds the gateway in the brokers.
func fixtureAllInOne(t *testing.T) Input {
	t.Helper()
	return newInput(t, func(in *Input) {
		in.Cluster.Spec.Gateway = &v1.GatewaySpec{Mode: v1.ComponentModeEmbedded}
		in.Effective = NewEffective(in.Cluster.Spec)
	})
}

// fixtureSeparated runs every web application standalone, plus connectors.
func fixtureSeparated(t *testing.T) Input {
	t.Helper()
	return newInput(t, func(in *Input) {
		in.Cluster.Spec.Operate = &v1.WebAppSpec{Mode: v1.ComponentModeStandalone}
		in.Cluster.Spec.Tasklist = &v1.WebAppSpec{Mode: v1.ComponentModeStandalone}
		in.Cluster.Spec.Identity = &v1.WebAppSpec{Mode: v1.ComponentModeStandalone}
		in.Cluster.Spec.Connectors = &v1.ConnectorsSpec{Enabled: new(true), Version: "8.9.7"}
		in.Effective = NewEffective(in.Cluster.Spec)
	})
}

// fixtureRDBMS binds the cluster to a PostgreSQL database.
func fixtureRDBMS(t *testing.T) Input {
	t.Helper()
	return newInput(t, func(in *Input) {
		in.Storage = Storage{
			Type: v1.SecondaryStorageTypeRDBMS,
			RDBMS: &RDBMSStorage{
				Host:     "my-db-rw.my-cluster-ns.svc",
				Port:     5432,
				Database: "camunda",
				Credentials: v1.CredentialsSecretRef{
					Name: "camunda-db-user", Namespace: "my-cluster-ns", UsernameKey: "username", PasswordKey: "password",
				},
			},
		}
	})
}

// fixtureOIDC uses a platform config with OIDC and discovery overrides, a
// cluster auth override of the client id, and an externalUrl.
func fixtureOIDC(t *testing.T) Input {
	t.Helper()
	return newInput(t, func(in *Input) {
		in.Platform = oidcPlatform()
		in.Cluster.Spec.ExternalURL = "https://my-cluster.camunda.example.com"
		in.Cluster.Spec.Auth = &v1.ClusterAuthSpec{
			ClientID: "my-cluster-client",
			ClientSecretRef: &v1.SecretKeyRef{
				Name:      "my-cluster-oidc-secret",
				Namespace: "my-cluster-ns",
				Key:       "client-secret",
			},
		}
		in.Cluster.Spec.Connectors = &v1.ConnectorsSpec{Enabled: new(true), Version: "8.9.7"}
		in.Effective = NewEffective(in.Cluster.Spec)
	})
}

// fixturePreset sets only the references on the cluster; the medium preset
// provides everything else.
func fixturePreset(t *testing.T) Input {
	t.Helper()
	return newInput(t, func(in *Input) {
		in.Cluster.Spec = v1.CamundaClusterSpec{
			PlatformConfigRef: "my-platform-config",
			PresetRef:         "medium",
			StorageRef:        "my-storage-config",
		}
		in.Effective = NewEffective(MergePreset(in.Cluster.Spec, mediumPreset()))
	})
}

// fixtureSuspended is the minimal cluster with suspend set.
func fixtureSuspended(t *testing.T) Input {
	t.Helper()
	return newInput(t, func(in *Input) {
		in.Cluster.Spec.Suspend = true
		in.Effective = NewEffective(in.Cluster.Spec)
	})
}

// goldenFixtures returns every fixture by name.
func goldenFixtures(t *testing.T) map[string]Input {
	t.Helper()
	return map[string]Input{
		"minimal":    fixtureMinimal(t),
		"default":    fixtureDefault(t),
		"all-in-one": fixtureAllInOne(t),
		"separated":  fixtureSeparated(t),
		"rdbms":      fixtureRDBMS(t),
		"oidc":       fixtureOIDC(t),
		"preset":     fixturePreset(t),
		"suspended":  fixtureSuspended(t),
	}
}
