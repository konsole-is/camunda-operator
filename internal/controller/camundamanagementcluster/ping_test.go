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
	"context"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"

	v1 "github.com/konsole-is/camunda-operator/api/v1"
	components "github.com/konsole-is/camunda-operator/pkg/components/camundamanagementcluster"
)

var _ = Describe("Console and the ping of the clusters it lists", func() {
	It("deploys Console next to Management Identity", func() {
		s := newScenario(withConsole)

		expectReadyWhileStamping(
			s.mc,
			client.ObjectKey{Namespace: s.namespace, Name: components.IdentityName(s.mc)},
			client.ObjectKey{Namespace: s.namespace, Name: components.ConsoleName(s.mc)},
		)

		var workload appsv1.Deployment
		key := client.ObjectKey{Namespace: s.namespace, Name: components.ConsoleName(s.mc)}
		Expect(k8sClient.Get(ctx, key, &workload)).To(Succeed())
		Expect(workload.Spec.Template.Spec.Containers[0].Env).To(ContainElement(
			corev1.EnvVar{Name: "CAMUNDA_CONSOLE_EXPERIMENTAL_DISCOVERY_MODE", Value: "true"},
		))

		var svc corev1.Service
		Expect(k8sClient.Get(ctx, key, &svc)).To(Succeed())
	})

	It("points every attached cluster at Console", func() {
		s := newScenario(withSelector(map[string]string{}), withConsole)
		cluster := createOrchestrationCluster(s, nil, true)

		expectAttached(s.mc, cluster)

		Eventually(func(g Gomega) {
			g.Expect(pingOf(g, cluster)).To(Equal(map[string]string{
				"CAMUNDA_CONSOLE_PING_ENABLED":     "true",
				"CAMUNDA_CONSOLE_PING_ENDPOINT":    components.ConsoleServiceURL(s.mc),
				"CAMUNDA_CONSOLE_PING_CLUSTERNAME": cluster.Name,
				"CAMUNDA_CONSOLE_PING_PINGPERIOD":  "1h",
			}))
		}, timeout, interval).Should(Succeed())
	})

	// The claim and the ping are two server-side applies against one cluster.
	// Under a single field manager each one would strip what the other wrote,
	// and the two would take turns for as long as the management plane runs.
	It("keeps the claim and the ping side by side", func() {
		s := newScenario(withSelector(map[string]string{}), withConsole)
		cluster := createOrchestrationCluster(s, nil, true)

		expectAttached(s.mc, cluster)
		expectPingApplied(cluster, s.mc)

		Consistently(func(g Gomega) {
			latest := readOrchestrationCluster(g, cluster)
			g.Expect(latest.Annotations).To(HaveKeyWithValue(components.ClaimAnnotation, ClaimValue(s.mc)))
			g.Expect(components.PingsConsole(latest.Spec.ExtraEnv, components.ConsoleServiceURL(s.mc))).
				To(BeTrue())
		}, "3s", interval).Should(Succeed())
	})

	// Camunda 8.10 renamed Console to Hub and the ping settings with it, so a
	// cluster of that version reads a key set of its own.
	It("gives a cluster on 8.10 the hub settings", func() {
		s := newScenario(withSelector(map[string]string{}), withConsole)
		cluster := createOrchestrationCluster(s, nil, true)
		publishVersion(cluster, "8.10.0")

		Eventually(func(g Gomega) {
			g.Expect(pingOf(g, cluster)).To(Equal(map[string]string{
				"CAMUNDA_HUB_PING_ENABLED":     "true",
				"CAMUNDA_HUB_PING_ENDPOINT":    components.ConsoleServiceURL(s.mc),
				"CAMUNDA_HUB_PING_CLUSTERNAME": cluster.Name,
				"CAMUNDA_HUB_PING_PINGPERIOD":  "1h",
			}))
		}, timeout, interval).Should(Succeed())
	})

	// Only a returned error retries a ping that failed for a reason other
	// than the cluster changing. The row of the cluster is stable, so it
	// brings no watch event and nothing else would reach the cluster again.
	It("returns a ping that failed for any other reason", func() {
		mc := newManagementCluster("camunda", "identity-db")
		mc.Spec.ClusterSelector = &metav1.LabelSelector{}
		mc.Spec.Console = &v1.ConsoleSpec{Version: "8.9.4", ExternalURL: consoleExternalURL}
		cluster := readyCluster("cc-unreachable", mc.Namespace, "platform")

		r := &Reconciler{
			Client:    unavailableClient(),
			APIReader: readerWith(cluster),
			Scheme:    k8sClient.Scheme(),
		}
		attached := []components.AttachedCluster{{
			Name:      cluster.Name,
			Namespace: cluster.Namespace,
			UID:       cluster.UID,
			Version:   cluster.Spec.Version,
		}}

		err := r.syncPing(ctx, mc, []v1.CamundaCluster{*cluster}, attached)

		Expect(err).To(MatchError(ContainSubstring("applying the Console ping settings")))
	})

	It("withdraws the ping when Console is disabled, and keeps the claim", func() {
		s := newScenario(withSelector(map[string]string{}), withConsole)
		cluster := createOrchestrationCluster(s, nil, true)

		expectAttached(s.mc, cluster)
		expectPingApplied(cluster, s.mc)

		Eventually(func(g Gomega) {
			latest := readManagementCluster(g, s.mc)
			latest.Spec.Console = nil
			g.Expect(k8sClient.Update(ctx, latest)).To(Succeed())
		}, timeout, interval).Should(Succeed())

		expectPingWithdrawn(cluster, s.mc)

		Expect(readOrchestrationCluster(Default, cluster).Annotations).To(
			HaveKeyWithValue(components.ClaimAnnotation, ClaimValue(s.mc)),
		)
	})

	It("withdraws the ping from a cluster that leaves the selector", func() {
		s := newScenario(withSelector(map[string]string{"tier": "production"}), withConsole)
		cluster := createOrchestrationCluster(s, map[string]string{"tier": "production"}, true)

		expectAttached(s.mc, cluster)
		expectPingApplied(cluster, s.mc)

		Eventually(func(g Gomega) {
			latest := readOrchestrationCluster(g, cluster)
			latest.Labels = map[string]string{"tier": "staging"}
			g.Expect(k8sClient.Update(ctx, latest)).To(Succeed())
		}, timeout, interval).Should(Succeed())

		expectPingWithdrawn(cluster, s.mc)
		expectClaimWithdrawn(cluster)
	})

	It("withdraws the ping with the management cluster", func() {
		s := newScenario(withSelector(map[string]string{}), withConsole)
		cluster := createOrchestrationCluster(s, nil, true)

		expectAttached(s.mc, cluster)
		expectPingApplied(cluster, s.mc)

		Expect(deleteManagementCluster(s.mc)).To(Succeed())

		expectPingWithdrawn(cluster, s.mc)
	})
})

// consoleExternalURL is where a browser reaches the Console of a scenario.
const consoleExternalURL = "https://console.example.com"

// withConsole deploys Console on the management cluster under test and
// declares its public client on the platform config.
func withConsole(f *fixture) {
	f.mc.Spec.Console = &v1.ConsoleSpec{Version: "8.9.4", ExternalURL: consoleExternalURL}
	f.platform.Spec.Auth.OIDC.Management.Clients.Console = &v1.PublicClientSpec{ClientID: "console"}
}

// publishVersion writes the Camunda version that a cluster reports, the way
// its own controller publishes it once the cluster is up.
func publishVersion(cluster *v1.CamundaCluster, version string) {
	GinkgoHelper()

	Eventually(func(g Gomega) {
		latest := readOrchestrationCluster(g, cluster)
		latest.Status.Management = &v1.ManagementBinding{
			Endpoint:   "http://" + latest.Name + "-zeebe." + latest.Namespace + ".svc:9600",
			Auth:       v1.ManagementAuth{Method: v1.ManagementAuthMethodNone},
			Version:    version,
			Partitions: 1,
		}
		g.Expect(k8sClient.Status().Update(ctx, latest)).To(Succeed())
	}, timeout, interval).Should(Succeed())
}

// pingOf returns the ping settings of one orchestration cluster, by name. The
// entries of the user are left out, so a spec pins what the management plane
// wrote and nothing else.
func pingOf(g Gomega, cluster *v1.CamundaCluster) map[string]string {
	ping := map[string]string{}
	for _, e := range readOrchestrationCluster(g, cluster).Spec.ExtraEnv {
		if strings.HasPrefix(e.Name, "CAMUNDA_CONSOLE_PING_") || strings.HasPrefix(e.Name, "CAMUNDA_HUB_PING_") {
			ping[e.Name] = e.Value
		}
	}

	return ping
}

// unavailableClient returns a client that answers every apply with a service
// that is unavailable, the way the API server answers while it cannot reach
// its backing store.
func unavailableClient() client.Client {
	return fake.NewClientBuilder().
		WithScheme(k8sClient.Scheme()).
		WithInterceptorFuncs(interceptor.Funcs{
			Patch: func(
				_ context.Context,
				_ client.WithWatch,
				_ client.Object,
				_ client.Patch,
				_ ...client.PatchOption,
			) error {
				return apierrors.NewServiceUnavailable("the API server is not reachable")
			},
		}).
		Build()
}

// expectPingApplied polls until the orchestration cluster reports to the
// Console of the management cluster.
func expectPingApplied(cluster *v1.CamundaCluster, mc *v1.CamundaManagementCluster) {
	GinkgoHelper()

	Eventually(func(g Gomega) {
		latest := readOrchestrationCluster(g, cluster)
		g.Expect(components.PingsConsole(latest.Spec.ExtraEnv, components.ConsoleServiceURL(mc))).To(BeTrue())
	}, timeout, interval).Should(Succeed())
}

// expectPingWithdrawn polls until no ping setting of the management cluster is
// left on the orchestration cluster.
func expectPingWithdrawn(cluster *v1.CamundaCluster, mc *v1.CamundaManagementCluster) {
	GinkgoHelper()

	Eventually(func(g Gomega) {
		g.Expect(pingOf(g, cluster)).To(BeEmpty())
		latest := readOrchestrationCluster(g, cluster)
		g.Expect(components.PingsConsole(latest.Spec.ExtraEnv, components.ConsoleServiceURL(mc))).To(BeFalse())
	}, timeout, interval).Should(Succeed())
}
