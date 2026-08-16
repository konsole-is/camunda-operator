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

// Package secretref checks the Secret references named by contract CRDs.
package secretref

import (
	"context"
	"fmt"

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
			return nil, fmt.Sprintf("Secret %q not found", ref), nil
		}
		return nil, "", err
	}

	for _, key := range keys {
		if _, ok := secret.Data[key]; !ok {
			return nil, fmt.Sprintf("Secret %q is missing key %q", ref, key), nil
		}
	}

	return &secret, "", nil
}
