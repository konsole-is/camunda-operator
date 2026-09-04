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

package keycloakadmin

import (
	"context"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFindUser(t *testing.T) {
	t.Parallel()

	fake := &fakeKeycloak{token: "an-access-token"}
	fake.handler = func(w http.ResponseWriter, _ *http.Request, _ string) {
		_, _ = w.Write([]byte(`[{"id":"9f2a","username":"platform-admin"}]`))
	}

	id, err := New(fake.start(t), "camunda-platform", "admin", "secret").
		FindUser(context.Background(), "platform-admin")
	require.NoError(t, err)

	assert.Equal(t, "9f2a", id)
	calls := fake.recorded()
	require.Len(t, calls, 2)
	assert.Equal(t, "/auth/admin/realms/camunda-platform/users", calls[1].path)
	assert.Equal(t, "exact=true&username=platform-admin", calls[1].query)
}

// The lookup is exact, so a realm that holds no user of that name answers with
// an empty list rather than with a user whose name starts the same.
func TestFindUserWithoutAMatch(t *testing.T) {
	t.Parallel()

	fake := &fakeKeycloak{token: "an-access-token"}
	fake.handler = func(w http.ResponseWriter, _ *http.Request, _ string) {
		_, _ = w.Write([]byte(`[]`))
	}

	id, err := New(fake.start(t), "camunda-platform", "admin", "secret").
		FindUser(context.Background(), "platform-admin")

	require.NoError(t, err)
	assert.Empty(t, id)
}

func TestFindRealmRole(t *testing.T) {
	t.Parallel()

	fake := &fakeKeycloak{token: "an-access-token"}
	fake.handler = func(w http.ResponseWriter, _ *http.Request, _ string) {
		_, _ = w.Write([]byte(`{"id":"1b7c","name":"Optimize","composite":false}`))
	}

	role, err := New(fake.start(t), "camunda-platform", "admin", "secret").
		FindRealmRole(context.Background(), "Optimize")
	require.NoError(t, err)

	require.NotNil(t, role)
	assert.Equal(t, RealmRole{ID: "1b7c", Name: "Optimize"}, *role)
	calls := fake.recorded()
	require.Len(t, calls, 2)
	assert.Equal(t, "/auth/admin/realms/camunda-platform/roles/Optimize", calls[1].path)
}

// A realm that Management Identity bootstrapped without Optimize holds no
// Optimize role, and the administration API answers that with a 404.
func TestFindRealmRoleThatTheRealmDoesNotHold(t *testing.T) {
	t.Parallel()

	fake := &fakeKeycloak{token: "an-access-token"}
	fake.handler = func(w http.ResponseWriter, _ *http.Request, _ string) {
		w.WriteHeader(http.StatusNotFound)
	}

	role, err := New(fake.start(t), "camunda-platform", "admin", "secret").
		FindRealmRole(context.Background(), "Optimize")

	require.NoError(t, err)
	assert.Nil(t, role)
}

// Every other refused answer stays a failure, so a Keycloak that refuses the
// administrator is never read as a realm without the role.
func TestFindRealmRoleRefused(t *testing.T) {
	t.Parallel()

	fake := &fakeKeycloak{token: "an-access-token"}
	fake.handler = func(w http.ResponseWriter, _ *http.Request, _ string) {
		w.WriteHeader(http.StatusForbidden)
	}

	_, err := New(fake.start(t), "camunda-platform", "admin", "secret").
		FindRealmRole(context.Background(), "Optimize")

	require.Error(t, err)
	assert.Contains(t, err.Error(), "status 403")
}

// The read is the composite one, so a role that the user holds through a group
// or inside another role is in the list. A read of the direct mappings would
// report a role the user holds as missing.
func TestUserRealmRolesReadsTheRolesTheUserHolds(t *testing.T) {
	t.Parallel()

	fake := &fakeKeycloak{token: "an-access-token"}
	fake.handler = func(w http.ResponseWriter, _ *http.Request, _ string) {
		_, _ = w.Write([]byte(`[{"id":"aa","name":"Identity"},{"id":"bb","name":"Console"}]`))
	}

	held, err := New(fake.start(t), "camunda-platform", "admin", "secret").
		UserRealmRoles(context.Background(), "9f2a")
	require.NoError(t, err)

	assert.Equal(t, []RealmRole{{ID: "aa", Name: "Identity"}, {ID: "bb", Name: "Console"}}, held)
	calls := fake.recorded()
	require.Len(t, calls, 2)
	assert.Equal(
		t,
		"/auth/admin/realms/camunda-platform/users/9f2a/role-mappings/realm/composite",
		calls[1].path,
	)
}

func TestAddUserRealmRole(t *testing.T) {
	t.Parallel()

	fake := &fakeKeycloak{token: "an-access-token"}
	fake.handler = func(w http.ResponseWriter, _ *http.Request, _ string) {
		w.WriteHeader(http.StatusNoContent)
	}

	err := New(fake.start(t), "camunda-platform", "admin", "secret").
		AddUserRealmRole(context.Background(), "9f2a", RealmRole{ID: "1b7c", Name: "Optimize"})
	require.NoError(t, err)

	calls := fake.recorded()
	require.Len(t, calls, 2)
	assert.Equal(t, http.MethodPost, calls[1].method)
	assert.Equal(
		t,
		"/auth/admin/realms/camunda-platform/users/9f2a/role-mappings/realm",
		calls[1].path,
	)
	assert.JSONEq(t, `[{"id":"1b7c","name":"Optimize"}]`, calls[1].body)
}

func TestAddUserRealmRoleRefused(t *testing.T) {
	t.Parallel()

	fake := &fakeKeycloak{token: "an-access-token"}
	fake.handler = func(w http.ResponseWriter, _ *http.Request, _ string) {
		w.WriteHeader(http.StatusForbidden)
		_, _ = w.Write([]byte(`{"error":"unknown_error"}`))
	}

	err := New(fake.start(t), "camunda-platform", "admin", "secret").
		AddUserRealmRole(context.Background(), "9f2a", RealmRole{ID: "1b7c", Name: "Optimize"})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "status 403")
	assert.Contains(t, err.Error(), "Optimize")
}
