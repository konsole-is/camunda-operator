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
	"strings"
	"time"

	cnpgv1 "github.com/cloudnative-pg/api/pkg/api/v1"
	"github.com/google/uuid"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	eventsv1 "k8s.io/api/events/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	utilrand "k8s.io/apimachinery/pkg/util/rand"
	"sigs.k8s.io/controller-runtime/pkg/client"

	v1 "github.com/konsole-is/camunda-operator/api/v1"
	components "github.com/konsole-is/camunda-operator/pkg/components/databaseserver"
	"github.com/konsole-is/camunda-operator/pkg/labels"
	"github.com/konsole-is/camunda-operator/pkg/wrappers/barmanobjectstore"
)

// recoveredSystemIdentifier is the identity that CloudNativePG reports for a
// cluster a recovery built. The suite gives it a value of its own, so a spec
// can tell which cluster the server read the identity from.
const recoveredSystemIdentifier = "7000000000000000002"

// testHoldFinalizer keeps a CloudNativePG cluster terminating, so a spec can
// show what the operator does while one is on its way out. envtest runs no
// CloudNativePG, so nothing else holds a deleted cluster.
const testHoldFinalizer = "camunda.io/test-hold"

// archivingServer creates a server that archives to a bucket of its own,
// brings its cluster up, and takes one base backup, so its archive is open and
// a recovery has somewhere to start from. It returns the server and the point
// that base backup completed at.
func archivingServer() (*v1.DatabaseServer, metav1.Time) {
	GinkgoHelper()

	bucket := archiveBucket()
	server := serverInNamespace(&v1.DatabaseServerArchiveSpec{
		ObjectStorageRef:    bucket.Name,
		RetentionPeriodDays: 30,
	})
	writeSuperuserSecret(server)
	makeClusterHealthy(server, "7000000000000000001")

	from := metav1.NewTime(time.Now().Add(-2 * time.Hour).Truncate(time.Second))
	completeBaseBackup(server, "base-1", from)

	Eventually(func() []v1.ArchiveRecord {
		return archiveHistory(server)
	}, timeout, interval).Should(HaveLen(1))

	return server, from
}

// archivingServerIn is archivingServer in a namespace that already exists,
// under a name and a contract name of its own. It writes no superuser Secret,
// so the caller decides whether the contract component of this server blocks
// or publishes. Two of them is how a spec puts two servers on one contract.
func archivingServerIn(namespace, name, contract string, from metav1.Time) *v1.DatabaseServer {
	GinkgoHelper()

	bucket := archiveBucket()
	server := &v1.DatabaseServer{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: namespace},
		Spec: v1.DatabaseServerSpec{
			Version:              "17",
			Instances:            new(int32(1)),
			StorageSize:          new(resource.MustParse("1Gi")),
			DatabaseServerConfig: contract,
			Archive: &v1.DatabaseServerArchiveSpec{
				ObjectStorageRef:    bucket.Name,
				RetentionPeriodDays: 30,
			},
		},
	}
	Expect(k8sClient.Create(ctx, server)).To(Succeed())

	makeClusterHealthy(server, "7000000000000000001")
	completeBaseBackup(server, "base-1", from)

	Eventually(func() []v1.ArchiveRecord {
		return archiveHistory(server)
	}, timeout, interval).Should(HaveLen(1))

	return server
}

// archivingServerOnPreset is archivingServer with the archive block on a
// cluster-scoped preset instead of inline, so a spec can edit the baseline
// that the preset merge reads. It returns the server, the preset, and the
// point its archive opens at.
func archivingServerOnPreset() (*v1.DatabaseServer, *v1.DatabaseServerPreset, metav1.Time) {
	GinkgoHelper()

	bucket := archiveBucket()
	preset := &v1.DatabaseServerPreset{
		ObjectMeta: metav1.ObjectMeta{Name: "dbsp-" + utilrand.String(8)},
		Spec: v1.DatabaseServerPresetSpec{
			Server: v1.DatabaseServerSpec{
				Archive: &v1.DatabaseServerArchiveSpec{
					ObjectStorageRef:    bucket.Name,
					RetentionPeriodDays: 30,
				},
			},
		},
	}
	Expect(k8sClient.Create(ctx, preset)).To(Succeed())
	DeferCleanup(func() { _ = k8sClient.Delete(ctx, preset) })

	server := serverInNamespace(nil)
	Eventually(func(g Gomega) {
		var latest v1.DatabaseServer
		g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(server), &latest)).To(Succeed())
		latest.Spec.PresetRef = preset.Name
		g.Expect(k8sClient.Update(ctx, &latest)).To(Succeed())
	}, timeout, interval).Should(Succeed())

	writeSuperuserSecret(server)
	makeClusterHealthy(server, "7000000000000000001")

	from := metav1.NewTime(time.Now().Add(-2 * time.Hour).Truncate(time.Second))
	completeBaseBackup(server, "base-1", from)

	Eventually(func() []v1.ArchiveRecord {
		return archiveHistory(server)
	}, timeout, interval).Should(HaveLen(1))

	return server, preset, from
}

// askForRecovery writes a recovery request on the contract of the server, the
// way a PointInTimeRestore does. It returns the request it wrote.
func askForRecovery(server *v1.DatabaseServer, target time.Time) v1.RecoveryRequest {
	GinkgoHelper()

	return askForRecoveryOn(server, contractKey(server), target)
}

// askForRecoveryOn writes a recovery request on the named contract.
func askForRecoveryOn(
	server *v1.DatabaseServer,
	key client.ObjectKey,
	target time.Time,
) v1.RecoveryRequest {
	GinkgoHelper()

	request := v1.RecoveryRequest{
		RequestID:   uuid.NewString(),
		RequestedBy: server.Namespace + "/pitr-1",
		TargetTime:  target.UTC().Format(time.RFC3339Nano),
	}
	Eventually(func(g Gomega) {
		var contract v1.DatabaseServerConfig
		g.Expect(k8sClient.Get(ctx, key, &contract)).To(Succeed())
		contract.Spec.Recovery = &request
		g.Expect(k8sClient.Update(ctx, &contract)).To(Succeed())
	}, timeout, interval).Should(Succeed())

	return request
}

// renderedArchive is what the archive of a server looks like on the cluster:
// the retention policy of its ObjectStore, the schedule of its
// ScheduledBackup, and the retention its contract publishes. A hold has to
// keep all three at what the rollback recorded, because the retention prunes
// the base backup the rollback starts from.
type renderedArchive struct {
	retentionPolicy string
	schedule        string
	published       int32
}

// archiveOnCluster reads the rendered archive of a server that has not cut
// over yet, so its ObjectStore and its ScheduledBackup both carry its name.
func archiveOnCluster(g Gomega, server *v1.DatabaseServer) renderedArchive {
	key := client.ObjectKey{Namespace: server.Namespace, Name: server.Name}

	var store barmanobjectstore.ObjectStore
	g.Expect(k8sClient.Get(ctx, key, &store)).To(Succeed())

	var baseBackup cnpgv1.ScheduledBackup
	g.Expect(k8sClient.Get(ctx, key, &baseBackup)).To(Succeed())

	pitr := publishedContract(server).Spec.PITR
	g.Expect(pitr).NotTo(BeNil())
	g.Expect(pitr.Enabled).To(BeTrue(), "the contract keeps advertising the archive the rollback reads")
	g.Expect(pitr.RetentionPeriodDays).NotTo(BeNil())

	return renderedArchive{
		retentionPolicy: store.Spec.RetentionPolicy,
		schedule:        baseBackup.Spec.Schedule,
		published:       *pitr.RetentionPeriodDays,
	}
}

// setBucketRole moves the bucket contract onto workload identity under the
// named IAM role.
func setBucketRole(bucket *v1.ObjectStorageConfig, roleARN string) {
	GinkgoHelper()

	Eventually(func(g Gomega) {
		var latest v1.ObjectStorageConfig
		g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(bucket), &latest)).To(Succeed())
		latest.Spec.S3.Auth = v1.S3StorageAuth{
			Type:             v1.ObjectStorageAuthTypeWorkloadIdentity,
			WorkloadIdentity: &v1.S3WorkloadIdentity{RoleARN: roleARN},
		}
		g.Expect(k8sClient.Update(ctx, &latest)).To(Succeed())
	}, timeout, interval).Should(Succeed())
}

// clusterRole reads the IAM role that the named CloudNativePG cluster gives
// its instance pods, or the empty string when it gives them none.
func clusterRole(g Gomega, server *v1.DatabaseServer, name string) string {
	var cluster cnpgv1.Cluster
	g.Expect(k8sClient.Get(
		ctx, client.ObjectKey{Namespace: server.Namespace, Name: name}, &cluster,
	)).To(Succeed())
	if cluster.Spec.ServiceAccountTemplate == nil {
		return ""
	}

	return cluster.Spec.ServiceAccountTemplate.Metadata.Annotations[v1.IRSARoleARNAnnotation]
}

// probeContract records the probe that the DatabaseServerConfig controller
// writes once it reached the server the contract names. That controller does
// not run in this suite, and the server waits for the probe before it declares
// a rollback complete.
//
// It waits for the contract to name the recovered server first. A probe
// recorded against the endpoint of before the rollback says nothing about the
// one after it, and the server reads the generation to tell the two apart.
func probeContract(server *v1.DatabaseServer) {
	GinkgoHelper()

	Eventually(func(g Gomega) {
		var contract v1.DatabaseServerConfig
		g.Expect(k8sClient.Get(ctx, contractKey(server), &contract)).To(Succeed())
		g.Expect(contract.Spec.Host).To(Equal(recoveryHost(server)))
		contract.Status.SystemIdentifier = recoveredSystemIdentifier
		contract.Status.ObservedGeneration = contract.Generation
		g.Expect(k8sClient.Status().Update(ctx, &contract)).To(Succeed())
	}, timeout, interval).Should(Succeed())
}

// probeAnyEndpoint records the probe against whatever endpoint the contract
// names at that moment. probeContract waits for the endpoint of the running
// recovery first, and a spec whose server abandons that recovery cannot wait
// for an endpoint that moves back while it waits.
func probeAnyEndpoint(server *v1.DatabaseServer) {
	GinkgoHelper()

	Eventually(func(g Gomega) {
		var contract v1.DatabaseServerConfig
		g.Expect(k8sClient.Get(ctx, contractKey(server), &contract)).To(Succeed())
		contract.Status.SystemIdentifier = recoveredSystemIdentifier
		contract.Status.ObservedGeneration = contract.Generation
		g.Expect(k8sClient.Status().Update(ctx, &contract)).To(Succeed())
	}, timeout, interval).Should(Succeed())
}

// contractKey is the contract that the server publishes.
func contractKey(server *v1.DatabaseServer) client.ObjectKey {
	return client.ObjectKey{Namespace: server.Namespace, Name: "camunda"}
}

// publishedContract reads the contract of the server.
func publishedContract(server *v1.DatabaseServer) *v1.DatabaseServerConfig {
	GinkgoHelper()

	var contract v1.DatabaseServerConfig
	Expect(k8sClient.Get(ctx, contractKey(server), &contract)).To(Succeed())

	return &contract
}

// reconciledServer reads the server as the operator left it.
func reconciledServer(server *v1.DatabaseServer) *v1.DatabaseServer {
	GinkgoHelper()

	var latest v1.DatabaseServer
	Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(server), &latest)).To(Succeed())

	return &latest
}

// expectLastRecovery waits until the contract publishes an outcome with the
// given result, and returns it.
func expectLastRecovery(server *v1.DatabaseServer, result v1.RecoveryResult) *v1.RecoveryOutcome {
	GinkgoHelper()

	Eventually(func(g Gomega) {
		contract := publishedContract(server)
		g.Expect(contract.Spec.PITR).NotTo(BeNil())
		g.Expect(contract.Spec.PITR.LastRecovery).NotTo(BeNil())
		g.Expect(contract.Spec.PITR.LastRecovery.Result).To(Equal(result))
	}, timeout, interval).Should(Succeed())

	return publishedContract(server).Spec.PITR.LastRecovery
}

// recoveryCluster is the CloudNativePG cluster that the recorded recovery
// builds.
func recoveryCluster(server *v1.DatabaseServer) string {
	GinkgoHelper()

	var name string
	Eventually(func(g Gomega) {
		recovery := reconciledServer(server).Status.Recovery
		g.Expect(recovery).NotTo(BeNil())
		g.Expect(recovery.Cluster).NotTo(BeEmpty())
		name = recovery.Cluster
	}, timeout, interval).Should(Succeed())

	return name
}

// recoveryHost is the endpoint of the cluster that the recorded recovery
// builds.
func recoveryHost(server *v1.DatabaseServer) string {
	GinkgoHelper()

	return recoveryCluster(server) + "-rw." + server.Namespace + ".svc"
}

// bringRecoveryClusterUp writes the Secret and the status that CloudNativePG
// writes for a recovery cluster that finished its bootstrap.
func bringRecoveryClusterUp(server *v1.DatabaseServer, name string) {
	GinkgoHelper()

	Expect(k8sClient.Create(ctx, &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: name + "-superuser", Namespace: server.Namespace},
		Data:       map[string][]byte{"username": []byte("postgres"), "password": []byte("s3cret")},
	})).To(Succeed())

	setRecoveryClusterPhase(server, name, cnpgv1.PhaseHealthy, recoveredSystemIdentifier)
}

// recoverySucceeds brings the recovery cluster of the server up and records the
// probe of the endpoint it moves the contract to.
func recoverySucceeds(server *v1.DatabaseServer) {
	GinkgoHelper()

	bringRecoveryClusterUp(server, recoveryCluster(server))
	probeContract(server)
}

// setRecoveryClusterPhase writes the phase that CloudNativePG reports for the
// recovery cluster, once the operator has created it.
func setRecoveryClusterPhase(server *v1.DatabaseServer, name, phase, systemID string) {
	GinkgoHelper()

	key := client.ObjectKey{Namespace: server.Namespace, Name: name}
	Eventually(func(g Gomega) {
		var cluster cnpgv1.Cluster
		g.Expect(k8sClient.Get(ctx, key, &cluster)).To(Succeed())
		cluster.Status.Phase = phase
		cluster.Status.ReadyInstances = cluster.Spec.Instances
		cluster.Status.SystemID = systemID
		// A cluster that a spec wrote by hand carries no image of ours, and
		// CloudNativePG reports the major of a data directory it wrote.
		if major := imageMajorVersion(cluster.Spec.ImageName); major > 0 {
			cluster.Status.PGDataImageInfo = &cnpgv1.ImageInfo{
				Image: cluster.Spec.ImageName, MajorVersion: major,
			}
		}
		g.Expect(k8sClient.Status().Update(ctx, &cluster)).To(Succeed())
	}, timeout, interval).Should(Succeed())
}

// expectRecoveryCluster waits until the operator built the cluster that the
// recorded recovery names.
func expectRecoveryCluster(server *v1.DatabaseServer) {
	GinkgoHelper()

	key := client.ObjectKey{Namespace: server.Namespace, Name: recoveryCluster(server)}
	Eventually(func() error {
		return k8sClient.Get(ctx, key, &cnpgv1.Cluster{})
	}, timeout, interval).Should(Succeed())
}

// expectAbsent asserts that no CloudNativePG cluster of the given name exists.
func expectAbsent(server *v1.DatabaseServer, name string) {
	GinkgoHelper()

	key := client.ObjectKey{Namespace: server.Namespace, Name: name}
	Expect(k8sClient.Get(ctx, key, &cnpgv1.Cluster{})).To(MatchError(apierrors.IsNotFound, "not found"))
}

// holdRecoveryCluster puts a finalizer on the recovery cluster, so a delete
// leaves it terminating the way CloudNativePG does while it takes the
// instances down.
func holdRecoveryCluster(server *v1.DatabaseServer, name string) {
	GinkgoHelper()

	key := client.ObjectKey{Namespace: server.Namespace, Name: name}
	Eventually(func(g Gomega) {
		var cluster cnpgv1.Cluster
		g.Expect(k8sClient.Get(ctx, key, &cluster)).To(Succeed())
		cluster.Finalizers = append(cluster.Finalizers, testHoldFinalizer)
		g.Expect(k8sClient.Update(ctx, &cluster)).To(Succeed())
	}, timeout, interval).Should(Succeed())
}

// releaseRecoveryCluster takes the finalizer off, so the cluster goes.
func releaseRecoveryCluster(server *v1.DatabaseServer, name string) {
	GinkgoHelper()

	key := client.ObjectKey{Namespace: server.Namespace, Name: name}
	Eventually(func(g Gomega) {
		var cluster cnpgv1.Cluster
		g.Expect(k8sClient.Get(ctx, key, &cluster)).To(Succeed())
		cluster.Finalizers = nil
		g.Expect(k8sClient.Update(ctx, &cluster)).To(Succeed())
	}, timeout, interval).Should(Succeed())
}

// expectTerminating waits until the named cluster carries a deletion stamp.
func expectTerminating(server *v1.DatabaseServer, name string) {
	GinkgoHelper()

	key := client.ObjectKey{Namespace: server.Namespace, Name: name}
	Eventually(func(g Gomega) {
		var cluster cnpgv1.Cluster
		g.Expect(k8sClient.Get(ctx, key, &cluster)).To(Succeed())
		g.Expect(cluster.DeletionTimestamp.IsZero()).To(BeFalse())
	}, timeout, interval).Should(Succeed())
}

// recoveryEventReasons lists the reasons of the events that the operator
// recorded about the server.
func recoveryEventReasons(server *v1.DatabaseServer) []string {
	GinkgoHelper()

	var events eventsv1.EventList
	Expect(k8sClient.List(ctx, &events, client.InNamespace(server.Namespace))).To(Succeed())

	reasons := make([]string, 0, len(events.Items))
	for _, event := range events.Items {
		if event.Regarding.Name != server.Name {
			continue
		}
		reasons = append(reasons, event.Reason)
	}

	return reasons
}

// expectGone waits until obj no longer exists.
func expectGone(key client.ObjectKey, obj client.Object) {
	GinkgoHelper()

	Eventually(func() bool {
		return apierrors.IsNotFound(k8sClient.Get(ctx, key, obj))
	}, timeout, interval).Should(BeTrue())
}

var _ = Describe("DatabaseServer recovery", func() {
	It("recovers the server to the requested point and repoints its contract", func() {
		server, from := archivingServer()
		target := from.Add(time.Hour)
		request := askForRecovery(server, target)

		By("recording the request before it builds anything")
		Eventually(func(g Gomega) {
			recovery := reconciledServer(server).Status.Recovery
			g.Expect(recovery).NotTo(BeNil())
			g.Expect(recovery.RequestID).To(Equal(request.RequestID))
			g.Expect(recovery.RequestedBy).To(Equal(request.RequestedBy))
			g.Expect(recovery.Contract).To(Equal("camunda"))
			g.Expect(recovery.Cluster).To(Equal("camunda-r1"))
			g.Expect(recovery.CompletedAt).To(BeNil())
		}, timeout, interval).Should(Succeed())

		By("building a cluster that recovers from the archive of the cluster it replaces")
		var recovered cnpgv1.Cluster
		key := client.ObjectKey{Namespace: server.Namespace, Name: "camunda-r1"}
		Eventually(func() error {
			return k8sClient.Get(ctx, key, &recovered)
		}, timeout, interval).Should(Succeed())
		Expect(recovered.Spec.Bootstrap.Recovery.Source).To(Equal("camunda"))
		Expect(recovered.Spec.Bootstrap.Recovery.RecoveryTarget.TargetTime).
			To(Equal(target.UTC().Format(time.RFC3339)))
		Expect(recovered.Spec.Plugins[0].Parameters).To(HaveKeyWithValue("serverName", "camunda-r1"))
		Expect(recovered.Spec.ExternalClusters[0].PluginConfiguration.Parameters).
			To(HaveKeyWithValue("serverName", "camunda"))

		By("waiting for CloudNativePG before it touches the contract")
		Consistently(func() string {
			return publishedContract(server).Spec.Host
		}, "1s", interval).Should(Equal("camunda-rw." + server.Namespace + ".svc"))

		bringRecoveryClusterUp(server, "camunda-r1")

		By("pointing the contract at the recovered server")
		Eventually(func(g Gomega) {
			contract := publishedContract(server)
			g.Expect(contract.Spec.Host).To(Equal("camunda-r1-rw." + server.Namespace + ".svc"))
			g.Expect(contract.Spec.AdminCredentialsSecretRef.Name).To(Equal("camunda-r1-superuser"))
		}, timeout, interval).Should(Succeed())

		By("waiting for the contract to reach the server it names now")
		Consistently(func() *v1.RecoveryOutcome {
			contract := publishedContract(server)
			if contract.Spec.PITR == nil {
				return nil
			}

			return contract.Spec.PITR.LastRecovery
		}, "1s", interval).Should(BeNil())

		probeContract(server)

		By("publishing the outcome")
		outcome := expectLastRecovery(server, v1.RecoveryResultCompleted)
		Expect(outcome.RequestID).To(Equal(request.RequestID))
		Expect(outcome.RequestedBy).To(Equal(request.RequestedBy))
		Expect(outcome.TargetTime).To(Equal(request.TargetTime))

		By("removing the cluster and the base backup schedule it replaced")
		expectGone(client.ObjectKey{Namespace: server.Namespace, Name: "camunda"}, &cnpgv1.Cluster{})
		expectGone(
			client.ObjectKey{Namespace: server.Namespace, Name: "camunda"}, &cnpgv1.ScheduledBackup{},
		)

		var schedule cnpgv1.ScheduledBackup
		Eventually(func() error {
			return k8sClient.Get(
				ctx, client.ObjectKey{Namespace: server.Namespace, Name: "camunda-r1"}, &schedule,
			)
		}, timeout, interval).Should(Succeed())
		Expect(schedule.Spec.Cluster.Name).To(Equal("camunda-r1"))

		By("recording the answer, so the request is not answered twice")
		Eventually(func(g Gomega) {
			latest := reconciledServer(server)
			g.Expect(latest.Status.Cluster).To(Equal("camunda-r1"))
			g.Expect(latest.Status.SystemIdentifier).To(Equal(recoveredSystemIdentifier))
			g.Expect(latest.Status.Recovery.CompletedAt).NotTo(BeNil())
			// The answer keeps the whole record: nothing else holds the
			// cluster the server came from or the archive it read.
			g.Expect(latest.Status.Recovery.PreviousCluster).To(Equal("camunda"))
			g.Expect(latest.Status.Recovery.Archive).NotTo(BeNil())
		}, timeout, interval).Should(Succeed())

		By("closing the archive it recovered from at the moment the contract moved")
		var closedAt metav1.Time
		Eventually(func(g Gomega) {
			history := archiveHistory(server)
			g.Expect(history).To(HaveLen(1))
			g.Expect(history[0].ServerName).To(Equal("camunda"))
			g.Expect(history[0].To).NotTo(BeNil())
			closedAt = *history[0].To
		}, timeout, interval).Should(Succeed())

		By("opening the archive it writes now at its first base backup")
		firstBackup := metav1.NewTime(closedAt.Add(time.Minute).Truncate(time.Second))
		completeBaseBackup(reconciledServer(server), "base-2", firstBackup)
		Eventually(func(g Gomega) {
			history := archiveHistory(server)
			g.Expect(history).To(HaveLen(2))
			g.Expect(history[1].ServerName).To(Equal("camunda-r1"))
			g.Expect(history[1].From.Time).To(BeTemporally("==", firstBackup.Time))
		}, timeout, interval).Should(Succeed())

		By("keeping the outcome and the request on the contract it republishes")
		Consistently(func(g Gomega) {
			contract := publishedContract(server)
			g.Expect(contract.Spec.PITR.LastRecovery).NotTo(BeNil())
			g.Expect(contract.Spec.PITR.LastRecovery.Result).To(Equal(v1.RecoveryResultCompleted))
			g.Expect(contract.Spec.Recovery).NotTo(BeNil())
		}, "2s", interval).Should(Succeed())

		// The old cluster stopped writing at the cutover and the new archive
		// reaches no point before its first base backup, so the window between
		// the two lies in no interval.
		gap := askForRecovery(server, closedAt.Add(30*time.Second))
		Eventually(func(g Gomega) {
			outcome := publishedContract(server).Spec.PITR.LastRecovery
			g.Expect(outcome).NotTo(BeNil())
			g.Expect(outcome.RequestID).To(Equal(gap.RequestID))
			g.Expect(outcome.Result).To(Equal(v1.RecoveryResultUnavailable))
		}, timeout, interval).Should(Succeed())

	})

	It("refuses a point that no archive of the server holds", func() {
		server, from := archivingServer()

		request := askForRecovery(server, from.Add(-time.Hour))

		outcome := expectLastRecovery(server, v1.RecoveryResultUnavailable)
		Expect(outcome.RequestID).To(Equal(request.RequestID))
		Expect(outcome.Message).To(ContainSubstring("lies in none of those windows"))

		// Nothing was built, so nothing has to be cleaned up.
		Expect(reconciledServer(server).Status.Recovery.Cluster).To(BeEmpty())
		Expect(k8sClient.Get(
			ctx, client.ObjectKey{Namespace: server.Namespace, Name: "camunda-r1"}, &cnpgv1.Cluster{},
		)).To(MatchError(apierrors.IsNotFound, "not found"))
	})

	It("keeps waiting while CloudNativePG reports a plugin phase it retries", func() {
		server, from := archivingServer()
		askForRecovery(server, from.Add(time.Hour))
		expectRecoveryCluster(server)

		// CloudNativePG registers this phase on a plugin error and requeues
		// in seconds. A recovery cluster that is seconds old reports it while
		// the plugin loads, so grading it as a failure answers the request
		// from a state the operator recovers from on its own.
		By("reporting a plugin error on the cluster the recovery built")
		key := client.ObjectKey{Namespace: server.Namespace, Name: "camunda-r1"}
		Eventually(func(g Gomega) {
			var recovered cnpgv1.Cluster
			g.Expect(k8sClient.Get(ctx, key, &recovered)).To(Succeed())
			recovered.Status.Phase = cnpgv1.PhaseFailurePlugin
			recovered.Status.ReadyInstances = 0
			g.Expect(k8sClient.Status().Update(ctx, &recovered)).To(Succeed())
		}, timeout, interval).Should(Succeed())

		By("answering nothing and keeping the cluster")
		Consistently(func(g Gomega) {
			contract := publishedContract(server)
			if contract.Spec.PITR != nil {
				g.Expect(contract.Spec.PITR.LastRecovery).To(BeNil())
			}
			g.Expect(k8sClient.Get(ctx, key, &cnpgv1.Cluster{})).To(Succeed())
		}, "2s", interval).Should(Succeed())

		By("completing once the plugin loads and the instances are ready")
		recoverySucceeds(server)

		outcome := expectLastRecovery(server, v1.RecoveryResultCompleted)
		Expect(outcome.Result).To(Equal(v1.RecoveryResultCompleted))
		Expect(publishedContract(server).Spec.Host).
			To(Equal("camunda-r1-rw." + server.Namespace + ".svc"))
	})

	It("reports a recovery that CloudNativePG cannot finish, and removes what it built", func() {
		server, from := archivingServer()
		askForRecovery(server, from.Add(time.Hour))

		setRecoveryClusterPhase(server, "camunda-r1", cnpgv1.PhaseUnrecoverable, "")

		outcome := expectLastRecovery(server, v1.RecoveryResultFailed)
		Expect(outcome.Message).To(ContainSubstring(cnpgv1.PhaseUnrecoverable))

		expectGone(
			client.ObjectKey{Namespace: server.Namespace, Name: "camunda-r1"}, &cnpgv1.Cluster{},
		)

		// The server the restore started from is untouched.
		var running cnpgv1.Cluster
		Expect(k8sClient.Get(
			ctx, client.ObjectKey{Namespace: server.Namespace, Name: "camunda"}, &running,
		)).To(Succeed())
		Expect(publishedContract(server).Spec.Host).To(Equal("camunda-rw." + server.Namespace + ".svc"))
	})

	It("takes no request from a contract that says nobody rolls the server back", func() {
		server := serverInNamespace(nil)
		writeSuperuserSecret(server)
		makeClusterHealthy(server, "7000000000000000001")
		expectCondition(server, v1.ConditionContractReady, metav1.ConditionTrue)

		// A server without an archive publishes pitr.recovery: external. A
		// request on that contract was written against a server that never
		// offered to roll itself back, so the server does not take it.
		Expect(publishedContract(server).Spec.PITR.Recovery).To(Equal(v1.RecoveryModeExternal))
		askForRecovery(server, time.Now().Add(-time.Hour))

		Consistently(func(g Gomega) {
			contract := publishedContract(server)
			g.Expect(contract.Spec.PITR.LastRecovery).To(BeNil())
			g.Expect(reconciledServer(server).Status.Recovery).To(BeNil())
		}, "2s", interval).Should(Succeed())
	})

	It("takes no request from a contract that another server owns", func() {
		namespace := "dbs-" + utilrand.String(8)
		Expect(k8sClient.Create(ctx, &corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{Name: namespace},
		})).To(Succeed())

		from := metav1.NewTime(time.Now().Add(-2 * time.Hour).Truncate(time.Second))
		shared := client.ObjectKey{Namespace: namespace, Name: "shared"}

		// The first server publishes the contract, so it is the controller of
		// it.
		owner := archivingServerIn(namespace, "camunda", "shared", from)
		writeSuperuserSecret(owner)
		expectCondition(owner, v1.ConditionContractReady, metav1.ConditionTrue)

		// The second names the same contract. The guard of its contract
		// component blocks while the first server holds the name, so it
		// publishes nothing and the contract stays as the first wrote it.
		loser := archivingServerIn(namespace, "second", "shared", from)
		writeSuperuserSecret(loser)
		Eventually(func(g Gomega) {
			var contract v1.DatabaseServerConfig
			g.Expect(k8sClient.Get(ctx, shared, &contract)).To(Succeed())
			g.Expect(metav1.IsControlledBy(&contract, reconciledServer(owner))).To(BeTrue())
			g.Expect(metav1.IsControlledBy(&contract, reconciledServer(loser))).To(BeFalse())
		}, timeout, interval).Should(Succeed())

		askForRecoveryOn(owner, shared, from.Add(time.Hour))

		Eventually(func(g Gomega) {
			recovery := reconciledServer(owner).Status.Recovery
			g.Expect(recovery).NotTo(BeNil())
			g.Expect(recovery.Cluster).To(Equal("camunda-r1"))
		}, timeout, interval).Should(Succeed())

		// The request names the archive and the endpoint of the first server.
		// A server that reads it off a contract it does not own recovers its
		// own archive and answers a question nobody asked it.
		Consistently(func(g Gomega) {
			g.Expect(reconciledServer(loser).Status.Recovery).To(BeNil())
			g.Expect(k8sClient.Get(
				ctx, client.ObjectKey{Namespace: namespace, Name: "second-r1"}, &cnpgv1.Cluster{},
			)).To(MatchError(apierrors.IsNotFound, "not found"))

			var contract v1.DatabaseServerConfig
			g.Expect(k8sClient.Get(ctx, shared, &contract)).To(Succeed())
			g.Expect(contract.Spec.PITR).NotTo(BeNil())
			g.Expect(contract.Spec.PITR.LastRecovery).To(BeNil())
		}, "2s", interval).Should(Succeed())
	})

	It("keeps the archive of a running recovery that the spec takes away", func() {
		server, from := archivingServer()
		request := askForRecovery(server, from.Add(time.Hour))
		expectRecoveryCluster(server)

		recorded := reconciledServer(server).Status.Recovery.Archive
		Expect(recorded).NotTo(BeNil())

		storeKey := client.ObjectKey{Namespace: server.Namespace, Name: "camunda"}
		var store barmanobjectstore.ObjectStore
		Expect(k8sClient.Get(ctx, storeKey, &store)).To(Succeed())
		reading := store.Spec.Configuration.DestinationPath
		rendered := archiveOnCluster(Default, server)

		By("removing the archive while the rollback is unanswered")
		setArchive(server, nil)

		ready := expectCondition(server, v1.ConditionReady, metav1.ConditionFalse)
		Expect(ready.Reason).To(Equal(v1.ReasonInvalidReference))
		Expect(ready.Message).To(ContainSubstring("Put spec.archive back"))

		// The rollback recovers out of this archive, so the removal reaches
		// neither the archive it reads nor the record that holds the point it
		// asked for, and nobody is answered that there is no archive.
		Consistently(func(g Gomega) {
			g.Expect(k8sClient.Get(ctx, storeKey, &store)).To(Succeed())
			g.Expect(store.Spec.Configuration.DestinationPath).To(Equal(reading))
			g.Expect(archiveOnCluster(g, server)).To(Equal(rendered))

			archive := reconciledServer(server).Status.Archive
			g.Expect(archive).NotTo(BeNil())
			g.Expect(archive.History).To(HaveLen(1))
			g.Expect(archive.History[0].To).To(BeNil())

			g.Expect(publishedContract(server).Spec.PITR.LastRecovery).To(BeNil())
		}, "2s", interval).Should(Succeed())

		By("finishing the rollback the removal was held for")
		recoverySucceeds(server)

		outcome := expectLastRecovery(server, v1.RecoveryResultCompleted)
		Expect(outcome.RequestID).To(Equal(request.RequestID))

		By("applying the removal once the rollback is answered")
		expectGone(storeKey, &barmanobjectstore.ObjectStore{})
		Eventually(func(g Gomega) {
			contract := publishedContract(server)
			g.Expect(contract.Spec.PITR).NotTo(BeNil())
			g.Expect(contract.Spec.PITR.Enabled).To(BeFalse())
		}, timeout, interval).Should(Succeed())
	})

	It("keeps building the cluster it recorded when the archive history moves", func() {
		server, from := archivingServer()
		askForRecovery(server, from.Add(time.Hour))

		expectRecoveryCluster(server)

		// The name of the recovery cluster is derived from the number of
		// archives. A recovery that read it again after the history grew would
		// build a second cluster and abandon the first.
		Eventually(func(g Gomega) {
			var latest v1.DatabaseServer
			g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(server), &latest)).To(Succeed())
			latest.Status.Archive.History = append(latest.Status.Archive.History, v1.ArchiveRecord{
				ServerName: "camunda-x",
				From:       from,
				To:         &from,
			})
			g.Expect(k8sClient.Status().Update(ctx, &latest)).To(Succeed())
		}, timeout, interval).Should(Succeed())

		recoverySucceeds(server)
		expectLastRecovery(server, v1.RecoveryResultCompleted)

		Expect(reconciledServer(server).Status.Cluster).To(Equal("camunda-r1"))
		expectAbsent(server, "camunda-r2")
	})

	It("keeps the answer of a refusal when its status write is lost", func() {
		server, from := archivingServer()
		askForRecovery(server, from.Add(time.Hour))

		setRecoveryClusterPhase(server, "camunda-r1", cnpgv1.PhaseUnrecoverable, "")
		expectLastRecovery(server, v1.RecoveryResultFailed)

		By("losing the record of the answer")
		Eventually(func(g Gomega) {
			var latest v1.DatabaseServer
			g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(server), &latest)).To(Succeed())
			latest.Status.Recovery.CompletedAt = nil
			latest.Status.Recovery.Result = ""
			g.Expect(k8sClient.Status().Update(ctx, &latest)).To(Succeed())
		}, timeout, interval).Should(Succeed())

		// The answer is published before the cluster it abandons goes, so the
		// look that finds the record incomplete finds the cluster too. It
		// answers again rather than building a second one.
		Eventually(func(g Gomega) {
			g.Expect(reconciledServer(server).Status.Recovery.CompletedAt).NotTo(BeNil())
		}, timeout, interval).Should(Succeed())
		Expect(expectLastRecovery(server, v1.RecoveryResultFailed).Message).
			To(ContainSubstring(cnpgv1.PhaseUnrecoverable))

		// The cluster of the refused recovery stays gone. A look that read the
		// record as unanswered builds it again under the same name.
		key := client.ObjectKey{Namespace: server.Namespace, Name: "camunda-r1"}
		Consistently(func() error {
			return k8sClient.Get(ctx, key, &cnpgv1.Cluster{})
		}, "2s", interval).ShouldNot(Succeed())
	})

	It("publishes the answer again on a contract that was replaced under its name", func() {
		server, from := archivingServer()
		request := askForRecovery(server, from.Add(-time.Hour))
		expectLastRecovery(server, v1.RecoveryResultUnavailable)

		By("losing the answer that the contract carried")
		Eventually(func(g Gomega) {
			var contract v1.DatabaseServerConfig
			g.Expect(k8sClient.Get(ctx, contractKey(server), &contract)).To(Succeed())
			contract.Spec.PITR.LastRecovery = nil
			g.Expect(k8sClient.Update(ctx, &contract)).To(Succeed())
		}, timeout, interval).Should(Succeed())

		outcome := expectLastRecovery(server, v1.RecoveryResultUnavailable)
		Expect(outcome.RequestID).To(Equal(request.RequestID))
		Expect(outcome.Message).To(ContainSubstring("lies in none of those windows"))
	})

	It("finishes the recovery it started before it answers one that arrived later", func() {
		server, from := archivingServer()
		first := askForRecovery(server, from.Add(time.Hour))

		expectRecoveryCluster(server)

		By("asking for another point while the first recovery builds")
		second := askForRecovery(server, from.Add(90*time.Minute))
		Consistently(func() string {
			return reconciledServer(server).Status.Recovery.RequestID
		}, "1s", interval).Should(Equal(first.RequestID))

		recoverySucceeds(server)

		By("answering the first, then starting the second under a name of its own")
		Eventually(func(g Gomega) {
			outcome := publishedContract(server).Spec.PITR.LastRecovery
			g.Expect(outcome).NotTo(BeNil())
			g.Expect(outcome.RequestID).To(Equal(first.RequestID))
			g.Expect(outcome.Result).To(Equal(v1.RecoveryResultCompleted))
		}, timeout, interval).Should(Succeed())

		completeBaseBackup(reconciledServer(server), "base-2", metav1.Now())
		Eventually(func(g Gomega) {
			recovery := reconciledServer(server).Status.Recovery
			g.Expect(recovery.RequestID).To(Equal(second.RequestID))
			g.Expect(recovery.Cluster).NotTo(Equal("camunda-r1"))
		}, timeout, interval).Should(Succeed())
	})

	It("refuses a recovery whose name is taken by a cluster it does not own", func() {
		server, from := archivingServer()

		// It carries the label of this server and no owner reference. A label
		// is a value anybody can write, so it is not what says who owns a
		// cluster that holds a database.
		Expect(k8sClient.Create(ctx, &cnpgv1.Cluster{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "camunda-r1",
				Namespace: server.Namespace,
				Labels:    map[string]string{labels.DatabaseServerKey: server.Name},
			},
			Spec: cnpgv1.ClusterSpec{
				Instances:            1,
				StorageConfiguration: cnpgv1.StorageConfiguration{Size: "1Gi"},
			},
		})).To(Succeed())

		askForRecovery(server, from.Add(time.Hour))

		outcome := expectLastRecovery(server, v1.RecoveryResultFailed)
		Expect(outcome.Message).To(ContainSubstring("does not own it"))

		// The occupant holds somebody else's data. It stays.
		Expect(k8sClient.Get(
			ctx, client.ObjectKey{Namespace: server.Namespace, Name: "camunda-r1"}, &cnpgv1.Cluster{},
		)).To(Succeed())
	})

	It("abandons a rollback whose cluster another owner took after the cutover", func() {
		server, from := archivingServer()
		askForRecovery(server, from.Add(time.Hour))
		bringRecoveryClusterUp(server, "camunda-r1")

		By("waiting until the contract points at the recovered cluster")
		host := "camunda-r1-rw." + server.Namespace + ".svc"
		Eventually(func(g Gomega) {
			g.Expect(reconciledServer(server).Status.Cluster).To(Equal("camunda-r1"))
			g.Expect(publishedContract(server).Spec.Host).To(Equal(host))
		}, timeout, interval).Should(Succeed())

		// What a delete and a create under the derived name leaves behind: a
		// converged cluster of that name that holds the database of somebody
		// else. The name of a recovery cluster comes back once the number of
		// archives comes back, so it is a name anybody can take.
		By("giving the recovered cluster to another server")
		stranger := serverNamed(server.Namespace, "stranger", "stranger", nil)
		key := client.ObjectKey{Namespace: server.Namespace, Name: "camunda-r1"}
		Eventually(func(g Gomega) {
			var taken cnpgv1.Cluster
			g.Expect(k8sClient.Get(ctx, key, &taken)).To(Succeed())
			taken.OwnerReferences = []metav1.OwnerReference{{
				APIVersion: v1.GroupVersion.String(),
				Kind:       "DatabaseServer",
				Name:       stranger.Name,
				UID:        reconciledServer(stranger).UID,
				Controller: new(true),
			}}
			g.Expect(k8sClient.Update(ctx, &taken)).To(Succeed())
		}, timeout, interval).Should(Succeed())

		// The probe is the last thing the server waits for before it declares
		// a rollback complete, so writing it leaves the ownership test as the
		// only thing that can stop the cutover.
		probeAnyEndpoint(server)

		outcome := expectLastRecovery(server, v1.RecoveryResultFailed)
		Expect(outcome.Message).To(ContainSubstring("does not own it"))

		By("running from the cluster it came from again")
		Eventually(func(g Gomega) {
			g.Expect(reconciledServer(server).Status.Cluster).To(Equal("camunda"))
			g.Expect(publishedContract(server).Spec.Host).
				To(Equal("camunda-rw." + server.Namespace + ".svc"))
		}, timeout, interval).Should(Succeed())

		Consistently(func(g Gomega) {
			// The cluster that holds the data of this server is still there.
			g.Expect(k8sClient.Get(
				ctx, client.ObjectKey{Namespace: server.Namespace, Name: "camunda"}, &cnpgv1.Cluster{},
			)).To(Succeed())

			// So is the one it does not own, with the owner it was given.
			var taken cnpgv1.Cluster
			g.Expect(k8sClient.Get(ctx, key, &taken)).To(Succeed())
			g.Expect(metav1.IsControlledBy(&taken, reconciledServer(stranger))).To(BeTrue())
		}, "2s", interval).Should(Succeed())
	})

	It("refuses the cluster it goes back to when another owner took that one", func() {
		server, from := archivingServer()
		askForRecovery(server, from.Add(time.Hour))
		bringRecoveryClusterUp(server, "camunda-r1")

		Eventually(func() string {
			return reconciledServer(server).Status.Cluster
		}, timeout, interval).Should(Equal("camunda-r1"))

		// Nothing applies over the cluster the rollback replaced while the
		// rollback runs, so it keeps whatever owner it is given here.
		By("taking the previous cluster away from the server")
		previous := client.ObjectKey{Namespace: server.Namespace, Name: "camunda"}
		Eventually(func(g Gomega) {
			var cluster cnpgv1.Cluster
			g.Expect(k8sClient.Get(ctx, previous, &cluster)).To(Succeed())
			cluster.OwnerReferences = nil
			g.Expect(k8sClient.Update(ctx, &cluster)).To(Succeed())
		}, timeout, interval).Should(Succeed())

		// The server goes back to the previous cluster in the same pass that
		// abandons the rollback, so that pass is the one that has to read the
		// name it goes back to.
		By("removing the cluster the rollback built")
		Expect(k8sClient.Delete(ctx, &cnpgv1.Cluster{
			ObjectMeta: metav1.ObjectMeta{Name: "camunda-r1", Namespace: server.Namespace},
		})).To(Succeed())

		// The answer is read off the record, not off the contract. The server
		// goes back to a name it does not own, and it withdraws the contract
		// with everything else that names that cluster.
		Eventually(func(g Gomega) {
			recorded := reconciledServer(server).Status.Recovery
			g.Expect(recorded).NotTo(BeNil())
			g.Expect(recorded.CompletedAt).NotTo(BeNil())
			g.Expect(recorded.Result).To(Equal(v1.RecoveryResultFailed))
			g.Expect(recorded.Message).To(ContainSubstring("was removed"))
		}, timeout, interval).Should(Succeed())

		taken := expectCondition(server, v1.ConditionClusterReady, metav1.ConditionFalse)
		Expect(taken.Reason).To(Equal(v1.ReasonClusterTaken))
		Expect(taken.Message).To(ContainSubstring(`CloudNativePG cluster "camunda"`))

		Consistently(func(g Gomega) {
			var cluster cnpgv1.Cluster
			g.Expect(k8sClient.Get(ctx, previous, &cluster)).To(Succeed())
			g.Expect(cluster.OwnerReferences).To(BeEmpty())
		}, "2s", interval).Should(Succeed())
	})

	It("holds the contract it publishes until the recovery on it is answered", func() {
		server, from := archivingServer()
		askForRecovery(server, from.Add(time.Hour))

		expectRecoveryCluster(server)

		By("naming another contract while the recovery runs")
		renamed := "camunda-" + strings.ToLower(utilrand.String(5))
		Eventually(func(g Gomega) {
			var latest v1.DatabaseServer
			g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(server), &latest)).To(Succeed())
			latest.Spec.DatabaseServerConfig = renamed
			g.Expect(k8sClient.Update(ctx, &latest)).To(Succeed())
		}, timeout, interval).Should(Succeed())

		ready := expectCondition(server, v1.ConditionReady, metav1.ConditionFalse)
		Expect(ready.Message).To(ContainSubstring("Set spec.databaseServerConfig back"))

		// Nothing is published under the new name while the rollback runs.
		renamedKey := client.ObjectKey{Namespace: server.Namespace, Name: renamed}
		Consistently(func() error {
			return k8sClient.Get(ctx, renamedKey, &v1.DatabaseServerConfig{})
		}, "1s", interval).ShouldNot(Succeed())

		By("finishing the recovery on the contract that asked for it")
		recoverySucceeds(server)
		expectLastRecovery(server, v1.RecoveryResultCompleted)

		By("publishing the new name")
		Eventually(func() error {
			return k8sClient.Get(ctx, renamedKey, &v1.DatabaseServerConfig{})
		}, timeout, interval).Should(Succeed())

		// The contract that asked is the only place the answer is published,
		// so the rename leaves it where it is. Whoever asked reads the result
		// there, however long they take to look.
		Consistently(func(g Gomega) {
			contract := publishedContract(server)
			g.Expect(contract.Spec.PITR.LastRecovery).NotTo(BeNil())
			g.Expect(contract.Spec.PITR.LastRecovery.Result).To(Equal(v1.RecoveryResultCompleted))
		}, "2s", interval).Should(Succeed())
	})

	It("leaves a cluster it does not own where it is when it cleans up", func() {
		server, from := archivingServer()

		// The label of this server, and no owner reference. The cleanup runs
		// over a label selector, and a label is a value anybody can write.
		stranger := &cnpgv1.Cluster{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "camunda-stranger",
				Namespace: server.Namespace,
				Labels:    map[string]string{labels.DatabaseServerKey: server.Name},
			},
			Spec: cnpgv1.ClusterSpec{
				Instances:            1,
				StorageConfiguration: cnpgv1.StorageConfiguration{Size: "1Gi"},
			},
		}
		Expect(k8sClient.Create(ctx, stranger)).To(Succeed())

		askForRecovery(server, from.Add(time.Hour))
		recoverySucceeds(server)
		expectLastRecovery(server, v1.RecoveryResultCompleted)

		expectGone(client.ObjectKey{Namespace: server.Namespace, Name: "camunda"}, &cnpgv1.Cluster{})
		Consistently(func() error {
			return k8sClient.Get(ctx, client.ObjectKeyFromObject(stranger), &cnpgv1.Cluster{})
		}, "2s", interval).Should(Succeed())
	})

	It("waits for the cluster of a refused recovery to go before it builds again", func() {
		server, from := archivingServer()
		first := askForRecovery(server, from.Add(time.Hour))

		// CloudNativePG holds its cluster while it takes the instances down.
		// The name of the next recovery is the name this one is still using.
		holdRecoveryCluster(server, "camunda-r1")
		setRecoveryClusterPhase(server, "camunda-r1", cnpgv1.PhaseUnrecoverable, "")
		Expect(expectLastRecovery(server, v1.RecoveryResultFailed).RequestID).To(Equal(first.RequestID))
		expectTerminating(server, "camunda-r1")

		By("asking again while the cluster of the refusal is still going")
		second := askForRecovery(server, from.Add(90*time.Minute))
		Eventually(func(g Gomega) {
			g.Expect(reconciledServer(server).Status.Recovery.RequestID).To(Equal(second.RequestID))
		}, timeout, interval).Should(Succeed())

		// Grading the cluster that is going answers the new request from the
		// state of the dead one, and it answers it at once.
		Consistently(func() string {
			return publishedContract(server).Spec.PITR.LastRecovery.RequestID
		}, "2s", interval).Should(Equal(first.RequestID))

		By("letting it go")
		releaseRecoveryCluster(server, "camunda-r1")

		Eventually(func(g Gomega) {
			var rebuilt cnpgv1.Cluster
			key := client.ObjectKey{Namespace: server.Namespace, Name: "camunda-r1"}
			g.Expect(k8sClient.Get(ctx, key, &rebuilt)).To(Succeed())
			g.Expect(rebuilt.DeletionTimestamp.IsZero()).To(BeTrue())
			g.Expect(rebuilt.Spec.Bootstrap.Recovery.RecoveryTarget.TargetTime).
				To(Equal(second.TargetTime))
		}, timeout, interval).Should(Succeed())
	})

	It("finishes a recovery that the spec suspends after the contract moved", func() {
		server, from := archivingServer()
		askForRecovery(server, from.Add(time.Hour))

		bringRecoveryClusterUp(server, "camunda-r1")
		Eventually(func() string {
			return reconciledServer(server).Status.Cluster
		}, timeout, interval).Should(Equal("camunda-r1"))

		// Past the cutover the contract already names the recovered server and
		// the old cluster is on its way out. Refusing here leaves the server
		// on a cluster nothing points at.
		suspend(server)

		probeContract(server)
		expectLastRecovery(server, v1.RecoveryResultCompleted)
		expectGone(client.ObjectKey{Namespace: server.Namespace, Name: "camunda"}, &cnpgv1.Cluster{})
	})

	It("gives the server back to the cluster it came from when the new one fails", func() {
		server, from := archivingServer()
		askForRecovery(server, from.Add(time.Hour))

		bringRecoveryClusterUp(server, "camunda-r1")
		Eventually(func() string {
			return reconciledServer(server).Status.Cluster
		}, timeout, interval).Should(Equal("camunda-r1"))

		// It broke after the contract moved to it. The cluster it replaced is
		// still there and still holds the data, so that is where the server
		// goes back to.
		setRecoveryClusterPhase(server, "camunda-r1", cnpgv1.PhaseUnrecoverable, "")

		outcome := expectLastRecovery(server, v1.RecoveryResultFailed)
		Expect(outcome.Message).To(ContainSubstring("runs from \"camunda\" again"))

		Eventually(func(g Gomega) {
			latest := reconciledServer(server)
			g.Expect(latest.Status.Cluster).To(Equal("camunda"))
			g.Expect(latest.Status.Recovery.Cluster).To(BeEmpty())
		}, timeout, interval).Should(Succeed())

		Eventually(func() string {
			return publishedContract(server).Spec.Host
		}, timeout, interval).Should(Equal("camunda-rw." + server.Namespace + ".svc"))

		// The archive of the cluster the server runs from stays open: the
		// contract never settled on the recovered one.
		history := archiveHistory(server)
		Expect(history).To(HaveLen(1))
		Expect(history[0].To).To(BeNil())
	})

	It("names no cluster in the record when it refuses one it does not own", func() {
		server, from := archivingServer()

		Expect(k8sClient.Create(ctx, &cnpgv1.Cluster{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "camunda-r1",
				Namespace: server.Namespace,
				Labels:    map[string]string{labels.DatabaseServerKey: server.Name},
			},
			Spec: cnpgv1.ClusterSpec{
				Instances:            1,
				StorageConfiguration: cnpgv1.StorageConfiguration{Size: "1Gi"},
			},
		})).To(Succeed())

		askForRecovery(server, from.Add(time.Hour))
		expectLastRecovery(server, v1.RecoveryResultFailed)

		// The cleanup reads that name. A refusal owns no cluster.
		Eventually(func(g Gomega) {
			g.Expect(reconciledServer(server).Status.Recovery.Cluster).To(BeEmpty())
		}, timeout, interval).Should(Succeed())
		Consistently(func() error {
			return k8sClient.Get(
				ctx, client.ObjectKey{Namespace: server.Namespace, Name: "camunda-r1"}, &cnpgv1.Cluster{},
			)
		}, "2s", interval).Should(Succeed())
	})

	It("keeps the archive of the cluster it replaces open until the contract moved", func() {
		server, from := archivingServer()
		askForRecovery(server, from.Add(time.Hour))

		bringRecoveryClusterUp(server, "camunda-r1")
		Eventually(func() string {
			return reconciledServer(server).Status.Cluster
		}, timeout, interval).Should(Equal("camunda-r1"))

		// The pointer moved and nothing has reached the new server yet. A
		// record closed here strands every point after it if the move never
		// becomes visible.
		Consistently(func(g Gomega) {
			history := archiveHistory(server)
			g.Expect(history).To(HaveLen(1))
			g.Expect(history[0].To).To(BeNil())
		}, "2s", interval).Should(Succeed())

		probeContract(server)
		expectLastRecovery(server, v1.RecoveryResultCompleted)

		Eventually(func(g Gomega) {
			history := archiveHistory(server)
			g.Expect(history).To(HaveLen(1))
			g.Expect(history[0].To).NotTo(BeNil())
		}, timeout, interval).Should(Succeed())
	})

	It("keeps the archive of the cluster it built open when the base backup lands first", func() {
		server, from := archivingServer()
		askForRecovery(server, from.Add(time.Hour))

		bringRecoveryClusterUp(server, "camunda-r1")
		Eventually(func() string {
			return reconciledServer(server).Status.Cluster
		}, timeout, interval).Should(Equal("camunda-r1"))

		// CloudNativePG takes the first base backup of the recovered cluster
		// before the contract has reached it, so the archive of that cluster
		// opens before the cutover finishes.
		//
		// The record of the cluster it replaces closes there, at the start of
		// the record that opens, and not at the cutover further down.
		backupAt := metav1.NewTime(time.Now().Truncate(time.Second))
		completeBaseBackup(reconciledServer(server), "base-2", backupAt)
		Eventually(func(g Gomega) {
			history := archiveHistory(server)
			g.Expect(history).To(HaveLen(2))
			g.Expect(history[0].ServerName).To(Equal("camunda"))
			g.Expect(history[0].To).To(Equal(&backupAt))
			g.Expect(history[1].ServerName).To(Equal("camunda-r1"))
			g.Expect(history[1].From).To(Equal(backupAt))
			g.Expect(history[1].To).To(BeNil())
		}, timeout, interval).Should(Succeed())

		probeContract(server)
		expectLastRecovery(server, v1.RecoveryResultCompleted)

		// The cutover closes the archive of the cluster it replaces and no
		// other. A closed archive of the cluster the server runs from now
		// leaves it with no point to roll back to, for good.
		Consistently(func(g Gomega) {
			history := archiveHistory(server)
			g.Expect(history).To(HaveLen(2))
			g.Expect(history[0].ServerName).To(Equal("camunda"))
			g.Expect(history[0].To).NotTo(BeNil())
			g.Expect(history[1].ServerName).To(Equal("camunda-r1"))
			g.Expect(history[1].To).To(BeNil())
		}, "2s", interval).Should(Succeed())

		expectCondition(server, v1.ConditionArchiveReady, metav1.ConditionTrue)
	})

	// The name of the bucket contract does not move here, so the hold that
	// pins the name never fires. The ObjectStore is one object, and rewriting
	// it points the cluster that is recovering at objects the archive it asked
	// for is not in.
	It("holds a rollback while the archive moves under its bucket contract", func() {
		server, from := archivingServer()
		askForRecovery(server, from.Add(time.Hour))
		expectRecoveryCluster(server)

		var bucket v1.ObjectStorageConfig
		Expect(k8sClient.Get(
			ctx, client.ObjectKey{Name: server.Spec.Archive.ObjectStorageRef}, &bucket,
		)).To(Succeed())

		recorded := reconciledServer(server).Status.Recovery.Archive
		Expect(recorded).NotTo(BeNil())
		Expect(recorded.Location).To(ContainSubstring("s3://" + bucket.Name + "/"))

		storeKey := client.ObjectKey{Namespace: server.Namespace, Name: "camunda"}
		var store barmanobjectstore.ObjectStore
		Expect(k8sClient.Get(ctx, storeKey, &store)).To(Succeed())
		reading := store.Spec.Configuration.DestinationPath

		By("moving the bucket under the name the rollback recorded")
		moveBucket(&bucket, bucket.Name+"-moved")

		ready := expectCondition(server, v1.ConditionReady, metav1.ConditionFalse)
		Expect(ready.Reason).To(Equal(v1.ReasonInvalidReference))
		Expect(ready.Message).To(ContainSubstring(recorded.Location))
		Expect(ready.Message).To(ContainSubstring("-moved"))

		// The archive the rollback reads is at the old location, so the
		// ObjectStore that describes it must not follow the contract. Nothing
		// applies the location the contract names now, so no record of the
		// server belongs to it either: the interval stays open and no move is
		// recorded.
		Consistently(func(g Gomega) {
			g.Expect(k8sClient.Get(ctx, storeKey, &store)).To(Succeed())
			g.Expect(store.Spec.Configuration.DestinationPath).To(Equal(reading))

			archive := reconciledServer(server).Status.Archive
			g.Expect(archive).NotTo(BeNil())
			g.Expect(archive.History).To(HaveLen(1))
			g.Expect(archive.History[0].To).To(BeNil())
			g.Expect(archive.History[0].Location).To(Equal(recorded.Location))
			g.Expect(archive.Boundary).To(BeNil())
		}, "2s", interval).Should(Succeed())

		By("finishing the rollback once the bucket is back")
		moveBucket(&bucket, bucket.Name)

		recoverySucceeds(server)
		expectLastRecovery(server, v1.RecoveryResultCompleted)

		// The hold left the history as it was. The rollback closes the record
		// of the cluster it replaces itself, at the moment the contract moves,
		// and it is still the record of the location the server wrote to.
		archive := reconciledServer(server).Status.Archive
		Expect(archive.History).To(HaveLen(1))
		Expect(archive.History[0].Location).To(Equal(recorded.Location))
		Expect(archive.Boundary).To(BeNil())
	})

	// The identity of a bucket with workload identity is on the pods, not in
	// the ObjectStore, so the hold on that object holds nothing of it. What
	// the rollback recorded is what keeps the pods on the bucket they read.
	It("keeps the identity a rollback started with while the archive is held", func() {
		const before = "arn:aws:iam::123456789012:role/before"
		const after = "arn:aws:iam::123456789012:role/after"

		server, from := archivingServer()

		var bucket v1.ObjectStorageConfig
		Expect(k8sClient.Get(
			ctx, client.ObjectKey{Name: server.Spec.Archive.ObjectStorageRef}, &bucket,
		)).To(Succeed())

		By("putting the bucket on workload identity before the rollback starts")
		setBucketRole(&bucket, before)
		Eventually(func(g Gomega) string {
			return clusterRole(g, server, server.Name)
		}, timeout, interval).Should(Equal(before))

		askForRecovery(server, from.Add(time.Hour))
		expectRecoveryCluster(server)

		recorded := reconciledServer(server).Status.Recovery.Archive
		Expect(recorded).NotTo(BeNil())
		Expect(recorded.Identity).NotTo(BeNil())
		Expect(recorded.Identity.Annotations).
			To(HaveKeyWithValue(v1.IRSARoleARNAnnotation, before))

		By("moving the bucket and its identity while the rollback is unanswered")
		moveBucket(&bucket, bucket.Name+"-moved")
		setBucketRole(&bucket, after)

		ready := expectCondition(server, v1.ConditionReady, metav1.ConditionFalse)
		Expect(ready.Reason).To(Equal(v1.ReasonInvalidReference))

		// The identity of the bucket the contract names now opens nothing in
		// the bucket the rollback reads. The cluster that runs writes its
		// archive there, and the cluster that recovers reads it there.
		Consistently(func(g Gomega) {
			g.Expect(clusterRole(g, server, server.Name)).To(Equal(before))
			g.Expect(clusterRole(g, server, recoveryCluster(server))).To(Equal(before))
		}, "2s", interval).Should(Succeed())

		By("letting the new identity through once the bucket is back")
		moveBucket(&bucket, bucket.Name)
		Eventually(func(g Gomega) string {
			return clusterRole(g, server, server.Name)
		}, timeout, interval).Should(Equal(after))

		recoverySucceeds(server)
		expectLastRecovery(server, v1.RecoveryResultCompleted)
	})

	// A credential is not part of where the archive is, so a rollback holds
	// nothing of it. A held credential leaves both clusters presenting a key
	// that the bucket no longer takes.
	It("takes a credential rotation while a rollback is unanswered", func() {
		server, from := archivingServer()
		askForRecovery(server, from.Add(time.Hour))
		expectRecoveryCluster(server)

		var bucket v1.ObjectStorageConfig
		Expect(k8sClient.Get(
			ctx, client.ObjectKey{Name: server.Spec.Archive.ObjectStorageRef}, &bucket,
		)).To(Succeed())

		By("pointing the contract at a rotated key pair")
		rotated := &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{Name: bucket.Name + "-rotated", Namespace: "default"},
			Data: map[string][]byte{
				"accessKeyId":     []byte("rotated-root"),
				"secretAccessKey": []byte("rotated-secret"),
			},
		}
		Expect(k8sClient.Create(ctx, rotated)).To(Succeed())
		DeferCleanup(func() { _ = k8sClient.Delete(ctx, rotated) })

		Eventually(func(g Gomega) {
			var latest v1.ObjectStorageConfig
			g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(&bucket), &latest)).To(Succeed())
			latest.Spec.S3.Auth.Credentials.SecretRef.Name = rotated.Name
			g.Expect(k8sClient.Update(ctx, &latest)).To(Succeed())
		}, timeout, interval).Should(Succeed())

		key := client.ObjectKey{
			Namespace: server.Namespace, Name: components.ArchiveSecretName(server),
		}
		Eventually(func(g Gomega) []byte {
			var secret corev1.Secret
			g.Expect(k8sClient.Get(ctx, key, &secret)).To(Succeed())

			return secret.Data["accessKeyId"]
		}, timeout, interval).Should(Equal([]byte("rotated-root")))

		recoverySucceeds(server)
		expectLastRecovery(server, v1.RecoveryResultCompleted)
	})

	It("keeps the bucket of a running recovery while the spec names another", func() {
		server, from := archivingServer()
		askForRecovery(server, from.Add(time.Hour))
		expectRecoveryCluster(server)

		recorded := reconciledServer(server).Status.Recovery.Archive
		Expect(recorded).NotTo(BeNil())
		Expect(recorded.ServerName).To(Equal("camunda"))
		Expect(recorded.RetentionPeriodDays).To(BeEquivalentTo(30))
		Expect(recorded.BaseBackupSchedule).To(Equal(components.DefaultBaseBackupSchedule))

		rendered := archiveOnCluster(Default, server)

		By("pointing the archive at another bucket, and shrinking it, while the recovery runs")
		other := archiveBucket()
		setArchive(server, &v1.DatabaseServerArchiveSpec{
			ObjectStorageRef:    other.Name,
			RetentionPeriodDays: 1,
			BaseBackupSchedule:  "0 0 5 * * *",
		})

		ready := expectCondition(server, v1.ConditionReady, metav1.ConditionFalse)
		Expect(ready.Message).To(ContainSubstring("Set spec.archive.objectStorageRef back"))

		// A shrunk retention would become the retention policy of the bucket
		// and prune the base backup the rollback starts from, so the hold
		// keeps every setting of the archive and not the bucket alone.
		var store barmanobjectstore.ObjectStore
		Consistently(func(g Gomega) {
			g.Expect(k8sClient.Get(
				ctx, client.ObjectKey{Namespace: server.Namespace, Name: "camunda"}, &store,
			)).To(Succeed())
			g.Expect(store.Spec.Configuration.DestinationPath).
				To(ContainSubstring(recorded.ObjectStorageRef))
			g.Expect(archiveOnCluster(g, server)).To(Equal(rendered))
		}, "2s", interval).Should(Succeed())

		recoverySucceeds(server)
		expectLastRecovery(server, v1.RecoveryResultCompleted)
	})

	It("keeps the archive of a running recovery that a preset shrinks", func() {
		server, preset, from := archivingServerOnPreset()
		askForRecovery(server, from.Add(time.Hour))
		expectRecoveryCluster(server)

		recorded := reconciledServer(server).Status.Recovery.Archive
		Expect(recorded).NotTo(BeNil())
		Expect(recorded.RetentionPeriodDays).To(BeEquivalentTo(30))
		Expect(recorded.BaseBackupSchedule).To(Equal(components.DefaultBaseBackupSchedule))

		rendered := archiveOnCluster(Default, server)
		Expect(rendered.retentionPolicy).To(Equal("30d"))

		// The preset merge runs before the hold, so a baseline that shrinks
		// the retention reaches the merged spec the same way an inline edit
		// does, and the hold has to catch it there.
		By("shrinking the retention of the baseline while the recovery runs")
		updatePreset(preset, func(p *v1.DatabaseServerPreset) {
			p.Spec.Server.Archive.RetentionPeriodDays = 1
			p.Spec.Server.Archive.BaseBackupSchedule = "0 0 5 * * *"
		})

		Consistently(func(g Gomega) {
			g.Expect(archiveOnCluster(g, server)).To(Equal(rendered))
		}, "2s", interval).Should(Succeed())

		By("letting the shrink through once the rollback is answered")
		recoverySucceeds(server)
		expectLastRecovery(server, v1.RecoveryResultCompleted)

		Eventually(func(g Gomega) {
			var store barmanobjectstore.ObjectStore
			g.Expect(k8sClient.Get(
				ctx, client.ObjectKey{Namespace: server.Namespace, Name: server.Name}, &store,
			)).To(Succeed())
			g.Expect(store.Spec.RetentionPolicy).To(Equal("1d"))
		}, timeout, interval).Should(Succeed())
	})

	It("keeps removing what an answered recovery replaced", func() {
		server, from := archivingServer()
		askForRecovery(server, from.Add(time.Hour))
		recoverySucceeds(server)
		expectLastRecovery(server, v1.RecoveryResultCompleted)
		expectGone(
			client.ObjectKey{Namespace: server.Namespace, Name: "camunda"}, &cnpgv1.ScheduledBackup{},
		)

		// The delete of a superseded object can fail, and the answer is
		// written before it runs. What is left of the cluster that went has to
		// go on a later look, not only on the look that answered.
		latest := reconciledServer(server)
		Expect(k8sClient.Create(ctx, &cnpgv1.ScheduledBackup{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "camunda",
				Namespace: server.Namespace,
				Labels:    map[string]string{labels.DatabaseServerKey: server.Name},
				OwnerReferences: []metav1.OwnerReference{
					*metav1.NewControllerRef(latest, v1.GroupVersion.WithKind("DatabaseServer")),
				},
			},
			Spec: cnpgv1.ScheduledBackupSpec{
				Schedule: components.DefaultBaseBackupSchedule,
				Cluster:  cnpgv1.LocalObjectReference{Name: "camunda"},
			},
		})).To(Succeed())

		expectGone(
			client.ObjectKey{Namespace: server.Namespace, Name: "camunda"}, &cnpgv1.ScheduledBackup{},
		)
	})

	It("reads the cluster it runs from back off the contract when its record is lost", func() {
		server, from := archivingServer()
		askForRecovery(server, from.Add(time.Hour))
		recoverySucceeds(server)
		expectLastRecovery(server, v1.RecoveryResultCompleted)

		By("losing the whole record of the recovery")
		Eventually(func(g Gomega) {
			var latest v1.DatabaseServer
			g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(server), &latest)).To(Succeed())
			latest.Status.Recovery = nil
			latest.Status.Cluster = "camunda"
			g.Expect(k8sClient.Status().Update(ctx, &latest)).To(Succeed())
		}, timeout, interval).Should(Succeed())

		// The contract names the recovered server. Reading status back from
		// the record alone calls the recovered cluster the one to remove.
		Eventually(func(g Gomega) {
			latest := reconciledServer(server)
			g.Expect(latest.Status.Cluster).To(Equal("camunda-r1"))
			g.Expect(latest.Status.Recovery).NotTo(BeNil())
		}, timeout, interval).Should(Succeed())

		Consistently(func() error {
			return k8sClient.Get(
				ctx, client.ObjectKey{Namespace: server.Namespace, Name: "camunda-r1"}, &cnpgv1.Cluster{},
			)
		}, "2s", interval).Should(Succeed())
	})

	It("gives the server back when the cluster it moved to is removed", func() {
		server, from := archivingServer()
		askForRecovery(server, from.Add(time.Hour))

		bringRecoveryClusterUp(server, "camunda-r1")
		Eventually(func() string {
			return reconciledServer(server).Status.Cluster
		}, timeout, interval).Should(Equal("camunda-r1"))

		// Somebody removed it after the contract moved to it. Reading a
		// missing cluster as an error holds the request for ever.
		var recovered cnpgv1.Cluster
		key := client.ObjectKey{Namespace: server.Namespace, Name: "camunda-r1"}
		Expect(k8sClient.Get(ctx, key, &recovered)).To(Succeed())
		Expect(k8sClient.Delete(ctx, &recovered)).To(Succeed())

		outcome := expectLastRecovery(server, v1.RecoveryResultFailed)
		Expect(outcome.Message).To(ContainSubstring("was removed"))
		Expect(outcome.Message).To(ContainSubstring("runs from \"camunda\" again"))

		Eventually(func() string {
			return reconciledServer(server).Status.Cluster
		}, timeout, interval).Should(Equal("camunda"))
	})

	It("keeps running from its own cluster when the contract names one it does not own", func() {
		server, from := archivingServer()
		askForRecovery(server, from.Add(time.Hour))
		recoverySucceeds(server)
		expectLastRecovery(server, v1.RecoveryResultCompleted)

		// Two servers can name one contract. The one that loses the race reads
		// the endpoint of the other, and adopting that name makes it delete
		// its own live cluster as the superseded one.
		stranger := &cnpgv1.Cluster{
			ObjectMeta: metav1.ObjectMeta{Name: "camunda-r9", Namespace: server.Namespace},
			Spec: cnpgv1.ClusterSpec{
				Instances:            1,
				StorageConfiguration: cnpgv1.StorageConfiguration{Size: "1Gi"},
			},
		}
		Expect(k8sClient.Create(ctx, stranger)).To(Succeed())

		// The contract stops being republished while the superuser Secret of
		// the cluster it names is missing, which is what lets the endpoint of
		// another server stand long enough to be read.
		var superuser corev1.Secret
		Expect(k8sClient.Get(
			ctx, client.ObjectKey{
				Namespace: server.Namespace, Name: "camunda-r1-superuser",
			}, &superuser,
		)).To(Succeed())
		Expect(k8sClient.Delete(ctx, &superuser)).To(Succeed())

		Eventually(func(g Gomega) {
			var contract v1.DatabaseServerConfig
			g.Expect(k8sClient.Get(ctx, contractKey(server), &contract)).To(Succeed())
			contract.Spec.Host = "camunda-r9-rw." + server.Namespace + ".svc"
			g.Expect(k8sClient.Update(ctx, &contract)).To(Succeed())
		}, timeout, interval).Should(Succeed())

		Consistently(func() string {
			return publishedContract(server).Spec.Host
		}, "1s", interval).Should(Equal("camunda-r9-rw." + server.Namespace + ".svc"))

		// The endpoint is read back only on a look that finds the record
		// missing, so the record goes last, once that endpoint is the one
		// every look reads.
		Eventually(func(g Gomega) {
			var latest v1.DatabaseServer
			g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(server), &latest)).To(Succeed())
			latest.Status.Recovery = nil
			g.Expect(k8sClient.Status().Update(ctx, &latest)).To(Succeed())
		}, timeout, interval).Should(Succeed())

		Consistently(func(g Gomega) {
			g.Expect(reconciledServer(server).Status.Cluster).To(Equal("camunda-r1"))
			g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(stranger), &cnpgv1.Cluster{})).
				To(Succeed())
			g.Expect(k8sClient.Get(
				ctx, client.ObjectKey{Namespace: server.Namespace, Name: "camunda-r1"}, &cnpgv1.Cluster{},
			)).To(Succeed())
		}, "3s", interval).Should(Succeed())

		// The refusal is its own reason. A reader of kubectl describe learns
		// that two servers are writing one contract, which is not what a
		// refused rollback means.
		Eventually(func() []string {
			return recoveryEventReasons(server)
		}, timeout, interval).Should(ContainElement("RecoveryClusterNotOwned"))
	})

	It("answers a request while the server is suspended and its bucket is gone", func() {
		bucket := archiveBucket()
		server := serverInNamespace(&v1.DatabaseServerArchiveSpec{
			ObjectStorageRef:    bucket.Name,
			RetentionPeriodDays: 30,
		})
		writeSuperuserSecret(server)
		makeClusterHealthy(server, "7000000000000000001")
		expectCondition(server, v1.ConditionContractReady, metav1.ConditionTrue)

		suspend(server)
		hibernate(server)
		expectCondition(server, v1.ConditionClusterReady, metav1.ConditionTrue)

		// A suspended server whose bucket stops resolving holds its whole
		// reconcile. The answer to a request has to come out from in front of
		// that hold, or whoever asked waits for ever.
		Expect(k8sClient.Delete(ctx, bucket)).To(Succeed())

		askForRecovery(server, time.Now())

		outcome := expectLastRecovery(server, v1.RecoveryResultFailed)
		Expect(outcome.Message).To(ContainSubstring("suspended"))
	})
})
