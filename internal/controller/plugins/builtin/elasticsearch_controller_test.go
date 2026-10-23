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

	corev1 "k8s.io/api/core/v1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	utilrand "k8s.io/apimachinery/pkg/util/rand"
	"sigs.k8s.io/controller-runtime/pkg/client"

	fleetv1alpha1 "github.com/konsole-is/camunda-operator/api/fleet/v1alpha1"
	pluginsv1alpha1 "github.com/konsole-is/camunda-operator/api/plugins/v1alpha1"
	v1 "github.com/konsole-is/camunda-operator/api/v1"
	"github.com/konsole-is/camunda-operator/internal/controller/plugins/builtin/adapter"
	"github.com/konsole-is/camunda-operator/internal/pluginstartup"
)

var _ = Describe("Built-in elasticsearch requests", func() {
	It("goes to OutputsReady when the contract exists, then to Ready with the cluster", func() {
		claim := "es-" + utilrand.String(6)
		request := newElasticsearchRequest(
			claim,
			`{"preset":"small","releaseRef":"camunda","overrides":{`+
				`"resources":{"limits":{"memory":"4Gi"}},`+
				`"scheduling":{"tolerations":[{"key":"pool","value":"es"}]}}}`,
			pluginsv1alpha1.DeletionPolicyDelete,
			func(r *pluginsv1alpha1.PluginRequest) {
				r.Spec.Inputs = map[string]string{
					pluginstartup.OutputServiceAccountAnnotations: `{"eks.amazonaws.com/role-arn":"arn:aws:iam::1:role/es"}`,
					pluginstartup.OutputObjectStorageConfig:       "orders-backup",
					pluginstartup.OutputStorageClassName:          "encrypted",
				}
			},
		)
		name := claim + "-es"

		Eventually(func(g Gomega) {
			cluster := liveCluster(g, name)
			g.Expect(metav1.IsControlledBy(cluster, liveRequest(g, request))).To(BeTrue())
			g.Expect(cluster.Labels).
				To(HaveKeyWithValue(pluginsv1alpha1.LabelRegistration, pluginstartup.RegistrationElasticsearch))
			g.Expect(cluster.Labels).To(HaveKeyWithValue(pluginsv1alpha1.LabelRequest, request.Name))
			g.Expect(cluster.Spec.SecondaryStorageConfig).To(Equal(name))
			g.Expect(cluster.Spec.SnapshotStorageRef).To(Equal("orders-backup"))
			g.Expect(cluster.Spec.StorageClassName).To(HaveValue(Equal("encrypted")))
			g.Expect(cluster.Spec.ServiceAccount).NotTo(BeNil())
			g.Expect(cluster.Spec.ServiceAccount.Annotations).
				To(HaveKeyWithValue("eks.amazonaws.com/role-arn", "arn:aws:iam::1:role/es"))
			g.Expect(cluster.Spec.Resources).NotTo(BeNil())
			g.Expect(cluster.Spec.Resources.Limits.Memory().String()).To(Equal("4Gi"))
			g.Expect(cluster.Spec.Scheduling).NotTo(BeNil())
			g.Expect(cluster.Spec.Scheduling.Tolerations).
				To(ConsistOf(corev1.Toleration{Key: "pool", Value: "es"}))
		}, timeout, interval).Should(Succeed())
		expectCondition(request, pluginsv1alpha1.ConditionOutputsReady, metav1.ConditionFalse, "Blocked")

		createContract(&v1.SecondaryStorageConfig{
			ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: "default"},
			Spec: v1.SecondaryStorageConfigSpec{
				Type:  v1.SecondaryStorageTypeRDBMS,
				RDBMS: &v1.RDBMSStorage{DatabaseConfigRef: "my-camunda-db"},
			},
		})

		live := expectCondition(request, pluginsv1alpha1.ConditionOutputsReady, metav1.ConditionTrue, v1.ReasonHealthy)
		Expect(meta.IsStatusConditionTrue(live.Status.Conditions, v1.ConditionReady)).To(BeFalse())
		Expect(live.Status.Outputs).To(Equal(map[string]string{pluginstartup.OutputSecondaryStorageConfig: name}))

		setClusterReady(name)

		live = expectCondition(request, v1.ConditionReady, metav1.ConditionTrue, v1.ReasonHealthy)
		Expect(live.Status.Resources).To(Equal([]pluginsv1alpha1.ResourceRef{{
			Kind: adapter.KindElasticsearchCluster, Name: name, OnOrphan: pluginsv1alpha1.OnOrphanKept,
		}}))
		Expect(
			live.Status.PodSelector.MatchLabels,
		).To(Equal(map[string]string{"camunda.io/elasticsearch-cluster": name}))
	})

	It("keeps a larger storage size and warns about the smaller one", func() {
		claim := "es-" + utilrand.String(6)
		request := newElasticsearchRequest(
			claim,
			`{"preset":"small","releaseRef":"camunda","overrides":{"storageSize":"50Gi"}}`,
			pluginsv1alpha1.DeletionPolicyDelete,
		)
		name := claim + "-es"
		Eventually(func(g Gomega) {
			g.Expect(storageSizeOf(liveCluster(g, name))).To(Equal("50Gi"))
		}, timeout, interval).Should(Succeed())

		Eventually(func() error {
			var fresh pluginsv1alpha1.PluginRequest
			if err := k8sClient.Get(ctx, client.ObjectKeyFromObject(request), &fresh); err != nil {
				return err
			}
			fresh.Spec.Parameters = &apiextensionsv1.JSON{
				Raw: []byte(`{"preset":"small","releaseRef":"camunda","overrides":{"storageSize":"20Gi"}}`),
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
				Value:    "Storage size 20Gi is smaller than 50Gi, kept 50Gi",
				Severity: pluginsv1alpha1.SeverityWarning,
			}))
		}, timeout, interval).Should(Succeed())
		Consistently(func(g Gomega) {
			g.Expect(storageSizeOf(liveCluster(g, name))).To(Equal("50Gi"))
		}, 2*time.Second, interval).Should(Succeed())
	})

	It("drops a removed storage size override without a warning and reports the size of the volumes", func() {
		claim := "es-" + utilrand.String(6)
		request := newElasticsearchRequest(
			claim,
			`{"preset":"small","releaseRef":"camunda","overrides":{"storageSize":"50Gi"}}`,
			pluginsv1alpha1.DeletionPolicyDelete,
		)
		name := claim + "-es"
		disk := pluginsv1alpha1.SummaryItem{
			Label:    "Disk",
			Value:    "50Gi per node",
			Severity: pluginsv1alpha1.SeverityInfo,
		}
		Eventually(func(g Gomega) {
			g.Expect(storageSizeOf(liveCluster(g, name))).To(Equal("50Gi"))
		}, timeout, interval).Should(Succeed())

		setClusterVolumes(name, "50Gi")
		// Only a reconcile that read the volumes reports the Disk item.
		Eventually(func(g Gomega) {
			g.Expect(liveRequest(g, request).Status.Summary).To(ContainElement(disk))
		}, timeout, interval).Should(Succeed())

		Eventually(func() error {
			var fresh pluginsv1alpha1.PluginRequest
			if err := k8sClient.Get(ctx, client.ObjectKeyFromObject(request), &fresh); err != nil {
				return err
			}
			fresh.Spec.Parameters = &apiextensionsv1.JSON{Raw: []byte(`{"preset":"small","releaseRef":"camunda"}`)}
			return k8sClient.Update(ctx, &fresh)
		}, timeout, interval).Should(Succeed())

		Eventually(func(g Gomega) {
			live := liveRequest(g, request)
			ready := meta.FindStatusCondition(live.Status.Conditions, v1.ConditionReady)
			g.Expect(ready).NotTo(BeNil())
			g.Expect(ready.ObservedGeneration).To(Equal(live.Generation))
			g.Expect(storageSizeOf(liveCluster(g, name))).To(BeEmpty())
			g.Expect(live.Status.Summary).To(ContainElement(disk))
			g.Expect(live.Status.Summary).NotTo(ContainElement(HaveField("Label", "Storage size")))
		}, timeout, interval).Should(Succeed())
	})

	It("deletes the cluster with the Delete policy", func() {
		claim := "es-" + utilrand.String(6)
		request := newElasticsearchRequest(
			claim,
			`{"preset":"small","releaseRef":"camunda"}`,
			pluginsv1alpha1.DeletionPolicyDelete,
		)
		cluster := &v1.ElasticsearchCluster{ObjectMeta: metav1.ObjectMeta{Name: claim + "-es", Namespace: "default"}}
		Eventually(func(g Gomega) { liveCluster(g, cluster.Name) }, timeout, interval).Should(Succeed())

		Expect(k8sClient.Delete(ctx, request)).To(Succeed())

		expectGone(request)
		expectGone(cluster)
	})

	It("keeps the cluster without the owner reference with the Orphan policy", func() {
		claim := "es-" + utilrand.String(6)
		request := newElasticsearchRequest(
			claim,
			`{"preset":"small","releaseRef":"camunda"}`,
			pluginsv1alpha1.DeletionPolicyOrphan,
		)
		name := claim + "-es"
		DeferCleanup(func() {
			_ = k8sClient.Delete(
				ctx,
				&v1.ElasticsearchCluster{ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: "default"}},
			)
		})
		var owner *pluginsv1alpha1.PluginRequest
		Eventually(func(g Gomega) {
			owner = liveRequest(g, request)
			g.Expect(metav1.IsControlledBy(liveCluster(g, name), owner)).To(BeTrue())
		}, timeout, interval).Should(Succeed())

		Expect(k8sClient.Delete(ctx, request)).To(Succeed())

		expectGone(request)
		Eventually(func(g Gomega) {
			cluster := liveCluster(g, name)
			g.Expect(metav1.IsControlledBy(cluster, owner)).To(BeFalse())
			g.Expect(cluster.Labels).
				To(HaveKeyWithValue(pluginsv1alpha1.LabelRegistration, pluginstartup.RegistrationElasticsearch))
			g.Expect(cluster.Labels).To(HaveKeyWithValue(pluginsv1alpha1.LabelRequest, request.Name))
			g.Expect(cluster.Spec.SecondaryStorageConfig).To(Equal(name))
		}, timeout, interval).Should(Succeed())
	})

	It("finishes the deletion when a cluster of that name has another controller", func() {
		claim := "es-" + utilrand.String(6)
		name := claim + "-es"
		holder := &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Name: "holder-" + claim, Namespace: "default"}}
		createContract(holder)
		foreign := &v1.ElasticsearchCluster{
			ObjectMeta: metav1.ObjectMeta{
				Name:      name,
				Namespace: "default",
				OwnerReferences: []metav1.OwnerReference{{
					APIVersion: "v1", Kind: "ConfigMap", Name: holder.Name, UID: holder.UID, Controller: new(true),
				}},
			},
			Spec: v1.ElasticsearchClusterSpec{SecondaryStorageConfig: name},
		}
		createContract(foreign)
		request := newElasticsearchRequest(
			claim,
			`{"preset":"small","releaseRef":"camunda"}`,
			pluginsv1alpha1.DeletionPolicyDelete,
		)
		Eventually(func(g Gomega) {
			live := liveRequest(g, request)
			g.Expect(live.Finalizers).To(ContainElement(pluginsv1alpha1.FinalizerCleanup))
			g.Expect(meta.FindStatusCondition(live.Status.Conditions, v1.ConditionReady)).NotTo(BeNil())
		}, timeout, interval).Should(Succeed())

		Expect(k8sClient.Delete(ctx, request)).To(Succeed())

		expectGone(request)
		cluster := &v1.ElasticsearchCluster{}
		Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(foreign), cluster)).To(Succeed())
		Expect(metav1.IsControlledBy(cluster, holder)).To(BeTrue())
	})
})

// newElasticsearchRequest creates an elasticsearch request of claim with
// params and policy. Each edit changes the request before it is created.
func newElasticsearchRequest(
	claim, params string,
	policy pluginsv1alpha1.DeletionPolicy,
	edits ...func(*pluginsv1alpha1.PluginRequest),
) *pluginsv1alpha1.PluginRequest {
	GinkgoHelper()
	claimEdit := func(r *pluginsv1alpha1.PluginRequest) {
		r.Spec.ClaimRef.Name = claim
		r.Labels[fleetv1alpha1.LabelClaim] = claim
	}
	return newRequest(pluginstartup.RegistrationElasticsearch, params, policy, append(edits, claimEdit)...)
}

// liveCluster reads the ElasticsearchCluster name in default.
func liveCluster(g Gomega, name string) *v1.ElasticsearchCluster {
	var cluster v1.ElasticsearchCluster
	g.Expect(k8sClient.Get(ctx, client.ObjectKey{Name: name, Namespace: "default"}, &cluster)).To(Succeed())
	return &cluster
}

// setClusterReady writes Ready=True on the ElasticsearchCluster name at its
// current generation. Its controller does not run in this suite.
func setClusterReady(name string) {
	GinkgoHelper()
	Eventually(func() error {
		var cluster v1.ElasticsearchCluster
		if err := k8sClient.Get(ctx, client.ObjectKey{Name: name, Namespace: "default"}, &cluster); err != nil {
			return err
		}
		meta.SetStatusCondition(&cluster.Status.Conditions, metav1.Condition{
			Type:               v1.ConditionReady,
			Status:             metav1.ConditionTrue,
			Reason:             v1.ReasonHealthy,
			Message:            "Ready",
			ObservedGeneration: cluster.Generation,
		})
		cluster.Status.ObservedGeneration = cluster.Generation
		return k8sClient.Status().Update(ctx, &cluster)
	}, timeout, interval).Should(Succeed())
}

// setClusterVolumes writes one data volume of capacity in the status of the
// ElasticsearchCluster name. Its controller does not run in this suite.
func setClusterVolumes(name, capacity string) {
	GinkgoHelper()
	Eventually(func() error {
		var cluster v1.ElasticsearchCluster
		if err := k8sClient.Get(ctx, client.ObjectKey{Name: name, Namespace: "default"}, &cluster); err != nil {
			return err
		}
		cluster.Status.Volumes = []v1.VolumeStatus{
			{Name: "data-" + name + "-0", Capacity: resource.MustParse(capacity)},
		}
		return k8sClient.Status().Update(ctx, &cluster)
	}, timeout, interval).Should(Succeed())
}

// storageSizeOf returns spec.storageSize of cluster, or "" when it is unset.
func storageSizeOf(cluster *v1.ElasticsearchCluster) string {
	if cluster.Spec.StorageSize == nil {
		return ""
	}
	return cluster.Spec.StorageSize.String()
}
