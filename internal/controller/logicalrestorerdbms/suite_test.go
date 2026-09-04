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

package logicalrestorerdbms

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

	"github.com/konsole-is/camunda-operator/internal/testenv"
)

// timeout and interval bound the Eventually polling of every envtest
// assertion.
const (
	timeout  = testenv.Timeout
	interval = testenv.Interval
)

// cliImage is the camunda-operator-cli image that the suite hands the
// reconciler. The pg_restore Job renders it.
const cliImage = "ghcr.io/konsole-is/camunda-operator-cli:test"

// midRunGrace is how long a started restore of the suite waits on a
// dependency that stopped resolving. It is short, so the specs that prove the
// grace runs out finish inside the test timeout.
const midRunGrace = 3 * time.Second

// retryInterval paces a hold in Pending that no watch resolves. It stays under
// the test timeout, so a spec that waits for the timer still finishes. It stays
// above watchWindow, so watchWindow tells a watch from the timer.
const retryInterval = 5 * time.Second

// watchWindow is the budget of a wake that a watch delivers. The chain is the
// watch event, the enqueue, one reconcile, and the status write. The queue of
// the restores that the earlier specs left running sits in front of it. The
// window stays under retryInterval, so only a watch ends a hold inside it.
const watchWindow = 2 * time.Second

var (
	env       *testenv.Env
	ctx       context.Context
	k8sClient client.Client
)

func TestLogicalRestoreRDBMSController(t *testing.T) {
	RegisterFailHandler(Fail)

	RunSpecs(t, "LogicalRestoreRDBMS Controller Suite")
}

var _ = BeforeSuite(func() {
	logf.SetLogger(zap.New(zap.WriteTo(GinkgoWriter), zap.UseDevMode(true)))

	env = testenv.Start(func(mgr ctrl.Manager) error {
		return New(mgr.GetClient(), mgr.GetAPIReader(), mgr.GetScheme(), Options{
			CLIImage:      cliImage,
			PollInterval:  100 * time.Millisecond,
			RetryInterval: retryInterval,
			MidRunGrace:   midRunGrace,
		}).SetupWithManager(mgr)
	})

	ctx, k8sClient = env.Ctx, env.Client
})

var _ = AfterSuite(func() {
	By("tearing down the test environment")
	Eventually(env.Stop, time.Minute, time.Second).Should(Succeed())
})
