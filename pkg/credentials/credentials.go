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

// Package credentials generates the passwords that the controllers publish in
// credential Secrets. It also reads them back, so a password stays stable
// after creation, and it binds the republished value to the Secret it came
// from, so a delete of the Secret always rotates the password.
package credentials

import (
	"context"
	"crypto/rand"
	"fmt"
	"math/big"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// passwordAlphabet spans the alphanumeric characters that every supported
// backend accepts without quoting or encoding.
const passwordAlphabet = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"

// passwordLength is the generated password length: 32 alphanumeric characters,
// which is just under 191 bits of entropy.
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

// Password is a resolved credential: the value to publish in a Secret, and the
// identity of the Secret the value was read from.
//
// A reused value belongs to one object. The apply that republishes it must
// update that object and must never create a new one, because a new object
// would hold the old password and no later reconcile could see the difference.
// PreconditionAnnotations turns SourceUID into the apply precondition that
// NewApplyClient enforces.
type Password struct {
	// Value is the password to publish under the credential key.
	Value string
	// SourceUID is the UID of the Secret that Value was read from. It is empty
	// when no Secret held a password and Value is new.
	SourceUID types.UID
}

// PreconditionAnnotations returns the annotations that bind an apply of the
// credential Secret to the object that Value came from. A new password has no
// source object and gets no precondition, so the apply is free to create the
// Secret. NewApplyClient turns the annotation into metadata.uid on the patch,
// so it never reaches the cluster.
func (p Password) PreconditionAnnotations() map[string]string {
	if p.SourceUID == "" {
		return nil
	}

	return map[string]string{PreconditionAnnotation: string(p.SourceUID)}
}

// LookupOrNew returns the password stored under field in the Secret at key,
// together with the UID of that Secret. If the Secret is absent, or the field
// is absent or empty, it returns a new password and no UID: an empty value
// is never a credential, so it is replaced, not kept. Controllers call it
// with an uncached
// reader before they publish a credential Secret. A password then stays stable
// after creation, and to rotate it you delete the Secret.
//
// The returned UID is what keeps that rotation contract through a concurrent
// delete: the component that publishes the Secret must stamp
// PreconditionAnnotations on it, and the controller must reconcile through
// NewApplyClient. It returns an error only for a transient API failure or an
// exhausted entropy source.
func LookupOrNew(
	ctx context.Context,
	r client.Reader,
	key client.ObjectKey,
	field string,
) (Password, error) {
	password, err := lookup(ctx, r, key, field)
	if err != nil || password.Value != "" {
		return password, err
	}

	value, err := NewPassword()
	if err != nil {
		return Password{}, err
	}

	return Password{Value: value}, nil
}

// lookup reads field from the Secret at key. A missing Secret, a missing
// field, and an empty value are all the zero Password: an empty password is
// never a credential to keep. It returns an error only for a transient API
// failure.
func lookup(ctx context.Context, r client.Reader, key client.ObjectKey, field string) (Password, error) {
	var secret corev1.Secret
	if err := r.Get(ctx, key, &secret); err != nil {
		if apierrors.IsNotFound(err) {
			return Password{}, nil
		}
		return Password{}, fmt.Errorf("reading Secret %q: %w", key, err)
	}

	value, ok := secret.Data[field]
	if !ok || len(value) == 0 {
		return Password{}, nil
	}

	return Password{Value: string(value), SourceUID: secret.UID}, nil
}
