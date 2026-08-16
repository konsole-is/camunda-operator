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

package v1

import "k8s.io/apimachinery/pkg/api/resource"

// CredentialsSecretRef references a username/password pair stored in a Secret.
// Namespace is required so that references stay uniform and explicit across
// all contract kinds, cluster-scoped and namespaced alike. The reference may
// name a Secret in any namespace; RBAC on the kinds embedding it governs who
// may create the referencing objects.
type CredentialsSecretRef struct {
	// Name of the Secret holding the credentials.
	// +kubebuilder:validation:MinLength=1
	Name string `json:"name"`
	// Namespace of the Secret.
	// +kubebuilder:validation:MinLength=1
	Namespace string `json:"namespace"`
	// UsernameKey is the key in the Secret holding the plaintext username.
	// +kubebuilder:validation:MinLength=1
	UsernameKey string `json:"usernameKey"`
	// PasswordKey is the key in the Secret holding the plaintext password.
	// +kubebuilder:validation:MinLength=1
	PasswordKey string `json:"passwordKey"`
}

// SecretKeyRef references a single value inside a Secret.
// Namespace is required so that references stay uniform and explicit across
// all contract kinds, cluster-scoped and namespaced alike. The reference may
// name a Secret in any namespace; RBAC on the kinds embedding it governs who
// may create the referencing objects.
type SecretKeyRef struct {
	// Name of the Secret holding the value.
	// +kubebuilder:validation:MinLength=1
	Name string `json:"name"`
	// Namespace of the Secret.
	// +kubebuilder:validation:MinLength=1
	Namespace string `json:"namespace"`
	// Key in the Secret holding the value.
	// +kubebuilder:validation:MinLength=1
	Key string `json:"key"`
}

// VolumeStatus is the observed size of one data PersistentVolumeClaim of a
// cluster. A status lists one entry per bound claim, sorted by name, so a
// resize of a single claim outside the spec, for example by an auto-resize
// controller, shows here.
type VolumeStatus struct {
	// Name is the name of the PersistentVolumeClaim.
	Name string `json:"name"`
	// Capacity is the storage capacity that the bound claim reports.
	Capacity resource.Quantity `json:"capacity"`
}
