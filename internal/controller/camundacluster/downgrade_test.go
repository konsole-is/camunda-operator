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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	v1 "github.com/konsole-is/camunda-operator/api/v1"
	components "github.com/konsole-is/camunda-operator/pkg/components/camundacluster"
)

func TestConsumeDowngradeSanction(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		// sanctioned is the version that the annotation names. Empty means
		// that the cluster carries no annotation.
		sanctioned string
		// want is the annotation value on the live object after the call.
		// Empty means that the cluster carries no annotation.
		want string
		// patched is true when the call must write the live object.
		patched bool
	}{
		{
			name: "no annotation",
		},
		{
			name:       "the annotation names another version",
			sanctioned: "8.9.7",
			want:       "8.9.7",
		},
		{
			name:       "the annotation names the running version",
			sanctioned: "8.9.9",
			patched:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			scheme, err := v1.SchemeBuilder.Build()
			require.NoError(t, err)

			cluster := &v1.CamundaCluster{ObjectMeta: metav1.ObjectMeta{Name: "cc", Namespace: "ns"}}
			if tt.sanctioned != "" {
				cluster.Annotations = map[string]string{
					components.AllowVersionDowngradeAnnotation: tt.sanctioned,
				}
			}
			r := &CamundaClusterReconciler{
				Client: fake.NewClientBuilder().WithScheme(scheme).WithObjects(cluster).Build(),
			}
			key := client.ObjectKeyFromObject(cluster)

			var live v1.CamundaCluster
			require.NoError(t, r.Get(context.Background(), key, &live))
			applied := live.ResourceVersion

			storage := brokerStorageRunning("camunda/camunda:8.9.9")
			require.NoError(t, r.consumeDowngradeSanction(context.Background(), cluster, storage))

			require.NoError(t, r.Get(context.Background(), key, &live))
			assert.Equal(t, tt.want, live.Annotations[components.AllowVersionDowngradeAnnotation])
			if tt.patched {
				assert.NotEqual(t, applied, live.ResourceVersion, "a spent sanction is removed with a patch")
			} else {
				assert.Equal(t, applied, live.ResourceVersion, "there is no sanction to spend, so nothing is written")
			}
		})
	}
}

func TestRefuseDowngrade(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		// sanctioned is the version that the annotation names. Empty means
		// that the cluster carries no annotation.
		sanctioned string
		// effective is the version of the preset-merged spec.
		effective string
		// image is the broker image of the applied StatefulSet. Empty means
		// that no StatefulSet is applied yet.
		image string
		// running is the version that a refusal must name. Empty means that
		// the case expects no refusal.
		running string
	}{
		{
			name:      "a lower patch without a sanction",
			effective: "8.9.8",
			image:     "camunda/camunda:8.9.9",
			running:   "8.9.9",
		},
		{
			name:      "a lower minor without a sanction",
			effective: "8.8.9",
			image:     "camunda/camunda:8.9.9",
			running:   "8.9.9",
		},
		{
			name:       "the sanction names the effective version",
			sanctioned: "8.9.8",
			effective:  "8.9.8",
			image:      "camunda/camunda:8.9.9",
		},
		{
			name:       "the sanction names another version",
			sanctioned: "8.9.7",
			effective:  "8.9.8",
			image:      "camunda/camunda:8.9.9",
			running:    "8.9.9",
		},
		{
			name:      "no StatefulSet before the first apply",
			effective: "8.9.8",
		},
		{
			name:      "the running tag is not a version",
			effective: "8.9.8",
			image:     "camunda/camunda:latest",
		},
		{
			name:      "the effective version equals the running one",
			effective: "8.9.9",
			image:     "camunda/camunda:8.9.9",
		},
		{
			name:      "the effective version is above the running one",
			effective: "8.9.10",
			image:     "camunda/camunda:8.9.9",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			cluster := &v1.CamundaCluster{ObjectMeta: metav1.ObjectMeta{Name: "cc", Namespace: "ns"}}
			if tt.sanctioned != "" {
				cluster.Annotations = map[string]string{
					components.AllowVersionDowngradeAnnotation: tt.sanctioned,
				}
			}
			in := components.Input{
				Effective: components.NewEffective(v1.CamundaClusterSpec{Version: tt.effective}),
			}
			storage := brokerStorage{}
			if tt.image != "" {
				storage = brokerStorageRunning(tt.image)
			}

			failure := refuseDowngrade(cluster, in, storage)

			if tt.running == "" {
				assert.Nil(t, failure)
				return
			}

			require.NotNil(t, failure)
			assert.Equal(t, v1.ReasonVersionDowngradeRefused, failure.Reason)
			assert.Contains(t, failure.Message, tt.effective+" is below the running version "+tt.running)
			assert.Contains(t, failure.Message, "Set the version to "+tt.running+" or later")
			assert.Contains(t, failure.Message, "restore a backup taken with "+tt.effective)
			assert.Contains(
				t, failure.Message,
				components.AllowVersionDowngradeAnnotation+`="`+tt.effective+`"`,
				"the message names the annotation that sanctions this exact version",
			)
		})
	}
}
