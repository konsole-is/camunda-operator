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
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	utilrand "k8s.io/apimachinery/pkg/util/rand"
	"sigs.k8s.io/controller-runtime/pkg/client"

	fleetv1alpha1 "github.com/konsole-is/camunda-operator/api/fleet/v1alpha1"
	pluginsv1alpha1 "github.com/konsole-is/camunda-operator/api/plugins/v1alpha1"
	v1 "github.com/konsole-is/camunda-operator/api/v1"
	"github.com/konsole-is/camunda-operator/internal/pluginstartup"
	servercomponents "github.com/konsole-is/camunda-operator/pkg/components/databaseserver"
	"github.com/konsole-is/camunda-operator/pkg/conditions"
)

// The suite runs neither the DatabaseServer nor the Database controller, so a
// spec writes the SecondaryStorageConfig and the Ready conditions itself.
var _ = Describe("Built-in PostgreSQL requests", func() {
	It("goes to OutputsReady with the contract, then to Ready with the server and the database", func() {
		preset := createDatabaseServerPreset(&v1.DatabaseServerArchiveSpec{
			ObjectStorageRef: "fleet-backups", RetentionPeriodDays: 7,
		})
		release := &v1.CamundaRelease{
			ObjectMeta: metav1.ObjectMeta{Name: "release-" + utilrand.String(6)},
			Spec: v1.CamundaReleaseSpec{
				Version:        "8.9.0",
				DatabaseServer: &v1.ReleaseDatabaseServerSpec{Version: "17"},
			},
		}
		createContract(release)
		claim := "pg-" + utilrand.String(6)
		request := newPostgresRequest(
			claim,
			`{"preset":"`+preset.Name+`","releaseRef":"`+release.Name+`","overrides":{"instances":2}}`,
			pluginsv1alpha1.DeletionPolicyDelete,
			func(r *pluginsv1alpha1.PluginRequest) {
				r.Spec.Inputs = map[string]string{
					pluginstartup.OutputServiceAccountAnnotations: `{"eks.amazonaws.com/role-arn":"arn:aws:iam::1:role/pg"}`,
					pluginstartup.OutputObjectStorageConfig:       "orders-backup",
					pluginstartup.OutputStorageClassName:          "encrypted",
				}
			},
		)
		name := claim + "-pg"

		var server v1.DatabaseServer
		Eventually(func(g Gomega) {
			live := liveRequest(g, request)
			g.Expect(ownedObject(g, &server, name, live)).To(BeTrue())
			g.Expect(ownedObject(g, &v1.Database{}, claim, live)).To(BeTrue())
			g.Expect(server.Spec.Archive).To(Equal(&v1.DatabaseServerArchiveSpec{
				ObjectStorageRef: "orders-backup", RetentionPeriodDays: 7, BaseBackupSchedule: "0 0 2 * * *",
			}))
		}, timeout, interval).Should(Succeed())
		Expect(
			server.Labels,
		).To(HaveKeyWithValue(pluginsv1alpha1.LabelRegistration, pluginstartup.RegistrationDatabaseServer))
		Expect(server.Labels).To(HaveKeyWithValue(pluginsv1alpha1.LabelRequest, request.Name))
		Expect(server.Spec.ServiceAccount.Annotations).
			To(HaveKeyWithValue("eks.amazonaws.com/role-arn", "arn:aws:iam::1:role/pg"))
		Expect(servercomponents.ValidateMerged(
			servercomponents.MergeSpec(server.Spec, &preset.Spec, &release.Spec),
		)).To(Succeed())
		expectCondition(request, pluginsv1alpha1.ConditionOutputsReady, metav1.ConditionFalse, "Blocked")

		createContract(&v1.SecondaryStorageConfig{
			ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: "default"},
			Spec: v1.SecondaryStorageConfigSpec{
				Type:  v1.SecondaryStorageTypeRDBMS,
				RDBMS: &v1.RDBMSStorage{DatabaseConfigRef: claim},
			},
		})

		live := expectCondition(request, pluginsv1alpha1.ConditionOutputsReady, metav1.ConditionTrue, v1.ReasonHealthy)
		Expect(meta.IsStatusConditionTrue(live.Status.Conditions, v1.ConditionReady)).To(BeFalse())
		Expect(live.Status.Outputs).To(Equal(map[string]string{pluginstartup.OutputSecondaryStorageConfig: name}))

		setBackingReady(&v1.DatabaseServer{}, name)
		setBackingReady(&v1.Database{}, claim)

		live = expectCondition(request, v1.ConditionReady, metav1.ConditionTrue, v1.ReasonHealthy)
		Expect(live.Status.Resources).To(Equal([]pluginsv1alpha1.ResourceRef{
			{Kind: kindDatabaseServer, Name: name, OnOrphan: pluginsv1alpha1.OnOrphanKept},
			{Kind: kindDatabase, Name: claim, OnOrphan: pluginsv1alpha1.OnOrphanKept},
		}))
		Expect(live.Status.PodSelector.MatchLabels).To(Equal(map[string]string{"camunda.io/database-server": name}))
	})

	It("keeps a larger storage size and warns about the smaller one", func() {
		claim := "pg-" + utilrand.String(6)
		request := newPostgresRequest(
			claim,
			`{"preset":"small","releaseRef":"camunda","overrides":{"storageSize":"20Gi"}}`,
			pluginsv1alpha1.DeletionPolicyDelete,
		)
		name := claim + "-pg"
		Eventually(func(g Gomega) {
			g.Expect(serverStorageSize(g, name)).To(Equal("20Gi"))
		}, timeout, interval).Should(Succeed())

		Eventually(func() error {
			var fresh pluginsv1alpha1.PluginRequest
			if err := k8sClient.Get(ctx, client.ObjectKeyFromObject(request), &fresh); err != nil {
				return err
			}
			fresh.Spec.Parameters = &apiextensionsv1.JSON{
				Raw: []byte(`{"preset":"small","releaseRef":"camunda","overrides":{"storageSize":"10Gi"}}`),
			}
			return k8sClient.Update(ctx, &fresh)
		}, timeout, interval).Should(Succeed())

		Eventually(func(g Gomega) {
			live := liveRequest(g, request)
			ready := meta.FindStatusCondition(live.Status.Conditions, v1.ConditionReady)
			g.Expect(ready).NotTo(BeNil())
			g.Expect(ready.ObservedGeneration).To(Equal(live.Generation))
			g.Expect(live.Status.Summary).To(ContainElement(pluginsv1alpha1.SummaryItem{
				Label:    "Storage size",
				Value:    "Storage size 10Gi is smaller than 20Gi, kept 20Gi",
				Severity: pluginsv1alpha1.SeverityWarning,
			}))
		}, timeout, interval).Should(Succeed())
		Consistently(func(g Gomega) {
			g.Expect(serverStorageSize(g, name)).To(Equal("20Gi"))
		}, 2*time.Second, interval).Should(Succeed())
	})

	It("deletes the DatabaseServer only after the Database is gone", func() {
		claim := "pg-" + utilrand.String(6)
		request := newPostgresRequest(
			claim,
			`{"preset":"small","releaseRef":"camunda"}`,
			pluginsv1alpha1.DeletionPolicyDelete,
		)
		server := &v1.DatabaseServer{ObjectMeta: metav1.ObjectMeta{Name: claim + "-pg", Namespace: "default"}}
		db := &v1.Database{ObjectMeta: metav1.ObjectMeta{Name: claim, Namespace: "default"}}
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
		}, 3*time.Second, interval).Should(Succeed())

		setHold(db, false)

		expectGone(db)
		expectGone(server)
		expectGone(request)
	})

	It("keeps the server and the database without owner references with Orphan", func() {
		claim := "pg-" + utilrand.String(6)
		request := newPostgresRequest(
			claim,
			`{"preset":"small","releaseRef":"camunda"}`,
			pluginsv1alpha1.DeletionPolicyOrphan,
		)
		objects := []client.Object{
			&v1.DatabaseServer{ObjectMeta: metav1.ObjectMeta{Name: claim + "-pg", Namespace: "default"}},
			&v1.Database{ObjectMeta: metav1.ObjectMeta{Name: claim, Namespace: "default"}},
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
				pluginsv1alpha1.LabelRegistration, pluginstartup.RegistrationDatabaseServer,
			))
			Expect(obj.GetLabels()).To(HaveKeyWithValue(pluginsv1alpha1.LabelRequest, request.Name))
		}
	})
})

// newPostgresRequest creates a database-server request of claim with params
// and policy. Each edit changes the request before it is created.
func newPostgresRequest(
	claim, params string,
	policy pluginsv1alpha1.DeletionPolicy,
	edits ...func(*pluginsv1alpha1.PluginRequest),
) *pluginsv1alpha1.PluginRequest {
	GinkgoHelper()
	claimEdit := func(r *pluginsv1alpha1.PluginRequest) {
		r.Spec.ClaimRef.Name = claim
		r.Labels[fleetv1alpha1.LabelClaim] = claim
	}
	return newRequest(pluginstartup.RegistrationDatabaseServer, params, policy, append(edits, claimEdit)...)
}

// createDatabaseServerPreset creates a DatabaseServerPreset with a unique
// name, a storage size, and archive.
func createDatabaseServerPreset(archive *v1.DatabaseServerArchiveSpec) *v1.DatabaseServerPreset {
	GinkgoHelper()
	preset := &v1.DatabaseServerPreset{
		ObjectMeta: metav1.ObjectMeta{Name: "preset-" + utilrand.String(6)},
		Spec: v1.DatabaseServerPresetSpec{Server: v1.DatabaseServerSpec{
			StorageSize: new(resource.MustParse("10Gi")),
			Archive:     archive,
		}},
	}
	createContract(preset)
	return preset
}

// setBackingReady writes Ready=True on obj of name in default at its current
// generation. Its controller does not run in this suite.
func setBackingReady(obj conditions.Owner, name string) {
	GinkgoHelper()
	Eventually(func() error {
		if err := k8sClient.Get(ctx, client.ObjectKey{Name: name, Namespace: "default"}, obj); err != nil {
			return err
		}
		meta.SetStatusCondition(obj.GetStatusConditions(), metav1.Condition{
			Type:               v1.ConditionReady,
			Status:             metav1.ConditionTrue,
			Reason:             v1.ReasonHealthy,
			Message:            "Ready",
			ObservedGeneration: obj.GetGeneration(),
		})
		obj.SetObservedGeneration(obj.GetGeneration())
		return k8sClient.Status().Update(ctx, obj)
	}, timeout, interval).Should(Succeed())
}

// serverStorageSize returns spec.storageSize of the DatabaseServer name in
// default, or "" when it is unset.
func serverStorageSize(g Gomega, name string) string {
	var server v1.DatabaseServer
	g.Expect(k8sClient.Get(ctx, client.ObjectKey{Name: name, Namespace: "default"}, &server)).To(Succeed())
	if server.Spec.StorageSize == nil {
		return ""
	}
	return server.Spec.StorageSize.String()
}
