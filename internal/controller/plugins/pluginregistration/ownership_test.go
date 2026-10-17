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

package pluginregistration

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	utilrand "k8s.io/apimachinery/pkg/util/rand"
	"k8s.io/client-go/util/retry"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	pluginsv1alpha1 "github.com/konsole-is/camunda-operator/api/plugins/v1alpha1"
	v1 "github.com/konsole-is/camunda-operator/api/v1"
	"github.com/konsole-is/camunda-operator/internal/ownerstatus"
)

// racingClient is a client whose first status update is preceded by race.
// The update then carries a stale resourceVersion and conflicts.
type racingClient struct {
	client.Client
	race    func(ctx context.Context, obj client.Object)
	updates int
}

func (c *racingClient) Status() client.SubResourceWriter {
	return &racingStatusWriter{SubResourceWriter: c.Client.Status(), c: c}
}

type racingStatusWriter struct {
	client.SubResourceWriter
	c *racingClient
}

func (w *racingStatusWriter) Update(
	ctx context.Context,
	obj client.Object,
	opts ...client.SubResourceUpdateOption,
) error {
	w.c.updates++
	if w.c.updates == 1 {
		w.c.race(ctx, obj)
	}

	return w.SubResourceWriter.Update(ctx, obj, opts...)
}

var _ = Describe("PluginRegistration Accepted next to the plugin controller", func() {
	It("keeps the Ready and observedGeneration that the plugin wrote between the read and the write", func() {
		section := createSection("section-" + utilrand.String(5))
		reg := newRegistration(pluginsv1alpha1.ModeOptional, section.Name)
		createRegistration(reg)
		expectAccepted(reg.Name, metav1.ConditionTrue, v1.ReasonHealthy)

		var pluginVersion string
		writeAsPlugin := func(ctx context.Context, _ client.Object) {
			Expect(retry.RetryOnConflict(retry.DefaultRetry, func() error {
				var latest pluginsv1alpha1.PluginRegistration
				if err := k8sClient.Get(ctx, client.ObjectKeyFromObject(reg), &latest); err != nil {
					return err
				}
				latest.Status.ObservedGeneration = latest.Generation
				meta.SetStatusCondition(&latest.Status.Conditions, metav1.Condition{
					Type:               v1.ConditionReady,
					Status:             metav1.ConditionTrue,
					Reason:             v1.ReasonHealthy,
					Message:            "Written by the plugin controller",
					ObservedGeneration: latest.Generation,
				})
				if err := k8sClient.Status().Update(ctx, &latest); err != nil {
					return err
				}
				pluginVersion = latest.ResourceVersion

				return nil
			})).To(Succeed())
		}
		racing := &racingClient{Client: managerClient, race: writeAsPlugin}
		r := &Reconciler{Client: racing, OperatorVersion: operatorVersion}
		req := ctrl.Request{NamespacedName: client.ObjectKeyFromObject(reg)}

		By("losing the status write to the plugin controller")
		result, err := r.Reconcile(ctx, req)
		Expect(err).NotTo(HaveOccurred(), "a conflict must not be a reconcile error")
		Expect(result).To(Equal(ctrl.Result{RequeueAfter: ownerstatus.ConflictRequeue}))
		Expect(racing.updates).To(Equal(1))

		By("waiting until the cache of the next reconcile holds the write of the plugin")
		Eventually(func(g Gomega) {
			var cached pluginsv1alpha1.PluginRegistration
			g.Expect(managerClient.Get(ctx, client.ObjectKeyFromObject(reg), &cached)).To(Succeed())
			g.Expect(cached.ResourceVersion).To(Equal(pluginVersion))
		}, timeout, interval).Should(Succeed())

		By("writing Accepted on the next reconcile")
		result, err = r.Reconcile(ctx, req)
		Expect(err).NotTo(HaveOccurred())
		Expect(result).To(Equal(ctrl.Result{}))
		Expect(racing.updates).To(Equal(2))

		var latest pluginsv1alpha1.PluginRegistration
		Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(reg), &latest)).To(Succeed())
		Expect(
			latest.Status.ObservedGeneration,
		).To(Equal(latest.Generation), "the Accepted write rolled back observedGeneration")
		ready := meta.FindStatusCondition(latest.Status.Conditions, v1.ConditionReady)
		Expect(ready).NotTo(BeNil(), "the Accepted write dropped Ready")
		Expect(ready.Message).To(Equal("Written by the plugin controller"))
		accepted := meta.FindStatusCondition(latest.Status.Conditions, pluginsv1alpha1.ConditionAccepted)
		Expect(accepted).NotTo(BeNil())
		Expect(accepted.Status).To(Equal(metav1.ConditionTrue))
		Expect(accepted.Reason).To(Equal(v1.ReasonHealthy))
		Expect(accepted.ObservedGeneration).To(Equal(latest.Generation))
	})
})
