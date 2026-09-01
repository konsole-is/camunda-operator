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
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	utilrand "k8s.io/apimachinery/pkg/util/rand"

	"k8s.io/client-go/util/retry"
	"sigs.k8s.io/controller-runtime/pkg/client"

	v1 "github.com/konsole-is/camunda-operator/api/v1"
	"github.com/konsole-is/camunda-operator/internal/fixtures"
)

// updateSpec applies mutate to the latest revision of obj and returns the
// admission error of the update. The controller writes status concurrently,
// so a stale revision would conflict.
func updateSpec(obj *v1.CamundaCluster, mutate func(*v1.CamundaCluster)) error {
	return retry.RetryOnConflict(retry.DefaultRetry, func() error {
		var latest v1.CamundaCluster
		if err := k8sClient.Get(ctx, client.ObjectKeyFromObject(obj), &latest); err != nil {
			return err
		}
		mutate(&latest)
		return k8sClient.Update(ctx, &latest)
	})
}

// applyZeebeEnv applies one spec.zeebe.extraEnv entry to obj under the given
// field manager, the way an extension controller attaches to a cluster it does
// not own.
func applyZeebeEnv(obj *v1.CamundaCluster, manager string, env corev1.EnvVar) error {
	patch := &v1.CamundaCluster{
		TypeMeta: metav1.TypeMeta{APIVersion: v1.GroupVersion.String(), Kind: "CamundaCluster"},
		ObjectMeta: metav1.ObjectMeta{
			Name:      obj.Name,
			Namespace: obj.Namespace,
		},
		Spec: v1.CamundaClusterSpec{
			Zeebe: &v1.ZeebeSpec{WorkloadSpec: v1.WorkloadSpec{ExtraEnv: []corev1.EnvVar{env}}},
		},
	}

	// No ForceOwnership: with a map list the two managers own different
	// entries and never conflict. A conflict here means the list went atomic
	// again, and the test must see that error rather than swallow it.
	//nolint:staticcheck // the repository applies through the deprecated client.Apply patch
	return k8sClient.Patch(ctx, patch, client.Apply, client.FieldOwner(manager))
}

// minimalCamundaCluster returns the minimal example of the CRD doc with a
// unique name in the schema test namespace.
func minimalCamundaCluster() *v1.CamundaCluster {
	return &v1.CamundaCluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "cc-" + utilrand.String(8),
			Namespace: fixtures.SchemaTestNamespace,
		},
		Spec: v1.CamundaClusterSpec{
			PlatformConfigRef: "my-platform-config",
			Version:           "8.9.0",
			StorageRef:        "my-storage-config",
		},
	}
}

// realisticCamundaCluster returns the realistic example of the CRD doc, with
// the zeebe storage fields set so the transition rules have a value to hold
// on to, with a unique name in the schema test namespace.
func realisticCamundaCluster() *v1.CamundaCluster {
	cluster := minimalCamundaCluster()
	cluster.Spec = v1.CamundaClusterSpec{
		PlatformConfigRef: "my-platform-config",
		PresetRef:         "medium",
		Version:           "8.9.1",
		ExternalURL:       "https://my-cluster.camunda.example.com",
		ServiceAccount: &v1.ServiceAccountSpec{
			Annotations: map[string]string{
				"eks.amazonaws.com/role-arn": "arn:aws:iam::123456789012:role/my-cluster-role",
			},
		},
		Zeebe: &v1.ZeebeSpec{
			WorkloadSpec: v1.WorkloadSpec{
				Replicas: new(int32(5)),
				Resources: &corev1.ResourceRequirements{
					Requests: corev1.ResourceList{corev1.ResourceMemory: resource.MustParse("8Gi")},
				},
				ExtraEnv: []corev1.EnvVar{{Name: "JAVA_OPTS", Value: "-Xmx6g"}},
			},
			Partitions:        new(int32(3)),
			ReplicationFactor: new(int32(3)),
			StorageClassName:  new("ssd"),
			StorageSize:       new(resource.MustParse("32Gi")),
		},
		Connectors:       &v1.ConnectorsSpec{Enabled: new(true), Version: "8.9.7"},
		StorageRef:       "my-storage-config",
		BackupStorageRef: "my-backup-config",
		Monitoring: &v1.ClusterMonitoringSpec{
			ServiceMonitor: &v1.ServiceMonitorSpec{
				Enabled: true,
				Labels:  map[string]string{"prometheus": "platform"},
			},
		},
	}
	return cluster
}

// minimalPreset returns the minimal example of the preset doc with a unique
// name.
func minimalPreset() *v1.CamundaClusterPreset {
	return &v1.CamundaClusterPreset{
		ObjectMeta: metav1.ObjectMeta{Name: "ccp-" + utilrand.String(8)},
		Spec: v1.CamundaClusterPresetSpec{Cluster: v1.CamundaClusterSpec{
			Zeebe: &v1.ZeebeSpec{
				WorkloadSpec:      v1.WorkloadSpec{Replicas: new(int32(1))},
				Partitions:        new(int32(1)),
				ReplicationFactor: new(int32(1)),
				StorageSize:       new(resource.MustParse("10Gi")),
			},
		}},
	}
}

// realisticPreset returns the realistic example of the preset doc with a
// unique name.
func realisticPreset() *v1.CamundaClusterPreset {
	preset := minimalPreset()
	preset.Spec.Cluster = v1.CamundaClusterSpec{
		PodLabels:      map[string]string{"company.com/team": "automation-ops"},
		PodAnnotations: map[string]string{"company.com/cluster-preset": "medium"},
		Zeebe: &v1.ZeebeSpec{
			WorkloadSpec: v1.WorkloadSpec{
				Replicas: new(int32(3)),
				Resources: &corev1.ResourceRequirements{
					Requests: corev1.ResourceList{
						corev1.ResourceCPU:    resource.MustParse("1"),
						corev1.ResourceMemory: resource.MustParse("2Gi"),
					},
					Limits: corev1.ResourceList{
						corev1.ResourceCPU:    resource.MustParse("2"),
						corev1.ResourceMemory: resource.MustParse("4Gi"),
					},
				},
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
				Resources: &corev1.ResourceRequirements{
					Requests: corev1.ResourceList{
						corev1.ResourceCPU:    resource.MustParse("500m"),
						corev1.ResourceMemory: resource.MustParse("1Gi"),
					},
				},
			},
		},
		Operate:  &v1.WebAppSpec{Mode: v1.ComponentModeEmbedded},
		Tasklist: &v1.WebAppSpec{Mode: v1.ComponentModeEmbedded},
		Admin:    &v1.WebAppSpec{Mode: v1.ComponentModeEmbedded},
		Connectors: &v1.ConnectorsSpec{
			Enabled: new(true),
			WorkloadSpec: v1.WorkloadSpec{
				Replicas: new(int32(1)),
				Resources: &corev1.ResourceRequirements{
					Requests: corev1.ResourceList{
						corev1.ResourceCPU:    resource.MustParse("250m"),
						corev1.ResourceMemory: resource.MustParse("512Mi"),
					},
				},
			},
		},
	}
	return preset
}

var _ = Describe("CamundaCluster schema", func() {
	DescribeTable(
		"admission",
		func(build func() *v1.CamundaCluster, mutate func(*v1.CamundaCluster), wantErr string) {
			obj := build()
			mutate(obj)
			err := k8sClient.Create(ctx, obj)
			if wantErr == "" {
				Expect(err).NotTo(HaveOccurred())
				DeferCleanup(func() { _ = k8sClient.Delete(ctx, obj) })
			} else {
				Expect(err).To(MatchError(ContainSubstring(wantErr)))
			}
		},
		Entry("accepts the minimal doc example", minimalCamundaCluster, func(*v1.CamundaCluster) {}, ""),
		Entry("accepts the realistic doc example", realisticCamundaCluster, func(*v1.CamundaCluster) {}, ""),
		Entry(
			"rejects a missing storageRef",
			minimalCamundaCluster, func(o *v1.CamundaCluster) { o.Spec.StorageRef = "" },
			"spec.storageRef is required",
		),
		Entry(
			"rejects a missing platformConfigRef",
			minimalCamundaCluster, func(o *v1.CamundaCluster) { o.Spec.PlatformConfigRef = "" },
			"spec.platformConfigRef is required",
		),
		Entry(
			"rejects a non-DNS-1123 platformConfigRef",
			minimalCamundaCluster, func(o *v1.CamundaCluster) { o.Spec.PlatformConfigRef = fixtures.NotAResourceName },
			"platformConfigRef",
		),
		Entry(
			"rejects a two-segment version",
			minimalCamundaCluster, func(o *v1.CamundaCluster) { o.Spec.Version = "8.9" },
			"version",
		),
		Entry(
			"rejects a replicationFactor above replicas",
			realisticCamundaCluster, func(o *v1.CamundaCluster) { o.Spec.Zeebe.ReplicationFactor = new(int32(6)) },
			"zeebe.replicationFactor must not exceed zeebe.replicas",
		),
		Entry(
			"rejects zero partitions",
			realisticCamundaCluster, func(o *v1.CamundaCluster) { o.Spec.Zeebe.Partitions = new(int32(0)) },
			"partitions",
		),
		Entry(
			"rejects a connectors version that is not x.y.z",
			realisticCamundaCluster, func(o *v1.CamundaCluster) { o.Spec.Connectors.Version = "8.9" },
			"version",
		),
		Entry(
			"rejects an unknown component mode",
			realisticCamundaCluster, func(o *v1.CamundaCluster) {
				o.Spec.Gateway = &v1.GatewaySpec{Mode: "Sideways"}
			},
			"mode",
		),
		Entry(
			"accepts an admin block with all three member kinds",
			minimalCamundaCluster, func(o *v1.CamundaCluster) {
				o.Spec.Auth = &v1.ClusterAuthSpec{Admin: &v1.ClusterAdminSpec{
					Users:   []string{"ada@example.com"},
					Clients: []string{"my-cluster-client"},
					MappingRules: []v1.AdminMappingRule{
						{ID: "platform-admins", ClaimName: "groups", ClaimValue: "camunda-admins"},
					},
				}}
			},
			"",
		),
		Entry(
			"rejects an empty admin user",
			minimalCamundaCluster, func(o *v1.CamundaCluster) {
				o.Spec.Auth = &v1.ClusterAuthSpec{Admin: &v1.ClusterAdminSpec{Users: []string{""}}}
			},
			"users",
		),
		Entry(
			"rejects an empty admin client",
			minimalCamundaCluster, func(o *v1.CamundaCluster) {
				o.Spec.Auth = &v1.ClusterAuthSpec{Admin: &v1.ClusterAdminSpec{Clients: []string{""}}}
			},
			"clients",
		),
		Entry(
			"rejects a mapping rule without a claim value",
			minimalCamundaCluster, func(o *v1.CamundaCluster) {
				o.Spec.Auth = &v1.ClusterAuthSpec{Admin: &v1.ClusterAdminSpec{
					MappingRules: []v1.AdminMappingRule{{ID: "platform-admins", ClaimName: "groups"}},
				}}
			},
			"claimValue",
		),
		Entry(
			"rejects two zeebe extraEnv entries with the same name",
			minimalCamundaCluster, func(o *v1.CamundaCluster) {
				o.Spec.Zeebe = &v1.ZeebeSpec{WorkloadSpec: v1.WorkloadSpec{ExtraEnv: []corev1.EnvVar{
					{Name: "JAVA_TOOL_OPTIONS", Value: "-Xmx4g"},
					{Name: "JAVA_TOOL_OPTIONS", Value: "-Xmx8g"},
				}}}
			},
			"Duplicate value",
		),
		Entry(
			"rejects two top-level extraEnv entries with the same name",
			minimalCamundaCluster, func(o *v1.CamundaCluster) {
				o.Spec.ExtraEnv = []corev1.EnvVar{
					{Name: "LOG_LEVEL", Value: "info"},
					{Name: "LOG_LEVEL", Value: "debug"},
				}
			},
			"Duplicate value",
		),
		Entry(
			"rejects an extraEnv entry that sets both value and valueFrom",
			minimalCamundaCluster, func(o *v1.CamundaCluster) {
				o.Spec.ExtraEnv = []corev1.EnvVar{{
					Name:  "LOG_LEVEL",
					Value: "info",
					ValueFrom: &corev1.EnvVarSource{SecretKeyRef: &corev1.SecretKeySelector{
						LocalObjectReference: corev1.LocalObjectReference{Name: "logging"}, Key: "level",
					}},
				}}
			},
			"value or valueFrom",
		),
		Entry(
			"rejects an unknown retention policy",
			realisticCamundaCluster, func(o *v1.CamundaCluster) {
				o.Spec.Zeebe.PersistentVolumeClaimRetentionPolicy = &v1.PersistentVolumeClaimRetentionPolicy{
					WhenDeleted: "Keep",
				}
			},
			"whenDeleted",
		),
	)

	It("keeps the zeebe extraEnv entries of two field managers side by side", func() {
		obj := minimalCamundaCluster()

		Expect(k8sClient.Create(ctx, obj)).To(Succeed())
		DeferCleanup(func() { _ = k8sClient.Delete(ctx, obj) })

		Expect(applyZeebeEnv(obj, "camunda-operator/first", corev1.EnvVar{Name: "FIRST", Value: "1"})).To(Succeed())
		Expect(applyZeebeEnv(obj, "camunda-operator/second", corev1.EnvVar{Name: "SECOND", Value: "2"})).To(Succeed())

		var latest v1.CamundaCluster
		Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(obj), &latest)).To(Succeed())
		Expect(latest.Spec.Zeebe.ExtraEnv).To(ConsistOf(
			corev1.EnvVar{Name: "FIRST", Value: "1"},
			corev1.EnvVar{Name: "SECOND", Value: "2"},
		))
	})

	// Two managers that apply the same name merge field by field inside the
	// entry, so one can own value while the other owns valueFrom. A container
	// rejects an entry that carries both, so the schema rule must catch the
	// combination at the CamundaCluster rather than at the rendered pod.
	It("rejects a second field manager that adds valueFrom to an entry that already has a value", func() {
		obj := minimalCamundaCluster()

		Expect(k8sClient.Create(ctx, obj)).To(Succeed())
		DeferCleanup(func() { _ = k8sClient.Delete(ctx, obj) })

		Expect(applyZeebeEnv(obj, "e2e-user", corev1.EnvVar{Name: "SHARED", Value: "plain"})).To(Succeed())

		err := applyZeebeEnv(obj, "camunda-operator/extension", corev1.EnvVar{
			Name: "SHARED",
			ValueFrom: &corev1.EnvVarSource{SecretKeyRef: &corev1.SecretKeySelector{
				LocalObjectReference: corev1.LocalObjectReference{Name: "creds"}, Key: "password",
			}},
		})
		Expect(err).To(MatchError(ContainSubstring("value or valueFrom")))
	})

	It("rejects a partitions decrease on update and accepts an increase", func() {
		obj := realisticCamundaCluster()

		Expect(k8sClient.Create(ctx, obj)).To(Succeed())
		DeferCleanup(func() { _ = k8sClient.Delete(ctx, obj) })

		Expect(
			updateSpec(obj, func(c *v1.CamundaCluster) { c.Spec.Zeebe.Partitions = new(int32(2)) }),
		).To(MatchError(ContainSubstring("zeebe.partitions cannot be decreased or removed once set")))

		Expect(updateSpec(obj, func(c *v1.CamundaCluster) { c.Spec.Zeebe.Partitions = new(int32(4)) })).To(Succeed())
	})

	It("rejects removing partitions once set", func() {
		obj := realisticCamundaCluster()

		Expect(k8sClient.Create(ctx, obj)).To(Succeed())
		DeferCleanup(func() { _ = k8sClient.Delete(ctx, obj) })

		Expect(
			updateSpec(obj, func(c *v1.CamundaCluster) { c.Spec.Zeebe.Partitions = nil }),
		).To(MatchError(ContainSubstring("zeebe.partitions cannot be decreased or removed once set")))
	})

	It("rejects a storageClassName change on update", func() {
		obj := realisticCamundaCluster()

		Expect(k8sClient.Create(ctx, obj)).To(Succeed())
		DeferCleanup(func() { _ = k8sClient.Delete(ctx, obj) })

		Expect(
			updateSpec(obj, func(c *v1.CamundaCluster) { c.Spec.Zeebe.StorageClassName = new("hdd") }),
		).To(MatchError(ContainSubstring("zeebe.storageClassName is immutable")))

		Expect(
			updateSpec(obj, func(c *v1.CamundaCluster) { c.Spec.Zeebe.StorageClassName = nil }),
		).To(MatchError(ContainSubstring("zeebe.storageClassName is immutable")))
	})

	It("rejects a storageSize shrink on update and accepts growth", func() {
		obj := realisticCamundaCluster()

		Expect(k8sClient.Create(ctx, obj)).To(Succeed())
		DeferCleanup(func() { _ = k8sClient.Delete(ctx, obj) })

		Expect(
			updateSpec(obj, func(c *v1.CamundaCluster) { c.Spec.Zeebe.StorageSize = new(resource.MustParse("16Gi")) }),
		).To(MatchError(ContainSubstring("zeebe.storageSize cannot be shrunk")))

		Expect(
			updateSpec(obj, func(c *v1.CamundaCluster) { c.Spec.Zeebe.StorageSize = new(resource.MustParse("64Gi")) }),
		).To(Succeed())
	})
})

// minimalRelease returns the minimal example of the release doc with a
// unique name.
func minimalRelease(version string) *v1.CamundaRelease {
	return &v1.CamundaRelease{
		ObjectMeta: metav1.ObjectMeta{Name: "cr-" + utilrand.String(8)},
		Spec:       v1.CamundaReleaseSpec{Version: version},
	}
}

var _ = Describe("CamundaRelease schema", func() {
	DescribeTable(
		"admission",
		func(mutate func(*v1.CamundaRelease), wantErr string) {
			obj := minimalRelease("8.9.4")
			mutate(obj)
			err := k8sClient.Create(ctx, obj)
			if wantErr == "" {
				Expect(err).NotTo(HaveOccurred())
				DeferCleanup(func() { _ = k8sClient.Delete(ctx, obj) })
			} else {
				Expect(err).To(MatchError(ContainSubstring(wantErr)))
			}
		},
		Entry("accepts the minimal doc example", func(*v1.CamundaRelease) {}, ""),
		Entry(
			"accepts pinned images, per-component env, and a connectors version",
			func(o *v1.CamundaRelease) {
				o.Spec.Connectors = &v1.ReleaseConnectorsSpec{Version: "8.9.7"}
				o.Spec.Images = &v1.ReleaseImagesSpec{
					Camunda: "mirror.example.com/camunda/camunda@sha256:" +
						"7d865e959b2466918c9863afca942d0fb89d7c9ac0c99bafc3749504ded97730",
				}
				o.Spec.Zeebe = &v1.ReleaseEnvSpec{
					ExtraEnv: []corev1.EnvVar{{Name: "JAVA_OPTS", Value: "-Xmx8g"}},
				}
			},
			"",
		),
		Entry(
			"rejects a missing version",
			func(o *v1.CamundaRelease) { o.Spec.Version = "" },
			"spec.version",
		),
		Entry(
			"rejects a version that is not x.y.z",
			func(o *v1.CamundaRelease) { o.Spec.Version = "8.9" },
			"spec.version",
		),
		Entry(
			"rejects a connectors version that is not x.y.z",
			func(o *v1.CamundaRelease) {
				o.Spec.Connectors = &v1.ReleaseConnectorsSpec{Version: "latest"}
			},
			"spec.connectors.version",
		),
		Entry(
			"rejects an extraEnv entry with value and valueFrom",
			func(o *v1.CamundaRelease) {
				o.Spec.ExtraEnv = []corev1.EnvVar{{
					Name:      "BOTH",
					Value:     "x",
					ValueFrom: &corev1.EnvVarSource{},
				}}
			},
			"value or valueFrom",
		),
	)
})

var _ = Describe("CamundaClusterPreset schema", func() {
	const instanceBound = "instance-bound fields"

	DescribeTable(
		"admission",
		func(build func() *v1.CamundaClusterPreset, mutate func(*v1.CamundaClusterPreset), wantErr string) {
			obj := build()
			mutate(obj)
			err := k8sClient.Create(ctx, obj)
			if wantErr == "" {
				Expect(err).NotTo(HaveOccurred())
				DeferCleanup(func() { _ = k8sClient.Delete(ctx, obj) })
			} else {
				Expect(err).To(MatchError(ContainSubstring(wantErr)))
			}
		},
		Entry("accepts the minimal doc example", minimalPreset, func(*v1.CamundaClusterPreset) {}, ""),
		Entry("accepts the realistic doc example", realisticPreset, func(*v1.CamundaClusterPreset) {}, ""),
		Entry(
			"rejects a storageRef",
			minimalPreset, func(o *v1.CamundaClusterPreset) { o.Spec.Cluster.StorageRef = "my-storage-config" },
			instanceBound,
		),
		Entry(
			"rejects a platformConfigRef",
			minimalPreset, func(o *v1.CamundaClusterPreset) { o.Spec.Cluster.PlatformConfigRef = "my-platform-config" },
			instanceBound,
		),
		Entry(
			"rejects a presetRef",
			minimalPreset, func(o *v1.CamundaClusterPreset) { o.Spec.Cluster.PresetRef = "small" },
			instanceBound,
		),
		Entry(
			"rejects suspend: true",
			minimalPreset, func(o *v1.CamundaClusterPreset) { o.Spec.Cluster.Suspend = true },
			instanceBound,
		),
		Entry(
			"rejects pause: true",
			minimalPreset, func(o *v1.CamundaClusterPreset) { o.Spec.Cluster.Pause = true },
			instanceBound,
		),
		Entry(
			"rejects a monitoring block",
			minimalPreset, func(o *v1.CamundaClusterPreset) {
				o.Spec.Cluster.Monitoring = &v1.ClusterMonitoringSpec{}
			},
			instanceBound,
		),
		Entry(
			"rejects a serviceAccount block",
			minimalPreset, func(o *v1.CamundaClusterPreset) {
				o.Spec.Cluster.ServiceAccount = &v1.ServiceAccountSpec{}
			},
			instanceBound,
		),
		Entry(
			"rejects an externalUrl",
			minimalPreset, func(o *v1.CamundaClusterPreset) { o.Spec.Cluster.ExternalURL = "https://example.com" },
			instanceBound,
		),
		Entry(
			"rejects a releaseRef",
			minimalPreset, func(o *v1.CamundaClusterPreset) { o.Spec.Cluster.ReleaseRef = "camunda-8-9-4" },
			instanceBound,
		),
		Entry(
			"rejects a version",
			minimalPreset, func(o *v1.CamundaClusterPreset) { o.Spec.Cluster.Version = "8.9.0" },
			"belong to a CamundaRelease",
		),
		Entry(
			"rejects a connectors.version",
			minimalPreset, func(o *v1.CamundaClusterPreset) {
				o.Spec.Cluster.Connectors = &v1.ConnectorsSpec{Version: "8.9.7"}
			},
			"belong to a CamundaRelease",
		),
	)

	It("accepts a connectors block without a version", func() {
		obj := minimalPreset()
		obj.Spec.Cluster.Connectors = &v1.ConnectorsSpec{Enabled: new(true)}
		Expect(k8sClient.Create(ctx, obj)).To(Succeed())
		DeferCleanup(func() { _ = k8sClient.Delete(ctx, obj) })
	})

	It("does not bind the transition rules of the cluster to a preset", func() {
		obj := realisticPreset()

		Expect(k8sClient.Create(ctx, obj)).To(Succeed())
		DeferCleanup(func() { _ = k8sClient.Delete(ctx, obj) })

		obj.Spec.Cluster.Zeebe.Partitions = new(int32(2))
		obj.Spec.Cluster.Zeebe.StorageClassName = new("hdd")
		obj.Spec.Cluster.Zeebe.StorageSize = new(resource.MustParse("16Gi"))
		Expect(k8sClient.Update(ctx, obj)).To(Succeed())
	})
})
