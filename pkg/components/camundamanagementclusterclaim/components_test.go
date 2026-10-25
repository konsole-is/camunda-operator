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

package camundamanagementclusterclaim

import (
	"flag"
	"testing"

	"github.com/sourcehawk/operator-component-framework/pkg/testing/golden"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/runtime"

	pluginsv1alpha1 "github.com/konsole-is/camunda-operator/api/plugins/v1alpha1"
	v1 "github.com/konsole-is/camunda-operator/api/v1"
	"github.com/konsole-is/camunda-operator/pkg/claimpolicy"
	plane "github.com/konsole-is/camunda-operator/pkg/components/camundamanagementcluster"
)

// updateGolden refreshes the golden manifests with the rendered output:
// go test ./pkg/components/camundamanagementclusterclaim/ -run Golden -update-golden
var updateGolden = flag.Bool("update-golden", false, "update golden files")

func TestComponentsGolden(t *testing.T) {
	scheme := runtime.NewScheme()
	require.NoError(t, pluginsv1alpha1.AddToScheme(scheme))
	require.NoError(t, v1.AddToScheme(scheme))

	in := fullInput(t)
	params, err := WithDatabases(
		[]claimpolicy.RequestParameters{
			{Section: databaseSection, Registration: "management-database-server"},
			{Section: accessSection, Registration: "test-access"},
		},
		in.Sections,
		Databases(in.Claim.Spec.Management),
	)
	require.NoError(t, err)
	in.Parameters = params

	comps, err := Build(in)
	require.NoError(t, err)

	golden.AssertComponentYAML(
		t, "testdata/requests.yaml", comps.Requests, golden.WithScheme(scheme), golden.Update(*updateGolden),
	)
	golden.AssertComponentYAML(
		t,
		"testdata/managementcluster.yaml",
		comps.ManagementCluster,
		golden.WithScheme(scheme),
		golden.Update(*updateGolden),
	)
}

// A grant to the target of a plane request reaches the workloads of the
// plane, not an account that no pod of the plane runs under.
func TestRequestsTargetTheAccountOfThePlane(t *testing.T) {
	in := fullInput(t)
	comps, err := Build(in)
	require.NoError(t, err)
	desired, _, err := managementCluster(in)
	require.NoError(t, err)

	objects, err := comps.Requests.Preview()
	require.NoError(t, err)
	require.NotEmpty(t, objects)
	for _, obj := range objects {
		request, ok := obj.(*pluginsv1alpha1.PluginRequest)
		require.True(t, ok)
		assert.Equal(t, plane.ServiceAccountName(desired), request.Spec.Target.ServiceAccountName, request.Name)
	}
}

func TestEverySlotPathOfThePlaneHasAField(t *testing.T) {
	in := fullInput(t)
	fields, err := claimpolicy.SlotFields(
		pluginsv1alpha1.ClaimKindCamundaManagementClusterClaim, in.Sections, in.Claim.Spec.Plugins, in.Requests,
	)
	require.NoError(t, err)

	for _, f := range fields {
		if f.Kind == claimpolicy.KindCamundaManagementCluster {
			assert.Contains(t, slotFields, f.Path)
		}
	}
}
