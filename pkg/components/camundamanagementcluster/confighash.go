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

package camundamanagementcluster

import (
	"crypto/sha256"
	"encoding/hex"
	"slices"
	"strings"

	corev1 "k8s.io/api/core/v1"
)

// configHashLength is the number of hex characters of the config hash: the
// first 64 bits of the SHA-256 digest.
const configHashLength = 16

// ConfigHash hashes the rendered environment of one component (names, values,
// and Secret references, never Secret data) together with in.HashInputs and
// the digest of every generated credential that the component reads. It is
// stable across reconciles for the same input, so the pods roll only when a
// value rendered for that component or a credential it reads changes.
//
// The environment alone is not enough. Every credential arrives through a
// Secret reference, and the reference does not change when the data behind it
// does. HashInputs carries the resource version of every Secret the
// controller read, and the digests carry the credentials that the operator
// itself generates, which no reference records either.
//
// A generated credential and the in.ComponentInputs entry of a component
// enter the hash of that component and of no other. A change to either rolls
// that component alone.
func ConfigHash(in Input, comp string) string {
	var b strings.Builder
	b.WriteString("component=" + comp + "\n")
	for _, e := range componentEnv(in, comp) {
		b.WriteString(e.Name + "=" + envValue(e) + "\n")
	}

	inputs := slices.Concat(in.HashInputs, in.ComponentInputs[comp])
	slices.Sort(inputs)
	for _, input := range inputs {
		b.WriteString("input=" + input + "\n")
	}

	for _, digest := range componentCredentialHashes(in, comp) {
		b.WriteString("credential=" + digest + "\n")
	}

	sum := sha256.Sum256([]byte(b.String()))

	return hex.EncodeToString(sum[:])[:configHashLength]
}

// componentEnv returns the rendered environment of a component. A component
// this package does not render yet has none.
func componentEnv(in Input, comp string) []corev1.EnvVar {
	if comp == ComponentIdentity {
		return baseEnv(in)
	}

	return nil
}

// componentCredentialHashes returns the digest of every generated credential
// that a component reads, in the order the Secrets are rendered. Management
// Identity reads all of them: it creates the clients and the first user with
// them on its first start.
func componentCredentialHashes(in Input, comp string) []string {
	if comp != ComponentIdentity {
		return nil
	}

	var digests []string
	for _, gen := range generatedSecrets(in) {
		if digest := passwordHash(in.Secrets.Values[gen.name].Value); digest != "" {
			digests = append(digests, digest)
		}
	}

	return digests
}

// passwordHash returns the hash input of a credential: the first 64 bits of
// its SHA-256 digest, hex encoded. An empty value returns "", so a management
// cluster without the credential adds no input. The digest of a generated
// password, which carries 191 bits of entropy, does not expose the password.
func passwordHash(value string) string {
	if value == "" {
		return ""
	}

	sum := sha256.Sum256([]byte(value))

	return hex.EncodeToString(sum[:])[:configHashLength]
}

// envValue renders the value of an environment entry as a reference, never as
// Secret data.
func envValue(e corev1.EnvVar) string {
	switch {
	case e.ValueFrom == nil:
		return e.Value
	case e.ValueFrom.SecretKeyRef != nil:
		return "secretKeyRef:" + e.ValueFrom.SecretKeyRef.Name + "/" + e.ValueFrom.SecretKeyRef.Key
	case e.ValueFrom.ConfigMapKeyRef != nil:
		return "configMapKeyRef:" + e.ValueFrom.ConfigMapKeyRef.Name + "/" + e.ValueFrom.ConfigMapKeyRef.Key
	default:
		return ""
	}
}
