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
	"testing"
	"time"

	cnpgv1 "github.com/cloudnative-pg/api/pkg/api/v1"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/stretchr/testify/assert"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	v1 "github.com/konsole-is/camunda-operator/api/v1"
	components "github.com/konsole-is/camunda-operator/pkg/components/databaseserver"
)

var _ = Describe("DatabaseServer recovery from the archive it writes now", func() {
	It("builds the recovered cluster only once the archive holds the point", func() {
		server, from := archivingServer()
		primaries.holdArchive(server.Namespace)
		askForRecovery(server, from.Add(time.Hour))

		By("closing the segment of the point")
		var segment string
		Eventually(func(g Gomega) {
			recovery := reconciledServer(server).Status.Recovery
			g.Expect(recovery).NotTo(BeNil())
			g.Expect(recovery.StartedAt).NotTo(BeNil())
			g.Expect(recovery.ClosedSegment).NotTo(BeEmpty())
			segment = recovery.ClosedSegment
		}, timeout, interval).Should(Succeed())

		By("building nothing and answering nothing while the archive lacks it")
		key := client.ObjectKey{Namespace: server.Namespace, Name: recoveryCluster(server)}
		Consistently(func(g Gomega) {
			g.Expect(k8sClient.Get(ctx, key, &cnpgv1.Cluster{})).
				To(MatchError(apierrors.IsNotFound, "not found"))
			g.Expect(publishedContract(g, server).Spec.PITR.LastRecovery).To(BeNil())
		}, "1s", interval).Should(Succeed())

		By("building the cluster once the archive holds it")
		primaries.releaseArchive(server.Namespace)
		expectRecoveryCluster(server)
		Expect(reconciledServer(server).Status.Recovery.ClosedSegment).To(Equal(segment))
	})

	It("builds the recovered cluster when the archive does not get the segment in time", func() {
		server, from := archivingServer()
		primaries.holdArchive(server.Namespace)
		askForRecovery(server, from.Add(time.Hour))

		key := client.ObjectKey{Namespace: server.Namespace, Name: recoveryCluster(server)}
		var recovered cnpgv1.Cluster
		Eventually(func() error {
			return k8sClient.Get(ctx, key, &recovered)
		}, timeout+suiteArchiveWait, interval).Should(Succeed())

		// The archive can still hold an older point, so the recovery decides,
		// and the request stays open for it.
		recovery := reconciledServer(server).Status.Recovery
		Expect(recovery.ClosedSegment).NotTo(BeEmpty())
		Expect(recovery.CompletedAt).To(BeNil())
		Expect(publishedContract(Default, server).Spec.PITR.LastRecovery).To(BeNil())
		// Both times are whole seconds, so one second of the wait can hide in
		// the rounding.
		Expect(recovered.CreationTimestamp.Sub(recovery.StartedAt.Time)).
			To(BeNumerically(">=", suiteArchiveWait-time.Second))
	})

	It("builds the recovered cluster at once while the uploads of the archive fail", func() {
		server, from := archivingServer()
		reportArchiving(
			server, metav1.ConditionFalse,
			metav1.NewTime(metav1.Now().Rfc3339Copy().Add(-2*components.ArchiveOutageGracePeriod)),
		)
		expectConditionReason(server, v1.ConditionArchiveReady, metav1.ConditionFalse, v1.ReasonArchiveFailing)

		askForRecovery(server, from.Add(time.Hour))

		key := client.ObjectKey{Namespace: server.Namespace, Name: recoveryCluster(server)}
		var recovered cnpgv1.Cluster
		Eventually(func() error {
			return k8sClient.Get(ctx, key, &recovered)
		}, timeout, interval).Should(Succeed())

		recovery := reconciledServer(server).Status.Recovery
		Expect(recovery.ClosedSegment).To(BeEmpty())
		Expect(primaries.closedIn(server.Namespace)).To(BeZero())
	})

	It("builds the recovered cluster without the check when the primary does not answer", func() {
		server, from := archivingServer()
		primaries.silence(server.Namespace)
		askForRecovery(server, from.Add(time.Hour))

		key := client.ObjectKey{Namespace: server.Namespace, Name: recoveryCluster(server)}
		var recovered cnpgv1.Cluster
		Eventually(func() error {
			return k8sClient.Get(ctx, key, &recovered)
		}, timeout+suiteArchiveWait, interval).Should(Succeed())

		recovery := reconciledServer(server).Status.Recovery
		Expect(recovery.ClosedSegment).To(BeEmpty())
		Expect(recovery.StartedAt).NotTo(BeNil())
		// Both times are whole seconds, so one second of the wait can hide in
		// the rounding.
		Expect(recovered.CreationTimestamp.Sub(recovery.StartedAt.Time)).
			To(BeNumerically(">=", suiteArchiveWait-time.Second))
	})

	It("builds the recovered cluster at once from an archive that the server no longer writes", func() {
		server, from := archivingServer()
		askForRecovery(server, from.Add(time.Hour))
		recoverySucceeds(server)
		expectLastRecovery(server, v1.RecoveryResultCompleted)

		// The cluster the server runs from now opens an archive of its own,
		// and the archive of the cluster it replaced is closed.
		var closedAt metav1.Time
		Eventually(func(g Gomega) {
			history := archiveHistory(server)
			g.Expect(history).To(HaveLen(1))
			g.Expect(history[0].To).NotTo(BeNil())
			closedAt = *history[0].To
		}, timeout, interval).Should(Succeed())
		completeBaseBackup(
			reconciledServer(server), "base-2", metav1.NewTime(closedAt.Add(time.Minute).Truncate(time.Second)),
		)
		Eventually(func() []v1.ArchiveRecord {
			return archiveHistory(server)
		}, timeout, interval).Should(HaveLen(2))

		closed := primaries.closedIn(server.Namespace)
		request := askForRecovery(server, from.Add(30*time.Minute))

		key := client.ObjectKey{Namespace: server.Namespace, Name: server.Name + "-r2"}
		Eventually(func() error {
			return k8sClient.Get(ctx, key, &cnpgv1.Cluster{})
		}, timeout, interval).Should(Succeed())

		recovery := reconciledServer(server).Status.Recovery
		Expect(recovery.RequestID).To(Equal(request.RequestID))
		Expect(recovery.ClosedSegment).To(BeEmpty())
		Expect(primaries.closedIn(server.Namespace)).To(Equal(closed))
	})
})

// The recovery starts only once the archive holds the segment it closed. A
// name that counts as later than it starts a recovery that PostgreSQL fails,
// and one that never counts holds the rollback for the whole wait.
func TestArchivedThrough(t *testing.T) {
	t.Parallel()

	const closed = "000000010000000A00000042"

	cases := []struct {
		name string
		last string
		want bool
	}{
		{name: "the closed segment itself", last: closed, want: true},
		{name: "a later segment of the same log", last: "000000010000000A00000043", want: true},
		{name: "a segment of a later log", last: "000000010000000B00000001", want: true},
		{name: "an earlier segment", last: "000000010000000A00000041", want: false},
		{name: "an earlier log", last: "000000010000000900000099", want: false},
		{name: "a later timeline at a later position", last: "000000020000000A00000043", want: false},
		{name: "a later timeline at an earlier position", last: "000000020000000A00000001", want: false},
		{name: "a lower case name of the closed segment", last: "000000010000000a00000042", want: true},
		{name: "the partial file of the closed segment", last: closed + ".partial", want: false},
		{name: "the partial file of a later segment", last: "000000010000000A00000043.partial", want: false},
		{name: "a backup label in the closed segment", last: closed + ".00000028.backup", want: true},
		{name: "a backup label in a later segment", last: "000000010000000A00000043.00000028.backup", want: true},
		{name: "a backup label in an earlier segment", last: "000000010000000A00000040.00000028.backup", want: false},
		{name: "a timeline history file", last: "00000002.history", want: false},
		{name: "nothing archived yet", last: "", want: false},
		{name: "a name that is not hexadecimal", last: "00000001000000XA00000042", want: false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, tc.want, archivedThrough(tc.last, closed))
		})
	}
}
