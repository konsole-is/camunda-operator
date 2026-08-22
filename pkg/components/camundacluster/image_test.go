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

package camundacluster

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestImageTag(t *testing.T) {
	t.Parallel()

	cases := map[string]struct{ image, want string }{
		"plain":              {"camunda/camunda:8.9.9", "8.9.9"},
		"registry with port": {"registry.example.com:5000/camunda/camunda:8.9.9", "8.9.9"},
		"digest after tag":   {"camunda/camunda:8.9.9@sha256:abc", "8.9.9"},
		"no tag":             {"camunda/camunda", ""},
		"port but no tag":    {"registry.example.com:5000/camunda/camunda", ""},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tc.want, ImageTag(tc.image))
		})
	}
}
