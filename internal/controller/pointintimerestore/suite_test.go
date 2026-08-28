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

package pointintimerestore

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
	"github.com/konsole-is/camunda-operator/pkg/pgbootstrap"
)

// timeout and interval bound the Eventually polling of every envtest
// assertion.
const (
	timeout  = testenv.Timeout
	interval = testenv.Interval
)

// testClaimNamespace holds the claim Leases of this suite. In a cluster this
// is the namespace of the operator.
const testClaimNamespace = "default"

// retryInterval paces a hold in Pending that no watch resolves. It is short
// enough that a spec which waits for the timer finishes inside the test
// timeout, and long enough that watchWindow can tell a watch from the timer.
const retryInterval = 2 * time.Second

// watchWindow is shorter than retryInterval. A hold that ends inside it was
// ended by a watch, because the timer cannot have fired yet.
const watchWindow = 750 * time.Millisecond

var (
	env       *testenv.Env
	ctx       context.Context
	k8sClient client.Client
	// exporter answers the database-state check of every spec. The suite
	// needs no PostgreSQL.
	exporter = &exporterPositions{}
	// jobLogs answers the log read of every failed restore Job. envtest runs
	// no pod, so no real log exists.
	jobLogs = &restoreJobLogs{}
)

// answer is what the fake reader returns for one logical database.
type answer struct {
	positions []v1.PartitionPosition
	err       error
}

// exporterPositions is the fake exporter-position reader of the suite. Every
// world registers the answer for its own logical database, so the specs stay
// independent of each other and of their order.
type exporterPositions struct {
	mu      sync.Mutex
	answers map[string]answer
}

// set records what the reader answers for database.
func (e *exporterPositions) set(database string, a answer) {
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.answers == nil {
		e.answers = map[string]answer{}
	}
	e.answers[database] = a
}

func (e *exporterPositions) read(
	_ context.Context, _ pgbootstrap.Connection, database string,
) ([]v1.PartitionPosition, error) {
	e.mu.Lock()
	defer e.mu.Unlock()
	a := e.answers[database]

	return a.positions, a.err
}

// restoreJobLogs is the fake restore-Job log reader of the suite. A spec
// registers the log of the Job it wants read, and every other Job answers an
// empty log, which names no cause.
type restoreJobLogs struct {
	mu   sync.Mutex
	logs map[string]string
}

// set records the log that the reader answers for the Job named name.
func (l *restoreJobLogs) set(name, output string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.logs == nil {
		l.logs = map[string]string{}
	}
	l.logs[name] = output
}

func (l *restoreJobLogs) read(_ context.Context, job types.NamespacedName) (string, error) {
	l.mu.Lock()
	defer l.mu.Unlock()

	return l.logs[job.Name], nil
}

func TestPointInTimeRestoreController(t *testing.T) {
	RegisterFailHandler(Fail)

	RunSpecs(t, "PointInTimeRestore Controller Suite")
}

var _ = BeforeSuite(func() {
	logf.SetLogger(zap.New(zap.WriteTo(GinkgoWriter), zap.UseDevMode(true)))

	env = testenv.Start(func(mgr ctrl.Manager) error {
		return New(mgr.GetClient(), mgr.GetAPIReader(), mgr.GetScheme(), testClaimNamespace, Options{
			// Short, so a poll of the restore and a hold that no watch
			// resolves both fit inside the test timeout.
			PollInterval:  100 * time.Millisecond,
			RetryInterval: retryInterval,
			// Wide enough that a spec which arranges a broken dependency for
			// one look does not terminalize before it asserts the hold, and
			// short enough that the spec which waits out the grace fits in
			// the test timeout.
			MidRunGrace:   3 * time.Second,
			ReadPositions: exporter.read,
			ReadJobLog:    jobLogs.read,
		}).SetupWithManager(mgr)
	})

	ctx, k8sClient = env.Ctx, env.Client
})

var _ = AfterSuite(func() {
	By("tearing down the test environment")
	Eventually(env.Stop, time.Minute, time.Second).Should(Succeed())
})
