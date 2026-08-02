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
	utilrand "k8s.io/apimachinery/pkg/util/rand"

	v1 "github.com/konsole-is/camunda-operator/api/v1"
)

// validManagementAuthConfig returns the doc's minimal example with a unique name.
func validManagementAuthConfig() *v1.ManagementAuthConfig {
	return &v1.ManagementAuthConfig{
		ObjectMeta: metav1.ObjectMeta{Name: "mac-" + utilrand.String(8)},
		Spec: v1.ManagementAuthConfigSpec{
			BaseURL:   "https://identity.camunda.example.com",
			IssuerURL: "https://identity.camunda.example.com/auth/realms/camunda-platform",
			AuthURL:   "https://identity.camunda.example.com/auth/realms/camunda-platform/protocol/openid-connect/auth",
			TokenURL:  "https://identity.camunda.example.com/auth/realms/camunda-platform/protocol/openid-connect/token",
			JwksURL:   "https://identity.camunda.example.com/auth/realms/camunda-platform/protocol/openid-connect/certs",
			ClientID:  "camunda-management",
			Audience:  "camunda-management",
			ClientSecretRef: v1.SecretKeyRef{
				Name: "management-auth-secret", Namespace: "camunda-system", Key: "client-secret",
			},
		},
	}
}

var _ = Describe("ManagementAuthConfig schema", func() {
	DescribeTable("admission",
		func(mutate func(*v1.ManagementAuthConfig), wantErr string) {
			obj := validManagementAuthConfig()
			mutate(obj)
			err := k8sClient.Create(ctx, obj)
			if wantErr == "" {
				Expect(err).NotTo(HaveOccurred())
				DeferCleanup(func() { _ = k8sClient.Delete(ctx, obj) })
			} else {
				Expect(err).To(MatchError(ContainSubstring(wantErr)))
			}
		},
		Entry("accepts the minimal doc example", func(*v1.ManagementAuthConfig) {}, ""),
		Entry("accepts with issuerBackendUrl", func(o *v1.ManagementAuthConfig) {
			o.Spec.IssuerBackendURL = "http://identity.camunda-management.svc.cluster.local/auth/realms/camunda-platform"
		}, ""),
		Entry("rejects non-URL baseUrl", func(o *v1.ManagementAuthConfig) {
			o.Spec.BaseURL = "not a url"
		}, "baseUrl"),
		Entry("rejects ftp tokenUrl", func(o *v1.ManagementAuthConfig) {
			o.Spec.TokenURL = "ftp://identity.camunda.example.com/token"
		}, "tokenUrl"),
		Entry("rejects non-URL jwksUrl", func(o *v1.ManagementAuthConfig) {
			o.Spec.JwksURL = "not a url"
		}, "jwksUrl"),
		Entry("rejects empty clientId", func(o *v1.ManagementAuthConfig) {
			o.Spec.ClientID = ""
		}, "clientId"),
		Entry("rejects clientSecretRef with empty key", func(o *v1.ManagementAuthConfig) {
			o.Spec.ClientSecretRef.Key = ""
		}, "key"),
	)
})
