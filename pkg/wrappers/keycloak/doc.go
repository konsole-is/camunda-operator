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

// Package keycloak holds the Go types of the Keycloak custom resource of the
// Keycloak Operator, k8s.keycloak.org/v2alpha1, and the ocf primitive that
// reconciles it. The Keycloak project publishes no Go module for its CRDs, so
// the types here carry the fields that the operator sets and reads. The schema
// they follow is vendored in internal/testenv/crds/keycloak.
//
// v2alpha1 is deprecated since Keycloak 26.7 in favor of v2beta1. It is the
// only version that every supported 26.x Keycloak Operator serves.
//
// Each type in types.go carries the object:generate marker of its own. The
// marker of the whole package would also reach the wrapper, whose mutation
// type names an ocf interface that controller-gen cannot copy.
// +groupName=k8s.keycloak.org
package keycloak
