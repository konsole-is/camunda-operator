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
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	utilrand "k8s.io/apimachinery/pkg/util/rand"
	"sigs.k8s.io/controller-runtime/pkg/client"

	pluginsv1alpha1 "github.com/konsole-is/camunda-operator/api/plugins/v1alpha1"
)

// validPluginRegistration returns a registration for the backup-storage
// section under a unique name. It has one input, one output, and one grant
// template that requires the input.
func validPluginRegistration() *pluginsv1alpha1.PluginRegistration {
	return &pluginsv1alpha1.PluginRegistration{
		ObjectMeta: metav1.ObjectMeta{Name: "registration-" + utilrand.String(8)},
		Spec: pluginsv1alpha1.PluginRegistrationSpec{
			Sections: []string{"backup-storage"},
			Label:    "New S3 bucket",
			Mode:     pluginsv1alpha1.ModeOptional,
			ServiceAccount: pluginsv1alpha1.ServiceAccountRef{
				Namespace: "camunda-plugins",
				Name:      "s3-backup-bucket-controller",
			},
			Parameters: &apiextensionsv1.JSON{
				Raw: []byte(`{"type":"object","properties":{"retentionDays":{"type":"integer","minimum":1}}}`),
			},
			ParameterValues: &apiextensionsv1.JSON{Raw: []byte(`{"retentionDays":30}`)},
			Inputs:          []pluginsv1alpha1.InputDecl{{Name: "kmsKey", From: "encryption-key/keyRef"}},
			Outputs:         []pluginsv1alpha1.OutputDecl{{Key: "objectStorageConfig"}},
			GrantTemplates: []pluginsv1alpha1.GrantTemplate{{
				Name:   "key-access",
				Format: pluginsv1alpha1.GrantFormatAWSIAMStatement,
				Template: &apiextensionsv1.JSON{
					Raw: []byte(`{"Effect":"Allow","Action":["kms:Decrypt"],"Resource":"*"}`),
				},
				RequiresInputs: []string{"kmsKey"},
			}},
			DeletionPolicies: []pluginsv1alpha1.DeletionPolicy{pluginsv1alpha1.DeletionPolicyDelete},
		},
	}
}

var _ = Describe("PluginRegistration schema", func() {
	DescribeTable(
		"admission",
		func(mutate func(*pluginsv1alpha1.PluginRegistration), wantErr string) {
			obj := validPluginRegistration()
			mutate(obj)
			expectAdmission(obj, wantErr)
		},
		Entry(
			"admits a registration with an input that a grant template requires",
			func(*pluginsv1alpha1.PluginRegistration) {}, "",
		),
		Entry(
			"admits a name of 63 characters",
			func(o *pluginsv1alpha1.PluginRegistration) { o.Name = nameOfLength(63) }, "",
		),
		Entry(
			"refuses a name of 64 characters",
			func(o *pluginsv1alpha1.PluginRegistration) { o.Name = nameOfLength(64) },
			"metadata.name must be at most 63 characters",
		),
		Entry(
			"admits a registration for two sections",
			func(o *pluginsv1alpha1.PluginRegistration) {
				o.Spec.Sections = append(o.Spec.Sections, "encryption-key")
			}, "",
		),
		Entry(
			"refuses an empty sections list",
			func(o *pluginsv1alpha1.PluginRegistration) { o.Spec.Sections = []string{} },
			"sections must list at least one section",
		),
		Entry(
			"admits deletionPolicies Delete and Orphan",
			func(o *pluginsv1alpha1.PluginRegistration) {
				o.Spec.DeletionPolicies = []pluginsv1alpha1.DeletionPolicy{
					pluginsv1alpha1.DeletionPolicyDelete,
					pluginsv1alpha1.DeletionPolicyOrphan,
				}
			}, "",
		),
		Entry(
			"refuses deletionPolicies without Delete",
			func(o *pluginsv1alpha1.PluginRegistration) {
				o.Spec.DeletionPolicies = []pluginsv1alpha1.DeletionPolicy{pluginsv1alpha1.DeletionPolicyOrphan}
			},
			"deletionPolicies must contain Delete",
		),
		Entry(
			"refuses an empty deletionPolicies list",
			func(o *pluginsv1alpha1.PluginRegistration) {
				o.Spec.DeletionPolicies = []pluginsv1alpha1.DeletionPolicy{}
			},
			"deletionPolicies must contain Delete",
		),
		Entry(
			"refuses an input from a section without an output",
			func(o *pluginsv1alpha1.PluginRegistration) { o.Spec.Inputs[0].From = "backup-storage" },
			"spec.inputs[0].from: Invalid value",
		),
		Entry(
			"refuses a duplicate input name",
			func(o *pluginsv1alpha1.PluginRegistration) {
				o.Spec.Inputs = append(
					o.Spec.Inputs,
					pluginsv1alpha1.InputDecl{Name: "kmsKey", From: "cloud-identity/principal"},
				)
			},
			"spec.inputs[1]: Duplicate value",
		),
		Entry(
			"refuses a duplicate output key",
			func(o *pluginsv1alpha1.PluginRegistration) {
				o.Spec.Outputs = append(o.Spec.Outputs, pluginsv1alpha1.OutputDecl{Key: "objectStorageConfig"})
			},
			"spec.outputs[1]: Duplicate value",
		),
		Entry(
			"refuses a duplicate grant template name",
			func(o *pluginsv1alpha1.PluginRegistration) {
				o.Spec.GrantTemplates = append(o.Spec.GrantTemplates, o.Spec.GrantTemplates[0])
			},
			"spec.grantTemplates[1]: Duplicate value",
		),
		Entry(
			"refuses requiresInputs with an unknown input",
			func(o *pluginsv1alpha1.PluginRegistration) {
				o.Spec.GrantTemplates[0].RequiresInputs = []string{"roleArn"}
			},
			"each requiresInputs item of a grant template must name an input of the registration",
		),
		Entry(
			"refuses requiresInputs when the registration has no inputs",
			func(o *pluginsv1alpha1.PluginRegistration) { o.Spec.Inputs = nil },
			"each requiresInputs item of a grant template must name an input of the registration",
		),
	)

	It("keeps parameters and parameterValues as written", func() {
		obj := validPluginRegistration()
		expectAdmission(obj, "")

		var got pluginsv1alpha1.PluginRegistration
		Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(obj), &got)).To(Succeed())

		Expect(got.Spec.Parameters).NotTo(BeNil())
		Expect(got.Spec.Parameters.Raw).To(MatchJSON(obj.Spec.Parameters.Raw))
		Expect(got.Spec.ParameterValues).NotTo(BeNil())
		Expect(got.Spec.ParameterValues.Raw).To(MatchJSON(obj.Spec.ParameterValues.Raw))
	})
})
