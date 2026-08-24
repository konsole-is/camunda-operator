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

// GetUser reads user through GET /v2/users/{username}
// (https://docs.camunda.io/docs/apis-tools/orchestration-cluster-api-rest/specifications/get-user/).
// A username the cluster does not hold is reported by the second return
// value, not by an error.
//
// The endpoint is eventually consistent: it reads what the exporter of the
// cluster wrote, so a user that was created moments ago can still be reported
// absent. A caller that acts on the absence must be safe against that answer.
func (c *UserClient) GetUser(ctx context.Context, username string) (User, bool, error) {
	body, status, err := c.api.Do(ctx, adminhttp.Request{
		Method: http.MethodGet,
		Path:   "/v2/users/" + url.PathEscape(username),
		Accept: adminhttp.Status(http.StatusOK),
	})
	if status == http.StatusNotFound {
		return User{}, false, nil
	}
	if err := classifyUserError(err, status); err != nil {
		return User{}, false, err
	}

	var answer struct {
		Username string `json:"username"`
		Name     string `json:"name"`
		Email    string `json:"email"`
	}
	if err := json.Unmarshal(body, &answer); err != nil {
		return User{}, false, fmt.Errorf("decoding the answer: %w", err)
	}

	return User{Username: answer.Username, Name: answer.Name, Email: answer.Email}, true, nil
}

// UpdateUserProfile sets the name and the email of user through PUT
// /v2/users/{username} and leaves the password alone: the endpoint keeps the
// current password when the request carries none. The errors are the ones of
// UpdateUserPassword, because it is the same call.
func (c *UserClient) UpdateUserProfile(ctx context.Context, user User) error {
	return c.putUser(ctx, user, "")
}

// putUser sends one update of user. An empty password leaves the password of
// the user unchanged, which is what the endpoint does with a blank one.
func (c *UserClient) putUser(ctx context.Context, user User, password string) error {
	body, err := json.Marshal(struct {
		Name     string `json:"name"`
		Email    string `json:"email"`
		Password string `json:"password"`
	}{Name: user.Name, Email: user.Email, Password: password})
	if err != nil {
		return fmt.Errorf("encoding request: %w", err)
	}

	_, status, err := c.api.Do(ctx, adminhttp.Request{
		Method: http.MethodPut,
		Path:   "/v2/users/" + url.PathEscape(user.Username),
		Body:   body,
		Accept: adminhttp.Status(http.StatusOK),
	})
	if err != nil && status == http.StatusUnauthorized {
		return fmt.Errorf("%w: %w", ErrUnauthenticated, err)
	}

	return err
}

// UpdateUserPassword sets the password of user through PUT
// /v2/users/{username}, authenticated with the credentials of the client. A
// wrong credential is ErrUnauthenticated, which travels with ErrRejected;
// every other refusal, such as a profile that the cluster does not accept,
// is ErrRejected alone. A caller that holds a second password retries on the
// first and reports the second, whose message carries the answer of the
// cluster. An empty
// password is an error before any call: the endpoint reads an empty
// password as "keep the current one", so the call would report success and
// change nothing.
func (c *UserClient) UpdateUserPassword(ctx context.Context, user User, password string) error {
	if password == "" {
		return errors.New("an empty password never updates: the endpoint keeps the current one")
	}

	return c.putUser(ctx, user, password)
}

// CreateUser creates user with password through POST /v2/users
// (https://docs.camunda.io/docs/apis-tools/orchestration-cluster-api-rest/specifications/create-user/).
// A username that the cluster already holds is ErrAlreadyExists, which travels
// with ErrRejected: the caller decides whether to adopt that user, which it
// can only do by setting a password it knows. A wrong credential is
// ErrUnauthenticated.
func (c *UserClient) CreateUser(ctx context.Context, user User, password string) error {
	body, err := json.Marshal(struct {
		Username string `json:"username"`
		Password string `json:"password"`
		Name     string `json:"name"`
		Email    string `json:"email"`
	}{Username: user.Username, Password: password, Name: user.Name, Email: user.Email})
	if err != nil {
		return fmt.Errorf("encoding request: %w", err)
	}

	_, status, err := c.api.Do(ctx, adminhttp.Request{
		Method: http.MethodPost,
		Path:   "/v2/users",
		Body:   body,
		Accept: adminhttp.Status(http.StatusCreated),
	})

	return classifyUserError(err, status)
}

// DeleteUser removes user through DELETE /v2/users/{username}
// (https://docs.camunda.io/docs/apis-tools/orchestration-cluster-api-rest/specifications/delete-user/).
// A user that the cluster does not hold is success, so a finalizer that runs
// twice reports no failure the second time.
func (c *UserClient) DeleteUser(ctx context.Context, username string) error {
	_, status, err := c.api.Do(ctx, adminhttp.Request{
		Method: http.MethodDelete,
		Path:   "/v2/users/" + url.PathEscape(username),
		Accept: adminhttp.Status(http.StatusNoContent),
	})
	if status == http.StatusNotFound {
		return nil
	}

	return classifyUserError(err, status)
}

// classifyUserError names the two refusals that a caller acts on: a credential
// the cluster did not accept, and a name it already holds. Both travel with
// ErrRejected, so a caller that only separates a rejected call from an
// unreachable one is unaffected.
func classifyUserError(err error, status int) error {
	switch {
	case err == nil:
		return nil
	case status == http.StatusUnauthorized:
		return fmt.Errorf("%w: %w", ErrUnauthenticated, err)
	case status == http.StatusConflict:
		return fmt.Errorf("%w: %w", ErrAlreadyExists, err)
	default:
		return err
	}
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
