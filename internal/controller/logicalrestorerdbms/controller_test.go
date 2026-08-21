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

package logicalrestorerdbms

import (
	"maps"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	v1 "github.com/konsole-is/camunda-operator/api/v1"
	components "github.com/konsole-is/camunda-operator/pkg/components/logicalrestorerdbms"
	"github.com/konsole-is/camunda-operator/pkg/labels"
	restorepkg "github.com/konsole-is/camunda-operator/pkg/restore"
)

var _ = Describe("LogicalRestoreRDBMS admission", func() {
	It("suspends a target that still runs, and waits for its brokers to stop", func() {
		w := newWorld(func(cluster *v1.CamundaCluster) { cluster.Spec.Suspend = false })
		w.setRunningBrokers(worldBrokers)
		backup := createBackup(w)

		lrr := createRestore(w, backup.Name)

		By("suspending the target and recording that it did")
		Eventually(func(g Gomega) {
			var cluster v1.CamundaCluster
			g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(w.cluster), &cluster)).To(Succeed())
			g.Expect(cluster.Spec.Suspend).To(BeTrue())
			g.Expect(latest(g, lrr).Status.ClusterSuspended).To(BeTrue())
		}, timeout, interval).Should(Succeed())

		By("touching nothing while the brokers still run")
		Consistently(func(g Gomega) {
			g.Expect(latest(g, lrr).Status.Phase).To(Equal(v1.LogicalRestorePending))
			var jobs batchv1.JobList
			g.Expect(k8sClient.List(ctx, &jobs, client.InNamespace(w.namespace))).To(Succeed())
			g.Expect(jobs.Items).To(BeEmpty())
		}, "2s", interval).Should(Succeed())

		By("continuing once the brokers stopped")
		w.setRunningBrokers(0)
		Eventually(func(g Gomega) {
			g.Expect(latest(g, lrr).Status.BackupID).To(Equal(backupID))
		}, timeout, interval).Should(Succeed())
	})

	// A cluster that its owner suspended carries no record of this restore.
	// The restore leaves it suspended when it finishes.
	It("records nothing when the target was already suspended", func() {
		w := newWorld()
		backup := createBackup(w)

		lrr := createRestore(w, backup.Name)

		Eventually(func(g Gomega) {
			current := latest(g, lrr)
			g.Expect(current.Status.BackupID).To(Equal(backupID))
			g.Expect(current.Status.ClusterSuspended).To(BeFalse())
		}, timeout, interval).Should(Succeed())
	})

	It("holds a restore whose backup does not exist", func() {
		w := newWorld()

		lrr := createRestore(w, "no-such-backup")

		expectReason(lrr, v1.LogicalRestorePending, v1.ReasonInvalidReference)
	})

	It("holds a restore whose backup is not completed", func() {
		w := newWorld()
		backup := createBackup(w, func(b *v1.LogicalBackupRDBMS) {
			b.Status.Phase = v1.LogicalBackupRunning
		})

		lrr := createRestore(w, backup.Name)

		reached := expectReason(lrr, v1.LogicalRestorePending, v1.ReasonInvalidReference)
		Expect(readyCondition(reached).Message).To(ContainSubstring("Completed backup"))
	})

	It("holds a restore whose target cluster does not exist", func() {
		w := newWorld()
		backup := createBackup(w)

		lrr := &v1.LogicalRestoreRDBMS{
			ObjectMeta: metav1.ObjectMeta{Name: "lrr-no-cluster", Namespace: w.namespace},
			Spec: v1.LogicalRestoreRDBMSSpec{
				BackupRef:        v1.LogicalBackupRef{Name: backup.Name},
				TargetClusterRef: v1.ClusterRef{Name: "no-such-cluster"},
			},
		}
		Expect(k8sClient.Create(ctx, lrr)).To(Succeed())

		expectReason(lrr, v1.LogicalRestorePending, v1.ReasonInvalidReference)
	})

	It("pins what it reads once the target is suspended and the backup is completed", func() {
		w := newWorld()
		backup := createBackup(w)

		lrr := createRestore(w, backup.Name)

		Eventually(func(g Gomega) {
			current := latest(g, lrr)
			g.Expect(current.Status.BackupID).To(Equal(backupID))
			g.Expect(current.Status.TargetClusterUID).To(Equal(w.cluster.UID))
		}, timeout, interval).Should(Succeed())
	})

	It("holds a target whose brokers never stop, and touches nothing", func() {
		w := newWorld()
		w.setRunningBrokers(worldBrokers)
		backup := createBackup(w)

		lrr := createRestore(w, backup.Name)

		reached := expectReason(lrr, v1.LogicalRestorePending, v1.ReasonProgressing)
		Expect(readyMessage(reached)).To(ContainSubstring("brokers still run"))

		// Nothing bounds this hold. The restore erased nothing, so it waits
		// for the cluster instead of ending itself.
		Consistently(func(g Gomega) {
			current := latest(g, lrr)
			g.Expect(current.Status.Phase).To(Equal(v1.LogicalRestorePending))
			g.Expect(current.Status.BackupID).To(BeZero())
			var jobs batchv1.JobList
			g.Expect(k8sClient.List(ctx, &jobs, client.InNamespace(w.namespace))).To(Succeed())
			g.Expect(jobs.Items).To(BeEmpty())
		}, "2s", interval).Should(Succeed())
	})

	// The controller watches CamundaCluster and enqueues the restores that
	// name it. Without that watch, a restore whose target appears later waits
	// for the retry timer instead. watchWindow is shorter than the retry
	// interval of the suite, so a restore that moves inside it was woken by
	// the watch and by nothing else.
	It("wakes a waiting restore through the cluster watch when its target appears", func() {
		w := newWorld()
		backup := createBackup(w)

		By("removing the target of the restore")
		Expect(k8sClient.Delete(ctx, w.cluster)).To(Succeed())
		Eventually(func(g Gomega) {
			var gone v1.CamundaCluster
			err := k8sClient.Get(ctx, client.ObjectKeyFromObject(w.cluster), &gone)
			g.Expect(apierrors.IsNotFound(err)).To(BeTrue())
		}, timeout, interval).Should(Succeed())

		lrr := createRestore(w, backup.Name)
		expectReason(lrr, v1.LogicalRestorePending, v1.ReasonInvalidReference)

		By("creating the target again")
		replacement := &v1.CamundaCluster{
			ObjectMeta: metav1.ObjectMeta{Name: w.cluster.Name, Namespace: w.namespace},
			Spec:       *w.cluster.Spec.DeepCopy(),
		}
		Expect(k8sClient.Create(ctx, replacement)).To(Succeed())

		Eventually(func(g Gomega) {
			g.Expect(latest(g, lrr).Status.BackupID).To(Equal(backupID))
		}, watchWindow, interval).Should(Succeed())
	})

	It("wakes a waiting restore through the backup watch when the backup completes", func() {
		w := newWorld()
		backup := createBackup(w, func(b *v1.LogicalBackupRDBMS) {
			b.Status.Phase = v1.LogicalBackupRunning
		})

		lrr := createRestore(w, backup.Name)
		expectReason(lrr, v1.LogicalRestorePending, v1.ReasonInvalidReference)

		By("completing the backup")
		Eventually(func(g Gomega) {
			var current v1.LogicalBackupRDBMS
			g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(backup), &current)).To(Succeed())
			current.Status.Phase = v1.LogicalBackupCompleted
			g.Expect(k8sClient.Status().Update(ctx, &current)).To(Succeed())
		}, timeout, interval).Should(Succeed())

		// watchWindow is shorter than the retry interval of the suite, so
		// only the watch can move the restore in time.
		Eventually(func(g Gomega) {
			g.Expect(latest(g, lrr).Status.BackupID).To(Equal(backupID))
		}, watchWindow, interval).Should(Succeed())
	})
})

var _ = Describe("LogicalRestoreRDBMS suspension of its target", func() {
	// The restore owns the suspension it applied. It gives it back when the
	// target holds the backup again.
	It("unsuspends the target it suspended when it completes", func() {
		w := newWorld(func(cluster *v1.CamundaCluster) { cluster.Spec.Suspend = false })
		backup := createBackup(w)
		lrr := createRestore(w, backup.Name)
		completeSecondaryStorage(w, lrr)
		Expect(latestOf(lrr).Status.ClusterSuspended).To(BeTrue())

		owner := labels.LogicalRestoreRDBMS(lrr.Name)
		Eventually(func(g Gomega) {
			g.Expect(latest(g, lrr).Status.PrimaryJobNames).To(HaveLen(int(worldBrokers)))
		}, timeout, interval).Should(Succeed())
		for ordinal := range worldBrokers {
			markJob(w.namespace, restorepkg.JobName(owner, ordinal), batchv1.JobComplete)
		}

		expectReason(lrr, v1.LogicalRestoreCompleted, v1.ReasonCompleted)
		Eventually(func(g Gomega) {
			g.Expect(clusterSuspended(g, w)).To(BeFalse())
		}, timeout, interval).Should(Succeed())
	})

	// The broker volumes of a failed restore can be empty or half written.
	// Brokers that start over them are worse than a cluster that is down.
	It("leaves the target suspended when it fails", func() {
		w := newWorld(func(cluster *v1.CamundaCluster) { cluster.Spec.Suspend = false })
		backup := createBackup(w)
		lrr := createRestore(w, backup.Name)
		completeSecondaryStorage(w, lrr)

		owner := labels.LogicalRestoreRDBMS(lrr.Name)
		Eventually(func(g Gomega) {
			g.Expect(latest(g, lrr).Status.PrimaryJobNames).To(HaveLen(int(worldBrokers)))
		}, timeout, interval).Should(Succeed())
		markJob(w.namespace, restorepkg.JobName(owner, 0), batchv1.JobFailed)

		expectReason(lrr, v1.LogicalRestoreFailed, v1.ReasonFailed)
		Consistently(func(g Gomega) {
			g.Expect(clusterSuspended(g, w)).To(BeTrue())
		}, "1s", interval).Should(Succeed())
	})

	// A target that its owner suspended stays suspended. The restore recorded
	// no suspension of its own, so it withdraws none.
	It("leaves a target that its owner suspended suspended", func() {
		w := newWorld()
		backup := createBackup(w)
		lrr := createRestore(w, backup.Name)
		completeSecondaryStorage(w, lrr)
		Expect(latestOf(lrr).Status.ClusterSuspended).To(BeFalse())

		owner := labels.LogicalRestoreRDBMS(lrr.Name)
		Eventually(func(g Gomega) {
			g.Expect(latest(g, lrr).Status.PrimaryJobNames).To(HaveLen(int(worldBrokers)))
		}, timeout, interval).Should(Succeed())
		for ordinal := range worldBrokers {
			markJob(w.namespace, restorepkg.JobName(owner, ordinal), batchv1.JobComplete)
		}

		expectReason(lrr, v1.LogicalRestoreCompleted, v1.ReasonCompleted)
		Consistently(func(g Gomega) {
			g.Expect(clusterSuspended(g, w)).To(BeTrue())
		}, "1s", interval).Should(Succeed())
	})
})

var _ = Describe("LogicalRestoreRDBMS cluster claim", func() {
	// One backup or restore of a cluster runs at a time. A restore that
	// started while a backup runs would rewrite the database under it.
	It("holds a restore whose cluster another operation holds, and touches nothing", func() {
		w := newWorld()
		holder := holdCluster(w)
		backup := createBackup(w)

		lrr := createRestore(w, backup.Name)

		reached := expectReason(lrr, v1.LogicalRestorePending, v1.ReasonClusterClaimed)
		Expect(readyCondition(reached).Message).To(ContainSubstring(holder.Name))
		Expect(claimHolder(w)).To(Equal("LogicalBackupRDBMS/" + holder.Name))

		var jobs batchv1.JobList
		Expect(k8sClient.List(ctx, &jobs, client.InNamespace(w.namespace))).To(Succeed())
		Expect(jobs.Items).To(BeEmpty())
	})

	// Nothing bounds the hold, and no spec change ends it. No watch of this
	// controller covers a holder that the restore does not reference, so the
	// retry timer is what takes the claim over once the holder is terminal.
	It("starts on its own once the holder finishes", func() {
		w := newWorld()
		holder := holdCluster(w)
		backup := createBackup(w)

		lrr := createRestore(w, backup.Name)
		expectReason(lrr, v1.LogicalRestorePending, v1.ReasonClusterClaimed)

		Eventually(func(g Gomega) {
			var current v1.LogicalBackupRDBMS
			g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(holder), &current)).To(Succeed())
			current.Status.Phase = v1.LogicalBackupCompleted
			g.Expect(k8sClient.Status().Update(ctx, &current)).To(Succeed())
		}, timeout, interval).Should(Succeed())

		Eventually(func() string {
			return claimHolder(w)
		}, timeout, interval).Should(Equal("LogicalRestoreRDBMS/" + lrr.Name))
	})

	It("gives the claim back when it completes", func() {
		w := newWorld()
		backup := createBackup(w)
		lrr := createRestore(w, backup.Name)
		completeSecondaryStorage(w, lrr)

		owner := labels.LogicalRestoreRDBMS(lrr.Name)
		Eventually(func(g Gomega) {
			g.Expect(latest(g, lrr).Status.PrimaryJobNames).To(HaveLen(int(worldBrokers)))
		}, timeout, interval).Should(Succeed())
		for ordinal := range worldBrokers {
			markJob(w.namespace, restorepkg.JobName(owner, ordinal), batchv1.JobComplete)
		}

		expectReason(lrr, v1.LogicalRestoreCompleted, v1.ReasonCompleted)
		Eventually(func() string {
			return claimHolder(w)
		}, timeout, interval).Should(BeEmpty())
	})
})

var _ = Describe("LogicalRestoreRDBMS compatibility", func() {
	It("walks a compatible pair into the secondary storage phase", func() {
		w := newWorld()
		backup := createBackup(w)

		lrr := createRestore(w, backup.Name)

		expectPhase(lrr, v1.LogicalRestoreRestoringSecondaryStorage)
	})

	It("fails a pair whose backup bucket differs from the target's", func() {
		w := newWorld()
		backup := createBackup(w, func(b *v1.LogicalBackupRDBMS) {
			b.Status.BucketRef = "another-bucket"
		})

		lrr := createRestore(w, backup.Name)

		reached := expectReason(lrr, v1.LogicalRestoreFailed, v1.ReasonIncompatibleTarget)
		Expect(reached.Status.FailureMessage).To(ContainSubstring("another-bucket"))
		Expect(reached.Status.FailureMessage).To(ContainSubstring(w.bucket.Name))
	})

	It("fails a backup that recorded no Camunda version", func() {
		w := newWorld()
		backup := createBackup(w, func(b *v1.LogicalBackupRDBMS) { b.Status.Version = "" })

		lrr := createRestore(w, backup.Name)

		reached := expectReason(lrr, v1.LogicalRestoreFailed, v1.ReasonIncompatibleTarget)
		Expect(reached.Status.FailureMessage).To(ContainSubstring("did not record"))
	})

	// The version rule of a relational backup accepts the same minor or one
	// minor newer. The restore carries the target to the version of the
	// backup in every case, so a target of an older minor is moved forward
	// instead of refused.
	It("moves a target of an older Camunda minor to the version of the backup", func() {
		w := newWorld()
		backup := createBackup(w, func(b *v1.LogicalBackupRDBMS) { b.Status.Version = "8.10.0" })

		lrr := createRestore(w, backup.Name)

		By("writing the version of the backup on the target")
		Eventually(func(g Gomega) {
			var cluster v1.CamundaCluster
			g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(w.cluster), &cluster)).To(Succeed())
			g.Expect(cluster.Spec.Version).To(Equal("8.10.0"))
		}, timeout, interval).Should(Succeed())

		By("holding until the brokers carry that version")
		Consistently(func(g Gomega) {
			g.Expect(latest(g, lrr).Status.Phase).To(Equal(v1.LogicalRestorePending))
		}, "1s", interval).Should(Succeed())

		By("continuing once the CamundaCluster controller rolled it out")
		w.rollBrokerImage("8.10.0")
		Eventually(func(g Gomega) {
			g.Expect(latest(g, lrr).Status.BackupID).To(Equal(backupID))
		}, timeout, interval).Should(Succeed())
	})

	// The version rule would accept this target as it is. The restore still
	// moves it back to the version of the backup, so the cluster comes back
	// one minor behind where it was and the owner upgrades forward again.
	It("moves a target of a newer Camunda minor back to the version of the backup", func() {
		w := newWorld()
		w.rollBrokerImage("8.10.0")
		backup := createBackup(w)

		lrr := createRestore(w, backup.Name)

		Eventually(func(g Gomega) {
			var cluster v1.CamundaCluster
			g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(w.cluster), &cluster)).To(Succeed())
			g.Expect(cluster.Spec.Version).To(Equal(worldVersion))
		}, timeout, interval).Should(Succeed())

		w.rollBrokerImage(worldVersion)
		Eventually(func(g Gomega) {
			g.Expect(latest(g, lrr).Status.BackupID).To(Equal(backupID))
		}, timeout, interval).Should(Succeed())
	})

	// A version that the restore cannot write is not a wait. Such a backup
	// breaks the version rule, and the rule still ends the restore.
	It("fails a backup whose recorded version is not a version", func() {
		w := newWorld()
		backup := createBackup(w, func(b *v1.LogicalBackupRDBMS) { b.Status.Version = "latest" })

		lrr := createRestore(w, backup.Name)

		reached := expectReason(lrr, v1.LogicalRestoreFailed, v1.ReasonIncompatibleTarget)
		Expect(reached.Status.FailureMessage).To(ContainSubstring("not a version"))
	})
})

var _ = Describe("LogicalRestoreRDBMS of the logical database", func() {
	It("restores the database with one Job and then moves to the broker volumes", func() {
		w := newWorld()
		backup := createBackup(w)

		lrr := createRestore(w, backup.Name)

		By("applying exactly one pg_restore Job and recording it")
		var jobName string
		Eventually(func(g Gomega) {
			current := latest(g, lrr)
			g.Expect(current.Status.Phase).To(Equal(v1.LogicalRestoreRestoringSecondaryStorage))
			g.Expect(current.Status.SecondaryJobName).NotTo(BeEmpty())
			jobName = current.Status.SecondaryJobName
		}, timeout, interval).Should(Succeed())

		Consistently(func(g Gomega) {
			var jobs batchv1.JobList
			g.Expect(k8sClient.List(ctx, &jobs, client.InNamespace(w.namespace))).To(Succeed())
			g.Expect(jobs.Items).To(HaveLen(1))
		}, "2s", interval).Should(Succeed())

		By("restoring the dump as the role that owns the database of the target")
		job := jobNamed(w.namespace, jobName)
		Expect(job.Spec.Template.Spec.Containers).To(HaveLen(1))
		container := job.Spec.Template.Spec.Containers[0]
		Expect(envValue(container, "PGDATABASE")).To(Equal("camunda"))
		Expect(envValue(container, "PGHOST")).To(Equal("postgres.databases.svc"))
		// The backup role that wrote the archive owns none of the objects it
		// dumped, and pg_restore --clean drops every one of them, so a restore
		// that connected as the backup role would fail on every DROP.
		Expect(secretOfEnv(container, "PGUSER")).To(Equal("app-user"))

		By("keeping the pg_restore Job out of the broker-Job selector of the next phase")
		Expect(job.Labels).To(HaveKeyWithValue(labels.ComponentKey, components.ComponentName))
		Expect(job.Labels[labels.ComponentKey]).NotTo(Equal(restorepkg.ComponentRestore))

		By("moving on to primary storage when the Job completed")
		markJob(w.namespace, jobName, batchv1.JobComplete)
		expectPhase(lrr, v1.LogicalRestoreRestoringPrimaryStorage)
	})

	It("refuses a pg_restore Job that another restore left behind under its name", func() {
		w := newWorld()
		backup := createBackup(w)

		lrr := &v1.LogicalRestoreRDBMS{
			ObjectMeta: metav1.ObjectMeta{Name: "lrr-pg-successor", Namespace: w.namespace},
			Spec: v1.LogicalRestoreRDBMSSpec{
				BackupRef:        v1.LogicalBackupRef{Name: backup.Name},
				TargetClusterRef: v1.ClusterRef{Name: w.cluster.Name},
			},
		}
		createForeignJob(w, components.JobName(lrr))

		Expect(k8sClient.Create(ctx, lrr)).To(Succeed())

		reached := expectReason(lrr, v1.LogicalRestoreFailed, v1.ReasonFailed)
		Expect(reached.Status.FailureMessage).To(ContainSubstring(components.JobName(lrr)))
		Expect(reached.Status.FailureMessage).To(ContainSubstring("another restore"))
	})

	It("fails when the pg_restore Job fails", func() {
		w := newWorld()
		backup := createBackup(w)

		lrr := createRestore(w, backup.Name)

		var jobName string
		Eventually(func(g Gomega) {
			jobName = latest(g, lrr).Status.SecondaryJobName
			g.Expect(jobName).NotTo(BeEmpty())
		}, timeout, interval).Should(Succeed())

		markJob(w.namespace, jobName, batchv1.JobFailed)

		reached := expectReason(lrr, v1.LogicalRestoreFailed, v1.ReasonFailed)
		Expect(reached.Status.FailureMessage).To(ContainSubstring(jobName))
	})

	It("fails when the recorded pg_restore Job is gone", func() {
		w := newWorld()
		backup := createBackup(w)

		lrr := createRestore(w, backup.Name)

		var jobName string
		Eventually(func(g Gomega) {
			jobName = latest(g, lrr).Status.SecondaryJobName
			g.Expect(jobName).NotTo(BeEmpty())
		}, timeout, interval).Should(Succeed())

		// The logical database then holds a partial restore that only a new
		// attempt repairs, so the restore must not create a second Job.
		job := jobNamed(w.namespace, jobName)
		Expect(k8sClient.Delete(ctx, job, client.PropagationPolicy(metav1.DeletePropagationBackground))).
			To(Succeed())

		reached := expectReason(lrr, v1.LogicalRestoreFailed, v1.ReasonFailed)
		Expect(reached.Status.FailureMessage).To(ContainSubstring("disappeared"))
	})

	It("reports a pod that cannot start and fails after the grace", func() {
		w := newWorld()
		backup := createBackup(w)

		lrr := createRestore(w, backup.Name)

		var jobName string
		Eventually(func(g Gomega) {
			jobName = latest(g, lrr).Status.SecondaryJobName
			g.Expect(jobName).NotTo(BeEmpty())
		}, timeout, interval).Should(Succeed())

		By("standing in for a kubelet that cannot mount a Secret of the pod")
		stuckPod(w, jobName, map[string]string{
			components.RestoreUIDLabel: string(lrr.UID),
			labels.ComponentKey:        components.ComponentName,
		})

		expectReason(lrr, v1.LogicalRestoreRestoringSecondaryStorage, v1.ReasonMissingSecret)

		reached := expectReason(lrr, v1.LogicalRestoreFailed, v1.ReasonFailed)
		Expect(reached.Status.FailureMessage).To(ContainSubstring("did not recover"))
	})
})

var _ = Describe("LogicalRestoreRDBMS of primary storage", func() {
	It("recreates the broker volumes, runs one Job per broker, and completes", func() {
		w := newWorld()
		backup := createBackup(w)
		lrr := createRestore(w, backup.Name)
		completeSecondaryStorage(w, lrr)

		By("recording the broker count and the recreated volumes")
		Eventually(func(g Gomega) {
			current := latest(g, lrr)
			g.Expect(current.Status.Brokers).To(Equal(worldBrokers))
			g.Expect(current.Status.RecreatedClaims).To(ConsistOf(w.claimNames()))
		}, timeout, interval).Should(Succeed())

		By("running one restore Job per broker, labelled as the restore of this cluster")
		owner := labels.LogicalRestoreRDBMS(lrr.Name)
		names := []string{restorepkg.JobName(owner, 0), restorepkg.JobName(owner, 1)}
		Eventually(func(g Gomega) {
			g.Expect(latest(g, lrr).Status.PrimaryJobNames).To(Equal(names))
		}, timeout, interval).Should(Succeed())

		By("sizing every volume from the recorded restore size, with no owner reference")
		for _, name := range w.claimNames() {
			claim := claimNamed(w.namespace, name)
			Expect(claim.Spec.Resources.Requests[corev1.ResourceStorage]).To(
				Equal(resource.MustParse(worldRestoreSize)),
			)
			Expect(claim.OwnerReferences).To(BeEmpty())
		}

		job := jobNamed(w.namespace, names[0])
		Expect(job.Labels).To(HaveKeyWithValue(labels.ComponentKey, restorepkg.ComponentRestore))
		Expect(job.Labels).To(HaveKeyWithValue(labels.LogicalRestoreRDBMSKey, lrr.Name))
		Expect(job.Labels).To(HaveKeyWithValue(labels.ClusterKey, w.cluster.Name))
		// The restore application reads the exporter position from the
		// restored database and picks the primary-storage backups itself.
		Expect(job.Spec.Template.Spec.Containers[0].Args).To(BeEmpty())
		// envtest runs no garbage collector, so the controller reference is
		// what proves that deleting the restore removes its Jobs.
		Expect(job.OwnerReferences).To(HaveLen(1))
		Expect(job.OwnerReferences[0].Name).To(Equal(lrr.Name))
		Expect(job.OwnerReferences[0].Controller).NotTo(BeNil())

		By("completing when every Job completed")
		for _, name := range names {
			markJob(w.namespace, name, batchv1.JobComplete)
		}
		reached := expectReason(lrr, v1.LogicalRestoreCompleted, v1.ReasonCompleted)
		Expect(readyCondition(reached).Status).To(Equal(metav1.ConditionTrue))
		Expect(reached.Status.CompletionTime).NotTo(BeNil())

		By("leaving the recreated volumes behind when the restore is deleted")
		Expect(k8sClient.Delete(ctx, reached)).To(Succeed())
		Consistently(func(g Gomega) {
			for _, name := range w.claimNames() {
				g.Expect(k8sClient.Get(
					ctx, types.NamespacedName{Namespace: w.namespace, Name: name},
					&corev1.PersistentVolumeClaim{},
				)).To(Succeed())
			}
		}, "2s", interval).Should(Succeed())
	})

	It("sizes the volumes from the claim template when the backup recorded no size", func() {
		w := newWorld()
		backup := createBackup(w, func(b *v1.LogicalBackupRDBMS) {
			b.Status.StorageSizes = v1.LogicalBackupStorageSizes{}
		})
		lrr := createRestore(w, backup.Name)
		completeSecondaryStorage(w, lrr)

		expectPhase(lrr, v1.LogicalRestoreRestoringPrimaryStorage)
		Eventually(func(g Gomega) {
			g.Expect(latest(g, lrr).Status.PrimaryJobNames).To(HaveLen(int(worldBrokers)))
		}, timeout, interval).Should(Succeed())

		claim := claimNamed(w.namespace, w.claimNames()[0])
		Expect(claim.Spec.Resources.Requests[corev1.ResourceStorage]).To(
			Equal(resource.MustParse(worldClaimSize)),
		)
	})

	It("reports a restore pod that cannot start and fails after the grace", func() {
		w := newWorld()
		backup := createBackup(w)
		lrr := createRestore(w, backup.Name)
		completeSecondaryStorage(w, lrr)

		owner := labels.LogicalRestoreRDBMS(lrr.Name)
		Eventually(func(g Gomega) {
			g.Expect(latest(g, lrr).Status.PrimaryJobNames).To(HaveLen(int(worldBrokers)))
		}, timeout, interval).Should(Succeed())

		By("standing in for a kubelet that cannot mount a Secret of the pod")
		stuckPod(w, restorepkg.JobName(owner, 0), restorepkg.JobSelector(owner))

		expectReason(lrr, v1.LogicalRestoreRestoringPrimaryStorage, v1.ReasonMissingSecret)

		// The grace runs out only when the clock survives a look that resolved
		// every reference again. A restore that cleared it on each look would
		// hold here for ever.
		reached := expectReason(lrr, v1.LogicalRestoreFailed, v1.ReasonFailed)
		Expect(reached.Status.FailureMessage).To(ContainSubstring("did not recover"))
	})

	It("stops when the target is unsuspended while the restore runs", func() {
		w := newWorld()
		backup := createBackup(w)
		lrr := createRestore(w, backup.Name)
		completeSecondaryStorage(w, lrr)
		expectPhase(lrr, v1.LogicalRestoreRestoringPrimaryStorage)

		By("starting the workloads of the target again")
		w.suspend(false)

		// The restore deletes the data volumes of the brokers in this phase.
		// A cluster whose workloads run again must hold it, and the grace
		// then ends it.
		expectReason(lrr, v1.LogicalRestoreRestoringPrimaryStorage, v1.ReasonClusterNotSuspended)

		reached := expectReason(lrr, v1.LogicalRestoreFailed, v1.ReasonFailed)
		Expect(reached.Status.FailureMessage).To(ContainSubstring("did not recover"))
	})

	It("refuses a backup that was replaced under its name", func() {
		w := newWorld()
		backup := createBackup(w)
		lrr := createRestore(w, backup.Name)

		Eventually(func(g Gomega) {
			g.Expect(latest(g, lrr).Status.BackupID).To(Equal(backupID))
		}, timeout, interval).Should(Succeed())

		By("replacing the backup with another completed one of the same name")
		Expect(k8sClient.Delete(ctx, backup)).To(Succeed())
		replacement := &v1.LogicalBackupRDBMS{
			ObjectMeta: metav1.ObjectMeta{Name: backup.Name, Namespace: w.namespace},
			Spec:       v1.LogicalBackupRDBMSSpec{ClusterRef: v1.ClusterRef{Name: w.cluster.Name}},
		}
		Eventually(func(g Gomega) {
			g.Expect(k8sClient.Create(ctx, replacement)).To(Succeed())
		}, timeout, interval).Should(Succeed())
		replacement.Status = v1.LogicalBackupRDBMSStatus{
			Phase:     v1.LogicalBackupCompleted,
			BackupID:  backupID + 1,
			Version:   worldVersion,
			BucketRef: w.bucket.Name,
			ObjectKey: "clusters/elsewhere/camunda.dump",
		}
		Expect(k8sClient.Status().Update(ctx, replacement)).To(Succeed())

		// The dump of the replacement lies under another key. A restore that
		// followed the name would download it and call it the backup it
		// pinned.
		reached := expectReason(lrr, v1.LogicalRestoreFailed, v1.ReasonFailed)
		Expect(reached.Status.FailureMessage).To(ContainSubstring("another backup"))
	})

	It("refuses a broker Job that another restore left behind under its name", func() {
		w := newWorld()
		backup := createBackup(w)

		// The name of a restore Job comes from the name of the restore and the
		// broker ordinal, so a restore that somebody deleted and created again
		// finds the Jobs of its predecessor. Only the controller reference
		// tells the two apart.
		name := restorepkg.JobName(labels.LogicalRestoreRDBMS("lrr-successor"), 0)
		createForeignJob(w, name)

		lrr := createNamedRestore(w, "lrr-successor", backup.Name)
		completeSecondaryStorage(w, lrr)

		reached := expectReason(lrr, v1.LogicalRestoreFailed, v1.ReasonFailed)
		Expect(reached.Status.FailureMessage).To(ContainSubstring(name))
		Expect(reached.Status.FailureMessage).To(ContainSubstring("Remove the Job"))

		By("writing nothing to the Job of the other restore")
		foreign := jobNamed(w.namespace, name)
		Expect(foreign.OwnerReferences).To(BeEmpty())
		for _, entry := range foreign.ManagedFields {
			Expect(entry.Manager).NotTo(Equal(string(restorepkg.FieldManagerLogicalRestoreRDBMS)))
		}
	})

	It("fails and names the broker when one restore Job fails", func() {
		w := newWorld()
		backup := createBackup(w)
		lrr := createRestore(w, backup.Name)
		completeSecondaryStorage(w, lrr)

		owner := labels.LogicalRestoreRDBMS(lrr.Name)
		Eventually(func(g Gomega) {
			g.Expect(latest(g, lrr).Status.PrimaryJobNames).To(HaveLen(int(worldBrokers)))
		}, timeout, interval).Should(Succeed())

		markJob(w.namespace, restorepkg.JobName(owner, 0), batchv1.JobComplete)
		markJob(w.namespace, restorepkg.JobName(owner, 1), batchv1.JobFailed)

		reached := expectReason(lrr, v1.LogicalRestoreFailed, v1.ReasonFailed)
		Expect(reached.Status.FailureMessage).To(ContainSubstring("broker 1"))
	})
})

// completeSecondaryStorage drives a restore through its pg_restore Job, so a
// spec about primary storage starts where that phase ends.
func completeSecondaryStorage(w *world, lrr *v1.LogicalRestoreRDBMS) {
	GinkgoHelper()

	var jobName string
	Eventually(func(g Gomega) {
		jobName = latest(g, lrr).Status.SecondaryJobName
		g.Expect(jobName).NotTo(BeEmpty())
	}, timeout, interval).Should(Succeed())

	markJob(w.namespace, jobName, batchv1.JobComplete)
}

// stuckPod creates a pod of a Job of the restore that reports a container
// which cannot start. The selector labels are the ones that the controller
// lists the pods of that Job by. envtest runs no kubelet, so the suite writes
// the state that a kubelet would report.
func stuckPod(w *world, jobName string, selector map[string]string) {
	GinkgoHelper()

	podLabels := map[string]string{"batch.kubernetes.io/job-name": jobName}
	maps.Copy(podLabels, selector)

	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      jobName + "-stuck",
			Namespace: w.namespace,
			Labels:    podLabels,
		},
		Spec: corev1.PodSpec{
			RestartPolicy: corev1.RestartPolicyNever,
			Containers:    []corev1.Container{{Name: "restore", Image: "postgres:17"}},
		},
	}
	Expect(k8sClient.Create(ctx, pod)).To(Succeed())

	pod.Status.Phase = corev1.PodPending
	pod.Status.ContainerStatuses = []corev1.ContainerStatus{{
		Name: "restore",
		State: corev1.ContainerState{Waiting: &corev1.ContainerStateWaiting{
			Reason:  "CreateContainerConfigError",
			Message: "secret \"app-user\" not found",
		}},
	}}
	Expect(k8sClient.Status().Update(ctx, pod)).To(Succeed())
}

// createForeignJob creates a completed Job under name that no restore of the
// suite owns. It stands in for the Job of a restore that somebody deleted and
// created again under the same name.
func createForeignJob(w *world, name string) {
	GinkgoHelper()

	job := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: w.namespace},
		Spec: batchv1.JobSpec{
			Template: corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{
					RestartPolicy: corev1.RestartPolicyNever,
					Containers:    []corev1.Container{{Name: "restore", Image: "camunda/camunda:8.9.9"}},
				},
			},
		},
	}
	Expect(k8sClient.Create(ctx, job)).To(Succeed())
	markJob(w.namespace, name, batchv1.JobComplete)
}

// claimNamed reads one broker data claim.
func claimNamed(namespace, name string) *corev1.PersistentVolumeClaim {
	GinkgoHelper()

	var claim corev1.PersistentVolumeClaim
	Eventually(func(g Gomega) {
		g.Expect(k8sClient.Get(
			ctx, types.NamespacedName{Namespace: namespace, Name: name}, &claim,
		)).To(Succeed())
	}, timeout, interval).Should(Succeed())

	return &claim
}

// envValue returns the literal value of an environment variable of the
// container, or the empty string.
func envValue(container corev1.Container, name string) string {
	for _, env := range container.Env {
		if env.Name == name {
			return env.Value
		}
	}

	return ""
}

// secretOfEnv returns the Secret that an environment variable of the
// container reads from, or the empty string.
func secretOfEnv(container corev1.Container, name string) string {
	for _, env := range container.Env {
		if env.Name == name && env.ValueFrom != nil && env.ValueFrom.SecretKeyRef != nil {
			return env.ValueFrom.SecretKeyRef.Name
		}
	}

	return ""
}
