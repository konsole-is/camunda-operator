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

package camundacluster

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"sigs.k8s.io/controller-runtime/pkg/client"

	v1 "github.com/konsole-is/camunda-operator/api/v1"
)

// expectGateway polls until the published gateway binding satisfies match.
func expectGateway(cluster *v1.CamundaCluster, match func(Gomega, *v1.GatewayBinding)) {
	GinkgoHelper()
	Eventually(func(g Gomega) {
		var latest v1.CamundaCluster
		g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(cluster), &latest)).To(Succeed())
		match(g, latest.Status.Gateway)
	}, timeout, interval).Should(Succeed())
}

var _ = Describe("CamundaCluster gateway binding", func() {
	// The default topology runs a standalone gateway, which serves both APIs.
	It("names the gRPC and the REST endpoint of the gateway Service", func() {
		cluster := createDefaultCluster()

		expectGateway(cluster, func(g Gomega, gateway *v1.GatewayBinding) {
			g.Expect(gateway).NotTo(BeNil())
			g.Expect(gateway.GRPCEndpoint).To(Equal(
				cluster.Name + "-gateway." + cluster.Namespace + ".svc:26500",
			))
			g.Expect(gateway.RESTEndpoint).To(Equal(
				"http://" + cluster.Name + "-gateway." + cluster.Namespace + ".svc:8080",
			))
		})
	})

	// With an embedded gateway the brokers serve the same APIs, so the
	// binding follows the topology instead of naming a Service that the
	// cluster does not render.
	It("names the broker Service when the gateway is embedded", func() {
		ns := newNamespace()
		cluster := newCluster(ns, createPlatformConfig(), createBinding(ns, true))
		cluster.Spec.Gateway = &v1.GatewaySpec{Mode: v1.ComponentModeEmbedded}
		createCluster(cluster)

		expectGateway(cluster, func(g Gomega, gateway *v1.GatewayBinding) {
			g.Expect(gateway).NotTo(BeNil())
			g.Expect(gateway.GRPCEndpoint).To(Equal(
				cluster.Name + "-zeebe." + cluster.Namespace + ".svc:26500",
			))
			g.Expect(gateway.RESTEndpoint).To(Equal(
				"http://" + cluster.Name + "-zeebe." + cluster.Namespace + ".svc:8080",
			))
		})
	})

	// A suspended cluster has every workload scaled to zero. A consumer that
	// kept the endpoints would deploy against a cluster that answers nothing.
	It("is cleared while the cluster is suspended, and returns after", func() {
		cluster := createDefaultCluster()
		expectGateway(cluster, func(g Gomega, gateway *v1.GatewayBinding) {
			g.Expect(gateway).NotTo(BeNil())
		})

		updateCluster(cluster, func(c *v1.CamundaCluster) { c.Spec.Suspend = true })
		expectGateway(cluster, func(g Gomega, gateway *v1.GatewayBinding) {
			g.Expect(gateway).To(BeNil())
		})

		updateCluster(cluster, func(c *v1.CamundaCluster) { c.Spec.Suspend = false })
		expectGateway(cluster, func(g Gomega, gateway *v1.GatewayBinding) {
			g.Expect(gateway).NotTo(BeNil())
		})
	})
})
