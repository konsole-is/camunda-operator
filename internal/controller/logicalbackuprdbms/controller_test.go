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
// them directly; the spec for Secrets that live elsewhere stands in the copies.
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
			Engine: v1.DatabaseEnginePostgres,
			Host:   "postgres.databases.svc",
			Port:   5432,
			AdminCredentialsSecretRef: v1.CredentialsSecretRef{
				Name: "admin", Namespace: namespace,
				UsernameKey: "username", PasswordKey: "password",
			},
		},
	}
	Expect(k8sClient.Create(ctx, server)).To(Succeed())
	DeferCleanup(func() { _ = k8sClient.Delete(ctx, server) })

	// The probed version and the current Ready that the DatabaseServerConfig
	// controller would publish after reaching the server.
	probeServer(server, "17", metav1.ConditionTrue, v1.ReasonHealthy)

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
		Endpoint:   managementAPI.URL(),
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

// probeServer stands in for the DatabaseServerConfig controller: it publishes
// the probed version and a Ready condition observed at the server's current
// generation.
func probeServer(
	server *v1.DatabaseServerConfig, version string, status metav1.ConditionStatus, reason string,
) {
	GinkgoHelper()
	Eventually(func(g Gomega) {
		g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(server), server)).To(Succeed())
		now := metav1.Now()
		server.Status.ServerVersion = version
		server.Status.ProbedAt = &now
		meta.SetStatusCondition(&server.Status.Conditions, metav1.Condition{
			Type: v1.ConditionReady, Status: status, Reason: reason,
			Message: "stand-in probe", ObservedGeneration: server.Generation,
		})
		g.Expect(k8sClient.Status().Update(ctx, server)).To(Succeed())
	}, timeout, interval).Should(Succeed())
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
	It("dumps, requests the Zeebe backup, and completes", func() {
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

		By("moving to the Zeebe backup once the Job completes")
		markJob(backup, w, batchv1.JobComplete)
		Eventually(func(g Gomega) {
			g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(backup), backup)).To(Succeed())
			g.Expect(backup.Status.Step).To(Equal(v1.StepZeebeBackup))
			g.Expect(backup.Status.ZeebeBackupID).NotTo(BeNil())
		}, timeout, interval).Should(Succeed())

		By("recording the id the cluster generated and never re-requesting it")
		id := *backup.Status.ZeebeBackupID
		Consistently(func(g Gomega) {
			g.Expect(managementAPI.RuntimeStarts(id)).To(Equal(1))
		}, "2s", interval).Should(Succeed())

		By("completing once the cluster reports the backup done")
		managementAPI.SetRuntimeState(id, string(camundaadmin.StateCompleted), "")
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
			g.Expect(first.Status.ZeebeBackupID).NotTo(BeNil())
		}, timeout, interval).Should(Succeed())
		managementAPI.SetRuntimeState(
			*first.Status.ZeebeBackupID, string(camundaadmin.StateCompleted), "",
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

	It("waits until the server has been probed for its version", func() {
		w := createWorld()
		Eventually(func(g Gomega) {
			g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(w.server), w.server)).To(Succeed())
			w.server.Status.ServerVersion = ""
			w.server.Status.ProbedAt = nil
			g.Expect(k8sClient.Status().Update(ctx, w.server)).To(Succeed())
		}, timeout, interval).Should(Succeed())

		backup := createBackup(w)
		expectPending(backup, v1.ReasonInvalidReference)
		Expect(readyCondition(backup).Message).To(ContainSubstring("not been probed"))
	})

	// The DatabaseServerConfig controller keeps the last version while a
	// retargeted server is unreachable; a new backup must not start on it.
	It("waits when the server was retargeted and the probe of the new spec failed", func() {
		w := createWorld()
		Eventually(func(g Gomega) {
			g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(w.server), w.server)).To(Succeed())
			w.server.Spec.Host = "postgres-18.databases.svc"
			g.Expect(k8sClient.Update(ctx, w.server)).To(Succeed())
		}, timeout, interval).Should(Succeed())
		probeServer(w.server, "17", metav1.ConditionFalse, v1.ReasonConnectionFailed)

		backup := createBackup(w)
		expectPending(backup, v1.ReasonInvalidReference)
		Expect(readyCondition(backup).Message).To(ContainSubstring("current spec"))
	})

	// F3: the Job reserves its connection and upload variables.
	It("rejects a dump block that sets a reserved environment variable", func() {
		w := createWorld()
		backup := createBackup(w, func(backup *v1.LogicalBackupRDBMS) {
			backup.Spec.Dump = &v1.DumpPodSpec{ExtraEnv: []corev1.EnvVar{
				{Name: "PGSSLMODE", Value: "require"},
				{Name: "PGPASSWORD", Value: "hijack"},
			}}
		})

		expectPending(backup, v1.ReasonInvalidReference)
		Expect(readyCondition(backup).Message).To(ContainSubstring("PGPASSWORD"))
		Expect(readyCondition(backup).Message).NotTo(ContainSubstring("PGSSLMODE"))
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
		// gigabytes it cannot pair with a Zeebe backup afterwards.
		backup := createBackup(w)
		expectPending(backup, v1.ReasonProgressing)

		By("starting once the binding is published")
		Eventually(func(g Gomega) {
			g.Expect(
				k8sClient.Get(ctx, client.ObjectKeyFromObject(w.cluster), w.cluster),
			).To(Succeed())
			w.cluster.Status.Management = &v1.ManagementBinding{
				Endpoint:   managementAPI.URL(),
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

	It("deletes the Job and the dump object on deletion, never the Zeebe backups", func() {
		w := createWorld()
		backup := createBackup(w)

		markJob(backup, w, batchv1.JobComplete)
		Eventually(func(g Gomega) {
			g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(backup), backup)).To(Succeed())
			g.Expect(backup.Status.ZeebeBackupID).NotTo(BeNil())
		}, timeout, interval).Should(Succeed())
		id := *backup.Status.ZeebeBackupID
		managementAPI.SetRuntimeState(id, string(camundaadmin.StateCompleted), "")
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

		By("leaving the Zeebe backup alone")
		Expect(managementAPI.RuntimeBackup(id)).NotTo(BeNil())

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

	It("retries a conflicted Zeebe request instead of failing", func() {
		w := createWorld()
		managementAPI.ConflictNextRuntimeStart(1)
		backup := createBackup(w)

		// The dump already succeeded; one bad answer must not discard it. The
		// conflict is retried with backoff and the retry generates a fresh
		// id — the id that conflicted is never adopted.
		markJob(backup, w, batchv1.JobComplete)
		Eventually(func(g Gomega) {
			g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(backup), backup)).To(Succeed())
			g.Expect(backup.Status.ZeebeBackupID).NotTo(BeNil())
			g.Expect(backup.Status.Phase).To(Equal(v1.LogicalBackupRunning))
		}, timeout, interval).Should(Succeed())

		id := *backup.Status.ZeebeBackupID
		managementAPI.SetRuntimeState(id, string(camundaadmin.StateCompleted), "")
		Eventually(func(g Gomega) {
			g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(backup), backup)).To(Succeed())
			g.Expect(backup.Status.Phase).To(Equal(v1.LogicalBackupCompleted))
		}, timeout, interval).Should(Succeed())
	})

	It("applies the cluster's own dump block when the backup sets none", func() {
		w := createWorld(func(cluster *v1.CamundaCluster) {
			cluster.Spec.Backup = &v1.ClusterBackupSpec{
				Dump: &v1.BackupDumpSpec{
					DumpPodSpec: v1.DumpPodSpec{
						PodAnnotations: map[string]string{"linkerd.io/inject": "disabled"},
					},
					PostgresImage: "mirror.example/postgres:17.4",
				},
			}
		})
		backup := createBackup(w)

		job := jobOf(backup, w)
		Expect(job.Spec.Template.Annotations).To(
			HaveKeyWithValue("linkerd.io/inject", "disabled"),
		)
		Expect(job.Spec.Template.Spec.InitContainers[0].Image).To(Equal("mirror.example/postgres:17.4"))
	})

	// The image is cluster-level policy: a backup that replaces the pod
	// settings still runs the cluster's image.
	It("keeps the cluster's dump image when the backup overrides the pod settings", func() {
		w := createWorld(func(cluster *v1.CamundaCluster) {
			cluster.Spec.Backup = &v1.ClusterBackupSpec{
				Dump: &v1.BackupDumpSpec{PostgresImage: "mirror.example/postgres:17.4"},
			}
		})
		backup := createBackup(w, func(backup *v1.LogicalBackupRDBMS) {
			backup.Spec.Dump = &v1.DumpPodSpec{PodLabels: map[string]string{"from": "backup"}}
		})

		job := jobOf(backup, w)
		Expect(job.Spec.Template.Labels).To(HaveKeyWithValue("from", "backup"))
		Expect(job.Spec.Template.Spec.InitContainers[0].Image).To(Equal("mirror.example/postgres:17.4"))
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
			backup.Spec.Dump = &v1.DumpPodSpec{
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
			g.Expect(backup.Status.Step).To(Equal(v1.StepZeebeBackup))
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
		// missing cluster bites when the Zeebe step starts.
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
			Endpoint:   managementAPI.URL(),
			Auth:       v1.ManagementAuth{Method: v1.ManagementAuthMethodNone},
			Version:    "8.9.9",
			Partitions: 3,
		}
		Expect(k8sClient.Status().Update(ctx, revived)).To(Succeed())

		By("recovering: the failure clock clears and the backup completes")
		Eventually(func(g Gomega) {
			g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(backup), backup)).To(Succeed())
			g.Expect(backup.Status.ZeebeBackupID).NotTo(BeNil())
			g.Expect(backup.Status.FirstFailedAt).To(BeNil())
		}, timeout, interval).Should(Succeed())
		managementAPI.SetRuntimeState(
			*backup.Status.ZeebeBackupID, string(camundaadmin.StateCompleted), "",
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

	It("tolerates a Zeebe backup that has not registered yet", func() {
		w := createWorld()
		backup := createBackup(w)

		markJob(backup, w, batchv1.JobComplete)
		Eventually(func(g Gomega) {
			g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(backup), backup)).To(Succeed())
			g.Expect(backup.Status.ZeebeBackupID).NotTo(BeNil())
			g.Expect(backup.Status.ZeebeBackupRequestedAt).NotTo(BeNil())
		}, timeout, interval).Should(Succeed())
		id := *backup.Status.ZeebeBackupID

		// The partitions register their parts asynchronously after the 202,
		// so a backup the cluster does not report yet is normal at first.
		managementAPI.SetRuntimeState(id, string(camundaadmin.StateDoesNotExist), "")
		Consistently(func(g Gomega) {
			g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(backup), backup)).To(Succeed())
			g.Expect(backup.Status.Phase).To(Equal(v1.LogicalBackupRunning))
		}, "1500ms", interval).Should(Succeed())

		managementAPI.SetRuntimeState(id, string(camundaadmin.StateCompleted), "")
		Eventually(func(g Gomega) {
			g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(backup), backup)).To(Succeed())
			g.Expect(backup.Status.Phase).To(Equal(v1.LogicalBackupCompleted))
		}, timeout, interval).Should(Succeed())
	})

	It("fails a Zeebe backup still unregistered past the grace", func() {
		w := createWorld()
		backup := createBackup(w)

		markJob(backup, w, batchv1.JobComplete)
		Eventually(func(g Gomega) {
			g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(backup), backup)).To(Succeed())
			g.Expect(backup.Status.ZeebeBackupID).NotTo(BeNil())
		}, timeout, interval).Should(Succeed())

		managementAPI.SetRuntimeState(
			*backup.Status.ZeebeBackupID, string(camundaadmin.StateDoesNotExist), "",
		)
		Eventually(func(g Gomega) {
			g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(backup), backup)).To(Succeed())
			g.Expect(backup.Status.Phase).To(Equal(v1.LogicalBackupFailed))
			g.Expect(backup.Status.FailureMessage).To(ContainSubstring("DOES_NOT_EXIST"))
		}, timeout, interval).Should(Succeed())
	})

	It("reads credentials that live outside the cluster namespace through the cluster controller's copies", func() {
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

	// P1: a pod that cannot start never fails its Job and consumes no retry;
	// it must run through the bounded grace instead of holding the queue.
	It("fails a dump whose pod is stuck in a non-progressing waiting state", func() {
		w := createWorld()
		backup := createBackup(w)
		job := jobOf(backup, w)

		pod := &corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Name: job.Name + "-stuck", Namespace: w.namespace,
				Labels: map[string]string{"batch.kubernetes.io/job-name": job.Name},
			},
			Spec: corev1.PodSpec{Containers: []corev1.Container{{Name: "upload", Image: "cli"}}},
		}
		Expect(k8sClient.Create(ctx, pod)).To(Succeed())
		DeferCleanup(func() { _ = k8sClient.Delete(ctx, pod, client.GracePeriodSeconds(0)) })
		pod.Status.Phase = corev1.PodPending
		pod.Status.InitContainerStatuses = []corev1.ContainerStatus{{
			Name: "dump",
			State: corev1.ContainerState{Waiting: &corev1.ContainerStateWaiting{
				Reason: "CreateContainerConfigError", Message: `secret "backup-user" not found`,
			}},
		}}
		Expect(k8sClient.Status().Update(ctx, pod)).To(Succeed())

		Eventually(func(g Gomega) {
			g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(backup), backup)).To(Succeed())
			g.Expect(backup.Status.Phase).To(Equal(v1.LogicalBackupFailed))
			g.Expect(backup.Status.FailureMessage).To(ContainSubstring("CreateContainerConfigError"))
			g.Expect(backup.Status.FailureMessage).To(ContainSubstring(pod.Name))
		}, "15s", interval).Should(Succeed())
	})

	// P2: the Zeebe backup goes to the cluster's current backup store; a
	// retarget between the dump and the request breaks the restore point.
	It("fails when the cluster's backup store was retargeted after the dump", func() {
		w := createWorld()
		backup := createBackup(w)
		jobOf(backup, w)

		other := w.bucket.DeepCopy()
		other.ObjectMeta = metav1.ObjectMeta{Name: w.bucket.Name + "-other"}
		other.Spec.S3.BucketName = "the-other-bucket"
		Expect(k8sClient.Create(ctx, other)).To(Succeed())
		DeferCleanup(func() { _ = k8sClient.Delete(ctx, other) })
		Eventually(func(g Gomega) {
			g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(w.cluster), w.cluster)).To(Succeed())
			w.cluster.Spec.BackupStorageRef = other.Name
			g.Expect(k8sClient.Update(ctx, w.cluster)).To(Succeed())
		}, timeout, interval).Should(Succeed())

		markJob(backup, w, batchv1.JobComplete)
		Eventually(func(g Gomega) {
			g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(backup), backup)).To(Succeed())
			g.Expect(backup.Status.Phase).To(Equal(v1.LogicalBackupFailed))
			g.Expect(backup.Status.FailureMessage).To(ContainSubstring(other.Name))
			g.Expect(backup.Status.FailureMessage).To(ContainSubstring(w.bucket.Name))
			g.Expect(backup.Status.ZeebeBackupID).To(BeNil())
		}, "15s", interval).Should(Succeed())
	})

	// S2: the Job is released once the dump is recorded, so a PVC-backed
	// scratch volume does not live as long as the backup; the backup itself
	// is unaffected and deletion stays clean.
	It("releases the dump Job once the dump is recorded", func() {
		w := createWorld()
		backup := createBackup(w)
		job := jobOf(backup, w)

		markJob(backup, w, batchv1.JobComplete)
		Eventually(func(g Gomega) {
			g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(backup), backup)).To(Succeed())
			g.Expect(backup.Status.ZeebeBackupID).NotTo(BeNil())
			g.Expect(backup.Status.JobName).To(BeEmpty())
			g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(job), job)).NotTo(Succeed())
		}, timeout, interval).Should(Succeed())

		managementAPI.SetRuntimeState(
			*backup.Status.ZeebeBackupID, string(camundaadmin.StateCompleted), "",
		)
		Eventually(func(g Gomega) {
			g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(backup), backup)).To(Succeed())
			g.Expect(backup.Status.Phase).To(Equal(v1.LogicalBackupCompleted))
		}, timeout, interval).Should(Succeed())
		objectKey := backup.Status.ObjectKey

		Expect(k8sClient.Delete(ctx, backup)).To(Succeed())
		Eventually(func(g Gomega) {
			err := k8sClient.Get(ctx, client.ObjectKeyFromObject(backup), backup)
			g.Expect(err).To(HaveOccurred())
			g.Expect(client.IgnoreNotFound(err)).To(Succeed())
			g.Expect(bucket.Deleted()).To(ContainElement(objectKey))
		}, timeout, interval).Should(Succeed())
	})

	// F2: a management API that keeps rejecting the call is bounded like an
	// unreachable one; a Running backup never parks forever.
	It("fails when the management API keeps rejecting the Zeebe backup request", func() {
		w := createWorld()
		managementAPI.FailNext("runtimeStart", 1000)
		DeferCleanup(func() { managementAPI.FailNext("runtimeStart", 0) })

		backup := createBackup(w)
		markJob(backup, w, batchv1.JobComplete)

		Eventually(func(g Gomega) {
			g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(backup), backup)).To(Succeed())
			g.Expect(backup.Status.Phase).To(Equal(v1.LogicalBackupFailed))
			g.Expect(backup.Status.FailureMessage).To(ContainSubstring("rejected the call"))
			g.Expect(backup.Status.ZeebeBackupID).To(BeNil())
		}, "15s", interval).Should(Succeed())
	})

	// F5: a retargeted bucket must never make the finalizer delete a
	// stranger's object at the same key.
	It("leaves the object behind when the pinned bucket was retargeted", func() {
		w := createWorld()
		backup := createBackup(w)
		Eventually(func(g Gomega) {
			g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(backup), backup)).To(Succeed())
			g.Expect(backup.Status.BucketLocation).NotTo(BeEmpty())
		}, timeout, interval).Should(Succeed())
		objectKey := backup.Status.ObjectKey

		Eventually(func(g Gomega) {
			g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(w.bucket), w.bucket)).To(Succeed())
			w.bucket.Spec.S3.BucketName = "someone-elses-bucket"
			g.Expect(k8sClient.Update(ctx, w.bucket)).To(Succeed())
		}, timeout, interval).Should(Succeed())

		Expect(k8sClient.Delete(ctx, backup)).To(Succeed())
		Eventually(func(g Gomega) {
			err := k8sClient.Get(ctx, client.ObjectKeyFromObject(backup), backup)
			g.Expect(err).To(HaveOccurred())
			g.Expect(client.IgnoreNotFound(err)).To(Succeed())

			var events eventsv1.EventList
			g.Expect(k8sClient.List(ctx, &events, client.InNamespace(w.namespace))).To(Succeed())
			notes := make([]string, 0, len(events.Items))
			for _, event := range events.Items {
				notes = append(notes, event.Reason+": "+event.Note)
			}
			g.Expect(notes).To(ContainElement(And(
				HavePrefix("ArtifactCleanupFailed"), ContainSubstring("someone-elses-bucket"),
			)))
		}, timeout, interval).Should(Succeed())
		Expect(bucket.Deleted()).NotTo(ContainElement(objectKey))
	})

	// F6: the object is deleted only once the Job and its pods are gone, so
	// a terminating uploader cannot recreate it after the delete.
	It("waits for the Job's pods before deleting the object", func() {
		w := createWorld()
		backup := createBackup(w)
		job := jobOf(backup, w)
		objectKey := backup.Status.ObjectKey
		Eventually(func(g Gomega) {
			g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(backup), backup)).To(Succeed())
			g.Expect(backup.Status.ObjectKey).NotTo(BeEmpty())
			objectKey = backup.Status.ObjectKey
		}, timeout, interval).Should(Succeed())

		// A pod of the Job, still around: envtest runs no kubelet, so it
		// stays until it is deleted by hand.
		pod := &corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Name: job.Name + "-x1", Namespace: w.namespace,
				Labels: map[string]string{"batch.kubernetes.io/job-name": job.Name},
			},
			Spec: corev1.PodSpec{Containers: []corev1.Container{{Name: "upload", Image: "cli"}}},
		}
		Expect(k8sClient.Create(ctx, pod)).To(Succeed())

		Expect(k8sClient.Delete(ctx, backup)).To(Succeed())
		By("holding the object while the pod lives")
		Consistently(func(g Gomega) {
			g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(backup), backup)).To(Succeed())
			g.Expect(bucket.Deleted()).NotTo(ContainElement(objectKey))
		}, "3s", interval).Should(Succeed())

		By("deleting the object once the pod is gone")
		Expect(k8sClient.Delete(ctx, pod, client.GracePeriodSeconds(0))).To(Succeed())
		Eventually(func(g Gomega) {
			err := k8sClient.Get(ctx, client.ObjectKeyFromObject(backup), backup)
			g.Expect(err).To(HaveOccurred())
			g.Expect(client.IgnoreNotFound(err)).To(Succeed())
			g.Expect(bucket.Deleted()).To(ContainElement(objectKey))
		}, timeout, interval).Should(Succeed())
	})

	// S1: a Secret that exists but lost a key is repairable; the deletion
	// must hold, visibly, until it is repaired — only a missing Secret takes
	// the best-effort release path.
	It("holds the deletion while the bucket credentials are broken, and finishes once repaired", func() {
		w := createWorld()
		backup := createBackup(w)
		Eventually(func(g Gomega) {
			g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(backup), backup)).To(Succeed())
			g.Expect(backup.Status.ObjectKey).NotTo(BeEmpty())
		}, timeout, interval).Should(Succeed())
		objectKey := backup.Status.ObjectKey

		By("removing a key from the credentials Secret")
		var secret corev1.Secret
		key := types.NamespacedName{Namespace: w.namespace, Name: "minio-credentials"}
		Expect(k8sClient.Get(ctx, key, &secret)).To(Succeed())
		delete(secret.Data, "secretAccessKey")
		Expect(k8sClient.Update(ctx, &secret)).To(Succeed())

		Expect(k8sClient.Delete(ctx, backup)).To(Succeed())

		By("holding the finalizer and saying why")
		Eventually(func(g Gomega) {
			var events eventsv1.EventList
			g.Expect(k8sClient.List(ctx, &events, client.InNamespace(w.namespace))).To(Succeed())
			notes := make([]string, 0, len(events.Items))
			for _, event := range events.Items {
				notes = append(notes, event.Reason+": "+event.Note)
			}
			g.Expect(notes).To(ContainElement(And(
				HavePrefix("ArtifactCleanupFailed"), ContainSubstring("secretAccessKey"),
			)))
		}, timeout, interval).Should(Succeed())
		Consistently(func(g Gomega) {
			g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(backup), backup)).To(Succeed())
			g.Expect(bucket.Deleted()).NotTo(ContainElement(objectKey))
		}, "2s", interval).Should(Succeed())

		By("finishing the deletion once the key is back")
		Expect(k8sClient.Get(ctx, key, &secret)).To(Succeed())
		secret.Data["secretAccessKey"] = []byte("minio-secret")
		Expect(k8sClient.Update(ctx, &secret)).To(Succeed())
		Eventually(func(g Gomega) {
			err := k8sClient.Get(ctx, client.ObjectKeyFromObject(backup), backup)
			g.Expect(err).To(HaveOccurred())
			g.Expect(client.IgnoreNotFound(err)).To(Succeed())
			g.Expect(bucket.Deleted()).To(ContainElement(objectKey))
		}, "20s", interval).Should(Succeed())
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
