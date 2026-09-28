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

package pointintimerestore

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/events"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"

	v1 "github.com/konsole-is/camunda-operator/api/v1"
)

func TestReplacementBetweenTwoReadsKeepsTheRollbackHeld(t *testing.T) {
	scheme := runtime.NewScheme()
	require.NoError(t, v1.AddToScheme(scheme))

	pitr := &v1.PointInTimeRestore{
		ObjectMeta: metav1.ObjectMeta{Name: "pitr", Namespace: "ns", UID: "pitr-uid"},
		Spec: v1.PointInTimeRestoreSpec{
			ClusterRef: v1.ClusterRef{Name: "cc"},
			Timestamp:  metav1.Now(),
		},
		Status: v1.PointInTimeRestoreStatus{
			Phase:           v1.PointInTimeRestoreRestoringDatabase,
			RestoreProgress: v1.RestoreProgress{TargetClusterUID: "pinned"},
			Storage: &v1.PointInTimeRestoreStorage{
				DatabaseServerConfig:    "dbsc",
				DatabaseServerConfigUID: "dbsc-uid",
			},
		},
	}
	request := recoveryRequest(pitr)
	contract := &v1.DatabaseServerConfig{
		ObjectMeta: metav1.ObjectMeta{Name: "dbsc", Namespace: "ns", UID: "dbsc-uid"},
		Spec: v1.DatabaseServerConfigSpec{
			PITR:     &v1.PITRCapability{Enabled: true, Recovery: v1.RecoveryModeOperator},
			Recovery: &request,
		},
	}
	cluster := &v1.CamundaCluster{ObjectMeta: metav1.ObjectMeta{Name: "cc", Namespace: "ns", UID: "pinned"}}

	reads := 0
	reader := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(contract, cluster).
		WithInterceptorFuncs(interceptor.Funcs{
			Get: func(
				ctx context.Context,
				c client.WithWatch,
				key client.ObjectKey,
				obj client.Object,
				opts ...client.GetOption,
			) error {
				if err := c.Get(ctx, key, obj, opts...); err != nil {
					return err
				}
				if read, ok := obj.(*v1.CamundaCluster); ok {
					reads++
					if reads > 1 {
						read.UID = "replacement"
					}
				}

				return nil
			},
		}).
		Build()
	r := &Reconciler{
		APIReader:     reader,
		EventRecorder: events.NewFakeRecorder(10),
		opts:          Options{}.withDefaults(),
	}

	_, err := r.enterDatabaseRecovery(context.Background(), pitr)
	require.NoError(t, err)

	assert.Equal(t, v1.PointInTimeRestoreRestoringDatabase, pitr.Status.Phase)
	ready := meta.FindStatusCondition(pitr.Status.Conditions, v1.ConditionReady)
	require.NotNil(t, ready)
	assert.Contains(t, ready.Message, "was replaced while its database server rolls back")
}
