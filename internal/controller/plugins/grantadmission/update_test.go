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
	"fmt"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	rbacv1 "k8s.io/api/rbac/v1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/uuid"
	"sigs.k8s.io/controller-runtime/pkg/client"

	pluginsv1alpha1 "github.com/konsole-is/camunda-operator/api/plugins/v1alpha1"
	v1 "github.com/konsole-is/camunda-operator/api/v1"
	"github.com/konsole-is/camunda-operator/test/envtest"
)

var _ = Describe("Grant admission on update", func() {
	var plugin, stranger client.Client
	var grant *pluginsv1alpha1.Grant

	BeforeEach(func() {
		plugin = asServiceAccount(pluginNamespace, pluginServiceAccount)
		stranger = asServiceAccount(pluginNamespace, strangerServiceAccount)

		grant = newGrant(grantLabels(sampleRegistration))
		grant.OwnerReferences = []metav1.OwnerReference{{
			APIVersion: pluginsv1alpha1.SchemeGroupVersion.String(),
			Kind:       "PluginRequest",
			Name:       "orders-backup-storage",
			UID:        uuid.NewUUID(),
		}}
		Eventually(func() error {
			return plugin.Create(ctx, grant)
		}, envtest.Timeout, envtest.Interval).Should(Succeed())
		DeferCleanup(func() { _ = admin.Delete(ctx, grant) })
	})

	It("admits a change of the fragment that keeps the label, by the writer that serves the registration", func() {
		grant.Spec.Fragment = &apiextensionsv1.JSON{
			Raw: []byte(`{"Effect":"Allow","Action":["s3:GetObject"],"Resource":["arn:aws:s3:::orders-backups/*"]}`),
		}

		Expect(plugin.Update(ctx, grant)).To(Succeed())
	})

	It("refuses a change of the fragment by a requester without serve", func() {
		grant.Spec.Fragment = &apiextensionsv1.JSON{
			Raw: []byte(`{"Effect":"Allow","Action":["s3:*"],"Resource":["*"]}`),
		}

		Eventually(func() error {
			return stranger.Update(ctx, grant)
		}, envtest.Timeout, envtest.Interval).Should(MatchError(ContainSubstring(messageNoServe + " " + sampleRegistration)))
	})

	It("refuses a move of the label to a registration that the writer does not serve", func() {
		grant.Labels[pluginsv1alpha1.LabelRegistration] = "aws-kms-key"

		Expect(plugin.Update(ctx, grant)).To(MatchError(ContainSubstring(messageNoServe + " aws-kms-key")))
	})

	It("refuses a move of the label of a Grant of another registration by the controller of the new one", func() {
		bindClusterRole(
			"aws-kms-key-controller",
			pluginNamespace,
			"aws-kms-key-controller",
			rbacv1.PolicyRule{
				APIGroups:     []string{pluginsv1alpha1.SchemeGroupVersion.Group},
				Resources:     []string{"pluginregistrations"},
				ResourceNames: []string{"aws-kms-key"},
				Verbs:         []string{"serve"},
			},
			rbacv1.PolicyRule{
				APIGroups: []string{pluginsv1alpha1.SchemeGroupVersion.Group},
				Resources: []string{"grants"},
				Verbs:     []string{"get", "update", "patch"},
			},
		)
		kms := asServiceAccount(pluginNamespace, "aws-kms-key-controller")

		grant.Labels[pluginsv1alpha1.LabelRegistration] = "aws-kms-key"

		Eventually(func() error {
			return kms.Update(ctx, grant)
		}, envtest.Timeout, envtest.Interval).Should(MatchError(ContainSubstring(messageNoServe + " " + sampleRegistration)))
	})

	// The garbage collector sends this merge patch when the owner of a Grant
	// is deleted with the Orphan propagation policy. It has no serve verb.
	It("lets the garbage collector remove the owner reference of an orphaned Grant", func() {
		gc := asServiceAccount("kube-system", "generic-garbage-collector")
		patch := fmt.Sprintf(`{"metadata":{"ownerReferences":[],"uid":%q}}`, grant.UID)

		Eventually(func() error {
			return gc.Patch(ctx, grant, client.RawPatch(types.MergePatchType, []byte(patch)))
		}, envtest.Timeout, envtest.Interval).Should(Succeed())

		Expect(admin.Get(ctx, client.ObjectKeyFromObject(grant), grant)).To(Succeed())
		Expect(grant.OwnerReferences).To(BeEmpty())
	})

	It("refuses a change of the fragment in a patch of the garbage collector", func() {
		gc := asServiceAccount("kube-system", "generic-garbage-collector")
		patch := `{"metadata":{"ownerReferences":[]},"spec":{"fragment":{"Effect":"Allow","Action":["s3:*"],"Resource":["*"]}}}`

		Eventually(func() error {
			return gc.Patch(ctx, grant, client.RawPatch(types.MergePatchType, []byte(patch)))
		}, envtest.Timeout, envtest.Interval).Should(MatchError(ContainSubstring(messageNoServe)))
	})

	It("refuses a removal of the registration label by a requester without serve", func() {
		gc := asServiceAccount("kube-system", "generic-garbage-collector")
		patch := `{"metadata":{"labels":{"plugins.camunda.io/registration":null}}}`

		Eventually(func() error {
			return gc.Patch(ctx, grant, client.RawPatch(types.MergePatchType, []byte(patch)))
		}, envtest.Timeout, envtest.Interval).Should(MatchError(ContainSubstring(messageMissingLabel)))
	})

	It("lets the identity extension write the status without serve", func() {
		bindClusterRole(
			"identity-extension",
			pluginNamespace,
			"identity-extension",
			rbacv1.PolicyRule{
				APIGroups: []string{pluginsv1alpha1.SchemeGroupVersion.Group},
				Resources: []string{"grants/status"},
				Verbs:     []string{"update", "patch"},
			},
		)
		extension := asServiceAccount(pluginNamespace, "identity-extension")

		meta.SetStatusCondition(&grant.Status.Conditions, metav1.Condition{
			Type:    pluginsv1alpha1.ConditionApplied,
			Status:  metav1.ConditionTrue,
			Reason:  v1.ReasonHealthy,
			Message: "The statement is part of the policy of the cluster identity",
		})

		Eventually(func() error {
			return extension.Status().Update(ctx, grant)
		}, envtest.Timeout, envtest.Interval).Should(Succeed())
	})
})
