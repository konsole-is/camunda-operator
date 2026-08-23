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

package camundaadmin

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"

	"github.com/konsole-is/camunda-operator/pkg/adminhttp"
)

// OwnerType is what an authorization grants its permissions to
// (https://docs.camunda.io/docs/apis-tools/orchestration-cluster-api-rest/specifications/create-authorization/).
type OwnerType string

// The owner types that this operator grants to. The API accepts more of them.
const (
	// OwnerUser is a user of the orchestration cluster, named by its
	// username.
	OwnerUser OwnerType = "USER"
	// OwnerRole is a role, named by its role id. Everything the role is on
	// inherits the permissions.
	OwnerRole OwnerType = "ROLE"
)

// Authorization grants permissions on one resource to one owner
// (https://docs.camunda.io/docs/components/concepts/access-control/authorizations/).
// The orchestration cluster enforces it only while authorizations are enabled
// on that cluster.
type Authorization struct {
	// OwnerID is the username, the role id, or the group id of the owner.
	OwnerID string
	// OwnerType is what OwnerID names.
	OwnerType OwnerType
	// ResourceType is the kind of resource, for example RESOURCE or
	// PROCESS_DEFINITION.
	ResourceType string
	// ResourceID names the resource, or "*" for every resource of that type.
	// The cluster does not check that it exists, so an authorization can be
	// created before the resource it covers.
	ResourceID string
	// PermissionTypes are the actions to grant, for example CREATE or
	// CREATE_PROCESS_INSTANCE. The set is unique to the resource type.
	PermissionTypes []string
}

// AssignRole gives roleID to username through
// PUT /v2/roles/{roleId}/users/{username}
// (https://docs.camunda.io/docs/apis-tools/orchestration-cluster-api-rest/specifications/assign-role-to-user/).
// The user inherits every authorization of the role.
//
// A role the user already holds is ErrAlreadyExists, so a caller that
// converges towards a state reads it as success. A missing role or user is
// ErrRejected, and a wrong credential is ErrUnauthenticated.
func (c *UserClient) AssignRole(ctx context.Context, roleID, username string) error {
	_, status, err := c.api.Do(ctx, adminhttp.Request{
		Method: http.MethodPut,
		Path:   "/v2/roles/" + url.PathEscape(roleID) + "/users/" + url.PathEscape(username),
		Accept: adminhttp.Status(http.StatusNoContent),
	})

	return classifyUserError(err, status)
}

// CreateAuthorization creates authorization through POST /v2/authorizations
// (https://docs.camunda.io/docs/apis-tools/orchestration-cluster-api-rest/specifications/create-authorization/).
//
// The endpoint creates, it does not converge: calling it twice with the same
// authorization leaves the cluster with two of them. A caller therefore grants
// the permissions of an owner once, when it creates that owner, and never on
// every reconcile.
func (c *UserClient) CreateAuthorization(ctx context.Context, authorization Authorization) error {
	body, err := json.Marshal(struct {
		OwnerID         string   `json:"ownerId"`
		OwnerType       string   `json:"ownerType"`
		ResourceID      string   `json:"resourceId"`
		ResourceType    string   `json:"resourceType"`
		PermissionTypes []string `json:"permissionTypes"`
	}{
		OwnerID:         authorization.OwnerID,
		OwnerType:       string(authorization.OwnerType),
		ResourceID:      authorization.ResourceID,
		ResourceType:    authorization.ResourceType,
		PermissionTypes: authorization.PermissionTypes,
	})
	if err != nil {
		return fmt.Errorf("encoding request: %w", err)
	}

	_, status, err := c.api.Do(ctx, adminhttp.Request{
		Method: http.MethodPost,
		Path:   "/v2/authorizations",
		Body:   body,
		Accept: adminhttp.Status(http.StatusCreated),
	})

	return classifyUserError(err, status)
}
