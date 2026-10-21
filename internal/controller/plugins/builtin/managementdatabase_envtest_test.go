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
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	pluginsv1alpha1 "github.com/konsole-is/camunda-operator/api/plugins/v1alpha1"
	v1 "github.com/konsole-is/camunda-operator/api/v1"
	"github.com/konsole-is/camunda-operator/internal/pluginstartup"
)

// The suite runs neither the DatabaseServer nor the Database controller, so a
// spec writes each DatabaseConfig itself, and no finalizer holds a Database
// unless the spec adds one.
var _ = Describe("Built-in management databases", func() {
	It("publishes the outputs of the listed components only", func() {
		request, plane := newPlaneRequest(`["keycloak","identity"]`, pluginsv1alpha1.DeletionPolicyDelete)

		Eventually(func(g Gomega) {
			live := liveRequest(g, request)
			g.Expect(ownedObject(g, &v1.DatabaseServer{}, plane+"-db", live)).To(BeTrue())
			g.Expect(ownedObject(g, &v1.Database{}, plane+"-keycloak", live)).To(BeTrue())
			g.Expect(ownedObject(g, &v1.Database{}, plane+"-identity", live)).To(BeTrue())
		}, timeout, interval).Should(Succeed())
		expectAbsent(&v1.Database{}, plane+"-web-modeler")
		expectCondition(request, pluginsv1alpha1.ConditionOutputsReady, metav1.ConditionFalse, "Blocked")

		createContract(newDatabaseConfig(plane + "-keycloak"))
		createContract(newDatabaseConfig(plane + "-identity"))

		live := expectCondition(request, pluginsv1alpha1.ConditionOutputsReady, metav1.ConditionTrue, v1.ReasonHealthy)
		Expect(live.Status.Outputs).To(Equal(map[string]string{
			pluginstartup.OutputKeycloakDatabaseConfig: plane + "-keycloak",
			pluginstartup.OutputIdentityDatabaseConfig: plane + "-identity",
		}))
	})

	It("removes the Database and the output of a component that leaves databases", func() {
		request, plane := newPlaneRequest(`["keycloak","identity","webModeler"]`, pluginsv1alpha1.DeletionPolicyDelete)
		for _, c := range []string{"-keycloak", "-identity", "-web-modeler"} {
			createContract(newDatabaseConfig(plane + c))
		}
		Eventually(func(g Gomega) {
			g.Expect(liveRequest(g, request).Status.Outputs).To(HaveLen(3))
		}, timeout, interval).Should(Succeed())

		Eventually(func() error {
			var live pluginsv1alpha1.PluginRequest
			if err := k8sClient.Get(ctx, client.ObjectKeyFromObject(request), &live); err != nil {
				return err
			}
			live.Spec.Parameters = &apiextensionsv1.JSON{
				Raw: []byte(`{"preset":"small","databases":["keycloak","identity"]}`),
			}
			return k8sClient.Update(ctx, &live)
		}, timeout, interval).Should(Succeed())

		expectGone(&v1.Database{ObjectMeta: metav1.ObjectMeta{Name: plane + "-web-modeler", Namespace: "default"}})
		Eventually(func(g Gomega) {
			live := liveRequest(g, request)
			g.Expect(live.Status.Outputs).NotTo(HaveKey(pluginstartup.OutputWebModelerDatabaseConfig))
			g.Expect(live.Status.Outputs).To(HaveLen(2))
			g.Expect(live.Status.Summary).To(ContainElement(pluginsv1alpha1.SummaryItem{
				Label:    kindDatabase,
				Value:    plane + "-web-modeler removed, the logical database web_modeler stays on " + plane + "-db",
				Severity: pluginsv1alpha1.SeverityWarning,
			}))
		}, timeout, interval).Should(Succeed())
	})

	It("deletes the DatabaseServer only after every Database is gone", func() {
		request, plane := newPlaneRequest(`["identity"]`, pluginsv1alpha1.DeletionPolicyDelete)
		server := &v1.DatabaseServer{ObjectMeta: metav1.ObjectMeta{Name: plane + "-db", Namespace: "default"}}
		db := &v1.Database{ObjectMeta: metav1.ObjectMeta{Name: plane + "-identity", Namespace: "default"}}
		Eventually(func(g Gomega) {
			g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(server), &v1.DatabaseServer{})).To(Succeed())
			g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(db), &v1.Database{})).To(Succeed())
		}, timeout, interval).Should(Succeed())

		setHold(db, true)
		DeferCleanup(func() { setHold(db, false) })
		Expect(k8sClient.Delete(ctx, request)).To(Succeed())

		Eventually(func(g Gomega) {
			var live v1.Database
			g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(db), &live)).To(Succeed())
			g.Expect(live.DeletionTimestamp.IsZero()).To(BeFalse())
		}, timeout, interval).Should(Succeed())
		Consistently(func(g Gomega) {
			var live v1.DatabaseServer
			g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(server), &live)).To(Succeed())
			g.Expect(live.DeletionTimestamp.IsZero()).To(BeTrue())
			g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(request), &pluginsv1alpha1.PluginRequest{})).
				To(Succeed())
		}, 2*time.Second, interval).Should(Succeed())

		setHold(db, false)

		expectGone(db)
		expectGone(server)
		expectGone(request)
	})

	It("keeps every resource without owner references with Orphan", func() {
		request, plane := newPlaneRequest(`["keycloak","identity","webModeler"]`, pluginsv1alpha1.DeletionPolicyOrphan)
		objects := []client.Object{
			&v1.DatabaseServer{ObjectMeta: metav1.ObjectMeta{Name: plane + "-db", Namespace: "default"}},
			&v1.Database{ObjectMeta: metav1.ObjectMeta{Name: plane + "-keycloak", Namespace: "default"}},
			&v1.Database{ObjectMeta: metav1.ObjectMeta{Name: plane + "-identity", Namespace: "default"}},
			&v1.Database{ObjectMeta: metav1.ObjectMeta{Name: plane + "-web-modeler", Namespace: "default"}},
		}
		var uid string
		Eventually(func(g Gomega) {
			live := liveRequest(g, request)
			uid = string(live.UID)
			for _, obj := range objects {
				g.Expect(ownedObject(g, obj, obj.GetName(), live)).To(BeTrue(), obj.GetName())
			}
		}, timeout, interval).Should(Succeed())
		for _, obj := range objects {
			DeferCleanup(func() { _ = k8sClient.Delete(ctx, obj) })
		}

		Expect(k8sClient.Delete(ctx, request)).To(Succeed())
		expectGone(request)

		for _, obj := range objects {
			Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(obj), obj)).To(Succeed())
			for _, ref := range obj.GetOwnerReferences() {
				Expect(string(ref.UID)).NotTo(Equal(uid), obj.GetName())
			}
			Expect(obj.GetLabels()).To(HaveKeyWithValue(
				pluginsv1alpha1.LabelRegistration, pluginstartup.RegistrationManagementDatabaseServer,
			))
			Expect(obj.GetLabels()).To(HaveKeyWithValue(pluginsv1alpha1.LabelRequest, request.Name))
		}
	})
})

// ownedObject reads obj of name in default into obj and reports whether
// request controls it.
func ownedObject(g Gomega, obj client.Object, name string, request *pluginsv1alpha1.PluginRequest) bool {
	g.Expect(k8sClient.Get(ctx, client.ObjectKey{Namespace: "default", Name: name}, obj)).To(Succeed())
	return metav1.IsControlledBy(obj, request)
}

// expectAbsent checks that obj of name does not exist in default.
func expectAbsent(obj client.Object, name string) {
	GinkgoHelper()
	err := k8sClient.Get(ctx, client.ObjectKey{Namespace: "default", Name: name}, obj)
	Expect(client.IgnoreNotFound(err)).To(Succeed())
	Expect(err).To(HaveOccurred())
}

// setHold adds or removes the test finalizer on the Database db.
func setHold(db *v1.Database, on bool) {
	GinkgoHelper()
	Eventually(func() error {
		var live v1.Database
		if err := k8sClient.Get(ctx, client.ObjectKeyFromObject(db), &live); err != nil {
			return client.IgnoreNotFound(err)
		}
		var changed bool
		if on {
			changed = controllerutil.AddFinalizer(&live, testHold)
		} else {
			changed = controllerutil.RemoveFinalizer(&live, testHold)
		}
		if !changed {
			return nil
		}
		return k8sClient.Update(ctx, &live)
	}, timeout, interval).Should(Succeed())
}
