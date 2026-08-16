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
	"slices"
	"strings"

	corev1 "k8s.io/api/core/v1"
)

// configHashLength is the number of hex characters of the config hash: the
// first 64 bits of the SHA-256 digest.
const configHashLength = 16

// ConfigHash hashes the rendered environment of one process (names, values,
// Secret and field references, never Secret data) and its envFrom sources,
// together with in.HashInputs. It is stable across reconciles for the same
// input, so the pods of a process roll only when a value rendered for that
// process or a referenced object changes.
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
