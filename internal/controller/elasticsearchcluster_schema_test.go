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
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	utilrand "k8s.io/apimachinery/pkg/util/rand"

	v1 "github.com/konsole-is/camunda-operator/api/v1"
)

// validElasticsearchCluster returns the doc's minimal example with a unique
// name; the caller chooses the namespace.
func validElasticsearchCluster() *v1.ElasticsearchCluster {
	return &v1.ElasticsearchCluster{
		ObjectMeta: metav1.ObjectMeta{Name: "esc-" + utilrand.String(8)},
		Spec: v1.ElasticsearchClusterSpec{
			PresetRef:              "standard",
			SecondaryStorageConfig: "my-storage-config",
		},
	}
}

// realisticElasticsearchCluster returns the doc's realistic example with a
// unique name; the caller chooses the namespace.
func realisticElasticsearchCluster() *v1.ElasticsearchCluster {
	replicas := int32(3)
	storageSize := resource.MustParse("128Gi")
	storageClassName := "ssd"

	return &v1.ElasticsearchCluster{
		ObjectMeta: metav1.ObjectMeta{Name: "esc-" + utilrand.String(8)},
		Spec: v1.ElasticsearchClusterSpec{
			Version:  "9.2.4",
			Replicas: &replicas,
			Resources: &corev1.ResourceRequirements{
				Requests: corev1.ResourceList{
					corev1.ResourceCPU:    resource.MustParse("2"),
					corev1.ResourceMemory: resource.MustParse("4Gi"),
				},
				Limits: corev1.ResourceList{
					corev1.ResourceMemory: resource.MustParse("4Gi"),
				},
			},
			StorageSize:      &storageSize,
			StorageClassName: &storageClassName,
			PodLabels:        map[string]string{"team": "platform"},
			Scheduling: &v1.SchedulingSpec{
				Tolerations: []corev1.Toleration{{
					Key:      "dedicated",
					Operator: corev1.TolerationOpEqual,
					Value:    "elasticsearch",
					Effect:   corev1.TaintEffectNoSchedule,
				}},
			},
			SecondaryStorageConfig: "my-storage-config",
			Monitoring: &v1.MonitoringSpec{
				ServiceMonitor: &v1.ServiceMonitorSpec{
					Enabled: true,
					Labels:  map[string]string{"release": "prometheus"},
				},
			},
		},
	}
}

var _ = Describe("ElasticsearchCluster schema", func() {
	DescribeTable("admission",
		func(build func() *v1.ElasticsearchCluster, mutate func(*v1.ElasticsearchCluster), wantErr string) {
			obj := build()
			obj.Namespace = schemaTestNamespace
			mutate(obj)
			err := k8sClient.Create(ctx, obj)
			if wantErr == "" {
				Expect(err).NotTo(HaveOccurred())
				DeferCleanup(func() { _ = k8sClient.Delete(ctx, obj) })
			} else {
				Expect(err).To(MatchError(ContainSubstring(wantErr)))
			}
		},
		Entry("accepts the minimal doc example",
			validElasticsearchCluster, func(*v1.ElasticsearchCluster) {}, ""),
		Entry("accepts the realistic doc example",
			realisticElasticsearchCluster, func(*v1.ElasticsearchCluster) {}, ""),
		Entry("rejects a missing secondaryStorageConfig",
			validElasticsearchCluster, func(o *v1.ElasticsearchCluster) {
				o.Spec.SecondaryStorageConfig = ""
			}, "secondaryStorageConfig"),
		Entry("rejects a non-DNS-1123 secondaryStorageConfig",
			validElasticsearchCluster, func(o *v1.ElasticsearchCluster) {
				o.Spec.SecondaryStorageConfig = "Not_A_Name"
			}, "secondaryStorageConfig"),
		Entry("rejects a two-segment version",
			realisticElasticsearchCluster, func(o *v1.ElasticsearchCluster) {
				o.Spec.Version = "9.2"
			}, "version"),
		Entry("rejects a v-prefixed version",
			realisticElasticsearchCluster, func(o *v1.ElasticsearchCluster) {
				o.Spec.Version = "v9.2.4"
			}, "version"),
		Entry("rejects zero replicas",
			realisticElasticsearchCluster, func(o *v1.ElasticsearchCluster) {
				zero := int32(0)
				o.Spec.Replicas = &zero
			}, "replicas"),
	)

	It("rejects shrinking storageSize on update and accepts growth", func() {
		obj := realisticElasticsearchCluster()
		obj.Namespace = schemaTestNamespace

		Expect(k8sClient.Create(ctx, obj)).To(Succeed())
		DeferCleanup(func() { _ = k8sClient.Delete(ctx, obj) })

		smaller := resource.MustParse("64Gi")
		obj.Spec.StorageSize = &smaller
		Expect(k8sClient.Update(ctx, obj)).To(MatchError(ContainSubstring("storageSize")))

		larger := resource.MustParse("256Gi")
		obj.Spec.StorageSize = &larger
		Expect(k8sClient.Update(ctx, obj)).To(Succeed())
	})
})
