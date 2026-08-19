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
	"bytes"
	"fmt"
	"net/http"
	"net/http/httptest"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	coordinationv1 "k8s.io/api/coordination/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	utilrand "k8s.io/apimachinery/pkg/util/rand"
	"sigs.k8s.io/controller-runtime/pkg/client"

	v1 "github.com/konsole-is/camunda-operator/api/v1"
	"github.com/konsole-is/camunda-operator/pkg/camundaadmin/camundaadmintest"
	"github.com/konsole-is/camunda-operator/pkg/conditions"
	"github.com/konsole-is/camunda-operator/pkg/esadmin"
	"github.com/konsole-is/camunda-operator/pkg/esadmin/esadmintest"
	"github.com/konsole-is/camunda-operator/pkg/logicalbackup"
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
	// repository before any backup runs. The rig plays that part.
	admin, err := esadmin.New(r.search.URL(), "elastic", "elastic-password", r.search.CertificatePEM())
	Expect(err).NotTo(HaveOccurred())
	Expect(admin.EnsureSnapshotRepository(ctx, r.repository, esadmin.RepositoryConfig{
		Type:     esadmin.RepositoryTypeS3,
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

// publishBrokenBinding writes a management binding that no client can be
// built from: basic auth naming a credentials Secret that does not exist.
func (r *rig) publishBrokenBinding() {
	GinkgoHelper()
	Eventually(func(g Gomega) {
		var cluster v1.CamundaCluster
		g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(r.cluster), &cluster)).To(Succeed())
		cluster.Status.Management = &v1.ManagementBinding{
			Endpoint: r.management.URL(),
			Auth: v1.ManagementAuth{
				Method: v1.ManagementAuthMethodBasic,
				CredentialsSecretRef: &v1.CredentialsSecretRef{
					Name:        "management-credentials-gone",
					Namespace:   r.namespace,
					UsernameKey: "username",
					PasswordKey: "password",
				},
			},
			Version:          "8.9.9",
			Partitions:       3,
			BackupRepository: r.repository,
		}
		g.Expect(k8sClient.Status().Update(ctx, &cluster)).To(Succeed())
	}, timeout, interval).Should(Succeed())
}

// completeBackup drives the backup through the whole procedure against
// the fakes and returns its id. It is the happy path in one call, for the
// specs that start from a Completed backup.
func completeBackup(r *rig, backup *v1.LogicalBackupElasticsearch) int64 {
	GinkgoHelper()
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

	return id
}

// keepRegistrationGraceOpen stamps the history and runtime acceptance times
// of a terminal backup ahead of now. The registration grace measured from
// them then cannot elapse while the spec runs, whatever the load. The
// controller does not rewrite the status of a terminal backup.
func keepRegistrationGraceOpen(backup *v1.LogicalBackupElasticsearch) {
	GinkgoHelper()
	Eventually(func(g Gomega) {
		current := currentBackup(backup)
		ahead := metav1.NewTime(time.Now().Add(time.Hour))
		current.Status.HistoryAcceptedTime = &ahead
		current.Status.RuntimeAcceptedTime = &ahead
		g.Expect(k8sClient.Status().Update(ctx, current)).To(Succeed())
	}, timeout, interval).Should(Succeed())
}

// currentCluster re-reads the rig's CamundaCluster.
func currentCluster(r *rig) *v1.CamundaCluster {
	GinkgoHelper()
	var cluster v1.CamundaCluster
	Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(r.cluster), &cluster)).To(Succeed())
	return &cluster
}

// leaseHolder returns the exact identity of the holder of the claim Lease
// of the rig's cluster. That is the Claimant from the annotations, or the
// raw holderIdentity of a foreign Lease. Without a Lease it returns "".
func (r *rig) leaseHolder() string {
	GinkgoHelper()
	var lease coordinationv1.Lease
	err := k8sClient.Get(
		ctx, client.ObjectKey{
			Namespace: r.namespace, Name: logicalbackup.ClaimLeaseName(r.cluster.Name),
		}, &lease,
	)
	if apierrors.IsNotFound(err) {
		return ""
	}
	Expect(err).NotTo(HaveOccurred())
	annotations := lease.GetAnnotations()
	kind, name, uid := annotations[logicalbackup.ClaimHolderKindAnnotation],
		annotations[logicalbackup.ClaimHolderNameAnnotation],
		annotations[logicalbackup.ClaimHolderUIDAnnotation]
	if kind != "" && name != "" && uid != "" {
		return logicalbackup.Claimant{Kind: kind, Name: name, UID: types.UID(uid)}.String()
	}
	if lease.Spec.HolderIdentity == nil {
		return ""
	}
	return *lease.Spec.HolderIdentity
}

// holdLease writes the claim Lease of the cluster of the rig for the given
// holder identity, the way another actor writes it: no holder annotations.
func (r *rig) holdLease(holder string) {
	GinkgoHelper()
	lease := &coordinationv1.Lease{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: r.namespace, Name: logicalbackup.ClaimLeaseName(r.cluster.Name),
		},
		Spec: coordinationv1.LeaseSpec{HolderIdentity: &holder},
	}
	Expect(k8sClient.Create(ctx, lease)).To(Succeed())
}

// setLeaseHolder rewrites the existing claim Lease as one that the given
// claimant holds, in one write, so no claimant finds the Lease absent in
// between.
func (r *rig) setLeaseHolder(holder logicalbackup.Claimant) {
	GinkgoHelper()
	Eventually(func(g Gomega) {
		var lease coordinationv1.Lease
		g.Expect(k8sClient.Get(
			ctx, client.ObjectKey{
				Namespace: r.namespace, Name: logicalbackup.ClaimLeaseName(r.cluster.Name),
			}, &lease,
		)).To(Succeed())
		identity := holder.HolderIdentity()
		lease.Spec.HolderIdentity = &identity
		lease.Annotations = map[string]string{
			logicalbackup.ClaimHolderKindAnnotation: holder.Kind,
			logicalbackup.ClaimHolderNameAnnotation: holder.Name,
			logicalbackup.ClaimHolderUIDAnnotation:  string(holder.UID),
		}
		g.Expect(k8sClient.Update(ctx, &lease)).To(Succeed())
	}, timeout, interval).Should(Succeed())
}

// newBackup creates a backup of the rig's cluster. It registers a cleanup
// that never blocks the suite. When the ordinary deletion cannot finish, the
// cleanup strips the finalizer.
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
		Expect(final.Status.HistoryRequestedTime).NotTo(BeNil())
		Expect(final.Status.HistoryAcceptedTime).NotTo(BeNil())
		Expect(final.Status.PartitionsCount).To(Equal(int32(5)))
		Expect(final.Status.Repository).To(Equal(r.repository))
		Expect(final.Status.Storage).To(Equal(&v1.PinnedStorage{
			SecondaryStorageConfig: "storage",
			Endpoint:               r.search.URL(),
			BucketRef:              r.cluster.Spec.BackupStorageRef,
			BucketLocation:         "s3://backups/ (region eu-west-1)",
		}))
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
		// Resume never succeeds. The deadline of the suite is two seconds.
		r.management.FailNext("resume", 100000)
		r.management.FailNext("historyStart", 1)

		backup := r.newBackup()

		expectPhase(backup, v1.LogicalBackupFailed)
		expectReady(backup, metav1.ConditionFalse, v1.ReasonResumeFailed)
		expectEvent(backup, eventReasonResumeFailed, corev1.EventTypeWarning)
		Expect(r.leaseHolder()).To(
			Equal(claimant(currentBackup(backup)).String()),
			"a ResumeFailed backup keeps its claim: the cluster is still paused",
		)

		By("reporting the step failure and the resume failure side by side")
		final := currentBackup(backup)
		Expect(final.Status.FailureMessage).To(ContainSubstring("BackupHistory"))
		Expect(final.Status.ResumeFailureMessage).To(ContainSubstring("resumed"))
		ready := meta.FindStatusCondition(final.Status.Conditions, v1.ConditionReady)
		Expect(ready.Message).To(SatisfyAll(
			ContainSubstring("BackupHistory"),
			ContainSubstring("resumed"),
		))
	})

	It("polls an absent runtime backup through the registration grace instead of starting it again", func() {
		r := newRig()
		// The cluster registers a runtime backup asynchronously. After it
		// accepted the start it reports the backup absent for a moment. A
		// second start in that moment answers 409 and fails the step.
		r.management.HideRuntimeStatus(3)

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
		Eventually(func() v1.BackupPartState {
			return currentBackup(backup).Status.Runtime.State
		}, timeout, interval).Should(Equal(v1.BackupPartInProgress))
		Consistently(func(g Gomega) {
			g.Expect(currentBackup(backup).Status.Runtime.State).To(Equal(v1.BackupPartInProgress))
		}, "1s", interval).Should(Succeed())
		r.management.SetRuntimeState(id, "COMPLETED", "")

		expectPhase(backup, v1.LogicalBackupCompleted)
		Expect(r.management.RuntimeStarts(id)).To(Equal(1))
		final := currentBackup(backup)
		Expect(final.Status.RuntimeRequestedTime).NotTo(BeNil())
		Expect(final.Status.RuntimeAcceptedTime).NotTo(BeNil())
	})

	It("allocates the next id after a sibling that holds a higher one", func() {
		r := newRig()
		// The sibling fails fast and ends terminal, so it does not block.
		r.management.FailNext("historyStart", 1)
		sibling := r.newBackup()
		expectPhase(sibling, v1.LogicalBackupFailed)

		By("moving the id of the sibling far ahead of the clock, as a stepped-back clock would see it")
		ahead := time.Now().Add(365 * 24 * time.Hour).UnixMilli()
		Eventually(func(g Gomega) {
			current := currentBackup(sibling)
			current.Status.BackupID = ahead
			g.Expect(k8sClient.Status().Update(ctx, current)).To(Succeed())
		}, timeout, interval).Should(Succeed())

		backup := r.newBackup()
		Expect(backupID(backup)).To(Equal(ahead + 1))
	})

	It("holds the deletion while the history backup is in progress", func() {
		r := newRig()
		backup := r.newBackup()
		id := backupID(backup)

		Eventually(func() int { return r.management.HistoryStarts(id) }, timeout, interval).Should(Equal(1))

		By("deleting the backup while the web applications still create snapshots")
		Expect(k8sClient.Delete(ctx, backup)).To(Succeed())

		By("resuming exporting first, then holding on the in-progress history backup")
		Eventually(func() string { return r.management.Exporting() }, timeout, interval).Should(Equal("running"))
		expectEvent(backup, eventReasonDeleteHeld, corev1.EventTypeWarning)
		Consistently(func(g Gomega) {
			var held v1.LogicalBackupElasticsearch
			g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(backup), &held)).To(Succeed())
		}, "1s", interval).Should(Succeed())

		By("releasing once the history backup is terminal")
		r.management.SetHistoryState(id, "COMPLETED", "")
		Eventually(func() bool {
			var gone v1.LogicalBackupElasticsearch
			return k8sClient.Get(ctx, client.ObjectKeyFromObject(backup), &gone) != nil
		}, timeout, interval).Should(BeTrue())
	})

	It("keeps the status writable when the management API answers with an oversized body", func() {
		r := newRig()
		// Every call is rejected with a body far larger than a condition
		// allows. Pausing fails on it, so does every resume, and the backup
		// ends as ResumeFailed with both messages carrying the error.
		oversized := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = w.Write(bytes.Repeat([]byte("x"), 200_000))
		}))
		DeferCleanup(oversized.Close)
		Eventually(func(g Gomega) {
			var cluster v1.CamundaCluster
			g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(r.cluster), &cluster)).To(Succeed())
			cluster.Status.Management.Endpoint = oversized.URL
			g.Expect(k8sClient.Status().Update(ctx, &cluster)).To(Succeed())
		}, timeout, interval).Should(Succeed())

		backup := r.newBackup()

		expectPhase(backup, v1.LogicalBackupFailed)
		expectReady(backup, metav1.ConditionFalse, v1.ReasonResumeFailed)
		final := currentBackup(backup)
		Expect(final.Status.FailureMessage).To(ContainSubstring("PauseExporting"))
		Expect(len(final.Status.FailureMessage)).To(BeNumerically("<", conditions.MaxMessageLength))
		Expect(len(final.Status.ResumeFailureMessage)).To(BeNumerically("<", conditions.MaxMessageLength))
		ready := meta.FindStatusCondition(final.Status.Conditions, v1.ConditionReady)
		Expect(len(ready.Message)).To(BeNumerically("<=", conditions.MaxMessageLength+100))
		Expect(ready.Message).To(ContainSubstring("truncated"))
	})

	It("fails the records step when only the snapshot creation stays unreachable", func() {
		r := newRig()
		// The status query is served, the creation is dropped: a proxy that
		// serves GET and drops PUT. The successful query must not reset the
		// unreachable bound, or this retries forever with exporting paused.
		r.search.DropNext("snapshotCreate", 1000000)

		backup := r.newBackup()
		id := backupID(backup)

		Eventually(func() int { return r.management.HistoryStarts(id) }, timeout, interval).Should(Equal(1))
		r.management.SetHistoryState(id, "COMPLETED", "")

		expectPhase(backup, v1.LogicalBackupFailed)
		final := currentBackup(backup)
		Expect(final.Status.FailureMessage).To(SatisfyAll(
			ContainSubstring("SnapshotRecords"),
			ContainSubstring("unreachable"),
		))
		Expect(final.Status.Records.State).To(Equal(v1.BackupPartFailed))
		Expect(r.management.Exporting()).To(Equal("running"))
	})

	It("fails the records step through resume when Elasticsearch stays unreachable", func() {
		r := newRig()
		backup := r.newBackup()
		id := backupID(backup)

		Eventually(func() int { return r.management.HistoryStarts(id) }, timeout, interval).Should(Equal(1))

		By("taking Elasticsearch down before the records snapshot")
		r.search.Close()
		r.management.SetHistoryState(id, "COMPLETED", "")

		By("retrying with ConnectionFailed for a bounded time")
		// One read for both, so the bound cannot elapse between them.
		Eventually(func(g Gomega) {
			current := currentBackup(backup)
			ready := meta.FindStatusCondition(current.Status.Conditions, v1.ConditionReady)
			g.Expect(ready).NotTo(BeNil())
			g.Expect(ready.Reason).To(Equal(v1.ReasonConnectionFailed))
			g.Expect(current.Status.Step).To(Equal(v1.StepSnapshotRecords))
		}, timeout, interval).Should(Succeed())

		By("failing the step and resuming exporting after the bound")
		expectPhase(backup, v1.LogicalBackupFailed)
		expectReady(backup, metav1.ConditionFalse, v1.ReasonFailed)
		final := currentBackup(backup)
		Expect(final.Status.FailureMessage).To(SatisfyAll(
			ContainSubstring("SnapshotRecords"),
			ContainSubstring("unreachable"),
		))
		Expect(final.Status.Records.State).To(Equal(v1.BackupPartFailed))
		Expect(r.management.Exporting()).To(Equal("running"))
	})

	It("serializes two backups of one cluster on the claim Lease", func() {
		r := newRig()

		// Both are created within the same second. The tie-break orders
		// them. The Lease decides. Exactly one holds it and starts, the
		// other waits and names the holder.
		first := r.newBackup()
		second := r.newBackup()

		var holder, waiter *v1.LogicalBackupElasticsearch
		Eventually(func(g Gomega) {
			a, b := currentBackup(first), currentBackup(second)
			started := 0
			for _, candidate := range []*v1.LogicalBackupElasticsearch{a, b} {
				if candidate.Status.BackupID != 0 {
					started++
					holder = candidate
				}
			}
			g.Expect(started).To(Equal(1), "exactly one started")
			waiter = a
			if holder == a {
				waiter = b
			}
		}, timeout, interval).Should(Succeed())
		id := holder.Status.BackupID

		Expect(r.leaseHolder()).To(Equal(claimant(holder).String()))
		expectReady(waiter, metav1.ConditionFalse, v1.ReasonBackupInProgress)
		// The pre-filter or the Lease names the holder. In both cases the
		// waiter says who goes first.
		ready := meta.FindStatusCondition(currentBackup(waiter).Status.Conditions, v1.ConditionReady)
		Expect(ready.Message).To(ContainSubstring(holder.Name))
		Expect(currentBackup(waiter).Status.Phase).To(Equal(v1.LogicalBackupPending))
		Consistently(func() int64 { return currentBackup(waiter).Status.BackupID }, "1s", interval).Should(BeZero())

		By("proceeding once the holder finishes and the Lease is gone")
		r.management.SetHistoryState(id, "COMPLETED", "")
		name := RecordsSnapshotName(id)
		Eventually(func() int {
			return r.search.SnapshotCreates(r.repository, name)
		}, timeout, interval).Should(Equal(1))
		r.search.SetSnapshotState(r.repository, name, "SUCCESS")
		Eventually(func() int { return r.management.RuntimeStarts(id) }, timeout, interval).Should(Equal(1))
		r.management.SetRuntimeState(id, "COMPLETED", "")
		expectPhase(holder, v1.LogicalBackupCompleted)

		Eventually(func() v1.LogicalBackupPhase {
			return currentBackup(waiter).Status.Phase
		}, timeout, interval).Should(Equal(v1.LogicalBackupRunning))
		Expect(r.leaseHolder()).To(Equal(claimant(currentBackup(waiter)).String()))
	})

	It("waits on a claim that another holder took, and takes over a stale one", func() {
		r := newRig()

		By("blocking on a Lease that is not ours: no takeover, only a wait that names it")
		r.holdLease("someone-else")
		backup := r.newBackup()
		expectReady(backup, metav1.ConditionFalse, v1.ReasonBackupInProgress)
		ready := meta.FindStatusCondition(currentBackup(backup).Status.Conditions, v1.ConditionReady)
		Expect(ready.Message).To(ContainSubstring("someone-else"))
		Consistently(func() int64 { return currentBackup(backup).Status.BackupID }, "1s", interval).Should(BeZero())
		Expect(r.leaseHolder()).To(Equal("someone-else"))

		By("taking over a Lease whose holder is a backup that no longer exists")
		r.setLeaseHolder(logicalbackup.Claimant{
			Kind: "LogicalBackupElasticsearch", Name: "deleted-long-ago", UID: "uid-of-the-past",
		})
		backupID(backup)
		Expect(r.leaseHolder()).To(Equal(claimant(currentBackup(backup)).String()))
	})

	It("releases the claim when the holder ends and when a running holder is deleted", func() {
		r := newRig()
		backup := r.newBackup()
		id := backupID(backup)
		Expect(r.leaseHolder()).To(Equal(claimant(currentBackup(backup)).String()))

		By("failing the backup fast, then finding the Lease gone")
		r.management.SetHistoryState(id, "FAILED", "disk full")
		expectPhase(backup, v1.LogicalBackupFailed)
		Eventually(r.leaseHolder, timeout, interval).Should(BeEmpty())

		By("deleting a running backup, then finding the Lease gone with it")
		running := r.newBackup()
		runningID := backupID(running)
		Eventually(func() int { return r.management.HistoryStarts(runningID) }, timeout, interval).Should(Equal(1))
		Expect(r.leaseHolder()).To(Equal(claimant(currentBackup(running)).String()))
		r.management.SetHistoryState(runningID, "COMPLETED", "")
		Expect(k8sClient.Delete(ctx, running)).To(Succeed())
		Eventually(func() bool {
			var gone v1.LogicalBackupElasticsearch
			return k8sClient.Get(ctx, client.ObjectKeyFromObject(running), &gone) != nil
		}, timeout, interval).Should(BeTrue())
		Expect(r.leaseHolder()).To(BeEmpty())
	})

	// Round 14 (from #85's review, mirrored here): the claim is taken
	// before the ID is allocated. A backup that left admission without a
	// start was able to keep the Lease. If a pre-check then parked it, the held
	// Lease blocked every sibling for as long as the park lasted. Every
	// exit of admission without a start now releases the claim.
	It("holds no claim while admission parks it", func() {
		r := newRig()
		Eventually(func(g Gomega) {
			var cluster v1.CamundaCluster
			g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(r.cluster), &cluster)).To(Succeed())
			cluster.Spec.Suspend = true
			g.Expect(k8sClient.Update(ctx, &cluster)).To(Succeed())
		}, timeout, interval).Should(Succeed())
		backup := r.newBackup()
		expectReady(backup, metav1.ConditionFalse, v1.ReasonClusterSuspended)

		By("a Lease this parked backup holds from an interrupted admission")
		self := claimant(currentBackup(backup))
		identity := self.HolderIdentity()
		lease := &coordinationv1.Lease{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: r.namespace, Name: logicalbackup.ClaimLeaseName(r.cluster.Name),
				Annotations: map[string]string{
					logicalbackup.ClaimHolderKindAnnotation: self.Kind,
					logicalbackup.ClaimHolderNameAnnotation: self.Name,
					logicalbackup.ClaimHolderUIDAnnotation:  string(self.UID),
				},
			},
			Spec: coordinationv1.LeaseSpec{HolderIdentity: &identity},
		}
		Expect(k8sClient.Create(ctx, lease)).To(Succeed())

		By("the parked backup releases it on its next admission pass")
		Eventually(func() string { return r.leaseHolder() }, timeout, interval).Should(BeEmpty())

		By("resuming the cluster: the backup claims and starts")
		Eventually(func(g Gomega) {
			var cluster v1.CamundaCluster
			g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(r.cluster), &cluster)).To(Succeed())
			cluster.Spec.Suspend = false
			g.Expect(k8sClient.Update(ctx, &cluster)).To(Succeed())
		}, timeout, interval).Should(Succeed())
		r.publishBinding(3)
		Expect(backupID(backup)).NotTo(BeZero())
		Expect(r.leaseHolder()).To(Equal(self.String()))
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
		// The web applications wrote the snapshots into the repository, so
		// the deletion below has something to delete by name.
		for _, snapshot := range snapshots {
			r.search.SetSnapshotState(r.repository, snapshot, "SUCCESS")
		}

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
		// The intent was recorded, the conflict was checked against the
		// cluster, and the ID was not there: a genuine conflict.
		Expect(final.Status.RuntimeRequestedTime).NotTo(BeNil())
		Expect(r.management.RuntimeStarts(id)).To(BeZero())
		Expect(r.management.Exporting()).To(Equal("running"))
	})

	It("records the runtime request before it posts it", func() {
		r := newRig()
		// The start is rejected for good. The intent must be in the status
		// anyway, because it is written one reconcile before the request.
		r.management.FailNext("runtimeStart", 1000000)

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
		Expect(final.Status.RuntimeRequestedTime).NotTo(BeNil())
		Expect(final.Status.FailureMessage).To(ContainSubstring("BackupRuntime"))
		Expect(r.management.RuntimeStarts(id)).To(BeZero())
		Expect(r.management.Exporting()).To(Equal("running"))
	})

	It("ends terminally without touching a same-named replacement cluster", func() {
		r := newRig()
		backup := r.newBackup()
		id := backupID(backup)
		Eventually(func() int { return r.management.HistoryStarts(id) }, timeout, interval).Should(Equal(1))

		By("replacing the cluster mid-run: same name, new UID, its own management API")
		old := r.management
		replacement := camundaadmintest.New()
		DeferCleanup(replacement.Close)
		spec := currentCluster(r).Spec
		Expect(k8sClient.Delete(ctx, r.cluster)).To(Succeed())
		Eventually(func() bool {
			var gone v1.CamundaCluster
			return k8sClient.Get(ctx, client.ObjectKeyFromObject(r.cluster), &gone) != nil
		}, timeout, interval).Should(BeTrue())
		recreated := &v1.CamundaCluster{
			ObjectMeta: metav1.ObjectMeta{Name: r.cluster.Name, Namespace: r.namespace},
			Spec:       spec,
		}
		Expect(k8sClient.Create(ctx, recreated)).To(Succeed())
		r.management = replacement
		r.publishBinding(3)

		By("failing terminally, with no call against the replacement")
		expectPhase(backup, v1.LogicalBackupFailed)
		final := currentBackup(backup)
		// The reconciler reads the cluster live. Timing decides whether it
		// observes the gap between the delete and the create, or only the
		// replacement. Both end the same way, and the message names which.
		Expect(final.Status.FailureMessage).To(SatisfyAny(
			ContainSubstring("replaced"), ContainSubstring("gone"),
		))
		Expect(final.Status.TerminalReason).To(Equal(v1.ReasonFailed))
		Expect(replacement.PauseCalls()).To(BeZero())
		Expect(replacement.ResumeAttempts()).To(BeZero())
		Expect(replacement.HistoryStarts(id)).To(BeZero())
		Expect(replacement.RuntimeStarts(id)).To(BeZero())
		Expect(old.Exporting()).To(Equal("softPaused"), "the old cluster's pause died with it")

		By("releasing the claim: nothing of this backup pauses the replacement")
		Eventually(func() string { return r.leaseHolder() }, timeout, interval).Should(BeEmpty())
	})

	It("ends terminally and releases the claim when the cluster is gone mid-run", func() {
		r := newRig()
		backup := r.newBackup()
		id := backupID(backup)
		Eventually(func() int { return r.management.HistoryStarts(id) }, timeout, interval).Should(Equal(1))
		resumesBefore := r.management.ResumeAttempts()

		By("deleting the cluster mid-run, with nothing in its place")
		spec := currentCluster(r).Spec
		Expect(k8sClient.Delete(ctx, r.cluster)).To(Succeed())
		Eventually(func() bool {
			var gone v1.CamundaCluster
			return k8sClient.Get(ctx, client.ObjectKeyFromObject(r.cluster), &gone) != nil
		}, timeout, interval).Should(BeTrue())

		By("failing terminally as Failed, never ResumeFailed, and without a resume against the dead endpoint")
		expectPhase(backup, v1.LogicalBackupFailed)
		expectReady(backup, metav1.ConditionFalse, v1.ReasonFailed)
		final := currentBackup(backup)
		Expect(final.Status.TerminalReason).To(Equal(v1.ReasonFailed))
		Expect(final.Status.FailureMessage).To(ContainSubstring("gone"))
		Expect(r.management.ResumeAttempts()).To(Equal(resumesBefore))

		By("releasing the claim, so a cluster recreated under the name is not blocked forever")
		Eventually(func() string { return r.leaseHolder() }, timeout, interval).Should(BeEmpty())
		recreated := &v1.CamundaCluster{
			ObjectMeta: metav1.ObjectMeta{Name: r.cluster.Name, Namespace: r.namespace},
			Spec:       spec,
		}
		Expect(k8sClient.Create(ctx, recreated)).To(Succeed())
		r.publishBinding(3)
		next := r.newBackup()
		Expect(backupID(next)).To(BeNumerically(">", id), "the next backup of the recreated cluster starts")
	})

	// Round 13 P1: the start answer names the scheduled snapshots, and the
	// documentation requires them to be persisted with the backup id. A
	// controller that recorded names only from the status poll left a
	// window. If the cluster was gone between the acceptance and the first
	// poll, the finalizer had no history names to sweep.
	It(
		"records the scheduled history snapshot names at acceptance and sweeps them when the cluster is gone before the first poll",
		func() {
			r := newRig()
			// The cluster hides the accepted backup from every status query, so
			// no poll can record a name. Only the start answer can.
			r.management.HideHistoryStatus(1_000_000)
			backup := r.newBackup()
			id := backupID(backup)
			scheduled := camundaadmintest.HistorySnapshotName(id)

			Eventually(func() int { return r.management.HistoryStarts(id) }, timeout, interval).Should(Equal(1))
			Eventually(func() []string {
				return currentBackup(backup).Status.HistorySnapshots
			}, timeout, interval).Should(ConsistOf(scheduled), "the scheduled names are recorded with the acceptance")
			Expect(currentBackup(backup).Status.HistoryAcceptedTime).NotTo(BeNil())
			Expect(r.management.HistoryStarts(id)).To(Equal(1))
			// The web applications wrote the snapshot into the repository.
			r.search.SetSnapshotState(r.repository, scheduled, "SUCCESS")

			By("deleting the cluster before any status poll answered")
			Expect(k8sClient.Delete(ctx, r.cluster)).To(Succeed())
			expectPhase(backup, v1.LogicalBackupFailed)

			By("deleting the backup: the finalizer sweeps the scheduled snapshot from the pinned repository")
			Expect(k8sClient.Delete(ctx, backup)).To(Succeed())
			Eventually(func() bool {
				var gone v1.LogicalBackupElasticsearch
				return k8sClient.Get(ctx, client.ObjectKeyFromObject(backup), &gone) != nil
			}, timeout, interval).Should(BeTrue())
			Expect(
				r.search.SnapshotExists(r.repository, scheduled),
			).To(BeFalse(), "the scheduled history snapshot was swept")
		},
	)

	It("sweeps its snapshots and skips every management call when the cluster was replaced", func() {
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
		historySnapshot := currentBackup(backup).Status.HistorySnapshots[0]
		r.search.SetSnapshotState(r.repository, historySnapshot, "SUCCESS")

		By("replacing the cluster after the backup completed")
		old := r.management
		replacement := camundaadmintest.New()
		DeferCleanup(replacement.Close)
		spec := currentCluster(r).Spec
		Expect(k8sClient.Delete(ctx, r.cluster)).To(Succeed())
		Eventually(func() bool {
			var gone v1.CamundaCluster
			return k8sClient.Get(ctx, client.ObjectKeyFromObject(r.cluster), &gone) != nil
		}, timeout, interval).Should(BeTrue())
		recreated := &v1.CamundaCluster{
			ObjectMeta: metav1.ObjectMeta{Name: r.cluster.Name, Namespace: r.namespace},
			Spec:       spec,
		}
		Expect(k8sClient.Create(ctx, recreated)).To(Succeed())
		r.management = replacement
		r.publishBinding(3)

		By("deleting the backup: the snapshots go, the management APIs stay untouched")
		Expect(k8sClient.Delete(ctx, backup)).To(Succeed())
		Eventually(func() bool {
			var gone v1.LogicalBackupElasticsearch
			return k8sClient.Get(ctx, client.ObjectKeyFromObject(backup), &gone) != nil
		}, timeout, interval).Should(BeTrue())

		Expect(r.search.SnapshotExists(r.repository, name)).To(BeFalse(), "the records snapshot was swept")
		Expect(r.search.SnapshotExists(r.repository, historySnapshot)).To(BeFalse(), "the history snapshot was swept")
		runtime := old.RuntimeBackup(id)
		Expect(runtime).NotTo(BeNil())
		Expect(runtime.State).To(Equal("COMPLETED"), "no delete against the old cluster's management API")
		Expect(replacement.PauseCalls()).To(BeZero())
		Expect(replacement.ResumeAttempts()).To(BeZero())
		expectEvent(backup, eventReasonReleased, corev1.EventTypeWarning)
	})

	// Round 14 P1: the sweep of a gone or replaced cluster verified the
	// Elasticsearch contract and endpoint, but not the pinned bucket. The
	// repository name can be registered against the new bucket by then. The
	// sweep then aimed its deletes at the wrong storage, and the originals
	// leaked.
	It("abandons the sweep of a gone cluster when the pinned bucket contract points elsewhere", func() {
		r := newRig()
		backup := r.newBackup()
		id := completeBackup(r, backup)
		name := RecordsSnapshotName(id)
		historySnapshot := currentBackup(backup).Status.HistorySnapshots[0]
		r.search.SetSnapshotState(r.repository, historySnapshot, "SUCCESS")

		By("retargeting the bucket contract, then deleting the cluster with nothing in its place")
		Eventually(func(g Gomega) {
			var bucket v1.ObjectStorageConfig
			g.Expect(k8sClient.Get(ctx, client.ObjectKey{Name: r.cluster.Spec.BackupStorageRef}, &bucket)).To(Succeed())
			bucket.Spec.S3.BucketName = "backups-moved"
			g.Expect(k8sClient.Update(ctx, &bucket)).To(Succeed())
		}, timeout, interval).Should(Succeed())
		Expect(k8sClient.Delete(ctx, r.cluster)).To(Succeed())
		Eventually(func() bool {
			var gone v1.CamundaCluster
			return k8sClient.Get(ctx, client.ObjectKeyFromObject(r.cluster), &gone) != nil
		}, timeout, interval).Should(BeTrue())

		By("deleting the backup: it releases, and no snapshot is deleted through the moved bucket")
		Expect(k8sClient.Delete(ctx, backup)).To(Succeed())
		Eventually(func() bool {
			var gone v1.LogicalBackupElasticsearch
			return k8sClient.Get(ctx, client.ObjectKeyFromObject(backup), &gone) != nil
		}, timeout, interval).Should(BeTrue())
		Expect(r.search.SnapshotExists(r.repository, name)).To(BeTrue(), "the records snapshot stays")
		Expect(r.search.SnapshotExists(r.repository, historySnapshot)).To(BeTrue(), "the history snapshot stays")
		expectEvent(backup, eventReasonReleased, corev1.EventTypeWarning)
	})

	It("never adopts a history backup it did not see accepted, and never deletes its snapshots", func() {
		r := newRig()
		// The management API is unreachable through the pause, so the
		// procedure holds there. Meanwhile a history backup appears under
		// the backup's ID: another actor's, or a lost response. Nothing
		// tells the two apart.
		unreachable := r.management.URL()
		r.management.Close()
		r.management = camundaadmintest.New()
		DeferCleanup(r.management.Close)
		Eventually(func(g Gomega) {
			var cluster v1.CamundaCluster
			g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(r.cluster), &cluster)).To(Succeed())
			cluster.Status.Management.Endpoint = unreachable
			g.Expect(k8sClient.Status().Update(ctx, &cluster)).To(Succeed())
		}, timeout, interval).Should(Succeed())

		backup := r.newBackup()
		id := backupID(backup)
		expectReady(backup, metav1.ConditionFalse, v1.ReasonConnectionFailed)

		foreignSnapshot := fmt.Sprintf("camunda_webapps_%d_8.9_part_1_of_1", id)
		r.management.SetHistoryState(id, "IN_PROGRESS", "")
		r.search.SetSnapshotState(r.repository, foreignSnapshot, "SUCCESS")
		r.publishBinding(3)

		By("failing through resume without adopting")
		expectPhase(backup, v1.LogicalBackupFailed)
		final := currentBackup(backup)
		Expect(final.Status.FailureMessage).To(SatisfyAll(
			ContainSubstring("BackupHistory"),
			ContainSubstring("did not see accepted"),
			ContainSubstring("not adopted"),
		))
		Expect(final.Status.HistoryAcceptedTime).To(BeNil())
		Expect(final.Status.HistorySnapshots).To(BeEmpty(), "foreign snapshot names are never recorded")
		Expect(r.management.HistoryStarts(id)).To(BeZero())
		Expect(r.management.Exporting()).To(Equal("running"))

		By("leaving the foreign snapshots alone on deletion")
		Expect(k8sClient.Delete(ctx, backup)).To(Succeed())
		Eventually(func() bool {
			var gone v1.LogicalBackupElasticsearch
			return k8sClient.Get(ctx, client.ObjectKeyFromObject(backup), &gone) != nil
		}, timeout, interval).Should(BeTrue())
		Expect(r.search.SnapshotExists(r.repository, foreignSnapshot)).To(BeTrue())
	})

	It("never adopts a records snapshot it did not create, and never deletes it", func() {
		r := newRig()
		backup := r.newBackup()
		id := backupID(backup)

		// While the history backup is still in progress, a snapshot appears
		// under the deterministic records name: an ID reuse by a deleted or
		// other-kind backup. It carries no metadata of this backup.
		Eventually(func() int { return r.management.HistoryStarts(id) }, timeout, interval).Should(Equal(1))
		name := RecordsSnapshotName(id)
		r.search.SetSnapshotState(r.repository, name, "SUCCESS")
		r.management.SetHistoryState(id, "COMPLETED", "")

		By("failing through resume without adopting")
		expectPhase(backup, v1.LogicalBackupFailed)
		final := currentBackup(backup)
		Expect(final.Status.FailureMessage).To(SatisfyAll(
			ContainSubstring("SnapshotRecords"),
			ContainSubstring("did not create"),
			ContainSubstring("not adopted"),
		))
		Expect(final.Status.Records.State).To(Equal(v1.BackupPartFailed))
		Expect(r.search.SnapshotCreates(r.repository, name)).To(BeZero())
		Expect(r.management.Exporting()).To(Equal("running"))

		By("leaving the foreign snapshot alone on deletion")
		Expect(k8sClient.Delete(ctx, backup)).To(Succeed())
		Eventually(func() bool {
			var gone v1.LogicalBackupElasticsearch
			return k8sClient.Get(ctx, client.ObjectKeyFromObject(backup), &gone) != nil
		}, timeout, interval).Should(BeTrue())
		Expect(r.search.SnapshotExists(r.repository, name)).To(BeTrue())
	})

	It(
		"tells a foreign records snapshot with metadata of any JSON type apart, and its deletion still finishes",
		func() {
			r := newRig()
			backup := r.newBackup()
			id := backupID(backup)

			// Elasticsearch stores any JSON value as snapshot metadata. A
			// snapshot that another tool created under the deterministic name
			// carries numbers and objects. It never carries the UID of this
			// backup as a string. The status must decode. If it does not,
			// the step and later the finalizer fail on the decode forever.
			Eventually(func() int { return r.management.HistoryStarts(id) }, timeout, interval).Should(Equal(1))
			name := RecordsSnapshotName(id)
			r.search.SetSnapshotMetadata(r.repository, name, map[string]any{
				"retention-days":    float64(30),
				"created-by":        map[string]any{"tool": "curator"},
				snapshotOwnerUIDKey: float64(7),
			})
			r.management.SetHistoryState(id, "COMPLETED", "")

			By("failing through resume without adopting")
			expectPhase(backup, v1.LogicalBackupFailed)
			final := currentBackup(backup)
			Expect(final.Status.FailureMessage).To(SatisfyAll(
				ContainSubstring("SnapshotRecords"),
				ContainSubstring("did not create"),
			))
			Expect(r.search.SnapshotCreates(r.repository, name)).To(BeZero())
			Expect(r.management.Exporting()).To(Equal("running"))

			By("leaving the foreign snapshot alone on deletion, and finishing the deletion")
			Expect(k8sClient.Delete(ctx, backup)).To(Succeed())
			Eventually(func() bool {
				var gone v1.LogicalBackupElasticsearch
				return k8sClient.Get(ctx, client.ObjectKeyFromObject(backup), &gone) != nil
			}, timeout, interval).Should(BeTrue())
			Expect(r.search.SnapshotExists(r.repository, name)).To(BeTrue())
		},
	)

	It("never adopts a runtime backup it did not see accepted, and never deletes it", func() {
		r := newRig()
		backup := r.newBackup()
		id := backupID(backup)

		// The cluster holds our ID already: an earlier request whose
		// response was lost, or another actor that won the ID. Nothing
		// tells the two apart. It hides the backup for the two status
		// queries that follow, so the intent is recorded and the request
		// goes out and conflicts.
		r.management.SetRuntimeState(id, "IN_PROGRESS", "")
		r.management.HideRuntimeStatus(2)

		Eventually(func() int { return r.management.HistoryStarts(id) }, timeout, interval).Should(Equal(1))
		r.management.SetHistoryState(id, "COMPLETED", "")
		name := RecordsSnapshotName(id)
		Eventually(func() int {
			return r.search.SnapshotCreates(r.repository, name)
		}, timeout, interval).Should(Equal(1))
		r.search.SetSnapshotState(r.repository, name, "SUCCESS")

		By("failing through resume without adopting")
		expectPhase(backup, v1.LogicalBackupFailed)
		final := currentBackup(backup)
		Expect(final.Status.FailureMessage).To(SatisfyAll(
			ContainSubstring("BackupRuntime"),
			ContainSubstring("not adopted"),
			ContainSubstring("finalizer will not delete"),
		))
		Expect(final.Status.RuntimeRequestedTime).NotTo(BeNil())
		Expect(final.Status.RuntimeAcceptedTime).To(BeNil())
		Expect(final.Status.Runtime.State).To(Equal(v1.BackupPartFailed))
		Expect(r.management.RuntimeStarts(id)).To(BeZero())
		Expect(r.management.Exporting()).To(Equal("running"))

		By("leaving that runtime backup alone on deletion")
		r.management.SetRuntimeState(id, "COMPLETED", "")
		Expect(k8sClient.Delete(ctx, backup)).To(Succeed())
		Eventually(func() bool {
			var gone v1.LogicalBackupElasticsearch
			return k8sClient.Get(ctx, client.ObjectKeyFromObject(backup), &gone) != nil
		}, timeout, interval).Should(BeTrue())
		runtime := r.management.RuntimeBackup(id)
		Expect(runtime).NotTo(BeNil())
		Expect(runtime.State).To(Equal("COMPLETED"), "the runtime backup that was not ours is still there")
	})

	It("does not probe Elasticsearch for sizes while exporting is paused", func() {
		r := newRig()
		// The size probe fails at start and stays failing, so the sizes are
		// missing throughout. Once the pause step is behind, no reconcile
		// asks Elasticsearch for statistics again until exporting runs.
		r.search.FailNext("stats", 1000000)
		r.management.FailNext("historyStart", 1)
		// Resume keeps failing while the probe count is watched, so the
		// procedure stays at ResumeExporting with exporting paused.
		r.management.FailNext("resume", 1000000)

		backup := r.newBackup()
		id := backupID(backup)
		// The window under test starts once resume attempts are under way.
		// No probe of an earlier step is still in flight then. The window
		// spans five more attempts.
		Eventually(r.management.ResumeAttempts, timeout, interval).Should(BeNumerically(">=", 3))
		before, attempts := r.search.StatsCalls(), r.management.ResumeAttempts()
		Eventually(r.management.ResumeAttempts, timeout, interval).Should(BeNumerically(">=", attempts+5))
		Expect(r.search.StatsCalls()).To(Equal(before), "no size probe between resume attempts")
		Expect(currentBackup(backup).Status.Step).To(Equal(v1.StepResumeExporting))
		Expect(r.management.HistoryStarts(id)).To(BeZero())

		By("backfilling once exporting runs again")
		r.management.FailNext("resume", 0)
		expectPhase(backup, v1.LogicalBackupFailed)
		Expect(r.search.StatsCalls()).To(BeNumerically(">", before))
	})

	It("fails the records step through resume when the CA bundle is unusable", func() {
		r := newRig()
		backup := r.newBackup()
		id := backupID(backup)

		Eventually(func() int { return r.management.HistoryStarts(id) }, timeout, interval).Should(Equal(1))

		By("replacing the CA bundle with data that is not a certificate")
		Eventually(func(g Gomega) {
			var ca corev1.Secret
			g.Expect(k8sClient.Get(ctx, client.ObjectKey{Namespace: r.namespace, Name: "es-ca"}, &ca)).To(Succeed())
			ca.Data["ca.crt"] = []byte("not a certificate")
			g.Expect(k8sClient.Update(ctx, &ca)).To(Succeed())
		}, timeout, interval).Should(Succeed())
		r.management.SetHistoryState(id, "COMPLETED", "")

		expectPhase(backup, v1.LogicalBackupFailed)
		final := currentBackup(backup)
		Expect(final.Status.FailureMessage).To(SatisfyAll(
			ContainSubstring("SnapshotRecords"),
			ContainSubstring("CA bundle"),
		))
		Expect(final.Status.Records.State).To(Equal(v1.BackupPartFailed))
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

	// Round 12 P2: a management API that stays unreachable mid-pause was
	// retried without a bound. A route that black-holes only the backup
	// endpoint while resume is healthy left the cluster paused for good.
	// The bound routes the step through resume.
	It("fails the step through resume when the management API stays unreachable mid-pause past the bound", func() {
		r := newRig()
		backup := r.newBackup()
		id := backupID(backup)
		Eventually(func() int { return r.management.HistoryStarts(id) }, timeout, interval).Should(Equal(1))

		By("losing the management API with exporting paused")
		r.management.Close()

		By("holding with ConnectionFailed within the bound, then failing the step through resume")
		expectReady(backup, metav1.ConditionFalse, v1.ReasonConnectionFailed)
		Eventually(func(g Gomega) {
			current := currentBackup(backup)
			g.Expect(current.Status.Step).To(Equal(v1.StepResumeExporting))
			g.Expect(current.Status.FailureMessage).To(SatisfyAll(
				ContainSubstring("BackupHistory"),
				ContainSubstring("stayed unreachable"),
			))
			g.Expect(current.Status.UnreachableSince).NotTo(BeNil())
		}, timeout, interval).Should(Succeed())

		By("ending as ResumeFailed once the resume deadline passes against the same dead API")
		expectPhase(backup, v1.LogicalBackupFailed)
		expectReady(backup, metav1.ConditionFalse, v1.ReasonResumeFailed)
	})

	// Round 12 P1: an accepted runtime or history backup registers
	// asynchronously. A finalizer that read the unregistered answer as
	// "nothing to delete" released while the backup was still able to
	// register.
	It("holds the deletion of an accepted runtime backup through the registration grace", func() {
		r := newRig()
		backup := r.newBackup()
		id := completeBackup(r, backup)

		By("deleting while the runtime backup answers as unregistered, inside the registration grace")
		keepRegistrationGraceOpen(backup)
		r.management.HideRuntimeStatus(1_000_000)
		Expect(k8sClient.Delete(ctx, backup)).To(Succeed())
		expectEvent(backup, eventReasonDeleteHeld, corev1.EventTypeWarning)
		Consistently(func(g Gomega) {
			var held v1.LogicalBackupElasticsearch
			g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(backup), &held)).To(Succeed())
			g.Expect(r.management.RuntimeBackup(id)).
				NotTo(BeNil(), "the runtime backup is not deleted while it registers")
		}, "800ms", interval).Should(Succeed())

		By("deleting it once it registers")
		r.management.HideRuntimeStatus(0)
		Eventually(func() bool {
			var gone v1.LogicalBackupElasticsearch
			return k8sClient.Get(ctx, client.ObjectKeyFromObject(backup), &gone) != nil
		}, timeout, interval).Should(BeTrue())
		Expect(r.management.RuntimeBackup(id)).To(BeNil(), "the runtime backup was deleted")
	})

	It("holds the deletion of an accepted history backup through the registration grace", func() {
		r := newRig()
		backup := r.newBackup()
		id := completeBackup(r, backup)
		snapshots := currentBackup(backup).Status.HistorySnapshots
		Expect(snapshots).NotTo(BeEmpty())
		for _, snapshot := range snapshots {
			r.search.SetSnapshotState(r.repository, snapshot, "SUCCESS")
		}

		By("deleting while the history backup answers as unregistered, inside the registration grace")
		keepRegistrationGraceOpen(backup)
		r.management.HideHistoryStatus(1_000_000)
		Expect(k8sClient.Delete(ctx, backup)).To(Succeed())
		expectEvent(backup, eventReasonDeleteHeld, corev1.EventTypeWarning)
		Consistently(func(g Gomega) {
			var held v1.LogicalBackupElasticsearch
			g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(backup), &held)).To(Succeed())
			for _, snapshot := range snapshots {
				g.Expect(r.search.SnapshotExists(r.repository, snapshot)).To(BeTrue())
			}
		}, "800ms", interval).Should(Succeed())

		By("deleting its snapshots once it registers")
		r.management.HideHistoryStatus(0)
		Eventually(func() bool {
			var gone v1.LogicalBackupElasticsearch
			return k8sClient.Get(ctx, client.ObjectKeyFromObject(backup), &gone) != nil
		}, timeout, interval).Should(BeTrue())
		for _, snapshot := range snapshots {
			Expect(r.search.SnapshotExists(r.repository, snapshot)).To(BeFalse())
		}
		_ = id
	})

	It("keeps a deletion waiting while the cluster exists without a binding", func() {
		r := newRig()
		backup := r.newBackup()
		id := backupID(backup)

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
		// The procedure can have started the history backup before the
		// binding went away. A deletion waits for that backup to end, which
		// another test covers. Here it ends, so the missing binding is the
		// only thing that held the deletion.
		if r.management.HistoryStarts(id) > 0 {
			r.management.SetHistoryState(id, "COMPLETED", "")
		}
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

		By("deleting the storage contract while the history backup runs")
		var storage v1.SecondaryStorageConfig
		Expect(k8sClient.Get(
			ctx, client.ObjectKey{Namespace: r.namespace, Name: "storage"}, &storage,
		)).To(Succeed())
		Expect(k8sClient.Delete(ctx, &storage)).To(Succeed())

		// The destination is verified on every poll of the history backup,
		// so the loss is caught there, before the status of that backup is
		// trusted. The history backup stays in progress until then. If the
		// spec completed it now, it raced a reconcile that verified the
		// destination before the delete and then saw the completed history.
		// That reconcile fails one step later, in SnapshotRecords.
		expectPhase(backup, v1.LogicalBackupFailed)
		final := currentBackup(backup)
		Expect(final.Status.FailureMessage).To(ContainSubstring("BackupHistory"))
		Expect(final.Status.History.State).To(Equal(v1.BackupPartFailed))
		Expect(r.management.Exporting()).To(Equal("running"))
	})

	It("fails before the history backup starts when the repository is repointed after the pause", func() {
		r := newRig()
		// The management API is unreachable through the pause, so the
		// procedure holds at PauseExporting with its destination pinned.
		unreachable := r.management.URL()
		r.management.Close()
		r.management = camundaadmintest.New()
		DeferCleanup(r.management.Close)
		pinned := r.repository
		Eventually(func(g Gomega) {
			var cluster v1.CamundaCluster
			g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(r.cluster), &cluster)).To(Succeed())
			cluster.Status.Management.Endpoint = unreachable
			g.Expect(k8sClient.Status().Update(ctx, &cluster)).To(Succeed())
		}, timeout, interval).Should(Succeed())

		backup := r.newBackup()
		id := backupID(backup)
		expectReady(backup, metav1.ConditionFalse, v1.ReasonConnectionFailed)
		Expect(currentBackup(backup).Status.Repository).To(Equal(pinned))

		By("repointing the repository and bringing the management API back")
		r.repository = "another-repository"
		r.publishBinding(3)

		expectPhase(backup, v1.LogicalBackupFailed)
		final := currentBackup(backup)
		Expect(final.Status.FailureMessage).To(SatisfyAll(
			ContainSubstring("BackupHistory"),
			ContainSubstring(pinned),
			ContainSubstring("another-repository"),
		))
		Expect(final.Status.History.State).To(Equal(v1.BackupPartFailed))
		Expect(r.management.HistoryStarts(id)).To(BeZero(), "no history backup was started into the new repository")
		Expect(r.management.Exporting()).To(Equal("running"))
	})

	It(
		"fails a started history backup when the repository is repointed, and the finalizer touches the pinned one",
		func() {
			r := newRig()
			pinned := r.repository
			backup := r.newBackup()
			id := backupID(backup)

			Eventually(func() int { return r.management.HistoryStarts(id) }, timeout, interval).Should(Equal(1))
			Eventually(func() []string {
				return currentBackup(backup).Status.HistorySnapshots
			}, timeout, interval).ShouldNot(BeEmpty())
			historySnapshot := currentBackup(backup).Status.HistorySnapshots[0]
			// The web applications wrote the snapshot into the pinned repository.
			r.search.SetSnapshotState(pinned, historySnapshot, "SUCCESS")

			By("repointing the repository while the history backup runs")
			// The history backup stays in progress until the step failed
			// on the repoint. The step verifies the destination before it
			// trusts the status, so no reconcile can advance past
			// BackupHistory in between. If the spec completes the history
			// here, it races a reconcile that read the old binding and then
			// sees the completed history. That reconcile fails one step
			// later.
			r.repository = "another-repository"
			r.publishBinding(3)

			expectPhase(backup, v1.LogicalBackupFailed)
			final := currentBackup(backup)
			Expect(final.Status.FailureMessage).To(SatisfyAll(
				ContainSubstring("BackupHistory"),
				ContainSubstring(pinned),
				ContainSubstring("another-repository"),
			))
			Expect(final.Status.Repository).To(Equal(pinned))
			Expect(r.management.Exporting()).To(Equal("running"))

			By("deleting the backup: the finalizer removes the snapshot from the pinned repository")
			// A deletion holds while the history backup is in progress. It
			// ends now, so only the pinned repository decides the cleanup.
			r.management.SetHistoryState(id, "COMPLETED", "")
			Expect(k8sClient.Delete(ctx, backup)).To(Succeed())
			Eventually(func() bool {
				var gone v1.LogicalBackupElasticsearch
				return k8sClient.Get(ctx, client.ObjectKeyFromObject(backup), &gone) != nil
			}, timeout, interval).Should(BeTrue())
			Expect(r.search.SnapshotExists(pinned, historySnapshot)).To(BeFalse())
		},
	)

	It("fails the step when the storage endpoint is repointed mid-run, and holds the deletion", func() {
		r := newRig()
		backup := r.newBackup()
		id := backupID(backup)

		Eventually(func() int { return r.management.HistoryStarts(id) }, timeout, interval).Should(Equal(1))

		By("repointing the storage contract at another cluster with the same repository name")
		other := esadmintest.NewTLS()
		DeferCleanup(other.Close)
		pointStorageAt := func(endpoint string) {
			GinkgoHelper()
			Eventually(func(g Gomega) {
				var storage v1.SecondaryStorageConfig
				g.Expect(k8sClient.Get(
					ctx, client.ObjectKey{Namespace: r.namespace, Name: "storage"}, &storage,
				)).To(Succeed())
				storage.Spec.Elasticsearch.Endpoint = endpoint
				g.Expect(k8sClient.Update(ctx, &storage)).To(Succeed())
			}, timeout, interval).Should(Succeed())
		}
		pointStorageAt(other.URL())
		r.management.SetHistoryState(id, "COMPLETED", "")

		// The destination is verified on every poll of the history backup,
		// so the repoint is caught there, before anything is written to the
		// other cluster.
		expectPhase(backup, v1.LogicalBackupFailed)
		final := currentBackup(backup)
		Expect(final.Status.FailureMessage).To(SatisfyAll(
			ContainSubstring("BackupHistory"),
			ContainSubstring("endpoint"),
		))
		Expect(final.Status.History.State).To(Equal(v1.BackupPartFailed))
		Expect(r.management.Exporting()).To(Equal("running"))
		Expect(other.SnapshotCreates(r.repository, RecordsSnapshotName(id))).To(BeZero())

		By("holding the deletion while the contract points elsewhere")
		Expect(k8sClient.Delete(ctx, backup)).To(Succeed())
		Consistently(func(g Gomega) {
			var held v1.LogicalBackupElasticsearch
			g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(backup), &held)).To(Succeed())
		}, "1s", interval).Should(Succeed())

		By("completing the deletion once the contract points back")
		pointStorageAt(r.search.URL())
		Eventually(func() bool {
			var gone v1.LogicalBackupElasticsearch
			return k8sClient.Get(ctx, client.ObjectKeyFromObject(backup), &gone) != nil
		}, timeout, interval).Should(BeTrue())
	})

	// Round 10 P1: the runtime backup and the snapshot repository land in
	// the cluster's backup bucket, which is as mutable as the Elasticsearch
	// endpoint. The pin covers it, every step verifies it, and the
	// finalizer never deletes through a bucket the set is not in.
	It("fails the runtime step when the backup bucket is retargeted mid-run, and holds the deletion", func() {
		r := newRig()
		backup := r.newBackup()
		id := backupID(backup)

		Eventually(func() int { return r.management.HistoryStarts(id) }, timeout, interval).Should(Equal(1))
		r.management.SetHistoryState(id, "COMPLETED", "")
		name := RecordsSnapshotName(id)
		Eventually(func() int {
			return r.search.SnapshotCreates(r.repository, name)
		}, timeout, interval).Should(Equal(1))

		By("retargeting the bucket contract while the records snapshot runs")
		pointBucketAt := func(bucketName string) {
			GinkgoHelper()
			Eventually(func(g Gomega) {
				var bucket v1.ObjectStorageConfig
				g.Expect(k8sClient.Get(
					ctx, client.ObjectKey{Name: r.cluster.Spec.BackupStorageRef}, &bucket,
				)).To(Succeed())
				bucket.Spec.S3.BucketName = bucketName
				g.Expect(k8sClient.Update(ctx, &bucket)).To(Succeed())
			}, timeout, interval).Should(Succeed())
		}
		pointBucketAt("backups-moved")
		r.search.SetSnapshotState(r.repository, name, "SUCCESS")

		By("failing before the runtime backup is requested, naming both locations")
		expectPhase(backup, v1.LogicalBackupFailed)
		final := currentBackup(backup)
		Expect(final.Status.FailureMessage).To(SatisfyAll(
			ContainSubstring("s3://backups-moved/"),
			ContainSubstring("s3://backups/"),
		))
		Expect(final.Status.Storage.BucketRef).To(Equal(r.cluster.Spec.BackupStorageRef))
		Expect(final.Status.Storage.BucketLocation).To(HavePrefix("s3://backups/"))
		Expect(r.management.RuntimeStarts(id)).To(BeZero(), "nothing is written into the moved bucket")
		Expect(r.management.Exporting()).To(Equal("running"))

		By("holding the deletion while the contract points elsewhere")
		Expect(k8sClient.Delete(ctx, backup)).To(Succeed())
		Consistently(func(g Gomega) {
			var held v1.LogicalBackupElasticsearch
			g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(backup), &held)).To(Succeed())
			g.Expect(r.search.SnapshotExists(r.repository, name)).To(BeTrue())
		}, "1s", interval).Should(Succeed())

		By("completing the deletion, artifacts included, once the contract points back")
		pointBucketAt("backups")
		Eventually(func() bool {
			var gone v1.LogicalBackupElasticsearch
			return k8sClient.Get(ctx, client.ObjectKeyFromObject(backup), &gone) != nil
		}, timeout, interval).Should(BeTrue())
		Expect(r.search.SnapshotExists(r.repository, name)).To(BeFalse())
	})

	It("fails the step when the cluster switches to another backup bucket contract mid-run", func() {
		r := newRig()
		backup := r.newBackup()
		id := backupID(backup)
		Eventually(func() int { return r.management.HistoryStarts(id) }, timeout, interval).Should(Equal(1))

		By("pointing the cluster at another contract")
		other := &v1.ObjectStorageConfig{
			ObjectMeta: metav1.ObjectMeta{Name: "bucket-other-" + utilrand.String(8)},
			Spec: v1.ObjectStorageConfigSpec{
				Type: v1.ObjectStorageTypeS3,
				S3: &v1.S3Storage{
					BucketName: "backups-other",
					Region:     "eu-west-1",
					Auth:       v1.S3StorageAuth{Type: v1.ObjectStorageAuthTypeWorkloadIdentity},
				},
			},
		}
		Expect(k8sClient.Create(ctx, other)).To(Succeed())
		DeferCleanup(func() { _ = k8sClient.Delete(ctx, other) })
		pinned := r.cluster.Spec.BackupStorageRef
		Eventually(func(g Gomega) {
			var cluster v1.CamundaCluster
			g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(r.cluster), &cluster)).To(Succeed())
			cluster.Spec.BackupStorageRef = other.Name
			g.Expect(k8sClient.Update(ctx, &cluster)).To(Succeed())
		}, timeout, interval).Should(Succeed())

		// The history backup stays in progress until the step failed on
		// the switch, for the same reason as in the storage-loss spec: a
		// completed history races the reconcile that verified the old
		// contract, and the failure then lands in SnapshotRecords.
		expectPhase(backup, v1.LogicalBackupFailed)
		final := currentBackup(backup)
		Expect(final.Status.FailureMessage).To(SatisfyAll(
			ContainSubstring("BackupHistory"),
			ContainSubstring(other.Name),
			ContainSubstring(pinned),
		))
		Expect(r.management.Exporting()).To(Equal("running"))
	})

	It("holds a deletion that may leave exporting paused while the management client cannot be built", func() {
		r := newRig()
		backup := r.newBackup()
		id := backupID(backup)

		// Exporting is paused and the history backup ends, so nothing but
		// the binding decides the deletion.
		Eventually(func() int { return r.management.HistoryStarts(id) }, timeout, interval).Should(Equal(1))
		r.management.SetHistoryState(id, "COMPLETED", "")

		By("breaking the binding by construction: basic auth with a Secret that does not exist")
		r.publishBrokenBinding()

		By("holding the deletion instead of releasing the only thing that resumes exporting")
		Expect(k8sClient.Delete(ctx, backup)).To(Succeed())
		expectEvent(backup, eventReasonDeleteHeld, corev1.EventTypeWarning)
		Consistently(func(g Gomega) {
			var held v1.LogicalBackupElasticsearch
			g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(backup), &held)).To(Succeed())
			g.Expect(r.management.Exporting()).To(Equal("softPaused"))
		}, "1s", interval).Should(Succeed())

		By("resuming and releasing once the binding is usable again")
		r.publishBinding(3)
		Eventually(func() bool {
			var gone v1.LogicalBackupElasticsearch
			return k8sClient.Get(ctx, client.ObjectKeyFromObject(backup), &gone) != nil
		}, timeout, interval).Should(BeTrue())
		Expect(r.management.Exporting()).To(Equal("running"))
	})

	It("releases a deletion of a backup that holds no pause while the management client cannot be built", func() {
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
		Expect(r.management.Exporting()).To(Equal("running"))

		By("breaking the binding by construction, then deleting")
		r.publishBrokenBinding()
		Expect(k8sClient.Delete(ctx, backup)).To(Succeed())

		By("releasing without cleanup: nothing is paused, and the artifacts are not reachable")
		Eventually(func() bool {
			var gone v1.LogicalBackupElasticsearch
			return k8sClient.Get(ctx, client.ObjectKeyFromObject(backup), &gone) != nil
		}, timeout, interval).Should(BeTrue())
		Expect(r.search.SnapshotExists(r.repository, name)).To(BeTrue())
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

	It("keeps the claim of a ResumeFailed backup until its deletion resumes exporting", func() {
		r := newRig()
		r.management.FailNext("historyStart", 1)
		r.management.FailNext("resume", 1000000)

		backup := r.newBackup()
		backupID(backup)

		expectPhase(backup, v1.LogicalBackupFailed)
		expectReady(backup, metav1.ConditionFalse, v1.ReasonResumeFailed)
		Expect(r.management.Exporting()).To(Equal("softPaused"))

		By("holding the Lease after the terminal phase: the cluster is still paused")
		Consistently(func() string { return r.leaseHolder() }, "1s", interval).
			Should(Equal(claimant(currentBackup(backup)).String()))

		By("blocking a sibling with a message that names the pause")
		sibling := r.newBackup()
		expectReady(sibling, metav1.ConditionFalse, v1.ReasonBackupInProgress)
		ready := meta.FindStatusCondition(currentBackup(sibling).Status.Conditions, v1.ConditionReady)
		Expect(ready.Message).To(SatisfyAll(
			ContainSubstring(backup.Name),
			ContainSubstring("still paused"),
			ContainSubstring("deletion or repair"),
		))

		By("deleting the ResumeFailed backup while resume still fails: the deletion holds")
		Expect(k8sClient.Delete(ctx, backup)).To(Succeed())
		Consistently(func(g Gomega) {
			var held v1.LogicalBackupElasticsearch
			g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(backup), &held)).To(Succeed())
			g.Expect(currentBackup(sibling).Status.BackupID).To(BeZero(), "the sibling stays blocked")
		}, "1s", interval).Should(Succeed())

		By("resuming, releasing, and only then letting the sibling start")
		pausesBefore := r.management.PauseCalls()
		r.management.FailNext("resume", 0)
		Eventually(func() bool {
			var gone v1.LogicalBackupElasticsearch
			return k8sClient.Get(ctx, client.ObjectKeyFromObject(backup), &gone) != nil
		}, timeout, interval).Should(BeTrue())
		Expect(r.management.ResumeCalls()).To(BeNumerically(">=", 1), "the finalizer resumed exporting")

		siblingID := backupID(sibling)
		Expect(r.leaseHolder()).To(Equal(claimant(currentBackup(sibling)).String()))
		// The sibling pauses exporting again for its own run. Its pause is
		// a new one, after the resume of the finalizer. The Consistently
		// block above proved that the sibling never started while the
		// holder lived. The resume therefore never fired inside a running
		// sibling.
		Eventually(func() int { return r.management.HistoryStarts(siblingID) }, timeout, interval).Should(Equal(1))
		Expect(r.management.PauseCalls()).To(Equal(pausesBefore+1), "the sibling paused anew for its own run")
		Expect(r.management.Exporting()).To(Equal("softPaused"))
	})

	It("holds the deletion while the history status query is rejected", func() {
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

		By("rejecting every history status query, then deleting")
		r.management.FailNext("historyStatus", 1000000)
		Expect(k8sClient.Delete(ctx, backup)).To(Succeed())

		Consistently(func(g Gomega) {
			var held v1.LogicalBackupElasticsearch
			g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(backup), &held)).To(Succeed())
			g.Expect(r.search.SnapshotExists(r.repository, name)).To(BeTrue())
		}, "1s", interval).Should(Succeed())

		By("releasing and cleaning up once the query answers again")
		r.management.FailNext("historyStatus", 0)
		Eventually(func() bool {
			var gone v1.LogicalBackupElasticsearch
			return k8sClient.Get(ctx, client.ObjectKeyFromObject(backup), &gone) != nil
		}, timeout, interval).Should(BeTrue())
		Expect(r.search.SnapshotExists(r.repository, name)).To(BeFalse())
	})

	// Round 10 S1: the finalizer runs before the deferred status flush of
	// Reconcile. A history snapshot name it discovers must be durable before
	// the first delete. If it is not, a crash mid-way and a cluster gone by
	// the next reconcile leave the sweep the old list. The name then leaks.
	It("persists the history snapshot names it discovers before it deletes anything", func() {
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
		recorded := currentBackup(backup).Status.HistorySnapshots
		Expect(recorded).NotTo(BeEmpty())
		// The web applications wrote the snapshots into the repository.
		for _, snapshot := range recorded {
			r.search.SetSnapshotState(r.repository, snapshot, "SUCCESS")
		}

		By("forgetting the names, as a resource that died before recording them would have")
		Eventually(func(g Gomega) {
			current := currentBackup(backup)
			current.Status.HistorySnapshots = nil
			g.Expect(k8sClient.Status().Update(ctx, current)).To(Succeed())
		}, timeout, interval).Should(Succeed())

		By("deleting while every snapshot delete is rejected: nothing goes, the names come back durably")
		r.search.FailNext("snapshotDelete", 1000000)
		Expect(k8sClient.Delete(ctx, backup)).To(Succeed())
		Eventually(func() []string {
			return currentBackup(backup).Status.HistorySnapshots
		}, timeout, interval).Should(ConsistOf(recorded), "the discovered names are persisted before the deletes")
		for _, snapshot := range recorded {
			Expect(r.search.SnapshotExists(r.repository, snapshot)).To(BeTrue())
		}

		By("deleting them once the deletes are served")
		r.search.FailNext("snapshotDelete", 0)
		Eventually(func() bool {
			var gone v1.LogicalBackupElasticsearch
			return k8sClient.Get(ctx, client.ObjectKeyFromObject(backup), &gone) != nil
		}, timeout, interval).Should(BeTrue())
		for _, snapshot := range recorded {
			Expect(r.search.SnapshotExists(r.repository, snapshot)).To(BeFalse())
		}
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
