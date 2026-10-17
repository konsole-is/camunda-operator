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

package camundamanagementcluster

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	utilrand "k8s.io/apimachinery/pkg/util/rand"
	"k8s.io/client-go/util/retry"
	"sigs.k8s.io/controller-runtime/pkg/client"

	v1 "github.com/konsole-is/camunda-operator/api/v1"
)

var _ = Describe("CamundaManagementCluster status write that loses a conflict", func() {
	// The spec is renamed while the first contract is created, so the status
	// write of the pass that created it conflicts.
	It("withdraws the first contract after a rename that raced its status write", func() {
		first := "first-" + utilrand.String(6)
		renamed := first + "-renamed"
		var key client.ObjectKey
		renames := make(chan error, 1)
		contractCreates.beforeCreate(first, func() {
			renames <- retry.RetryOnConflict(retry.DefaultRetry, func() error {
				var latest v1.CamundaManagementCluster
				if err := k8sClient.Get(ctx, key, &latest); err != nil {
					return err
				}
				latest.Spec.ManagementAuthConfigName = renamed

				return k8sClient.Update(ctx, &latest)
			})
		})
		DeferCleanup(func() { contractCreates.beforeCreate(first, nil) })

		s := newScenario(func(f *fixture) {
			f.mc.Spec.ManagementAuthConfigName = first
			key = client.ObjectKeyFromObject(f.mc)
		})
		Eventually(renames, timeout, interval).Should(Receive(Succeed()))

		Eventually(func(g Gomega) {
			var written v1.ManagementAuthConfig
			g.Expect(k8sClient.Get(ctx, client.ObjectKey{Name: renamed}, &written)).To(Succeed())
			g.Expect(apierrors.IsNotFound(k8sClient.Get(ctx, client.ObjectKey{Name: first}, &written))).
				To(BeTrue(), "the first contract stayed behind")
			g.Expect(readManagementCluster(g, s.mc).Status.ManagementAuthConfig).To(Equal(renamed))
		}, timeout, interval).Should(Succeed())
	})
})
