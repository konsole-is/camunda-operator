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
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
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
		{"a status write of a peer", touchServerStatus},
	} {
		// The boundary takes the time of the apply of the new ObjectStore. A
		// later time leaves out a base backup that began in between.
		It("keeps the boundary at the time of the first apply after "+tc.writer, func() {
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
			// so a pass that took the time from its own clock would record a
			// time well after the first apply.
			second := bucketInNamespace(server.Namespace)
			var applies atomic.Int32
			var firstApplied atomic.Int64
			touched := make(chan error, 1)
			labelled := make(chan error, 1)
			objectStoreApplies.onApply(func(store *barmanobjectstore.ObjectStore) {
				if store.GetNamespace() != server.Namespace ||
					!strings.Contains(store.Spec.Configuration.DestinationPath, second.Name) {
					return
				}
				switch applies.Add(1) {
				case 1:
					touched <- tc.touch(server)
					firstApplied.Store(time.Now().UnixNano())
				case 2:
					// Another manager labels the ObjectStore well after the
					// move. Its apply is not the time of the move.
					time.Sleep(2500 * time.Millisecond)
					labelled <- labelObjectStore(store)
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
			Eventually(labelled, timeout, interval).Should(Receive(Succeed()))

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

		touched := make(chan error, 1)
		backupLists.onNextOutcome(func() { touched <- touchServer(server) })
		DeferCleanup(func() { backupLists.onNextOutcome(nil) })
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

// labelObjectStore applies a label on store with a field manager of its own,
// as another tool can.
func labelObjectStore(store *barmanobjectstore.ObjectStore) error {
	label := &unstructured.Unstructured{}
	label.SetAPIVersion("barmancloud.cnpg.io/v1")
	label.SetKind("ObjectStore")
	label.SetNamespace(store.GetNamespace())
	label.SetName(store.GetName())
	label.SetLabels(map[string]string{"example.com/labelled": "true"})

	//nolint:staticcheck // the suite applies through the deprecated client.Apply patch, as the operator does
	return k8sClient.Patch(ctx, label, client.Apply, client.FieldOwner("example.com/labeller"))
}

// touchServerStatus writes a condition of its own into the status of server,
// as a second controller that shares the status can.
func touchServerStatus(server *v1.DatabaseServer) error {
	return retry.RetryOnConflict(retry.DefaultRetry, func() error {
		var latest v1.DatabaseServer
		if err := k8sClient.Get(ctx, client.ObjectKeyFromObject(server), &latest); err != nil {
			return err
		}
		meta.SetStatusCondition(&latest.Status.Conditions, metav1.Condition{
			Type:   "example.com/Touched",
			Status: metav1.ConditionTrue,
			Reason: "Touched",
		})

		return k8sClient.Status().Update(ctx, &latest)
	})
}

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
