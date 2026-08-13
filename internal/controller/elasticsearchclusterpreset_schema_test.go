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

// validElasticsearchClusterPreset returns the doc's minimal example with a
// unique name.
func validElasticsearchClusterPreset() *v1.ElasticsearchClusterPreset {
	replicas := int32(3)
	storageSize := resource.MustParse("64Gi")

	return &v1.ElasticsearchClusterPreset{
		ObjectMeta: metav1.ObjectMeta{Name: "escp-" + utilrand.String(8)},
		Spec: v1.ElasticsearchClusterPresetSpec{
			Cluster: v1.ElasticsearchClusterSpec{
				Version:     "9.2.4",
				Replicas:    &replicas,
				StorageSize: &storageSize,
			},
		},
	}
}

// realisticElasticsearchClusterPreset returns the doc's realistic example with
// a unique name.
func realisticElasticsearchClusterPreset() *v1.ElasticsearchClusterPreset {
	preset := validElasticsearchClusterPreset()
	replicas := int32(5)
	storageSize := resource.MustParse("256Gi")
	storageClassName := "ssd"

	preset.Spec.Cluster.Replicas = &replicas
	preset.Spec.Cluster.StorageSize = &storageSize
	preset.Spec.Cluster.StorageClassName = &storageClassName
	preset.Spec.Cluster.Resources = &corev1.ResourceRequirements{
		Requests: corev1.ResourceList{
			corev1.ResourceCPU:    resource.MustParse("4"),
			corev1.ResourceMemory: resource.MustParse("8Gi"),
		},
		Limits: corev1.ResourceList{
			corev1.ResourceMemory: resource.MustParse("8Gi"),
		},
	}
	preset.Spec.Cluster.Scheduling = &v1.SchedulingSpec{
		Tolerations: []corev1.Toleration{{
			Key:      "dedicated",
			Operator: corev1.TolerationOpEqual,
			Value:    "elasticsearch",
			Effect:   corev1.TaintEffectNoSchedule,
		}},
	}

	return preset
}

var _ = Describe("ElasticsearchClusterPreset schema", func() {
	DescribeTable("admission",
		func(build func() *v1.ElasticsearchClusterPreset, mutate func(*v1.ElasticsearchClusterPreset), wantErr string) {
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
		Entry("accepts the minimal doc example",
			validElasticsearchClusterPreset, func(*v1.ElasticsearchClusterPreset) {}, ""),
		Entry("accepts the realistic doc example",
			realisticElasticsearchClusterPreset, func(*v1.ElasticsearchClusterPreset) {}, ""),
		Entry("rejects presetRef inside a preset",
			validElasticsearchClusterPreset, func(o *v1.ElasticsearchClusterPreset) {
				o.Spec.Cluster.PresetRef = "other"
			}, "instance-bound"),
		Entry("rejects secondaryStorageConfig inside a preset",
			validElasticsearchClusterPreset, func(o *v1.ElasticsearchClusterPreset) {
				o.Spec.Cluster.SecondaryStorageConfig = "my-storage-config"
			}, "instance-bound"),
		Entry("rejects suspend inside a preset",
			validElasticsearchClusterPreset, func(o *v1.ElasticsearchClusterPreset) {
				o.Spec.Cluster.Suspend = true
			}, "instance-bound"),
		Entry("rejects monitoring inside a preset",
			validElasticsearchClusterPreset, func(o *v1.ElasticsearchClusterPreset) {
				o.Spec.Cluster.Monitoring = &v1.MonitoringSpec{
					ServiceMonitor: &v1.ServiceMonitorSpec{Enabled: true},
				}
			}, "instance-bound"),
	)
})
