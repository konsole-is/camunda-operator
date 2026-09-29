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

package logicalrestorerdbms

import (
	"strconv"
	"strings"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
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
	"github.com/konsole-is/camunda-operator/pkg/camundaconfig"
	"github.com/konsole-is/camunda-operator/pkg/clusterclaim"
	camundacluster "github.com/konsole-is/camunda-operator/pkg/components/camundacluster"
	"github.com/konsole-is/camunda-operator/pkg/labels"
	"github.com/konsole-is/camunda-operator/pkg/logicalbackup"
	restorepkg "github.com/konsole-is/camunda-operator/pkg/restore"
	"github.com/konsole-is/camunda-operator/pkg/storagewriter"
)

// The facts of every world. The version is the tag of the broker image and
// the version that a backup of the world records, so a pair is compatible
// until a spec changes one of them.
const (
	worldVersion = "8.9.9"
	worldBrokers = int32(2)
	// worldClaimSize is the request of the claim template of the broker
	// StatefulSet. A backup that recorded no size gives the recreated volumes
	// this one.
	worldClaimSize = "10Gi"
	// worldRestoreSize is the effective restore size that a backup records. It
	// is larger than the template, which is the point of recording it.
	worldRestoreSize = "25Gi"
)

// backupID is the id of every backup of a spec. It is fixed, so the names of
// the artifacts and of the Jobs are predictable.
const backupID = int64(1772001869309)

// world is one suspended, relational-backed target cluster with everything a
// restore of it reads: the storage contract, the logical database, the server
// that holds it, the backup bucket, the broker StatefulSet, and the volumes
// the brokers hold.
type world struct {
	namespace string
	cluster   *v1.CamundaCluster
	storage   *v1.SecondaryStorageConfig
	bucket    *v1.ObjectStorageConfig
	dbConfig  *v1.DatabaseConfig
	server    *v1.DatabaseServerConfig
}

func newNamespace() string {
	GinkgoHelper()
	name := "lrr-ns-" + utilrand.String(8)
	Expect(k8sClient.Create(ctx, &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{Name: name},
	})).To(Succeed())

	return name
}

// newWorld builds a suspended relational target with the database chain that
// a pg_restore Job renders from.
func newWorld(mutate ...func(*v1.CamundaCluster)) *world {
	GinkgoHelper()

	w := &world{namespace: newNamespace()}
	suffix := utilrand.String(6)

	w.server = &v1.DatabaseServerConfig{
		ObjectMeta: metav1.ObjectMeta{Name: "dbsc-" + suffix, Namespace: w.namespace},
		Spec: v1.DatabaseServerConfigSpec{
			Engine: v1.DatabaseEnginePostgres,
			// Each world is a backend of its own, so the storage claim of one
			// world never parks the restore of another.
			Host: "postgres-" + suffix + ".databases.svc",
			Port: 5432,
			AdminCredentialsSecretRef: v1.LocalCredentialsSecretRef{
				Name: "admin", UsernameKey: "username", PasswordKey: "password",
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

	// The application role owns the database and every object in it. A
	// restore connects as that role, because only an owner can drop what
	// pg_restore --clean drops.
	appUser := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: "app-user", Namespace: w.namespace},
		Data:       map[string][]byte{"username": []byte("camunda"), "password": []byte("app-secret")},
	}
	Expect(k8sClient.Create(ctx, appUser)).To(Succeed())

	w.dbConfig = &v1.DatabaseConfig{
		ObjectMeta: metav1.ObjectMeta{Name: "dbc-" + suffix, Namespace: w.namespace},
		Spec: v1.DatabaseConfigSpec{
			ServerRef:    w.server.Name,
			DatabaseName: "camunda",
			CredentialsSecretRef: v1.LocalCredentialsSecretRef{
				Name:        appUser.Name,
				UsernameKey: "username", PasswordKey: "password",
			},
			BackupCredentialsSecretRef: &v1.LocalCredentialsSecretRef{
				Name:        backupUser.Name,
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

	w.finish(mutate...)

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

	platform := newCamundaPlatformConfigBasic()
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
	w.holdBackend(w.cluster)

	w.renderBrokers()
	w.createBrokerVolumes()
	releaseTerminatingClaims(w)
}

// holdBackend stands in for the storage claim that the CamundaCluster
// controller takes: the Lease of the backend of the world names holder, which
// is the cluster of the world unless a spec parks it. Worlds can share one
// backend, so a Lease of an earlier world is taken over.
func (w *world) holdBackend(holder *v1.CamundaCluster) {
	GinkgoHelper()

	storage, failure, err := restorepkg.ResolveStorage(ctx, k8sClient, w.cluster)
	Expect(err).NotTo(HaveOccurred())
	Expect(failure).NotTo(HaveOccurred())
	key, failure, err := restorepkg.BackendOf(ctx, k8sClient, storage)
	Expect(err).NotTo(HaveOccurred())
	Expect(failure).NotTo(HaveOccurred())

	lease := camundacluster.StorageClaimSchema().NewLease(claimNamespace, key, holder)
	err = k8sClient.Create(ctx, lease)
	if apierrors.IsAlreadyExists(err) {
		var held coordinationv1.Lease
		Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(lease), &held)).To(Succeed())
		held.Labels, held.Annotations, held.Spec = lease.Labels, lease.Annotations, lease.Spec
		err = k8sClient.Update(ctx, &held)
	}
	Expect(err).NotTo(HaveOccurred())
}

// newCamundaPlatformConfigBasic returns a platform config with basic
// authentication, no license, and no registry, with a unique name.
func newCamundaPlatformConfigBasic() *v1.CamundaPlatformConfig {
	return &v1.CamundaPlatformConfig{
		ObjectMeta: metav1.ObjectMeta{Name: "cpc-" + utilrand.String(8)},
		Spec: v1.CamundaPlatformConfigSpec{
			Auth: &v1.PlatformAuthSpec{Method: v1.AuthenticationMethodBasic},
		},
	}
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

// overrideHolds clears spec.suspend and every suspension hold of the target.
func (w *world) overrideHolds() {
	GinkgoHelper()
	Eventually(func(g Gomega) {
		g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(w.cluster), w.cluster)).To(Succeed())
		w.cluster.Spec.Suspend = false
		for key := range w.cluster.Annotations {
			if strings.HasPrefix(key, v1.SuspensionHoldPrefix) {
				delete(w.cluster.Annotations, key)
			}
		}
		g.Expect(k8sClient.Update(ctx, w.cluster)).To(Succeed())
	}, timeout, interval).Should(Succeed())
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

// createBackup creates a completed relational backup of the world. Mutators
// shape its status before it is written.
func createBackup(w *world, mutate ...func(*v1.LogicalBackupRDBMS)) *v1.LogicalBackupRDBMS {
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

// createRestore creates a LogicalRestoreRDBMS of the world that names the
// given backup.
func createRestore(w *world, name string) *v1.LogicalRestoreRDBMS {
	GinkgoHelper()

	return createNamedRestore(w, "lrr-"+utilrand.String(6), name)
}

// createNamedRestore is createRestore with a chosen name, for a spec that has
// to know the name of a Job before the restore exists.
func createNamedRestore(w *world, restoreName, backupName string) *v1.LogicalRestoreRDBMS {
	GinkgoHelper()

	lrr := &v1.LogicalRestoreRDBMS{
		ObjectMeta: metav1.ObjectMeta{Name: restoreName, Namespace: w.namespace},
		Spec: v1.LogicalRestoreRDBMSSpec{
			BackupRef:        v1.LogicalBackupRef{Name: backupName},
			TargetClusterRef: v1.ClusterRef{Name: w.cluster.Name},
		},
	}
	Expect(k8sClient.Create(ctx, lrr)).To(Succeed())

	return lrr
}

// latest reads the restore again and hands it to the assertion.
func latest(g Gomega, lrr *v1.LogicalRestoreRDBMS) *v1.LogicalRestoreRDBMS {
	var current v1.LogicalRestoreRDBMS
	g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(lrr), &current)).To(Succeed())

	return &current
}

// latestOf reads the restore as it is now, outside an Eventually.
func latestOf(lrr *v1.LogicalRestoreRDBMS) *v1.LogicalRestoreRDBMS {
	GinkgoHelper()

	var current *v1.LogicalRestoreRDBMS
	Eventually(func(g Gomega) { current = latest(g, lrr) }, timeout, interval).Should(Succeed())

	return current
}

// clusterSuspended reports spec.suspend of the target as it is now.
func clusterSuspended(g Gomega, w *world) bool {
	var cluster v1.CamundaCluster
	g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(w.cluster), &cluster)).To(Succeed())

	return cluster.Spec.Suspend
}

// holdOf returns the reason of the restore's hold on the target, and whether
// the target carries it.
func holdOf(g Gomega, w *world, lrr *v1.LogicalRestoreRDBMS) (string, bool) {
	var cluster v1.CamundaCluster
	g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(w.cluster), &cluster)).To(Succeed())
	reason, ok := cluster.Annotations[v1.SuspensionHoldPrefix+string(latest(g, lrr).UID)]

	return reason, ok
}

func readyCondition(lrr *v1.LogicalRestoreRDBMS) *metav1.Condition {
	return meta.FindStatusCondition(lrr.Status.Conditions, v1.ConditionReady)
}

// expectPhase waits until the restore reports the given phase.
func expectPhase(lrr *v1.LogicalRestoreRDBMS, phase v1.LogicalRestorePhase) {
	GinkgoHelper()

	Eventually(func(g Gomega) {
		reached := latest(g, lrr)
		g.Expect(reached.Status.Phase).To(Equal(phase), "Ready: %s", readyMessage(reached))
	}, timeout, interval).Should(Succeed())
}

// expectReason waits until the restore reports the given phase and Ready
// reason.
func expectReason(
	lrr *v1.LogicalRestoreRDBMS,
	phase v1.LogicalRestorePhase,
	reason string,
) *v1.LogicalRestoreRDBMS {
	GinkgoHelper()

	var reached *v1.LogicalRestoreRDBMS
	Eventually(func(g Gomega) {
		reached = latest(g, lrr)
		g.Expect(reached.Status.Phase).To(Equal(phase))
		condition := readyCondition(reached)
		g.Expect(condition).NotTo(BeNil())
		g.Expect(condition.Reason).To(Equal(reason), "Ready: %s", readyMessage(reached))
	}, timeout, interval).Should(Succeed())

	return reached
}

// readyMessage renders the Ready condition of a restore for a failed
// assertion.
func readyMessage(lrr *v1.LogicalRestoreRDBMS) string {
	condition := readyCondition(lrr)
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

// holdCluster makes a backup of the world hold the claim on the cluster,
// which is what a restore of that cluster then waits for.
func holdCluster(w *world) *v1.LogicalBackupRDBMS {
	GinkgoHelper()

	backup := &v1.LogicalBackupRDBMS{
		ObjectMeta: metav1.ObjectMeta{
			Name: "holder-" + utilrand.String(6), Namespace: w.namespace,
		},
		Spec: v1.LogicalBackupRDBMSSpec{ClusterRef: v1.ClusterRef{Name: w.cluster.Name}},
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

const denyWritersPolicy = "deny-writer-leases"

// denyWriterLeases makes the API server refuse every new writer Lease, and
// waits until it does.
func denyWriterLeases() {
	GinkgoHelper()
	policy := &admissionregistrationv1.ValidatingAdmissionPolicy{
		ObjectMeta: metav1.ObjectMeta{Name: denyWritersPolicy},
		Spec: admissionregistrationv1.ValidatingAdmissionPolicySpec{
			MatchConstraints: &admissionregistrationv1.MatchResources{
				ResourceRules: []admissionregistrationv1.NamedRuleWithOperations{{
					RuleWithOperations: admissionregistrationv1.RuleWithOperations{
						Operations: []admissionregistrationv1.OperationType{admissionregistrationv1.Create},
						Rule: admissionregistrationv1.Rule{
							APIGroups:   []string{"coordination.k8s.io"},
							APIVersions: []string{"v1"},
							Resources:   []string{"leases"},
						},
					},
				}},
			},
			Validations: []admissionregistrationv1.Validation{{
				Expression: "!has(object.metadata.labels) || !('" + labels.ComponentKey + "' in object.metadata.labels) || " +
					"object.metadata.labels['" + labels.ComponentKey + "'] != '" + storagewriter.Component + "'",
			}},
		},
	}
	binding := &admissionregistrationv1.ValidatingAdmissionPolicyBinding{
		ObjectMeta: metav1.ObjectMeta{Name: denyWritersPolicy},
		Spec: admissionregistrationv1.ValidatingAdmissionPolicyBindingSpec{
			PolicyName:        denyWritersPolicy,
			ValidationActions: []admissionregistrationv1.ValidationAction{admissionregistrationv1.Deny},
		},
	}
	Expect(k8sClient.Create(ctx, policy)).To(Succeed())
	Expect(k8sClient.Create(ctx, binding)).To(Succeed())
	DeferCleanup(allowWriterLeases)

	Eventually(func() bool {
		probe := probeWriterLease()
		err := k8sClient.Create(ctx, probe)
		if err == nil {
			_ = k8sClient.Delete(ctx, probe)
		}

		return apierrors.IsInvalid(err) || apierrors.IsForbidden(err)
	}, timeout, interval).Should(BeTrue(), "the policy refuses writer Leases")
}

// allowWriterLeases removes the policy of denyWriterLeases, and waits until the
// API server accepts writer Leases again.
func allowWriterLeases() {
	GinkgoHelper()
	for _, obj := range []client.Object{
		&admissionregistrationv1.ValidatingAdmissionPolicyBinding{ObjectMeta: metav1.ObjectMeta{Name: denyWritersPolicy}},
		&admissionregistrationv1.ValidatingAdmissionPolicy{ObjectMeta: metav1.ObjectMeta{Name: denyWritersPolicy}},
	} {
		Expect(client.IgnoreNotFound(k8sClient.Delete(ctx, obj))).To(Succeed())
	}
	Eventually(func() error {
		probe := probeWriterLease()
		if err := k8sClient.Create(ctx, probe); err != nil {
			return err
		}

		return k8sClient.Delete(ctx, probe)
	}, timeout, interval).Should(Succeed())
}

func probeWriterLease() *coordinationv1.Lease {
	return &coordinationv1.Lease{ObjectMeta: metav1.ObjectMeta{
		Namespace: claimNamespace,
		Name:      "probe-" + utilrand.String(8),
		Labels:    map[string]string{labels.ComponentKey: storagewriter.Component},
	}}
}

// writersNaming returns the writer Leases in the claim namespace that name the
// restore, as their writer annotation.
func writersNaming(restore *v1.LogicalRestoreRDBMS) []string {
	GinkgoHelper()
	var leases coordinationv1.LeaseList
	Expect(k8sClient.List(
		ctx,
		&leases,
		client.InNamespace(claimNamespace),
		client.MatchingLabels{labels.ComponentKey: storagewriter.Component},
	)).To(Succeed())

	var writers []string
	for i := range leases.Items {
		writer := leases.Items[i].Annotations[storagewriter.WriterAnnotation]
		if writer == "LogicalRestoreRDBMS "+restore.Namespace+"/"+restore.Name {
			writers = append(writers, writer)
		}
	}

	return writers
}

// failRestore fails the pg_restore Job of the restore and waits until the
// restore reports Failed.
func failRestore(w *world, lrr *v1.LogicalRestoreRDBMS) {
	GinkgoHelper()
	var jobName string
	Eventually(func(g Gomega) {
		jobName = latest(g, lrr).Status.SecondaryJobName
		g.Expect(jobName).NotTo(BeEmpty())
	}, timeout, interval).Should(Succeed())

	markJob(w.namespace, jobName, batchv1.JobFailed)
	expectReason(lrr, v1.LogicalRestoreFailed, v1.ReasonFailed)
}

// ageWriterLeases makes every writer Lease of the writer with UID uid read as
// renewed a year ago with a one-second duration.
func ageWriterLeases(uid types.UID) {
	GinkgoHelper()
	old := metav1.NewMicroTime(time.Now().AddDate(-1, 0, 0))
	Eventually(func(g Gomega) {
		var leases coordinationv1.LeaseList
		g.Expect(k8sClient.List(
			ctx,
			&leases,
			client.InNamespace(claimNamespace),
			client.MatchingLabels{labels.WriterUIDKey: string(uid)},
		)).To(Succeed())
		g.Expect(leases.Items).NotTo(BeEmpty())
		for i := range leases.Items {
			lease := &leases.Items[i]
			lease.Spec.AcquireTime = &old
			lease.Spec.RenewTime = &old
			lease.Spec.LeaseDurationSeconds = new(int32(1))
			g.Expect(k8sClient.Update(ctx, lease)).To(Succeed())
		}
	}, timeout, interval).Should(Succeed())
}
