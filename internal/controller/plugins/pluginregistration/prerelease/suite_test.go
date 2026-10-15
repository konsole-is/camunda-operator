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

// Package prerelease runs the PluginRegistration controller as a prerelease
// build of the operator. It holds its own suite because the operator version
// is fixed for the life of a manager.
package prerelease

import (
	"context"
	"testing"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	pluginsv1alpha1 "github.com/konsole-is/camunda-operator/api/plugins/v1alpha1"
	"github.com/konsole-is/camunda-operator/internal/controller/plugins/pluginregistration"
	"github.com/konsole-is/camunda-operator/test/envtest"
)

// operatorVersion is a prerelease of the release that the range below holds.
const operatorVersion = "0.3.0-rc.1"

var (
	env       *envtest.Env
	ctx       context.Context
	k8sClient client.Client
)

func TestPrereleaseOperator(t *testing.T) {
	RegisterFailHandler(Fail)

	RunSpecs(t, "PluginRegistration Controller Prerelease Suite")
}

var _ = BeforeSuite(func() {
	logf.SetLogger(zap.New(zap.WriteTo(GinkgoWriter), zap.UseDevMode(true)))

	env = envtest.Start(func(mgr ctrl.Manager) error {
		return (&pluginregistration.Reconciler{
			Client:          mgr.GetClient(),
			APIReader:       mgr.GetAPIReader(),
			Scheme:          mgr.GetScheme(),
			OperatorVersion: operatorVersion,
		}).SetupWithManager(mgr)
	})

	ctx, k8sClient = env.Ctx, env.Client
})

var _ = AfterSuite(func() {
	By("tearing down the test environment")
	Eventually(env.Stop, time.Minute, time.Second).Should(Succeed())
})

var _ = Describe("PluginRegistration on a prerelease operator", func() {
	It("reports OperatorVersionUnsupported for a plain range that holds the release", func() {
		section := &pluginsv1alpha1.PluginSection{
			ObjectMeta: metav1.ObjectMeta{Name: "backup-storage"},
			Spec: pluginsv1alpha1.PluginSectionSpec{
				Label:     "Backup storage",
				Selection: pluginsv1alpha1.SelectionOne,
				AppliesTo: []pluginsv1alpha1.ClaimKind{pluginsv1alpha1.ClaimKindCamundaClusterClaim},
			},
		}
		Expect(k8sClient.Create(ctx, section)).To(Succeed())

		reg := &pluginsv1alpha1.PluginRegistration{
			ObjectMeta: metav1.ObjectMeta{Name: "s3-backup-bucket"},
			Spec: pluginsv1alpha1.PluginRegistrationSpec{
				Sections: []string{section.Name},
				Label:    "New S3 bucket",
				Mode:     pluginsv1alpha1.ModeOptional,
				ServiceAccount: pluginsv1alpha1.ServiceAccountRef{
					Namespace: "camunda-plugins",
					Name:      "s3-controller",
				},
				OperatorVersions: ">=0.3.0 <0.4.0",
				DeletionPolicies: []pluginsv1alpha1.DeletionPolicy{pluginsv1alpha1.DeletionPolicyDelete},
			},
		}
		Expect(k8sClient.Create(ctx, reg)).To(Succeed())

		Eventually(func(g Gomega) {
			var latest pluginsv1alpha1.PluginRegistration
			g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(reg), &latest)).To(Succeed())
			cond := meta.FindStatusCondition(latest.Status.Conditions, pluginsv1alpha1.ConditionAccepted)
			g.Expect(cond).NotTo(BeNil())
			g.Expect(cond.Status).To(Equal(metav1.ConditionFalse))
			g.Expect(cond.Reason).To(Equal(pluginsv1alpha1.ReasonOperatorVersionUnsupported))
			g.Expect(cond.Message).To(ContainSubstring(operatorVersion))
			g.Expect(cond.Message).To(ContainSubstring(">=0.3.0 <0.4.0"))
		}, envtest.Timeout, envtest.Interval).Should(Succeed())
	})
})
