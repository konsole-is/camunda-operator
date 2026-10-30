//go:build e2e
// +build e2e

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

package e2e

import (
	"fmt"
	"os"
	"os/exec"
	"slices"
	"strings"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	pluginsv1alpha1 "github.com/konsole-is/camunda-operator/api/plugins/v1alpha1"
	v1 "github.com/konsole-is/camunda-operator/api/v1"
	"github.com/konsole-is/camunda-operator/conformance"
	"github.com/konsole-is/camunda-operator/internal/pluginstartup"
	"github.com/konsole-is/camunda-operator/test/utils"
)

const (
	// echoRegistration is the registration of the echo plugin pod. Its one
	// grant template gives GRANT-ADMISSION a Grant to refuse.
	echoRegistration = "e2e-echo"
	echoNamespace    = "builtin-registrations-echo"
	echoManifest     = "test/e2e/testdata/echo-plugin.yaml"
	fakePluginImage  = "example.com/camunda-operator-fakeplugin:v0.0.1"
	builtinPreset    = "builtin-registrations-es"
	builtinRelease   = "builtin-registrations-release"
	// The namespaces of the runs. Each run needs its own (conformance.Config).
	esRunNamespace     = "builtin-es"
	bucketRunNamespace = "builtin-bucket"
	echoRunNamespace   = "builtin-echo"
	orderRunNamespace  = "builtin-order"

	pluginRegistrationResource = "pluginregistrations.plugins.camunda.io"
	esPresetResource           = "elasticsearchclusterpresets.core.camunda.io"
)

// reqLogsSkip is the skip of REQ-LOGS on a built-in registration.
var reqLogsSkip = conformance.Skip{
	ID:     conformance.ReqLogs,
	Reason: "the manager logs no JSON with a request key (#933)",
}

// builtinSkips are the config skips of a built-in run in the elasticsearch job.
var builtinSkips = []conformance.Skip{
	{
		ID: conformance.OrderRegistrationFirst,
		Reason: "it scales the operator manager to zero, so it runs in the flow builtin-registrations-order, " +
			"in a job of its own",
	},
	reqLogsSkip,
}

// builtinSelfSkips are the checks that skip themselves on a built-in
// registration: no built-in has grant templates or serves access.
var builtinSelfSkips = []conformance.CheckID{
	conformance.GrantMatch,
	conformance.GrantRequires,
	conformance.GrantAdmission,
	conformance.GrantReady,
	conformance.DeleteRoutes,
}

var _ = Describe("Built-in registrations", Ordered, Label(utils.LabelBuiltinRegistrations), func() {
	var runNamespace string

	BeforeAll(func() {
		By("creating the release and the Elasticsearch preset of the elasticsearch run")
		Expect(apply(&v1.CamundaRelease{
			TypeMeta:   metav1.TypeMeta{APIVersion: v1.GroupVersion.String(), Kind: "CamundaRelease"},
			ObjectMeta: metav1.ObjectMeta{Name: builtinRelease},
			Spec: v1.CamundaReleaseSpec{
				Version: os.Getenv(envCamundaVersion),
				Elasticsearch: &v1.ReleaseElasticsearchSpec{
					Version: os.Getenv(envElasticsearchVersion),
				},
			},
		})).To(Succeed())
		Expect(apply(&v1.ElasticsearchClusterPreset{
			TypeMeta:   metav1.TypeMeta{APIVersion: v1.GroupVersion.String(), Kind: "ElasticsearchClusterPreset"},
			ObjectMeta: metav1.ObjectMeta{Name: builtinPreset},
			Spec: v1.ElasticsearchClusterPresetSpec{
				Cluster: v1.ElasticsearchClusterSpec{
					Replicas:    new(int32(1)),
					StorageSize: new(resource.MustParse(esStorageSize)),
					Resources:   capped("500m", "1Gi", "1Gi"),
					ExtraEnv:    esHeapEnv(),
				},
			},
		})).To(Succeed())

		By("building the fake plugin image and loading it on Kind")
		_, err := utils.Run(exec.Command("make", "docker-build-fakeplugin", "FAKEPLUGIN_IMG="+fakePluginImage))
		Expect(err).NotTo(HaveOccurred(), "Failed to build the fake plugin image")
		Expect(utils.LoadImageToKindClusterWithName(fakePluginImage)).To(Succeed())

		By("deploying the echo plugin")
		_, err = utils.Kubectl("apply", "-f", echoManifest)
		Expect(err).NotTo(HaveOccurred())
		Eventually(func(g Gomega) {
			_, err := utils.Kubectl(
				"wait", "deployment/echo", "-n", echoNamespace, "--for=condition=Available", "--timeout=5s",
			)
			g.Expect(err).NotTo(HaveOccurred())
			expectCondition(
				g,
				pluginRegistrationResource,
				echoRegistration,
				"",
				pluginsv1alpha1.ConditionAccepted,
				v1.ReasonHealthy,
			)
			expectReady(g, pluginRegistrationResource, echoRegistration, "", v1.ReasonHealthy)
		}, 3*time.Minute, 5*time.Second).Should(Succeed())
	})

	AfterAll(func() {
		By("removing the echo plugin, its registration, the preset, and the release")
		_, _ = utils.Kubectl("delete", "-f", echoManifest, "--ignore-not-found", "--wait=false")
		_, _ = utils.Kubectl("delete", pluginRegistrationResource, echoRegistration, "--ignore-not-found")
		_, _ = utils.Kubectl("delete", esPresetResource, builtinPreset, "--ignore-not-found")
		_, _ = utils.Kubectl("delete", ccReleaseResource, builtinRelease, "--ignore-not-found")
	})

	// dumpDiagnostics writes the manager logs once. The echo plugin gets its
	// pod and its logs only.
	AfterEach(func() {
		dumpDiagnostics(runNamespace)
		if !CurrentSpecReport().Failed() {
			return
		}

		for name, args := range map[string][]string{
			"echo plugin pod":  {"describe", "pods", "-n", echoNamespace},
			"echo plugin logs": {"logs", "-n", echoNamespace, "deployment/echo", "--tail=300"},
		} {
			out, err := utils.Kubectl(args...)
			if err != nil {
				out = err.Error()
			}
			_, _ = fmt.Fprintf(GinkgoWriter, "%s:\n%s\n", name, out)
		}
	})

	It(
		"keeps the plugin contract with the elasticsearch registration",
		SpecTimeout(50*time.Minute),
		func(ctx SpecContext) {
			runNamespace = esRunNamespace
			prepareRunNamespace(runNamespace)
			cfg := conformance.Config{
				Registration: pluginstartup.RegistrationElasticsearch,
				Namespace:    runNamespace,
				Cases: []conformance.Case{{
					Name: "elasticsearch",
					Parameters: &apiextensionsv1.JSON{Raw: []byte(
						`{"preset":"` + builtinPreset + `","releaseRef":"` + builtinRelease + `"}`,
					)},
					Target: pluginsv1alpha1.TargetRef{Kind: "CamundaCluster", Name: "builtin-target"},
				}},
				// One Elasticsearch node for each deletion policy, image pull included.
				Timeouts: conformance.Timeouts{OutputsReady: esReadyTimeout, Ready: esReadyTimeout},
				Skip:     builtinSkips,
			}

			results, err := conformance.Execute(ctx, cfg)
			reportResults(cfg.Registration, results)
			Expect(err).NotTo(HaveOccurred())
			expectConformant(results, slices.Concat(skipIDs(builtinSkips), builtinSelfSkips)...)
		},
	)

	It(
		"keeps the plugin contract with the existing-object-storage registration",
		SpecTimeout(15*time.Minute),
		func(ctx SpecContext) {
			runNamespace = bucketRunNamespace
			prepareRunNamespace(runNamespace)
			createBackupStorage(runNamespace)
			cfg := conformance.Config{
				Registration: pluginstartup.RegistrationExistingObjectStorage,
				Namespace:    runNamespace,
				Cases:        []conformance.Case{existingObjectStorageCase()},
				Skip:         builtinSkips,
			}

			results, err := conformance.Execute(ctx, cfg)
			reportResults(cfg.Registration, results)
			Expect(err).NotTo(HaveOccurred())
			expectConformant(results, slices.Concat(skipIDs(builtinSkips), builtinSelfSkips)...)
		},
	)

	It(
		"refuses a Grant of the echo registration from a requester without the verb serve",
		SpecTimeout(20*time.Minute),
		func(ctx SpecContext) {
			runNamespace = echoRunNamespace
			prepareRunNamespace(runNamespace)
			cfg := conformance.Config{
				Registration: echoRegistration,
				Namespace:    runNamespace,
				Cases: []conformance.Case{{
					Name:       "echo",
					Parameters: &apiextensionsv1.JSON{Raw: []byte(`{"message":"hi"}`)},
					Target:     pluginsv1alpha1.TargetRef{Kind: "CamundaCluster", Name: "builtin-target"},
				}},
			}

			results, err := conformance.Execute(ctx, cfg)
			reportResults(cfg.Registration, results)
			Expect(err).NotTo(HaveOccurred())
			// The echo template has no required input, and echo serves no access.
			expectConformant(results, conformance.GrantRequires, conformance.DeleteRoutes)
			Expect(outcomeOf(results, conformance.GrantAdmission)).To(
				Equal(conformance.OutcomePassed),
				"GRANT-ADMISSION must run against the installed policy and see the refusal",
			)
			Expect(outcomeOf(results, conformance.RegRBAC)).To(Equal(conformance.OutcomePassed))
		},
	)
})

var _ = Describe("Built-in registration order", Ordered, Label(utils.LabelBuiltinRegistrationsOrder), func() {
	AfterEach(func() {
		dumpDiagnostics(orderRunNamespace)
	})

	It(
		"writes the existing-object-storage registration again before it serves a request",
		SpecTimeout(20*time.Minute),
		func(ctx SpecContext) {
			prepareRunNamespace(orderRunNamespace)
			createBackupStorage(orderRunNamespace)
			cfg := conformance.Config{
				Registration: pluginstartup.RegistrationExistingObjectStorage,
				Namespace:    orderRunNamespace,
				Cases:        []conformance.Case{existingObjectStorageCase()},
				Skip:         []conformance.Skip{reqLogsSkip},
			}

			results, err := conformance.Execute(ctx, cfg)
			reportResults(cfg.Registration, results)
			Expect(err).NotTo(HaveOccurred())
			expectConformant(results, slices.Concat([]conformance.CheckID{reqLogsSkip.ID}, builtinSelfSkips)...)
			Expect(outcomeOf(results, conformance.OrderRegistrationFirst)).To(Equal(conformance.OutcomePassed))

			// The check scales the manager back. A broken scale back fails here,
			// not in a later flow.
			By("checking that the manager runs again")
			Eventually(func(g Gomega) {
				out, err := utils.Kubectl(
					"get", "deployment", "-n", namespace, "-l", "control-plane=controller-manager",
					"-o", "jsonpath={.items[0].status.availableReplicas}",
				)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(out).To(Equal("1"))
			}, 2*time.Minute).Should(Succeed())
		},
	)
})

// prepareRunNamespace creates the namespace of a conformance run, and deletes
// it when the spec ends.
func prepareRunNamespace(ns string) {
	_, err := utils.Kubectl("create", "ns", ns)
	Expect(err).NotTo(HaveOccurred())
	DeferCleanup(func() {
		_, _ = utils.Kubectl("delete", "ns", ns, "--wait=false")
	})
}

// existingObjectStorageCase is the case of a run against the
// existing-object-storage registration. It names the ObjectStorageConfig that
// createBackupStorage writes in the run namespace.
func existingObjectStorageCase() conformance.Case {
	return conformance.Case{
		Name:       "bucket",
		Parameters: &apiextensionsv1.JSON{Raw: []byte(`{"name":"` + backupStorage + `"}`)},
		Target:     pluginsv1alpha1.TargetRef{Kind: "CamundaCluster", Name: "builtin-target"},
	}
}

// reportResults writes one line for each result of the run of registration,
// so the CI log holds the message of every check.
func reportResults(registration string, results []conformance.Result) {
	for _, r := range results {
		_, _ = fmt.Fprintf(
			GinkgoWriter,
			"conformance %s %s: %s (%s) %s\n",
			registration,
			r.ID,
			r.Outcome,
			r.Duration.Round(time.Second),
			r.Message,
		)
	}
}

// expectConformant asserts that results holds one result for each registered
// check, skipped when its ID is in skipped and passed otherwise. It fails once
// with every mismatch.
func expectConformant(results []conformance.Result, skipped ...conformance.CheckID) {
	GinkgoHelper()

	Expect(results).To(HaveLen(len(conformance.Checks())), "one result for each registered check")

	var mismatches []string
	for _, r := range results {
		want := conformance.OutcomePassed
		if slices.Contains(skipped, r.ID) {
			want = conformance.OutcomeSkipped
		}
		if r.Outcome != want {
			mismatches = append(mismatches, fmt.Sprintf("%s: %s, want %s: %s", r.ID, r.Outcome, want, r.Message))
		}
	}
	Expect(mismatches).To(BeEmpty(), strings.Join(mismatches, "\n"))
}

func outcomeOf(results []conformance.Result, id conformance.CheckID) conformance.Outcome {
	for _, r := range results {
		if r.ID == id {
			return r.Outcome
		}
	}

	return ""
}

func skipIDs(skips []conformance.Skip) []conformance.CheckID {
	ids := make([]conformance.CheckID, 0, len(skips))
	for _, s := range skips {
		ids = append(ids, s.ID)
	}

	return ids
}
