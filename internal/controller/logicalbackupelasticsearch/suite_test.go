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

package logicalbackupelasticsearch

import (
	"context"
	"sync"
	"testing"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	"github.com/konsole-is/camunda-operator/internal/testenv"
	"github.com/konsole-is/camunda-operator/pkg/refindex"
)

// timeout and interval bound the Eventually polling of every envtest
// assertion.
const (
	timeout  = testenv.Timeout
	interval = testenv.Interval
)

var (
	env       *testenv.Env
	ctx       context.Context
	k8sClient client.Client

	// siblings fakes the other backup kind. A test marks a cluster as held
	// by a STARTED sibling backup, and the pre-checks must wait on it. The
	// contract of SiblingInProgress reports only started siblings; a pending
	// sibling never blocks. Keyed by namespace/name of the cluster.
	siblings sync.Map
)

func TestLogicalBackupElasticsearchController(t *testing.T) {
	RegisterFailHandler(Fail)

	RunSpecs(t, "LogicalBackupElasticsearch Controller Suite")
}

var _ = BeforeSuite(func() {
	logf.SetLogger(zap.New(zap.WriteTo(GinkgoWriter), zap.UseDevMode(true)))

	env = testenv.Start(func(mgr ctrl.Manager) error {
		return New(mgr.GetClient(), mgr.GetAPIReader(), mgr.GetScheme(), Options{
			// Short, so the polling of a running procedure and the resume
			// deadline both fit inside the test timeout. The deadline fires
			// only when resume keeps failing, which only the deadline test
			// arranges.
			PollInterval:   100 * time.Millisecond,
			ResumeDeadline: 2 * time.Second,
			SiblingInProgress: func(_ context.Context, cluster types.NamespacedName) (string, error) {
				if held, ok := siblings.Load(refindex.NamespacedKey(cluster.Namespace, cluster.Name)); ok {
					return held.(string), nil
				}
				return "", nil
			},
		}).SetupWithManager(mgr)
	})

	ctx, k8sClient = env.Ctx, env.Client
})

var _ = AfterSuite(func() {
	By("tearing down the test environment")
	Eventually(env.Stop, time.Minute, time.Second).Should(Succeed())
})
