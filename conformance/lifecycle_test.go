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
	"maps"
	"slices"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/validation"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"

	fleetv1alpha1 "github.com/konsole-is/camunda-operator/api/fleet/v1alpha1"
	pluginsv1alpha1 "github.com/konsole-is/camunda-operator/api/plugins/v1alpha1"
)

func TestRunsOfRunsAnEmptyPolicyOnceForEachSupportedPolicy(t *testing.T) {
	runs, err := runsOf(
		[]Case{{Name: "basic"}, {Name: "kept", DeletionPolicy: pluginsv1alpha1.DeletionPolicyOrphan}},
		[]string{"echo"},
		[]pluginsv1alpha1.DeletionPolicy{pluginsv1alpha1.DeletionPolicyDelete, pluginsv1alpha1.DeletionPolicyOrphan},
		"",
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

func TestRunsOfRunsACaseOnceForEachSectionOfARegistrationWithSeveral(t *testing.T) {
	runs, err := runsOf(
		[]Case{{Name: "basic", DeletionPolicy: pluginsv1alpha1.DeletionPolicyDelete}},
		[]string{"backup-storage", "access"},
		[]pluginsv1alpha1.DeletionPolicy{pluginsv1alpha1.DeletionPolicyDelete},
		"",
	)
	require.NoError(t, err)

	require.Len(t, runs, 2)
	assert.Equal(t, "basic-backup-storage-delete", runs[0].name)
	assert.Equal(t, "backup-storage", runs[0].section)
	assert.Equal(t, "basic-access-delete", runs[1].name)
	assert.Equal(t, "access", runs[1].section)
}

func TestRunsOfGivesEachPolicyVariantOfACaseATargetOfItsOwn(t *testing.T) {
	target := pluginsv1alpha1.TargetRef{
		Kind:               "CamundaCluster",
		Name:               "echo-target",
		ServiceAccountName: "echo-target-camunda",
	}
	runs, err := runsOf(
		[]Case{
			{Name: "basic", Target: target},
			{Name: "kept", Target: target, DeletionPolicy: pluginsv1alpha1.DeletionPolicyOrphan},
		},
		[]string{"echo"},
		[]pluginsv1alpha1.DeletionPolicy{pluginsv1alpha1.DeletionPolicyDelete, pluginsv1alpha1.DeletionPolicyOrphan},
		"",
	)
	require.NoError(t, err)

	require.Len(t, runs, 3)
	assert.Equal(t, "echo-target-delete", runs[0].tc.Target.Name)
	assert.Equal(t, "echo-target-camunda-delete", runs[0].tc.Target.ServiceAccountName)
	assert.Equal(t, "echo-target-orphan", runs[1].tc.Target.Name)
	assert.Equal(t, target, runs[2].tc.Target, "a case with its own policy keeps its target")
}

func TestRunsOfGivesEachPolicyOfACaseAClaimOfItsOwn(t *testing.T) {
	runs, err := runsOf(
		[]Case{{Name: "basic"}},
		[]string{"echo"},
		[]pluginsv1alpha1.DeletionPolicy{pluginsv1alpha1.DeletionPolicyDelete, pluginsv1alpha1.DeletionPolicyOrphan},
		"abc123",
	)
	require.NoError(t, err)

	require.Len(t, runs, 2)
	assert.Equal(t, "basic-delete-abc123", runs[0].claim)
	assert.Equal(t, "basic-orphan-abc123", runs[1].claim)
}

func TestRunsOfGivesTheSectionsOfOnePolicyOfACaseOneClaim(t *testing.T) {
	runs, err := runsOf(
		[]Case{{Name: "basic"}},
		[]string{"backup-storage", "access"},
		[]pluginsv1alpha1.DeletionPolicy{pluginsv1alpha1.DeletionPolicyDelete},
		"abc123",
	)
	require.NoError(t, err)

	require.Len(t, runs, 2)
	assert.Equal(t, "basic-delete-abc123", runs[0].claim)
	assert.Equal(t, runs[0].claim, runs[1].claim)
}

func TestRunsOfGivesClaimNamesThatAClusterClaimTakes(t *testing.T) {
	runs, err := runsOf(
		[]Case{{Name: "postgresql-with-backup-storage"}, {Name: "1st"}},
		[]string{"backup-storage", "access"},
		[]pluginsv1alpha1.DeletionPolicy{pluginsv1alpha1.DeletionPolicyDelete, pluginsv1alpha1.DeletionPolicyOrphan},
		"0123456789",
	)
	require.NoError(t, err)

	claims := map[string]string{}
	for _, r := range runs {
		assert.LessOrEqual(t, len(r.claim), 40, "claim %s", r.claim)
		assert.Empty(t, validation.IsDNS1035Label(r.claim), "claim %s", r.claim)

		key := r.caseName + "/" + string(r.policy)
		if other, ok := claims[key]; ok {
			assert.Equal(t, other, r.claim, "the sections of %s share a claim", key)
		}
		claims[key] = r.claim
	}

	assert.Len(t, claims, 4)
	assert.Len(t, slices.Compact(slices.Sorted(maps.Values(claims))), 4, "each case and policy has a claim of its own")
}

func TestRunsOfRefusesCaseNamesThatGiveOneRequestName(t *testing.T) {
	_, err := runsOf(
		[]Case{{Name: "Basic"}, {Name: "basic"}},
		[]string{"echo"},
		[]pluginsv1alpha1.DeletionPolicy{pluginsv1alpha1.DeletionPolicyDelete},
		"",
	)

	assert.ErrorContains(t, err, `case "basic" gives the request name "basic-delete", which case "Basic" also gives`)
}

func TestRunsOfRefusesARequestNameThatIsNotADNSLabel(t *testing.T) {
	_, err := runsOf(
		[]Case{{Name: "with space"}},
		[]string{"echo"},
		[]pluginsv1alpha1.DeletionPolicy{pluginsv1alpha1.DeletionPolicyDelete},
		"",
	)

	assert.ErrorContains(t, err, `case "with space"`)
}

func TestRequestForWritesTheRequestOfARun(t *testing.T) {
	params := &apiextensionsv1.JSON{Raw: []byte(`{"message":"hi"}`)}
	run := requestRun{
		name:   "basic-orphan",
		claim:  "basic-orphan-abc123",
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
	assert.Equal(
		t, map[string]string{
			pluginsv1alpha1.LabelSection:      "echo",
			pluginsv1alpha1.LabelRegistration: "echo-1",
			fleetv1alpha1.LabelClaim:          "basic-orphan-abc123",
		}, req.Labels,
	)
	assert.Equal(
		t, pluginsv1alpha1.PluginRequestSpec{
			Section:      "echo",
			Registration: "echo-1",
			ClaimRef: pluginsv1alpha1.ClaimRef{
				Kind: pluginsv1alpha1.ClaimKindCamundaClusterClaim,
				Name: "basic-orphan-abc123",
			},
			Target:         run.tc.Target,
			Inputs:         run.tc.Inputs,
			Parameters:     params,
			DeletionPolicy: pluginsv1alpha1.DeletionPolicyOrphan,
		}, req.Spec,
	)
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

func TestRunIDOfGivesEachNamespaceItsOwnRequestNames(t *testing.T) {
	policies := []pluginsv1alpha1.DeletionPolicy{pluginsv1alpha1.DeletionPolicyDelete}

	first, err := runsOf([]Case{{Name: "basic"}}, []string{"echo"}, policies, runIDOf("ci-plugin-a"))
	require.NoError(t, err)
	second, err := runsOf([]Case{{Name: "basic"}}, []string{"echo"}, policies, runIDOf("ci-plugin-b"))
	require.NoError(t, err)

	assert.Equal(t, "basic-delete-"+runIDOf("ci-plugin-a"), first[0].name)
	assert.NotEqual(t, first[0].name, second[0].name)
	assert.Equal(t, runIDOf("ci-plugin-a"), runIDOf("ci-plugin-a"), "the same namespace gives the same names")
}

func TestRunIDOfKeepsApartTwoNamespacesThatCollidedBefore(t *testing.T) {
	assert.NotEqual(t, runIDOf("ci-plugin-18"), runIDOf("ci-plugin-920"))
}

// The UIDs of a request that the run created and of a request that replaced
// it.
const (
	runUID         types.UID = "run-uid"
	replacementUID types.UID = "replacement-uid"
)

func TestDeleteRunObjectsKeepsTheObjectsOfAReplacement(t *testing.T) {
	replacement := &pluginsv1alpha1.PluginRequest{}
	replacement.Namespace = validConfig().Namespace
	replacement.Name = "basic-delete"
	replacement.UID = replacementUID
	cm := &corev1.ConfigMap{}
	cm.Namespace = validConfig().Namespace
	cm.Name = "basic-delete-echo"
	cm.Labels = map[string]string{pluginsv1alpha1.LabelRequest: "basic-delete"}
	s := newTestSuite(t, validConfig(), interceptor.Funcs{}, replacement, cm)
	created := replacement.DeepCopy()
	created.UID = runUID
	kinds := []kindInfo{{gvk: schema.GroupVersionKind{Version: "v1", Kind: "ConfigMap"}, namespaced: true}}

	require.NoError(t, s.deleteRunObjects(t.Context(), kinds, []*pluginsv1alpha1.PluginRequest{created}))

	assert.NoError(t, s.client.Get(t.Context(), client.ObjectKeyFromObject(cm), &corev1.ConfigMap{}))
}

func TestDeleteRunObjectsDeletesTheObjectsOfACreatedRequest(t *testing.T) {
	cm := &corev1.ConfigMap{}
	cm.Namespace = validConfig().Namespace
	cm.Name = "basic-delete-echo"
	cm.Labels = map[string]string{pluginsv1alpha1.LabelRequest: "basic-delete"}
	s := newTestSuite(t, validConfig(), interceptor.Funcs{}, cm)
	created := &pluginsv1alpha1.PluginRequest{}
	created.Namespace = validConfig().Namespace
	created.Name = "basic-delete"
	created.UID = runUID
	kinds := []kindInfo{{gvk: schema.GroupVersionKind{Version: "v1", Kind: "ConfigMap"}, namespaced: true}}

	require.NoError(t, s.deleteRunObjects(t.Context(), kinds, []*pluginsv1alpha1.PluginRequest{created}))

	err := s.client.Get(t.Context(), client.ObjectKeyFromObject(cm), &corev1.ConfigMap{})
	assert.True(t, apierrors.IsNotFound(err), "got %v", err)
}

func TestEnsureNamespaceWaitsForATerminatingNamespace(t *testing.T) {
	cfg := validConfig()
	cfg.Timeouts.Deletion = time.Second
	ns := &corev1.Namespace{}
	ns.Name = cfg.Namespace
	ns.Finalizers = []string{"kubernetes"}
	now := metav1.Now()
	ns.DeletionTimestamp = &now
	s := newTestSuite(t, cfg, interceptor.Funcs{}, ns)

	err := s.ensureNamespace(t.Context())

	assert.ErrorContains(t, err, "is still terminating")
}

func TestDeleteCreatedWaitsUntilTheRequestsAreGone(t *testing.T) {
	cfg := validConfig()
	cfg.Timeouts.Deletion = time.Second
	stuck := &pluginsv1alpha1.PluginRequest{}
	stuck.Namespace = cfg.Namespace
	stuck.Name = testRequestName
	stuck.UID = runUID
	stuck.Finalizers = []string{pluginsv1alpha1.FinalizerCleanup}
	s := newTestSuite(t, cfg, interceptor.Funcs{}, stuck)

	err := s.deleteCreated(t.Context(), []*pluginsv1alpha1.PluginRequest{stuck.DeepCopy()})

	assert.ErrorContains(t, err, "still exists")
}
