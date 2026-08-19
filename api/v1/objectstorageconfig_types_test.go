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
						Type: v1.ObjectStorageAuthTypeWorkloadIdentity,
						WorkloadIdentity: &v1.GCSWorkloadIdentity{
							ServiceAccountEmail: "camunda@p.iam.gserviceaccount.com",
						},
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
						Type: v1.ObjectStorageAuthTypeWorkloadIdentity,
						WorkloadIdentity: &v1.AzureBlobWorkloadIdentity{
							ClientID: "11111111-2222-3333-4444-555555555555",
						},
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

// The helpers dispatch on the declared type. A contract whose type and block
// disagree yields nothing, never the wrong block — the same rule
// objectstore.Open documents.
func TestHelpersDispatchOnTheDeclaredType(t *testing.T) {
	t.Parallel()

	mismatched := &v1.ObjectStorageConfig{Spec: v1.ObjectStorageConfigSpec{
		Type: v1.ObjectStorageTypeS3,
		GCS: &v1.GCSStorage{
			BucketName: "backups",
			BasePath:   "clusters",
			Auth: v1.GCSStorageAuth{
				Type:        v1.ObjectStorageAuthTypeCredentials,
				Credentials: &v1.GCSCredentials{SecretRef: v1.SecretKeyRef{Name: "k", Namespace: "n", Key: "key.json"}},
			},
		},
	}}

	assert.Empty(t, mismatched.BasePath())
	assert.Nil(t, mismatched.CredentialsSecret())
	assert.Nil(t, mismatched.WorkloadIdentityAnnotations())
	assert.Nil(t, mismatched.WorkloadIdentityPodLabels())
}

// Leading and trailing slashes are trimmed, so the repository registration
// and the object keys derive one layout from a contract admitted before the
// pattern forbade them.
func TestBasePathTrimsSlashes(t *testing.T) {
	t.Parallel()

	cfg := &v1.ObjectStorageConfig{Spec: v1.ObjectStorageConfigSpec{
		Type: v1.ObjectStorageTypeS3,
		S3: &v1.S3Storage{
			BucketName: "backups",
			BasePath:   "/clusters/prod/",
			Auth:       v1.S3StorageAuth{Type: v1.ObjectStorageAuthTypeWorkloadIdentity},
		},
	}}

	assert.Equal(t, "clusters/prod", cfg.BasePath())
}

// Azure workload identity needs a pod label on top of the ServiceAccount
// annotation; no other provider does.
func TestWorkloadIdentityPodLabels(t *testing.T) {
	t.Parallel()

	azure := &v1.ObjectStorageConfig{Spec: v1.ObjectStorageConfigSpec{
		Type: v1.ObjectStorageTypeAzureBlob,
		AzureBlob: &v1.AzureBlobStorage{
			AccountName: "camundabackups",
			Container:   "backups",
			Auth:        v1.AzureBlobStorageAuth{Type: v1.ObjectStorageAuthTypeWorkloadIdentity},
		},
	}}
	assert.Equal(
		t,
		map[string]string{v1.AzureWorkloadIdentityUseLabel: "true"},
		azure.WorkloadIdentityPodLabels(),
	)

	azure.Spec.AzureBlob.Auth = v1.AzureBlobStorageAuth{
		Type:        v1.ObjectStorageAuthTypeCredentials,
		Credentials: &v1.AzureBlobCredentials{SecretRef: v1.SecretKeyRef{Name: "k", Namespace: "n", Key: "accountKey"}},
	}
	assert.Nil(t, azure.WorkloadIdentityPodLabels(), "static credentials need no identity label")

	s3 := &v1.ObjectStorageConfig{Spec: v1.ObjectStorageConfigSpec{
		Type: v1.ObjectStorageTypeS3,
		S3: &v1.S3Storage{
			BucketName: "backups",
			Auth:       v1.S3StorageAuth{Type: v1.ObjectStorageAuthTypeWorkloadIdentity},
		},
	}}
	assert.Nil(t, s3.WorkloadIdentityPodLabels(), "only Azure needs a pod label")
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

// Location changes when, and only when, a key written through the contract
// would land somewhere else; auth is not part of it.
func TestLocationPinsWhereObjectsLive(t *testing.T) {
	t.Parallel()

	s3 := &v1.ObjectStorageConfig{Spec: v1.ObjectStorageConfigSpec{
		Type: v1.ObjectStorageTypeS3,
		S3: &v1.S3Storage{
			BucketName: "backups",
			BasePath:   "/clusters/",
			Endpoint:   "http://minio.minio.svc:9000",
			Auth:       v1.S3StorageAuth{Type: v1.ObjectStorageAuthTypeWorkloadIdentity},
		},
	}}
	assert.Equal(t, "s3://backups/clusters (endpoint http://minio.minio.svc:9000)", s3.Location())

	// The two spellings of one endpoint are one location: a trailing slash
	// must never read as a retarget and strand a dump.
	slashed := s3.DeepCopy()
	slashed.Spec.S3.Endpoint = "http://minio.minio.svc:9000/"
	assert.Equal(t, s3.Location(), slashed.Location())

	rotated := s3.DeepCopy()
	rotated.Spec.S3.Auth = v1.S3StorageAuth{
		Type: v1.ObjectStorageAuthTypeCredentials,
		Credentials: &v1.S3Credentials{SecretRef: v1.S3CredentialsSecretRef{
			Name: "keys", Namespace: "ns", AccessKeyIDKey: "id", SecretAccessKeyKey: "key",
		}},
	}
	assert.Equal(t, s3.Location(), rotated.Location(), "auth changes do not move the objects")

	retargeted := s3.DeepCopy()
	retargeted.Spec.S3.BucketName = "other"
	assert.NotEqual(t, s3.Location(), retargeted.Location())

	regional := &v1.ObjectStorageConfig{Spec: v1.ObjectStorageConfigSpec{
		Type: v1.ObjectStorageTypeS3,
		S3: &v1.S3Storage{
			BucketName: "backups",
			Region:     "eu-west-1",
			Auth:       v1.S3StorageAuth{Type: v1.ObjectStorageAuthTypeWorkloadIdentity},
		},
	}}
	assert.Equal(t, "s3://backups/ (region eu-west-1)", regional.Location())

	gcs := &v1.ObjectStorageConfig{Spec: v1.ObjectStorageConfigSpec{
		Type: v1.ObjectStorageTypeGCS,
		GCS: &v1.GCSStorage{
			BucketName: "b",
			BasePath:   "p",
			Auth:       v1.GCSStorageAuth{Type: v1.ObjectStorageAuthTypeWorkloadIdentity},
		},
	}}
	assert.Equal(t, "gs://b/p", gcs.Location())

	azure := &v1.ObjectStorageConfig{Spec: v1.ObjectStorageConfigSpec{
		Type: v1.ObjectStorageTypeAzureBlob,
		AzureBlob: &v1.AzureBlobStorage{
			AccountName: "acct", Container: "c", BasePath: "p",
			Auth: v1.AzureBlobStorageAuth{Type: v1.ObjectStorageAuthTypeWorkloadIdentity},
		},
	}}
	// Azure endpoints render through ServiceEndpoint: an unset endpoint, the
	// explicit public one, and its slashed spelling are all one place, so
	// they are one location.
	assert.Equal(t, "azblob://acct/c/p (endpoint https://acct.blob.core.windows.net)", azure.Location())
	explicit := azure.DeepCopy()
	explicit.Spec.AzureBlob.Endpoint = "https://acct.blob.core.windows.net/"
	assert.Equal(t, azure.Location(), explicit.Location())

	mismatched := &v1.ObjectStorageConfig{Spec: v1.ObjectStorageConfigSpec{Type: v1.ObjectStorageTypeGCS}}
	assert.Empty(t, mismatched.Location())
}

func TestSigningRegionOfAnS3Block(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		storage v1.S3Storage
		want    string
	}{
		"the region of the block wins over the placeholder": {
			storage: v1.S3Storage{Region: "eu-west-1", Endpoint: "http://minio.minio.svc:9000"},
			want:    "eu-west-1",
		},
		"an endpoint without a region gets the placeholder": {
			storage: v1.S3Storage{Endpoint: "http://minio.minio.svc:9000"},
			want:    v1.PlaceholderS3Region,
		},
		"a region without an endpoint is AWS S3 itself": {
			storage: v1.S3Storage{Region: "eu-west-1"},
			want:    "eu-west-1",
		},
		// The CRD admits no such block: region is required unless endpoint
		// is set. A placeholder here would aim every request of an AWS
		// bucket at the wrong region, so the empty answer stays empty and
		// each consumer falls back to its own chain.
		"neither one gets no placeholder": {
			storage: v1.S3Storage{},
			want:    "",
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tc.want, tc.storage.SigningRegion())
		})
	}
}
