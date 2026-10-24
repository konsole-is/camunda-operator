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

package existing

import (
	"testing"

	"github.com/sourcehawk/operator-component-framework/pkg/component"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	pluginsv1alpha1 "github.com/konsole-is/camunda-operator/api/plugins/v1alpha1"
	v1 "github.com/konsole-is/camunda-operator/api/v1"
	"github.com/konsole-is/camunda-operator/internal/controller/plugins/builtin/adapter"
	"github.com/konsole-is/camunda-operator/internal/pluginstartup"
	"github.com/konsole-is/camunda-operator/pkg/conditions"
)

func TestExistingHandlersReadTheNamedContract(t *testing.T) {
	tests := []struct {
		registration, kind, output, identity string
	}{
		{
			pluginstartup.RegistrationExistingObjectStorage, adapter.KindObjectStorageConfig,
			pluginstartup.OutputObjectStorageConfig,
			"core.camunda.io/v1/ObjectStorageConfig/my-cluster-ns/my-bucket",
		},
		{
			pluginstartup.RegistrationExistingSecondaryStorage, adapter.KindSecondaryStorageConfig,
			pluginstartup.OutputSecondaryStorageConfig,
			"core.camunda.io/v1/SecondaryStorageConfig/my-cluster-ns/my-bucket",
		},
	}

	for _, tt := range tests {
		t.Run(tt.registration, func(t *testing.T) {
			request := requestFor(tt.registration, `{"name":"my-bucket"}`)
			h := handlerFor(tt.registration)

			s, err := h.Build(request, nil)
			require.NoError(t, err)

			assert.Nil(t, s.Backing, "an existing handler writes nothing")
			_, ok := s.Outputs.Resource(tt.identity)
			assert.True(t, ok)
			assert.Equal(
				t,
				[]adapter.ContractRef{{Output: tt.output, Kind: tt.kind, Name: "my-bucket"}},
				h.Contracts(request),
			)
		})
	}
}

func TestDatabaseConfigsReadEachNamedConfig(t *testing.T) {
	request := requestFor(pluginstartup.RegistrationExistingDatabaseConfigs, `{"webModeler":"wm","keycloak":"kc"}`)
	h := handlerFor(pluginstartup.RegistrationExistingDatabaseConfigs)

	s, err := h.Build(request, nil)
	require.NoError(t, err)

	assert.Nil(t, s.Backing)
	assert.Equal(
		t, []adapter.ContractRef{
			{Output: "keycloakDatabaseConfig", Kind: adapter.KindDatabaseConfig, Name: "kc"},
			{Output: "webModelerDatabaseConfig", Kind: adapter.KindDatabaseConfig, Name: "wm"},
		}, h.Contracts(request),
	)
	for _, name := range []string{"kc", "wm"} {
		_, ok := s.Outputs.Resource("core.camunda.io/v1/DatabaseConfig/my-cluster-ns/" + name)
		assert.True(t, ok, name)
	}
}

func TestDatabaseConfigsRefuseEqualNames(t *testing.T) {
	request := requestFor(
		pluginstartup.RegistrationExistingDatabaseConfigs,
		`{"keycloak":"shared","identity":"shared","webModeler":"wm"}`,
	)

	_, err := handlerFor(pluginstartup.RegistrationExistingDatabaseConfigs).Build(request, nil)

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
	h := handlerFor(pluginstartup.RegistrationExistingDatabaseConfigs)

	s, err := h.Build(request, nil)
	require.NoError(t, err)
	reconcileOutputs(t, s, request, fakeReader(t))

	assert.Empty(t, h.Contracts(request))
	assert.Equal(t, metav1.ConditionTrue, s.Outputs.GetCondition(request).Status)
	assert.Empty(t, s.Report().Outputs)
}

func TestExistingPublishesTheOutputAndTheReadyStateOfTheContract(t *testing.T) {
	request := requestFor(pluginstartup.RegistrationExistingObjectStorage, `{"name":"my-bucket"}`)
	s, err := handlerFor(pluginstartup.RegistrationExistingObjectStorage).Build(request, nil)
	require.NoError(t, err)

	reconcileOutputs(t, s, request, fakeReader(t, objectStorageConfig("my-bucket", &metav1.Condition{
		Type: v1.ConditionReady, Status: metav1.ConditionTrue, Reason: v1.ReasonHealthy,
	})))

	assert.Equal(t, metav1.ConditionTrue, s.Outputs.GetCondition(request).Status)
	assert.Equal(t, map[string]string{pluginstartup.OutputObjectStorageConfig: "my-bucket"}, s.Report().Outputs)
	assert.Equal(
		t, []pluginsv1alpha1.SummaryItem{{
			Label: adapter.KindObjectStorageConfig, Value: "my-bucket, Ready", Severity: pluginsv1alpha1.SeverityInfo,
		}}, s.Report().Summary,
	)
	assert.Empty(t, s.Report().Resources)
}

func TestExistingWaitsForAMissingContract(t *testing.T) {
	request := requestFor(pluginstartup.RegistrationExistingObjectStorage, `{"name":"my-bucket"}`)
	s, err := handlerFor(pluginstartup.RegistrationExistingObjectStorage).Build(request, nil)
	require.NoError(t, err)

	reconcileOutputs(t, s, request, fakeReader(t))

	cond := s.Outputs.GetCondition(request)
	assert.Equal(t, metav1.ConditionFalse, cond.Status)
	assert.Equal(t, string(component.GuardBlocked), cond.Reason)
	assert.Equal(
		t, []pluginsv1alpha1.SummaryItem{{
			Label: adapter.KindObjectStorageConfig, Value: "my-bucket, waiting", Severity: pluginsv1alpha1.SeverityWarning,
		}}, s.Report().Summary,
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
			s, err := handlerFor(pluginstartup.RegistrationExistingObjectStorage).Build(request, nil)
			require.NoError(t, err)

			reconcileOutputs(t, s, request, fakeReader(t, objectStorageConfig("my-bucket", tt.ready)))

			assert.Equal(
				t, []pluginsv1alpha1.SummaryItem{{
					Label: adapter.KindObjectStorageConfig, Value: tt.want, Severity: pluginsv1alpha1.SeverityWarning,
				}}, s.Report().Summary,
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

			c, err := handlerFor(pluginstartup.RegistrationExistingObjectStorage).Cleanup(request, policy, nil)
			require.NoError(t, err)

			assert.Nil(t, c.Backing)
			assert.Empty(t, c.Remaining)
		})
	}
}

func TestExistingHasNoBackingObjects(t *testing.T) {
	for _, name := range []string{
		pluginstartup.RegistrationExistingObjectStorage,
		pluginstartup.RegistrationExistingSecondaryStorage,
		pluginstartup.RegistrationExistingDatabaseConfigs,
	} {
		t.Run(name, func(t *testing.T) {
			assert.Nil(t, handlerFor(name).Backing(requestFor(name, `{"name":"my-bucket"}`)))
		})
	}
}

func TestExistingReadsOnlyTheExactParameterNames(t *testing.T) {
	tests := []struct {
		registration, parameters string
		want                     []adapter.ContractRef
	}{
		{
			pluginstartup.RegistrationExistingObjectStorage, `{"name":"bucket-a","NAME":"bucket-b"}`,
			[]adapter.ContractRef{{
				Output: pluginstartup.OutputObjectStorageConfig, Kind: adapter.KindObjectStorageConfig, Name: "bucket-a",
			}},
		},
		{
			pluginstartup.RegistrationExistingObjectStorage, `{"name":"bucket-a","NAME":3}`,
			[]adapter.ContractRef{{
				Output: pluginstartup.OutputObjectStorageConfig, Kind: adapter.KindObjectStorageConfig, Name: "bucket-a",
			}},
		},
		{
			pluginstartup.RegistrationExistingDatabaseConfigs, `{"KEYCLOAK":"kc","identity":"id"}`,
			[]adapter.ContractRef{{
				Output: "identityDatabaseConfig", Kind: adapter.KindDatabaseConfig, Name: "id",
			}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.parameters, func(t *testing.T) {
			assert.Equal(t, tt.want, handlerFor(tt.registration).Contracts(requestFor(tt.registration, tt.parameters)))
		})
	}
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
