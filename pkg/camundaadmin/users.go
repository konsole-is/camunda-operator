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
	"strconv"
	"strings"

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
	// Version is the Camunda version of the cluster, for example 8.9.9. It
	// is checked against a floor only: the user API endpoints are stable
	// across minors, unlike the actuator endpoints of Binding.Version.
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

// userAPIVersionFloor is the oldest Camunda version whose user API this
// client calls. It is the floor the operator itself supports.
const userAPIVersionFloor = "8.9"

// NewUserClient builds a client for the cluster that binding describes. It
// returns an error when the endpoint is empty or the Camunda version is
// below the floor.
func NewUserClient(binding UserBinding) (*UserClient, error) {
	if binding.Endpoint == "" {
		return nil, errors.New("user API binding has no endpoint")
	}

	if err := checkUserAPIVersion(binding.Version); err != nil {
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

// checkUserAPIVersion rejects a Camunda version below userAPIVersionFloor. A
// later minor or major passes: the user API endpoints keep their shape
// across minors, so the operator must call every version it accepts. This is
// why the user client does not share the endpoint-set check of Client.
func checkUserAPIVersion(version string) error {
	major, minor, err := majorMinor(version)
	if err != nil {
		return err
	}
	floorMajor, floorMinor, _ := majorMinor(userAPIVersionFloor)

	if major < floorMajor || (major == floorMajor && minor < floorMinor) {
		return fmt.Errorf(
			"unsupported Camunda version %q: the user API needs %s or later", version, userAPIVersionFloor,
		)
	}

	return nil
}

// majorMinor reads the first two segments of a version such as 8.9 or
// 8.10.1. A segment that follows the minor is ignored.
func majorMinor(version string) (major int, minor int, err error) {
	segments := strings.Split(version, ".")
	if len(segments) < 2 {
		return 0, 0, fmt.Errorf("unsupported Camunda version %q: it is not of the form major.minor", version)
	}

	if major, err = strconv.Atoi(segments[0]); err != nil {
		return 0, 0, fmt.Errorf("unsupported Camunda version %q: its major is not a number", version)
	}
	if minor, err = strconv.Atoi(segments[1]); err != nil {
		return 0, 0, fmt.Errorf("unsupported Camunda version %q: its minor is not a number", version)
	}

	return major, minor, nil
}
