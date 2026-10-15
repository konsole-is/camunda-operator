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

package environment

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	utilrand "k8s.io/apimachinery/pkg/util/rand"
	"sigs.k8s.io/controller-runtime/pkg/client"

	fleetv1alpha1 "github.com/konsole-is/camunda-operator/api/fleet/v1alpha1"
	pluginsv1alpha1 "github.com/konsole-is/camunda-operator/api/plugins/v1alpha1"
	v1 "github.com/konsole-is/camunda-operator/api/v1"
)

// uniqueName returns prefix with a random suffix. Every object of these
// specs is cluster-scoped, so each spec needs names of its own.
func uniqueName(prefix string) string {
	return prefix + "-" + utilrand.String(8)
}

// create creates obj and registers its deletion.
func create(obj client.Object) client.Object {
	GinkgoHelper()
	Expect(k8sClient.Create(ctx, obj)).To(Succeed())
	DeferCleanup(func() { _ = k8sClient.Delete(ctx, obj) })
	return obj
}

// createEnvironment creates a uniquely named Environment with spec.
func createEnvironment(spec fleetv1alpha1.EnvironmentSpec) *fleetv1alpha1.Environment {
	GinkgoHelper()
	return create(&fleetv1alpha1.Environment{
		ObjectMeta: metav1.ObjectMeta{Name: uniqueName("env")},
		Spec:       spec,
	}).(*fleetv1alpha1.Environment)
}

func newRelease(name string) client.Object {
	return &v1.CamundaRelease{
		ObjectMeta: metav1.ObjectMeta{Name: name},
		Spec:       v1.CamundaReleaseSpec{Version: "8.9.9"},
	}
}

func newClusterPreset(name string) client.Object {
	return &v1.CamundaClusterPreset{ObjectMeta: metav1.ObjectMeta{Name: name}}
}

func newElasticsearchPreset(name string) client.Object {
	return &v1.ElasticsearchClusterPreset{
		ObjectMeta: metav1.ObjectMeta{Name: name},
		Spec: v1.ElasticsearchClusterPresetSpec{
			Cluster: v1.ElasticsearchClusterSpec{Replicas: new(int32(3))},
		},
	}
}

func newDatabaseServerPreset(name string) client.Object {
	return &v1.DatabaseServerPreset{
		ObjectMeta: metav1.ObjectMeta{Name: name},
		Spec: v1.DatabaseServerPresetSpec{
			Server: v1.DatabaseServerSpec{Instances: new(int32(2)), StorageSize: new(resource.MustParse("20Gi"))},
		},
	}
}

func newPlatformConfig(name string) client.Object {
	return &v1.CamundaPlatformConfig{
		ObjectMeta: metav1.ObjectMeta{Name: name},
		Spec:       v1.CamundaPlatformConfigSpec{Auth: &v1.PlatformAuthSpec{Method: v1.AuthenticationMethodBasic}},
	}
}

func newSection(name string) client.Object {
	return &pluginsv1alpha1.PluginSection{
		ObjectMeta: metav1.ObjectMeta{Name: name},
		Spec: pluginsv1alpha1.PluginSectionSpec{
			Label:            "Backup storage",
			Order:            40,
			Selection:        pluginsv1alpha1.SelectionOne,
			EmptyOptionLabel: "No backups",
			Slot:             pluginsv1alpha1.SlotBackupStorage,
			AppliesTo:        []pluginsv1alpha1.ClaimKind{pluginsv1alpha1.ClaimKindCamundaClusterClaim},
			Outputs:          []pluginsv1alpha1.OutputDecl{{Key: "objectStorageConfig"}},
		},
	}
}

func newRegistration(name string) client.Object {
	return &pluginsv1alpha1.PluginRegistration{
		ObjectMeta: metav1.ObjectMeta{Name: name},
		Spec: pluginsv1alpha1.PluginRegistrationSpec{
			Sections: []string{"backup-storage"},
			Label:    "New S3 bucket",
			Mode:     pluginsv1alpha1.ModeOptional,
			ServiceAccount: pluginsv1alpha1.ServiceAccountRef{
				Namespace: "camunda-plugins",
				Name:      "s3-backup-bucket-controller",
			},
			Outputs:          []pluginsv1alpha1.OutputDecl{{Key: "objectStorageConfig"}},
			DeletionPolicies: []pluginsv1alpha1.DeletionPolicy{pluginsv1alpha1.DeletionPolicyDelete},
		},
	}
}

// expectReady polls until the Ready condition of env matches the given status
// and reason, its message matches the matcher, and the observed generation is
// current.
func expectReady(
	env *fleetv1alpha1.Environment,
	status metav1.ConditionStatus,
	reason string,
	message OmegaMatcher,
) {
	GinkgoHelper()
	Eventually(func(g Gomega) {
		var latest fleetv1alpha1.Environment
		g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(env), &latest)).To(Succeed())
		ready := meta.FindStatusCondition(latest.Status.Conditions, v1.ConditionReady)
		g.Expect(ready).NotTo(BeNil())
		g.Expect(ready.Status).To(Equal(status))
		g.Expect(ready.Reason).To(Equal(reason))
		g.Expect(ready.Message).To(message)
		g.Expect(ready.ObservedGeneration).To(Equal(latest.Generation))
		g.Expect(latest.Status.ObservedGeneration).To(Equal(latest.Generation))
	}, timeout, interval).Should(Succeed())
}

// generationOf reads the current generation of env.
func generationOf(env *fleetv1alpha1.Environment) int64 {
	GinkgoHelper()
	var latest fleetv1alpha1.Environment
	Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(env), &latest)).To(Succeed())
	return latest.Generation
}

// createNamespace creates a uniquely named namespace with labels. envtest
// runs no namespace controller, so a deleted namespace stays Terminating.
// The specs therefore never reuse an environment name.
func createNamespace(labels map[string]string) *corev1.Namespace {
	GinkgoHelper()
	return create(&corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{Name: uniqueName("ns"), Labels: labels},
	}).(*corev1.Namespace)
}

// setEnvironmentLabel sets the environment label of ns to name, or removes it
// when name is empty.
func setEnvironmentLabel(ns *corev1.Namespace, name string) {
	GinkgoHelper()
	patch := client.MergeFrom(ns.DeepCopy())
	if name == "" {
		delete(ns.Labels, fleetv1alpha1.LabelEnvironment)
	} else {
		if ns.Labels == nil {
			ns.Labels = map[string]string{}
		}
		ns.Labels[fleetv1alpha1.LabelEnvironment] = name
	}
	Expect(k8sClient.Patch(ctx, ns, patch)).To(Succeed())
}

// expectNamespaces polls until status.namespaces of env is count.
func expectNamespaces(env *fleetv1alpha1.Environment, count int32) {
	GinkgoHelper()
	Eventually(func(g Gomega) {
		var latest fleetv1alpha1.Environment
		g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(env), &latest)).To(Succeed())
		g.Expect(latest.Status.Namespaces).To(HaveValue(Equal(count)))
	}, timeout, interval).Should(Succeed())
}
