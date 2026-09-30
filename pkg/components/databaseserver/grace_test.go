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

package databaseserver

import (
	"context"
	"testing"
	"time"

	cnpgv1 "github.com/cloudnative-pg/api/pkg/api/v1"
	"github.com/sourcehawk/operator-component-framework/pkg/component"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/api/meta/testrestmapper"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/events"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	v1 "github.com/konsole-is/camunda-operator/api/v1"
)

// TestClusterGracePeriod runs a server of two instances.
func TestClusterGracePeriod(t *testing.T) {
	const gracePeriod = 30 * time.Minute

	tests := []struct {
		name       string
		ready      *int
		since      time.Duration
		wantReason component.Status
	}{
		{
			name:  "keeps Creating inside the grace period",
			since: time.Minute, wantReason: component.AliveCreating,
		},
		{
			name:  "reports Down after the grace period without a ready instance",
			since: time.Hour, wantReason: component.Down,
		},
		{
			name:  "reports Degraded after the grace period with one of two instances ready",
			ready: new(1),
			since: time.Hour, wantReason: component.Degraded,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server, preset, release := goldenMinimalDatabaseServer()
			merged := MergeSpec(server.Spec, preset, release)
			comp, _, err := ClusterComponent(server, merged, nil, "", nil, "", gracePeriod)
			require.NoError(t, err)

			var existing []client.Object
			if tt.ready != nil {
				existing = append(existing, &cnpgv1.Cluster{
					ObjectMeta: metav1.ObjectMeta{Name: ClusterName(server), Namespace: server.Namespace},
					Spec:       cnpgv1.ClusterSpec{Instances: 2},
					Status: cnpgv1.ClusterStatus{
						Phase:          "Creating a new replica",
						ReadyInstances: *tt.ready,
					},
				})
			}

			cond := reconcileCreating(t, server, comp, tt.since, existing...)

			assert.Equal(t, metav1.ConditionFalse, cond.Status)
			assert.Equal(t, string(tt.wantReason), cond.Reason)
		})
	}
}

// reconcileCreating reconciles comp once against a fake API server that
// holds existing, on an owner whose condition has reported Creating for
// since. It returns the condition that the component staged.
func reconcileCreating(
	t *testing.T,
	owner *v1.DatabaseServer,
	comp *component.Component,
	since time.Duration,
	existing ...client.Object,
) component.Condition {
	t.Helper()

	scheme := goldenScheme(t)
	owner.Status.Conditions = []metav1.Condition{{
		Type:               comp.GetCondition(owner).Type,
		Status:             metav1.ConditionFalse,
		Reason:             string(component.AliveCreating),
		LastTransitionTime: metav1.NewTime(time.Now().Add(-since)),
	}}

	c := fake.NewClientBuilder().
		WithScheme(scheme).
		WithRESTMapper(testrestmapper.TestOnlyStaticRESTMapper(scheme)).
		WithObjects(existing...).
		WithStatusSubresource(existing...).
		Build()
	require.NoError(t, comp.Reconcile(context.Background(), component.ReconcileContext{
		Client:        c,
		APIReader:     c,
		Scheme:        scheme,
		EventRecorder: events.NewFakeRecorder(100),
		Owner:         owner,
	}))

	return comp.GetCondition(owner)
}
