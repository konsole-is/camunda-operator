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
	"slices"

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

// SearchAuthorizations returns the authorizations of one owner through POST
// /v2/authorizations/search
// (https://docs.camunda.io/docs/apis-tools/orchestration-cluster-api-rest/specifications/search-authorizations/).
// It reads at most limit of them, in one page: a caller that compares against
// a desired set of a few permissions never needs a second one, and a truncated
// answer would only report a permission as absent that the cluster already
// holds.
//
// The endpoint is eventually consistent: a permission that was granted moments
// ago can still be absent from the answer. A caller that grants what is
// missing must therefore leave the export enough time, or it creates a second
// row of a permission the owner already has.
func (c *UserClient) SearchAuthorizations(
	ctx context.Context,
	ownerType OwnerType,
	ownerID string,
	limit int,
) ([]Authorization, error) {
	body, err := json.Marshal(map[string]any{
		"filter": map[string]any{"ownerId": ownerID, "ownerType": string(ownerType)},
		"page":   map[string]any{"limit": limit},
	})
	if err != nil {
		return nil, fmt.Errorf("encoding request: %w", err)
	}

	answer, status, err := c.api.Do(ctx, adminhttp.Request{
		Method: http.MethodPost,
		Path:   "/v2/authorizations/search",
		Body:   body,
		Accept: adminhttp.Status(http.StatusOK),
	})
	if err := classifyUserError(err, status); err != nil {
		return nil, err
	}

	var page struct {
		Items []struct {
			OwnerID         string   `json:"ownerId"`
			OwnerType       string   `json:"ownerType"`
			ResourceID      string   `json:"resourceId"`
			ResourceType    string   `json:"resourceType"`
			PermissionTypes []string `json:"permissionTypes"`
		} `json:"items"`
	}
	if err := json.Unmarshal(answer, &page); err != nil {
		return nil, fmt.Errorf("decoding the answer: %w", err)
	}

	found := make([]Authorization, 0, len(page.Items))
	for _, item := range page.Items {
		found = append(found, Authorization{
			OwnerID:         item.OwnerID,
			OwnerType:       OwnerType(item.OwnerType),
			ResourceID:      item.ResourceID,
			ResourceType:    item.ResourceType,
			PermissionTypes: item.PermissionTypes,
		})
	}

	return found, nil
}

// MissingAuthorizations returns what to create so that the owner of desired
// holds every permission of it. Each returned authorization carries the
// permissions of one entry of desired that granted does not cover, in the
// order desired names them; an entry that granted covers in full is left out,
// and a fully covered desired set returns nothing.
//
// The permissions of one resource can sit on several rows of granted, because
// the create endpoint adds a row per call, so the cover is their union. A row
// of another owner, another resource type, or another resource id covers
// nothing.
func MissingAuthorizations(desired, granted []Authorization) []Authorization {
	held := map[resource][]string{}
	for _, authorization := range granted {
		key := resourceOf(authorization)
		held[key] = append(held[key], authorization.PermissionTypes...)
	}

	var missing []Authorization
	for _, want := range desired {
		absent := make([]string, 0, len(want.PermissionTypes))
		for _, permission := range want.PermissionTypes {
			if !slices.Contains(held[resourceOf(want)], permission) {
				absent = append(absent, permission)
			}
		}
		if len(absent) == 0 {
			continue
		}

		want.PermissionTypes = absent
		missing = append(missing, want)
	}

	return missing
}

// resource is what an authorization grants on: one owner and one resource.
// The permissions of one such key are what MissingAuthorizations compares.
type resource struct {
	ownerID      string
	ownerType    OwnerType
	resourceType string
	resourceID   string
}

// resourceOf returns the comparison key of authorization.
func resourceOf(authorization Authorization) resource {
	return resource{
		ownerID:      authorization.OwnerID,
		ownerType:    authorization.OwnerType,
		resourceType: authorization.ResourceType,
		resourceID:   authorization.ResourceID,
	}
}
