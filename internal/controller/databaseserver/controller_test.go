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

package databaseserver

import (
	cnpgv1 "github.com/cloudnative-pg/api/pkg/api/v1"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	utilrand "k8s.io/apimachinery/pkg/util/rand"
	"sigs.k8s.io/controller-runtime/pkg/client"

	v1 "github.com/konsole-is/camunda-operator/api/v1"
	components "github.com/konsole-is/camunda-operator/pkg/components/databaseserver"
	"github.com/konsole-is/camunda-operator/pkg/wrappers/barmanobjectstore"
)

// serverInNamespace creates a namespace and a minimal DatabaseServer in it,
// and returns the server. The caller drives the CloudNativePG objects.
func serverInNamespace(archive *v1.DatabaseServerArchiveSpec) *v1.DatabaseServer {
	GinkgoHelper()

	namespace := "dbs-" + utilrand.String(8)
	Expect(k8sClient.Create(ctx, &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{Name: namespace},
	})).To(Succeed())

	server := &v1.DatabaseServer{
		ObjectMeta: metav1.ObjectMeta{Name: "camunda", Namespace: namespace},
		Spec: v1.DatabaseServerSpec{
			Version:              "17",
			Instances:            new(int32(1)),
			StorageSize:          new(resource.MustParse("1Gi")),
			DatabaseServerConfig: "camunda",
			Archive:              archive,
		},
	}
	Expect(k8sClient.Create(ctx, server)).To(Succeed())

	return server
}

// makeClusterHealthy writes the status that CloudNativePG reports for a
// healthy cluster, including the system identifier the server mirrors.
func makeClusterHealthy(server *v1.DatabaseServer, systemID string) {
	GinkgoHelper()

	var cluster cnpgv1.Cluster
	key := client.ObjectKey{Namespace: server.Namespace, Name: components.ClusterName(server)}
	Eventually(func() error { return k8sClient.Get(ctx, key, &cluster) }, timeout, interval).Should(Succeed())

	cluster.Status.Phase = cnpgv1.PhaseHealthy
	cluster.Status.ReadyInstances = cluster.Spec.Instances
	cluster.Status.SystemID = systemID
	Expect(k8sClient.Status().Update(ctx, &cluster)).To(Succeed())
}

// writeSuperuserSecret creates the Secret that CloudNativePG writes for the
// superuser of the cluster.
func writeSuperuserSecret(server *v1.DatabaseServer) {
	GinkgoHelper()

	Expect(k8sClient.Create(ctx, &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      components.SuperuserSecretName(server),
			Namespace: server.Namespace,
		},
		Data: map[string][]byte{"username": []byte("postgres"), "password": []byte("s3cret")},
	})).To(Succeed())
}

// completeBaseBackup creates a completed Backup of the cluster, as the
// CloudNativePG operator would after the ScheduledBackup fired.
func completeBaseBackup(server *v1.DatabaseServer, at metav1.Time) {
	GinkgoHelper()

	backup := &cnpgv1.Backup{
		ObjectMeta: metav1.ObjectMeta{
			Name:      components.ClusterName(server) + "-base",
			Namespace: server.Namespace,
			Labels:    map[string]string{components.CNPGClusterNameLabel: components.ClusterName(server)},
		},
		Spec: cnpgv1.BackupSpec{
			Cluster: cnpgv1.LocalObjectReference{Name: components.ClusterName(server)},
			Method:  cnpgv1.BackupMethodPlugin,
		},
	}
	Expect(k8sClient.Create(ctx, backup)).To(Succeed())

	backup.Status.Phase = cnpgv1.BackupPhaseCompleted
	backup.Status.StoppedAt = &at
	Expect(k8sClient.Status().Update(ctx, backup)).To(Succeed())
}

// conditionOf reads one condition of the reconciled server.
func conditionOf(server *v1.DatabaseServer, conditionType string) *metav1.Condition {
	GinkgoHelper()

	var latest v1.DatabaseServer
	Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(server), &latest)).To(Succeed())

	return meta.FindStatusCondition(latest.Status.Conditions, conditionType)
}

// expectCondition waits until the named condition of the server carries the
// given status, and returns it.
func expectCondition(
	server *v1.DatabaseServer,
	conditionType string,
	status metav1.ConditionStatus,
) *metav1.Condition {
	GinkgoHelper()

	Eventually(func(g Gomega) {
		condition := conditionOf(server, conditionType)
		g.Expect(condition).NotTo(BeNil(), conditionType)
		g.Expect(condition.Status).To(Equal(status), conditionType+": "+condition.Message)
	}, timeout, interval).Should(Succeed())

	return conditionOf(server, conditionType)
}

// archiveBucket creates a cluster-scoped bucket contract with static
// credentials and the Secret those credentials live in.
func archiveBucket() *v1.ObjectStorageConfig {
	GinkgoHelper()

	name := "bucket-" + utilrand.String(8)
	Expect(k8sClient.Create(ctx, &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: "default"},
		Data: map[string][]byte{
			"accessKeyId":     []byte("minio-root"),
			"secretAccessKey": []byte("minio-secret"),
		},
	})).To(Succeed())

	bucket := &v1.ObjectStorageConfig{
		ObjectMeta: metav1.ObjectMeta{Name: name},
		Spec: v1.ObjectStorageConfigSpec{
			Type: v1.ObjectStorageTypeS3,
			S3: &v1.S3Storage{
				BucketName: "camunda-backups",
				BasePath:   "clusters",
				Region:     "eu-west-1",
				Auth: v1.S3StorageAuth{
					Type: v1.ObjectStorageAuthTypeCredentials,
					Credentials: &v1.S3Credentials{
						SecretRef: v1.S3CredentialsSecretRef{
							Name:               name,
							Namespace:          "default",
							AccessKeyIDKey:     "accessKeyId",
							SecretAccessKeyKey: "secretAccessKey",
						},
					},
				},
			},
		},
	}
	Expect(k8sClient.Create(ctx, bucket)).To(Succeed())
	DeferCleanup(func() { _ = k8sClient.Delete(ctx, bucket) })

	return bucket
}

var _ = Describe("DatabaseServer controller", func() {
	It("runs a server without an archive and publishes its contract", func() {
		server := serverInNamespace(nil)
		writeSuperuserSecret(server)
		makeClusterHealthy(server, "7000000000000000001")

		expectCondition(server, components.ConditionCluster, metav1.ConditionTrue)
		expectCondition(server, components.ConditionContract, metav1.ConditionTrue)

		var contract v1.DatabaseServerConfig
		Expect(k8sClient.Get(
			ctx, client.ObjectKey{Namespace: server.Namespace, Name: "camunda"}, &contract,
		)).To(Succeed())
		Expect(contract.Spec.Host).To(Equal("camunda-rw." + server.Namespace + ".svc"))
		Expect(contract.Spec.Port).To(Equal(int32(5432)))
		Expect(contract.Spec.AdminCredentialsSecretRef.Name).To(Equal("camunda-superuser"))
		Expect(contract.Spec.PITR).NotTo(BeNil())
		Expect(contract.Spec.PITR.Enabled).To(BeFalse())

		// A server with no archive block has nothing to archive, so the
		// condition reports the component as disabled rather than failing.
		archive := expectCondition(server, components.ConditionArchive, metav1.ConditionTrue)
		Expect(archive.Reason).To(Equal("Disabled"))

		expectCondition(server, v1.ConditionReady, metav1.ConditionTrue)
	})

	It("mirrors the system identifier and the cluster CloudNativePG reports", func() {
		server := serverInNamespace(nil)
		writeSuperuserSecret(server)
		makeClusterHealthy(server, "7000000000000000042")

		Eventually(func(g Gomega) {
			var latest v1.DatabaseServer
			g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(server), &latest)).To(Succeed())
			g.Expect(latest.Status.Cluster).To(Equal("camunda"))
			g.Expect(latest.Status.SystemIdentifier).To(Equal("7000000000000000042"))
			g.Expect(latest.Status.ObservedGeneration).To(Equal(latest.Generation))
		}, timeout, interval).Should(Succeed())
	})

	It("holds the contract until CloudNativePG has written the superuser Secret", func() {
		server := serverInNamespace(nil)

		contract := expectCondition(server, components.ConditionContract, metav1.ConditionFalse)
		Expect(contract.Message).To(ContainSubstring("camunda-superuser"))

		var published v1.DatabaseServerConfig
		err := k8sClient.Get(ctx, client.ObjectKey{Namespace: server.Namespace, Name: "camunda"}, &published)
		Expect(err).To(HaveOccurred(), "no contract before the credentials exist")

		writeSuperuserSecret(server)
		expectCondition(server, components.ConditionContract, metav1.ConditionTrue)
	})

	It("archives to a bucket and reports the archive ready after the first base backup", func() {
		bucket := archiveBucket()
		server := serverInNamespace(&v1.DatabaseServerArchiveSpec{
			ObjectStorageRef:    bucket.Name,
			RetentionPeriodDays: 30,
			BaseBackupSchedule:  components.DefaultBaseBackupSchedule,
		})
		writeSuperuserSecret(server)
		makeClusterHealthy(server, "7000000000000000002")

		var store barmanobjectstore.ObjectStore
		Eventually(func() error {
			return k8sClient.Get(
				ctx, client.ObjectKey{Namespace: server.Namespace, Name: "camunda"}, &store,
			)
		}, timeout, interval).Should(Succeed())
		Expect(store.Spec.Configuration.DestinationPath).To(Equal(
			"s3://camunda-backups/clusters/databaseserver/" + server.Namespace + "/camunda",
		))
		Expect(store.Spec.RetentionPolicy).To(Equal("30d"))

		var schedule cnpgv1.ScheduledBackup
		Eventually(func() error {
			return k8sClient.Get(
				ctx, client.ObjectKey{Namespace: server.Namespace, Name: "camunda"}, &schedule,
			)
		}, timeout, interval).Should(Succeed())
		Expect(schedule.Spec.Method).To(Equal(cnpgv1.BackupMethodPlugin))
		Expect(schedule.Spec.Immediate).To(HaveValue(BeTrue()))

		var cluster cnpgv1.Cluster
		Expect(k8sClient.Get(
			ctx, client.ObjectKey{Namespace: server.Namespace, Name: "camunda"}, &cluster,
		)).To(Succeed())
		Expect(cluster.Spec.Plugins).To(HaveLen(1))
		Expect(cluster.Spec.Plugins[0].Parameters).To(HaveKeyWithValue("serverName", "camunda"))

		blocked := expectCondition(server, components.ConditionArchive, metav1.ConditionFalse)
		Expect(blocked.Message).To(ContainSubstring("base backup"))

		completedAt := metav1.NewTime(metav1.Now().Rfc3339Copy().Time)
		completeBaseBackup(server, completedAt)

		expectCondition(server, components.ConditionArchive, metav1.ConditionTrue)

		var contract v1.DatabaseServerConfig
		Expect(k8sClient.Get(
			ctx, client.ObjectKey{Namespace: server.Namespace, Name: "camunda"}, &contract,
		)).To(Succeed())
		Expect(contract.Spec.PITR.Enabled).To(BeTrue())
		Expect(contract.Spec.PITR.RetentionPeriodDays).To(HaveValue(Equal(int32(30))))

		// The archive interval opens at the first base backup: nothing before
		// it can be recovered to.
		Eventually(func(g Gomega) {
			var latest v1.DatabaseServer
			g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(server), &latest)).To(Succeed())
			g.Expect(latest.Status.Archive).NotTo(BeNil())
			g.Expect(latest.Status.Archive.History).To(HaveLen(1))
			g.Expect(latest.Status.Archive.History[0].ServerName).To(Equal("camunda"))
			g.Expect(latest.Status.Archive.History[0].From.Time).To(BeTemporally("==", completedAt.Time))
			g.Expect(latest.Status.Archive.History[0].To).To(BeNil())
		}, timeout, interval).Should(Succeed())
	})

	It("reports InvalidReference for a preset that does not exist", func() {
		server := serverInNamespace(nil)

		Eventually(func(g Gomega) {
			var latest v1.DatabaseServer
			g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(server), &latest)).To(Succeed())
			latest.Spec.PresetRef = "no-such-preset"
			g.Expect(k8sClient.Update(ctx, &latest)).To(Succeed())
		}, timeout, interval).Should(Succeed())

		Eventually(func(g Gomega) {
			ready := conditionOf(server, v1.ConditionReady)
			g.Expect(ready).NotTo(BeNil())
			g.Expect(ready.Reason).To(Equal(v1.ReasonInvalidReference))
			g.Expect(ready.Message).To(ContainSubstring("no-such-preset"))
		}, timeout, interval).Should(Succeed())
	})

	It("reports InvalidReference for a bucket that does not exist", func() {
		server := serverInNamespace(&v1.DatabaseServerArchiveSpec{
			ObjectStorageRef:    "no-such-bucket",
			RetentionPeriodDays: 30,
		})

		Eventually(func(g Gomega) {
			ready := conditionOf(server, v1.ConditionReady)
			g.Expect(ready).NotTo(BeNil())
			g.Expect(ready.Reason).To(Equal(v1.ReasonInvalidReference))
			g.Expect(ready.Message).To(ContainSubstring("no-such-bucket"))
		}, timeout, interval).Should(Succeed())
	})

	It("hibernates the cluster while spec.suspend is true", func() {
		server := serverInNamespace(nil)
		writeSuperuserSecret(server)
		makeClusterHealthy(server, "7000000000000000003")
		expectCondition(server, components.ConditionCluster, metav1.ConditionTrue)

		var cluster cnpgv1.Cluster
		key := client.ObjectKey{Namespace: server.Namespace, Name: "camunda"}
		Expect(k8sClient.Get(ctx, key, &cluster)).To(Succeed())
		Expect(cluster.Annotations).To(HaveKeyWithValue("cnpg.io/hibernation", "off"))

		Eventually(func(g Gomega) {
			var latest v1.DatabaseServer
			g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(server), &latest)).To(Succeed())
			latest.Spec.Suspend = true
			g.Expect(k8sClient.Update(ctx, &latest)).To(Succeed())
		}, timeout, interval).Should(Succeed())

		Eventually(func(g Gomega) {
			g.Expect(k8sClient.Get(ctx, key, &cluster)).To(Succeed())
			g.Expect(cluster.Annotations).To(HaveKeyWithValue("cnpg.io/hibernation", "on"))
		}, timeout, interval).Should(Succeed())

		// CloudNativePG reports the hibernation condition only once every pod
		// is gone, so until the spec writes it the component is still
		// suspending.
		Eventually(func(g Gomega) {
			condition := conditionOf(server, components.ConditionCluster)
			g.Expect(condition).NotTo(BeNil())
			g.Expect(condition.Reason).To(Equal("Suspending"))
		}, timeout, interval).Should(Succeed())

		meta.SetStatusCondition(&cluster.Status.Conditions, metav1.Condition{
			Type:    "cnpg.io/hibernation",
			Status:  metav1.ConditionTrue,
			Reason:  "Hibernated",
			Message: "Cluster hibernated",
		})
		Expect(k8sClient.Status().Update(ctx, &cluster)).To(Succeed())

		Eventually(func(g Gomega) {
			condition := conditionOf(server, components.ConditionCluster)
			g.Expect(condition).NotTo(BeNil())
			g.Expect(condition.Reason).To(Equal("Suspended"))
		}, timeout, interval).Should(Succeed())
	})

	// The control plane of this suite serves no prometheus-operator CRDs,
	// which is the cluster a user without Prometheus has. Enabling scraping
	// there must leave the server running rather than fail every reconcile
	// against a kind the API server does not know.
	It("stays ready with scraping enabled on a cluster without the PodMonitor kind", func() {
		server := serverInNamespace(nil)
		writeSuperuserSecret(server)
		makeClusterHealthy(server, "7000000000000000004")

		disabled := expectCondition(server, components.ConditionMonitoring, metav1.ConditionTrue)
		Expect(disabled.Reason).To(Equal("Disabled"))

		Eventually(func(g Gomega) {
			var latest v1.DatabaseServer
			g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(server), &latest)).To(Succeed())
			latest.Spec.Monitoring = &v1.DatabaseServerMonitoringSpec{
				PodMonitor: &v1.PodMonitorSpec{Enabled: true},
			}
			g.Expect(k8sClient.Update(ctx, &latest)).To(Succeed())
		}, timeout, interval).Should(Succeed())

		Eventually(func(g Gomega) {
			var latest v1.DatabaseServer
			g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(server), &latest)).To(Succeed())
			g.Expect(latest.Status.ObservedGeneration).To(Equal(latest.Generation))
		}, timeout, interval).Should(Succeed())

		expectCondition(server, components.ConditionMonitoring, metav1.ConditionTrue)
		expectCondition(server, v1.ConditionReady, metav1.ConditionTrue)
	})
})
