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
	"net"
	"strconv"
	"strings"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/sourcehawk/operator-component-framework/pkg/component"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	utilrand "k8s.io/apimachinery/pkg/util/rand"
	"sigs.k8s.io/controller-runtime/pkg/client"

	v1 "github.com/konsole-is/camunda-operator/api/v1"
	components "github.com/konsole-is/camunda-operator/pkg/components/camundacluster"
	"github.com/konsole-is/camunda-operator/test/utils"
)

// podTimeout bounds one in-cluster helper pod, image pull included.
const podTimeout = 3 * time.Minute

// suspendTimeout bounds the wait for a cluster to scale to zero, and for a
// suspended cluster to run again.
const suspendTimeout = 5 * time.Minute

// maxHealthDumps caps how many pods dumpNotReadyPodHealth reads. Each one
// costs a helper pod, and a namespace whose storage is gone holds every pod
// not ready at once.
const maxHealthDumps = 3

// actuatorHealthPath is the Spring Boot health endpoint. Every group of a
// process, such as /actuator/health/readiness, sits under it, and the path
// itself reports every indicator the process has.
const actuatorHealthPath = "/actuator/health"

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

// expectConditionFalse asserts that resource name reports condition condType
// as False with reason. It is written for Eventually and for Consistently: a
// hold that the operator must keep is read the same way both times.
func expectConditionFalse(g Gomega, resource, name, namespace, condType, reason string) {
	cond, err := utils.Condition(resource, name, namespace, condType)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(cond).NotTo(BeNil(), "%s %q has no %s condition yet", resource, name, condType)
	g.Expect(cond.Status).To(Equal(metav1.ConditionFalse), "%s %q %s: %s", resource, name, condType, cond.Message)
	g.Expect(cond.Reason).To(Equal(reason), "%s %q %s: %s", resource, name, condType, cond.Message)
}

// expectPhase asserts that resource name reports status.phase. It is written
// for Eventually and for Consistently.
func expectPhase(g Gomega, resource, name, namespace, phase string) {
	var obj struct {
		Status struct {
			Phase string `json:"phase"`
		} `json:"status"`
	}
	g.Expect(utils.Get(resource, name, namespace, &obj)).To(Succeed())
	g.Expect(obj.Status.Phase).To(Equal(phase), "%s %q reports another phase", resource, name)
}

// expectReconciledReady asserts that resource name reports Ready as True with
// reason Healthy for the generation it currently carries. It is written for
// Eventually.
//
// expectReady alone accepts a Ready condition that the operator wrote before
// the last edit of the spec. A spec that edits a resource and then waits for it
// therefore passes on the answer to the previous question. Comparing
// status.observedGeneration with metadata.generation is what makes the wait
// mean "reconciled since my edit".
func expectReconciledReady(g Gomega, resource, name, namespace string) {
	var obj struct {
		Metadata struct {
			Generation int64 `json:"generation"`
		} `json:"metadata"`
		Status struct {
			ObservedGeneration int64 `json:"observedGeneration"`
		} `json:"status"`
	}
	g.Expect(utils.Get(resource, name, namespace, &obj)).To(Succeed())
	g.Expect(obj.Status.ObservedGeneration).To(
		Equal(obj.Metadata.Generation),
		"%s %q has not reconciled generation %d yet", resource, name, obj.Metadata.Generation,
	)

	expectReady(g, resource, name, namespace, v1.ReasonHealthy)
}

// expectGone asserts that resource name no longer exists. It is written for
// Eventually.
func expectGone(g Gomega, resource, name, namespace string) {
	exists, err := utils.Exists(resource, name, namespace)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(exists).To(BeFalse(), "%s %q still exists", resource, name)
}

// suspend sets spec.suspend on the cluster and waits until it reports Ready
// Suspended with every workload of the cluster at zero replicas. A restore
// rewrites the storage of its cluster, so no workload of the cluster runs
// while the restore does. Connectors is checked when the cluster enables it.
func suspend(cluster *v1.CamundaCluster) {
	By("setting spec.suspend on the CamundaCluster")
	_, err := utils.Kubectl(
		"patch", ccResource, cluster.Name, "-n", cluster.Namespace,
		"--type=merge", "-p", `{"spec":{"suspend":true}}`,
	)
	Expect(err).NotTo(HaveOccurred())

	By("waiting for Ready Suspended and the workloads at zero replicas")
	Eventually(func(g Gomega) {
		expectReady(g, ccResource, cluster.Name, cluster.Namespace, string(component.Suspended))
		expectScaledToZero(
			g,
			"statefulset",
			components.WorkloadName(cluster, components.ComponentZeebe),
			cluster.Namespace,
		)
		expectScaledToZero(
			g,
			"deployment",
			components.WorkloadName(cluster, components.ComponentGateway),
			cluster.Namespace,
		)
		connectors := cluster.Spec.Connectors
		if connectors == nil || connectors.Enabled == nil || !*connectors.Enabled {
			return
		}

		expectScaledToZero(
			g,
			"deployment",
			components.WorkloadName(cluster, components.ComponentConnectors),
			cluster.Namespace,
		)
	}, suspendTimeout, 5*time.Second).Should(Succeed())
}

// unsuspend clears spec.suspend and waits for the cluster to report Ready
// Healthy again.
//
// A resume starts every process of the cluster in the same second, so this
// wait is one of the two places where connectors races the gateway. When it
// times out on "connectors: Waiting for replicas: 0/1 ready", read the
// "actuator health of ..." block of the connectors pod; the resume spec of
// CamundaCluster carries the full reading of it (issue #144).
func unsuspend(cluster *v1.CamundaCluster) {
	By("clearing spec.suspend on the CamundaCluster")
	_, err := utils.Kubectl(
		"patch", ccResource, cluster.Name, "-n", cluster.Namespace,
		"--type=merge", "-p", `{"spec":{"suspend":false}}`,
	)
	Expect(err).NotTo(HaveOccurred())

	By("waiting for Ready Healthy")
	Eventually(func(g Gomega) {
		expectReady(g, ccResource, cluster.Name, cluster.Namespace, v1.ReasonHealthy)
	}, ccReadyTimeout, 5*time.Second).Should(Succeed())
}

// letTheRestoreTakeOver removes spec.suspend and does not wait for the cluster
// to be healthy again. A spec that erased the state of the cluster by hand
// calls it before it creates the restore, so the restore is what suspends the
// cluster and what unsuspends it afterwards. That is the flow a user gets: the
// only thing they do is create the restore.
//
// The field is removed rather than set to false, so that nothing owns it when
// the restore applies it. The restore withdraws its suspension by applying an
// object without the field, and server-side apply keeps a field that another
// manager still declares. A caller that set false would leave the value of
// that caller behind the withdrawal, which is not the flow this proves.
//
// The patch fails when the field is absent, which is the state that says the
// spec did not suspend the cluster first.
func letTheRestoreTakeOver(cluster *v1.CamundaCluster) {
	By("removing spec.suspend so the restore suspends the cluster itself")
	_, err := utils.Kubectl(
		"patch", ccResource, cluster.Name, "-n", cluster.Namespace,
		"--type=json", "-p", `[{"op":"remove","path":"/spec/suspend"}]`,
	)
	Expect(err).NotTo(HaveOccurred())
}

// expectUnsuspended waits until the restore withdrew its suspension and the
// cluster reports Ready Healthy again. Nobody clears spec.suspend by hand.
func expectUnsuspended(cluster *v1.CamundaCluster) {
	By("waiting for the restore to unsuspend the cluster")
	Eventually(func(g Gomega) {
		var current v1.CamundaCluster
		g.Expect(utils.Get(ccResource, cluster.Name, cluster.Namespace, &current)).To(Succeed())
		g.Expect(current.Spec.Suspend).To(BeFalse(), "the restore left the cluster suspended")
	}, 2*time.Minute, 5*time.Second).Should(Succeed())

	By("waiting for Ready Healthy")
	Eventually(func(g Gomega) {
		expectReady(g, ccResource, cluster.Name, cluster.Namespace, v1.ReasonHealthy)
	}, ccReadyTimeout, 5*time.Second).Should(Succeed())
}

// brokerClaims returns the data volume claims of the brokers of the cluster,
// by name. The identity of a claim is its UID and its creation time, so a
// caller that holds the answer can tell a claim that survived from one that
// was created again under the same name.
func brokerClaims(cluster *v1.CamundaCluster) map[string]corev1.PersistentVolumeClaim {
	var list corev1.PersistentVolumeClaimList
	Expect(utils.List("pvc", cluster.Namespace, brokerClaimSelector(cluster), &list)).To(Succeed())

	claims := make(map[string]corev1.PersistentVolumeClaim, len(list.Items))
	for _, claim := range list.Items {
		claims[claim.Name] = claim
	}

	return claims
}

// psql runs sql against the logical database of the flow in namespace, as the
// user whose credentials Secret ref names. It returns the unaligned,
// tuples-only output.
//
// The pod name carries a random suffix. RunPod deletes its pod at the end and
// swallows the error of that delete, so a fixed name makes the next create
// fail with AlreadyExists.
func psql(namespace, name string, ref v1.CredentialsSecretRef, sql string) (string, error) {
	return utils.RunPod(&corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "psql-" + name + "-" + utilrand.String(5),
			Namespace: namespace,
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{{
				Name:    "psql",
				Image:   "postgres:17",
				Command: []string{"psql", "-v", "ON_ERROR_STOP=1", "-tA", "-c", sql},
				Env: []corev1.EnvVar{
					{Name: "PGHOST", Value: postgresService},
					{Name: "PGDATABASE", Value: dbDatabaseName},
					utils.SecretEnv("PGUSER", ref.Name, ref.UsernameKey),
					utils.SecretEnv("PGPASSWORD", ref.Name, ref.PasswordKey),
				},
			}},
		},
	}, podTimeout)
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
				"camundaclusters,camundamanagementclusters,camundaoptimizes," +
				"logicalbackupelasticsearches,logicalbackuprdbmses,backupschedules," +
				"logicalrestoreelasticsearches,logicalrestorerdbmses,pointintimerestores",
			"-n", testNamespace,
		},
		"object storage contracts": {"get", "objectstorageconfigs", "-o", "wide"},
		// The Management Identity contract is cluster-scoped, so it never
		// appears in the resource table of a namespace. YAML rather than wide:
		// the kind has no print columns, and its Ready reason is what tells a
		// CamundaOptimize with a dangling reference from one with a Secret
		// that lacks the key.
		"management identity contracts": {"get", "managementauthconfigs", "-o", "yaml"},
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
		// The Keycloak Operator owns this resource the same way, and its
		// status is the only record of why a Keycloak did not come up. It is
		// absent when the suite skipped the Keycloak Operator.
		"keycloak of the Keycloak Operator": {
			"get", "keycloaks.k8s.keycloak.org", "-n", testNamespace, "-o", "yaml",
		},
		"pods": {"describe", "pods", "-n", testNamespace},
		"workload logs": {
			"logs", "-l", "camunda.io/cluster", "-n", testNamespace, "--all-containers", "--prefix", "--tail=200",
		},
		// The workloads of a management plane carry the owner label of their
		// own kind, so the selector above reaches none of them.
		"management plane logs": {
			"logs", "-l", "camunda.io/management-cluster", "-n", testNamespace,
			"--all-containers", "--prefix", "--tail=200",
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
	dumpNotReadyPodHealth(testNamespace)
	dumpRestartedPodLogs(testNamespace)
	dumpRestartedPodLogs(namespace)
}

// dumpNotReadyPodHealth writes the actuator health document of every pod of
// namespace that holds a container which is not ready.
//
// A pod that stays not ready explains itself nowhere else. The pod state says
// only that the readiness probe answered 503, and no log line of the process
// names the indicator that answered it. The health document is the one record
// of which indicator is down. Connectors 8.9.7 wrote no application log at
// all, and that is what first made this dump necessary. 8.9.8 fixed the log,
// and the dump keeps its value because it reports what no log line carries.
//
// The distinction it makes is the one that matters for connectors. Its
// readiness group holds zeebeClient and processDefinitionImport, and the
// document reports each of them on its own line. zeebeClient down means the
// gateway does not answer. processDefinitionImport down means the inbound
// import of the process definitions over the REST API of the gateway does not
// complete. Either one narrows the search without ending it: the address of
// the gateway and the admin credentials both come from this operator.
//
// It reads the endpoint from a curl pod, over the pod IP. Two other routes do
// not work here. The pod proxy of the API server is one call, but kubectl
// discards the body of an answer that is not 2xx, and this endpoint answers
// 503 in exactly the case worth reading. A Service is the other, and a pod
// that is not ready has no endpoint behind one.
func dumpNotReadyPodHealth(namespace string) {
	var pods corev1.PodList
	if err := utils.List("pods", namespace, "", &pods); err != nil {
		_, _ = fmt.Fprintf(GinkgoWriter, "Failed to list the pods of %s: %s\n", namespace, err)
		return
	}

	probed := 0
	for i := range pods.Items {
		pod := &pods.Items[i]

		port := actuatorHealthPort(pod)
		if port == 0 || pod.Status.PodIP == "" {
			continue
		}

		if probed == maxHealthDumps {
			_, _ = fmt.Fprintf(
				GinkgoWriter, "More pods are not ready; stopped after %d health dumps\n", probed,
			)
			return
		}
		probed++

		res, err := utils.CamundaREST(utils.CamundaRequest{
			Namespace: namespace,
			Name:      "health",
			Method:    "GET",
			URL: "http://" + net.JoinHostPort(
				pod.Status.PodIP, strconv.Itoa(int(port)),
			) + actuatorHealthPath,
			Timeout: podTimeout,
		})
		if err != nil {
			_, _ = fmt.Fprintf(
				GinkgoWriter, "Failed to get the actuator health of %s: %s\n", pod.Name, err,
			)
			continue
		}
		_, _ = fmt.Fprintf(
			GinkgoWriter, "actuator health of %s (%d):\n%s\n", pod.Name, res.Status, res.Body,
		)
	}
}

// actuatorHealthPort returns the port that serves /actuator/health on the
// first container of pod which is not ready. It returns zero when every
// container is ready, and when no probe of that container reads an actuator
// health endpoint.
//
// It reads the port off a probe rather than off a constant, because the port
// differs by process: the connectors runtime serves the actuator on its HTTP
// port, the unified binary and Optimize on their management port.
//
// The readiness probe alone does not answer the question. Optimize reads
// /api/readyz on its HTTP port and serves the actuator on its management port,
// so its readiness port would point the dump at the wrong endpoint. Only a
// probe whose own path is an actuator health endpoint names the right port.
func actuatorHealthPort(pod *corev1.Pod) int32 {
	for _, status := range pod.Status.ContainerStatuses {
		if status.Ready {
			continue
		}

		for _, container := range pod.Spec.Containers {
			if container.Name != status.Name {
				continue
			}

			probes := []*corev1.Probe{
				container.ReadinessProbe, container.LivenessProbe, container.StartupProbe,
			}
			for _, p := range probes {
				if p == nil || p.HTTPGet == nil ||
					!strings.HasPrefix(p.HTTPGet.Path, actuatorHealthPath) {
					continue
				}

				return containerPortNumber(container, p.HTTPGet.Port)
			}
		}
	}

	return 0
}

// containerPortNumber resolves port against the ports of container. A probe
// gives the name of a port as often as its number, and the pod proxy of the
// API server takes a number.
func containerPortNumber(container corev1.Container, port intstr.IntOrString) int32 {
	if port.Type == intstr.Int {
		return port.IntVal
	}

	for _, declared := range container.Ports {
		if declared.Name == port.StrVal {
			return declared.ContainerPort
		}
	}

	return 0
}

// customResourceKinds are the namespaced custom resources of this operator,
// in the order a reader follows a failure: the storage backends first, then
// the cluster, then what attaches to it, then the backups of it, and last the
// restores that read them.
var customResourceKinds = []string{
	"elasticsearchclusters",
	"secondarystorageconfigs",
	"databaseconfigs",
	"camundaclusters",
	"camundamanagementclusters",
	"camundaoptimizes",
	"logicalbackupelasticsearches",
	"logicalbackuprdbmses",
	"backupschedules",
	"logicalrestoreelasticsearches",
	"logicalrestorerdbmses",
	"pointintimerestores",
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
