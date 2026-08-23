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

package controller

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	utilrand "k8s.io/apimachinery/pkg/util/rand"

	v1 "github.com/konsole-is/camunda-operator/api/v1"
	"github.com/konsole-is/camunda-operator/internal/fixtures"
)

// validManagementCluster returns the minimal oidc-mode example of the CRD doc
// with a unique name. The caller chooses the namespace.
func validManagementCluster() *v1.CamundaManagementCluster {
	return &v1.CamundaManagementCluster{
		ObjectMeta: metav1.ObjectMeta{Name: "mc-" + utilrand.String(8)},
		Spec: v1.CamundaManagementClusterSpec{
			PlatformConfigRef: "my-platform",
			IdentityProvider:  v1.IdentityProviderSpec{OIDC: &v1.ManagementOIDCSpec{}},
			Identity: v1.IdentitySpec{
				Version:           "8.9.0",
				ExternalURL:       "https://identity.example.com",
				DatabaseConfigRef: "identity-db",
				Admin: v1.IdentityAdminSpec{
					ClaimName:  "oid",
					ClaimValue: "1f8c0e2a-2b1a-4a5f-9f0d-2b0a1c3d4e5f",
				},
			},
		},
	}
}

// realisticManagementCluster returns the realistic keycloak-mode example of
// the CRD doc with a unique name. The caller chooses the namespace.
func realisticManagementCluster() *v1.CamundaManagementCluster {
	return &v1.CamundaManagementCluster{
		ObjectMeta: metav1.ObjectMeta{Name: "mc-" + utilrand.String(8)},
		Spec: v1.CamundaManagementClusterSpec{
			PlatformConfigRef: "my-platform",
			ClusterSelector: &metav1.LabelSelector{
				MatchLabels: map[string]string{"camunda.io/tier": "production"},
			},
			IdentityProvider: v1.IdentityProviderSpec{
				Keycloak: &v1.ManagedKeycloakSpec{
					Version:           "26.0.7",
					ExternalURL:       "https://keycloak.example.com/auth",
					DatabaseConfigRef: "keycloak-db",
				},
			},
			Identity: v1.IdentitySpec{
				Version:           "8.9.0",
				ExternalURL:       "https://identity.example.com",
				DatabaseConfigRef: "identity-db",
				Admin:             v1.IdentityAdminSpec{Username: "admin"},
			},
			Console: &v1.ConsoleSpec{
				Version:     "8.9.0",
				ExternalURL: "https://console.example.com",
			},
			WebModeler: &v1.WebModelerSpec{
				Version:               "8.9.0",
				ExternalURL:           "https://modeler.example.com",
				WebsocketsExternalURL: "https://modeler-ws.example.com",
				DatabaseConfigRef:     "web-modeler-db",
				Mail: v1.WebModelerMailSpec{
					SMTPHost:    "smtp.example.com",
					FromAddress: "camunda@example.com",
				},
			},
		},
	}
}

var _ = Describe("CamundaManagementCluster schema", func() {
	DescribeTable(
		"admission",
		func(
			build func() *v1.CamundaManagementCluster,
			mutate func(*v1.CamundaManagementCluster),
			wantErr string,
		) {
			obj := build()
			obj.Namespace = fixtures.SchemaTestNamespace
			mutate(obj)
			err := k8sClient.Create(ctx, obj)
			if wantErr == "" {
				Expect(err).NotTo(HaveOccurred())
				DeferCleanup(func() { _ = k8sClient.Delete(ctx, obj) })
			} else {
				Expect(err).To(MatchError(ContainSubstring(wantErr)))
			}
		},
		Entry(
			"accepts the minimal doc example",
			validManagementCluster, func(*v1.CamundaManagementCluster) {}, "",
		),
		Entry(
			"accepts the realistic doc example",
			realisticManagementCluster, func(*v1.CamundaManagementCluster) {}, "",
		),
		Entry(
			"rejects two identity providers",
			validManagementCluster, func(o *v1.CamundaManagementCluster) {
				o.Spec.IdentityProvider.Keycloak = &v1.ManagedKeycloakSpec{
					Version:           "26.0.7",
					ExternalURL:       "https://keycloak.example.com/auth",
					DatabaseConfigRef: "keycloak-db",
				}
			}, "exactly one identity provider",
		),
		Entry(
			"rejects no identity provider",
			validManagementCluster, func(o *v1.CamundaManagementCluster) {
				o.Spec.IdentityProvider.OIDC = nil
			}, "exactly one identity provider",
		),
		Entry(
			"rejects an identity externalUrl without a scheme",
			validManagementCluster, func(o *v1.CamundaManagementCluster) {
				o.Spec.Identity.ExternalURL = "identity.example.com"
			}, "externalUrl must be a valid http or https URL",
		),
		Entry(
			"rejects an identity externalUrl without a host",
			validManagementCluster, func(o *v1.CamundaManagementCluster) {
				o.Spec.Identity.ExternalURL = "https://"
			}, "externalUrl must be a valid http or https URL",
		),
		Entry(
			"rejects a Web Modeler without a mail fromAddress",
			realisticManagementCluster, func(o *v1.CamundaManagementCluster) {
				o.Spec.WebModeler.Mail.FromAddress = ""
			}, "fromAddress",
		),
		Entry(
			"rejects a websocketsExternalUrl without a scheme",
			realisticManagementCluster, func(o *v1.CamundaManagementCluster) {
				o.Spec.WebModeler.WebsocketsExternalURL = "modeler-ws.example.com"
			}, "websocketsExternalUrl must be a valid http or https URL",
		),
		Entry(
			"rejects a websocketsExternalUrl without a host",
			realisticManagementCluster, func(o *v1.CamundaManagementCluster) {
				o.Spec.WebModeler.WebsocketsExternalURL = "https://"
			}, "websocketsExternalUrl must be a valid http or https URL",
		),
		Entry(
			"rejects an admin with both a claim and a username",
			validManagementCluster, func(o *v1.CamundaManagementCluster) {
				o.Spec.Identity.Admin.Username = "admin"
			}, "set claimName and claimValue",
		),
		Entry(
			"rejects an admin with neither a claim nor a username",
			validManagementCluster, func(o *v1.CamundaManagementCluster) {
				o.Spec.Identity.Admin = v1.IdentityAdminSpec{}
			}, "set claimName and claimValue",
		),
		Entry(
			"rejects an admin with a claimName and no claimValue",
			validManagementCluster, func(o *v1.CamundaManagementCluster) {
				o.Spec.Identity.Admin = v1.IdentityAdminSpec{ClaimName: "oid", Username: "admin"}
			}, "set claimName and claimValue together",
		),
		Entry(
			"rejects a two-segment Keycloak version",
			realisticManagementCluster, func(o *v1.CamundaManagementCluster) {
				o.Spec.IdentityProvider.Keycloak.Version = "26.0"
			}, "version",
		),
		Entry(
			"rejects an empty platformConfigRef",
			validManagementCluster, func(o *v1.CamundaManagementCluster) {
				o.Spec.PlatformConfigRef = ""
			}, "platformConfigRef",
		),
	)

	// The typed client drops an empty string under omitempty, so only an
	// unstructured object carries one to the API server.
	DescribeTable(
		"an explicitly empty admin field",
		func(build func() *v1.CamundaManagementCluster, field string) {
			obj := build()
			obj.Namespace = fixtures.SchemaTestNamespace
			raw, err := runtime.DefaultUnstructuredConverter.ToUnstructured(obj)
			Expect(err).NotTo(HaveOccurred())

			u := &unstructured.Unstructured{Object: raw}
			u.SetAPIVersion(v1.GroupVersion.String())
			u.SetKind("CamundaManagementCluster")
			Expect(
				unstructured.SetNestedField(u.Object, "", "spec", "identity", "admin", field),
			).To(Succeed())

			Expect(k8sClient.Create(ctx, u)).To(MatchError(And(
				ContainSubstring("spec.identity.admin."+field),
				ContainSubstring("should be at least 1 chars long"),
			)))
		},
		Entry("rejects an empty claimName", validManagementCluster, "claimName"),
		Entry("rejects an empty claimValue", validManagementCluster, "claimValue"),
		Entry("rejects an empty username", realisticManagementCluster, "username"),
	)
})
