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

package grantadmission

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/konsole-is/camunda-operator/test/envtest"
)

// RBAC bindings reach the authorizer a short time after they are created, so
// each spec waits for the policy outcome. A refusal from RBAC alone never
// matches a policy message, so no spec can pass early.
var _ = Describe("Grant admission on create", func() {
	var plugin, stranger client.Client

	BeforeEach(func() {
		plugin = asServiceAccount(pluginNamespace, pluginServiceAccount)
		stranger = asServiceAccount(pluginNamespace, strangerServiceAccount)
	})

	It("admits a Grant from a ServiceAccount with serve on the registration in its label", func() {
		grant := newGrant(grantLabels(sampleRegistration))

		Eventually(func() error {
			return plugin.Create(ctx, grant)
		}, envtest.Timeout, envtest.Interval).Should(Succeed())
		DeferCleanup(func() { _ = admin.Delete(ctx, grant) })
	})

	It("refuses a Grant from a ServiceAccount without serve, and names the requester and the registration", func() {
		Eventually(func() error {
			return stranger.Create(ctx, newGrant(grantLabels(sampleRegistration)))
		}, envtest.Timeout, envtest.Interval).Should(MatchError(SatisfyAll(
			ContainSubstring("system:serviceaccount:"+pluginNamespace+":"+strangerServiceAccount),
			ContainSubstring(messageNoServe+" "+sampleRegistration),
		)))
	})

	It("refuses a Grant for a registration that its writer does not serve", func() {
		Eventually(func() error {
			return plugin.Create(ctx, newGrant(grantLabels("aws-kms-key")))
		}, envtest.Timeout, envtest.Interval).Should(MatchError(ContainSubstring(messageNoServe + " aws-kms-key")))
	})

	It("refuses a Grant without the registration label", func() {
		Eventually(func() error {
			return plugin.Create(ctx, newGrant(nil))
		}, envtest.Timeout, envtest.Interval).Should(MatchError(ContainSubstring(messageMissingLabel)))
	})

	// An empty name would ask the authorizer for serve on every registration.
	It("refuses a Grant whose registration label is empty", func() {
		Eventually(func() error {
			return plugin.Create(ctx, newGrant(grantLabels("")))
		}, envtest.Timeout, envtest.Interval).Should(MatchError(ContainSubstring(messageMissingLabel)))
	})

	// The format rule of the policy acts only when the enum gets a format that
	// no identity extension supports.
	It("refuses a Grant with an unknown format through the CRD schema, and names the format", func() {
		grant := newGrant(grantLabels(sampleRegistration))
		grant.Spec.Format = "gcp-iam-binding"

		Eventually(func() error {
			return plugin.Create(ctx, grant)
		}, envtest.Timeout, envtest.Interval).Should(MatchError(ContainSubstring(`"gcp-iam-binding"`)))
	})
})
