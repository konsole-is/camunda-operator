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

package camundaclusterclaim

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
)

// updateGolden refreshes the golden manifests with the rendered output:
// go test ./pkg/components/camundaclusterclaim/ -run Golden -update-golden
var updateGolden = flag.Bool("update-golden", false, "update golden files")

func TestComponentsGolden(t *testing.T) {
	scheme := runtime.NewScheme()
	require.NoError(t, pluginsv1alpha1.AddToScheme(scheme))
	require.NoError(t, v1.AddToScheme(scheme))

	comps, err := Build(fullInput(t))
	require.NoError(t, err)

	golden.AssertComponentYAML(
		t, "testdata/requests.yaml", comps.Requests, golden.WithScheme(scheme), golden.Update(*updateGolden),
	)
	golden.AssertComponentYAML(
		t, "testdata/cluster.yaml", comps.Cluster, golden.WithScheme(scheme), golden.Update(*updateGolden),
	)
}

func TestEverySlotPathOfTheClusterHasASetter(t *testing.T) {
	in := fullInput(t)
	fields, err := claimpolicy.SlotFields(
		pluginsv1alpha1.ClaimKindCamundaClusterClaim, in.Sections, in.Claim.Spec.Plugins, in.Requests,
	)
	require.NoError(t, err)

	for _, f := range fields {
		if f.Kind == claimpolicy.KindCamundaCluster {
			assert.Contains(t, slotSetters, f.Path)
		}
	}
	_, err = Build(in)
	assert.NoError(t, err)
}

func TestRequestsInputTrustsTheBuiltinStorageOnTheCloudIdentity(t *testing.T) {
	in := fullInput(t)

	requests := RequestsInput(in)

	assert.Equal(t, map[string][]string{"cloud-identity": {"orders-es-es"}}, requests.AdditionalServiceAccounts)
	assert.Equal(t, pluginsv1alpha1.ClaimKindCamundaClusterClaim, requests.ClaimKind)
	assert.Equal(t, claimpolicy.KindCamundaCluster, requests.TargetKind)
	assert.Equal(t, "camunda-8-9", requests.ReleaseRef)
	assert.Equal(t, pluginsv1alpha1.DeletionPolicyDelete, requests.DeletionPolicy)
}

func TestRequestsInputAddsNoAccountWithoutBuiltinStorage(t *testing.T) {
	in := fullInput(t)
	in.Claim.Spec.Plugins[2].Registration = "existing-secondary-storage"

	assert.Nil(t, RequestsInput(in).AdditionalServiceAccounts)
}
