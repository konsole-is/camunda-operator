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

package images

import (
	"testing"

	"github.com/stretchr/testify/assert"

	v1 "github.com/konsole-is/camunda-operator/api/v1"
)

func TestResolveDefaults(t *testing.T) {
	tests := []struct {
		image Image
		want  string
	}{
		{Camunda, "camunda/camunda:8.9.9"},
		{Connectors, "camunda/connectors-bundle:8.9.9"},
		{Optimize, "camunda/optimize:8.9.9"},
		{Identity, "camunda/identity:8.9.9"},
		{Console, "camunda/console:8.9.9"},
		{WebModelerRestapi, "camunda/web-modeler-restapi:8.9.9"},
		{WebModelerWebsockets, "camunda/web-modeler-websockets:8.9.9"},
	}
	for _, tt := range tests {
		t.Run(string(tt.image), func(t *testing.T) {
			assert.Equal(t, tt.want, Resolve(nil, tt.image, "8.9.9"))
			assert.Equal(t, tt.want, Resolve(&v1.CamundaPlatformConfigSpec{}, tt.image, "8.9.9"))
		})
	}
}

func TestResolveWebModelerRestapiByVersion(t *testing.T) {
	tests := []struct {
		name    string
		version string
		want    string
	}{
		{name: "below the boundary", version: "8.9.9", want: "camunda/web-modeler-restapi:8.9.9"},
		{name: "at the boundary", version: "8.10.0", want: "camunda/hub:8.10.0"},
		{name: "above the boundary", version: "9.0.0", want: "camunda/hub:9.0.0"},
		{name: "one segment", version: "8", want: "camunda/web-modeler-restapi:8"},
		{name: "not a version", version: "latest", want: "camunda/web-modeler-restapi:latest"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, Resolve(nil, WebModelerRestapi, tt.version))
		})
	}
}

func TestResolveWebModelerRestapiOverrideWinsFromHub(t *testing.T) {
	spec := &v1.CamundaPlatformConfigSpec{
		Images: &v1.ImagesSpec{WebModelerRestapi: "mirror.example.com/team/restapi"},
	}

	assert.Equal(t, "mirror.example.com/team/restapi:8.10.0", Resolve(spec, WebModelerRestapi, "8.10.0"))
}

func TestResolveKeycloakTag(t *testing.T) {
	assert.Equal(t, "camunda/keycloak:quay-optimized-26.0.7", Resolve(nil, Keycloak, "26.0.7"))
}

func TestResolveRegistry(t *testing.T) {
	tests := []struct {
		name     string
		registry string
		want     string
	}{
		{name: "no registry", registry: "", want: "camunda/optimize:8.9.9"},
		{name: "registry", registry: "registry.example.com", want: "registry.example.com/camunda/optimize:8.9.9"},
		{
			name:     "registry with a trailing slash",
			registry: "registry.example.com/",
			want:     "registry.example.com/camunda/optimize:8.9.9",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			spec := &v1.CamundaPlatformConfigSpec{ImageRegistry: tt.registry}
			assert.Equal(t, tt.want, Resolve(spec, Optimize, "8.9.9"))
		})
	}
}

func TestResolveOverrideWinsOverRegistry(t *testing.T) {
	spec := &v1.CamundaPlatformConfigSpec{
		ImageRegistry: "registry.example.com",
		Images:        &v1.ImagesSpec{Optimize: "mirror.example.com/team/optimize"},
	}

	assert.Equal(t, "mirror.example.com/team/optimize:8.9.9", Resolve(spec, Optimize, "8.9.9"))
	// The override applies to one image only.
	assert.Equal(t, "registry.example.com/camunda/identity:8.9.9", Resolve(spec, Identity, "8.9.9"))
}

func TestResolveOverridePerImage(t *testing.T) {
	tests := []struct {
		image  Image
		images v1.ImagesSpec
	}{
		{Camunda, v1.ImagesSpec{Camunda: "mirror.example.com/camunda"}},
		{Connectors, v1.ImagesSpec{Connectors: "mirror.example.com/connectors"}},
		{Optimize, v1.ImagesSpec{Optimize: "mirror.example.com/optimize"}},
		{Identity, v1.ImagesSpec{Identity: "mirror.example.com/identity"}},
		{Console, v1.ImagesSpec{Console: "mirror.example.com/console"}},
		{WebModelerRestapi, v1.ImagesSpec{WebModelerRestapi: "mirror.example.com/restapi"}},
		{WebModelerWebsockets, v1.ImagesSpec{WebModelerWebsockets: "mirror.example.com/websockets"}},
		{Keycloak, v1.ImagesSpec{Keycloak: "mirror.example.com/keycloak"}},
	}
	for _, tt := range tests {
		t.Run(string(tt.image), func(t *testing.T) {
			spec := &v1.CamundaPlatformConfigSpec{Images: &tt.images}
			got := Resolve(spec, tt.image, "1.2.3")
			assert.Contains(t, got, "mirror.example.com/")
			assert.NotContains(t, got, "camunda/")
		})
	}
}

func TestResolveOverrideTrailingSlash(t *testing.T) {
	spec := &v1.CamundaPlatformConfigSpec{
		Images: &v1.ImagesSpec{Optimize: "mirror.example.com/team/optimize/"},
	}

	assert.Equal(t, "mirror.example.com/team/optimize:8.9.9", Resolve(spec, Optimize, "8.9.9"))
}

// Every declared image must resolve to a repository. A constant without one
// would render a reference that starts with a colon.
func TestEveryImageHasARepository(t *testing.T) {
	all := []Image{
		Camunda, Connectors, Optimize, Identity,
		Console, WebModelerRestapi, WebModelerWebsockets, Keycloak,
	}
	assert.Len(t, repositories, len(all))
	for _, img := range all {
		assert.NotEmpty(t, repositories[img], "image %q has no repository", img)
	}
}
