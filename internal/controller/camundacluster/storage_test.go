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
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

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
		{
			name:    "retained claims without a StatefulSet",
			storage: brokerStorage{claims: []corev1.PersistentVolumeClaim{stampedClaim("pvc-0", "8.9.9")}},
			want:    "8.9.9",
		},
		{
			name: "the StatefulSet wins over the retained stamp",
			storage: brokerStorage{
				statefulSet: brokerStorageRunning("camunda/camunda:8.9.9").statefulSet,
				claims:      []corev1.PersistentVolumeClaim{stampedClaim("pvc-0", "8.9.8")},
			},
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

// stampedClaim returns a bound broker claim that carries version as the
// broker version annotation. An empty version leaves the claim unstamped.
func stampedClaim(name, version string) corev1.PersistentVolumeClaim {
	claim := corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: "ns"},
	}
	if version != "" {
		claim.Annotations = map[string]string{components.BrokerVersionAnnotation: version}
	}
	return claim
}

func TestStampBrokerVersion(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		// image is the broker image of the applied StatefulSet. Empty means
		// that no StatefulSet exists.
		image string
		// stamped is the annotation value the claim carries before the call.
		stamped string
		// want is the annotation value on the live claim after the call.
		want string
		// patched is true when the call must write the live claim.
		patched bool
	}{
		{
			name:  "stamps an unstamped claim",
			image: "camunda/camunda:8.9.9",
			want:  "8.9.9", patched: true,
		},
		{
			name:  "restamps a claim of an earlier version",
			image: "camunda/camunda:8.9.9", stamped: "8.9.8",
			want: "8.9.9", patched: true,
		},
		{
			name:  "leaves a current stamp alone",
			image: "camunda/camunda:8.9.9", stamped: "8.9.9",
			want: "8.9.9",
		},
		{
			name:    "writes nothing without a StatefulSet",
			stamped: "8.9.9",
			want:    "8.9.9",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			scheme := runtime.NewScheme()
			require.NoError(t, clientgoscheme.AddToScheme(scheme))

			claim := stampedClaim("data-cc-zeebe-0", tt.stamped)
			r := &CamundaClusterReconciler{
				Client: fake.NewClientBuilder().WithScheme(scheme).WithObjects(&claim).Build(),
			}
			key := client.ObjectKeyFromObject(&claim)

			var live corev1.PersistentVolumeClaim
			require.NoError(t, r.Get(context.Background(), key, &live))
			applied := live.ResourceVersion

			storage := brokerStorage{claims: []corev1.PersistentVolumeClaim{live}}
			if tt.image != "" {
				storage.statefulSet = brokerStorageRunning(tt.image).statefulSet
			}
			require.NoError(t, r.stampBrokerVersion(context.Background(), storage))

			require.NoError(t, r.Get(context.Background(), key, &live))
			assert.Equal(t, tt.want, live.Annotations[components.BrokerVersionAnnotation])
			if tt.patched {
				assert.NotEqual(t, applied, live.ResourceVersion, "the stamp is written with a patch")
			} else {
				assert.Equal(t, applied, live.ResourceVersion, "a current stamp is not rewritten")
			}
		})
	}
}
