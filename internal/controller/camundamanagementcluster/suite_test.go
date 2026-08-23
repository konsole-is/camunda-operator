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

package camundamanagementcluster

import (
	"context"
	"testing"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	"github.com/konsole-is/camunda-operator/internal/controller/camundaplatformconfig"
	"github.com/konsole-is/camunda-operator/internal/controller/databaseconfig"
	"github.com/konsole-is/camunda-operator/internal/controller/managementauthconfig"
	"github.com/konsole-is/camunda-operator/internal/testenv"
)

// timeout and interval bound the Eventually polling of every envtest assertion.
const (
	timeout  = testenv.Timeout
	interval = testenv.Interval
)

var (
	env       *testenv.Env
	ctx       context.Context
	k8sClient client.Client
)

func TestCamundaManagementClusterController(t *testing.T) {
	RegisterFailHandler(Fail)

	RunSpecs(t, "CamundaManagementCluster Controller Suite")
}

var _ = BeforeSuite(func() {
	logf.SetLogger(zap.New(zap.WriteTo(GinkgoWriter), zap.UseDevMode(true)))

	// The two contract controllers are registered too: the Secret watch of
	// this controller lists platform configs and DatabaseConfigs through the
	// Secret indexes that those controllers own.
	env = testenv.Start(func(mgr ctrl.Manager) error {
		if err := (&camundaplatformconfig.CamundaPlatformConfigReconciler{
			Client:    mgr.GetClient(),
			APIReader: mgr.GetAPIReader(),
			Scheme:    mgr.GetScheme(),
		}).SetupWithManager(mgr); err != nil {
			return err
		}
		if err := (&databaseconfig.DatabaseConfigReconciler{
			Client:    mgr.GetClient(),
			APIReader: mgr.GetAPIReader(),
			Scheme:    mgr.GetScheme(),
		}).SetupWithManager(mgr); err != nil {
			return err
		}
		if err := (&managementauthconfig.ManagementAuthConfigReconciler{
			Client:    mgr.GetClient(),
			APIReader: mgr.GetAPIReader(),
			Scheme:    mgr.GetScheme(),
		}).SetupWithManager(mgr); err != nil {
			return err
		}

		r := New(mgr.GetClient(), mgr.GetAPIReader(), mgr.GetScheme())
		// A refused user API is called again soon, so that a spec can watch
		// the retry, and a cluster that holds the user is read again soon, so
		// that a spec can watch the repair.
		r.RetryInterval = time.Second
		r.ConvergeInterval = time.Second

		return r.SetupWithManager(mgr)
	})

	ctx, k8sClient = env.Ctx, env.Client
})

var _ = AfterSuite(func() {
	By("tearing down the test environment")
	Eventually(env.Stop, time.Minute, time.Second).Should(Succeed())
})
