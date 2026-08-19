//go:build e2e
// +build e2e

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

package e2e

import (
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	v1 "github.com/konsole-is/camunda-operator/api/v1"
	"github.com/konsole-is/camunda-operator/pkg/labels"
	"github.com/konsole-is/camunda-operator/test/utils"
)

const (
	// scheduleName is the BackupSchedule of the relational flow.
	scheduleName = "camunda-nightly"
	// scheduleDecoyName is a backup that carries the schedule's label and can
	// never finish, so the first trigger of the schedule meets an overlap on
	// purpose.
	scheduleDecoyName = "camunda-nightly-decoy"

	bsResource = "backupschedules.core.camunda.io"

	// triggerTimeout bounds the wait for one trigger of a one-minute cron:
	// the trigger itself plus the reconcile that consumes it.
	triggerTimeout = 2 * time.Minute
)

// itSchedulesBackups registers the BackupSchedule specs of the relational
// flow. The specs are ordered: the first proves the overlap skip against a
// seeded non-terminal backup, the second removes the seed so the next
// trigger creates a real backup, then deletes the schedule and shows the
// backup outlives it.
func itSchedulesBackups(cluster *v1.CamundaCluster) {
	scheduleLabel := labels.BackupScheduleKey + "=" + scheduleName

	It("skips the trigger of a one-minute cron while a backup of the schedule is not terminal", func() {
		By("seeding a labeled backup that can never finish")
		// The decoy references a cluster that does not exist, so its
		// controller parks it in Pending forever. It carries the schedule's
		// label, so the schedule counts it as its own.
		Expect(apply(&v1.LogicalBackupRDBMS{
			TypeMeta: metav1.TypeMeta{
				APIVersion: v1.GroupVersion.String(),
				Kind:       "LogicalBackupRDBMS",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name: scheduleDecoyName, Namespace: cluster.Namespace,
				Labels: map[string]string{
					labels.ClusterKey:        cluster.Name,
					labels.BackupScheduleKey: scheduleName,
				},
			},
			Spec: v1.LogicalBackupRDBMSSpec{
				ClusterRef: v1.ClusterRef{Name: "no-such-cluster"},
			},
		})).To(Succeed())

		By("creating the BackupSchedule with a one-minute cron")
		Expect(apply(&v1.BackupSchedule{
			TypeMeta: metav1.TypeMeta{
				APIVersion: v1.GroupVersion.String(),
				Kind:       "BackupSchedule",
			},
			ObjectMeta: metav1.ObjectMeta{Name: scheduleName, Namespace: cluster.Namespace},
			Spec: v1.BackupScheduleSpec{
				ClusterRef: v1.ClusterRef{Name: cluster.Name},
				Schedule:   "* * * * *",
			},
		})).To(Succeed())

		By("waiting for the first trigger to skip with an event")
		Eventually(func(g Gomega) {
			expectReady(g, bsResource, scheduleName, cluster.Namespace, v1.ReasonHealthy)

			var schedule v1.BackupSchedule
			g.Expect(utils.Get(bsResource, scheduleName, cluster.Namespace, &schedule)).To(Succeed())
			g.Expect(schedule.Status.LastScheduleTime).NotTo(BeNil())
			g.Expect(schedule.Status.LastBackupName).To(BeEmpty())

			var events corev1.EventList
			g.Expect(utils.List("events", cluster.Namespace, "", &events)).To(Succeed())
			skipped := false
			for _, event := range events.Items {
				if event.InvolvedObject.Name == scheduleName && event.Reason == "TriggerSkipped" {
					skipped = true
				}
			}
			g.Expect(skipped).To(BeTrue(), "no TriggerSkipped event on the schedule")
		}, triggerTimeout, 5*time.Second).Should(Succeed())

		By("checking that the schedule created no backup past the seed")
		var backups v1.LogicalBackupRDBMSList
		Expect(utils.List(lbrdbmsResource, cluster.Namespace, scheduleLabel, &backups)).To(Succeed())
		Expect(backups.Items).To(HaveLen(1))
		Expect(backups.Items[0].Name).To(Equal(scheduleDecoyName))
	})

	It("creates the matching backup kind at the next trigger, and the backup outlives the schedule", func() {
		By("removing the seeded backup so a trigger can run")
		_, err := utils.Kubectl(
			"delete", lbrdbmsResource, scheduleDecoyName, "-n", cluster.Namespace,
		)
		Expect(err).NotTo(HaveOccurred())

		By("waiting for the schedule to create a LogicalBackupRDBMS")
		var backup v1.LogicalBackupRDBMS
		Eventually(func(g Gomega) {
			var backups v1.LogicalBackupRDBMSList
			g.Expect(utils.List(lbrdbmsResource, cluster.Namespace, scheduleLabel, &backups)).To(Succeed())
			g.Expect(backups.Items).To(HaveLen(1))
			backup = backups.Items[0]

			var schedule v1.BackupSchedule
			g.Expect(utils.Get(bsResource, scheduleName, cluster.Namespace, &schedule)).To(Succeed())
			g.Expect(schedule.Status.LastBackupName).To(Equal(backup.Name))
		}, triggerTimeout, 5*time.Second).Should(Succeed())
		Expect(backup.Labels).To(HaveKeyWithValue(labels.ClusterKey, cluster.Name))
		Expect(backup.OwnerReferences).To(BeEmpty())
		Expect(backup.Spec.ClusterRef.Name).To(Equal(cluster.Name))

		By("deleting the schedule while its backup runs")
		_, err = utils.Kubectl("delete", bsResource, scheduleName, "-n", cluster.Namespace)
		Expect(err).NotTo(HaveOccurred())
		Eventually(func(g Gomega) {
			expectGone(g, bsResource, scheduleName, cluster.Namespace)
		}, time.Minute, 5*time.Second).Should(Succeed())

		By("watching the backup complete without its schedule")
		Eventually(func(g Gomega) {
			expectReady(g, lbrdbmsResource, backup.Name, cluster.Namespace, v1.ReasonCompleted)
		}, backupTimeout, 5*time.Second).Should(Succeed())

		By("cleaning up the scheduled backup through its finalizer")
		_, err = utils.Kubectl(
			"delete", lbrdbmsResource, backup.Name, "-n", cluster.Namespace, "--wait=false",
		)
		Expect(err).NotTo(HaveOccurred())
		Eventually(func(g Gomega) {
			expectGone(g, lbrdbmsResource, backup.Name, cluster.Namespace)
		}, 5*time.Minute, 5*time.Second).Should(Succeed())
	})
}
