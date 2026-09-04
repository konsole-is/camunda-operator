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
	"io"
	"net/http"
	"net/http/httptest"
	"slices"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// call is one request that the fake Keycloak answered.
type call struct {
	method string
	path   string
	query  string
	auth   string
	body   string
}

// fakeKeycloak is a Keycloak that serves the token endpoint and the clients
// collection of one realm. handler answers everything under /admin.
//
// token and handler are set before the server starts. The recorded calls are
// written by the goroutine of the server and read by the test, so they go
// through mu.
type fakeKeycloak struct {
	token   string
	handler func(w http.ResponseWriter, r *http.Request, body string)

	mu    sync.Mutex
	calls []call
}

// start runs the fake behind an httptest server and returns its base URL.
func (f *fakeKeycloak) start(t *testing.T) string {
	t.Helper()

	server := httptest.NewServer(f.serve(t))
	t.Cleanup(server.Close)

	return server.URL + "/auth"
}

// serve records every request, answers the token endpoint, and passes
// everything else to handler.
func (f *fakeKeycloak) serve(t *testing.T) http.HandlerFunc {
	t.Helper()

	// t.FailNow is only for the goroutine of the test, so this handler reports
	// through assert and answers the request itself.
	return func(w http.ResponseWriter, r *http.Request) {
		raw, err := io.ReadAll(r.Body)
		if !assert.NoError(t, err) {
			w.WriteHeader(http.StatusInternalServerError)

			return
		}

		f.mu.Lock()
		f.calls = append(f.calls, call{
			method: r.Method,
			path:   r.URL.Path,
			query:  r.URL.RawQuery,
			auth:   r.Header.Get("Authorization"),
			body:   string(raw),
		})
		f.mu.Unlock()

		if r.URL.Path == "/auth/realms/master/protocol/openid-connect/token" {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"access_token":"` + f.token + `"}`))

			return
		}

		f.handler(w, r, string(raw))
	}
}

// recorded returns the requests that the fake Keycloak has answered.
func (f *fakeKeycloak) recorded() []call {
	f.mu.Lock()
	defer f.mu.Unlock()

	return slices.Clone(f.calls)
}

func TestFindClient(t *testing.T) {
	t.Parallel()

	fake := &fakeKeycloak{token: "an-access-token"}
	fake.handler = func(w http.ResponseWriter, _ *http.Request, _ string) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`[{"id":"6c4c","clientId":"optimize","redirectUris":["https://a/cb"]}]`))
	}

	stored, err := New(fake.start(t), "camunda-platform", "admin", "secret").
		FindClient(context.Background(), "optimize")
	require.NoError(t, err)

	assert.Equal(t, "6c4c", stored.ID())
	assert.Equal(t, []string{"https://a/cb"}, stored.RedirectURIs())

	calls := fake.recorded()
	require.Len(t, calls, 2)
	assert.Contains(t, calls[0].body, "grant_type=password")
	assert.Contains(t, calls[0].body, "client_id=admin-cli")
	assert.Contains(t, calls[0].body, "username=admin")
	assert.Contains(t, calls[0].body, "password=secret")
	assert.Equal(t, "/auth/admin/realms/camunda-platform/clients", calls[1].path)
	assert.Equal(t, "clientId=optimize", calls[1].query)
	assert.Equal(t, "Bearer an-access-token", calls[1].auth)
}

// A realm that Management Identity has not bootstrapped yet holds no Optimize
// client, and the administration API answers that with an empty list.
func TestFindClientWithoutAMatch(t *testing.T) {
	t.Parallel()

	fake := &fakeKeycloak{token: "an-access-token"}
	fake.handler = func(w http.ResponseWriter, _ *http.Request, _ string) {
		_, _ = w.Write([]byte(`[]`))
	}

	stored, err := New(fake.start(t), "camunda-platform", "admin", "secret").
		FindClient(context.Background(), "optimize")

	require.NoError(t, err)
	assert.Nil(t, stored)
}

// The update carries the whole representation that the lookup returned, so a
// field this package never reads survives it.
func TestUpdateClientSendsTheWholeRepresentation(t *testing.T) {
	t.Parallel()

	fake := &fakeKeycloak{token: "an-access-token"}
	fake.handler = func(w http.ResponseWriter, r *http.Request, _ string) {
		if r.Method == http.MethodGet {
			_, _ = w.Write([]byte(
				`[{"id":"6c4c","clientId":"optimize","webOrigins":["+"],"redirectUris":["https://a/cb"]}]`,
			))

			return
		}
		w.WriteHeader(http.StatusNoContent)
	}

	client := New(fake.start(t), "camunda-platform", "admin", "secret")
	ctx := context.Background()

	stored, err := client.FindClient(ctx, "optimize")
	require.NoError(t, err)

	stored.SetRedirectURIs([]string{"https://a/cb", "https://b/cb"})
	require.NoError(t, client.UpdateClient(ctx, stored))

	calls := fake.recorded()
	update := calls[len(calls)-1]
	assert.Equal(t, http.MethodPut, update.method)
	assert.Equal(t, "/auth/admin/realms/camunda-platform/clients/6c4c", update.path)

	var sent map[string]any
	require.NoError(t, json.Unmarshal([]byte(update.body), &sent))
	assert.Equal(t, "optimize", sent["clientId"])
	assert.Equal(t, []any{"+"}, sent["webOrigins"])
	assert.Equal(t, []any{"https://a/cb", "https://b/cb"}, sent["redirectUris"])
}

// The administrator signs in once, and the calls after that reuse the token.
func TestClientSignsInOnce(t *testing.T) {
	t.Parallel()

	fake := &fakeKeycloak{token: "an-access-token"}
	fake.handler = func(w http.ResponseWriter, _ *http.Request, _ string) {
		_, _ = w.Write([]byte(`[]`))
	}

	client := New(fake.start(t), "camunda-platform", "admin", "secret")
	ctx := context.Background()

	_, err := client.FindClient(ctx, "optimize")
	require.NoError(t, err)
	_, err = client.FindClient(ctx, "optimize")
	require.NoError(t, err)

	tokenCalls := 0
	for _, c := range fake.recorded() {
		if c.path == "/auth/realms/master/protocol/openid-connect/token" {
			tokenCalls++
		}
	}
	assert.Equal(t, 1, tokenCalls)
}

func TestFindClientCarriesTheRefusalOfKeycloak(t *testing.T) {
	t.Parallel()

	fake := &fakeKeycloak{token: "an-access-token"}
	fake.handler = func(w http.ResponseWriter, _ *http.Request, _ string) {
		w.WriteHeader(http.StatusForbidden)
		_, _ = w.Write([]byte(`{"error":"unknown_error"}`))
	}

	_, err := New(fake.start(t), "camunda-platform", "admin", "secret").
		FindClient(context.Background(), "optimize")

	require.Error(t, err)
	assert.Contains(t, err.Error(), "status 403 from Keycloak")
	assert.Contains(t, err.Error(), "unknown_error")
}

func TestSignInCarriesTheRefusalOfKeycloak(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"error":"invalid_grant"}`))
	}))
	t.Cleanup(server.Close)

	_, err := New(server.URL, "camunda-platform", "admin", "wrong").
		FindClient(context.Background(), "optimize")

	require.Error(t, err)
	assert.Contains(t, err.Error(), "status 401 from Keycloak")
	assert.Contains(t, err.Error(), "invalid_grant")
}

// A token that Keycloak refuses is exchanged once and the request is sent
// again, body and method intact.
func TestUpdateClientRetriesOnceAfterARefusedToken(t *testing.T) {
	t.Parallel()

	// The handler runs on the goroutine of the server and the test reads the
	// count, so both counters are atomic.
	var tokens atomic.Int64
	var refused atomic.Bool
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/protocol/openid-connect/token") {
			issued := strconv.FormatInt(tokens.Add(1), 10)
			_, _ = w.Write([]byte(`{"access_token":"token-` + issued + `"}`))

			return
		}
		if !refused.Swap(true) {
			w.WriteHeader(http.StatusUnauthorized)

			return
		}

		raw, err := io.ReadAll(r.Body)
		if !assert.NoError(t, err) {
			w.WriteHeader(http.StatusInternalServerError)

			return
		}
		assert.Equal(t, http.MethodPut, r.Method)
		assert.Equal(t, "Bearer token-2", r.Header.Get("Authorization"))

		var sent map[string]any
		if !assert.NoError(t, json.Unmarshal(raw, &sent)) {
			w.WriteHeader(http.StatusInternalServerError)

			return
		}
		assert.Equal(t, []any{"https://a/cb"}, sent["redirectUris"])

		w.WriteHeader(http.StatusNoContent)
	}))
	t.Cleanup(server.Close)

	rep := Representation{"id": "6c4c", "clientId": "optimize"}
	rep.SetRedirectURIs([]string{"https://a/cb"})

	require.NoError(t, New(server.URL, "camunda-platform", "admin", "secret").
		UpdateClient(context.Background(), rep))
	assert.Equal(t, int64(2), tokens.Load())
}

// A Keycloak that the operator does not run is on the other end, so an answer
// that never stops must not grow the operator until it is killed.
func TestFindClientRefusesAnOversizedAnswer(t *testing.T) {
	t.Parallel()

	fake := &fakeKeycloak{token: "an-access-token"}
	fake.handler = func(w http.ResponseWriter, _ *http.Request, _ string) {
		chunk := bytes.Repeat([]byte("a"), 1<<20)
		for range 8 {
			if _, err := w.Write(chunk); err != nil {
				return
			}
		}
	}

	_, err := New(fake.start(t), "camunda-platform", "admin", "secret").
		FindClient(context.Background(), "optimize")

	require.Error(t, err)
	assert.Contains(t, err.Error(), "longer than")
}

// A redirect on the sign-in would make Go replay the form body, which carries
// the administrator name and password, at whatever host the answer names.
func TestSignInRefusesToFollowARedirect(t *testing.T) {
	t.Parallel()

	// The client never follows the redirect, so nothing orders the handler of
	// the redirect target against the test. The flag is therefore atomic.
	var leaked atomic.Bool
	elsewhere := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		raw, _ := io.ReadAll(r.Body)
		if strings.Contains(string(raw), "password=secret") {
			leaked.Store(true)
		}
	}))
	t.Cleanup(elsewhere.Close)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Redirect(w, &http.Request{}, elsewhere.URL, http.StatusTemporaryRedirect)
	}))
	t.Cleanup(server.Close)

	_, err := New(server.URL, "camunda-platform", "admin", "secret").
		FindClient(context.Background(), "optimize")

	require.Error(t, err)
	assert.False(
		t, leaked.Load(), "the administrator credentials were replayed at the redirect target",
	)
}

func TestUpdateClientWithoutAnID(t *testing.T) {
	t.Parallel()

	err := New("http://keycloak.example.com", "camunda-platform", "admin", "secret").
		UpdateClient(context.Background(), Representation{"clientId": "optimize"})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "no id")
}
