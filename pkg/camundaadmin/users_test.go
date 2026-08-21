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

// adminUser is the seeded administrator that the rotation flow updates.
var adminUser = camundaadmin.User{Username: "admin", Name: "admin", Email: "admin@example.com"}

// currentPassword is the password every test seeds the fake with and
// authenticates the client with.
const currentPassword = "old-password"

// newUserClient builds a user client against a fresh fake that holds the
// admin user with currentPassword.
func newUserClient(t *testing.T) (*camundaadmin.UserClient, *camundaadmintest.UserAPI) {
	t.Helper()

	server := camundaadmintest.NewUserAPI()
	t.Cleanup(server.Close)
	server.SetUser(adminUser.Username, adminUser.Name, adminUser.Email, currentPassword)

	client, err := camundaadmin.NewUserClient(camundaadmin.UserBinding{
		Endpoint: server.URL(),
		Version:  "8.9.9",
		Auth:     camundaadmin.Auth{Username: adminUser.Username, Password: currentPassword},
	})
	require.NoError(t, err)

	return client, server
}

func TestNewUserClientChecksTheVersionFloor(t *testing.T) {
	tests := []struct {
		version string
		wantErr bool
	}{
		{"8.9.9", false},
		{"8.9", false},
		{"8.10.0", false},
		{"9.0.0", false},
		{"8.8.3", true},
		{"7.23.0", true},
		{"", true},
		{"latest", true},
	}

	for _, tt := range tests {
		t.Run("version "+tt.version, func(t *testing.T) {
			_, err := camundaadmin.NewUserClient(camundaadmin.UserBinding{
				Endpoint: "http://cluster:8080",
				Version:  tt.version,
			})
			if tt.wantErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.version)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestNewUserClientRequiresAnEndpoint(t *testing.T) {
	_, err := camundaadmin.NewUserClient(camundaadmin.UserBinding{Version: "8.9.9"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "endpoint")
}

func TestUpdateUserPasswordSetsThePassword(t *testing.T) {
	client, server := newUserClient(t)

	err := client.UpdateUserPassword(context.Background(), adminUser, "new-password")
	require.NoError(t, err)

	assert.Equal(t, "new-password", server.Password("admin"))
	name, email := server.Profile("admin")
	assert.Equal(t, "admin", name)
	assert.Equal(t, "admin@example.com", email)
}

func TestUpdateUserPasswordReportsAnEmailThatTheClusterRefuses(t *testing.T) {
	client, server := newUserClient(t)

	user := adminUser
	user.Email = "admin@localhost"
	err := client.UpdateUserPassword(context.Background(), user, "new-password")
	require.ErrorIs(t, err, camundaadmin.ErrRejected)
	assert.NotErrorIs(t, err, camundaadmin.ErrUnauthenticated, "another password answers this the same way")
	assert.Contains(t, err.Error(), "is not valid")

	assert.Equal(t, currentPassword, server.Password("admin"), "a refused update changes nothing")
}

func TestUpdateUserPasswordRejectsAnEmptyPassword(t *testing.T) {
	client, server := newUserClient(t)

	err := client.UpdateUserPassword(context.Background(), adminUser, "")
	require.Error(t, err)

	assert.Equal(t, 0, server.UpdateCalls())
	assert.Equal(t, currentPassword, server.Password("admin"))
}

func TestUpdateUserPasswordReportsARejectedCall(t *testing.T) {
	client, server := newUserClient(t)
	server.SetUser(adminUser.Username, adminUser.Name, adminUser.Email, "changed-out-of-band")

	err := client.UpdateUserPassword(context.Background(), adminUser, "new-password")
	require.ErrorIs(t, err, camundaadmin.ErrRejected)
	assert.ErrorIs(t, err, camundaadmin.ErrUnauthenticated, "a caller with a second password may retry")
	assert.Contains(t, err.Error(), "bad credentials")

	assert.Equal(t, "changed-out-of-band", server.Password("admin"))
}

func TestUpdateUserPasswordReportsAnUnreachableEndpoint(t *testing.T) {
	client, server := newUserClient(t)
	server.DropNext("updateUser", 1)

	err := client.UpdateUserPassword(context.Background(), adminUser, "new-password")
	require.ErrorIs(t, err, camundaadmin.ErrUnreachable)

	assert.Equal(t, currentPassword, server.Password("admin"))
}

func TestUpdateUserProfileKeepsThePassword(t *testing.T) {
	client, server := newUserClient(t)

	updated := adminUser
	updated.Email = "team@example.com"
	require.NoError(t, client.UpdateUserProfile(context.Background(), updated))

	_, email := server.Profile("admin")
	assert.Equal(t, "team@example.com", email)
	assert.Equal(t, currentPassword, server.Password("admin"), "a profile update never touches the password")
}
