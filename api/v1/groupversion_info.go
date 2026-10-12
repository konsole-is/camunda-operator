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

// Package v1 contains API Schema definitions for the core v1 API group.
// +kubebuilder:object:generate=true
// +groupName=core.camunda.io
package v1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

var (
	// SchemeGroupVersion is group version used to register these objects.
	// Applyconfiguration generators, for example controller-gen, use this name.
	SchemeGroupVersion = schema.GroupVersion{Group: "core.camunda.io", Version: "v1"}

	// GroupVersion is an alias for SchemeGroupVersion, for backward compatibility.
	GroupVersion = SchemeGroupVersion

	// SchemeBuilder is used to add go types to the GroupVersionKind scheme.
	SchemeBuilder = NewBuilder(SchemeGroupVersion)

	// AddToScheme adds the types in this group-version to the given scheme.
	AddToScheme = SchemeBuilder.AddToScheme
)

// Builder registers the Go types of one group version with a runtime.Scheme.
// It is a small stand-in for the controller-runtime scheme builder, so this
// module does not depend on controller-runtime.
// +kubebuilder:object:generate=false
type Builder struct {
	runtime.SchemeBuilder

	groupVersion schema.GroupVersion
}

// NewBuilder returns a Builder that registers types under gv.
func NewBuilder(gv schema.GroupVersion) *Builder {
	return &Builder{groupVersion: gv}
}

// Register adds objects to the builder under its group version. Each type
// file calls it from init, so the result is the full set of kinds of the group.
func (b *Builder) Register(objects ...runtime.Object) *Builder {
	b.SchemeBuilder.Register(func(s *runtime.Scheme) error {
		s.AddKnownTypes(b.groupVersion, objects...)
		metav1.AddToGroupVersion(s, b.groupVersion)
		return nil
	})

	return b
}

// Build returns a new scheme that holds every registered type.
func (b *Builder) Build() (*runtime.Scheme, error) {
	s := runtime.NewScheme()

	return s, b.AddToScheme(s)
}
