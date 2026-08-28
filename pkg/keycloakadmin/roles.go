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
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
)

// RealmRole is one realm role of the administered realm. The id is what a
// role mapping is written by, and the name is what a token carries.
type RealmRole struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// FindUser returns the internal id of the user of the realm whose username is
// exactly username, or an empty string when the realm holds no such user.
//
// The lookup is the users endpoint of the administration API with exact=true
// (https://www.keycloak.org/docs-api/latest/rest-api/index.html). Without it
// Keycloak searches by prefix and answers with every user whose name starts
// the same.
func (c *Client) FindUser(ctx context.Context, username string) (string, error) {
	query := url.Values{"username": {username}, "exact": {"true"}}

	req, err := http.NewRequestWithContext(
		ctx, http.MethodGet, c.usersURL()+"?"+query.Encode(), nil,
	)
	if err != nil {
		return "", fmt.Errorf("building the user lookup request: %w", err)
	}

	body, err := c.do(ctx, req)
	if err != nil {
		return "", fmt.Errorf("looking up user %q of realm %q: %w", username, c.realm, err)
	}

	var found []struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(body, &found); err != nil {
		return "", fmt.Errorf("reading the answer of the %q user lookup: %w", username, err)
	}
	if len(found) == 0 {
		return "", nil
	}

	return found[0].ID, nil
}

// FindRealmRole returns the realm role named name, or nil when the realm holds
// none. Management Identity creates a role with the client of each component
// it renders, so a role is absent until the component it belongs to is there.
func (c *Client) FindRealmRole(ctx context.Context, name string) (*RealmRole, error) {
	endpoint := c.rolesURL() + "/" + url.PathEscape(name)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("building the role lookup request: %w", err)
	}

	body, status, err := c.authorized(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("looking up role %q of realm %q: %w", name, c.realm, err)
	}
	// The role endpoint addresses one role by name, so a realm without it
	// answers 404. Every other refused answer stays a failure.
	if status == http.StatusNotFound {
		return nil, nil
	}
	if status < 200 || status > 299 {
		return nil, fmt.Errorf(
			"looking up role %q of realm %q: %w", name, c.realm, statusError(status, body),
		)
	}

	var role RealmRole
	if err := json.Unmarshal(body, &role); err != nil {
		return nil, fmt.Errorf("reading the answer of the %q role lookup: %w", name, err)
	}

	return &role, nil
}

// UserRealmRoles returns the realm roles that are mapped to the user, which is
// what a token of that user carries. A role that the user holds through a
// group is not one of them.
func (c *Client) UserRealmRoles(ctx context.Context, userID string) ([]RealmRole, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.roleMappingsURL(userID), nil)
	if err != nil {
		return nil, fmt.Errorf("building the role mapping request: %w", err)
	}

	body, err := c.do(ctx, req)
	if err != nil {
		return nil, fmt.Errorf(
			"reading the realm roles of user %q of realm %q: %w", userID, c.realm, err,
		)
	}

	var held []RealmRole
	if err := json.Unmarshal(body, &held); err != nil {
		return nil, fmt.Errorf("reading the answer of the role mapping lookup: %w", err)
	}

	return held, nil
}

// AddUserRealmRole maps role to the user. Every role the user already holds
// stays, and a role the user holds already is accepted again.
func (c *Client) AddUserRealmRole(ctx context.Context, userID string, role RealmRole) error {
	body, err := json.Marshal([]RealmRole{role})
	if err != nil {
		return fmt.Errorf("building the role mapping: %w", err)
	}

	req, err := http.NewRequestWithContext(
		ctx, http.MethodPost, c.roleMappingsURL(userID), bytes.NewReader(body),
	)
	if err != nil {
		return fmt.Errorf("building the role mapping request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	if _, err := c.do(ctx, req); err != nil {
		return fmt.Errorf(
			"granting role %q to user %q of realm %q: %w", role.Name, userID, c.realm, err,
		)
	}

	return nil
}

// usersURL is the users collection of the administered realm.
func (c *Client) usersURL() string {
	return c.baseURL + "/admin/realms/" + url.PathEscape(c.realm) + "/users"
}

// rolesURL is the realm roles collection of the administered realm. A client
// of the realm holds roles of its own, which this package never reads.
func (c *Client) rolesURL() string {
	return c.baseURL + "/admin/realms/" + url.PathEscape(c.realm) + "/roles"
}

// roleMappingsURL is the realm role mappings of one user.
func (c *Client) roleMappingsURL(userID string) string {
	return c.usersURL() + "/" + url.PathEscape(userID) + "/role-mappings/realm"
}
