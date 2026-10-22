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
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"

	pluginsv1alpha1 "github.com/konsole-is/camunda-operator/api/plugins/v1alpha1"
	v1 "github.com/konsole-is/camunda-operator/api/v1"
)

func TestMissingInputs(t *testing.T) {
	assert.Empty(t, missingInputs(map[string]string{"a": "x"}, []string{"a"}))
	assert.Equal(t, []string{"a", "b"}, missingInputs(map[string]string{"a": ""}, []string{"a", "b"}))
	assert.Empty(t, missingInputs(nil, nil))
}

func TestSameJSON(t *testing.T) {
	tests := []struct {
		name string
		a, b string
		same bool
	}{
		{"key order", `{"a":1,"b":[1,2]}`, `{"b":[1,2],"a":1}`, true},
		{"number form", `{"a":1}`, `{"a":1.0}`, true},
		{"escaped html", `{"a":"<&>"}`, `{"a":"<&>"}`, true},
		{"exponent", `{"a":1e2}`, `{"a":100}`, true},
		{"large integers", `{"a":9007199254740992}`, `{"a":9007199254740993}`, false},
		{"number and string", `{"a":1}`, `{"a":"1"}`, false},
		{"array order", `{"a":[1,2]}`, `{"a":[2,1]}`, false},
		{"extra key", `{"a":1}`, `{"a":1,"b":2}`, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			same, err := sameJSON([]byte(tt.a), []byte(tt.b))

			require.NoError(t, err)
			assert.Equal(t, tt.same, same)
		})
	}
}

func TestSameJSONRefusesADocumentThatIsNotJSON(t *testing.T) {
	_, err := sameJSON([]byte(`{`), []byte(`{}`))

	assert.Error(t, err)
}

// grantCheckRequest is a request of the claim orders in team-a, with the
// output bucket and the parameter prefix.
func grantCheckRequest() *pluginsv1alpha1.PluginRequest {
	req := &pluginsv1alpha1.PluginRequest{
		ObjectMeta: metav1.ObjectMeta{Namespace: "team-a", Name: "r", UID: "uid-r"},
		Spec: pluginsv1alpha1.PluginRequestSpec{
			Registration: "echo",
			ClaimRef:     pluginsv1alpha1.ClaimRef{Kind: pluginsv1alpha1.ClaimKindCamundaClusterClaim, Name: "orders"},
			Target: pluginsv1alpha1.TargetRef{
				Kind:               "CamundaCluster",
				Name:               "orders",
				ServiceAccountName: "orders-camunda",
			},
			Parameters: &apiextensionsv1.JSON{
				Raw: []byte(`{"prefix":"{{ .namespace }}/data","size":12345678901234567890}`),
			},
		},
	}
	req.Status.Outputs = map[string]string{"bucket": "orders-bucket"}

	return req
}

func grantTemplate(template string) pluginsv1alpha1.GrantTemplate {
	return pluginsv1alpha1.GrantTemplate{
		Name:     "read",
		Format:   pluginsv1alpha1.GrantFormatAWSIAMStatement,
		Template: &apiextensionsv1.JSON{Raw: []byte(template)},
	}
}

func TestExpectedFragmentFillsTheTemplateWithTheRequest(t *testing.T) {
	tmpl := grantTemplate(`{"Resource":"arn:aws:s3:::{{ .outputs.bucket }}/{{ .namespace }}/{{ .request }}",` +
		`"Condition":{"sa":"{{ .serviceAccount }}","claim":"{{ .claim }}","size":"{{ .parameters.size }}"}}`)

	got, err := expectedFragment(tmpl, nil, grantCheckRequest())

	require.NoError(t, err)
	want := `{"Resource":"arn:aws:s3:::orders-bucket/team-a/r",` +
		`"Condition":{"sa":"orders-camunda","claim":"orders","size":"12345678901234567890"}}`
	assert.JSONEq(t, want, string(got))
}

func TestExpectedFragmentRendersTemplateParametersFirst(t *testing.T) {
	parameters := &apiextensionsv1.JSON{Raw: []byte(
		`{"type":"object","properties":{"prefix":{"type":"string","x-fleet-template":true}}}`)}
	tmpl := grantTemplate(`{"Resource":"{{ .parameters.prefix }}"}`)

	got, err := expectedFragment(tmpl, parameters, grantCheckRequest())

	require.NoError(t, err)
	assert.JSONEq(t, `{"Resource":"team-a/data"}`, string(got))
}

func TestExpectedFragmentRefusesAnOutputInATemplateParameter(t *testing.T) {
	parameters := &apiextensionsv1.JSON{Raw: []byte(
		`{"type":"object","properties":{"prefix":{"type":"string","x-fleet-template":true}}}`)}
	tmpl := grantTemplate(`{"Resource":"{{ .parameters.prefix }}"}`)
	req := grantCheckRequest()
	req.Spec.Parameters = &apiextensionsv1.JSON{Raw: []byte(`{"prefix":"{{ .outputs.bucket }}/data"}`)}

	_, err := expectedFragment(tmpl, parameters, req)

	assert.ErrorContains(t, err, "spec.parameters.prefix")
	assert.ErrorContains(t, err, "is rendered before the plugin writes the outputs")
}

func TestExpectedFragmentKeepsAnUnmarkedParameterAsItIs(t *testing.T) {
	got, err := expectedFragment(grantTemplate(`{"Resource":"{{ .parameters.prefix }}"}`), nil, grantCheckRequest())

	require.NoError(t, err)
	assert.JSONEq(t, `{"Resource":"{{ .namespace }}/data"}`, string(got))
}

func TestExpectedFragmentNamesAMissingOutput(t *testing.T) {
	_, err := expectedFragment(grantTemplate(`{"Resource":"{{ .outputs.other }}"}`), nil, grantCheckRequest())

	assert.ErrorContains(t, err, `output "other" has no value`)
}

// goodGrant is the Grant of grantCheckRequest for the template read, with
// the fragment {"a":"b"}.
func goodGrant() pluginsv1alpha1.Grant {
	return pluginsv1alpha1.Grant{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "team-a",
			Name:      "r-read",
			Labels: map[string]string{
				pluginsv1alpha1.LabelRegistration: "echo",
				pluginsv1alpha1.LabelRequest:      "r",
				pluginsv1alpha1.LabelCluster:      "orders",
			},
			OwnerReferences: []metav1.OwnerReference{{
				APIVersion: pluginsv1alpha1.GroupVersion.String(),
				Kind:       kindPluginRequest,
				Name:       "r",
				UID:        "uid-r",
			}},
		},
		Spec: pluginsv1alpha1.GrantSpec{
			Cluster:  v1.ClusterRef{Name: "orders"},
			Template: "read",
			Format:   pluginsv1alpha1.GrantFormatAWSIAMStatement,
			Fragment: &apiextensionsv1.JSON{Raw: []byte(`{"a":"b"}`)},
		},
	}
}

func TestGrantMatchProblems(t *testing.T) {
	tests := []struct {
		name     string
		grants   func() []pluginsv1alpha1.Grant
		problems []string // each holds one substring, in order; empty: no problem
	}{
		{"match", func() []pluginsv1alpha1.Grant { return []pluginsv1alpha1.Grant{goodGrant()} }, nil},
		{
			"no grant",
			func() []pluginsv1alpha1.Grant { return nil },
			[]string{"no Grant of the template read carries the label plugins.camunda.io/request: r"},
		},
		{
			"two grants",
			func() []pluginsv1alpha1.Grant {
				other := goodGrant()
				other.Name = "r-read-2"
				return []pluginsv1alpha1.Grant{goodGrant(), other}
			},
			[]string{"2 Grants of the template read: r-read, r-read-2"},
		},
		{
			"wrong cluster label",
			func() []pluginsv1alpha1.Grant {
				g := goodGrant()
				g.Labels[pluginsv1alpha1.LabelCluster] = "other"
				return []pluginsv1alpha1.Grant{g}
			},
			[]string{`Grant r-read has the label plugins.camunda.io/cluster: "other", and the request needs "orders"`},
		},
		{
			"wrong registration label",
			func() []pluginsv1alpha1.Grant {
				g := goodGrant()
				delete(g.Labels, pluginsv1alpha1.LabelRegistration)
				return []pluginsv1alpha1.Grant{g}
			},
			[]string{`the label plugins.camunda.io/registration: "", and the request needs "echo"`},
		},
		{
			"wrong cluster reference",
			func() []pluginsv1alpha1.Grant {
				g := goodGrant()
				g.Spec.Cluster.Name = "other"
				return []pluginsv1alpha1.Grant{g}
			},
			[]string{`Grant r-read has spec.cluster.name "other", and the request needs "orders"`},
		},
		{
			"wrong format",
			func() []pluginsv1alpha1.Grant {
				g := goodGrant()
				g.Spec.Format = "gcp-iam-binding"
				return []pluginsv1alpha1.Grant{g}
			},
			[]string{"Grant r-read has the format gcp-iam-binding, and the template has aws-iam-statement"},
		},
		{
			"wrong fragment",
			func() []pluginsv1alpha1.Grant {
				g := goodGrant()
				g.Spec.Fragment = &apiextensionsv1.JSON{Raw: []byte(`{"a":"c"}`)}
				return []pluginsv1alpha1.Grant{g}
			},
			[]string{`Grant r-read has the fragment {"a":"c"}, and the filled-in template is {"a":"b"}`},
		},
		{
			"fragment in another key order",
			func() []pluginsv1alpha1.Grant {
				g := goodGrant()
				g.Spec.Fragment = &apiextensionsv1.JSON{Raw: []byte(` { "a" : "b" } `)}
				return []pluginsv1alpha1.Grant{g}
			},
			nil,
		},
		{
			"no fragment",
			func() []pluginsv1alpha1.Grant {
				g := goodGrant()
				g.Spec.Fragment = nil
				return []pluginsv1alpha1.Grant{g}
			},
			[]string{"Grant r-read has the fragment null"},
		},
		{
			"no owner",
			func() []pluginsv1alpha1.Grant {
				g := goodGrant()
				g.OwnerReferences = nil
				return []pluginsv1alpha1.Grant{g}
			},
			[]string{"Grant r-read has no owner reference to the request"},
		},
		{
			"no request label",
			func() []pluginsv1alpha1.Grant {
				g := goodGrant()
				delete(g.Labels, pluginsv1alpha1.LabelRequest)
				return []pluginsv1alpha1.Grant{g}
			},
			[]string{`Grant r-read has the label plugins.camunda.io/request: "", and the request needs "r"`},
		},
		{
			"owner with another name",
			func() []pluginsv1alpha1.Grant {
				g := goodGrant()
				g.OwnerReferences[0].Name = "another"
				return []pluginsv1alpha1.Grant{g}
			},
			[]string{"Grant r-read has no owner reference to the request"},
		},
		{
			"owner of another request",
			func() []pluginsv1alpha1.Grant {
				g := goodGrant()
				g.OwnerReferences[0].UID = "uid-other"
				return []pluginsv1alpha1.Grant{g}
			},
			[]string{"Grant r-read has no owner reference to the request"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := grantMatchProblems(grantCheckRequest(), grantTemplate(`{}`), []byte(`{"a":"b"}`), tt.grants())

			require.Len(t, got, len(tt.problems), "%v", got)
			for i, want := range tt.problems {
				assert.Contains(t, got[i], want)
			}
		})
	}
}

func TestUndeclaredTemplateProblems(t *testing.T) {
	undeclared := goodGrant()
	undeclared.Name = "r-extra"
	undeclared.Spec.Template = "extra"

	got := undeclaredTemplateProblems(
		[]pluginsv1alpha1.Grant{goodGrant(), undeclared},
		[]pluginsv1alpha1.GrantTemplate{grantTemplate(`{}`)},
	)

	assert.Equal(t, []string{"Grant r-extra names the template extra, which the registration does not declare"}, got)
}

func TestGrantChecksSkipARegistrationWithoutGrantTemplates(t *testing.T) {
	reg := validRegistration()
	reg.Spec.GrantTemplates = nil
	s := newTestSuite(t, validConfig(), interceptor.Funcs{}, append(testSections(), reg)...)

	for _, check := range []func(context.Context, *suite) error{checkGrantMatch, checkGrantRequires, checkGrantAdmission} {
		reason, skipped := skipReasonOf(check(t.Context(), s))

		assert.True(t, skipped)
		assert.Equal(t, "the registration has no grant templates", reason)
	}
}

func TestAdmissionVerdict(t *testing.T) {
	grants := schema.GroupResource{Group: "plugins.camunda.io", Resource: "grants"}
	policyRefusal := apierrors.NewForbidden(grants, "g", errors.New(
		"ValidatingAdmissionPolicy 'plugins.camunda.io-grant-author' with binding "+
			"'plugins.camunda.io-grant-author-binding' denied request: no serve",
	))
	rbacRefusal := apierrors.NewForbidden(grants, "g", errors.New(
		`User "system:serviceaccount:ns:sa" cannot create resource "grants"`,
	))

	policy := "plugins.camunda.io-grant-author"

	assert.NoError(t, admissionVerdict(policyRefusal, policy, "u", "echo"))
	assert.ErrorContains(t, admissionVerdict(nil, policy, "u", "echo"), "admitted a Grant of echo from u")
	assert.ErrorContains(t, admissionVerdict(rbacRefusal, policy, "u", "echo"), "not through the policy")
	assert.ErrorContains(
		t,
		admissionVerdict(policyRefusal, "camunda-operator-plugins.camunda.io-grant-author", "u", "echo"),
		"not through the policy camunda-operator-plugins.camunda.io-grant-author",
	)
}

func TestGrantAdmissionSkipsWhenThePolicyIsNotInstalled(t *testing.T) {
	s := newTestSuite(t, validConfig(), interceptor.Funcs{}, append(testSections(), validRegistration())...)

	reason, skipped := skipReasonOf(checkGrantAdmission(t.Context(), s))

	assert.True(t, skipped)
	want := "no ValidatingAdmissionPolicy whose name ends with plugins.camunda.io-grant-author is installed"
	assert.Equal(t, want, reason)
}

func TestGrantAuthorPolicyNameFindsAPrefixedPolicy(t *testing.T) {
	prefixed := &admissionregistrationv1.ValidatingAdmissionPolicy{
		ObjectMeta: metav1.ObjectMeta{Name: "camunda-operator-plugins.camunda.io-grant-author"},
	}
	other := &admissionregistrationv1.ValidatingAdmissionPolicy{ObjectMeta: metav1.ObjectMeta{Name: "other"}}
	s := newTestSuite(t, validConfig(), interceptor.Funcs{}, prefixed, other)

	got, err := grantAuthorPolicyName(t.Context(), s)

	require.NoError(t, err)
	assert.Equal(t, "camunda-operator-plugins.camunda.io-grant-author", got)
}

func TestRequestGrantsFindsAGrantByItsLabelOrItsOwner(t *testing.T) {
	labelled := goodGrant()
	labelled.OwnerReferences = nil
	owned := goodGrant()
	owned.Name = "r-owned"
	delete(owned.Labels, pluginsv1alpha1.LabelRequest)
	unrelated := goodGrant()
	unrelated.Name = "unrelated"
	unrelated.Labels[pluginsv1alpha1.LabelRequest] = "unrelated"
	unrelated.OwnerReferences[0].UID = "uid-unrelated"
	s := newTestSuite(t, validConfig(), interceptor.Funcs{}, &labelled, &owned, &unrelated)

	got, err := s.requestGrants(t.Context(), grantCheckRequest())

	require.NoError(t, err)
	names := make([]string, 0, len(got))
	for _, g := range got {
		names = append(names, g.Name)
	}
	assert.Equal(t, []string{"r-owned", "r-read"}, names)
}
