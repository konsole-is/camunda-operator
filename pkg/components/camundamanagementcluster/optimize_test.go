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

package camundamanagementcluster

import (
	"testing"

	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	v1 "github.com/konsole-is/camunda-operator/api/v1"
)

// optimizeAt returns a CamundaOptimize with the given identity and URL.
func optimizeAt(namespace, name, url string) v1.CamundaOptimize {
	return v1.CamundaOptimize{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: namespace},
		Spec:       v1.CamundaOptimizeSpec{ExternalURL: url},
	}
}

func TestAttachedOptimizes(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		discovered []v1.CamundaOptimize
		want       []v1.AttachedOptimizeStatus
	}{
		{
			name: "orders by namespace and then by name",
			discovered: []v1.CamundaOptimize{
				optimizeAt("green", "b", "https://gb.example.com"),
				optimizeAt("blue", "b", "https://bb.example.com"),
				optimizeAt("green", "a", "https://ga.example.com"),
			},
			want: []v1.AttachedOptimizeStatus{
				{Namespace: "blue", Name: "b", ExternalURL: "https://bb.example.com"},
				{Namespace: "green", Name: "a", ExternalURL: "https://ga.example.com"},
				{Namespace: "green", Name: "b", ExternalURL: "https://gb.example.com"},
			},
		},
		{
			name: "leaves out an Optimize that names no URL",
			discovered: []v1.CamundaOptimize{
				optimizeAt("blue", "a", ""),
				optimizeAt("blue", "b", "https://bb.example.com"),
			},
			want: []v1.AttachedOptimizeStatus{
				{Namespace: "blue", Name: "b", ExternalURL: "https://bb.example.com"},
			},
		},
		{
			name:       "no Optimize is no row",
			discovered: nil,
			want:       []v1.AttachedOptimizeStatus{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, tt.want, AttachedOptimizes(tt.discovered))
		})
	}
}

func TestOptimizeURLs(t *testing.T) {
	t.Parallel()

	rows := []v1.AttachedOptimizeStatus{
		{Namespace: "blue", Name: "a", ExternalURL: "https://blue.example.com"},
		{Namespace: "green", Name: "a", ExternalURL: "https://green.example.com"},
	}

	tests := []struct {
		name string
		spec *v1.ManagementOptimizeSpec
		rows []v1.AttachedOptimizeStatus
		want []string
	}{
		{
			name: "the spec entry comes first",
			spec: &v1.ManagementOptimizeSpec{ExternalURL: "https://spec.example.com"},
			rows: rows,
			want: []string{
				"https://spec.example.com", "https://blue.example.com", "https://green.example.com",
			},
		},
		{
			name: "a row that repeats the spec entry is dropped",
			spec: &v1.ManagementOptimizeSpec{ExternalURL: "https://blue.example.com"},
			rows: rows,
			want: []string{"https://blue.example.com", "https://green.example.com"},
		},
		{
			name: "an unset spec entry leaves the rows alone",
			rows: rows,
			want: []string{"https://blue.example.com", "https://green.example.com"},
		},
		{
			name: "no spec entry and no row is no URL",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			mc := &v1.CamundaManagementCluster{
				Spec: v1.CamundaManagementClusterSpec{Optimize: tt.spec},
			}

			assert.Equal(t, tt.want, OptimizeURLs(mc, tt.rows))
		})
	}
}

// The callback is the URL and the path, with nothing removed and nothing
// added, in the order of the URL list. Management Identity concatenates the
// two the same way, so both writers produce the same entry.
func TestOptimizeCallbacks(t *testing.T) {
	t.Parallel()

	in := Input{OptimizeURLs: []string{
		"https://one.example.com",
		"https://three.example.com/optimize",
	}}

	assert.Equal(
		t, []string{
			"https://one.example.com/api/authentication/callback",
			"https://three.example.com/optimize/api/authentication/callback",
		}, OptimizeCallbacks(in),
	)
}

func TestOptimizeCallbacksWithoutURLs(t *testing.T) {
	t.Parallel()

	assert.Empty(t, OptimizeCallbacks(Input{}))
}
