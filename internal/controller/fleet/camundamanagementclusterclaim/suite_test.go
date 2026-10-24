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

package camundamanagementclusterclaim

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
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	fleetv1alpha1 "github.com/konsole-is/camunda-operator/api/fleet/v1alpha1"
	"github.com/konsole-is/camunda-operator/pkg/wrappers/camundamanagementcluster"
	"github.com/konsole-is/camunda-operator/pkg/wrappers/pluginrequest"
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
	plugin    *fakePlugin
	witness   *applyWitness
)

func TestCamundaManagementClusterClaimController(t *testing.T) {
	RegisterFailHandler(Fail)

	RunSpecs(t, "CamundaManagementClusterClaim Controller Suite")
}

var _ = BeforeSuite(func() {
	logf.SetLogger(zap.New(zap.WriteTo(GinkgoWriter), zap.UseDevMode(true)))

	plugin = newFakePlugin()
	witness = &applyWitness{policyRecorded: map[types.NamespacedName][]bool{}}

	env = envtest.Start(func(mgr ctrl.Manager) error {
		uncached, err := client.NewWithWatch(mgr.GetConfig(), client.Options{
			Scheme: mgr.GetScheme(),
			Mapper: mgr.GetRESTMapper(),
		})
		if err != nil {
			return err
		}
		witnessed := interceptor.NewClient(uncached, interceptor.Funcs{Patch: witness.patch})

		if err := (&Reconciler{
			Client:          mgr.GetClient(),
			APIReader:       mgr.GetAPIReader(),
			Scheme:          mgr.GetScheme(),
			componentClient: pluginrequest.NewApplyClient(camundamanagementcluster.NewApplyClient(witnessed)),
		}).SetupWithManager(mgr); err != nil {
			return err
		}

		return plugin.SetupWithManager(mgr)
	})

	ctx, k8sClient = env.Ctx, env.Client
	createFixtures()
})

var _ = AfterSuite(func() {
	By("tearing down the test environment")
	Eventually(env.Stop, time.Minute, time.Second).Should(Succeed())
})

// applyWitness records, for each apply of a PluginRequest, whether the claim
// named by its claim label had status.resolvedPolicy at that moment. It
// passes every call through.
type applyWitness struct {
	mu             sync.Mutex
	policyRecorded map[types.NamespacedName][]bool
}

func (w *applyWitness) patch(
	ctx context.Context,
	c client.WithWatch,
	obj client.Object,
	patch client.Patch,
	opts ...client.PatchOption,
) error {
	kind := obj.GetObjectKind().GroupVersionKind().Kind
	if claimName := obj.GetLabels()[fleetv1alpha1.LabelClaim]; kind == "PluginRequest" && claimName != "" {
		key := types.NamespacedName{Namespace: obj.GetNamespace(), Name: claimName}
		var claim fleetv1alpha1.CamundaManagementClusterClaim
		recorded := c.Get(ctx, key, &claim) == nil && claim.Status.ResolvedPolicy != nil

		w.mu.Lock()
		w.policyRecorded[key] = append(w.policyRecorded[key], recorded)
		w.mu.Unlock()
	}

	return c.Patch(ctx, obj, patch, opts...)
}

// recorded returns what the witness saw for the applies of the requests of
// the claim key.
func (w *applyWitness) recorded(key types.NamespacedName) []bool {
	w.mu.Lock()
	defer w.mu.Unlock()

	return append([]bool(nil), w.policyRecorded[key]...)
}
