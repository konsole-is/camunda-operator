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

package camundaoptimize

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	utilrand "k8s.io/apimachinery/pkg/util/rand"

	v1 "github.com/konsole-is/camunda-operator/api/v1"
	"github.com/konsole-is/camunda-operator/internal/fixtures"
)

// minimalCamundaOptimize returns the minimal example of the CRD doc with a
// unique name in the schema test namespace.
func minimalCamundaOptimize() *v1.CamundaOptimize {
	return &v1.CamundaOptimize{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "co-" + utilrand.String(8),
			Namespace: fixtures.SchemaTestNamespace,
		},
		Spec: v1.CamundaOptimizeSpec{
			Version:           "8.9.4",
			ManagementAuthRef: "my-management-auth",
			ClusterRef:        v1.ClusterRef{Name: "my-cluster"},
		},
	}
}

// realisticCamundaOptimize returns the realistic example of the CRD doc, with
// a unique name in the schema test namespace.
func realisticCamundaOptimize() *v1.CamundaOptimize {
	optimize := minimalCamundaOptimize()
	optimize.Spec.Webapp = &v1.WorkloadSpec{
		Replicas: new(int32(2)),
		Resources: &corev1.ResourceRequirements{
			Requests: corev1.ResourceList{corev1.ResourceMemory: resource.MustParse("1Gi")},
		},
		ExtraEnv:       []corev1.EnvVar{{Name: "OPTIMIZE_JAVA_OPTS", Value: "-Xmx2048m"}},
		PodLabels:      map[string]string{"team": "platform"},
		PodAnnotations: map[string]string{"prometheus.io/scrape": "true"},
	}
	optimize.Spec.Importer = &v1.WorkloadSpec{
		Replicas: new(int32(1)),
		Resources: &corev1.ResourceRequirements{
			Requests: corev1.ResourceList{corev1.ResourceMemory: resource.MustParse("2Gi")},
		},
	}
	optimize.Spec.Monitoring = &v1.OptimizeMonitoringSpec{
		ServiceMonitor: &v1.ServiceMonitorSpec{
			Enabled: true,
			Labels:  map[string]string{"prometheus": "platform"},
		},
	}

	return optimize
}

var _ = Describe("CamundaOptimize schema", func() {
	It("accepts the documented examples", func() {
		for _, optimize := range []*v1.CamundaOptimize{minimalCamundaOptimize(), realisticCamundaOptimize()} {
			Expect(k8sClient.Create(ctx, optimize)).To(Succeed())
			DeferCleanup(func() { _ = k8sClient.Delete(ctx, optimize) })
		}
	})

	It("rejects an importer with more than one replica", func() {
		optimize := minimalCamundaOptimize()
		optimize.Spec.Importer = &v1.WorkloadSpec{Replicas: new(int32(2))}

		Expect(k8sClient.Create(ctx, optimize)).To(MatchError(ContainSubstring("importer.replicas must be 0 or 1")))
	})

	// Zero is the suspend value of every WorkloadSpec. It satisfies "at most
	// one active importer", and it is the only way to stop the import while a
	// restore or an index rewrite runs.
	It("accepts an importer with no replicas", func() {
		optimize := minimalCamundaOptimize()
		optimize.Spec.Importer = &v1.WorkloadSpec{Replicas: new(int32(0))}

		Expect(k8sClient.Create(ctx, optimize)).To(Succeed())
		DeferCleanup(func() { _ = k8sClient.Delete(ctx, optimize) })
	})

	It("rejects a missing version", func() {
		optimize := minimalCamundaOptimize()
		optimize.Spec.Version = ""

		Expect(k8sClient.Create(ctx, optimize)).To(MatchError(ContainSubstring("version")))
	})

	It("rejects a version that is not a full semantic version", func() {
		optimize := minimalCamundaOptimize()
		optimize.Spec.Version = "8.9"

		Expect(k8sClient.Create(ctx, optimize)).To(MatchError(ContainSubstring("version")))
	})

	It("rejects an empty managementAuthRef", func() {
		optimize := minimalCamundaOptimize()
		optimize.Spec.ManagementAuthRef = ""

		Expect(k8sClient.Create(ctx, optimize)).To(MatchError(ContainSubstring("managementAuthRef")))
	})

	It("rejects an empty clusterRef name", func() {
		optimize := minimalCamundaOptimize()
		optimize.Spec.ClusterRef.Name = ""

		Expect(k8sClient.Create(ctx, optimize)).To(MatchError(ContainSubstring("clusterRef")))
	})
})
