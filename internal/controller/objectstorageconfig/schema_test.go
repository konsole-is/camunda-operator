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

package objectstorageconfig

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	utilrand "k8s.io/apimachinery/pkg/util/rand"

	v1 "github.com/konsole-is/camunda-operator/api/v1"
)

// validObjectStorageConfig returns the minimal aws/S3 example of the CRD doc
// with a unique name.
func validObjectStorageConfig() *v1.ObjectStorageConfig {
	return &v1.ObjectStorageConfig{
		ObjectMeta: metav1.ObjectMeta{Name: "osc-" + utilrand.String(8)},
		Spec: v1.ObjectStorageConfigSpec{
			Provider:   v1.ObjectStorageProviderAWS,
			Type:       v1.ObjectStorageTypeS3,
			BucketID:   "arn:aws:s3:::my-cluster-backup-bucket",
			BucketName: "my-cluster-backup-bucket",
			AccountID:  "arn:aws:iam::123456789012:role/my-cluster-workload-role",
		},
	}
}

var _ = Describe("ObjectStorageConfig schema", func() {
	DescribeTable(
		"admission",
		func(mutate func(*v1.ObjectStorageConfig), wantErr string) {
			obj := validObjectStorageConfig()
			mutate(obj)
			err := k8sClient.Create(ctx, obj)
			if wantErr == "" {
				Expect(err).NotTo(HaveOccurred())
				DeferCleanup(func() { _ = k8sClient.Delete(ctx, obj) })
			} else {
				Expect(err).To(MatchError(ContainSubstring(wantErr)))
			}
		},
		Entry("accepts the aws/S3 doc example", func(*v1.ObjectStorageConfig) {}, ""),
		Entry(
			"accepts the gcp/GCS doc example with basePath", func(o *v1.ObjectStorageConfig) {
				o.Spec.Provider = v1.ObjectStorageProviderGCP
				o.Spec.Type = v1.ObjectStorageTypeGCS
				o.Spec.BucketID = "my-cluster-documents"
				o.Spec.BucketName = "my-cluster-documents"
				o.Spec.BasePath = "documents"
				o.Spec.AccountID = "my-cluster-workload@my-project.iam.gserviceaccount.com"
			}, "",
		),
		Entry(
			"accepts azure/AzureBlob", func(o *v1.ObjectStorageConfig) {
				o.Spec.Provider = v1.ObjectStorageProviderAzure
				o.Spec.Type = v1.ObjectStorageTypeAzureBlob
				o.Spec.BucketID = "my-cluster-backups"
				o.Spec.BucketName = "my-cluster-backups"
				o.Spec.AccountID = "11111111-2222-3333-4444-555555555555"
			}, "",
		),
		Entry(
			"rejects aws with GCS", func(o *v1.ObjectStorageConfig) {
				o.Spec.Type = v1.ObjectStorageTypeGCS
			}, "must match spec.provider",
		),
		Entry(
			"rejects gcp with S3", func(o *v1.ObjectStorageConfig) {
				o.Spec.Provider = v1.ObjectStorageProviderGCP
			}, "must match spec.provider",
		),
		Entry(
			"rejects unknown provider", func(o *v1.ObjectStorageConfig) {
				o.Spec.Provider = "digitalocean"
			}, "spec.provider",
		),
		Entry(
			"rejects empty bucketId", func(o *v1.ObjectStorageConfig) {
				o.Spec.BucketID = ""
			}, "bucketId",
		),
		Entry(
			"rejects empty accountId", func(o *v1.ObjectStorageConfig) {
				o.Spec.AccountID = ""
			}, "accountId",
		),
	)
})
