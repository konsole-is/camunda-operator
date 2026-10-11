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

package database

import (
	"context"
	"testing"

	"github.com/sourcehawk/operator-component-framework/pkg/component"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta/testrestmapper"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/events"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	apiv1 "github.com/konsole-is/camunda-operator/api/v1"
)

// TestComponentReleasesTheDatabase proves that the ocf resource options of a
// deleted owner act on the wrapped kind: Delete removes the Database, and
// Orphan keeps it with its labels and spec and without the owner reference
// of the owner.
func TestComponentReleasesTheDatabase(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		option component.ResourceOption
		kept   bool
	}{
		{name: "Delete removes the Database", option: component.Delete()},
		{name: "Orphan keeps the Database", option: component.OrphanWhen(true), kept: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			owner := &apiv1.CamundaCluster{
				ObjectMeta: metav1.ObjectMeta{Name: "orders", Namespace: "team-a", UID: "owner-uid"},
			}
			live := goldenDatabase()
			live.Labels = map[string]string{"camunda.io/test": "kept"}
			live.OwnerReferences = []metav1.OwnerReference{{
				APIVersion: apiv1.GroupVersion.String(),
				Kind:       "CamundaCluster",
				Name:       owner.Name,
				UID:        owner.UID,
				Controller: ptr.To(true),
			}}

			res, err := NewBuilder(goldenDatabase()).Build()
			require.NoError(t, err)

			comp, err := component.NewComponentBuilder().
				WithName("backing").
				WithConditionType("Ready").
				WithResource(res, tt.option).
				Build()
			require.NoError(t, err)

			scheme := goldenScheme(t)
			c := fake.NewClientBuilder().
				WithScheme(scheme).
				WithRESTMapper(testrestmapper.TestOnlyStaticRESTMapper(scheme)).
				WithObjects(live).
				Build()
			require.NoError(t, comp.Reconcile(context.Background(), component.ReconcileContext{
				Client:        c,
				APIReader:     c,
				Scheme:        scheme,
				EventRecorder: events.NewFakeRecorder(10),
				Owner:         owner,
			}))

			var got apiv1.Database
			err = c.Get(context.Background(), client.ObjectKeyFromObject(live), &got)
			if !tt.kept {
				assert.True(t, apierrors.IsNotFound(err), "the Database must be gone, got %v", err)

				return
			}

			require.NoError(t, err)
			assert.Empty(t, got.OwnerReferences)
			assert.Equal(t, live.Labels, got.Labels)
			assert.Equal(t, live.Spec, got.Spec)
		})
	}
}
