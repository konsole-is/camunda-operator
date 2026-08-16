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

package camundacluster

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
// go test ./pkg/components/camundacluster/ -run Golden -update-golden
var updateGolden = flag.Bool("update-golden", false, "update golden files")

// goldenPassword replaces the generated admin password, so the golden
// manifests stay deterministic.
const goldenPassword = "golden-test-password"

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

// assertGoldens renders every component of the fixture and pins each one
// against testdata/golden/<dir>/<component>.yaml. The admin Secret is pinned
// only for a basic-auth fixture, because it exists only then.
func assertGoldens(t *testing.T, dir string, in Input) {
	t.Helper()

	scheme := goldenScheme(t)
	base := filepath.Join("testdata", "golden", dir)

	comps, err := Build(in)
	require.NoError(t, err)
	for _, comp := range comps {
		golden.AssertComponentYAML(
			t, filepath.Join(base, comp.GetName()+".yaml"), comp,
			golden.WithScheme(scheme), golden.Update(*updateGolden),
		)
	}

	if ResolveAuth(in).Method == v1.AuthenticationMethodBasic {
		admin, err := AdminSecretComponent(in.Cluster, goldenPassword)
		require.NoError(t, err)
		golden.AssertComponentYAML(
			t, filepath.Join(base, "admin-secret.yaml"), admin,
			golden.WithScheme(scheme), golden.Update(*updateGolden),
		)
	}
}

func TestCamundaClusterGoldens(t *testing.T) {
	t.Parallel()

	for name, in := range goldenFixtures(t) {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			assertGoldens(t, name, in)
		})
	}
}

// previewObjects returns the previewed objects of a component.
func previewObjects(t *testing.T, comp *component.Component) []client.Object {
	t.Helper()

	objects, err := comp.Preview()
	require.NoError(t, err)
	return objects
}

// previewedPodTemplate returns the pod template of the workload in the previewed
// objects.
func previewedPodTemplate(t *testing.T, objects []client.Object) corev1.PodTemplateSpec {
	t.Helper()

	for _, obj := range objects {
		switch typed := obj.(type) {
		case *appsv1.StatefulSet:
			return typed.Spec.Template
		case *appsv1.Deployment:
			return typed.Spec.Template
		}
	}
	require.Fail(t, "no workload in the previewed objects")
	return corev1.PodTemplateSpec{}
}

func TestBuildOrderAndConditions(t *testing.T) {
	t.Parallel()

	in := fixtureSeparated(t)
	comps, err := Build(in)
	require.NoError(t, err)

	names := make([]string, 0, len(comps))
	conditions := make([]component.ConditionType, 0, len(comps))
	for _, comp := range comps {
		names = append(names, comp.GetName())
		conditions = append(conditions, comp.GetCondition(in.Cluster).ConditionType())
	}
	assert.Equal(t, []string{"zeebe", "gateway", "operate", "tasklist", "admin", "connectors"}, names)
	assert.Equal(
		t, []component.ConditionType{
			v1.ConditionZeebeReady, v1.ConditionGatewayReady, v1.ConditionOperateReady,
			v1.ConditionTasklistReady, v1.ConditionAdminReady, v1.ConditionConnectorsReady,
		}, conditions,
	)
}

// User pod labels must not override the discovery labels that extensions
// and selectors find the pods by.
func TestPodLabelsDoNotOverrideDiscoveryLabels(t *testing.T) {
	t.Parallel()

	in := fixtureMinimal(t)
	in.Cluster.Spec.PodLabels = map[string]string{
		"camunda.io/cluster":   "someone-else",
		"camunda.io/component": "not-zeebe",
		"team":                 "platform",
	}
	in.Effective = NewEffective(in.Cluster.Spec)

	comps, err := Build(in)
	require.NoError(t, err)
	for _, comp := range comps {
		template := previewedPodTemplate(t, previewObjects(t, comp))
		assert.Equal(t, "my-cluster", template.Labels["camunda.io/cluster"], comp.GetName())
		assert.Equal(t, comp.GetName(), template.Labels["camunda.io/component"], comp.GetName())
		assert.Equal(t, "platform", template.Labels["team"], comp.GetName())
	}
}

func TestConfigHashAnnotationOnEveryPodTemplate(t *testing.T) {
	t.Parallel()

	in := fixtureDefault(t)
	comps, err := Build(in)
	require.NoError(t, err)
	require.Len(t, comps, 3)

	hashes := map[string]string{}
	for i, comp := range comps {
		template := previewedPodTemplate(t, previewObjects(t, comp))
		p := Resolve(in.Effective)[i]
		hashes[p.Component] = template.Annotations[ConfigHashAnnotation]
		assert.Equal(t, ConfigHash(in, p), hashes[p.Component], comp.GetName())
	}

	// A change to one process rolls that process only.
	changed := fixtureDefault(t)
	changed.Cluster.Spec.Connectors.ExtraEnv = []corev1.EnvVar{{Name: "LOGGING_LEVEL_ROOT", Value: "debug"}}
	changed.Effective = NewEffective(MergePreset(changed.Cluster.Spec, mediumPreset()))
	comps, err = Build(changed)
	require.NoError(t, err)
	for i, comp := range comps {
		template := previewedPodTemplate(t, previewObjects(t, comp))
		p := Resolve(changed.Effective)[i]
		if p.Component == ComponentConnectors {
			assert.NotEqual(t, hashes[p.Component], template.Annotations[ConfigHashAnnotation], comp.GetName())
		} else {
			assert.Equal(t, hashes[p.Component], template.Annotations[ConfigHashAnnotation], comp.GetName())
		}
	}
}

// A cluster without the ServiceMonitor CRD must not render one, even when
// monitoring is enabled on the spec.
func TestServiceMonitorOmittedWhenUnsupported(t *testing.T) {
	t.Parallel()

	in := fixtureDefault(t)
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

// Every ServiceMonitor scrapes /actuator/prometheus: on the management port
// of a unified process and on the HTTP port of connectors.
func TestServiceMonitorScrapesPrometheusEndpoint(t *testing.T) {
	t.Parallel()

	comps, err := Build(fixtureDefault(t))
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
			want := "management"
			if comp.GetName() == ComponentConnectors {
				want = "http"
			}
			assert.Equal(t, want, monitor.Spec.Endpoints[0].Port, comp.GetName())
		}
	}
	assert.Equal(t, 3, seen)
}

// The service account is rendered by the first component only when the spec
// asks for it, and every pod template names it.
func TestServiceAccount(t *testing.T) {
	t.Parallel()

	comps, err := Build(fixtureDefault(t))
	require.NoError(t, err)

	hasAccount := false
	for _, obj := range previewObjects(t, comps[0]) {
		if _, ok := obj.(*corev1.ServiceAccount); ok {
			hasAccount = true
		}
	}
	assert.True(t, hasAccount)
	for _, comp := range comps {
		assert.Equal(t, "my-cluster-camunda", previewedPodTemplate(t, previewObjects(t, comp)).Spec.ServiceAccountName)
	}

	minimal, err := Build(fixtureMinimal(t))
	require.NoError(t, err)
	for _, comp := range minimal {
		for _, obj := range previewObjects(t, comp) {
			_, isAccount := obj.(*corev1.ServiceAccount)
			assert.False(t, isAccount, comp.GetName())
		}
		assert.Empty(t, previewedPodTemplate(t, previewObjects(t, comp)).Spec.ServiceAccountName)
	}
}

// The password must land in the published Secret verbatim, under the
// documented keys.
func TestAdminSecretComponentCarriesThePassword(t *testing.T) {
	t.Parallel()

	in := fixtureMinimal(t)
	comp, err := AdminSecretComponent(in.Cluster, "s3cret")
	require.NoError(t, err)
	assert.Equal(
		t,
		component.ConditionType(v1.ConditionAdminSecretReady),
		comp.GetCondition(in.Cluster).ConditionType(),
	)

	objects := previewObjects(t, comp)
	require.Len(t, objects, 1)
	secret, ok := objects[0].(*corev1.Secret)
	require.True(t, ok)
	assert.Equal(t, "my-cluster-camunda-admin", secret.Name)
	assert.Equal(t, []byte("admin"), secret.Data["username"])
	assert.Equal(t, []byte("s3cret"), secret.Data["password"])
}

func TestBrokerClaimSelector(t *testing.T) {
	t.Parallel()

	assert.Equal(
		t,
		map[string]string{"camunda.io/cluster": "my-cluster", "camunda.io/component": "zeebe"},
		BrokerClaimSelector(fixtureMinimal(t).Cluster),
	)
}
