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

package secondarystorageconfig

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	utilrand "k8s.io/apimachinery/pkg/util/rand"
	"sigs.k8s.io/controller-runtime/pkg/client"

	v1 "github.com/konsole-is/camunda-operator/api/v1"
	"github.com/konsole-is/camunda-operator/internal/fixtures"
)

// validSecondaryStorageConfigES returns the minimal Elasticsearch example of
// the CRD doc with a unique name. The caller chooses the namespace.
func validSecondaryStorageConfigES() *v1.SecondaryStorageConfig {
	return &v1.SecondaryStorageConfig{
		ObjectMeta: metav1.ObjectMeta{Name: "ssc-" + utilrand.String(8)},
		Spec: v1.SecondaryStorageConfigSpec{
			Type: v1.SecondaryStorageTypeElasticsearch,
			Elasticsearch: &v1.ElasticsearchStorage{
				Endpoint: "https://my-cluster-es:9200",
				CredentialsSecretRef: v1.CredentialsSecretRef{
					Name: "my-cluster-es-credentials", Namespace: "my-cluster-ns",
					UsernameKey: "username", PasswordKey: "password",
				},
			},
		},
	}
}

// validSecondaryStorageConfigRDBMS returns the RDBMS example of the CRD doc
// with a unique name. The caller chooses the namespace.
func validSecondaryStorageConfigRDBMS() *v1.SecondaryStorageConfig {
	return &v1.SecondaryStorageConfig{
		ObjectMeta: metav1.ObjectMeta{Name: "ssc-" + utilrand.String(8)},
		Spec: v1.SecondaryStorageConfigSpec{
			Type:  v1.SecondaryStorageTypeRDBMS,
			RDBMS: &v1.RDBMSStorage{DatabaseConfigRef: "my-camunda-db"},
		},
	}
}

var _ = Describe("SecondaryStorageConfig schema", func() {
	DescribeTable("admission",
		func(build func() *v1.SecondaryStorageConfig, mutate func(*v1.SecondaryStorageConfig), wantErr string) {
			obj := build()
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
		Entry("accepts the Elasticsearch doc example",
			validSecondaryStorageConfigES, func(*v1.SecondaryStorageConfig) {}, ""),
		Entry("accepts the RDBMS doc example",
			validSecondaryStorageConfigRDBMS, func(*v1.SecondaryStorageConfig) {}, ""),
		Entry("rejects type elasticsearch without elasticsearch block",
			validSecondaryStorageConfigES, func(o *v1.SecondaryStorageConfig) {
				o.Spec.Elasticsearch = nil
			}, "matching spec.type"),
		Entry("rejects type rdbms with elasticsearch block set",
			validSecondaryStorageConfigES, func(o *v1.SecondaryStorageConfig) {
				o.Spec.Type = v1.SecondaryStorageTypeRDBMS
			}, "matching spec.type"),
		Entry("rejects both blocks set",
			validSecondaryStorageConfigES, func(o *v1.SecondaryStorageConfig) {
				o.Spec.RDBMS = &v1.RDBMSStorage{DatabaseConfigRef: "my-camunda-db"}
			}, "matching spec.type"),
		Entry("rejects unknown type",
			validSecondaryStorageConfigES, func(o *v1.SecondaryStorageConfig) {
				o.Spec.Type = "opensearch"
			}, "spec.type"),
		Entry("rejects non-URL endpoint",
			validSecondaryStorageConfigES, func(o *v1.SecondaryStorageConfig) {
				o.Spec.Elasticsearch.Endpoint = fixtures.NotAURL
			}, "endpoint"),
		Entry("rejects ftp endpoint",
			validSecondaryStorageConfigES, func(o *v1.SecondaryStorageConfig) {
				o.Spec.Elasticsearch.Endpoint = "ftp://x:9200"
			}, "endpoint"),
		Entry("rejects empty databaseConfigRef",
			validSecondaryStorageConfigRDBMS, func(o *v1.SecondaryStorageConfig) {
				o.Spec.RDBMS.DatabaseConfigRef = ""
			}, "databaseConfigRef"),
		Entry("rejects caSecretRef with an http endpoint",
			validSecondaryStorageConfigES, func(o *v1.SecondaryStorageConfig) {
				o.Spec.Elasticsearch.Endpoint = "http://my-cluster-es:9200"
				o.Spec.Elasticsearch.CASecretRef = &v1.SecretKeyRef{
					Name: "my-cluster-es-http-certs-public", Namespace: "my-cluster-ns", Key: "ca.crt",
				}
			}, "caSecretRef requires an https endpoint"),
		Entry("rejects caSecretRef with empty key",
			validSecondaryStorageConfigES, func(o *v1.SecondaryStorageConfig) {
				o.Spec.Elasticsearch.CASecretRef = &v1.SecretKeyRef{
					Name: "my-cluster-es-http-certs-public", Namespace: "my-cluster-ns", Key: "",
				}
			}, "key"),
	)

	It("round-trips caSecretRef", func() {
		obj := validSecondaryStorageConfigES()
		obj.Namespace = fixtures.SchemaTestNamespace
		obj.Spec.Elasticsearch.CASecretRef = &v1.SecretKeyRef{
			Name: "my-cluster-es-http-certs-public", Namespace: "my-cluster-ns", Key: "ca.crt",
		}

		Expect(k8sClient.Create(ctx, obj)).To(Succeed())
		DeferCleanup(func() { _ = k8sClient.Delete(ctx, obj) })

		var fetched v1.SecondaryStorageConfig
		Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(obj), &fetched)).To(Succeed())
		Expect(fetched.Spec.Elasticsearch.CASecretRef).To(Equal(obj.Spec.Elasticsearch.CASecretRef))
	})
})
