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

var _ = Describe("LogicalBackupElasticsearch controller", func() {
	It("takes a backup as one coordinated set and resumes exporting", func() {
		r := newRig()
		r.search.SetNodeFS("node-0", 100<<30, 40<<30)
		Eventually(func(g Gomega) {
			var cluster v1.CamundaCluster
			g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(r.cluster), &cluster)).To(Succeed())
			cluster.Status.Volumes = []v1.VolumeStatus{{Name: "data-0", Capacity: resource.MustParse("20Gi")}}
			g.Expect(k8sClient.Status().Update(ctx, &cluster)).To(Succeed())
		}, timeout, interval).Should(Succeed())

		backup := r.newBackup()
		id := backupID(backup)

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
		Expect(r.management.Exporting()).To(Equal("running"))

		final := currentBackup(backup)
		Expect(final.Status.PartitionsCount).To(Equal(int32(3)))
		Expect(final.Status.CompletionTime).NotTo(BeNil())
		Expect(final.Status.HistorySnapshots).NotTo(BeEmpty())
		Expect(final.Status.History.State).To(Equal(v1.BackupPartCompleted))
		Expect(final.Status.Records.State).To(Equal(v1.BackupPartCompleted))
		Expect(final.Status.Runtime.State).To(Equal(v1.BackupPartCompleted))
		Expect(final.Status.StorageSizes.Zeebe.String()).To(Equal("20Gi"))
		Expect(final.Status.StorageSizes.Elasticsearch).NotTo(BeNil())

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
	})
})
