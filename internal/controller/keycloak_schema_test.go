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
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	utilrand "k8s.io/apimachinery/pkg/util/rand"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/konsole-is/camunda-operator/internal/fixtures"
	"github.com/konsole-is/camunda-operator/pkg/wrappers/keycloak"
)

// The Keycloak types are hand written against the CRD vendored in
// internal/testenv/crds/keycloak, because the Keycloak project publishes no Go
// module for them. A field that the schema does not declare is pruned on
// write, without an error, and the operator would then run a Keycloak that
// silently ignores what the spec asked for. This spec writes every field the
// operator sets and reads the object back.
var _ = Describe("Keycloak types", func() {
	It("round-trips every field the operator sets through the vendored schema", func() {
		kc := &keycloak.Keycloak{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "kc-" + utilrand.String(8),
				Namespace: fixtures.SchemaTestNamespace,
			},
			Spec: keycloak.KeycloakSpec{
				Instances: ptr.To(int32(2)),
				Image:     "camunda/keycloak:quay-optimized-26.0.7",
				DB: &keycloak.KeycloakDBSpec{
					Vendor:   "postgres",
					Host:     "postgres.my-cluster-ns.svc",
					Port:     ptr.To(int32(5432)),
					Database: "keycloak",
					UsernameSecret: &corev1.SecretKeySelector{
						LocalObjectReference: corev1.LocalObjectReference{Name: "keycloak-db"},
						Key:                  "username",
					},
					PasswordSecret: &corev1.SecretKeySelector{
						LocalObjectReference: corev1.LocalObjectReference{Name: "keycloak-db"},
						Key:                  "password",
					},
				},
				HTTP: &keycloak.KeycloakHTTPSpec{
					HTTPEnabled: ptr.To(true),
					HTTPPort:    ptr.To(int32(8080)),
				},
				Hostname: &keycloak.KeycloakHostnameSpec{
					Hostname: "https://keycloak.example.com/auth",
					Strict:   ptr.To(false),
				},
				Ingress: &keycloak.KeycloakIngressSpec{Enabled: ptr.To(false)},
				AdditionalOptions: []keycloak.KeycloakValueOrSecret{
					{Name: "http-relative-path", Value: "/auth"},
					{Name: "proxy-headers", Value: "xforwarded"},
					{Name: "log-level", Secret: &corev1.SecretKeySelector{
						LocalObjectReference: corev1.LocalObjectReference{Name: "keycloak-options"},
						Key:                  "log-level",
					}},
				},
				Unsupported: &keycloak.KeycloakUnsupportedSpec{
					PodTemplate: &corev1.PodTemplateSpec{
						ObjectMeta: metav1.ObjectMeta{
							Labels: map[string]string{"camunda.io/component": "keycloak"},
						},
					},
				},
				Resources: &corev1.ResourceRequirements{
					Requests: corev1.ResourceList{
						corev1.ResourceCPU:    resource.MustParse("500m"),
						corev1.ResourceMemory: resource.MustParse("1Gi"),
					},
					Limits: corev1.ResourceList{corev1.ResourceMemory: resource.MustParse("2Gi")},
				},
			},
		}
		want := kc.Spec.DeepCopy()

		Expect(k8sClient.Create(ctx, kc)).To(Succeed())
		DeferCleanup(func() { _ = k8sClient.Delete(ctx, kc) })

		var stored keycloak.Keycloak
		Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(kc), &stored)).To(Succeed())
		Expect(&stored.Spec).To(Equal(want))
	})

	It("reads the conditions of the status subresource", func() {
		kc := &keycloak.Keycloak{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "kc-" + utilrand.String(8),
				Namespace: fixtures.SchemaTestNamespace,
			},
			Spec: keycloak.KeycloakSpec{Instances: ptr.To(int32(1))},
		}
		Expect(k8sClient.Create(ctx, kc)).To(Succeed())
		DeferCleanup(func() { _ = k8sClient.Delete(ctx, kc) })

		kc.Status = keycloak.KeycloakStatus{
			Instances: 1,
			Conditions: []keycloak.KeycloakCondition{{
				Type:    keycloak.ConditionReady,
				Status:  "True",
				Message: "Keycloak is ready",
			}},
		}
		Expect(k8sClient.Status().Update(ctx, kc)).To(Succeed())

		var stored keycloak.Keycloak
		Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(kc), &stored)).To(Succeed())
		Expect(stored.Status.Instances).To(Equal(int32(1)))
		Expect(stored.Status.Conditions).To(HaveLen(1))
		Expect(stored.Status.Conditions[0].Type).To(Equal(keycloak.ConditionReady))
		Expect(stored.Status.Conditions[0].Status).To(Equal("True"))
	})
})
