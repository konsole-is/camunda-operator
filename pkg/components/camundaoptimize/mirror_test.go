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
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/konsole-is/camunda-operator/pkg/components/internal/declared"
)

// Every MirrorPurpose constant of mirror.go is in MirrorPurposes, once. A
// constant outside the set gets a name from MirroredSecretName and no Secret
// from the component.
func TestMirrorPurposesCoverEveryConstant(t *testing.T) {
	t.Parallel()

	values := declared.Constants(t, "mirror.go", "MirrorPurpose")
	want := make([]MirrorPurpose, 0, len(values))
	for _, value := range values {
		want = append(want, MirrorPurpose(value))
	}
	assert.NotEmpty(t, want)
	assert.ElementsMatch(t, want, MirrorPurposes)
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
