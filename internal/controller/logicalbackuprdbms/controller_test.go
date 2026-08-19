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
	"fmt"
	"strings"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	coordinationv1 "k8s.io/api/coordination/v1"
	corev1 "k8s.io/api/core/v1"
	eventsv1 "k8s.io/api/events/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	utilrand "k8s.io/apimachinery/pkg/util/rand"
	"sigs.k8s.io/controller-runtime/pkg/client"

	v1 "github.com/konsole-is/camunda-operator/api/v1"
	"github.com/konsole-is/camunda-operator/internal/fixtures"
	"github.com/konsole-is/camunda-operator/pkg/camundaadmin"
	"github.com/konsole-is/camunda-operator/pkg/camundaconfig"
	camundacluster "github.com/konsole-is/camunda-operator/pkg/components/camundacluster"
	components "github.com/konsole-is/camunda-operator/pkg/components/logicalbackuprdbms"
	"github.com/konsole-is/camunda-operator/pkg/conditions"
	"github.com/konsole-is/camunda-operator/pkg/logicalbackup"
)

// world is the resolved fixture set of one spec. It holds a relational cluster
// with a published binding, its storage chain, and every Secret that the
// backup needs. Both credential Secrets live in the cluster namespace, so the
// backup reads them directly. The spec for Secrets that live elsewhere stands
// in the copies.
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
// the fake management API, with the credential copies that the CamundaCluster
// controller makes. Mutators shape the cluster before it is created.
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
	// controller publishes after it reaches the server.
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

	// The volume status, the binding, the converged Ready, and the Zeebe
	// workload with its config hash that the CamundaCluster controller
	// publishes and renders.
	cluster.Status.Volumes = []v1.VolumeStatus{
		{Name: "data-cc-0", Capacity: resource.MustParse("15Gi")},
	}
	Expect(k8sClient.Status().Update(ctx, cluster)).To(Succeed())
	renderZeebe(cluster, "hash-1", worldRDBMSURL)
	converge(cluster)

	// The bucket credentials live in the cluster namespace, so the backup
	// uses the source Secret directly. No copy is involved.
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

// worldRDBMSURL is the relational storage URL that the Zeebe of the world
// runs. It is built from the DatabaseServerConfig host and port and the
// DatabaseConfig database name of createWorld, rendered as the cluster
// controller renders them.
var worldRDBMSURL = camundacluster.RDBMSURL("postgres.databases.svc", 5432, "camunda")

// renderZeebe stands in for the rendering of the CamundaCluster controller.
// It creates the Zeebe StatefulSet. Its pod template carries the config hash
// of the configuration that Zeebe runs and the relational storage URL that
// Zeebe runs against. A change of the hash stands in for a rollout to another
// configuration, for example a swapped database. The URL says which database
// that is.
func renderZeebe(cluster *v1.CamundaCluster, hash, rdbmsURL string) {
	GinkgoHelper()
	key := types.NamespacedName{
		Namespace: cluster.Namespace,
		Name:      camundacluster.WorkloadName(cluster, camundacluster.ComponentZeebe),
	}
	container := corev1.Container{
		Name: "zeebe", Image: "z",
		Env: []corev1.EnvVar{camundaconfig.Var(camundaconfig.KeyRDBMSURL, rdbmsURL)},
	}
	Eventually(func(g Gomega) {
		var workload appsv1.StatefulSet
		err := k8sClient.Get(ctx, key, &workload)
		if apierrors.IsNotFound(err) {
			workload = appsv1.StatefulSet{
				ObjectMeta: metav1.ObjectMeta{Name: key.Name, Namespace: key.Namespace},
				Spec: appsv1.StatefulSetSpec{
					Selector: &metav1.LabelSelector{MatchLabels: map[string]string{"app": key.Name}},
					Template: corev1.PodTemplateSpec{
						ObjectMeta: metav1.ObjectMeta{
							Labels:      map[string]string{"app": key.Name},
							Annotations: map[string]string{camundacluster.ConfigHashAnnotation: hash},
						},
						Spec: corev1.PodSpec{Containers: []corev1.Container{container}},
					},
				},
			}
			g.Expect(k8sClient.Create(ctx, &workload)).To(Succeed())

			return
		}
		g.Expect(err).NotTo(HaveOccurred())
		workload.Spec.Template.Annotations[camundacluster.ConfigHashAnnotation] = hash
		workload.Spec.Template.Spec.Containers = []corev1.Container{container}
		g.Expect(k8sClient.Update(ctx, &workload)).To(Succeed())
	}, timeout, interval).Should(Succeed())
}

// converge stands in for the CamundaCluster controller. It publishes the
// management binding and reports the cluster Ready for its current
// generation, with status.observedGeneration caught up. That is the state
// that a backup is admitted against.
func converge(cluster *v1.CamundaCluster) {
	GinkgoHelper()
	Eventually(func(g Gomega) {
		g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(cluster), cluster)).To(Succeed())
		cluster.Status.Management = &v1.ManagementBinding{
			Endpoint:   managementAPI.URL(),
			Auth:       v1.ManagementAuth{Method: v1.ManagementAuthMethodNone},
			Version:    "8.9.9",
			Partitions: 3,
		}
		cluster.Status.ObservedGeneration = cluster.Generation
		meta.SetStatusCondition(&cluster.Status.Conditions, metav1.Condition{
			Type: v1.ConditionReady, Status: metav1.ConditionTrue, Reason: v1.ReasonHealthy,
			Message: "stand-in", ObservedGeneration: cluster.Generation,
		})
		g.Expect(k8sClient.Status().Update(ctx, cluster)).To(Succeed())
	}, timeout, interval).Should(Succeed())
}

// probeServer stands in for the DatabaseServerConfig controller. It publishes
// the probed version and a Ready condition observed at the current generation
// of the server.
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

// patchStatusUntilStable stands in for another writer of the status of the
// backup and makes the patch stick. The controller flushes the whole status
// that it staged from its last live read. On a write conflict, ocf applies
// that staged status again over a concurrent writer. This is by design, so a
// controller never loses its own state. As a result, a patch that lands while
// a reconcile is in flight is overwritten. The helper patches, waits one poll
// interval, and requires that the patch survived. Otherwise it patches again.
func patchStatusUntilStable(
	backup *v1.LogicalBackupRDBMS,
	mutate func(*v1.LogicalBackupRDBMS),
	stuck func(*v1.LogicalBackupRDBMS) bool,
) {
	GinkgoHelper()
	Eventually(func(g Gomega) {
		g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(backup), backup)).To(Succeed())
		if !stuck(backup) {
			mutate(backup)
			g.Expect(k8sClient.Status().Update(ctx, backup)).To(Succeed())
		}
		time.Sleep(2 * time.Second) // longer than the controller's poll interval
		g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(backup), backup)).To(Succeed())
		g.Expect(stuck(backup)).To(BeTrue(), "the controller re-applied its own status over the patch")
	}, "40s", interval).Should(Succeed())
}

// unconvergeCluster bumps the generation of the cluster and publishes no new
// Ready for it, so admission parks every backup of the cluster.
func unconvergeCluster(w *world) {
	GinkgoHelper()
	Eventually(func(g Gomega) {
		g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(w.cluster), w.cluster)).To(Succeed())
		if w.cluster.Spec.Backup == nil {
			w.cluster.Spec.Backup = &v1.ClusterBackupSpec{}
		}
		w.cluster.Spec.Backup.Dump = &v1.BackupDumpSpec{
			DumpPodSpec: v1.DumpPodSpec{PodLabels: map[string]string{"rollout": utilrand.String(4)}},
		}
		g.Expect(k8sClient.Update(ctx, w.cluster)).To(Succeed())
	}, timeout, interval).Should(Succeed())
}

// stageAdmitted hand-stages the state that a backup is in right after
// admission flushed its identity and before its Job exists. That is the gap
// that a crash or a slow requeue leaves. The state is pinned to the bucket as
// it is now.
func stageAdmitted(w *world, backup *v1.LogicalBackupRDBMS) {
	GinkgoHelper()
	Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(w.bucket), w.bucket)).To(Succeed())
	id := time.Now().UnixMilli()
	patchStatusUntilStable(
		backup, func(b *v1.LogicalBackupRDBMS) {
			b.Status.BackupID = id
			b.Status.Phase = v1.LogicalBackupRunning
			b.Status.Step = v1.StepDumping
			b.Status.BucketRef = w.bucket.Name
			b.Status.BucketGeneration = w.bucket.Generation
			b.Status.BucketLocation = w.bucket.Location()
			b.Status.WorkloadConfigHash = "hash-1"
			b.Status.ClusterUID = w.cluster.UID
			b.Status.ObjectKey = components.DumpObjectKey(
				w.bucket.BasePath(), w.namespace, w.cluster.Name, id, b.UID,
			)
		}, func(b *v1.LogicalBackupRDBMS) bool { return b.Status.BackupID == id },
	)
}

// makeBucketWIF flips the bucket of the world to workload identity. Then the
// manager holds no credentials for it, and cleanup must run where the
// identity lives. The cluster binds the identity on its ServiceAccount, so
// the account it publishes is the derived one.
func makeBucketWIF(w *world) {
	GinkgoHelper()
	Eventually(func(g Gomega) {
		g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(w.bucket), w.bucket)).To(Succeed())
		w.bucket.Spec.S3.Auth = v1.S3StorageAuth{
			Type:             v1.ObjectStorageAuthTypeWorkloadIdentity,
			WorkloadIdentity: &v1.S3WorkloadIdentity{RoleARN: "arn:aws:iam::1:role/backup"},
		}
		g.Expect(k8sClient.Update(ctx, w.bucket)).To(Succeed())
	}, timeout, interval).Should(Succeed())

	publishServiceAccount(w.cluster, w.cluster.Name+"-camunda")
}

// publishServiceAccount stands in for the CamundaCluster controller, which
// publishes the account its pods run under. An empty name is a cluster whose
// pods run under the default account of the namespace, which is what
// createWorld leaves behind.
func publishServiceAccount(cluster *v1.CamundaCluster, name string) {
	GinkgoHelper()
	Eventually(func(g Gomega) {
		g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(cluster), cluster)).To(Succeed())
		cluster.Status.ServiceAccountName = name
		g.Expect(k8sClient.Status().Update(ctx, cluster)).To(Succeed())
	}, timeout, interval).Should(Succeed())
}

// gateState describes why a waiting backup did not start yet. It holds the
// own phase and Ready of the backup and the state of the sibling that it can
// wait on. It also names who holds the Lease of the cluster. It annotates the
// assertions of the serialization specs, so a failure explains itself.

// leaseHolder returns the exact identity that the claim Lease records in its
// holder annotations. For a Lease that another actor wrote, it returns the
// raw holderIdentity. When the Lease is absent, it returns "".
func leaseHolder(lease *coordinationv1.Lease) string {
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

// staleLease writes the claim Lease of the cluster in the shape that a backup
// of this kind leaves behind when it no longer exists. That shape is a bounded
// holderIdentity plus the exact identity in the holder annotations.
func staleLease(w *world, holder logicalbackup.Claimant) *coordinationv1.Lease {
	identity := holder.HolderIdentity()

	return &coordinationv1.Lease{
		ObjectMeta: metav1.ObjectMeta{
			Name: logicalbackup.ClaimLeaseName(w.cluster.Name), Namespace: w.namespace,
			Annotations: map[string]string{
				logicalbackup.ClaimHolderKindAnnotation: holder.Kind,
				logicalbackup.ClaimHolderNameAnnotation: holder.Name,
				logicalbackup.ClaimHolderUIDAnnotation:  string(holder.UID),
			},
		},
		Spec: coordinationv1.LeaseSpec{HolderIdentity: &identity},
	}
}

func gateState(w *world, sibling, waiting *v1.LogicalBackupRDBMS) string {
	var latestSibling, latestWaiting v1.LogicalBackupRDBMS
	_ = k8sClient.Get(ctx, client.ObjectKeyFromObject(sibling), &latestSibling)
	_ = k8sClient.Get(ctx, client.ObjectKeyFromObject(waiting), &latestWaiting)
	describe := func(b *v1.LogicalBackupRDBMS) string {
		reason, message := "-", "-"
		if cond := readyCondition(b); cond != nil {
			reason, message = cond.Reason, cond.Message
		}

		return fmt.Sprintf(
			"%s: phase=%s backupId=%d Ready=%s (%s)",
			b.Name,
			b.Status.Phase,
			b.Status.BackupID,
			reason,
			message,
		)
	}
	holder := "<no Lease>"
	var lease coordinationv1.Lease
	if err := k8sClient.Get(
		ctx, types.NamespacedName{
			Namespace: w.namespace, Name: logicalbackup.ClaimLeaseName(w.cluster.Name),
		}, &lease,
	); err == nil {
		holder = leaseHolder(&lease)
	}

	return fmt.Sprintf(
		"waiting %s | sibling %s | Lease holder %s", describe(&latestWaiting), describe(&latestSibling), holder,
	)
}

// podOfBackup builds a pod the way the Job controller creates it from the
// template of the dump Job. The pod is labeled with the Job name and the
// backup UID.
func podOfBackup(backup *v1.LogicalBackupRDBMS, job *batchv1.Job, suffix string) *corev1.Pod {
	return &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name: job.Name + "-" + suffix, Namespace: backup.Namespace,
			Labels: map[string]string{
				"batch.kubernetes.io/job-name": job.Name,
				components.BackupUIDLabel:      string(backup.UID),
			},
		},
		Spec: corev1.PodSpec{Containers: []corev1.Container{{Name: "upload", Image: "cli"}}},
	}
}

// envValueOf returns the literal value of a container's env variable, or "".
func envValueOf(container corev1.Container, name string) string {
	for _, env := range container.Env {
		if env.Name == name {
			return env.Value
		}
	}

	return ""
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

// markJob flips the Job of backup to the given terminal condition. It stands
// in for a kubelet that ran the pod.
func markJob(backup *v1.LogicalBackupRDBMS, w *world, kind batchv1.JobConditionType) {
	GinkgoHelper()
	markJobNamed(components.JobName(backup), w, kind)
}

// markJobNamed is markJob for any Job name, for example the cleanup Job.
func markJobNamed(name string, w *world, kind batchv1.JobConditionType) {
	GinkgoHelper()
	var job batchv1.Job
	key := types.NamespacedName{Namespace: w.namespace, Name: name}
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

		By("keying the dump on the id and on the UID, so a reused id never names another backup's dump")
		Expect(backup.Status.ObjectKey).To(Equal(components.DumpObjectKey(
			w.bucket.BasePath(), w.namespace, w.cluster.Name, backup.Status.BackupID, backup.UID,
		)))
		Expect(backup.Status.ObjectKey).To(HaveSuffix("/" + string(backup.UID) + "/camunda.dump"))

		// The bucket of this cluster carries static credentials and the spec
		// names no account, so the cluster renders none and its own pods run
		// under the default account of the namespace. The Job runs there too.
		// Naming the derived account here would be rejected by the API
		// server, which refuses a pod whose ServiceAccount does not exist.
		By("rendering the Job under no ServiceAccount, as the cluster workloads run")
		job := jobOf(backup, w)
		Expect(job.Spec.Template.Spec.ServiceAccountName).To(BeEmpty())
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
		// The in-kind pre-filter names the started sibling. The same backup
		// holds the Lease behind it.
		Expect(readyCondition(second).Message).To(ContainSubstring(first.Name))
		leaseKey := types.NamespacedName{
			Namespace: w.namespace, Name: logicalbackup.ClaimLeaseName(w.cluster.Name),
		}
		var lease coordinationv1.Lease
		Expect(k8sClient.Get(ctx, leaseKey, &lease)).To(Succeed())
		Expect(leaseHolder(&lease)).To(Equal(claimant(first).String()))

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

		By("letting the waiting backup claim and start, so a done backup never deadlocks the queue")
		Eventually(func(g Gomega) {
			g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(second), second)).To(Succeed())
			g.Expect(second.Status.BackupID).NotTo(BeZero())
			g.Expect(second.Status.Phase).To(Equal(v1.LogicalBackupRunning))
			g.Expect(k8sClient.Get(ctx, leaseKey, &lease)).To(Succeed())
			g.Expect(leaseHolder(&lease)).To(Equal(claimant(second).String()))
		}, timeout, interval).Should(Succeed())
	})

	It("releases the Lease once the holder is terminal", func() {
		w := createWorld()
		backup := createBackup(w)
		leaseKey := types.NamespacedName{
			Namespace: w.namespace, Name: logicalbackup.ClaimLeaseName(w.cluster.Name),
		}
		Eventually(func(g Gomega) {
			g.Expect(k8sClient.Get(ctx, leaseKey, &coordinationv1.Lease{})).To(Succeed())
		}, timeout, interval).Should(Succeed())

		markJob(backup, w, batchv1.JobFailed)
		Eventually(func(g Gomega) {
			g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(backup), backup)).To(Succeed())
			g.Expect(backup.Status.Phase).To(Equal(v1.LogicalBackupFailed))
			g.Expect(k8sClient.Get(ctx, leaseKey, &coordinationv1.Lease{})).NotTo(Succeed())
		}, timeout, interval).Should(Succeed())
	})

	// The claim on a cluster is a Lease. A foreign holder blocks. A foreign
	// holder is anything that this operator did not write, or the other
	// backup kind. A foreign identity is never taken over.
	It("waits behind a foreign Lease and never takes it over", func() {
		w := createWorld()
		holder := "someone-else"
		lease := &coordinationv1.Lease{
			ObjectMeta: metav1.ObjectMeta{
				Name: logicalbackup.ClaimLeaseName(w.cluster.Name), Namespace: w.namespace,
			},
			Spec: coordinationv1.LeaseSpec{HolderIdentity: &holder},
		}
		Expect(k8sClient.Create(ctx, lease)).To(Succeed())

		backup := createBackup(w)
		expectPending(backup, v1.ReasonBackupInProgress)
		Expect(readyCondition(backup).Message).To(ContainSubstring("someone-else"))
		Consistently(func(g Gomega) {
			var current coordinationv1.Lease
			g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(lease), &current)).To(Succeed())
			g.Expect(*current.Spec.HolderIdentity).To(Equal("someone-else"))
		}, "2s", interval).Should(Succeed())
	})

	// A holder that is gone no longer needs the claim. The claim is taken
	// over, so a crash between the claim and the release never blocks the
	// cluster.
	It("takes over the Lease of a holder that no longer exists", func() {
		w := createWorld()
		lease := staleLease(w, logicalbackup.Claimant{
			Kind: "LogicalBackupRDBMS", Name: "ghost", UID: "00000000-0000-0000-0000-000000000000",
		})
		Expect(k8sClient.Create(ctx, lease)).To(Succeed())

		backup := createBackup(w)
		Eventually(func(g Gomega) {
			g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(backup), backup)).To(Succeed())
			g.Expect(backup.Status.Phase).To(Equal(v1.LogicalBackupRunning))
			var current coordinationv1.Lease
			g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(lease), &current)).To(Succeed())
			g.Expect(leaseHolder(&current)).To(Equal(claimant(backup).String()))
		}, timeout, interval).Should(Succeed())
	})

	It("releases the Lease when a running holder is deleted", func() {
		w := createWorld()
		backup := createBackup(w)
		leaseKey := types.NamespacedName{
			Namespace: w.namespace, Name: logicalbackup.ClaimLeaseName(w.cluster.Name),
		}
		Eventually(func(g Gomega) {
			var current coordinationv1.Lease
			g.Expect(k8sClient.Get(ctx, leaseKey, &current)).To(Succeed())
			g.Expect(leaseHolder(&current)).To(Equal(claimant(backup).String()))
		}, timeout, interval).Should(Succeed())

		Expect(k8sClient.Delete(ctx, backup)).To(Succeed())
		Eventually(func(g Gomega) {
			g.Expect(k8sClient.Get(ctx, leaseKey, &coordinationv1.Lease{})).NotTo(Succeed())
		}, timeout, interval).Should(Succeed())
	})

	// The claim comes before the last checks of admission and before the id
	// flush. A park after the claim, or a re-entry after a failed flush that
	// parks, must not keep the Lease. A held Lease with no id blocks every
	// sibling for as long as the park lasts.
	It("releases the Lease when the backup parks after the claim", func() {
		w := createWorld()
		renderZeebe(w.cluster, "", worldRDBMSURL)
		backup := createBackup(w)
		leaseKey := types.NamespacedName{
			Namespace: w.namespace, Name: logicalbackup.ClaimLeaseName(w.cluster.Name),
		}

		expectPending(backup, v1.ReasonProgressing)
		Expect(readyCondition(backup).Message).To(ContainSubstring("no config hash yet"))
		Eventually(func(g Gomega) {
			g.Expect(k8sClient.Get(ctx, leaseKey, &coordinationv1.Lease{})).NotTo(Succeed())
		}, timeout, interval).Should(Succeed())

		By("claiming and starting once the workload carries its hash")
		renderZeebe(w.cluster, "hash-1", worldRDBMSURL)
		Eventually(func(g Gomega) {
			g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(backup), backup)).To(Succeed())
			g.Expect(backup.Status.Phase).To(Equal(v1.LogicalBackupRunning))
			var current coordinationv1.Lease
			g.Expect(k8sClient.Get(ctx, leaseKey, &current)).To(Succeed())
			g.Expect(leaseHolder(&current)).To(Equal(claimant(backup).String()))
		}, timeout, interval).Should(Succeed())
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

	// The id is allocated after the highest one that a visible sibling holds.
	// So a clock that stepped backwards cannot reuse an id and overwrite its
	// dump.
	It("allocates the backup id after the highest sibling id", func() {
		w := createWorld()
		first := createBackup(w)
		Eventually(func(g Gomega) {
			g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(first), first)).To(Succeed())
			g.Expect(first.Status.BackupID).NotTo(BeZero())
		}, timeout, interval).Should(Succeed())

		By("letting the first backup finish through the controller, so it is quiescent")
		markJob(first, w, batchv1.JobFailed)
		leaseKey := types.NamespacedName{
			Namespace: w.namespace, Name: logicalbackup.ClaimLeaseName(w.cluster.Name),
		}
		Eventually(func(g Gomega) {
			g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(first), first)).To(Succeed())
			g.Expect(first.Status.Phase).To(Equal(v1.LogicalBackupFailed))
			g.Expect(k8sClient.Get(ctx, leaseKey, &coordinationv1.Lease{})).NotTo(Succeed())
		}, timeout, interval).Should(Succeed())

		By("standing in for a sibling that holds an id a year ahead")
		ahead := time.Now().Add(365 * 24 * time.Hour).UnixMilli()
		patchStatusUntilStable(
			first,
			func(b *v1.LogicalBackupRDBMS) { b.Status.BackupID = ahead },
			func(b *v1.LogicalBackupRDBMS) bool { return b.Status.BackupID == ahead },
		)

		second := createBackup(w)
		Eventually(func(g Gomega) {
			g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(second), second)).To(Succeed())
			g.Expect(second.Status.BackupID).To(Equal(ahead+1), gateState(w, first, second))
			g.Expect(second.Status.ObjectKey).To(ContainSubstring(fmt.Sprintf("/%d/", ahead+1)))
		}, timeout, interval).Should(Succeed())
	})

	// A message that the management API reports is bounded before it reaches
	// the free-form status field, so an oversized one cannot make the status
	// unwritable.
	It("bounds an oversized management failure reason in status", func() {
		w := createWorld()
		backup := createBackup(w)
		markJob(backup, w, batchv1.JobComplete)
		Eventually(func(g Gomega) {
			g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(backup), backup)).To(Succeed())
			g.Expect(backup.Status.ZeebeBackupID).NotTo(BeNil())
		}, timeout, interval).Should(Succeed())

		huge := strings.Repeat("x", 4*conditions.MaxMessageLength)
		managementAPI.SetRuntimeState(*backup.Status.ZeebeBackupID, string(camundaadmin.StateFailed), huge)

		Eventually(func(g Gomega) {
			g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(backup), backup)).To(Succeed())
			g.Expect(backup.Status.Phase).To(Equal(v1.LogicalBackupFailed))
			g.Expect(len(backup.Status.FailureMessage)).To(BeNumerically("<", conditions.MaxMessageLength+64))
			g.Expect(backup.Status.FailureMessage).To(ContainSubstring("(truncated"))
			condition := readyCondition(backup)
			g.Expect(condition).NotTo(BeNil())
			g.Expect(condition.Reason).To(Equal(v1.ReasonFailed))
		}, timeout, interval).Should(Succeed())
	})

	// A backup can be admitted against a desired spec that Zeebe does not
	// run yet. Then it can pair a dump with a Zeebe backup of the previous
	// configuration.
	It("waits in Pending until the cluster has converged on its current spec, then starts", func() {
		w := createWorld()
		Eventually(func(g Gomega) {
			g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(w.cluster), w.cluster)).To(Succeed())
			if w.cluster.Spec.Backup == nil {
				w.cluster.Spec.Backup = &v1.ClusterBackupSpec{}
			}
			w.cluster.Spec.Backup.Dump = &v1.BackupDumpSpec{
				DumpPodSpec: v1.DumpPodSpec{PodLabels: map[string]string{"rollout": "pending"}},
			}
			g.Expect(k8sClient.Update(ctx, w.cluster)).To(Succeed())
		}, timeout, interval).Should(Succeed())

		backup := createBackup(w)
		expectPending(backup, v1.ReasonProgressing)
		Expect(readyCondition(backup).Message).To(ContainSubstring("has not converged"))

		By("starting once the operator reports the new generation Ready")
		converge(w.cluster)
		Eventually(func(g Gomega) {
			g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(backup), backup)).To(Succeed())
			g.Expect(backup.Status.Phase).To(Equal(v1.LogicalBackupRunning))
		}, timeout, interval).Should(Succeed())
	})

	// At the Zeebe step: after a spec change between the dump and the
	// request, Zeebe can still roll. The request waits, then fails.
	It("fails when the cluster changed generation between the dump and the Zeebe backup", func() {
		w := createWorld()
		backup := createBackup(w)
		jobOf(backup, w)

		Eventually(func(g Gomega) {
			g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(w.cluster), w.cluster)).To(Succeed())
			if w.cluster.Spec.Backup == nil {
				w.cluster.Spec.Backup = &v1.ClusterBackupSpec{}
			}
			w.cluster.Spec.Backup.Dump = &v1.BackupDumpSpec{
				DumpPodSpec: v1.DumpPodSpec{PodLabels: map[string]string{"rollout": "started"}},
			}
			g.Expect(k8sClient.Update(ctx, w.cluster)).To(Succeed())
		}, timeout, interval).Should(Succeed())

		markJob(backup, w, batchv1.JobComplete)
		Eventually(func(g Gomega) {
			g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(backup), backup)).To(Succeed())
			g.Expect(backup.Status.Phase).To(Equal(v1.LogicalBackupFailed))
			g.Expect(backup.Status.FailureMessage).To(ContainSubstring("has not converged"))
			g.Expect(backup.Status.ZeebeBackupID).To(BeNil())
		}, "20s", interval).Should(Succeed())
	})

	// The operator applies the deadline default, not the schema. So a
	// cluster block that sets only the image still inherits the deadline of a
	// preset. The unit tests bypass API defaulting. This spec goes through it.
	It("keeps a preset's dump deadline when the cluster block sets only the image", func() {
		preset := &v1.CamundaClusterPreset{
			ObjectMeta: metav1.ObjectMeta{Name: "preset-" + utilrand.String(6)},
			Spec: v1.CamundaClusterPresetSpec{Cluster: v1.CamundaClusterSpec{
				Backup: &v1.ClusterBackupSpec{Dump: &v1.BackupDumpSpec{
					DumpPodSpec: v1.DumpPodSpec{ActiveDeadlineSeconds: new(int64(3600))},
				}},
			}},
		}
		Expect(k8sClient.Create(ctx, preset)).To(Succeed())
		DeferCleanup(func() { _ = k8sClient.Delete(ctx, preset) })

		w := createWorld(func(cluster *v1.CamundaCluster) {
			cluster.Spec.PresetRef = preset.Name
			cluster.Spec.Backup = &v1.ClusterBackupSpec{
				Dump: &v1.BackupDumpSpec{PostgresImage: "mirror.example/postgres:17.4"},
			}
		})
		backup := createBackup(w)

		job := jobOf(backup, w)
		Expect(*job.Spec.ActiveDeadlineSeconds).To(Equal(int64(3600)))
		Expect(job.Spec.Template.Spec.InitContainers[0].Image).To(Equal("mirror.example/postgres:17.4"))
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
	// retargeted server is unreachable. A new backup must not start on it.
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

	// The dump block of a backup must not supply anything under
	// the PG or UPLOAD_ prefixes. The rule is prefix-based, so names that the
	// Job never sets (PGOPTIONS, PGHOSTADDR) are covered too. PGSSLMODE stays
	// available in the own block of the cluster, where its owner sets policy.
	It("rejects a backup dump block that sets a reserved environment variable", func() {
		w := createWorld()
		backup := createBackup(w, func(backup *v1.LogicalBackupRDBMS) {
			backup.Spec.Dump = &v1.DumpPodSpec{ExtraEnv: []corev1.EnvVar{
				{Name: "TZ", Value: "UTC"},
				{Name: "PGOPTIONS", Value: "-c synchronous_commit=off"},
			}}
		})

		expectPending(backup, v1.ReasonInvalidReference)
		Expect(readyCondition(backup).Message).To(ContainSubstring("PGOPTIONS"))
		Expect(readyCondition(backup).Message).NotTo(ContainSubstring("TZ"))
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
		// gigabytes that it cannot pair with a Zeebe backup afterwards.
		backup := createBackup(w)
		expectPending(backup, v1.ReasonProgressing)

		By("starting once the binding is published")
		converge(w.cluster)
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

		By("never creating a cleanup Job: the manager holds the credentials")
		Expect(k8sClient.Get(
			ctx, types.NamespacedName{
				Namespace: w.namespace, Name: components.CleanupJobName(backup),
			}, &batchv1.Job{},
		)).NotTo(Succeed())

		// envtest runs no garbage collector. If a Job is still present here,
		// the finalizer relied on the owner reference instead of a delete.
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

		// The dump already succeeded. One bad answer must not discard it. The
		// conflict is retried with backoff, and the retry generates a fresh
		// id. The id that conflicted is never adopted.
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

	// The image is cluster-level policy. A backup that replaces the pod
	// settings still runs the image of the cluster.
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

	// The Job runs under the account the cluster publishes, whatever bound
	// it. This controller never rebuilds that rule from the spec of the
	// cluster and the buckets it references.
	It("runs the Job under the account the cluster publishes", func() {
		w := createWorld(func(cluster *v1.CamundaCluster) {
			cluster.Spec.ServiceAccount = &v1.ServiceAccountSpec{Name: "platform-sa"}
		})
		publishServiceAccount(w.cluster, "platform-sa")
		backup := createBackup(w)

		job := jobOf(backup, w)
		Expect(job.Spec.Template.Spec.ServiceAccountName).To(Equal("platform-sa"))
	})

	// A document bucket binds an identity for the pods of the cluster, and
	// the dump never touches that bucket. The Job still runs under the
	// published account, because the cluster renders it. The narrower rule
	// that this controller used to rebuild is gone.
	It("runs the Job under the account that only the document bucket binds", func() {
		documents := &v1.ObjectStorageConfig{
			ObjectMeta: metav1.ObjectMeta{Name: "docs-" + utilrand.String(6)},
			Spec: v1.ObjectStorageConfigSpec{
				Type: v1.ObjectStorageTypeS3,
				S3: &v1.S3Storage{
					BucketName: "documents",
					Region:     "eu-west-1",
					Auth:       v1.S3StorageAuth{Type: v1.ObjectStorageAuthTypeWorkloadIdentity},
				},
			},
		}
		Expect(k8sClient.Create(ctx, documents)).To(Succeed())
		DeferCleanup(func() { _ = k8sClient.Delete(ctx, documents) })

		w := createWorld(func(cluster *v1.CamundaCluster) {
			cluster.Spec.DocumentStorageRef = documents.Name
		})
		publishServiceAccount(w.cluster, w.cluster.Name+"-camunda")
		backup := createBackup(w)

		job := jobOf(backup, w)
		Expect(job.Spec.Template.Spec.ServiceAccountName).To(Equal(w.cluster.Name + "-camunda"))
	})

	// The environment of the backup's own block reaches the dump container
	// only. The upload container runs cloud SDKs that read endpoint and
	// proxy variables, and a backup author must not steer the upload.
	It("honors the backup's own dump block over the cluster's, outside the upload container", func() {
		w := createWorld()
		backup := createBackup(w, func(backup *v1.LogicalBackupRDBMS) {
			backup.Spec.Dump = &v1.DumpPodSpec{
				PodAnnotations: map[string]string{"sidecar.istio.io/inject": "false"},
				ExtraEnv:       []corev1.EnvVar{{Name: "AWS_ENDPOINT_URL", Value: "http://evil.example"}},
			}
		})

		job := jobOf(backup, w)
		Expect(job.Spec.Template.Annotations).To(
			HaveKeyWithValue("sidecar.istio.io/inject", "false"),
		)
		dump := job.Spec.Template.Spec.InitContainers[0]
		Expect(dump.Env).To(ContainElement(corev1.EnvVar{Name: "AWS_ENDPOINT_URL", Value: "http://evil.example"}))
		upload := job.Spec.Template.Spec.Containers[0]
		Expect(upload.Env).NotTo(ContainElement(HaveField("Name", "AWS_ENDPOINT_URL")))
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
		}, "20s", interval).Should(Succeed())
	})

	// A dependency that returns within the grace is a binding, a Secret, or a
	// reachable API. It is never the cluster itself: a cluster created again
	// under the same name is another cluster, and the pinned UID fails the
	// backup (see the replaced-cluster spec).
	It("holds a mid-run failure and recovers when the dependency returns", func() {
		w := createWorld()
		backup := createBackup(w)
		jobOf(backup, w)

		// Once the Job is tracked, the dump step needs no resolution. The
		// missing binding bites when the Zeebe step starts.
		By("unpublishing the management binding, then finishing the dump")
		Eventually(func(g Gomega) {
			g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(w.cluster), w.cluster)).To(Succeed())
			w.cluster.Status.Management = nil
			g.Expect(k8sClient.Status().Update(ctx, w.cluster)).To(Succeed())
		}, timeout, interval).Should(Succeed())
		markJob(backup, w, batchv1.JobComplete)

		By("holding the backup within the grace, failure clock running")
		Eventually(func(g Gomega) {
			g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(backup), backup)).To(Succeed())
			g.Expect(backup.Status.Phase).To(Equal(v1.LogicalBackupRunning))
			g.Expect(backup.Status.FirstFailedAt).NotTo(BeNil())
		}, timeout, interval).Should(Succeed())

		By("publishing the binding again within the grace")
		converge(w.cluster)

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
		}, "20s", interval).Should(Succeed())
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
		// so a backup that the cluster does not report yet is normal at first.
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

	// The Job is rendered from a live read after the flush of
	// admission. The pinned invariants are checked again right before, so a
	// retarget in that gap never renders a Job that uploads elsewhere.
	It("does not render the Job when the bucket was retargeted between admission and the Job", func() {
		w := createWorld()
		unconvergeCluster(w)
		backup := createBackup(w)
		expectPending(backup, v1.ReasonProgressing)
		stageAdmitted(w, backup)

		By("retargeting the bucket in the gap, then letting the cluster converge")
		Eventually(func(g Gomega) {
			g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(w.bucket), w.bucket)).To(Succeed())
			w.bucket.Spec.S3.BucketName = "moved-elsewhere"
			g.Expect(k8sClient.Update(ctx, w.bucket)).To(Succeed())
		}, timeout, interval).Should(Succeed())
		converge(w.cluster)

		Eventually(func(g Gomega) {
			g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(backup), backup)).To(Succeed())
			g.Expect(backup.Status.Phase).To(Equal(v1.LogicalBackupFailed))
			g.Expect(backup.Status.FailureMessage).To(ContainSubstring("moved-elsewhere"))
			g.Expect(backup.Status.FailureMessage).To(ContainSubstring("camunda-backups"))
		}, "20s", interval).Should(Succeed())
		var job batchv1.Job
		Expect(k8sClient.Get(
			ctx, types.NamespacedName{
				Namespace: w.namespace, Name: components.JobName(backup),
			}, &job,
		)).NotTo(Succeed(), "no Job was rendered against the moved bucket")
	})

	// A cluster that was deleted and created again under the same name is
	// another cluster with other primary storage, and its config hash can
	// match. The pinned UID tells the two apart, and the backup fails at
	// once instead of pairing its dump with the Zeebe backup of a stranger.
	It("fails when the cluster was replaced under the same name after admission", func() {
		w := createWorld()
		unconvergeCluster(w)
		backup := createBackup(w)
		expectPending(backup, v1.ReasonProgressing)
		stageAdmitted(w, backup)

		By("pinning the UID of a cluster that no longer exists; the world's cluster is its replacement")
		const gone = "uid-of-the-cluster-that-admitted-the-backup"
		patchStatusUntilStable(
			backup, func(b *v1.LogicalBackupRDBMS) { b.Status.ClusterUID = gone },
			func(b *v1.LogicalBackupRDBMS) bool { return b.Status.ClusterUID == gone },
		)
		converge(w.cluster)

		Eventually(func(g Gomega) {
			g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(backup), backup)).To(Succeed())
			g.Expect(backup.Status.Phase).To(Equal(v1.LogicalBackupFailed))
			g.Expect(backup.Status.FailureMessage).To(ContainSubstring("was replaced"))
			g.Expect(backup.Status.FailureMessage).To(ContainSubstring(string(w.cluster.UID)))
		}, "20s", interval).Should(Succeed())
		var job batchv1.Job
		Expect(k8sClient.Get(
			ctx, types.NamespacedName{Namespace: w.namespace, Name: components.JobName(backup)}, &job,
		)).NotTo(Succeed(), "no Job was rendered against the replacement cluster")
	})

	It("holds the Job while the cluster rolls out in the gap, then renders it against the pinned inputs", func() {
		w := createWorld()
		unconvergeCluster(w)
		backup := createBackup(w)
		expectPending(backup, v1.ReasonProgressing)
		stageAdmitted(w, backup)
		pinnedKey := backup.Status.ObjectKey

		By("holding: no Job while the cluster has not converged")
		Consistently(func(g Gomega) {
			g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(backup), backup)).To(Succeed())
			g.Expect(backup.Status.Phase).To(Equal(v1.LogicalBackupRunning), backup.Status.FailureMessage)
			g.Expect(backup.Status.JobName).To(BeEmpty())
			g.Expect(k8sClient.Get(
				ctx, types.NamespacedName{
					Namespace: w.namespace, Name: components.JobName(backup),
				}, &batchv1.Job{},
			)).NotTo(Succeed())
		}, "2s", interval).Should(Succeed())

		converge(w.cluster)
		job := jobOf(backup, w)
		Expect(envValueOf(job.Spec.Template.Spec.Containers[0], components.EnvUploadKey)).To(Equal(pinnedKey))
	})

	// The converged generation cannot tell a database swap.
	// Mutable referents enter the workload config hash and do not bump the
	// generation. The pinned hash can tell the swap.
	It("fails when Zeebe rolled to another configuration between the dump and the Zeebe backup", func() {
		w := createWorld()
		backup := createBackup(w)
		jobOf(backup, w)
		Eventually(func(g Gomega) {
			g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(backup), backup)).To(Succeed())
			g.Expect(backup.Status.WorkloadConfigHash).To(Equal("hash-1"), "the hash is pinned at start")
		}, timeout, interval).Should(Succeed())

		By("rolling the Zeebe workload to another configuration, still converged")
		renderZeebe(w.cluster, "hash-2", worldRDBMSURL)

		markJob(backup, w, batchv1.JobComplete)
		Eventually(func(g Gomega) {
			g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(backup), backup)).To(Succeed())
			g.Expect(backup.Status.Phase).To(Equal(v1.LogicalBackupFailed))
			g.Expect(backup.Status.FailureMessage).To(ContainSubstring("hash-1"))
			g.Expect(backup.Status.FailureMessage).To(ContainSubstring("hash-2"))
			g.Expect(backup.Status.ZeebeBackupID).To(BeNil())
		}, "20s", interval).Should(Succeed())
	})

	// The pinned hash proves that Zeebe did not roll since the
	// start. It does not prove that the referents that the dump reads are the
	// ones that Zeebe runs. Between an edit of the DatabaseConfig and the
	// rendering of the cluster controller, the two differ. The own URL of the
	// template tells them apart.
	It("waits at admission until Zeebe runs the database the dump would capture", func() {
		w := createWorld()
		By("Zeebe still runs the previous database while the DatabaseConfig names the next one")
		renderZeebe(w.cluster, "hash-1", camundacluster.RDBMSURL("postgres.databases.svc", 5432, "camunda-old"))
		backup := createBackup(w)

		expectPending(backup, v1.ReasonProgressing)
		Eventually(func(g Gomega) {
			g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(backup), backup)).To(Succeed())
			ready := meta.FindStatusCondition(backup.Status.Conditions, v1.ConditionReady)
			g.Expect(ready).NotTo(BeNil())
			g.Expect(ready.Message).To(SatisfyAll(
				ContainSubstring("camunda-old"),
				ContainSubstring("/camunda"),
				ContainSubstring("waits until Zeebe runs"),
			))
			g.Expect(backup.Status.BackupID).
				To(BeZero(), "no identity is allocated against a database Zeebe does not run")
		}, timeout, interval).Should(Succeed())

		By("starting once the cluster rolled to the referenced database")
		renderZeebe(w.cluster, "hash-2", worldRDBMSURL)
		jobOf(backup, w)
	})

	It("fails before the Job when the referents changed under an unrolled Zeebe, naming both databases", func() {
		w := createWorld()
		unconvergeCluster(w)
		backup := createBackup(w)
		expectPending(backup, v1.ReasonProgressing)
		stageAdmitted(w, backup)

		By("renaming the database in the gap, before the cluster controller rendered it")
		Eventually(func(g Gomega) {
			g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(w.dbConfig), w.dbConfig)).To(Succeed())
			w.dbConfig.Spec.DatabaseName = "camunda-next"
			g.Expect(k8sClient.Update(ctx, w.dbConfig)).To(Succeed())
		}, timeout, interval).Should(Succeed())
		converge(w.cluster)

		By("failing through the mid-run grace: the dump would read camunda-next while Zeebe runs camunda")
		Eventually(func(g Gomega) {
			g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(backup), backup)).To(Succeed())
			g.Expect(backup.Status.Phase).To(Equal(v1.LogicalBackupFailed))
			g.Expect(backup.Status.FailureMessage).To(SatisfyAll(
				ContainSubstring("5432/camunda,"),
				ContainSubstring("camunda-next"),
			))
		}, "20s", interval).Should(Succeed())
		Expect(k8sClient.Get(
			ctx, types.NamespacedName{
				Namespace: w.namespace, Name: components.JobName(backup),
			}, &batchv1.Job{},
		)).NotTo(Succeed(), "no Job was rendered against a database Zeebe does not run")
	})

	// A pod that cannot start never fails its Job and consumes no retry.
	// It must run through the bounded grace. It must not hold the queue.
	It("fails a dump whose pod is stuck in a non-progressing waiting state", func() {
		w := createWorld()
		backup := createBackup(w)
		job := jobOf(backup, w)

		pod := podOfBackup(backup, job, "stuck")
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
		}, "20s", interval).Should(Succeed())
	})

	// The Zeebe backup goes to the current backup store of the cluster. A
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
		}, "20s", interval).Should(Succeed())
	})

	// The Job is released once the dump is recorded, so a PVC-backed
	// scratch volume does not live as long as the backup. The backup itself
	// is unaffected, and deletion stays clean.
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

	// A management API that rejects the call again and again is bounded
	// like an unreachable one. A Running backup never parks forever.
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
		}, "20s", interval).Should(Succeed())
	})

	// A retargeted bucket must never make the finalizer delete a
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

	// The object is deleted only once the Job and its pods are gone. So
	// an uploader that still terminates cannot recreate it after the delete.
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

		// A pod of the Job that is still around. envtest runs no kubelet, so
		// the pod stays until it is deleted by hand.
		pod := podOfBackup(backup, job, "x1")
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

	// A Secret that exists but lost a key is repairable. The deletion
	// must hold visibly until the Secret is repaired. Only a missing Secret
	// takes the best-effort release path.
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

	// After a background delete, a same-named foreign Job can
	// take the name. The pod check follows the UID of this backup, so its own
	// uploader that still terminates holds the deletion, and a foreign pod
	// never does.
	It("holds deletion for its own pods even when a foreign Job took the name", func() {
		w := createWorld()
		backup := createBackup(w)
		job := jobOf(backup, w)
		var objectKey string
		Eventually(func(g Gomega) {
			g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(backup), backup)).To(Succeed())
			g.Expect(backup.Status.ObjectKey).NotTo(BeEmpty())
			objectKey = backup.Status.ObjectKey
		}, timeout, interval).Should(Succeed())

		By("leaving this backup's uploader terminating while a foreign Job takes the name")
		own := podOfBackup(backup, job, "own")
		Expect(k8sClient.Create(ctx, own)).To(Succeed())
		foreignPod := podOfBackup(backup, job, "foreign")
		foreignPod.Labels[components.BackupUIDLabel] = "someone-else"
		Expect(k8sClient.Create(ctx, foreignPod)).To(Succeed())
		DeferCleanup(func() { _ = k8sClient.Delete(ctx, foreignPod, client.GracePeriodSeconds(0)) })

		propagation := metav1.DeletePropagationBackground
		Expect(k8sClient.Delete(ctx, job, &client.DeleteOptions{PropagationPolicy: &propagation})).To(Succeed())
		Eventually(func(g Gomega) {
			g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(job), job)).NotTo(Succeed())
		}, timeout, interval).Should(Succeed())
		foreign := &batchv1.Job{
			ObjectMeta: metav1.ObjectMeta{
				Name: job.Name, Namespace: w.namespace,
				Labels: map[string]string{components.BackupUIDLabel: "someone-else"},
			},
			Spec: batchv1.JobSpec{Template: corev1.PodTemplateSpec{Spec: corev1.PodSpec{
				RestartPolicy: corev1.RestartPolicyNever,
				Containers:    []corev1.Container{{Name: "x", Image: "x"}},
			}}},
		}
		Expect(k8sClient.Create(ctx, foreign)).To(Succeed())
		DeferCleanup(
			func() { _ = k8sClient.Delete(ctx, foreign, &client.DeleteOptions{PropagationPolicy: &propagation}) },
		)

		Expect(k8sClient.Delete(ctx, backup)).To(Succeed())
		By("holding the object while this backup's pod lives")
		Consistently(func(g Gomega) {
			g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(backup), backup)).To(Succeed())
			g.Expect(bucket.Deleted()).NotTo(ContainElement(objectKey))
		}, "3s", interval).Should(Succeed())

		By("deleting the object once it is gone; the foreign Job and pod never held it")
		Expect(k8sClient.Delete(ctx, own, client.GracePeriodSeconds(0))).To(Succeed())
		Eventually(func(g Gomega) {
			err := k8sClient.Get(ctx, client.ObjectKeyFromObject(backup), backup)
			g.Expect(err).To(HaveOccurred())
			g.Expect(client.IgnoreNotFound(err)).To(Succeed())
			g.Expect(bucket.Deleted()).To(ContainElement(objectKey))
		}, timeout, interval).Should(Succeed())
		Expect(
			k8sClient.Get(ctx, client.ObjectKeyFromObject(foreign), foreign),
		).To(Succeed(), "the foreign Job is left alone")
	})

	// A workload-identity bucket binds the cluster
	// ServiceAccount, which the manager does not have. Cleanup runs where
	// the identity lives, in a Job under that ServiceAccount.
	It("cleans a workload-identity bucket through a Job under the cluster ServiceAccount", func() {
		w := createWorld()
		makeBucketWIF(w)
		backup := createBackup(w)

		markJob(backup, w, batchv1.JobComplete)
		Eventually(func(g Gomega) {
			g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(backup), backup)).To(Succeed())
			g.Expect(backup.Status.ZeebeBackupID).NotTo(BeNil())
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

		By("creating the cleanup Job on the dump Job's identity surface")
		var cleanup batchv1.Job
		cleanupKey := types.NamespacedName{
			Namespace: w.namespace, Name: components.CleanupJobName(backup),
		}
		Eventually(func(g Gomega) {
			g.Expect(k8sClient.Get(ctx, cleanupKey, &cleanup)).To(Succeed())
		}, timeout, interval).Should(Succeed())
		pod := cleanup.Spec.Template.Spec
		Expect(pod.ServiceAccountName).To(Equal(w.cluster.Name + "-camunda"))
		Expect(pod.Containers[0].Image).To(Equal("ghcr.io/konsole-is/camunda-operator-cli:test"))
		Expect(pod.Containers[0].Args).To(Equal([]string{"delete"}))
		Expect(cleanup.Labels).To(HaveKeyWithValue(components.BackupUIDLabel, string(backup.UID)))
		Expect(envValueOf(pod.Containers[0], components.EnvUploadKey)).To(Equal(objectKey))

		By("releasing once the cleanup Job completes, without an in-manager delete")
		markJobNamed(cleanup.Name, w, batchv1.JobComplete)
		Eventually(func(g Gomega) {
			err := k8sClient.Get(ctx, client.ObjectKeyFromObject(backup), backup)
			g.Expect(err).To(HaveOccurred())
			g.Expect(client.IgnoreNotFound(err)).To(Succeed())
		}, timeout, interval).Should(Succeed())
		Expect(bucket.Deleted()).NotTo(ContainElement(objectKey), "the manager never opened the bucket")
	})

	It("holds the deletion visibly while the cleanup Job fails", func() {
		w := createWorld()
		makeBucketWIF(w)
		backup := createBackup(w)
		Eventually(func(g Gomega) {
			g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(backup), backup)).To(Succeed())
			g.Expect(backup.Status.ObjectKey).NotTo(BeEmpty())
		}, timeout, interval).Should(Succeed())

		Expect(k8sClient.Delete(ctx, backup)).To(Succeed())
		cleanupKey := types.NamespacedName{
			Namespace: w.namespace, Name: components.CleanupJobName(backup),
		}
		Eventually(func(g Gomega) {
			g.Expect(k8sClient.Get(ctx, cleanupKey, &batchv1.Job{})).To(Succeed())
		}, timeout, interval).Should(Succeed())
		markJobNamed(cleanupKey.Name, w, batchv1.JobFailed)

		By("holding the finalizer and naming the failed Job")
		Eventually(func(g Gomega) {
			var events eventsv1.EventList
			g.Expect(k8sClient.List(ctx, &events, client.InNamespace(w.namespace))).To(Succeed())
			notes := make([]string, 0, len(events.Items))
			for _, event := range events.Items {
				notes = append(notes, event.Reason+": "+event.Note)
			}
			g.Expect(notes).To(ContainElement(And(
				HavePrefix("ArtifactCleanupFailed"), ContainSubstring(cleanupKey.Name),
			)))
		}, timeout, interval).Should(Succeed())
		Consistently(func(g Gomega) {
			g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(backup), backup)).To(Succeed())
		}, "2s", interval).Should(Succeed())
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
