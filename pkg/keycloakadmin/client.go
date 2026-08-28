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

// Package keycloakadmin reads and writes clients through the administration
// API of a Keycloak realm.
//
// The operator administers one field of one client with it: the redirect URIs
// of the Optimize client. Management Identity creates every client of the
// realm and rewrites the whole representation from its own environment on
// every start. An update through this package reads the stored
// representation, replaces that one field, and writes the whole
// representation back, so every other field that Identity wrote survives.
package keycloakadmin

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
	"unicode/utf8"
)

// DefaultTimeout bounds one call to Keycloak. The reconcile has no other
// deadline of its own, and a Keycloak that hangs must not hold the worker.
const DefaultTimeout = 15 * time.Second

// maxAnswerSize bounds how much of an answer this package reads. A Keycloak
// that the operator does not run is on the other end, so a large or endless
// answer must not grow the operator until it is killed. A client
// representation is a few kilobytes, and the realm holds one.
const maxAnswerSize = 4 << 20

// adminRealm and adminClientID are where a Keycloak administrator signs in.
// Every Keycloak serves the admin-cli client of the master realm, and the
// Keycloak Operator writes its first administrator into that realm.
const (
	adminRealm    = "master"
	adminClientID = "admin-cli"
)

// Representation is one Keycloak client as the administration API returned
// it. It keeps every field the server holds, the ones this package does not
// read included, so that an update sends them all back unchanged.
type Representation map[string]any

// Client calls the administration API of one Keycloak realm as one
// administrator. It signs in on the first call and reuses the access token
// for the calls after it, so one Client belongs to one reconcile and is not
// safe for concurrent use.
type Client struct {
	baseURL  string
	realm    string
	username string
	password string
	http     *http.Client
	token    string
}

// New builds a client of the Keycloak at baseURL that administers realm as
// the given user. baseURL is the address that the operator reaches Keycloak
// at, the base path included, for example http://keycloak.camunda.svc:8080/auth.
// The user signs in through admin-cli of the master realm, so it is a
// Keycloak administrator and not a user of realm.
//
// Without an Option the client verifies an https Keycloak against the trust
// store of the operator image. Pass WithRootCAs for a Keycloak whose
// certificate comes from an authority that store does not carry.
func New(baseURL, realm, username, password string, opts ...Option) *Client {
	c := &Client{
		baseURL:  strings.TrimRight(baseURL, "/"),
		realm:    realm,
		username: username,
		password: password,
		http: &http.Client{
			Timeout: DefaultTimeout,
			// A Keycloak this operator does not run is on the other end. A
			// 307 or a 308 makes Go replay the request body at the new host,
			// and the body of the sign-in carries the administrator name and
			// password. No administration endpoint of Keycloak redirects, so
			// a redirect is refused rather than followed.
			CheckRedirect: func(*http.Request, []*http.Request) error {
				return http.ErrUseLastResponse
			},
		},
	}
	for _, opt := range opts {
		opt(c)
	}

	return c
}

// FindClient returns the client of the realm whose client id is clientID, or
// nil when the realm holds none. A realm that Management Identity has not
// bootstrapped yet holds none.
func (c *Client) FindClient(ctx context.Context, clientID string) (Representation, error) {
	endpoint := c.clientsURL() + "?clientId=" + url.QueryEscape(clientID)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("building the client lookup request: %w", err)
	}

	body, err := c.do(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("looking up the %q client of realm %q: %w", clientID, c.realm, err)
	}

	var found []Representation
	if err := json.Unmarshal(body, &found); err != nil {
		return nil, fmt.Errorf("reading the answer of the %q client lookup: %w", clientID, err)
	}
	if len(found) == 0 {
		return nil, nil
	}

	return found[0], nil
}

// UpdateClient writes rep back to the realm. rep carries the internal id that
// FindClient returned, so pass a representation that came from FindClient and
// not one built from nothing.
func (c *Client) UpdateClient(ctx context.Context, rep Representation) error {
	id := rep.ID()
	if id == "" {
		return errors.New("the client representation carries no id")
	}

	body, err := json.Marshal(rep)
	if err != nil {
		return fmt.Errorf("building the client update: %w", err)
	}

	req, err := http.NewRequestWithContext(
		ctx, http.MethodPut, c.clientsURL()+"/"+url.PathEscape(id), bytes.NewReader(body),
	)
	if err != nil {
		return fmt.Errorf("building the client update request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	if _, err := c.do(ctx, req); err != nil {
		return fmt.Errorf("updating client %q of realm %q: %w", id, c.realm, err)
	}

	return nil
}

// clientsURL is the clients collection of the administered realm.
func (c *Client) clientsURL() string {
	return c.baseURL + "/admin/realms/" + url.PathEscape(c.realm) + "/clients"
}

// do signs in if needed, sends req with the access token, and returns the
// body of a 2xx answer. Any other status becomes an error that carries the
// status and the start of the body.
//
// A refused token is exchanged once and the request is sent again, so a token
// that expired between two calls of the same Client does not surface as a
// failure of the call that met it.
func (c *Client) do(ctx context.Context, req *http.Request) ([]byte, error) {
	body, status, err := c.send(ctx, req)
	if err != nil {
		return nil, err
	}
	if status == http.StatusUnauthorized {
		c.token = ""
		body, status, err = c.send(ctx, req)
		if err != nil {
			return nil, err
		}
	}
	if status < 200 || status > 299 {
		return nil, statusError(status, body)
	}

	return body, nil
}

// send signs in if needed, sends req with the access token, and returns the
// body and the status of the answer. It rewinds the body of req, so a caller
// can send the same request twice.
func (c *Client) send(ctx context.Context, req *http.Request) ([]byte, int, error) {
	if c.token == "" {
		token, err := c.signIn(ctx)
		if err != nil {
			return nil, 0, err
		}
		c.token = token
	}
	req.Header.Set("Authorization", "Bearer "+c.token)

	if req.GetBody != nil {
		rewound, err := req.GetBody()
		if err != nil {
			return nil, 0, fmt.Errorf("rewinding the request body: %w", err)
		}
		req.Body = rewound
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, 0, err
	}
	defer func() { _ = resp.Body.Close() }()

	body, err := readAnswer(resp)
	if err != nil {
		return nil, 0, err
	}

	return body, resp.StatusCode, nil
}

// signIn exchanges the administrator credentials for an access token through
// the password grant of admin-cli.
func (c *Client) signIn(ctx context.Context) (string, error) {
	form := url.Values{
		"grant_type": {"password"},
		"client_id":  {adminClientID},
		"username":   {c.username},
		"password":   {c.password},
	}
	endpoint := c.baseURL + "/realms/" + adminRealm + "/protocol/openid-connect/token"

	req, err := http.NewRequestWithContext(
		ctx, http.MethodPost, endpoint, strings.NewReader(form.Encode()),
	)
	if err != nil {
		return "", fmt.Errorf("building the sign-in request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := c.http.Do(req)
	if err != nil {
		return "", fmt.Errorf("signing in at Keycloak: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	body, err := readAnswer(resp)
	if err != nil {
		return "", err
	}
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return "", fmt.Errorf("signing in at Keycloak: %w", statusError(resp.StatusCode, body))
	}

	var answer struct {
		AccessToken string `json:"access_token"`
	}
	if err := json.Unmarshal(body, &answer); err != nil {
		return "", fmt.Errorf("reading the sign-in answer: %w", err)
	}
	if answer.AccessToken == "" {
		return "", errors.New("no access token in the answer of Keycloak")
	}

	return answer.AccessToken, nil
}

// readAnswer reads at most maxAnswerSize bytes of the body of resp, and
// refuses an answer that reaches the bound rather than returning a body this
// package never expects to see.
func readAnswer(resp *http.Response) ([]byte, error) {
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxAnswerSize+1))
	if err != nil {
		return nil, fmt.Errorf("reading the answer of Keycloak: %w", err)
	}
	if len(body) > maxAnswerSize {
		return nil, fmt.Errorf("the answer of Keycloak is longer than %d bytes", maxAnswerSize)
	}

	return body, nil
}

// maxErrorBody is how much of a refused answer an error message carries. A
// Keycloak error page is longer than a condition message can hold.
const maxErrorBody = 512

// statusError turns a refused answer into an error that names the status and
// the start of the body.
func statusError(status int, body []byte) error {
	text := strings.TrimSpace(string(body))
	if len(text) > maxErrorBody {
		text = text[:maxErrorBody]
		for !utf8.ValidString(text) {
			text = text[:len(text)-1]
		}
	}
	if text == "" {
		return fmt.Errorf("status %d from Keycloak", status)
	}

	return fmt.Errorf("status %d from Keycloak: %s", status, text)
}
