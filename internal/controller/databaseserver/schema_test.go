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
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	utilrand "k8s.io/apimachinery/pkg/util/rand"
	"sigs.k8s.io/controller-runtime/pkg/client"

	v1 "github.com/konsole-is/camunda-operator/api/v1"
	"github.com/konsole-is/camunda-operator/internal/fixtures"
	components "github.com/konsole-is/camunda-operator/pkg/components/databaseserver"
)

// validDatabaseServer returns the minimal example of the CRD doc with a
// unique name. The caller chooses the namespace.
func validDatabaseServer() *v1.DatabaseServer {
	return &v1.DatabaseServer{
		ObjectMeta: metav1.ObjectMeta{Name: "dbs-" + utilrand.String(8)},
		Spec: v1.DatabaseServerSpec{
			PresetRef:            "standard",
			ReleaseRef:           "camunda-8-9-4",
			DatabaseServerConfig: "my-database-server",
		},
	}
}

// realisticDatabaseServer returns the realistic example of the CRD doc with a
// unique name. The caller chooses the namespace.
func realisticDatabaseServer() *v1.DatabaseServer {
	return &v1.DatabaseServer{
		ObjectMeta: metav1.ObjectMeta{Name: "dbs-" + utilrand.String(8)},
		Spec: v1.DatabaseServerSpec{
			Version:   "17",
			Instances: new(int32(2)),
			Resources: &corev1.ResourceRequirements{
				Requests: corev1.ResourceList{
					corev1.ResourceCPU:    resource.MustParse("2"),
					corev1.ResourceMemory: resource.MustParse("4Gi"),
				},
				Limits: corev1.ResourceList{
					corev1.ResourceMemory: resource.MustParse("4Gi"),
				},
			},
			StorageSize:      new(resource.MustParse("128Gi")),
			StorageClassName: new("ssd"),
			WALStorageSize:   new(resource.MustParse("16Gi")),
			ServiceAccount: &v1.DatabaseServerServiceAccountSpec{
				Annotations: map[string]string{
					"eks.amazonaws.com/role-arn": "arn:aws:iam::123456789012:role/my-archive-role",
				},
			},
			Scheduling: &v1.SchedulingSpec{
				Tolerations: []corev1.Toleration{{
					Key:      "dedicated",
					Operator: corev1.TolerationOpEqual,
					Value:    "postgres",
					Effect:   corev1.TaintEffectNoSchedule,
				}},
			},
			PodLabels:      map[string]string{"team": "platform"},
			PodAnnotations: map[string]string{"prometheus.io/scrape": "true"},
			Monitoring: &v1.DatabaseServerMonitoringSpec{
				PodMonitor: &v1.PodMonitorSpec{
					Enabled:  true,
					Labels:   map[string]string{"release": "prometheus"},
					Interval: "30s",
				},
			},
			DatabaseServerConfig: "my-database-server",
			Archive: &v1.DatabaseServerArchiveSpec{
				ObjectStorageRef:    "my-backup-config",
				RetentionPeriodDays: 30,
				BaseBackupSchedule:  "0 0 2 * * *",
			},
		},
	}
}

// createdDatabaseServer creates the realistic example in the schema test
// namespace and returns it.
func createdDatabaseServer() *v1.DatabaseServer {
	GinkgoHelper()

	obj := realisticDatabaseServer()
	obj.Namespace = fixtures.SchemaTestNamespace

	Expect(k8sClient.Create(ctx, obj)).To(Succeed())
	DeferCleanup(func() { _ = k8sClient.Delete(ctx, obj) })

	return obj
}

// resize applies mutate to the latest revision of obj and returns what
// admission answered. The controller writes the status of the same object, so
// a conflict is retried: without that the update fails on the stale revision
// rather than on the rule under test.
func resize(obj *v1.DatabaseServer, mutate func(*v1.DatabaseServer)) error {
	GinkgoHelper()

	var err error
	Eventually(func() bool {
		var latest v1.DatabaseServer
		Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(obj), &latest)).To(Succeed())
		mutate(&latest)
		err = k8sClient.Update(ctx, &latest)

		return !apierrors.IsConflict(err)
	}, timeout, interval).Should(BeTrue(), "the update never got past a conflict")

	return err
}

var _ = Describe("DatabaseServer schema", func() {
	DescribeTable(
		"admission",
		func(build func() *v1.DatabaseServer, mutate func(*v1.DatabaseServer), wantErr string) {
			obj := build()
			obj.Namespace = fixtures.SchemaTestNamespace
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
			validDatabaseServer, func(*v1.DatabaseServer) {}, "",
		),
		Entry(
			"accepts the realistic doc example",
			realisticDatabaseServer, func(*v1.DatabaseServer) {}, "",
		),
		Entry(
			"rejects a missing databaseServerConfig",
			validDatabaseServer, func(o *v1.DatabaseServer) {
				o.Spec.DatabaseServerConfig = ""
			}, "databaseServerConfig",
		),
		Entry(
			"rejects a non-DNS-1123 databaseServerConfig",
			validDatabaseServer, func(o *v1.DatabaseServer) {
				o.Spec.DatabaseServerConfig = fixtures.NotAResourceName
			}, "databaseServerConfig",
		),
		Entry(
			"rejects a dotted version",
			realisticDatabaseServer, func(o *v1.DatabaseServer) {
				o.Spec.Version = "17.2"
			}, "version",
		),
		Entry(
			"rejects a v-prefixed version",
			realisticDatabaseServer, func(o *v1.DatabaseServer) {
				o.Spec.Version = "v17"
			}, "version",
		),
		Entry(
			"rejects zero instances",
			realisticDatabaseServer, func(o *v1.DatabaseServer) {
				o.Spec.Instances = new(int32(0))
			}, "instances",
		),
		Entry(
			"rejects a retention period of zero days",
			realisticDatabaseServer, func(o *v1.DatabaseServer) {
				o.Spec.Archive.RetentionPeriodDays = 0
			}, "retentionPeriodDays",
		),
		// The operator counts the reachable window in nanoseconds, which holds
		// 106751 days. A longer period overflows that count and puts the
		// oldest reachable point in the future, so every rollback request
		// reads as unreachable.
		Entry(
			"accepts a retention period of a hundred years",
			realisticDatabaseServer, func(o *v1.DatabaseServer) {
				o.Spec.Archive.RetentionPeriodDays = 36500
			}, "",
		),
		Entry(
			"rejects a retention period above a hundred years",
			realisticDatabaseServer, func(o *v1.DatabaseServer) {
				o.Spec.Archive.RetentionPeriodDays = 36501
			}, "retentionPeriodDays",
		),
		Entry(
			"rejects an archive without an objectStorageRef",
			realisticDatabaseServer, func(o *v1.DatabaseServer) {
				o.Spec.Archive.ObjectStorageRef = ""
			}, "objectStorageRef",
		),
		Entry(
			"accepts the documented default baseBackupSchedule",
			realisticDatabaseServer, func(o *v1.DatabaseServer) {
				o.Spec.Archive.BaseBackupSchedule = components.DefaultBaseBackupSchedule
			}, "",
		),
		Entry(
			"accepts a step in baseBackupSchedule",
			realisticDatabaseServer, func(o *v1.DatabaseServer) {
				o.Spec.Archive.BaseBackupSchedule = "0 */15 * * * *"
			}, "",
		),
		Entry(
			"accepts a named weekday in baseBackupSchedule",
			realisticDatabaseServer, func(o *v1.DatabaseServer) {
				o.Spec.Archive.BaseBackupSchedule = "0 0 3 * * SUN"
			}, "",
		),
		// CloudNativePG reads the first field as seconds, so the five-field
		// cron of a Kubernetes CronJob runs at a different time from the one
		// its author meant.
		Entry(
			"rejects a five-field baseBackupSchedule",
			realisticDatabaseServer, func(o *v1.DatabaseServer) {
				o.Spec.Archive.BaseBackupSchedule = "0 2 * * *"
			}, "baseBackupSchedule",
		),
		Entry(
			"rejects a word in baseBackupSchedule",
			realisticDatabaseServer, func(o *v1.DatabaseServer) {
				o.Spec.Archive.BaseBackupSchedule = "0 0 2 * * daily"
			}, "baseBackupSchedule",
		),
		Entry(
			"rejects a seven-field baseBackupSchedule",
			realisticDatabaseServer, func(o *v1.DatabaseServer) {
				o.Spec.Archive.BaseBackupSchedule = "0 0 2 * * * 2026"
			}, "baseBackupSchedule",
		),
		Entry(
			"accepts a descriptor as baseBackupSchedule",
			realisticDatabaseServer, func(o *v1.DatabaseServer) {
				o.Spec.Archive.BaseBackupSchedule = "@daily"
			}, "",
		),
		Entry(
			"accepts a baseBackupSchedule that a manifest indented",
			realisticDatabaseServer, func(o *v1.DatabaseServer) {
				o.Spec.Archive.BaseBackupSchedule = "  0 0 2 * * *  "
			}, "",
		),
		// Each field is bounded to what CloudNativePG takes there, so a value
		// that reads plausibly but names no hour, no weekday, or no second is
		// refused rather than left to stop the base backups.
		Entry(
			"rejects an hour of 24 in baseBackupSchedule",
			realisticDatabaseServer, func(o *v1.DatabaseServer) {
				o.Spec.Archive.BaseBackupSchedule = "0 0 24 * * *"
			}, "baseBackupSchedule",
		),
		Entry(
			"rejects a weekday of 7 in baseBackupSchedule",
			realisticDatabaseServer, func(o *v1.DatabaseServer) {
				o.Spec.Archive.BaseBackupSchedule = "0 0 2 * * 7"
			}, "baseBackupSchedule",
		),
		Entry(
			"rejects a month name in the seconds field of baseBackupSchedule",
			realisticDatabaseServer, func(o *v1.DatabaseServer) {
				o.Spec.Archive.BaseBackupSchedule = "JAN 0 0 * * *"
			}, "baseBackupSchedule",
		),
		// A number long enough to overflow the parser of CloudNativePG stops
		// the base backups, which is what the bounds on the field exist to
		// prevent. The numbers a schedule is written with are far shorter.
		Entry(
			"rejects an @every interval of twenty-four digits in baseBackupSchedule",
			realisticDatabaseServer, func(o *v1.DatabaseServer) {
				o.Spec.Archive.BaseBackupSchedule = "@every 999999999999999999999999h"
			}, "baseBackupSchedule",
		),
		Entry(
			"rejects a step of twenty digits in baseBackupSchedule",
			realisticDatabaseServer, func(o *v1.DatabaseServer) {
				o.Spec.Archive.BaseBackupSchedule = "0 0 2 * * */99999999999999999999"
			}, "baseBackupSchedule",
		),
		Entry(
			"accepts a fractional @every interval as baseBackupSchedule",
			realisticDatabaseServer, func(o *v1.DatabaseServer) {
				o.Spec.Archive.BaseBackupSchedule = "@every 1.5h"
			}, "",
		),
		Entry(
			"accepts a step in the weekday field of baseBackupSchedule",
			realisticDatabaseServer, func(o *v1.DatabaseServer) {
				o.Spec.Archive.BaseBackupSchedule = "0 0 2 * * */2"
			}, "",
		),
		// The name of the CR names the CloudNativePG cluster, and
		// CloudNativePG takes a DNS-1035 label of at most 50 characters. The
		// "-r99" of a rollback leaves 46 for the name itself.
		Entry(
			"rejects a dot in the name",
			validDatabaseServer, func(o *v1.DatabaseServer) {
				o.Name = "my.db"
			}, "metadata.name",
		),
		Entry(
			"rejects a name that starts with a digit",
			validDatabaseServer, func(o *v1.DatabaseServer) {
				o.Name = "1db"
			}, "metadata.name",
		),
		Entry(
			"rejects a name of 47 characters",
			validDatabaseServer, func(o *v1.DatabaseServer) {
				o.Name = strings.Repeat("a", 47)
			}, "metadata.name",
		),
		Entry(
			"accepts a name of 46 characters",
			validDatabaseServer, func(o *v1.DatabaseServer) {
				o.Name = strings.Repeat("a", 46)
			}, "",
		),
		Entry(
			"rejects a scrape interval that is not a Prometheus duration",
			realisticDatabaseServer, func(o *v1.DatabaseServer) {
				o.Spec.Monitoring.PodMonitor.Interval = "every 30 seconds"
			}, "interval",
		),
	)

	// The name rule runs on create only. A name never changes on update, so an
	// edit of any other field has to pass whatever the name is.
	It("checks the name on create and leaves an update alone", func() {
		obj := createdDatabaseServer()

		Expect(resize(obj, func(o *v1.DatabaseServer) {
			o.Labels = map[string]string{"team": "platform"}
		})).To(Succeed())
	})

	// storageSize serializes as int-or-string, so the no-shrink rule must
	// handle the integer form that a raw manifest can submit.
	It("accepts integer-form storageSize and still rejects shrink", func() {
		obj := &unstructured.Unstructured{Object: map[string]any{
			"apiVersion": "core.camunda.io/v1",
			"kind":       "DatabaseServer",
			"metadata": map[string]any{
				"name":      "dbs-" + utilrand.String(8),
				"namespace": fixtures.SchemaTestNamespace,
			},
			"spec": map[string]any{
				"version":              "17",
				"instances":            int64(1),
				"storageSize":          int64(1073741824),
				"databaseServerConfig": "my-database-server",
			},
		}}

		Expect(k8sClient.Create(ctx, obj)).To(Succeed())
		DeferCleanup(func() { _ = k8sClient.Delete(ctx, obj) })

		Expect(unstructured.SetNestedField(obj.Object, int64(2147483648), "spec", "storageSize")).To(Succeed())
		Expect(k8sClient.Update(ctx, obj)).To(Succeed(), "integer-form growth")

		Expect(unstructured.SetNestedField(obj.Object, "3Gi", "spec", "storageSize")).To(Succeed())
		Expect(k8sClient.Update(ctx, obj)).To(Succeed(), "integer-to-string growth")

		Expect(unstructured.SetNestedField(obj.Object, int64(536870912), "spec", "storageSize")).To(Succeed())
		Expect(k8sClient.Update(ctx, obj)).To(MatchError(ContainSubstring("storageSize")), "integer-form shrink")
	})

	It("rejects shrinking storageSize on update and accepts growth", func() {
		obj := createdDatabaseServer()

		Expect(resize(obj, func(o *v1.DatabaseServer) {
			o.Spec.StorageSize = new(resource.MustParse("64Gi"))
		})).To(MatchError(ContainSubstring("storageSize")))

		Expect(resize(obj, func(o *v1.DatabaseServer) {
			o.Spec.StorageSize = new(resource.MustParse("256Gi"))
		})).To(Succeed())
	})

	// The write-ahead log volume cannot shrink either, and it is a separate
	// claim from the data volume, so it carries its own rule.
	It("rejects shrinking walStorageSize on update", func() {
		obj := createdDatabaseServer()

		Expect(resize(obj, func(o *v1.DatabaseServer) {
			o.Spec.WALStorageSize = new(resource.MustParse("8Gi"))
		})).To(MatchError(ContainSubstring("walStorageSize")))

		Expect(resize(obj, func(o *v1.DatabaseServer) {
			o.Spec.WALStorageSize = new(resource.MustParse("32Gi"))
		})).To(Succeed())
	})
})
