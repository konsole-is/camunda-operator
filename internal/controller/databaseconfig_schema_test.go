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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	utilrand "k8s.io/apimachinery/pkg/util/rand"

	v1 "github.com/konsole-is/camunda-operator/api/v1"
)

// validDatabaseConfig returns the doc's minimal example with a unique name.
func validDatabaseConfig() *v1.DatabaseConfig {
	return &v1.DatabaseConfig{
		ObjectMeta: metav1.ObjectMeta{Name: "dbc-" + utilrand.String(8)},
		Spec: v1.DatabaseConfigSpec{
			ServerRef:    "my-db-server",
			DatabaseName: "camunda",
			CredentialsSecretRef: v1.CredentialsSecretRef{
				Name: "my-camunda-db-credentials", Namespace: "my-cluster-ns",
				UsernameKey: "username", PasswordKey: "password",
			},
		},
	}
}

var _ = Describe("DatabaseConfig schema", func() {
	DescribeTable("admission",
		func(mutate func(*v1.DatabaseConfig), wantErr string) {
			obj := validDatabaseConfig()
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
		Entry("accepts with backup credentials ref", func(o *v1.DatabaseConfig) {
			o.Spec.BackupCredentialsSecretRef = &v1.CredentialsSecretRef{
				Name: "my-camunda-db-backup-credentials", Namespace: "my-cluster-ns",
				UsernameKey: "username", PasswordKey: "password",
			}
		}, ""),
		Entry("rejects empty serverRef", func(o *v1.DatabaseConfig) { o.Spec.ServerRef = "" }, "spec.serverRef"),
		Entry("rejects empty databaseName", func(o *v1.DatabaseConfig) { o.Spec.DatabaseName = "" }, "spec.databaseName"),
		Entry("rejects missing credentials namespace", func(o *v1.DatabaseConfig) {
			o.Spec.CredentialsSecretRef.Namespace = ""
		}, "namespace"),
		Entry("rejects backup ref with empty usernameKey", func(o *v1.DatabaseConfig) {
			o.Spec.BackupCredentialsSecretRef = &v1.CredentialsSecretRef{
				Name: "my-camunda-db-backup-credentials", Namespace: "my-cluster-ns",
				PasswordKey: "password",
			}
		}, "usernameKey"),
	)
})
