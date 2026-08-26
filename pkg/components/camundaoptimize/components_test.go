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
	"flag"
	"path/filepath"
	"testing"

	monitoringv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	"github.com/sourcehawk/operator-component-framework/pkg/component"
	"github.com/sourcehawk/operator-component-framework/pkg/testing/golden"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"

	v1 "github.com/konsole-is/camunda-operator/api/v1"
)

// updateGolden refreshes the golden manifests with the rendered output:
// go test ./pkg/components/camundaoptimize/ -run Golden -update-golden
var updateGolden = flag.Bool("update-golden", false, "update golden files")

// goldenScheme registers every type for which the golden serializer must
// resolve TypeMeta.
func goldenScheme(t *testing.T) *runtime.Scheme {
	t.Helper()

	scheme := runtime.NewScheme()
	require.NoError(t, clientgoscheme.AddToScheme(scheme))
	require.NoError(t, monitoringv1.AddToScheme(scheme))
	require.NoError(t, v1.AddToScheme(scheme))

	return scheme
}

// previewObjects returns the previewed objects of a component.
func previewObjects(t *testing.T, comp *component.Component) []client.Object {
	t.Helper()

	objects, err := comp.Preview()
	require.NoError(t, err)

	return objects
}

// previewedPodTemplate returns the pod template of the Deployment in the
// previewed objects.
func previewedPodTemplate(t *testing.T, objects []client.Object) corev1.PodTemplateSpec {
	t.Helper()

	for _, obj := range objects {
		if workload, ok := obj.(*appsv1.Deployment); ok {
			return workload.Spec.Template
		}
	}
	require.Fail(t, "no Deployment in the previewed objects")

	return corev1.PodTemplateSpec{}
}

// TestCamundaOptimizeGoldens pins every component of every fixture against
// testdata/golden/<fixture>/<component>.yaml.
func TestCamundaOptimizeGoldens(t *testing.T) {
	t.Parallel()

	for name, in := range goldenFixtures(t) {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			scheme := goldenScheme(t)
			base := filepath.Join("testdata", "golden", name)

			comps, err := Build(in)
			require.NoError(t, err)
			for _, comp := range comps {
				golden.AssertComponentYAML(
					t, filepath.Join(base, comp.GetName()+".yaml"), comp,
					golden.WithScheme(scheme), golden.Update(*updateGolden),
				)
			}

			// The mirrored Secrets component is gated off without a copy to
			// make, so it applies nothing and there is nothing to pin.
			mirrors := goldenMirrors(name)
			if len(mirrors) == 0 {
				return
			}
			mirrored, err := MirroredSecretComponent(in.Optimize, mirrors)
			require.NoError(t, err)
			golden.AssertComponentYAML(
				t, filepath.Join(base, mirroredComponentName+".yaml"), mirrored,
				golden.WithScheme(scheme), golden.Update(*updateGolden),
			)
		})
	}
}

// goldenMirrors returns the copied Secret data of a fixture: none for the
// minimal one, one copy per purpose for the realistic one.
func goldenMirrors(fixture string) map[MirrorPurpose]map[string][]byte {
	if fixture != "realistic" {
		return nil
	}

	return map[MirrorPurpose]map[string][]byte{
		MirrorPurposeLicense:    {"license": []byte("golden-license")},
		MirrorPurposeAuthClient: {"client-secret": []byte("golden-client-secret")},
	}
}

// Build returns the two components in reconcile order, each reporting its own
// condition.
func TestBuildOrderAndConditions(t *testing.T) {
	t.Parallel()

	in := fixtureMinimal(t)
	comps, err := Build(in)
	require.NoError(t, err)

	names := make([]string, 0, len(comps))
	conditions := make([]component.ConditionType, 0, len(comps))
	for _, comp := range comps {
		names = append(names, comp.GetName())
		conditions = append(conditions, comp.GetCondition(in.Optimize).ConditionType())
	}

	assert.Equal(t, []string{"optimize-webapp", "optimize-importer"}, names)
	assert.Equal(
		t,
		[]component.ConditionType{
			component.ConditionType(v1.ConditionWebappReady),
			component.ConditionType(v1.ConditionImporterReady),
		},
		conditions,
	)
}

// Every rendered resource carries the name of the referenced cluster, so an
// extension finds the Optimize workloads of a cluster the same way it finds
// the workloads of the cluster itself.
func TestResourcesCarryTheReferencedClusterLabel(t *testing.T) {
	t.Parallel()

	comps, err := Build(fixtureRealistic(t))
	require.NoError(t, err)

	seen := 0
	for _, comp := range comps {
		for _, obj := range previewObjects(t, comp) {
			seen++
			assert.Equal(t, "my-cluster", obj.GetLabels()["camunda.io/cluster"], obj.GetName())
			assert.Equal(t, comp.GetName(), obj.GetLabels()["camunda.io/component"], obj.GetName())
			assert.Equal(t, "camunda-operator", obj.GetLabels()["app.kubernetes.io/managed-by"], obj.GetName())
		}
	}
	assert.Equal(t, 6, seen, "two components with a Deployment, a Service, and a ServiceMonitor each")
}

// The workload names come from the CamundaOptimize, so two instances attached
// to one cluster do not collide.
func TestWorkloadNamesComeFromTheCustomResource(t *testing.T) {
	t.Parallel()

	comps, err := Build(fixtureMinimal(t))
	require.NoError(t, err)

	names := make([]string, 0, 2*len(comps))
	for _, comp := range comps {
		for _, obj := range previewObjects(t, comp) {
			names = append(names, obj.GetName())
		}
	}

	assert.Equal(
		t,
		[]string{"my-optimize-webapp", "my-optimize-webapp", "my-optimize-importer", "my-optimize-importer"},
		names,
	)
}

// The webapp scales with the spec. The importer takes 0 or 1, which CEL holds
// it to, and 0 is the suspend value that stops the import.
func TestImporterReplicasSuspendTheImport(t *testing.T) {
	t.Parallel()

	in := fixtureRealistic(t)
	in.Optimize.Spec.Importer.Replicas = new(int32(0))
	comps, err := Build(in)
	require.NoError(t, err)

	assert.Equal(t, int32(2), *previewedDeployment(t, comps[0]).Spec.Replicas)
	assert.Equal(t, int32(0), *previewedDeployment(t, comps[1]).Spec.Replicas)
}

// A spec that names no replica count runs one of each workload.
func TestReplicasDefaultToOne(t *testing.T) {
	t.Parallel()

	in := fixtureRealistic(t)
	in.Optimize.Spec.Webapp.Replicas = nil
	in.Optimize.Spec.Importer.Replicas = nil
	comps, err := Build(in)
	require.NoError(t, err)

	assert.Equal(t, int32(1), *previewedDeployment(t, comps[0]).Spec.Replicas)
	assert.Equal(t, int32(1), *previewedDeployment(t, comps[1]).Spec.Replicas)
}

// A rolling update starts the new importer pod before it stops the old one,
// and two importers that write the same indices at the same time make the
// analytics data inconsistent. The webapp serves read traffic and rolls.
func TestImporterReplacesRatherThanRolls(t *testing.T) {
	t.Parallel()

	comps, err := Build(fixtureRealistic(t))
	require.NoError(t, err)

	assert.Equal(
		t,
		appsv1.RollingUpdateDeploymentStrategyType,
		previewedDeployment(t, comps[0]).Spec.Strategy.Type,
	)
	assert.Equal(
		t,
		appsv1.RecreateDeploymentStrategyType,
		previewedDeployment(t, comps[1]).Spec.Strategy.Type,
	)
}

// Only the importer imports the exported records of the cluster.
func TestOnlyTheImporterImports(t *testing.T) {
	t.Parallel()

	comps, err := Build(fixtureMinimal(t))
	require.NoError(t, err)

	webapp := previewedPodTemplate(t, previewObjects(t, comps[0]))
	importer := previewedPodTemplate(t, previewObjects(t, comps[1]))
	assert.Equal(t, "false", envValueNamed(t, webapp.Spec.Containers[0].Env, envZeebeEnabled))
	assert.Equal(t, "true", envValueNamed(t, importer.Spec.Containers[0].Env, envZeebeEnabled))
}

// A cluster without the ServiceMonitor CRD must not render one, even when
// monitoring is enabled on the spec.
func TestServiceMonitorOmittedWhenUnsupported(t *testing.T) {
	t.Parallel()

	in := fixtureRealistic(t)
	in.ServiceMonitorSupported = false
	comps, err := Build(in)
	require.NoError(t, err)

	for _, comp := range comps {
		for _, obj := range previewObjects(t, comp) {
			_, isMonitor := obj.(*monitoringv1.ServiceMonitor)
			assert.False(t, isMonitor, comp.GetName())
		}
	}
}

// Every ServiceMonitor scrapes /actuator/prometheus on the management port.
func TestServiceMonitorScrapesPrometheusEndpoint(t *testing.T) {
	t.Parallel()

	comps, err := Build(fixtureRealistic(t))
	require.NoError(t, err)

	seen := 0
	for _, comp := range comps {
		for _, obj := range previewObjects(t, comp) {
			monitor, ok := obj.(*monitoringv1.ServiceMonitor)
			if !ok {
				continue
			}
			seen++
			require.Len(t, monitor.Spec.Endpoints, 1, comp.GetName())
			assert.Equal(t, "/actuator/prometheus", monitor.Spec.Endpoints[0].Path, comp.GetName())
			assert.Equal(t, "management", monitor.Spec.Endpoints[0].Port, comp.GetName())
			assert.Equal(t, "platform", monitor.Labels["prometheus"], comp.GetName())
		}
	}
	assert.Equal(t, 2, seen)
}

// Both probes poll the readiness endpoint on the HTTP port. Optimize answers
// it only while it is connected to its database.
func TestProbesUseTheReadinessEndpoint(t *testing.T) {
	t.Parallel()

	comps, err := Build(fixtureMinimal(t))
	require.NoError(t, err)

	for _, comp := range comps {
		container := previewedPodTemplate(t, previewObjects(t, comp)).Spec.Containers[0]
		require.NotNil(t, container.StartupProbe, comp.GetName())
		require.NotNil(t, container.ReadinessProbe, comp.GetName())
		require.NotNil(t, container.LivenessProbe, comp.GetName())

		// The startup probe carries the whole first schema build, so the
		// readiness and liveness probes never see it.
		assert.Equal(t, "/api/readyz", container.StartupProbe.HTTPGet.Path, comp.GetName())
		assert.Equal(t, "http", container.StartupProbe.HTTPGet.Port.StrVal, comp.GetName())
		assert.Equal(t, int32(60), container.StartupProbe.FailureThreshold, comp.GetName())

		assert.Equal(t, "/api/readyz", container.ReadinessProbe.HTTPGet.Path, comp.GetName())
		assert.Equal(t, "http", container.ReadinessProbe.HTTPGet.Port.StrVal, comp.GetName())
	}
}

// The liveness probe must not depend on Elasticsearch. /api/readyz answers 503
// while Optimize cannot reach its database, so using it for liveness restarts
// every pod in a loop for the length of an Elasticsearch outage.
func TestLivenessDoesNotDependOnElasticsearch(t *testing.T) {
	t.Parallel()

	comps, err := Build(fixtureMinimal(t))
	require.NoError(t, err)

	for _, comp := range comps {
		liveness := previewedPodTemplate(t, previewObjects(t, comp)).Spec.Containers[0].LivenessProbe
		require.NotNil(t, liveness, comp.GetName())
		assert.Equal(t, "/actuator/health", liveness.HTTPGet.Path, comp.GetName())
		assert.Equal(t, "management", liveness.HTTPGet.Port.StrVal, comp.GetName())
		assert.NotEqual(t, readinessPath, liveness.HTTPGet.Path, comp.GetName())
	}
}

// The config hash rolls the pods when a rendered value or a referenced object
// changes, and only then. Every credential arrives through a Secret
// reference, so without the hash a rotated password never reaches the pods.
func TestConfigHashRollsOnlyOnChange(t *testing.T) {
	t.Parallel()

	base := fixtureMinimal(t)
	assert.Equal(t, ConfigHash(base, ComponentWebapp), ConfigHash(fixtureMinimal(t), ComponentWebapp))
	assert.NotEqual(t, ConfigHash(base, ComponentWebapp), ConfigHash(base, ComponentImporter))

	rotated := fixtureMinimal(t)
	rotated.HashInputs = []string{"Secret/camunda/es-credentials=1234"}
	assert.NotEqual(t, ConfigHash(base, ComponentWebapp), ConfigHash(rotated, ComponentWebapp))

	// The order of the inputs is not part of the hash.
	reordered := fixtureMinimal(t)
	reordered.HashInputs = []string{"b=2", "a=1"}
	sorted := fixtureMinimal(t)
	sorted.HashInputs = []string{"a=1", "b=2"}
	assert.Equal(t, ConfigHash(reordered, ComponentWebapp), ConfigHash(sorted, ComponentWebapp))

	// No Secret data ever enters the hash, only the reference.
	partitions := fixtureMinimal(t)
	partitions.Partitions = 9
	assert.NotEqual(t, ConfigHash(base, ComponentWebapp), ConfigHash(partitions, ComponentWebapp))
}

// Every pod template carries the hash, and the two components carry different
// ones, so a change to one does not roll the other.
func TestConfigHashAnnotationOnEveryPodTemplate(t *testing.T) {
	t.Parallel()

	in := fixtureMinimal(t)
	comps, err := Build(in)
	require.NoError(t, err)

	hashes := map[string]string{}
	for _, comp := range comps {
		template := previewedPodTemplate(t, previewObjects(t, comp))
		hashes[comp.GetName()] = template.Annotations[ConfigHashAnnotation]
		assert.Equal(t, ConfigHash(in, comp.GetName()), hashes[comp.GetName()], comp.GetName())
	}
	assert.NotEqual(t, hashes[ComponentWebapp], hashes[ComponentImporter])
}

// The mirrored Secrets component reads Disabled without a copy to make, and
// carries one Secret per copied purpose when there is one.
func TestMirroredSecretComponent(t *testing.T) {
	t.Parallel()

	in := fixtureMinimal(t)
	empty, err := MirroredSecretComponent(in.Optimize, nil)
	require.NoError(t, err)
	assert.Equal(
		t,
		component.ConditionType(v1.ConditionMirroredSecretsReady),
		empty.GetCondition(in.Optimize).ConditionType(),
	)

	mirrored, err := MirroredSecretComponent(in.Optimize, map[MirrorPurpose]map[string][]byte{
		MirrorPurposeAuthClient: {"client-secret": []byte("s3cret")},
	})
	require.NoError(t, err)

	// Only the copied purpose is applied. The Secret of every other purpose
	// is gated off, so a reference that went away has its copy deleted.
	objects := previewObjects(t, mirrored)
	require.Len(t, objects, 1)
	assert.Equal(t, "my-optimize-optimize-auth-client", objects[0].GetName())
	assert.Equal(t, "my-cluster", objects[0].GetLabels()["camunda.io/cluster"])
}

// previewedDeployment returns the Deployment of a component.
func previewedDeployment(t *testing.T, comp *component.Component) *appsv1.Deployment {
	t.Helper()

	for _, obj := range previewObjects(t, comp) {
		if workload, ok := obj.(*appsv1.Deployment); ok {
			return workload
		}
	}
	require.Fail(t, "no Deployment in the previewed objects")

	return nil
}
