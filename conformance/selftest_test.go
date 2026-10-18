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
	"github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	storagev1 "k8s.io/api/storage/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	pluginsv1alpha1 "github.com/konsole-is/camunda-operator/api/plugins/v1alpha1"
	"github.com/konsole-is/camunda-operator/conformance/echo"
)

var _ = ginkgo.Describe("The echo plugin", func() {
	ginkgo.It("passes every check", func() {
		got := runAll(echoRun(echo.DefectNone))

		for id, o := range got {
			Expect(o.err).NotTo(HaveOccurred(), "check %s", id)
		}
		Expect(skippedIDs(got)).To(ConsistOf(CheckRequestLogs, CheckOrderRegistrationFirst, CheckDeleteRoutes))
		Expect(got[CheckDeleteDelete].skipped).To(BeEmpty())
		Expect(got[CheckDeleteOrphan].skipped).To(BeEmpty())
	})

	ginkgo.It("passes every check when it serves the access section", func() {
		got := runAll(accessRun(echo.DefectNone))

		for id, o := range got {
			Expect(o.err).NotTo(HaveOccurred(), "check %s", id)
		}
		Expect(skippedIDs(got)).To(ConsistOf(CheckRequestLogs, CheckOrderRegistrationFirst))
	})

	ginkgo.It("passes REQ-LABELS and DELETE-ORPHAN with a cluster-scoped object that carries the labels only", func() {
		cfg := echoRun(echo.DefectNone)
		got := runAll(cfg)

		var sc storagev1.StorageClass
		Expect(k8sClient.Get(ctx, client.ObjectKey{Name: cfg.Namespace + "-basic-orphan-echo"}, &sc)).To(Succeed())
		Expect(sc.Labels).To(HaveKeyWithValue(pluginsv1alpha1.LabelRequest, "basic-orphan"))
		Expect(sc.Labels).To(HaveKeyWithValue(pluginsv1alpha1.LabelRegistration, cfg.Registration))
		Expect(sc.OwnerReferences).To(BeEmpty())
		Expect(got[CheckRequestLabels].err).NotTo(HaveOccurred())
		Expect(got[CheckDeleteOrphan].err).NotTo(HaveOccurred())
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
			CheckRequestFinalizer,
			"appeared before the finalizer",
		),
		ginkgo.Entry(
			"stale OutputsReady",
			echoRun,
			echo.DefectStaleOutputsReady,
			CheckRequestOutputsReady,
			"observedGeneration",
		),
		ginkgo.Entry("a missing output", echoRun, echo.DefectMissingOutput, CheckRequestOutputs, `"message"`),
		ginkgo.Entry("Ready=False", echoRun, echo.DefectNotReady, CheckRequestReady, "EchoDefect"),
		ginkgo.Entry("no owner reference", echoRun, echo.DefectNoOwnerReference, CheckRequestLabels, "ConfigMap"),
		ginkgo.Entry(
			"an owner reference on a cluster-scoped object", echoRun, echo.DefectClusterOwnerReference,
			CheckRequestLabels, "StorageClass",
		),
		ginkgo.Entry(
			"an unlisted resource",
			echoRun,
			echo.DefectUnlistedResource,
			CheckRequestResources,
			"StorageClass",
		),
		ginkgo.Entry("no events", echoRun, echo.DefectNoEvents, CheckRequestEvents, "event"),
		ginkgo.Entry("a relative link", echoRun, echo.DefectRelativeLink, CheckRequestDisplay, "/echo"),
		ginkgo.Entry("a dropped Accepted", echoRun, echo.DefectDropAccepted, CheckRequestAcceptedKept, "Accepted"),
		ginkgo.Entry("objects kept on Delete", echoRun, echo.DefectKeepOnDelete, CheckDeleteDelete, "ConfigMap"),
		ginkgo.Entry(
			"an owner reference kept on Orphan", echoRun, echo.DefectKeepOwnerOnOrphan, CheckDeleteOrphan,
			"owner reference",
		),
		ginkgo.Entry("a route kept on Orphan", accessRun, echo.DefectKeepRouteOnOrphan, CheckDeleteRoutes, "Ingress"),
	)
})
