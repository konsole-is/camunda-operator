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

package databaseserver

import (
	"context"
	"sync"
	"testing"
	"time"

	cnpgv1 "github.com/cloudnative-pg/api/pkg/api/v1"
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

	// backupLists counts the base backup reads of the reconciler, so a spec
	// can show that a server without an archive pays for none.
	backupLists *backupListCounter
)

// backupListCounter is the client of the manager with a counter over the
// CloudNativePG backup reads, keyed by the namespace each read is scoped to.
// Every spec runs in a namespace of its own, so the counts never cross.
type backupListCounter struct {
	client.Client

	mu     sync.Mutex
	counts map[string]int
}

// List counts a read of the CloudNativePG backups and passes every read on.
func (c *backupListCounter) List(ctx context.Context, list client.ObjectList, opts ...client.ListOption) error {
	if _, ok := list.(*cnpgv1.BackupList); ok {
		options := &client.ListOptions{}
		for _, opt := range opts {
			opt.ApplyToList(options)
		}
		c.mu.Lock()
		c.counts[options.Namespace]++
		c.mu.Unlock()
	}

	return c.Client.List(ctx, list, opts...)
}

// countIn returns how often the reconciler has read the backups of namespace.
func (c *backupListCounter) countIn(namespace string) int {
	c.mu.Lock()
	defer c.mu.Unlock()

	return c.counts[namespace]
}

func TestDatabaseServerController(t *testing.T) {
	RegisterFailHandler(Fail)

	RunSpecs(t, "DatabaseServer Controller Suite")
}

var _ = BeforeSuite(func() {
	logf.SetLogger(zap.New(zap.WriteTo(GinkgoWriter), zap.UseDevMode(true)))

	env = testenv.Start(func(mgr ctrl.Manager) error {
		backupLists = &backupListCounter{Client: mgr.GetClient(), counts: map[string]int{}}

		return (&DatabaseServerReconciler{
			Client:    backupLists,
			APIReader: mgr.GetAPIReader(),
			Scheme:    mgr.GetScheme(),
			// Short, so the specs exercise the requeue that waits on the
			// superuser Secret inside their timeout.
			RetryInterval: 500 * time.Millisecond,
		}).SetupWithManager(mgr)
	})

	ctx, k8sClient = env.Ctx, env.Client
})

var _ = AfterSuite(func() {
	By("tearing down the test environment")
	Eventually(env.Stop, time.Minute, time.Second).Should(Succeed())
})
