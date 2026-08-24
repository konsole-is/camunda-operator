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
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	utilrand "k8s.io/apimachinery/pkg/util/rand"

	v1 "github.com/konsole-is/camunda-operator/api/v1"
)

// validDatabaseServerPreset returns the minimal example of the CRD doc with a
// unique name.
func validDatabaseServerPreset() *v1.DatabaseServerPreset {
	return &v1.DatabaseServerPreset{
		ObjectMeta: metav1.ObjectMeta{Name: "dbsp-" + utilrand.String(8)},
		Spec: v1.DatabaseServerPresetSpec{
			Server: v1.DatabaseServerSpec{
				Version:     "17",
				Instances:   new(int32(2)),
				StorageSize: new(resource.MustParse("20Gi")),
			},
		},
	}
}

// realisticDatabaseServerPreset returns the realistic example of the CRD doc
// with a unique name.
func realisticDatabaseServerPreset() *v1.DatabaseServerPreset {
	preset := validDatabaseServerPreset()

	preset.Spec.Server.Instances = new(int32(3))
	preset.Spec.Server.StorageSize = new(resource.MustParse("256Gi"))
	preset.Spec.Server.StorageClassName = new("ssd")
	preset.Spec.Server.WALStorageSize = new(resource.MustParse("32Gi"))
	preset.Spec.Server.Resources = &corev1.ResourceRequirements{
		Requests: corev1.ResourceList{
			corev1.ResourceCPU:    resource.MustParse("4"),
			corev1.ResourceMemory: resource.MustParse("8Gi"),
		},
		Limits: corev1.ResourceList{
			corev1.ResourceMemory: resource.MustParse("8Gi"),
		},
	}
	preset.Spec.Server.Scheduling = &v1.SchedulingSpec{
		Tolerations: []corev1.Toleration{{
			Key:      "dedicated",
			Operator: corev1.TolerationOpEqual,
			Value:    "postgres",
			Effect:   corev1.TaintEffectNoSchedule,
		}},
	}
	preset.Spec.Server.Monitoring = &v1.DatabaseServerMonitoringSpec{
		PodMonitor: &v1.PodMonitorSpec{Enabled: true},
	}
	preset.Spec.Server.Archive = &v1.DatabaseServerArchiveSpec{
		ObjectStorageRef:    "my-backup-config",
		RetentionPeriodDays: 30,
	}

	return preset
}

var _ = Describe("DatabaseServerPreset schema", func() {
	DescribeTable(
		"admission",
		func(build func() *v1.DatabaseServerPreset, mutate func(*v1.DatabaseServerPreset), wantErr string) {
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
		Entry(
			"accepts the minimal doc example",
			validDatabaseServerPreset, func(*v1.DatabaseServerPreset) {}, "",
		),
		Entry(
			"accepts the realistic doc example",
			realisticDatabaseServerPreset, func(*v1.DatabaseServerPreset) {}, "",
		),
		Entry(
			"rejects presetRef inside a preset",
			validDatabaseServerPreset, func(o *v1.DatabaseServerPreset) {
				o.Spec.Server.PresetRef = "other"
			}, "instance-bound",
		),
		Entry(
			"rejects databaseServerConfig inside a preset",
			validDatabaseServerPreset, func(o *v1.DatabaseServerPreset) {
				o.Spec.Server.DatabaseServerConfig = "my-database-server"
			}, "instance-bound",
		),
		Entry(
			"rejects suspend inside a preset",
			validDatabaseServerPreset, func(o *v1.DatabaseServerPreset) {
				o.Spec.Server.Suspend = true
			}, "instance-bound",
		),
	)

	// The no-shrink ratchet binds DatabaseServer only. A preset is passive
	// data, and its baseline can be resized freely.
	It("allows lowering a preset's storageSize baseline", func() {
		preset := realisticDatabaseServerPreset()

		Expect(k8sClient.Create(ctx, preset)).To(Succeed())
		DeferCleanup(func() { _ = k8sClient.Delete(ctx, preset) })

		preset.Spec.Server.StorageSize = new(resource.MustParse("20Gi"))
		Expect(k8sClient.Update(ctx, preset)).To(Succeed())
	})

	// Templated YAML renders unset fields as explicit zero values. The
	// instance-bound deny list must not trip on them.
	It("tolerates explicit zero values for instance-bound fields", func() {
		obj := &unstructured.Unstructured{Object: map[string]any{
			"apiVersion": "core.camunda.io/v1",
			"kind":       "DatabaseServerPreset",
			"metadata":   map[string]any{"name": "dbsp-" + utilrand.String(8)},
			"spec": map[string]any{
				"server": map[string]any{
					"version":     "17",
					"instances":   int64(2),
					"storageSize": "20Gi",
					"presetRef":   "",
					"suspend":     false,
					"monitoring":  map[string]any{},
				},
			},
		}}

		Expect(k8sClient.Create(ctx, obj)).To(Succeed())
		DeferCleanup(func() { _ = k8sClient.Delete(ctx, obj) })
	})

	It("rejects an empty-string databaseServerConfig by the name pattern", func() {
		obj := &unstructured.Unstructured{Object: map[string]any{
			"apiVersion": "core.camunda.io/v1",
			"kind":       "DatabaseServerPreset",
			"metadata":   map[string]any{"name": "dbsp-" + utilrand.String(8)},
			"spec": map[string]any{
				"server": map[string]any{
					"version":              "17",
					"instances":            int64(2),
					"storageSize":          "20Gi",
					"databaseServerConfig": "",
				},
			},
		}}

		Expect(k8sClient.Create(ctx, obj)).To(MatchError(ContainSubstring("should match")))
	})
})
