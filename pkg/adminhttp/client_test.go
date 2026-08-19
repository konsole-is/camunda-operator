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

package adminhttp_test

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/konsole-is/camunda-operator/pkg/adminhttp"
)

// The sentinels of a fake client package. Every error of the core wraps the
// sentinels it was built with, which is how each client keeps its own
// vocabulary.
var (
	errUnreachable = errors.New("the fake API did not answer")
	errRejected    = errors.New("the fake API rejected the call")
)

// record is what a request looked like at the server.
type record struct {
	path        string
	auth        string
	contentType string
	body        string
}

// newRecordingClient starts a server that records the request it serves and
// answers with status and body. cfg carries the settings under test; the
// endpoint and the sentinels are filled in.
func newRecordingClient(t *testing.T, cfg adminhttp.Config, status int, body []byte) (*adminhttp.Client, *record) {
	t.Helper()

	last := &record{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		read, _ := io.ReadAll(r.Body)
		*last = record{
			path:        r.URL.RequestURI(),
			auth:        r.Header.Get("Authorization"),
			contentType: r.Header.Get("Content-Type"),
			body:        string(read),
		}

		w.WriteHeader(status)
		_, _ = w.Write(body)
	}))
	t.Cleanup(server.Close)

	cfg.Endpoint = server.URL + cfg.Endpoint
	cfg.Unreachable = errUnreachable
	cfg.Rejected = errRejected
	client, err := adminhttp.New(cfg)
	require.NoError(t, err)

	return client, last
}

// newClient is newRecordingClient with the default settings.
func newClient(t *testing.T, status int, body []byte) (*adminhttp.Client, *record) {
	t.Helper()

	return newRecordingClient(t, adminhttp.Config{}, status, body)
}

// A client without both sentinels would format them into every error as a
// nil verb, which loses the classification the callers branch on.
func TestNewRequiresBothSentinels(t *testing.T) {
	t.Parallel()

	_, err := adminhttp.New(adminhttp.Config{Endpoint: "http://api:9600", Rejected: errRejected})
	require.ErrorContains(t, err, "required")

	_, err = adminhttp.New(adminhttp.Config{Endpoint: "http://api:9600", Unreachable: errUnreachable})
	require.ErrorContains(t, err, "required")
}

// A CA that is present but unusable means the caller read the wrong Secret
// key. Falling back to the system pool would hide that as a TLS failure on
// every later call.
func TestNewRejectsAnUnusableCABundle(t *testing.T) {
	t.Parallel()

	cfg := adminhttp.Config{Endpoint: "https://api:9200", Unreachable: errUnreachable, Rejected: errRejected}

	cfg.CABundle = []byte{}
	_, err := adminhttp.New(cfg)
	require.ErrorContains(t, err, "empty")

	cfg.CABundle = []byte("not a certificate")
	_, err = adminhttp.New(cfg)
	require.ErrorContains(t, err, "no PEM certificate")
}

func TestDoAcceptsEvery2xxByDefault(t *testing.T) {
	t.Parallel()

	for _, status := range []int{http.StatusOK, http.StatusAccepted, http.StatusNoContent} {
		client, _ := newClient(t, status, nil)

		_, got, err := client.Do(context.Background(), adminhttp.Request{Method: http.MethodGet, Path: "/health"})

		require.NoError(t, err)
		assert.Equal(t, status, got)
	}
}

// An API that answers one exact status per call reads any other status as a
// failure, even a 2xx: the 8.9 exporting endpoints answer 200 with the real
// outcome inside the body.
func TestStatusAcceptsOnlyTheNamedStatus(t *testing.T) {
	t.Parallel()

	client, _ := newClient(t, http.StatusOK, []byte("scheduled"))

	_, status, err := client.Do(context.Background(), adminhttp.Request{
		Method: http.MethodPost,
		Path:   "/backup",
		Accept: adminhttp.Status(http.StatusAccepted),
	})

	require.ErrorIs(t, err, errRejected)
	assert.Equal(t, http.StatusOK, status)
	assert.Contains(t, err.Error(), "POST /backup returned 200: scheduled")
}

// A rejection returns the body as well as the error: the caller reads the
// error type out of it to tell one 404 from another.
func TestRejectedCallReturnsTheBodyAndTheStatus(t *testing.T) {
	t.Parallel()

	client, _ := newClient(t, http.StatusNotFound, []byte(`{"error":"missing"}`))

	payload, status, err := client.Do(context.Background(), adminhttp.Request{Method: http.MethodGet, Path: "/x"})

	require.ErrorIs(t, err, errRejected)
	assert.Equal(t, http.StatusNotFound, status)
	assert.JSONEq(t, `{"error":"missing"}`, string(payload))
}

func TestUnreachableEndpoint(t *testing.T) {
	t.Parallel()

	client, err := adminhttp.New(adminhttp.Config{
		Endpoint:    "http://127.0.0.1:1",
		Unreachable: errUnreachable,
		Rejected:    errRejected,
	})
	require.NoError(t, err)

	_, _, err = client.Do(context.Background(), adminhttp.Request{Method: http.MethodGet, Path: "/health"})

	require.ErrorIs(t, err, errUnreachable)
	assert.NotErrorIs(t, err, errRejected)
}

// A rejection carries the response body so an operator can read why. A body
// far larger than a condition allows must not travel whole into the error,
// or every status flush that carries it is refused.
func TestRejectedErrorBoundsTheBody(t *testing.T) {
	t.Parallel()

	client, _ := newClient(t, http.StatusInternalServerError, bytes.Repeat([]byte("x"), 100_000))

	_, _, err := client.Do(context.Background(), adminhttp.Request{Method: http.MethodGet, Path: "/x"})

	require.ErrorIs(t, err, errRejected)
	assert.Less(t, len(err.Error()), 2_000)
	assert.Contains(t, err.Error(), "(truncated, 100000 bytes)")
}

func TestRejectedErrorKeepsASmallBodyWholeAndTrimmed(t *testing.T) {
	t.Parallel()

	client, _ := newClient(t, http.StatusInternalServerError, []byte("  repository is missing  "))

	_, _, err := client.Do(context.Background(), adminhttp.Request{Method: http.MethodGet, Path: "/x"})

	require.ErrorIs(t, err, errRejected)
	assert.True(t, strings.HasSuffix(err.Error(), "returned 500: repository is missing"), err.Error())
	assert.NotContains(t, err.Error(), "truncated")
}

// An administration API answers a status or an error, never a stream. The
// cap keeps a misdirected request from reading a body into the memory of the
// operator.
func TestDoCapsTheBodyItReads(t *testing.T) {
	t.Parallel()

	client, _ := newClient(t, http.StatusOK, bytes.Repeat([]byte("x"), 2<<20))

	payload, _, err := client.Do(context.Background(), adminhttp.Request{Method: http.MethodGet, Path: "/x"})

	require.NoError(t, err)
	assert.Len(t, payload, 1<<20)
}

func TestDoSendsTheRequestTheAPIExpects(t *testing.T) {
	t.Parallel()

	// The endpoint carries a trailing slash, which must not double the
	// leading one of the path.
	client, last := newRecordingClient(
		t, adminhttp.Config{
			Endpoint: "/",
			Auth:     adminhttp.BasicAuth{Username: "camunda", Password: "secret"},
		}, http.StatusOK, nil,
	)

	_, _, err := client.Do(context.Background(), adminhttp.Request{
		Method: http.MethodPost,
		Path:   "/backup?soft=true",
		Body:   []byte(`{"backupId":7}`),
	})

	require.NoError(t, err)
	assert.Equal(t, "/backup?soft=true", last.path)
	assert.Equal(t, "application/json", last.contentType)
	assert.JSONEq(t, `{"backupId":7}`, last.body)
	user, pass, ok := basicAuth(t, last.auth)
	assert.True(t, ok)
	assert.Equal(t, "camunda", user)
	assert.Equal(t, "secret", pass)
}

// A client without credentials sends no Authorization header: the 8.9
// management port is unsecured by default, and an empty basic auth header
// is a credential the API can reject.
func TestDoOmitsAuthenticationWhenThereAreNoCredentials(t *testing.T) {
	t.Parallel()

	client, last := newClient(t, http.StatusOK, nil)

	_, _, err := client.Do(context.Background(), adminhttp.Request{Method: http.MethodGet, Path: "/health"})

	require.NoError(t, err)
	assert.Empty(t, last.auth)
	assert.Empty(t, last.contentType, "a request without a body is not JSON")
}

// basicAuth decodes an Authorization header the way a server does.
func basicAuth(t *testing.T, header string) (user, pass string, ok bool) {
	t.Helper()

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", header)

	return req.BasicAuth()
}
