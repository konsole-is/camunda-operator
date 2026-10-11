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
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"

	v1 "github.com/konsole-is/camunda-operator/api/v1"
)

// expectEndpoints polls until the published cluster satisfies match.
func expectEndpoints(cluster *v1.CamundaCluster, match func(Gomega, *v1.CamundaCluster)) {
	GinkgoHelper()
	Eventually(func(g Gomega) {
		var latest v1.CamundaCluster
		g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(cluster), &latest)).To(Succeed())
		match(g, &latest)
	}, timeout, interval).Should(Succeed())
}

// endpointsByName indexes a route table by name.
func endpointsByName(endpoints []v1.Endpoint) map[string]v1.Endpoint {
	byName := make(map[string]v1.Endpoint, len(endpoints))
	for _, endpoint := range endpoints {
		byName[endpoint.Name] = endpoint
	}
	return byName
}

// createEmbeddedGatewayCluster creates a cluster whose brokers run the
// gateway.
func createEmbeddedGatewayCluster() *v1.CamundaCluster {
	GinkgoHelper()
	ns := newNamespace()
	cluster := newCluster(ns, createPlatformConfig(), createBinding(ns, true))
	cluster.Spec.Gateway = &v1.GatewaySpec{Mode: v1.ComponentModeEmbedded}
	createCluster(cluster)
	return cluster
}

var _ = Describe("CamundaCluster endpoints", func() {
	It("publishes a route for every path of a new cluster", func() {
		cluster := createDefaultCluster()
		gateway, http := cluster.Name+"-gateway", v1.EndpointProtocolHTTP

		expectEndpoints(cluster, func(g Gomega, latest *v1.CamundaCluster) {
			g.Expect(latest.Status.Endpoints).To(Equal([]v1.Endpoint{
				{Name: "admin", Service: gateway, Port: 8080, PathPrefix: "/admin", Protocol: http},
				{Name: "default", Service: gateway, Port: 8080, PathPrefix: "/", Protocol: http},
				{Name: "grpc", Service: gateway, Port: 26500, Protocol: v1.EndpointProtocolGRPC},
				{Name: "operate", Service: gateway, Port: 8080, PathPrefix: "/operate", Protocol: http},
				{Name: "rest", Service: gateway, Port: 8080, PathPrefix: "/v2", Protocol: http},
				{Name: "tasklist", Service: gateway, Port: 8080, PathPrefix: "/tasklist", Protocol: http},
			}))
		})
	})

	// A consumer that routes each path must see the one route that moved,
	// and only that one, or it rewrites routes that still work.
	It("changes only the operate route when Operate runs standalone", func() {
		cluster := createDefaultCluster()
		var before map[string]v1.Endpoint
		expectEndpoints(cluster, func(g Gomega, latest *v1.CamundaCluster) {
			g.Expect(latest.Status.Endpoints).NotTo(BeEmpty())
			before = endpointsByName(latest.Status.Endpoints)
		})

		updateCluster(cluster, func(c *v1.CamundaCluster) {
			c.Spec.Operate = &v1.WebAppSpec{Mode: v1.ComponentModeStandalone}
		})

		expectEndpoints(cluster, func(g Gomega, latest *v1.CamundaCluster) {
			after := endpointsByName(latest.Status.Endpoints)
			g.Expect(after["operate"].Service).To(Equal(cluster.Name + "-operate"))

			delete(after, "operate")
			unchanged := make(map[string]v1.Endpoint, len(before))
			for name, endpoint := range before {
				if name != "operate" {
					unchanged[name] = endpoint
				}
			}
			g.Expect(after).To(Equal(unchanged))
		})
	})

	// The routes describe Services, and the Services stay while the pods are
	// at zero. A route kept through a suspension answers again on resume
	// without a new hostname or certificate.
	It("keeps its routes while suspended, while status.gateway is cleared", func() {
		cluster := createEmbeddedGatewayCluster()
		var before []v1.Endpoint
		expectEndpoints(cluster, func(g Gomega, latest *v1.CamundaCluster) {
			g.Expect(latest.Status.Gateway).NotTo(BeNil())
			g.Expect(latest.Status.Endpoints).NotTo(BeEmpty())
			before = latest.Status.Endpoints
		})

		updateCluster(cluster, func(c *v1.CamundaCluster) { c.Spec.Suspend = true })

		expectEndpoints(cluster, func(g Gomega, latest *v1.CamundaCluster) {
			g.Expect(latest.Status.Gateway).To(BeNil(), "the suspension reached the status")
			g.Expect(latest.Status.Endpoints).To(Equal(before))
		})
		clientKey := client.ObjectKey{Namespace: cluster.Namespace, Name: cluster.Name + "-zeebe-client"}
		Expect(k8sClient.Get(ctx, clientKey, &corev1.Service{})).To(Succeed(), "the routed Service stays")
	})

	It("routes an embedded gateway through the client Service, and deletes it on a standalone gateway", func() {
		cluster := createEmbeddedGatewayCluster()
		clientKey := client.ObjectKey{Namespace: cluster.Namespace, Name: cluster.Name + "-zeebe-client"}

		expectEndpoints(cluster, func(g Gomega, latest *v1.CamundaCluster) {
			routes := endpointsByName(latest.Status.Endpoints)
			g.Expect(routes["default"].Service).To(Equal(cluster.Name + "-zeebe-client"))
			g.Expect(routes["grpc"].Service).To(Equal(cluster.Name + "-zeebe-client"))
			g.Expect(latest.Status.Gateway).NotTo(BeNil())
			g.Expect(latest.Status.Gateway.GRPCEndpoint).To(Equal(
				cluster.Name+"-zeebe."+cluster.Namespace+".svc:26500",
			), "clients keep the address they have")
		})
		var svc corev1.Service
		Expect(k8sClient.Get(ctx, clientKey, &svc)).To(Succeed())
		Expect(svc.Spec.ClusterIP).NotTo(Equal(corev1.ClusterIPNone))
		Expect(svc.Spec.PublishNotReadyAddresses).To(BeFalse())

		updateCluster(cluster, func(c *v1.CamundaCluster) {
			c.Spec.Gateway = &v1.GatewaySpec{Mode: v1.ComponentModeStandalone}
		})

		expectEndpoints(cluster, func(g Gomega, latest *v1.CamundaCluster) {
			g.Expect(endpointsByName(latest.Status.Endpoints)["default"].Service).To(Equal(cluster.Name + "-gateway"))
		})
		Eventually(func(g Gomega) {
			g.Expect(apierrors.IsNotFound(k8sClient.Get(ctx, clientKey, &corev1.Service{}))).To(BeTrue())
		}, timeout, interval).Should(Succeed())
	})
})
