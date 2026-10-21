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
	"sync/atomic"
	"time"

	cnpgv1 "github.com/cloudnative-pg/api/pkg/api/v1"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/util/retry"
	"sigs.k8s.io/controller-runtime/pkg/client"

	v1 "github.com/konsole-is/camunda-operator/api/v1"
	"github.com/konsole-is/camunda-operator/pkg/wrappers/barmanobjectstore"
)

var _ = Describe("DatabaseServer status write that loses a conflict", func() {
	for _, tc := range []struct {
		writer string
		touch  func(server *v1.DatabaseServer) error
	}{
		{"a metadata write", touchServer},
	} {
		// The boundary takes the time of the pass that applied the new
		// ObjectStore. A later time leaves out a base backup that began in
		// between.
		It("keeps the boundary that the pass of the move recorded after "+tc.writer, func() {
			first := archiveBucket()
			server := serverInBucketNamespace(first, &v1.DatabaseServerArchiveSpec{
				ObjectStorageRef:    first.Name,
				RetentionPeriodDays: 30,
			})
			writeSuperuserSecret(server)
			makeClusterHealthy(server, "7000000000000000040")
			completeBaseBackup(server, "first", metav1.NewTime(metav1.Now().Rfc3339Copy().Time))
			expectCondition(server, v1.ConditionArchiveReady, metav1.ConditionTrue)

			// The first apply of the new ObjectStore writes a label on the server,
			// so the status write of that pass conflicts. Each later apply waits,
			// so a later pass records a time well after the first one.
			second := bucketInNamespace(server.Namespace)
			var applies atomic.Int32
			var firstApplied atomic.Int64
			touched := make(chan error, 1)
			objectStoreApplies.onApply(func(store *barmanobjectstore.ObjectStore) {
				if store.GetNamespace() != server.Namespace ||
					!strings.Contains(store.Spec.Configuration.DestinationPath, second.Name) {
					return
				}
				switch applies.Add(1) {
				case 1:
					touched <- tc.touch(server)
					firstApplied.Store(time.Now().UnixNano())
				default:
					time.Sleep(3 * time.Second)
				}
			})
			DeferCleanup(func() { objectStoreApplies.onApply(nil) })

			setArchive(server, &v1.DatabaseServerArchiveSpec{
				ObjectStorageRef:    second.Name,
				RetentionPeriodDays: 30,
			})
			Eventually(touched, timeout, interval).Should(Receive(Succeed()))

			var boundary *v1.ArchiveBoundary
			Eventually(func(g Gomega) {
				boundary = reconciledServer(server).Status.Archive.Boundary
				g.Expect(boundary).NotTo(BeNil())
				g.Expect(boundary.ObjectStorageRef).To(Equal(second.Name))
			}, timeout, interval).Should(Succeed())
			Expect(boundary.At.Time).To(
				BeTemporally("<", time.Unix(0, firstApplied.Load()).Add(1500*time.Millisecond)),
				"a later pass recorded the boundary",
			)
		})
	}
})

var _ = Describe("DatabaseServer abandoned recovery whose status write loses a conflict", func() {
	// The hook writes the server before the Failed answer is published, so the
	// status write of the pass that abandons the rollback conflicts.
	It("keeps the cluster that holds the data", func() {
		server, from := archivingServer()
		askForRecovery(server, from.Add(time.Hour))
		bringRecoveryClusterUp(server, "camunda-r1")
		Eventually(func(g Gomega) {
			g.Expect(reconciledServer(server).Status.Cluster).To(Equal("camunda-r1"))
		}, timeout, interval).Should(Succeed())

		// The pass that sees the stranger own camunda-r1 abandons the
		// rollback, so the hook is in place before the stranger takes it.
		touched := make(chan error, 1)
		backupLists.onNextOutcome(func() { touched <- touchServer(server) })
		DeferCleanup(func() { backupLists.onNextOutcome(nil) })

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
		probeAnyEndpoint(server)

		expectLastRecovery(server, v1.RecoveryResultFailed)
		Eventually(touched, timeout, interval).Should(Receive(Succeed()))
		Eventually(func(g Gomega) {
			g.Expect(reconciledServer(server).Status.Cluster).To(Equal("camunda"))
		}, timeout, interval).Should(Succeed())
		Consistently(func(g Gomega) {
			g.Expect(k8sClient.Get(
				ctx, client.ObjectKey{Namespace: server.Namespace, Name: "camunda"}, &cnpgv1.Cluster{},
			)).To(Succeed(), "the cluster that holds the data was removed")
		}, "3s", interval).Should(Succeed())
	})
})

var _ = Describe("DatabaseServer abandoned recovery whose receipt is lost", func() {
	// The answer is published, the receipt of the move back fails, and the
	// status write at the end of the pass conflicts: nothing records the move.
	It("keeps the cluster that holds the data from the answer on the contract", func() {
		server, from := archivingServer()
		askForRecovery(server, from.Add(time.Hour))
		bringRecoveryClusterUp(server, "camunda-r1")
		Eventually(func(g Gomega) {
			g.Expect(reconciledServer(server).Status.Cluster).To(Equal("camunda-r1"))
		}, timeout, interval).Should(Succeed())

		// The pass that sees the stranger own camunda-r1 abandons the
		// rollback, so the hook is in place before the stranger takes it.
		touched := make(chan error, 1)
		backupLists.onNextOutcome(func() {
			backupLists.failNextStatusPatch()
			touched <- touchServer(server)
		})
		DeferCleanup(func() { backupLists.onNextOutcome(nil) })

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
		probeAnyEndpoint(server)

		expectLastRecovery(server, v1.RecoveryResultFailed)
		Eventually(touched, timeout, interval).Should(Receive(Succeed()))
		Eventually(func(g Gomega) {
			g.Expect(reconciledServer(server).Status.Cluster).To(Equal("camunda"))
		}, timeout, interval).Should(Succeed())
		Consistently(func(g Gomega) {
			g.Expect(k8sClient.Get(
				ctx, client.ObjectKey{Namespace: server.Namespace, Name: "camunda"}, &cnpgv1.Cluster{},
			)).To(Succeed(), "the cluster that holds the data was removed")
		}, "3s", interval).Should(Succeed())
	})
})

// touchServer writes a label on server, as a user or a GitOps tool can while
// the controller works on it.
func touchServer(server *v1.DatabaseServer) error {
	return retry.RetryOnConflict(retry.DefaultRetry, func() error {
		var latest v1.DatabaseServer
		if err := k8sClient.Get(ctx, client.ObjectKeyFromObject(server), &latest); err != nil {
			return err
		}
		if latest.Labels == nil {
			latest.Labels = map[string]string{}
		}
		latest.Labels["example.com/touched"] = "true"

		return k8sClient.Update(ctx, &latest)
	})
}
