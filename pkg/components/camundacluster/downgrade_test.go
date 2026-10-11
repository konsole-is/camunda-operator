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
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestVersionDowngrade(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		effective, running string
		want               bool
	}{
		"patch below":           {"8.9.8", "8.9.9", true},
		"minor below":           {"8.9.9", "8.10.0", true},
		"major below":           {"8.10.0", "9.0.0", true},
		"same version":          {"8.9.9", "8.9.9", false},
		"minor above":           {"8.10.0", "8.9.9", false},
		"no running version":    {"8.9.8", "", false},
		"running tag not x.y.z": {"8.9.8", "latest", false},
		"effective not x.y.z":   {"8.9", "8.9.9", false},
		"numeric, not lexical":  {"8.9.10", "8.9.9", false},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tc.want, VersionDowngrade(tc.effective, tc.running))
		})
	}
}

func TestVersionSkip(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		effective, running string
		want               bool
	}{
		"a patch move":                 {"8.9.10", "8.9.9", false},
		"the same version":             {"8.9.9", "8.9.9", false},
		"a one-minor move":             {"8.10.0", "8.9.9", false},
		"a two-minor move":             {"8.11.0", "8.9.9", true},
		"a three-minor move":           {"8.12.3", "8.9.0", true},
		"a downgrade":                  {"8.8.0", "8.9.9", false},
		"a change of major":            {"9.0.0", "8.9.9", false},
		"a change of major, low minor": {"9.11.0", "8.9.9", false},
		"no running version":           {"8.11.0", "", false},
		"running tag not x.y.z":        {"8.11.0", "latest", false},
		"effective not x.y.z":          {"8.11", "8.9.9", false},
		"numeric, not lexical":         {"8.10.0", "8.9.9", false},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tc.want, VersionSkip(tc.effective, tc.running))
		})
	}
}

func TestNextMinor(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		version, want string
	}{
		"a patch of 8.9":       {"8.9.9", "8.10"},
		"the first of a minor": {"8.10.0", "8.11"},
		"not x.y.z":            {"latest", ""},
		"empty":                {"", ""},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tc.want, NextMinor(tc.version))
		})
	}
}

// stampedClaims returns one claim per version, each carrying it as the
// broker version annotation.
func stampedClaims(versions ...string) []corev1.PersistentVolumeClaim {
	claims := make([]corev1.PersistentVolumeClaim, 0, len(versions))
	for _, version := range versions {
		claims = append(claims, corev1.PersistentVolumeClaim{
			ObjectMeta: metav1.ObjectMeta{
				Annotations: map[string]string{BrokerVersionAnnotation: version},
			},
		})
	}
	return claims
}

func TestRetainedVersion(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		claims []corev1.PersistentVolumeClaim
		want   string
	}{
		"no claims":                    {nil, ""},
		"a claim without the stamp":    {[]corev1.PersistentVolumeClaim{{}}, ""},
		"one stamped claim":            {stampedClaims("8.9.9"), "8.9.9"},
		"the highest stamp, numeric":   {stampedClaims("8.9.8", "8.9.10", "8.9.9"), "8.9.10"},
		"a malformed stamp is skipped": {stampedClaims("latest", "8.9.9"), "8.9.9"},
		"only malformed stamps":        {stampedClaims("latest", "8.9"), ""},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tc.want, RetainedVersion(tc.claims))
		})
	}
}
