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
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/sourcehawk/operator-component-framework/pkg/component"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	v1 "github.com/konsole-is/camunda-operator/api/v1"
)

// expectOptimize polls until the stored CamundaOptimize satisfies match.
func expectOptimize(optimize *v1.CamundaOptimize, match func(Gomega, *v1.CamundaOptimize)) {
	GinkgoHelper()
	Eventually(func(g Gomega) {
		var latest v1.CamundaOptimize
		g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(optimize), &latest)).To(Succeed())
		match(g, &latest)
	}, timeout, interval).Should(Succeed())
}

// readyReason returns the reason of Ready, or "" when Ready is not set.
func readyReason(o *v1.CamundaOptimize) string {
	if ready := meta.FindStatusCondition(o.Status.Conditions, v1.ConditionReady); ready != nil {
		return ready.Reason
	}
	return ""
}

// blockDeployment creates a Deployment with a selector that the operator does
// not render. A selector is immutable, so every apply of the workload of that
// name fails before its Service exists.
func blockDeployment(namespace, name string) {
	GinkgoHelper()
	selector := map[string]string{"app": "not-optimize"}
	Expect(k8sClient.Create(ctx, &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: namespace},
		Spec: appsv1.DeploymentSpec{
			Selector: &metav1.LabelSelector{MatchLabels: selector},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{Labels: selector},
				Spec:       corev1.PodSpec{Containers: []corev1.Container{{Name: "app", Image: "busybox"}}},
			},
		},
	})).To(Succeed())
}

var _ = Describe("CamundaOptimize endpoints", func() {
	It("publishes the route of the webapp on a new Optimize", func() {
		s := newScenario("8.9.4")

		expectOptimize(s.optimize, func(g Gomega, latest *v1.CamundaOptimize) {
			g.Expect(latest.Status.Endpoints).To(Equal([]v1.Endpoint{{
				Name:       "optimize",
				Service:    s.optimize.Name + "-webapp",
				Port:       8090,
				PathPrefix: "/",
				Protocol:   v1.EndpointProtocolHTTP,
			}}))
		})
	})

	// The route describes the Service, and the Service stays while the pods
	// are at zero. A route kept through a suspension answers again on resume
	// without a new hostname or certificate.
	It("keeps its route while its cluster is suspended", func() {
		s := newScenario("8.9.4")
		var before []v1.Endpoint
		expectOptimize(s.optimize, func(g Gomega, latest *v1.CamundaOptimize) {
			g.Expect(latest.Status.Endpoints).NotTo(BeEmpty())
			before = latest.Status.Endpoints
		})

		setClusterSuspend(s.cluster, true)

		expectOptimize(s.optimize, func(g Gomega, latest *v1.CamundaOptimize) {
			g.Expect(readyReason(latest)).To(Equal(string(component.Suspended)), "the suspension reached the status")
			g.Expect(latest.Status.Endpoints).To(Equal(before))
		})
	})

	It("keeps its route while its cluster is suspended and a check of its own fails", func() {
		s := newScenario("8.9.4")
		var before []v1.Endpoint
		expectOptimize(s.optimize, func(g Gomega, latest *v1.CamundaOptimize) {
			g.Expect(latest.Status.Endpoints).NotTo(BeEmpty())
			before = latest.Status.Endpoints
		})

		Expect(k8sClient.Delete(ctx, &corev1.Secret{ObjectMeta: metav1.ObjectMeta{
			Name:      s.auth.Spec.ClientSecretRef.Name,
			Namespace: s.auth.Spec.ClientSecretRef.Namespace,
		}})).To(Succeed())
		setClusterSuspend(s.cluster, true)

		expectOptimize(s.optimize, func(g Gomega, latest *v1.CamundaOptimize) {
			ready := meta.FindStatusCondition(latest.Status.Conditions, v1.ConditionReady)
			g.Expect(ready).NotTo(BeNil())
			g.Expect(ready.Reason).To(Equal(v1.ReasonMissingSecret))
			g.Expect(ready.Message).To(ContainSubstring("scaled to zero"), "the suspension reached the status")
			g.Expect(latest.Status.Endpoints).To(Equal(before))
		})
	})

	// No webapp Service exists before the first render, so there is nothing to
	// route to.
	It("publishes no route while its first check fails", func() {
		s := newScenario("8.8.1")

		expectNotReady(s.optimize, v1.ReasonVersionMismatch)
		expectOptimize(s.optimize, func(g Gomega, latest *v1.CamundaOptimize) {
			g.Expect(latest.Status.Endpoints).To(BeEmpty())
		})
	})

	It("publishes no route while its webapp cannot be applied", func() {
		ns := newNamespace()
		cluster := createCluster(ns, createBinding(ns))
		blockDeployment(ns, "co-blocked-webapp")

		optimize := createNamedOptimize("co-blocked", ns, cluster, createAuth(ns, true), "8.9.4")

		expectClusterEnv(cluster, ContainElement("CAMUNDA_DATA_EXPORTERS_ELASTICSEARCH_CLASSNAME"))
		Consistently(func(g Gomega) {
			var latest v1.CamundaOptimize
			g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(optimize), &latest)).To(Succeed())
			g.Expect(latest.Status.Endpoints).To(BeEmpty())
		}, "3s", interval).Should(Succeed())
	})

	It("publishes its route while only its importer cannot be applied", func() {
		ns := newNamespace()
		cluster := createCluster(ns, createBinding(ns))
		blockDeployment(ns, "co-blocked-importer")

		optimize := createNamedOptimize("co-blocked", ns, cluster, createAuth(ns, true), "8.9.4")

		expectOptimize(optimize, func(g Gomega, latest *v1.CamundaOptimize) {
			g.Expect(latest.Status.Endpoints).To(HaveExactElements(
				HaveField("Service", "co-blocked-webapp"),
			))
		})
	})

	// A released instance deletes its Services, so a kept route would send
	// traffic to nothing.
	It("drops its route when its cluster is deleted", func() {
		s := newScenario("8.9.4")
		expectOptimize(s.optimize, func(g Gomega, latest *v1.CamundaOptimize) {
			g.Expect(latest.Status.Endpoints).NotTo(BeEmpty())
		})

		Expect(k8sClient.Delete(ctx, s.cluster)).To(Succeed())

		expectOptimize(s.optimize, func(g Gomega, latest *v1.CamundaOptimize) {
			g.Expect(readyReason(latest)).To(Equal(v1.ReasonInvalidReference))
			g.Expect(latest.Status.Endpoints).To(BeEmpty())
		})
	})

	It("publishes no route on an Optimize that another one parks", func() {
		_, second := newAttachedPair("8.9.4")

		expectNotReady(second, v1.ReasonClusterAlreadyAttached)
		expectOptimize(second, func(g Gomega, latest *v1.CamundaOptimize) {
			g.Expect(latest.Status.Endpoints).To(BeEmpty())
		})
	})
})
