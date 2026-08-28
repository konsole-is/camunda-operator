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

package camundamanagementcluster

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	v1 "github.com/konsole-is/camunda-operator/api/v1"
	"github.com/konsole-is/camunda-operator/internal/controller/camundaplatformconfig"
	"github.com/konsole-is/camunda-operator/internal/controller/databaseconfig"
	"github.com/konsole-is/camunda-operator/internal/controller/managementauthconfig"
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
	// clusterListFault sits in front of the APIReader of the reconciler that
	// the manager runs, so a spec can refuse the CamundaCluster list of a
	// reconcile step and read what the controller reports.
	clusterListFault = &listFault{}
)

// errClusterListRefused is what the armed fault answers.
var errClusterListRefused = errors.New("the fault reader refused the CamundaCluster list")

// listFault is a client.Reader that refuses every CamundaCluster list while it
// is armed and passes every other call through.
type listFault struct {
	client.Reader
	armed atomic.Bool
}

// wrap puts the fault in front of reader and returns it.
func (f *listFault) wrap(reader client.Reader) client.Reader {
	f.Reader = reader

	return f
}

// arm refuses every CamundaCluster list until the returned function runs. The
// caller defers that function, so a failed assertion cannot leave the rest of
// the suite with a refused list.
func (f *listFault) arm() func() {
	f.armed.Store(true)

	return func() { f.armed.Store(false) }
}

// List refuses a CamundaCluster list while the fault is armed.
func (f *listFault) List(ctx context.Context, list client.ObjectList, opts ...client.ListOption) error {
	if _, ok := list.(*v1.CamundaClusterList); ok && f.armed.Load() {
		return errClusterListRefused
	}

	return f.Reader.List(ctx, list, opts...)
}

func TestCamundaManagementClusterController(t *testing.T) {
	RegisterFailHandler(Fail)

	RunSpecs(t, "CamundaManagementCluster Controller Suite")
}

var _ = BeforeSuite(func() {
	logf.SetLogger(zap.New(zap.WriteTo(GinkgoWriter), zap.UseDevMode(true)))

	// The two contract controllers are registered too: the Secret watch of
	// this controller lists platform configs and DatabaseConfigs through the
	// Secret indexes that those controllers own.
	env = testenv.Start(func(mgr ctrl.Manager) error {
		if err := (&camundaplatformconfig.CamundaPlatformConfigReconciler{
			Client:    mgr.GetClient(),
			APIReader: mgr.GetAPIReader(),
			Scheme:    mgr.GetScheme(),
		}).SetupWithManager(mgr); err != nil {
			return err
		}
		if err := (&databaseconfig.DatabaseConfigReconciler{
			Client:    mgr.GetClient(),
			APIReader: mgr.GetAPIReader(),
			Scheme:    mgr.GetScheme(),
		}).SetupWithManager(mgr); err != nil {
			return err
		}
		if err := (&managementauthconfig.ManagementAuthConfigReconciler{
			Client:    mgr.GetClient(),
			APIReader: mgr.GetAPIReader(),
			Scheme:    mgr.GetScheme(),
		}).SetupWithManager(mgr); err != nil {
			return err
		}

		r := New(mgr.GetClient(), clusterListFault.wrap(mgr.GetAPIReader()), mgr.GetScheme())
		// A refused user API is called again soon, so that a spec can watch
		// the retry, and a cluster that holds the user is read again soon, so
		// that a spec can watch the repair.
		r.RetryInterval = time.Second
		r.ConvergeInterval = time.Second

		return r.SetupWithManager(mgr)
	})

	ctx, k8sClient = env.Ctx, env.Client
})

var _ = AfterSuite(func() {
	By("tearing down the test environment")
	Eventually(env.Stop, time.Minute, time.Second).Should(Succeed())
})
