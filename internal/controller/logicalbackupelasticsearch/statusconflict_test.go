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

package logicalbackupelasticsearch

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/client-go/util/retry"
	"sigs.k8s.io/controller-runtime/pkg/client"

	v1 "github.com/konsole-is/camunda-operator/api/v1"
)

// The fake answers a start only after the hook wrote a label on the backup.
// The status write of the reconcile that made the start therefore conflicts.
var _ = Describe("LogicalBackupElasticsearch status write that races a metadata write", func() {
	It("keeps the accepted history request", func() {
		r := newRig()
		touched := make(chan error, 1)
		r.management.BeforeNextHistoryAnswer(func(int64) { touched <- touchBackups(r.namespace) })
		backup := r.newBackup()

		id := completeBackup(r, backup)
		Eventually(touched, timeout, interval).Should(Receive(Succeed()))
		Expect(currentBackup(backup).Status.HistoryAcceptedTime).NotTo(BeNil())
		Expect(r.management.HistoryStarts(id)).To(Equal(1))
	})

	It("keeps the accepted runtime request", func() {
		r := newRig()
		touched := make(chan error, 1)
		r.management.BeforeNextRuntimeAnswer(func(int64) { touched <- touchBackups(r.namespace) })
		backup := r.newBackup()

		id := completeBackup(r, backup)
		Eventually(touched, timeout, interval).Should(Receive(Succeed()))
		Expect(currentBackup(backup).Status.RuntimeAcceptedTime).NotTo(BeNil())
		Expect(r.management.RuntimeStarts(id)).To(Equal(1))
	})
})

// touchBackups writes a label on every backup in namespace, as a user or a
// GitOps tool can while the controller works on it.
func touchBackups(namespace string) error {
	var list v1.LogicalBackupElasticsearchList
	if err := k8sClient.List(ctx, &list, client.InNamespace(namespace)); err != nil {
		return err
	}

	for _, item := range list.Items {
		err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
			var backup v1.LogicalBackupElasticsearch
			if err := k8sClient.Get(ctx, client.ObjectKeyFromObject(&item), &backup); err != nil {
				return err
			}
			if backup.Labels == nil {
				backup.Labels = map[string]string{}
			}
			backup.Labels["example.com/touched"] = "true"

			return k8sClient.Update(ctx, &backup)
		})
		if err != nil {
			return err
		}
	}

	return nil
}
