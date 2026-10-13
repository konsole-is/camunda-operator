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

package main

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"

	"github.com/konsole-is/camunda-operator/internal/manager"
)

type fixtureKind struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   fixtureSpec    `json:"spec,omitempty"`
	Status *fixtureStatus `json:"status,omitempty"`
}

type fixtureSpec struct {
	fixtureWorkload `json:",inline"`

	Parts     []fixturePart               `json:"parts,omitempty"`
	Resources corev1.ResourceRequirements `json:"resources,omitempty"`
	Labels    map[string]fixturePart      `json:"labels,omitempty"`
}

type fixtureWorkload struct {
	Replicas int32 `json:"replicas,omitempty"`
}

type fixturePart struct {
	Name string `json:"name"`
}

type fixtureStatus struct {
	Phase string `json:"phase,omitempty"`
}

// object returns an object schema with a description and properties.
func object(description string, properties map[string]apiextensionsv1.JSONSchemaProps) apiextensionsv1.JSONSchemaProps {
	return apiextensionsv1.JSONSchemaProps{Type: "object", Description: description, Properties: properties}
}

// leaf returns a string schema with a description.
func leaf(description string) apiextensionsv1.JSONSchemaProps {
	return apiextensionsv1.JSONSchemaProps{Type: "string", Description: description}
}

// fixtureSchema is the schema that controller-gen would write for fixtureKind.
func fixtureSchema() *apiextensionsv1.JSONSchemaProps {
	part := object("", map[string]apiextensionsv1.JSONSchemaProps{"name": leaf("Name of the part.")})
	root := object("A fixture.", map[string]apiextensionsv1.JSONSchemaProps{
		"apiVersion": leaf("APIVersion."),
		"kind":       leaf("Kind."),
		"metadata":   object("", nil),
		"spec": object("The spec.", map[string]apiextensionsv1.JSONSchemaProps{
			"replicas": leaf("Replicas of the workload."),
			"parts": {
				Type:        "array",
				Description: "The parts.",
				Items:       &apiextensionsv1.JSONSchemaPropsOrArray{Schema: &part},
			},
			"resources": object("The resources.", map[string]apiextensionsv1.JSONSchemaProps{
				"limits": leaf("Limits."),
			}),
			"labels": {
				Type:                 "object",
				Description:          "The labels.",
				AdditionalProperties: &apiextensionsv1.JSONSchemaPropsOrBool{Schema: &part},
			},
		}),
		"status": object("The status.", map[string]apiextensionsv1.JSONSchemaProps{
			"phase": leaf("The phase."),
		}),
	})
	return &root
}

func TestWalkDocumentsEachFieldUnderTheTypeThatHoldsIt(t *testing.T) {
	kind := reflect.TypeOf(fixtureKind{})

	docs, err := walk(kind, fixtureSchema(), kind.PkgPath())
	require.NoError(t, err)

	assert.Equal(t, map[string]fieldDoc{
		"spec":            {Description: "The spec.", Anchor: "#fixturekind"},
		"spec.replicas":   {Description: "Replicas of the workload.", Anchor: "#fixturespec"},
		"spec.parts":      {Description: "The parts.", Anchor: "#fixturespec"},
		"spec.parts.name": {Description: "Name of the part.", Anchor: "#fixturepart"},
		"spec.resources":  {Description: "The resources.", Anchor: "#fixturespec"},
		"spec.labels":     {Description: "The labels.", Anchor: "#fixturespec"},
		"status":          {Description: "The status.", Anchor: "#fixturekind"},
		"status.phase":    {Description: "The phase.", Anchor: "#fixturestatus"},
	}, docs)
}

func TestWalkRefusesAFieldThatTheGoTypeDoesNotHave(t *testing.T) {
	kind := reflect.TypeOf(fixtureKind{})
	schema := fixtureSchema()
	schema.Properties["spec"].Properties["colour"] = leaf("A field that only the CRD has.")

	_, err := walk(kind, schema, kind.PkgPath())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "spec.colour")
	assert.Contains(t, err.Error(), "fixtureSpec")
}

func TestBuildDocumentsCamundaClusterPartitionsUnderZeebeSpec(t *testing.T) {
	scheme := runtime.NewScheme()
	require.NoError(t, manager.AddToScheme(scheme))

	docs, err := build("../../config/crd/bases", scheme)
	require.NoError(t, err)

	partitions := docs["core.camunda.io"]["CamundaCluster"]["spec.zeebe.partitions"]
	assert.Equal(t, "#zeebespec", partitions.Anchor)
	assert.Contains(t, partitions.Description, "Partitions is the number of partitions.")
}

const widgetCRD = `apiVersion: apiextensions.k8s.io/v1
kind: CustomResourceDefinition
metadata:
  name: widgets.example.camunda.io
spec:
  group: example.camunda.io
  names: {kind: Widget, plural: widgets}
  scope: Namespaced
  versions:
  - {name: v1, served: true, storage: true, schema: {openAPIV3Schema: {type: object}}}
`

func TestBuildRefuses(t *testing.T) {
	tests := []struct {
		name    string
		crd     string
		wantErr string
	}{
		{
			name:    "a kind that the scheme of the manager does not have",
			crd:     widgetCRD,
			wantErr: "Widget",
		},
		{
			name: "a CRD that serves two versions",
			crd: widgetCRD +
				"  - {name: v2, served: true, storage: false, schema: {openAPIV3Schema: {type: object}}}\n",
			wantErr: "serves 2 versions",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			require.NoError(t, os.WriteFile(filepath.Join(dir, "widgets.yaml"), []byte(tt.crd), 0o644))

			_, err := build(dir, runtime.NewScheme())
			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.wantErr)
		})
	}
}
