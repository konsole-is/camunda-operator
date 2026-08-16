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

	v1 "github.com/konsole-is/camunda-operator/api/v1"
	"github.com/konsole-is/camunda-operator/internal/fixtures"
)

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
			Version: "8.9.0",
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
		Version:        "8.9.0",
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
		Identity: &v1.WebAppSpec{Mode: v1.ComponentModeEmbedded},
		Connectors: &v1.ConnectorsSpec{
			Enabled: new(true),
			Version: "8.9.7",
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
			"rejects an unknown retention policy",
			realisticCamundaCluster, func(o *v1.CamundaCluster) {
				o.Spec.Zeebe.PersistentVolumeClaimRetentionPolicy = &v1.PersistentVolumeClaimRetentionPolicy{
					WhenDeleted: "Keep",
				}
			},
			"whenDeleted",
		),
	)

	It("rejects a partitions decrease on update and accepts an increase", func() {
		obj := realisticCamundaCluster()

		Expect(k8sClient.Create(ctx, obj)).To(Succeed())
		DeferCleanup(func() { _ = k8sClient.Delete(ctx, obj) })

		obj.Spec.Zeebe.Partitions = new(int32(2))
		Expect(
			k8sClient.Update(ctx, obj),
		).To(MatchError(ContainSubstring("zeebe.partitions cannot be decreased or removed once set")))

		obj.Spec.Zeebe.Partitions = new(int32(4))
		Expect(k8sClient.Update(ctx, obj)).To(Succeed())
	})

	It("rejects removing partitions once set", func() {
		obj := realisticCamundaCluster()

		Expect(k8sClient.Create(ctx, obj)).To(Succeed())
		DeferCleanup(func() { _ = k8sClient.Delete(ctx, obj) })

		obj.Spec.Zeebe.Partitions = nil
		Expect(
			k8sClient.Update(ctx, obj),
		).To(MatchError(ContainSubstring("zeebe.partitions cannot be decreased or removed once set")))
	})

	It("rejects a storageClassName change on update", func() {
		obj := realisticCamundaCluster()

		Expect(k8sClient.Create(ctx, obj)).To(Succeed())
		DeferCleanup(func() { _ = k8sClient.Delete(ctx, obj) })

		obj.Spec.Zeebe.StorageClassName = new("hdd")
		Expect(k8sClient.Update(ctx, obj)).To(MatchError(ContainSubstring("zeebe.storageClassName is immutable")))

		obj.Spec.Zeebe.StorageClassName = nil
		Expect(k8sClient.Update(ctx, obj)).To(MatchError(ContainSubstring("zeebe.storageClassName is immutable")))
	})

	It("rejects a storageSize shrink on update and accepts growth", func() {
		obj := realisticCamundaCluster()

		Expect(k8sClient.Create(ctx, obj)).To(Succeed())
		DeferCleanup(func() { _ = k8sClient.Delete(ctx, obj) })

		obj.Spec.Zeebe.StorageSize = new(resource.MustParse("16Gi"))
		Expect(k8sClient.Update(ctx, obj)).To(MatchError(ContainSubstring("zeebe.storageSize cannot be shrunk")))

		obj.Spec.Zeebe.StorageSize = new(resource.MustParse("64Gi"))
		Expect(k8sClient.Update(ctx, obj)).To(Succeed())
	})
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
	)

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
