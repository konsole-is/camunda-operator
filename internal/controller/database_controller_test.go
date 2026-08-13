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

package controller

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

var _ = Describe("Database controller", func() {
	// Scaffold smoke spec: the reconciler is registered but reconciles nothing
	// yet; the real reconciliation specs land with the controller (#38).
	It("reconciles a valid resource without error", func() {
		database := validDatabase()
		Expect(k8sClient.Create(ctx, database)).To(Succeed())
		DeferCleanup(func() { _ = k8sClient.Delete(ctx, database) })

		reconciler := &DatabaseReconciler{
			Client: k8sClient,
			Scheme: k8sClient.Scheme(),
		}

		_, err := reconciler.Reconcile(ctx, reconcile.Request{
			NamespacedName: client.ObjectKeyFromObject(database),
		})
		Expect(err).NotTo(HaveOccurred())
	})
})
