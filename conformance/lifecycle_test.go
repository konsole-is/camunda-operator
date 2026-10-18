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

package conformance

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"

	fleetv1alpha1 "github.com/konsole-is/camunda-operator/api/fleet/v1alpha1"
	pluginsv1alpha1 "github.com/konsole-is/camunda-operator/api/plugins/v1alpha1"
)

func TestRunsOfRunsAnEmptyPolicyOnceForEachSupportedPolicy(t *testing.T) {
	runs, err := runsOf(
		[]Case{{Name: "basic"}, {Name: "kept", DeletionPolicy: pluginsv1alpha1.DeletionPolicyOrphan}},
		[]pluginsv1alpha1.DeletionPolicy{pluginsv1alpha1.DeletionPolicyDelete, pluginsv1alpha1.DeletionPolicyOrphan},
	)
	require.NoError(t, err)

	names := make([]string, 0, len(runs))
	for _, r := range runs {
		names = append(names, r.name)
	}
	assert.Equal(t, []string{"basic-delete", "basic-orphan", "kept-orphan"}, names)
	assert.Equal(t, pluginsv1alpha1.DeletionPolicyOrphan, runs[1].policy)
	assert.Equal(t, "basic", runs[1].tc.Name)
}

func TestRunsOfRefusesCaseNamesThatGiveOneRequestName(t *testing.T) {
	_, err := runsOf(
		[]Case{{Name: "Basic"}, {Name: "basic"}},
		[]pluginsv1alpha1.DeletionPolicy{pluginsv1alpha1.DeletionPolicyDelete},
	)

	assert.ErrorContains(t, err, `case "basic" gives the request name "basic-delete", which case "Basic" also gives`)
}

func TestRunsOfRefusesARequestNameThatIsNotADNSLabel(t *testing.T) {
	_, err := runsOf(
		[]Case{{Name: "with space"}},
		[]pluginsv1alpha1.DeletionPolicy{pluginsv1alpha1.DeletionPolicyDelete},
	)

	assert.ErrorContains(t, err, `case "with space"`)
}

func TestRequestForWritesTheRequestOfARun(t *testing.T) {
	params := &apiextensionsv1.JSON{Raw: []byte(`{"message":"hi"}`)}
	run := requestRun{
		name:   "basic-orphan",
		policy: pluginsv1alpha1.DeletionPolicyOrphan,
		tc: Case{
			Name:       "basic",
			Inputs:     map[string]string{"source": "hello"},
			Parameters: params,
			Target:     pluginsv1alpha1.TargetRef{Kind: "CamundaCluster", Name: "echo-target"},
		},
	}
	section := &pluginsv1alpha1.PluginSection{
		Spec: pluginsv1alpha1.PluginSectionSpec{
			AppliesTo: []pluginsv1alpha1.ClaimKind{pluginsv1alpha1.ClaimKindCamundaClusterClaim},
		},
	}
	section.Name = "echo"

	req := requestFor("echo-1", section, "ns", run)

	assert.Equal(t, "basic-orphan", req.Name)
	assert.Equal(t, "ns", req.Namespace)
	assert.Equal(t, map[string]string{
		pluginsv1alpha1.LabelSection:      "echo",
		pluginsv1alpha1.LabelRegistration: "echo-1",
		fleetv1alpha1.LabelClaim:          claimName,
	}, req.Labels)
	assert.Equal(t, pluginsv1alpha1.PluginRequestSpec{
		Section:        "echo",
		Registration:   "echo-1",
		ClaimRef:       pluginsv1alpha1.ClaimRef{Kind: pluginsv1alpha1.ClaimKindCamundaClusterClaim, Name: claimName},
		Target:         run.tc.Target,
		Inputs:         run.tc.Inputs,
		Parameters:     params,
		DeletionPolicy: pluginsv1alpha1.DeletionPolicyOrphan,
	}, req.Spec)
}

func TestRequestForUsesThePlaneClaimForASectionOfPlanesOnly(t *testing.T) {
	section := &pluginsv1alpha1.PluginSection{
		Spec: pluginsv1alpha1.PluginSectionSpec{
			AppliesTo: []pluginsv1alpha1.ClaimKind{pluginsv1alpha1.ClaimKindCamundaManagementClusterClaim},
		},
	}

	req := requestFor("echo-1", section, "ns", requestRun{name: "basic-delete"})

	assert.Equal(t, pluginsv1alpha1.ClaimKindCamundaManagementClusterClaim, req.Spec.ClaimRef.Kind)
}
