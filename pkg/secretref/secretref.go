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

// Package secretref checks the Secret references named by contract CRDs and
// digests the data they point to.
package secretref

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"maps"
	"slices"
	"strings"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// CheckKeys reports whether the Secret at ref exists and contains every key.
// It returns a condition-ready failure message when the Secret is missing or
// lacks a key, an error only for transient API failures, and ("", nil) when
// all keys are present. Pass an uncached reader: callers watch Secrets
// metadata-only, so data must be read live.
func CheckKeys(ctx context.Context, reader client.Reader, ref types.NamespacedName, keys ...string) (string, error) {
	_, msg, err := Get(ctx, reader, ref, keys...)
	return msg, err
}

// Get is CheckKeys for a caller that also needs the Secret: it returns the
// Secret when every key is present, or a nil Secret and the failure message
// of CheckKeys. It returns an error only for transient API failures.
func Get(
	ctx context.Context,
	reader client.Reader,
	ref types.NamespacedName,
	keys ...string,
) (*corev1.Secret, string, error) {
	var secret corev1.Secret
	if err := reader.Get(ctx, ref, &secret); err != nil {
		if apierrors.IsNotFound(err) {
			return nil, fmt.Sprintf("Secret %s not found", ref), nil
		}
		return nil, "", err
	}

	for _, key := range keys {
		if _, ok := secret.Data[key]; !ok {
			return nil, fmt.Sprintf("Secret %s is missing key %q", ref, key), nil
		}
	}

	return &secret, "", nil
}

// DataDigest returns a hash input for the values of keys in secret: 16 hex
// characters that move when the value of a named key changes, and stay when
// only the metadata or an unnamed key changes. No keys means every key of the
// Secret. The digest never carries a value in clear.
func DataDigest(secret *corev1.Secret, keys ...string) string {
	if len(keys) == 0 {
		keys = slices.Collect(maps.Keys(secret.Data))
	}
	keys = slices.Clone(keys)
	slices.Sort(keys)
	keys = slices.Compact(keys)

	// A Secret key cannot hold "=" or a newline, so each line maps to one
	// key and the digest of its value.
	var b strings.Builder
	for _, key := range keys {
		value := sha256.Sum256(secret.Data[key])
		b.WriteString(key + "=" + hex.EncodeToString(value[:]) + "\n")
	}

	sum := sha256.Sum256([]byte(b.String()))
	return hex.EncodeToString(sum[:])[:16]
}
