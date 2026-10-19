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

package builtin

import (
	"strings"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"

	pluginsv1alpha1 "github.com/konsole-is/camunda-operator/api/plugins/v1alpha1"
	v1 "github.com/konsole-is/camunda-operator/api/v1"
	"github.com/konsole-is/camunda-operator/internal/pluginstartup"
	"github.com/konsole-is/camunda-operator/pkg/conditions"
)

var _ = Describe("Built-in requests", func() {
	DescribeTable(
		"publishes the output once the contract exists",
		func(registration, output string, contract func() client.Object) {
			obj := contract()
			request := newRequest(registration, `{"name":"`+obj.GetName()+`"}`, pluginsv1alpha1.DeletionPolicyDelete)

			live := expectCondition(request, pluginsv1alpha1.ConditionOutputsReady, metav1.ConditionFalse, "Blocked")
			Expect(live.Finalizers).To(ContainElement(pluginsv1alpha1.FinalizerCleanup))
			Expect(meta.IsStatusConditionFalse(live.Status.Conditions, v1.ConditionReady)).To(BeTrue())
			Expect(live.Status.Outputs).To(BeEmpty())

			createContract(obj)

			live = expectCondition(
				request,
				pluginsv1alpha1.ConditionOutputsReady,
				metav1.ConditionTrue,
				v1.ReasonHealthy,
			)
			live = expectCondition(live, v1.ConditionReady, metav1.ConditionTrue, v1.ReasonHealthy)
			Expect(live.Status.ObservedGeneration).To(Equal(live.Generation))
			Expect(live.Status.Outputs).To(Equal(map[string]string{output: obj.GetName()}))
			Expect(live.Status.Resources).To(BeEmpty())
		},
		Entry(
			pluginstartup.RegistrationExistingObjectStorage,
			pluginstartup.RegistrationExistingObjectStorage, pluginstartup.OutputObjectStorageConfig,
			func() client.Object { return newObjectStorageConfig() },
		),
		Entry(
			pluginstartup.RegistrationExistingSecondaryStorage,
			pluginstartup.RegistrationExistingSecondaryStorage, pluginstartup.OutputSecondaryStorageConfig,
			func() client.Object { return newSecondaryStorageConfig() },
		),
	)

	It("withdraws the output when the contract is deleted", func() {
		contract := newObjectStorageConfig()
		createContract(contract)
		request := newRequest(
			pluginstartup.RegistrationExistingObjectStorage,
			`{"name":"`+contract.Name+`"}`,
			pluginsv1alpha1.DeletionPolicyDelete,
		)
		expectCondition(request, pluginsv1alpha1.ConditionOutputsReady, metav1.ConditionTrue, v1.ReasonHealthy)

		Expect(k8sClient.Delete(ctx, contract)).To(Succeed())

		live := expectCondition(request, pluginsv1alpha1.ConditionOutputsReady, metav1.ConditionFalse, "Blocked")
		Expect(live.Status.Outputs).To(BeEmpty())
	})

	It("shows the Ready state of the contract in the summary", func() {
		contract := newObjectStorageConfig()
		createContract(contract)
		request := newRequest(
			pluginstartup.RegistrationExistingObjectStorage,
			`{"name":"`+contract.Name+`"}`,
			pluginsv1alpha1.DeletionPolicyDelete,
		)
		live := expectCondition(request, pluginsv1alpha1.ConditionOutputsReady, metav1.ConditionTrue, v1.ReasonHealthy)
		Expect(live.Status.Summary).To(HaveExactElements(pluginsv1alpha1.SummaryItem{
			Label: "ObjectStorageConfig", Value: contract.Name + ", not Ready", Severity: pluginsv1alpha1.SeverityWarning,
		}))

		// The controller of the contract does not run in this suite, so the
		// spec writes its Ready condition.
		Eventually(func() error {
			var fresh v1.ObjectStorageConfig
			if err := k8sClient.Get(ctx, client.ObjectKeyFromObject(contract), &fresh); err != nil {
				return err
			}
			meta.SetStatusCondition(&fresh.Status.Conditions, metav1.Condition{
				Type: v1.ConditionReady, Status: metav1.ConditionTrue, Reason: v1.ReasonHealthy, Message: "Ready",
			})
			return k8sClient.Status().Update(ctx, &fresh)
		}, timeout, interval).Should(Succeed())

		Eventually(func(g Gomega) {
			live := liveRequest(g, request)
			g.Expect(live.Status.Summary).To(HaveExactElements(pluginsv1alpha1.SummaryItem{
				Label: "ObjectStorageConfig", Value: contract.Name + ", Ready", Severity: pluginsv1alpha1.SeverityInfo,
			}))
			g.Expect(live.Status.SummaryTime).NotTo(BeNil())
		}, timeout, interval).Should(Succeed())
	})

	It("reports a missing contract whose name is longer than a condition message", func() {
		name := strings.Repeat("a", 40000)
		request := newRequest(
			pluginstartup.RegistrationExistingObjectStorage,
			`{"name":"`+name+`"}`,
			pluginsv1alpha1.DeletionPolicyDelete,
		)

		Eventually(func(g Gomega) {
			live := liveRequest(g, request)
			cond := meta.FindStatusCondition(live.Status.Conditions, pluginsv1alpha1.ConditionOutputsReady)
			g.Expect(cond).NotTo(BeNil())
			g.Expect(cond.Status).To(Equal(metav1.ConditionFalse))
			g.Expect(len(cond.Message)).To(BeNumerically("<=", conditions.MaxMessageLength))
		}, timeout, interval).Should(Succeed())
	})

	It("refuses two equal DatabaseConfig names", func() {
		request := newRequest(
			pluginstartup.RegistrationExistingDatabaseConfigs,
			`{"keycloak":"shared","identity":"shared"}`,
			pluginsv1alpha1.DeletionPolicyDelete,
		)

		expectCondition(
			request,
			pluginsv1alpha1.ConditionOutputsReady,
			metav1.ConditionFalse,
			pluginsv1alpha1.ReasonInvalidParameters,
		)
		live := expectCondition(
			request,
			v1.ConditionReady,
			metav1.ConditionFalse,
			pluginsv1alpha1.ReasonInvalidParameters,
		)
		ready := meta.FindStatusCondition(live.Status.Conditions, v1.ConditionReady)
		Expect(ready.Message).To(ContainSubstring("spec.parameters.keycloak"))
		Expect(ready.Message).To(ContainSubstring("spec.parameters.identity"))
		Expect(live.Status.Outputs).To(BeEmpty())
	})

	It("refuses parameters that do not pass the schema", func() {
		request := newRequest(
			pluginstartup.RegistrationExistingObjectStorage,
			`{}`,
			pluginsv1alpha1.DeletionPolicyDelete,
		)

		expectCondition(
			request,
			pluginsv1alpha1.ConditionOutputsReady,
			metav1.ConditionFalse,
			pluginsv1alpha1.ReasonInvalidParameters,
		)
		live := expectCondition(
			request,
			v1.ConditionReady,
			metav1.ConditionFalse,
			pluginsv1alpha1.ReasonInvalidParameters,
		)
		Expect(meta.FindStatusCondition(live.Status.Conditions, v1.ConditionReady).Message).
			To(ContainSubstring("spec.parameters.name"))
	})

	DescribeTable(
		"removes the finalizer and keeps the contract on deletion",
		func(policy pluginsv1alpha1.DeletionPolicy) {
			contract := newObjectStorageConfig()
			createContract(contract)
			request := newRequest(
				pluginstartup.RegistrationExistingObjectStorage,
				`{"name":"`+contract.Name+`"}`,
				policy,
			)
			expectCondition(request, pluginsv1alpha1.ConditionOutputsReady, metav1.ConditionTrue, v1.ReasonHealthy)

			Expect(k8sClient.Delete(ctx, request)).To(Succeed())

			expectGone(request)
			Consistently(func() error {
				return k8sClient.Get(ctx, client.ObjectKeyFromObject(contract), &v1.ObjectStorageConfig{})
			}, 2*time.Second, interval).Should(Succeed())
		},
		Entry("Delete", pluginsv1alpha1.DeletionPolicyDelete),
		Entry("Orphan", pluginsv1alpha1.DeletionPolicyOrphan),
	)

	DescribeTable(
		"finishes the deletion of a request whose label changed",
		func(change func(labels map[string]string)) {
			contract := newObjectStorageConfig()
			createContract(contract)
			request := newRequest(
				pluginstartup.RegistrationExistingObjectStorage,
				`{"name":"`+contract.Name+`"}`,
				pluginsv1alpha1.DeletionPolicyDelete,
			)
			expectCondition(request, pluginsv1alpha1.ConditionOutputsReady, metav1.ConditionTrue, v1.ReasonHealthy)

			Eventually(func() error {
				live := &pluginsv1alpha1.PluginRequest{}
				if err := k8sClient.Get(ctx, client.ObjectKeyFromObject(request), live); err != nil {
					return err
				}
				change(live.Labels)
				return k8sClient.Update(ctx, live)
			}, timeout, interval).Should(Succeed())
			Expect(k8sClient.Delete(ctx, request)).To(Succeed())

			expectGone(request)
		},
		Entry("label removed", func(labels map[string]string) {
			delete(labels, pluginsv1alpha1.LabelRegistration)
		}),
		Entry("label replaced", func(labels map[string]string) {
			labels[pluginsv1alpha1.LabelRegistration] = "s3-backup-bucket"
		}),
	)

	DescribeTable(
		"leaves alone a request that the operator does not serve",
		func(label, registration string) {
			request := createRequest(label, registration, `{"name":"x"}`, pluginsv1alpha1.DeletionPolicyDelete)

			Consistently(func(g Gomega) {
				live := liveRequest(g, request)
				g.Expect(live.Finalizers).To(BeEmpty())
				g.Expect(live.Status.Conditions).To(BeEmpty())
			}, 2*time.Second, interval).Should(Succeed())
		},
		Entry("another registration", "s3-backup-bucket", "s3-backup-bucket"),
		Entry("label and spec disagree", pluginstartup.RegistrationExistingObjectStorage, "s3-backup-bucket"),
	)

	It("refuses to start without the registration of a handler", func() {
		mgr, err := ctrl.NewManager(env.Cfg, ctrl.Options{Metrics: metricsserver.Options{BindAddress: "0"}})
		Expect(err).NotTo(HaveOccurred())

		err = (&Reconciler{
			Client:    mgr.GetClient(),
			APIReader: mgr.GetAPIReader(),
			Scheme:    mgr.GetScheme(),
		}).SetupWithManager(mgr)

		Expect(err).To(MatchError(ContainSubstring(pluginstartup.RegistrationExistingObjectStorage)))
	})

	It("refuses to start with a parameter schema that it cannot read", func() {
		mgr, err := ctrl.NewManager(env.Cfg, ctrl.Options{Metrics: metricsserver.Options{BindAddress: "0"}})
		Expect(err).NotTo(HaveOccurred())
		registrations := pluginstartup.Registrations("0.3.0", testServiceAccount)
		for i := range registrations {
			if registrations[i].Name == pluginstartup.RegistrationExistingSecondaryStorage {
				registrations[i].Spec.Parameters = &apiextensionsv1.JSON{Raw: []byte(`{"type":`)}
			}
		}

		err = (&Reconciler{
			Client:        mgr.GetClient(),
			APIReader:     mgr.GetAPIReader(),
			Scheme:        mgr.GetScheme(),
			Registrations: registrations,
		}).SetupWithManager(mgr)

		Expect(err).To(MatchError(ContainSubstring(pluginstartup.RegistrationExistingSecondaryStorage)))
	})
})
