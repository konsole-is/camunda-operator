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

// Package camundaconfig declares every unified-configuration key, Spring
// profile, and plain environment variable that the operator sets on a
// Camunda 8.9 container. Each declaration carries a comment that names the
// source that declares it: a file and line of the camunda/camunda repository
// at tag 8.9.9, or a path in the Camunda Helm chart 14.8.3. A key without a
// source does not exist here, and the renderer in pkg/components/camundacluster
// emits only keys from this package. Spring ignores an environment variable
// that it does not recognize, so a wrong key fails silently at runtime. This
// package is the place where a key is checked once, against its source.
//
// The package also converts a dotted key to its environment variable name
// under Spring relaxed binding: upper case, a dot becomes an underscore, a
// dash is removed, and a list index "[N]" becomes "_N".
package camundaconfig
