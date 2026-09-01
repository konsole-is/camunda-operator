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
		Auth: &v1.ClusterAuthSpec{
			ClientID: "medium-clusters",
			Audience: "medium-clusters",
			ClientSecretRef: &v1.LocalSecretKeyRef{
				Name: "medium-clusters-oidc-secret", Key: "client-secret",
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
		Admin:      &v1.WebAppSpec{Mode: v1.ComponentModeEmbedded},
		Connectors: &v1.ConnectorsSpec{Enabled: new(true)},
	}}
}

// mediumRelease is the release that the preset fixtures pair with the medium
// preset: the versions that the preset no longer carries.
func mediumRelease() *v1.CamundaReleaseSpec {
	return &v1.CamundaReleaseSpec{
		Version:    "8.9.0",
		Connectors: &v1.ReleaseConnectorsSpec{Version: "8.9.7"},
	}
}

// fixtureMinimal is the doc's minimal cluster: an Elasticsearch binding
// without CA, basic auth, and a platform config without license or registry.
func fixtureMinimal(t *testing.T) Input {
	t.Helper()
	return newInput(t, nil)
}

// fixtureDefault is the doc's realistic cluster on the medium preset and
// the medium release: three brokers, resources, extra env, connectors,
// monitoring, a service account, a license, and a mirror.
func fixtureDefault(t *testing.T) Input {
	t.Helper()
	return newInput(t, func(in *Input) {
		in.Cluster.Spec = v1.CamundaClusterSpec{
			PlatformConfigRef: "my-platform-config",
			PresetRef:         "medium",
			ReleaseRef:        "camunda-8-9-0",
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
		in.Effective = NewEffective(MergeSpec(in.Cluster.Spec, mediumPreset(), mediumRelease()))
		in.Platform = v1.CamundaPlatformConfigSpec{
			LicenseSecretRef: &v1.SecretKeyRef{Name: "camunda-license", Namespace: "camunda-system", Key: "key"},
			Images: &v1.ImagesSpec{
				Camunda:    "registry.example.com/camunda/camunda",
				Connectors: "registry.example.com/camunda/connectors-bundle",
			},
		}
		in.Storage.Elasticsearch.CASecretRef = &v1.LocalSecretKeyRef{
			Name: "my-cluster-es-es-http-certs-public", Key: "ca.crt",
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
		in.Cluster.Spec.Admin = &v1.WebAppSpec{Mode: v1.ComponentModeStandalone}
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
				Credentials: v1.LocalCredentialsSecretRef{
					Name: "camunda-db-user", UsernameKey: "username", PasswordKey: "password",
				},
			},
		}
	})
}

// fixtureOIDC uses a platform config with OIDC, discovery overrides, and both
// claim names, a cluster auth override of the client id, an admin bootstrap
// with all three member kinds, and an externalUrl.
func fixtureOIDC(t *testing.T) Input {
	t.Helper()
	return newInput(t, func(in *Input) {
		in.Platform = oidcPlatform()
		in.Platform.Auth.OIDC.UsernameClaim = "preferred_username"
		in.Platform.Auth.OIDC.ClientIDClaim = "client_id"
		in.Cluster.Spec.ExternalURL = "https://my-cluster.camunda.example.com"
		in.Cluster.Spec.Auth = &v1.ClusterAuthSpec{
			ClientID: "my-cluster-client",
			ClientSecretRef: &v1.LocalSecretKeyRef{
				Name: "my-cluster-oidc-secret",
				Key:  "client-secret",
			},
			Admin: &v1.ClusterAdminSpec{
				Users:   []string{"ada@example.com"},
				Clients: []string{"my-cluster-client"},
				MappingRules: []v1.AdminMappingRule{
					{ID: "platform-admins", ClaimName: "groups", ClaimValue: "camunda-admins"},
				},
			},
		}
		in.Cluster.Spec.Connectors = &v1.ConnectorsSpec{Enabled: new(true), Version: "8.9.7"}
		in.Effective = NewEffective(in.Cluster.Spec)
	})
}

// fixturePreset sets only the references on the cluster; the medium preset
// provides the shape and the medium release the versions.
func fixturePreset(t *testing.T) Input {
	t.Helper()
	return newInput(t, func(in *Input) {
		in.Cluster.Spec = v1.CamundaClusterSpec{
			PlatformConfigRef: "my-platform-config",
			PresetRef:         "medium",
			ReleaseRef:        "camunda-8-9-0",
			StorageRef:        "my-storage-config",
		}
		in.Effective = NewEffective(MergeSpec(in.Cluster.Spec, mediumPreset(), mediumRelease()))
	})
}

// fixtureRelease pins the images of both processes through the release. The
// broker StatefulSet keeps the effective version in its annotation while the
// containers pull the pinned references.
func fixtureRelease(t *testing.T) Input {
	t.Helper()
	return newInput(t, func(in *Input) {
		in.Cluster.Spec = v1.CamundaClusterSpec{
			PlatformConfigRef: "my-platform-config",
			PresetRef:         "medium",
			ReleaseRef:        "camunda-8-9-0",
			StorageRef:        "my-storage-config",
		}
		release := mediumRelease()
		release.Images = &v1.ReleaseImagesSpec{
			Camunda: "mirror.example.com/camunda/camunda@sha256:" +
				"7d865e959b2466918c9863afca942d0fb89d7c9ac0c99bafc3749504ded97730",
			Connectors: "mirror.example.com/camunda/connectors-bundle:8.9.7-patched",
		}
		in.Effective = NewEffective(MergeSpec(in.Cluster.Spec, mediumPreset(), release))
		in.Images = release.Images
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

// fixtureBackupElasticsearch is an Elasticsearch cluster that backs up to an
// S3-compatible bucket with static keys: the shape the kind suite runs.
func fixtureBackupElasticsearch(t *testing.T) Input {
	t.Helper()
	return newInput(t, func(in *Input) {
		in.Cluster.Spec.BackupStorageRef = "my-backup-config"
		in.Effective = NewEffective(in.Cluster.Spec)
		in.Storage.Elasticsearch.SnapshotRepository = fixtureSnapshotRepository
		in.Backup = minioBucket()
	})
}

// fixtureBackupRDBMS is a relational cluster that backs up to a cloud bucket
// through workload identity, with a backup policy that overrides the
// documented defaults.
func fixtureBackupRDBMS(t *testing.T) Input {
	t.Helper()
	return newInput(t, func(in *Input) {
		in.Cluster.Spec.BackupStorageRef = "my-backup-config"
		in.Cluster.Spec.Backup = &v1.ClusterBackupSpec{
			PrimaryStorage: &v1.PrimaryStorageBackupSpec{
				Schedule:  "PT30M",
				Retention: &v1.PrimaryStorageRetentionSpec{Window: "P14D"},
			},
		}
		in.Effective = NewEffective(in.Cluster.Spec)
		in.Storage = Storage{
			Type: v1.SecondaryStorageTypeRDBMS,
			RDBMS: &RDBMSStorage{
				Host:     "my-db-rw.my-cluster-ns.svc",
				Port:     5432,
				Database: "camunda",
				Credentials: v1.LocalCredentialsSecretRef{
					Name: "camunda-db-user", UsernameKey: "username", PasswordKey: "password",
				},
			},
		}
		in.Backup = s3Bucket()
		in.ServiceAccountAnnotations = map[string]string{
			v1.IRSARoleARNAnnotation: "arn:aws:iam::123456789012:role/camunda",
		}
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
		"release":    fixtureRelease(t),
		"suspended":  fixtureSuspended(t),

		"backup-elasticsearch": fixtureBackupElasticsearch(t),
		"backup-rdbms":         fixtureBackupRDBMS(t),
	}
}
