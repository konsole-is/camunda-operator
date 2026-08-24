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
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	v1 "github.com/konsole-is/camunda-operator/api/v1"
	components "github.com/konsole-is/camunda-operator/pkg/components/databaseserver"
)

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

// askForRecovery writes a recovery request on the contract of the server, the
// way a PointInTimeRestore does. It returns the requestedBy it wrote.
func askForRecovery(server *v1.DatabaseServer, target time.Time) string {
	GinkgoHelper()

	requestedBy := server.Namespace + "/pitr-1"
	Eventually(func(g Gomega) {
		var contract v1.DatabaseServerConfig
		g.Expect(k8sClient.Get(ctx, contractKey(server), &contract)).To(Succeed())
		contract.Spec.Recovery = &v1.RecoveryRequest{
			RequestedBy: requestedBy,
			TargetTime:  target.UTC().Format(time.RFC3339),
		}
		g.Expect(k8sClient.Update(ctx, &contract)).To(Succeed())
	}, timeout, interval).Should(Succeed())

	return requestedBy
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

// bringRecoveryClusterUp writes the Secret and the status that CloudNativePG
// writes for a recovery cluster that finished its bootstrap.
func bringRecoveryClusterUp(server *v1.DatabaseServer, name, systemID string) {
	GinkgoHelper()

	Expect(k8sClient.Create(ctx, &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: name + "-superuser", Namespace: server.Namespace},
		Data:       map[string][]byte{"username": []byte("postgres"), "password": []byte("s3cret")},
	})).To(Succeed())

	setRecoveryClusterPhase(server, name, cnpgv1.PhaseHealthy, systemID)
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
		g.Expect(k8sClient.Status().Update(ctx, &cluster)).To(Succeed())
	}, timeout, interval).Should(Succeed())
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
		requestedBy := askForRecovery(server, target)

		By("recording the request before it builds anything")
		Eventually(func(g Gomega) {
			recovery := reconciledServer(server).Status.Recovery
			g.Expect(recovery).NotTo(BeNil())
			g.Expect(recovery.RequestedBy).To(Equal(requestedBy))
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

		bringRecoveryClusterUp(server, "camunda-r1", "7000000000000000002")

		By("pointing the contract at the recovered server")
		Eventually(func(g Gomega) {
			contract := publishedContract(server)
			g.Expect(contract.Spec.Host).To(Equal("camunda-r1-rw." + server.Namespace + ".svc"))
			g.Expect(contract.Spec.AdminCredentialsSecretRef.Name).To(Equal("camunda-r1-superuser"))
		}, timeout, interval).Should(Succeed())

		By("publishing the outcome")
		outcome := expectLastRecovery(server, v1.RecoveryResultCompleted)
		Expect(outcome.RequestedBy).To(Equal(requestedBy))
		Expect(outcome.TargetTime).To(Equal(target.UTC().Format(time.RFC3339)))

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
			g.Expect(latest.Status.SystemIdentifier).To(Equal("7000000000000000002"))
			g.Expect(latest.Status.Recovery.CompletedAt).NotTo(BeNil())
		}, timeout, interval).Should(Succeed())

		By("closing the archive it recovered from and opening the one it writes now")
		completeBaseBackup(reconciledServer(server), "base-2", metav1.Now())
		Eventually(func(g Gomega) {
			history := archiveHistory(server)
			g.Expect(history).To(HaveLen(2))
			g.Expect(history[0].ServerName).To(Equal("camunda"))
			g.Expect(history[0].To).NotTo(BeNil())
			g.Expect(history[1].ServerName).To(Equal("camunda-r1"))
		}, timeout, interval).Should(Succeed())

		By("keeping the outcome and the request on the contract it republishes")
		Consistently(func(g Gomega) {
			contract := publishedContract(server)
			g.Expect(contract.Spec.PITR.LastRecovery).NotTo(BeNil())
			g.Expect(contract.Spec.PITR.LastRecovery.Result).To(Equal(v1.RecoveryResultCompleted))
			g.Expect(contract.Spec.Recovery).NotTo(BeNil())
		}, "2s", interval).Should(Succeed())
	})

	It("refuses a point that no archive of the server holds", func() {
		server, from := archivingServer()

		requestedBy := askForRecovery(server, from.Add(-time.Hour))

		outcome := expectLastRecovery(server, v1.RecoveryResultUnavailable)
		Expect(outcome.RequestedBy).To(Equal(requestedBy))
		Expect(outcome.Message).To(ContainSubstring("lies in none of those windows"))

		// Nothing was built, so nothing has to be cleaned up.
		Expect(reconciledServer(server).Status.Recovery.Cluster).To(BeEmpty())
		Expect(k8sClient.Get(
			ctx, client.ObjectKey{Namespace: server.Namespace, Name: "camunda-r1"}, &cnpgv1.Cluster{},
		)).To(MatchError(apierrors.IsNotFound, "not found"))
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

	It("answers a request while the server is suspended and its bucket is gone", func() {
		bucket := archiveBucket()
		server := serverInNamespace(&v1.DatabaseServerArchiveSpec{
			ObjectStorageRef:    bucket.Name,
			RetentionPeriodDays: 30,
		})
		writeSuperuserSecret(server)
		makeClusterHealthy(server, "7000000000000000001")
		expectCondition(server, components.ConditionContract, metav1.ConditionTrue)

		suspend(server)
		hibernate(server)
		expectCondition(server, components.ConditionCluster, metav1.ConditionTrue)

		// A suspended server whose bucket stops resolving holds its whole
		// reconcile. The answer to a request has to come out from in front of
		// that hold, or whoever asked waits for ever.
		Expect(k8sClient.Delete(ctx, bucket)).To(Succeed())

		askForRecovery(server, time.Now())

		outcome := expectLastRecovery(server, v1.RecoveryResultFailed)
		Expect(outcome.Message).To(ContainSubstring("suspended"))
	})
})
