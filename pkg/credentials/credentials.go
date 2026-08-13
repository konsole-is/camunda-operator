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

// Package credentials generates the passwords the storage backend controllers
// publish in credential Secrets and reads them back for stable-once-created
// semantics.
package credentials

import (
	"context"
	"crypto/rand"
	"fmt"
	"math/big"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// passwordAlphabet spans the alphanumeric characters every supported backend
// accepts without quoting or encoding concerns.
const passwordAlphabet = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"

// passwordLength is the generated password length: 32 alphanumeric characters,
// just under 191 bits of entropy.
const passwordLength = 32

// NewPassword returns a new 32-character alphanumeric password drawn from
// crypto/rand.
func NewPassword() (string, error) {
	password := make([]byte, passwordLength)
	for i := range password {
		n, err := rand.Int(rand.Reader, big.NewInt(int64(len(passwordAlphabet))))
		if err != nil {
			return "", fmt.Errorf("generating password: %w", err)
		}
		password[i] = passwordAlphabet[n.Int64()]
	}

	return string(password), nil
}

// Lookup reads field from the Secret at key and reports whether it was found.
// A missing Secret or a missing field is ("", false, nil); an error is
// returned only for transient API failures. Controllers call it with an
// uncached reader before generating a new password, so a password stays
// stable once created and deleting the Secret is the rotation mechanism.
func Lookup(ctx context.Context, r client.Reader, key client.ObjectKey, field string) (string, bool, error) {
	var secret corev1.Secret
	if err := r.Get(ctx, key, &secret); err != nil {
		if apierrors.IsNotFound(err) {
			return "", false, nil
		}
		return "", false, fmt.Errorf("reading Secret %q: %w", key, err)
	}

	value, ok := secret.Data[field]
	if !ok {
		return "", false, nil
	}

	return string(value), true, nil
}
