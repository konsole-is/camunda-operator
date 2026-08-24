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
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/events"
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
		// effective is the version of the preset-merged spec.
		effective string
		// want is the annotation value on the live object after the call.
		// Empty means that the cluster carries no annotation.
		want string
		// patched is true when the call must write the live object.
		patched bool
		// staleWrite is true when another manager updates the stored object
		// after the test reads it, so the cluster passed to the call carries
		// a resourceVersion older than the one on the server.
		staleWrite bool
	}{
		{
			name:      "no annotation",
			effective: "8.9.8",
		},
		{
			name:       "the annotation names the pending effective version",
			sanctioned: "8.9.8",
			effective:  "8.9.8",
			want:       "8.9.8",
		},
		{
			name:       "the annotation names the running version",
			sanctioned: "8.9.9",
			effective:  "8.9.8",
			patched:    true,
		},
		{
			name:       "the annotation names neither version",
			sanctioned: "8.9.7",
			effective:  "8.9.8",
			patched:    true,
		},
		{
			name:       "the brokers carry the sanctioned effective version",
			sanctioned: "8.9.9",
			effective:  "8.9.9",
			patched:    true,
		},
		{
			name:       "a concurrent write conflicts",
			sanctioned: "8.9.9",
			effective:  "8.9.8",
			want:       "8.9.9",
			staleWrite: true,
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

			if tt.staleWrite {
				live.Labels = map[string]string{"changed-by": "another-manager"}
				require.NoError(t, r.Update(context.Background(), &live))
			}

			in := components.Input{
				Effective: components.NewEffective(v1.CamundaClusterSpec{Version: tt.effective}),
			}
			storage := brokerStorageRunning("camunda/camunda:8.9.9")
			err = r.consumeDowngradeSanction(context.Background(), cluster, in, storage)

			if tt.staleWrite {
				require.Error(t, err)
				assert.True(t, apierrors.IsConflict(err), "a stale patch reports a conflict, got: %v", err)
				require.NoError(t, r.Get(context.Background(), key, &live))
				assert.Equal(t, tt.want, live.Annotations[components.AllowVersionDowngradeAnnotation])
				return
			}

			require.NoError(t, err)
			require.NoError(t, r.Get(context.Background(), key, &live))
			assert.Equal(t, tt.want, live.Annotations[components.AllowVersionDowngradeAnnotation])
			if tt.patched {
				assert.NotEqual(t, applied, live.ResourceVersion, "a spent sanction is removed with a patch")
			} else {
				assert.Equal(t, applied, live.ResourceVersion, "an unspent sanction is left in place")
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
			assert.Contains(t, failure.Message, "To run "+tt.effective+" on the data of a backup taken with it")
			assert.Contains(
				t, failure.Message, "cannot start while this refusal stands",
				"the message orders the remedies: the version goes back first, the restore comes after",
			)
			assert.Contains(
				t, failure.Message,
				components.AllowVersionDowngradeAnnotation+`="`+tt.effective+`"`,
				"the message names the annotation that sanctions this exact version",
			)
		})
	}
}

func TestRecordRefusedDowngrade(t *testing.T) {
	t.Parallel()

	refused := metav1.Condition{
		Type:    v1.ConditionReady,
		Status:  metav1.ConditionFalse,
		Reason:  v1.ReasonVersionDowngradeRefused,
		Message: "the effective version 8.9.8 is below the running version 8.9.9",
	}
	other := refused
	other.Message = "the effective version 8.9.7 is below the running version 8.9.9"

	tests := []struct {
		name string
		// record runs the refusals of the case against r and the clusters it
		// builds. Every cluster it passes stands for the copy that one
		// reconcile received.
		record func(r *CamundaClusterReconciler)
		// want is the number of events that must reach the recorder.
		want int
	}{
		{
			name: "the first refusal is recorded",
			record: func(r *CamundaClusterReconciler) {
				r.recordRefusedDowngrade(refusedCluster(nil), refused)
			},
			want: 1,
		},
		{
			name: "a reconcile that reads the cluster before the refusal lands records no second event",
			record: func(r *CamundaClusterReconciler) {
				r.recordRefusedDowngrade(refusedCluster(nil), refused)
				r.recordRefusedDowngrade(refusedCluster(nil), refused)
				r.recordRefusedDowngrade(refusedCluster(nil), refused)
			},
			want: 1,
		},
		{
			name: "the Ready condition stops a manager that starts with an empty memo",
			record: func(r *CamundaClusterReconciler) {
				r.recordRefusedDowngrade(refusedCluster(&refused), refused)
			},
			want: 0,
		},
		{
			name: "another refusal of the same cluster is recorded",
			record: func(r *CamundaClusterReconciler) {
				r.recordRefusedDowngrade(refusedCluster(nil), refused)
				r.recordRefusedDowngrade(refusedCluster(nil), other)
			},
			want: 2,
		},
		{
			name: "a cluster recreated under the same name is recorded",
			record: func(r *CamundaClusterReconciler) {
				r.recordRefusedDowngrade(refusedCluster(nil), refused)
				recreated := refusedCluster(nil)
				recreated.UID = "second"
				r.recordRefusedDowngrade(recreated, refused)
			},
			want: 2,
		},
		{
			name: "a refusal that came back after the cluster ran is recorded",
			record: func(r *CamundaClusterReconciler) {
				cluster := refusedCluster(nil)
				r.recordRefusedDowngrade(cluster, refused)
				r.refusals.forget(client.ObjectKeyFromObject(cluster))
				r.recordRefusedDowngrade(cluster, refused)
			},
			want: 2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			recorder := events.NewFakeRecorder(8)
			r := &CamundaClusterReconciler{EventRecorder: recorder}

			tt.record(r)

			assert.Equal(t, tt.want, len(recorder.Events))
		})
	}
}

// refusedCluster returns the cluster that one reconcile received, with ready
// as its Ready condition. A nil ready stands for a copy that does not carry
// the refusal that the reconcile before it staged.
func refusedCluster(ready *metav1.Condition) *v1.CamundaCluster {
	cluster := &v1.CamundaCluster{
		ObjectMeta: metav1.ObjectMeta{Name: "cc", Namespace: "ns", UID: "first"},
	}
	if ready != nil {
		cluster.Status.Conditions = []metav1.Condition{*ready}
	}

	return cluster
}
