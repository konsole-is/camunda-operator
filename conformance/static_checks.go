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
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strings"

	authorizationv1 "k8s.io/api/authorization/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	pluginsv1alpha1 "github.com/konsole-is/camunda-operator/api/plugins/v1alpha1"
	v1 "github.com/konsole-is/camunda-operator/api/v1"
)

// The IDs of the static checks, which read the registration and need no
// running plugin.
const (
	// RegAccepted checks that the registration is Accepted and Ready.
	RegAccepted CheckID = "REG-ACCEPTED"
	// RegSchema checks the parameter schema and the parameters of each case.
	RegSchema CheckID = "REG-SCHEMA"
	// RegOutputs checks that the registration declares each section output.
	RegOutputs CheckID = "REG-OUTPUTS"
	// RegInputs checks that each input reads an output of an installed section.
	RegInputs CheckID = "REG-INPUTS"
	// RegGrants checks the format and the placeholders of each grant template.
	RegGrants CheckID = "REG-GRANTS"
	// RegTemplates checks the placeholders of each x-fleet-template default.
	RegTemplates CheckID = "REG-TEMPLATES"
	// RegDeletion checks that the registration supports the Delete policy.
	RegDeletion CheckID = "REG-DELETION"
	// RegRBAC checks that the ServiceAccount of the registration may serve it.
	RegRBAC CheckID = "REG-RBAC"
	// RegMetrics checks the placeholders of each metric query.
	RegMetrics CheckID = "REG-METRICS"
)

// staticChecks are the checks of the registration alone, in run order.
var staticChecks = []Check{
	{
		ID:          RegAccepted,
		Description: "The registration has Accepted=True and Ready=True.",
		run:         checkAccepted,
	},
	{
		ID:          RegSchema,
		Description: "spec.parameters is a valid OpenAPI v3 schema, and the parameters of every case pass it.",
		run:         checkSchema,
	},
	{
		ID:          RegOutputs,
		Description: "spec.outputs covers every output of each section in spec.sections.",
		run:         checkOutputs,
	},
	{
		ID:          RegInputs,
		Description: "Each input names <section>/<output> of an installed section.",
		run:         checkInputs,
	},
	{
		ID:          RegGrants,
		Description: "Each grant template has a supported format and uses only the placeholders of the contract.",
		run:         checkGrants,
	},
	{
		ID:          RegTemplates,
		Description: "Each default of an x-fleet-template parameter uses only the placeholders of the contract.",
		run:         checkParameterTemplates,
	},
	{
		ID:          RegDeletion,
		Description: "spec.deletionPolicies holds Delete.",
		run:         checkDeletion,
	},
	{
		ID:          RegRBAC,
		Description: "The ServiceAccount of the registration has the verb serve on the registration name.",
		run:         checkRBAC,
	},
	{
		ID:          RegMetrics,
		Description: "Each metric query uses only the placeholders of the contract and .pods.",
		run:         checkMetrics,
	},
}

func checkAccepted(ctx context.Context, s *suite) error {
	reg, err := s.registration(ctx)
	if err != nil {
		return err
	}

	var errs []error
	for _, condType := range []string{pluginsv1alpha1.ConditionAccepted, v1.ConditionReady} {
		cond := meta.FindStatusCondition(reg.Status.Conditions, condType)
		switch {
		case cond == nil:
			errs = append(errs, fmt.Errorf("status.conditions[%s] is missing", condType))
		case cond.Status != metav1.ConditionTrue:
			errs = append(errs, fmt.Errorf(
				"status.conditions[%s] is %s, reason %s: %s",
				condType,
				cond.Status,
				cond.Reason,
				cond.Message,
			))
		}
	}

	return errors.Join(errs...)
}

func checkOutputs(ctx context.Context, s *suite) error {
	reg, err := s.registration(ctx)
	if err != nil {
		return err
	}

	declared := map[string]bool{}
	for _, out := range reg.Spec.Outputs {
		declared[out.Key] = true
	}

	var errs []error
	for i, name := range reg.Spec.Sections {
		section, err := s.section(ctx, name)
		if err != nil {
			errs = append(errs, fmt.Errorf("spec.sections[%d]: %w", i, err))
			continue
		}

		for _, out := range section.Spec.Outputs {
			if !declared[out.Key] {
				errs = append(errs, fmt.Errorf("spec.outputs misses the output %s of section %s", out.Key, name))
			}
		}
	}

	return errors.Join(errs...)
}

// section gets the PluginSection name. A missing section gives an error for
// which apierrors.IsNotFound is true.
func (s *suite) section(ctx context.Context, name string) (*pluginsv1alpha1.PluginSection, error) {
	var section pluginsv1alpha1.PluginSection
	if err := s.client.Get(ctx, types.NamespacedName{Name: name}, &section); err != nil {
		if apierrors.IsNotFound(err) {
			return nil, fmt.Errorf("PluginSection %q not found: %w", name, err)
		}
		return nil, fmt.Errorf("PluginSection %q: %w", name, err)
	}

	return &section, nil
}

func checkInputs(ctx context.Context, s *suite) error {
	reg, err := s.registration(ctx)
	if err != nil {
		return err
	}

	var errs []error
	for i, in := range reg.Spec.Inputs {
		sectionName, output, ok := strings.Cut(in.From, "/")
		if !ok || sectionName == "" || output == "" {
			errs = append(errs, fmt.Errorf("spec.inputs[%d].from: %q is not <section>/<output>", i, in.From))
			continue
		}

		section, err := s.section(ctx, sectionName)
		switch {
		case apierrors.IsNotFound(err):
			errs = append(errs, fmt.Errorf("spec.inputs[%d].from: section %s is not installed", i, sectionName))
		case err != nil:
			errs = append(errs, fmt.Errorf("spec.inputs[%d].from: %w", i, err))
		case !slices.ContainsFunc(section.Spec.Outputs, func(o pluginsv1alpha1.OutputDecl) bool { return o.Key == output }):
			errs = append(errs, fmt.Errorf(
				"spec.inputs[%d].from: section %s has no output %s",
				i,
				sectionName,
				output,
			))
		}
	}

	return errors.Join(errs...)
}

// supportedGrantFormats are the formats of a grant template that an identity
// extension renders.
var supportedGrantFormats = []pluginsv1alpha1.GrantFormat{pluginsv1alpha1.GrantFormatAWSIAMStatement}

func checkGrants(ctx context.Context, s *suite) error {
	reg, err := s.registration(ctx)
	if err != nil {
		return err
	}

	var errs []error
	for i, tmpl := range reg.Spec.GrantTemplates {
		if !slices.Contains(supportedGrantFormats, tmpl.Format) {
			errs = append(
				errs,
				fmt.Errorf("spec.grantTemplates[%d].format: %s is not a supported format", i, tmpl.Format),
			)
		}

		path := fmt.Sprintf("spec.grantTemplates[%d].template", i)
		if tmpl.Template == nil || !isJSONObject(tmpl.Template.Raw) {
			errs = append(errs, fmt.Errorf("%s is not a JSON object", path))
			continue
		}

		errs = append(errs, checkJSONTemplates(path, tmpl.Template.Raw, grantPlaceholders))
	}

	return errors.Join(errs...)
}

func isJSONObject(raw []byte) bool {
	var object map[string]any
	return json.Unmarshal(raw, &object) == nil && object != nil
}

func checkDeletion(ctx context.Context, s *suite) error {
	reg, err := s.registration(ctx)
	if err != nil {
		return err
	}

	if !slices.Contains(reg.Spec.DeletionPolicies, pluginsv1alpha1.DeletionPolicyDelete) {
		return errors.New("spec.deletionPolicies does not hold Delete")
	}

	return nil
}

// The verb that the Grant admission policy reads: the ServiceAccount of a
// plugin may write the Grants of the registration that it serves.
const (
	verbServe                   = "serve"
	resourcePluginRegistrations = "pluginregistrations"
)

func checkRBAC(ctx context.Context, s *suite) error {
	reg, err := s.registration(ctx)
	if err != nil {
		return err
	}

	sa := reg.Spec.ServiceAccount
	review, err := serveReview(ctx, s, sa, reg.Name)
	if err != nil {
		return fmt.Errorf("spec.serviceAccount %s/%s: %w", sa.Namespace, sa.Name, err)
	}

	if review.Status.Allowed {
		return nil
	}

	denied := fmt.Sprintf(
		"spec.serviceAccount %s/%s has no verb %s on %s/%s",
		sa.Namespace,
		sa.Name,
		verbServe,
		resourcePluginRegistrations,
		reg.Name,
	)
	if review.Status.Reason != "" {
		denied += ": " + review.Status.Reason
	}

	return errors.New(denied)
}

// serveReview asks the API server whether the ServiceAccount sa has the
// verb serve on the registration, and returns the review with its status.
func serveReview(
	ctx context.Context,
	s *suite,
	sa pluginsv1alpha1.ServiceAccountRef,
	registration string,
) (*authorizationv1.SubjectAccessReview, error) {
	review := &authorizationv1.SubjectAccessReview{
		Spec: authorizationv1.SubjectAccessReviewSpec{
			User: serviceAccountUser(sa),
			Groups: []string{
				"system:serviceaccounts",
				"system:serviceaccounts:" + sa.Namespace,
				"system:authenticated",
			},
			ResourceAttributes: &authorizationv1.ResourceAttributes{
				Group:    pluginsv1alpha1.GroupVersion.Group,
				Resource: resourcePluginRegistrations,
				Verb:     verbServe,
				Name:     registration,
			},
		},
	}
	if err := s.client.Create(ctx, review); err != nil {
		return nil, fmt.Errorf("creating a SubjectAccessReview: %w", err)
	}

	return review, nil
}

// serviceAccountUser is the user name of the ServiceAccount sa.
func serviceAccountUser(sa pluginsv1alpha1.ServiceAccountRef) string {
	return "system:serviceaccount:" + sa.Namespace + ":" + sa.Name
}

func checkMetrics(ctx context.Context, s *suite) error {
	reg, err := s.registration(ctx)
	if err != nil {
		return err
	}

	var errs []error
	for i, q := range reg.Spec.MetricQueries {
		if err := checkTemplate(q.Query, metricPlaceholders); err != nil {
			errs = append(errs, fmt.Errorf("spec.metricQueries[%d].query: %w", i, err))
		}
	}

	return errors.Join(errs...)
}
