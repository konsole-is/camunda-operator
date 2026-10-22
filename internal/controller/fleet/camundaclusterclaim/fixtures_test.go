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

package camundaclusterclaim

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	utilrand "k8s.io/apimachinery/pkg/util/rand"
	"sigs.k8s.io/controller-runtime/pkg/client"

	fleetv1alpha1 "github.com/konsole-is/camunda-operator/api/fleet/v1alpha1"
	pluginsv1alpha1 "github.com/konsole-is/camunda-operator/api/plugins/v1alpha1"
	v1 "github.com/konsole-is/camunda-operator/api/v1"
	"github.com/konsole-is/camunda-operator/internal/pluginstartup"
)

// The cluster-scoped objects that every claim of the suite names.
const (
	presetName         = "suite-preset"
	releaseName        = "suite-release"
	platformConfigName = "suite-platform"
)

// The test registrations, and the outputs that the fake plugin writes for
// them.
const (
	registrationIdentity   = "test-identity"
	registrationKey        = "test-key"
	registrationVolumes    = "test-volumes"
	registrationConnectors = "test-connectors"
	registrationAccess     = "test-access"
	// registrationVersioned declares the parameter releaseRef (CL55).
	registrationVersioned = "test-versioned"
	// registrationDeleteOnly supports only the deletion policy Delete.
	registrationDeleteOnly = "test-delete-only"

	camundaRoleARN      = `{"eks.amazonaws.com/role-arn":"arn:camunda"}`
	connectorsRoleARN   = `{"eks.amazonaws.com/role-arn":"arn:connectors"}`
	keyARN              = "arn:aws:kms:key"
	encryptedClass      = "encrypted-gp3"
	accessURL           = "https://orders.example.com"
	existingStorageName = "existing-es"
)

// createFixtures creates the core sections, the built-in and test
// registrations, a preset, a release, and a platform config, and tells the
// fake plugin what to write for each registration.
func createFixtures() {
	GinkgoHelper()

	for _, section := range pluginstartup.Sections() {
		Expect(k8sClient.Create(ctx, &section)).To(Succeed())
	}
	for _, registration := range pluginstartup.Registrations(
		"0.0.0", pluginsv1alpha1.ServiceAccountRef{Namespace: "camunda-operator", Name: "manager"},
	) {
		Expect(k8sClient.Create(ctx, &registration)).To(Succeed())
	}

	volumes := testRegistration(registrationVolumes, pluginstartup.SectionVolumeEncryption)
	volumes.Spec.Inputs = []pluginsv1alpha1.InputDecl{{
		Name: "keyRef", From: pluginstartup.SectionEncryptionKey + "/" + pluginstartup.OutputKeyRef, Required: true,
	}}
	versioned := testRegistration(registrationVersioned, pluginstartup.SectionEncryptionKey)
	versioned.Spec.Parameters = &apiextensionsv1.JSON{
		Raw: []byte(`{"type":"object","properties":{"releaseRef":{"type":"string"}}}`),
	}
	deleteOnly := testRegistration(registrationDeleteOnly, pluginstartup.SectionEncryptionKey)
	deleteOnly.Spec.DeletionPolicies = []pluginsv1alpha1.DeletionPolicy{pluginsv1alpha1.DeletionPolicyDelete}
	for _, registration := range []*pluginsv1alpha1.PluginRegistration{
		deleteOnly,
		testRegistration(registrationIdentity, pluginstartup.SectionCloudIdentity),
		testRegistration(registrationKey, pluginstartup.SectionEncryptionKey),
		volumes,
		testRegistration(registrationConnectors, pluginstartup.SectionConnectorsPermissions),
		testRegistration(registrationAccess, pluginstartup.SectionAccess),
		versioned,
	} {
		Expect(k8sClient.Create(ctx, registration)).To(Succeed())
	}

	Expect(k8sClient.Create(ctx, &v1.CamundaClusterPreset{ObjectMeta: metav1.ObjectMeta{Name: presetName}})).
		To(Succeed())
	Expect(k8sClient.Create(ctx, &v1.CamundaRelease{
		ObjectMeta: metav1.ObjectMeta{Name: releaseName},
		Spec:       v1.CamundaReleaseSpec{Version: "8.9.9"},
	})).To(Succeed())
	Expect(k8sClient.Create(ctx, &v1.CamundaPlatformConfig{
		ObjectMeta: metav1.ObjectMeta{Name: platformConfigName},
		Spec:       v1.CamundaPlatformConfigSpec{Auth: &v1.PlatformAuthSpec{Method: v1.AuthenticationMethodBasic}},
	})).To(Succeed())

	plugin.serve(registrationIdentity, map[string]string{
		pluginstartup.OutputPrincipal:                 "arn:camunda",
		pluginstartup.OutputServiceAccountAnnotations: camundaRoleARN,
	})
	plugin.serve(registrationKey, map[string]string{pluginstartup.OutputKeyRef: keyARN})
	plugin.serve(registrationDeleteOnly, map[string]string{pluginstartup.OutputKeyRef: keyARN})
	plugin.serve(registrationVolumes, map[string]string{pluginstartup.OutputStorageClassName: encryptedClass})
	plugin.serve(registrationConnectors, map[string]string{
		pluginstartup.OutputServiceAccountAnnotations: connectorsRoleARN,
	})
	plugin.serve(registrationAccess, map[string]string{pluginstartup.OutputExternalURL: accessURL})
	plugin.serve(pluginstartup.RegistrationExistingSecondaryStorage, map[string]string{
		pluginstartup.OutputSecondaryStorageConfig: existingStorageName,
	})
	plugin.serve(pluginstartup.RegistrationElasticsearch, map[string]string{
		pluginstartup.OutputSecondaryStorageConfig: "orders-es",
	})
}

// testRegistration returns a registration that serves section, declares
// the outputs of the section, and supports both deletion policies.
func testRegistration(name, section string) *pluginsv1alpha1.PluginRegistration {
	return &pluginsv1alpha1.PluginRegistration{
		ObjectMeta: metav1.ObjectMeta{Name: name},
		Spec: pluginsv1alpha1.PluginRegistrationSpec{
			Sections: []string{section},
			Label:    name,
			Mode:     pluginsv1alpha1.ModeOptional,
			ServiceAccount: pluginsv1alpha1.ServiceAccountRef{
				Namespace: "camunda-plugins",
				Name:      name,
			},
			DeletionPolicies: []pluginsv1alpha1.DeletionPolicy{
				pluginsv1alpha1.DeletionPolicyDelete,
				pluginsv1alpha1.DeletionPolicyOrphan,
			},
		},
	}
}

// uniqueName returns prefix with a random suffix.
func uniqueName(prefix string) string {
	return prefix + "-" + utilrand.String(8)
}

// create creates obj and registers its deletion.
func create(obj client.Object) {
	GinkgoHelper()
	Expect(k8sClient.Create(ctx, obj)).To(Succeed())
	DeferCleanup(func() { _ = k8sClient.Delete(ctx, obj) })
}

// newNamespace creates a uniquely named namespace with labels. envtest runs
// no namespace controller, so a deleted namespace stays Terminating, and the
// specs never reuse one.
func newNamespace(labels map[string]string) string {
	GinkgoHelper()
	ns := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: uniqueName("ns"), Labels: labels}}
	create(ns)
	return ns.Name
}

// createEnvironment creates an Environment and, after the spec, deletes it
// and waits until it is gone: an Environment refuses every claim of the
// suite in a namespace without the label.
func createEnvironment(environment *fleetv1alpha1.Environment) {
	GinkgoHelper()
	Expect(k8sClient.Create(ctx, environment)).To(Succeed())
	DeferCleanup(func() {
		Expect(client.IgnoreNotFound(k8sClient.Delete(ctx, environment))).To(Succeed())
		Eventually(func() bool {
			return apierrors.IsNotFound(k8sClient.Get(ctx, client.ObjectKeyFromObject(environment), environment))
		}, timeout, interval).Should(BeTrue())
	})
}

// allowingEnvironment returns an Environment that allows the suite preset
// and release, and each registration of sections in its section.
func allowingEnvironment(sections map[string][]string) *fleetv1alpha1.Environment {
	policies := map[string]fleetv1alpha1.SectionPolicy{}
	for section, allowed := range sections {
		policies[section] = fleetv1alpha1.SectionPolicy{Allowed: allowed}
	}

	return &fleetv1alpha1.Environment{
		ObjectMeta: metav1.ObjectMeta{Name: uniqueName("env")},
		Spec: fleetv1alpha1.EnvironmentSpec{
			PlatformConfigRef: platformConfigName,
			Presets:           fleetv1alpha1.EnvironmentPresets{CamundaCluster: []string{presetName}},
			Releases:          &fleetv1alpha1.EnvironmentReleases{Allowed: []string{releaseName}, Default: releaseName},
			Sections:          policies,
		},
	}
}

// newClaim returns a claim orders in namespace that names the suite preset,
// release, and platform config, and selects existing-secondary-storage
// before selections, so that its cluster has spec.storageRef.
func newClaim(namespace string, selections ...fleetv1alpha1.PluginSelection) *fleetv1alpha1.CamundaClusterClaim {
	return &fleetv1alpha1.CamundaClusterClaim{
		ObjectMeta: metav1.ObjectMeta{Name: "orders", Namespace: namespace},
		Spec: fleetv1alpha1.CamundaClusterClaimSpec{
			PresetRef:         presetName,
			ReleaseRef:        releaseName,
			PlatformConfigRef: platformConfigName,
			Plugins: append([]fleetv1alpha1.PluginSelection{
				selection(pluginstartup.SectionSecondaryStorage, pluginstartup.RegistrationExistingSecondaryStorage),
			}, selections...),
		},
	}
}

func selection(section, registration string) fleetv1alpha1.PluginSelection {
	return fleetv1alpha1.PluginSelection{Section: section, Registration: registration}
}

// requestOf reads the request of claim in section.
func requestOf(claim *fleetv1alpha1.CamundaClusterClaim, section string) (*pluginsv1alpha1.PluginRequest, error) {
	var request pluginsv1alpha1.PluginRequest
	err := k8sClient.Get(ctx, client.ObjectKey{Namespace: claim.Namespace, Name: claim.Name + "-" + section}, &request)
	return &request, err
}

// clusterOf reads the CamundaCluster of claim.
func clusterOf(claim *fleetv1alpha1.CamundaClusterClaim) (*v1.CamundaCluster, error) {
	var cluster v1.CamundaCluster
	err := k8sClient.Get(ctx, client.ObjectKeyFromObject(claim), &cluster)
	return &cluster, err
}

// latest reads claim again.
func latest(g Gomega, claim *fleetv1alpha1.CamundaClusterClaim) *fleetv1alpha1.CamundaClusterClaim {
	var current fleetv1alpha1.CamundaClusterClaim
	g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(claim), &current)).To(Succeed())
	return &current
}

// expectFalse polls until the condition of claim is False with reason, and
// its message matches message.
func expectFalse(
	claim *fleetv1alpha1.CamundaClusterClaim,
	conditionType string,
	reason string,
	message OmegaMatcher,
) {
	GinkgoHelper()
	Eventually(func(g Gomega) {
		condition := meta.FindStatusCondition(latest(g, claim).Status.Conditions, conditionType)
		g.Expect(condition).NotTo(BeNil())
		g.Expect(condition.Status).To(Equal(metav1.ConditionFalse))
		g.Expect(condition.Reason).To(Equal(reason))
		g.Expect(condition.Message).To(message)
	}, timeout, interval).Should(Succeed())
}

// eventuallyCluster polls until the CamundaCluster of claim exists and
// returns it.
func eventuallyCluster(claim *fleetv1alpha1.CamundaClusterClaim) *v1.CamundaCluster {
	GinkgoHelper()
	var cluster *v1.CamundaCluster
	Eventually(func() error {
		var err error
		cluster, err = clusterOf(claim)
		return err
	}, timeout, interval).Should(Succeed())
	return cluster
}

// setClusterReady writes the Ready condition of the CamundaCluster of
// claim for its current generation. No controller of the suite reconciles a
// CamundaCluster, so the spec plays its part.
func setClusterReady(claim *fleetv1alpha1.CamundaClusterClaim, status metav1.ConditionStatus, reason, message string) {
	GinkgoHelper()
	Eventually(func() error {
		cluster, err := clusterOf(claim)
		if err != nil {
			return err
		}
		cluster.Status.ObservedGeneration = cluster.Generation
		meta.SetStatusCondition(&cluster.Status.Conditions, metav1.Condition{
			Type:               v1.ConditionReady,
			Status:             status,
			Reason:             reason,
			Message:            message,
			ObservedGeneration: cluster.Generation,
		})
		return k8sClient.Status().Update(ctx, cluster)
	}, timeout, interval).Should(Succeed())
}
