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

package camundaoptimize

import (
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/sourcehawk/operator-component-framework/pkg/component"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	v1 "github.com/konsole-is/camunda-operator/api/v1"
)

var _ = Describe("CamundaOptimize grace period", func() {
	It("comes back when the grace period of a workload that is not ready runs out", func() {
		s := newScenario("8.9.4")
		// The workloads stay at zero until the cluster holds its backend, so
		// the webapp can report Updating rather than Creating.
		expectCondition(s.optimize, v1.ConditionWebappReady, SatisfyAny(
			Equal(string(component.AliveCreating)), Equal(string(component.AliveUpdating)),
		))

		req := ctrl.Request{NamespacedName: client.ObjectKeyFromObject(s.optimize)}
		Eventually(func(g Gomega) {
			result, err := reconciler.Reconcile(ctx, req)
			g.Expect(err).NotTo(HaveOccurred())
			g.Expect(result.RequeueAfter).To(BeNumerically(">", suiteGracePeriods.Workload-time.Minute))
			g.Expect(result.RequeueAfter).To(BeNumerically("<=", suiteGracePeriods.Workload+time.Second))
		}, timeout, interval).Should(Succeed())
	})
})
