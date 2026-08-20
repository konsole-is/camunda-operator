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

package logicalrestore

import (
	"maps"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	v1 "github.com/konsole-is/camunda-operator/api/v1"
	components "github.com/konsole-is/camunda-operator/pkg/components/logicalrestore"
	"github.com/konsole-is/camunda-operator/pkg/labels"
	restorepkg "github.com/konsole-is/camunda-operator/pkg/restore"
)

var _ = Describe("LogicalRestore admission", func() {
	It("holds a restore whose target still runs", func() {
		w := newElasticsearchWorld(func(cluster *v1.CamundaCluster) { cluster.Spec.Suspend = false })
		backup := createElasticsearchBackup(w)

		restore := createRestore(w, v1.LogicalBackupKindElasticsearch, backup.Name)

		reached := expectReason(restore, v1.LogicalRestorePending, v1.ReasonClusterNotSuspended)
		Expect(reached.Status.BackupID).To(BeZero())

		By("touching nothing while it waits")
		Consistently(func(g Gomega) {
			g.Expect(w.search.RepositoryPuts(w.repository)).To(BeZero())
			var jobs batchv1.JobList
			g.Expect(k8sClient.List(ctx, &jobs, client.InNamespace(w.namespace))).To(Succeed())
			g.Expect(jobs.Items).To(BeEmpty())
		}, "2s", interval).Should(Succeed())
	})

	It("holds a restore whose backup does not exist", func() {
		w := newElasticsearchWorld()

		restore := createRestore(w, v1.LogicalBackupKindElasticsearch, "no-such-backup")

		expectReason(restore, v1.LogicalRestorePending, v1.ReasonInvalidReference)
	})

	It("holds a restore whose backup is not completed", func() {
		w := newElasticsearchWorld()
		backup := createElasticsearchBackup(w, func(b *v1.LogicalBackupElasticsearch) {
			b.Status.Phase = v1.LogicalBackupRunning
		})

		restore := createRestore(w, v1.LogicalBackupKindElasticsearch, backup.Name)

		reached := expectReason(restore, v1.LogicalRestorePending, v1.ReasonInvalidReference)
		Expect(readyCondition(reached).Message).To(ContainSubstring("Completed backup"))
	})

	It("holds a restore whose target cluster does not exist", func() {
		w := newElasticsearchWorld()
		backup := createElasticsearchBackup(w)

		restore := &v1.LogicalRestore{
			ObjectMeta: metav1.ObjectMeta{Name: "lr-no-cluster", Namespace: w.namespace},
			Spec: v1.LogicalRestoreSpec{
				BackupRef:        v1.LogicalBackupRef{Kind: v1.LogicalBackupKindElasticsearch, Name: backup.Name},
				TargetClusterRef: v1.ClusterRef{Name: "no-such-cluster"},
			},
		}
		Expect(k8sClient.Create(ctx, restore)).To(Succeed())

		expectReason(restore, v1.LogicalRestorePending, v1.ReasonInvalidReference)
	})

	It("pins what it reads once the target is suspended and the backup is completed", func() {
		w := newElasticsearchWorld()
		w.registerRepository()
		backup := createElasticsearchBackup(w)

		restore := createRestore(w, v1.LogicalBackupKindElasticsearch, backup.Name)

		Eventually(func(g Gomega) {
			current := latest(g, restore)
			g.Expect(current.Status.BackupID).To(Equal(backupID))
			g.Expect(current.Status.StorageType).To(Equal(v1.SecondaryStorageTypeElasticsearch))
			g.Expect(current.Status.TargetClusterUID).To(Equal(w.cluster.UID))
		}, timeout, interval).Should(Succeed())
	})

	It("wakes a waiting restore through the cluster watch when the target is suspended", func() {
		w := newElasticsearchWorld(func(cluster *v1.CamundaCluster) { cluster.Spec.Suspend = false })
		w.registerRepository()
		backup := createElasticsearchBackup(w)

		restore := createRestore(w, v1.LogicalBackupKindElasticsearch, backup.Name)
		expectReason(restore, v1.LogicalRestorePending, v1.ReasonClusterNotSuspended)

		By("suspending the target")
		w.suspend(true)

		// The retry interval of the suite is far longer than the timeout, so
		// only the watch can move the restore in time.
		Eventually(func(g Gomega) {
			g.Expect(latest(g, restore).Status.BackupID).To(Equal(backupID))
		}, timeout, interval).Should(Succeed())
	})
})

var _ = Describe("LogicalRestore compatibility", func() {
	It("walks a compatible pair into the secondary storage phase", func() {
		w := newElasticsearchWorld()
		w.registerRepository()
		backup := createElasticsearchBackup(w)

		restore := createRestore(w, v1.LogicalBackupKindElasticsearch, backup.Name)

		expectPhase(restore, v1.LogicalRestoreRestoringSecondaryStorage)
	})

	It("fails a pair whose partition counts differ", func() {
		w := newElasticsearchWorld()
		backup := createElasticsearchBackup(w, func(b *v1.LogicalBackupElasticsearch) {
			b.Status.PartitionsCount = worldPartitions + 4
		})

		restore := createRestore(w, v1.LogicalBackupKindElasticsearch, backup.Name)

		reached := expectReason(restore, v1.LogicalRestoreFailed, v1.ReasonIncompatibleTarget)
		Expect(reached.Status.FailureMessage).To(ContainSubstring("7 partitions"))
		Expect(reached.Status.FailureMessage).To(ContainSubstring("runs 3"))
		Expect(reached.Status.CompletionTime).NotTo(BeNil())
	})

	It("fails a backup that recorded no Camunda version", func() {
		w := newElasticsearchWorld()
		backup := createElasticsearchBackup(w, func(b *v1.LogicalBackupElasticsearch) {
			b.Status.Version = ""
		})

		restore := createRestore(w, v1.LogicalBackupKindElasticsearch, backup.Name)

		reached := expectReason(restore, v1.LogicalRestoreFailed, v1.ReasonIncompatibleTarget)
		Expect(reached.Status.FailureMessage).To(ContainSubstring("did not record"))
	})
})

var _ = Describe("LogicalRestore of a relational cluster", func() {
	It("restores the database with one Job and then the broker volumes", func() {
		w := newRelationalWorld()
		backup := createRelationalBackup(w)

		restore := createRestore(w, v1.LogicalBackupKindRDBMS, backup.Name)

		By("applying exactly one pg_restore Job and recording it")
		var jobName string
		Eventually(func(g Gomega) {
			current := latest(g, restore)
			g.Expect(current.Status.Phase).To(Equal(v1.LogicalRestoreRestoringSecondaryStorage))
			g.Expect(current.Status.SecondaryJobName).NotTo(BeEmpty())
			jobName = current.Status.SecondaryJobName
		}, timeout, interval).Should(Succeed())

		Consistently(func(g Gomega) {
			var jobs batchv1.JobList
			g.Expect(k8sClient.List(ctx, &jobs, client.InNamespace(w.namespace))).To(Succeed())
			g.Expect(jobs.Items).To(HaveLen(1))
		}, "2s", interval).Should(Succeed())

		By("reading the dump of the backup with the backup credentials of the target")
		job := jobNamed(w.namespace, jobName)
		Expect(job.Spec.Template.Spec.Containers).To(HaveLen(1))
		container := job.Spec.Template.Spec.Containers[0]
		Expect(envValue(container, "PGDATABASE")).To(Equal("camunda"))
		Expect(envValue(container, "PGHOST")).To(Equal("postgres.databases.svc"))
		Expect(secretOfEnv(container, "PGUSER")).To(Equal("backup-user"))

		By("moving on to primary storage when the Job completed")
		markJob(w.namespace, jobName, batchv1.JobComplete)
		expectPhase(restore, v1.LogicalRestoreRestoringPrimaryStorage)
	})

	It("refuses a pg_restore Job that another restore left behind under its name", func() {
		w := newRelationalWorld()
		backup := createRelationalBackup(w)

		restore := &v1.LogicalRestore{
			ObjectMeta: metav1.ObjectMeta{Name: "lr-pg-successor", Namespace: w.namespace},
			Spec: v1.LogicalRestoreSpec{
				BackupRef:        v1.LogicalBackupRef{Kind: v1.LogicalBackupKindRDBMS, Name: backup.Name},
				TargetClusterRef: v1.ClusterRef{Name: w.cluster.Name},
			},
		}
		createForeignJob(w, components.JobName(restore))

		Expect(k8sClient.Create(ctx, restore)).To(Succeed())

		reached := expectReason(restore, v1.LogicalRestoreFailed, v1.ReasonFailed)
		Expect(reached.Status.FailureMessage).To(ContainSubstring(components.JobName(restore)))
		Expect(reached.Status.FailureMessage).To(ContainSubstring("another restore"))
	})

	It("fails when the pg_restore Job fails", func() {
		w := newRelationalWorld()
		backup := createRelationalBackup(w)

		restore := createRestore(w, v1.LogicalBackupKindRDBMS, backup.Name)

		var jobName string
		Eventually(func(g Gomega) {
			jobName = latest(g, restore).Status.SecondaryJobName
			g.Expect(jobName).NotTo(BeEmpty())
		}, timeout, interval).Should(Succeed())

		markJob(w.namespace, jobName, batchv1.JobFailed)

		reached := expectReason(restore, v1.LogicalRestoreFailed, v1.ReasonFailed)
		Expect(reached.Status.FailureMessage).To(ContainSubstring(jobName))
	})

	It("reports a pod that cannot start and fails after the grace", func() {
		w := newRelationalWorld()
		backup := createRelationalBackup(w)

		restore := createRestore(w, v1.LogicalBackupKindRDBMS, backup.Name)

		var jobName string
		Eventually(func(g Gomega) {
			jobName = latest(g, restore).Status.SecondaryJobName
			g.Expect(jobName).NotTo(BeEmpty())
		}, timeout, interval).Should(Succeed())

		By("standing in for a kubelet that cannot mount a Secret of the pod")
		createStuckPod(w, jobName, map[string]string{
			"camunda.io/logical-restore-uid": string(restore.UID),
			"camunda.io/component":           "pg-restore",
		})

		expectReason(restore, v1.LogicalRestoreRestoringSecondaryStorage, v1.ReasonMissingSecret)

		reached := expectReason(restore, v1.LogicalRestoreFailed, v1.ReasonFailed)
		Expect(reached.Status.FailureMessage).To(ContainSubstring("did not recover"))
	})

	It("runs the restore application without arguments on the relational path", func() {
		w := newRelationalWorld()
		backup := createRelationalBackup(w)

		restore := createRestore(w, v1.LogicalBackupKindRDBMS, backup.Name)
		completeSecondaryStorage(w, restore)

		expectPhase(restore, v1.LogicalRestoreRestoringPrimaryStorage)
		job := jobNamed(w.namespace, restorepkg.JobName(labels.LogicalRestore(restore.Name), 0))
		Expect(job.Spec.Template.Spec.Containers[0].Args).To(BeEmpty())
	})
})

var _ = Describe("LogicalRestore of primary storage", func() {
	It("recreates the broker volumes, runs one Job per broker, and completes", func() {
		w := newRelationalWorld()
		backup := createRelationalBackup(w)
		restore := createRestore(w, v1.LogicalBackupKindRDBMS, backup.Name)
		completeSecondaryStorage(w, restore)

		By("recording the broker count and the recreated volumes")
		Eventually(func(g Gomega) {
			current := latest(g, restore)
			g.Expect(current.Status.Brokers).To(Equal(worldBrokers))
			g.Expect(current.Status.RecreatedClaims).To(ConsistOf(w.claimNames()))
		}, timeout, interval).Should(Succeed())

		By("running one restore Job per broker, labelled as the restore of this cluster")
		owner := labels.LogicalRestore(restore.Name)
		names := []string{restorepkg.JobName(owner, 0), restorepkg.JobName(owner, 1)}
		Eventually(func(g Gomega) {
			g.Expect(latest(g, restore).Status.PrimaryJobNames).To(Equal(names))
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
		Expect(job.Labels).To(HaveKeyWithValue("camunda.io/component", "restore"))
		Expect(job.Labels).To(HaveKeyWithValue("camunda.io/logical-restore", restore.Name))
		Expect(job.Labels).To(HaveKeyWithValue("camunda.io/cluster", w.cluster.Name))
		// envtest runs no garbage collector, so the controller reference is
		// what proves that deleting the restore removes its Jobs.
		Expect(job.OwnerReferences).To(HaveLen(1))
		Expect(job.OwnerReferences[0].Name).To(Equal(restore.Name))
		Expect(job.OwnerReferences[0].Controller).NotTo(BeNil())

		By("completing when every Job completed")
		for _, name := range names {
			markJob(w.namespace, name, batchv1.JobComplete)
		}
		reached := expectReason(restore, v1.LogicalRestoreCompleted, v1.ReasonCompleted)
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
		w := newRelationalWorld()
		backup := createRelationalBackup(w, func(b *v1.LogicalBackupRDBMS) {
			b.Status.StorageSizes = v1.LogicalBackupStorageSizes{}
		})
		restore := createRestore(w, v1.LogicalBackupKindRDBMS, backup.Name)
		completeSecondaryStorage(w, restore)

		expectPhase(restore, v1.LogicalRestoreRestoringPrimaryStorage)
		Eventually(func(g Gomega) {
			g.Expect(latest(g, restore).Status.PrimaryJobNames).To(HaveLen(int(worldBrokers)))
		}, timeout, interval).Should(Succeed())

		claim := claimNamed(w.namespace, w.claimNames()[0])
		Expect(claim.Spec.Resources.Requests[corev1.ResourceStorage]).To(
			Equal(resource.MustParse(worldClaimSize)),
		)
	})

	It("reports a restore pod that cannot start and fails after the grace", func() {
		w := newRelationalWorld()
		backup := createRelationalBackup(w)
		restore := createRestore(w, v1.LogicalBackupKindRDBMS, backup.Name)
		completeSecondaryStorage(w, restore)

		owner := labels.LogicalRestore(restore.Name)
		Eventually(func(g Gomega) {
			g.Expect(latest(g, restore).Status.PrimaryJobNames).To(HaveLen(int(worldBrokers)))
		}, timeout, interval).Should(Succeed())

		By("standing in for a kubelet that cannot mount a Secret of the pod")
		createStuckPod(w, restorepkg.JobName(owner, 0), restorepkg.JobSelector(owner))

		expectReason(restore, v1.LogicalRestoreRestoringPrimaryStorage, v1.ReasonMissingSecret)

		// The grace runs out only when the clock survives a look that resolved
		// every reference again. A restore that cleared it on each look would
		// hold here for ever.
		reached := expectReason(restore, v1.LogicalRestoreFailed, v1.ReasonFailed)
		Expect(reached.Status.FailureMessage).To(ContainSubstring("did not recover"))
	})

	It("refuses a Job that another restore left behind under its name", func() {
		w := newRelationalWorld()
		backup := createRelationalBackup(w)

		// The name of a restore Job comes from the name of the restore and the
		// broker ordinal, so a restore that somebody deleted and created again
		// finds the Jobs of its predecessor. Only the controller reference
		// tells the two apart.
		restore := &v1.LogicalRestore{
			ObjectMeta: metav1.ObjectMeta{Name: "lr-successor", Namespace: w.namespace},
			Spec: v1.LogicalRestoreSpec{
				BackupRef:        v1.LogicalBackupRef{Kind: v1.LogicalBackupKindRDBMS, Name: backup.Name},
				TargetClusterRef: v1.ClusterRef{Name: w.cluster.Name},
			},
		}
		name := restorepkg.JobName(labels.LogicalRestore(restore.Name), 0)
		createForeignJob(w, name)

		Expect(k8sClient.Create(ctx, restore)).To(Succeed())
		completeSecondaryStorage(w, restore)

		reached := expectReason(restore, v1.LogicalRestoreFailed, v1.ReasonFailed)
		Expect(reached.Status.FailureMessage).To(ContainSubstring(name))
		Expect(reached.Status.FailureMessage).To(ContainSubstring("Remove the Job"))
	})

	It("fails and names the broker when one restore Job fails", func() {
		w := newRelationalWorld()
		backup := createRelationalBackup(w)
		restore := createRestore(w, v1.LogicalBackupKindRDBMS, backup.Name)
		completeSecondaryStorage(w, restore)

		owner := labels.LogicalRestore(restore.Name)
		Eventually(func(g Gomega) {
			g.Expect(latest(g, restore).Status.PrimaryJobNames).To(HaveLen(int(worldBrokers)))
		}, timeout, interval).Should(Succeed())

		markJob(w.namespace, restorepkg.JobName(owner, 0), batchv1.JobComplete)
		markJob(w.namespace, restorepkg.JobName(owner, 1), batchv1.JobFailed)

		reached := expectReason(restore, v1.LogicalRestoreFailed, v1.ReasonFailed)
		Expect(reached.Status.FailureMessage).To(ContainSubstring("broker 1"))
	})
})

// completeSecondaryStorage drives a relational restore through its pg_restore
// Job, so a spec about primary storage starts where that phase ends.
func completeSecondaryStorage(w *world, restore *v1.LogicalRestore) {
	GinkgoHelper()

	var jobName string
	Eventually(func(g Gomega) {
		jobName = latest(g, restore).Status.SecondaryJobName
		g.Expect(jobName).NotTo(BeEmpty())
	}, timeout, interval).Should(Succeed())

	markJob(w.namespace, jobName, batchv1.JobComplete)
}

// createStuckPod creates a pod of a Job of the restore that reports a
// container which cannot start. The selector labels are the ones that the
// controller lists the pods of that Job by. envtest runs no kubelet, so the
// suite writes the state that a kubelet would report.
func createStuckPod(w *world, jobName string, selector map[string]string) {
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
			Message: "secret \"backup-user\" not found",
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
