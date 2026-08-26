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

// Package withoutbarman runs the DatabaseServer controller against a control
// plane that serves CloudNativePG without the Barman Cloud plugin, the cluster
// of a user who installed one and not the other. It holds its own suite
// because the CRD set is fixed for the life of a control plane.
package withoutbarman

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
	RunSpecs(t, "DatabaseServer Controller Without The Barman Cloud Plugin Suite")
}

var _ = BeforeSuite(func() {
	logf.SetLogger(zap.New(zap.WriteTo(GinkgoWriter), zap.UseDevMode(true)))

	env = testenv.StartWith(testenv.Options{WithoutBarmanPlugin: true}, func(mgr ctrl.Manager) error {
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

// server returns a DatabaseServer with the given archive block.
func server(name string, archive *v1.DatabaseServerArchiveSpec) *v1.DatabaseServer {
	return &v1.DatabaseServer{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: "default"},
		Spec: v1.DatabaseServerSpec{
			Version:              "17",
			Instances:            new(int32(1)),
			StorageSize:          new(resource.MustParse("1Gi")),
			DatabaseServerConfig: name,
			Archive:              archive,
		},
	}
}

var _ = Describe("DatabaseServer without the Barman Cloud plugin", func() {
	It("reports BarmanPluginNotInstalled on a server that asks for an archive", func() {
		archived := server("archived", &v1.DatabaseServerArchiveSpec{
			ObjectStorageRef:    "backups",
			RetentionPeriodDays: 30,
		})
		Expect(k8sClient.Create(ctx, archived)).To(Succeed())

		Eventually(func(g Gomega) {
			var latest v1.DatabaseServer
			g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(archived), &latest)).To(Succeed())
			ready := meta.FindStatusCondition(latest.Status.Conditions, v1.ConditionReady)
			g.Expect(ready).NotTo(BeNil())
			g.Expect(ready.Status).To(Equal(metav1.ConditionFalse))
			g.Expect(ready.Reason).To(Equal(v1.ReasonBarmanPluginNotInstalled))
			g.Expect(ready.Message).To(ContainSubstring("restart the operator"))
		}, testenv.Timeout, testenv.Interval).Should(Succeed())
	})

	It("runs a server that asks for no archive", func() {
		plain := server("plain", nil)
		Expect(k8sClient.Create(ctx, plain)).To(Succeed())

		Eventually(func(g Gomega) {
			var cluster v1.DatabaseServer
			g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(plain), &cluster)).To(Succeed())
			ready := meta.FindStatusCondition(cluster.Status.Conditions, v1.ConditionReady)
			g.Expect(ready).NotTo(BeNil())
			g.Expect(ready.Reason).NotTo(Equal(v1.ReasonBarmanPluginNotInstalled))
		}, testenv.Timeout, testenv.Interval).Should(Succeed())
	})
})
