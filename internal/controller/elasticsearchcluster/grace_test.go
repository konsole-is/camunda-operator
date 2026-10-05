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

package elasticsearchcluster

import (
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/api/meta"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	v1 "github.com/konsole-is/camunda-operator/api/v1"
	components "github.com/konsole-is/camunda-operator/pkg/components/elasticsearchcluster"
)

var _ = Describe("ElasticsearchCluster grace periods", func() {
	It("comes back when the grace period of a component that is not ready runs out", func() {
		preset := createElasticsearchClusterPreset(smallClusterSpec())
		cluster := validElasticsearchCluster()
		cluster.Spec.PresetRef = preset.Name
		createElasticsearchCluster(cluster)
		expectConditionFalse(cluster, components.ConditionElasticsearch)

		By("waiting on the datastore period while only Elasticsearch is not ready")
		expectRequeueNear(cluster, suiteGracePeriods.Datastore)

		By("waiting on the workload period once the exporter is not ready either")
		Eventually(func(g Gomega) {
			var latest v1.ElasticsearchCluster
			g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(cluster), &latest)).To(Succeed())
			latest.Spec.Monitoring = &v1.MonitoringSpec{ServiceMonitor: &v1.ServiceMonitorSpec{Enabled: true}}
			g.Expect(k8sClient.Update(ctx, &latest)).To(Succeed())
		}, timeout, interval).Should(Succeed())
		expectConditionFalse(cluster, components.ConditionMetrics)
		expectRequeueNear(cluster, suiteGracePeriods.Workload)
	})
})

// expectConditionFalse polls until cluster reports conditionType as False.
func expectConditionFalse(cluster *v1.ElasticsearchCluster, conditionType string) {
	GinkgoHelper()
	Eventually(func(g Gomega) {
		var latest v1.ElasticsearchCluster
		g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(cluster), &latest)).To(Succeed())
		g.Expect(meta.IsStatusConditionFalse(latest.Status.Conditions, conditionType)).To(BeTrue())
	}, timeout, interval).Should(Succeed())
}

// expectRequeueNear reconciles cluster once and expects a requeue at most
// period away, and less than a minute short of it.
func expectRequeueNear(cluster *v1.ElasticsearchCluster, period time.Duration) {
	GinkgoHelper()
	Eventually(func(g Gomega) {
		result, err := reconciler.Reconcile(ctx, ctrl.Request{NamespacedName: client.ObjectKeyFromObject(cluster)})
		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(result.RequeueAfter).To(BeNumerically(">", period-time.Minute))
		g.Expect(result.RequeueAfter).To(BeNumerically("<=", period+time.Second))
	}, timeout, interval).Should(Succeed())
}
