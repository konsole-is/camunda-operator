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

package backupschedule

import (
	"context"
	"sync"
	"testing"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

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
	clock     *fakeClock
)

// fakeClock is the time source of the reconciler under test. The specs move
// it past a trigger instead of waiting for one.
type fakeClock struct {
	mu  sync.Mutex
	now time.Time
}

func (c *fakeClock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()

	return c.now
}

func (c *fakeClock) Set(t time.Time) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.now = t
}

func TestBackupScheduleController(t *testing.T) {
	RegisterFailHandler(Fail)

	RunSpecs(t, "BackupSchedule Controller Suite")
}

// The backup controllers are deliberately absent. A backup that the schedule
// creates stays in its zero phase, which is not terminal, and the specs move
// phases by hand. So the suite tests the schedule against the backup kinds'
// status contract, not against the controllers that fill it.
var _ = BeforeSuite(func() {
	logf.SetLogger(zap.New(zap.WriteTo(GinkgoWriter), zap.UseDevMode(true)))

	clock = &fakeClock{now: time.Now().UTC()}

	env = testenv.Start(func(mgr ctrl.Manager) error {
		reconciler := &BackupScheduleReconciler{
			Client:    mgr.GetClient(),
			APIReader: mgr.GetAPIReader(),
			Scheme:    mgr.GetScheme(),
		}

		return reconciler.SetupWithManager(mgr, Options{Now: clock.Now})
	})

	ctx, k8sClient = env.Ctx, env.Client
})

var _ = AfterSuite(func() {
	By("tearing down the test environment")
	Eventually(env.Stop, time.Minute, time.Second).Should(Succeed())
})
