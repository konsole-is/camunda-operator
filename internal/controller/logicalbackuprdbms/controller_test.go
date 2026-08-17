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
// controller would have made.
func createWorld() *world {
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

	// The bucket credentials copy that the CamundaCluster controller mirrors.
	mirror := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name: camundacluster.MirroredSecretName(
				cluster, camundacluster.MirrorPurposeBackupCredentials,
			),
			Namespace: namespace,
		},
		Data: map[string][]byte{
			"accessKeyId": []byte("minio-root"), "secretAccessKey": []byte("minio-secret"),
		},
	}
	Expect(k8sClient.Create(ctx, mirror)).To(Succeed())

	return &world{
		namespace: namespace,
		cluster:   cluster,
		storage:   storage,
		dbConfig:  dbConfig,
		server:    server,
		bucket:    bucket,
	}
}

func createBackup(w *world) *v1.LogicalBackupRDBMS {
	backup := &v1.LogicalBackupRDBMS{
		ObjectMeta: metav1.ObjectMeta{
			Name: "backup-" + utilrand.String(6), Namespace: w.namespace,
		},
		Spec: v1.LogicalBackupRDBMSSpec{
			ClusterRef: v1.ClusterRef{Name: w.cluster.Name},
		},
	}
	Expect(k8sClient.Create(ctx, backup)).To(Succeed())

	return backup
}

func readyCondition(backup *v1.LogicalBackupRDBMS) *metav1.Condition {
	return meta.FindStatusCondition(backup.Status.Conditions, v1.ConditionReady)
}

func expectReason(backup *v1.LogicalBackupRDBMS, reason string) {
	GinkgoHelper()
	Eventually(func(g Gomega) {
		g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(backup), backup)).To(Succeed())
		condition := readyCondition(backup)
		g.Expect(condition).NotTo(BeNil())
		g.Expect(condition.Reason).To(Equal(reason))
	}, timeout, interval).Should(Succeed())
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
	job.Status.Conditions = append(job.Status.Conditions,
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

		By("rendering the Job under the cluster ServiceAccount with the recorded key")
		var job batchv1.Job
		key := types.NamespacedName{Namespace: w.namespace, Name: components.JobName(backup)}
		Eventually(func(g Gomega) {
			g.Expect(k8sClient.Get(ctx, key, &job)).To(Succeed())
		}, timeout, interval).Should(Succeed())
		Expect(job.Spec.Template.Spec.ServiceAccountName).To(Equal(w.cluster.Name + "-camunda"))
		Expect(job.OwnerReferences).NotTo(BeEmpty())

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

		expectReason(backup, v1.ReasonInvalidReference)
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
		expectReason(backup, v1.ReasonClusterSuspended)
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
		expectReason(backup, v1.ReasonStorageTypeMismatch)
	})

	It("serializes backups of one cluster", func() {
		w := createWorld()
		first := createBackup(w)

		Eventually(func(g Gomega) {
			g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(first), first)).To(Succeed())
			g.Expect(first.Status.Phase).To(Equal(v1.LogicalBackupRunning))
		}, timeout, interval).Should(Succeed())

		second := createBackup(w)
		expectReason(second, v1.ReasonBackupInProgress)
	})

	It("consults the sibling kind through the seam", func() {
		w := createWorld()
		reconciler.SiblingInProgress = func(
			context.Context, types.NamespacedName,
		) (string, error) {
			return "an-elasticsearch-backup", nil
		}
		DeferCleanup(func() { reconciler.SiblingInProgress = nil })

		backup := createBackup(w)
		expectReason(backup, v1.ReasonBackupInProgress)
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
		expectReason(backup, v1.ReasonMissingSecret)
	})

	It("requires the server version", func() {
		w := createWorld()
		Eventually(func(g Gomega) {
			g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(w.server), w.server)).To(Succeed())
			w.server.Spec.Version = ""
			g.Expect(k8sClient.Update(ctx, w.server)).To(Succeed())
		}, timeout, interval).Should(Succeed())

		backup := createBackup(w)
		expectReason(backup, v1.ReasonInvalidReference)
		Expect(readyCondition(backup).Message).To(ContainSubstring("version"))
	})

	It("reports MissingCredentials until the bucket credentials copy exists", func() {
		w := createWorld()
		mirror := camundacluster.MirroredSecretName(
			w.cluster, camundacluster.MirrorPurposeBackupCredentials,
		)
		Expect(k8sClient.Delete(ctx, &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{Name: mirror, Namespace: w.namespace},
		})).To(Succeed())

		backup := createBackup(w)
		expectReason(backup, v1.ReasonMissingCredentials)
	})

	It("waits in Pending until the binding is published", func() {
		w := createWorld()
		Eventually(func(g Gomega) {
			g.Expect(
				k8sClient.Get(ctx, client.ObjectKeyFromObject(w.cluster), w.cluster),
			).To(Succeed())
			w.cluster.Status.Management = nil
			g.Expect(k8sClient.Status().Update(ctx, w.cluster)).To(Succeed())
		}, timeout, interval).Should(Succeed())

		backup := createBackup(w)
		Eventually(func(g Gomega) {
			g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(backup), backup)).To(Succeed())
			g.Expect(backup.Status.BackupID).NotTo(BeZero())
			condition := readyCondition(backup)
			g.Expect(condition).NotTo(BeNil())
			g.Expect(condition.Reason).To(Equal(v1.ReasonProgressing))
		}, timeout, interval).Should(Succeed())

		// The dump can run without the binding; only the primary-storage
		// step needs it, so the backup holds there.
		markJob(backup, w, batchv1.JobComplete)
		Consistently(func(g Gomega) {
			g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(backup), backup)).To(Succeed())
			g.Expect(backup.Status.PrimaryBackupID).To(BeNil())
		}, "2s", interval).Should(Succeed())
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

		By("removing the credentials copy")
		var secret corev1.Secret
		err := k8sClient.Get(
			ctx, types.NamespacedName{
				Namespace: w.namespace, Name: dbSecretName(backup),
			}, &secret,
		)
		Expect(err).To(HaveOccurred())
	})

	It("honors the backup's own dump block over the cluster's", func() {
		w := createWorld()
		backup := &v1.LogicalBackupRDBMS{
			ObjectMeta: metav1.ObjectMeta{
				Name: "backup-" + utilrand.String(6), Namespace: w.namespace,
			},
			Spec: v1.LogicalBackupRDBMSSpec{
				ClusterRef: v1.ClusterRef{Name: w.cluster.Name},
				Dump: &v1.BackupDumpSpec{
					PodAnnotations: map[string]string{"sidecar.istio.io/inject": "false"},
				},
			},
		}
		Expect(k8sClient.Create(ctx, backup)).To(Succeed())

		var job batchv1.Job
		key := types.NamespacedName{Namespace: w.namespace, Name: components.JobName(backup)}
		Eventually(func(g Gomega) {
			g.Expect(k8sClient.Get(ctx, key, &job)).To(Succeed())
		}, timeout, interval).Should(Succeed())
		Expect(job.Spec.Template.Annotations).To(
			HaveKeyWithValue("sidecar.istio.io/inject", "false"),
		)
	})
})

var _ = Describe("LogicalBackupRDBMS finalizer helper", func() {
	It("exports the shared finalizer", func() {
		Expect(logicalbackup.Finalizer).To(Equal("core.camunda.io/backup-artifacts"))
	})
})
