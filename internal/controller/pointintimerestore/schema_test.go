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
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	utilrand "k8s.io/apimachinery/pkg/util/rand"

	v1 "github.com/konsole-is/camunda-operator/api/v1"
)

// The admission schema of the PointInTimeRestore CRD.
var _ = Describe("PointInTimeRestore schema", func() {
	valid := func() *v1.PointInTimeRestore {
		return &v1.PointInTimeRestore{
			ObjectMeta: metav1.ObjectMeta{
				Name: "pitr-" + utilrand.String(6), Namespace: "default",
			},
			Spec: v1.PointInTimeRestoreSpec{
				ClusterRef: v1.ClusterRef{Name: "my-cluster"},
				Timestamp:  metav1.NewTime(suiteStart.Add(-time.Hour)),
			},
		}
	}

	It("rejects a spec change: a retry is a new resource", func() {
		pitr := valid()
		Expect(k8sClient.Create(ctx, pitr)).To(Succeed())
		DeferCleanup(func() { _ = k8sClient.Delete(ctx, pitr) })

		pitr.Spec.Timestamp = metav1.NewTime(suiteStart.Add(-2 * time.Hour))
		err := k8sClient.Update(ctx, pitr)
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("immutable"))
	})

	It("requires the cluster name", func() {
		pitr := valid()
		pitr.Spec.ClusterRef.Name = ""
		Expect(k8sClient.Create(ctx, pitr)).To(HaveOccurred())
	})

	It("requires the timestamp", func() {
		pitr := valid()
		pitr.Spec.Timestamp = metav1.Time{}
		Expect(k8sClient.Create(ctx, pitr)).To(HaveOccurred())
	})
})
