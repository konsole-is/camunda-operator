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

package logicalbackuprdbms

import (
	"sync/atomic"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	batchv1 "k8s.io/api/batch/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/util/retry"
	"sigs.k8s.io/controller-runtime/pkg/client"

	v1 "github.com/konsole-is/camunda-operator/api/v1"
)

var _ = Describe("LogicalBackupRDBMS status write that loses a conflict", func() {
	// The fake answers the start only after the hook wrote the backup, so the
	// status write at the end of the reconcile that made the start conflicts.
	for _, tc := range []struct {
		writer string
		touch  func(backup *v1.LogicalBackupRDBMS) error
	}{
		{"a metadata write", touchBackup},
		{"a status write of a peer", touchBackupStatus},
	} {
		It("keeps the Zeebe backup id that the cluster generated after "+tc.writer, func() {
			w := createWorld()
			backup := createBackup(w)
			var generated atomic.Int64
			touched := make(chan error, 1)
			managementAPI.BeforeNextRuntimeAnswer(func(id int64) {
				generated.Store(id)
				touched <- tc.touch(backup)
			})
			DeferCleanup(func() { managementAPI.BeforeNextRuntimeAnswer(nil) })

			markJob(backup, w, batchv1.JobComplete)
			Eventually(touched, timeout, interval).Should(Receive(Succeed()))

			Eventually(func(g Gomega) {
				g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(backup), backup)).To(Succeed())
				g.Expect(backup.Status.ZeebeBackupID).NotTo(BeNil())
			}, timeout, interval).Should(Succeed())
			Consistently(func(g Gomega) {
				g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(backup), backup)).To(Succeed())
				g.Expect(*backup.Status.ZeebeBackupID).To(Equal(generated.Load()), "a second backup was requested")
			}, "2s", interval).Should(Succeed())
		})
	}
})

// touchBackup writes a label on backup, as a user or a GitOps tool can while
// the controller works on it.
func touchBackup(backup *v1.LogicalBackupRDBMS) error {
	return retry.RetryOnConflict(retry.DefaultRetry, func() error {
		var latest v1.LogicalBackupRDBMS
		if err := k8sClient.Get(ctx, client.ObjectKeyFromObject(backup), &latest); err != nil {
			return err
		}
		if latest.Labels == nil {
			latest.Labels = map[string]string{}
		}
		latest.Labels["example.com/touched"] = "true"

		return k8sClient.Update(ctx, &latest)
	})
}

// touchBackupStatus writes a condition of its own into the status of backup,
// as a second controller that shares the status can.
func touchBackupStatus(backup *v1.LogicalBackupRDBMS) error {
	return retry.RetryOnConflict(retry.DefaultRetry, func() error {
		var latest v1.LogicalBackupRDBMS
		if err := k8sClient.Get(ctx, client.ObjectKeyFromObject(backup), &latest); err != nil {
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
