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

package camundamanagementcluster

import (
	"flag"
	"path/filepath"
	"testing"

	"github.com/sourcehawk/operator-component-framework/pkg/component"
	"github.com/sourcehawk/operator-component-framework/pkg/testing/golden"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	appsv1 "k8s.io/api/apps/v1"
	"k8s.io/apimachinery/pkg/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"

	v1 "github.com/konsole-is/camunda-operator/api/v1"
	"github.com/konsole-is/camunda-operator/pkg/wrappers/keycloak"
)

// updateGolden refreshes the golden manifests with the rendered output:
// go test ./pkg/components/camundamanagementcluster/ -run Golden -update-golden
var updateGolden = flag.Bool("update-golden", false, "update golden files")

// TestCamundaManagementClusterGoldens pins every component of every fixture
// against testdata/golden/<fixture>/<component>.yaml.
func TestCamundaManagementClusterGoldens(t *testing.T) {
	t.Parallel()

	for name, in := range goldenFixtures(t) {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			scheme := goldenScheme(t)
			base := filepath.Join("testdata", "golden", name)

			built, err := Build(in)
			require.NoError(t, err)
			for _, comp := range built.Components {
				objects, err := comp.Preview()
				require.NoError(t, err)
				// A component whose resources are all gated off applies
				// nothing, so there is nothing to pin. The copies of
				// referenced Secrets are gated off on a management cluster
				// that references none.
				if len(objects) == 0 {
					continue
				}
				golden.AssertComponentYAML(
					t, filepath.Join(base, comp.GetName()+".yaml"), comp,
					golden.WithScheme(scheme), golden.Update(*updateGolden),
				)
			}
		})
	}
}

// The oidc mode renders Management Identity alone. The copies of referenced
// Secrets and the Keycloak are always built, so a reference that moved into
// the management namespace has its copy deleted and a move off the keycloak
// mode has its Keycloak deleted. Both take part in Ready only while they are
// on.
func TestBuildRendersIdentityAndTheCopiesOfReferencedSecrets(t *testing.T) {
	t.Parallel()

	minimal, err := Build(fixtureMinimal(t))
	require.NoError(t, err)
	assert.Equal(
		t,
		[]string{ComponentMirroredSecrets, ComponentKeycloak, ComponentIdentity},
		componentNames(minimal.Components),
	)
	assert.Equal(t, []string{ComponentIdentity}, componentNames(minimal.Ready))

	realistic, err := Build(fixtureRealistic(t))
	require.NoError(t, err)
	assert.Equal(
		t, []string{ComponentMirroredSecrets, ComponentIdentity}, componentNames(realistic.Ready),
	)
}

// A purpose outside MirrorPurposes renders no Secret, so the copy would go
// missing without a word. Build refuses it instead.
func TestBuildRefusesAnUnknownMirrorPurpose(t *testing.T) {
	t.Parallel()

	in := fixtureMinimal(t)
	in.Mirrors = map[MirrorPurpose]map[string][]byte{"licence": {"license": []byte("typo")}}

	_, err := Build(in)

	require.Error(t, err)
	assert.Contains(t, err.Error(), `purpose "licence" is not in MirrorPurposes`)
}

// componentNames returns the ocf name of each component, in order.
func componentNames(comps []*component.Component) []string {
	names := make([]string, 0, len(comps))
	for _, comp := range comps {
		names = append(names, comp.GetName())
	}

	return names
}

// builtComponent returns the component of the given name, and fails when the
// build rendered none.
func builtComponent(t *testing.T, built Built, name string) *component.Component {
	t.Helper()

	for _, comp := range built.Components {
		if comp.GetName() == name {
			return comp
		}
	}
	require.FailNowf(t, "component not built", "no component named %q", name)

	return nil
}

// goldenScheme registers every type for which the golden serializer must
// resolve TypeMeta.
func goldenScheme(t *testing.T) *runtime.Scheme {
	t.Helper()

	scheme := runtime.NewScheme()
	require.NoError(t, clientgoscheme.AddToScheme(scheme))
	require.NoError(t, v1.AddToScheme(scheme))
	require.NoError(t, keycloak.AddToScheme(scheme))

	return scheme
}

// previewedDeployment returns the Deployment in the previewed objects of a
// component.
func previewedDeployment(t *testing.T, objects []client.Object) *appsv1.Deployment {
	t.Helper()

	for _, obj := range objects {
		if workload, ok := obj.(*appsv1.Deployment); ok {
			return workload
		}
	}
	require.Fail(t, "no Deployment in the previewed objects")

	return nil
}
