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
	"errors"
	"fmt"
	"net/http"
	"net/url"

	"github.com/konsole-is/camunda-operator/pkg/adminhttp"
)

// User is the profile of the user that UpdateUserPassword updates. The
// update endpoint replaces the whole profile, so the name and the email
// travel with every update; a call that left them out would clear them on
// the cluster.
type User struct {
	// Username of the user, the path parameter of the update endpoint.
	Username string
	// Name of the user.
	Name string
	// Email of the user.
	Email string
}

// UserBinding locates and authenticates the user API of one cluster: the
// /v2 endpoints of the orchestration cluster REST API, on the HTTP port of
// the process that runs the gateway.
type UserBinding struct {
	// Endpoint is the base URL of the gateway HTTP port, for example
	// http://my-cluster-zeebe.ns.svc:8080.
	Endpoint string
	// Version is the Camunda version of the cluster, for example 8.9.9. Its
	// minor selects the endpoint set, like Binding.Version does.
	Version string
	// Auth authenticates the calls. The user API, unlike the management
	// port, always authenticates its callers; under basic authentication
	// these are the credentials of a cluster user.
	Auth Auth
}

// UserClient calls the user API of one cluster. It is separate from Client
// because the two APIs live on different ports: Client calls the actuator
// endpoints of the management port, UserClient the /v2 endpoints of the
// gateway HTTP port.
type UserClient struct {
	api *adminhttp.Client
}

// NewUserClient builds a client for the cluster that binding describes. It
// returns an error when the endpoint is empty or the Camunda version is not
// one the client knows.
func NewUserClient(binding UserBinding) (*UserClient, error) {
	if binding.Endpoint == "" {
		return nil, errors.New("user API binding has no endpoint")
	}

	if err := checkVersion(binding.Version); err != nil {
		return nil, err
	}

	api, err := adminhttp.New(adminhttp.Config{
		Endpoint:    binding.Endpoint,
		Auth:        binding.Auth,
		Unreachable: ErrUnreachable,
		Rejected:    ErrRejected,
	})
	if err != nil {
		return nil, err
	}

	return &UserClient{api: api}, nil
}

// UpdateUserPassword sets the password of user through PUT
// /v2/users/{username}, authenticated with the credentials of the client. A
// wrong credential is ErrRejected, so a caller that holds a possibly stale
// password can tell a rejected call from an unreachable cluster. An empty
// password is an error before any call: the endpoint reads an empty
// password as "keep the current one", so the call would report success and
// change nothing.
func (c *UserClient) UpdateUserPassword(ctx context.Context, user User, password string) error {
	if password == "" {
		return errors.New("an empty password never updates: the endpoint keeps the current one")
	}

	body, err := json.Marshal(struct {
		Name     string `json:"name"`
		Email    string `json:"email"`
		Password string `json:"password"`
	}{Name: user.Name, Email: user.Email, Password: password})
	if err != nil {
		return fmt.Errorf("encoding request: %w", err)
	}

	_, _, err = c.api.Do(ctx, adminhttp.Request{
		Method: http.MethodPut,
		Path:   "/v2/users/" + url.PathEscape(user.Username),
		Body:   body,
		Accept: adminhttp.Status(http.StatusOK),
	})

	return err
}
