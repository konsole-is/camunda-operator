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
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
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
type fakeKeycloak struct {
	token   string
	calls   []call
	handler func(w http.ResponseWriter, r *http.Request, body string)
}

// start runs the fake behind an httptest server and returns its base URL.
func (f *fakeKeycloak) start(t *testing.T) string {
	t.Helper()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, err := io.ReadAll(r.Body)
		require.NoError(t, err)
		f.calls = append(f.calls, call{
			method: r.Method,
			path:   r.URL.Path,
			query:  r.URL.RawQuery,
			auth:   r.Header.Get("Authorization"),
			body:   string(raw),
		})

		if r.URL.Path == "/auth/realms/master/protocol/openid-connect/token" {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"access_token":"` + f.token + `"}`))

			return
		}

		f.handler(w, r, string(raw))
	}))
	t.Cleanup(server.Close)

	return server.URL + "/auth"
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

	require.Len(t, fake.calls, 2)
	assert.Contains(t, fake.calls[0].body, "grant_type=password")
	assert.Contains(t, fake.calls[0].body, "client_id=admin-cli")
	assert.Contains(t, fake.calls[0].body, "username=admin")
	assert.Contains(t, fake.calls[0].body, "password=secret")
	assert.Equal(t, "/auth/admin/realms/camunda-platform/clients", fake.calls[1].path)
	assert.Equal(t, "clientId=optimize", fake.calls[1].query)
	assert.Equal(t, "Bearer an-access-token", fake.calls[1].auth)
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

	update := fake.calls[len(fake.calls)-1]
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
	for _, c := range fake.calls {
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
	assert.Contains(t, err.Error(), "403")
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
	assert.Contains(t, err.Error(), "401")
	assert.Contains(t, err.Error(), "invalid_grant")
}

func TestUpdateClientWithoutAnID(t *testing.T) {
	t.Parallel()

	err := New("http://keycloak.example.com", "camunda-platform", "admin", "secret").
		UpdateClient(context.Background(), Representation{"clientId": "optimize"})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "no id")
}
