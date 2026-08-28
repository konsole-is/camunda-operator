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
	"errors"
	"sync"
	"testing"
	"time"

	cnpgv1 "github.com/cloudnative-pg/api/pkg/api/v1"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	"github.com/konsole-is/camunda-operator/internal/testenv"
	"github.com/konsole-is/camunda-operator/pkg/wrappers/barmanobjectstore"
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

	// objectStoreApplies is the client the components apply through, with a
	// switch that refuses the ObjectStore of one namespace.
	objectStoreApplies *objectStoreApplyBlocker
)

// objectStoreApplyBlocker refuses the apply of a Barman Cloud ObjectStore in
// the namespaces a spec named, and passes every other write on. It is how a
// spec shows what the server records while the archive is still in the place
// it was: a rejected apply leaves the ObjectStore describing the bucket the
// server came from.
type objectStoreApplyBlocker struct {
	client.Client

	mu       sync.Mutex
	refusing map[string]bool
}

// Patch refuses the ObjectStore of a namespace the spec named.
func (c *objectStoreApplyBlocker) Patch(
	ctx context.Context,
	obj client.Object,
	patch client.Patch,
	opts ...client.PatchOption,
) error {
	if _, ok := obj.(*barmanobjectstore.ObjectStore); ok && c.refuses(obj.GetNamespace()) {
		return apierrors.NewInternalError(errors.New("the spec refuses this ObjectStore apply"))
	}

	return c.Client.Patch(ctx, obj, patch, opts...)
}

// refuse makes every later ObjectStore apply in namespace fail.
func (c *objectStoreApplyBlocker) refuse(namespace string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.refusing[namespace] = true
}

// allow lets the ObjectStore applies of namespace through again.
func (c *objectStoreApplyBlocker) allow(namespace string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	delete(c.refusing, namespace)
}

// refuses reports whether the ObjectStore applies of namespace fail.
func (c *objectStoreApplyBlocker) refuses(namespace string) bool {
	c.mu.Lock()
	defer c.mu.Unlock()

	return c.refusing[namespace]
}

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

		// Uncached, as SetupWithManager builds it: see the componentClient
		// field. The spec keeps a handle on it to refuse one apply.
		applies, err := client.New(mgr.GetConfig(), client.Options{
			Scheme: mgr.GetScheme(),
			Mapper: mgr.GetRESTMapper(),
		})
		if err != nil {
			return err
		}
		objectStoreApplies = &objectStoreApplyBlocker{Client: applies, refusing: map[string]bool{}}

		return (&DatabaseServerReconciler{
			Client:          backupLists,
			APIReader:       mgr.GetAPIReader(),
			Scheme:          mgr.GetScheme(),
			componentClient: objectStoreApplies,
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
