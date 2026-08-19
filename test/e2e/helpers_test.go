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
	"encoding/json"
	"fmt"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	v1 "github.com/konsole-is/camunda-operator/api/v1"
	"github.com/konsole-is/camunda-operator/test/utils"
)

// podTimeout bounds one in-cluster helper pod, image pull included.
const podTimeout = 3 * time.Minute

// apply applies obj through kubectl. obj must carry its apiVersion and kind.
func apply(obj client.Object) error {
	manifest, err := json.Marshal(obj)
	if err != nil {
		return fmt.Errorf("encoding %T %q: %w", obj, obj.GetName(), err)
	}

	_, err = utils.KubectlWithStdin(string(manifest), "apply", "-f", "-")
	return err
}

// expectReady asserts that resource name reports Ready=True with reason. It
// is written for Eventually.
func expectReady(g Gomega, resource, name, namespace, reason string) {
	expectCondition(g, resource, name, namespace, v1.ConditionReady, reason)
}

// expectCondition asserts that resource name reports condition condType as
// True with reason. It is written for Eventually.
func expectCondition(g Gomega, resource, name, namespace, condType, reason string) {
	cond, err := utils.Condition(resource, name, namespace, condType)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(cond).NotTo(BeNil(), "%s %q has no %s condition yet", resource, name, condType)
	g.Expect(cond.Status).To(Equal(metav1.ConditionTrue), "%s %q %s: %s", resource, name, condType, cond.Message)
	g.Expect(cond.Reason).To(Equal(reason), "%s %q %s: %s", resource, name, condType, cond.Message)
}

// expectGone asserts that resource name no longer exists. It is written for
// Eventually.
func expectGone(g Gomega, resource, name, namespace string) {
	exists, err := utils.Exists(resource, name, namespace)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(exists).To(BeFalse(), "%s %q still exists", resource, name)
}

// dumpDiagnostics writes the controller-manager logs and the events,
// resources, pod descriptions, and CamundaCluster workload logs of
// testNamespace to the Ginkgo writer when the current spec failed.
//
// It dumps the logs of the previous container of every restarted pod too. A
// pod in CrashLoopBackOff runs no container while it waits, so a plain log
// call returns nothing and the crash explains itself nowhere. The previous
// instance holds the only record of why it exited.
//
// Three entries answer questions that earlier failures left open. The
// EndpointSlices say whether a Service had a programmed endpoint when a
// client got connection refused on its ClusterIP. The ECK Elasticsearch
// resource carries the view of the other operator, its own health and phase
// included. The custom resources come out as YAML, so every condition
// arrives with its lastTransitionTime, which dates a stale message.
func dumpDiagnostics(testNamespace string) {
	if !CurrentSpecReport().Failed() {
		return
	}

	for name, args := range map[string][]string{
		"controller-manager logs": {
			"logs", "-l", "control-plane=controller-manager", "-n", namespace, "--tail=-1",
		},
		"events": {"get", "events", "-n", testNamespace, "--sort-by=.lastTimestamp"},
		"resources": {
			"get",
			"all,pvc,secrets,elasticsearchclusters,databases,databaseconfigs,secondarystorageconfigs," +
				"camundaclusters,logicalbackupelasticsearches,logicalbackuprdbmses,backupschedules",
			"-n", testNamespace,
		},
		"object storage contracts": {"get", "objectstorageconfigs", "-o", "wide"},
		// A Service answers on its ClusterIP only once kube-proxy programmed
		// an endpoint for it. A ready pod is not enough, so the slices are
		// the only record of whether a refused connection had a target. YAML
		// rather than wide: the ready condition of an endpoint decides
		// whether kube-proxy programmed it, and the wide columns show only
		// the addresses.
		"endpoint slices": {"get", "endpointslices", "-n", testNamespace, "-o", "yaml"},
		// ECK owns this resource and the operator only reads it. Its status
		// says whether ECK reached Elasticsearch, which no resource of this
		// operator reports. It is absent when the suite skipped ECK.
		"elasticsearch of ECK": {
			"get", "elasticsearches.elasticsearch.k8s.elastic.co", "-n", testNamespace, "-o", "yaml",
		},
		"pods": {"describe", "pods", "-n", testNamespace},
		"workload logs": {
			"logs", "-l", "camunda.io/cluster", "-n", testNamespace, "--all-containers", "--prefix", "--tail=200",
		},
	} {
		out, err := utils.Kubectl(args...)
		if err != nil {
			_, _ = fmt.Fprintf(GinkgoWriter, "Failed to get %s: %s\n", name, err)
			continue
		}
		_, _ = fmt.Fprintf(GinkgoWriter, "%s:\n%s\n", name, out)
	}

	dumpCustomResources(testNamespace)
	dumpRestartedPodLogs(testNamespace)
	dumpRestartedPodLogs(namespace)
}

// customResourceKinds are the namespaced custom resources of this operator,
// in the order a reader follows a failure: the storage backends first, then
// the cluster, then the backups of it.
var customResourceKinds = []string{
	"elasticsearchclusters",
	"secondarystorageconfigs",
	"databaseconfigs",
	"camundaclusters",
	"logicalbackupelasticsearches",
	"logicalbackuprdbmses",
	"backupschedules",
}

// dumpCustomResources writes every custom resource of this operator in
// namespace as YAML. The resource table of dumpDiagnostics lists the same
// objects without their status, and YAML carries each condition with its
// reason, its message, and the time it last changed. A stale message dates
// itself that way.
//
// It reads one kind at a time. One call for every kind fails as a whole when
// a single kind is not installed, which would drop the status of the kinds
// that are.
func dumpCustomResources(namespace string) {
	for _, kind := range customResourceKinds {
		out, err := utils.Kubectl("get", kind, "-n", namespace, "-o", "yaml")
		if err != nil {
			_, _ = fmt.Fprintf(GinkgoWriter, "Failed to get %s of %s: %s\n", kind, namespace, err)
			continue
		}
		_, _ = fmt.Fprintf(GinkgoWriter, "%s of %s:\n%s\n", kind, namespace, out)
	}
}

// dumpRestartedPodLogs writes the logs of the previous container of every pod
// in namespace that restarted at least once. It reads the pods one by one:
// a call by label selector fails as a whole when one matched pod has no
// previous instance, which is the common case, because a healthy pod stands
// next to the crashing one.
func dumpRestartedPodLogs(namespace string) {
	var pods corev1.PodList
	if err := utils.List("pods", namespace, "", &pods); err != nil {
		_, _ = fmt.Fprintf(GinkgoWriter, "Failed to list the pods of %s: %s\n", namespace, err)
		return
	}

	for i := range pods.Items {
		pod := &pods.Items[i]
		for _, status := range pod.Status.ContainerStatuses {
			if status.RestartCount == 0 {
				continue
			}

			out, err := utils.Kubectl(
				"logs", pod.Name, "-n", namespace, "-c", status.Name, "--previous", "--tail=-1",
			)
			if err != nil {
				_, _ = fmt.Fprintf(
					GinkgoWriter, "Failed to get the previous logs of %s/%s container %s: %s\n",
					namespace, pod.Name, status.Name, err,
				)
				continue
			}
			_, _ = fmt.Fprintf(
				GinkgoWriter, "previous logs of %s/%s container %s (%d restarts):\n%s\n",
				namespace, pod.Name, status.Name, status.RestartCount, out,
			)
		}
	}
}
