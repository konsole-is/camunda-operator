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

package databaseserver

import (
	"testing"

	cnpgv1 "github.com/cloudnative-pg/api/pkg/api/v1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	v1 "github.com/konsole-is/camunda-operator/api/v1"
)

// A refusal keeps a running server on the major it has. The two states that
// must not refuse are the ones a server passes through on its way up: no
// cluster yet, and a cluster that has not reported the major of its data
// directory.
func TestRefusedMajorChange(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		version string
		running *cnpgv1.ImageInfo
		refuses bool
	}{
		{
			name:    "the same major",
			version: "17",
			running: &cnpgv1.ImageInfo{MajorVersion: 17},
		},
		{
			name:    "a higher major",
			version: "18",
			running: &cnpgv1.ImageInfo{MajorVersion: 17},
			refuses: true,
		},
		{
			name:    "a lower major",
			version: "16",
			running: &cnpgv1.ImageInfo{MajorVersion: 17},
			refuses: true,
		},
		{
			name:    "no major reported yet",
			version: "18",
		},
		{
			name:    "an empty major reported",
			version: "18",
			running: &cnpgv1.ImageInfo{Image: "ghcr.io/cloudnative-pg/postgresql:18"},
		},
		{
			name:    "a version that does not parse",
			version: "seventeen",
			running: &cnpgv1.ImageInfo{MajorVersion: 17},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			refused := refusedMajorChange(tt.version, tt.running)
			if !tt.refuses {
				assert.Nil(t, refused)
				return
			}

			require.NotNil(t, refused)
			assert.Equal(t, v1.ReasonVersionChangeRefused, refused.Reason)
			assert.Contains(t, refused.Message, tt.version)
			assert.Contains(t, refused.Message, "17")
		})
	}
}
