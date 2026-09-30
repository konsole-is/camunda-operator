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
	"time"

	"github.com/sourcehawk/operator-component-framework/pkg/component"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/api/meta/testrestmapper"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/tools/events"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	v1 "github.com/konsole-is/camunda-operator/api/v1"
)

// TestProcessGracePeriod covers every process kind: the StatefulSet of the
// brokers and the Deployments of the gateway, the web applications, and
// connectors. None of their workloads is ready.
func TestProcessGracePeriod(t *testing.T) {
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
			in := fixtureSeparated(t)
			in.GracePeriod = gracePeriod
			processes, err := Build(in)
			require.NoError(t, err)

			for _, pc := range processes {
				if !pc.Process.Enabled {
					continue
				}

				t.Run(pc.Process.Component, func(t *testing.T) {
					cond := reconcileCreating(t, in.Cluster.DeepCopy(), pc.Component, tt.since)

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
	t *testing.T, owner *v1.CamundaCluster, comp *component.Component, since time.Duration,
) component.Condition {
	t.Helper()

	scheme := runtime.NewScheme()
	require.NoError(t, clientgoscheme.AddToScheme(scheme))
	require.NoError(t, v1.AddToScheme(scheme))

	condType := comp.GetCondition(owner).Type
	owner.Status.Conditions = []metav1.Condition{{
		Type:               condType,
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
