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

package secondarystorageconfig

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	v1 "github.com/konsole-is/camunda-operator/api/v1"
)

func TestHolderOf(t *testing.T) {
	contract := &v1.SecondaryStorageConfig{ObjectMeta: metav1.ObjectMeta{Name: "storage", Namespace: "team-a"}}
	_, held := HolderOf(contract)
	assert.False(t, held, "an unannotated contract has no holder")

	contract.Annotations = map[string]string{
		ClaimHolderAnnotation:    "team-a/orchestration",
		ClaimHolderUIDAnnotation: "uid-1",
	}
	holder, held := HolderOf(contract)
	require.True(t, held)
	assert.Equal(
		t,
		Holder{Cluster: types.NamespacedName{Namespace: "team-a", Name: "orchestration"}, UID: "uid-1"},
		holder,
	)

	for name, annotations := range map[string]map[string]string{
		"no uid":       {ClaimHolderAnnotation: "team-a/orchestration"},
		"no namespace": {ClaimHolderAnnotation: "orchestration", ClaimHolderUIDAnnotation: "uid-1"},
		"empty name":   {ClaimHolderAnnotation: "team-a/", ClaimHolderUIDAnnotation: "uid-1"},
	} {
		t.Run(name, func(t *testing.T) {
			contract.Annotations = annotations
			_, held := HolderOf(contract)
			assert.False(t, held)
		})
	}
}

func TestClaim(t *testing.T) {
	scheme := runtime.NewScheme()
	require.NoError(t, clientgoscheme.AddToScheme(scheme))
	require.NoError(t, v1.AddToScheme(scheme))
	holder := Holder{Cluster: types.NamespacedName{Namespace: "team-a", Name: "orchestration"}, UID: "uid-1"}

	t.Run("writes the holder onto an unclaimed contract", func(t *testing.T) {
		contract := &v1.SecondaryStorageConfig{ObjectMeta: metav1.ObjectMeta{Name: "storage", Namespace: "team-a"}}
		c := fake.NewClientBuilder().WithScheme(scheme).WithObjects(contract).Build()
		var live v1.SecondaryStorageConfig
		require.NoError(t, c.Get(context.Background(), client.ObjectKeyFromObject(contract), &live))

		require.NoError(t, Claim(context.Background(), c, &live, holder))

		require.NoError(t, c.Get(context.Background(), client.ObjectKeyFromObject(contract), &live))
		got, held := HolderOf(&live)
		require.True(t, held)
		assert.Equal(t, holder, got)
	})

	t.Run("a stale read loses to a concurrent claim", func(t *testing.T) {
		contract := &v1.SecondaryStorageConfig{ObjectMeta: metav1.ObjectMeta{Name: "storage", Namespace: "team-a"}}
		c := fake.NewClientBuilder().WithScheme(scheme).WithObjects(contract).Build()
		var first, second v1.SecondaryStorageConfig
		require.NoError(t, c.Get(context.Background(), client.ObjectKeyFromObject(contract), &first))
		require.NoError(t, c.Get(context.Background(), client.ObjectKeyFromObject(contract), &second))

		require.NoError(t, Claim(context.Background(), c, &first, holder))
		other := Holder{Cluster: types.NamespacedName{Namespace: "team-b", Name: "other"}, UID: "uid-2"}
		err := Claim(context.Background(), c, &second, other)
		require.Error(t, err)
		assert.True(
			t,
			apierrors.IsConflict(err),
			"the second claim must fail on the resourceVersion precondition, got: %v",
			err,
		)

		var live v1.SecondaryStorageConfig
		require.NoError(t, c.Get(context.Background(), client.ObjectKeyFromObject(contract), &live))
		got, _ := HolderOf(&live)
		assert.Equal(t, holder, got)
	})

	t.Run("keeps the other annotations of the contract", func(t *testing.T) {
		contract := &v1.SecondaryStorageConfig{ObjectMeta: metav1.ObjectMeta{
			Name: "storage", Namespace: "team-a", Annotations: map[string]string{"example.com/keep": "yes"},
		}}
		c := fake.NewClientBuilder().WithScheme(scheme).WithObjects(contract).Build()
		var live v1.SecondaryStorageConfig
		require.NoError(t, c.Get(context.Background(), client.ObjectKeyFromObject(contract), &live))
		require.NoError(t, Claim(context.Background(), c, &live, holder))
		require.NoError(t, c.Get(context.Background(), client.ObjectKeyFromObject(contract), &live))
		assert.Equal(t, "yes", live.Annotations["example.com/keep"])
	})
}
