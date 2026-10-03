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
	"context"
	"testing"
	"time"

	"github.com/sourcehawk/operator-component-framework/pkg/component"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/api/meta/testrestmapper"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/events"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	v1 "github.com/konsole-is/camunda-operator/api/v1"
)

// TestWorkloadGracePeriod covers the Keycloak and the Deployments of
// Management Identity, Console, and Web Modeler. None of them is ready.
func TestWorkloadGracePeriod(t *testing.T) {
	const gracePeriod = 10 * time.Minute

	tests := []struct {
		name       string
		since      time.Duration
		wantReason component.Status
	}{
		{name: "keeps Creating inside the grace period", since: time.Minute, wantReason: component.AliveCreating},
		{name: "reports Down after the grace period", since: time.Hour, wantReason: component.Down},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			in := fixtureKeycloakRealistic(t, true)
			in.GracePeriod = gracePeriod
			built, err := Build(in)
			require.NoError(t, err)

			for _, name := range []string{ComponentKeycloak, ComponentIdentity, ComponentConsole, ComponentWebModeler} {
				comp := builtComponent(t, built, name)
				t.Run(comp.GetName(), func(t *testing.T) {
					cond := reconcileCreating(t, in.Cluster.DeepCopy(), comp, tt.since)

					assert.Equal(t, metav1.ConditionFalse, cond.Status)
					assert.Equal(t, string(tt.wantReason), cond.Reason)
				})
			}
		})
	}
}

// reconcileCreating reconciles comp once against a fake API server, on an
// owner whose condition has reported Creating for since. It returns the
// condition that the component staged.
func reconcileCreating(
	t *testing.T, owner *v1.CamundaManagementCluster, comp *component.Component, since time.Duration,
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
