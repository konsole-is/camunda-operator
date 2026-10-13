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

// Package version holds the version of the operator build. A release build
// sets it from the tag with -ldflags.
package version

import (
	"fmt"

	"github.com/Masterminds/semver/v3"
)

// Dev is the version of a build without a release tag.
const Dev = "0.0.0-dev"

// version is set by -ldflags "-X github.com/konsole-is/camunda-operator/internal/version.version=<version>".
var version = Dev

// Version returns the version of the operator build, for example 0.3.0, or
// Dev for a build without a release tag.
func Version() string {
	return version
}

// InRange reports whether the version of the build is inside rangeExpr, a
// semver range such as ">=0.3.0 <0.5.0". Satisfies gives the rules.
func InRange(rangeExpr string) (bool, error) {
	return Satisfies(version, rangeExpr)
}

// Satisfies reports whether v is inside rangeExpr, with the range syntax of
// github.com/Masterminds/semver/v3. Dev is inside every range that parses. A
// prerelease such as 0.3.0-rc.1 is inside a range only when the range names a
// prerelease, such as ">=0.3.0-0". It returns an error when rangeExpr or v is
// not valid semver.
func Satisfies(v, rangeExpr string) (bool, error) {
	constraint, err := semver.NewConstraint(rangeExpr)
	if err != nil {
		return false, fmt.Errorf("parsing operator version range %q: %w", rangeExpr, err)
	}

	if v == Dev {
		return true, nil
	}

	parsed, err := semver.StrictNewVersion(v)
	if err != nil {
		return false, fmt.Errorf("parsing operator version %q: %w", v, err)
	}

	return constraint.Check(parsed), nil
}
