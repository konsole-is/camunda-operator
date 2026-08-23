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

// newRoleClient starts a fake user API with an administrator and returns a
// client authenticated as that administrator.
func newRoleClient(t *testing.T) (*camundaadmintest.UserAPI, *camundaadmin.UserClient) {
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

func TestAssignRoleGivesTheRoleToTheUser(t *testing.T) {
	t.Parallel()

	api, client := newRoleClient(t)
	api.SetUser("web-modeler", "Web Modeler", "web-modeler@example.com", "secret")

	require.NoError(t, client.AssignRole(context.Background(), "admin", "web-modeler"))

	assert.Equal(t, []string{"admin"}, api.Roles("web-modeler"))
}

// A role the user already holds means the state the caller asked for, so the
// error names it and a converging caller reads it as success.
func TestAssignRoleReportsARoleTheUserAlreadyHolds(t *testing.T) {
	t.Parallel()

	api, client := newRoleClient(t)
	api.SetUser("web-modeler", "Web Modeler", "web-modeler@example.com", "secret")
	require.NoError(t, client.AssignRole(context.Background(), "admin", "web-modeler"))

	err := client.AssignRole(context.Background(), "admin", "web-modeler")

	require.ErrorIs(t, err, camundaadmin.ErrAlreadyExists)
	require.ErrorIs(t, err, camundaadmin.ErrRejected)
}

func TestAssignRoleReportsAMissingUser(t *testing.T) {
	t.Parallel()

	_, client := newRoleClient(t)

	err := client.AssignRole(context.Background(), "admin", "nobody")

	require.ErrorIs(t, err, camundaadmin.ErrRejected)
	assert.NotErrorIs(t, err, camundaadmin.ErrAlreadyExists)
}

func TestCreateAuthorizationRecordsThePermissions(t *testing.T) {
	t.Parallel()

	api, client := newRoleClient(t)

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

	api, client := newRoleClient(t)
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

	api, client := newRoleClient(t)
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

	api, client := newRoleClient(t)

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

	api, client := newRoleClient(t)
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

	api, _ := newRoleClient(t)
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

	api, client := newRoleClient(t)
	api.SetUser("web-modeler", "Web Modeler", "web-modeler@example.com", "secret")

	require.NoError(t, client.DeleteUser(context.Background(), "web-modeler"))

	assert.False(t, api.Exists("web-modeler"))
}

// A finalizer runs again after a partial failure, so a user that is already
// gone is the state it asked for.
func TestDeleteUserAcceptsAUserThatIsGone(t *testing.T) {
	t.Parallel()

	_, client := newRoleClient(t)

	assert.NoError(t, client.DeleteUser(context.Background(), "web-modeler"))
}
