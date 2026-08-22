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

import "strings"

// VersionDowngrade reports whether effective is below running, segment by
// segment. It reports false when either value is not of the form x.y.z: a
// cluster without a running version has nothing to move back from, and an
// effective version that is not x.y.z is refused by ValidateMerged before
// this rule runs.
func VersionDowngrade(effective, running string) bool {
	want, err := parseVersion(effective)
	if err != nil {
		return false
	}
	have, err := parseVersion(running)
	if err != nil {
		return false
	}

	for i := range 3 {
		switch {
		case want[i] < have[i]:
			return true
		case want[i] > have[i]:
			return false
		}
	}

	return false
}

// ImageTag returns the tag of an image reference, or the empty string when
// it carries none. A digest follows the tag after an "@", and a registry
// host can carry a port, so only a colon after the last slash and before the
// digest starts a tag.
func ImageTag(image string) string {
	if at := strings.Index(image, "@"); at >= 0 {
		image = image[:at]
	}

	colon := strings.LastIndex(image, ":")
	if colon < 0 || colon < strings.LastIndex(image, "/") {
		return ""
	}

	return image[colon+1:]
}
