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

package camundaconfig

import (
	"strconv"
	"strings"

	corev1 "k8s.io/api/core/v1"
)

// envReplacer applies Spring relaxed binding: a dot becomes an underscore, a
// dash is removed, and a list index "[N]" becomes "_N". The upper-casing
// happens after the replacement.
var envReplacer = strings.NewReplacer(".", "_", "-", "", "[", "_", "]", "")

// Env returns the environment variable name of the key under Spring relaxed
// binding, for example "CAMUNDA_DATA_SECONDARYSTORAGE_TYPE" for
// "camunda.data.secondary-storage.type" and
// "CAMUNDA_SECURITY_INITIALIZATION_USERS_0_USERNAME" for
// "camunda.security.initialization.users[0].username".
func (k Key) Env() string {
	return strings.ToUpper(envReplacer.Replace(string(k)))
}

// Index returns the key of one element of a list key: the list key with the
// index appended, then rest after a dot when rest is not empty. For example
// Index(KeyInitializationUsers, 0, "username") is
// "camunda.security.initialization.users[0].username" and
// Index(KeyDefaultRolesAdminUsers, 0, "") is
// "camunda.security.initialization.default-roles.admin.users[0]".
func Index(list Key, i int, rest string) Key {
	k := string(list) + "[" + strconv.Itoa(i) + "]"
	if rest != "" {
		k += "." + rest
	}
	return Key(k)
}

// Var builds the container environment entry of a key with a literal value.
func Var(k Key, value string) corev1.EnvVar {
	return corev1.EnvVar{Name: k.Env(), Value: value}
}

// VarFrom builds the container environment entry of a key with a value from
// a source, for example a Secret key or a pod field.
func VarFrom(k Key, source *corev1.EnvVarSource) corev1.EnvVar {
	return corev1.EnvVar{Name: k.Env(), ValueFrom: source}
}
