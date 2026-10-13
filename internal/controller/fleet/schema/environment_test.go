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

package schema

import (
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	utilrand "k8s.io/apimachinery/pkg/util/rand"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	fleetv1alpha1 "github.com/konsole-is/camunda-operator/api/fleet/v1alpha1"
	pluginsv1alpha1 "github.com/konsole-is/camunda-operator/api/plugins/v1alpha1"
)

// validEnvironment returns an environment with a unique name that sets every
// field that a rule reads.
func validEnvironment() *fleetv1alpha1.Environment {
	return &fleetv1alpha1.Environment{
		ObjectMeta: metav1.ObjectMeta{Name: "env-" + utilrand.String(8)},
		Spec: fleetv1alpha1.EnvironmentSpec{
			DisplayName: "Production",
			Color:       "#2E7D32",
			Releases: &fleetv1alpha1.EnvironmentReleases{
				Allowed: []string{"camunda-8-9"},
				Default: "camunda-8-9",
			},
			Sections: map[string]fleetv1alpha1.SectionPolicy{
				"backup-storage": {Required: true, Default: "s3-backups", Allowed: []string{"s3-backups"}},
			},
			Parameters: map[string]fleetv1alpha1.ParameterPolicies{
				"aws-connectors": {
					"policies": {
						ClaimEditable: true,
						Allowed:       []fleetv1alpha1.NamedValue{choice("sqs", "SQS access")},
					},
				},
			},
			NamespaceCreation: &fleetv1alpha1.NamespaceCreation{
				Allowed: true,
				Labels: map[string]fleetv1alpha1.LabelValue{
					"owner":                   "platform",
					"example.com/cost-center": "cc-1234",
				},
				RequiredLabelKeys: []string{"team", "example.com/cost-center"},
			},
		},
	}
}

func choice(name, label string) fleetv1alpha1.NamedValue {
	return fleetv1alpha1.NamedValue{
		Name:  name,
		Label: label,
		Value: &apiextensionsv1.JSON{Raw: []byte(`"arn:aws:iam::aws:policy/AmazonSQSFullAccess"`)},
	}
}

func policy(e *fleetv1alpha1.Environment) fleetv1alpha1.ParameterPolicy {
	return e.Spec.Parameters["aws-connectors"]["policies"]
}

func setPolicy(e *fleetv1alpha1.Environment, p fleetv1alpha1.ParameterPolicy) {
	e.Spec.Parameters["aws-connectors"]["policies"] = p
}

func setSection(e *fleetv1alpha1.Environment, s fleetv1alpha1.SectionPolicy) {
	e.Spec.Sections["backup-storage"] = s
}

// unstructuredEnvironment returns validEnvironment with value at path. The
// typed client drops an empty list under omitempty, so a spec that sends
// allowed: [] must go through an unstructured object.
func unstructuredEnvironment(value map[string]any, path ...string) *unstructured.Unstructured {
	typed := validEnvironment()
	content, err := runtime.DefaultUnstructuredConverter.ToUnstructured(typed)
	Expect(err).NotTo(HaveOccurred())

	obj := &unstructured.Unstructured{Object: content}
	obj.SetGroupVersionKind(fleetv1alpha1.GroupVersion.WithKind("Environment"))
	Expect(unstructured.SetNestedField(obj.Object, value, path...)).To(Succeed())

	return obj
}

var _ = Describe("Environment schema", func() {
	DescribeTable(
		"admission",
		func(mutate func(*fleetv1alpha1.Environment), wantErr string) {
			obj := validEnvironment()
			mutate(obj)
			err := k8sClient.Create(ctx, obj)
			if wantErr == "" {
				Expect(err).NotTo(HaveOccurred())
				DeferCleanup(func() { _ = k8sClient.Delete(ctx, obj) })
			} else {
				Expect(err).To(MatchError(ContainSubstring(wantErr)))
			}
		},
		Entry("admits an environment that sets every rule", func(*fleetv1alpha1.Environment) {}, ""),
		Entry(
			"admits an empty spec", func(e *fleetv1alpha1.Environment) {
				e.Spec = fleetv1alpha1.EnvironmentSpec{}
			}, "",
		),

		Entry(
			"admits a name of 63 characters", func(e *fleetv1alpha1.Environment) {
				e.Name = "e" + strings.Repeat("a", 62)
			}, "",
		),
		Entry(
			"refuses a name of 64 characters", func(e *fleetv1alpha1.Environment) {
				e.Name = "e" + strings.Repeat("a", 63)
			}, "metadata.name must be at most 63 characters, because it is used as a label value",
		),

		Entry(
			"refuses a color that is not a hex color", func(e *fleetv1alpha1.Environment) {
				e.Spec.Color = "green"
			}, "spec.color",
		),
		Entry(
			"refuses a display name of 65 characters", func(e *fleetv1alpha1.Environment) {
				e.Spec.DisplayName = strings.Repeat("a", 65)
			}, "spec.displayName",
		),

		Entry(
			"admits a release list without a default", func(e *fleetv1alpha1.Environment) {
				e.Spec.Releases.Default = ""
			}, "",
		),
		Entry(
			"refuses a release default outside the allowed releases", func(e *fleetv1alpha1.Environment) {
				e.Spec.Releases.Default = "camunda-8-8"
			}, "releases.default must be one of releases.allowed",
		),
		Entry(
			"refuses an empty allowed release", func(e *fleetv1alpha1.Environment) {
				e.Spec.Releases = &fleetv1alpha1.EnvironmentReleases{Allowed: []string{""}}
			}, "spec.releases.allowed[0]",
		),

		Entry(
			"admits a section without a default", func(e *fleetv1alpha1.Environment) {
				setSection(e, fleetv1alpha1.SectionPolicy{Allowed: []string{"s3-backups"}})
			}, "",
		),
		Entry(
			"refuses a section default outside allowed", func(e *fleetv1alpha1.Environment) {
				setSection(e, fleetv1alpha1.SectionPolicy{Default: "gcs-backups", Allowed: []string{"s3-backups"}})
			}, "default must be one of allowed",
		),
		Entry(
			"admits an optional section with no allowed registration", func(e *fleetv1alpha1.Environment) {
				setSection(e, fleetv1alpha1.SectionPolicy{})
			}, "",
		),
		Entry(
			"refuses a required section with no allowed registration", func(e *fleetv1alpha1.Environment) {
				setSection(e, fleetv1alpha1.SectionPolicy{Required: true})
			}, "required needs at least one registration in allowed",
		),
		Entry(
			"refuses a required section whose only allowed registration is empty", func(e *fleetv1alpha1.Environment) {
				setSection(e, fleetv1alpha1.SectionPolicy{Required: true, Allowed: []string{""}})
			}, "allowed[0]",
		),

		Entry(
			"refuses the environment label in the namespace labels", func(e *fleetv1alpha1.Environment) {
				e.Spec.NamespaceCreation.Labels[fleetv1alpha1.LabelEnvironment] = "staging"
			}, "the environment label is set by the app",
		),
		Entry(
			"refuses the environment label in the required label keys", func(e *fleetv1alpha1.Environment) {
				e.Spec.NamespaceCreation.RequiredLabelKeys = []string{fleetv1alpha1.LabelEnvironment}
			}, "the environment label is set by the app",
		),
		Entry(
			"refuses a namespace label whose key is not a label key", func(e *fleetv1alpha1.Environment) {
				e.Spec.NamespaceCreation.Labels["Team Name"] = "platform"
			}, "namespaceCreation.labels must have valid label keys",
		),
		Entry(
			"refuses a namespace label whose value is not a label value", func(e *fleetv1alpha1.Environment) {
				e.Spec.NamespaceCreation.Labels["team"] = "platform team"
			}, "spec.namespaceCreation.labels.team",
		),
		Entry(
			"refuses a required label key that is not a label key", func(e *fleetv1alpha1.Environment) {
				e.Spec.NamespaceCreation.RequiredLabelKeys = []string{"Team Name"}
			}, "namespaceCreation.requiredLabelKeys must be valid label keys",
		),
		Entry(
			"refuses a required label key whose name is longer than 63 characters", func(e *fleetv1alpha1.Environment) {
				e.Spec.NamespaceCreation.RequiredLabelKeys = []string{"example.com/" + strings.Repeat("a", 64)}
			}, "namespaceCreation.requiredLabelKeys must be valid label keys",
		),

		Entry(
			"admits a parameter value without claimEditable", func(e *fleetv1alpha1.Environment) {
				setPolicy(e, fleetv1alpha1.ParameterPolicy{Value: &apiextensionsv1.JSON{Raw: []byte(`3`)}})
			}, "",
		),
		Entry(
			"refuses allowed choices without claimEditable", func(e *fleetv1alpha1.Environment) {
				p := policy(e)
				p.ClaimEditable = false
				setPolicy(e, p)
			}, "allowed needs claimEditable: true",
		),
		Entry(
			"refuses an allowed item without a name", func(e *fleetv1alpha1.Environment) {
				p := policy(e)
				p.Allowed = []fleetv1alpha1.NamedValue{choice("", "SQS access")}
				setPolicy(e, p)
			}, "allowed[0].name",
		),
		Entry(
			"refuses an allowed item without a label", func(e *fleetv1alpha1.Environment) {
				p := policy(e)
				p.Allowed = []fleetv1alpha1.NamedValue{choice("sqs", "")}
				setPolicy(e, p)
			}, "allowed[0].label",
		),
		Entry(
			"refuses an allowed item without a value", func(e *fleetv1alpha1.Environment) {
				p := policy(e)
				p.Allowed = []fleetv1alpha1.NamedValue{{Name: "sqs", Label: "SQS access"}}
				setPolicy(e, p)
			}, "allowed[0].value",
		),
		Entry(
			"refuses two allowed items with one name", func(e *fleetv1alpha1.Environment) {
				p := policy(e)
				p.Allowed = []fleetv1alpha1.NamedValue{choice("sqs", "SQS access"), choice("sqs", "SQS again")}
				setPolicy(e, p)
			}, "Duplicate value",
		),

		Entry(
			"refuses a deletion policy outside Delete and Orphan", func(e *fleetv1alpha1.Environment) {
				e.Spec.DeletionPolicy = "Retain"
			}, "spec.deletionPolicy",
		),
	)

	DescribeTable(
		"admission of an explicit empty allowed list",
		func(value map[string]any, path []string, wantErr string) {
			obj := unstructuredEnvironment(value, path...)
			err := k8sClient.Create(ctx, obj)
			if wantErr == "" {
				Expect(err).NotTo(HaveOccurred())
				DeferCleanup(func() { _ = k8sClient.Delete(ctx, obj) })
			} else {
				Expect(err).To(MatchError(ContainSubstring(wantErr)))
			}
		},
		Entry(
			"admits a parameter policy with an empty allowed list and without claimEditable",
			map[string]any{"value": int64(3), "allowed": []any{}},
			[]string{"spec", "parameters", "aws-connectors", "policies"},
			"",
		),
		Entry(
			"refuses a required section with an empty allowed list",
			map[string]any{"required": true, "allowed": []any{}},
			[]string{"spec", "sections", "backup-storage"},
			"required needs at least one registration in allowed",
		),
	)

	It("defaults the deletion policy to Delete", func() {
		obj := validEnvironment()
		Expect(k8sClient.Create(ctx, obj)).To(Succeed())
		DeferCleanup(func() { _ = k8sClient.Delete(ctx, obj) })

		var got fleetv1alpha1.Environment
		Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(obj), &got)).To(Succeed())
		Expect(got.Spec.DeletionPolicy).To(Equal(pluginsv1alpha1.DeletionPolicyDelete))
	})

	It("keeps a namespace count of zero in the status", func() {
		obj := validEnvironment()
		Expect(k8sClient.Create(ctx, obj)).To(Succeed())
		DeferCleanup(func() { _ = k8sClient.Delete(ctx, obj) })

		obj.Status.Namespaces = ptr.To[int32](0)
		Expect(k8sClient.Status().Update(ctx, obj)).To(Succeed())

		got := &unstructured.Unstructured{}
		got.SetGroupVersionKind(fleetv1alpha1.GroupVersion.WithKind("Environment"))
		Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(obj), got)).To(Succeed())

		count, found, err := unstructured.NestedInt64(got.Object, "status", "namespaces")
		Expect(err).NotTo(HaveOccurred())
		Expect(found).To(BeTrue(), "status.namespaces is missing")
		Expect(count).To(BeZero())
	})
})
