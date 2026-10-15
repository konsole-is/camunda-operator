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

package pluginstartup_test

import (
	"testing"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	ctrl "sigs.k8s.io/controller-runtime"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	pluginsv1alpha1 "github.com/konsole-is/camunda-operator/api/plugins/v1alpha1"
	"github.com/konsole-is/camunda-operator/internal/pluginstartup"
	"github.com/konsole-is/camunda-operator/test/envtest"
)

// suiteVersion is the operator version of the writer that the manager of the
// suite starts. Every test build reports version.Dev, so the suite sets a
// release version.
const suiteVersion = "0.3.0"

var (
	env            *envtest.Env
	serviceAccount = pluginsv1alpha1.ServiceAccountRef{Namespace: "camunda-system", Name: "camunda-operator"}
)

func TestPluginStartup(t *testing.T) {
	RegisterFailHandler(Fail)

	RunSpecs(t, "Plugin Startup Suite")
}

var _ = BeforeSuite(func() {
	logf.SetLogger(zap.New(zap.WriteTo(GinkgoWriter), zap.UseDevMode(true)))

	env = envtest.Start(func(mgr ctrl.Manager) error {
		return mgr.Add(pluginstartup.New(mgr.GetClient(), suiteVersion, serviceAccount))
	})
})

var _ = AfterSuite(func() {
	Eventually(env.Stop, time.Minute, time.Second).Should(Succeed())
})
