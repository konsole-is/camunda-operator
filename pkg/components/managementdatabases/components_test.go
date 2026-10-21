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

package managementdatabases

import (
	"testing"

	"github.com/sourcehawk/operator-component-framework/pkg/component"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta/testrestmapper"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/events"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	pluginsv1alpha1 "github.com/konsole-is/camunda-operator/api/plugins/v1alpha1"
	v1 "github.com/konsole-is/camunda-operator/api/v1"
)

func TestBackingDeletesTheUnlistedDatabasesOnlyIfOwned(t *testing.T) {
	t.Parallel()

	owner := testOwner()
	owned := &v1.Database{ObjectMeta: metav1.ObjectMeta{
		Name: "ops-keycloak", Namespace: "ops-ns",
		OwnerReferences: []metav1.OwnerReference{{
			APIVersion: pluginsv1alpha1.GroupVersion.String(), Kind: "PluginRequest", Name: owner.Name, UID: owner.UID,
		}},
	}}
	foreign := &v1.Database{ObjectMeta: metav1.ObjectMeta{Name: "ops-web-modeler", Namespace: "ops-ns"}}
	c := testClient(t, owner, owned, foreign)

	b, err := NewBacking(goldenValues(Identity), "Ready")
	require.NoError(t, err)
	require.NoError(t, b.Component.Reconcile(t.Context(), component.ReconcileContext{
		Client: c, APIReader: c, Scheme: c.Scheme(), Owner: owner, EventRecorder: events.NewFakeRecorder(100),
	}))

	assert.NoError(t, c.Get(t.Context(), key("ops-db"), &v1.DatabaseServer{}))
	assert.NoError(t, c.Get(t.Context(), key("ops-identity"), &v1.Database{}))
	assert.True(
		t, apierrors.IsNotFound(c.Get(t.Context(), key("ops-keycloak"), &v1.Database{})),
		"an unlisted Database that the owner owns goes",
	)
	assert.NoError(
		t, c.Get(t.Context(), key("ops-web-modeler"), &v1.Database{}),
		"an unlisted Database that the owner does not own stays",
	)
}

func TestDeleteDatabasesAndDeleteServerDeleteOnlyOwnedObjects(t *testing.T) {
	t.Parallel()

	owner := testOwner()
	ref := []metav1.OwnerReference{
		{APIVersion: pluginsv1alpha1.GroupVersion.String(), Kind: "PluginRequest", Name: owner.Name, UID: owner.UID},
	}
	c := testClient(
		t, owner,
		&v1.Database{ObjectMeta: metav1.ObjectMeta{Name: "ops-identity", Namespace: "ops-ns", OwnerReferences: ref}},
		&v1.Database{ObjectMeta: metav1.ObjectMeta{Name: "ops-keycloak", Namespace: "ops-ns"}},
		&v1.DatabaseServer{ObjectMeta: metav1.ObjectMeta{Name: "ops-db", Namespace: "ops-ns", OwnerReferences: ref}},
	)
	rec := component.ReconcileContext{
		Client: c, APIReader: c, Scheme: c.Scheme(), Owner: owner, EventRecorder: events.NewFakeRecorder(100),
	}

	databases, err := DeleteDatabases(goldenValues())
	require.NoError(t, err)
	require.NoError(t, databases.Reconcile(t.Context(), rec))

	assert.True(t, apierrors.IsNotFound(c.Get(t.Context(), key("ops-identity"), &v1.Database{})))
	assert.NoError(t, c.Get(t.Context(), key("ops-keycloak"), &v1.Database{}))
	assert.NoError(t, c.Get(t.Context(), key("ops-db"), &v1.DatabaseServer{}), "the databases stage keeps the server")

	server, err := DeleteServer(goldenValues())
	require.NoError(t, err)
	require.NoError(t, server.Reconcile(t.Context(), rec))

	assert.True(t, apierrors.IsNotFound(c.Get(t.Context(), key("ops-db"), &v1.DatabaseServer{})))
}

func TestOrphanRemovesTheOwnerReferenceAndKeepsTheObjects(t *testing.T) {
	t.Parallel()

	owner := testOwner()
	ref := []metav1.OwnerReference{{
		APIVersion: pluginsv1alpha1.GroupVersion.String(), Kind: "PluginRequest", Name: owner.Name, UID: owner.UID,
		Controller: new(true), BlockOwnerDeletion: new(true),
	}}
	v := goldenValues()
	objs := make([]client.Object, 0, len(Components)+2)
	objs = append(objs, owner)
	for _, c := range Components {
		db := Database(v, c)
		db.OwnerReferences = ref
		objs = append(objs, db)
	}
	server := Server(v)
	server.OwnerReferences = ref
	objs = append(objs, server)
	c := testClient(t, objs...)

	orphan, err := Orphan(v)
	require.NoError(t, err)
	require.NoError(t, orphan.Reconcile(t.Context(), component.ReconcileContext{
		Client: c, APIReader: c, Scheme: c.Scheme(), Owner: owner, EventRecorder: events.NewFakeRecorder(100),
	}))

	for _, name := range []string{"ops-keycloak", "ops-identity", "ops-web-modeler"} {
		var db v1.Database
		require.NoError(t, c.Get(t.Context(), key(name), &db))
		assert.Empty(t, db.OwnerReferences, name)
	}
	var live v1.DatabaseServer
	require.NoError(t, c.Get(t.Context(), key("ops-db"), &live))
	assert.Empty(t, live.OwnerReferences)
}

func testOwner() *pluginsv1alpha1.PluginRequest {
	return &pluginsv1alpha1.PluginRequest{
		ObjectMeta: metav1.ObjectMeta{Name: "ops-management-database", Namespace: "ops-ns", UID: "owner-uid"},
	}
}

func testClient(t *testing.T, objs ...client.Object) client.Client {
	scheme := runtime.NewScheme()
	require.NoError(t, pluginsv1alpha1.AddToScheme(scheme))
	require.NoError(t, v1.AddToScheme(scheme))

	return fake.NewClientBuilder().
		WithScheme(scheme).
		WithRESTMapper(testrestmapper.TestOnlyStaticRESTMapper(scheme)).
		WithObjects(objs...).
		WithStatusSubresource(&v1.DatabaseServer{}, &v1.Database{}).
		Build()
}

func key(name string) client.ObjectKey {
	return client.ObjectKey{Namespace: "ops-ns", Name: name}
}
