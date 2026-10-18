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
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	authorizationv1 "k8s.io/api/authorization/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"

	pluginsv1alpha1 "github.com/konsole-is/camunda-operator/api/plugins/v1alpha1"
	v1 "github.com/konsole-is/camunda-operator/api/v1"
)

// staticCase is one registration that a check of the table runs on. An
// empty message means that the check passes.
type staticCase struct {
	name    string
	mutate  func(*pluginsv1alpha1.PluginRegistration)
	message string
}

// runStaticCases runs check on validRegistration, changed by each case,
// with the sections of testSections.
func runStaticCases(t *testing.T, check func(context.Context, *suite) error, tests []staticCase) {
	t.Helper()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reg := validRegistration()
			tt.mutate(reg)
			s := newTestSuite(t, validConfig(), interceptor.Funcs{}, append(testSections(), reg)...)

			err := check(t.Context(), s)

			if tt.message == "" {
				assert.NoError(t, err)
				return
			}
			assert.ErrorContains(t, err, tt.message)
		})
	}
}

func unchanged(*pluginsv1alpha1.PluginRegistration) {}

func TestAcceptedCheck(t *testing.T) {
	runStaticCases(t, checkAccepted, []staticCase{
		{"accepted and ready", unchanged, ""},
		{
			"not accepted",
			setCondition(pluginsv1alpha1.ConditionAccepted, metav1.ConditionFalse, "SectionMissing", "section x is missing"),
			"status.conditions[Accepted] is False, reason SectionMissing: section x is missing",
		},
		{
			"not ready",
			setCondition(v1.ConditionReady, metav1.ConditionFalse, "CredentialsMissing", "no key"),
			"status.conditions[Ready] is False, reason CredentialsMissing: no key",
		},
		{"no Accepted condition", removeCondition(pluginsv1alpha1.ConditionAccepted), "status.conditions[Accepted] is missing"},
		{"no Ready condition", removeCondition(v1.ConditionReady), "status.conditions[Ready] is missing"},
	})
}

func TestAcceptedCheckNamesBothConditions(t *testing.T) {
	reg := validRegistration()
	removeCondition(pluginsv1alpha1.ConditionAccepted)(reg)
	removeCondition(v1.ConditionReady)(reg)
	s := newTestSuite(t, validConfig(), interceptor.Funcs{}, reg)

	err := checkAccepted(t.Context(), s)

	assert.ErrorContains(t, err, "status.conditions[Accepted] is missing")
	assert.ErrorContains(t, err, "status.conditions[Ready] is missing")
}

func TestOutputsCheck(t *testing.T) {
	runStaticCases(t, checkOutputs, []staticCase{
		{"covers every output", unchanged, ""},
		{
			"misses an output",
			func(r *pluginsv1alpha1.PluginRegistration) {
				r.Spec.Outputs = []pluginsv1alpha1.OutputDecl{{Key: "other"}}
			},
			"spec.outputs misses the output value of section echo",
		},
		{
			"section not installed",
			func(r *pluginsv1alpha1.PluginRegistration) { r.Spec.Sections = []string{"missing"} },
			`spec.sections[0]: PluginSection "missing" not found`,
		},
	})
}

func TestInputsCheck(t *testing.T) {
	runStaticCases(t, checkInputs, []staticCase{
		{"reads an installed output", unchanged, ""},
		{
			"section not installed",
			func(r *pluginsv1alpha1.PluginRegistration) { r.Spec.Inputs[0].From = "missing/value" },
			"spec.inputs[0].from: section missing is not installed",
		},
		{
			"output of no section",
			func(r *pluginsv1alpha1.PluginRegistration) { r.Spec.Inputs[0].From = "source/other" },
			"spec.inputs[0].from: section source has no output other",
		},
		{
			"no separator",
			func(r *pluginsv1alpha1.PluginRegistration) { r.Spec.Inputs[0].From = "nosep" },
			`spec.inputs[0].from: "nosep" is not <section>/<output>`,
		},
		{
			"empty output",
			func(r *pluginsv1alpha1.PluginRegistration) { r.Spec.Inputs[0].From = "source/" },
			`spec.inputs[0].from: "source/" is not <section>/<output>`,
		},
	})
}

func TestDeletionCheck(t *testing.T) {
	runStaticCases(t, checkDeletion, []staticCase{
		{"holds Delete", unchanged, ""},
		{
			"only Orphan",
			func(r *pluginsv1alpha1.PluginRegistration) {
				r.Spec.DeletionPolicies = []pluginsv1alpha1.DeletionPolicy{pluginsv1alpha1.DeletionPolicyOrphan}
			},
			"spec.deletionPolicies does not hold Delete",
		},
	})
}

func TestRBACCheck(t *testing.T) {
	tests := []struct {
		name      string
		allowed   bool
		reason    string
		createErr error
		message   string
	}{
		{name: "allowed", allowed: true},
		{
			name:    "denied",
			message: "spec.serviceAccount echo-system/echo has no verb serve on pluginregistrations/echo",
		},
		{name: "denied with a reason", reason: "no RBAC policy matched", message: "no RBAC policy matched"},
		{name: "review fails", createErr: errors.New("forbidden"), message: "forbidden"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var review *authorizationv1.SubjectAccessReview
			funcs := interceptor.Funcs{
				Create: func(_ context.Context, _ client.WithWatch, obj client.Object, _ ...client.CreateOption) error {
					review = obj.(*authorizationv1.SubjectAccessReview)
					review.Status.Allowed = tt.allowed
					review.Status.Reason = tt.reason
					return tt.createErr
				},
			}
			s := newTestSuite(t, validConfig(), funcs, validRegistration())

			err := checkRBAC(t.Context(), s)

			require.NotNil(t, review)
			assert.Equal(t, "system:serviceaccount:echo-system:echo", review.Spec.User)
			assert.ElementsMatch(
				t,
				[]string{"system:serviceaccounts", "system:serviceaccounts:echo-system", "system:authenticated"},
				review.Spec.Groups,
			)
			assert.Equal(t, &authorizationv1.ResourceAttributes{
				Group:    "plugins.camunda.io",
				Resource: "pluginregistrations",
				Verb:     "serve",
				Name:     "echo",
			}, review.Spec.ResourceAttributes)

			if tt.message == "" {
				assert.NoError(t, err)
				return
			}
			assert.ErrorContains(t, err, tt.message)
		})
	}
}

func TestStaticChecksFailWhenTheRegistrationIsMissing(t *testing.T) {
	for _, c := range staticChecks {
		t.Run(string(c.ID), func(t *testing.T) {
			s := newTestSuite(t, validConfig(), interceptor.Funcs{})

			assert.ErrorContains(t, c.run(t.Context(), s), `PluginRegistration "echo"`)
		})
	}
}

func TestStaticChecksRunInContractOrder(t *testing.T) {
	var ids []CheckID
	for _, c := range staticChecks {
		ids = append(ids, c.ID)
		assert.False(t, c.Skippable, "check %s is mandatory", c.ID)
	}

	assert.Equal(t, []CheckID{RegAccepted, RegOutputs, RegInputs, RegDeletion, RegRBAC}, ids)
}
