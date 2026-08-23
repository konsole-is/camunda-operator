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
	"errors"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	utilrand "k8s.io/apimachinery/pkg/util/rand"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"

	v1 "github.com/konsole-is/camunda-operator/api/v1"
	components "github.com/konsole-is/camunda-operator/pkg/components/camundamanagementcluster"
)

var _ = Describe("Orchestration cluster attachment", func() {
	It("claims the clusters that the selector matches", func() {
		s := newScenario(withSelector(map[string]string{"tier": "production"}))
		served := createOrchestrationCluster(s, map[string]string{"tier": "production"}, true)
		other := createOrchestrationCluster(s, map[string]string{"tier": "staging"}, true)

		expectAttached(s.mc, served)

		Expect(readOrchestrationCluster(Default, other).Annotations).NotTo(
			HaveKey(components.ClaimAnnotation),
		)
	})

	It("claims no cluster while the selector is unset", func() {
		s := newScenario()
		cluster := createOrchestrationCluster(s, map[string]string{"tier": "production"}, true)

		Eventually(func(g Gomega) {
			g.Expect(readManagementCluster(g, s.mc).Status.Clusters).To(BeEmpty())
			g.Expect(readOrchestrationCluster(g, cluster).Annotations).NotTo(
				HaveKey(components.ClaimAnnotation),
			)
		}, timeout, interval).Should(Succeed())
	})

	It("withdraws the claim when the selector is unset again", func() {
		s := newScenario(withSelector(map[string]string{}))
		cluster := createOrchestrationCluster(s, nil, true)

		expectAttached(s.mc, cluster)

		Eventually(func(g Gomega) {
			latest := readManagementCluster(g, s.mc)
			latest.Spec.ClusterSelector = nil
			g.Expect(k8sClient.Update(ctx, latest)).To(Succeed())
		}, timeout, interval).Should(Succeed())

		expectClaimWithdrawn(cluster)
	})

	It("reports a cluster that another management plane serves", func() {
		s := newScenario(withSelector(map[string]string{}))
		claimed := createOrchestrationCluster(s, nil, true)

		Eventually(func(g Gomega) {
			latest := readOrchestrationCluster(g, claimed)
			latest.Annotations = map[string]string{components.ClaimAnnotation: "other-ns/other-management"}
			g.Expect(k8sClient.Update(ctx, latest)).To(Succeed())
		}, timeout, interval).Should(Succeed())

		Eventually(func(g Gomega) {
			row := rowOf(g, s.mc, claimed)
			g.Expect(row.Attached).To(BeFalse())
			g.Expect(row.Reason).To(Equal(v1.ReasonClaimedElsewhere))
			g.Expect(readOrchestrationCluster(g, claimed).Annotations).To(
				HaveKeyWithValue(components.ClaimAnnotation, "other-ns/other-management"),
			)
		}, timeout, interval).Should(Succeed())
	})

	It("reports a cluster that publishes no gateway endpoints", func() {
		s := newScenario(withSelector(map[string]string{}))
		starting := createOrchestrationCluster(s, nil, false)

		Eventually(func(g Gomega) {
			row := rowOf(g, s.mc, starting)
			g.Expect(row.Attached).To(BeFalse())
			g.Expect(row.Reason).To(Equal(v1.ReasonNotReady))
			g.Expect(readOrchestrationCluster(g, starting).Annotations).To(
				HaveKeyWithValue(components.ClaimAnnotation, ClaimValue(s.mc)),
			)
		}, timeout, interval).Should(Succeed())
	})

	It("withdraws the claim from a cluster that leaves the selector", func() {
		s := newScenario(withSelector(map[string]string{"tier": "production"}))
		cluster := createOrchestrationCluster(s, map[string]string{"tier": "production"}, true)

		expectAttached(s.mc, cluster)

		Eventually(func(g Gomega) {
			latest := readOrchestrationCluster(g, cluster)
			latest.Labels = map[string]string{"tier": "staging"}
			g.Expect(k8sClient.Update(ctx, latest)).To(Succeed())
		}, timeout, interval).Should(Succeed())

		expectClaimWithdrawn(cluster)
	})

	// A refused claim concerns one cluster: the cluster changed while the
	// reconcile ran, and its own event brings the next one. The API server is
	// reachable, so every other cluster must still converge in this pass.
	It("refuses the second of two management clusters that claim one cluster at once", func() {
		// Two management clusters that read an unclaimed cluster at the same
		// time both try the claim. The annotation belongs to the manager of
		// the first, and the API server refuses the second without forced
		// ownership, so one holder remains and the other reads it on its next
		// pass.
		s := newScenario(withSelector(map[string]string{"unmatched": "none"}))
		cluster := createOrchestrationCluster(s, nil, true)

		first := newManagementCluster("first-plane", "identity-db")
		first.UID = types.UID("11111111-1111-1111-1111-111111111111")
		second := newManagementCluster("second-plane", "identity-db")
		second.UID = types.UID("22222222-2222-2222-2222-222222222222")
		r := &Reconciler{Client: k8sClient, APIReader: k8sClient, Scheme: k8sClient.Scheme()}

		var latest v1.CamundaCluster
		Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(cluster), &latest)).To(Succeed())
		Expect(r.claim(ctx, first, &latest)).To(Succeed())

		err := r.claim(ctx, second, &latest)
		Expect(apierrors.IsConflict(err)).To(BeTrue(), "the second claim must be refused, got: %v", err)
		Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(cluster), &latest)).To(Succeed())
		Expect(latest.Annotations).To(HaveKeyWithValue(components.ClaimAnnotation, ClaimValue(first)))

		// The holder withdraws, and the other takes the cluster on its next
		// pass.
		Expect(r.withdrawClaim(ctx, first, &latest)).To(Succeed())
		Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(cluster), &latest)).To(Succeed())
		Expect(r.claim(ctx, second, &latest)).To(Succeed())
		Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(cluster), &latest)).To(Succeed())
		Expect(latest.Annotations).To(HaveKeyWithValue(components.ClaimAnnotation, ClaimValue(second)))
	})

	It("serves only the namespaces that the namespaceSelector matches", func() {
		s := newScenario(withSelector(map[string]string{}), withNamespaceSelector(map[string]string{"team": "a"}))

		setNamespaceLabels(s.namespace, map[string]string{"team": "a"})
		inside := createOrchestrationCluster(s, nil, true)

		outside := inside.DeepCopy()
		outside.ObjectMeta = metav1.ObjectMeta{
			Name:      "cc-" + utilrand.String(8),
			Namespace: newTestNamespace(),
		}
		outside.ResourceVersion = ""
		outside.Status = v1.CamundaClusterStatus{}
		Expect(k8sClient.Create(ctx, outside)).To(Succeed())
		DeferCleanup(func() { _ = k8sClient.Delete(ctx, outside) })

		Eventually(func(g Gomega) {
			g.Expect(readOrchestrationCluster(g, inside).Annotations).To(
				HaveKeyWithValue(components.ClaimAnnotation, ClaimValue(s.mc)),
			)
			g.Expect(readOrchestrationCluster(g, outside).Annotations).NotTo(HaveKey(components.ClaimAnnotation))
			latest := readManagementCluster(g, s.mc)
			g.Expect(latest.Status.Clusters).To(HaveLen(1))
			g.Expect(latest.Status.Clusters[0].Name).To(Equal(inside.Name))
		}, timeout, interval).Should(Succeed())

		// The namespace leaves the bound, and the claim goes with it.
		setNamespaceLabels(s.namespace, nil)
		Eventually(func(g Gomega) {
			g.Expect(readOrchestrationCluster(g, inside).Annotations).NotTo(HaveKey(components.ClaimAnnotation))
			g.Expect(readManagementCluster(g, s.mc).Status.Clusters).To(BeEmpty())
		}, timeout, interval).Should(Succeed())
	})

	It("goes on when the API server refuses one claim", func() {
		mc := newManagementCluster("camunda", "identity-db")
		mc.Spec.ClusterSelector = &metav1.LabelSelector{}

		platform := newPlatformConfig(mc.Namespace)
		held := readyCluster("cc-held", mc.Namespace, platform.Name)
		held.Annotations = map[string]string{components.ClaimAnnotation: ClaimValue(mc)}
		refused := readyCluster("cc-refused", mc.Namespace, platform.Name)

		r := &Reconciler{
			Client:    refusingClient(),
			APIReader: readerWith(platform, held, refused),
			Scheme:    k8sClient.Scheme(),
		}

		clusters, err := r.listClusters(ctx)
		Expect(err).NotTo(HaveOccurred())

		attached, rows, err := r.attachedClusters(ctx, mc, clusters, nil)

		Expect(err).NotTo(HaveOccurred())
		Expect(attached).To(HaveLen(1))
		Expect(attached[0].Name).To(Equal(held.Name))
		Expect(rows).To(HaveLen(2))
		Expect(rows[0].Attached).To(BeTrue())
		Expect(rows[1].Attached).To(BeFalse())
		Expect(rows[1].Reason).To(Equal(v1.ReasonNotReady))
	})

	It("withdraws every claim with the management cluster", func() {
		s := newScenario(withSelector(map[string]string{}))
		cluster := createOrchestrationCluster(s, nil, true)

		expectAttached(s.mc, cluster)

		Expect(deleteManagementCluster(s.mc)).To(Succeed())

		expectClaimWithdrawn(cluster)
	})
})

// withSelector sets spec.clusterSelector of the management cluster under test.
func withSelector(matchLabels map[string]string) func(f *fixture) {
	return func(f *fixture) {
		f.mc.Spec.ClusterSelector = &metav1.LabelSelector{MatchLabels: matchLabels}
	}
}

func withNamespaceSelector(matchLabels map[string]string) func(f *fixture) {
	return func(f *fixture) {
		f.mc.Spec.NamespaceSelector = &metav1.LabelSelector{MatchLabels: matchLabels}
	}
}

// userEnv is an entry that the user of an orchestration cluster set. Every
// cluster of a scenario carries it, so a spec can pin that the management
// plane owns the ping entries and leaves the rest of spec.extraEnv alone.
var userEnv = corev1.EnvVar{Name: "USER_SETTING", Value: "1"}

// createOrchestrationCluster creates a CamundaCluster in the namespace of the
// scenario with the given labels. A ready cluster publishes its gateway
// endpoints, the way its own controller does once it is up.
// setNamespaceLabels replaces the labels of a namespace.
func setNamespaceLabels(name string, set map[string]string) {
	GinkgoHelper()

	Eventually(func(g Gomega) {
		var namespace corev1.Namespace
		g.Expect(k8sClient.Get(ctx, client.ObjectKey{Name: name}, &namespace)).To(Succeed())
		namespace.Labels = set
		g.Expect(k8sClient.Update(ctx, &namespace)).To(Succeed())
	}, timeout, interval).Should(Succeed())
}

// newTestNamespace creates a namespace of its own and returns its name.
func newTestNamespace() string {
	GinkgoHelper()

	namespace := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "ns-" + utilrand.String(8)}}
	Expect(k8sClient.Create(ctx, namespace)).To(Succeed())
	DeferCleanup(func() { _ = k8sClient.Delete(ctx, namespace) })

	return namespace.Name
}

func createOrchestrationCluster(
	s scenario,
	clusterLabels map[string]string,
	ready bool,
) *v1.CamundaCluster {
	GinkgoHelper()

	cluster := &v1.CamundaCluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "cc-" + utilrand.String(8),
			Namespace: s.namespace,
			Labels:    clusterLabels,
		},
		Spec: v1.CamundaClusterSpec{
			Version:           "8.9.4",
			StorageRef:        "storage",
			PlatformConfigRef: s.mc.Spec.PlatformConfigRef,
			ExtraEnv:          []corev1.EnvVar{userEnv},
		},
	}
	Expect(k8sClient.Create(ctx, cluster)).To(Succeed())
	DeferCleanup(func() { _ = k8sClient.Delete(ctx, cluster) })

	if ready {
		cluster.Status.Gateway = &v1.GatewayBinding{
			GRPCEndpoint: cluster.Name + "-gateway." + s.namespace + ".svc:26500",
			RESTEndpoint: "http://" + cluster.Name + "-gateway." + s.namespace + ".svc:8080",
		}
		Expect(k8sClient.Status().Update(ctx, cluster)).To(Succeed())
	}

	return cluster
}

// readyCluster returns a CamundaCluster that publishes its gateway endpoints.
// It is never created: the specs that use it drive a Reconciler that reads
// through a fake client.
func readyCluster(name, namespace, platformRef string) *v1.CamundaCluster {
	return &v1.CamundaCluster{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: namespace, UID: types.UID(name)},
		Spec: v1.CamundaClusterSpec{
			Version:           "8.9.4",
			StorageRef:        "storage",
			PlatformConfigRef: platformRef,
		},
		Status: v1.CamundaClusterStatus{Gateway: &v1.GatewayBinding{
			GRPCEndpoint: name + "-gateway." + namespace + ".svc:26500",
			RESTEndpoint: "http://" + name + "-gateway." + namespace + ".svc:8080",
		}},
	}
}

// readerWith returns a reader that holds the given objects.
func readerWith(objects ...client.Object) client.Reader {
	return fake.NewClientBuilder().WithScheme(k8sClient.Scheme()).WithObjects(objects...).Build()
}

// refusingClient returns a client that answers every apply with a conflict,
// the way the API server answers an apply against a cluster that changed
// under the UID precondition of the claim.
func refusingClient() client.Client {
	return fake.NewClientBuilder().
		WithScheme(k8sClient.Scheme()).
		WithInterceptorFuncs(interceptor.Funcs{
			Patch: func(
				_ context.Context,
				_ client.WithWatch,
				obj client.Object,
				_ client.Patch,
				_ ...client.PatchOption,
			) error {
				return apierrors.NewConflict(
					v1.GroupVersion.WithResource("camundaclusters").GroupResource(),
					obj.GetName(),
					errors.New("uid mismatch"),
				)
			},
		}).
		Build()
}

// readOrchestrationCluster reads a CamundaCluster as the API server holds it
// now.
func readOrchestrationCluster(g Gomega, cluster *v1.CamundaCluster) *v1.CamundaCluster {
	var latest v1.CamundaCluster
	g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(cluster), &latest)).To(Succeed())

	return &latest
}

// rowOf reads the status.clusters row of one orchestration cluster and asserts
// that the management cluster reports it.
func rowOf(g Gomega, mc *v1.CamundaManagementCluster, cluster *v1.CamundaCluster) v1.AttachedClusterStatus {
	for _, row := range readManagementCluster(g, mc).Status.Clusters {
		if row.Name == cluster.Name && row.Namespace == cluster.Namespace {
			return row
		}
	}
	g.Expect(false).To(BeTrue(), "no status.clusters row for %s/%s", cluster.Namespace, cluster.Name)

	return v1.AttachedClusterStatus{}
}

// expectAttached polls until the management cluster serves the orchestration
// cluster: the claim is on it and its row reports it attached.
func expectAttached(mc *v1.CamundaManagementCluster, cluster *v1.CamundaCluster) {
	GinkgoHelper()

	Eventually(func(g Gomega) {
		g.Expect(readOrchestrationCluster(g, cluster).Annotations).To(
			HaveKeyWithValue(components.ClaimAnnotation, ClaimValue(mc)),
		)
		g.Expect(rowOf(g, mc, cluster).Attached).To(BeTrue())
	}, timeout, interval).Should(Succeed())
}

// expectClaimWithdrawn polls until no claim is left on the orchestration
// cluster.
func expectClaimWithdrawn(cluster *v1.CamundaCluster) {
	GinkgoHelper()

	Eventually(func(g Gomega) {
		g.Expect(readOrchestrationCluster(g, cluster).Annotations).NotTo(
			HaveKey(components.ClaimAnnotation),
		)
	}, timeout, interval).Should(Succeed())
}
