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
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/client-go/util/retry"
	"sigs.k8s.io/controller-runtime/pkg/client"

	v1 "github.com/konsole-is/camunda-operator/api/v1"
	"github.com/konsole-is/camunda-operator/pkg/logicalbackup"
)

var _ = Describe("LogicalRestoreElasticsearch status write that loses a conflict", func() {
	// The fake answers the first restore only after the hook wrote a label on
	// the restore, so the status write at the end of the pass that asked for
	// the restores conflicts.
	It("keeps the restored snapshots and asks for each restore once", func() {
		w := newWorld()
		backup := createBackup(w)
		w.seedSnapshots(elasticsearchSnapshots...)
		w.search.SetIndices(targetIndices...)
		w.search.SetRecoveryActive(true)

		var restore *v1.LogicalRestoreElasticsearch
		touched := make(chan error, 1)
		w.search.BeforeNextRestoreAnswer(func(string) { touched <- touchRestore(restore) })
		restore = createRestore(w, backup.Name)

		wanted := append(
			append([]string{}, elasticsearchSnapshots...),
			logicalbackup.RecordsSnapshotName(backupID),
		)
		Eventually(touched, timeout, interval).Should(Receive(Succeed()))
		Eventually(func(g Gomega) {
			g.Expect(latest(g, restore).Status.RestoredSnapshots).To(Equal(wanted))
		}, timeout, interval).Should(Succeed())

		Consistently(func(g Gomega) {
			g.Expect(w.search.RestoreRequests()).To(HaveLen(len(wanted)), "a restore was asked for again")
			g.Expect(w.search.IndexDeleteCalls()).To(Equal(1), "the restored indices were deleted again")
		}, "2s", interval).Should(Succeed())
	})
})

// touchRestore writes a label on restore, as a user or a GitOps tool can while
// the controller works on it.
func touchRestore(restore *v1.LogicalRestoreElasticsearch) error {
	return retry.RetryOnConflict(retry.DefaultRetry, func() error {
		var current v1.LogicalRestoreElasticsearch
		if err := k8sClient.Get(ctx, client.ObjectKeyFromObject(restore), &current); err != nil {
			return err
		}
		if current.Labels == nil {
			current.Labels = map[string]string{}
		}
		current.Labels["example.com/touched"] = "true"

		return k8sClient.Update(ctx, &current)
	})
}
