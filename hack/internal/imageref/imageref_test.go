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

package imageref

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSplit(t *testing.T) {
	t.Parallel()

	repo, tag, err := Split("localhost:5000/cli:1.2.3")
	require.NoError(t, err)
	assert.Equal(t, "localhost:5000/cli", repo)
	assert.Equal(t, "1.2.3", tag)

	_, _, err = Split("ghcr.io/konsole-is/camunda-operator-cli")
	require.Error(t, err)
	_, _, err = Split("ghcr.io/x/cli@sha256:abc")
	require.Error(t, err)
	// An empty repository or tag renders "repo:" or ":tag" into the chart,
	// an invalid image. It must fail here instead.
	_, _, err = Split("ghcr.io/x/cli:")
	require.Error(t, err)
	_, _, err = Split(":1.2.3")
	require.Error(t, err)
}
