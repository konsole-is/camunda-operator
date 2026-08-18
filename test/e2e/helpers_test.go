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
				"camundaclusters,logicalbackupelasticsearches,logicalbackuprdbmses",
			"-n", testNamespace,
		},
		"object storage contracts": {"get", "objectstorageconfigs", "-o", "wide"},
		"pods":                     {"describe", "pods", "-n", testNamespace},
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
}
