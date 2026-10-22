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
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"math/big"
	"slices"
	"strings"
	"time"

	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/util/retry"
	"sigs.k8s.io/controller-runtime/pkg/client"

	pluginsv1alpha1 "github.com/konsole-is/camunda-operator/api/plugins/v1alpha1"
	v1 "github.com/konsole-is/camunda-operator/api/v1"
)

// The IDs of the grant checks, which check the Grants that the plugin writes
// for each request of the run.
const (
	// GrantMatch checks that each Grant is its grant template filled in.
	GrantMatch CheckID = "GRANT-MATCH"
	// GrantRequires checks that a template with an empty required input has
	// no Grant.
	GrantRequires CheckID = "GRANT-REQUIRES"
	// GrantAdmission checks that only the plugin can write the Grants of its
	// registration. It is skippable.
	GrantAdmission CheckID = "GRANT-ADMISSION"
	// GrantReady checks that a request is not Ready=True while one of its
	// Grants has no Ready condition, and that it gets Ready=True when the
	// suite sets Ready=True on its Grants. It is skippable.
	GrantReady CheckID = "GRANT-READY"
)

// grantChecks run for each case with grant templates, in this order.
var grantChecks = []Check{
	{
		ID: GrantMatch,
		Description: "Each grant template whose required inputs have values has one Grant of the request, with " +
			"the labels, the owner, the format, and the fragment of the filled-in template.",
		run: checkGrantMatch,
	},
	{
		ID:          GrantRequires,
		Description: "A grant template with a required input without a value has no Grant.",
		run:         checkGrantRequires,
	},
	{
		ID: GrantAdmission,
		Description: "The API server refuses a Grant of the registration from a requester without the verb serve " +
			"on it. It skips itself when the Grant admission policy is not installed.",
		Skippable: true,
		run:       checkGrantAdmission,
	},
	{
		ID: GrantReady,
		Description: "A request is not Ready=True while one of its Grants has no Ready condition. It gets " +
			"Ready=True within the Ready timeout after the suite sets Ready=True on its Grants.",
		Skippable: true,
		run:       checkGrantReady,
	},
}

// noGrantTemplates is the skip reason of a grant check on a registration
// without grant templates.
const noGrantTemplates = "the registration has no grant templates"

func checkGrantMatch(ctx context.Context, s *suite) error {
	reg, err := s.registration(ctx)
	if err != nil {
		return err
	}
	if len(reg.Spec.GrantTemplates) == 0 {
		return SkipCheck(noGrantTemplates)
	}

	life, err := s.lifecycle(ctx)
	if err != nil {
		return err
	}

	return eachRun(life.runs, func(r requestRun) error {
		deadline := life.createdAt[r.name].Add(s.cfg.Timeouts.Ready)
		var errs []error
		for _, tmpl := range reg.Spec.GrantTemplates {
			if len(missingInputs(r.tc.Inputs, tmpl.RequiresInputs)) > 0 {
				continue
			}

			what := "a Grant that matches the template " + tmpl.Name
			errs = append(errs, waitUntil(ctx, deadline, s.cfg.Timeouts.Ready, what, func(ctx context.Context) error {
				req, err := s.request(ctx, r.name)
				if err != nil {
					return err
				}

				// The template can read the outputs, which are complete only
				// at OutputsReady=True.
				if err := outputsReadyProblem(req, true); err != nil {
					return err
				}

				want, err := expectedFragment(tmpl, reg.Spec.Parameters, req)
				if err != nil {
					return err
				}

				grants, err := s.grantsOf(ctx, req, tmpl.Name)
				if err != nil {
					return err
				}

				return problemsError(grantMatchProblems(req, tmpl, want, grants))
			}))
		}

		if err := waitForOutputsReady(ctx, s, r.name, life.createdAt[r.name], true); err != nil {
			return errors.Join(append(errs, err)...)
		}

		req, err := s.request(ctx, r.name)
		if err != nil {
			return errors.Join(append(errs, err)...)
		}

		grants, err := s.requestGrants(ctx, req)
		if err != nil {
			return errors.Join(append(errs, err)...)
		}
		errs = append(errs, problemsError(undeclaredTemplateProblems(grants, reg.Spec.GrantTemplates)))

		return errors.Join(errs...)
	})
}

// undeclaredTemplateProblems names each of grants whose template the
// registration does not declare in templates.
func undeclaredTemplateProblems(
	grants []pluginsv1alpha1.Grant,
	templates []pluginsv1alpha1.GrantTemplate,
) []string {
	var problems []string
	for _, g := range grants {
		declared := slices.ContainsFunc(templates, func(t pluginsv1alpha1.GrantTemplate) bool {
			return t.Name == g.Spec.Template
		})
		if !declared {
			problems = append(problems, fmt.Sprintf(
				"Grant %s names the template %s, which the registration does not declare",
				g.Name,
				g.Spec.Template,
			))
		}
	}

	return problems
}

// missingInputs returns each of required that inputs leaves empty or does
// not hold.
func missingInputs(inputs map[string]string, required []string) []string {
	var missing []string
	for _, name := range required {
		if inputs[name] == "" {
			missing = append(missing, name)
		}
	}

	return missing
}

// expectedFragment is the fragment of tmpl for req. Each top-level parameter
// that schema marks x-fleet-template is rendered first, and the template then
// reads the rendered value. The outputs come from the status of req.
func expectedFragment(
	tmpl pluginsv1alpha1.GrantTemplate,
	schema *apiextensionsv1.JSON,
	req *pluginsv1alpha1.PluginRequest,
) ([]byte, error) {
	if tmpl.Template == nil {
		return nil, fmt.Errorf("the grant template %s has no template", tmpl.Name)
	}

	data := templateData{
		namespace:      req.Namespace,
		claim:          req.Spec.ClaimRef.Name,
		request:        req.Name,
		serviceAccount: req.Spec.Target.ServiceAccountName,
		parameters:     map[string]any{},
		outputs:        req.Status.Outputs,
	}
	if req.Spec.Parameters != nil && len(req.Spec.Parameters.Raw) > 0 {
		if err := decodeNumbers(req.Spec.Parameters.Raw, &data.parameters); err != nil {
			return nil, fmt.Errorf("spec.parameters: %w", err)
		}
	}

	marked, err := templateParameters(schema)
	if err != nil {
		return nil, err
	}

	parameterData := data
	parameterData.outputs = nil
	rendered := maps.Clone(data.parameters)
	for _, name := range marked {
		value, ok := data.parameters[name]
		if !ok {
			continue
		}

		raw, err := json.Marshal(value)
		if err != nil {
			return nil, fmt.Errorf("spec.parameters.%s: %w", name, err)
		}

		raw, err = renderJSONTemplate("spec.parameters."+name, raw, parameterPlaceholders, parameterData)
		if err != nil {
			return nil, err
		}

		var v any
		if err := decodeNumbers(raw, &v); err != nil {
			return nil, fmt.Errorf("spec.parameters.%s: %w", name, err)
		}
		rendered[name] = v
	}

	data.parameters = rendered

	return renderJSONTemplate("the grant template "+tmpl.Name, tmpl.Template.Raw, grantPlaceholders, data)
}

// decodeNumbers decodes raw into v, with each number as a json.Number, so
// that a large integer keeps its digits.
func decodeNumbers(raw []byte, v any) error {
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()

	return decoder.Decode(v)
}

// templateParameters returns the names of the top-level properties of schema
// that carry x-fleet-template: true.
func templateParameters(schema *apiextensionsv1.JSON) ([]string, error) {
	if schema == nil || len(schema.Raw) == 0 {
		return nil, nil
	}

	var doc struct {
		Properties map[string]map[string]any `json:"properties"`
	}
	if err := json.Unmarshal(schema.Raw, &doc); err != nil {
		return nil, fmt.Errorf("spec.parameters of the registration: %w", err)
	}

	var names []string
	for _, name := range slices.Sorted(maps.Keys(doc.Properties)) {
		if marked, _ := doc.Properties[name][fleetTemplateKey].(bool); marked {
			names = append(names, name)
		}
	}

	return names, nil
}

// grantsOf returns the Grants of requestGrants that name template.
func (s *suite) grantsOf(
	ctx context.Context,
	req *pluginsv1alpha1.PluginRequest,
	template string,
) ([]pluginsv1alpha1.Grant, error) {
	grants, err := s.requestGrants(ctx, req)
	if err != nil {
		return nil, err
	}

	return slices.DeleteFunc(grants, func(g pluginsv1alpha1.Grant) bool { return g.Spec.Template != template }), nil
}

// requestGrants returns the Grants in the namespace of req that carry its
// request label or have an owner reference to it, sorted by name. It does not
// check their labels or their owner.
func (s *suite) requestGrants(
	ctx context.Context,
	req *pluginsv1alpha1.PluginRequest,
) ([]pluginsv1alpha1.Grant, error) {
	var list pluginsv1alpha1.GrantList
	if err := s.client.List(ctx, &list, client.InNamespace(req.Namespace)); err != nil {
		return nil, fmt.Errorf("listing the Grants of the request: %w", err)
	}

	grants := slices.DeleteFunc(list.Items, func(g pluginsv1alpha1.Grant) bool {
		owned := slices.ContainsFunc(g.OwnerReferences, func(o metav1.OwnerReference) bool { return o.UID == req.UID })
		return g.Labels[pluginsv1alpha1.LabelRequest] != req.Name && !owned
	})
	slices.SortFunc(grants, func(a, b pluginsv1alpha1.Grant) int { return strings.Compare(a.Name, b.Name) })

	return grants, nil
}

// grantMatchProblems names each way in which grants, the Grants of req for
// tmpl, are not one Grant with the fragment want.
func grantMatchProblems(
	req *pluginsv1alpha1.PluginRequest,
	tmpl pluginsv1alpha1.GrantTemplate,
	want []byte,
	grants []pluginsv1alpha1.Grant,
) []string {
	switch len(grants) {
	case 0:
		return []string{fmt.Sprintf(
			"no Grant of the template %s carries the label %s: %s or an owner reference to the request",
			tmpl.Name,
			pluginsv1alpha1.LabelRequest,
			req.Name,
		)}
	case 1:
	default:
		names := make([]string, 0, len(grants))
		for _, g := range grants {
			names = append(names, g.Name)
		}
		return []string{fmt.Sprintf(
			"%d Grants of the template %s: %s. The plugin writes one Grant for each template",
			len(grants),
			tmpl.Name,
			strings.Join(names, ", "),
		)}
	}

	g := grants[0]
	var problems []string
	if g.Spec.Cluster.Name != req.Spec.Target.Name {
		problems = append(problems, fmt.Sprintf(
			"Grant %s has spec.cluster.name %q, and the request needs %q",
			g.Name,
			g.Spec.Cluster.Name,
			req.Spec.Target.Name,
		))
	}

	for _, label := range []struct{ key, want string }{
		{pluginsv1alpha1.LabelRequest, req.Name},
		{pluginsv1alpha1.LabelRegistration, req.Spec.Registration},
		{pluginsv1alpha1.LabelCluster, req.Spec.Target.Name},
	} {
		if got := g.Labels[label.key]; got != label.want {
			problems = append(problems, fmt.Sprintf(
				"Grant %s has the label %s: %q, and the request needs %q",
				g.Name,
				label.key,
				got,
				label.want,
			))
		}
	}

	if g.Spec.Format != tmpl.Format {
		problems = append(
			problems,
			fmt.Sprintf("Grant %s has the format %s, and the template has %s", g.Name, g.Spec.Format, tmpl.Format),
		)
	}

	got := []byte("null")
	if g.Spec.Fragment != nil {
		got = g.Spec.Fragment.Raw
	}
	if same, err := sameJSON(got, want); err != nil || !same {
		problems = append(problems, fmt.Sprintf(
			"Grant %s has the fragment %s, and the filled-in template is %s",
			g.Name,
			got,
			want,
		))
	}

	owned := slices.ContainsFunc(g.OwnerReferences, func(o metav1.OwnerReference) bool {
		return isPluginRequestOwner(o) && o.Name == req.Name && o.UID == req.UID
	})
	if !owned {
		problems = append(problems, fmt.Sprintf("Grant %s has no owner reference to the request", g.Name))
	}

	return problems
}

// sameJSON says whether a and b hold the same JSON value. The order of the
// keys and the form of a number do not count, and numbers compare exactly.
func sameJSON(a, b []byte) (bool, error) {
	var va, vb any
	if err := decodeNumbers(a, &va); err != nil {
		return false, err
	}
	if err := decodeNumbers(b, &vb); err != nil {
		return false, err
	}

	return sameValue(va, vb), nil
}

func sameValue(a, b any) bool {
	switch x := a.(type) {
	case json.Number:
		y, ok := b.(json.Number)
		if !ok {
			return false
		}
		rx, okx := new(big.Rat).SetString(string(x))
		ry, oky := new(big.Rat).SetString(string(y))
		return okx && oky && rx.Cmp(ry) == 0
	case []any:
		y, ok := b.([]any)
		return ok && slices.EqualFunc(x, y, sameValue)
	case map[string]any:
		y, ok := b.(map[string]any)
		return ok && maps.EqualFunc(x, y, sameValue)
	default:
		return a == b
	}
}

func checkGrantRequires(ctx context.Context, s *suite) error {
	reg, err := s.registration(ctx)
	if err != nil {
		return err
	}
	if len(reg.Spec.GrantTemplates) == 0 {
		return SkipCheck(noGrantTemplates)
	}

	life, err := s.lifecycle(ctx)
	if err != nil {
		return err
	}

	tested := slices.ContainsFunc(life.runs, func(r requestRun) bool {
		return slices.ContainsFunc(reg.Spec.GrantTemplates, func(t pluginsv1alpha1.GrantTemplate) bool {
			return len(missingInputs(r.tc.Inputs, t.RequiresInputs)) > 0
		})
	})
	if !tested {
		return SkipCheck("no case leaves a required input of a grant template empty")
	}

	return eachRun(life.runs, func(r requestRun) error {
		missing := map[string][]string{}
		for _, tmpl := range reg.Spec.GrantTemplates {
			if m := missingInputs(r.tc.Inputs, tmpl.RequiresInputs); len(m) > 0 {
				missing[tmpl.Name] = m
			}
		}
		if len(missing) == 0 {
			return nil
		}

		// A Grant that is wrong can still be on its way before
		// OutputsReady=True, so the check looks only after it.
		if err := waitForOutputsReady(ctx, s, r.name, life.createdAt[r.name], true); err != nil {
			return err
		}

		req, err := s.request(ctx, r.name)
		if err != nil {
			return err
		}

		var problems []string
		for _, name := range slices.Sorted(maps.Keys(missing)) {
			grants, err := s.grantsOf(ctx, req, name)
			if err != nil {
				return err
			}

			for _, g := range grants {
				problems = append(problems, fmt.Sprintf(
					"Grant %s of the template %s exists, and the case leaves the required input %s without a value",
					g.Name,
					name,
					missing[name][0],
				))
			}
		}

		return problemsError(problems)
	})
}

// grantAuthorPolicy ends the name of the Grant admission policy. An install
// can add a prefix to the name.
const grantAuthorPolicy = "plugins.camunda.io-grant-author"

// admissionTimeout is how long GRANT-ADMISSION waits for the refusal. A new
// RoleBinding reaches the authorizer a short time after it is created.
const admissionTimeout = time.Minute

// checkGrantAdmission sends a dry-run Grant of the registration as a
// ServiceAccount that can create Grants and has no verb serve, and expects
// the policy to refuse it. The identity of the run needs create on
// ServiceAccounts, Roles, and RoleBindings, and impersonate on
// ServiceAccounts, in the run namespace. A cluster admin has them.
func checkGrantAdmission(ctx context.Context, s *suite) error {
	reg, err := s.registration(ctx)
	if err != nil {
		return err
	}
	if len(reg.Spec.GrantTemplates) == 0 {
		return SkipCheck(noGrantTemplates)
	}

	policy, err := grantAuthorPolicyName(ctx, s)
	if err != nil {
		return err
	}
	if policy == "" {
		return SkipCheck("no ValidatingAdmissionPolicy whose name ends with " + grantAuthorPolicy + " is installed")
	}

	if _, err := s.lifecycle(ctx); err != nil {
		return err
	}

	sa, err := createGrantAuthor(ctx, s)
	if err != nil {
		return err
	}

	review, err := serveReview(ctx, s, sa, reg.Name)
	if err != nil {
		return err
	}
	if review.Status.Allowed {
		return fmt.Errorf(
			"the ServiceAccount %s/%s that the check created has the verb serve on %s, so the check cannot "+
				"test a refusal. A role in the cluster gives serve to it",
			sa.Namespace,
			sa.Name,
			reg.Name,
		)
	}

	user := serviceAccountUser(sa)
	impersonated := rest.CopyConfig(s.restConfig)
	// Each other impersonation attribute of the run would add to the rights
	// that serveReview checked.
	impersonated.Impersonate = rest.ImpersonationConfig{UserName: user}
	c, err := client.New(impersonated, client.Options{Scheme: s.client.Scheme(), Mapper: s.client.RESTMapper()})
	if err != nil {
		return fmt.Errorf("connecting as %s: %w", user, err)
	}

	what := "a refusal of the Grant by the policy " + policy
	return waitFor(ctx, admissionTimeout, what, func(ctx context.Context) error {
		grant := &pluginsv1alpha1.Grant{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: s.cfg.Namespace,
				Name:      "conformance-grant-author",
				Labels:    map[string]string{pluginsv1alpha1.LabelRegistration: reg.Name},
			},
			Spec: pluginsv1alpha1.GrantSpec{
				Cluster:  v1.ClusterRef{Name: "conformance"},
				Template: reg.Spec.GrantTemplates[0].Name,
				Format:   pluginsv1alpha1.GrantFormatAWSIAMStatement,
				Fragment: &apiextensionsv1.JSON{
					Raw: []byte(`{"Effect":"Allow","Action":["s3:ListBucket"],"Resource":["*"]}`),
				},
			},
		}

		// A dry run stores nothing, so a policy that admits the Grant
		// leaves no object behind.
		return admissionVerdict(c.Create(ctx, grant, client.DryRunAll), policy, user, reg.Name)
	})
}

// grantAuthorPolicyName returns the name of the installed
// ValidatingAdmissionPolicy that ends with grantAuthorPolicy, or an empty
// name when there is none.
func grantAuthorPolicyName(ctx context.Context, s *suite) (string, error) {
	var list admissionregistrationv1.ValidatingAdmissionPolicyList
	if err := s.client.List(ctx, &list); err != nil {
		return "", fmt.Errorf("listing the ValidatingAdmissionPolicies: %w", err)
	}

	names := make([]string, 0, len(list.Items))
	for _, p := range list.Items {
		if strings.HasSuffix(p.Name, grantAuthorPolicy) {
			names = append(names, p.Name)
		}
	}
	if len(names) == 0 {
		return "", nil
	}

	return slices.Min(names), nil
}

// createGrantAuthor creates, in the run namespace, a ServiceAccount with a
// Role that can create Grants, and returns it. The cleanups of the suite
// delete what it created.
func createGrantAuthor(ctx context.Context, s *suite) (pluginsv1alpha1.ServiceAccountRef, error) {
	name := "conformance-grant-author-" + runIDOf(s.cfg.Namespace)
	objectMeta := metav1.ObjectMeta{Namespace: s.cfg.Namespace, Name: name}
	sa := pluginsv1alpha1.ServiceAccountRef{Namespace: s.cfg.Namespace, Name: name}

	objs := []client.Object{
		&corev1.ServiceAccount{ObjectMeta: objectMeta},
		&rbacv1.Role{
			ObjectMeta: objectMeta,
			Rules: []rbacv1.PolicyRule{{
				APIGroups: []string{pluginsv1alpha1.GroupVersion.Group},
				Resources: []string{"grants"},
				Verbs:     []string{"create"},
			}},
		},
		&rbacv1.RoleBinding{
			ObjectMeta: objectMeta,
			RoleRef:    rbacv1.RoleRef{APIGroup: rbacv1.GroupName, Kind: "Role", Name: name},
			Subjects:   []rbacv1.Subject{{Kind: rbacv1.ServiceAccountKind, Namespace: sa.Namespace, Name: name}},
		},
	}

	for _, obj := range objs {
		if err := s.client.Create(ctx, obj); err != nil {
			return sa, fmt.Errorf("creating the %T %s/%s: %w", obj, obj.GetNamespace(), obj.GetName(), err)
		}
		s.onClose(func(ctx context.Context) error {
			return client.IgnoreNotFound(s.client.Delete(ctx, obj))
		})
	}

	return sa, nil
}

// admissionVerdict passes err when it is a refusal by policy of a Grant of
// registration from user.
func admissionVerdict(err error, policy, user, registration string) error {
	switch {
	case err == nil:
		return fmt.Errorf(
			"the API server admitted a Grant of %s from %s, which has no verb serve on it",
			registration,
			user,
		)
	case strings.Contains(err.Error(), "'"+policy+"'"):
		return nil
	default:
		return fmt.Errorf("the API server refused the Grant, but not through the policy %s: %w", policy, err)
	}
}

// grantReadyMessage is the message of the Ready condition that GRANT-READY
// sets on a Grant.
const grantReadyMessage = "Set by the conformance suite, which plays the identity extension"

// checkGrantReady plays the identity extension: it sets Ready=True on the
// Grants of each request. It skips itself when a Grant already has a Ready
// condition, because then an identity extension serves the Grants. The
// identity of the run needs update on grants/status.
func checkGrantReady(ctx context.Context, s *suite) error {
	life, err := s.lifecycle(ctx)
	if err != nil {
		return err
	}

	grants := make(map[string][]pluginsv1alpha1.Grant, len(life.runs))
	for _, r := range life.runs {
		owned, err := s.grantsAtReady(ctx, r, life.createdAt[r.name])
		if err != nil {
			return fmt.Errorf("request %s: %w", r.name, err)
		}
		grants[r.name] = owned
	}

	var all []pluginsv1alpha1.Grant
	for _, r := range life.runs {
		all = append(all, grants[r.name]...)
	}
	if len(all) == 0 {
		return SkipCheck("the plugin wrote no Grant")
	}

	for _, g := range all {
		if meta.FindStatusCondition(g.Status.Conditions, v1.ConditionReady) != nil {
			return SkipCheck(fmt.Sprintf(
				"Grant %s/%s has a Ready condition, so an identity extension serves the Grants, and the suite "+
					"does not write their status",
				g.Namespace,
				g.Name,
			))
		}
	}

	return eachRun(life.runs, func(r requestRun) error {
		if len(grants[r.name]) == 0 {
			return nil
		}

		return s.grantReadyRun(ctx, r.name, grants[r.name])
	})
}

// grantsAtReady waits until the request of r, created at createdAt, has a
// Ready condition, and returns its Grants then.
func (s *suite) grantsAtReady(
	ctx context.Context,
	r requestRun,
	createdAt time.Time,
) ([]pluginsv1alpha1.Grant, error) {
	var owned []pluginsv1alpha1.Grant
	deadline := createdAt.Add(s.cfg.Timeouts.Ready)
	err := waitUntil(ctx, deadline, s.cfg.Timeouts.Ready, "a Ready condition", func(ctx context.Context) error {
		req, err := s.request(ctx, r.name)
		if err != nil {
			return err
		}

		if meta.FindStatusCondition(req.Status.Conditions, v1.ConditionReady) == nil {
			return errors.New("the request has no Ready condition")
		}

		owned, err = s.ownedGrants(ctx, req)
		return err
	})

	return owned, err
}

// grantReadyRun checks the request name with grants, none of which has a
// Ready condition yet.
func (s *suite) grantReadyRun(ctx context.Context, name string, grants []pluginsv1alpha1.Grant) error {
	var ready *metav1.Condition
	err := waitFor(ctx, s.cfg.Timeouts.Ready, "Ready not True while a Grant waits", func(ctx context.Context) error {
		req, err := s.request(ctx, name)
		if err != nil {
			return err
		}

		ready = meta.FindStatusCondition(req.Status.Conditions, v1.ConditionReady)
		return waitingVerdict(ready, grants[0].Name)
	})
	if err != nil {
		return err
	}

	// REQ-READY judges each other state of Ready.
	if ready == nil || ready.Status != metav1.ConditionFalse || ready.Reason != reasonWaitingForGrants {
		return nil
	}

	if err := s.setGrantsReady(ctx, grants); err != nil {
		return err
	}

	return waitFor(ctx, s.cfg.Timeouts.Ready, "Ready=True after the Grants got Ready", func(ctx context.Context) error {
		req, err := s.request(ctx, name)
		if err != nil {
			return err
		}

		return grantsReadyVerdict(meta.FindStatusCondition(req.Status.Conditions, v1.ConditionReady))
	})
}

// waitingVerdict fails ready when it is True while grant has no Ready
// condition.
func waitingVerdict(ready *metav1.Condition, grant string) error {
	if ready == nil || ready.Status != metav1.ConditionTrue {
		return nil
	}

	return fmt.Errorf("the request is Ready=True while Grant %s has no Ready condition", grant)
}

// setGrantsReady sets Ready=True on each of grants, and keeps their other
// conditions.
func (s *suite) setGrantsReady(ctx context.Context, grants []pluginsv1alpha1.Grant) error {
	for _, g := range grants {
		err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
			var current pluginsv1alpha1.Grant
			if err := s.client.Get(ctx, client.ObjectKeyFromObject(&g), &current); err != nil {
				return err
			}

			meta.SetStatusCondition(&current.Status.Conditions, metav1.Condition{
				Type:               v1.ConditionReady,
				Status:             metav1.ConditionTrue,
				Reason:             v1.ReasonHealthy,
				Message:            grantReadyMessage,
				ObservedGeneration: current.Generation,
			})

			return s.client.Status().Update(ctx, &current)
		})
		if err != nil {
			return fmt.Errorf("setting Ready=True on the Grant %s: %w", g.Name, err)
		}
	}

	return nil
}

// grantsReadyVerdict passes ready when it is True.
func grantsReadyVerdict(ready *metav1.Condition) error {
	switch {
	case ready == nil:
		return errors.New("the request has no Ready condition")
	case ready.Status == metav1.ConditionTrue:
		return nil
	}

	return fmt.Errorf(
		"the request is not Ready=True after every Grant of it got Ready=True: Ready is %s, reason %s: %s",
		ready.Status,
		ready.Reason,
		ready.Message,
	)
}
