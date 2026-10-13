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
	"sigs.k8s.io/controller-runtime/pkg/client"

	pluginsv1alpha1 "github.com/konsole-is/camunda-operator/api/plugins/v1alpha1"
	v1 "github.com/konsole-is/camunda-operator/api/v1"
)

var _ = Describe("Grant schema", func() {
	It("admits a name of 63 characters", func() {
		grant := newGrant()
		grant.Name = strings.Repeat("a", 63)

		Expect(createGrant(grant)).To(Succeed())
	})

	It("refuses a name of 64 characters", func() {
		grant := newGrant()
		grant.Name = strings.Repeat("a", 64)

		Expect(createGrant(grant)).To(MatchError(ContainSubstring(
			"metadata.name must be at most 63 characters, because it is used as a label value",
		)))
	})

	DescribeTable(
		"refuses an update of an immutable field",
		func(change func(*pluginsv1alpha1.Grant), message string) {
			grant := newGrant()
			Expect(createGrant(grant)).To(Succeed())

			change(grant)

			Expect(k8sClient.Update(ctx, grant)).To(MatchError(ContainSubstring(message)))
		},
		Entry(
			"cluster",
			func(g *pluginsv1alpha1.Grant) { g.Spec.Cluster.Name = "billing" },
			"cluster is immutable",
		),
		Entry(
			"template",
			func(g *pluginsv1alpha1.Grant) { g.Spec.Template = "bucket-read" },
			"template is immutable",
		),
	)

	It("refuses an update of the format", func() {
		grant := newGrant()
		Expect(createGrant(grant)).To(Succeed())

		grant.Spec.Format = "gcp-iam-binding"

		Expect(k8sClient.Update(ctx, grant)).To(MatchError(ContainSubstring("spec.format: Unsupported value")))
	})

	// The API server skips CEL rules after an enum error, so the message of
	// this rule is reachable only once a second format exists.
	It("serves format with a rule that keeps it immutable", func() {
		served := &unstructured.Unstructured{}
		served.SetGroupVersionKind(apiextensionsv1.SchemeGroupVersion.WithKind("CustomResourceDefinition"))
		Expect(k8sClient.Get(ctx, client.ObjectKey{Name: "grants.plugins.camunda.io"}, served)).To(Succeed())

		var crd apiextensionsv1.CustomResourceDefinition
		Expect(runtime.DefaultUnstructuredConverter.FromUnstructured(served.Object, &crd)).To(Succeed())

		Expect(crd.Spec.Versions).To(HaveLen(1))
		format := crd.Spec.Versions[0].Schema.OpenAPIV3Schema.Properties["spec"].Properties["format"]
		Expect(format.XValidations).To(ContainElement(SatisfyAll(
			HaveField("Rule", "self == oldSelf"),
			HaveField("Message", ContainSubstring("format is immutable")),
		)))
	})

	// The plugin renders the fragment again when the template or its inputs
	// change, so the fragment must stay writable.
	It("admits an update of the fragment", func() {
		grant := newGrant()
		Expect(createGrant(grant)).To(Succeed())

		grant.Spec.Fragment = &apiextensionsv1.JSON{
			Raw: []byte(`{"Effect":"Allow","Action":["s3:GetObject"],"Resource":["arn:aws:s3:::orders-backups/*"]}`),
		}

		Expect(k8sClient.Update(ctx, grant)).To(Succeed())
	})

	It("refuses a fragment that is a JSON array", func() {
		grant := newGrant()
		grant.Spec.Fragment = &apiextensionsv1.JSON{Raw: []byte(`[{"Effect":"Allow"}]`)}

		err := createGrant(grant)

		Expect(err).To(MatchError(ContainSubstring("spec.fragment")))
		Expect(err).To(MatchError(ContainSubstring("must be of type object")))
	})
})

// newGrant returns a valid grant with a generated name, in the shape that a
// plugin controller writes.
func newGrant() *pluginsv1alpha1.Grant {
	return &pluginsv1alpha1.Grant{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: "orders-backup-storage-",
			Namespace:    metav1.NamespaceDefault,
		},
		Spec: pluginsv1alpha1.GrantSpec{
			Cluster:  v1.ClusterRef{Name: "orders"},
			Template: "bucket-access",
			Format:   pluginsv1alpha1.GrantFormatAWSIAMStatement,
			Fragment: &apiextensionsv1.JSON{
				Raw: []byte(
					`{"Effect":"Allow","Action":["s3:GetObject","s3:PutObject"],"Resource":["arn:aws:s3:::orders-backups/*"]}`,
				),
			},
		},
	}
}

// createGrant creates the grant and deletes it when the spec ends.
func createGrant(grant *pluginsv1alpha1.Grant) error {
	if err := k8sClient.Create(ctx, grant); err != nil {
		return err
	}

	DeferCleanup(k8sClient.Delete, ctx, grant)

	return nil
}
