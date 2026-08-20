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
	"strings"

	"github.com/konsole-is/camunda-operator/pkg/adminhttp/adminhttptest"
)

// userRecord is the stored profile of one fake user.
type userRecord struct {
	name     string
	email    string
	password string
}

// UserAPI fakes the user endpoints of the orchestration cluster REST API:
// PUT /v2/users/{username} on the gateway HTTP port. Every exported method
// is safe for concurrent use.
//
// The fake authenticates every call with basic auth against its stored
// users, the way the real endpoint does under basic authentication. It
// replaces the name and the email of the updated user with whatever the
// request carries, and it keeps the password when the request carries an
// empty one; that mirrors the overlay of the real update processor.
//
// The operation name for the inherited FailNext and DropNext is
// "updateUser". A failing call answers 500.
type UserAPI struct {
	adminhttptest.Fake

	users         map[string]userRecord
	updateCalls   int
	refusals      int
	refusalReason string
}

// NewUserAPI starts a fake user API with no users. Close it with Close.
func NewUserAPI() *UserAPI {
	api := &UserAPI{users: map[string]userRecord{}}
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

// handle serves the fake. It runs under the lock of the request.
func (s *UserAPI) handle(w http.ResponseWriter, r *http.Request) {
	username, ok := strings.CutPrefix(r.URL.Path, "/v2/users/")
	if !ok || username == "" || r.Method != http.MethodPut {
		problem(w, http.StatusNotFound, "no such endpoint: "+r.Method+" "+r.URL.Path)
		return
	}
	s.updateCalls++

	if s.Dropping(w, "updateUser") {
		return
	}
	if reason, refuse := s.refusing(); refuse {
		problem(w, http.StatusBadRequest, reason)
		return
	}
	if s.Failing("updateUser") {
		problem(w, http.StatusInternalServerError, "injected failure")
		return
	}

	caller, password, ok := r.BasicAuth()
	if callerRecord, exists := s.users[caller]; !ok || !exists || callerRecord.password != password {
		problem(w, http.StatusUnauthorized, "bad credentials")
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

// validEmail mirrors the check that the orchestration cluster runs on an
// update: the domain must carry a dot, so an address such as admin@localhost
// is refused with 400 INVALID_ARGUMENT. The cluster seeds the initial user
// from its configuration without this check, so only an update finds a bad
// address.
func validEmail(email string) bool {
	_, domain, found := strings.Cut(email, "@")
	return found && strings.Contains(domain, ".")
}
