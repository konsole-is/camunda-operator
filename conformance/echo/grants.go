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
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"slices"
	"strings"
	"text/template"

	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	pluginsv1alpha1 "github.com/konsole-is/camunda-operator/api/plugins/v1alpha1"
	v1 "github.com/konsole-is/camunda-operator/api/v1"
)

// grants returns one Grant for each grant template of the options whose
// required inputs all have a value in req.
func (c *Controller) grants(req *pluginsv1alpha1.PluginRequest) ([]*pluginsv1alpha1.Grant, error) {
	parameters := map[string]any{}
	if req.Spec.Parameters != nil && len(req.Spec.Parameters.Raw) > 0 {
		decoder := json.NewDecoder(bytes.NewReader(req.Spec.Parameters.Raw))
		decoder.UseNumber()
		if err := decoder.Decode(&parameters); err != nil {
			return nil, fmt.Errorf("decoding spec.parameters: %w", err)
		}
	}

	values := map[string]any{
		"namespace":      req.Namespace,
		"claim":          req.Spec.ClaimRef.Name,
		"request":        req.Name,
		"serviceAccount": req.Spec.Target.ServiceAccountName,
		"parameters":     parameters,
		"outputs":        c.outputs(req),
	}

	var grants []*pluginsv1alpha1.Grant
	for _, tmpl := range c.opts.GrantTemplates {
		missing := slices.ContainsFunc(tmpl.RequiresInputs, func(name string) bool {
			return req.Spec.Inputs[name] == ""
		})
		if missing && c.opts.Defect != DefectIgnoreRequiredInputs {
			continue
		}

		fragment, err := renderFragment(tmpl.Template.Raw, values)
		if err != nil {
			return nil, fmt.Errorf("rendering the grant template %s: %w", tmpl.Name, err)
		}

		if c.opts.Defect == DefectWrongFragment {
			if fragment, err = addDefectKey(fragment); err != nil {
				return nil, err
			}
		}

		grant := &pluginsv1alpha1.Grant{
			TypeMeta: metav1.TypeMeta{APIVersion: pluginsv1alpha1.GroupVersion.String(), Kind: "Grant"},
			ObjectMeta: metav1.ObjectMeta{
				Name:      grantName(req.Name, tmpl.Name),
				Namespace: req.Namespace,
				Labels: map[string]string{
					pluginsv1alpha1.LabelRegistration: c.opts.Registration,
					pluginsv1alpha1.LabelRequest:      req.Name,
					pluginsv1alpha1.LabelCluster:      req.Spec.Target.Name,
				},
				OwnerReferences: requestOwner(req),
			},
			Spec: pluginsv1alpha1.GrantSpec{
				Cluster:  v1.ClusterRef{Name: req.Spec.Target.Name},
				Template: tmpl.Name,
				Format:   tmpl.Format,
				Fragment: &apiextensionsv1.JSON{Raw: fragment},
			},
		}
		grants = append(grants, grant)
	}

	return grants, nil
}

// grantName is the name of the Grant of request for the grant template tmpl.
// It is a valid object name of at most 63 characters, the limit of the Grant
// CRD, and two pairs of request and template get two names.
func grantName(request, tmpl string) string {
	name := strings.Map(func(r rune) rune {
		switch {
		case 'a' <= r && r <= 'z', '0' <= r && r <= '9':
			return r
		case 'A' <= r && r <= 'Z':
			return r - 'A' + 'a'
		default:
			return '-'
		}
	}, request+"-"+tmpl)
	if len(name) > 54 {
		name = strings.TrimRight(name[:54], "-")
	}

	sum := sha256.Sum256([]byte(request + "/" + tmpl))

	return name + "-" + hex.EncodeToString(sum[:4])
}

// renderFragment executes each string value of the JSON document raw, at any
// depth, as a Go template on values, and returns the document as JSON. An
// object key is never a template. A missing value is an error. It does not
// check the placeholder rule of the contract, which REG-GRANTS checks.
func renderFragment(raw []byte, values map[string]any) ([]byte, error) {
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	var doc any
	if err := decoder.Decode(&doc); err != nil {
		return nil, fmt.Errorf("decoding the template: %w", err)
	}

	rendered, err := renderValue(doc, values)
	if err != nil {
		return nil, err
	}

	return json.Marshal(rendered)
}

func renderValue(value any, values map[string]any) (any, error) {
	switch v := value.(type) {
	case string:
		tmpl, err := template.New("").Option("missingkey=error").Parse(v)
		if err != nil {
			return nil, err
		}

		var out strings.Builder
		if err := tmpl.Execute(&out, values); err != nil {
			return nil, err
		}
		return out.String(), nil
	case []any:
		for i, item := range v {
			rendered, err := renderValue(item, values)
			if err != nil {
				return nil, err
			}
			v[i] = rendered
		}
	case map[string]any:
		for key, item := range v {
			rendered, err := renderValue(item, values)
			if err != nil {
				return nil, err
			}
			v[key] = rendered
		}
	}

	return value, nil
}

// addDefectKey adds the key EchoDefect to the JSON object fragment.
func addDefectKey(fragment []byte) ([]byte, error) {
	var object map[string]json.RawMessage
	if err := json.Unmarshal(fragment, &object); err != nil {
		return nil, fmt.Errorf("decoding the fragment: %w", err)
	}

	object["EchoDefect"] = json.RawMessage(`"` + string(DefectWrongFragment) + `"`)

	return json.Marshal(object)
}
