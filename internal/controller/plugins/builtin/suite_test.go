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

package builtin

import (
	"bytes"
	"context"
	"io"
	"sync"
	"testing"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	"github.com/konsole-is/camunda-operator/internal/pluginstartup"
	"github.com/konsole-is/camunda-operator/test/envtest"
)

// timeout and interval bound the Eventually polling of every envtest assertion.
const (
	timeout  = envtest.Timeout
	interval = envtest.Interval
)

var (
	env       *envtest.Env
	ctx       context.Context
	k8sClient client.Client
	// logs holds every line that the manager of the suite logs, as JSON.
	logs = &syncBuffer{}
)

func TestBuiltin(t *testing.T) {
	RegisterFailHandler(Fail)

	RunSpecs(t, "Built-in Adapter Suite")
}

var _ = BeforeSuite(func() {
	logf.SetLogger(zap.New(zap.WriteTo(io.MultiWriter(GinkgoWriter, logs)), zap.UseDevMode(true), zap.JSONEncoder()))

	env = envtest.StartWith(
		envtest.Options{WithoutKeycloak: true, WithoutCNPG: true},
		func(mgr ctrl.Manager) error {
			return (&Reconciler{
				Client:        mgr.GetClient(),
				APIReader:     mgr.GetAPIReader(),
				Scheme:        mgr.GetScheme(),
				Registrations: pluginstartup.Registrations("0.3.0", testServiceAccount),
			}).SetupWithManager(mgr)
		},
	)

	ctx, k8sClient = env.Ctx, env.Client

	// The controller of the built-in registrations does not run in this
	// suite, so the suite writes each registration Ready.
	for _, reg := range pluginstartup.Registrations("0.3.0", testServiceAccount) {
		Expect(k8sClient.Create(ctx, &reg)).To(Succeed())
		setRegistrationReady(reg.Name, metav1.ConditionTrue)
	}
})

var _ = AfterSuite(func() {
	By("tearing down the test environment")
	Eventually(env.Stop, time.Minute, time.Second).Should(Succeed())
})

// syncBuffer is a buffer that the logger and the specs share.
type syncBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (b *syncBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Write(p)
}

func (b *syncBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.String()
}
