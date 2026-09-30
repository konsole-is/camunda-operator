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

	v1 "github.com/konsole-is/camunda-operator/api/v1"
	"github.com/konsole-is/camunda-operator/pkg/esadmin/esadmintest"
)

// strandedPrimary is a restored primary that no node takes. Elasticsearch
// does not try it again without a change to the cluster.
var strandedPrimary = esadmintest.Shard{
	Primary:          true,
	State:            "UNASSIGNED",
	RecoverySource:   "SNAPSHOT",
	UnassignedReason: "NEW_INDEX_RESTORED",
	AllocationStatus: "deciders_no",
}

var _ = Describe("LogicalRestoreElasticsearch with a restored primary that no node takes", func() {
	It("fails and names the index instead of moving on to the broker volumes", func() {
		w := newWorld()
		backup := createBackup(w)
		w.seedSnapshots(elasticsearchSnapshots...)
		w.search.SetRecoveryActive(false)
		w.search.SetShards(targetIndices[0], strandedPrimary)

		restore := createRestore(w, backup.Name)
		serveRestoredIndices(w, restore)

		failed := expectReason(restore, v1.LogicalRestoreFailed, v1.ReasonFailed)
		Expect(failed.Status.FailureMessage).To(ContainSubstring(targetIndices[0]))
		Expect(failed.Status.FailureMessage).To(ContainSubstring("deciders_no"))
		Expect(failed.Status.FailureMessage).To(ContainSubstring(
			`{"index":"`+targetIndices[0]+`","shard":0,"primary":true}`,
		), "the allocation explain request names this primary, not the first unassigned shard")
		Expect(failed.Status.PrimaryJobNames).To(BeEmpty(), "the broker volumes were never restored")
	})

	It("gives its backend back after it fails, because no recovery comes for that primary", func() {
		w := newWorld()
		backup := createBackup(w)
		restore := startedRestore(w, backup)
		w.search.SetShards(targetIndices[0], strandedPrimary)

		failRestore(restore)

		Eventually(func(g Gomega) {
			g.Expect(writersNaming(restore)).To(BeEmpty())
			g.Expect(latest(g, restore).Status.RecoveryHeld).To(HaveValue(BeFalse()))
		}, timeout, interval).Should(Succeed())
	})
})
