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

package controller

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"sigs.k8s.io/yaml"
)

// The JSONPaths of the three columns that every kind reporting Ready prints.
const (
	readyStatusPath = `.status.conditions[?(@.type=="Ready")].status`
	readyReasonPath = `.status.conditions[?(@.type=="Ready")].reason`
	creationPath    = ".metadata.creationTimestamp"
)

// phaseFirstKinds are the one-shot kinds that run once and stop. Their
// status.phase is the outcome, and their Ready condition restates it, so the
// phase leads the row and Ready gets no column of its own.
var phaseFirstKinds = map[string]bool{
	"LogicalBackupElasticsearch":  true,
	"LogicalBackupRDBMS":          true,
	"LogicalRestoreElasticsearch": true,
	"LogicalRestoreRDBMS":         true,
	"PointInTimeRestore":          true,
}

// crdBase is the part of a generated CRD that carries the printer columns.
type crdBase struct {
	Spec struct {
		Names struct {
			Kind string `json:"kind"`
		} `json:"names"`
		Versions []crdVersion `json:"versions"`
	} `json:"spec"`
}

// crdVersion is one served version of a CRD.
type crdVersion struct {
	Name                     string          `json:"name"`
	AdditionalPrinterColumns []printerColumn `json:"additionalPrinterColumns"`
	Schema                   struct {
		OpenAPIV3Schema jsonSchema `json:"openAPIV3Schema"`
	} `json:"schema"`
}

// printerColumn is one entry of additionalPrinterColumns. A column of a
// priority above zero appears under kubectl get -o wide only.
type printerColumn struct {
	Name     string `json:"name"`
	Type     string `json:"type"`
	JSONPath string `json:"jsonPath"`
	Priority int32  `json:"priority"`
}

// jsonSchema reads the property tree of an OpenAPI schema and nothing else.
type jsonSchema struct {
	Properties map[string]jsonSchema `json:"properties"`
}

// A kind whose row does not open with the state it reports sends every reader
// to a second command against every object. The columns live in a marker on
// the Go type and reach a cluster through the generated CRD alone, so this
// test reads the generated CRDs.
func TestEveryCRDPrintsTheStateItReports(t *testing.T) {
	t.Parallel()

	bases, err := filepath.Glob(filepath.Join("..", "..", "config", "crd", "bases", "*.yaml"))
	require.NoError(t, err)
	require.NotEmpty(t, bases)

	for _, base := range bases {
		data, err := os.ReadFile(base)
		require.NoError(t, err, base)

		var crd crdBase
		require.NoError(t, yaml.Unmarshal(data, &crd), base)

		for _, version := range crd.Spec.Versions {
			t.Run(crd.Spec.Names.Kind+"/"+version.Name, func(t *testing.T) {
				columns := version.AdditionalPrinterColumns
				require.NotEmpty(t, columns, "the kind prints no columns")

				age := columns[len(columns)-1]
				assert.Equal(t, "Age", age.Name)
				assert.Equal(t, creationPath, age.JSONPath)
				assert.Equal(t, "date", age.Type)

				switch {
				case phaseFirstKinds[crd.Spec.Names.Kind]:
					assert.Equal(t, "Phase", columns[0].Name)
				case version.reportsConditions():
					assertReadyLeads(t, columns)
				}
			})
		}
	}
}

// assertReadyLeads asserts the uniform shape: Ready and its reason open the
// row, and at most one column of the default width stands between the reason
// and the age.
func assertReadyLeads(t *testing.T, columns []printerColumn) {
	t.Helper()

	require.GreaterOrEqual(t, len(columns), 3)
	assert.Equal(t, "Ready", columns[0].Name)
	assert.Equal(t, readyStatusPath, columns[0].JSONPath)
	assert.Equal(t, "Reason", columns[1].Name)
	assert.Equal(t, readyReasonPath, columns[1].JSONPath)

	var wide int
	for _, column := range columns[2 : len(columns)-1] {
		if column.Priority == 0 {
			wide++
		}
	}

	assert.LessOrEqual(t, wide, 1, "at most one column stands between Reason and Age")
}

// reportsConditions reports whether the status of the version carries
// conditions, which is what a Ready column reads.
func (v crdVersion) reportsConditions() bool {
	status, ok := v.Schema.OpenAPIV3Schema.Properties["status"]
	if !ok {
		return false
	}

	_, ok = status.Properties["conditions"]

	return ok
}
