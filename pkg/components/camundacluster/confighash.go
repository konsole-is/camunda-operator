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

package camundacluster

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"slices"
	"strings"

	corev1 "k8s.io/api/core/v1"

	v1 "github.com/konsole-is/camunda-operator/api/v1"
)

// configHashLength is the number of hex characters of the config hash: the
// first 64 bits of the SHA-256 digest.
const configHashLength = 16

// ConfigHash hashes the rendered environment of one process (names, values,
// Secret and field references, never Secret data) and its envFrom sources,
// together with in.HashInputs. It is stable across reconciles for the same
// input, so the pods of a process roll only when a value rendered for that
// process or a referenced object changes.
//
// The hash of connectors also takes in.AdminPasswordHash, the digest of the
// admin password that the admin Secret publishes. Connectors authenticate
// every call with that password at runtime, so they must restart when it
// changes. The unified processes read it once, as the create-once initial
// user seed, so a rotation does not restart the brokers.
func ConfigHash(in Input, p Process) string {
	return configHash(in, p, render(in, p))
}

// configHash is ConfigHash for an already rendered process.
func configHash(in Input, p Process, r rendered) string {
	var b strings.Builder
	b.WriteString("component=" + p.Component + "\n")
	for _, e := range r.env {
		b.WriteString(e.Name + "=" + envValue(e) + "\n")
	}
	for _, source := range r.envFrom {
		b.WriteString("envFrom=" + envFromValue(source) + "\n")
	}

	inputs := slices.Clone(in.HashInputs)
	slices.Sort(inputs)
	for _, input := range inputs {
		b.WriteString("input=" + input + "\n")
	}

	if p.Component == ComponentConnectors && in.AdminPasswordHash != "" {
		b.WriteString("adminPassword=" + in.AdminPasswordHash + "\n")
	}

	sum := sha256.Sum256([]byte(b.String()))
	return hex.EncodeToString(sum[:])[:configHashLength]
}

// envValue renders the value of an environment entry as a reference, never
// as Secret data.
func envValue(e corev1.EnvVar) string {
	switch {
	case e.ValueFrom == nil:
		return e.Value
	case e.ValueFrom.SecretKeyRef != nil:
		return "secretKeyRef:" + e.ValueFrom.SecretKeyRef.Name + "/" + e.ValueFrom.SecretKeyRef.Key
	case e.ValueFrom.ConfigMapKeyRef != nil:
		return "configMapKeyRef:" + e.ValueFrom.ConfigMapKeyRef.Name + "/" + e.ValueFrom.ConfigMapKeyRef.Key
	case e.ValueFrom.FieldRef != nil:
		return "fieldRef:" + e.ValueFrom.FieldRef.FieldPath
	case e.ValueFrom.ResourceFieldRef != nil:
		return "resourceFieldRef:" + e.ValueFrom.ResourceFieldRef.Resource
	default:
		return ""
	}
}

// envFromValue renders an envFrom source as its kind, name, and prefix.
func envFromValue(source corev1.EnvFromSource) string {
	switch {
	case source.SecretRef != nil:
		return "secretRef:" + source.SecretRef.Name + ":" + source.Prefix
	case source.ConfigMapRef != nil:
		return "configMapRef:" + source.ConfigMapRef.Name + ":" + source.Prefix
	default:
		return "prefix:" + source.Prefix
	}
}

// PasswordHash returns the hash input for a credential value: the first 64
// bits of its SHA-256 digest, hex encoded. An empty value returns "", so a
// cluster without the credential adds no input. The digest of a generated
// password (191 bits of entropy) does not expose the password.
func PasswordHash(value string) string {
	if value == "" {
		return ""
	}

	sum := sha256.Sum256([]byte(value))
	return hex.EncodeToString(sum[:])[:configHashLength]
}

// PresetFingerprint is the hash input that stands for a CamundaClusterPreset:
// a digest of its spec with the fields that no process renders removed. The
// controller records it instead of the generation of the preset, because the
// generation moves for every spec change, and the fingerprint of a preset is
// an input of every process hash.
//
// The whole of spec.cluster.auth.basic is what this exists for, and neither
// of its fields renders into a pod template. passwordRotation requests a
// rotation of the admin password, and only connectors follow that password,
// through Input.AdminPasswordHash. adminEmail travels to the processes
// through the admin Secret. Neither may restart a workload, so neither may
// reach the fingerprint. Every other change of a preset still moves it and
// rolls the workloads that inherit it.
func PresetFingerprint(spec v1.CamundaClusterPresetSpec) (string, error) {
	rendered := spec.DeepCopy()
	// The wrappers go with the value. The first rotation of a preset adds
	// the blocks that carry it, and a fingerprint that saw an empty basic or
	// auth block appear would roll every workload once for that rotation.
	if auth := rendered.Cluster.Auth; auth != nil {
		if auth.Basic != nil {
			auth.Basic.PasswordRotation = ""
			auth.Basic.AdminEmail = ""
			if *auth.Basic == (v1.BasicAuthSpec{}) {
				auth.Basic = nil
			}
		}
		if *auth == (v1.ClusterAuthSpec{}) {
			rendered.Cluster.Auth = nil
		}
	}

	encoded, err := json.Marshal(rendered)
	if err != nil {
		return "", fmt.Errorf("encoding the preset spec: %w", err)
	}

	sum := sha256.Sum256(encoded)
	return hex.EncodeToString(sum[:])[:configHashLength], nil
}
