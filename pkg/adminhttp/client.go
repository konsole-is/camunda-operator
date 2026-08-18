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

// Package adminhttp is the HTTP core of the administration clients of this
// operator. It holds the one request path they share: the timeout, the TLS
// trust, basic auth, and the body cap. It also classifies every failure as
// an unreachable server or a rejected call.
//
// A client package keeps its own typed methods and its own sentinels. It
// names the sentinels in the Config, and every error of this package wraps
// them. A caller therefore reads the vocabulary of the API it called, and
// there is one implementation underneath.
package adminhttp

import (
	"bytes"
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
	"unicode/utf8"
)

// requestTimeout bounds one call. The operator calls these APIs from a
// reconcile, where a call that never answers holds a worker.
const requestTimeout = 30 * time.Second

// bodyLimit bounds how much of a response the client reads. An
// administration API answers a status or an error, never a stream, so a
// larger body is a misdirected request or a hostile endpoint.
const bodyLimit = 1 << 20

// errorBodyLimit bounds how much of a rejected response body an error message
// carries. Do still returns everything it read, up to bodyLimit, so a caller
// that parses the body of a rejection sees more than its message does. Only
// the message is bounded, so an error can land in a condition or an event
// without exceeding their limits.
const errorBodyLimit = 1 << 10

// BasicAuth authenticates the calls of a client with HTTP basic auth. The
// zero value sends no credentials.
type BasicAuth struct {
	// Username and Password are sent when either one is set.
	Username string
	Password string
}

// Config builds a Client.
type Config struct {
	// Endpoint is the base URL of the API, with or without a trailing
	// slash. A trailing one is trimmed, so it never doubles the leading
	// slash of a path.
	Endpoint string
	// Auth authenticates every request.
	Auth BasicAuth
	// CABundle is the PEM bundle that verifies the TLS certificate of the
	// endpoint. nil means the system pool.
	//
	// A bundle that is present but holds no certificate is an error, not a
	// fallback: an empty or unparseable bundle means the caller read the
	// wrong Secret key, and the system pool would hide that as an
	// unexplained TLS failure on every call. Only a caller that deliberately
	// passes nil gets the system pool.
	CABundle []byte
	// Unreachable is the sentinel that every transport failure wraps: the
	// server did not answer. It is required.
	Unreachable error
	// Rejected is the sentinel that every unaccepted status wraps: the
	// server answered with an error. It is required.
	Rejected error
}

// Client sends the requests of one administration client.
type Client struct {
	base        string
	auth        BasicAuth
	unreachable error
	rejected    error
	http        *http.Client
}

// New builds a client from cfg. It returns an error when a sentinel is
// missing or the CA bundle is unusable.
func New(cfg Config) (*Client, error) {
	if cfg.Unreachable == nil || cfg.Rejected == nil {
		return nil, errors.New("the Unreachable and the Rejected sentinel are both required")
	}

	transport := http.DefaultTransport.(*http.Transport).Clone()
	if cfg.CABundle != nil {
		if len(cfg.CABundle) == 0 {
			return nil, errors.New("the CA bundle is empty")
		}
		pool := x509.NewCertPool()
		if !pool.AppendCertsFromPEM(cfg.CABundle) {
			return nil, errors.New("the CA bundle holds no PEM certificate")
		}
		transport.TLSClientConfig = &tls.Config{RootCAs: pool, MinVersion: tls.VersionTLS12}
	}

	return &Client{
		base:        strings.TrimRight(cfg.Endpoint, "/"),
		auth:        cfg.Auth,
		unreachable: cfg.Unreachable,
		rejected:    cfg.Rejected,
		http:        &http.Client{Timeout: requestTimeout, Transport: transport},
	}, nil
}

// Request is one call of a Client.
type Request struct {
	// Method is the HTTP method.
	Method string
	// Path is the path of the call. It starts with a slash and is appended
	// to the endpoint of the client as it stands. It carries the query
	// string when the call has one.
	Path string
	// Body is the JSON request body, or nil for a call that sends none. A
	// request that carries one is sent as application/json.
	Body []byte
	// Accept reports whether the response status is a success. nil accepts
	// every 2xx. An API that answers one exact status per call names it with
	// Status.
	Accept func(status int) bool
}

// Status returns an Accept predicate that accepts want and nothing else.
func Status(want int) func(status int) bool {
	return func(status int) bool { return status == want }
}

// Do sends req and returns the response body and the status code. A
// transport failure wraps the Unreachable sentinel of the client. A status
// that req.Accept rejects wraps the Rejected sentinel with the response body
// in the message. The caller reads the returned status for the branches that
// are not failures.
func (c *Client) Do(ctx context.Context, req Request) ([]byte, int, error) {
	var reader io.Reader
	if req.Body != nil {
		reader = bytes.NewReader(req.Body)
	}

	request, err := http.NewRequestWithContext(ctx, req.Method, c.base+req.Path, reader)
	if err != nil {
		return nil, 0, fmt.Errorf("building request: %w", err)
	}
	if req.Body != nil {
		request.Header.Set("Content-Type", "application/json")
	}
	if c.auth.Username != "" || c.auth.Password != "" {
		request.SetBasicAuth(c.auth.Username, c.auth.Password)
	}

	resp, err := c.http.Do(request)
	if err != nil {
		return nil, 0, fmt.Errorf("%w: %s %s: %w", c.unreachable, req.Method, req.Path, err)
	}
	defer func() { _ = resp.Body.Close() }()

	payload, err := io.ReadAll(io.LimitReader(resp.Body, bodyLimit))
	if err != nil {
		return nil, resp.StatusCode, fmt.Errorf(
			"%w: reading response of %s %s: %w", c.unreachable, req.Method, req.Path, err,
		)
	}

	accept := req.Accept
	if accept == nil {
		accept = successful
	}
	if !accept(resp.StatusCode) {
		return payload, resp.StatusCode, fmt.Errorf(
			"%w: %s %s returned %d: %s",
			c.rejected, req.Method, req.Path, resp.StatusCode, errorBody(payload),
		)
	}

	return payload, resp.StatusCode, nil
}

func successful(status int) bool {
	return status >= 200 && status <= 299
}

// errorBody returns the trimmed body for an error message, cut to
// errorBodyLimit bytes on a rune boundary and marked when cut.
func errorBody(payload []byte) string {
	body := strings.TrimSpace(string(payload))
	if len(body) <= errorBodyLimit {
		return body
	}
	cut := body[:errorBodyLimit]
	for !utf8.ValidString(cut) {
		cut = cut[:len(cut)-1]
	}

	return fmt.Sprintf("%s... (truncated, %d bytes)", cut, len(body))
}
