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
	"regexp"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const exampleGroupInfo = `// +groupName=example.camunda.io
package v1
`

const exampleConditions = `package v1

const (
	ConditionReady = "Ready"
	ConditionGearsReady = "GearsReady"
	ReasonHealthy = "Healthy"
)
`

const widgetTypes = `package v1

// +kubebuilder:object:root=true
type Widget struct{}

// +kubebuilder:object:root=true
type WidgetList struct{}

const (
	ConditionGearsReady ConditionType = "GearsReady"
	ConditionBeltReady = "BeltReady"
	ReasonJammed = "Jammed"
)
`

// gadgetTypes sets the root marker off from the doc comment with a blank
// line, as the real types files do.
const gadgetTypes = `package v1

// +kubebuilder:object:root=true

// Gadget is a kind without own condition types.
type Gadget struct{}
`

// writeAPI writes the files of each group package into a new api directory
// and returns that directory.
func writeAPI(t *testing.T, packages map[string]map[string]string) string {
	api := t.TempDir()
	for dir, files := range packages {
		require.NoError(t, os.MkdirAll(filepath.Join(api, dir), 0o755))
		for name, src := range files {
			require.NoError(t, os.WriteFile(filepath.Join(api, dir, name), []byte(src), 0o644))
		}
	}
	return api
}

// exampleAPI is one group with shared types, a kind with own types, and a
// kind without own types.
func exampleAPI(t *testing.T) string {
	return writeAPI(t, map[string]map[string]string{
		"v1": {
			"groupversion_info.go": exampleGroupInfo,
			"conditions.go":        exampleConditions,
			"widget_types.go":      widgetTypes,
			"gadget_types.go":      gadgetTypes,
		},
	})
}

// set returns the elements of the const array name in out.
func set(t *testing.T, out []byte, name string) []string {
	block := regexp.MustCompile(`(?s)export const ` + name + ` = \[\n(.*?)\] as const;`).FindSubmatch(out)
	require.NotNil(t, block, "no const array %s in:\n%s", name, out)
	var types []string
	for _, m := range regexp.MustCompile(`"([^"]+)"`).FindAllSubmatch(block[1], -1) {
		types = append(types, string(m[1]))
	}
	return types
}

func TestGenerateGivesEachKindItsOwnAndTheSharedTypes(t *testing.T) {
	out, err := generate(exampleAPI(t))
	require.NoError(t, err)

	assert.Equal(t, []string{"GearsReady", "Ready"}, set(t, out, "exampleSharedConditionTypes"))
	assert.Equal(t, []string{"BeltReady", "GearsReady", "Ready"}, set(t, out, "exampleWidgetConditionTypes"))
	assert.Equal(t, []string{"GearsReady", "Ready"}, set(t, out, "exampleGadgetConditionTypes"))
	assert.Contains(t, string(out),
		"export type ExampleWidgetConditionType = (typeof exampleWidgetConditionTypes)[number];")
}

func TestGenerateGivesReadyToAKindOfAGroupThatDoesNotDeclareIt(t *testing.T) {
	api := writeAPI(t, map[string]map[string]string{
		"other/v1alpha1": {
			"groupversion_info.go": "// +groupName=other.camunda.io\npackage v1alpha1\n",
			"conditions.go":        "package v1alpha1\n\nconst ConditionBeltReady = \"BeltReady\"\n",
			"gizmo_types.go":       "package v1alpha1\n\n// +kubebuilder:object:root=true\ntype Gizmo struct{}\n",
		},
	})

	out, err := generate(api)
	require.NoError(t, err)

	assert.Equal(t, []string{"BeltReady", "Ready"}, set(t, out, "otherGizmoConditionTypes"))
	assert.Equal(t, []string{"BeltReady"}, set(t, out, "otherSharedConditionTypes"))
}

func TestGenerateLeavesOutListTypesReasonsAndTestFiles(t *testing.T) {
	api := exampleAPI(t)
	require.NoError(t, os.WriteFile(
		filepath.Join(api, "v1", "widget_types_test.go"),
		[]byte("package v1\n\nconst ConditionTestOnly = \"TestOnly\"\n"),
		0o644,
	))

	out, err := generate(api)
	require.NoError(t, err)

	assert.NotContains(t, string(out), "WidgetList")
	assert.NotContains(t, string(out), "Jammed")
	assert.NotContains(t, string(out), "TestOnly")
}

func TestGenerateKeepsTheGroupOfATypeThatTwoGroupsDeclare(t *testing.T) {
	api := writeAPI(t, map[string]map[string]string{
		"v1": {"groupversion_info.go": exampleGroupInfo, "widget_types.go": widgetTypes},
		"other/v1alpha1": {
			"groupversion_info.go": "// +groupName=other.camunda.io\npackage v1alpha1\n",
			"conditions.go":        "package v1alpha1\n\nconst ConditionBeltReady = \"BeltReady\"\n",
		},
	})

	out, err := generate(api)
	require.NoError(t, err)

	assert.Contains(t, set(t, out, "exampleWidgetConditionTypes"), "BeltReady")
	assert.Equal(t, []string{"BeltReady"}, set(t, out, "otherSharedConditionTypes"))
}

func TestGenerateShowsANewConditionConstant(t *testing.T) {
	api := exampleAPI(t)
	before, err := generate(api)
	require.NoError(t, err)

	again, err := generate(api)
	require.NoError(t, err)
	assert.Equal(t, before, again, "two runs on the same input must write the same bytes")

	require.NoError(t, os.WriteFile(
		filepath.Join(api, "v1", "gadget_types.go"),
		[]byte(gadgetTypes+"\nconst ConditionSprocketReady = \"SprocketReady\"\n"),
		0o644,
	))
	after, err := generate(api)
	require.NoError(t, err)

	assert.NotEqual(t, before, after)
	assert.Contains(t, set(t, after, "exampleGadgetConditionTypes"), "SprocketReady")
}

func TestGenerateRefuses(t *testing.T) {
	tests := []struct {
		name    string
		files   map[string]string
		wantErr string
	}{
		{
			name: "a condition type in a file that declares no kind",
			files: map[string]string{
				"groupversion_info.go": exampleGroupInfo,
				"shared.go":            "package v1\n\nconst ConditionBeltReady = \"BeltReady\"\n",
			},
			wantErr: "shared.go",
		},
		{
			name: "a condition type in a file that declares two kinds",
			files: map[string]string{
				"groupversion_info.go": exampleGroupInfo,
				"both_types.go":        widgetTypes + "\n// +kubebuilder:object:root=true\ntype Gadget struct{}\n",
			},
			wantErr: "both_types.go",
		},
		{
			name: "a condition type that is not a string literal",
			files: map[string]string{
				"groupversion_info.go": exampleGroupInfo,
				"conditions.go":        "package v1\n\nconst ConditionBeltReady = prefix + \"Ready\"\n",
			},
			wantErr: "ConditionBeltReady",
		},
		{
			name:    "a group package without a groupName marker",
			files:   map[string]string{"groupversion_info.go": "package v1\n"},
			wantErr: "+groupName",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := generate(writeAPI(t, map[string]map[string]string{"v1": tt.files}))
			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.wantErr)
		})
	}
}

func TestGenerateRefusesTwoGroupsWithTheSamePrefix(t *testing.T) {
	api := writeAPI(t, map[string]map[string]string{
		"v1":       {"groupversion_info.go": exampleGroupInfo},
		"v1alpha1": {"groupversion_info.go": "// +groupName=example.camunda.io\npackage v1alpha1\n"},
	})

	_, err := generate(api)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "example")
}

func TestGenerateReadsTheConditionTypesOfTheRealAPI(t *testing.T) {
	out, err := generate("../../api")
	require.NoError(t, err)

	cluster := set(t, out, "coreCamundaClusterConditionTypes")
	assert.Subset(t, cluster, []string{"Ready", "MirroredSecretsReady", "ZeebeReady", "AdminSecretReady"})
	assert.Subset(t, set(t, out, "coreDatabaseServerConditionTypes"), []string{"ClusterReady"})
	assert.Subset(t, set(t, out, "pluginsSharedConditionTypes"), []string{"Accepted", "Applied"})
}
