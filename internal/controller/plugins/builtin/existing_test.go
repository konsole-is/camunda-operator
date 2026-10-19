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

package builtin

import (
	"testing"

	"github.com/sourcehawk/operator-component-framework/pkg/component"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	pluginsv1alpha1 "github.com/konsole-is/camunda-operator/api/plugins/v1alpha1"
	v1 "github.com/konsole-is/camunda-operator/api/v1"
	"github.com/konsole-is/camunda-operator/internal/pluginstartup"
	"github.com/konsole-is/camunda-operator/pkg/conditions"
)

func TestExistingHandlersReadTheNamedContract(t *testing.T) {
	tests := []struct {
		registration, kind, output, identity string
	}{
		{
			pluginstartup.RegistrationExistingObjectStorage, kindObjectStorageConfig,
			pluginstartup.OutputObjectStorageConfig,
			"core.camunda.io/v1/ObjectStorageConfig/my-cluster-ns/my-bucket",
		},
		{
			pluginstartup.RegistrationExistingSecondaryStorage, kindSecondaryStorageConfig,
			pluginstartup.OutputSecondaryStorageConfig,
			"core.camunda.io/v1/SecondaryStorageConfig/my-cluster-ns/my-bucket",
		},
	}

	for _, tt := range tests {
		t.Run(tt.registration, func(t *testing.T) {
			request := requestFor(tt.registration, `{"name":"my-bucket"}`)
			h := handlers()[tt.registration]

			s, err := h.build(request)
			require.NoError(t, err)

			assert.Nil(t, s.backing, "an existing handler writes nothing")
			_, ok := s.outputs.Resource(tt.identity)
			assert.True(t, ok)
			assert.Equal(t, []contractRef{{output: tt.output, kind: tt.kind, name: "my-bucket"}}, h.contracts(request))
		})
	}
}

func TestDatabaseConfigsReadEachNamedConfig(t *testing.T) {
	request := requestFor(pluginstartup.RegistrationExistingDatabaseConfigs, `{"webModeler":"wm","keycloak":"kc"}`)
	h := handlers()[pluginstartup.RegistrationExistingDatabaseConfigs]

	s, err := h.build(request)
	require.NoError(t, err)

	assert.Nil(t, s.backing)
	assert.Equal(
		t, []contractRef{
			{output: pluginstartup.OutputKeycloakDatabaseConfig, kind: kindDatabaseConfig, name: "kc"},
			{output: pluginstartup.OutputWebModelerDatabaseConfig, kind: kindDatabaseConfig, name: "wm"},
		}, h.contracts(request),
	)
	for _, name := range []string{"kc", "wm"} {
		_, ok := s.outputs.Resource("core.camunda.io/v1/DatabaseConfig/my-cluster-ns/" + name)
		assert.True(t, ok, name)
	}
}

func TestDatabaseConfigsRefuseEqualNames(t *testing.T) {
	request := requestFor(
		pluginstartup.RegistrationExistingDatabaseConfigs,
		`{"keycloak":"shared","identity":"shared","webModeler":"wm"}`,
	)

	_, err := handlers()[pluginstartup.RegistrationExistingDatabaseConfigs].build(request)

	var failure *conditions.PreCheckFailure
	require.ErrorAs(t, err, &failure)
	assert.Equal(t, pluginsv1alpha1.ReasonInvalidParameters, failure.Reason)
	assert.Contains(t, failure.Message, "spec.parameters.keycloak")
	assert.Contains(t, failure.Message, "spec.parameters.identity")
	assert.Contains(t, failure.Message, "shared")
	assert.NotContains(t, failure.Message, "spec.parameters.webModeler")
}

func TestDatabaseConfigsWithoutNamesReadNothing(t *testing.T) {
	request := requestFor(pluginstartup.RegistrationExistingDatabaseConfigs, `{}`)
	h := handlers()[pluginstartup.RegistrationExistingDatabaseConfigs]

	s, err := h.build(request)
	require.NoError(t, err)
	reconcileOutputs(t, s, request, fakeReader(t))

	assert.Empty(t, h.contracts(request))
	assert.Equal(t, metav1.ConditionTrue, s.outputs.GetCondition(request).Status)
	assert.Empty(t, s.report().outputs)
}

func TestExistingPublishesTheOutputAndTheReadyStateOfTheContract(t *testing.T) {
	request := requestFor(pluginstartup.RegistrationExistingObjectStorage, `{"name":"my-bucket"}`)
	s, err := handlers()[pluginstartup.RegistrationExistingObjectStorage].build(request)
	require.NoError(t, err)

	reconcileOutputs(t, s, request, fakeReader(t, objectStorageConfig("my-bucket", &metav1.Condition{
		Type: v1.ConditionReady, Status: metav1.ConditionTrue, Reason: v1.ReasonHealthy,
	})))

	assert.Equal(t, metav1.ConditionTrue, s.outputs.GetCondition(request).Status)
	assert.Equal(t, map[string]string{pluginstartup.OutputObjectStorageConfig: "my-bucket"}, s.report().outputs)
	assert.Equal(
		t, []pluginsv1alpha1.SummaryItem{{
			Label: kindObjectStorageConfig, Value: "my-bucket, Ready", Severity: pluginsv1alpha1.SeverityInfo,
		}}, s.report().summary,
	)
	assert.Empty(t, s.report().resources)
}

func TestExistingWaitsForAMissingContract(t *testing.T) {
	request := requestFor(pluginstartup.RegistrationExistingObjectStorage, `{"name":"my-bucket"}`)
	s, err := handlers()[pluginstartup.RegistrationExistingObjectStorage].build(request)
	require.NoError(t, err)

	reconcileOutputs(t, s, request, fakeReader(t))

	cond := s.outputs.GetCondition(request)
	assert.Equal(t, metav1.ConditionFalse, cond.Status)
	assert.Equal(t, string(component.GuardBlocked), cond.Reason)
	assert.Equal(
		t, []pluginsv1alpha1.SummaryItem{{
			Label: kindObjectStorageConfig, Value: "my-bucket, waiting", Severity: pluginsv1alpha1.SeverityWarning,
		}}, s.report().summary,
	)
}

func TestExistingReportsAContractThatIsNotReady(t *testing.T) {
	tests := []struct {
		name  string
		ready *metav1.Condition
		want  string
	}{
		{
			"Ready is False",
			&metav1.Condition{Type: v1.ConditionReady, Status: metav1.ConditionFalse, Reason: v1.ReasonMissingSecret},
			"my-bucket, not Ready (MissingSecret)",
		},
		{"no Ready condition", nil, "my-bucket, not Ready"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			request := requestFor(pluginstartup.RegistrationExistingObjectStorage, `{"name":"my-bucket"}`)
			s, err := handlers()[pluginstartup.RegistrationExistingObjectStorage].build(request)
			require.NoError(t, err)

			reconcileOutputs(t, s, request, fakeReader(t, objectStorageConfig("my-bucket", tt.ready)))

			assert.Equal(
				t, []pluginsv1alpha1.SummaryItem{{
					Label: kindObjectStorageConfig, Value: tt.want, Severity: pluginsv1alpha1.SeverityWarning,
				}}, s.report().summary,
			)
		})
	}
}

func TestExistingCleanupWritesNothing(t *testing.T) {
	for _, policy := range []pluginsv1alpha1.DeletionPolicy{
		pluginsv1alpha1.DeletionPolicyDelete,
		pluginsv1alpha1.DeletionPolicyOrphan,
	} {
		t.Run(string(policy), func(t *testing.T) {
			request := requestFor(pluginstartup.RegistrationExistingObjectStorage, `{"name":"my-bucket"}`)

			c, err := handlers()[pluginstartup.RegistrationExistingObjectStorage].cleanup(request, policy)
			require.NoError(t, err)

			assert.Empty(t, c.components)
			assert.Empty(t, c.remaining)
		})
	}
}

func TestContractsOfUnparsableParametersAreEmpty(t *testing.T) {
	for name, h := range handlers() {
		t.Run(name, func(t *testing.T) {
			assert.Empty(t, h.contracts(requestFor(name, `{"name":3,"keycloak":3}`)))
		})
	}
}

// reconcileOutputs reconciles the outputs component of s against c.
func reconcileOutputs(t *testing.T, s served, request *pluginsv1alpha1.PluginRequest, c client.Client) {
	require.NoError(t, s.outputs.Reconcile(t.Context(), component.ReconcileContext{
		Client: c, Scheme: c.Scheme(), Owner: request,
	}))
}

// objectStorageConfig returns an ObjectStorageConfig of name in the namespace
// of requestFor, with ready as its only condition when it is set.
func objectStorageConfig(name string, ready *metav1.Condition) *v1.ObjectStorageConfig {
	osc := &v1.ObjectStorageConfig{ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: "my-cluster-ns"}}
	if ready != nil {
		osc.Status.Conditions = []metav1.Condition{*ready}
	}

	return osc
}
