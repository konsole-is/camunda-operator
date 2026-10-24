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

package camundamanagementclusterclaim

import (
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	utilrand "k8s.io/apimachinery/pkg/util/rand"
	"sigs.k8s.io/controller-runtime/pkg/client"

	fleetv1alpha1 "github.com/konsole-is/camunda-operator/api/fleet/v1alpha1"
	pluginsv1alpha1 "github.com/konsole-is/camunda-operator/api/plugins/v1alpha1"
	v1 "github.com/konsole-is/camunda-operator/api/v1"
	"github.com/konsole-is/camunda-operator/internal/pluginstartup"
	"github.com/konsole-is/camunda-operator/pkg/planedatabases"
)

// consistently is how long a spec watches that nothing changes.
const consistently = 2 * time.Second

// platformConfigName is the CamundaPlatformConfig that every Environment of
// the suite names.
const platformConfigName = "suite-platform"

// The test registrations, and the outputs that the fake plugin writes for
// them.
const (
	registrationAccess   = "test-access"
	registrationIdentity = "test-identity"
	// registrationDeleteOnly serves management-database and supports only
	// the deletion policy Delete.
	registrationDeleteOnly = "test-delete-only"

	managementHost = "plane.example.com"
	// ownIdentityURL is the Identity URL that a claim sets itself.
	ownIdentityURL = "https://own.example.com/identity"
)

// databaseOutputs are the outputs of every management-database request.
var databaseOutputs = map[string]string{
	planedatabases.Of(planedatabases.Keycloak).Output:   "plane-keycloak",
	planedatabases.Of(planedatabases.Identity).Output:   "plane-identity",
	planedatabases.Of(planedatabases.WebModeler).Output: "plane-webmodeler",
}

// createFixtures creates the core sections, the built-in and test
// registrations, and the platform config, and tells the fake plugin what to
// write for each registration.
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

	deleteOnly := testRegistration(registrationDeleteOnly, pluginstartup.SectionManagementDatabase)
	deleteOnly.Spec.DeletionPolicies = []pluginsv1alpha1.DeletionPolicy{pluginsv1alpha1.DeletionPolicyDelete}
	for _, registration := range []*pluginsv1alpha1.PluginRegistration{
		deleteOnly,
		testRegistration(registrationAccess, pluginstartup.SectionAccess),
		testRegistration(registrationIdentity, pluginstartup.SectionCloudIdentity),
	} {
		Expect(k8sClient.Create(ctx, registration)).To(Succeed())
	}

	Expect(k8sClient.Create(ctx, &v1.CamundaPlatformConfig{
		ObjectMeta: metav1.ObjectMeta{Name: platformConfigName},
		Spec:       v1.CamundaPlatformConfigSpec{Auth: &v1.PlatformAuthSpec{Method: v1.AuthenticationMethodBasic}},
	})).To(Succeed())

	plugin.serve(pluginstartup.RegistrationManagementDatabaseServer, databaseOutputs)
	plugin.serve(registrationDeleteOnly, databaseOutputs)
	plugin.serve(registrationAccess, map[string]string{pluginstartup.OutputManagementHost: managementHost})
	plugin.serve(registrationIdentity, map[string]string{
		pluginstartup.OutputServiceAccountAnnotations: `{"eks.amazonaws.com/role-arn":"arn:plane"}`,
	})
}

// testRegistration returns a registration that serves section and supports
// both deletion policies.
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

// planeEnvironment returns an Environment that names the suite platform
// config and allows the built-in and test registrations of the plane
// sections, and test-identity in cloud-identity.
func planeEnvironment() *fleetv1alpha1.Environment {
	return &fleetv1alpha1.Environment{
		ObjectMeta: metav1.ObjectMeta{Name: uniqueName("env")},
		Spec: fleetv1alpha1.EnvironmentSpec{
			PlatformConfigRef: platformConfigName,
			Sections: map[string]fleetv1alpha1.SectionPolicy{
				pluginstartup.SectionManagementDatabase: {Allowed: []string{
					pluginstartup.RegistrationManagementDatabaseServer,
					registrationDeleteOnly,
				}},
				pluginstartup.SectionAccess:        {Allowed: []string{registrationAccess}},
				pluginstartup.SectionCloudIdentity: {Allowed: []string{registrationIdentity}},
			},
		},
	}
}

// newEnvironmentNamespace creates environment and a namespace that carries
// its label, and returns the name of the namespace.
func newEnvironmentNamespace(environment *fleetv1alpha1.Environment) string {
	GinkgoHelper()
	createEnvironment(environment)
	return newNamespace(map[string]string{fleetv1alpha1.LabelEnvironment: environment.Name})
}

// newClaim returns a claim my-plane in namespace that serves environments,
// in the Keycloak mode with Console and Web Modeler, and selects the
// built-in database server and test-access.
func newClaim(namespace string, environments ...string) *fleetv1alpha1.CamundaManagementClusterClaim {
	return &fleetv1alpha1.CamundaManagementClusterClaim{
		ObjectMeta: metav1.ObjectMeta{Name: "my-plane", Namespace: namespace},
		Spec: fleetv1alpha1.CamundaManagementClusterClaimSpec{
			Environments: environments,
			Management: fleetv1alpha1.ManagementOverrides{
				IdentityProvider: fleetv1alpha1.ManagementIdentityProvider{
					Keycloak: &fleetv1alpha1.KeycloakOverrides{},
				},
				Identity: fleetv1alpha1.IdentityOverrides{
					Admin: v1.IdentityAdminSpec{Username: "admin", Email: "admin@example.com"},
				},
				Console: &fleetv1alpha1.ConsoleOverrides{},
				WebModeler: &fleetv1alpha1.WebModelerOverrides{
					Mail: v1.WebModelerMailSpec{SMTPHost: "smtp.example.com", FromAddress: "modeler@example.com"},
				},
			},
			Versions: fleetv1alpha1.ManagementVersions{
				Identity:   "8.9.10",
				Keycloak:   "26.3.1",
				Console:    "8.9.122",
				WebModeler: "8.9.11",
			},
			Plugins: []fleetv1alpha1.PluginSelection{
				selection(pluginstartup.SectionAccess, registrationAccess),
				selection(pluginstartup.SectionManagementDatabase, pluginstartup.RegistrationManagementDatabaseServer),
			},
		},
	}
}

// newPlaneClaim creates an Environment and a namespace with its label, and
// returns newClaim in that namespace, serving that Environment.
func newPlaneClaim() *fleetv1alpha1.CamundaManagementClusterClaim {
	GinkgoHelper()
	environment := planeEnvironment()
	return newClaim(newEnvironmentNamespace(environment), environment.Name)
}

// setURLs sets every URL of the claim, and drops the access selection.
func setURLs(claim *fleetv1alpha1.CamundaManagementClusterClaim) {
	m := &claim.Spec.Management
	m.IdentityProvider.Keycloak.ExternalURL = "https://login.example.com/auth"
	m.Identity.ExternalURL = ownIdentityURL
	m.Console.ExternalURL = "https://console.example.com"
	m.WebModeler.ExternalURL = "https://modeler.example.com"
	m.WebModeler.WebsocketsExternalURL = "https://modeler-ws.example.com"
	claim.Spec.Plugins = []fleetv1alpha1.PluginSelection{
		selection(pluginstartup.SectionManagementDatabase, pluginstartup.RegistrationManagementDatabaseServer),
	}
}

func selection(section, registration string) fleetv1alpha1.PluginSelection {
	return fleetv1alpha1.PluginSelection{Section: section, Registration: registration}
}

// requestKey returns the key of the request of claim in section.
func requestKey(claim *fleetv1alpha1.CamundaManagementClusterClaim, section string) types.NamespacedName {
	return types.NamespacedName{Namespace: claim.Namespace, Name: claim.Name + "-" + section}
}

// requestOf reads the request of claim in section.
func requestOf(
	claim *fleetv1alpha1.CamundaManagementClusterClaim,
	section string,
) (*pluginsv1alpha1.PluginRequest, error) {
	var request pluginsv1alpha1.PluginRequest
	err := k8sClient.Get(ctx, requestKey(claim, section), &request)
	return &request, err
}

// planeOf reads the CamundaManagementCluster of claim.
func planeOf(claim *fleetv1alpha1.CamundaManagementClusterClaim) (*v1.CamundaManagementCluster, error) {
	var plane v1.CamundaManagementCluster
	err := k8sClient.Get(ctx, client.ObjectKeyFromObject(claim), &plane)
	return &plane, err
}

// latest reads claim again.
func latest(g Gomega, claim *fleetv1alpha1.CamundaManagementClusterClaim) *fleetv1alpha1.CamundaManagementClusterClaim {
	var current fleetv1alpha1.CamundaManagementClusterClaim
	g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(claim), &current)).To(Succeed())
	return &current
}

// update applies change to the current claim and updates it.
func update(
	claim *fleetv1alpha1.CamundaManagementClusterClaim,
	change func(*fleetv1alpha1.CamundaManagementClusterClaim),
) {
	GinkgoHelper()
	Eventually(func() error {
		current := &fleetv1alpha1.CamundaManagementClusterClaim{}
		if err := k8sClient.Get(ctx, client.ObjectKeyFromObject(claim), current); err != nil {
			return err
		}
		change(current)
		return k8sClient.Update(ctx, current)
	}, timeout, interval).Should(Succeed())
}

// expectCondition polls until the condition of claim has status and
// reason, and its message matches message.
func expectCondition(
	claim *fleetv1alpha1.CamundaManagementClusterClaim,
	conditionType string,
	status metav1.ConditionStatus,
	reason string,
	message OmegaMatcher,
) {
	GinkgoHelper()
	Eventually(func(g Gomega) {
		condition := meta.FindStatusCondition(latest(g, claim).Status.Conditions, conditionType)
		g.Expect(condition).NotTo(BeNil())
		g.Expect(condition.Status).To(Equal(status))
		g.Expect(condition.Reason).To(Equal(reason))
		g.Expect(condition.Message).To(message)
	}, timeout, interval).Should(Succeed())
}

// expectNotReady polls until Ready of claim is False with reason, and its
// message matches message.
func expectNotReady(claim *fleetv1alpha1.CamundaManagementClusterClaim, reason string, message OmegaMatcher) {
	GinkgoHelper()
	expectCondition(claim, v1.ConditionReady, metav1.ConditionFalse, reason, message)
}

// eventuallyPlane polls until the CamundaManagementCluster of claim exists
// and passes check, and returns it.
func eventuallyPlane(
	claim *fleetv1alpha1.CamundaManagementClusterClaim,
	check func(g Gomega, plane *v1.CamundaManagementCluster),
) *v1.CamundaManagementCluster {
	GinkgoHelper()
	var plane *v1.CamundaManagementCluster
	Eventually(func(g Gomega) {
		var err error
		plane, err = planeOf(claim)
		g.Expect(err).NotTo(HaveOccurred())
		if check != nil {
			check(g, plane)
		}
	}, timeout, interval).Should(Succeed())
	return plane
}

// setPlaneReady writes the Ready condition of the CamundaManagementCluster
// of claim for its current generation. No controller of the suite
// reconciles a CamundaManagementCluster, so the spec plays its part.
func setPlaneReady(
	claim *fleetv1alpha1.CamundaManagementClusterClaim,
	status metav1.ConditionStatus,
	reason, message string,
) {
	GinkgoHelper()
	Eventually(func() error {
		plane, err := planeOf(claim)
		if err != nil {
			return err
		}
		plane.Status.ObservedGeneration = plane.Generation
		meta.SetStatusCondition(&plane.Status.Conditions, metav1.Condition{
			Type:               v1.ConditionReady,
			Status:             status,
			Reason:             reason,
			Message:            message,
			ObservedGeneration: plane.Generation,
		})
		return k8sClient.Status().Update(ctx, plane)
	}, timeout, interval).Should(Succeed())
}

// expectGone polls until obj does not exist.
func expectGone(obj client.Object) {
	GinkgoHelper()
	Eventually(func() bool {
		return apierrors.IsNotFound(k8sClient.Get(ctx, client.ObjectKeyFromObject(obj), obj))
	}, timeout, interval).Should(BeTrue())
}
