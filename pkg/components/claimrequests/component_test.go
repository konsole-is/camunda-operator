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

package claimrequests

import (
	"flag"
	"testing"

	"github.com/sourcehawk/operator-component-framework/pkg/testing/golden"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"

	fleetv1alpha1 "github.com/konsole-is/camunda-operator/api/fleet/v1alpha1"
	pluginsv1alpha1 "github.com/konsole-is/camunda-operator/api/plugins/v1alpha1"
	"github.com/konsole-is/camunda-operator/pkg/claimpolicy"
)

// updateGolden refreshes the golden manifests with the rendered output:
// go test ./pkg/components/claimrequests/ -run Golden -update-golden
var updateGolden = flag.Bool("update-golden", false, "update golden files")

func TestComponentWritesOneRequestForEachSelectionInOrder(t *testing.T) {
	requests := previewRequests(t, volumeInput(t))

	require.Len(t, requests, 2)
	assert.Equal(t, "orders-encryption-key", requests[0].Name)
	assert.Equal(t, "orders-volume-encryption", requests[1].Name)

	volume := requests[1]
	assert.Equal(t, "team-a", volume.Namespace)
	assert.Equal(
		t, map[string]string{
			pluginsv1alpha1.LabelSection:      "volume-encryption",
			pluginsv1alpha1.LabelRegistration: "test-volumes",
			fleetv1alpha1.LabelClaim:          "orders",
		}, volume.Labels,
	)
	assert.Equal(
		t, pluginsv1alpha1.PluginRequestSpec{
			Section:      "volume-encryption",
			Registration: "test-volumes",
			ClaimRef:     pluginsv1alpha1.ClaimRef{Kind: pluginsv1alpha1.ClaimKindCamundaClusterClaim, Name: "orders"},
			Target: pluginsv1alpha1.TargetRef{
				Kind: claimpolicy.KindCamundaCluster, Name: "orders", ServiceAccountName: "orders-camunda",
			},
			Inputs:         map[string]string{"keyRef": "arn:aws:kms:key"},
			DeletionPolicy: pluginsv1alpha1.DeletionPolicyDelete,
		}, volume.Spec,
	)
}

func TestComponentTargetsTheConnectorsAccountForConnectorsPermissions(t *testing.T) {
	in := volumeInput(t)
	in.Sections = append(in.Sections, section("connectors-permissions", pluginsv1alpha1.SlotConnectorsIdentity))
	in.Registrations = append(in.Registrations, registration("test-connectors", "connectors-permissions"))
	in.Selections = []fleetv1alpha1.PluginSelection{
		{Section: "connectors-permissions", Registration: "test-connectors"},
	}
	in.Order = in.Selections

	requests := previewRequests(t, in)

	require.Len(t, requests, 1)
	assert.Equal(t, "orders-connectors", requests[0].Spec.Target.ServiceAccountName)
}

func TestComponentCarriesTheParametersAndAdditionalServiceAccounts(t *testing.T) {
	in := volumeInput(t)
	in.Parameters = []claimpolicy.RequestParameters{{
		Section:      "encryption-key",
		Registration: "test-key",
		Parameters:   &apiextensionsv1.JSON{Raw: []byte(`{"rotation":"yearly"}`)},
	}}
	in.AdditionalServiceAccounts = map[string][]string{"encryption-key": {"orders-es-es"}}

	requests := previewRequests(t, in)

	require.Len(t, requests, 2)
	assert.JSONEq(t, `{"rotation":"yearly"}`, string(requests[0].Spec.Parameters.Raw))
	assert.Equal(t, []string{"orders-es-es"}, requests[0].Spec.Target.AdditionalServiceAccounts)
	assert.Nil(t, requests[1].Spec.Parameters)
	assert.Empty(t, requests[1].Spec.Target.AdditionalServiceAccounts)
}

// A registration whose parameter schema declares releaseRef gets the release
// of the claim, over the value that the policy gave it (CL55).
func TestComponentSetsTheReleaseOfARegistrationThatDeclaresIt(t *testing.T) {
	tests := []struct {
		name       string
		schema     string
		parameters string
		release    string
		want       string
	}{
		{
			name:    "a declared releaseRef gets the release",
			schema:  `{"type":"object","properties":{"releaseRef":{"type":"string"}}}`,
			release: "camunda-8-9",
			want:    `{"releaseRef":"camunda-8-9"}`,
		},
		{
			name:       "the release replaces another value and keeps the other parameters",
			schema:     `{"type":"object","properties":{"releaseRef":{"type":"string"},"preset":{"type":"string"}}}`,
			parameters: `{"preset":"small","releaseRef":"old"}`,
			release:    "camunda-8-9",
			want:       `{"preset":"small","releaseRef":"camunda-8-9"}`,
		},
		{
			name:       "a schema without releaseRef keeps the parameters",
			schema:     `{"type":"object","properties":{"preset":{"type":"string"}}}`,
			parameters: `{"preset":"small"}`,
			release:    "camunda-8-9",
			want:       `{"preset":"small"}`,
		},
		{
			name:       "a releaseRef that is not a string is not the release",
			schema:     `{"type":"object","properties":{"releaseRef":{"type":"object"}}}`,
			parameters: `{"preset":"small"}`,
			release:    "camunda-8-9",
			want:       `{"preset":"small"}`,
		},
		{
			name:       "an empty release sets nothing",
			schema:     `{"type":"object","properties":{"releaseRef":{"type":"string"}}}`,
			parameters: `{"preset":"small"}`,
			want:       `{"preset":"small"}`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			in := volumeInput(t)
			in.Registrations[1].Spec.Parameters = &apiextensionsv1.JSON{Raw: []byte(tt.schema)}
			in.ReleaseRef = tt.release
			if tt.parameters != "" {
				in.Parameters = []claimpolicy.RequestParameters{{
					Section:      "encryption-key",
					Registration: "test-key",
					Parameters:   &apiextensionsv1.JSON{Raw: []byte(tt.parameters)},
				}}
			}

			requests := previewRequests(t, in)

			require.NotNil(t, requests[0].Spec.Parameters)
			assert.JSONEq(t, tt.want, string(requests[0].Spec.Parameters.Raw))
		})
	}
}

func TestComponentGoldenRequests(t *testing.T) {
	comp, err := Component(claim(), volumeInput(t))
	require.NoError(t, err)

	golden.AssertComponentYAML(
		t,
		"testdata/requests.yaml",
		comp,
		golden.WithScheme(scheme(t)),
		golden.Update(*updateGolden),
	)
}

func TestComponentRefusesASelectionWithoutItsRegistration(t *testing.T) {
	in := volumeInput(t)
	in.Registrations = in.Registrations[:1]

	_, err := Component(claim(), in)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "test-key")
}

func TestComponentRefusesASelectionWithoutItsSection(t *testing.T) {
	in := volumeInput(t)
	in.Sections = in.Sections[:1]

	_, err := Component(claim(), in)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "encryption-key")
}

func TestName(t *testing.T) {
	assert.Equal(t, "orders-access", Name("orders", "access"))
}

// volumeInput is the input of a claim orders that selects test-volumes,
// which reads keyRef from encryption-key, and test-key, whose request
// reports keyRef.
func volumeInput(t *testing.T) Input {
	t.Helper()

	sections := []pluginsv1alpha1.PluginSection{
		section("volume-encryption", pluginsv1alpha1.SlotVolumeEncryption),
		section("encryption-key", ""),
	}
	volumes := registration("test-volumes", "volume-encryption")
	volumes.Spec.Inputs = []pluginsv1alpha1.InputDecl{{Name: "keyRef", From: "encryption-key/keyRef", Required: true}}
	registrations := []pluginsv1alpha1.PluginRegistration{volumes, registration("test-key", "encryption-key")}
	selections := []fleetv1alpha1.PluginSelection{
		{Section: "volume-encryption", Registration: "test-volumes"},
		{Section: "encryption-key", Registration: "test-key"},
	}

	order, err := claimpolicy.Order(selections, registrations)
	require.NoError(t, err)

	key := pluginsv1alpha1.PluginRequest{
		ObjectMeta: metav1.ObjectMeta{Name: "orders-encryption-key", Namespace: "team-a", Generation: 1},
		Spec:       pluginsv1alpha1.PluginRequestSpec{Section: "encryption-key", Registration: "test-key"},
		Status: pluginsv1alpha1.PluginRequestStatus{
			Conditions: []metav1.Condition{{
				Type:               pluginsv1alpha1.ConditionOutputsReady,
				Status:             metav1.ConditionTrue,
				ObservedGeneration: 1,
			}},
			Outputs: map[string]string{"keyRef": "arn:aws:kms:key"},
		},
	}

	return Input{
		ClaimKind:      pluginsv1alpha1.ClaimKindCamundaClusterClaim,
		TargetKind:     claimpolicy.KindCamundaCluster,
		ConditionType:  fleetv1alpha1.ConditionRequestsReady,
		Selections:     selections,
		Order:          order,
		Sections:       sections,
		Registrations:  registrations,
		Requests:       []pluginsv1alpha1.PluginRequest{key},
		DeletionPolicy: pluginsv1alpha1.DeletionPolicyDelete,
	}
}

func claim() *fleetv1alpha1.CamundaClusterClaim {
	return &fleetv1alpha1.CamundaClusterClaim{ObjectMeta: metav1.ObjectMeta{Name: "orders", Namespace: "team-a"}}
}

func section(name string, slot pluginsv1alpha1.Slot) pluginsv1alpha1.PluginSection {
	return pluginsv1alpha1.PluginSection{
		ObjectMeta: metav1.ObjectMeta{Name: name},
		Spec:       pluginsv1alpha1.PluginSectionSpec{Slot: slot},
	}
}

func registration(name, section string) pluginsv1alpha1.PluginRegistration {
	return pluginsv1alpha1.PluginRegistration{
		ObjectMeta: metav1.ObjectMeta{Name: name},
		Spec:       pluginsv1alpha1.PluginRegistrationSpec{Sections: []string{section}},
	}
}

func scheme(t *testing.T) *runtime.Scheme {
	t.Helper()

	s := runtime.NewScheme()
	require.NoError(t, pluginsv1alpha1.AddToScheme(s))

	return s
}

// previewRequests returns the requests that the component of in renders,
// in registration order.
func previewRequests(t *testing.T, in Input) []*pluginsv1alpha1.PluginRequest {
	t.Helper()

	comp, err := Component(claim(), in)
	require.NoError(t, err)

	objects, err := comp.Preview()
	require.NoError(t, err)

	requests := make([]*pluginsv1alpha1.PluginRequest, 0, len(objects))
	for _, obj := range objects {
		request, ok := obj.(*pluginsv1alpha1.PluginRequest)
		require.True(t, ok, "unexpected %T", obj)
		requests = append(requests, request)
	}

	return requests
}
