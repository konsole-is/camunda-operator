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

package databaseconfig

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"

	v1 "github.com/konsole-is/camunda-operator/api/v1"
	"github.com/konsole-is/camunda-operator/internal/fixtures"
)

var _ = Describe("DatabaseConfig schema", func() {
	DescribeTable(
		"admission",
		func(mutate func(*v1.DatabaseConfig), wantErr string) {
			obj := fixtures.DatabaseConfig()
			obj.Namespace = fixtures.SchemaTestNamespace
			mutate(obj)
			err := k8sClient.Create(ctx, obj)
			if wantErr == "" {
				Expect(err).NotTo(HaveOccurred())
				DeferCleanup(func() { _ = k8sClient.Delete(ctx, obj) })
			} else {
				Expect(err).To(MatchError(ContainSubstring(wantErr)))
			}
		},
		Entry("accepts the minimal doc example", func(*v1.DatabaseConfig) {}, ""),
		Entry(
			"accepts with backup credentials ref", func(o *v1.DatabaseConfig) {
				o.Spec.BackupCredentialsSecretRef = &v1.LocalCredentialsSecretRef{
					Name:        "my-camunda-db-backup-credentials",
					UsernameKey: "username", PasswordKey: "password",
				}
			}, "",
		),
		Entry("rejects empty serverRef", func(o *v1.DatabaseConfig) { o.Spec.ServerRef = "" }, "spec.serverRef"),
		Entry(
			"rejects empty databaseName",
			func(o *v1.DatabaseConfig) { o.Spec.DatabaseName = "" },
			"spec.databaseName",
		),
		Entry(
			"rejects missing credentials name", func(o *v1.DatabaseConfig) {
				o.Spec.CredentialsSecretRef.Name = ""
			}, "name",
		),
		Entry(
			"accepts a backup ref without a usernameKey and defaults it to username", func(o *v1.DatabaseConfig) {
				o.Spec.BackupCredentialsSecretRef = &v1.LocalCredentialsSecretRef{
					Name:        "my-camunda-db-backup-credentials",
					PasswordKey: "password",
				}
			}, "",
		),
	)

	// The typed client drops an empty string under omitempty, so only an
	// unstructured object carries one to the API server. The API server
	// applies the usernameKey default only when the field is absent, so an
	// explicit empty string still breaks the minimum-length rule.
	It("rejects a backup ref with an explicitly empty usernameKey", func() {
		obj := fixtures.DatabaseConfig()
		obj.Namespace = fixtures.SchemaTestNamespace
		obj.Spec.BackupCredentialsSecretRef = &v1.LocalCredentialsSecretRef{
			Name: "my-camunda-db-backup-credentials", PasswordKey: "password",
		}
		raw, err := runtime.DefaultUnstructuredConverter.ToUnstructured(obj)
		Expect(err).NotTo(HaveOccurred())

		u := &unstructured.Unstructured{Object: raw}
		u.SetAPIVersion(v1.GroupVersion.String())
		u.SetKind("DatabaseConfig")
		Expect(unstructured.SetNestedField(
			u.Object, "", "spec", "backupCredentialsSecretRef", "usernameKey",
		)).To(Succeed())

		Expect(k8sClient.Create(ctx, u)).To(MatchError(ContainSubstring("usernameKey")))
	})
})
