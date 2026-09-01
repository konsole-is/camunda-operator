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

// Package images resolves the container images that the operator pulls. Every
// component asks this package instead of writing a repository of its own, so
// the platform config governs all of them the same way.
package images

import (
	"strconv"
	"strings"

	v1 "github.com/konsole-is/camunda-operator/api/v1"
)

// Image names one container image that the operator pulls.
type Image string

// The images that the operator pulls. The name of each one matches the field
// of spec.images that renames it.
const (
	// Camunda is the unified image of every orchestration cluster process.
	Camunda Image = "camunda"
	// Connectors is the connectors runtime.
	Connectors Image = "connectors"
	// Optimize is Optimize.
	Optimize Image = "optimize"
	// Identity is Management Identity.
	Identity Image = "identity"
	// Console is Console.
	Console Image = "console"
	// WebModelerRestapi is the Web Modeler restapi process.
	WebModelerRestapi Image = "web-modeler-restapi"
	// WebModelerWebsockets is the Web Modeler websockets process.
	WebModelerWebsockets Image = "web-modeler-websockets"
	// Keycloak is the Camunda build of Keycloak.
	Keycloak Image = "keycloak"
	// Postgres is the PostgreSQL that a DatabaseServer runs.
	Postgres Image = "postgres"
)

// keycloakTagPrefix is what Camunda publishes its Keycloak build under. The
// tag is quay-optimized-<version>, not the bare version
// (https://docs.camunda.io/docs/self-managed/deployment/helm/configure/operator-based-infrastructure/).
const keycloakTagPrefix = "quay-optimized-"

// repositories holds the default repository of each image. The sources are
// camunda.Dockerfile for the unified image, the values.yaml of the 8.9 Helm
// chart for the other Camunda images, and the CloudNativePG image catalog for
// PostgreSQL.
var repositories = map[Image]string{
	Camunda:              "camunda/camunda",
	Connectors:           "camunda/connectors-bundle",
	Optimize:             "camunda/optimize",
	Identity:             "camunda/identity",
	Console:              "camunda/console",
	WebModelerRestapi:    "camunda/web-modeler-restapi",
	WebModelerWebsockets: "camunda/web-modeler-websockets",
	Keycloak:             "camunda/keycloak",
	Postgres:             "ghcr.io/cloudnative-pg/postgresql",
}

// hubRepositories holds the repository that replaces the default from 8.10
// on. Camunda renames the two Web Modeler images to the Hub product in that
// release
// (https://github.com/camunda/camunda-platform-helm/blob/main/charts/camunda-platform-8.10/README.md).
var hubRepositories = map[Image]string{
	WebModelerRestapi:    "camunda/hub",
	WebModelerWebsockets: "camunda/hub-websockets",
}

// Resolve returns the reference of img at version: the repository that
// spec.images renames it to, or the default repository, with version as the
// tag. p can be nil, which resolves the default repository. A default
// repository can change with the version, so a caller that resolves one image
// for two versions can get two repositories.
func Resolve(p *v1.CamundaPlatformConfigSpec, img Image, version string) string {
	tag := version
	if img == Keycloak {
		tag = keycloakTagPrefix + version
	}

	if repo := strings.TrimRight(override(p, img), "/"); repo != "" {
		return repo + ":" + tag
	}

	return defaultRepository(img, version) + ":" + tag
}

// override returns the repository that spec.images renames img to, or empty
// when the platform config renames nothing.
func override(p *v1.CamundaPlatformConfigSpec, img Image) string {
	if p == nil || p.Images == nil {
		return ""
	}

	switch img {
	case Camunda:
		return p.Images.Camunda
	case Connectors:
		return p.Images.Connectors
	case Optimize:
		return p.Images.Optimize
	case Identity:
		return p.Images.Identity
	case Console:
		return p.Images.Console
	case WebModelerRestapi:
		return p.Images.WebModelerRestapi
	case WebModelerWebsockets:
		return p.Images.WebModelerWebsockets
	case Keycloak:
		return p.Images.Keycloak
	case Postgres:
		return p.Images.Postgres
	}

	return ""
}

// defaultRepository returns the repository that Camunda publishes img under at
// version.
func defaultRepository(img Image, version string) string {
	if repo, ok := hubRepositories[img]; ok && atLeastMinor(version, 8, 10) {
		return repo
	}

	return repositories[img]
}

// atLeastMinor reports whether version is major.minor or later. A version that
// does not start with two numbers reports false, so a malformed version keeps
// the oldest repository.
func atLeastMinor(version string, major, minor int) bool {
	segments := strings.Split(version, ".")
	if len(segments) < 2 {
		return false
	}

	gotMajor, err := strconv.Atoi(segments[0])
	if err != nil {
		return false
	}

	gotMinor, err := strconv.Atoi(segments[1])
	if err != nil {
		return false
	}

	return gotMajor > major || (gotMajor == major && gotMinor >= minor)
}
