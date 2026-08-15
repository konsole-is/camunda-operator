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

package databaseserverconfig

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	v1 "github.com/konsole-is/camunda-operator/api/v1"
	"github.com/konsole-is/camunda-operator/internal/fixtures"
)

var _ = Describe("DatabaseServerConfig schema", func() {
	DescribeTable(
		"admission",
		func(mutate func(*v1.DatabaseServerConfig), wantErr string) {
			obj := fixtures.DatabaseServerConfig()
			mutate(obj)
			err := k8sClient.Create(ctx, obj)
			if wantErr == "" {
				Expect(err).NotTo(HaveOccurred())
				DeferCleanup(func() { _ = k8sClient.Delete(ctx, obj) })
			} else {
				Expect(err).To(MatchError(ContainSubstring(wantErr)))
			}
		},
		Entry("accepts the minimal doc example", func(*v1.DatabaseServerConfig) {}, ""),
		Entry(
			"accepts pitr with retention", func(o *v1.DatabaseServerConfig) {
				o.Spec.PITR = &v1.PITRCapability{Enabled: true, RetentionPeriodDays: new(int32(7))}
			}, "",
		),
		Entry("rejects unknown engine", func(o *v1.DatabaseServerConfig) { o.Spec.Engine = "mysql" }, "spec.engine"),
		Entry("rejects port 0", func(o *v1.DatabaseServerConfig) { o.Spec.Port = 0 }, "spec.port"),
		Entry("rejects port above 65535", func(o *v1.DatabaseServerConfig) { o.Spec.Port = 70000 }, "spec.port"),
		Entry("rejects empty host", func(o *v1.DatabaseServerConfig) { o.Spec.Host = "" }, "spec.host"),
		Entry(
			"rejects missing secret namespace", func(o *v1.DatabaseServerConfig) {
				o.Spec.AdminCredentialsSecretRef.Namespace = ""
			}, "namespace",
		),
		Entry(
			"rejects pitr enabled without retention", func(o *v1.DatabaseServerConfig) {
				o.Spec.PITR = &v1.PITRCapability{Enabled: true}
			}, "retentionPeriodDays",
		),
		Entry(
			"rejects pitr retention 0", func(o *v1.DatabaseServerConfig) {
				o.Spec.PITR = &v1.PITRCapability{Enabled: true, RetentionPeriodDays: new(int32(0))}
			}, "retentionPeriodDays",
		),
	)
})
