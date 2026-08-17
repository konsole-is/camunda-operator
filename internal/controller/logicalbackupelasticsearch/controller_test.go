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

package logicalbackupelasticsearch

import (
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	utilrand "k8s.io/apimachinery/pkg/util/rand"
	"sigs.k8s.io/controller-runtime/pkg/client"

	v1 "github.com/konsole-is/camunda-operator/api/v1"
	"github.com/konsole-is/camunda-operator/pkg/camundaadmin/camundaadmintest"
	"github.com/konsole-is/camunda-operator/pkg/esadmin"
	"github.com/konsole-is/camunda-operator/pkg/esadmin/esadmintest"
	"github.com/konsole-is/camunda-operator/pkg/refindex"
)

// rig is everything one test needs: the fakes and the referenced resources of
// one Elasticsearch-backed cluster, wired the way production wires them.
type rig struct {
	namespace  string
	management *camundaadmintest.Server
	search     *esadmintest.Server
	repository string
	cluster    *v1.CamundaCluster
}

// newNamespace creates a unique namespace for one test.
func newNamespace() string {
	GinkgoHelper()
	namespace := "lbes-ns-" + utilrand.String(8)
	Expect(k8sClient.Create(ctx, &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{Name: namespace},
	})).To(Succeed())
	return namespace
}

// newRig builds the fakes, the contracts, and the cluster, and publishes the
// management binding. Everything is cleaned up in reverse order, the backup
// CRs first, so the finalizer still reaches the fakes.
func newRig() *rig {
	GinkgoHelper()

	r := &rig{namespace: newNamespace()}
	r.management = camundaadmintest.New()
	DeferCleanup(r.management.Close)
	r.search = esadmintest.NewTLS()
	DeferCleanup(r.search.Close)

	credentials := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: "es-credentials", Namespace: r.namespace},
		Data:       map[string][]byte{"username": []byte("camunda"), "password": []byte("secret")},
	}
	Expect(k8sClient.Create(ctx, credentials)).To(Succeed())

	ca := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: "es-ca", Namespace: r.namespace},
		Data:       map[string][]byte{"ca.crt": r.search.CertificatePEM()},
	}
	Expect(k8sClient.Create(ctx, ca)).To(Succeed())

	storage := &v1.SecondaryStorageConfig{
		ObjectMeta: metav1.ObjectMeta{Name: "storage", Namespace: r.namespace},
		Spec: v1.SecondaryStorageConfigSpec{
			Type: v1.SecondaryStorageTypeElasticsearch,
			Elasticsearch: &v1.ElasticsearchStorage{
				Endpoint: r.search.URL(),
				CredentialsSecretRef: v1.CredentialsSecretRef{
					Name:        credentials.Name,
					Namespace:   r.namespace,
					UsernameKey: "username",
					PasswordKey: "password",
				},
				CASecretRef: &v1.SecretKeyRef{Name: ca.Name, Namespace: r.namespace, Key: "ca.crt"},
			},
		},
	}
	Expect(k8sClient.Create(ctx, storage)).To(Succeed())

	platform := &v1.CamundaPlatformConfig{
		ObjectMeta: metav1.ObjectMeta{Name: "platform-" + utilrand.String(8)},
	}
	Expect(k8sClient.Create(ctx, platform)).To(Succeed())
	DeferCleanup(func() { _ = k8sClient.Delete(ctx, platform) })

	bucket := &v1.ObjectStorageConfig{
		ObjectMeta: metav1.ObjectMeta{Name: "bucket-" + utilrand.String(8)},
		Spec: v1.ObjectStorageConfigSpec{
			Type: v1.ObjectStorageTypeS3,
			S3: &v1.S3Storage{
				BucketName: "backups",
				Region:     "eu-west-1",
				Auth:       v1.S3StorageAuth{Type: v1.ObjectStorageAuthTypeWorkloadIdentity},
			},
		},
	}
	Expect(k8sClient.Create(ctx, bucket)).To(Succeed())
	DeferCleanup(func() { _ = k8sClient.Delete(ctx, bucket) })

	r.cluster = &v1.CamundaCluster{
		ObjectMeta: metav1.ObjectMeta{Name: "cc-" + utilrand.String(8), Namespace: r.namespace},
		Spec: v1.CamundaClusterSpec{
			PlatformConfigRef: platform.Name,
			Version:           "8.9.9",
			StorageRef:        storage.Name,
			BackupStorageRef:  bucket.Name,
		},
	}
	Expect(k8sClient.Create(ctx, r.cluster)).To(Succeed())

	r.repository = r.cluster.Name
	// In production the ElasticsearchCluster controller registers the
	// repository before any backup runs; the rig plays that part.
	admin, err := esadmin.New(r.search.URL(), "elastic", "elastic-password", r.search.CertificatePEM())
	Expect(err).NotTo(HaveOccurred())
	Expect(admin.EnsureSnapshotRepository(ctx, r.repository, esadmin.S3RepositoryConfig{
		Bucket:   "backups",
		BasePath: r.namespace + "/" + r.cluster.Name,
	})).To(Succeed())

	r.publishBinding(3)

	return r
}

// publishBinding writes the management binding the way the CamundaCluster
// controller publishes it. The suite runs no CamundaCluster controller, so
// the tests own the status.
func (r *rig) publishBinding(partitions int32) {
	GinkgoHelper()
	Eventually(func(g Gomega) {
		var cluster v1.CamundaCluster
		g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(r.cluster), &cluster)).To(Succeed())
		cluster.Status.Management = &v1.ManagementBinding{
			Endpoint:         r.management.URL(),
			Auth:             v1.ManagementAuth{Method: v1.ManagementAuthMethodNone},
			Version:          "8.9.9",
			Partitions:       partitions,
			BackupRepository: r.repository,
		}
		g.Expect(k8sClient.Status().Update(ctx, &cluster)).To(Succeed())
	}, timeout, interval).Should(Succeed())
}

// newBackup creates a backup of the rig's cluster and registers a cleanup
// that never blocks the suite: when the ordinary deletion cannot finish, the
// finalizer is stripped.
func (r *rig) newBackup() *v1.LogicalBackupElasticsearch {
	GinkgoHelper()

	backup := &v1.LogicalBackupElasticsearch{
		ObjectMeta: metav1.ObjectMeta{Name: "backup-" + utilrand.String(8), Namespace: r.namespace},
		Spec:       v1.LogicalBackupElasticsearchSpec{ClusterRef: v1.ClusterRef{Name: r.cluster.Name}},
	}
	Expect(k8sClient.Create(ctx, backup)).To(Succeed())
	DeferCleanup(func() {
		_ = k8sClient.Delete(ctx, backup)
		Eventually(func() bool {
			var current v1.LogicalBackupElasticsearch
			err := k8sClient.Get(ctx, client.ObjectKeyFromObject(backup), &current)
			if err != nil {
				return true
			}
			current.Finalizers = nil
			_ = k8sClient.Update(ctx, &current)
			return false
		}, timeout, interval).Should(BeTrue())
	})

	return backup
}

// currentBackup re-reads the backup.
func currentBackup(backup *v1.LogicalBackupElasticsearch) *v1.LogicalBackupElasticsearch {
	GinkgoHelper()
	var current v1.LogicalBackupElasticsearch
	Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(backup), &current)).To(Succeed())
	return &current
}

// backupID waits for the controller to allocate the id.
func backupID(backup *v1.LogicalBackupElasticsearch) int64 {
	GinkgoHelper()
	var id int64
	Eventually(func(g Gomega) {
		id = currentBackup(backup).Status.BackupID
		g.Expect(id).NotTo(BeZero())
	}, timeout, interval).Should(Succeed())
	return id
}

// expectReady waits for the Ready condition to carry the given status and
// reason.
func expectReady(backup *v1.LogicalBackupElasticsearch, status metav1.ConditionStatus, reason string) {
	GinkgoHelper()
	Eventually(func(g Gomega) {
		ready := meta.FindStatusCondition(currentBackup(backup).Status.Conditions, v1.ConditionReady)
		g.Expect(ready).NotTo(BeNil())
		g.Expect(ready.Status).To(Equal(status))
		g.Expect(ready.Reason).To(Equal(reason))
	}, timeout, interval).Should(Succeed())
}

// expectPhase waits for the phase.
func expectPhase(backup *v1.LogicalBackupElasticsearch, phase v1.LogicalBackupPhase) {
	GinkgoHelper()
	Eventually(func(g Gomega) {
		g.Expect(currentBackup(backup).Status.Phase).To(Equal(phase))
	}, timeout, interval).Should(Succeed())
}

// expectEvent polls until an event with the given reason and type exists for
// the backup.
func expectEvent(backup *v1.LogicalBackupElasticsearch, reason, eventType string) {
	GinkgoHelper()
	Eventually(func(g Gomega) {
		var events corev1.EventList
		g.Expect(k8sClient.List(ctx, &events, client.InNamespace(backup.Namespace))).To(Succeed())
		g.Expect(events.Items).To(ContainElement(SatisfyAll(
			HaveField("Reason", reason),
			HaveField("InvolvedObject.Name", backup.Name),
			HaveField("Type", eventType),
		)))
	}, timeout, interval).Should(Succeed())
}

var _ = Describe("LogicalBackupElasticsearch controller", func() {
	It("takes a backup as one coordinated set and resumes exporting", func() {
		r := newRig()
		// A partition count that only this test uses proves the recorded
		// value flows from the binding rather than a fixture default.
		r.publishBinding(5)
		// The statistics stay unavailable through the start, so the recorded
		// size must be backfilled by a later reconcile, not lost forever.
		r.search.FailNext("stats", 1000000)
		Eventually(func(g Gomega) {
			var cluster v1.CamundaCluster
			g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(r.cluster), &cluster)).To(Succeed())
			cluster.Status.Volumes = []v1.VolumeStatus{{Name: "data-0", Capacity: resource.MustParse("20Gi")}}
			g.Expect(k8sClient.Status().Update(ctx, &cluster)).To(Succeed())
		}, timeout, interval).Should(Succeed())

		backup := r.newBackup()
		id := backupID(backup)

		r.search.SetNodeFS("node-0", 100<<30, 40<<30)
		r.search.FailNext("stats", 0)

		By("pausing exporting softly and backing up the web-application indices")
		Eventually(func() int { return r.management.HistoryStarts(id) }, timeout, interval).Should(Equal(1))
		Expect(r.management.Exporting()).To(Equal("softPaused"))
		r.management.SetHistoryState(id, "COMPLETED", "")

		By("snapshotting the exported record indices")
		name := RecordsSnapshotName(id)
		Eventually(func() int {
			return r.search.SnapshotCreates(r.repository, name)
		}, timeout, interval).Should(Equal(1))
		r.search.SetSnapshotState(r.repository, name, "SUCCESS")

		By("backing up the Zeebe partitions")
		Eventually(func() int { return r.management.RuntimeStarts(id) }, timeout, interval).Should(Equal(1))
		r.management.SetRuntimeState(id, "COMPLETED", "")

		By("resuming exporting and completing")
		expectPhase(backup, v1.LogicalBackupCompleted)
		expectReady(backup, metav1.ConditionTrue, v1.ReasonCompleted)
		expectEvent(backup, eventReasonCompleted, corev1.EventTypeNormal)
		Expect(r.management.Exporting()).To(Equal("running"))

		final := currentBackup(backup)
		Expect(final.Status.PartitionsCount).To(Equal(int32(5)))
		Expect(final.Status.Repository).To(Equal(r.repository))
		Expect(final.Status.CompletionTime).NotTo(BeNil())
		Expect(final.Status.HistorySnapshots).NotTo(BeEmpty())
		Expect(final.Status.History.State).To(Equal(v1.BackupPartCompleted))
		Expect(final.Status.Records.State).To(Equal(v1.BackupPartCompleted))
		Expect(final.Status.Runtime.State).To(Equal(v1.BackupPartCompleted))
		Expect(final.Status.StorageSizes.Zeebe.String()).To(Equal("20Gi"))
		// 40Gi used of 100Gi total: used times 1.5 is 60Gi, under the total.
		Expect(final.Status.StorageSizes.Elasticsearch.String()).To(Equal("60Gi"))

		By("never repeating a start across the many reconciles that ran")
		Expect(r.management.PauseCalls()).To(Equal(1))
		Expect(r.management.HistoryStarts(id)).To(Equal(1))
		Expect(r.search.SnapshotCreates(r.repository, name)).To(Equal(1))
		Expect(r.management.RuntimeStarts(id)).To(Equal(1))
	})

	It("routes a step failure through resume before the terminal phase", func() {
		r := newRig()
		r.management.FailNext("historyStart", 1)

		backup := r.newBackup()
		id := backupID(backup)

		expectPhase(backup, v1.LogicalBackupFailed)
		expectReady(backup, metav1.ConditionFalse, v1.ReasonFailed)

		final := currentBackup(backup)
		Expect(final.Status.FailureMessage).To(ContainSubstring("BackupHistory"))
		Expect(final.Status.History.State).To(Equal(v1.BackupPartFailed))
		Expect(r.management.Exporting()).To(Equal("running"))
		Expect(r.management.HistoryStarts(id)).To(BeZero())
	})

	It("fails the runtime part with the reported reason, exporting resumed", func() {
		r := newRig()
		backup := r.newBackup()
		id := backupID(backup)

		Eventually(func() int { return r.management.HistoryStarts(id) }, timeout, interval).Should(Equal(1))
		r.management.SetHistoryState(id, "COMPLETED", "")
		name := RecordsSnapshotName(id)
		Eventually(func() int {
			return r.search.SnapshotCreates(r.repository, name)
		}, timeout, interval).Should(Equal(1))
		r.search.SetSnapshotState(r.repository, name, "SUCCESS")
		Eventually(func() int { return r.management.RuntimeStarts(id) }, timeout, interval).Should(Equal(1))
		r.management.SetRuntimeState(id, "FAILED", "partition 2 lost its snapshot")

		expectPhase(backup, v1.LogicalBackupFailed)
		final := currentBackup(backup)
		Expect(final.Status.Runtime.State).To(Equal(v1.BackupPartFailed))
		Expect(final.Status.Runtime.FailureReason).To(ContainSubstring("partition 2"))
		Expect(final.Status.FailureMessage).To(ContainSubstring("BackupRuntime"))
		Expect(r.management.Exporting()).To(Equal("running"))
	})

	It("gives up on resume at the deadline with ResumeFailed", func() {
		r := newRig()
		// Resume never succeeds; the deadline of the suite is two seconds.
		r.management.FailNext("resume", 100000)
		r.management.FailNext("historyStart", 1)

		backup := r.newBackup()

		expectPhase(backup, v1.LogicalBackupFailed)
		expectReady(backup, metav1.ConditionFalse, v1.ReasonResumeFailed)
		expectEvent(backup, eventReasonResumeFailed, corev1.EventTypeWarning)
	})

	It("serializes backups of one cluster", func() {
		r := newRig()

		first := r.newBackup()
		id := backupID(first)

		second := r.newBackup()
		expectReady(second, metav1.ConditionFalse, v1.ReasonBackupInProgress)
		Expect(currentBackup(second).Status.Phase).To(Equal(v1.LogicalBackupPending))

		By("proceeding once the first backup finishes")
		r.management.SetHistoryState(id, "COMPLETED", "")
		name := RecordsSnapshotName(id)
		Eventually(func() int {
			return r.search.SnapshotCreates(r.repository, name)
		}, timeout, interval).Should(Equal(1))
		r.search.SetSnapshotState(r.repository, name, "SUCCESS")
		Eventually(func() int { return r.management.RuntimeStarts(id) }, timeout, interval).Should(Equal(1))
		r.management.SetRuntimeState(id, "COMPLETED", "")
		expectPhase(first, v1.LogicalBackupCompleted)

		Eventually(func() v1.LogicalBackupPhase {
			return currentBackup(second).Status.Phase
		}, timeout, interval).Should(Equal(v1.LogicalBackupRunning))
	})

	It("waits on a sibling backup of the other kind", func() {
		r := newRig()
		key := refindex.NamespacedKey(r.namespace, r.cluster.Name)
		siblings.Store(key, "rdbms-backup-of-the-same-cluster")
		DeferCleanup(func() { siblings.Delete(key) })

		backup := r.newBackup()
		expectReady(backup, metav1.ConditionFalse, v1.ReasonBackupInProgress)

		siblings.Delete(key)
		Eventually(func() v1.LogicalBackupPhase {
			return currentBackup(backup).Status.Phase
		}, timeout, interval).Should(Equal(v1.LogicalBackupRunning))
	})

	It("stays Pending until the cluster publishes its binding", func() {
		r := newRig()
		Eventually(func(g Gomega) {
			var cluster v1.CamundaCluster
			g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(r.cluster), &cluster)).To(Succeed())
			cluster.Status.Management = nil
			g.Expect(k8sClient.Status().Update(ctx, &cluster)).To(Succeed())
		}, timeout, interval).Should(Succeed())

		backup := r.newBackup()
		expectReady(backup, metav1.ConditionFalse, v1.ReasonProgressing)
		Expect(currentBackup(backup).Status.Phase).To(Equal(v1.LogicalBackupPending))
		Expect(currentBackup(backup).Status.BackupID).To(BeZero())

		By("starting once the binding appears")
		r.publishBinding(3)
		expectPhase(backup, v1.LogicalBackupRunning)
	})

	It("reports the pre-check reasons", func() {
		namespace := newNamespace()

		By("a missing cluster")
		orphan := &v1.LogicalBackupElasticsearch{
			ObjectMeta: metav1.ObjectMeta{Name: "orphan", Namespace: namespace},
			Spec:       v1.LogicalBackupElasticsearchSpec{ClusterRef: v1.ClusterRef{Name: "no-such-cluster"}},
		}
		Expect(k8sClient.Create(ctx, orphan)).To(Succeed())
		DeferCleanup(func() { _ = k8sClient.Delete(ctx, orphan) })
		expectReady(orphan, metav1.ConditionFalse, v1.ReasonInvalidReference)

		By("a suspended cluster")
		r := newRig()
		Eventually(func(g Gomega) {
			var cluster v1.CamundaCluster
			g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(r.cluster), &cluster)).To(Succeed())
			cluster.Spec.Suspend = true
			g.Expect(k8sClient.Update(ctx, &cluster)).To(Succeed())
		}, timeout, interval).Should(Succeed())
		suspended := r.newBackup()
		expectReady(suspended, metav1.ConditionFalse, v1.ReasonClusterSuspended)
	})

	It("deletes the artifacts by their exact id on deletion", func() {
		r := newRig()
		backup := r.newBackup()
		id := backupID(backup)

		Eventually(func() int { return r.management.HistoryStarts(id) }, timeout, interval).Should(Equal(1))
		r.management.SetHistoryState(id, "COMPLETED", "")
		name := RecordsSnapshotName(id)
		Eventually(func() int {
			return r.search.SnapshotCreates(r.repository, name)
		}, timeout, interval).Should(Equal(1))
		r.search.SetSnapshotState(r.repository, name, "SUCCESS")
		Eventually(func() int { return r.management.RuntimeStarts(id) }, timeout, interval).Should(Equal(1))
		r.management.SetRuntimeState(id, "COMPLETED", "")
		expectPhase(backup, v1.LogicalBackupCompleted)

		snapshots := currentBackup(backup).Status.HistorySnapshots
		Expect(snapshots).NotTo(BeEmpty())

		Expect(k8sClient.Delete(ctx, backup)).To(Succeed())
		Eventually(func() bool {
			var gone v1.LogicalBackupElasticsearch
			return client.IgnoreNotFound(k8sClient.Get(ctx, client.ObjectKeyFromObject(backup), &gone)) == nil &&
				k8sClient.Get(ctx, client.ObjectKeyFromObject(backup), &gone) != nil
		}, timeout, interval).Should(BeTrue())

		Expect(r.search.SnapshotExists(r.repository, name)).To(BeFalse())
		for _, snapshot := range snapshots {
			Expect(r.search.SnapshotExists(r.repository, snapshot)).To(BeFalse())
		}
		runtime := r.management.RuntimeBackup(id)
		if runtime != nil {
			Expect(runtime.State).To(Equal("DELETED"))
		}
	})

	It("releases the finalizer when the cluster is gone", func() {
		r := newRig()
		backup := r.newBackup()
		backupID(backup)

		Expect(k8sClient.Delete(ctx, r.cluster)).To(Succeed())
		Eventually(func() bool {
			var gone v1.CamundaCluster
			return k8sClient.Get(ctx, client.ObjectKeyFromObject(r.cluster), &gone) != nil
		}, timeout, interval).Should(BeTrue())

		Expect(k8sClient.Delete(ctx, backup)).To(Succeed())
		Eventually(func() bool {
			var gone v1.LogicalBackupElasticsearch
			return k8sClient.Get(ctx, client.ObjectKeyFromObject(backup), &gone) != nil
		}, timeout, interval).Should(BeTrue())
		expectEvent(backup, eventReasonReleased, corev1.EventTypeWarning)
	})

	It("routes a pause failure through resume before the terminal phase", func() {
		r := newRig()
		r.management.FailNext("pause", 1)

		backup := r.newBackup()
		id := backupID(backup)

		expectPhase(backup, v1.LogicalBackupFailed)
		final := currentBackup(backup)
		Expect(final.Status.FailureMessage).To(ContainSubstring("PauseExporting"))
		Expect(r.management.Exporting()).To(Equal("running"))
		Expect(r.management.HistoryStarts(id)).To(BeZero())
	})

	It("routes a records-snapshot failure through resume before the terminal phase", func() {
		r := newRig()
		backup := r.newBackup()
		id := backupID(backup)

		Eventually(func() int { return r.management.HistoryStarts(id) }, timeout, interval).Should(Equal(1))
		r.management.SetHistoryState(id, "COMPLETED", "")

		name := RecordsSnapshotName(id)
		Eventually(func() int {
			return r.search.SnapshotCreates(r.repository, name)
		}, timeout, interval).Should(Equal(1))
		r.search.SetSnapshotState(r.repository, name, "FAILED")

		expectPhase(backup, v1.LogicalBackupFailed)
		final := currentBackup(backup)
		Expect(final.Status.Records.State).To(Equal(v1.BackupPartFailed))
		Expect(final.Status.FailureMessage).To(ContainSubstring("SnapshotRecords"))
		Expect(r.management.Exporting()).To(Equal("running"))
		Expect(r.management.RuntimeStarts(id)).To(BeZero())
	})

	It("fails on a fresh-id conflict instead of adopting another backup", func() {
		r := newRig()
		// The cluster already holds a far higher id, as if another actor
		// backed it up: every fresh id now conflicts.
		r.management.SetRuntimeState(9_000_000_000_000_000, "COMPLETED", "")

		backup := r.newBackup()
		id := backupID(backup)

		Eventually(func() int { return r.management.HistoryStarts(id) }, timeout, interval).Should(Equal(1))
		r.management.SetHistoryState(id, "COMPLETED", "")
		name := RecordsSnapshotName(id)
		Eventually(func() int {
			return r.search.SnapshotCreates(r.repository, name)
		}, timeout, interval).Should(Equal(1))
		r.search.SetSnapshotState(r.repository, name, "SUCCESS")

		expectPhase(backup, v1.LogicalBackupFailed)
		final := currentBackup(backup)
		Expect(final.Status.FailureMessage).To(ContainSubstring("BackupRuntime"))
		Expect(final.Status.FailureMessage).To(ContainSubstring("already exists"))
		Expect(r.management.RuntimeStarts(id)).To(BeZero())
		Expect(r.management.Exporting()).To(Equal("running"))
	})

	It("holds the step with ConnectionFailed while the endpoint is unreachable", func() {
		r := newRig()
		// The binding points at a closed server: reachable never, refused
		// always.
		r.management.Close()

		backup := r.newBackup()
		backupID(backup)

		expectReady(backup, metav1.ConditionFalse, v1.ReasonConnectionFailed)
		Consistently(func(g Gomega) {
			current := currentBackup(backup)
			g.Expect(current.Status.Step).To(Equal(v1.StepPauseExporting))
			g.Expect(current.Status.Phase).To(Equal(v1.LogicalBackupRunning))
		}, "1s", interval).Should(Succeed())
	})

	It("keeps a deletion waiting while the cluster exists without a binding", func() {
		r := newRig()
		backup := r.newBackup()
		backupID(backup)

		Eventually(func(g Gomega) {
			var cluster v1.CamundaCluster
			g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(r.cluster), &cluster)).To(Succeed())
			cluster.Status.Management = nil
			g.Expect(k8sClient.Status().Update(ctx, &cluster)).To(Succeed())
		}, timeout, interval).Should(Succeed())

		Expect(k8sClient.Delete(ctx, backup)).To(Succeed())
		Consistently(func(g Gomega) {
			var held v1.LogicalBackupElasticsearch
			g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(backup), &held)).To(Succeed())
		}, "1s", interval).Should(Succeed())

		By("completing the deletion once the binding is back")
		r.publishBinding(3)
		Eventually(func() bool {
			var gone v1.LogicalBackupElasticsearch
			return k8sClient.Get(ctx, client.ObjectKeyFromObject(backup), &gone) != nil
		}, timeout, interval).Should(BeTrue())
	})

	It("parks a running procedure in place while the binding is gone", func() {
		r := newRig()
		backup := r.newBackup()
		id := backupID(backup)

		Eventually(func() int { return r.management.HistoryStarts(id) }, timeout, interval).Should(Equal(1))

		By("clearing the binding mid-run")
		Eventually(func(g Gomega) {
			var cluster v1.CamundaCluster
			g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(r.cluster), &cluster)).To(Succeed())
			cluster.Status.Management = nil
			g.Expect(k8sClient.Status().Update(ctx, &cluster)).To(Succeed())
		}, timeout, interval).Should(Succeed())

		Consistently(func(g Gomega) {
			current := currentBackup(backup)
			g.Expect(current.Status.Phase).To(Equal(v1.LogicalBackupRunning))
			g.Expect(current.Status.Step).To(Equal(v1.StepBackupHistory))
			g.Expect(current.Status.BackupID).To(Equal(id))
		}, "1s", interval).Should(Succeed())

		By("continuing where it parked once the binding is back")
		r.publishBinding(3)
		r.management.SetHistoryState(id, "COMPLETED", "")
		name := RecordsSnapshotName(id)
		Eventually(func() int {
			return r.search.SnapshotCreates(r.repository, name)
		}, timeout, interval).Should(Equal(1))
		r.search.SetSnapshotState(r.repository, name, "SUCCESS")
		Eventually(func() int { return r.management.RuntimeStarts(id) }, timeout, interval).Should(Equal(1))
		r.management.SetRuntimeState(id, "COMPLETED", "")
		expectPhase(backup, v1.LogicalBackupCompleted)

		By("never having re-started")
		Expect(r.management.HistoryStarts(id)).To(Equal(1))
	})

	It("routes a mid-run storage loss through resume to a terminal phase", func() {
		r := newRig()
		backup := r.newBackup()
		id := backupID(backup)

		Eventually(func() int { return r.management.HistoryStarts(id) }, timeout, interval).Should(Equal(1))

		By("deleting the storage contract before the records snapshot")
		var storage v1.SecondaryStorageConfig
		Expect(k8sClient.Get(
			ctx, client.ObjectKey{Namespace: r.namespace, Name: "storage"}, &storage,
		)).To(Succeed())
		Expect(k8sClient.Delete(ctx, &storage)).To(Succeed())
		r.management.SetHistoryState(id, "COMPLETED", "")

		expectPhase(backup, v1.LogicalBackupFailed)
		final := currentBackup(backup)
		Expect(final.Status.FailureMessage).To(ContainSubstring("SnapshotRecords"))
		Expect(final.Status.Records.State).To(Equal(v1.BackupPartFailed))
		Expect(r.management.Exporting()).To(Equal("running"))
	})

	It("fails the step when the repository is repointed mid-run", func() {
		r := newRig()
		backup := r.newBackup()
		id := backupID(backup)

		Eventually(func() int { return r.management.HistoryStarts(id) }, timeout, interval).Should(Equal(1))

		By("repointing the published repository before the records snapshot")
		r.repository = "another-repository"
		r.publishBinding(3)
		r.management.SetHistoryState(id, "COMPLETED", "")

		expectPhase(backup, v1.LogicalBackupFailed)
		final := currentBackup(backup)
		Expect(final.Status.FailureMessage).To(ContainSubstring("changed"))
		Expect(final.Status.Records.State).To(Equal(v1.BackupPartFailed))
		Expect(r.management.Exporting()).To(Equal("running"))
	})

	It("resumes exporting before a mid-run deletion releases", func() {
		r := newRig()
		backup := r.newBackup()
		id := backupID(backup)

		Eventually(func() int { return r.management.HistoryStarts(id) }, timeout, interval).Should(Equal(1))
		r.management.SetHistoryState(id, "COMPLETED", "")
		name := RecordsSnapshotName(id)
		Eventually(func() int {
			return r.search.SnapshotCreates(r.repository, name)
		}, timeout, interval).Should(Equal(1))
		r.search.SetSnapshotState(r.repository, name, "SUCCESS")
		Eventually(func() int { return r.management.RuntimeStarts(id) }, timeout, interval).Should(Equal(1))

		By("deleting the backup while the partition backup is in progress")
		Expect(k8sClient.Delete(ctx, backup)).To(Succeed())

		By("resuming exporting first, holding on the undeletable runtime backup")
		Eventually(func() string { return r.management.Exporting() }, timeout, interval).Should(Equal("running"))
		expectEvent(backup, eventReasonDeleteHeld, corev1.EventTypeWarning)
		Consistently(func(g Gomega) {
			var held v1.LogicalBackupElasticsearch
			g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(backup), &held)).To(Succeed())
		}, "1s", interval).Should(Succeed())

		By("releasing once the runtime backup is deletable")
		r.management.SetRuntimeState(id, "COMPLETED", "")
		Eventually(func() bool {
			var gone v1.LogicalBackupElasticsearch
			return k8sClient.Get(ctx, client.ObjectKeyFromObject(backup), &gone) != nil
		}, timeout, interval).Should(BeTrue())
		Expect(r.search.SnapshotExists(r.repository, name)).To(BeFalse())
	})

	It("releases with an event when a client cannot be built anymore", func() {
		r := newRig()
		backup := r.newBackup()
		id := backupID(backup)

		Eventually(func() int { return r.management.HistoryStarts(id) }, timeout, interval).Should(Equal(1))
		r.management.SetHistoryState(id, "COMPLETED", "")
		name := RecordsSnapshotName(id)
		Eventually(func() int {
			return r.search.SnapshotCreates(r.repository, name)
		}, timeout, interval).Should(Equal(1))
		r.search.SetSnapshotState(r.repository, name, "SUCCESS")
		Eventually(func() int { return r.management.RuntimeStarts(id) }, timeout, interval).Should(Equal(1))
		r.management.SetRuntimeState(id, "COMPLETED", "")
		expectPhase(backup, v1.LogicalBackupCompleted)

		By("deleting the Elasticsearch credentials, then the backup")
		Expect(k8sClient.Delete(ctx, &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{Name: "es-credentials", Namespace: r.namespace},
		})).To(Succeed())

		Expect(k8sClient.Delete(ctx, backup)).To(Succeed())
		Eventually(func() bool {
			var gone v1.LogicalBackupElasticsearch
			return k8sClient.Get(ctx, client.ObjectKeyFromObject(backup), &gone) != nil
		}, timeout, interval).Should(BeTrue())
		expectEvent(backup, eventReasonReleased, corev1.EventTypeWarning)
	})

	It("does not count parked time against the resume deadline", func() {
		r := newRig()
		r.management.FailNext("historyStart", 1)
		r.management.FailNext("resume", 1000000)

		backup := r.newBackup()
		backupID(backup)

		By("reaching the failing resume attempts")
		Eventually(func(g Gomega) {
			g.Expect(currentBackup(backup).Status.Step).To(Equal(v1.StepResumeExporting))
			g.Expect(currentBackup(backup).Status.LastResumeAttemptTime).NotTo(BeNil())
		}, timeout, interval).Should(Succeed())

		By("parking longer than the whole deadline")
		Eventually(func(g Gomega) {
			var cluster v1.CamundaCluster
			g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(r.cluster), &cluster)).To(Succeed())
			cluster.Status.Management = nil
			g.Expect(k8sClient.Status().Update(ctx, &cluster)).To(Succeed())
		}, timeout, interval).Should(Succeed())
		time.Sleep(2500 * time.Millisecond)

		By("resuming successfully after the park: the step failure ends it, not the deadline")
		r.management.FailNext("resume", 0)
		r.publishBinding(3)

		expectPhase(backup, v1.LogicalBackupFailed)
		expectReady(backup, metav1.ConditionFalse, v1.ReasonFailed)
		final := currentBackup(backup)
		Expect(final.Status.TerminalReason).To(Equal(v1.ReasonFailed))
		Expect(final.Status.FailureMessage).To(ContainSubstring("BackupHistory"))
		Expect(r.management.Exporting()).To(Equal("running"))
	})

	It("re-stages the terminal condition when a conflict restored an older one", func() {
		r := newRig()
		backup := r.newBackup()
		id := backupID(backup)

		Eventually(func() int { return r.management.HistoryStarts(id) }, timeout, interval).Should(Equal(1))
		r.management.SetHistoryState(id, "COMPLETED", "")
		name := RecordsSnapshotName(id)
		Eventually(func() int {
			return r.search.SnapshotCreates(r.repository, name)
		}, timeout, interval).Should(Equal(1))
		r.search.SetSnapshotState(r.repository, name, "SUCCESS")
		Eventually(func() int { return r.management.RuntimeStarts(id) }, timeout, interval).Should(Equal(1))
		r.management.SetRuntimeState(id, "COMPLETED", "")
		expectPhase(backup, v1.LogicalBackupCompleted)
		expectReady(backup, metav1.ConditionTrue, v1.ReasonCompleted)

		By("corrupting the terminal condition the way a lost write conflict would")
		Eventually(func(g Gomega) {
			current := currentBackup(backup)
			meta.SetStatusCondition(&current.Status.Conditions, metav1.Condition{
				Type:               v1.ConditionReady,
				Status:             metav1.ConditionFalse,
				Reason:             v1.ReasonProgressing,
				Message:            "stale",
				ObservedGeneration: current.Generation,
			})
			g.Expect(k8sClient.Status().Update(ctx, current)).To(Succeed())
		}, timeout, interval).Should(Succeed())

		expectReady(backup, metav1.ConditionTrue, v1.ReasonCompleted)
	})
})
