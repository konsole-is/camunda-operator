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

package echo

import (
	"slices"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/util/validation"

	pluginsv1alpha1 "github.com/konsole-is/camunda-operator/api/plugins/v1alpha1"
)

// readTemplate is a grant template that reads each placeholder of a grant
// and needs the input source.
func readTemplate() pluginsv1alpha1.GrantTemplate {
	return pluginsv1alpha1.GrantTemplate{
		Name:           "read",
		Format:         pluginsv1alpha1.GrantFormatAWSIAMStatement,
		RequiresInputs: []string{"source"},
		Template: &apiextensionsv1.JSON{Raw: []byte(
			`{"Resource":"arn:aws:s3:::{{ .outputs.message }}/{{ .namespace }}/{{ .request }}",` +
				`"Sid":"{{ .parameters.message }}",` +
				`"Condition":{"StringEquals":{"sa":"{{ .serviceAccount }}","claim":"{{ .claim }}"}}}`,
		)},
	}
}

func newGrantController(t *testing.T, defect Defect) *Controller {
	t.Helper()

	opts := validOptions()
	opts.GrantTemplates = []pluginsv1alpha1.GrantTemplate{readTemplate()}
	opts.Defect = defect
	c, err := New(opts)
	require.NoError(t, err)

	return c
}

func grantRequest(inputs map[string]string) *pluginsv1alpha1.PluginRequest {
	req := &pluginsv1alpha1.PluginRequest{}
	req.Namespace = "ns"
	req.Name = requestName
	req.UID = "request-uid"
	req.Spec.Registration = validOptions().Registration
	req.Spec.ClaimRef = pluginsv1alpha1.ClaimRef{Kind: pluginsv1alpha1.ClaimKindCamundaClusterClaim, Name: "orders"}
	req.Spec.Target = pluginsv1alpha1.TargetRef{
		Kind:               "CamundaCluster",
		Name:               "orders",
		ServiceAccountName: "orders-camunda",
	}
	req.Spec.Inputs = inputs
	req.Spec.Parameters = &apiextensionsv1.JSON{Raw: []byte(`{"message":"hi"}`)}

	return req
}

func TestGrantsRenderEachTemplateWithTheRequest(t *testing.T) {
	c := newGrantController(t, DefectNone)
	req := grantRequest(map[string]string{"source": "hello"})

	grants, err := c.grants(req)

	require.NoError(t, err)
	require.Len(t, grants, 1)
	g := grants[0]
	assert.Equal(t, grantName(requestName, "read"), g.Name)
	assert.Equal(t, "ns", g.Namespace)
	wantLabels := map[string]string{
		pluginsv1alpha1.LabelRegistration: validOptions().Registration,
		pluginsv1alpha1.LabelRequest:      requestName,
		pluginsv1alpha1.LabelCluster:      "orders",
	}
	assert.Equal(t, wantLabels, g.Labels)
	assert.Equal(t, "orders", g.Spec.Cluster.Name)
	assert.Equal(t, "read", g.Spec.Template)
	assert.Equal(t, pluginsv1alpha1.GrantFormatAWSIAMStatement, g.Spec.Format)
	wantFragment := `{"Resource":"arn:aws:s3:::hello/ns/` + requestName + `","Sid":"hi",` +
		`"Condition":{"StringEquals":{"sa":"orders-camunda","claim":"orders"}}}`
	assert.JSONEq(t, wantFragment, string(g.Spec.Fragment.Raw))
	require.Len(t, g.OwnerReferences, 1)
	assert.Equal(t, req.UID, g.OwnerReferences[0].UID)
}

func TestGrantsLeaveOutATemplateWhoseRequiredInputIsEmpty(t *testing.T) {
	c := newGrantController(t, DefectNone)

	for _, inputs := range []map[string]string{nil, {"source": ""}} {
		grants, err := c.grants(grantRequest(inputs))

		require.NoError(t, err)
		assert.Empty(t, grants)
	}
}

func TestGrantsKeepALargeIntegerParameterExact(t *testing.T) {
	opts := validOptions()
	tmpl := readTemplate()
	tmpl.RequiresInputs = nil
	tmpl.Template = &apiextensionsv1.JSON{Raw: []byte(`{"Sid":"{{ .parameters.size }}"}`)}
	opts.GrantTemplates = []pluginsv1alpha1.GrantTemplate{tmpl}
	c, err := New(opts)
	require.NoError(t, err)
	req := grantRequest(nil)
	req.Spec.Parameters = &apiextensionsv1.JSON{Raw: []byte(`{"size":12345678901234567890}`)}

	grants, err := c.grants(req)

	require.NoError(t, err)
	require.Len(t, grants, 1)
	assert.JSONEq(t, `{"Sid":"12345678901234567890"}`, string(grants[0].Spec.Fragment.Raw))
}

func TestWrongFragmentDefectChangesTheFragment(t *testing.T) {
	c := newGrantController(t, DefectWrongFragment)

	grants, err := c.grants(grantRequest(map[string]string{"source": "hello"}))

	require.NoError(t, err)
	require.Len(t, grants, 1)
	assert.Contains(t, string(grants[0].Spec.Fragment.Raw), `"EchoDefect":"WrongFragment"`)
}

func TestRenderFragmentNamesAMissingValue(t *testing.T) {
	_, err := renderFragment([]byte(`{"a":"{{ .outputs.nope }}"}`), map[string]any{"outputs": map[string]string{}})

	assert.ErrorContains(t, err, "nope")
}

func TestGrantNameHasAtMost63CharactersAndKeepsPairsApart(t *testing.T) {
	long := strings.Repeat("a", 63)
	names := []string{
		grantName(long, "read"),
		grantName(long, "write"),
		grantName(long+"b", "read"),
		grantName(requestName, "read"),
	}

	for _, name := range names {
		assert.LessOrEqual(t, len(name), 63, name)
	}
	assert.Len(t, slices.Compact(slices.Sorted(slices.Values(names))), len(names))
	assert.True(t, strings.HasPrefix(grantName(requestName, "read"), requestName+"-read-"))
}

func TestGrantNameIsAValidObjectNameForAnyTemplateName(t *testing.T) {
	names := []string{grantName(requestName, "Read S3"), grantName(requestName, "read_s3")}

	for _, name := range names {
		assert.Empty(t, validation.IsDNS1123Subdomain(name), name)
		assert.True(t, strings.HasPrefix(name, requestName+"-read-s3-"), name)
	}
	assert.NotEqual(t, names[0], names[1])
}
