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

package conformance

import (
	"maps"
	"sync"
	"time"

	"github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	storagev1 "k8s.io/api/storage/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	fleetv1alpha1 "github.com/konsole-is/camunda-operator/api/fleet/v1alpha1"
	pluginsv1alpha1 "github.com/konsole-is/camunda-operator/api/plugins/v1alpha1"
	"github.com/konsole-is/camunda-operator/conformance/echo"
)

var _ = ginkgo.Describe("The echo plugin", func() {
	ginkgo.It("passes every check, with a claim name of its own for the Delete and the Orphan request", func() {
		cfg := echoRun(echo.DefectNone)
		requests := watchRequests(cfg.Namespace)

		got := runAll(cfg)

		for id, o := range got {
			Expect(o.err).NotTo(HaveOccurred(), "check %s", id)
		}
		Expect(skippedIDs(got)).To(ConsistOf(
			ReqLogs, OrderRegistrationFirst, DeleteRoutes, GrantMatch, GrantRequires, GrantAdmission,
		))
		Expect(got[DeleteDelete].skipped).To(BeEmpty())
		Expect(got[DeleteOrphan].skipped).To(BeEmpty())
		for _, policy := range []string{"delete", "orphan"} {
			name := requestName("basic", "", policy, runIDOf(cfg.Namespace))
			Eventually(func(g Gomega) {
				seen := requests()
				g.Expect(seen).To(HaveKey(name))
				g.Expect(seen[name].Spec.ClaimRef.Name).To(Equal(name))
				g.Expect(seen[name].Labels).To(HaveKeyWithValue(fleetv1alpha1.LabelClaim, name))
			}, time.Minute).Should(Succeed())
		}
	})

	ginkgo.It("passes every check when it serves the access section", func() {
		got := runAll(accessRun(echo.DefectNone))

		for id, o := range got {
			Expect(o.err).NotTo(HaveOccurred(), "check %s", id)
		}
		Expect(skippedIDs(got)).To(ConsistOf(
			ReqLogs, OrderRegistrationFirst, GrantMatch, GrantRequires, GrantAdmission,
		))
	})

	ginkgo.It("runs each case for each section of a registration that serves two", func() {
		cfg := twoSectionRun()
		got, s := runOpen(cfg)
		ginkgo.DeferCleanup(func() { Expect(s.close(ctx)).To(Succeed()) })

		for id, o := range got {
			Expect(o.err).NotTo(HaveOccurred(), "check %s", id)
		}
		Expect(skippedIDs(got)).To(ConsistOf(
			ReqLogs, OrderRegistrationFirst, GrantMatch, GrantRequires, GrantAdmission,
		))

		// The orphaned ConfigMap of each section shows that a request of
		// each section ran.
		for _, section := range []string{"echo", "access"} {
			name := requestName("basic", section, "orphan", runIDOf(cfg.Namespace)) + "-echo"
			var cm corev1.ConfigMap
			Expect(k8sClient.Get(ctx, client.ObjectKey{Namespace: cfg.Namespace, Name: name}, &cm)).To(Succeed())
		}
	})

	ginkgo.It("passes every grant check when it writes a grant from a template", func() {
		got := runAll(grantRun(echo.DefectNone))

		for id, o := range got {
			Expect(o.err).NotTo(HaveOccurred(), "check %s", id)
		}
		Expect(skippedIDs(got)).To(ConsistOf(ReqLogs, OrderRegistrationFirst, DeleteRoutes))
	})

	ginkgo.It("fails GRANT-ADMISSION when a role gives serve to its ServiceAccount", func() {
		cfg := grantRun(echo.DefectNone)
		role := &rbacv1.ClusterRole{
			ObjectMeta: metav1.ObjectMeta{Name: cfg.Registration + "-serve-all"},
			Rules: []rbacv1.PolicyRule{{
				APIGroups: []string{pluginsv1alpha1.GroupVersion.Group},
				Resources: []string{"pluginregistrations"},
				Verbs:     []string{"serve"},
			}},
		}
		binding := &rbacv1.ClusterRoleBinding{
			ObjectMeta: metav1.ObjectMeta{Name: role.Name},
			RoleRef:    rbacv1.RoleRef{APIGroup: rbacv1.GroupName, Kind: "ClusterRole", Name: role.Name},
			Subjects: []rbacv1.Subject{
				{Kind: rbacv1.GroupKind, APIGroup: rbacv1.GroupName, Name: "system:serviceaccounts:" + cfg.Namespace},
			},
		}
		Expect(k8sClient.Create(ctx, role)).To(Succeed())
		Expect(k8sClient.Create(ctx, binding)).To(Succeed())
		ginkgo.DeferCleanup(func() {
			Expect(k8sClient.Delete(ctx, binding)).To(Succeed())
			Expect(k8sClient.Delete(ctx, role)).To(Succeed())
		})
		s := newSuite(cfg, restCfg, k8sClient)
		ginkgo.DeferCleanup(func() { Expect(s.close(ctx)).To(Succeed()) })

		// A new binding reaches the authorizer a short time after it is
		// created.
		Eventually(func(g Gomega) {
			sa := pluginsv1alpha1.ServiceAccountRef{Namespace: cfg.Namespace, Name: "any"}
			review, err := serveReview(ctx, s, sa, cfg.Registration)
			g.Expect(err).NotTo(HaveOccurred())
			g.Expect(review.Status.Allowed).To(BeTrue())
		}, time.Minute).Should(Succeed())

		Expect(checkGrantAdmission(ctx, s)).To(MatchError(ContainSubstring("cannot test a refusal")))
	})

	ginkgo.It("passes REQ-LABELS and DELETE-ORPHAN with a cluster-scoped object that carries the labels only", func() {
		cfg := echoRun(echo.DefectNone)
		got, s := runOpen(cfg)
		ginkgo.DeferCleanup(func() { Expect(s.close(ctx)).To(Succeed()) })

		var sc storagev1.StorageClass
		request := requestName("basic", "", "orphan", runIDOf(cfg.Namespace))
		Expect(k8sClient.Get(ctx, client.ObjectKey{Name: cfg.Namespace + "-" + request + "-echo"}, &sc)).To(Succeed())
		Expect(sc.Labels).To(HaveKeyWithValue(pluginsv1alpha1.LabelRequest, request))
		Expect(sc.Labels).To(HaveKeyWithValue(pluginsv1alpha1.LabelRegistration, cfg.Registration))
		Expect(sc.Labels).To(HaveKeyWithValue(labelRequestNamespace, cfg.Namespace))
		Expect(sc.OwnerReferences).To(BeEmpty())
		Expect(got[ReqLabels].err).NotTo(HaveOccurred())
		Expect(got[DeleteOrphan].err).NotTo(HaveOccurred())
	})

	ginkgo.It("passes a second run in the namespace of an earlier run", func() {
		cfg := echoRun(echo.DefectNone)
		// The suite deletes a namespace that it created, and envtest never
		// finishes the deletion of a namespace.
		Expect(
			k8sClient.Create(ctx, &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: cfg.Namespace}}),
		).To(Succeed())

		for range 2 {
			got := runAll(cfg)
			for id, o := range got {
				Expect(o.err).NotTo(HaveOccurred(), "check %s", id)
			}
		}
	})

	ginkgo.It("refuses a run when an object of an earlier run is left", func() {
		cfg := echoRun(echo.DefectNone)
		left := &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{
			Namespace: "default",
			Name:      cfg.Registration + "-left",
			Labels: map[string]string{
				pluginsv1alpha1.LabelRequest: requestName("basic", "", "delete", runIDOf(cfg.Namespace)),
			},
		}}
		Expect(k8sClient.Create(ctx, left)).To(Succeed())

		got := runAll(cfg)

		for _, c := range append(lifecycleChecks, deletionChecks...) {
			if got[c.ID].skipped == "" {
				Expect(got[c.ID].err).To(MatchError(ContainSubstring("of an earlier run")), "check %s", c.ID)
			}
		}
	})

	ginkgo.DescribeTable(
		"fails exactly the check of each defect, and names the defect",
		func(run func(echo.Defect) Config, defect echo.Defect, id CheckID, message string) {
			got := runAll(run(defect))

			Expect(got[id].err).To(MatchError(ContainSubstring(message)))
			for other, o := range got {
				if other != id {
					Expect(o.err).NotTo(HaveOccurred(), "check %s", other)
				}
			}
		},
		ginkgo.Entry(
			"a late finalizer",
			echoRun,
			echo.DefectLateFinalizer,
			ReqFinalizer,
			"appeared before the finalizer",
		),
		ginkgo.Entry(
			"stale OutputsReady",
			echoRun,
			echo.DefectStaleOutputsReady,
			ReqOutputsReady,
			"observedGeneration",
		),
		ginkgo.Entry("a missing output", echoRun, echo.DefectMissingOutput, ReqOutputs, `"message"`),
		ginkgo.Entry("Ready=False", echoRun, echo.DefectNotReady, ReqReady, "EchoDefect"),
		ginkgo.Entry("no owner reference", echoRun, echo.DefectNoOwnerReference, ReqLabels, "ConfigMap"),
		ginkgo.Entry(
			"an owner reference on a cluster-scoped object", echoRun, echo.DefectClusterOwnerReference,
			ReqLabels, "StorageClass",
		),
		ginkgo.Entry(
			"a cluster-scoped object without the request namespace", echoRun, echo.DefectNoRequestNamespace,
			ReqLabels, labelRequestNamespace,
		),
		ginkgo.Entry(
			"an unlisted resource",
			echoRun,
			echo.DefectUnlistedResource,
			ReqResources,
			"StorageClass",
		),
		ginkgo.Entry("no events", echoRun, echo.DefectNoEvents, ReqEvents, "event"),
		ginkgo.Entry("a relative link", echoRun, echo.DefectRelativeLink, ReqDisplay, "/echo"),
		ginkgo.Entry("a dropped Accepted", echoRun, echo.DefectDropAccepted, ReqAcceptedKept, "Accepted"),
		ginkgo.Entry("objects kept on Delete", echoRun, echo.DefectKeepOnDelete, DeleteDelete, "ConfigMap"),
		ginkgo.Entry(
			"an owner reference kept on Orphan", echoRun, echo.DefectKeepOwnerOnOrphan, DeleteOrphan,
			"owner reference",
		),
		ginkgo.Entry("a route kept on Orphan", accessRun, echo.DefectKeepRouteOnOrphan, DeleteRoutes, "Ingress"),
		ginkgo.Entry("a wrong fragment", grantRun, echo.DefectWrongFragment, GrantMatch, "has the fragment"),
		ginkgo.Entry(
			"a Grant without its required input",
			grantRun,
			echo.DefectIgnoreRequiredInputs,
			GrantRequires,
			"without a value",
		),
	)
})

// watchRequests watches the PluginRequests of namespace until the spec ends,
// and returns a func that gives the latest copy of each, by name.
func watchRequests(namespace string) func() map[string]pluginsv1alpha1.PluginRequest {
	c, err := client.NewWithWatch(restCfg, client.Options{Scheme: k8sClient.Scheme()})
	Expect(err).NotTo(HaveOccurred())
	w, err := c.Watch(ctx, &pluginsv1alpha1.PluginRequestList{}, client.InNamespace(namespace))
	Expect(err).NotTo(HaveOccurred())
	ginkgo.DeferCleanup(w.Stop)

	var mu sync.Mutex
	seen := map[string]pluginsv1alpha1.PluginRequest{}
	go func() {
		for event := range w.ResultChan() {
			if req, ok := event.Object.(*pluginsv1alpha1.PluginRequest); ok {
				mu.Lock()
				seen[req.Name] = *req
				mu.Unlock()
			}
		}
	}()

	return func() map[string]pluginsv1alpha1.PluginRequest {
		mu.Lock()
		defer mu.Unlock()

		return maps.Clone(seen)
	}
}
