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
	"strconv"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
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
	"github.com/konsole-is/camunda-operator/pkg/camundaconfig"
	camundacluster "github.com/konsole-is/camunda-operator/pkg/components/camundacluster"
	"github.com/konsole-is/camunda-operator/pkg/esadmin"
	"github.com/konsole-is/camunda-operator/pkg/esadmin/esadmintest"
	"github.com/konsole-is/camunda-operator/pkg/logicalbackup"
)

// The facts of every world. The version is the tag of the broker image and
// the version that a backup of the world records, so a pair is compatible
// until a spec changes one of them.
const (
	worldVersion    = "8.9.9"
	worldBrokers    = int32(2)
	worldPartitions = int32(3)
	// worldClaimSize is the request of the claim template of the broker
	// StatefulSet. A backup that recorded no size gives the recreated volumes
	// this one.
	worldClaimSize = "10Gi"
	// worldRestoreSize is the effective restore size that a backup records. It
	// is larger than the template, which is the point of recording it.
	worldRestoreSize = "25Gi"
)

// world is one suspended target cluster with everything a restore of it
// reads: the storage contract, the backup bucket, the broker StatefulSet, and
// the volumes the brokers hold. The relational fields are set on a relational
// world alone, and search on an Elasticsearch world alone.
type world struct {
	namespace string
	cluster   *v1.CamundaCluster
	storage   *v1.SecondaryStorageConfig
	bucket    *v1.ObjectStorageConfig

	// search is the fake Elasticsearch of the target.
	search *esadmintest.Server
	// repository is the snapshot repository that the cluster registers.
	repository string

	// dbConfig and server describe the logical database of a relational
	// world.
	dbConfig *v1.DatabaseConfig
	server   *v1.DatabaseServerConfig
}

func newNamespace() string {
	GinkgoHelper()
	name := "lr-ns-" + utilrand.String(8)
	Expect(k8sClient.Create(ctx, &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{Name: name},
	})).To(Succeed())

	return name
}

// newElasticsearchWorld builds a suspended Elasticsearch-backed target with a
// fake Elasticsearch behind its storage contract.
func newElasticsearchWorld(mutate ...func(*v1.CamundaCluster)) *world {
	GinkgoHelper()

	w := &world{namespace: newNamespace()}
	w.search = esadmintest.NewTLS()
	DeferCleanup(w.search.Close)

	credentials := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: "es-credentials", Namespace: w.namespace},
		Data:       map[string][]byte{"username": []byte("camunda"), "password": []byte("secret")},
	}
	Expect(k8sClient.Create(ctx, credentials)).To(Succeed())

	ca := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: "es-ca", Namespace: w.namespace},
		Data:       map[string][]byte{"ca.crt": w.search.CertificatePEM()},
	}
	Expect(k8sClient.Create(ctx, ca)).To(Succeed())

	w.storage = &v1.SecondaryStorageConfig{
		ObjectMeta: metav1.ObjectMeta{Name: "ssc-" + utilrand.String(6), Namespace: w.namespace},
		Spec: v1.SecondaryStorageConfigSpec{
			Type: v1.SecondaryStorageTypeElasticsearch,
			Elasticsearch: &v1.ElasticsearchStorage{
				Endpoint: w.search.URL(),
				CredentialsSecretRef: v1.CredentialsSecretRef{
					Name:        credentials.Name,
					Namespace:   w.namespace,
					UsernameKey: "username",
					PasswordKey: "password",
				},
				CASecretRef: &v1.SecretKeyRef{Name: ca.Name, Namespace: w.namespace, Key: "ca.crt"},
			},
		},
	}
	Expect(k8sClient.Create(ctx, w.storage)).To(Succeed())

	w.finish(mutate...)
	w.repository = "es-" + utilrand.String(6)

	return w
}

// newRelationalWorld builds a suspended relational target with the database
// chain that a pg_restore Job renders from.
func newRelationalWorld() *world {
	GinkgoHelper()

	w := &world{namespace: newNamespace()}
	suffix := utilrand.String(6)

	w.server = &v1.DatabaseServerConfig{
		ObjectMeta: metav1.ObjectMeta{Name: "dbsc-" + suffix},
		Spec: v1.DatabaseServerConfigSpec{
			Engine: v1.DatabaseEnginePostgres,
			Host:   "postgres.databases.svc",
			Port:   5432,
			AdminCredentialsSecretRef: v1.CredentialsSecretRef{
				Name: "admin", Namespace: w.namespace, UsernameKey: "username", PasswordKey: "password",
			},
		},
	}
	Expect(k8sClient.Create(ctx, w.server)).To(Succeed())
	DeferCleanup(func() { _ = k8sClient.Delete(ctx, w.server) })
	probeServer(w.server, "17")

	backupUser := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: "backup-user", Namespace: w.namespace},
		Data:       map[string][]byte{"username": []byte("backup"), "password": []byte("s3cr3t")},
	}
	Expect(k8sClient.Create(ctx, backupUser)).To(Succeed())

	w.dbConfig = &v1.DatabaseConfig{
		ObjectMeta: metav1.ObjectMeta{Name: "dbc-" + suffix, Namespace: w.namespace},
		Spec: v1.DatabaseConfigSpec{
			ServerRef:    w.server.Name,
			DatabaseName: "camunda",
			CredentialsSecretRef: v1.CredentialsSecretRef{
				Name: "app-user", Namespace: w.namespace, UsernameKey: "username", PasswordKey: "password",
			},
			BackupCredentialsSecretRef: &v1.CredentialsSecretRef{
				Name: backupUser.Name, Namespace: w.namespace,
				UsernameKey: "username", PasswordKey: "password",
			},
		},
	}
	Expect(k8sClient.Create(ctx, w.dbConfig)).To(Succeed())

	w.storage = &v1.SecondaryStorageConfig{
		ObjectMeta: metav1.ObjectMeta{Name: "ssc-" + suffix, Namespace: w.namespace},
		Spec: v1.SecondaryStorageConfigSpec{
			Type:  v1.SecondaryStorageTypeRDBMS,
			RDBMS: &v1.RDBMSStorage{DatabaseConfigRef: w.dbConfig.Name},
		},
	}
	Expect(k8sClient.Create(ctx, w.storage)).To(Succeed())

	w.finish()

	return w
}

// finish creates what both worlds share: the bucket, the cluster, the broker
// StatefulSet, and the data volumes that the brokers hold.
func (w *world) finish(mutate ...func(*v1.CamundaCluster)) {
	GinkgoHelper()

	w.bucket = &v1.ObjectStorageConfig{
		ObjectMeta: metav1.ObjectMeta{Name: "osc-" + utilrand.String(6)},
		Spec: v1.ObjectStorageConfigSpec{
			Type: v1.ObjectStorageTypeS3,
			S3: &v1.S3Storage{
				BucketName: "camunda-backups",
				BasePath:   "clusters",
				Region:     "eu-west-1",
				Endpoint:   "http://minio.minio.svc:9000",
				Auth:       v1.S3StorageAuth{Type: v1.ObjectStorageAuthTypeWorkloadIdentity},
			},
		},
	}
	Expect(k8sClient.Create(ctx, w.bucket)).To(Succeed())
	DeferCleanup(func() { _ = k8sClient.Delete(ctx, w.bucket) })

	platform := fixtures.CamundaPlatformConfigBasic()
	Expect(k8sClient.Create(ctx, platform)).To(Succeed())
	DeferCleanup(func() { _ = k8sClient.Delete(ctx, platform) })

	w.cluster = &v1.CamundaCluster{
		ObjectMeta: metav1.ObjectMeta{Name: "cc-" + utilrand.String(6), Namespace: w.namespace},
		Spec: v1.CamundaClusterSpec{
			Version:           worldVersion,
			PlatformConfigRef: platform.Name,
			StorageRef:        w.storage.Name,
			BackupStorageRef:  w.bucket.Name,
			Suspend:           true,
		},
	}
	for _, m := range mutate {
		m(w.cluster)
	}
	Expect(k8sClient.Create(ctx, w.cluster)).To(Succeed())

	w.renderBrokers()
	w.createBrokerVolumes()
	releaseTerminatingClaims(w)
}

// renderBrokers stands in for the CamundaCluster controller. It creates the
// broker StatefulSet of a suspended cluster: zero replicas, the broker
// container with the cluster sizes it runs, and the data claim template.
func (w *world) renderBrokers() {
	GinkgoHelper()

	name := camundacluster.WorkloadName(w.cluster, camundacluster.ComponentZeebe)
	selector := map[string]string{"camunda.io/cluster": w.cluster.Name, "camunda.io/component": "zeebe"}

	workload := &appsv1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: w.namespace, Labels: selector},
		Spec: appsv1.StatefulSetSpec{
			Replicas:    new(int32(0)),
			ServiceName: name,
			Selector:    &metav1.LabelSelector{MatchLabels: selector},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{Labels: selector},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{{
						Name:  camundacluster.ContainerCamunda,
						Image: "camunda/camunda:" + worldVersion,
						Env: []corev1.EnvVar{
							camundaconfig.Var(camundaconfig.KeyClusterSize, "2"),
							camundaconfig.Var(camundaconfig.KeyClusterPartitionCount, "3"),
						},
						VolumeMounts: []corev1.VolumeMount{
							{Name: camundacluster.DataVolumeName, MountPath: "/usr/local/camunda/data"},
						},
					}},
				},
			},
			VolumeClaimTemplates: []corev1.PersistentVolumeClaim{{
				ObjectMeta: metav1.ObjectMeta{Name: camundacluster.DataVolumeName, Labels: selector},
				Spec: corev1.PersistentVolumeClaimSpec{
					AccessModes: []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce},
					Resources: corev1.VolumeResourceRequirements{
						Requests: corev1.ResourceList{
							corev1.ResourceStorage: resource.MustParse(worldClaimSize),
						},
					},
				},
			}},
		},
	}
	Expect(k8sClient.Create(ctx, workload)).To(Succeed())
}

// createBrokerVolumes creates the data claims that the StatefulSet of a
// running cluster left behind. A restore deletes them and creates them again.
func (w *world) createBrokerVolumes() {
	GinkgoHelper()

	for _, name := range w.claimNames() {
		claim := &corev1.PersistentVolumeClaim{
			ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: w.namespace},
			Spec: corev1.PersistentVolumeClaimSpec{
				AccessModes: []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce},
				Resources: corev1.VolumeResourceRequirements{
					Requests: corev1.ResourceList{
						corev1.ResourceStorage: resource.MustParse(worldClaimSize),
					},
				},
			},
		}
		Expect(k8sClient.Create(ctx, claim)).To(Succeed())
	}
}

// claimNames are the data claims of the brokers, in broker order.
func (w *world) claimNames() []string {
	names := make([]string, 0, worldBrokers)
	for ordinal := range worldBrokers {
		names = append(names, camundacluster.DataVolumeName+"-"+
			camundacluster.WorkloadName(w.cluster, camundacluster.ComponentZeebe)+"-"+
			strconv.FormatInt(int64(ordinal), 10))
	}

	return names
}

// suspend flips spec.suspend of the target.
func (w *world) suspend(suspended bool) {
	GinkgoHelper()
	Eventually(func(g Gomega) {
		g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(w.cluster), w.cluster)).To(Succeed())
		w.cluster.Spec.Suspend = suspended
		g.Expect(k8sClient.Update(ctx, w.cluster)).To(Succeed())
	}, timeout, interval).Should(Succeed())
}

// registerRepository stands in for the ElasticsearchCluster controller, which
// registers the snapshot repository of the cluster before any backup runs.
func (w *world) registerRepository() {
	GinkgoHelper()

	admin, err := esadmin.New(w.search.URL(), "elastic", "secret", w.search.CertificatePEM())
	Expect(err).NotTo(HaveOccurred())
	Expect(admin.EnsureSnapshotRepository(ctx, w.repository, esadmin.RepositoryConfig{
		Type:     esadmin.RepositoryTypeS3,
		Bucket:   w.bucket.Spec.S3.BucketName,
		BasePath: logicalbackup.ClusterPrefix(w.bucket.BasePath(), w.namespace, w.repository),
	})).To(Succeed())
}

// seedSnapshots puts the snapshots of a backup into the fake Elasticsearch,
// in the repository that the backup recorded. The restore asks for a snapshot
// by name, and Elasticsearch answers a name it does not hold with an error.
func (w *world) seedSnapshots(names ...string) {
	GinkgoHelper()

	w.registerRepository()
	for _, name := range append(names, logicalbackup.RecordsSnapshotName(backupID)) {
		w.search.SetSnapshotState(w.repository, name, "SUCCESS")
	}
}

// probeServer stands in for the DatabaseServerConfig controller, which
// publishes the probed major version and a current Ready.
func probeServer(server *v1.DatabaseServerConfig, version string) {
	GinkgoHelper()
	Eventually(func(g Gomega) {
		g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(server), server)).To(Succeed())
		now := metav1.Now()
		server.Status.ServerVersion = version
		server.Status.ProbedAt = &now
		meta.SetStatusCondition(&server.Status.Conditions, metav1.Condition{
			Type: v1.ConditionReady, Status: metav1.ConditionTrue, Reason: v1.ReasonHealthy,
			Message: "stand-in probe", ObservedGeneration: server.Generation,
		})
		g.Expect(k8sClient.Status().Update(ctx, server)).To(Succeed())
	}, timeout, interval).Should(Succeed())
}

// backupID is the id of every backup of a spec. It is fixed, so the names of
// the snapshots and of the Jobs are predictable.
const backupID = int64(1772001869309)

// elasticsearchSnapshots are the web-application snapshots that an
// Elasticsearch backup of the world records, without an Optimize snapshot.
var elasticsearchSnapshots = []string{
	"camunda_webapps_1772001869309_8.9.9_part_1_of_2",
	"camunda_webapps_1772001869309_8.9.9_part_2_of_2",
}

// optimizeSnapshot is what a backup that also holds Optimize data records.
const optimizeSnapshot = "camunda_optimize_1772001869309_8.9.9_part_1_of_1"

// createElasticsearchBackup creates a completed Elasticsearch backup of the
// world. Mutators shape its status before it is written.
func createElasticsearchBackup(
	w *world,
	mutate ...func(*v1.LogicalBackupElasticsearch),
) *v1.LogicalBackupElasticsearch {
	GinkgoHelper()

	backup := &v1.LogicalBackupElasticsearch{
		ObjectMeta: metav1.ObjectMeta{Name: "lbes-" + utilrand.String(6), Namespace: w.namespace},
		Spec:       v1.LogicalBackupElasticsearchSpec{ClusterRef: v1.ClusterRef{Name: w.cluster.Name}},
	}
	Expect(k8sClient.Create(ctx, backup)).To(Succeed())

	size := resource.MustParse(worldRestoreSize)
	backup.Status = v1.LogicalBackupElasticsearchStatus{
		Phase:            v1.LogicalBackupCompleted,
		BackupID:         backupID,
		PartitionsCount:  worldPartitions,
		Version:          worldVersion,
		Repository:       w.repository,
		HistorySnapshots: elasticsearchSnapshots,
		StorageSizes:     v1.LogicalBackupStorageSizes{Zeebe: &size},
		Storage: &v1.PinnedStorage{
			SecondaryStorageConfig: w.storage.Name,
			BucketRef:              w.bucket.Name,
			BucketLocation:         w.bucket.Location(),
		},
	}
	for _, m := range mutate {
		m(backup)
	}
	Expect(k8sClient.Status().Update(ctx, backup)).To(Succeed())

	return backup
}

// createRelationalBackup creates a completed relational backup of the world.
func createRelationalBackup(w *world, mutate ...func(*v1.LogicalBackupRDBMS)) *v1.LogicalBackupRDBMS {
	GinkgoHelper()

	backup := &v1.LogicalBackupRDBMS{
		ObjectMeta: metav1.ObjectMeta{Name: "lbr-" + utilrand.String(6), Namespace: w.namespace},
		Spec:       v1.LogicalBackupRDBMSSpec{ClusterRef: v1.ClusterRef{Name: w.cluster.Name}},
	}
	Expect(k8sClient.Create(ctx, backup)).To(Succeed())

	size := resource.MustParse(worldRestoreSize)
	backup.Status = v1.LogicalBackupRDBMSStatus{
		Phase:          v1.LogicalBackupCompleted,
		BackupID:       backupID,
		Version:        worldVersion,
		BucketRef:      w.bucket.Name,
		BucketLocation: w.bucket.Location(),
		ObjectKey: logicalbackup.ObjectKeyPrefix(
			w.bucket.BasePath(), w.namespace, w.cluster.Name, backupID,
		) + "/" + string(backup.UID) + "/camunda.dump",
		StorageSizes: v1.LogicalBackupStorageSizes{Zeebe: &size},
	}
	for _, m := range mutate {
		m(backup)
	}
	Expect(k8sClient.Status().Update(ctx, backup)).To(Succeed())

	return backup
}

// createRestore creates a LogicalRestore of the world that names the given
// backup.
func createRestore(w *world, kind v1.LogicalBackupKind, name string) *v1.LogicalRestore {
	GinkgoHelper()

	restore := &v1.LogicalRestore{
		ObjectMeta: metav1.ObjectMeta{Name: "lr-" + utilrand.String(6), Namespace: w.namespace},
		Spec: v1.LogicalRestoreSpec{
			BackupRef:        v1.LogicalBackupRef{Kind: kind, Name: name},
			TargetClusterRef: v1.ClusterRef{Name: w.cluster.Name},
		},
	}
	Expect(k8sClient.Create(ctx, restore)).To(Succeed())

	return restore
}

// latest reads the restore again and hands it to the assertion.
func latest(g Gomega, restore *v1.LogicalRestore) *v1.LogicalRestore {
	var current v1.LogicalRestore
	g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(restore), &current)).To(Succeed())

	return &current
}

func readyCondition(restore *v1.LogicalRestore) *metav1.Condition {
	return meta.FindStatusCondition(restore.Status.Conditions, v1.ConditionReady)
}

// expectPhase waits until the restore reports the given phase.
func expectPhase(restore *v1.LogicalRestore, phase v1.LogicalRestorePhase) {
	GinkgoHelper()

	Eventually(func(g Gomega) {
		reached := latest(g, restore)
		g.Expect(reached.Status.Phase).To(Equal(phase), "Ready: %s", readyMessage(reached))
	}, timeout, interval).Should(Succeed())
}

// expectReason waits until the restore reports the given phase and Ready
// reason.
func expectReason(restore *v1.LogicalRestore, phase v1.LogicalRestorePhase, reason string) *v1.LogicalRestore {
	GinkgoHelper()

	var reached *v1.LogicalRestore
	Eventually(func(g Gomega) {
		reached = latest(g, restore)
		g.Expect(reached.Status.Phase).To(Equal(phase))
		condition := readyCondition(reached)
		g.Expect(condition).NotTo(BeNil())
		g.Expect(condition.Reason).To(Equal(reason), "Ready: %s", readyMessage(reached))
	}, timeout, interval).Should(Succeed())

	return reached
}

// readyMessage renders the Ready condition of a restore for a failed
// assertion.
func readyMessage(restore *v1.LogicalRestore) string {
	condition := readyCondition(restore)
	if condition == nil {
		return "<no Ready condition>"
	}

	return condition.Reason + ": " + condition.Message
}

// markJob flips the Job to the given terminal condition. It stands in for a
// kubelet that ran the pod, which envtest does not run.
func markJob(namespace, name string, kind batchv1.JobConditionType) {
	GinkgoHelper()

	key := types.NamespacedName{Namespace: namespace, Name: name}

	var job batchv1.Job
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
			Type: precursor, Status: corev1.ConditionTrue, Reason: "Test", Message: "marked by the suite",
		},
		batchv1.JobCondition{
			Type: kind, Status: corev1.ConditionTrue, Reason: "Test", Message: "marked by the suite",
		},
	)
	if kind == batchv1.JobComplete {
		job.Status.Succeeded = 1
		job.Status.CompletionTime = &now
	}
	Expect(k8sClient.Status().Update(ctx, &job)).To(Succeed())
}

// jobNamed waits for a Job and returns it.
func jobNamed(namespace, name string) *batchv1.Job {
	GinkgoHelper()

	var job batchv1.Job
	Eventually(func(g Gomega) {
		g.Expect(k8sClient.Get(
			ctx, types.NamespacedName{Namespace: namespace, Name: name}, &job,
		)).To(Succeed())
	}, timeout, interval).Should(Succeed())

	return &job
}

// releaseTerminatingClaims stands in for the volume controller of a real
// cluster, for as long as the spec runs. The StorageObjectInUseProtection
// admission plugin stamps the pvc-protection finalizer on every claim, and
// envtest runs no controller that removes it again. Without this loop a
// deleted claim stays in Terminating for ever, and a restore waits for a
// volume that never goes.
func releaseTerminatingClaims(w *world) {
	stop, done := make(chan struct{}), make(chan struct{})

	go func() {
		defer close(done)
		for {
			select {
			case <-stop:
				return
			case <-time.After(20 * time.Millisecond):
			}

			var claims corev1.PersistentVolumeClaimList
			if err := k8sClient.List(ctx, &claims, client.InNamespace(w.namespace)); err != nil {
				continue
			}
			for i := range claims.Items {
				claim := &claims.Items[i]
				if claim.DeletionTimestamp == nil || len(claim.Finalizers) == 0 {
					continue
				}
				claim.Finalizers = nil
				_ = k8sClient.Update(ctx, claim)
			}
		}
	}()

	DeferCleanup(func() {
		close(stop)
		<-done
	})
}
