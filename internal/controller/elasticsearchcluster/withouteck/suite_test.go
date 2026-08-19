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

// Package withouteck runs the ElasticsearchCluster controller against a
// control plane that does not serve the ECK CRDs. It holds its own suite
// because the CRD set is fixed for the life of a control plane.
package withouteck

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
	"github.com/konsole-is/camunda-operator/internal/controller/elasticsearchcluster"
	"github.com/konsole-is/camunda-operator/internal/testenv"
)

var (
	env       *testenv.Env
	ctx       context.Context
	k8sClient client.Client
)

func TestControllers(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "ElasticsearchCluster Controller Without ECK Suite")
}

var _ = BeforeSuite(func() {
	logf.SetLogger(zap.New(zap.WriteTo(GinkgoWriter), zap.UseDevMode(true)))

	env = testenv.StartWith(testenv.Options{WithoutECK: true}, func(mgr ctrl.Manager) error {
		return (&elasticsearchcluster.ElasticsearchClusterReconciler{
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

var _ = Describe("ElasticsearchCluster without the ECK CRDs", func() {
	It("starts the manager and reports ECKNotInstalled on the cluster", func() {
		cluster := &v1.ElasticsearchCluster{
			ObjectMeta: metav1.ObjectMeta{Name: "es", Namespace: "default"},
			Spec: v1.ElasticsearchClusterSpec{
				Version:                "9.2.0",
				Replicas:               new(int32(1)),
				StorageSize:            new(resource.MustParse("1Gi")),
				SecondaryStorageConfig: "es-storage",
			},
		}
		Expect(k8sClient.Create(ctx, cluster)).To(Succeed())

		Eventually(func(g Gomega) {
			var latest v1.ElasticsearchCluster
			g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(cluster), &latest)).To(Succeed())
			ready := meta.FindStatusCondition(latest.Status.Conditions, v1.ConditionReady)
			g.Expect(ready).NotTo(BeNil())
			g.Expect(ready.Status).To(Equal(metav1.ConditionFalse))
			g.Expect(ready.Reason).To(Equal(v1.ReasonECKNotInstalled))
			g.Expect(latest.Status.ObservedGeneration).To(Equal(latest.Generation))
		}, testenv.Timeout, testenv.Interval).Should(Succeed())

		var storage v1.SecondaryStorageConfig
		err := k8sClient.Get(ctx, client.ObjectKey{Namespace: "default", Name: "es-storage"}, &storage)
		Expect(err).To(HaveOccurred(), "no contract is published without ECK")
	})
})
