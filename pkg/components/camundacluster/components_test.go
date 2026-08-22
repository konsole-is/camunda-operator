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
	"strings"
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
	"github.com/konsole-is/camunda-operator/pkg/credentials"
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

// assertGoldens renders every enabled component of the fixture and pins each
// one against testdata/golden/<dir>/<component>.yaml. A disabled process
// still previews its resources (ocf's Preview does not evaluate the gate),
// but its reconcile only deletes, so it is left out. The admin Secret is
// pinned only for a basic-auth fixture, because it exists only then.
func assertGoldens(t *testing.T, dir string, in Input) {
	t.Helper()

	scheme := goldenScheme(t)
	base := filepath.Join("testdata", "golden", dir)

	comps, err := Build(in)
	require.NoError(t, err)
	for _, comp := range enabledComponents(comps) {
		golden.AssertComponentYAML(
			t, filepath.Join(base, comp.GetName()+".yaml"), comp,
			golden.WithScheme(scheme), golden.Update(*updateGolden),
		)
	}

	if ResolveAuth(in).Method == v1.AuthenticationMethodBasic {
		admin, err := AdminSecretComponent(
			in.Cluster,
			true,
			AdminSecretState{
				Password:     credentials.Password{Value: goldenPassword},
				Email:        DefaultAdminEmail,
				AppliedEmail: DefaultAdminEmail,
			},
		)
		require.NoError(t, err)
		golden.AssertComponentYAML(
			t, filepath.Join(base, "admin-secret.yaml"), admin,
			golden.WithScheme(scheme), golden.Update(*updateGolden),
		)
	}
}

// enabledComponents returns the components of the enabled processes.
func enabledComponents(comps []ProcessComponent) []*component.Component {
	var enabled []*component.Component
	for _, pc := range comps {
		if pc.Process.Enabled {
			enabled = append(enabled, pc.Component)
		}
	}
	return enabled
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

// Every fixture builds all six components in Resolve order, so a component
// of a process that an earlier topology enabled always exists to delete it.
func TestBuildOrderAndConditions(t *testing.T) {
	t.Parallel()

	for name, in := range goldenFixtures(t) {
		comps, err := Build(in)
		require.NoError(t, err, name)

		names := make([]string, 0, len(comps))
		conditions := make([]component.ConditionType, 0, len(comps))
		for _, pc := range comps {
			names = append(names, pc.Component.GetName())
			conditions = append(conditions, pc.Component.GetCondition(in.Cluster).ConditionType())
			assert.Equal(t, pc.Process.Component, pc.Component.GetName(), name)
		}
		assert.Equal(t, []string{"zeebe", "gateway", "operate", "tasklist", "admin", "connectors"}, names, name)
		assert.Equal(
			t, []component.ConditionType{
				v1.ConditionZeebeReady, v1.ConditionGatewayReady, v1.ConditionOperateReady,
				v1.ConditionTasklistReady, v1.ConditionAdminReady, v1.ConditionConnectorsReady,
			}, conditions, name,
		)
	}
}

// The enabled set follows the topology; the default fixture runs zeebe, the
// gateway, and connectors, the separated fixture everything.
func TestBuildEnabledProcesses(t *testing.T) {
	t.Parallel()

	enabled := func(in Input) []string {
		comps, err := Build(in)
		require.NoError(t, err)
		names := make([]string, 0, len(comps))
		for _, comp := range enabledComponents(comps) {
			names = append(names, comp.GetName())
		}
		return names
	}
	assert.Equal(t, []string{"zeebe", "gateway", "connectors"}, enabled(fixtureDefault(t)))
	assert.Equal(t, []string{"zeebe"}, enabled(fixtureAllInOne(t)))
	assert.Equal(
		t,
		[]string{"zeebe", "gateway", "operate", "tasklist", "admin", "connectors"},
		enabled(fixtureSeparated(t)),
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
	for _, comp := range enabledComponents(comps) {
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
	require.Len(t, comps, 6)

	hashes := map[string]string{}
	for _, pc := range comps {
		template := previewedPodTemplate(t, previewObjects(t, pc.Component))
		hashes[pc.Process.Component] = template.Annotations[ConfigHashAnnotation]
		assert.Equal(t, ConfigHash(in, pc.Process), hashes[pc.Process.Component], pc.Process.Component)
	}

	// A change to one process rolls that process only.
	changed := fixtureDefault(t)
	changed.Cluster.Spec.Connectors.ExtraEnv = []corev1.EnvVar{{Name: "LOGGING_LEVEL_ROOT", Value: "debug"}}
	changed.Effective = NewEffective(MergePreset(changed.Cluster.Spec, mediumPreset()))
	comps, err = Build(changed)
	require.NoError(t, err)
	for _, pc := range comps {
		template := previewedPodTemplate(t, previewObjects(t, pc.Component))
		if pc.Process.Component == ComponentConnectors {
			assert.NotEqual(t, hashes[pc.Process.Component], template.Annotations[ConfigHashAnnotation])
		} else {
			assert.Equal(t, hashes[pc.Process.Component], template.Annotations[ConfigHashAnnotation])
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
	for _, pc := range comps {
		for _, obj := range previewObjects(t, pc.Component) {
			_, isMonitor := obj.(*monitoringv1.ServiceMonitor)
			assert.False(t, isMonitor, pc.Process.Component)
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
	for _, comp := range enabledComponents(comps) {
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

// Connectors gets no liveness probe, because the liveness group of the
// runtime holds the zeebeClient indicator alone. A probe on that group
// restarts a working container for the whole time the gateway is away, and
// the restart does not bring the gateway back.
//
// A unified process keeps its liveness probe: its liveness group answers from
// the process itself.
func TestConnectorsHasNoLivenessProbe(t *testing.T) {
	t.Parallel()

	comps, err := Build(fixtureDefault(t))
	require.NoError(t, err)

	seen := 0
	for _, pc := range comps {
		if !pc.Process.Enabled {
			continue
		}

		container := previewedPodTemplate(t, previewObjects(t, pc.Component)).Spec.Containers[0]
		seen++

		if pc.Process.Component == ComponentConnectors {
			assert.Nil(t, container.LivenessProbe, pc.Process.Component)
			continue
		}

		assert.NotNil(t, container.LivenessProbe, pc.Process.Component)
	}
	assert.Equal(t, 3, seen)
}

// Every process, connectors included, gets a startup probe on the startup
// group. The group answers from the process, so it holds the readiness probe
// back over a slow boot without an opinion about the gateway.
func TestEveryProcessHasAStartupProbe(t *testing.T) {
	t.Parallel()

	comps, err := Build(fixtureDefault(t))
	require.NoError(t, err)

	seen := 0
	for _, pc := range comps {
		if !pc.Process.Enabled {
			continue
		}

		container := previewedPodTemplate(t, previewObjects(t, pc.Component)).Spec.Containers[0]
		seen++

		require.NotNil(t, container.StartupProbe, pc.Process.Component)
		assert.Equal(
			t, healthStartupPath, container.StartupProbe.HTTPGet.Path, pc.Process.Component,
		)

		want := portNameManagement
		if pc.Process.Component == ComponentConnectors {
			want = portNameHTTP
		}
		assert.Equal(
			t, want, container.StartupProbe.HTTPGet.Port.StrVal, pc.Process.Component,
		)
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
	for _, obj := range previewObjects(t, comps[0].Component) {
		if _, ok := obj.(*corev1.ServiceAccount); ok {
			hasAccount = true
		}
	}
	assert.True(t, hasAccount)
	for _, pc := range comps {
		assert.Equal(
			t, "my-cluster-camunda", previewedPodTemplate(t, previewObjects(t, pc.Component)).Spec.ServiceAccountName,
		)
	}

	minimal, err := Build(fixtureMinimal(t))
	require.NoError(t, err)
	for _, pc := range minimal {
		for _, obj := range previewObjects(t, pc.Component) {
			_, isAccount := obj.(*corev1.ServiceAccount)
			assert.False(t, isAccount, pc.Process.Component)
		}
		assert.Empty(t, previewedPodTemplate(t, previewObjects(t, pc.Component)).Spec.ServiceAccountName)
	}
}

// The password must land in the published Secret verbatim, under the
// documented keys.
func TestAdminSecretComponentCarriesThePassword(t *testing.T) {
	t.Parallel()

	in := fixtureMinimal(t)
	comp, err := AdminSecretComponent(
		in.Cluster,
		true,
		AdminSecretState{Password: credentials.Password{Value: "s3cret"}},
	)
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
	assert.Empty(t, secret.Annotations)
}

// A password that came from an existing Secret must bind its apply to that
// Secret, or a delete between the read and the apply recreates the Secret with
// the old password and the delete rotates nothing.
func TestAdminSecretComponentCarriesTheApplyPrecondition(t *testing.T) {
	t.Parallel()

	in := fixtureMinimal(t)
	reused := credentials.Password{Value: "s3cret", SourceUID: "uid-1"}
	comp, err := AdminSecretComponent(in.Cluster, true, AdminSecretState{Password: reused})
	require.NoError(t, err)

	objects := previewObjects(t, comp)
	require.Len(t, objects, 1)
	assert.Equal(
		t,
		map[string]string{credentials.PreconditionAnnotation: "uid-1"},
		objects[0].GetAnnotations(),
	)
}

func TestBrokerClaimSelector(t *testing.T) {
	t.Parallel()

	assert.Equal(
		t,
		map[string]string{"camunda.io/cluster": "my-cluster", "camunda.io/component": "zeebe"},
		BrokerClaimSelector(fixtureMinimal(t).Cluster),
	)
}

// While a rotation is in flight the Secret carries the requested password
// next to the active one, so a crash between the user API call and the
// publish can never lose it. The key disappears when the rotation completes.
func TestAdminSecretComponentCarriesTheRotationBookkeeping(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name            string
		email           string
		appliedEmail    string
		pending         string
		pendingRotation string
		rotation        string
		want            map[string]string
		absent          []string
	}{
		{
			name:            "a rotation in flight publishes the pending password and the request",
			pending:         "n3xt",
			pendingRotation: "2026-09",
			want: map[string]string{
				"password-pending": "n3xt", "password-pending-rotation": "2026-09",
			},
			absent: []string{"password-rotation"},
		},
		{
			name:     "an applied rotation travels with the password it answers",
			rotation: "2026-08",
			want:     map[string]string{"password-rotation": "2026-08"},
			absent:   []string{"password-pending", "password-pending-rotation"},
		},
		{
			name:   "a cluster that never rotated carries no bookkeeping",
			absent: []string{"password-pending", "password-pending-rotation", "password-rotation"},
		},
		{
			name:   "an address the cluster never accepted is not recorded as applied",
			email:  "team@example.com",
			want:   map[string]string{"email": "team@example.com"},
			absent: []string{"email-applied"},
		},
		{
			name:         "an accepted address is recorded beside the one the processes read",
			email:        "team@example.com",
			appliedEmail: "team@example.com",
			want:         map[string]string{"email": "team@example.com", "email-applied": "team@example.com"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			in := fixtureMinimal(t)
			comp, err := AdminSecretComponent(in.Cluster, true, AdminSecretState{
				Password:        credentials.Password{Value: "s3cret"},
				Email:           tt.email,
				AppliedEmail:    tt.appliedEmail,
				PendingPassword: tt.pending,
				PendingRotation: tt.pendingRotation,
				Rotation:        tt.rotation,
			})
			require.NoError(t, err)

			objects := previewObjects(t, comp)
			require.Len(t, objects, 1)
			secret, ok := objects[0].(*corev1.Secret)
			require.True(t, ok)

			assert.Equal(t, []byte("s3cret"), secret.Data["password"])
			for key, value := range tt.want {
				assert.Equal(t, []byte(value), secret.Data[key])
			}
			for _, key := range tt.absent {
				assert.NotContains(t, secret.Data, key)
			}
		})
	}
}

// The legacy Zeebe Elasticsearch exporter writes the zeebe-record indices
// that Optimize reads. It carries no TLS setting of its own and trusts what
// the JVM trusts, so a broker reaches an HTTPS Elasticsearch only when the
// JVM already trusts its certificate authority. A cluster whose secondary
// storage names one therefore builds a JVM trust store and runs every
// process against it (camunda/camunda#9839).
//
// This test names no constant of the package on purpose. It reads the
// rendered pod template the way a user reads it with kubectl.
func TestElasticsearchCAReachesTheJVMTrustStore(t *testing.T) {
	t.Parallel()

	comps, err := Build(fixtureDefault(t))
	require.NoError(t, err)

	seen := 0
	for _, pc := range comps {
		if !pc.Process.Enabled || pc.Process.Component == "connectors" {
			continue
		}
		seen++

		process := pc.Process.Component
		template := previewedPodTemplate(t, previewObjects(t, pc.Component))

		require.Len(t, template.Spec.InitContainers, 1, process)
		init := template.Spec.InitContainers[0]
		assert.Contains(t, strings.Join(init.Command, " "), "keytool", process)
		assert.Equal(t, template.Spec.Containers[0].Image, init.Image, process)

		options := containerEnvValue(template.Spec.Containers[0], "JAVA_TOOL_OPTIONS")
		assert.Contains(t, options, "-XX:+ExitOnOutOfMemoryError", process)
		assert.Contains(t, options, "-Djavax.net.ssl.trustStorePassword=changeit", process)

		store := trustStoreOptionValue(t, options)
		assert.True(
			t, mountsFile(template.Spec.Containers[0], store),
			"%s: the process container does not mount %q", process, store,
		)
		assert.True(
			t, mountsFile(init, store), "%s: the init container does not mount %q", process, store,
		)
	}
	assert.Equal(t, 2, seen)
}

// The connectors runtime never reads the secondary storage, so it gets no
// trust store.
func TestConnectorsGetNoTrustStore(t *testing.T) {
	t.Parallel()

	comps, err := Build(fixtureDefault(t))
	require.NoError(t, err)

	for _, pc := range comps {
		if pc.Process.Component != ComponentConnectors {
			continue
		}

		template := previewedPodTemplate(t, previewObjects(t, pc.Component))
		assert.Empty(t, template.Spec.InitContainers)
		assert.Empty(t, template.Spec.Volumes)
	}
}

// containerEnvValue returns the value of the named environment entry of a
// container, or "" when it carries none.
func containerEnvValue(c corev1.Container, name string) string {
	for _, e := range c.Env {
		if e.Name == name {
			return e.Value
		}
	}
	return ""
}

// trustStoreOptionValue returns the path that -Djavax.net.ssl.trustStore
// names in a JAVA_TOOL_OPTIONS value.
func trustStoreOptionValue(t *testing.T, options string) string {
	t.Helper()

	const prefix = "-Djavax.net.ssl.trustStore="
	for option := range strings.FieldsSeq(options) {
		if path, found := strings.CutPrefix(option, prefix); found {
			return path
		}
	}

	require.Fail(t, "no trust store option", options)
	return ""
}

// mountsFile reports whether a container mounts the volume that holds file.
func mountsFile(c corev1.Container, file string) bool {
	for _, mount := range c.VolumeMounts {
		if strings.HasPrefix(file, mount.MountPath+"/") {
			return true
		}
	}
	return false
}
