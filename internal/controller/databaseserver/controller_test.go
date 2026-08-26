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
	"strconv"
	"strings"
	"time"

	cnpgv1 "github.com/cloudnative-pg/api/pkg/api/v1"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/sourcehawk/operator-component-framework/pkg/component"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	utilrand "k8s.io/apimachinery/pkg/util/rand"
	"sigs.k8s.io/controller-runtime/pkg/client"

	v1 "github.com/konsole-is/camunda-operator/api/v1"
	components "github.com/konsole-is/camunda-operator/pkg/components/databaseserver"
	"github.com/konsole-is/camunda-operator/pkg/labels"
	"github.com/konsole-is/camunda-operator/pkg/wrappers/barmanobjectstore"
)

// presetStorageSize is the data volume that serverOnPreset puts on the preset.
// The clamp cases lower it under a running server and read the applied size
// back.
const presetStorageSize = "10Gi"

// serverInNamespace creates a namespace and a minimal DatabaseServer in it,
// and returns the server. The caller drives the CloudNativePG objects.
func serverInNamespace(archive *v1.DatabaseServerArchiveSpec) *v1.DatabaseServer {
	GinkgoHelper()

	namespace := "dbs-" + utilrand.String(8)
	Expect(k8sClient.Create(ctx, &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{Name: namespace},
	})).To(Succeed())

	return serverNamed(namespace, "camunda", "camunda", archive)
}

// serverNamed creates a minimal DatabaseServer in a namespace that exists
// already, under a name and a contract name of its own. Two of them is how a
// spec puts two servers on one contract name.
func serverNamed(
	namespace, name, contract string,
	archive *v1.DatabaseServerArchiveSpec,
) *v1.DatabaseServer {
	GinkgoHelper()

	server := &v1.DatabaseServer{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: namespace},
		Spec: v1.DatabaseServerSpec{
			Version:              "17",
			Instances:            new(int32(1)),
			StorageSize:          new(resource.MustParse("1Gi")),
			DatabaseServerConfig: contract,
			Archive:              archive,
		},
	}
	Expect(k8sClient.Create(ctx, server)).To(Succeed())

	return server
}

// serverOnPreset creates a namespace, a cluster-scoped preset that holds the
// version, a data volume of presetStorageSize, and the write-ahead log volume,
// and a DatabaseServer that inherits them. An empty walStorageSize leaves the
// write-ahead log on the data volume. It returns the server and the preset.
func serverOnPreset(walStorageSize string) (*v1.DatabaseServer, *v1.DatabaseServerPreset) {
	GinkgoHelper()

	preset := &v1.DatabaseServerPreset{
		ObjectMeta: metav1.ObjectMeta{Name: "dbsp-" + utilrand.String(8)},
		Spec: v1.DatabaseServerPresetSpec{
			Server: v1.DatabaseServerSpec{
				Version:     "17",
				Instances:   new(int32(1)),
				StorageSize: new(resource.MustParse(presetStorageSize)),
			},
		},
	}
	if walStorageSize != "" {
		preset.Spec.Server.WALStorageSize = new(resource.MustParse(walStorageSize))
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

// updatePreset applies mutate to the latest revision of the preset.
func updatePreset(preset *v1.DatabaseServerPreset, mutate func(*v1.DatabaseServerPreset)) {
	GinkgoHelper()

	Eventually(func(g Gomega) {
		var latest v1.DatabaseServerPreset
		g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(preset), &latest)).To(Succeed())
		mutate(&latest)
		g.Expect(k8sClient.Update(ctx, &latest)).To(Succeed())
	}, timeout, interval).Should(Succeed())
}

// renameContract puts name on spec.databaseServerConfig of the latest revision
// of the server.
func renameContract(server *v1.DatabaseServer, name string) {
	GinkgoHelper()

	Eventually(func(g Gomega) {
		var latest v1.DatabaseServer
		g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(server), &latest)).To(Succeed())
		latest.Spec.DatabaseServerConfig = name
		g.Expect(k8sClient.Update(ctx, &latest)).To(Succeed())
	}, timeout, interval).Should(Succeed())
}

// expectShrinkWarning waits until the controller recorded the
// StorageShrinkIgnored Warning about the named field. The field is read off the
// start of the message, because "storageSize" is a substring of
// "walStorageSize" and each volume is clamped on its own.
func expectShrinkWarning(server *v1.DatabaseServer, field string) {
	GinkgoHelper()

	Eventually(func(g Gomega) {
		var events corev1.EventList
		g.Expect(k8sClient.List(ctx, &events, client.InNamespace(server.Namespace))).To(Succeed())
		g.Expect(events.Items).To(ContainElement(SatisfyAll(
			HaveField("Reason", "StorageShrinkIgnored"),
			HaveField("InvolvedObject.Name", server.Name),
			HaveField("Type", corev1.EventTypeWarning),
			HaveField("Message", HavePrefix(field+" ")),
		)))
	}, timeout, interval).Should(Succeed())
}

// countEvents returns the number of times an event with the given reason was
// recorded for the server: the sum of the counts of the matching Event
// objects, because the recorder aggregates repeats of one event into one
// object.
func countEvents(g Gomega, server *v1.DatabaseServer, reason string) int32 {
	GinkgoHelper()

	var recorded corev1.EventList
	g.Expect(k8sClient.List(ctx, &recorded, client.InNamespace(server.Namespace))).To(Succeed())

	var count int32
	for _, event := range recorded.Items {
		if event.Reason == reason && event.InvolvedObject.Name == server.Name {
			count += max(event.Count, 1)
		}
	}

	return count
}

// reconcileAgain edits a spec field that the version refusal does not touch,
// and waits until status reports that generation. The caller then knows one
// more reconcile ran with whatever the refusal left standing.
func reconcileAgain(server *v1.DatabaseServer, round int) {
	GinkgoHelper()

	var generation int64
	Eventually(func(g Gomega) {
		var latest v1.DatabaseServer
		g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(server), &latest)).To(Succeed())
		latest.Spec.PodLabels = map[string]string{"round": strconv.Itoa(round)}
		g.Expect(k8sClient.Update(ctx, &latest)).To(Succeed())
		generation = latest.Generation
	}, timeout, interval).Should(Succeed())

	Eventually(func(g Gomega) {
		g.Expect(reconciledServer(server).Status.ObservedGeneration).
			To(BeNumerically(">=", generation))
	}, timeout, interval).Should(Succeed())
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
	major := imageMajorVersion(cluster.Spec.ImageName)
	Expect(major).To(BeNumerically(">", 0), cluster.Spec.ImageName)
	cluster.Status.PGDataImageInfo = &cnpgv1.ImageInfo{
		Image: cluster.Spec.ImageName, MajorVersion: major,
	}
	Expect(k8sClient.Status().Update(ctx, &cluster)).To(Succeed())
}

// imageMajorVersion reads the PostgreSQL major off the tag of a CloudNativePG
// image, which is what the operator reports for the data directory the
// instances run on. It returns 0 for an image with no numeric tag, which is
// what a cluster a spec wrote by hand carries. The tag starts after the last
// colon, so a registry that names a port keeps its port.
func imageMajorVersion(image string) int {
	colon := strings.LastIndex(image, ":")
	if colon < 0 {
		return 0
	}

	major, err := strconv.Atoi(image[colon+1:])
	if err != nil {
		return 0
	}

	return major
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

// completeBaseBackup creates a completed Backup of the cluster that ran at one
// moment, as the CloudNativePG operator would after the ScheduledBackup fired.
func completeBaseBackup(server *v1.DatabaseServer, name string, at metav1.Time) {
	GinkgoHelper()

	completeBaseBackupBetween(server, name, &at, at)
}

// completeBaseBackupBetween creates a completed Backup that ran from startedAt
// to stoppedAt. A backup that began before the server closed an archive
// interval and ended after it writes to the bucket the server left. A nil
// startedAt is a backup that recorded no start, which the CloudNativePG Backup
// type allows.
func completeBaseBackupBetween(
	server *v1.DatabaseServer,
	name string,
	startedAt *metav1.Time,
	stoppedAt metav1.Time,
) {
	GinkgoHelper()

	backup := &cnpgv1.Backup{
		ObjectMeta: metav1.ObjectMeta{
			Name:      components.ClusterName(server) + "-" + name,
			Namespace: server.Namespace,
			Labels:    map[string]string{components.CNPGClusterNameLabel: components.ClusterName(server)},
		},
		Spec: cnpgv1.BackupSpec{
			Cluster:             cnpgv1.LocalObjectReference{Name: components.ClusterName(server)},
			Method:              cnpgv1.BackupMethodPlugin,
			PluginConfiguration: &cnpgv1.BackupPluginConfiguration{Name: components.BarmanPluginName},
		},
	}
	Expect(k8sClient.Create(ctx, backup)).To(Succeed())

	backup.Status.Phase = cnpgv1.BackupPhaseCompleted
	backup.Status.StartedAt = startedAt
	backup.Status.StoppedAt = &stoppedAt
	Expect(k8sClient.Status().Update(ctx, backup)).To(Succeed())
}

// completeVolumeSnapshotBackup creates a completed Backup of the cluster that
// somebody took by hand with another method. It names the same cluster and
// carries the same label, and it puts nothing in the archive of the server.
func completeVolumeSnapshotBackup(server *v1.DatabaseServer, name string, at metav1.Time) {
	GinkgoHelper()

	backup := &cnpgv1.Backup{
		ObjectMeta: metav1.ObjectMeta{
			Name:      components.ClusterName(server) + "-" + name,
			Namespace: server.Namespace,
			Labels:    map[string]string{components.CNPGClusterNameLabel: components.ClusterName(server)},
		},
		Spec: cnpgv1.BackupSpec{
			Cluster: cnpgv1.LocalObjectReference{Name: components.ClusterName(server)},
			Method:  cnpgv1.BackupMethodVolumeSnapshot,
		},
	}
	Expect(k8sClient.Create(ctx, backup)).To(Succeed())

	backup.Status.Phase = cnpgv1.BackupPhaseCompleted
	backup.Status.StartedAt = &at
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

// setVersion puts version on the latest revision of the server.
func setVersion(server *v1.DatabaseServer, version string) {
	GinkgoHelper()

	Eventually(func(g Gomega) {
		var latest v1.DatabaseServer
		g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(server), &latest)).To(Succeed())
		latest.Spec.Version = version
		g.Expect(k8sClient.Update(ctx, &latest)).To(Succeed())
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

// moveBucket points the ObjectStorageConfig at another bucket, keeping its
// name. An ObjectStorageConfig is mutable, and a delete and create keeps the
// name too, so the name says nothing about where the objects are.
func moveBucket(bucket *v1.ObjectStorageConfig, bucketName string) {
	GinkgoHelper()

	Eventually(func(g Gomega) {
		var latest v1.ObjectStorageConfig
		g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(bucket), &latest)).To(Succeed())
		latest.Spec.S3.BucketName = bucketName
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

// mustCluster reads the CloudNativePG cluster at key, for an assertion about
// the object rather than about its presence.
func mustCluster(g Gomega, key client.ObjectKey) *cnpgv1.Cluster {
	var cluster cnpgv1.Cluster
	g.Expect(k8sClient.Get(ctx, key, &cluster)).To(Succeed())

	return &cluster
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
// credentials and the Secret those credentials live in. Each contract names a
// bucket of its own, so two of them describe two locations, which is what the
// archive history compares.
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
				BucketName: name,
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
			"s3://" + bucket.Name + "/clusters/databaseserver/" + server.Namespace + "/camunda",
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

	It("holds the archive until a base backup of the archive plugin completes", func() {
		bucket := archiveBucket()
		server := serverInNamespace(&v1.DatabaseServerArchiveSpec{
			ObjectStorageRef:    bucket.Name,
			RetentionPeriodDays: 30,
			BaseBackupSchedule:  components.DefaultBaseBackupSchedule,
		})
		writeSuperuserSecret(server)
		makeClusterHealthy(server, "7000000000000000012")

		expectCondition(server, v1.ConditionArchiveReady, metav1.ConditionFalse)

		completeVolumeSnapshotBackup(server, "snapshot", metav1.NewTime(metav1.Now().Rfc3339Copy().Time))

		// The snapshot recovers the volumes of one moment. It puts no base
		// backup in the bucket, so the archive still reaches no point and the
		// history stays empty.
		Consistently(func(g Gomega) {
			var latest v1.DatabaseServer
			g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(server), &latest)).To(Succeed())
			archive := meta.FindStatusCondition(latest.Status.Conditions, v1.ConditionArchiveReady)
			g.Expect(archive).NotTo(BeNil())
			g.Expect(archive.Status).To(Equal(metav1.ConditionFalse), archive.Message)

			var history []v1.ArchiveRecord
			if latest.Status.Archive != nil {
				history = latest.Status.Archive.History
			}
			g.Expect(history).To(BeEmpty())
		}, "2s", interval).Should(Succeed())

		completedAt := metav1.NewTime(metav1.Now().Rfc3339Copy().Time)
		completeBaseBackup(server, "base", completedAt)

		expectCondition(server, v1.ConditionArchiveReady, metav1.ConditionTrue)

		Eventually(func(g Gomega) {
			var latest v1.DatabaseServer
			g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(server), &latest)).To(Succeed())
			g.Expect(latest.Status.Archive).NotTo(BeNil())
			g.Expect(latest.Status.Archive.History).To(HaveLen(1))
			g.Expect(latest.Status.Archive.History[0].From.Time).To(BeTemporally("==", completedAt.Time))
		}, timeout, interval).Should(Succeed())
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

	It("removes the contract it published under the previous name", func() {
		server := serverInNamespace(nil)
		writeSuperuserSecret(server)
		makeClusterHealthy(server, "7000000000000000011")
		expectCondition(server, v1.ConditionContractReady, metav1.ConditionTrue)

		renameContract(server, "camunda-renamed")

		// The contract of the previous name keeps its owner reference and its
		// pitr.recovery: operator declaration, so a PointInTimeRestore that
		// resolves through it asks a server that never reads it again.
		Eventually(func(g Gomega) {
			g.Expect(k8sClient.Get(
				ctx, client.ObjectKey{Namespace: server.Namespace, Name: "camunda-renamed"},
				&v1.DatabaseServerConfig{},
			)).To(Succeed())
			g.Expect(k8sClient.Get(
				ctx, client.ObjectKey{Namespace: server.Namespace, Name: "camunda"},
				&v1.DatabaseServerConfig{},
			)).To(MatchError(apierrors.IsNotFound, "not found"))
		}, timeout, interval).Should(Succeed())
	})

	It("leaves a contract it does not own where it is", func() {
		server := serverInNamespace(nil)
		writeSuperuserSecret(server)
		makeClusterHealthy(server, "7000000000000000014")
		expectCondition(server, v1.ConditionContractReady, metav1.ConditionTrue)

		// The label of this server, and no owner reference. The sweep runs
		// over a label selector, and a label is a value anybody can write.
		stranger := &v1.DatabaseServerConfig{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "camunda-stranger",
				Namespace: server.Namespace,
				Labels:    map[string]string{labels.DatabaseServerKey: server.Name},
			},
			Spec: v1.DatabaseServerConfigSpec{
				Engine: v1.DatabaseEnginePostgres,
				Host:   "postgres.example.svc",
				Port:   5432,
				AdminCredentialsSecretRef: v1.LocalCredentialsSecretRef{
					Name:        "somebody-elses",
					UsernameKey: "username",
					PasswordKey: "password",
				},
			},
		}
		Expect(k8sClient.Create(ctx, stranger)).To(Succeed())

		renameContract(server, "camunda-renamed")

		Eventually(func() error {
			return k8sClient.Get(
				ctx, client.ObjectKey{Namespace: server.Namespace, Name: "camunda"},
				&v1.DatabaseServerConfig{},
			)
		}, timeout, interval).Should(MatchError(apierrors.IsNotFound, "not found"))

		Consistently(func() error {
			return k8sClient.Get(ctx, client.ObjectKeyFromObject(stranger), &v1.DatabaseServerConfig{})
		}, "2s", interval).Should(Succeed())
	})

	It("keeps the contract of the previous name until the new one is published", func() {
		server := serverInNamespace(nil)
		writeSuperuserSecret(server)
		makeClusterHealthy(server, "7000000000000000013")
		expectCondition(server, v1.ConditionContractReady, metav1.ConditionTrue)

		// The contract component blocks on the superuser Secret, so nothing is
		// published under the new name while the Secret is away.
		Expect(k8sClient.Delete(ctx, &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      components.SuperuserSecretName(server),
				Namespace: server.Namespace,
			},
		})).To(Succeed())
		renameContract(server, "camunda-renamed")

		expectCondition(server, v1.ConditionContractReady, metav1.ConditionFalse)

		// A sweep on that look leaves the server publishing nothing.
		Consistently(func(g Gomega) {
			g.Expect(k8sClient.Get(
				ctx, client.ObjectKey{Namespace: server.Namespace, Name: "camunda"},
				&v1.DatabaseServerConfig{},
			)).To(Succeed())
			g.Expect(k8sClient.Get(
				ctx, client.ObjectKey{Namespace: server.Namespace, Name: "camunda-renamed"},
				&v1.DatabaseServerConfig{},
			)).To(MatchError(apierrors.IsNotFound, "not found"))
		}, 2*time.Second, interval).Should(Succeed())

		By("sweeping once the new name is published")
		writeSuperuserSecret(server)
		Eventually(func(g Gomega) {
			g.Expect(k8sClient.Get(
				ctx, client.ObjectKey{Namespace: server.Namespace, Name: "camunda-renamed"},
				&v1.DatabaseServerConfig{},
			)).To(Succeed())
			g.Expect(k8sClient.Get(
				ctx, client.ObjectKey{Namespace: server.Namespace, Name: "camunda"},
				&v1.DatabaseServerConfig{},
			)).To(MatchError(apierrors.IsNotFound, "not found"))
		}, timeout, interval).Should(Succeed())
	})

	It("publishes nothing on a contract that another server holds", func() {
		owner := serverInNamespace(nil)
		writeSuperuserSecret(owner)
		expectCondition(owner, v1.ConditionContractReady, metav1.ConditionTrue)
		host := publishedContract(owner).Spec.Host

		// The second server names the contract of the first, and its own
		// superuser Secret is there. Nothing but the guard keeps it off.
		second := serverNamed(owner.Namespace, "second", "camunda", nil)
		writeSuperuserSecret(second)

		taken := expectCondition(second, v1.ConditionContractReady, metav1.ConditionFalse)
		Expect(taken.Reason).To(Equal(v1.ReasonContractTaken))
		Expect(taken.Message).To(ContainSubstring(`DatabaseServer "camunda"`))
		Expect(conditionOf(second, v1.ConditionReady).Status).To(Equal(metav1.ConditionFalse))

		// The apply moves the owner reference, the label, and the endpoint
		// together, so a contract the second server wrote shows in all three.
		Consistently(func(g Gomega) {
			contract := publishedContract(owner)
			g.Expect(contract.Spec.Host).To(Equal(host))
			g.Expect(contract.Labels).To(HaveKeyWithValue(labels.DatabaseServerKey, "camunda"))
			g.Expect(metav1.IsControlledBy(contract, reconciledServer(owner))).To(BeTrue())
		}, 3*time.Second, interval).Should(Succeed())
	})

	It("publishes the contract once the server that held it is gone", func() {
		owner := serverInNamespace(nil)
		writeSuperuserSecret(owner)
		expectCondition(owner, v1.ConditionContractReady, metav1.ConditionTrue)

		second := serverNamed(owner.Namespace, "second", "camunda", nil)
		writeSuperuserSecret(second)
		expectCondition(second, v1.ConditionContractReady, metav1.ConditionFalse)

		By("deleting the server that holds the contract")
		published := publishedContract(owner)
		Expect(k8sClient.Delete(ctx, owner)).To(Succeed())
		// envtest runs no garbage collector, so the contract the owner
		// reference points at goes here instead.
		Expect(k8sClient.Delete(ctx, published)).To(Succeed())

		expectCondition(second, v1.ConditionContractReady, metav1.ConditionTrue)
		contract := publishedContract(second)
		Expect(metav1.IsControlledBy(contract, reconciledServer(second))).To(BeTrue())
		Expect(contract.Spec.Host).To(Equal("second-rw." + second.Namespace + ".svc"))
	})

	It("writes nothing on a CloudNativePG cluster that another server holds", func() {
		namespace := "dbs-" + utilrand.String(8)
		Expect(k8sClient.Create(ctx, &corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{Name: namespace},
		})).To(Succeed())

		// The cluster of a DatabaseServer takes the name of the server, so a
		// second server of that name in the namespace derives a name that is
		// already running a database. The occupant carries the label of the
		// server under test too, because a label is a value anybody can write
		// and it is not what says who owns a database.
		holder := serverNamed(namespace, "holder", "holder", nil)
		occupant := &cnpgv1.Cluster{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "camunda",
				Namespace: namespace,
				Labels:    map[string]string{labels.DatabaseServerKey: "camunda"},
				OwnerReferences: []metav1.OwnerReference{{
					APIVersion: v1.GroupVersion.String(),
					Kind:       "DatabaseServer",
					Name:       holder.Name,
					UID:        reconciledServer(holder).UID,
					Controller: new(true),
				}},
			},
			Spec: cnpgv1.ClusterSpec{
				Instances:            3,
				StorageConfiguration: cnpgv1.StorageConfiguration{Size: "8Gi"},
			},
		}
		Expect(k8sClient.Create(ctx, occupant)).To(Succeed())

		bucket := archiveBucket()
		server := serverNamed(namespace, "camunda", "camunda", &v1.DatabaseServerArchiveSpec{
			ObjectStorageRef:    bucket.Name,
			RetentionPeriodDays: 30,
		})
		// CloudNativePG wrote this Secret for the occupant. Nothing but the
		// guard keeps the contract off the endpoint it belongs to.
		writeSuperuserSecret(server)

		taken := expectCondition(server, v1.ConditionClusterReady, metav1.ConditionFalse)
		Expect(taken.Reason).To(Equal(v1.ReasonClusterTaken))
		Expect(taken.Message).To(ContainSubstring(`DatabaseServer "holder"`))
		Expect(conditionOf(server, v1.ConditionReady).Status).To(Equal(metav1.ConditionFalse))

		key := client.ObjectKey{Namespace: namespace, Name: "camunda"}
		Consistently(func(g Gomega) {
			// The apply rewrites the spec and takes the owner reference
			// together, so a write of this server shows in either.
			var live cnpgv1.Cluster
			g.Expect(k8sClient.Get(ctx, key, &live)).To(Succeed())
			g.Expect(live.Spec.Instances).To(Equal(3))
			g.Expect(live.Spec.StorageConfiguration.Size).To(Equal("8Gi"))
			g.Expect(metav1.IsControlledBy(&live, reconciledServer(holder))).To(BeTrue())

			// The contract sends every consumer to the database of the
			// holder, and the schedule copies that database into the bucket
			// of this server, so neither is published.
			g.Expect(k8sClient.Get(ctx, key, &v1.DatabaseServerConfig{})).
				To(MatchError(apierrors.IsNotFound, "not found"))
			g.Expect(k8sClient.Get(ctx, key, &cnpgv1.ScheduledBackup{})).
				To(MatchError(apierrors.IsNotFound, "not found"))

			// The ObjectStore describes the bucket rather than the cluster,
			// so it stays and the archive of the server keeps its settings.
			g.Expect(k8sClient.Get(ctx, key, &barmanobjectstore.ObjectStore{})).To(Succeed())
		}, 3*time.Second, interval).Should(Succeed())

		By("removing the cluster that holds the name")
		Expect(k8sClient.Delete(ctx, occupant)).To(Succeed())

		Eventually(func(g Gomega) {
			var live cnpgv1.Cluster
			g.Expect(k8sClient.Get(ctx, key, &live)).To(Succeed())
			g.Expect(metav1.IsControlledBy(&live, reconciledServer(server))).To(BeTrue())
		}, timeout, interval).Should(Succeed())

		makeClusterHealthy(server, "7000000000000000001")
		expectCondition(server, v1.ConditionClusterReady, metav1.ConditionTrue)
		expectCondition(server, v1.ConditionContractReady, metav1.ConditionTrue)
	})

	It("withdraws what it published when another owner takes its cluster", func() {
		server, _ := archivingServer()

		key := client.ObjectKey{Namespace: server.Namespace, Name: "camunda"}
		Expect(k8sClient.Get(ctx, key, &cnpgv1.ScheduledBackup{})).To(Succeed())
		Expect(publishedContract(server).Spec.Host).To(Equal("camunda-rw." + server.Namespace + ".svc"))
		history := archiveHistory(server)
		Expect(history).To(HaveLen(1))

		// The state that a delete of the cluster and a create under the same
		// name leaves: the object of that name is there, and this server does
		// not control it.
		By("giving the cluster to another owner")
		stranger := serverNamed(server.Namespace, "stranger", "stranger", nil)
		Eventually(func(g Gomega) {
			var cluster cnpgv1.Cluster
			g.Expect(k8sClient.Get(ctx, key, &cluster)).To(Succeed())
			cluster.OwnerReferences = []metav1.OwnerReference{{
				APIVersion: v1.GroupVersion.String(),
				Kind:       "DatabaseServer",
				Name:       stranger.Name,
				UID:        reconciledServer(stranger).UID,
				Controller: new(true),
			}}
			g.Expect(k8sClient.Update(ctx, &cluster)).To(Succeed())
		}, timeout, interval).Should(Succeed())

		taken := expectCondition(server, v1.ConditionClusterReady, metav1.ConditionFalse)
		Expect(taken.Reason).To(Equal(v1.ReasonClusterTaken))
		Expect(taken.Message).To(ContainSubstring(`DatabaseServer "stranger"`))

		// Both name the cluster of that name. The contract sends consumers to
		// it, and the schedule takes base backups of it into this bucket.
		By("removing the contract and the base backup schedule")
		expectGone(contractKey(server), &v1.DatabaseServerConfig{})
		expectGone(key, &cnpgv1.ScheduledBackup{})

		// The archive the server already wrote is still in the bucket, and
		// the record of it is what a later restore reads.
		Consistently(func(g Gomega) {
			g.Expect(archiveHistory(server)).To(Equal(history))
			g.Expect(k8sClient.Get(ctx, key, &barmanobjectstore.ObjectStore{})).To(Succeed())
			g.Expect(metav1.IsControlledBy(mustCluster(g, key), reconciledServer(stranger))).To(BeTrue())
		}, "2s", interval).Should(Succeed())

		By("giving the name back")
		Expect(k8sClient.Delete(ctx, &cnpgv1.Cluster{
			ObjectMeta: metav1.ObjectMeta{Name: "camunda", Namespace: server.Namespace},
		})).To(Succeed())

		Eventually(func(g Gomega) {
			g.Expect(metav1.IsControlledBy(mustCluster(g, key), reconciledServer(server))).To(BeTrue())
		}, timeout, interval).Should(Succeed())

		makeClusterHealthy(server, "7000000000000000001")
		expectCondition(server, v1.ConditionClusterReady, metav1.ConditionTrue)
		expectCondition(server, v1.ConditionContractReady, metav1.ConditionTrue)
		Expect(publishedContract(server).Spec.Host).To(Equal("camunda-rw." + server.Namespace + ".svc"))
		Eventually(func() error {
			return k8sClient.Get(ctx, key, &cnpgv1.ScheduledBackup{})
		}, timeout, interval).Should(Succeed())
		Expect(archiveHistory(server)).To(Equal(history))
	})

	It("leaves a base backup schedule that another owner controls alone", func() {
		namespace := "dbs-" + utilrand.String(8)
		Expect(k8sClient.Create(ctx, &corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{Name: namespace},
		})).To(Succeed())

		// The schedule takes the name of the cluster, and the cluster takes the
		// name of the server, so a second server of that name derives a name
		// that already belongs to somebody. The cluster name itself is free
		// here: nothing but the block on the schedule keeps this server off it.
		holder := serverNamed(namespace, "holder", "holder", nil)
		occupant := &cnpgv1.ScheduledBackup{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "camunda",
				Namespace: namespace,
				OwnerReferences: []metav1.OwnerReference{{
					APIVersion: v1.GroupVersion.String(),
					Kind:       "DatabaseServer",
					Name:       holder.Name,
					UID:        reconciledServer(holder).UID,
					Controller: new(true),
				}},
			},
			Spec: cnpgv1.ScheduledBackupSpec{
				Schedule: "0 0 5 * * *",
				Cluster:  cnpgv1.LocalObjectReference{Name: "holder"},
			},
		}
		Expect(k8sClient.Create(ctx, occupant)).To(Succeed())

		bucket := archiveBucket()
		server := serverNamed(namespace, "camunda", "camunda", &v1.DatabaseServerArchiveSpec{
			ObjectStorageRef:    bucket.Name,
			RetentionPeriodDays: 30,
		})
		writeSuperuserSecret(server)

		blocked := expectCondition(server, v1.ConditionArchiveReady, metav1.ConditionFalse)
		Expect(blocked.Reason).To(Equal(string(component.GuardBlocked)), blocked.Message)
		Expect(blocked.Message).To(ContainSubstring("controlled by DatabaseServer holder"))

		key := client.ObjectKey{Namespace: namespace, Name: "camunda"}
		Consistently(func(g Gomega) {
			// The apply rewrites the schedule and the cluster it names together
			// with the owner reference, so a write of this server shows in any
			// of the three.
			var live cnpgv1.ScheduledBackup
			g.Expect(k8sClient.Get(ctx, key, &live)).To(Succeed())
			g.Expect(live.Spec.Schedule).To(Equal("0 0 5 * * *"))
			g.Expect(live.Spec.Cluster.Name).To(Equal("holder"))
			g.Expect(metav1.IsControlledBy(&live, reconciledServer(holder))).To(BeTrue())
		}, 3*time.Second, interval).Should(Succeed())

		// Dropping the archive turns the component gate off, which removes
		// every object the component manages. The ObjectStore is this server's
		// and goes; the schedule of the other owner stays.
		By("removing the archive")
		setArchive(server, nil)

		disabled := expectCondition(server, v1.ConditionArchiveReady, metav1.ConditionTrue)
		Expect(disabled.Reason).To(Equal("Disabled"))
		expectGone(key, &barmanobjectstore.ObjectStore{})

		Consistently(func(g Gomega) {
			var live cnpgv1.ScheduledBackup
			g.Expect(k8sClient.Get(ctx, key, &live)).To(Succeed())
			g.Expect(metav1.IsControlledBy(&live, reconciledServer(holder))).To(BeTrue())
		}, 3*time.Second, interval).Should(Succeed())
	})

	It("starts a new archive record when the bucket changes", func() {
		first := archiveBucket()
		server := serverInNamespace(&v1.DatabaseServerArchiveSpec{
			ObjectStorageRef:    first.Name,
			RetentionPeriodDays: 30,
		})
		writeSuperuserSecret(server)
		makeClusterHealthy(server, "7000000000000000010")
		completeBaseBackup(server, "first", metav1.NewTime(metav1.Now().Rfc3339Copy().Time))

		expectCondition(server, v1.ConditionArchiveReady, metav1.ConditionTrue)
		Eventually(func(g Gomega) {
			history := archiveHistory(server)
			g.Expect(history).To(HaveLen(1))
			g.Expect(history[0].ObjectStorageRef).To(Equal(first.Name))
		}, timeout, interval).Should(Succeed())

		second := archiveBucket()
		setArchive(server, &v1.DatabaseServerArchiveSpec{
			ObjectStorageRef:    second.Name,
			RetentionPeriodDays: 30,
		})

		// The interval of the first bucket ends where the second one starts,
		// so a restore of a point inside it knows which bucket holds it.
		Eventually(func(g Gomega) {
			history := archiveHistory(server)
			g.Expect(history).To(HaveLen(1))
			g.Expect(history[0].ObjectStorageRef).To(Equal(first.Name))
			g.Expect(history[0].To).NotTo(BeNil())
		}, timeout, interval).Should(Succeed())
		closedAt := *archiveHistory(server)[0].To

		// A base backup that was already running when the bucket moved keeps
		// the destination it started with, so its object lands in the bucket
		// the server left. Its end falls after the close, and the new interval
		// must not open on it. Only a backup that began after the close does.
		straddlingStart := metav1.NewTime(closedAt.Add(-2 * time.Second))
		completeBaseBackupBetween(
			server, "straddling", &straddlingStart, metav1.NewTime(closedAt.Add(2*time.Second)),
		)

		// A backup that recorded no start sits on neither side of the close.
		// It can have been running in the bucket the server left, so it opens
		// nothing either.
		completeBaseBackupBetween(
			server, "no-start", nil, metav1.NewTime(closedAt.Add(3*time.Second)),
		)

		// The new bucket holds no base backup yet, so no point in it can be
		// reached and no record of it opens.
		blocked := expectCondition(server, v1.ConditionArchiveReady, metav1.ConditionFalse)
		Expect(blocked.Message).To(ContainSubstring("base backup"))
		Consistently(func(g Gomega) {
			g.Expect(archiveHistory(server)).To(HaveLen(1))
		}, 2*time.Second, interval).Should(Succeed())

		openedAt := metav1.NewTime(closedAt.Add(5 * time.Second))
		completeBaseBackup(server, "second", openedAt)

		expectCondition(server, v1.ConditionArchiveReady, metav1.ConditionTrue)
		Eventually(func(g Gomega) {
			history := archiveHistory(server)
			g.Expect(history).To(HaveLen(2))
			g.Expect(history[1].ServerName).To(Equal("camunda"))
			g.Expect(history[1].ObjectStorageRef).To(Equal(second.Name))
			g.Expect(history[1].From.Time).To(BeTemporally("==", openedAt.Time))
			g.Expect(history[1].To).To(BeNil())
		}, timeout, interval).Should(Succeed())
	})

	// The bucket change and the guard on the archive land in one reconcile. A
	// boundary that waits for the recorded close is not moved yet on that
	// reconcile, so a base backup of the bucket the server leaves reports the
	// new archive ready, and the status write that closes the record carries
	// that ready with it.
	It("blocks the new archive while only the bucket it left holds a base backup", func() {
		first := archiveBucket()
		server := serverInNamespace(&v1.DatabaseServerArchiveSpec{
			ObjectStorageRef:    first.Name,
			RetentionPeriodDays: 30,
		})
		writeSuperuserSecret(server)
		makeClusterHealthy(server, "7000000000000000018")
		completeBaseBackup(server, "first", metav1.NewTime(metav1.Now().Rfc3339Copy().Time))

		expectCondition(server, v1.ConditionArchiveReady, metav1.ConditionTrue)

		second := archiveBucket()
		setArchive(server, &v1.DatabaseServerArchiveSpec{
			ObjectStorageRef:    second.Name,
			RetentionPeriodDays: 30,
		})

		// Every status the server writes from the close onwards reports the
		// archive of the new bucket, which holds nothing.
		Consistently(func(g Gomega) {
			latest := reconciledServer(server)
			if latest.Status.Archive == nil || latest.Status.Archive.History[0].To == nil {
				return
			}
			ready := meta.FindStatusCondition(latest.Status.Conditions, v1.ConditionArchiveReady)
			g.Expect(ready).NotTo(BeNil())
			g.Expect(ready.Status).To(Equal(metav1.ConditionFalse), ready.Message)
			g.Expect(latest.Status.Archive.History).To(HaveLen(1))
		}, 3*time.Second, "5ms").Should(Succeed())

		expectCondition(server, v1.ConditionArchiveReady, metav1.ConditionFalse)
	})

	// The name of an ObjectStorageConfig says nothing about where the objects
	// are: it can be edited in place, and a delete and create keeps the name.
	// The interval is compared by the location it was written to.
	It("closes the record when the bucket moves under one ObjectStorageConfig", func() {
		bucket := archiveBucket()
		server := serverInNamespace(&v1.DatabaseServerArchiveSpec{
			ObjectStorageRef:    bucket.Name,
			RetentionPeriodDays: 30,
		})
		writeSuperuserSecret(server)
		makeClusterHealthy(server, "7000000000000000020")
		completeBaseBackup(server, "first", metav1.NewTime(metav1.Now().Rfc3339Copy().Time))

		expectCondition(server, v1.ConditionArchiveReady, metav1.ConditionTrue)
		Eventually(func(g Gomega) {
			history := archiveHistory(server)
			g.Expect(history).To(HaveLen(1))
			g.Expect(history[0].Location).To(ContainSubstring("s3://" + bucket.Name + "/"))
		}, timeout, interval).Should(Succeed())

		By("moving the bucket the ObjectStorageConfig names")
		moved := bucket.Name + "-moved"
		moveBucket(bucket, moved)

		Eventually(func(g Gomega) {
			history := archiveHistory(server)
			g.Expect(history).To(HaveLen(1))
			g.Expect(history[0].To).NotTo(BeNil())
		}, timeout, interval).Should(Succeed())
		closedAt := *archiveHistory(server)[0].To

		blocked := expectCondition(server, v1.ConditionArchiveReady, metav1.ConditionFalse)
		Expect(blocked.Message).To(ContainSubstring("base backup"))

		// The reconcile that closes the record also applies the ObjectStore of
		// the new location. The boundary is read after that, so a base backup
		// that began while the old one still stood is behind it.
		var store barmanobjectstore.ObjectStore
		Eventually(func(g Gomega) {
			g.Expect(k8sClient.Get(
				ctx, client.ObjectKey{Namespace: server.Namespace, Name: "camunda"}, &store,
			)).To(Succeed())
			g.Expect(store.Spec.Configuration.DestinationPath).To(ContainSubstring("s3://" + moved + "/"))
		}, timeout, interval).Should(Succeed())

		By("opening a record of the new location on the next base backup")
		openedStart := metav1.NewTime(closedAt.Add(time.Second))
		completeBaseBackupBetween(
			server, "after-move", &openedStart, metav1.NewTime(closedAt.Add(2*time.Second)),
		)

		expectCondition(server, v1.ConditionArchiveReady, metav1.ConditionTrue)
		Eventually(func(g Gomega) {
			history := archiveHistory(server)
			g.Expect(history).To(HaveLen(2))
			// The contract still carries the name it always had.
			g.Expect(history[1].ObjectStorageRef).To(Equal(bucket.Name))
			g.Expect(history[1].Location).To(ContainSubstring("s3://" + moved + "/"))
		}, timeout, interval).Should(Succeed())
	})

	// Removing spec.archive closes every record, so a re-enable elsewhere finds
	// nothing open to close. The move has nowhere to live but
	// status.archive.boundary, and without it the server accepts any backup
	// that began after the old close, including one still writing to the
	// location it left.
	It("keeps the boundary of an archive re-enabled on another location", func() {
		first := archiveBucket()
		server := serverInNamespace(&v1.DatabaseServerArchiveSpec{
			ObjectStorageRef:    first.Name,
			RetentionPeriodDays: 30,
		})
		writeSuperuserSecret(server)
		makeClusterHealthy(server, "7000000000000000019")
		completeBaseBackup(server, "first", metav1.NewTime(metav1.Now().Rfc3339Copy().Time))

		expectCondition(server, v1.ConditionArchiveReady, metav1.ConditionTrue)

		By("closing the record when the archive is removed")
		setArchive(server, nil)
		Eventually(func(g Gomega) {
			history := archiveHistory(server)
			g.Expect(history).To(HaveLen(1))
			g.Expect(history[0].To).NotTo(BeNil())
		}, timeout, interval).Should(Succeed())
		closedAt := *archiveHistory(server)[0].To

		// status.archive stamps whole seconds, so the close and the move land
		// in one second unless the spec separates them. A backup that began
		// after the close and before the move needs them apart.
		time.Sleep(1500 * time.Millisecond)

		By("recording the move when the archive comes back on another location")
		second := archiveBucket()
		setArchive(server, &v1.DatabaseServerArchiveSpec{
			ObjectStorageRef:    second.Name,
			RetentionPeriodDays: 30,
		})

		var movedAt metav1.Time
		Eventually(func(g Gomega) {
			boundary := reconciledServer(server).Status.Archive.Boundary
			g.Expect(boundary).NotTo(BeNil())
			g.Expect(boundary.ObjectStorageRef).To(Equal(second.Name))
			movedAt = boundary.At
		}, timeout, interval).Should(Succeed())
		Expect(movedAt.Time).To(BeTemporally(">=", closedAt.Add(time.Second)))

		By("refusing a backup that began before the move")
		inFlightStart := metav1.NewTime(closedAt.Add(time.Second))
		completeBaseBackupBetween(
			server, "in-flight", &inFlightStart, metav1.NewTime(movedAt.Add(2*time.Second)),
		)
		blocked := expectCondition(server, v1.ConditionArchiveReady, metav1.ConditionFalse)
		Expect(blocked.Message).To(ContainSubstring("base backup"))
		Consistently(func(g Gomega) {
			g.Expect(archiveHistory(server)).To(HaveLen(1))
		}, 2*time.Second, interval).Should(Succeed())

		By("opening the record on a backup that began after it")
		openedStart := metav1.NewTime(movedAt.Add(5 * time.Second))
		completeBaseBackupBetween(
			server, "after-move", &openedStart, metav1.NewTime(movedAt.Add(6*time.Second)),
		)

		expectCondition(server, v1.ConditionArchiveReady, metav1.ConditionTrue)
		Eventually(func(g Gomega) {
			latest := reconciledServer(server)
			g.Expect(latest.Status.Archive.History).To(HaveLen(2))
			g.Expect(latest.Status.Archive.History[1].ObjectStorageRef).To(Equal(second.Name))
			g.Expect(latest.Status.Archive.History[1].Location).To(ContainSubstring(second.Name))
			// The record holds the move from here on.
			g.Expect(latest.Status.Archive.Boundary).To(BeNil())
		}, timeout, interval).Should(Succeed())
	})

	// status.startedAt is optional on a CloudNativePG Backup. The first archive
	// of a server has no boundary to place a backup against, so a completed
	// backup counts by its end rather than being left out for good.
	It("opens the first archive record on a backup that recorded no start", func() {
		bucket := archiveBucket()
		server := serverInNamespace(&v1.DatabaseServerArchiveSpec{
			ObjectStorageRef:    bucket.Name,
			RetentionPeriodDays: 30,
		})
		writeSuperuserSecret(server)
		makeClusterHealthy(server, "7000000000000000016")

		at := metav1.NewTime(metav1.Now().Rfc3339Copy().Time)
		completeBaseBackupBetween(server, "no-start", nil, at)

		expectCondition(server, v1.ConditionArchiveReady, metav1.ConditionTrue)
		Eventually(func(g Gomega) {
			history := archiveHistory(server)
			g.Expect(history).To(HaveLen(1))
			g.Expect(history[0].ObjectStorageRef).To(Equal(bucket.Name))
			g.Expect(history[0].From.Time).To(BeTemporally("==", at.Time))
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
		server, preset := serverOnPreset("")
		writeSuperuserSecret(server)
		makeClusterHealthy(server, "7000000000000000009")

		clusterKey := client.ObjectKey{Namespace: server.Namespace, Name: "camunda"}
		Eventually(func(g Gomega) {
			var cluster cnpgv1.Cluster
			g.Expect(k8sClient.Get(ctx, clusterKey, &cluster)).To(Succeed())
			g.Expect(cluster.Spec.StorageConfiguration.Size).To(Equal(presetStorageSize))
		}, timeout, interval).Should(Succeed())

		updatePreset(preset, func(p *v1.DatabaseServerPreset) {
			p.Spec.Server.StorageSize = new(resource.MustParse("1Gi"))
		})

		expectShrinkWarning(server, "storageSize")

		// CloudNativePG refuses a cluster whose storage is smaller than the
		// one it applied, so a server that let the smaller size through stops
		// converging.
		Consistently(func(g Gomega) {
			var cluster cnpgv1.Cluster
			g.Expect(k8sClient.Get(ctx, clusterKey, &cluster)).To(Succeed())
			g.Expect(cluster.Spec.StorageConfiguration.Size).To(Equal(presetStorageSize))
		}, 2*time.Second, interval).Should(Succeed())

		ready := conditionOf(server, v1.ConditionReady)
		Expect(ready).NotTo(BeNil())
		Expect(ready.Reason).NotTo(Equal(v1.ReasonInvalidReference))
	})

	// The write-ahead log volume is measured on its own. A clamp that read the
	// data claims for it raises walStorageSize to the data size, which is
	// larger here on purpose.
	It("keeps the applied write-ahead log size when a preset lowers it", func() {
		server, preset := serverOnPreset("4Gi")
		writeSuperuserSecret(server)
		makeClusterHealthy(server, "7000000000000000012")

		clusterKey := client.ObjectKey{Namespace: server.Namespace, Name: "camunda"}
		Eventually(func(g Gomega) {
			var cluster cnpgv1.Cluster
			g.Expect(k8sClient.Get(ctx, clusterKey, &cluster)).To(Succeed())
			g.Expect(cluster.Spec.WalStorage).NotTo(BeNil())
			g.Expect(cluster.Spec.WalStorage.Size).To(Equal("4Gi"))
		}, timeout, interval).Should(Succeed())

		updatePreset(preset, func(p *v1.DatabaseServerPreset) {
			p.Spec.Server.WALStorageSize = new(resource.MustParse("1Gi"))
		})

		expectShrinkWarning(server, "walStorageSize")

		Consistently(func(g Gomega) {
			var cluster cnpgv1.Cluster
			g.Expect(k8sClient.Get(ctx, clusterKey, &cluster)).To(Succeed())
			g.Expect(cluster.Spec.WalStorage).NotTo(BeNil())
			g.Expect(cluster.Spec.WalStorage.Size).To(Equal("4Gi"))
			g.Expect(cluster.Spec.StorageConfiguration.Size).To(Equal(presetStorageSize))
		}, 2*time.Second, interval).Should(Succeed())

		ready := conditionOf(server, v1.ConditionReady)
		Expect(ready).NotTo(BeNil())
		Expect(ready.Reason).NotTo(Equal(v1.ReasonInvalidReference))
	})

	// CloudNativePG refuses a cluster that gives up its write-ahead log volume,
	// with "walStorage cannot be disabled once configured". A preset that
	// clears the field must therefore not reach it.
	It("keeps the write-ahead log volume a preset tries to remove", func() {
		server, preset := serverOnPreset("4Gi")
		writeSuperuserSecret(server)
		makeClusterHealthy(server, "7000000000000000015")

		clusterKey := client.ObjectKey{Namespace: server.Namespace, Name: "camunda"}
		Eventually(func(g Gomega) {
			var cluster cnpgv1.Cluster
			g.Expect(k8sClient.Get(ctx, clusterKey, &cluster)).To(Succeed())
			g.Expect(cluster.Spec.WalStorage).NotTo(BeNil())
			g.Expect(cluster.Spec.WalStorage.Size).To(Equal("4Gi"))
		}, timeout, interval).Should(Succeed())

		updatePreset(preset, func(p *v1.DatabaseServerPreset) {
			p.Spec.Server.WALStorageSize = nil
		})

		Eventually(func(g Gomega) {
			var recorded corev1.EventList
			g.Expect(k8sClient.List(ctx, &recorded, client.InNamespace(server.Namespace))).To(Succeed())
			g.Expect(recorded.Items).To(ContainElement(SatisfyAll(
				HaveField("Reason", "WALStorageKept"),
				HaveField("InvolvedObject.Name", server.Name),
				HaveField("Type", corev1.EventTypeWarning),
			)))
		}, timeout, interval).Should(Succeed())

		Consistently(func(g Gomega) {
			var cluster cnpgv1.Cluster
			g.Expect(k8sClient.Get(ctx, clusterKey, &cluster)).To(Succeed())
			g.Expect(cluster.Spec.WalStorage).NotTo(BeNil())
			g.Expect(cluster.Spec.WalStorage.Size).To(Equal("4Gi"))
		}, 2*time.Second, interval).Should(Succeed())
	})

	// Admission cannot catch this either: a preset can raise the version, and
	// the major the data directory runs is on the CloudNativePG cluster rather
	// than on the spec.
	It("refuses a major version change and keeps the image the server runs", func() {
		server, preset := serverOnPreset("")
		writeSuperuserSecret(server)
		makeClusterHealthy(server, "7000000000000000014")

		expectCondition(server, v1.ConditionReady, metav1.ConditionTrue)

		updatePreset(preset, func(p *v1.DatabaseServerPreset) {
			p.Spec.Server.Version = "18"
		})

		Eventually(func(g Gomega) {
			ready := conditionOf(server, v1.ConditionReady)
			g.Expect(ready).NotTo(BeNil())
			g.Expect(ready.Status).To(Equal(metav1.ConditionFalse))
			g.Expect(ready.Reason).To(Equal(v1.ReasonVersionChangeRefused))
			g.Expect(ready.Message).To(ContainSubstring("18"))
			g.Expect(ready.Message).To(ContainSubstring("17"))
		}, timeout, interval).Should(Succeed())

		// CloudNativePG stops every instance to upgrade the data directory in
		// place, so the refused server must apply nothing at all.
		clusterKey := client.ObjectKey{Namespace: server.Namespace, Name: "camunda"}
		Consistently(func(g Gomega) {
			var cluster cnpgv1.Cluster
			g.Expect(k8sClient.Get(ctx, clusterKey, &cluster)).To(Succeed())
			g.Expect(cluster.Spec.ImageName).To(HaveSuffix(":17"))
		}, 2*time.Second, interval).Should(Succeed())

		var recorded corev1.EventList
		Expect(k8sClient.List(ctx, &recorded, client.InNamespace(server.Namespace))).To(Succeed())
		Expect(recorded.Items).To(ContainElement(SatisfyAll(
			HaveField("Reason", v1.ReasonVersionChangeRefused),
			HaveField("InvolvedObject.Name", server.Name),
			HaveField("Type", corev1.EventTypeWarning),
		)))
	})

	// The refusal is one thing that happened, and it stands until the version
	// comes back. Every reconcile while it stands records nothing more, so a
	// reader who runs kubectl describe sees the refusal rather than a page of
	// copies of it.
	It("records the version refusal once while it stands", func() {
		server, preset := serverOnPreset("")
		writeSuperuserSecret(server)
		makeClusterHealthy(server, "7000000000000000017")

		expectCondition(server, v1.ConditionReady, metav1.ConditionTrue)

		updatePreset(preset, func(p *v1.DatabaseServerPreset) {
			p.Spec.Server.Version = "18"
		})

		Eventually(func(g Gomega) {
			ready := conditionOf(server, v1.ConditionReady)
			g.Expect(ready).NotTo(BeNil())
			g.Expect(ready.Reason).To(Equal(v1.ReasonVersionChangeRefused))
		}, timeout, interval).Should(Succeed())

		reconcileAgain(server, 1)
		reconcileAgain(server, 2)

		Consistently(func(g Gomega) {
			g.Expect(countEvents(g, server, v1.ReasonVersionChangeRefused)).To(Equal(int32(1)))
		}, 2*time.Second, interval).Should(Succeed())
	})

	// The cluster a rollback replaced is gone. status.cluster is the record of
	// the one the server runs from now, and a status write that is lost leaves
	// it on the removed one. The guard has to reach the running cluster
	// anyway, or the version it refuses is applied to it.
	It("refuses a major change after a rollback whose recorded cluster was lost", func() {
		server, from := archivingServer()
		askForRecovery(server, from.Add(time.Hour))
		expectRecoveryCluster(server)
		recovered := recoveryCluster(server)
		recoverySucceeds(server)
		expectLastRecovery(server, v1.RecoveryResultCompleted)

		setVersion(server, "18")
		Eventually(func(g Gomega) {
			ready := conditionOf(server, v1.ConditionReady)
			g.Expect(ready).NotTo(BeNil())
			g.Expect(ready.Reason).To(Equal(v1.ReasonVersionChangeRefused))
		}, timeout, interval).Should(Succeed())

		By("losing the record of the cluster the rollback moved to")
		Eventually(func(g Gomega) {
			var latest v1.DatabaseServer
			g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(server), &latest)).To(Succeed())
			latest.Status.Recovery = nil
			latest.Status.Cluster = "camunda"
			g.Expect(k8sClient.Status().Update(ctx, &latest)).To(Succeed())
		}, timeout, interval).Should(Succeed())

		// The contract still names the recovered server, so the refusal reads
		// the major off that cluster and it keeps the image it runs.
		//
		// The poll is fast because the reconcile that reads the lost record is
		// one reconcile: a guard that gives up there renders the new major
		// once, and the reconcile after it repairs status.cluster and puts the
		// image back. Once is all CloudNativePG needs.
		key := client.ObjectKey{Namespace: server.Namespace, Name: recovered}
		replacedKey := client.ObjectKey{Namespace: server.Namespace, Name: "camunda"}
		Consistently(func(g Gomega) {
			var cluster cnpgv1.Cluster
			g.Expect(k8sClient.Get(ctx, key, &cluster)).To(Succeed())
			g.Expect(cluster.Spec.ImageName).To(HaveSuffix(":17"))

			// That reconcile renders the cluster status.cluster names, which
			// is the one the rollback removed. A guard that gave up puts it
			// back on the major it refuses.
			var replaced cnpgv1.Cluster
			if err := k8sClient.Get(ctx, replacedKey, &replaced); err == nil {
				g.Expect(replaced.Spec.ImageName).To(HaveSuffix(":17"))
			} else {
				g.Expect(apierrors.IsNotFound(err)).To(BeTrue(), err.Error())
			}
		}, 3*time.Second, "5ms").Should(Succeed())

		ready := conditionOf(server, v1.ConditionReady)
		Expect(ready).NotTo(BeNil())
		Expect(ready.Reason).To(Equal(v1.ReasonVersionChangeRefused))
	})

	// A refusal must not stop the server. A rollback in flight has to finish on
	// the major the archive holds, and whoever asked for it waits on the
	// contract for the answer.
	It("finishes a rollback in flight while the version refusal stands", func() {
		server, from := archivingServer()
		askForRecovery(server, from.Add(time.Hour))
		expectRecoveryCluster(server)

		setVersion(server, "18")

		Eventually(func(g Gomega) {
			ready := conditionOf(server, v1.ConditionReady)
			g.Expect(ready).NotTo(BeNil())
			g.Expect(ready.Reason).To(Equal(v1.ReasonVersionChangeRefused))
		}, timeout, interval).Should(Succeed())

		recovered := recoveryCluster(server)
		recoverySucceeds(server)

		expectLastRecovery(server, v1.RecoveryResultCompleted)

		// The contract moved to the recovered server, and the refusal still
		// stands over it.
		Expect(publishedContract(server).Spec.Host).
			To(Equal(recovered + "-rw." + server.Namespace + ".svc"))
		Eventually(func(g Gomega) {
			ready := conditionOf(server, v1.ConditionReady)
			g.Expect(ready).NotTo(BeNil())
			g.Expect(ready.Reason).To(Equal(v1.ReasonVersionChangeRefused))
		}, timeout, interval).Should(Succeed())

		// Both clusters keep the major the archive holds. A rollback onto
		// another major reads nothing.
		key := client.ObjectKey{Namespace: server.Namespace, Name: recovered}
		Consistently(func(g Gomega) {
			var cluster cnpgv1.Cluster
			g.Expect(k8sClient.Get(ctx, key, &cluster)).To(Succeed())
			g.Expect(cluster.Spec.ImageName).To(HaveSuffix(":17"))
		}, 2*time.Second, interval).Should(Succeed())
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
