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

package camundaadmintest

import (
	"encoding/json"
	"net/http"
	"slices"
	"strings"

	"github.com/konsole-is/camunda-operator/pkg/adminhttp/adminhttptest"
)

// userRecord is the stored profile of one fake user.
type userRecord struct {
	name     string
	email    string
	password string
}

// UserAPI fakes the user, role, and authorization endpoints of the
// orchestration cluster REST API on the gateway HTTP port. Every exported
// method is safe for concurrent use.
//
// The fake authenticates every call with basic auth against its stored
// users, the way the real endpoints do under basic authentication. On an
// update it replaces the name and the email of the user with whatever the
// request carries, and it keeps the password when the request carries an
// empty one; that mirrors the overlay of the real update processor.
//
// The operation names for the inherited FailNext and DropNext are
// "updateUser", "createUser", "deleteUser", "assignRole", and
// "createAuthorization". A failing call answers 500.
type UserAPI struct {
	adminhttptest.Fake

	users map[string]userRecord
	// roles holds the role ids of each user, in assignment order.
	roles map[string][]string
	// authorizations holds every created authorization, in creation order,
	// duplicates included: the real endpoint creates rather than converges.
	authorizations []Authorization
	updateCalls    int
	refusals       int
	refusalReason  string
}

// Authorization is one authorization that the fake recorded.
type Authorization struct {
	OwnerID         string   `json:"ownerId"`
	OwnerType       string   `json:"ownerType"`
	ResourceID      string   `json:"resourceId"`
	ResourceType    string   `json:"resourceType"`
	PermissionTypes []string `json:"permissionTypes"`
}

// NewUserAPI starts a fake user API with no users. Close it with Close.
func NewUserAPI() *UserAPI {
	api := &UserAPI{users: map[string]userRecord{}, roles: map[string][]string{}}
	api.Start(api.handle)

	return api
}

// RefuseNext makes the next n updates answer 400 with reason, the way the
// cluster refuses a profile it does not accept. It is not an authentication
// failure: the credentials are good and a retry with another password can
// never help.
func (s *UserAPI) RefuseNext(n int, reason string) {
	s.Lock()
	defer s.Unlock()
	s.refusals, s.refusalReason = n, reason
}

// refusing consumes one injected refusal and reports whether the handler
// must answer with one. It runs under the lock of the request.
func (s *UserAPI) refusing() (string, bool) {
	if s.refusals <= 0 {
		return "", false
	}
	s.refusals--

	return s.refusalReason, true
}

// SetUser stores or replaces a user, so a caller can authenticate as it and
// update it.
func (s *UserAPI) SetUser(username, name, email, password string) {
	s.Lock()
	defer s.Unlock()
	s.users[username] = userRecord{name: name, email: email, password: password}
}

// Password returns the stored password of username, or "" when the user does
// not exist.
func (s *UserAPI) Password(username string) string {
	s.Lock()
	defer s.Unlock()

	return s.users[username].password
}

// Profile returns the stored name and email of username.
func (s *UserAPI) Profile(username string) (name, email string) {
	s.Lock()
	defer s.Unlock()
	user := s.users[username]

	return user.name, user.email
}

// Exists reports whether the fake holds username.
func (s *UserAPI) Exists(username string) bool {
	s.Lock()
	defer s.Unlock()
	_, exists := s.users[username]

	return exists
}

// Roles returns the role ids assigned to username, in assignment order.
func (s *UserAPI) Roles(username string) []string {
	s.Lock()
	defer s.Unlock()

	return slices.Clone(s.roles[username])
}

// Authorizations returns every authorization the fake created, in creation
// order.
func (s *UserAPI) Authorizations() []Authorization {
	s.Lock()
	defer s.Unlock()

	return slices.Clone(s.authorizations)
}

// UpdateCalls counts the update requests that reached the fake, the rejected
// ones included.
func (s *UserAPI) UpdateCalls() int {
	s.Lock()
	defer s.Unlock()

	return s.updateCalls
}

// problem answers with an application/problem+json body, the error shape of
// the real API.
func problem(w http.ResponseWriter, status int, detail string) {
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]any{
		"type": "about:blank", "title": http.StatusText(status), "status": status, "detail": detail,
	})
}

// handle routes one request to the endpoint it names. It runs under the lock
// of the request.
func (s *UserAPI) handle(w http.ResponseWriter, r *http.Request) {
	switch {
	case r.Method == http.MethodPost && r.URL.Path == "/v2/users":
		s.createUser(w, r)
	case r.Method == http.MethodPost && r.URL.Path == "/v2/authorizations":
		s.createAuthorization(w, r)
	case r.Method == http.MethodPut && strings.HasPrefix(r.URL.Path, "/v2/roles/"):
		s.assignRole(w, r)
	case r.Method == http.MethodPut && strings.HasPrefix(r.URL.Path, "/v2/users/"):
		s.updateUser(w, r)
	case r.Method == http.MethodDelete && strings.HasPrefix(r.URL.Path, "/v2/users/"):
		s.deleteUser(w, r)
	default:
		problem(w, http.StatusNotFound, "no such endpoint: "+r.Method+" "+r.URL.Path)
	}
}

// updateUser serves PUT /v2/users/{username}.
func (s *UserAPI) updateUser(w http.ResponseWriter, r *http.Request) {
	username := strings.TrimPrefix(r.URL.Path, "/v2/users/")
	if username == "" {
		problem(w, http.StatusNotFound, "no such endpoint: "+r.Method+" "+r.URL.Path)
		return
	}
	s.updateCalls++

	if s.injected(w, "updateUser") || !s.authenticated(w, r) {
		return
	}

	user, exists := s.users[username]
	if !exists {
		problem(w, http.StatusNotFound, "user "+username+" was not found")
		return
	}

	var request struct {
		Name     string `json:"name"`
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		problem(w, http.StatusBadRequest, "decoding request: "+err.Error())
		return
	}

	if request.Email != "" && !validEmail(request.Email) {
		problem(w, http.StatusBadRequest, "The provided email '"+request.Email+"' is not valid.")
		return
	}

	user.name, user.email = request.Name, request.Email
	if request.Password != "" {
		user.password = request.Password
	}
	s.users[username] = user

	adminhttptest.WriteJSON(w, http.StatusOK, map[string]string{
		"username": username, "name": user.name, "email": user.email,
	})
}

// createUser serves POST /v2/users. A username the fake already holds answers
// 409, the way the real endpoint does.
func (s *UserAPI) createUser(w http.ResponseWriter, r *http.Request) {
	if s.injected(w, "createUser") || !s.authenticated(w, r) {
		return
	}

	var request struct {
		Username string `json:"username"`
		Password string `json:"password"`
		Name     string `json:"name"`
		Email    string `json:"email"`
	}
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		problem(w, http.StatusBadRequest, "decoding request: "+err.Error())
		return
	}
	if request.Username == "" || request.Password == "" {
		problem(w, http.StatusBadRequest, "username and password are required")
		return
	}
	if _, exists := s.users[request.Username]; exists {
		problem(w, http.StatusConflict, "user "+request.Username+" already exists")
		return
	}
	if request.Email != "" && !validEmail(request.Email) {
		problem(w, http.StatusBadRequest, "The provided email '"+request.Email+"' is not valid.")
		return
	}

	s.users[request.Username] = userRecord{
		name: request.Name, email: request.Email, password: request.Password,
	}

	adminhttptest.WriteJSON(w, http.StatusCreated, map[string]string{
		"username": request.Username, "name": request.Name, "email": request.Email,
	})
}

// deleteUser serves DELETE /v2/users/{username}.
func (s *UserAPI) deleteUser(w http.ResponseWriter, r *http.Request) {
	username := strings.TrimPrefix(r.URL.Path, "/v2/users/")
	if username == "" {
		problem(w, http.StatusNotFound, "no such endpoint: "+r.Method+" "+r.URL.Path)
		return
	}

	if s.injected(w, "deleteUser") || !s.authenticated(w, r) {
		return
	}

	if _, exists := s.users[username]; !exists {
		problem(w, http.StatusNotFound, "user "+username+" was not found")
		return
	}
	delete(s.users, username)
	delete(s.roles, username)

	w.WriteHeader(http.StatusNoContent)
}

// assignRole serves PUT /v2/roles/{roleId}/users/{username}. A role the user
// already holds answers 409, the way the real endpoint does.
func (s *UserAPI) assignRole(w http.ResponseWriter, r *http.Request) {
	roleID, username, found := strings.Cut(strings.TrimPrefix(r.URL.Path, "/v2/roles/"), "/users/")
	if !found || roleID == "" || username == "" {
		problem(w, http.StatusNotFound, "no such endpoint: "+r.Method+" "+r.URL.Path)
		return
	}

	if s.injected(w, "assignRole") || !s.authenticated(w, r) {
		return
	}

	if _, exists := s.users[username]; !exists {
		problem(w, http.StatusNotFound, "user "+username+" was not found")
		return
	}
	if slices.Contains(s.roles[username], roleID) {
		problem(w, http.StatusConflict, "role "+roleID+" is already assigned to "+username)
		return
	}
	s.roles[username] = append(s.roles[username], roleID)

	w.WriteHeader(http.StatusNoContent)
}

// createAuthorization serves POST /v2/authorizations. It records duplicates,
// because the real endpoint creates rather than converges.
func (s *UserAPI) createAuthorization(w http.ResponseWriter, r *http.Request) {
	if s.injected(w, "createAuthorization") || !s.authenticated(w, r) {
		return
	}

	var request Authorization
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		problem(w, http.StatusBadRequest, "decoding request: "+err.Error())
		return
	}
	if request.OwnerID == "" || request.ResourceType == "" || len(request.PermissionTypes) == 0 {
		problem(w, http.StatusBadRequest, "ownerId, resourceType, and permissionTypes are required")
		return
	}
	s.authorizations = append(s.authorizations, request)

	adminhttptest.WriteJSON(w, http.StatusCreated, request)
}

// injected answers the request with the failure that a test asked for, and
// reports whether it did.
func (s *UserAPI) injected(w http.ResponseWriter, operation string) bool {
	if s.Dropping(w, operation) {
		return true
	}
	if reason, refuse := s.refusing(); refuse {
		problem(w, http.StatusBadRequest, reason)
		return true
	}
	if s.Failing(operation) {
		problem(w, http.StatusInternalServerError, "injected failure")
		return true
	}

	return false
}

// authenticated checks the basic credentials of the caller against the stored
// users and answers 401 when they do not match.
func (s *UserAPI) authenticated(w http.ResponseWriter, r *http.Request) bool {
	caller, password, ok := r.BasicAuth()
	if record, exists := s.users[caller]; !ok || !exists || record.password != password {
		problem(w, http.StatusUnauthorized, "bad credentials")
		return false
	}

	return true
}

// validEmail mirrors the check that the orchestration cluster runs on an
// update: the domain must carry a dot, so an address such as admin@localhost
// is refused with 400 INVALID_ARGUMENT. The cluster seeds the initial user
// from its configuration without this check, so only an update finds a bad
// address.
func validEmail(email string) bool {
	_, domain, found := strings.Cut(email, "@")
	return found && strings.Contains(domain, ".")
}
