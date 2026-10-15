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

package pluginregistration

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

	"github.com/konsole-is/camunda-operator/test/envtest"
)

// operatorVersion is the release version that the controller of this suite
// runs as. The prerelease suite runs a prerelease.
const operatorVersion = "0.3.0"

// timeout and interval bound the Eventually polling of every envtest assertion.
const (
	timeout  = envtest.Timeout
	interval = envtest.Interval
)

var (
	env       *envtest.Env
	ctx       context.Context
	k8sClient client.Client
	// managerClient is the cached client of the manager. A spec that runs a
	// second reconciler uses it, because the SectionsField index exists only
	// in the cache.
	managerClient client.Client
)

func TestPluginRegistrationController(t *testing.T) {
	RegisterFailHandler(Fail)

	RunSpecs(t, "PluginRegistration Controller Suite")
}

var _ = BeforeSuite(func() {
	logf.SetLogger(zap.New(zap.WriteTo(GinkgoWriter), zap.UseDevMode(true)))

	env = envtest.Start(func(mgr ctrl.Manager) error {
		managerClient = mgr.GetClient()

		return (&Reconciler{
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
