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

	"github.com/stretchr/testify/require"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"

	pluginsv1alpha1 "github.com/konsole-is/camunda-operator/api/plugins/v1alpha1"
	v1 "github.com/konsole-is/camunda-operator/api/v1"
)

// validRegistration returns a registration that passes every static check
// with the objects of testSections, for the tests to break one field at a
// time. validConfig names it.
func validRegistration() *pluginsv1alpha1.PluginRegistration {
	reg := &pluginsv1alpha1.PluginRegistration{
		ObjectMeta: metav1.ObjectMeta{Name: "echo"},
		Spec: pluginsv1alpha1.PluginRegistrationSpec{
			Sections:       []string{"echo"},
			Label:          "Echo",
			Mode:           pluginsv1alpha1.ModeOptional,
			ServiceAccount: pluginsv1alpha1.ServiceAccountRef{Namespace: "echo-system", Name: "echo"},
			Parameters: &apiextensionsv1.JSON{
				Raw: []byte(`{"type":"object","properties":{"message":{"type":"string"}}}`),
			},
			Inputs:  []pluginsv1alpha1.InputDecl{{Name: "source", From: "source/value"}},
			Outputs: []pluginsv1alpha1.OutputDecl{{Key: "value"}},
			GrantTemplates: []pluginsv1alpha1.GrantTemplate{{
				Name:     "read",
				Format:   pluginsv1alpha1.GrantFormatAWSIAMStatement,
				Template: &apiextensionsv1.JSON{Raw: []byte(`{"Resource":"arn:aws:s3:::{{ .outputs.value }}"}`)},
			}},
			DeletionPolicies: []pluginsv1alpha1.DeletionPolicy{
				pluginsv1alpha1.DeletionPolicyDelete,
				pluginsv1alpha1.DeletionPolicyOrphan,
			},
			MetricQueries: []pluginsv1alpha1.MetricQuery{{
				Label: "Up",
				Query: `up{namespace="{{ .namespace }}",pod=~"{{ .pods }}"}`,
			}},
		},
	}
	setCondition(pluginsv1alpha1.ConditionAccepted, metav1.ConditionTrue, "Accepted", "")(reg)
	setCondition(v1.ConditionReady, metav1.ConditionTrue, "Ready", "")(reg)

	return reg
}

// testSections are the sections of validRegistration: echo, which it
// serves, and source, which its input reads.
func testSections() []client.Object {
	return []client.Object{
		&pluginsv1alpha1.PluginSection{
			ObjectMeta: metav1.ObjectMeta{Name: "echo"},
			Spec:       pluginsv1alpha1.PluginSectionSpec{Outputs: []pluginsv1alpha1.OutputDecl{{Key: "value"}}},
		},
		&pluginsv1alpha1.PluginSection{
			ObjectMeta: metav1.ObjectMeta{Name: "source"},
			Spec:       pluginsv1alpha1.PluginSectionSpec{Outputs: []pluginsv1alpha1.OutputDecl{{Key: "value"}}},
		},
	}
}

// newTestSuite returns the suite of cfg on a fake cluster that holds objs,
// with funcs around the fake client.
func newTestSuite(t *testing.T, cfg Config, funcs interceptor.Funcs, objs ...client.Object) *suite {
	t.Helper()

	scheme, err := newScheme()
	require.NoError(t, err)

	c := fake.NewClientBuilder().
		WithScheme(scheme).
		WithStatusSubresource(&pluginsv1alpha1.PluginRegistration{}, &pluginsv1alpha1.Grant{}).
		WithObjects(objs...).
		Build()

	return newSuite(cfg, nil, interceptor.NewClient(c, funcs))
}

func setCondition(
	condType string,
	status metav1.ConditionStatus,
	reason string,
	message string,
) func(*pluginsv1alpha1.PluginRegistration) {
	return func(reg *pluginsv1alpha1.PluginRegistration) {
		meta.SetStatusCondition(&reg.Status.Conditions, metav1.Condition{
			Type:    condType,
			Status:  status,
			Reason:  reason,
			Message: message,
		})
	}
}

func removeCondition(condType string) func(*pluginsv1alpha1.PluginRegistration) {
	return func(reg *pluginsv1alpha1.PluginRegistration) {
		meta.RemoveStatusCondition(&reg.Status.Conditions, condType)
	}
}
