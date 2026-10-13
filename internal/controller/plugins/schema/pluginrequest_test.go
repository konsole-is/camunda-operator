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
	"sigs.k8s.io/controller-runtime/pkg/client"

	pluginsv1alpha1 "github.com/konsole-is/camunda-operator/api/plugins/v1alpha1"
)

var _ = Describe("PluginRequest schema", func() {
	It("admits a name of 63 characters", func() {
		request := newRequest()
		request.Name = nameOfLength(63)

		expectAdmission(request, "")
	})

	It("refuses a name of 64 characters", func() {
		request := newRequest()
		request.Name = nameOfLength(64)

		expectAdmission(
			request,
			"metadata.name must be at most 63 characters, because it is used as a label value",
		)
	})

	DescribeTable(
		"refuses an update of an immutable field",
		func(change func(*pluginsv1alpha1.PluginRequest), message string) {
			request := newRequest()
			expectAdmission(request, "")

			change(request)

			Expect(k8sClient.Update(ctx, request)).To(MatchError(ContainSubstring(message)))
		},
		Entry(
			"section",
			func(r *pluginsv1alpha1.PluginRequest) { r.Spec.Section = "volume-encryption" },
			"section is immutable",
		),
		Entry(
			"registration",
			func(r *pluginsv1alpha1.PluginRequest) { r.Spec.Registration = "aws-s3-other" },
			"registration is immutable",
		),
		Entry(
			"claimRef.name",
			func(r *pluginsv1alpha1.PluginRequest) { r.Spec.ClaimRef.Name = "payments" },
			"claimRef is immutable",
		),
		Entry(
			"claimRef.kind",
			func(r *pluginsv1alpha1.PluginRequest) {
				r.Spec.ClaimRef.Kind = pluginsv1alpha1.ClaimKindCamundaManagementClusterClaim
			},
			"claimRef is immutable",
		),
		Entry(
			"target.name",
			func(r *pluginsv1alpha1.PluginRequest) { r.Spec.Target.Name = "payments" },
			"target.name is immutable",
		),
	)

	It("admits ServiceAccount names in target.additionalServiceAccounts", func() {
		request := newRequest()
		request.Spec.Target.AdditionalServiceAccounts = []string{"orders-es-es", "orders-pg-postgres"}

		expectAdmission(request, "")
	})

	// A reader looks up each name in the namespace of the request, so a
	// name of another namespace must never reach it.
	It("refuses a name with a namespace in target.additionalServiceAccounts", func() {
		request := newRequest()
		request.Spec.Target.AdditionalServiceAccounts = []string{"other-team/orders-es-es"}

		expectAdmission(request, "spec.target.additionalServiceAccounts[0] in body should match")
	})

	It("admits a ServiceAccount name in target.serviceAccountName", func() {
		request := newRequest()
		request.Spec.Target.ServiceAccountName = "orders.camunda"

		expectAdmission(request, "")
	})

	It("refuses a name with a namespace in target.serviceAccountName", func() {
		request := newRequest()
		request.Spec.Target.ServiceAccountName = "other-team/orders"

		expectAdmission(request, "spec.target.serviceAccountName in body should match")
	})

	// The claim controller applies the whole spec again on each reconcile,
	// so the fields that may change must stay writable.
	It("admits an update of the inputs, the parameters, the target ServiceAccount, and the deletion policy", func() {
		request := newRequest()
		expectAdmission(request, "")

		request.Spec.Inputs = map[string]string{"kmsKeyArn": "arn:aws:kms:eu-west-1:111122223333:key/1"}
		request.Spec.Parameters = &apiextensionsv1.JSON{Raw: []byte(`{"versioning":false}`)}
		request.Spec.Target.ServiceAccountName = "orders-connectors"
		request.Spec.DeletionPolicy = pluginsv1alpha1.DeletionPolicyOrphan

		Expect(k8sClient.Update(ctx, request)).To(Succeed())
	})

	It("defaults the severity of a summary item to Info after a status write", func() {
		request := newRequest()
		expectAdmission(request, "")

		request.Status.Summary = []pluginsv1alpha1.SummaryItem{{Label: "Bucket", Value: "orders-backups"}}
		Expect(k8sClient.Status().Update(ctx, request)).To(Succeed())

		var stored pluginsv1alpha1.PluginRequest
		Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(request), &stored)).To(Succeed())
		Expect(stored.Status.Summary).To(ConsistOf(
			HaveField("Severity", pluginsv1alpha1.SeverityInfo),
		))
	})
})

// newRequest returns a valid request with a generated name, in the shape that
// a claim controller writes.
func newRequest() *pluginsv1alpha1.PluginRequest {
	return &pluginsv1alpha1.PluginRequest{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: "orders-backup-storage-",
			Namespace:    metav1.NamespaceDefault,
		},
		Spec: pluginsv1alpha1.PluginRequestSpec{
			Section:      "backup-storage",
			Registration: "aws-s3",
			ClaimRef: pluginsv1alpha1.ClaimRef{
				Kind: pluginsv1alpha1.ClaimKindCamundaClusterClaim,
				Name: "orders",
			},
			Target: pluginsv1alpha1.TargetRef{
				Kind:               "CamundaCluster",
				Name:               "orders",
				ServiceAccountName: "orders-camunda",
			},
			Parameters:     &apiextensionsv1.JSON{Raw: []byte(`{"versioning":true}`)},
			DeletionPolicy: pluginsv1alpha1.DeletionPolicyDelete,
		},
	}
}
