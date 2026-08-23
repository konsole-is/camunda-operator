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
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	utilrand "k8s.io/apimachinery/pkg/util/rand"
	"sigs.k8s.io/controller-runtime/pkg/client"

	v1 "github.com/konsole-is/camunda-operator/api/v1"
	"github.com/konsole-is/camunda-operator/pkg/camundaadmin/camundaadmintest"
	clustercomponents "github.com/konsole-is/camunda-operator/pkg/components/camundacluster"
	components "github.com/konsole-is/camunda-operator/pkg/components/camundamanagementcluster"
)

// The administrator that every basic-auth orchestration cluster of this file
// holds, and that the operator authenticates as.
const (
	clusterAdminUsername = "admin"
	clusterAdminPassword = "cluster-admin-password"
)

// failureBudget is how many calls a spec makes the fake cluster fail when it
// wants every call of a run to fail. The reconcile retries, so the count only
// has to outlast the spec.
const failureBudget = 1000

var _ = Describe("Web Modeler", func() {
	It("deploys both processes with the pusher credentials paired", func() {
		s := newScenario(withWebModeler)

		Eventually(func(g Gomega) {
			pusher := readSecret(g, s.namespace, components.PusherSecretName(s.mc))
			g.Expect(pusher.Data).To(HaveKey(components.PusherAppKeyKey))

			restapi := readDeployment(g, s.namespace, components.WebModelerRestapiName(s.mc))
			websockets := readDeployment(g, s.namespace, components.WebModelerWebsocketsName(s.mc))
			g.Expect(envOf(restapi)).To(HaveKeyWithValue(
				"RESTAPI_PUSHER_HOST",
				components.WebModelerWebsocketsName(s.mc)+"."+s.namespace+".svc",
			))
			g.Expect(secretRefOf(restapi, "RESTAPI_PUSHER_KEY")).To(
				Equal(secretRefOf(websockets, "PUSHER_APP_KEY")),
			)
			g.Expect(secretRefOf(restapi, "RESTAPI_PUSHER_SECRET")).To(
				Equal(secretRefOf(websockets, "PUSHER_APP_SECRET")),
			)
		}, timeout, interval).Should(Succeed())
	})

	It("deploys neither process while the spec does not ask for Web Modeler", func() {
		s := newScenario()

		expectIdentityDeployed(s)

		var deployment appsv1.Deployment
		key := client.ObjectKey{
			Namespace: s.namespace, Name: components.WebModelerRestapiName(s.mc),
		}
		Expect(k8sClient.Get(ctx, key, &deployment)).NotTo(Succeed())
	})

	It("removes both Deployments when the spec stops asking for Web Modeler", func() {
		s := newScenario(withWebModeler)

		Eventually(func(g Gomega) {
			readDeployment(g, s.namespace, components.WebModelerRestapiName(s.mc))
			readDeployment(g, s.namespace, components.WebModelerWebsocketsName(s.mc))
		}, timeout, interval).Should(Succeed())

		disableWebModeler(s.mc)

		Eventually(func(g Gomega) {
			expectDeploymentGone(g, s.namespace, components.WebModelerRestapiName(s.mc))
			expectDeploymentGone(g, s.namespace, components.WebModelerWebsocketsName(s.mc))
		}, timeout, interval).Should(Succeed())
	})

	It("lists an attached oidc cluster with its own token", func() {
		s := newScenario(withWebModeler, withSelector(map[string]string{}))
		cluster := createOrchestrationCluster(s, nil, true)

		expectAttached(s.mc, cluster)

		Eventually(func(g Gomega) {
			env := envOf(readDeployment(g, s.namespace, components.WebModelerRestapiName(s.mc)))
			g.Expect(env).To(HaveKeyWithValue(
				"CAMUNDA_MODELER_CLUSTERS_0_NAME", s.namespace+"/"+cluster.Name,
			))
			g.Expect(env).To(HaveKeyWithValue(
				"CAMUNDA_MODELER_CLUSTERS_0_AUTHENTICATION", "BEARER_TOKEN",
			))
		}, timeout, interval).Should(Succeed())
	})

	It("gives Web Modeler its own user on an attached basic-auth cluster", func() {
		api := newClusterUserAPI()
		s := newScenario(withWebModeler, withSelector(map[string]string{}))
		cluster := createBasicCluster(s, api.URL())

		expectAttached(s.mc, cluster)

		Eventually(func(g Gomega) {
			g.Expect(api.Exists(components.WebModelerClusterUsername)).To(BeTrue())

			published := readSecret(
				g, s.namespace, components.WebModelerClusterUserSecretName(s.mc, cluster.UID),
			)
			g.Expect(string(published.Data[components.WebModelerClusterUserPasswordKey])).To(
				Equal(api.Password(components.WebModelerClusterUsername)),
			)
			g.Expect(string(published.Data[components.WebModelerClusterUserAppliedKey])).To(
				Equal("true"),
			)

			env := envOf(readDeployment(g, s.namespace, components.WebModelerRestapiName(s.mc)))
			g.Expect(env).To(HaveKeyWithValue(
				"CAMUNDA_MODELER_CLUSTERS_0_AUTHENTICATION", "BASIC",
			))
		}, timeout, interval).Should(Succeed())

		Expect(api.Authorizations()).To(ConsistOf(webModelerGrants()))
	})

	It("grants the authorizations on a cluster that already holds the user", func() {
		api := newClusterUserAPI()
		api.SetUser(
			components.WebModelerClusterUsername,
			components.WebModelerClusterUsername,
			"web-modeler@example.com",
			"a-password-the-operator-never-set",
		)

		s := newScenario(withWebModeler, withSelector(map[string]string{}))
		cluster := createBasicCluster(s, api.URL())

		expectAttached(s.mc, cluster)

		Eventually(func(g Gomega) {
			published := readSecret(
				g, s.namespace, components.WebModelerClusterUserSecretName(s.mc, cluster.UID),
			)
			g.Expect(string(published.Data[components.WebModelerClusterUserAppliedKey])).To(
				Equal("true"),
			)
			g.Expect(api.Password(components.WebModelerClusterUsername)).To(
				Equal(string(published.Data[components.WebModelerClusterUserPasswordKey])),
			)
			g.Expect(api.Authorizations()).To(ContainElements(webModelerGrants()))
		}, timeout, interval).Should(Succeed())
	})

	It("sets a new password on the cluster when the user Secret is deleted", func() {
		api := newClusterUserAPI()
		s := newScenario(withWebModeler, withSelector(map[string]string{}))
		cluster := createBasicCluster(s, api.URL())

		expectAttached(s.mc, cluster)

		name := components.WebModelerClusterUserSecretName(s.mc, cluster.UID)
		var first string
		Eventually(func(g Gomega) {
			published := readSecret(g, s.namespace, name)
			g.Expect(string(published.Data[components.WebModelerClusterUserAppliedKey])).To(
				Equal("true"),
			)
			first = string(published.Data[components.WebModelerClusterUserPasswordKey])
		}, timeout, interval).Should(Succeed())

		Expect(k8sClient.Delete(ctx, &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{Namespace: s.namespace, Name: name},
		})).To(Succeed())

		Eventually(func(g Gomega) {
			published := readSecret(g, s.namespace, name)
			rotated := string(published.Data[components.WebModelerClusterUserPasswordKey])
			g.Expect(rotated).NotTo(Equal(first))
			g.Expect(api.Password(components.WebModelerClusterUsername)).To(Equal(rotated))
		}, timeout, interval).Should(Succeed())
	})

	It("reports a cluster that refused the user and serves the rest", func() {
		api := newClusterUserAPI()
		api.RefuseNext(100, "the cluster manages its users elsewhere")

		s := newScenario(withWebModeler, withSelector(map[string]string{}))
		cluster := createBasicCluster(s, api.URL())

		Eventually(func(g Gomega) {
			row := rowOf(g, s.mc, cluster)
			g.Expect(row.Reason).To(Equal(v1.ReasonBasicAuthUserFailed))
			g.Expect(row.Message).To(ContainSubstring("manages its users elsewhere"))
			// The management plane still serves the cluster. Only the user of
			// Web Modeler on it is missing.
			g.Expect(row.Attached).To(BeTrue())
		}, timeout, interval).Should(Succeed())

		expectIdentityDeployed(s)
	})

	It("removes the user from a cluster that leaves the selector", func() {
		api := newClusterUserAPI()
		s := newScenario(withWebModeler, withSelector(map[string]string{}))
		cluster := createBasicCluster(s, api.URL())

		expectAttached(s.mc, cluster)
		name := components.WebModelerClusterUserSecretName(s.mc, cluster.UID)
		Eventually(func(g Gomega) {
			readSecret(g, s.namespace, name)
			g.Expect(api.Exists(components.WebModelerClusterUsername)).To(BeTrue())
		}, timeout, interval).Should(Succeed())

		Eventually(func(g Gomega) {
			latest := readManagementCluster(g, s.mc)
			latest.Spec.ClusterSelector = nil
			g.Expect(k8sClient.Update(ctx, latest)).To(Succeed())
		}, timeout, interval).Should(Succeed())

		Eventually(func(g Gomega) {
			g.Expect(api.Exists(components.WebModelerClusterUsername)).To(BeFalse())
			expectSecretGone(g, s.namespace, name)
		}, timeout, interval).Should(Succeed())
	})

	It("keeps the password Secret until the cluster has removed the user", func() {
		api := newClusterUserAPI()
		s := newScenario(withWebModeler, withSelector(map[string]string{}))
		cluster := createBasicCluster(s, api.URL())

		expectAttached(s.mc, cluster)
		name := components.WebModelerClusterUserSecretName(s.mc, cluster.UID)
		Eventually(func(g Gomega) {
			readSecret(g, s.namespace, name)
			g.Expect(api.Exists(components.WebModelerClusterUsername)).To(BeTrue())
		}, timeout, interval).Should(Succeed())

		// The cluster refuses every removal for a while. The Secret is the
		// record that makes the next reconcile try again, so it has to stay
		// as long as the user does.
		api.FailNext("deleteUser", 1000)
		Eventually(func(g Gomega) {
			latest := readManagementCluster(g, s.mc)
			latest.Spec.ClusterSelector = nil
			g.Expect(k8sClient.Update(ctx, latest)).To(Succeed())
		}, timeout, interval).Should(Succeed())

		Consistently(func(g Gomega) {
			g.Expect(api.Exists(components.WebModelerClusterUsername)).To(BeTrue())
			readSecret(g, s.namespace, name)
		}, 5*time.Second, interval).Should(Succeed())

		api.FailNext("deleteUser", 0)
		Eventually(func(g Gomega) {
			g.Expect(api.Exists(components.WebModelerClusterUsername)).To(BeFalse())
			expectSecretGone(g, s.namespace, name)
		}, timeout, interval).Should(Succeed())
	})

	It("removes the user when the spec stops asking for Web Modeler", func() {
		api := newClusterUserAPI()
		s := newScenario(withWebModeler, withSelector(map[string]string{}))
		cluster := createBasicCluster(s, api.URL())

		expectAttached(s.mc, cluster)
		name := components.WebModelerClusterUserSecretName(s.mc, cluster.UID)
		Eventually(func(g Gomega) {
			readSecret(g, s.namespace, name)
			g.Expect(api.Exists(components.WebModelerClusterUsername)).To(BeTrue())
		}, timeout, interval).Should(Succeed())

		disableWebModeler(s.mc)

		Eventually(func(g Gomega) {
			g.Expect(api.Exists(components.WebModelerClusterUsername)).To(BeFalse())
			expectSecretGone(g, s.namespace, name)
		}, timeout, interval).Should(Succeed())
	})

	It("removes the user when the cluster stops authenticating with basic credentials", func() {
		api := newClusterUserAPI()
		s := newScenario(withWebModeler, withSelector(map[string]string{}))
		cluster := createBasicCluster(s, api.URL())

		expectAttached(s.mc, cluster)
		name := components.WebModelerClusterUserSecretName(s.mc, cluster.UID)
		Eventually(func(g Gomega) {
			readSecret(g, s.namespace, name)
			g.Expect(api.Exists(components.WebModelerClusterUsername)).To(BeTrue())
		}, timeout, interval).Should(Succeed())

		// The management plane authenticates with oidc, so the platform config
		// it reads moves the cluster off basic credentials.
		Eventually(func(g Gomega) {
			var latest v1.CamundaCluster
			g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(cluster), &latest)).To(Succeed())
			latest.Spec.PlatformConfigRef = s.mc.Spec.PlatformConfigRef
			g.Expect(k8sClient.Update(ctx, &latest)).To(Succeed())
		}, timeout, interval).Should(Succeed())

		Eventually(func(g Gomega) {
			g.Expect(api.Exists(components.WebModelerClusterUsername)).To(BeFalse())
			expectSecretGone(g, s.namespace, name)
		}, timeout, interval).Should(Succeed())
	})

	It("deletes the password of a cluster that was deleted outright", func() {
		api := newClusterUserAPI()
		s := newScenario(withWebModeler, withSelector(map[string]string{}))
		cluster := createBasicCluster(s, api.URL())

		expectAttached(s.mc, cluster)
		name := components.WebModelerClusterUserSecretName(s.mc, cluster.UID)
		Eventually(func(g Gomega) {
			readSecret(g, s.namespace, name)
		}, timeout, interval).Should(Succeed())

		Expect(k8sClient.Delete(ctx, cluster)).To(Succeed())

		Eventually(func(g Gomega) {
			expectSecretGone(g, s.namespace, name)
		}, timeout, interval).Should(Succeed())
	})

	It("completes the user from a Secret that a failed pass left unmarked", func() {
		api := newClusterUserAPI()
		api.FailNext("createUser", failureBudget)

		s := newScenario(withWebModeler, withSelector(map[string]string{}))
		cluster := createBasicCluster(s, api.URL())

		name := components.WebModelerClusterUserSecretName(s.mc, cluster.UID)
		var reserved string
		Eventually(func(g Gomega) {
			published := readSecret(g, s.namespace, name)
			g.Expect(published.Data).NotTo(HaveKey(components.WebModelerClusterUserAppliedKey))
			reserved = string(published.Data[components.WebModelerClusterUserPasswordKey])
			g.Expect(reserved).NotTo(BeEmpty())
			g.Expect(api.Exists(components.WebModelerClusterUsername)).To(BeFalse())
		}, timeout, interval).Should(Succeed())

		api.FailNext("createUser", 0)
		touchCluster(cluster)

		Eventually(func(g Gomega) {
			published := readSecret(g, s.namespace, name)
			g.Expect(string(published.Data[components.WebModelerClusterUserAppliedKey])).To(
				Equal("true"),
			)
			g.Expect(string(published.Data[components.WebModelerClusterUserPasswordKey])).To(
				Equal(reserved),
			)
			g.Expect(api.Password(components.WebModelerClusterUsername)).To(Equal(reserved))
		}, timeout, interval).Should(Succeed())
	})

	It("removes the user with the management cluster", func() {
		api := newClusterUserAPI()
		s := newScenario(withWebModeler, withSelector(map[string]string{}))
		cluster := createBasicCluster(s, api.URL())

		expectAttached(s.mc, cluster)
		Eventually(func() bool {
			return api.Exists(components.WebModelerClusterUsername)
		}, timeout, interval).Should(BeTrue())

		Expect(deleteManagementCluster(s.mc)).To(Succeed())

		Expect(api.Exists(components.WebModelerClusterUsername)).To(BeFalse())
	})
})

// webModelerGrants are the authorizations that the operator must have granted
// the Web Modeler user on a basic-auth cluster.
func webModelerGrants() []camundaadmintest.Authorization {
	return []camundaadmintest.Authorization{
		{
			OwnerID:         components.WebModelerClusterUsername,
			OwnerType:       "USER",
			ResourceType:    "RESOURCE",
			ResourceID:      "*",
			PermissionTypes: []string{"CREATE"},
		},
		{
			OwnerID:      components.WebModelerClusterUsername,
			OwnerType:    "USER",
			ResourceType: "PROCESS_DEFINITION",
			ResourceID:   "*",
			PermissionTypes: []string{
				"CREATE_PROCESS_INSTANCE", "READ_PROCESS_DEFINITION", "READ_PROCESS_INSTANCE",
			},
		},
	}
}

// disableWebModeler removes spec.webModeler from the management cluster under
// test, retrying the write until no other writer holds the object.
func disableWebModeler(mc *v1.CamundaManagementCluster) {
	GinkgoHelper()

	Eventually(func() error {
		var latest v1.CamundaManagementCluster
		if err := k8sClient.Get(ctx, client.ObjectKeyFromObject(mc), &latest); err != nil {
			return err
		}
		latest.Spec.WebModeler = nil

		return k8sClient.Update(ctx, &latest)
	}, timeout, interval).Should(Succeed())
}

// touchCluster relabels an orchestration cluster, so that the management
// cluster which selects it reconciles again.
func touchCluster(cluster *v1.CamundaCluster) {
	GinkgoHelper()

	Eventually(func(g Gomega) {
		var latest v1.CamundaCluster
		g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(cluster), &latest)).To(Succeed())
		if latest.Labels == nil {
			latest.Labels = map[string]string{}
		}
		latest.Labels["touched"] = utilrand.String(8)
		g.Expect(k8sClient.Update(ctx, &latest)).To(Succeed())
	}, timeout, interval).Should(Succeed())
}

// expectDeploymentGone asserts that the API server holds no Deployment of that
// name.
func expectDeploymentGone(g Gomega, namespace, name string) {
	var deployment appsv1.Deployment
	err := k8sClient.Get(ctx, client.ObjectKey{Namespace: namespace, Name: name}, &deployment)
	g.Expect(apierrors.IsNotFound(err)).To(BeTrue(), "Deployment %s/%s is still there", namespace, name)
}

// expectServiceGone asserts that the API server holds no Service of that name.
func expectServiceGone(g Gomega, namespace, name string) {
	var svc corev1.Service
	err := k8sClient.Get(ctx, client.ObjectKey{Namespace: namespace, Name: name}, &svc)
	g.Expect(apierrors.IsNotFound(err)).To(BeTrue(), "Service %s/%s is still there", namespace, name)
}

// expectSecretGone asserts that the API server holds no Secret of that name.
func expectSecretGone(g Gomega, namespace, name string) {
	var secret corev1.Secret
	err := k8sClient.Get(ctx, client.ObjectKey{Namespace: namespace, Name: name}, &secret)
	g.Expect(apierrors.IsNotFound(err)).To(BeTrue(), "Secret %s/%s is still there", namespace, name)
}

// withWebModeler adds Web Modeler to the management cluster under test, with a
// database of its own and the two clients that the oidc mode needs.
func withWebModeler(f *fixture) {
	f.mc.Spec.WebModeler = &v1.WebModelerSpec{
		Version:               "8.9.4",
		ExternalURL:           "https://modeler.example.com",
		WebsocketsExternalURL: "https://modeler.example.com/modeler-ws",
		DatabaseConfigRef:     createWebModelerDatabase(f.mc.Namespace),
		Mail: v1.WebModelerMailSpec{
			SMTPHost:    "smtp.example.com",
			SMTPPort:    587,
			FromAddress: "noreply@example.com",
		},
	}

	clients := &f.platform.Spec.Auth.OIDC.Management.Clients
	clients.WebModeler = &v1.PublicClientSpec{ClientID: "web-modeler"}
	clients.WebModelerAPI = &v1.WebModelerAPIClientSpec{
		ConfidentialClientSpec: v1.ConfidentialClientSpec{
			ClientID: "web-modeler-api",
			ClientSecretRef: v1.SecretKeyRef{
				Name:      "oidc-credentials",
				Namespace: f.mc.Namespace,
				Key:       "identity-client-secret",
			},
		},
	}
}

// createWebModelerDatabase creates the DatabaseConfig that Web Modeler opens,
// with a credentials Secret of its own. The spec refuses two components that
// name one DatabaseConfig, so Web Modeler never shares the one Management
// Identity opens.
func createWebModelerDatabase(namespace string) string {
	GinkgoHelper()

	credentials := v1.CredentialsSecretRef{
		Name: "db-credentials-" + utilrand.String(8), Namespace: namespace,
		UsernameKey: "username", PasswordKey: "password",
	}

	server := &v1.DatabaseServerConfig{
		ObjectMeta: metav1.ObjectMeta{Name: "dbsc-modeler-" + utilrand.String(8)},
		Spec: v1.DatabaseServerConfigSpec{
			Engine:                    v1.DatabaseEnginePostgres,
			Host:                      "postgres." + namespace + ".svc",
			Port:                      5432,
			AdminCredentialsSecretRef: credentials,
		},
	}
	Expect(k8sClient.Create(ctx, server)).To(Succeed())
	DeferCleanup(func() { _ = k8sClient.Delete(ctx, server) })

	database := &v1.DatabaseConfig{
		ObjectMeta: metav1.ObjectMeta{
			Name: "dbc-modeler-" + utilrand.String(8), Namespace: namespace,
		},
		Spec: v1.DatabaseConfigSpec{
			ServerRef:            server.Name,
			DatabaseName:         "web-modeler",
			CredentialsSecretRef: credentials,
		},
	}
	Expect(k8sClient.Create(ctx, database)).To(Succeed())
	DeferCleanup(func() { _ = k8sClient.Delete(ctx, database) })
	createSecret(namespace, credentials.Name, map[string]string{
		"username": "web-modeler", "password": "db-s3cret",
	})

	return database.Name
}

// newClusterUserAPI starts a fake orchestration cluster user API that holds
// the administrator the operator authenticates as.
func newClusterUserAPI() *camundaadmintest.UserAPI {
	GinkgoHelper()

	api := camundaadmintest.NewUserAPI()
	DeferCleanup(api.Close)
	api.SetUser(clusterAdminUsername, "Admin", "admin@example.com", clusterAdminPassword)

	return api
}

// createBasicCluster creates a CamundaCluster that authenticates with basic
// credentials, publishes endpoint as its REST address, and holds the
// administrator credentials that its own controller would publish.
func createBasicCluster(s scenario, endpoint string) *v1.CamundaCluster {
	GinkgoHelper()

	platform := &v1.CamundaPlatformConfig{
		ObjectMeta: metav1.ObjectMeta{Name: "cpc-basic-" + utilrand.String(8)},
		Spec: v1.CamundaPlatformConfigSpec{
			Auth: &v1.PlatformAuthSpec{Method: v1.AuthenticationMethodBasic},
		},
	}
	Expect(k8sClient.Create(ctx, platform)).To(Succeed())
	DeferCleanup(func() { _ = k8sClient.Delete(ctx, platform) })

	cluster := &v1.CamundaCluster{
		ObjectMeta: metav1.ObjectMeta{Name: "cc-" + utilrand.String(8), Namespace: s.namespace},
		Spec: v1.CamundaClusterSpec{
			Version:           "8.9.4",
			StorageRef:        "storage",
			PlatformConfigRef: platform.Name,
		},
	}
	Expect(k8sClient.Create(ctx, cluster)).To(Succeed())
	DeferCleanup(func() { _ = k8sClient.Delete(ctx, cluster) })

	createSecret(s.namespace, clustercomponents.AdminSecretName(cluster), map[string]string{
		clustercomponents.AdminUsernameKey: clusterAdminUsername,
		clustercomponents.AdminPasswordKey: clusterAdminPassword,
	})

	cluster.Status.Gateway = &v1.GatewayBinding{
		GRPCEndpoint: cluster.Name + "-gateway." + s.namespace + ".svc:26500",
		RESTEndpoint: endpoint,
	}
	Expect(k8sClient.Status().Update(ctx, cluster)).To(Succeed())

	return cluster
}

// expectIdentityDeployed polls until Management Identity is applied, which is
// how a spec waits for one reconcile to have gone all the way through.
func expectIdentityDeployed(s scenario) {
	GinkgoHelper()

	Eventually(func(g Gomega) {
		readDeployment(g, s.namespace, components.IdentityName(s.mc))
	}, timeout, interval).Should(Succeed())
}

// readDeployment reads a Deployment as the API server holds it now.
func readDeployment(g Gomega, namespace, name string) *appsv1.Deployment {
	var deployment appsv1.Deployment
	g.Expect(
		k8sClient.Get(ctx, client.ObjectKey{Namespace: namespace, Name: name}, &deployment),
	).To(Succeed())

	return &deployment
}

// readSecret reads a Secret as the API server holds it now.
func readSecret(g Gomega, namespace, name string) *corev1.Secret {
	var secret corev1.Secret
	g.Expect(
		k8sClient.Get(ctx, client.ObjectKey{Namespace: namespace, Name: name}, &secret),
	).To(Succeed())

	return &secret
}

// envOf returns the plain environment values of the first container of a
// Deployment.
func envOf(deployment *appsv1.Deployment) map[string]string {
	values := map[string]string{}
	for _, e := range deployment.Spec.Template.Spec.Containers[0].Env {
		if e.ValueFrom == nil {
			values[e.Name] = e.Value
		}
	}

	return values
}

// secretRefOf returns "<secret>/<key>" of an environment entry that reads a
// Secret, so that two entries can be compared for the value they read.
func secretRefOf(deployment *appsv1.Deployment, name string) string {
	for _, e := range deployment.Spec.Template.Spec.Containers[0].Env {
		if e.Name == name && e.ValueFrom != nil && e.ValueFrom.SecretKeyRef != nil {
			return e.ValueFrom.SecretKeyRef.Name + "/" + e.ValueFrom.SecretKeyRef.Key
		}
	}

	return ""
}
