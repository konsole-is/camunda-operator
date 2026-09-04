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

package logicalrestoreelasticsearch

import (
	"strconv"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
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
	"github.com/konsole-is/camunda-operator/internal/fixtures"
	"github.com/konsole-is/camunda-operator/internal/testenv"
	"github.com/konsole-is/camunda-operator/pkg/camundaconfig"
	"github.com/konsole-is/camunda-operator/pkg/clusterclaim"
	camundacluster "github.com/konsole-is/camunda-operator/pkg/components/camundacluster"
	elasticsearchcluster "github.com/konsole-is/camunda-operator/pkg/components/elasticsearchcluster"
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

// backupID is the id of every backup of a spec. It is fixed, so the names of
// the snapshots and of the Jobs are predictable.
const backupID = int64(1772001869309)

// elasticsearchSnapshots are the web-application snapshots that a backup of
// the world records, without an Optimize snapshot.
var elasticsearchSnapshots = []string{
	"camunda_webapps_1772001869309_8.9.9_part_1_of_2",
	"camunda_webapps_1772001869309_8.9.9_part_2_of_2",
}

// optimizeSnapshot is what a backup that also holds Optimize data records.
const optimizeSnapshot = "camunda_optimize_1772001869309_8.9.9_part_1_of_1"

// targetIndices are the Camunda indices that the target holds before the
// restore. They are what the restore deletes, and the Optimize one is what it
// keeps unless the backup can put it back.
var targetIndices = []string{
	"camunda-list-view-8.9.0_",
	"operate-flownode-instance-8.9.0_",
	"tasklist-task-8.9.0_",
	"zeebe-record_process-instance_8.9.0_2026-08-20",
	"optimize-process-instance_8.9.0",
}

// world is one suspended target cluster with everything a restore of it
// reads: the storage contract, the fake Elasticsearch behind it, the backup
// bucket, the broker StatefulSet, and the volumes the brokers hold.
type world struct {
	namespace string
	cluster   *v1.CamundaCluster
	storage   *v1.SecondaryStorageConfig
	bucket    *v1.ObjectStorageConfig

	// search is the fake Elasticsearch of the target.
	search *esadmintest.Server
	// esNamespace and esCluster name the ElasticsearchCluster that registered
	// the repository of the backup. They are deliberately not the namespace
	// of the restore: a cluster reads through the Elasticsearch of any
	// namespace, and the snapshots lie under the prefix of that
	// Elasticsearch.
	esNamespace string
	esCluster   string
	// repository is the snapshot repository that the backup recorded.
	repository string
}

func newNamespace() string {
	GinkgoHelper()
	name := "lres-ns-" + utilrand.String(8)
	Expect(k8sClient.Create(ctx, &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{Name: name},
	})).To(Succeed())

	return name
}

// newWorld builds a suspended Elasticsearch-backed target with a fake
// Elasticsearch behind its storage contract.
func newWorld(mutate ...func(*v1.CamundaCluster)) *world {
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
				CredentialsSecretRef: v1.LocalCredentialsSecretRef{
					Name:        credentials.Name,
					UsernameKey: "username",
					PasswordKey: "password",
				},
				CASecretRef: &v1.LocalSecretKeyRef{Name: ca.Name, Key: "ca.crt"},
			},
		},
	}
	Expect(k8sClient.Create(ctx, w.storage)).To(Succeed())

	w.finish(mutate...)
	w.esNamespace = "es-ns-" + utilrand.String(6)
	w.esCluster = "es-" + utilrand.String(6)
	w.repository = elasticsearchcluster.RepositoryName(&v1.ElasticsearchCluster{
		ObjectMeta: metav1.ObjectMeta{Name: w.esCluster, Namespace: w.esNamespace},
	})

	return w
}

// finish creates the bucket, the cluster, the broker StatefulSet, and the
// data volumes that the brokers hold.
func (w *world) finish(mutate ...func(*v1.CamundaCluster)) {
	GinkgoHelper()

	w.bucket = &v1.ObjectStorageConfig{
		ObjectMeta: metav1.ObjectMeta{Name: "osc-" + utilrand.String(6), Namespace: w.namespace},
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
		ObjectMeta: metav1.ObjectMeta{
			Name:        name,
			Namespace:   w.namespace,
			Labels:      selector,
			Annotations: map[string]string{camundacluster.BrokerVersionAnnotation: worldVersion},
		},
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

// brokerStatefulSet reads the live broker StatefulSet of the world.
func (w *world) brokerStatefulSet(g Gomega) *appsv1.StatefulSet {
	name := camundacluster.WorkloadName(w.cluster, camundacluster.ComponentZeebe)

	var brokers appsv1.StatefulSet
	key := types.NamespacedName{Namespace: w.namespace, Name: name}
	g.Expect(k8sClient.Get(ctx, key, &brokers)).To(Succeed())

	return &brokers
}

// setRunningBrokers stands in for the StatefulSet controller, which envtest
// does not run. A restore reads status.replicas of the broker StatefulSet to
// learn whether the brokers of a suspended cluster really stopped.
func (w *world) setRunningBrokers(running int32) {
	GinkgoHelper()
	Eventually(func(g Gomega) {
		brokers := w.brokerStatefulSet(g)
		brokers.Status.Replicas = running
		g.Expect(k8sClient.Status().Update(ctx, brokers)).To(Succeed())
	}, timeout, interval).Should(Succeed())
}

// rollBrokerImage stands in for the CamundaCluster controller rolling a new
// spec.version into the broker StatefulSet. The broker version annotation of
// that StatefulSet is where a restore reads the Camunda version that the
// cluster really runs.
func (w *world) rollBrokerImage(version string) {
	GinkgoHelper()
	Eventually(func(g Gomega) {
		brokers := w.brokerStatefulSet(g)
		brokers.Annotations[camundacluster.BrokerVersionAnnotation] = version
		brokers.Spec.Template.Spec.Containers[0].Image = "camunda/camunda:" + version
		g.Expect(k8sClient.Update(ctx, brokers)).To(Succeed())
	}, timeout, interval).Should(Succeed())
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

// registerRepositoryAt registers the repository of the world over the given
// prefix. A spec uses another prefix to stand in for a registration that
// somebody else made and that the restore must not overwrite.
func (w *world) registerRepositoryAt(basePath string) {
	GinkgoHelper()

	admin, err := esadmin.New(w.search.URL(), "elastic", "secret", w.search.CertificatePEM())
	Expect(err).NotTo(HaveOccurred())
	Expect(admin.EnsureSnapshotRepository(ctx, w.repository, esadmin.RepositoryConfig{
		Type:     esadmin.RepositoryTypeS3,
		Bucket:   w.bucket.Spec.S3.BucketName,
		BasePath: basePath,
	})).To(Succeed())
}

// seedSnapshots puts the snapshots of a backup into the fake Elasticsearch,
// in the repository that the backup recorded. The restore asks for a snapshot
// by name, and Elasticsearch answers a name it does not hold with an error.
//
// It registers no repository. A target whose Elasticsearch already holds one
// is the other half of the story, and a spec of that half registers it with
// registerRepositoryAt first.
func (w *world) seedSnapshots(names ...string) {
	GinkgoHelper()

	for _, name := range append(names, logicalbackup.RecordsSnapshotName(backupID)) {
		w.search.SetSnapshotState(w.repository, name, "SUCCESS")
	}
}

// createBackup creates a completed Elasticsearch backup of the world.
// Mutators shape its status before it is written.
func createBackup(
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

// createRestore creates a LogicalRestoreElasticsearch of the world that names
// the given backup.
func createRestore(w *world, name string) *v1.LogicalRestoreElasticsearch {
	GinkgoHelper()

	restore := &v1.LogicalRestoreElasticsearch{
		ObjectMeta: metav1.ObjectMeta{Name: "lres-" + utilrand.String(6), Namespace: w.namespace},
		Spec: v1.LogicalRestoreElasticsearchSpec{
			BackupRef:        v1.LogicalBackupRef{Name: name},
			TargetClusterRef: v1.ClusterRef{Name: w.cluster.Name},
		},
	}
	Expect(k8sClient.Create(ctx, restore)).To(Succeed())

	return restore
}

// startedRestore creates a restore of a seeded world and drives it through
// the secondary-storage phase, so a spec about primary storage starts where
// that phase ends.
func startedRestore(w *world, backup *v1.LogicalBackupElasticsearch) *v1.LogicalRestoreElasticsearch {
	GinkgoHelper()

	w.seedSnapshots(backup.Status.HistorySnapshots...)
	w.search.SetRecoveryActive(false)

	restore := createRestore(w, backup.Name)
	serveRestoredIndices(w, restore)
	expectPhase(restore, v1.LogicalRestoreRestoringPrimaryStorage)

	return restore
}

// serveRestoredIndices stands in for the Elasticsearch that creates the
// restored indices. It waits until the restore asked for its snapshots, then
// seeds the indices those snapshots bring back.
func serveRestoredIndices(w *world, restore *v1.LogicalRestoreElasticsearch) {
	GinkgoHelper()

	Eventually(func(g Gomega) {
		g.Expect(latest(g, restore).Status.RestoredSnapshots).NotTo(BeEmpty())
	}, timeout, interval).Should(Succeed())

	w.search.SetIndices(targetIndices...)
}

// latest reads the restore again and hands it to the assertion.
func latest(g Gomega, restore *v1.LogicalRestoreElasticsearch) *v1.LogicalRestoreElasticsearch {
	var current v1.LogicalRestoreElasticsearch
	g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(restore), &current)).To(Succeed())

	return &current
}

// latestOf reads the restore as it is now, outside an Eventually.
func latestOf(restore *v1.LogicalRestoreElasticsearch) *v1.LogicalRestoreElasticsearch {
	GinkgoHelper()

	var current *v1.LogicalRestoreElasticsearch
	Eventually(func(g Gomega) { current = latest(g, restore) }, timeout, interval).Should(Succeed())

	return current
}

// clusterSuspended reports spec.suspend of the target as it is now.
func clusterSuspended(g Gomega, w *world) bool {
	var cluster v1.CamundaCluster
	g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(w.cluster), &cluster)).To(Succeed())

	return cluster.Spec.Suspend
}

func readyCondition(restore *v1.LogicalRestoreElasticsearch) *metav1.Condition {
	return meta.FindStatusCondition(restore.Status.Conditions, v1.ConditionReady)
}

// expectPhase waits until the restore reports the given phase.
func expectPhase(restore *v1.LogicalRestoreElasticsearch, phase v1.LogicalRestorePhase) {
	GinkgoHelper()

	Eventually(func(g Gomega) {
		reached := latest(g, restore)
		g.Expect(reached.Status.Phase).To(Equal(phase), "Ready: %s", readyMessage(reached))
	}, timeout, interval).Should(Succeed())
}

// expectReason waits until the restore reports the given phase and Ready
// reason.
func expectReason(
	restore *v1.LogicalRestoreElasticsearch,
	phase v1.LogicalRestorePhase,
	reason string,
) *v1.LogicalRestoreElasticsearch {
	GinkgoHelper()

	var reached *v1.LogicalRestoreElasticsearch
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
func readyMessage(restore *v1.LogicalRestoreElasticsearch) string {
	condition := readyCondition(restore)
	if condition == nil {
		return "<no Ready condition>"
	}

	return condition.Reason + ": " + condition.Message
}

// claimHolder returns the identity that the claim Lease of the cluster
// records, or "" when no Lease exists.
func claimHolder(w *world) string {
	GinkgoHelper()

	var lease coordinationv1.Lease
	key := types.NamespacedName{
		Namespace: w.namespace, Name: clusterclaim.ClaimLeaseName(w.cluster.Name),
	}
	err := k8sClient.Get(ctx, key, &lease)
	if apierrors.IsNotFound(err) {
		return ""
	}
	Expect(err).NotTo(HaveOccurred())

	annotations := lease.GetAnnotations()

	return annotations[clusterclaim.ClaimHolderKindAnnotation] + "/" +
		annotations[clusterclaim.ClaimHolderNameAnnotation]
}

// holdCluster creates a backup of the cluster and gives it the claim on that
// cluster, as the backup controller does when its admission passes.
func holdCluster(w *world) *v1.LogicalBackupElasticsearch {
	GinkgoHelper()

	backup := &v1.LogicalBackupElasticsearch{
		ObjectMeta: metav1.ObjectMeta{Name: "holder-" + utilrand.String(6), Namespace: w.namespace},
		Spec:       v1.LogicalBackupElasticsearchSpec{ClusterRef: v1.ClusterRef{Name: w.cluster.Name}},
	}
	Expect(k8sClient.Create(ctx, backup)).To(Succeed())

	holder, err := clusterclaim.Claim(
		ctx, k8sClient, k8sClient, w.namespace, w.cluster.Name,
		clusterclaim.Claimant{Kind: backup.GetKind(), Name: backup.Name, UID: backup.UID},
	)
	Expect(err).NotTo(HaveOccurred())
	Expect(holder).To(BeEmpty(), "the backup must hold the claim before the restore looks")

	return backup
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

// stuckPod creates a pod of a Job of the restore that reports a container
// which cannot start. It copies the labels of the pod template of that Job
// and takes a controller reference to it, the way the Job controller does.
// envtest runs no kubelet, so the suite writes the state that a kubelet
// reports.
func stuckPod(w *world, jobName string) {
	GinkgoHelper()

	// A restore records the name of a Job before it applies the Job, so the
	// name can reach the status before the Job exists.
	var job batchv1.Job
	Eventually(func(g Gomega) {
		g.Expect(k8sClient.Get(
			ctx, types.NamespacedName{Namespace: w.namespace, Name: jobName}, &job,
		)).To(Succeed())
	}, timeout, interval).Should(Succeed())

	pod := testenv.PodOfJob(&job, jobName+"-stuck", corev1.Container{
		Name: "restore", Image: "camunda/camunda:" + worldVersion,
	})
	Expect(k8sClient.Create(ctx, pod)).To(Succeed())

	pod.Status.Phase = corev1.PodPending
	pod.Status.ContainerStatuses = []corev1.ContainerStatus{{
		Name: "restore",
		State: corev1.ContainerState{Waiting: &corev1.ContainerStateWaiting{
			Reason:  "CreateContainerConfigError",
			Message: "secret \"camunda-backup-credentials\" not found",
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
					Containers: []corev1.Container{
						{Name: "restore", Image: "camunda/camunda:" + worldVersion},
					},
				},
			},
		},
	}
	Expect(k8sClient.Create(ctx, job)).To(Succeed())
	markJob(w.namespace, name, batchv1.JobComplete)
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

// expectJobsCollected asserts that the operator asked for every named Job with
// foreground propagation.
//
// envtest runs no garbage collector, so a Job that was deleted that way keeps
// the foreground finalizer and stays in place. The finalizer and the deletion
// timestamp are therefore what proves the delete. Background propagation
// returns before the pods are gone, and the pods are what hold the broker
// volumes.
func expectJobsCollected(namespace string, names []string) {
	GinkgoHelper()
	Eventually(func(g Gomega) {
		for _, name := range names {
			var job batchv1.Job
			key := types.NamespacedName{Namespace: namespace, Name: name}
			g.Expect(k8sClient.Get(ctx, key, &job)).To(Succeed())
			g.Expect(job.DeletionTimestamp).NotTo(BeNil(), "Job %q", name)
			g.Expect(job.Finalizers).To(
				ContainElement(metav1.FinalizerDeleteDependents),
				"Job %q goes without foreground propagation, so its pods outlive it", name,
			)
		}
	}, timeout, interval).Should(Succeed())
}

// expectJobsKept asserts that every named Job stays as it is.
func expectJobsKept(namespace string, names []string) {
	GinkgoHelper()
	Consistently(func(g Gomega) {
		for _, name := range names {
			var job batchv1.Job
			key := types.NamespacedName{Namespace: namespace, Name: name}
			g.Expect(k8sClient.Get(ctx, key, &job)).To(Succeed())
			g.Expect(job.DeletionTimestamp).To(BeNil(), "Job %q", name)
		}
	}, "1s", interval).Should(Succeed())
}

// collectDeletedJobs plays the garbage collector of a real cluster, for as long
// as the spec runs. Foreground propagation makes the API server stamp the
// foregroundDeletion finalizer on a deleted Job and keep the Job until its pods
// are gone. envtest runs no collector that removes it again, so a spec that
// waits for the Jobs of a completed restore to go needs this loop.
func collectDeletedJobs(namespace string) {
	stop := make(chan struct{})
	DeferCleanup(func() { close(stop) })

	go func() {
		defer GinkgoRecover()
		for {
			select {
			case <-stop:
				return
			case <-time.After(50 * time.Millisecond):
			}

			var jobs batchv1.JobList
			if err := k8sClient.List(ctx, &jobs, client.InNamespace(namespace)); err != nil {
				continue
			}
			for i := range jobs.Items {
				job := &jobs.Items[i]
				if job.DeletionTimestamp == nil || len(job.Finalizers) == 0 {
					continue
				}
				job.Finalizers = nil
				_ = k8sClient.Update(ctx, job)
			}
		}
	}()
}
