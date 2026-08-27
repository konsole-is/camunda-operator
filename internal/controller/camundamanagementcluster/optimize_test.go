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

// The two Optimize addresses of this file. Each one belongs to a
// CamundaOptimize that names the contract of the management plane.
const (
	blueOptimizeURL  = "https://optimize.blue.example.com"
	greenOptimizeURL = "https://optimize.green.example.com"
)

var _ = Describe("CamundaManagementCluster controller and the Optimize instances behind it", func() {
	It("discovers a CamundaOptimize that names its contract", func() {
		keycloak := startFakeKeycloak(withOptimizeClient())
		s := newScenario(withFakeKeycloak(keycloak))

		createOptimize(s.namespace, s.mc.Name, blueOptimizeURL)

		Eventually(func(g Gomega) {
			rows := readManagementCluster(g, s.mc).Status.Optimize
			g.Expect(rows).To(HaveLen(1))
			g.Expect(rows[0].Namespace).To(Equal(s.namespace))
			g.Expect(rows[0].ExternalURL).To(Equal(blueOptimizeURL))
		}, timeout, interval).Should(Succeed())

		Eventually(func(g Gomega) {
			g.Expect(identityEnv(g, s)["KEYCLOAK_INIT_OPTIMIZE_ROOT_URL"]).To(
				Equal(blueOptimizeURL),
			)
		}, timeout, interval).Should(Succeed())
	})

	It("withdraws a CamundaOptimize that is deleted", func() {
		keycloak := startFakeKeycloak(withOptimizeClient())
		s := newScenario(withFakeKeycloak(keycloak))

		optimize := createOptimize(s.namespace, s.mc.Name, blueOptimizeURL)

		Eventually(func(g Gomega) {
			g.Expect(readManagementCluster(g, s.mc).Status.Optimize).To(HaveLen(1))
		}, timeout, interval).Should(Succeed())

		Expect(k8sClient.Delete(ctx, optimize)).To(Succeed())

		Eventually(func(g Gomega) {
			g.Expect(readManagementCluster(g, s.mc).Status.Optimize).To(BeEmpty())
			g.Expect(identityEnv(g, s)).NotTo(HaveKey("KEYCLOAK_INIT_OPTIMIZE_ROOT_URL"))
		}, timeout, interval).Should(Succeed())
	})

	It("adds the missing callback of a second Optimize", func() {
		keycloak := startFakeKeycloak(withOptimizeClient(
			blueOptimizeURL + components.OptimizeCallbackPath,
		))
		s := newScenario(withFakeKeycloak(keycloak))

		createOptimize(s.namespace, s.mc.Name, blueOptimizeURL)
		createOptimize(s.namespace, s.mc.Name, greenOptimizeURL)

		Eventually(func(g Gomega) {
			stampIdentityReady(g, s)

			g.Expect(keycloak.redirectURIs()).To(ConsistOf(
				blueOptimizeURL+components.OptimizeCallbackPath,
				greenOptimizeURL+components.OptimizeCallbackPath,
			))

			condition := conditionOf(g, s.mc, v1.ConditionOptimizeCallbacksReady)
			g.Expect(condition.Status).To(Equal(metav1.ConditionTrue))
			g.Expect(condition.Reason).To(Equal(v1.ReasonHealthy))
		}, timeout, interval).Should(Succeed())
	})

	// A redirect URI that a person registered by hand does not end in the
	// login path of Optimize, so the operator leaves it where it is.
	It("keeps a redirect URI that this operator does not own", func() {
		keycloak := startFakeKeycloak(withOptimizeClient("https://legacy.example.com/*"))
		s := newScenario(withFakeKeycloak(keycloak))

		createOptimize(s.namespace, s.mc.Name, blueOptimizeURL)

		Eventually(func(g Gomega) {
			stampIdentityReady(g, s)

			g.Expect(keycloak.redirectURIs()).To(Equal([]string{
				"https://legacy.example.com/*",
				blueOptimizeURL + components.OptimizeCallbackPath,
			}))
		}, timeout, interval).Should(Succeed())
	})

	// The realm is bootstrapped by Management Identity, so the workload has to
	// be up before a missing client is the fault worth reporting. Ready only
	// takes the callback reason once every component is True.
	It("reports OptimizeClientMissing while the realm holds no Optimize client", func() {
		keycloak := startFakeKeycloak(nil)
		s := newScenario(withFakeKeycloak(keycloak))

		createOptimize(s.namespace, s.mc.Name, blueOptimizeURL)

		Eventually(func(g Gomega) {
			stampIdentityReady(g, s)

			condition := conditionOf(g, s.mc, v1.ConditionOptimizeCallbacksReady)
			g.Expect(condition.Status).To(Equal(metav1.ConditionFalse))
			g.Expect(condition.Reason).To(Equal(v1.ReasonOptimizeClientMissing))

			ready := conditionOf(g, s.mc, v1.ConditionReady)
			g.Expect(ready.Status).To(Equal(metav1.ConditionFalse))
			g.Expect(ready.Reason).To(Equal(v1.ReasonOptimizeClientMissing))
		}, timeout, interval).Should(Succeed())
	})

	It("reports NoCallbacks while no Optimize names a URL", func() {
		keycloak := startFakeKeycloak(withOptimizeClient())
		s := newScenario(withFakeKeycloak(keycloak))

		Eventually(func(g Gomega) {
			stampIdentityReady(g, s)

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

	// A plane that serves no Optimize renders no preset, so Management
	// Identity never creates the client. That absence is the resting state and
	// not a fault to retry.
	It("reports NoCallbacks while the realm holds no Optimize client either", func() {
		keycloak := startFakeKeycloak(nil)
		s := newScenario(withFakeKeycloak(keycloak))

		Eventually(func(g Gomega) {
			stampIdentityReady(g, s)

			condition := conditionOf(g, s.mc, v1.ConditionOptimizeCallbacksReady)
			g.Expect(condition.Status).To(Equal(metav1.ConditionTrue))
			g.Expect(condition.Reason).To(Equal(v1.ReasonNoCallbacks))
		}, timeout, interval).Should(Succeed())

		at := keycloak.requests()
		Consistently(func(g Gomega) {
			g.Expect(keycloak.requests()).To(Equal(at))
		}, "2s", interval).Should(Succeed())
	})

	It("reports WriteFailed while Keycloak refuses the change to the client", func() {
		keycloak := startFakeKeycloak(withOptimizeClient())
		keycloak.refuseUpdate = true
		s := newScenario(withFakeKeycloak(keycloak))

		createOptimize(s.namespace, s.mc.Name, blueOptimizeURL)

		Eventually(func(g Gomega) {
			stampIdentityReady(g, s)

			condition := conditionOf(g, s.mc, v1.ConditionOptimizeCallbacksReady)
			g.Expect(condition.Status).To(Equal(metav1.ConditionFalse))
			g.Expect(condition.Reason).To(Equal(v1.ReasonWriteFailed))
		}, timeout, interval).Should(Succeed())

		Expect(keycloak.redirectURIs()).To(BeEmpty())
	})

	// A suspended management plane is in its desired state, and the Keycloak
	// it runs is scaled to zero, so the realm is left alone.
	It("leaves the realm alone while the management cluster is suspended", func() {
		keycloak := startFakeKeycloak(withOptimizeClient())
		s := newScenario(withFakeKeycloak(keycloak))

		createOptimize(s.namespace, s.mc.Name, blueOptimizeURL)

		Eventually(func(g Gomega) {
			latest := readManagementCluster(g, s.mc)
			latest.Spec.Suspend = true
			g.Expect(k8sClient.Update(ctx, latest)).To(Succeed())
		}, timeout, interval).Should(Succeed())

		Eventually(func(g Gomega) {
			condition := conditionOf(g, s.mc, v1.ConditionOptimizeCallbacksReady)
			g.Expect(condition.Status).To(Equal(metav1.ConditionTrue))
			g.Expect(condition.Reason).To(Equal(string(component.Suspended)))
		}, timeout, interval).Should(Succeed())

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
		s := newScenario(withFakeKeycloak(keycloak))

		optimize := createOptimize(s.namespace, s.mc.Name, blueOptimizeURL)

		Eventually(func(g Gomega) {
			stampIdentityReady(g, s)

			g.Expect(keycloak.redirectURIs()).To(Equal([]string{
				blueOptimizeURL + components.OptimizeCallbackPath,
			}))
		}, timeout, interval).Should(Succeed())

		Expect(k8sClient.Delete(ctx, optimize)).To(Succeed())

		Eventually(func(g Gomega) {
			stampIdentityReady(g, s)

			g.Expect(keycloak.redirectURIs()).To(BeEmpty())

			condition := conditionOf(g, s.mc, v1.ConditionOptimizeCallbacksReady)
			g.Expect(condition.Reason).To(Equal(v1.ReasonNoCallbacks))
		}, timeout, interval).Should(Succeed())
	})

	// Management Identity writes the whole client while it starts, and this
	// step writes the same object back, so the two must not overlap.
	It("waits for Management Identity before it touches the client", func() {
		keycloak := startFakeKeycloak(withOptimizeClient())
		s := newScenario(withFakeKeycloak(keycloak))

		createOptimize(s.namespace, s.mc.Name, blueOptimizeURL)

		Eventually(func(g Gomega) {
			condition := conditionOf(g, s.mc, v1.ConditionOptimizeCallbacksReady)
			g.Expect(condition.Status).To(Equal(metav1.ConditionFalse))
			g.Expect(condition.Reason).To(Equal(string(component.PrerequisiteNotMet)))
		}, timeout, interval).Should(Succeed())

		Expect(keycloak.redirectURIs()).To(BeEmpty())

		// The component that is still starting is the cause, so Ready names it
		// and not the realm it has not bootstrapped yet.
		Expect(conditionOf(Default, s.mc, v1.ConditionReady).Reason).NotTo(
			Equal(string(component.PrerequisiteNotMet)),
		)

		Eventually(func(g Gomega) {
			stampIdentityReady(g, s)

			g.Expect(keycloak.redirectURIs()).To(Equal([]string{
				blueOptimizeURL + components.OptimizeCallbackPath,
			}))
		}, timeout, interval).Should(Succeed())
	})

	// A Keycloak that the user runs outlives this resource, so a callback left
	// behind would point at an Optimize that is gone.
	It("withdraws the callbacks when the management cluster is deleted", func() {
		keycloak := startFakeKeycloak(withOptimizeClient())
		s := newScenario(withFakeKeycloak(keycloak))

		createOptimize(s.namespace, s.mc.Name, blueOptimizeURL)

		Eventually(func(g Gomega) {
			stampIdentityReady(g, s)

			g.Expect(keycloak.redirectURIs()).To(Equal([]string{
				blueOptimizeURL + components.OptimizeCallbackPath,
			}))
		}, timeout, interval).Should(Succeed())

		Expect(deleteManagementCluster(s.mc)).To(Succeed())

		Eventually(func(g Gomega) {
			g.Expect(keycloak.redirectURIs()).To(BeEmpty())
		}, timeout, interval).Should(Succeed())
	})

	// Two planes can share one external Keycloak realm. A plane parked on a
	// name another owner holds never served those Optimize instances, so its
	// deletion must not take the holder's callbacks with it.
	It("leaves the realm alone when a plane that owns no contract is deleted", func() {
		keycloak := startFakeKeycloak(withOptimizeClient(
			blueOptimizeURL + components.OptimizeCallbackPath,
		))
		name := "mac-" + utilrand.String(8)
		createForeignContract(name, map[string]string{"camunda.io/management-cluster": "elsewhere"})

		s := newScenario(withFakeKeycloak(keycloak), func(f *fixture) {
			f.mc.Spec.ManagementAuthConfigName = name
		})

		Eventually(func(g Gomega) {
			g.Expect(conditionOf(g, s.mc, v1.ConditionReady).Reason).To(Equal(v1.ReasonConflict))
		}, timeout, interval).Should(Succeed())

		Expect(deleteManagementCluster(s.mc)).To(Succeed())

		Expect(keycloak.redirectURIs()).To(Equal([]string{
			blueOptimizeURL + components.OptimizeCallbackPath,
		}))
	})

	// A plane that never wrote a contract never served an Optimize behind it,
	// so an absent contract is no licence to clear the realm.
	It("leaves the realm alone when a plane with no contract is deleted", func() {
		keycloak := startFakeKeycloak(withOptimizeClient(
			blueOptimizeURL + components.OptimizeCallbackPath,
		))
		s := newScenario(withFakeKeycloak(keycloak), func(f *fixture) {
			// A dangling DatabaseConfig fails the pre-checks, so the reconcile
			// stops before it ever writes the contract.
			f.mc.Spec.Identity.DatabaseConfigRef = "dbc-does-not-exist"
		})

		Eventually(func(g Gomega) {
			g.Expect(conditionOf(g, s.mc, v1.ConditionReady).Reason).To(
				Equal(v1.ReasonInvalidReference),
			)
		}, timeout, interval).Should(Succeed())

		Expect(deleteManagementCluster(s.mc)).To(Succeed())

		Expect(keycloak.redirectURIs()).To(Equal([]string{
			blueOptimizeURL + components.OptimizeCallbackPath,
		}))
	})

	// A plane that does not hold the contract name does not know which
	// Optimize instances are its own, so it writes nothing to the realm.
	It("touches the realm only once the contract is written", func() {
		keycloak := startFakeKeycloak(withOptimizeClient())
		name := "mac-" + utilrand.String(8)
		createForeignContract(name, map[string]string{"camunda.io/management-cluster": "elsewhere"})

		s := newScenario(withFakeKeycloak(keycloak), func(f *fixture) {
			f.mc.Spec.ManagementAuthConfigName = name
		})
		createOptimize(s.namespace, s.mc.Name, blueOptimizeURL)

		Eventually(func(g Gomega) {
			g.Expect(conditionOf(g, s.mc, v1.ConditionReady).Reason).To(Equal(v1.ReasonConflict))
		}, timeout, interval).Should(Succeed())

		Consistently(func(g Gomega) {
			g.Expect(keycloak.redirectURIs()).To(BeEmpty())
		}, "2s", interval).Should(Succeed())
	})

	// The identity provider of the platform config holds the callback URLs of
	// the oidc mode, so a CamundaOptimize there is none of this resource's
	// business and gets no row.
	It("discovers no Optimize in the oidc mode", func() {
		s := newScenario()

		createOptimize(s.namespace, s.mc.Name, blueOptimizeURL)

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

		createOptimize(s.namespace, s.mc.Name, blueOptimizeURL)

		Eventually(func(g Gomega) {
			stampIdentityReady(g, s)

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

// createOptimize creates a CamundaOptimize at externalURL that names contract,
// and registers its deletion.
func createOptimize(namespace, contract, externalURL string) *v1.CamundaOptimize {
	GinkgoHelper()

	optimize := &v1.CamundaOptimize{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "optimize-" + utilrand.String(8),
			Namespace: namespace,
		},
		Spec: v1.CamundaOptimizeSpec{
			Version:           "8.9.4",
			ManagementAuthRef: contract,
			ExternalURL:       externalURL,
			ClusterRef:        v1.ClusterRef{Name: "my-cluster"},
		},
	}
	Expect(k8sClient.Create(ctx, optimize)).To(Succeed())
	DeferCleanup(func() { _ = k8sClient.Delete(ctx, optimize) })

	return optimize
}

// stampIdentityReady reports the Management Identity Deployment as ready. The
// callback step waits for it, because Management Identity owns the Optimize
// client while it starts. A re-render moves the generation, so every polling
// loop stamps again.
func stampIdentityReady(g Gomega, s scenario) {
	stampDeploymentReady(
		g, client.ObjectKey{Namespace: s.namespace, Name: components.IdentityName(s.mc)},
	)
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
	// refuseUpdate answers the client update with 403, the way a Keycloak does
	// for an administrator that cannot change a client of the realm.
	refuseUpdate bool

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

	if f.refuseUpdate {
		w.WriteHeader(http.StatusForbidden)

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
