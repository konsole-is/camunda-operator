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

package camundaadmin_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/konsole-is/camunda-operator/pkg/camundaadmin"
	"github.com/konsole-is/camunda-operator/pkg/camundaadmin/camundaadmintest"
)

// The administrator that every spec of this file authenticates as.
const (
	adminUsername = "admin"
	adminPassword = "admin-password"
)

// newAdminClient starts a fake user API with an administrator and returns a
// client authenticated as that administrator.
func newAdminClient(t *testing.T) (*camundaadmintest.UserAPI, *camundaadmin.UserClient) {
	t.Helper()

	api := camundaadmintest.NewUserAPI()
	t.Cleanup(api.Close)
	api.SetUser(adminUsername, "Admin", "admin@example.com", adminPassword)

	client, err := camundaadmin.NewUserClient(camundaadmin.UserBinding{
		Endpoint: api.URL(),
		Version:  "8.9.9",
		Auth:     camundaadmin.Auth{Username: adminUsername, Password: adminPassword},
	})
	require.NoError(t, err)

	return api, client
}

func TestCreateAuthorizationRecordsThePermissions(t *testing.T) {
	t.Parallel()

	api, client := newAdminClient(t)

	require.NoError(t, client.CreateAuthorization(context.Background(), camundaadmin.Authorization{
		OwnerID:         "web-modeler",
		OwnerType:       camundaadmin.OwnerUser,
		ResourceType:    "RESOURCE",
		ResourceID:      "*",
		PermissionTypes: []string{"CREATE"},
	}))

	assert.Equal(
		t, []camundaadmintest.Authorization{{
			OwnerID:         "web-modeler",
			OwnerType:       "USER",
			ResourceID:      "*",
			ResourceType:    "RESOURCE",
			PermissionTypes: []string{"CREATE"},
		}}, api.Authorizations(),
	)
}

// The endpoint creates rather than converges, so two identical calls leave two
// authorizations behind. A caller must grant permissions once.
func TestCreateAuthorizationCreatesADuplicate(t *testing.T) {
	t.Parallel()

	api, client := newAdminClient(t)
	authorization := camundaadmin.Authorization{
		OwnerID:         "web-modeler",
		OwnerType:       camundaadmin.OwnerUser,
		ResourceType:    "PROCESS_DEFINITION",
		ResourceID:      "*",
		PermissionTypes: []string{"CREATE_PROCESS_INSTANCE"},
	}

	require.NoError(t, client.CreateAuthorization(context.Background(), authorization))
	require.NoError(t, client.CreateAuthorization(context.Background(), authorization))

	assert.Len(t, api.Authorizations(), 2)
}

func TestCreateAuthorizationReportsARefusedCall(t *testing.T) {
	t.Parallel()

	api, client := newAdminClient(t)
	api.FailNext("createAuthorization", 1)

	err := client.CreateAuthorization(context.Background(), camundaadmin.Authorization{
		OwnerID:         "web-modeler",
		OwnerType:       camundaadmin.OwnerUser,
		ResourceType:    "RESOURCE",
		ResourceID:      "*",
		PermissionTypes: []string{"CREATE"},
	})

	require.ErrorIs(t, err, camundaadmin.ErrRejected)
}

func TestCreateUserStoresTheCredential(t *testing.T) {
	t.Parallel()

	api, client := newAdminClient(t)

	require.NoError(t, client.CreateUser(
		context.Background(),
		camundaadmin.User{
			Username: "web-modeler", Name: "Web Modeler", Email: "web-modeler@example.com",
		},
		"generated-password",
	))

	assert.True(t, api.Exists("web-modeler"))
	assert.Equal(t, "generated-password", api.Password("web-modeler"))
}

func TestCreateUserReportsAUsernameTheClusterHolds(t *testing.T) {
	t.Parallel()

	api, client := newAdminClient(t)
	api.SetUser("web-modeler", "Web Modeler", "web-modeler@example.com", "existing")

	err := client.CreateUser(
		context.Background(),
		camundaadmin.User{Username: "web-modeler", Name: "Web Modeler"},
		"generated-password",
	)

	require.ErrorIs(t, err, camundaadmin.ErrAlreadyExists)
	assert.Equal(t, "existing", api.Password("web-modeler"))
}

func TestCreateUserReportsAWrongAdministratorPassword(t *testing.T) {
	t.Parallel()

	api, _ := newAdminClient(t)
	client, err := camundaadmin.NewUserClient(camundaadmin.UserBinding{
		Endpoint: api.URL(),
		Version:  "8.9.9",
		Auth:     camundaadmin.Auth{Username: adminUsername, Password: "stale"},
	})
	require.NoError(t, err)

	err = client.CreateUser(
		context.Background(), camundaadmin.User{Username: "web-modeler"}, "generated-password",
	)

	require.ErrorIs(t, err, camundaadmin.ErrUnauthenticated)
}

func TestDeleteUserRemovesTheUser(t *testing.T) {
	t.Parallel()

	api, client := newAdminClient(t)
	api.SetUser("web-modeler", "Web Modeler", "web-modeler@example.com", "secret")

	require.NoError(t, client.DeleteUser(context.Background(), "web-modeler"))

	assert.False(t, api.Exists("web-modeler"))
}

// A finalizer runs again after a partial failure, so a user that is already
// gone is the state it asked for.
func TestDeleteUserAcceptsAUserThatIsGone(t *testing.T) {
	t.Parallel()

	_, client := newAdminClient(t)

	assert.NoError(t, client.DeleteUser(context.Background(), "web-modeler"))
}

func TestSearchAuthorizationsReturnsTheRowsOfTheOwner(t *testing.T) {
	t.Parallel()

	api, client := newAdminClient(t)
	api.SetAuthorization("web-modeler", "USER", "RESOURCE", "*", "CREATE")
	api.SetAuthorization("someone-else", "USER", "RESOURCE", "*", "CREATE")

	found, err := client.SearchAuthorizations(
		context.Background(), camundaadmin.OwnerUser, "web-modeler", 100,
	)
	require.NoError(t, err)

	assert.Equal(
		t, []camundaadmin.Authorization{{
			OwnerID:         "web-modeler",
			OwnerType:       camundaadmin.OwnerUser,
			ResourceType:    "RESOURCE",
			ResourceID:      "*",
			PermissionTypes: []string{"CREATE"},
		}}, found,
	)
}

func TestSearchAuthorizationsReturnsNothingForAnOwnerWithNoRows(t *testing.T) {
	t.Parallel()

	_, client := newAdminClient(t)

	found, err := client.SearchAuthorizations(
		context.Background(), camundaadmin.OwnerUser, "web-modeler", 100,
	)

	require.NoError(t, err)
	assert.Empty(t, found)
}

func TestSearchAuthorizationsReportsARefusedCall(t *testing.T) {
	t.Parallel()

	api, client := newAdminClient(t)
	api.FailNext("searchAuthorizations", 1)

	_, err := client.SearchAuthorizations(
		context.Background(), camundaadmin.OwnerUser, "web-modeler", 100,
	)

	require.ErrorIs(t, err, camundaadmin.ErrRejected)
}

// The permissions of one owner. The desired set of the operator is two rows,
// one per resource type.
func desiredAuthorizations() []camundaadmin.Authorization {
	return []camundaadmin.Authorization{
		{
			OwnerID:         "web-modeler",
			OwnerType:       camundaadmin.OwnerUser,
			ResourceType:    "RESOURCE",
			ResourceID:      "*",
			PermissionTypes: []string{"CREATE"},
		},
		{
			OwnerID:         "web-modeler",
			OwnerType:       camundaadmin.OwnerUser,
			ResourceType:    "PROCESS_DEFINITION",
			ResourceID:      "*",
			PermissionTypes: []string{"CREATE_PROCESS_INSTANCE", "READ_PROCESS_DEFINITION"},
		},
	}
}

func TestMissingAuthorizationsFindsNothingWhenEveryPermissionIsGranted(t *testing.T) {
	t.Parallel()

	assert.Empty(t, camundaadmin.MissingAuthorizations(
		desiredAuthorizations(), desiredAuthorizations(),
	))
}

func TestMissingAuthorizationsReturnsEverythingWhenNothingIsGranted(t *testing.T) {
	t.Parallel()

	assert.Equal(
		t, desiredAuthorizations(), camundaadmin.MissingAuthorizations(desiredAuthorizations(), nil),
	)
}

func TestMissingAuthorizationsReturnsOnlyThePermissionsThatAreAbsent(t *testing.T) {
	t.Parallel()

	granted := []camundaadmin.Authorization{
		{
			OwnerID:         "web-modeler",
			OwnerType:       camundaadmin.OwnerUser,
			ResourceType:    "RESOURCE",
			ResourceID:      "*",
			PermissionTypes: []string{"CREATE"},
		},
		{
			OwnerID:         "web-modeler",
			OwnerType:       camundaadmin.OwnerUser,
			ResourceType:    "PROCESS_DEFINITION",
			ResourceID:      "*",
			PermissionTypes: []string{"READ_PROCESS_DEFINITION"},
		},
	}

	assert.Equal(
		t, []camundaadmin.Authorization{{
			OwnerID:         "web-modeler",
			OwnerType:       camundaadmin.OwnerUser,
			ResourceType:    "PROCESS_DEFINITION",
			ResourceID:      "*",
			PermissionTypes: []string{"CREATE_PROCESS_INSTANCE"},
		}}, camundaadmin.MissingAuthorizations(desiredAuthorizations(), granted),
	)
}

// The cluster holds one row per call, so the permissions of one resource can
// sit on several rows. Together they are the granted set.
func TestMissingAuthorizationsCountsPermissionsSplitAcrossRows(t *testing.T) {
	t.Parallel()

	granted := []camundaadmin.Authorization{
		{
			OwnerID:         "web-modeler",
			OwnerType:       camundaadmin.OwnerUser,
			ResourceType:    "RESOURCE",
			ResourceID:      "*",
			PermissionTypes: []string{"CREATE"},
		},
		{
			OwnerID:         "web-modeler",
			OwnerType:       camundaadmin.OwnerUser,
			ResourceType:    "PROCESS_DEFINITION",
			ResourceID:      "*",
			PermissionTypes: []string{"CREATE_PROCESS_INSTANCE"},
		},
		{
			OwnerID:         "web-modeler",
			OwnerType:       camundaadmin.OwnerUser,
			ResourceType:    "PROCESS_DEFINITION",
			ResourceID:      "*",
			PermissionTypes: []string{"READ_PROCESS_DEFINITION"},
		},
	}

	assert.Empty(t, camundaadmin.MissingAuthorizations(desiredAuthorizations(), granted))
}

func TestMissingAuthorizationsIgnoresAnotherResource(t *testing.T) {
	t.Parallel()

	granted := []camundaadmin.Authorization{{
		OwnerID:         "web-modeler",
		OwnerType:       camundaadmin.OwnerUser,
		ResourceType:    "RESOURCE",
		ResourceID:      "some-process",
		PermissionTypes: []string{"CREATE"},
	}}

	assert.Equal(
		t, desiredAuthorizations(), camundaadmin.MissingAuthorizations(desiredAuthorizations(), granted),
	)
}
