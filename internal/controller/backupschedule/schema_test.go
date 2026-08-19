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

package backupschedule

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	utilrand "k8s.io/apimachinery/pkg/util/rand"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	v1 "github.com/konsole-is/camunda-operator/api/v1"
)

// The admission schema of the BackupSchedule CRD.
var _ = Describe("BackupSchedule schema", func() {
	valid := func() *v1.BackupSchedule {
		return &v1.BackupSchedule{
			ObjectMeta: metav1.ObjectMeta{
				Name: "schedule-" + utilrand.String(6), Namespace: "default",
			},
			Spec: v1.BackupScheduleSpec{
				ClusterRef: v1.ClusterRef{Name: "my-cluster"},
				Schedule:   "0 2 * * *",
			},
		}
	}

	create := func(schedule *v1.BackupSchedule) error {
		err := k8sClient.Create(ctx, schedule)
		if err == nil {
			DeferCleanup(func() { _ = k8sClient.Delete(ctx, schedule) })
		}

		return err
	}

	It("accepts the minimal example and fills the retained defaults", func() {
		schedule := valid()
		Expect(create(schedule)).To(Succeed())

		var created v1.BackupSchedule
		Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(schedule), &created)).To(Succeed())
		Expect(created.Spec.Retained).NotTo(BeNil())
		Expect(created.Spec.Retained.Completed).To(HaveValue(BeEquivalentTo(7)))
		Expect(created.Spec.Retained.Failed).To(HaveValue(BeEquivalentTo(3)))
	})

	It("keeps an explicit retained.failed of zero and fills the other default", func() {
		schedule := valid()
		schedule.Spec.Retained = &v1.RetainedBackups{Failed: ptr.To(int32(0))}
		Expect(create(schedule)).To(Succeed())

		var created v1.BackupSchedule
		Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(schedule), &created)).To(Succeed())
		Expect(created.Spec.Retained.Completed).To(HaveValue(BeEquivalentTo(7)))
		Expect(created.Spec.Retained.Failed).To(HaveValue(BeEquivalentTo(0)))
	})

	It("accepts steps, ranges, lists, and names in the cron fields", func() {
		for _, expr := range []string{
			"*/5 * * * *",
			"0 2-6 * * *",
			"0 2 1,15 * *",
			"0 2 * * MON-FRI",
			"15 14 1 JAN *",
		} {
			schedule := valid()
			schedule.Spec.Schedule = expr
			Expect(create(schedule)).To(Succeed(), "cron %q", expr)
		}
	})

	It("rejects a schedule that is not five cron fields", func() {
		for _, expr := range []string{
			"",
			"0 2 * *",
			"0 0 2 * * *",
			"@hourly",
			"CRON_TZ=Asia/Tokyo 0 2 * * *",
		} {
			schedule := valid()
			schedule.Spec.Schedule = expr
			Expect(create(schedule)).NotTo(Succeed(), "cron %q", expr)
		}
	})

	It("rejects retained bounds below their minimums", func() {
		schedule := valid()
		schedule.Spec.Retained = &v1.RetainedBackups{Completed: ptr.To(int32(0))}
		Expect(create(schedule)).NotTo(Succeed())

		schedule = valid()
		schedule.Spec.Retained = &v1.RetainedBackups{Failed: ptr.To(int32(-1))}
		Expect(create(schedule)).NotTo(Succeed())
	})

	It("rejects an empty clusterRef name", func() {
		schedule := valid()
		schedule.Spec.ClusterRef.Name = ""
		Expect(create(schedule)).NotTo(Succeed())
	})
})
