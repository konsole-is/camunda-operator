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

// minioEndpoint is the S3-compatible endpoint the tests use to mark a bucket
// as anything other than AWS S3.
const minioEndpoint = "http://minio.minio.svc:9000"

// validObjectStorageConfig returns the minimal S3 example of the CRD doc with
// a unique name: a cloud bucket accessed through workload identity.
func validObjectStorageConfig() *v1.ObjectStorageConfig {
	return &v1.ObjectStorageConfig{
		ObjectMeta: metav1.ObjectMeta{Name: "osc-" + utilrand.String(8), Namespace: "default"},
		Spec: v1.ObjectStorageConfigSpec{
			Type: v1.ObjectStorageTypeS3,
			S3: &v1.S3Storage{
				BucketName: "my-cluster-backup-bucket",
				Region:     "eu-west-1",
				Auth: v1.S3StorageAuth{
					Type: v1.ObjectStorageAuthTypeWorkloadIdentity,
					WorkloadIdentity: &v1.S3WorkloadIdentity{
						RoleARN: "arn:aws:iam::123456789012:role/my-cluster-workload-role",
					},
				},
			},
		},
	}
}

// s3Credentials returns the credentials block of the S3-compatible doc
// example.
func s3Credentials() *v1.S3Credentials {
	return &v1.S3Credentials{
		SecretRef: v1.S3CredentialsSecretRef{
			Name:               "minio-credentials",
			AccessKeyIDKey:     "accessKeyId",
			SecretAccessKeyKey: "secretAccessKey",
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
		Entry("accepts the S3 doc example", func(*v1.ObjectStorageConfig) {}, ""),
		Entry(
			"accepts S3 with an empty workloadIdentity block", func(o *v1.ObjectStorageConfig) {
				o.Spec.S3.Auth.WorkloadIdentity = nil
			}, "",
		),
		Entry(
			"accepts S3-compatible storage with endpoint and credentials", func(o *v1.ObjectStorageConfig) {
				o.Spec.S3.Region = ""
				o.Spec.S3.Endpoint = minioEndpoint
				o.Spec.S3.ForcePathStyle = true
				o.Spec.S3.Auth = v1.S3StorageAuth{
					Type:        v1.ObjectStorageAuthTypeCredentials,
					Credentials: s3Credentials(),
				}
			}, "",
		),
		Entry(
			"accepts the GCS doc example", func(o *v1.ObjectStorageConfig) {
				o.Spec.Type = v1.ObjectStorageTypeGCS
				o.Spec.S3 = nil
				o.Spec.GCS = &v1.GCSStorage{
					BucketName: "my-cluster-documents",
					BasePath:   "documents",
					Auth: v1.GCSStorageAuth{
						Type: v1.ObjectStorageAuthTypeWorkloadIdentity,
						WorkloadIdentity: &v1.GCSWorkloadIdentity{
							ServiceAccountEmail: "camunda@my-project.iam.gserviceaccount.com",
						},
					},
				}
			}, "",
		),
		Entry(
			"accepts AzureBlob with credentials", func(o *v1.ObjectStorageConfig) {
				o.Spec.Type = v1.ObjectStorageTypeAzureBlob
				o.Spec.S3 = nil
				o.Spec.AzureBlob = &v1.AzureBlobStorage{
					AccountName: "camundabackups",
					Container:   "backups",
					Auth: v1.AzureBlobStorageAuth{
						Type: v1.ObjectStorageAuthTypeCredentials,
						Credentials: &v1.AzureBlobCredentials{
							SecretRef: v1.LocalSecretKeyRef{
								Name: "azure-key",
								Key:  "accountKey",
							},
						},
					},
				}
			}, "",
		),
		Entry(
			"rejects type S3 without the s3 block", func(o *v1.ObjectStorageConfig) {
				o.Spec.S3 = nil
			}, "exactly the block matching spec.type must be set",
		),
		Entry(
			"rejects type S3 with a gcs block", func(o *v1.ObjectStorageConfig) {
				o.Spec.GCS = &v1.GCSStorage{
					BucketName: "extra",
					Auth:       v1.GCSStorageAuth{Type: v1.ObjectStorageAuthTypeWorkloadIdentity},
				}
			}, "exactly the block matching spec.type must be set",
		),
		Entry(
			"rejects an unknown type", func(o *v1.ObjectStorageConfig) {
				o.Spec.Type = "FTP"
			}, "spec.type",
		),
		Entry(
			"rejects S3 without region and endpoint", func(o *v1.ObjectStorageConfig) {
				o.Spec.S3.Region = ""
			}, "region is required unless endpoint is set",
		),
		Entry(
			"rejects auth.type credentials without the credentials block", func(o *v1.ObjectStorageConfig) {
				o.Spec.S3.Auth = v1.S3StorageAuth{Type: v1.ObjectStorageAuthTypeCredentials}
			}, "credentials is required when auth.type is credentials",
		),
		Entry(
			"rejects a credentials block under auth.type workloadIdentity", func(o *v1.ObjectStorageConfig) {
				o.Spec.S3.Auth.Credentials = s3Credentials()
			}, "credentials is required when auth.type is credentials",
		),
		Entry(
			"rejects a workloadIdentity block under auth.type credentials", func(o *v1.ObjectStorageConfig) {
				o.Spec.S3.Endpoint = minioEndpoint
				o.Spec.S3.Auth = v1.S3StorageAuth{
					Type:        v1.ObjectStorageAuthTypeCredentials,
					Credentials: s3Credentials(),
					WorkloadIdentity: &v1.S3WorkloadIdentity{
						RoleARN: "arn:aws:iam::123456789012:role/other",
					},
				}
			}, "workloadIdentity is only valid when auth.type is workloadIdentity",
		),
		Entry(
			"rejects an invalid s3 endpoint", func(o *v1.ObjectStorageConfig) {
				o.Spec.S3.Endpoint = "not a url"
			}, "endpoint must be a valid http or https URL",
		),
		Entry(
			"rejects an empty bucketName", func(o *v1.ObjectStorageConfig) {
				o.Spec.S3.BucketName = ""
			}, "bucketName",
		),
		Entry(
			"defaults auth.type to workloadIdentity", func(o *v1.ObjectStorageConfig) {
				o.Spec.S3.Auth = v1.S3StorageAuth{}
			}, "",
		),
	)
})
