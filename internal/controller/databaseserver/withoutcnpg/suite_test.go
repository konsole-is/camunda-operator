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

// Package withoutcnpg runs the DatabaseServer controller against a control
// plane that does not serve the CloudNativePG CRDs. It holds its own suite
// because the CRD set is fixed for the life of a control plane.
package withoutcnpg

import (
	"context"
	"testing"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	v1 "github.com/konsole-is/camunda-operator/api/v1"
	"github.com/konsole-is/camunda-operator/internal/controller/databaseserver"
	"github.com/konsole-is/camunda-operator/internal/testenv"
)

var (
	env       *testenv.Env
	ctx       context.Context
	k8sClient client.Client
)

func TestControllers(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "DatabaseServer Controller Without CloudNativePG Suite")
}

var _ = BeforeSuite(func() {
	logf.SetLogger(zap.New(zap.WriteTo(GinkgoWriter), zap.UseDevMode(true)))

	env = testenv.StartWith(testenv.Options{WithoutCNPG: true}, func(mgr ctrl.Manager) error {
		return (&databaseserver.DatabaseServerReconciler{
			Client:    mgr.GetClient(),
			APIReader: mgr.GetAPIReader(),
			Scheme:    mgr.GetScheme(),
		}).SetupWithManager(mgr)
	})

	ctx, k8sClient = env.Ctx, env.Client
})

var _ = AfterSuite(func() {
	By("tearing down the test environment")
	Eventually(env.Stop, time.Minute, time.Second).Should(Succeed())
})

var _ = Describe("DatabaseServer without the CloudNativePG CRDs", func() {
	It("starts the manager and reports CNPGNotInstalled on the server", func() {
		server := &v1.DatabaseServer{
			ObjectMeta: metav1.ObjectMeta{Name: "camunda", Namespace: "default"},
			Spec: v1.DatabaseServerSpec{
				Version:              "17",
				Instances:            new(int32(1)),
				StorageSize:          new(resource.MustParse("1Gi")),
				DatabaseServerConfig: "camunda",
			},
		}
		Expect(k8sClient.Create(ctx, server)).To(Succeed())

		Eventually(func(g Gomega) {
			var latest v1.DatabaseServer
			g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(server), &latest)).To(Succeed())
			ready := meta.FindStatusCondition(latest.Status.Conditions, v1.ConditionReady)
			g.Expect(ready).NotTo(BeNil())
			g.Expect(ready.Status).To(Equal(metav1.ConditionFalse))
			g.Expect(ready.Reason).To(Equal(v1.ReasonCNPGNotInstalled))
			g.Expect(ready.Message).To(ContainSubstring("restart the operator"))
			g.Expect(latest.Status.ObservedGeneration).To(Equal(latest.Generation))
		}, testenv.Timeout, testenv.Interval).Should(Succeed())

		var contract v1.DatabaseServerConfig
		err := k8sClient.Get(ctx, client.ObjectKey{Namespace: "default", Name: "camunda"}, &contract)
		Expect(err).To(HaveOccurred(), "no contract is published without CloudNativePG")
	})
})
