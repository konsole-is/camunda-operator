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

package reconcilers_test

import (
	"testing"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	ctrl "sigs.k8s.io/controller-runtime"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	pluginsv1alpha1 "github.com/konsole-is/camunda-operator/api/plugins/v1alpha1"
	"github.com/konsole-is/camunda-operator/internal/reconcilers"
	"github.com/konsole-is/camunda-operator/pkg/grace"
	"github.com/konsole-is/camunda-operator/test/envtest"
)

var env *envtest.Env

// suiteVersion and suiteServiceAccount are the options of the startup
// writer that Setup adds in this suite.
const suiteVersion = "0.3.0"

var suiteServiceAccount = pluginsv1alpha1.ServiceAccountRef{Namespace: "default", Name: "camunda-operator"}

func TestReconcilers(t *testing.T) {
	RegisterFailHandler(Fail)

	RunSpecs(t, "Reconcilers Suite")
}

var _ = BeforeSuite(func() {
	logf.SetLogger(zap.New(zap.WriteTo(GinkgoWriter), zap.UseDevMode(true)))

	env = envtest.Start(func(mgr ctrl.Manager) error {
		return reconcilers.Setup(mgr, reconcilers.Options{
			Namespace:       "default",
			CLIImage:        "ghcr.io/konsole-is/camunda-operator-cli:test",
			GracePeriods:    grace.Periods{Workload: time.Hour, Datastore: time.Hour},
			OperatorVersion: suiteVersion,
			ServiceAccount:  suiteServiceAccount,
		})
	})
})

var _ = AfterSuite(func() {
	Eventually(env.Stop, time.Minute, time.Second).Should(Succeed())
})
