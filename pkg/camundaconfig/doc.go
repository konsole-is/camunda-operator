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
// Camunda 8.9 container. Each declaration names the configuration class or
// file of the camunda/camunda repository that declares it, and
// keys_source_test.go proves the declared keys against a local checkout of
// that repository (set CAMUNDA_SOURCE_DIR to run it). The renderer in
// pkg/components/camundacluster emits only keys from this package. Spring
// ignores an environment variable that it does not recognize, so a wrong key
// fails silently at runtime. This package is the place where a key is
// checked once, against its source.
//
// The package also converts a dotted key to its environment variable name
// under Spring relaxed binding: upper case, a dot becomes an underscore, a
// dash is removed, and a list index "[N]" becomes "_N".
package camundaconfig
