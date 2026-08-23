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
	"time"

	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	v1 "github.com/konsole-is/camunda-operator/api/v1"
)

func TestOlderThan(t *testing.T) {
	earlier := metav1.NewTime(time.Unix(1000, 0))
	later := metav1.NewTime(time.Unix(2000, 0))

	cluster := func(namespace, name string, created metav1.Time) *v1.CamundaCluster {
		return &v1.CamundaCluster{
			ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: namespace, CreationTimestamp: created},
		}
	}

	cases := map[string]struct {
		a, b     *v1.CamundaCluster
		want     bool
		wantBack bool
	}{
		"earlier timestamp wins regardless of name": {
			a: cluster("ns", "z", earlier), b: cluster("ns", "a", later),
			want: true, wantBack: false,
		},
		"same timestamp, smaller name wins": {
			a: cluster("ns", "a", earlier), b: cluster("ns", "b", earlier),
			want: true, wantBack: false,
		},
		"same timestamp and name, smaller namespace wins": {
			a: cluster("ns-a", "cluster", earlier), b: cluster("ns-b", "cluster", earlier),
			want: true, wantBack: false,
		},
		"identical objects are not older than each other": {
			a: cluster("ns", "cluster", earlier), b: cluster("ns", "cluster", earlier),
			want: false, wantBack: false,
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			assert.Equal(t, tc.want, olderThan(tc.a, tc.b))
			assert.Equal(t, tc.wantBack, olderThan(tc.b, tc.a))
		})
	}
}
