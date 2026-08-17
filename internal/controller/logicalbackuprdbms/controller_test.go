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

package logicalbackuprdbms

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	eventsv1 "k8s.io/api/events/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	utilrand "k8s.io/apimachinery/pkg/util/rand"
	"sigs.k8s.io/controller-runtime/pkg/client"

	v1 "github.com/konsole-is/camunda-operator/api/v1"
	"github.com/konsole-is/camunda-operator/internal/fixtures"
	"github.com/konsole-is/camunda-operator/pkg/camundaadmin"
	camundacluster "github.com/konsole-is/camunda-operator/pkg/components/camundacluster"
	components "github.com/konsole-is/camunda-operator/pkg/components/logicalbackuprdbms"
	"github.com/konsole-is/camunda-operator/pkg/logicalbackup"
)

// world is the resolved fixture set of one spec: a relational cluster with a
// published binding, its storage chain, and every Secret the backup needs.
// Both credential Secrets live in the cluster namespace, so the backup reads
// them directly; the cross-namespace spec stands in the copies itself.
type world struct {
	namespace string
	cluster   *v1.CamundaCluster
	storage   *v1.SecondaryStorageConfig
	dbConfig  *v1.DatabaseConfig
	server    *v1.DatabaseServerConfig
	bucket    *v1.ObjectStorageConfig
}

func newNamespace() string {
	name := "lbr-ns-" + utilrand.String(8)
	Expect(k8sClient.Create(ctx, &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{Name: name},
	})).To(Succeed())

	return name
}

// createWorld builds a relational cluster whose management binding points at
// the fake management API, with the credential copies the CamundaCluster
// controller would have made. Mutators shape the cluster before it is
// created.
func createWorld(mutate ...func(*v1.CamundaCluster)) *world {
	namespace := newNamespace()
	suffix := utilrand.String(6)

	server := &v1.DatabaseServerConfig{
		ObjectMeta: metav1.ObjectMeta{Name: "dbsc-" + suffix},
		Spec: v1.DatabaseServerConfigSpec{
			Engine:  v1.DatabaseEnginePostgres,
			Host:    "postgres.databases.svc",
			Port:    5432,
			Version: "17",
			AdminCredentialsSecretRef: v1.CredentialsSecretRef{
				Name: "admin", Namespace: namespace,
				UsernameKey: "username", PasswordKey: "password",
			},
		},
	}
	Expect(k8sClient.Create(ctx, server)).To(Succeed())
	DeferCleanup(func() { _ = k8sClient.Delete(ctx, server) })

	dbCredentials := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: "backup-user", Namespace: namespace},
		Data:       map[string][]byte{"username": []byte("backup"), "password": []byte("s3cr3t")},
	}
	Expect(k8sClient.Create(ctx, dbCredentials)).To(Succeed())

	dbConfig := &v1.DatabaseConfig{
		ObjectMeta: metav1.ObjectMeta{Name: "dbc-" + suffix, Namespace: namespace},
		Spec: v1.DatabaseConfigSpec{
			ServerRef:    server.Name,
			DatabaseName: "camunda",
			CredentialsSecretRef: v1.CredentialsSecretRef{
				Name: "app-user", Namespace: namespace,
				UsernameKey: "username", PasswordKey: "password",
			},
			BackupCredentialsSecretRef: &v1.CredentialsSecretRef{
				Name: dbCredentials.Name, Namespace: namespace,
				UsernameKey: "username", PasswordKey: "password",
			},
		},
	}
	Expect(k8sClient.Create(ctx, dbConfig)).To(Succeed())

	storage := &v1.SecondaryStorageConfig{
		ObjectMeta: metav1.ObjectMeta{Name: "ssc-" + suffix, Namespace: namespace},
		Spec: v1.SecondaryStorageConfigSpec{
			Type:  v1.SecondaryStorageTypeRDBMS,
			RDBMS: &v1.RDBMSStorage{DatabaseConfigRef: dbConfig.Name},
		},
	}
	Expect(k8sClient.Create(ctx, storage)).To(Succeed())

	bucket := &v1.ObjectStorageConfig{
		ObjectMeta: metav1.ObjectMeta{Name: "osc-" + suffix},
		Spec: v1.ObjectStorageConfigSpec{
			Type: v1.ObjectStorageTypeS3,
			S3: &v1.S3Storage{
				BucketName: "camunda-backups",
				BasePath:   "clusters",
				Endpoint:   "http://minio.minio.svc:9000",
				Auth: v1.S3StorageAuth{
					Type: v1.ObjectStorageAuthTypeCredentials,
					Credentials: &v1.S3Credentials{
						SecretRef: v1.S3CredentialsSecretRef{
							Name: "minio-credentials", Namespace: namespace,
							AccessKeyIDKey: "accessKeyId", SecretAccessKeyKey: "secretAccessKey",
						},
					},
				},
			},
		},
	}
	Expect(k8sClient.Create(ctx, bucket)).To(Succeed())
	DeferCleanup(func() { _ = k8sClient.Delete(ctx, bucket) })

	platform := fixtures.CamundaPlatformConfigBasic()
	Expect(k8sClient.Create(ctx, platform)).To(Succeed())
	DeferCleanup(func() { _ = k8sClient.Delete(ctx, platform) })

	cluster := &v1.CamundaCluster{
		ObjectMeta: metav1.ObjectMeta{Name: "cc-" + suffix, Namespace: namespace},
		Spec: v1.CamundaClusterSpec{
			Version:           "8.9.9",
			PlatformConfigRef: platform.Name,
			StorageRef:        storage.Name,
			BackupStorageRef:  bucket.Name,
		},
	}
	for _, m := range mutate {
		m(cluster)
	}
	Expect(k8sClient.Create(ctx, cluster)).To(Succeed())

	// The binding and the volume status that the CamundaCluster controller
	// would publish.
	cluster.Status.Management = &v1.ManagementBinding{
		Endpoint:   management.URL(),
		Auth:       v1.ManagementAuth{Method: v1.ManagementAuthMethodNone},
		Version:    "8.9.9",
		Partitions: 3,
	}
	cluster.Status.Volumes = []v1.VolumeStatus{
		{Name: "data-cc-0", Capacity: resource.MustParse("15Gi")},
	}
	Expect(k8sClient.Status().Update(ctx, cluster)).To(Succeed())

	// The bucket credentials live in the cluster namespace, so the backup
	// uses the source Secret directly — no copy is involved.
	bucketCredentials := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: "minio-credentials", Namespace: namespace},
		Data: map[string][]byte{
			"accessKeyId": []byte("minio-root"), "secretAccessKey": []byte("minio-secret"),
		},
	}
	Expect(k8sClient.Create(ctx, bucketCredentials)).To(Succeed())

	return &world{
		namespace: namespace,
		cluster:   cluster,
		storage:   storage,
		dbConfig:  dbConfig,
		server:    server,
		bucket:    bucket,
	}
}

func createBackup(w *world, mutate ...func(*v1.LogicalBackupRDBMS)) *v1.LogicalBackupRDBMS {
	backup := &v1.LogicalBackupRDBMS{
		ObjectMeta: metav1.ObjectMeta{
			Name: "backup-" + utilrand.String(6), Namespace: w.namespace,
		},
		Spec: v1.LogicalBackupRDBMSSpec{
			ClusterRef: v1.ClusterRef{Name: w.cluster.Name},
		},
	}
	for _, m := range mutate {
		m(backup)
	}
	Expect(k8sClient.Create(ctx, backup)).To(Succeed())

	return backup
}

// jobOf waits for the dump Job of backup in the cluster namespace and returns
// it.
func jobOf(backup *v1.LogicalBackupRDBMS, w *world) *batchv1.Job {
	GinkgoHelper()
	var job batchv1.Job
	key := types.NamespacedName{Namespace: w.namespace, Name: components.JobName(backup)}
	Eventually(func(g Gomega) {
		g.Expect(k8sClient.Get(ctx, key, &job)).To(Succeed())
	}, timeout, interval).Should(Succeed())

	return &job
}

func readyCondition(backup *v1.LogicalBackupRDBMS) *metav1.Condition {
	return meta.FindStatusCondition(backup.Status.Conditions, v1.ConditionReady)
}

// expectPending asserts a backup parked at admission: the Pending phase, no
// identity allocated, and the given Ready reason.
func expectPending(backup *v1.LogicalBackupRDBMS, reason string) {
	GinkgoHelper()
	Eventually(func(g Gomega) {
		g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(backup), backup)).To(Succeed())
		g.Expect(backup.Status.Phase).To(Equal(v1.LogicalBackupPending))
		g.Expect(backup.Status.BackupID).To(BeZero())
		condition := readyCondition(backup)
		g.Expect(condition).NotTo(BeNil())
		g.Expect(condition.Reason).To(Equal(reason))
	}, timeout, interval).Should(Succeed())
}

// secretNameOfEnv returns the Secret a container's env variable reads from,
// or the empty string when the variable is absent or not a secretKeyRef.
func secretNameOfEnv(container corev1.Container, name string) string {
	for _, env := range container.Env {
		if env.Name == name && env.ValueFrom != nil && env.ValueFrom.SecretKeyRef != nil {
			return env.ValueFrom.SecretKeyRef.Name
		}
	}

	return ""
}

// markJob flips the Job of backup to the given terminal condition, standing
// in for a kubelet that ran the pod.
func markJob(backup *v1.LogicalBackupRDBMS, w *world, kind batchv1.JobConditionType) {
	GinkgoHelper()
	var job batchv1.Job
	key := types.NamespacedName{Namespace: w.namespace, Name: components.JobName(backup)}
	Eventually(func(g Gomega) {
		g.Expect(k8sClient.Get(ctx, key, &job)).To(Succeed())
	}, timeout, interval).Should(Succeed())

	// The API server demands the full bookkeeping of a finished Job: the
	// precursor condition, the start time, and, for a completed one, the
	// completion time.
	now := metav1.Now()
	precursor := batchv1.JobSuccessCriteriaMet
	if kind == batchv1.JobFailed {
		precursor = batchv1.JobFailureTarget
	}
	job.Status.StartTime = &now
	job.Status.Conditions = append(
		job.Status.Conditions,
		batchv1.JobCondition{
			Type: precursor, Status: corev1.ConditionTrue,
			Reason: "Test", Message: "marked by the suite",
		},
		batchv1.JobCondition{
			Type: kind, Status: corev1.ConditionTrue,
			Reason: "Test", Message: "marked by the suite",
		},
	)
	if kind == batchv1.JobComplete {
		job.Status.Succeeded = 1
		job.Status.CompletionTime = &now
	}
	Expect(k8sClient.Status().Update(ctx, &job)).To(Succeed())
}

var _ = Describe("LogicalBackupRDBMS controller", func() {
	It("dumps, requests the primary-storage backup, and completes", func() {
		w := createWorld()
		backup := createBackup(w)

		By("allocating the identity and starting the Job")
		Eventually(func(g Gomega) {
			g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(backup), backup)).To(Succeed())
			g.Expect(backup.Status.BackupID).NotTo(BeZero())
			g.Expect(backup.Status.Phase).To(Equal(v1.LogicalBackupRunning))
			g.Expect(backup.Status.Step).To(Equal(v1.StepDumping))
		}, timeout, interval).Should(Succeed())

		By("recording the effective restore size of the brokers, rounded up")
		Expect(backup.Status.StorageSizes.Zeebe).NotTo(BeNil())
		Expect(backup.Status.StorageSizes.Zeebe.String()).To(Equal("20Gi"))

		By("pinning the bucket the backup writes through")
		Expect(backup.Status.BucketRef).To(Equal(w.bucket.Name))

		By("rendering the Job under the cluster ServiceAccount with the recorded key")
		job := jobOf(backup, w)
		Expect(job.Spec.Template.Spec.ServiceAccountName).To(Equal(w.cluster.Name + "-camunda"))
		Expect(job.OwnerReferences).NotTo(BeEmpty())

		By("projecting the same-namespace bucket credentials Secret directly")
		upload := job.Spec.Template.Spec.Containers[0]
		Expect(secretNameOfEnv(upload, components.EnvUploadCredentialPrefix+"0")).To(
			Equal("minio-credentials"),
		)

		By("moving to the primary-storage backup once the Job completes")
		markJob(backup, w, batchv1.JobComplete)
		Eventually(func(g Gomega) {
			g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(backup), backup)).To(Succeed())
			g.Expect(backup.Status.Step).To(Equal(v1.StepPrimaryBackup))
			g.Expect(backup.Status.PrimaryBackupID).NotTo(BeNil())
		}, timeout, interval).Should(Succeed())

		By("recording the id the cluster generated and never re-requesting it")
		id := *backup.Status.PrimaryBackupID
		Consistently(func(g Gomega) {
			g.Expect(management.RuntimeStarts(id)).To(Equal(1))
		}, "2s", interval).Should(Succeed())

		By("completing once the cluster reports the backup done")
		management.SetRuntimeState(id, string(camundaadmin.StateCompleted), "")
		Eventually(func(g Gomega) {
			g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(backup), backup)).To(Succeed())
			g.Expect(backup.Status.Phase).To(Equal(v1.LogicalBackupCompleted))
			g.Expect(backup.Status.CompletionTime).NotTo(BeNil())
			condition := readyCondition(backup)
			g.Expect(condition).NotTo(BeNil())
			g.Expect(condition.Status).To(Equal(metav1.ConditionTrue))
			g.Expect(condition.Reason).To(Equal(v1.ReasonCompleted))
		}, timeout, interval).Should(Succeed())
	})

	It("fails when the dump Job fails, naming the reason", func() {
		w := createWorld()
		backup := createBackup(w)

		markJob(backup, w, batchv1.JobFailed)
		Eventually(func(g Gomega) {
			g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(backup), backup)).To(Succeed())
			g.Expect(backup.Status.Phase).To(Equal(v1.LogicalBackupFailed))
			condition := readyCondition(backup)
			g.Expect(condition).NotTo(BeNil())
			g.Expect(condition.Reason).To(Equal(v1.ReasonFailed))
			g.Expect(condition.Message).To(ContainSubstring("dump Job failed"))
		}, timeout, interval).Should(Succeed())
	})

	It("reports InvalidReference for a missing cluster", func() {
		namespace := newNamespace()
		backup := &v1.LogicalBackupRDBMS{
			ObjectMeta: metav1.ObjectMeta{Name: "backup-" + utilrand.String(6), Namespace: namespace},
			Spec:       v1.LogicalBackupRDBMSSpec{ClusterRef: v1.ClusterRef{Name: "nowhere"}},
		}
		Expect(k8sClient.Create(ctx, backup)).To(Succeed())

		expectPending(backup, v1.ReasonInvalidReference)
	})

	It("waits with ClusterSuspended while the cluster is suspended", func() {
		w := createWorld()
		Eventually(func(g Gomega) {
			g.Expect(
				k8sClient.Get(ctx, client.ObjectKeyFromObject(w.cluster), w.cluster),
			).To(Succeed())
			w.cluster.Spec.Suspend = true
			g.Expect(k8sClient.Update(ctx, w.cluster)).To(Succeed())
		}, timeout, interval).Should(Succeed())

		backup := createBackup(w)
		expectPending(backup, v1.ReasonClusterSuspended)
	})

	It("rejects a cluster on the wrong storage type", func() {
		w := createWorld()
		Eventually(func(g Gomega) {
			g.Expect(
				k8sClient.Get(ctx, client.ObjectKeyFromObject(w.storage), w.storage),
			).To(Succeed())
			w.storage.Spec = v1.SecondaryStorageConfigSpec{
				Type: v1.SecondaryStorageTypeElasticsearch,
				Elasticsearch: &v1.ElasticsearchStorage{
					Endpoint: "https://es." + w.namespace + ".svc:9200",
					CredentialsSecretRef: v1.CredentialsSecretRef{
						Name: "es", Namespace: w.namespace,
						UsernameKey: "username", PasswordKey: "password",
					},
				},
			}
			g.Expect(k8sClient.Update(ctx, w.storage)).To(Succeed())
		}, timeout, interval).Should(Succeed())

		backup := createBackup(w)
		expectPending(backup, v1.ReasonStorageTypeMismatch)
	})

	It("serializes backups of one cluster and lets the waiter start afterwards", func() {
		w := createWorld()
		first := createBackup(w)

		Eventually(func(g Gomega) {
			g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(first), first)).To(Succeed())
			g.Expect(first.Status.Phase).To(Equal(v1.LogicalBackupRunning))
		}, timeout, interval).Should(Succeed())

		second := createBackup(w)
		expectPending(second, v1.ReasonBackupInProgress)

		By("finishing the first backup")
		markJob(first, w, batchv1.JobComplete)
		Eventually(func(g Gomega) {
			g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(first), first)).To(Succeed())
			g.Expect(first.Status.PrimaryBackupID).NotTo(BeNil())
		}, timeout, interval).Should(Succeed())
		management.SetRuntimeState(
			*first.Status.PrimaryBackupID, string(camundaadmin.StateCompleted), "",
		)
		Eventually(func(g Gomega) {
			g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(first), first)).To(Succeed())
			g.Expect(first.Status.Phase).To(Equal(v1.LogicalBackupCompleted))
		}, timeout, interval).Should(Succeed())

		By("letting the waiting backup start, so a done backup never deadlocks the queue")
		Eventually(func(g Gomega) {
			g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(second), second)).To(Succeed())
			g.Expect(second.Status.BackupID).NotTo(BeZero())
			g.Expect(second.Status.Phase).To(Equal(v1.LogicalBackupRunning))
		}, timeout, interval).Should(Succeed())
	})

	It("consults the sibling kind through the seam", func() {
		w := createWorld()
		// The seam contract reports only STARTED, non-terminal siblings; the
		// fake stands in for one that already allocated its identity.
		setSibling(func(context.Context, types.NamespacedName) (string, error) {
			return "an-elasticsearch-backup", nil
		})
		DeferCleanup(func() { setSibling(nil) })

		backup := createBackup(w)
		expectPending(backup, v1.ReasonBackupInProgress)
	})

	It("reports MissingSecret when the database has no backup credentials", func() {
		w := createWorld()
		Eventually(func(g Gomega) {
			g.Expect(
				k8sClient.Get(ctx, client.ObjectKeyFromObject(w.dbConfig), w.dbConfig),
			).To(Succeed())
			w.dbConfig.Spec.BackupCredentialsSecretRef = nil
			g.Expect(k8sClient.Update(ctx, w.dbConfig)).To(Succeed())
		}, timeout, interval).Should(Succeed())

		backup := createBackup(w)
		expectPending(backup, v1.ReasonMissingSecret)
	})

	It("requires the server version", func() {
		w := createWorld()
		Eventually(func(g Gomega) {
			g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(w.server), w.server)).To(Succeed())
			w.server.Spec.Version = ""
			g.Expect(k8sClient.Update(ctx, w.server)).To(Succeed())
		}, timeout, interval).Should(Succeed())

		backup := createBackup(w)
		expectPending(backup, v1.ReasonInvalidReference)
		Expect(readyCondition(backup).Message).To(ContainSubstring("version"))
	})

	It("reports MissingCredentials until the bucket credentials resolve", func() {
		w := createWorld()
		Expect(k8sClient.Delete(ctx, &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{Name: "minio-credentials", Namespace: w.namespace},
		})).To(Succeed())

		backup := createBackup(w)
		expectPending(backup, v1.ReasonMissingCredentials)
	})

	It("parks in Pending until the binding is published, then starts", func() {
		w := createWorld()
		Eventually(func(g Gomega) {
			g.Expect(
				k8sClient.Get(ctx, client.ObjectKeyFromObject(w.cluster), w.cluster),
			).To(Succeed())
			w.cluster.Status.Management = nil
			g.Expect(k8sClient.Status().Update(ctx, w.cluster)).To(Succeed())
		}, timeout, interval).Should(Succeed())

		// The binding is checked at admission, so a backup never dumps
		// gigabytes it cannot pair with a primary-storage backup afterwards.
		backup := createBackup(w)
		expectPending(backup, v1.ReasonProgressing)

		By("starting once the binding is published")
		Eventually(func(g Gomega) {
			g.Expect(
				k8sClient.Get(ctx, client.ObjectKeyFromObject(w.cluster), w.cluster),
			).To(Succeed())
			w.cluster.Status.Management = &v1.ManagementBinding{
				Endpoint:   management.URL(),
				Auth:       v1.ManagementAuth{Method: v1.ManagementAuthMethodNone},
				Version:    "8.9.9",
				Partitions: 3,
			}
			g.Expect(k8sClient.Status().Update(ctx, w.cluster)).To(Succeed())
		}, timeout, interval).Should(Succeed())
		Eventually(func(g Gomega) {
			g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(backup), backup)).To(Succeed())
			g.Expect(backup.Status.BackupID).NotTo(BeZero())
			g.Expect(backup.Status.Phase).To(Equal(v1.LogicalBackupRunning))
		}, timeout, interval).Should(Succeed())
	})

	It("deletes the Job and the dump object on deletion, never the primary-storage backups", func() {
		w := createWorld()
		backup := createBackup(w)

		markJob(backup, w, batchv1.JobComplete)
		Eventually(func(g Gomega) {
			g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(backup), backup)).To(Succeed())
			g.Expect(backup.Status.PrimaryBackupID).NotTo(BeNil())
		}, timeout, interval).Should(Succeed())
		id := *backup.Status.PrimaryBackupID
		management.SetRuntimeState(id, string(camundaadmin.StateCompleted), "")
		Eventually(func(g Gomega) {
			g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(backup), backup)).To(Succeed())
			g.Expect(backup.Status.Phase).To(Equal(v1.LogicalBackupCompleted))
		}, timeout, interval).Should(Succeed())
		objectKey := backup.Status.ObjectKey

		Expect(k8sClient.Delete(ctx, backup)).To(Succeed())

		By("removing the finalizer once the artifacts are gone")
		Eventually(func(g Gomega) {
			err := k8sClient.Get(ctx, client.ObjectKeyFromObject(backup), backup)
			g.Expect(client.IgnoreNotFound(err)).To(Succeed())
			g.Expect(err).To(HaveOccurred())
		}, timeout, interval).Should(Succeed())

		By("deleting the dump object by its exact key")
		Expect(bucket.Deleted()).To(ContainElement(objectKey))

		By("leaving the primary-storage backup alone")
		Expect(management.RuntimeBackup(id)).NotTo(BeNil())

		// envtest runs no garbage collector, so a Job still present here
		// would mean the finalizer relied on the owner reference instead of
		// deleting it.
		By("deleting the Job itself")
		var leftover batchv1.Job
		Expect(k8sClient.Get(
			ctx, types.NamespacedName{
				Namespace: w.namespace, Name: components.JobName(backup),
			}, &leftover,
		)).To(HaveOccurred())
	})

	It("releases the finalizer when the cluster is already gone", func() {
		w := createWorld()
		backup := createBackup(w)

		By("waiting for the identity, so an object key exists to clean up")
		Eventually(func(g Gomega) {
			g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(backup), backup)).To(Succeed())
			g.Expect(backup.Status.ObjectKey).NotTo(BeEmpty())
		}, timeout, interval).Should(Succeed())

		By("deleting the cluster first")
		Expect(k8sClient.Delete(ctx, w.cluster)).To(Succeed())

		Expect(k8sClient.Delete(ctx, backup)).To(Succeed())

		By("recording what was left behind and releasing anyway")
		Eventually(func(g Gomega) {
			err := k8sClient.Get(ctx, client.ObjectKeyFromObject(backup), backup)
			g.Expect(err).To(HaveOccurred())
			g.Expect(client.IgnoreNotFound(err)).To(Succeed())

			var events eventsv1.EventList
			g.Expect(k8sClient.List(ctx, &events, client.InNamespace(w.namespace))).To(Succeed())
			reasons := make([]string, 0, len(events.Items))
			for _, event := range events.Items {
				reasons = append(reasons, event.Reason)
			}
			g.Expect(reasons).To(ContainElement("ArtifactCleanupFailed"))
		}, timeout, interval).Should(Succeed())
	})

	It("retries a conflicted primary-storage request instead of failing", func() {
		w := createWorld()
		management.ConflictNextRuntimeStart(1)
		backup := createBackup(w)

		// The dump already succeeded; one bad answer must not discard it. The
		// conflict is retried with backoff and the retry generates a fresh
		// id — the id that conflicted is never adopted.
		markJob(backup, w, batchv1.JobComplete)
		Eventually(func(g Gomega) {
			g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(backup), backup)).To(Succeed())
			g.Expect(backup.Status.PrimaryBackupID).NotTo(BeNil())
			g.Expect(backup.Status.Phase).To(Equal(v1.LogicalBackupRunning))
		}, timeout, interval).Should(Succeed())

		id := *backup.Status.PrimaryBackupID
		management.SetRuntimeState(id, string(camundaadmin.StateCompleted), "")
		Eventually(func(g Gomega) {
			g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(backup), backup)).To(Succeed())
			g.Expect(backup.Status.Phase).To(Equal(v1.LogicalBackupCompleted))
		}, timeout, interval).Should(Succeed())
	})

	It("applies the cluster's own dump block when the backup sets none", func() {
		w := createWorld(func(cluster *v1.CamundaCluster) {
			cluster.Spec.Backup = &v1.ClusterBackupSpec{
				Dump: &v1.BackupDumpSpec{
					PodAnnotations: map[string]string{"linkerd.io/inject": "disabled"},
				},
			}
		})
		backup := createBackup(w)

		job := jobOf(backup, w)
		Expect(job.Spec.Template.Annotations).To(
			HaveKeyWithValue("linkerd.io/inject", "disabled"),
		)
	})

	It("runs the Job under an overridden ServiceAccount name", func() {
		w := createWorld(func(cluster *v1.CamundaCluster) {
			cluster.Spec.ServiceAccount = &v1.ServiceAccountSpec{Name: "platform-sa"}
		})
		backup := createBackup(w)

		job := jobOf(backup, w)
		Expect(job.Spec.Template.Spec.ServiceAccountName).To(Equal("platform-sa"))
	})

	It("honors the backup's own dump block over the cluster's", func() {
		w := createWorld()
		backup := createBackup(w, func(backup *v1.LogicalBackupRDBMS) {
			backup.Spec.Dump = &v1.BackupDumpSpec{
				PodAnnotations: map[string]string{"sidecar.istio.io/inject": "false"},
			}
		})

		job := jobOf(backup, w)
		Expect(job.Spec.Template.Annotations).To(
			HaveKeyWithValue("sidecar.istio.io/inject", "false"),
		)
	})

	It("fails a running backup whose dependency stays broken past the grace", func() {
		w := createWorld()
		backup := createBackup(w)

		markJob(backup, w, batchv1.JobComplete)
		Eventually(func(g Gomega) {
			g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(backup), backup)).To(Succeed())
			g.Expect(backup.Status.Step).To(Equal(v1.StepPrimaryBackup))
		}, timeout, interval).Should(Succeed())

		By("deleting the cluster mid-run")
		Expect(k8sClient.Delete(ctx, w.cluster)).To(Succeed())

		By("terminalizing after the grace, so the backup never parks forever")
		Eventually(func(g Gomega) {
			g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(backup), backup)).To(Succeed())
			g.Expect(backup.Status.Phase).To(Equal(v1.LogicalBackupFailed))
			g.Expect(backup.Status.FailureMessage).To(ContainSubstring("stopped resolving"))
		}, "15s", interval).Should(Succeed())
	})

	It("holds a mid-run failure and recovers when the dependency returns", func() {
		w := createWorld()
		backup := createBackup(w)
		jobOf(backup, w)

		// Once the Job is tracked, the dump step needs no resolution; the
		// missing cluster bites when the primary-storage step starts.
		By("deleting the cluster, then finishing the dump")
		Expect(k8sClient.Delete(ctx, w.cluster)).To(Succeed())
		markJob(backup, w, batchv1.JobComplete)

		By("holding the backup within the grace, failure clock running")
		Eventually(func(g Gomega) {
			g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(backup), backup)).To(Succeed())
			g.Expect(backup.Status.Phase).To(Equal(v1.LogicalBackupRunning))
			g.Expect(backup.Status.FirstFailedAt).NotTo(BeNil())
		}, timeout, interval).Should(Succeed())

		By("bringing the cluster back within the grace")
		revived := &v1.CamundaCluster{
			ObjectMeta: metav1.ObjectMeta{Name: w.cluster.Name, Namespace: w.namespace},
			Spec:       w.cluster.Spec,
		}
		Expect(k8sClient.Create(ctx, revived)).To(Succeed())
		revived.Status.Management = &v1.ManagementBinding{
			Endpoint:   management.URL(),
			Auth:       v1.ManagementAuth{Method: v1.ManagementAuthMethodNone},
			Version:    "8.9.9",
			Partitions: 3,
		}
		Expect(k8sClient.Status().Update(ctx, revived)).To(Succeed())

		By("recovering: the failure clock clears and the backup completes")
		Eventually(func(g Gomega) {
			g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(backup), backup)).To(Succeed())
			g.Expect(backup.Status.PrimaryBackupID).NotTo(BeNil())
			g.Expect(backup.Status.FirstFailedAt).To(BeNil())
		}, timeout, interval).Should(Succeed())
		management.SetRuntimeState(
			*backup.Status.PrimaryBackupID, string(camundaadmin.StateCompleted), "",
		)
		Eventually(func(g Gomega) {
			g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(backup), backup)).To(Succeed())
			g.Expect(backup.Status.Phase).To(Equal(v1.LogicalBackupCompleted))
		}, timeout, interval).Should(Succeed())
	})

	It("fails when the management API stays unreachable past the grace", func() {
		w := createWorld()
		Eventually(func(g Gomega) {
			g.Expect(
				k8sClient.Get(ctx, client.ObjectKeyFromObject(w.cluster), w.cluster),
			).To(Succeed())
			w.cluster.Status.Management.Endpoint = "http://127.0.0.1:1"
			g.Expect(k8sClient.Status().Update(ctx, w.cluster)).To(Succeed())
		}, timeout, interval).Should(Succeed())

		backup := createBackup(w)
		markJob(backup, w, batchv1.JobComplete)

		By("terminalizing with the endpoint named, instead of parking forever")
		Eventually(func(g Gomega) {
			g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(backup), backup)).To(Succeed())
			g.Expect(backup.Status.Phase).To(Equal(v1.LogicalBackupFailed))
			g.Expect(backup.Status.FailureMessage).To(ContainSubstring("127.0.0.1:1"))
			g.Expect(backup.Status.FailureMessage).To(ContainSubstring("not reachable"))
		}, "15s", interval).Should(Succeed())
	})

	It("parks in Pending when the dump credentials Secret is gone", func() {
		w := createWorld()
		Expect(k8sClient.Delete(ctx, &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{Name: "backup-user", Namespace: w.namespace},
		})).To(Succeed())

		backup := createBackup(w)
		expectPending(backup, v1.ReasonMissingSecret)
	})

	It("tolerates a primary-storage backup that has not registered yet", func() {
		w := createWorld()
		backup := createBackup(w)

		markJob(backup, w, batchv1.JobComplete)
		Eventually(func(g Gomega) {
			g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(backup), backup)).To(Succeed())
			g.Expect(backup.Status.PrimaryBackupID).NotTo(BeNil())
			g.Expect(backup.Status.PrimaryBackupRequestedAt).NotTo(BeNil())
		}, timeout, interval).Should(Succeed())
		id := *backup.Status.PrimaryBackupID

		// The partitions register their parts asynchronously after the 202,
		// so a backup the cluster does not report yet is normal at first.
		management.SetRuntimeState(id, string(camundaadmin.StateDoesNotExist), "")
		Consistently(func(g Gomega) {
			g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(backup), backup)).To(Succeed())
			g.Expect(backup.Status.Phase).To(Equal(v1.LogicalBackupRunning))
		}, "1500ms", interval).Should(Succeed())

		management.SetRuntimeState(id, string(camundaadmin.StateCompleted), "")
		Eventually(func(g Gomega) {
			g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(backup), backup)).To(Succeed())
			g.Expect(backup.Status.Phase).To(Equal(v1.LogicalBackupCompleted))
		}, timeout, interval).Should(Succeed())
	})

	It("fails a primary-storage backup still unregistered past the grace", func() {
		w := createWorld()
		backup := createBackup(w)

		markJob(backup, w, batchv1.JobComplete)
		Eventually(func(g Gomega) {
			g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(backup), backup)).To(Succeed())
			g.Expect(backup.Status.PrimaryBackupID).NotTo(BeNil())
		}, timeout, interval).Should(Succeed())

		management.SetRuntimeState(
			*backup.Status.PrimaryBackupID, string(camundaadmin.StateDoesNotExist), "",
		)
		Eventually(func(g Gomega) {
			g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(backup), backup)).To(Succeed())
			g.Expect(backup.Status.Phase).To(Equal(v1.LogicalBackupFailed))
			g.Expect(backup.Status.FailureMessage).To(ContainSubstring("DOES_NOT_EXIST"))
		}, timeout, interval).Should(Succeed())
	})

	It("backs up a cluster in another namespace end to end", func() {
		w := createWorld()
		backupNamespace := newNamespace()
		backup := &v1.LogicalBackupRDBMS{
			ObjectMeta: metav1.ObjectMeta{
				Name: "backup-" + utilrand.String(6), Namespace: backupNamespace,
			},
			Spec: v1.LogicalBackupRDBMSSpec{
				ClusterRef: v1.ClusterRef{Name: w.cluster.Name, Namespace: w.namespace},
			},
		}
		Expect(k8sClient.Create(ctx, backup)).To(Succeed())

		By("creating the Job in the cluster namespace, without a cross-namespace owner")
		job := jobOf(backup, w)
		Expect(job.Namespace).To(Equal(w.namespace))
		Expect(job.OwnerReferences).To(BeEmpty())

		By("completing across the namespace boundary")
		markJob(backup, w, batchv1.JobComplete)
		Eventually(func(g Gomega) {
			g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(backup), backup)).To(Succeed())
			g.Expect(backup.Status.PrimaryBackupID).NotTo(BeNil())
		}, timeout, interval).Should(Succeed())
		management.SetRuntimeState(
			*backup.Status.PrimaryBackupID, string(camundaadmin.StateCompleted), "",
		)
		Eventually(func(g Gomega) {
			g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(backup), backup)).To(Succeed())
			g.Expect(backup.Status.Phase).To(Equal(v1.LogicalBackupCompleted))
		}, timeout, interval).Should(Succeed())

		By("cleaning up the Job in the cluster namespace on deletion")
		Expect(k8sClient.Delete(ctx, backup)).To(Succeed())
		Eventually(func(g Gomega) {
			err := k8sClient.Get(ctx, client.ObjectKeyFromObject(backup), backup)
			g.Expect(err).To(HaveOccurred())
			g.Expect(client.IgnoreNotFound(err)).To(Succeed())

			var leftover batchv1.Job
			g.Expect(k8sClient.Get(
				ctx, types.NamespacedName{
					Namespace: w.namespace, Name: components.JobName(backup),
				}, &leftover,
			)).To(HaveOccurred())
		}, timeout, interval).Should(Succeed())
	})

	It("reads cross-namespace credentials through the cluster controller's copies", func() {
		w := createWorld()
		remote := newNamespace()

		By("moving both credential sources out of the cluster namespace")
		Eventually(func(g Gomega) {
			g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(w.bucket), w.bucket)).To(Succeed())
			w.bucket.Spec.S3.Auth.Credentials.SecretRef.Namespace = remote
			g.Expect(k8sClient.Update(ctx, w.bucket)).To(Succeed())
		}, timeout, interval).Should(Succeed())
		Eventually(func(g Gomega) {
			g.Expect(
				k8sClient.Get(ctx, client.ObjectKeyFromObject(w.dbConfig), w.dbConfig),
			).To(Succeed())
			w.dbConfig.Spec.BackupCredentialsSecretRef.Namespace = remote
			g.Expect(k8sClient.Update(ctx, w.dbConfig)).To(Succeed())
		}, timeout, interval).Should(Succeed())

		By("standing in for the CamundaCluster controller's local copies")
		bucketMirror := camundacluster.MirroredSecretName(
			w.cluster, camundacluster.MirrorPurposeBackupCredentials,
		)
		Expect(k8sClient.Create(ctx, &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{Name: bucketMirror, Namespace: w.namespace},
			Data: map[string][]byte{
				"accessKeyId": []byte("minio-root"), "secretAccessKey": []byte("minio-secret"),
			},
		})).To(Succeed())
		dumpMirror := camundacluster.MirroredSecretName(
			w.cluster, camundacluster.MirrorPurposeDumpCredentials,
		)
		Expect(k8sClient.Create(ctx, &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{Name: dumpMirror, Namespace: w.namespace},
			Data:       map[string][]byte{"username": []byte("backup"), "password": []byte("s3cr3t")},
		})).To(Succeed())

		backup := createBackup(w)
		job := jobOf(backup, w)

		By("wiring the Job to the copies, not the unreachable sources")
		dump := job.Spec.Template.Spec.InitContainers[0]
		Expect(secretNameOfEnv(dump, "PGUSER")).To(Equal(dumpMirror))
		upload := job.Spec.Template.Spec.Containers[0]
		Expect(secretNameOfEnv(upload, components.EnvUploadCredentialPrefix+"0")).To(
			Equal(bucketMirror),
		)
	})

	It("releases the finalizer when the pinned bucket is gone", func() {
		w := createWorld()
		backup := createBackup(w)

		Eventually(func(g Gomega) {
			g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(backup), backup)).To(Succeed())
			g.Expect(backup.Status.ObjectKey).NotTo(BeEmpty())
		}, timeout, interval).Should(Succeed())

		By("deleting the pinned ObjectStorageConfig, then the backup")
		Expect(k8sClient.Delete(ctx, w.bucket)).To(Succeed())
		Expect(k8sClient.Delete(ctx, backup)).To(Succeed())

		By("recording what was left behind and releasing anyway")
		Eventually(func(g Gomega) {
			err := k8sClient.Get(ctx, client.ObjectKeyFromObject(backup), backup)
			g.Expect(err).To(HaveOccurred())
			g.Expect(client.IgnoreNotFound(err)).To(Succeed())

			var events eventsv1.EventList
			g.Expect(k8sClient.List(ctx, &events, client.InNamespace(w.namespace))).To(Succeed())
			reasons := make([]string, 0, len(events.Items))
			for _, event := range events.Items {
				reasons = append(reasons, event.Reason)
			}
			g.Expect(reasons).To(ContainElement("ArtifactCleanupFailed"))
		}, timeout, interval).Should(Succeed())
	})
})

var _ = Describe("LogicalBackupRDBMS finalizer helper", func() {
	It("exports the shared finalizer", func() {
		Expect(logicalbackup.Finalizer).To(Equal("core.camunda.io/backup-artifacts"))
	})
})
