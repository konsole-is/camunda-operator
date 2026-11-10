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
	"maps"
	"os"
	"slices"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	fleetv1alpha1 "github.com/konsole-is/camunda-operator/api/fleet/v1alpha1"
	v1 "github.com/konsole-is/camunda-operator/api/v1"
	"github.com/konsole-is/camunda-operator/internal/pluginstartup"
	"github.com/konsole-is/camunda-operator/test/utils"
)

const (
	// clusterClaimNamespace holds the claim, the bucket contract that it
	// names, and everything that the claim creates.
	clusterClaimNamespace = "cluster-claim-e2e"
	clusterClaimName      = "orders"
	// The cluster-scoped objects of the flow. The Environment gives the
	// release, the platform config, and the Elasticsearch preset, so the
	// claim names only the cluster preset and the bucket contract.
	clusterClaimEnvironment = "cluster-claim-e2e"
	clusterClaimPreset      = "cluster-claim-e2e"
	clusterClaimESPreset    = "cluster-claim-e2e-es"
	clusterClaimRelease     = "cluster-claim-e2e"
	clusterClaimPlatform    = "cluster-claim-e2e"

	clusterClaimResource = "camundaclusterclaims.fleet.camunda.io"

	// clusterClaimReadyTimeout covers the Elasticsearch node, whose output
	// the cluster waits for, and then the start of the cluster.
	clusterClaimReadyTimeout = esReadyTimeout + ccReadyTimeout
	// clusterClaimDeleteTimeout covers the deletion in its stages: the
	// cluster first, then the requests, whose adapter deletes the
	// Elasticsearch cluster.
	clusterClaimDeleteTimeout = 10 * time.Minute
)

var _ = Describe("CamundaClusterClaim", Ordered, Label(utils.LabelClusterClaim), func() {
	BeforeAll(func() {
		By("creating the namespace of the claim in the Environment")
		Expect(apply(&corev1.Namespace{
			TypeMeta: metav1.TypeMeta{APIVersion: "v1", Kind: "Namespace"},
			ObjectMeta: metav1.ObjectMeta{
				Name:   clusterClaimNamespace,
				Labels: map[string]string{fleetv1alpha1.LabelEnvironment: clusterClaimEnvironment},
			},
		})).To(Succeed())

		By("creating the presets, the release, and the platform config")
		Expect(apply(&v1.CamundaClusterPreset{
			TypeMeta:   metav1.TypeMeta{APIVersion: v1.GroupVersion.String(), Kind: "CamundaClusterPreset"},
			ObjectMeta: metav1.ObjectMeta{Name: clusterClaimPreset},
			Spec: v1.CamundaClusterPresetSpec{Cluster: v1.CamundaClusterSpec{
				Zeebe: &v1.ZeebeSpec{
					WorkloadSpec: v1.WorkloadSpec{Resources: requests("1", "1.5Gi")},
					StorageSize:  new(resource.MustParse("1Gi")),
				},
				Gateway: &v1.GatewaySpec{WorkloadSpec: v1.WorkloadSpec{Resources: requests("500m", "512Mi")}},
			}},
		})).To(Succeed())
		Expect(apply(&v1.ElasticsearchClusterPreset{
			TypeMeta:   metav1.TypeMeta{APIVersion: v1.GroupVersion.String(), Kind: "ElasticsearchClusterPreset"},
			ObjectMeta: metav1.ObjectMeta{Name: clusterClaimESPreset},
			Spec: v1.ElasticsearchClusterPresetSpec{
				Cluster: v1.ElasticsearchClusterSpec{
					Replicas:    new(int32(1)),
					StorageSize: new(resource.MustParse(esStorageSize)),
					Resources:   capped("500m", "1Gi", "1Gi"),
					ExtraEnv:    esHeapEnv(),
				},
			},
		})).To(Succeed())
		Expect(apply(&v1.CamundaRelease{
			TypeMeta:   metav1.TypeMeta{APIVersion: v1.GroupVersion.String(), Kind: "CamundaRelease"},
			ObjectMeta: metav1.ObjectMeta{Name: clusterClaimRelease},
			Spec: v1.CamundaReleaseSpec{
				Version:       os.Getenv(envCamundaVersion),
				Elasticsearch: &v1.ReleaseElasticsearchSpec{Version: os.Getenv(envElasticsearchVersion)},
			},
		})).To(Succeed())
		Expect(apply(basicPlatform(clusterClaimPlatform))).To(Succeed())

		By("creating the bucket contract that the claim names")
		createBackupStorage(clusterClaimNamespace)

		By("creating the Environment and waiting until it finds every object it names")
		Expect(apply(clusterClaimEnvironmentObject())).To(Succeed())
		Eventually(func(g Gomega) {
			expectReady(g, environmentResource, clusterClaimEnvironment, "", v1.ReasonHealthy)
		}, 2*time.Minute, 5*time.Second).Should(Succeed())
	})

	AfterAll(func() {
		By("removing the claim, the Environment, and the cluster-scoped resources")
		_, _ = utils.Kubectl(
			"delete", clusterClaimResource, clusterClaimName,
			"-n", clusterClaimNamespace, "--ignore-not-found", "--wait=false",
		)
		_, _ = utils.Kubectl("delete", environmentResource, clusterClaimEnvironment, "--ignore-not-found")
		_, _ = utils.Kubectl("delete", ccPresetResource, clusterClaimPreset, "--ignore-not-found")
		_, _ = utils.Kubectl("delete", esPresetResource, clusterClaimESPreset, "--ignore-not-found")
		_, _ = utils.Kubectl("delete", ccReleaseResource, clusterClaimRelease, "--ignore-not-found")
		_, _ = utils.Kubectl("delete", ccPlatformResource, clusterClaimPlatform, "--ignore-not-found")
		_, _ = utils.Kubectl("delete", "ns", clusterClaimNamespace, "--ignore-not-found", "--wait=false")
	})

	AfterEach(func() {
		dumpDiagnostics(clusterClaimNamespace)
	})

	It("runs a CamundaCluster on the built-in Elasticsearch and object storage options", func() {
		By("creating the claim")
		Expect(apply(clusterClaimObject())).To(Succeed())

		By("waiting for the claim, its CamundaCluster, and its Elasticsearch cluster to report Healthy")
		elasticsearch := clusterClaimName + pluginstartup.BackingSuffixElasticsearch
		Eventually(func(g Gomega) {
			expectReady(g, esResource, elasticsearch, clusterClaimNamespace, v1.ReasonHealthy)
			expectReady(g, ccResource, clusterClaimName, clusterClaimNamespace, v1.ReasonHealthy)
			expectReady(g, clusterClaimResource, clusterClaimName, clusterClaimNamespace, v1.ReasonHealthy)
		}, clusterClaimReadyTimeout, 10*time.Second).Should(Succeed())

		By("wiring the outputs of the requests and the defaults of the Environment into the cluster")
		var storage string
		Eventually(func(g Gomega) {
			requests := claimRequests(g, clusterClaimResource, clusterClaimName, clusterClaimNamespace)
			g.Expect(requests).To(HaveLen(2))
			storage = requestOutput(
				g,
				requests[pluginstartup.SectionSecondaryStorage],
				clusterClaimNamespace,
				pluginstartup.OutputSecondaryStorageConfig,
			)
			g.Expect(storage).NotTo(BeEmpty())
		}, time.Minute, 5*time.Second).Should(Succeed())

		var cluster v1.CamundaCluster
		Expect(utils.Get(ccResource, clusterClaimName, clusterClaimNamespace, &cluster)).To(Succeed())
		Expect(cluster.Labels).To(HaveKeyWithValue(fleetv1alpha1.LabelClaim, clusterClaimName))
		Expect(cluster.Spec.StorageRef).To(Equal(storage))
		Expect(cluster.Spec.BackupStorageRef).To(Equal(backupStorage))
		Expect(cluster.Spec.ReleaseRef).To(Equal(clusterClaimRelease))
		Expect(cluster.Spec.PlatformConfigRef).To(Equal(clusterClaimPlatform))
	})

	It("deletes the cluster, the Elasticsearch cluster, and the requests with the claim", func() {
		var names []string
		Eventually(func(g Gomega) {
			requests := claimRequests(g, clusterClaimResource, clusterClaimName, clusterClaimNamespace)
			g.Expect(requests).To(HaveLen(2))
			names = slices.Collect(maps.Values(requests))
		}, time.Minute, 5*time.Second).Should(Succeed())

		By("deleting the claim")
		_, err := utils.Kubectl(
			"delete", clusterClaimResource, clusterClaimName, "-n", clusterClaimNamespace, "--wait=false",
		)
		Expect(err).NotTo(HaveOccurred())

		By("waiting until the claim and everything it created are gone")
		Eventually(func(g Gomega) {
			expectGone(g, ccResource, clusterClaimName, clusterClaimNamespace)
			expectGone(g, esResource, clusterClaimName+pluginstartup.BackingSuffixElasticsearch, clusterClaimNamespace)
			for _, name := range names {
				expectGone(g, pluginRequestResource, name, clusterClaimNamespace)
			}
			expectGone(g, clusterClaimResource, clusterClaimName, clusterClaimNamespace)
		}, clusterClaimDeleteTimeout, 5*time.Second).Should(Succeed())

		By("keeping the bucket contract that the claim named")
		exists, err := utils.Exists(oscResource, backupStorage, clusterClaimNamespace)
		Expect(err).NotTo(HaveOccurred())
		Expect(exists).To(BeTrue(), "the claim deleted an ObjectStorageConfig that it did not create")
	})
})

// clusterClaimEnvironmentObject returns the Environment of the claim
// namespace. The claim must select Elasticsearch for secondary storage, and
// can select the existing bucket contract for its backups and name it.
func clusterClaimEnvironmentObject() *fleetv1alpha1.Environment {
	return &fleetv1alpha1.Environment{
		TypeMeta: metav1.TypeMeta{
			APIVersion: fleetv1alpha1.GroupVersion.String(),
			Kind:       "Environment",
		},
		ObjectMeta: metav1.ObjectMeta{Name: clusterClaimEnvironment},
		Spec: fleetv1alpha1.EnvironmentSpec{
			PlatformConfigRef: clusterClaimPlatform,
			Presets: fleetv1alpha1.EnvironmentPresets{
				CamundaCluster:       []string{clusterClaimPreset},
				ElasticsearchCluster: []string{clusterClaimESPreset},
			},
			Releases: &fleetv1alpha1.EnvironmentReleases{
				Allowed: []string{clusterClaimRelease},
				Default: clusterClaimRelease,
			},
			Sections: map[string]fleetv1alpha1.SectionPolicy{
				pluginstartup.SectionSecondaryStorage: {
					Required: true,
					Default:  pluginstartup.RegistrationElasticsearch,
					Allowed:  []string{pluginstartup.RegistrationElasticsearch},
				},
				pluginstartup.SectionBackupStorage: {
					Allowed: []string{pluginstartup.RegistrationExistingObjectStorage},
				},
			},
			Parameters: map[string]fleetv1alpha1.ParameterPolicies{
				pluginstartup.RegistrationElasticsearch: {
					"preset": {Value: &apiextensionsv1.JSON{Raw: []byte(`"` + clusterClaimESPreset + `"`)}},
				},
				pluginstartup.RegistrationExistingObjectStorage: {
					"name": {ClaimEditable: true},
				},
			},
		},
	}
}

// clusterClaimObject returns the claim of the flow. It names no release and
// no platform config, so both come from the Environment.
func clusterClaimObject() *fleetv1alpha1.CamundaClusterClaim {
	return &fleetv1alpha1.CamundaClusterClaim{
		TypeMeta: metav1.TypeMeta{
			APIVersion: fleetv1alpha1.GroupVersion.String(),
			Kind:       "CamundaClusterClaim",
		},
		ObjectMeta: metav1.ObjectMeta{Name: clusterClaimName, Namespace: clusterClaimNamespace},
		Spec: fleetv1alpha1.CamundaClusterClaimSpec{
			PresetRef: clusterClaimPreset,
			Plugins: []fleetv1alpha1.PluginSelection{
				{Section: pluginstartup.SectionSecondaryStorage, Registration: pluginstartup.RegistrationElasticsearch},
				{
					Section:      pluginstartup.SectionBackupStorage,
					Registration: pluginstartup.RegistrationExistingObjectStorage,
					Parameters:   &apiextensionsv1.JSON{Raw: []byte(`{"name":"` + backupStorage + `"}`)},
				},
			},
		},
	}
}
