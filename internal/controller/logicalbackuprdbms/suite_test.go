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

package logicalbackuprdbms

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

	v1 "github.com/konsole-is/camunda-operator/api/v1"
	"github.com/konsole-is/camunda-operator/internal/testenv"
	"github.com/konsole-is/camunda-operator/pkg/camundaadmin/camundaadmintest"
	"github.com/konsole-is/camunda-operator/pkg/logicalbackup"
	"github.com/konsole-is/camunda-operator/pkg/objectstore"
)

// timeout and interval bound the Eventually polling of every envtest assertion.
const (
	timeout  = testenv.Timeout
	interval = testenv.Interval
)

var (
	env        *testenv.Env
	ctx        context.Context
	k8sClient  client.Client
	management *camundaadmintest.Server
	bucket     *fakeBucket
	reconciler *LogicalBackupRDBMSReconciler
)

// sibling is the mutex-guarded seam behind reconciler.SiblingInProgress: the
// manager goroutine reads it while specs swap it, so a bare field write would
// race.
var (
	siblingMu sync.Mutex
	sibling   logicalbackup.SiblingInProgress
)

// setSibling swaps the sibling seam for one spec; call it again with nil to
// restore.
func setSibling(fn logicalbackup.SiblingInProgress) {
	siblingMu.Lock()
	defer siblingMu.Unlock()
	sibling = fn
}

// fakeBucket records the deletes of the finalizer, standing in for the
// backup bucket.
type fakeBucket struct {
	mu      sync.Mutex
	deleted []string
}

func (b *fakeBucket) Delete(_ context.Context, key string) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.deleted = append(b.deleted, key)

	return nil
}

func (b *fakeBucket) Close() {}

func (b *fakeBucket) Deleted() []string {
	b.mu.Lock()
	defer b.mu.Unlock()

	return append([]string(nil), b.deleted...)
}

func TestLogicalBackupRDBMSController(t *testing.T) {
	RegisterFailHandler(Fail)

	RunSpecs(t, "LogicalBackupRDBMS Controller Suite")
}

// The CamundaCluster controller is deliberately absent: the specs publish the
// management binding and the credential copies themselves, so the backup
// controller is tested against the contract, not the producer.
var _ = BeforeSuite(func() {
	logf.SetLogger(zap.New(zap.WriteTo(GinkgoWriter), zap.UseDevMode(true)))

	management = camundaadmintest.New()
	bucket = &fakeBucket{}

	env = testenv.Start(func(mgr ctrl.Manager) error {
		reconciler = &LogicalBackupRDBMSReconciler{
			Client:        mgr.GetClient(),
			APIReader:     mgr.GetAPIReader(),
			Scheme:        mgr.GetScheme(),
			OperatorImage: "ghcr.io/konsole-is/camunda-operator:test",
			RetryInterval: time.Second,
			// Small graces, so the specs that outwait them stay fast. The
			// windows the specs assert within must stay well inside these.
			MidRunGrace:       3 * time.Second,
			RegistrationGrace: 4 * time.Second,
			OpenBucket: func(
				context.Context, *v1.ObjectStorageConfig, *objectstore.Credentials,
			) (ArtifactBucket, error) {
				return bucket, nil
			},
			SiblingInProgress: func(ctx context.Context, cluster types.NamespacedName) (string, error) {
				siblingMu.Lock()
				fn := sibling
				siblingMu.Unlock()
				if fn == nil {
					return "", nil
				}

				return fn(ctx, cluster)
			},
		}

		return reconciler.SetupWithManager(mgr)
	})

	ctx, k8sClient = env.Ctx, env.Client
})

var _ = AfterSuite(func() {
	By("tearing down the test environment")
	management.Close()
	Eventually(env.Stop, time.Minute, time.Second).Should(Succeed())
})
