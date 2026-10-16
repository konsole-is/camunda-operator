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
	"bytes"
	"compress/gzip"
	"encoding/base64"
	"io"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const release = `{
  "name": "camunda-operator",
  "manifest": "kind: CustomResourceDefinition\n"
}
`

func TestSecretDataDecodesToTheCompactRelease(t *testing.T) {
	t.Parallel()

	data, err := secretData([]byte(release))
	require.NoError(t, err)

	zipped, err := base64.StdEncoding.DecodeString(string(data))
	require.NoError(t, err)
	zr, err := gzip.NewReader(bytes.NewReader(zipped))
	require.NoError(t, err)
	decoded, err := io.ReadAll(zr)
	require.NoError(t, err)
	assert.JSONEq(t, release, string(decoded))
	assert.Equal(t, `{"name":"camunda-operator","manifest":"kind: CustomResourceDefinition\n"}`, string(decoded))
}

func TestSecretDataDoesNotCountWhitespace(t *testing.T) {
	t.Parallel()

	indented, err := secretData([]byte(release))
	require.NoError(t, err)
	compact, err := secretData([]byte(`{"name":"camunda-operator","manifest":"kind: CustomResourceDefinition\n"}`))
	require.NoError(t, err)
	assert.Equal(t, compact, indented)
}

func TestSecretDataRejectsInputThatIsNotAnObject(t *testing.T) {
	t.Parallel()

	for _, input := range []string{"", "Error: INSTALLATION FAILED", `["a"]`} {
		_, err := secretData([]byte(input))
		assert.Error(t, err, "input %q", input)
	}
}

func TestRunPassesAtTheMaxAndFailsAboveIt(t *testing.T) {
	t.Parallel()

	data, err := secretData([]byte(release))
	require.NoError(t, err)

	require.NoError(t, run(strings.NewReader(release), len(data), "at the max"))

	err = run(strings.NewReader(release), len(data)-1, "above the max")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "above the max")
}
