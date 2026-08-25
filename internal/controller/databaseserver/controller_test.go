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
	"time"

	cnpgv1 "github.com/cloudnative-pg/api/pkg/api/v1"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/sourcehawk/operator-component-framework/pkg/component"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
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

// serverOnPreset creates a namespace, a cluster-scoped preset that holds the
// version and the storage size, and a DatabaseServer that inherits both. It
// returns the server and the preset.
func serverOnPreset(storageSize string) (*v1.DatabaseServer, *v1.DatabaseServerPreset) {
	GinkgoHelper()

	preset := &v1.DatabaseServerPreset{
		ObjectMeta: metav1.ObjectMeta{Name: "dbsp-" + utilrand.String(8)},
		Spec: v1.DatabaseServerPresetSpec{
			Server: v1.DatabaseServerSpec{
				Version:     "17",
				Instances:   new(int32(1)),
				StorageSize: new(resource.MustParse(storageSize)),
			},
		},
	}
	Expect(k8sClient.Create(ctx, preset)).To(Succeed())
	DeferCleanup(func() { _ = k8sClient.Delete(ctx, preset) })

	namespace := "dbs-" + utilrand.String(8)
	Expect(k8sClient.Create(ctx, &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{Name: namespace},
	})).To(Succeed())

	server := &v1.DatabaseServer{
		ObjectMeta: metav1.ObjectMeta{Name: "camunda", Namespace: namespace},
		Spec: v1.DatabaseServerSpec{
			PresetRef:            preset.Name,
			DatabaseServerConfig: "camunda",
		},
	}
	Expect(k8sClient.Create(ctx, server)).To(Succeed())

	return server, preset
}

// setPresetStorage puts storageSize on the latest revision of the preset.
func setPresetStorage(preset *v1.DatabaseServerPreset, storageSize string) {
	GinkgoHelper()

	Eventually(func(g Gomega) {
		var latest v1.DatabaseServerPreset
		g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(preset), &latest)).To(Succeed())
		latest.Spec.Server.StorageSize = new(resource.MustParse(storageSize))
		g.Expect(k8sClient.Update(ctx, &latest)).To(Succeed())
	}, timeout, interval).Should(Succeed())
}

// countEvents counts the events of one reason that the server carries.
func countEvents(g Gomega, server *v1.DatabaseServer, reason string) int32 {
	GinkgoHelper()

	var events corev1.EventList
	g.Expect(k8sClient.List(ctx, &events, client.InNamespace(server.Namespace))).To(Succeed())

	var count int32
	for _, event := range events.Items {
		if event.Reason == reason && event.InvolvedObject.Name == server.Name {
			count += max(event.Count, 1)
		}
	}

	return count
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
func completeBaseBackup(server *v1.DatabaseServer, name string, at metav1.Time) {
	GinkgoHelper()

	backup := &cnpgv1.Backup{
		ObjectMeta: metav1.ObjectMeta{
			Name:      components.ClusterName(server) + "-" + name,
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

// suspend asks the server to stop. Its instances stay up until CloudNativePG
// confirms the hibernation, which hibernate writes.
func suspend(server *v1.DatabaseServer) {
	GinkgoHelper()

	Eventually(func(g Gomega) {
		var latest v1.DatabaseServer
		g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(server), &latest)).To(Succeed())
		latest.Spec.Suspend = true
		g.Expect(k8sClient.Update(ctx, &latest)).To(Succeed())
	}, timeout, interval).Should(Succeed())
}

// hibernate writes the hibernation condition that CloudNativePG reports once
// every instance pod of the cluster is gone. It waits for the operator to ask
// for the hibernation first.
func hibernate(server *v1.DatabaseServer) {
	GinkgoHelper()

	key := client.ObjectKey{Namespace: server.Namespace, Name: components.ClusterName(server)}
	Eventually(func(g Gomega) {
		var cluster cnpgv1.Cluster
		g.Expect(k8sClient.Get(ctx, key, &cluster)).To(Succeed())
		g.Expect(cluster.Annotations).To(HaveKeyWithValue("cnpg.io/hibernation", "on"))
		meta.SetStatusCondition(&cluster.Status.Conditions, metav1.Condition{
			Type:    "cnpg.io/hibernation",
			Status:  metav1.ConditionTrue,
			Reason:  "Hibernated",
			Message: "Cluster hibernated",
		})
		g.Expect(k8sClient.Status().Update(ctx, &cluster)).To(Succeed())
	}, timeout, interval).Should(Succeed())
}

// setArchive puts archive on the latest revision of the server.
func setArchive(server *v1.DatabaseServer, archive *v1.DatabaseServerArchiveSpec) {
	GinkgoHelper()

	Eventually(func(g Gomega) {
		var latest v1.DatabaseServer
		g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(server), &latest)).To(Succeed())
		latest.Spec.Archive = archive
		g.Expect(k8sClient.Update(ctx, &latest)).To(Succeed())
	}, timeout, interval).Should(Succeed())
}

// archiveHistory reads the archive history of the reconciled server.
func archiveHistory(server *v1.DatabaseServer) []v1.ArchiveRecord {
	GinkgoHelper()

	var latest v1.DatabaseServer
	Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(server), &latest)).To(Succeed())
	if latest.Status.Archive == nil {
		return nil
	}

	return latest.Status.Archive.History
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

		expectCondition(server, v1.ConditionClusterReady, metav1.ConditionTrue)
		expectCondition(server, v1.ConditionContractReady, metav1.ConditionTrue)

		var contract v1.DatabaseServerConfig
		Expect(k8sClient.Get(
			ctx, client.ObjectKey{Namespace: server.Namespace, Name: "camunda"}, &contract,
		)).To(Succeed())
		Expect(contract.Spec.Host).To(Equal("camunda-rw." + server.Namespace + ".svc"))
		Expect(contract.Spec.Port).To(Equal(int32(5432)))
		Expect(contract.Spec.AdminCredentialsSecretRef.Name).To(Equal("camunda-superuser"))
		Expect(contract.Spec.PITR).NotTo(BeNil())
		Expect(contract.Spec.PITR.Enabled).To(BeFalse())

		// The typed read cannot tell an absent enabled from a false one, and
		// the CRD doc promises the published contract reads
		// pitr.enabled: false. The schema default is what puts it there.
		var raw unstructured.Unstructured
		raw.SetGroupVersionKind(v1.GroupVersion.WithKind("DatabaseServerConfig"))
		Expect(k8sClient.Get(
			ctx, client.ObjectKey{Namespace: server.Namespace, Name: "camunda"}, &raw,
		)).To(Succeed())
		pitr, found, err := unstructured.NestedMap(raw.Object, "spec", "pitr")
		Expect(err).NotTo(HaveOccurred())
		Expect(found).To(BeTrue())
		Expect(pitr).To(Equal(map[string]any{"enabled": false, "recovery": "external"}))

		// A server with no archive block has nothing to archive, so the
		// condition reports the component as disabled rather than failing.
		archive := expectCondition(server, v1.ConditionArchiveReady, metav1.ConditionTrue)
		Expect(archive.Reason).To(Equal("Disabled"))

		// A part the spec switched off is reported on its own condition and
		// never on Ready. The reason a reader sees for a server that runs is
		// Healthy, whether or not it archives and whether or not it scrapes.
		monitoring := expectCondition(server, v1.ConditionMonitoringReady, metav1.ConditionTrue)
		Expect(monitoring.Reason).To(Equal("Disabled"))
		ready := expectCondition(server, v1.ConditionReady, metav1.ConditionTrue)
		Expect(ready.Reason).To(Equal(v1.ReasonHealthy), ready.Message)

		// A server with no archive takes no base backups, so reading them
		// every reconcile would be a cluster-wide read for nothing.
		Expect(backupLists.countIn(server.Namespace)).To(BeZero())
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

		contract := expectCondition(server, v1.ConditionContractReady, metav1.ConditionFalse)
		Expect(contract.Reason).To(Equal("Blocked"))
		Expect(contract.Message).To(ContainSubstring("camunda-superuser"))

		var published v1.DatabaseServerConfig
		err := k8sClient.Get(ctx, client.ObjectKey{Namespace: server.Namespace, Name: "camunda"}, &published)
		Expect(err).To(HaveOccurred(), "no contract before the credentials exist")

		writeSuperuserSecret(server)
		expectCondition(server, v1.ConditionContractReady, metav1.ConditionTrue)
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

		blocked := expectCondition(server, v1.ConditionArchiveReady, metav1.ConditionFalse)
		Expect(blocked.Message).To(ContainSubstring("base backup"))

		// An archive the spec asks for takes part in Ready, so a server that
		// cannot be recovered to any point yet is not ready.
		held := expectCondition(server, v1.ConditionReady, metav1.ConditionFalse)
		Expect(held.Reason).To(Equal(string(component.GuardBlocked)), held.Message)

		completedAt := metav1.NewTime(metav1.Now().Rfc3339Copy().Time)
		completeBaseBackup(server, "base", completedAt)

		expectCondition(server, v1.ConditionArchiveReady, metav1.ConditionTrue)

		// Scraping is off here, and a part the spec switched off is reported
		// on its own condition and never on Ready.
		ready := expectCondition(server, v1.ConditionReady, metav1.ConditionTrue)
		Expect(ready.Reason).To(Equal(v1.ReasonHealthy), ready.Message)

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

		Expect(backupLists.countIn(server.Namespace)).To(BeNumerically(">", 0))
	})

	It("closes the archive record when the archive block is removed", func() {
		bucket := archiveBucket()
		server := serverInNamespace(&v1.DatabaseServerArchiveSpec{
			ObjectStorageRef:    bucket.Name,
			RetentionPeriodDays: 30,
		})
		writeSuperuserSecret(server)
		makeClusterHealthy(server, "7000000000000000005")
		completeBaseBackup(server, "base", metav1.NewTime(metav1.Now().Rfc3339Copy().Time))

		expectCondition(server, v1.ConditionArchiveReady, metav1.ConditionTrue)
		Eventually(func(g Gomega) {
			var latest v1.DatabaseServer
			g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(server), &latest)).To(Succeed())
			g.Expect(latest.Status.Archive).NotTo(BeNil())
			g.Expect(latest.Status.Archive.History).To(HaveLen(1))
		}, timeout, interval).Should(Succeed())

		Eventually(func(g Gomega) {
			var latest v1.DatabaseServer
			g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(server), &latest)).To(Succeed())
			latest.Spec.Archive = nil
			g.Expect(k8sClient.Update(ctx, &latest)).To(Succeed())
		}, timeout, interval).Should(Succeed())

		// The record the server was writing closes, the list keeps it, and no
		// record is written again: the bucket still holds what it wrote.
		Eventually(func(g Gomega) {
			var latest v1.DatabaseServer
			g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(server), &latest)).To(Succeed())
			g.Expect(latest.Status.Archive).NotTo(BeNil())
			g.Expect(latest.Status.Archive.History).To(HaveLen(1))
			g.Expect(latest.Status.Archive.History[0].To).NotTo(BeNil())
		}, timeout, interval).Should(Succeed())

		disabled := expectCondition(server, v1.ConditionArchiveReady, metav1.ConditionTrue)
		Expect(disabled.Reason).To(Equal("Disabled"))

		Eventually(func(g Gomega) {
			var contract v1.DatabaseServerConfig
			g.Expect(k8sClient.Get(
				ctx, client.ObjectKey{Namespace: server.Namespace, Name: "camunda"}, &contract,
			)).To(Succeed())
			g.Expect(contract.Spec.PITR).NotTo(BeNil())
			g.Expect(contract.Spec.PITR.Enabled).To(BeFalse())
		}, timeout, interval).Should(Succeed())

		// A closed record stays closed, and nothing reopens it.
		closed := archiveHistory(server)
		Consistently(func(g Gomega) {
			g.Expect(archiveHistory(server)).To(Equal(closed))
		}, time.Second, interval).Should(Succeed())
	})

	It("keeps the archive on the cluster while the server is suspended", func() {
		bucket := archiveBucket()
		server := serverInNamespace(&v1.DatabaseServerArchiveSpec{
			ObjectStorageRef:    bucket.Name,
			RetentionPeriodDays: 30,
		})
		writeSuperuserSecret(server)
		makeClusterHealthy(server, "7000000000000000006")

		archiveKey := client.ObjectKey{Namespace: server.Namespace, Name: "camunda-archive"}
		clusterKey := client.ObjectKey{Namespace: server.Namespace, Name: "camunda"}

		Eventually(func(g Gomega) {
			var cluster cnpgv1.Cluster
			g.Expect(k8sClient.Get(ctx, clusterKey, &cluster)).To(Succeed())
			g.Expect(cluster.Spec.Plugins).To(HaveLen(1))
			g.Expect(k8sClient.Get(ctx, archiveKey, &corev1.Secret{})).To(Succeed())
		}, timeout, interval).Should(Succeed())

		suspend(server)

		Eventually(func(g Gomega) {
			var cluster cnpgv1.Cluster
			g.Expect(k8sClient.Get(ctx, clusterKey, &cluster)).To(Succeed())
			g.Expect(cluster.Annotations).To(HaveKeyWithValue("cnpg.io/hibernation", "on"))
		}, timeout, interval).Should(Succeed())

		// The last write-ahead log segments archive while the instances go
		// away, so the plugin entry, the bucket settings, and the ObjectStore
		// all have to outlive the suspension.
		Consistently(func(g Gomega) {
			var cluster cnpgv1.Cluster
			g.Expect(k8sClient.Get(ctx, clusterKey, &cluster)).To(Succeed())
			g.Expect(cluster.Spec.Plugins).To(HaveLen(1))
			g.Expect(cluster.Spec.Plugins[0].Parameters).To(HaveKeyWithValue("serverName", "camunda"))
			g.Expect(k8sClient.Get(ctx, archiveKey, &corev1.Secret{})).To(Succeed())
			g.Expect(k8sClient.Get(ctx, clusterKey, &barmanobjectstore.ObjectStore{})).To(Succeed())
		}, 2*time.Second, interval).Should(Succeed())

		// The instances are gone, so a slot the schedule reaches would start a
		// backup that cannot run.
		Eventually(func(g Gomega) {
			var schedule cnpgv1.ScheduledBackup
			g.Expect(k8sClient.Get(ctx, clusterKey, &schedule)).To(Succeed())
			g.Expect(schedule.Spec.Suspend).To(HaveValue(BeTrue()))
		}, timeout, interval).Should(Succeed())

		// This server was suspended before its first base backup. That backup
		// can never complete now, so waiting on it would hold the server at
		// not ready for as long as the suspension lasts.
		hibernate(server)
		expectCondition(server, v1.ConditionArchiveReady, metav1.ConditionTrue)
		ready := expectCondition(server, v1.ConditionReady, metav1.ConditionTrue)
		Expect(ready.Reason).To(Equal(string(component.Suspended)))
	})

	// The suspension a server reached is what makes its bucket stop mattering.
	// The suspension it merely asks for does not: its instances are still up,
	// and holding the reconcile there would leave them running under a Ready
	// that claims otherwise.
	It("reports a bucket that goes away before the instances are down", func() {
		bucket := archiveBucket()
		server := serverInNamespace(&v1.DatabaseServerArchiveSpec{
			ObjectStorageRef:    bucket.Name,
			RetentionPeriodDays: 30,
		})
		writeSuperuserSecret(server)
		makeClusterHealthy(server, "7000000000000000008")
		expectCondition(server, v1.ConditionClusterReady, metav1.ConditionTrue)

		suspend(server)
		Expect(k8sClient.Delete(ctx, bucket)).To(Succeed())

		Eventually(func(g Gomega) {
			ready := conditionOf(server, v1.ConditionReady)
			g.Expect(ready).NotTo(BeNil())
			g.Expect(ready.Status).To(Equal(metav1.ConditionFalse))
			g.Expect(ready.Reason).To(Equal(v1.ReasonInvalidReference))
			g.Expect(ready.Message).To(ContainSubstring(bucket.Name))
		}, timeout, interval).Should(Succeed())
	})

	It("keeps Suspended when the bucket goes away after the instances are down", func() {
		bucket := archiveBucket()
		server := serverInNamespace(&v1.DatabaseServerArchiveSpec{
			ObjectStorageRef:    bucket.Name,
			RetentionPeriodDays: 30,
		})
		writeSuperuserSecret(server)
		makeClusterHealthy(server, "7000000000000000009")

		suspend(server)
		hibernate(server)
		ready := expectCondition(server, v1.ConditionReady, metav1.ConditionTrue)
		Expect(ready.Reason).To(Equal(string(component.Suspended)))

		Expect(k8sClient.Delete(ctx, bucket)).To(Succeed())

		// The server runs nothing, so the bucket matters again only when it
		// comes back. Nothing is rewritten in the meantime.
		clusterKey := client.ObjectKey{Namespace: server.Namespace, Name: "camunda"}
		Consistently(func(g Gomega) {
			held := conditionOf(server, v1.ConditionReady)
			g.Expect(held).NotTo(BeNil())
			g.Expect(held.Status).To(Equal(metav1.ConditionTrue))
			g.Expect(held.Reason).To(Equal(string(component.Suspended)))
			g.Expect(k8sClient.Get(ctx, clusterKey, &barmanobjectstore.ObjectStore{})).To(Succeed())
			g.Expect(k8sClient.Get(
				ctx, client.ObjectKey{Namespace: server.Namespace, Name: "camunda-archive"}, &corev1.Secret{},
			)).To(Succeed())
		}, 2*time.Second, interval).Should(Succeed())
	})

	It("starts a new archive record when the archive comes back", func() {
		bucket := archiveBucket()
		archive := &v1.DatabaseServerArchiveSpec{
			ObjectStorageRef:    bucket.Name,
			RetentionPeriodDays: 30,
		}
		server := serverInNamespace(archive)
		writeSuperuserSecret(server)
		makeClusterHealthy(server, "7000000000000000007")
		completeBaseBackup(server, "first", metav1.NewTime(metav1.Now().Rfc3339Copy().Time))

		expectCondition(server, v1.ConditionArchiveReady, metav1.ConditionTrue)
		Eventually(func(g Gomega) {
			g.Expect(archiveHistory(server)).To(HaveLen(1))
		}, timeout, interval).Should(Succeed())

		setArchive(server, nil)
		Eventually(func(g Gomega) {
			history := archiveHistory(server)
			g.Expect(history).To(HaveLen(1))
			g.Expect(history[0].To).NotTo(BeNil())
		}, timeout, interval).Should(Succeed())
		closedAt := *archiveHistory(server)[0].To

		setArchive(server, archive)

		// The backups of the archive the server wrote before reach no point in
		// the new one, so the archive is not recoverable again until a base
		// backup of the new one completes.
		blocked := expectCondition(server, v1.ConditionArchiveReady, metav1.ConditionFalse)
		Expect(blocked.Message).To(ContainSubstring("base backup"))
		Consistently(func(g Gomega) {
			g.Expect(archiveHistory(server)).To(HaveLen(1))
		}, time.Second, interval).Should(Succeed())

		reopenedAt := metav1.NewTime(closedAt.Add(5 * time.Second))
		completeBaseBackup(server, "second", reopenedAt)

		expectCondition(server, v1.ConditionArchiveReady, metav1.ConditionTrue)
		Eventually(func(g Gomega) {
			history := archiveHistory(server)
			g.Expect(history).To(HaveLen(2))
			g.Expect(history[0].To).NotTo(BeNil())
			g.Expect(history[1].ServerName).To(Equal("camunda"))
			g.Expect(history[1].From.Time).To(BeTemporally("==", reopenedAt.Time))
			g.Expect(history[1].To).To(BeNil())
		}, timeout, interval).Should(Succeed())
	})

	// A server that never converged has no honest condition to keep, so a
	// dangling reference under it must be reported rather than tolerated.
	It("reports a dangling bucket on a server created suspended", func() {
		server := serverInNamespace(&v1.DatabaseServerArchiveSpec{
			ObjectStorageRef:    "no-such-bucket",
			RetentionPeriodDays: 30,
		})
		Eventually(func(g Gomega) {
			var latest v1.DatabaseServer
			g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(server), &latest)).To(Succeed())
			latest.Spec.Suspend = true
			g.Expect(k8sClient.Update(ctx, &latest)).To(Succeed())
		}, timeout, interval).Should(Succeed())

		Eventually(func(g Gomega) {
			ready := conditionOf(server, v1.ConditionReady)
			g.Expect(ready).NotTo(BeNil())
			g.Expect(ready.Status).To(Equal(metav1.ConditionFalse))
			g.Expect(ready.Reason).To(Equal(v1.ReasonInvalidReference))
			g.Expect(ready.Message).To(ContainSubstring("no-such-bucket"))
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

	// Admission cannot catch this shrink. The CEL transition rules bind the
	// spec of the server, and lowering a preset never touches it.
	It("keeps the applied storage size when a preset lowers it", func() {
		server, preset := serverOnPreset("10Gi")
		writeSuperuserSecret(server)
		makeClusterHealthy(server, "7000000000000000009")

		clusterKey := client.ObjectKey{Namespace: server.Namespace, Name: "camunda"}
		Eventually(func(g Gomega) {
			var cluster cnpgv1.Cluster
			g.Expect(k8sClient.Get(ctx, clusterKey, &cluster)).To(Succeed())
			g.Expect(cluster.Spec.StorageConfiguration.Size).To(Equal("10Gi"))
		}, timeout, interval).Should(Succeed())

		setPresetStorage(preset, "1Gi")

		Eventually(func(g Gomega) {
			g.Expect(countEvents(g, server, "StorageShrinkIgnored")).To(Equal(int32(1)))
		}, timeout, interval).Should(Succeed())

		// CloudNativePG refuses a cluster whose storage is smaller than the
		// one it applied, so a server that let the smaller size through would
		// stop converging.
		Consistently(func(g Gomega) {
			var cluster cnpgv1.Cluster
			g.Expect(k8sClient.Get(ctx, clusterKey, &cluster)).To(Succeed())
			g.Expect(cluster.Spec.StorageConfiguration.Size).To(Equal("10Gi"))
			g.Expect(countEvents(g, server, "StorageShrinkIgnored")).To(Equal(int32(1)))
		}, 2*time.Second, interval).Should(Succeed())

		ready := conditionOf(server, v1.ConditionReady)
		Expect(ready).NotTo(BeNil())
		Expect(ready.Reason).NotTo(Equal(v1.ReasonInvalidReference))
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
		expectCondition(server, v1.ConditionClusterReady, metav1.ConditionTrue)

		var cluster cnpgv1.Cluster
		key := client.ObjectKey{Namespace: server.Namespace, Name: "camunda"}
		Expect(k8sClient.Get(ctx, key, &cluster)).To(Succeed())
		Expect(cluster.Annotations).To(HaveKeyWithValue("cnpg.io/hibernation", "off"))

		suspend(server)

		Eventually(func(g Gomega) {
			g.Expect(k8sClient.Get(ctx, key, &cluster)).To(Succeed())
			g.Expect(cluster.Annotations).To(HaveKeyWithValue("cnpg.io/hibernation", "on"))
		}, timeout, interval).Should(Succeed())

		// CloudNativePG reports the hibernation condition only once every pod
		// is gone, so until the spec writes it the component is still
		// suspending.
		Eventually(func(g Gomega) {
			condition := conditionOf(server, v1.ConditionClusterReady)
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
			condition := conditionOf(server, v1.ConditionClusterReady)
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

		disabled := expectCondition(server, v1.ConditionMonitoringReady, metav1.ConditionTrue)
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

		expectCondition(server, v1.ConditionMonitoringReady, metav1.ConditionTrue)
		expectCondition(server, v1.ConditionReady, metav1.ConditionTrue)
	})
})
