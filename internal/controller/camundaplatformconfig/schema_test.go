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

package camundaplatformconfig

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	utilrand "k8s.io/apimachinery/pkg/util/rand"
	"sigs.k8s.io/controller-runtime/pkg/client"

	v1 "github.com/konsole-is/camunda-operator/api/v1"
	"github.com/konsole-is/camunda-operator/internal/fixtures"
)

// minimalPlatformConfig returns the minimal example of the CRD doc with a
// unique name: basic authentication and nothing else.
func minimalPlatformConfig() *v1.CamundaPlatformConfig {
	return &v1.CamundaPlatformConfig{
		ObjectMeta: metav1.ObjectMeta{Name: "pfc-" + utilrand.String(8)},
		Spec: v1.CamundaPlatformConfigSpec{
			Auth: &v1.PlatformAuthSpec{Method: v1.AuthenticationMethodBasic},
		},
	}
}

// realisticPlatformConfig returns the realistic example of the CRD doc with a
// unique name: OIDC, a license, and an image registry.
func realisticPlatformConfig() *v1.CamundaPlatformConfig {
	return &v1.CamundaPlatformConfig{
		ObjectMeta: metav1.ObjectMeta{Name: "pfc-" + utilrand.String(8)},
		Spec: v1.CamundaPlatformConfigSpec{
			Auth: &v1.PlatformAuthSpec{
				Method: v1.AuthenticationMethodOIDC,
				OIDC: &v1.OIDCSpec{
					IssuerURL: "https://login.example.com/realms/camunda",
					ClientID:  "camunda-orchestration",
					Audience:  "camunda-orchestration",
					ClientSecretRef: v1.SecretKeyRef{
						Name: "oidc-credentials", Namespace: "camunda-system", Key: "client-secret",
					},
				},
			},
			LicenseSecretRef: &v1.SecretKeyRef{
				Name: "camunda-license", Namespace: "camunda-system", Key: "license-key",
			},
			ImageRegistry: "registry.example.com/camunda",
		},
	}
}

var _ = Describe("CamundaPlatformConfig schema", func() {
	DescribeTable(
		"admission",
		func(build func() *v1.CamundaPlatformConfig, mutate func(*v1.CamundaPlatformConfig), wantErr string) {
			obj := build()
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
			minimalPlatformConfig, func(*v1.CamundaPlatformConfig) {}, "",
		),
		Entry(
			"accepts the realistic doc example",
			realisticPlatformConfig, func(*v1.CamundaPlatformConfig) {}, "",
		),
		Entry(
			"accepts an empty spec",
			minimalPlatformConfig, func(o *v1.CamundaPlatformConfig) {
				o.Spec = v1.CamundaPlatformConfigSpec{}
			}, "",
		),
		Entry(
			"accepts explicit discovery endpoints",
			realisticPlatformConfig, func(o *v1.CamundaPlatformConfig) {
				o.Spec.Auth.OIDC.JWKSURL = "https://login.example.com/realms/camunda/protocol/openid-connect/certs"
				o.Spec.Auth.OIDC.TokenURL = "https://login.example.com/realms/camunda/protocol/openid-connect/token"
				o.Spec.Auth.OIDC.AuthURL = "https://login.example.com/realms/camunda/protocol/openid-connect/auth"
			}, "",
		),
		Entry(
			"rejects method oidc without an oidc block",
			realisticPlatformConfig, func(o *v1.CamundaPlatformConfig) {
				o.Spec.Auth.OIDC = nil
			}, "oidc is required when method is oidc",
		),
		Entry(
			"rejects method basic with an oidc block",
			realisticPlatformConfig, func(o *v1.CamundaPlatformConfig) {
				o.Spec.Auth.Method = v1.AuthenticationMethodBasic
			}, "must not be set when method is basic",
		),
		Entry(
			"rejects an oidc block when method is omitted and defaults to basic",
			realisticPlatformConfig, func(o *v1.CamundaPlatformConfig) {
				o.Spec.Auth.Method = ""
			}, "must not be set when method is basic",
		),
		Entry(
			"rejects an unknown method",
			minimalPlatformConfig, func(o *v1.CamundaPlatformConfig) {
				o.Spec.Auth.Method = "saml"
			}, "method",
		),
		Entry(
			"rejects an oidc block without issuerUrl",
			realisticPlatformConfig, func(o *v1.CamundaPlatformConfig) {
				o.Spec.Auth.OIDC.IssuerURL = ""
			}, "issuerUrl",
		),
		Entry(
			"rejects an issuerUrl that is not a URL",
			realisticPlatformConfig, func(o *v1.CamundaPlatformConfig) {
				o.Spec.Auth.OIDC.IssuerURL = fixtures.NotAURL
			}, "issuerUrl must be a valid http or https URL",
		),
		Entry(
			"rejects a jwksUrl with an ftp scheme",
			realisticPlatformConfig, func(o *v1.CamundaPlatformConfig) {
				o.Spec.Auth.OIDC.JWKSURL = "ftp://login.example.com/certs"
			}, "jwksUrl must be empty or a valid http or https URL",
		),
		Entry(
			"rejects a tokenUrl that is not a URL",
			realisticPlatformConfig, func(o *v1.CamundaPlatformConfig) {
				o.Spec.Auth.OIDC.TokenURL = fixtures.NotAURL
			}, "tokenUrl must be empty or a valid http or https URL",
		),
		Entry(
			"rejects an authUrl that is not a URL",
			realisticPlatformConfig, func(o *v1.CamundaPlatformConfig) {
				o.Spec.Auth.OIDC.AuthURL = fixtures.NotAURL
			}, "authUrl must be empty or a valid http or https URL",
		),
		Entry(
			"rejects an oidc block without clientId",
			realisticPlatformConfig, func(o *v1.CamundaPlatformConfig) {
				o.Spec.Auth.OIDC.ClientID = ""
			}, "clientId",
		),
		Entry(
			"rejects a clientSecretRef without key",
			realisticPlatformConfig, func(o *v1.CamundaPlatformConfig) {
				o.Spec.Auth.OIDC.ClientSecretRef.Key = ""
			}, "key",
		),
		Entry(
			"accepts an image override on a registry with a port",
			realisticPlatformConfig, func(o *v1.CamundaPlatformConfig) {
				o.Spec.Images = &v1.ImagesSpec{Optimize: "registry:5000/camunda/optimize"}
			}, "",
		),
		Entry(
			"rejects an image override with a tag",
			realisticPlatformConfig, func(o *v1.CamundaPlatformConfig) {
				o.Spec.Images = &v1.ImagesSpec{Optimize: "camunda/optimize:8.9.0"}
			}, "spec.images.optimize",
		),
		Entry(
			"rejects an image override with a digest",
			realisticPlatformConfig, func(o *v1.CamundaPlatformConfig) {
				o.Spec.Images = &v1.ImagesSpec{Optimize: "camunda/optimize@sha256:0f1e2d"}
			}, "spec.images.optimize",
		),
		Entry(
			"rejects a licenseSecretRef without namespace",
			realisticPlatformConfig, func(o *v1.CamundaPlatformConfig) {
				o.Spec.LicenseSecretRef.Namespace = ""
			}, "namespace",
		),
	)

	It("defaults method to basic", func() {
		obj := minimalPlatformConfig()
		obj.Spec.Auth = &v1.PlatformAuthSpec{}

		Expect(k8sClient.Create(ctx, obj)).To(Succeed())
		DeferCleanup(func() { _ = k8sClient.Delete(ctx, obj) })

		var fetched v1.CamundaPlatformConfig
		Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(obj), &fetched)).To(Succeed())
		Expect(fetched.Spec.Auth.Method).To(Equal(v1.AuthenticationMethodBasic))
	})

	It("round-trips the realistic doc example", func() {
		obj := realisticPlatformConfig()

		Expect(k8sClient.Create(ctx, obj)).To(Succeed())
		DeferCleanup(func() { _ = k8sClient.Delete(ctx, obj) })

		var fetched v1.CamundaPlatformConfig
		Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(obj), &fetched)).To(Succeed())
		Expect(fetched.Spec).To(Equal(obj.Spec))
	})
})
