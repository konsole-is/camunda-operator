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

package v1_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	v1 "github.com/konsole-is/camunda-operator/api/v1"
)

func TestWorkloadIdentityAnnotations(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		spec v1.ObjectStorageConfigSpec
		want map[string]string
	}{
		{
			name: "an S3 role ARN becomes the IRSA annotation",
			spec: v1.ObjectStorageConfigSpec{
				Type: v1.ObjectStorageTypeS3,
				S3: &v1.S3Storage{
					BucketName: "backups",
					Auth: v1.S3StorageAuth{
						Type:             v1.ObjectStorageAuthTypeWorkloadIdentity,
						WorkloadIdentity: &v1.S3WorkloadIdentity{RoleARN: "arn:aws:iam::1:role/camunda"},
					},
				},
			},
			want: map[string]string{v1.IRSARoleARNAnnotation: "arn:aws:iam::1:role/camunda"},
		},
		{
			name: "a GCS service account becomes the GKE annotation",
			spec: v1.ObjectStorageConfigSpec{
				Type: v1.ObjectStorageTypeGCS,
				GCS: &v1.GCSStorage{
					BucketName: "backups",
					Auth: v1.GCSStorageAuth{
						Type:             v1.ObjectStorageAuthTypeWorkloadIdentity,
						WorkloadIdentity: &v1.GCSWorkloadIdentity{ServiceAccountEmail: "camunda@p.iam.gserviceaccount.com"},
					},
				},
			},
			want: map[string]string{v1.GKEServiceAccountAnnotation: "camunda@p.iam.gserviceaccount.com"},
		},
		{
			name: "an Azure client ID becomes the Azure annotation",
			spec: v1.ObjectStorageConfigSpec{
				Type: v1.ObjectStorageTypeAzureBlob,
				AzureBlob: &v1.AzureBlobStorage{
					AccountName: "camundabackups",
					Container:   "backups",
					Auth: v1.AzureBlobStorageAuth{
						Type:             v1.ObjectStorageAuthTypeWorkloadIdentity,
						WorkloadIdentity: &v1.AzureBlobWorkloadIdentity{ClientID: "11111111-2222-3333-4444-555555555555"},
					},
				},
			},
			want: map[string]string{v1.AzureClientIDAnnotation: "11111111-2222-3333-4444-555555555555"},
		},
		{
			// Pod Identity and Workload Identity Federation bind the
			// ServiceAccount on the cloud side, so the operator adds nothing.
			name: "an empty identity block yields no annotation",
			spec: v1.ObjectStorageConfigSpec{
				Type: v1.ObjectStorageTypeS3,
				S3: &v1.S3Storage{
					BucketName: "backups",
					Auth: v1.S3StorageAuth{
						Type:             v1.ObjectStorageAuthTypeWorkloadIdentity,
						WorkloadIdentity: &v1.S3WorkloadIdentity{},
					},
				},
			},
			want: nil,
		},
		{
			name: "an absent identity block yields no annotation",
			spec: v1.ObjectStorageConfigSpec{
				Type: v1.ObjectStorageTypeS3,
				S3: &v1.S3Storage{
					BucketName: "backups",
					Auth:       v1.S3StorageAuth{Type: v1.ObjectStorageAuthTypeWorkloadIdentity},
				},
			},
			want: nil,
		},
		{
			name: "static credentials yield no annotation",
			spec: v1.ObjectStorageConfigSpec{
				Type: v1.ObjectStorageTypeS3,
				S3: &v1.S3Storage{
					BucketName: "backups",
					Auth: v1.S3StorageAuth{
						Type: v1.ObjectStorageAuthTypeCredentials,
						Credentials: &v1.S3Credentials{
							SecretRef: v1.S3CredentialsSecretRef{
								Name:               "minio",
								Namespace:          "camunda",
								AccessKeyIDKey:     "accessKeyId",
								SecretAccessKeyKey: "secretAccessKey",
							},
						},
					},
				},
			},
			want: nil,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			cfg := &v1.ObjectStorageConfig{Spec: test.spec}
			assert.Equal(t, test.want, cfg.WorkloadIdentityAnnotations())
		})
	}
}

func TestServiceAccountSpecCreates(t *testing.T) {
	t.Parallel()

	no := false
	yes := true

	tests := []struct {
		name string
		spec *v1.ServiceAccountSpec
		want bool
	}{
		{name: "an absent spec creates", spec: nil, want: true},
		{name: "an unset create creates", spec: &v1.ServiceAccountSpec{}, want: true},
		{name: "create true creates", spec: &v1.ServiceAccountSpec{Create: &yes}, want: true},
		{name: "create false does not create", spec: &v1.ServiceAccountSpec{Create: &no}, want: false},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, test.want, test.spec.Creates())
		})
	}
}
