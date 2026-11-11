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
	"encoding/json"
	"maps"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/runtime"

	"github.com/konsole-is/camunda-operator/internal/manager"
)

type schemaFixture struct {
	Replicas     *int32                       `json:"replicas,omitempty"`
	Mode         string                       `json:"mode,omitempty"`
	Size         *resource.Quantity           `json:"size,omitempty"`
	Resources    *corev1.ResourceRequirements `json:"resources,omitempty"`
	Labels       map[string]string            `json:"labels,omitempty"`
	Parts        []schemaPart                 `json:"parts,omitempty"`
	Nested       *schemaPart                  `json:"nested,omitempty"`
	ExtraEnvFrom []corev1.EnvFromSource       `json:"extraEnvFrom,omitempty"`
}

type schemaPart struct {
	Name string `json:"name"`
}

// enum returns the enum values of a schema, each as raw JSON.
func enum(values ...string) []apiextensionsv1.JSON {
	out := make([]apiextensionsv1.JSON, 0, len(values))
	for _, v := range values {
		out = append(out, apiextensionsv1.JSON{Raw: []byte(v)})
	}

	return out
}

// envFromSchema is the schema of extraEnvFrom in schemaFixtureSchema, with
// two descriptions.
func envFromSchema() apiextensionsv1.JSONSchemaProps {
	source := object("A source.", map[string]apiextensionsv1.JSONSchemaProps{
		"prefix": {Type: "string"},
	})

	return apiextensionsv1.JSONSchemaProps{
		Type:        "array",
		Description: "Extra environment sources.",
		Items:       &apiextensionsv1.JSONSchemaPropsOrArray{Schema: &source},
	}
}

// schemaFixtureSchema is the schema that controller-gen would write for schemaFixture.
func schemaFixtureSchema() *apiextensionsv1.JSONSchemaProps {
	part := object("", map[string]apiextensionsv1.JSONSchemaProps{"name": leaf("Name of the part.")})
	text := apiextensionsv1.JSONSchemaProps{Type: "string"}
	root := object("", map[string]apiextensionsv1.JSONSchemaProps{
		"replicas": {Type: "integer", Format: "int32", Minimum: new(0.0), Description: "Replicas."},
		"mode":     {Type: "string", Enum: enum(`"Standalone"`, `"Embedded"`)},
		"size": {
			AnyOf:        []apiextensionsv1.JSONSchemaProps{{Type: "integer"}, {Type: "string"}},
			Pattern:      `^\d+$`,
			XIntOrString: true,
		},
		"resources": object("The resources.", map[string]apiextensionsv1.JSONSchemaProps{
			"limits": leaf("Limits."),
		}),
		"labels": {
			Type:                 "object",
			AdditionalProperties: &apiextensionsv1.JSONSchemaPropsOrBool{Schema: &text},
		},
		"parts": {
			Type:  "array",
			Items: &apiextensionsv1.JSONSchemaPropsOrArray{Schema: &part},
		},
		"nested":       part,
		"extraEnvFrom": envFromSchema(),
	})

	return &root
}

func TestWalkSchemaGivesTheTypeOfEachField(t *testing.T) {
	fixture := reflect.TypeFor[schemaFixture]()
	var set schemaSet

	require.NoError(t, walkSchema(&set, "cluster", fixture, schemaFixtureSchema(), fixture.PkgPath()))

	object := &fieldSchema{Type: "object"}
	assert.Equal(t, map[string]fieldSchema{
		"replicas":     {Type: "integer", Format: "int32", Minimum: new(0.0)},
		"mode":         {Type: "string", Enum: []string{"Standalone", "Embedded"}},
		"size":         {IntOrString: true},
		"resources":    {Type: "object"},
		"labels":       {Type: "object"},
		"parts":        {Type: "array", Items: object},
		"nested":       {Type: "object"},
		"nested.name":  {Type: "string"},
		"extraEnvFrom": {Type: "array", Items: object},
	}, set.Fields["cluster"])
}

func TestWalkSchemaKeepsOneFullSchemaWithoutDescriptions(t *testing.T) {
	fixture := reflect.TypeFor[schemaFixture]()
	var set schemaSet

	require.NoError(t, walkSchema(&set, "cluster", fixture, schemaFixtureSchema(), fixture.PkgPath()))

	var full any
	require.NoError(t, json.Unmarshal(set.Full["extraEnvFrom"], &full))
	assert.Equal(t, map[string]any{
		"type": "array",
		"items": map[string]any{
			"type":       "object",
			"properties": map[string]any{"prefix": map[string]any{"type": "string"}},
		},
	}, full)
}

func TestWalkSchemaRefuses(t *testing.T) {
	fixture := reflect.TypeFor[schemaFixture]()
	tests := []struct {
		name    string
		change  func(s *apiextensionsv1.JSONSchemaProps)
		wantErr []string
	}{
		{
			name:    "a property that only the CRD has",
			change:  func(s *apiextensionsv1.JSONSchemaProps) { s.Properties["colour"] = leaf("A colour.") },
			wantErr: []string{"colour", "schemaFixture"},
		},
		{
			name:    "a property name with a dot",
			change:  func(s *apiextensionsv1.JSONSchemaProps) { s.Properties["a.b"] = leaf("A dotted name.") },
			wantErr: []string{"a.b"},
		},
		{
			name: "an enum value that is not a string",
			change: func(s *apiextensionsv1.JSONSchemaProps) {
				mode := s.Properties["mode"]
				mode.Enum = enum("1", "2")
				s.Properties["mode"] = mode
			},
			wantErr: []string{"mode"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			schema := schemaFixtureSchema()
			tt.change(schema)
			var set schemaSet

			err := walkSchema(&set, "cluster", fixture, schema, fixture.PkgPath())
			require.Error(t, err)
			for _, want := range tt.wantErr {
				assert.Contains(t, err.Error(), want)
			}
		})
	}
}

func TestWalkSchemaRefusesTwoFullSchemasOfOneNameThatDiffer(t *testing.T) {
	fixture := reflect.TypeFor[schemaFixture]()
	var set schemaSet
	require.NoError(t, walkSchema(&set, "cluster", fixture, schemaFixtureSchema(), fixture.PkgPath()))

	changed := schemaFixtureSchema()
	envFrom := changed.Properties["extraEnvFrom"]
	envFrom.Items.Schema.Properties["prefix"] = apiextensionsv1.JSONSchemaProps{Type: "integer"}
	changed.Properties["extraEnvFrom"] = envFrom

	err := walkSchema(&set, "optimize", fixture, changed, fixture.PkgPath())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "extraEnvFrom")
}

func TestWalkSchemaKeepsTwoFullSchemasOfOneNameThatDifferOnlyInDescriptions(t *testing.T) {
	fixture := reflect.TypeFor[schemaFixture]()
	var set schemaSet
	require.NoError(t, walkSchema(&set, "cluster", fixture, schemaFixtureSchema(), fixture.PkgPath()))

	changed := schemaFixtureSchema()
	envFrom := changed.Properties["extraEnvFrom"]
	envFrom.Description = "Other words."
	changed.Properties["extraEnvFrom"] = envFrom

	require.NoError(t, walkSchema(&set, "optimize", fixture, changed, fixture.PkgPath()))
}

func TestBuildSchemasDescribesTheClaimFields(t *testing.T) {
	scheme := runtime.NewScheme()
	require.NoError(t, manager.AddToScheme(scheme))

	set, err := buildSchemas("../../config/crd/bases", scheme)
	require.NoError(t, err)

	cluster := set.Fields["cluster"]
	assert.Equal(t, fieldSchema{Type: "integer", Format: "int32", Minimum: new(0.0)}, cluster["zeebe.replicas"])
	assert.Equal(t, []string{"Standalone", "Embedded"}, cluster["gateway.mode"].Enum)
	assert.Equal(t, fieldSchema{IntOrString: true}, cluster["zeebe.storageSize"])
	assert.Equal(t, fieldSchema{Type: "array", Items: &fieldSchema{Type: "object"}}, cluster["extraEnv"])
	for _, path := range []string{"zeebe.scheduling.tolerations", "zeebe.resources", "podLabels"} {
		assert.Contains(t, cluster, path)
	}
	for _, path := range []string{
		"zeebe.resources.requests",
		"zeebe.scheduling.nodeAffinity.requiredDuringSchedulingIgnoredDuringExecution",
	} {
		assert.NotContains(t, cluster, path)
	}

	optimize := set.Fields["optimize"]
	assert.Contains(t, optimize, "importer.replicas")
	assert.Equal(t, fieldSchema{Type: "string"}, optimize["managementAuthConfigRef"])

	assert.ElementsMatch(t, fullSchemaNames, slices.Collect(maps.Keys(set.Full)))
	var nodeAffinity map[string]any
	require.NoError(t, json.Unmarshal(set.Full["nodeAffinity"], &nodeAffinity))
	assert.Contains(t, nodeAffinity["properties"], "requiredDuringSchedulingIgnoredDuringExecution")
	assert.NotContains(t, string(set.Full["nodeAffinity"]), `"description"`)
}

// claimCRD is a CamundaClusterClaim CRD with the given schema of spec.
func claimCRD(spec string) string {
	return `apiVersion: apiextensions.k8s.io/v1
kind: CustomResourceDefinition
metadata:
  name: camundaclusterclaims.fleet.camunda.io
spec:
  group: fleet.camunda.io
  names: {kind: CamundaClusterClaim, plural: camundaclusterclaims}
  scope: Namespaced
  versions:
  - name: v1alpha1
    served: true
    storage: true
    schema: {openAPIV3Schema: {type: object, properties: {spec: ` + spec + `}}}
`
}

func TestBuildSchemasRefuses(t *testing.T) {
	tests := []struct {
		name    string
		spec    string
		wantErr string
	}{
		{
			name:    "a claim without spec.cluster",
			spec:    `{type: object, properties: {attachments: {type: object, properties: {optimize: {type: object}}}}}`,
			wantErr: "spec.cluster",
		},
		{
			name:    "a claim without spec.attachments.optimize",
			spec:    `{type: object, properties: {cluster: {type: object}, attachments: {type: object}}}`,
			wantErr: "spec.attachments.optimize",
		},
		{
			name: "a full schema name that no field has",
			spec: `{type: object, properties: {cluster: {type: object}, ` +
				`attachments: {type: object, properties: {optimize: {type: object}}}}}`,
			wantErr: "extraEnvFrom",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			scheme := runtime.NewScheme()
			require.NoError(t, manager.AddToScheme(scheme))
			dir := t.TempDir()
			require.NoError(t, os.WriteFile(filepath.Join(dir, claimCRDFile), []byte(claimCRD(tt.spec)), 0o644))

			_, err := buildSchemas(dir, scheme)
			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.wantErr)
		})
	}
}

func TestRenderSchemasWritesTheModule(t *testing.T) {
	set := schemaSet{
		Fields: map[string]map[string]fieldSchema{"cluster": {"replicas": {Type: "integer"}}},
		Full:   map[string]json.RawMessage{"extraEnvFrom": json.RawMessage(`{"type":"array"}`)},
	}

	raw, err := renderSchemas(set)
	require.NoError(t, err)

	assert.Equal(t, `// Code generated by hack/tsfielddocs from config/crd/bases and the api types. DO NOT EDIT.
// Run make fleet-generate to update it.

/**
 * FieldSchema is the type of one field of a CamundaClusterClaim, from its
 * CRD schema. intOrString marks a field that takes an integer or a string,
 * such as a quantity. items is the type of the items of a list.
 */
export interface FieldSchema {
  type?: "string" | "integer" | "number" | "boolean" | "object" | "array";
  enum?: string[];
  minimum?: number;
  maximum?: number;
  format?: string;
  intOrString?: boolean;
  items?: FieldSchema;
}

/** SchemaRoot is the part of a CamundaClusterClaim that holds a field: spec.cluster or spec.attachments.optimize. */
export type SchemaRoot = "cluster" | "optimize";

/**
 * FieldSchemas holds the FieldSchema of each field path under each root. A
 * path joins the JSON names with dots. A list, a map, and a type from
 * outside the api module are one field: the walk does not go into them.
 */
export type FieldSchemas = Record<SchemaRoot, Partial<Record<string, FieldSchema>>>;

/** FullSchemaName names a field that the YAML editor checks against its whole CRD schema. */
export type FullSchemaName = "extraEnvFrom" | "nodeAffinity" | "podAffinity";

/**
 * FullSchemas holds the CRD schema of each FullSchemaName, without
 * descriptions. Every field of that name has this schema.
 */
export type FullSchemas = Record<FullSchemaName, Record<string, unknown>>;

export const fieldSchemas: FieldSchemas = {
  "cluster": {
    "replicas": {
      "type": "integer"
    }
  }
};

export const fullSchemas: FullSchemas = {
  "extraEnvFrom": {
    "type": "array"
  }
};
`, string(raw))
}
