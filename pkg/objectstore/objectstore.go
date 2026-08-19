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
	// The declared type decides, not whichever block happens to be set. A
	// contract whose type and block disagree is a configuration error, and
	// naming the missing block is what makes it fixable.
	switch cfg.Spec.Type {
	case v1.ObjectStorageTypeS3:
		if cfg.Spec.S3 == nil {
			return nil, missingBlock(cfg, "s3")
		}
		return openS3(ctx, cfg.Spec.S3, creds)
	case v1.ObjectStorageTypeGCS:
		if cfg.Spec.GCS == nil {
			return nil, missingBlock(cfg, "gcs")
		}
		return openGCS(ctx, cfg.Spec.GCS, creds)
	case v1.ObjectStorageTypeAzureBlob:
		if cfg.Spec.AzureBlob == nil {
			return nil, missingBlock(cfg, "azureBlob")
		}
		return openAzureBlob(ctx, cfg.Spec.AzureBlob, creds)
	}

	return nil, fmt.Errorf("object storage config %q has unknown spec.type %q", cfg.Name, cfg.Spec.Type)
}

// missingBlock reports a contract whose declared type has no matching block.
func missingBlock(cfg *v1.ObjectStorageConfig, block string) error {
	return fmt.Errorf(
		"object storage config %q declares spec.type %q but has no spec.%s",
		cfg.Name, cfg.Spec.Type, block,
	)
}

// CredentialsFrom maps the data of the Secret that CredentialsSecret names
// onto the credentials of the contract's storage type. It returns nil when
// the contract uses workload identity, and an error when a configured key is
// absent from the Secret.
//
// The mapping lives here rather than in api/v1: api/v1 says which keys must
// exist, and this package knows what each one means to a storage client.
func CredentialsFrom(cfg *v1.ObjectStorageConfig, data map[string][]byte) (*Credentials, error) {
	secret := cfg.CredentialsSecret()
	if secret == nil {
		return nil, nil
	}

	value := func(key string) ([]byte, error) {
		v, ok := data[key]
		if !ok {
			return nil, fmt.Errorf(
				"object storage config %q names key %q, which Secret %s/%s does not hold",
				cfg.Name, key, secret.Namespace, secret.Name,
			)
		}

		return v, nil
	}

	switch {
	case cfg.Spec.S3 != nil && cfg.Spec.S3.Auth.Credentials != nil:
		ref := cfg.Spec.S3.Auth.Credentials.SecretRef
		id, err := value(ref.AccessKeyIDKey)
		if err != nil {
			return nil, err
		}
		key, err := value(ref.SecretAccessKeyKey)
		if err != nil {
			return nil, err
		}

		return &Credentials{AccessKeyID: string(id), SecretAccessKey: string(key)}, nil

	case cfg.Spec.GCS != nil && cfg.Spec.GCS.Auth.Credentials != nil:
		json, err := value(cfg.Spec.GCS.Auth.Credentials.SecretRef.Key)
		if err != nil {
			return nil, err
		}

		return &Credentials{ServiceAccountJSON: json}, nil

	case cfg.Spec.AzureBlob != nil && cfg.Spec.AzureBlob.Auth.Credentials != nil:
		accountKey, err := value(cfg.Spec.AzureBlob.Auth.Credentials.SecretRef.Key)
		if err != nil {
			return nil, err
		}

		return &Credentials{AccountKey: string(accountKey)}, nil
	}

	return nil, nil
}

// Upload writes the content of r to key, replacing any existing object. The
// object appears only when the whole of r was written: a read that fails
// partway leaves no object at all, so a truncated dump can never be mistaken
// for a whole one at restore time.
func (b *Bucket) Upload(ctx context.Context, key string, r io.Reader) error {
	// Cancelling this context is the only way to abandon a write. Closing the
	// writer commits whatever reached it, however little that is.
	writeCtx, abandon := context.WithCancel(ctx)
	defer abandon()

	w, err := b.bucket.NewWriter(writeCtx, key, nil)
	if err != nil {
		return fmt.Errorf("opening writer for %q: %w", key, err)
	}

	if _, err := io.Copy(w, r); err != nil {
		abandon()
		// The close error only reports the cancellation that was asked for.
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

// Walk calls fn for every key under prefix, in lexical order, and stops at
// the first error fn returns, which it returns unchanged.
//
// It streams rather than returning a slice on purpose. The same contract
// describes document-storage buckets, where the object count is unbounded and
// is the user's, not ours; collecting every key first would let one listing
// exhaust the memory of the manager and take every other controller down with
// it. A caller that needs a slice bounds it itself.
func (b *Bucket) Walk(ctx context.Context, prefix string, fn func(key string) error) error {
	iter := b.bucket.List(&blob.ListOptions{Prefix: prefix})
	for {
		obj, err := iter.Next(ctx)
		if errors.Is(err, io.EOF) {
			return nil
		}
		if err != nil {
			return fmt.Errorf("listing %q: %w", prefix, err)
		}

		if err := fn(obj.Key); err != nil {
			return err
		}
	}
}

// Close releases the bucket.
func (b *Bucket) Close() { _ = b.bucket.Close() }

// openS3 opens an S3 or S3-compatible bucket.
func openS3(ctx context.Context, spec *v1.S3Storage, creds *Credentials) (*Bucket, error) {
	var opts []func(*awsconfig.LoadOptions) error
	// The SDK requires a region even when a custom endpoint routes every
	// request, and SigningRegion answers with the placeholder for a contract
	// that carries an endpoint and no region. It answers with nothing only
	// for a contract that carries neither, which the CRD does not admit. The
	// region chain of the pod resolves the region then.
	if region := spec.SigningRegion(); region != "" {
		opts = append(opts, awsconfig.WithRegion(region))
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

// azureContainerURL is the URL of the container of spec. A trailing slash on
// the endpoint would double the separator, and the request signature is
// computed over the canonical resource — Azure then answers 403, which reads
// as bad credentials instead of a bad URL.
func azureContainerURL(spec *v1.AzureBlobStorage) string {
	return spec.ServiceEndpoint() + "/" + spec.Container
}

// openAzureBlob opens an Azure Blob Storage container.
func openAzureBlob(ctx context.Context, spec *v1.AzureBlobStorage, creds *Credentials) (*Bucket, error) {
	containerURL := azureContainerURL(spec)

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
