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

// LocalCredentialsSecretRef references a username and password pair in a
// Secret in the namespace of the object that holds the reference. Every
// namespaced kind uses it, so a reference never reaches the credentials of
// another namespace.
type LocalCredentialsSecretRef struct {
	// Name is the name of the Secret that holds the credentials.
	// +kubebuilder:validation:MinLength=1
	Name string `json:"name"`
	// UsernameKey is the key in the Secret that holds the plaintext username.
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:default=username
	// +optional
	UsernameKey string `json:"usernameKey,omitempty"`
	// PasswordKey is the key in the Secret that holds the plaintext password.
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:default=password
	// +optional
	PasswordKey string `json:"passwordKey,omitempty"`
}

// LocalSecretKeyRef references one value in a Secret in the namespace of the
// object that holds the reference. Every namespaced kind uses it, so a
// reference never reaches the Secrets of another namespace.
type LocalSecretKeyRef struct {
	// Name is the name of the Secret that holds the value.
	// +kubebuilder:validation:MinLength=1
	Name string `json:"name"`
	// Key is the key in the Secret that holds the value.
	// +kubebuilder:validation:MinLength=1
	Key string `json:"key"`
}

// SecretKeyRef references one value in a Secret in a named namespace. A
// cluster-scoped kind uses it, because it has no namespace of its own. A
// namespaced kind uses LocalSecretKeyRef.
type SecretKeyRef struct {
	// Name is the name of the Secret that holds the value.
	// +kubebuilder:validation:MinLength=1
	Name string `json:"name"`
	// Namespace is the namespace of the Secret.
	// +kubebuilder:validation:MinLength=1
	Namespace string `json:"namespace"`
	// Key is the key in the Secret that holds the value.
	// +kubebuilder:validation:MinLength=1
	Key string `json:"key"`
}

// VolumeStatus is the observed size of one data PersistentVolumeClaim of a
// cluster. A status lists one entry for each bound claim, sorted by name.
// Thus it also shows a resize of one claim outside the spec, for example by
// an auto-resize controller.
type VolumeStatus struct {
	// Name is the name of the PersistentVolumeClaim.
	Name string `json:"name"`
	// Capacity is the storage capacity that the bound claim reports.
	Capacity resource.Quantity `json:"capacity"`
}
