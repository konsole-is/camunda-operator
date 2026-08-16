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

// Package objectstore opens the bucket that an ObjectStorageConfig describes
// and gives its consumers the small surface that backups need: upload,
// delete, and list. It authenticates through the provider default chain
// (workload identity) or through resolved static credentials, and it never
// touches Kubernetes: the caller resolves Secrets and passes the values in.
package objectstore

import (
	"context"
	"errors"
	"fmt"
	"io"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
	"github.com/Azure/azure-sdk-for-go/sdk/storage/azblob"
	azcontainer "github.com/Azure/azure-sdk-for-go/sdk/storage/azblob/container"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"gocloud.dev/blob"
	"gocloud.dev/blob/azureblob"
	"gocloud.dev/blob/gcsblob"
	"gocloud.dev/blob/s3blob"
	"gocloud.dev/gcerrors"
	"gocloud.dev/gcp"
	"golang.org/x/oauth2/google"

	v1 "github.com/konsole-is/camunda-operator/api/v1"
)

// Credentials are the resolved static credentials of a bucket whose auth type
// is credentials. Exactly the fields of the contract's storage type are set.
// Nil means workload identity: the provider default chain authenticates.
type Credentials struct {
	// AccessKeyID is the S3 access key ID.
	AccessKeyID string
	// SecretAccessKey is the S3 secret access key.
	SecretAccessKey string
	// ServiceAccountJSON is the GCS service-account key.
	ServiceAccountJSON []byte
	// AccountKey is the Azure storage account key.
	AccountKey string
}

// Bucket is an open bucket. Keys are full object keys; the caller builds them
// (including any base path) itself.
type Bucket struct {
	bucket *blob.Bucket
}

// newBucket wraps an open blob bucket.
func newBucket(bucket *blob.Bucket) *Bucket { return &Bucket{bucket: bucket} }

// Open opens the bucket that cfg describes. With auth type credentials the
// caller passes the resolved values in creds; with workload identity creds is
// nil and the provider default chain authenticates. The caller must Close the
// bucket.
func Open(ctx context.Context, cfg *v1.ObjectStorageConfig, creds *Credentials) (*Bucket, error) {
	switch {
	case cfg.Spec.S3 != nil:
		return openS3(ctx, cfg.Spec.S3, creds)
	case cfg.Spec.GCS != nil:
		return openGCS(ctx, cfg.Spec.GCS, creds)
	case cfg.Spec.AzureBlob != nil:
		return openAzureBlob(ctx, cfg.Spec.AzureBlob, creds)
	}

	return nil, fmt.Errorf("object storage config %q has no storage block", cfg.Name)
}

// Upload writes the content of r to key, replacing any existing object.
func (b *Bucket) Upload(ctx context.Context, key string, r io.Reader) error {
	w, err := b.bucket.NewWriter(ctx, key, nil)
	if err != nil {
		return fmt.Errorf("opening writer for %q: %w", key, err)
	}

	if _, err := io.Copy(w, r); err != nil {
		// Abort the write; the close error only masks the copy error.
		_ = w.Close()
		return fmt.Errorf("uploading %q: %w", key, err)
	}

	if err := w.Close(); err != nil {
		return fmt.Errorf("finishing upload of %q: %w", key, err)
	}

	return nil
}

// Delete removes key. A key that does not exist is success, so a re-entrant
// finalizer can call it again.
func (b *Bucket) Delete(ctx context.Context, key string) error {
	err := b.bucket.Delete(ctx, key)
	if err != nil && gcerrors.Code(err) != gcerrors.NotFound {
		return fmt.Errorf("deleting %q: %w", key, err)
	}

	return nil
}

// List returns the keys under prefix, in lexical order.
func (b *Bucket) List(ctx context.Context, prefix string) ([]string, error) {
	var keys []string

	iter := b.bucket.List(&blob.ListOptions{Prefix: prefix})
	for {
		obj, err := iter.Next(ctx)
		if errors.Is(err, io.EOF) {
			return keys, nil
		}
		if err != nil {
			return nil, fmt.Errorf("listing %q: %w", prefix, err)
		}
		keys = append(keys, obj.Key)
	}
}

// Close releases the bucket.
func (b *Bucket) Close() { _ = b.bucket.Close() }

// openS3 opens an S3 or S3-compatible bucket.
func openS3(ctx context.Context, spec *v1.S3Storage, creds *Credentials) (*Bucket, error) {
	var opts []func(*awsconfig.LoadOptions) error
	if spec.Region != "" {
		opts = append(opts, awsconfig.WithRegion(spec.Region))
	} else {
		// The SDK requires a region even when a custom endpoint routes every
		// request; S3-compatible stores ignore it.
		opts = append(opts, awsconfig.WithRegion("us-east-1"))
	}

	if spec.Auth.Type == v1.ObjectStorageAuthTypeCredentials {
		if creds == nil || creds.AccessKeyID == "" || creds.SecretAccessKey == "" {
			return nil, errors.New("s3 auth type is credentials, but no resolved credentials were passed")
		}
		opts = append(opts, awsconfig.WithCredentialsProvider(
			credentials.NewStaticCredentialsProvider(creds.AccessKeyID, creds.SecretAccessKey, ""),
		))
	}

	awsCfg, err := awsconfig.LoadDefaultConfig(ctx, opts...)
	if err != nil {
		return nil, fmt.Errorf("loading AWS configuration: %w", err)
	}

	client := s3.NewFromConfig(awsCfg, func(o *s3.Options) {
		if spec.Endpoint != "" {
			o.BaseEndpoint = &spec.Endpoint
		}
		o.UsePathStyle = spec.ForcePathStyle
	})

	bucket, err := s3blob.OpenBucketV2(ctx, client, spec.BucketName, nil)
	if err != nil {
		return nil, fmt.Errorf("opening s3 bucket %q: %w", spec.BucketName, err)
	}

	return newBucket(bucket), nil
}

// openGCS opens a Google Cloud Storage bucket.
func openGCS(ctx context.Context, spec *v1.GCSStorage, creds *Credentials) (*Bucket, error) {
	var tokenSource gcp.TokenSource

	if spec.Auth.Type == v1.ObjectStorageAuthTypeCredentials {
		if creds == nil || len(creds.ServiceAccountJSON) == 0 {
			return nil, errors.New("gcs auth type is credentials, but no resolved credentials were passed")
		}
		// The type is asserted, not inferred: the key arrives from a Secret
		// that the contract names, and an unvalidated credential
		// configuration from outside the operator must never reach the
		// Google libraries.
		googleCreds, err := google.CredentialsFromJSONWithType(
			ctx,
			creds.ServiceAccountJSON,
			google.ServiceAccount,
			"https://www.googleapis.com/auth/devstorage.read_write",
		)
		if err != nil {
			return nil, fmt.Errorf("parsing GCS service-account key: %w", err)
		}
		tokenSource = googleCreds.TokenSource
	} else {
		googleCreds, err := gcp.DefaultCredentials(ctx)
		if err != nil {
			return nil, fmt.Errorf("loading GCS default credentials: %w", err)
		}
		tokenSource = googleCreds.TokenSource
	}

	client, err := gcp.NewHTTPClient(gcp.DefaultTransport(), tokenSource)
	if err != nil {
		return nil, fmt.Errorf("building GCS client: %w", err)
	}

	bucket, err := gcsblob.OpenBucket(ctx, client, spec.BucketName, nil)
	if err != nil {
		return nil, fmt.Errorf("opening gcs bucket %q: %w", spec.BucketName, err)
	}

	return newBucket(bucket), nil
}

// openAzureBlob opens an Azure Blob Storage container.
func openAzureBlob(ctx context.Context, spec *v1.AzureBlobStorage, creds *Credentials) (*Bucket, error) {
	serviceURL := spec.Endpoint
	if serviceURL == "" {
		serviceURL = fmt.Sprintf("https://%s.blob.core.windows.net", spec.AccountName)
	}
	containerURL := serviceURL + "/" + spec.Container

	var client *azcontainer.Client

	if spec.Auth.Type == v1.ObjectStorageAuthTypeCredentials {
		if creds == nil || creds.AccountKey == "" {
			return nil, errors.New("azureBlob auth type is credentials, but no resolved credentials were passed")
		}
		sharedKey, err := azblob.NewSharedKeyCredential(spec.AccountName, creds.AccountKey)
		if err != nil {
			return nil, fmt.Errorf("building Azure shared-key credential: %w", err)
		}
		client, err = azcontainer.NewClientWithSharedKeyCredential(containerURL, sharedKey, nil)
		if err != nil {
			return nil, fmt.Errorf("building Azure container client: %w", err)
		}
	} else {
		cred, err := azidentity.NewDefaultAzureCredential(nil)
		if err != nil {
			return nil, fmt.Errorf("loading Azure default credentials: %w", err)
		}
		client, err = azcontainer.NewClient(containerURL, azcore.TokenCredential(cred), nil)
		if err != nil {
			return nil, fmt.Errorf("building Azure container client: %w", err)
		}
	}

	bucket, err := azureblob.OpenBucket(ctx, client, nil)
	if err != nil {
		return nil, fmt.Errorf("opening azure container %q: %w", spec.Container, err)
	}

	return newBucket(bucket), nil
}
