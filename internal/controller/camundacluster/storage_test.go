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
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"

	components "github.com/konsole-is/camunda-operator/pkg/components/camundacluster"
)

// brokerStorageRunning returns the storage of a cluster whose applied broker
// StatefulSet runs the given image.
func brokerStorageRunning(image string) brokerStorage {
	return brokerStorageWithContainers(corev1.Container{Name: components.ContainerCamunda, Image: image})
}

// brokerStorageWithContainers returns the storage of a cluster whose applied
// broker StatefulSet carries the given containers.
func brokerStorageWithContainers(containers ...corev1.Container) brokerStorage {
	return brokerStorage{statefulSet: &appsv1.StatefulSet{
		Spec: appsv1.StatefulSetSpec{
			Template: corev1.PodTemplateSpec{Spec: corev1.PodSpec{Containers: containers}},
		},
	}}
}

func TestRunningVersion(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		storage brokerStorage
		want    string
	}{
		{
			name:    "no StatefulSet before the first apply",
			storage: brokerStorage{},
			want:    "",
		},
		{
			name:    "no camunda container",
			storage: brokerStorageWithContainers(corev1.Container{Name: "sidecar", Image: "camunda/camunda:8.9.9"}),
			want:    "",
		},
		{
			name:    "no containers at all",
			storage: brokerStorageWithContainers(),
			want:    "",
		},
		{
			name:    "untagged image",
			storage: brokerStorageRunning("camunda/camunda"),
			want:    "",
		},
		{
			name:    "tagged image",
			storage: brokerStorageRunning("camunda/camunda:8.9.9"),
			want:    "8.9.9",
		},
		{
			name:    "tagged image behind a registry with a port",
			storage: brokerStorageRunning("registry.example.com:5000/camunda/camunda:8.9.9"),
			want:    "8.9.9",
		},
		{
			name: "the camunda container is not the first one",
			storage: brokerStorageWithContainers(
				corev1.Container{Name: "sidecar", Image: "other:1.0.0"},
				corev1.Container{Name: components.ContainerCamunda, Image: "camunda/camunda:8.9.9"},
			),
			want: "8.9.9",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tt.want, tt.storage.runningVersion())
		})
	}
}
