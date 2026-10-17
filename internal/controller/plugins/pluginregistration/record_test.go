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
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	utilrand "k8s.io/apimachinery/pkg/util/rand"
	"sigs.k8s.io/controller-runtime/pkg/client"

	pluginsv1alpha1 "github.com/konsole-is/camunda-operator/api/plugins/v1alpha1"
	"github.com/konsole-is/camunda-operator/internal/ownerstatus"
)

// The receipt write runs against a real API server here, because a status
// patch on the API server drops a UID that it names.
var _ = Describe("ownerstatus.Record against the API server", func() {
	It("refuses a receipt for an object that replaced the owner under its name", func() {
		section := createSection("section-" + utilrand.String(5))
		reg := newRegistration(pluginsv1alpha1.ModeOptional, section.Name)
		createRegistration(reg)

		var read pluginsv1alpha1.PluginRegistration
		Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(reg), &read)).To(Succeed())

		By("replacing the registration under its name")
		Expect(k8sClient.Delete(ctx, reg)).To(Succeed())
		Eventually(func() bool {
			return apierrors.IsNotFound(
				k8sClient.Get(ctx, client.ObjectKeyFromObject(reg), &pluginsv1alpha1.PluginRegistration{}),
			)
		}, timeout, interval).Should(BeTrue())
		replacement := newRegistration(pluginsv1alpha1.ModeOptional, section.Name)
		replacement.Name = reg.Name
		createRegistration(replacement)

		By("recording a field on the registration as it was read")
		owner := read.DeepCopy()
		owner.Status.ObservedGeneration = 42
		Expect(ownerstatus.Record(ctx, k8sClient, k8sClient, &read, owner)).NotTo(Succeed())

		var stored pluginsv1alpha1.PluginRegistration
		Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(reg), &stored)).To(Succeed())
		Expect(stored.UID).NotTo(Equal(read.UID))
		Expect(stored.Status.ObservedGeneration).NotTo(Equal(int64(42)), "the receipt landed on the replacement")

		By("recording a field on the replacement itself")
		mine := stored.DeepCopy()
		mine.Status.ObservedGeneration = 42
		Expect(ownerstatus.Record(ctx, k8sClient, k8sClient, &stored, mine)).To(Succeed())
		Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(reg), &stored)).To(Succeed())
		Expect(stored.Status.ObservedGeneration).To(Equal(int64(42)))
	})
})
