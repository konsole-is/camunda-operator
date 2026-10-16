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

package schema

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	fleetv1alpha1 "github.com/konsole-is/camunda-operator/api/fleet/v1alpha1"
)

// expectAdmission creates obj and expects the API server to admit it when
// wantErr is empty, and to refuse it with an error that contains wantErr
// otherwise. An admitted object is deleted after the spec.
func expectAdmission(obj client.Object, wantErr string) {
	GinkgoHelper()

	err := k8sClient.Create(ctx, obj)
	if wantErr == "" {
		Expect(err).NotTo(HaveOccurred())
		DeferCleanup(func() { _ = k8sClient.Delete(ctx, obj) })

		return
	}

	Expect(err).To(MatchError(ContainSubstring(wantErr)))
}

// expectUpdate updates obj and expects the API server to admit the update
// when wantErr is empty, and to refuse it with an error that contains wantErr
// otherwise.
func expectUpdate(obj client.Object, wantErr string) {
	GinkgoHelper()

	err := k8sClient.Update(ctx, obj)
	if wantErr == "" {
		Expect(err).NotTo(HaveOccurred())

		return
	}

	Expect(err).To(MatchError(ContainSubstring(wantErr)))
}

// toUnstructured returns obj as an unstructured object of the fleet kind. The
// typed client sends an empty required string and drops an empty optional
// one, so a spec that leaves out a required field, or sends an explicit zero
// value, goes through this form.
func toUnstructured(obj runtime.Object, kind string) *unstructured.Unstructured {
	GinkgoHelper()

	content, err := runtime.DefaultUnstructuredConverter.ToUnstructured(obj)
	Expect(err).NotTo(HaveOccurred())

	u := &unstructured.Unstructured{Object: content}
	u.SetGroupVersionKind(fleetv1alpha1.GroupVersion.WithKind(kind))

	return u
}
