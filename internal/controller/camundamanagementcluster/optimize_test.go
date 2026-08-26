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
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/sourcehawk/operator-component-framework/pkg/component"
	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	utilrand "k8s.io/apimachinery/pkg/util/rand"
	"sigs.k8s.io/controller-runtime/pkg/client"

	v1 "github.com/konsole-is/camunda-operator/api/v1"
	components "github.com/konsole-is/camunda-operator/pkg/components/camundamanagementcluster"
	"github.com/konsole-is/camunda-operator/pkg/keycloakadmin"
)

// The two Optimize addresses of this file: the one that spec.optimize names,
// which is outside this operator and reported by no CamundaOptimize, and the
// one that the CamundaOptimize resources of the specs carry.
const (
	specOptimizeURL       = "https://optimize.example.com"
	discoveredOptimizeURL = "https://optimize.blue.example.com"
)

var _ = Describe("CamundaManagementCluster controller and the Optimize instances behind it", func() {
	It("discovers a CamundaOptimize that names its contract", func() {
		keycloak := startFakeKeycloak(withOptimizeClient())
		s := newScenario(withFakeKeycloak(keycloak))

		createOptimize(s.namespace, s.mc.Name)

		Eventually(func(g Gomega) {
			rows := readManagementCluster(g, s.mc).Status.Optimize
			g.Expect(rows).To(HaveLen(1))
			g.Expect(rows[0].Namespace).To(Equal(s.namespace))
			g.Expect(rows[0].ExternalURL).To(Equal(discoveredOptimizeURL))
		}, timeout, interval).Should(Succeed())

		Eventually(func(g Gomega) {
			g.Expect(identityEnv(g, s)["KEYCLOAK_INIT_OPTIMIZE_ROOT_URL"]).To(
				Equal(specOptimizeURL + "," + discoveredOptimizeURL),
			)
		}, timeout, interval).Should(Succeed())
	})

	It("withdraws a CamundaOptimize that is deleted", func() {
		keycloak := startFakeKeycloak(withOptimizeClient())
		s := newScenario(withFakeKeycloak(keycloak))

		optimize := createOptimize(s.namespace, s.mc.Name)

		Eventually(func(g Gomega) {
			g.Expect(readManagementCluster(g, s.mc).Status.Optimize).To(HaveLen(1))
		}, timeout, interval).Should(Succeed())

		Expect(k8sClient.Delete(ctx, optimize)).To(Succeed())

		Eventually(func(g Gomega) {
			g.Expect(readManagementCluster(g, s.mc).Status.Optimize).To(BeEmpty())
			g.Expect(identityEnv(g, s)["KEYCLOAK_INIT_OPTIMIZE_ROOT_URL"]).To(Equal(specOptimizeURL))
		}, timeout, interval).Should(Succeed())
	})

	It("adds the missing callback to the Optimize client of the realm", func() {
		keycloak := startFakeKeycloak(withOptimizeClient(
			specOptimizeURL + components.OptimizeCallbackPath,
		))
		s := newScenario(withFakeKeycloak(keycloak))

		createOptimize(s.namespace, s.mc.Name)

		Eventually(func(g Gomega) {
			g.Expect(keycloak.redirectURIs()).To(Equal([]string{
				specOptimizeURL + components.OptimizeCallbackPath,
				discoveredOptimizeURL + components.OptimizeCallbackPath,
			}))

			condition := conditionOf(g, s.mc, v1.ConditionOptimizeCallbacksReady)
			g.Expect(condition.Status).To(Equal(metav1.ConditionTrue))
			g.Expect(condition.Reason).To(Equal(v1.ReasonHealthy))
		}, timeout, interval).Should(Succeed())
	})

	// A redirect URI that a person registered by hand does not end in the
	// login path of Optimize, so the operator leaves it where it is.
	It("keeps a redirect URI that this operator does not own", func() {
		keycloak := startFakeKeycloak(withOptimizeClient("https://legacy.example.com/*"))
		newScenario(withFakeKeycloak(keycloak))

		Eventually(func(g Gomega) {
			g.Expect(keycloak.redirectURIs()).To(Equal([]string{
				"https://legacy.example.com/*",
				specOptimizeURL + components.OptimizeCallbackPath,
			}))
		}, timeout, interval).Should(Succeed())
	})

	// The realm is bootstrapped by Management Identity, so the workload has to
	// be up before a missing client is the fault worth reporting. Ready only
	// takes the callback reason once every component is True.
	It("reports OptimizeClientMissing while the realm holds no Optimize client", func() {
		keycloak := startFakeKeycloak(nil)
		s := newScenario(withFakeKeycloak(keycloak))

		identity := client.ObjectKey{Namespace: s.namespace, Name: components.IdentityName(s.mc)}
		Eventually(func(g Gomega) {
			stampDeploymentReady(g, identity)

			condition := conditionOf(g, s.mc, v1.ConditionOptimizeCallbacksReady)
			g.Expect(condition.Status).To(Equal(metav1.ConditionFalse))
			g.Expect(condition.Reason).To(Equal(v1.ReasonOptimizeClientMissing))

			ready := conditionOf(g, s.mc, v1.ConditionReady)
			g.Expect(ready.Status).To(Equal(metav1.ConditionFalse))
			g.Expect(ready.Reason).To(Equal(v1.ReasonOptimizeClientMissing))
		}, timeout, interval).Should(Succeed())
	})

	// A component that is still starting is the cause, and the realm it has
	// not bootstrapped yet is the symptom. Ready names the cause.
	It("lets a starting component decide Ready before the callbacks do", func() {
		keycloak := startFakeKeycloak(nil)
		s := newScenario(withFakeKeycloak(keycloak))

		Eventually(func(g Gomega) {
			g.Expect(conditionOf(g, s.mc, v1.ConditionOptimizeCallbacksReady).Reason).To(
				Equal(v1.ReasonOptimizeClientMissing),
			)
			g.Expect(conditionOf(g, s.mc, v1.ConditionReady).Reason).NotTo(
				Equal(v1.ReasonOptimizeClientMissing),
			)
		}, timeout, interval).Should(Succeed())
	})

	It("reports NoCallbacks while no Optimize names a URL", func() {
		keycloak := startFakeKeycloak(withOptimizeClient())
		s := newScenario(withFakeKeycloakOnly(keycloak))

		Eventually(func(g Gomega) {
			condition := conditionOf(g, s.mc, v1.ConditionOptimizeCallbacksReady)
			g.Expect(condition.Status).To(Equal(metav1.ConditionTrue))
			g.Expect(condition.Reason).To(Equal(v1.ReasonNoCallbacks))

			g.Expect(identityEnv(g, s)).NotTo(HaveKey("KEYCLOAK_INIT_OPTIMIZE_ROOT_URL"))
		}, timeout, interval).Should(Succeed())

		// A management plane that has reported an empty realm stops calling
		// Keycloak, so a plane that serves no Optimize costs it nothing.
		at := keycloak.requests()
		Consistently(func(g Gomega) {
			g.Expect(keycloak.requests()).To(Equal(at))
		}, "2s", interval).Should(Succeed())
	})

	// Nothing else removes the callback of an Optimize that went away. The
	// rendered environment of a management plane that serves none carries no
	// Optimize preset, so Management Identity never rewrites the client.
	It("withdraws the callback of the last Optimize and then rests", func() {
		keycloak := startFakeKeycloak(withOptimizeClient())
		s := newScenario(withFakeKeycloakOnly(keycloak))

		optimize := createOptimize(s.namespace, s.mc.Name)

		Eventually(func(g Gomega) {
			g.Expect(keycloak.redirectURIs()).To(Equal([]string{
				discoveredOptimizeURL + components.OptimizeCallbackPath,
			}))
		}, timeout, interval).Should(Succeed())

		Expect(k8sClient.Delete(ctx, optimize)).To(Succeed())

		Eventually(func(g Gomega) {
			g.Expect(keycloak.redirectURIs()).To(BeEmpty())

			condition := conditionOf(g, s.mc, v1.ConditionOptimizeCallbacksReady)
			g.Expect(condition.Reason).To(Equal(v1.ReasonNoCallbacks))
		}, timeout, interval).Should(Succeed())
	})

	// The identity provider of the platform config holds the callback URLs of
	// the oidc mode, so a CamundaOptimize there is none of this resource's
	// business and gets no row.
	It("discovers no Optimize in the oidc mode", func() {
		s := newScenario()

		createOptimize(s.namespace, s.mc.Name)

		Eventually(func(g Gomega) {
			g.Expect(conditionOf(g, s.mc, v1.ConditionOptimizeCallbacksReady).Reason).To(
				Equal(string(component.Disabled)),
			)
		}, timeout, interval).Should(Succeed())

		Consistently(func(g Gomega) {
			g.Expect(readManagementCluster(g, s.mc).Status.Optimize).To(BeEmpty())
		}, "2s", interval).Should(Succeed())
	})

	It("reports ConnectionFailed while Keycloak refuses the administrator", func() {
		keycloak := startFakeKeycloak(withOptimizeClient())
		keycloak.refuse = true
		s := newScenario(withFakeKeycloak(keycloak))

		Eventually(func(g Gomega) {
			condition := conditionOf(g, s.mc, v1.ConditionOptimizeCallbacksReady)
			g.Expect(condition.Status).To(Equal(metav1.ConditionFalse))
			g.Expect(condition.Reason).To(Equal(v1.ReasonConnectionFailed))
		}, timeout, interval).Should(Succeed())
	})

	// The identity provider of the platform config holds the callback URLs of
	// the oidc mode, so the operator administers no client there.
	It("reports Disabled in the oidc mode", func() {
		s := newScenario()

		Eventually(func(g Gomega) {
			condition := conditionOf(g, s.mc, v1.ConditionOptimizeCallbacksReady)
			g.Expect(condition.Status).To(Equal(metav1.ConditionTrue))
			g.Expect(condition.Reason).To(Equal(string(component.Disabled)))
		}, timeout, interval).Should(Succeed())
	})
})

// createOptimize creates a CamundaOptimize at discoveredOptimizeURL that names
// contract, and registers its deletion.
func createOptimize(namespace, contract string) *v1.CamundaOptimize {
	GinkgoHelper()

	optimize := &v1.CamundaOptimize{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "optimize-" + utilrand.String(8),
			Namespace: namespace,
		},
		Spec: v1.CamundaOptimizeSpec{
			Version:           "8.9.4",
			ManagementAuthRef: contract,
			ExternalURL:       discoveredOptimizeURL,
			ClusterRef:        v1.ClusterRef{Name: "my-cluster"},
		},
	}
	Expect(k8sClient.Create(ctx, optimize)).To(Succeed())
	DeferCleanup(func() { _ = k8sClient.Delete(ctx, optimize) })

	return optimize
}

// identityEnv returns the rendered environment of the Management Identity
// container as a map from name to literal value.
func identityEnv(g Gomega, s scenario) map[string]string {
	var workload appsv1.Deployment
	key := client.ObjectKey{Namespace: s.namespace, Name: components.IdentityName(s.mc)}
	g.Expect(k8sClient.Get(ctx, key, &workload)).To(Succeed())
	g.Expect(workload.Spec.Template.Spec.Containers).NotTo(BeEmpty())

	env := map[string]string{}
	for _, entry := range workload.Spec.Template.Spec.Containers[0].Env {
		env[entry.Name] = entry.Value
	}

	return env
}

// withFakeKeycloak turns a scenario into the externalKeycloak mode against the
// fake Keycloak. The mode carries the administrator credentials in the spec,
// so the base URL of the administration API is the URL of the spec and no test
// hook is needed.
func withFakeKeycloak(keycloak *fakeKeycloak) func(f *fixture) {
	return func(f *fixture) {
		withFakeKeycloakOnly(keycloak)(f)
		f.mc.Spec.Optimize = &v1.ManagementOptimizeSpec{ExternalURL: specOptimizeURL}
	}
}

// withFakeKeycloakOnly is withFakeKeycloak without spec.optimize, for a
// management plane that serves whatever the CamundaOptimize resources name and
// nothing else.
func withFakeKeycloakOnly(keycloak *fakeKeycloak) func(f *fixture) {
	return func(f *fixture) {
		withExternalKeycloak(f)
		f.mc.Spec.IdentityProvider.ExternalKeycloak.URL = keycloak.url
	}
}

// fakeKeycloak is a Keycloak that serves the token endpoint and the Optimize
// client of one realm. It records the redirect URIs that the operator writes.
type fakeKeycloak struct {
	url string
	// refuse answers every administration call with 401, the way a Keycloak
	// that does not know the administrator does.
	refuse bool

	mu       sync.Mutex
	stored   keycloakadmin.Representation
	answered int
}

// withOptimizeClient returns the Optimize client that the fake Keycloak starts
// with, carrying the given redirect URIs.
func withOptimizeClient(redirectURIs ...string) keycloakadmin.Representation {
	entries := make([]any, 0, len(redirectURIs))
	for _, uri := range redirectURIs {
		entries = append(entries, uri)
	}

	return keycloakadmin.Representation{
		"id":           "6c4c0c5c",
		"clientId":     "optimize",
		"webOrigins":   []any{"+"},
		"redirectUris": entries,
	}
}

// startFakeKeycloak runs a fake Keycloak that holds stored as its Optimize
// client, or no client at all when stored is nil.
func startFakeKeycloak(stored keycloakadmin.Representation) *fakeKeycloak {
	GinkgoHelper()

	fake := &fakeKeycloak{stored: stored}
	server := httptest.NewServer(http.HandlerFunc(fake.serve))
	DeferCleanup(server.Close)
	fake.url = server.URL

	return fake
}

// redirectURIs returns the redirect URIs of the stored Optimize client.
func (f *fakeKeycloak) redirectURIs() []string {
	f.mu.Lock()
	defer f.mu.Unlock()

	return f.stored.RedirectURIs()
}

// requests returns how many requests the fake Keycloak has answered.
func (f *fakeKeycloak) requests() int {
	f.mu.Lock()
	defer f.mu.Unlock()

	return f.answered
}

func (f *fakeKeycloak) serve(w http.ResponseWriter, r *http.Request) {
	f.mu.Lock()
	f.answered++
	f.mu.Unlock()

	if strings.HasSuffix(r.URL.Path, "/protocol/openid-connect/token") {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"access_token":"an-access-token"}`))

		return
	}
	if f.refuse {
		w.WriteHeader(http.StatusUnauthorized)

		return
	}

	f.mu.Lock()
	defer f.mu.Unlock()

	w.Header().Set("Content-Type", "application/json")
	if r.Method == http.MethodGet {
		found := []keycloakadmin.Representation{}
		if f.stored != nil {
			found = append(found, f.stored)
		}
		_ = json.NewEncoder(w).Encode(found)

		return
	}

	var update keycloakadmin.Representation
	if err := json.NewDecoder(r.Body).Decode(&update); err != nil {
		w.WriteHeader(http.StatusBadRequest)

		return
	}
	f.stored = update
	w.WriteHeader(http.StatusNoContent)
}
