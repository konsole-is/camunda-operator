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

package camundacluster

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

	v1 "github.com/konsole-is/camunda-operator/api/v1"
	"github.com/konsole-is/camunda-operator/internal/controller/camundaplatformconfig"
	"github.com/konsole-is/camunda-operator/internal/controller/databaseconfig"
	"github.com/konsole-is/camunda-operator/internal/controller/secondarystorageconfig"
	"github.com/konsole-is/camunda-operator/internal/testenv"
	components "github.com/konsole-is/camunda-operator/pkg/components/camundacluster"
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
)

// userAPIEndpoints maps the namespace of a cluster to the user API that
// serves it. The reconciler is process wide, so one fake for the whole suite
// would let a reconcile of an earlier cluster count against the fake of a
// later spec. createCluster waits for its cluster to go, which closes that
// window for everything but a reconcile already in flight; keying the
// endpoint by namespace closes it for that one too. A namespace with no
// entry has no user API, which is what a spec wants while it drives a
// rotation that cannot reach the cluster.
var userAPIEndpoints sync.Map

// unreachableUserAPI is the address of a cluster that answers nothing.
const unreachableUserAPI = "http://127.0.0.1:1"

// serveUserAPI points the clusters of namespace at endpoint.
func serveUserAPI(namespace, endpoint string) {
	userAPIEndpoints.Store(namespace, endpoint)
}

func init() {
}

func TestCamundaClusterController(t *testing.T) {
	RegisterFailHandler(Fail)

	RunSpecs(t, "CamundaCluster Controller Suite")
}

var _ = BeforeSuite(func() {
	logf.SetLogger(zap.New(zap.WriteTo(GinkgoWriter), zap.UseDevMode(true)))

	// The contract controllers are registered too: the Secret watch of the
	// cluster controller lists platform configs, bindings, and DatabaseConfigs
	// through their indexes.
	env = testenv.Start(func(mgr ctrl.Manager) error {
		if err := (&camundaplatformconfig.CamundaPlatformConfigReconciler{
			Client:    mgr.GetClient(),
			APIReader: mgr.GetAPIReader(),
			Scheme:    mgr.GetScheme(),
		}).SetupWithManager(mgr); err != nil {
			return err
		}
		if err := (&secondarystorageconfig.SecondaryStorageConfigReconciler{
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

		return (&CamundaClusterReconciler{
			Client:    mgr.GetClient(),
			APIReader: mgr.GetAPIReader(),
			Scheme:    mgr.GetScheme(),
			// The unwatched pre-check must come back within the Eventually
			// window of the tests.
			RetryInterval: time.Second,
			RESTEndpoint: func(cluster *v1.CamundaCluster, _ components.Effective) string {
				return clusterUserAPI(cluster)
			},
		}).SetupWithManager(mgr)
	})

	ctx, k8sClient = env.Ctx, env.Client
})

var _ = AfterSuite(func() {
	By("tearing down the test environment")
	Eventually(env.Stop, time.Minute, time.Second).Should(Succeed())
})

// clusterUserAPI returns the user API that serves cluster, or an address
// that answers nothing when its namespace registered none.
func clusterUserAPI(cluster *v1.CamundaCluster) string {
	if endpoint, ok := userAPIEndpoints.Load(cluster.Namespace); ok {
		return endpoint.(string)
	}

	return unreachableUserAPI
}
