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

package camundaoptimize

import (
	"go/ast"
	"go/parser"
	"go/token"
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Every MirrorPurpose constant of mirror.go is in MirrorPurposes, once. A
// constant outside the set gets a name from MirroredSecretName and no Secret
// from the component.
func TestMirrorPurposesCoverEveryConstant(t *testing.T) {
	t.Parallel()

	want := declaredPurposes(t)
	assert.NotEmpty(t, want)
	assert.ElementsMatch(t, want, MirrorPurposes)
}

// declaredPurposes returns the value of every MirrorPurpose constant of
// mirror.go, read from the source, so the test sees a constant that no slice
// names. A constant declared without the type on its own spec is not found.
func declaredPurposes(t *testing.T) []MirrorPurpose {
	t.Helper()

	parsed, err := parser.ParseFile(token.NewFileSet(), "mirror.go", nil, 0)
	require.NoError(t, err)

	var purposes []MirrorPurpose
	for _, decl := range parsed.Decls {
		gen, ok := decl.(*ast.GenDecl)
		if !ok || gen.Tok != token.CONST {
			continue
		}
		for _, spec := range gen.Specs {
			value, ok := spec.(*ast.ValueSpec)
			if !ok {
				continue
			}
			ident, ok := value.Type.(*ast.Ident)
			if !ok || ident.Name != "MirrorPurpose" {
				continue
			}
			for _, expr := range value.Values {
				lit, ok := expr.(*ast.BasicLit)
				require.True(t, ok, "constant %s is not a literal", value.Names[0].Name)
				unquoted, err := strconv.Unquote(lit.Value)
				require.NoError(t, err)
				purposes = append(purposes, MirrorPurpose(unquoted))
			}
		}
	}

	return purposes
}

func TestMirrorPurposeValid(t *testing.T) {
	t.Parallel()

	for _, purpose := range MirrorPurposes {
		assert.True(t, purpose.Valid(), purpose)
	}
	assert.False(t, MirrorPurpose("").Valid())
	assert.False(t, MirrorPurpose("licence").Valid())
}

func TestMirroredSecretComponentRejectsUnknownPurpose(t *testing.T) {
	t.Parallel()

	_, err := MirroredSecretComponent(fixtureMinimal(t).Optimize, map[MirrorPurpose]map[string][]byte{
		"licence": {"license": []byte("license")},
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), `purpose "licence" is not in MirrorPurposes`)
}
