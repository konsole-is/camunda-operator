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
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	k8slabels "k8s.io/apimachinery/pkg/labels"

	v1 "github.com/konsole-is/camunda-operator/api/v1"
	"github.com/konsole-is/camunda-operator/pkg/labels"
	"github.com/konsole-is/camunda-operator/test/utils"
)

const (
	esNamespace = "elasticsearch-e2e"
	esName      = "camunda-es"
	// esStorageConfig is the SecondaryStorageConfig that the cluster publishes.
	esStorageConfig = "camunda-es-storage"
	// esVersion is a Camunda 8.9 supported Elasticsearch release that ECK
	// runs.
	esVersion = "9.2.4"
	// esStorageSize is small: kind binds it from the local-path provisioner.
	esStorageSize = "1Gi"
	// esIndex holds the document that must survive suspend and resume. It has
	// no replicas, so a single node stays green.
	esIndex    = "documents"
	esDocument = `{"message":"survives suspension"}`
	// esResource and eckResource are the kubectl resource names of the CR
	// under test and of the ECK CR it renders.
	esResource  = "elasticsearchclusters.core.camunda.io"
	eckResource = "elasticsearches.elasticsearch.k8s.elastic.co"
	// esReadyTimeout covers the Elasticsearch image pull and the cluster
	// bootstrap on a fresh kind node.
	esReadyTimeout = 15 * time.Minute
)

// dataVolumeSelector selects the data volumes of the cluster through the
// discovery labels that the operator puts on the ECK volume claim template.
var dataVolumeSelector = k8slabels.SelectorFromSet(
	labels.Discovery(labels.ElasticsearchCluster(esName), "elasticsearch"),
).String()

var _ = Describe("ElasticsearchCluster", Ordered, func() {
	var (
		cluster = &v1.ElasticsearchCluster{
			TypeMeta:   metav1.TypeMeta{APIVersion: v1.GroupVersion.String(), Kind: "ElasticsearchCluster"},
			ObjectMeta: metav1.ObjectMeta{Name: esName, Namespace: esNamespace},
			Spec: v1.ElasticsearchClusterSpec{
				Version:     esVersion,
				Replicas:    new(int32(1)),
				StorageSize: new(resource.MustParse(esStorageSize)),
				Resources: &corev1.ResourceRequirements{
					Requests: corev1.ResourceList{
						corev1.ResourceCPU:    resource.MustParse("500m"),
						corev1.ResourceMemory: resource.MustParse("1Gi"),
					},
					Limits: corev1.ResourceList{corev1.ResourceMemory: resource.MustParse("1Gi")},
				},
				SecondaryStorageConfig: esStorageConfig,
				Monitoring: &v1.MonitoringSpec{
					ServiceMonitor: &v1.ServiceMonitorSpec{Enabled: true},
				},
			},
		}
		// dataVolume is the data PersistentVolumeClaim of the single node,
		// as it was bound before suspension.
		dataVolume corev1.PersistentVolumeClaim
	)

	BeforeAll(func() {
		By("creating the test namespace")
		_, err := utils.Kubectl("create", "ns", esNamespace)
		Expect(err).NotTo(HaveOccurred())
	})

	AfterAll(func() {
		By("removing the test namespace")
		_, _ = utils.Kubectl("delete", "ns", esNamespace, "--wait=false")
	})

	AfterEach(func() {
		dumpDiagnostics(esNamespace)
	})

	It("reaches Ready Healthy and reports the data volume size", func() {
		By("creating the ElasticsearchCluster")
		Expect(apply(cluster)).To(Succeed())

		By("waiting for Ready Healthy")
		Eventually(func(g Gomega) {
			expectReady(g, esResource, esName, esNamespace, v1.ReasonHealthy)
		}, esReadyTimeout, 5*time.Second).Should(Succeed())

		By("reading status.storageSize")
		var got v1.ElasticsearchCluster
		Expect(utils.Get(esResource, esName, esNamespace, &got)).To(Succeed())
		Expect(got.Status.StorageSize).NotTo(BeNil())
		Expect(got.Status.StorageSize.String()).To(Equal(esStorageSize))
	})

	It("publishes a SecondaryStorageConfig whose credentials authenticate over HTTPS", func() {
		var contract v1.SecondaryStorageConfig
		Expect(utils.Get(sscResource, esStorageConfig, esNamespace, &contract)).To(Succeed())
		Expect(contract.Spec.Type).To(Equal(v1.SecondaryStorageTypeElasticsearch))
		Expect(contract.Spec.Elasticsearch).NotTo(BeNil())
		Expect(contract.Spec.Elasticsearch.Endpoint).To(HavePrefix("https://"))
		Expect(contract.Spec.Elasticsearch.CASecretRef).NotTo(BeNil())

		By("waiting for the contract to validate its references")
		Eventually(func(g Gomega) {
			expectReady(
				g, sscResource, esStorageConfig, esNamespace,
				v1.ReasonHealthy,
			)
		}, 2*time.Minute).Should(Succeed())

		By("reading the cluster health with the published credentials and CA")
		out, err := curlElasticsearch(&contract, "health", "/_cluster/health")
		Expect(err).NotTo(HaveOccurred())
		Expect(out).To(ContainSubstring(`"status":"green"`))

		By("indexing a document")
		_, err = curlElasticsearch(
			&contract, "index", "/"+esIndex,
			"-X", "PUT", "-H", "Content-Type: application/json", "-d", `{"settings":{"number_of_replicas":0}}`,
		)
		Expect(err).NotTo(HaveOccurred())
		_, err = curlElasticsearch(
			&contract, "document", "/"+esIndex+"/_doc/1?refresh=true",
			"-X", "PUT", "-H", "Content-Type: application/json", "-d", esDocument,
		)
		Expect(err).NotTo(HaveOccurred())
	})

	It("serves Elasticsearch metrics through the exporter", func() {
		By("waiting for the metrics component")
		Eventually(func(g Gomega) {
			cond, err := utils.Condition(esResource, esName, esNamespace, "MetricsReady")
			g.Expect(err).NotTo(HaveOccurred())
			g.Expect(cond).NotTo(BeNil())
			g.Expect(cond.Status).To(Equal(metav1.ConditionTrue), cond.Message)
		}, 5*time.Minute).Should(Succeed())

		By("scraping the exporter Service")
		Eventually(func(g Gomega) {
			out, err := utils.RunPod(&corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{Name: "curl-exporter", Namespace: esNamespace},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{{
						Name:  "curl",
						Image: utils.CurlImage,
						Args: []string{
							"-fsS", fmt.Sprintf("http://%s-es-metrics.%s.svc:9114/metrics", esName, esNamespace),
						},
					}},
				},
			}, podTimeout)
			g.Expect(err).NotTo(HaveOccurred())
			g.Expect(out).To(MatchRegexp(`elasticsearch_cluster_health_status\{[^}]*color="green"[^}]*\} 1`))
		}, 3*time.Minute).Should(Succeed())
	})

	It("suspends by deleting the Elasticsearch resource and keeping the data volumes", func() {
		By("recording the bound data volume")
		var claims corev1.PersistentVolumeClaimList
		Expect(utils.List("pvc", esNamespace, dataVolumeSelector, &claims)).To(Succeed())
		Expect(claims.Items).To(HaveLen(1))
		Expect(claims.Items[0].Status.Phase).To(Equal(corev1.ClaimBound))
		dataVolume = claims.Items[0]

		By("setting spec.suspend")
		_, err := utils.Kubectl(
			"patch", esResource, esName, "-n", esNamespace,
			"--type=merge", "-p", `{"spec":{"suspend":true}}`,
		)
		Expect(err).NotTo(HaveOccurred())

		By("waiting for Ready Suspended and the Elasticsearch resource to be gone")
		Eventually(func(g Gomega) {
			expectReady(g, esResource, esName, esNamespace, "Suspended")
			expectGone(g, eckResource, esName, esNamespace)
		}, 5*time.Minute).Should(Succeed())

		By("checking that the data volume stays bound")
		var claim corev1.PersistentVolumeClaim
		Expect(utils.Get("pvc", dataVolume.Name, esNamespace, &claim)).To(Succeed())
		Expect(claim.UID).To(Equal(dataVolume.UID))
		Expect(claim.Status.Phase).To(Equal(corev1.ClaimBound))
	})

	It("resumes against the same data volume with the data intact", func() {
		By("clearing spec.suspend")
		_, err := utils.Kubectl(
			"patch", esResource, esName, "-n", esNamespace,
			"--type=merge", "-p", `{"spec":{"suspend":false}}`,
		)
		Expect(err).NotTo(HaveOccurred())

		By("waiting for Ready Healthy")
		Eventually(func(g Gomega) {
			expectReady(g, esResource, esName, esNamespace, v1.ReasonHealthy)
		}, esReadyTimeout, 5*time.Second).Should(Succeed())

		By("checking that the node runs on the same data volume")
		var claim corev1.PersistentVolumeClaim
		Expect(utils.Get("pvc", dataVolume.Name, esNamespace, &claim)).To(Succeed())
		Expect(claim.UID).To(Equal(dataVolume.UID))

		By("reading the document that was indexed before suspension")
		var contract v1.SecondaryStorageConfig
		Expect(utils.Get(sscResource, esStorageConfig, esNamespace, &contract)).To(Succeed())
		Eventually(func(g Gomega) {
			out, err := curlElasticsearch(&contract, "document", "/"+esIndex+"/_doc/1")
			g.Expect(err).NotTo(HaveOccurred())
			g.Expect(out).To(ContainSubstring(`"message":"survives suspension"`))
		}, 3*time.Minute).Should(Succeed())
	})

	It("garbage-collects its bindings and, by default, the data volume on deletion", func() {
		_, err := utils.Kubectl("delete", esResource, esName, "-n", esNamespace, "--wait=false")
		Expect(err).NotTo(HaveOccurred())

		By("waiting for the owned resources to be gone")
		Eventually(func(g Gomega) {
			expectGone(g, esResource, esName, esNamespace)
			expectGone(g, eckResource, esName, esNamespace)
			expectGone(g, sscResource, esStorageConfig, esNamespace)
			expectGone(g, "secret", esName+"-es-user", esNamespace)
			expectGone(g, "deployment", esName+"-es-exporter", esNamespace)
			expectGone(g, "service", esName+"-es-metrics", esNamespace)
		}, 5*time.Minute).Should(Succeed())

		By("waiting for ECK to remove the data volume")
		Eventually(func(g Gomega) {
			expectGone(g, "pvc", dataVolume.Name, esNamespace)
		}, 3*time.Minute).Should(Succeed())
	})
})

// curlElasticsearch runs curl in the namespace of contract against path on
// the endpoint that contract publishes. It authenticates with the credentials
// Secret and trusts the CA Secret that the contract references, so it proves
// the contract usable the way a consumer uses it. extra are further curl
// arguments. It returns the response body.
func curlElasticsearch(contract *v1.SecondaryStorageConfig, name, path string, extra ...string) (string, error) {
	es := contract.Spec.Elasticsearch
	env := []corev1.EnvVar{
		utils.SecretEnv("ES_USERNAME", es.CredentialsSecretRef.Name, es.CredentialsSecretRef.UsernameKey),
		utils.SecretEnv("ES_PASSWORD", es.CredentialsSecretRef.Name, es.CredentialsSecretRef.PasswordKey),
		utils.SecretEnv("ES_CA", es.CASecretRef.Name, es.CASecretRef.Key),
	}

	// $0 is "curl", the remaining arguments are the curl arguments.
	script := `printf '%s' "$ES_CA" > /tmp/ca.crt && ` +
		`exec curl -fsS --cacert /tmp/ca.crt -u "$ES_USERNAME:$ES_PASSWORD" "$@"`
	args := append([]string{"-ec", script, "curl", es.Endpoint + path}, extra...)

	return utils.RunPod(&corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: "curl-" + name, Namespace: contract.Namespace},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{{
				Name:    "curl",
				Image:   utils.CurlImage,
				Command: []string{"sh"},
				Args:    args,
				Env:     env,
			}},
		},
	}, podTimeout)
}
