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

import corev1 "k8s.io/api/core/v1"

// RetainedVersion returns the highest broker version that the
// BrokerVersionAnnotation stamps on claims, or the empty string when no
// claim carries a well-formed one. It is the running version of a cluster
// recreated on retained volumes, which has no StatefulSet to read one from.
func RetainedVersion(claims []corev1.PersistentVolumeClaim) string {
	var highest string
	for i := range claims {
		stamped := claims[i].Annotations[BrokerVersionAnnotation]
		if _, err := parseVersion(stamped); err != nil {
			continue
		}
		if highest == "" || VersionDowngrade(highest, stamped) {
			highest = stamped
		}
	}

	return highest
}

// VersionDowngrade reports whether effective is below running, segment by
// segment. It reports false when either value is not of the form x.y.z. A
// cluster without a running version has nothing to move back from.
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
