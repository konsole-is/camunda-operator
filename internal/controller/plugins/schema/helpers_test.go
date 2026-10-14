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
	utilrand "k8s.io/apimachinery/pkg/util/rand"
	"sigs.k8s.io/controller-runtime/pkg/client"
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

// nameOfLength returns a unique object name of n characters.
func nameOfLength(n int) string {
	return "p" + utilrand.String(n-1)
}
