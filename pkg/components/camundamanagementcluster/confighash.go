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

// unhashedEnv are the rendered environment entries that ConfigHash leaves out,
// so that a change to them alone does not roll the pods.
//
// The Optimize root URLs are the whole list. Management Identity reads them on
// its first start against Keycloak and writes the callbacks of the list onto
// the optimize client, and the operator writes the same callbacks through the
// administration API on every reconcile. A new Optimize therefore reaches
// Keycloak without a restart of Management Identity, and rolling Identity for
// it would sign every user out for nothing. The variable stays in the rendered
// environment, because a Management Identity that starts for another reason
// must find the whole list there.
//
// KEYCLOAK_INIT_OPTIMIZE_SECRET stays in the hash. It is a credential, and a
// rotated one has to reach the container.
var unhashedEnv = map[string]bool{keycloakEnvInitOptimizeRootURL: true}

// ConfigHash hashes the rendered environment of one component (names, values,
// and Secret references, never Secret data) together with in.HashInputs and
// the hash inputs of that component alone. It is stable across reconciles for
// the same input, so the pods roll only when a value rendered for that
// component or a referenced object changes. The entries of unhashedEnv are
// left out, so a change to one of them alone rolls nothing.
//
// The environment alone is not enough. Every credential arrives through a
// Secret reference, and the reference does not change when the data behind it
// does. HashInputs carries the resource version of every Secret the
// controller read, and componentInputs carries what one component alone
// reads, the credentials that the operator generates itself included.
func ConfigHash(in Input, comp string) string {
	var b strings.Builder
	b.WriteString("component=" + comp + "\n")
	for _, e := range componentEnv(in, comp) {
		if unhashedEnv[e.Name] {
			continue
		}
		b.WriteString(e.Name + "=" + envValue(e) + "\n")
	}

	inputs := slices.Concat(in.HashInputs, componentInputs(in, comp))
	slices.Sort(inputs)
	for _, input := range inputs {
		b.WriteString("input=" + input + "\n")
	}

	sum := sha256.Sum256([]byte(b.String()))

	return hex.EncodeToString(sum[:])[:configHashLength]
}

// componentEnv returns the rendered environment of a component. A component
// that this package renders no container for, Keycloak and the Secrets, has
// none.
func componentEnv(in Input, comp string) []corev1.EnvVar {
	switch comp {
	case ComponentIdentity:
		return identityEnv(in)
	case ComponentConsole:
		return consoleEnv(in)
	case ComponentWebModelerRestapi:
		return webModelerRestapiEnv(in)
	case ComponentWebModelerWebsockets:
		return webModelerWebsocketsEnv(in)
	}

	return nil
}

// componentInputs returns the hash inputs of one component alone: what the
// controller resolved under that component, the credentials that Management
// Identity creates the clients and the first user with, and the pusher
// credentials of the two Web Modeler processes. A credential that one
// component alone reads belongs here rather than in in.HashInputs, so that
// rotating it rolls the pods that read it and no others.
//
// The operator writes these Secrets itself, so the source of a reused
// credential is what a regeneration changes: the Secret is deleted, the next
// reconcile finds none, and the new one has a UID of its own. The very first
// reconcile renders no source, because the Secret does not exist yet, so the
// pods roll once more when the reconcile after it reads the created Secret
// back. Every credential is looked up on its own, so a Secret that holds one
// of two keys keeps that UID for it.
func componentInputs(in Input, comp string) []string {
	inputs := slices.Clone(in.ComponentHashInputs[comp])
	switch comp {
	case ComponentIdentity:
		for _, gen := range generatedSecrets(in) {
			if !gen.published {
				continue
			}
			inputs = append(inputs, "generated="+gen.name+"="+string(in.Secrets.Values[gen.name].SourceUID))
		}
	case ComponentWebModelerRestapi, ComponentWebModelerWebsockets:
		inputs = append(
			inputs,
			"pusherKey="+string(in.Pusher.Key.SourceUID),
			"pusherSecret="+string(in.Pusher.Secret.SourceUID),
		)
	}

	return inputs
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
