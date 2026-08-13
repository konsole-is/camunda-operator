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
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	utilrand "k8s.io/apimachinery/pkg/util/rand"

	v1 "github.com/konsole-is/camunda-operator/api/v1"
)

// notAResourceName is a value the schema's resource-name rules must reject.
const notAResourceName = "Not_A_Name"

// validDatabase returns the doc's minimal example with a unique name.
func validDatabase() *v1.Database {
	return &v1.Database{
		ObjectMeta: metav1.ObjectMeta{Name: "db-" + utilrand.String(8)},
		Spec: v1.DatabaseSpec{
			ServerRef:       "my-db-server",
			DatabaseName:    "camunda",
			TargetNamespace: "my-cluster-ns",
		},
	}
}

// realisticDatabase returns the doc's realistic example with a unique name.
func realisticDatabase() *v1.Database {
	db := validDatabase()
	db.Spec.ApplicationCredentials = &v1.CredentialsSpec{
		SecretName: "my-camunda-db-app",
	}
	db.Spec.BackupCredentials = &v1.BackupCredentialsSpec{
		CredentialsSpec: v1.CredentialsSpec{SecretName: "my-camunda-db-backup"},
	}
	db.Spec.DatabaseConfig = "my-camunda-db"
	db.Spec.SecondaryStorageConfig = "my-storage-config"
	return db
}

var _ = Describe("Database schema", func() {
	DescribeTable("admission",
		func(build func() *v1.Database, mutate func(*v1.Database), wantErr string) {
			obj := build()
			mutate(obj)
			err := k8sClient.Create(ctx, obj)
			if wantErr == "" {
				Expect(err).NotTo(HaveOccurred())
				DeferCleanup(func() { _ = k8sClient.Delete(ctx, obj) })
			} else {
				Expect(err).To(MatchError(ContainSubstring(wantErr)))
			}
		},
		Entry("accepts the minimal doc example",
			validDatabase, func(*v1.Database) {}, ""),
		Entry("accepts the realistic doc example",
			realisticDatabase, func(*v1.Database) {}, ""),
		Entry("accepts a 63-character databaseName",
			validDatabase, func(o *v1.Database) {
				o.Spec.DatabaseName = "d" + strings.Repeat("b", 62)
			}, ""),
		Entry("rejects an uppercase databaseName",
			validDatabase, func(o *v1.Database) {
				o.Spec.DatabaseName = "Camunda"
			}, "databaseName"),
		Entry("rejects a leading-digit databaseName",
			validDatabase, func(o *v1.Database) {
				o.Spec.DatabaseName = "1camunda"
			}, "databaseName"),
		Entry("rejects a 64-character databaseName",
			validDatabase, func(o *v1.Database) {
				o.Spec.DatabaseName = "d" + strings.Repeat("b", 63)
			}, "databaseName"),
		Entry("rejects a missing serverRef",
			validDatabase, func(o *v1.Database) {
				o.Spec.ServerRef = ""
			}, "serverRef"),
		Entry("rejects a missing targetNamespace",
			validDatabase, func(o *v1.Database) {
				o.Spec.TargetNamespace = ""
			}, "targetNamespace"),
		Entry("rejects a non-DNS-1123 targetNamespace",
			validDatabase, func(o *v1.Database) {
				o.Spec.TargetNamespace = "My_Cluster_NS"
			}, "targetNamespace"),
		Entry("rejects a 64-character targetNamespace",
			validDatabase, func(o *v1.Database) {
				o.Spec.TargetNamespace = "n" + strings.Repeat("s", 63)
			}, "targetNamespace"),
		Entry("rejects a non-DNS-1123 databaseConfig name",
			validDatabase, func(o *v1.Database) {
				o.Spec.DatabaseConfig = notAResourceName
			}, "databaseConfig"),
		Entry("rejects a non-DNS-1123 secondaryStorageConfig name",
			validDatabase, func(o *v1.Database) {
				o.Spec.SecondaryStorageConfig = notAResourceName
			}, "secondaryStorageConfig"),
	)
})
