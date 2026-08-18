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

// Package esadmin is the Elasticsearch administration client of the backup
// flows: snapshot repositories, snapshots, secure-settings reload, and node
// filesystem statistics. It is pure HTTP with no Kubernetes types; the
// caller resolves the endpoint, the CA, and the credentials from the
// SecondaryStorageConfig contract and passes them in.
//
// Deletion always names one exact snapshot. There is no delete-by-prefix on
// purpose: a backup finalizer must only ever remove its own artifacts.
package esadmin

import (
	"bytes"
	"context"
	"crypto/tls"
	"crypto/x509"
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

// ErrUnreachable and ErrRejected classify every failure of the client.
// ErrUnreachable means Elasticsearch did not answer; ErrRejected means it
// answered with an error, and the message carries the response body.
var (
	ErrUnreachable = errors.New("cannot reach Elasticsearch")
	ErrRejected    = errors.New("call rejected by Elasticsearch")
)

// DefaultS3Client is the name of the S3 client configuration that the
// repositories of this operator use. The node keystore entries
// (s3.client.<name>.access_key, s3.client.<name>.secret_key) and the
// repository settings must name the same client, so both sides build on this
// constant.
const DefaultS3Client = "default"

// SnapshotState is the state of one snapshot.
type SnapshotState string

// The snapshot states of Elasticsearch, plus SnapshotMissing for a snapshot
// that does not exist (a 404 is not an error: the state machine queries
// before it acts).
const (
	SnapshotInProgress SnapshotState = "IN_PROGRESS"
	SnapshotSuccess    SnapshotState = "SUCCESS"
	SnapshotFailed     SnapshotState = "FAILED"
	SnapshotPartial    SnapshotState = "PARTIAL"
	SnapshotMissing    SnapshotState = "MISSING"
)

// S3RepositoryConfig is the settings block of an s3 snapshot repository. The
// bucket credentials are not here: Elasticsearch reads them from the node
// keystore only.
type S3RepositoryConfig struct {
	// Bucket is the bucket name.
	Bucket string
	// BasePath is the key prefix of every snapshot in the repository.
	BasePath string
	// Endpoint is the URL of an S3-compatible store. Empty means AWS S3.
	Endpoint string
	// PathStyleAccess forces path-style bucket addressing.
	PathStyleAccess bool
}

// Client administers one Elasticsearch cluster.
type Client struct {
	base string
	user string
	pass string
	http *http.Client
}

// New builds a client for the cluster at endpoint, authenticated with basic
// auth. ca verifies the TLS certificate of the endpoint; nil means the system
// pool.
//
// A ca that is present but holds no certificate is an error, not a fallback:
// an empty or unparseable bundle means the caller read the wrong Secret key,
// and the system pool would hide that as an unexplained TLS failure on every
// call. Only a caller that deliberately passes nil gets the system pool.
func New(endpoint, user, pass string, ca []byte) (*Client, error) {
	transport := http.DefaultTransport.(*http.Transport).Clone()
	if ca != nil {
		if len(ca) == 0 {
			return nil, errors.New("the CA bundle is empty")
		}
		pool := x509.NewCertPool()
		if !pool.AppendCertsFromPEM(ca) {
			return nil, errors.New("the CA bundle holds no PEM certificate")
		}
		transport.TLSClientConfig = &tls.Config{RootCAs: pool, MinVersion: tls.VersionTLS12}
	}

	return &Client{
		base: strings.TrimRight(endpoint, "/"),
		user: user,
		pass: pass,
		http: &http.Client{Timeout: 30 * time.Second, Transport: transport},
	}, nil
}

// EnsureSnapshotRepository registers the s3 repository name with cfg,
// converging with an idempotent PUT: registering an already registered
// repository updates it in place.
func (c *Client) EnsureSnapshotRepository(ctx context.Context, name string, cfg S3RepositoryConfig) error {
	settings := map[string]any{
		"bucket": cfg.Bucket,
		"client": DefaultS3Client,
	}
	if cfg.BasePath != "" {
		settings["base_path"] = cfg.BasePath
	}
	if cfg.Endpoint != "" {
		settings["endpoint"] = cfg.Endpoint
	}
	if cfg.PathStyleAccess {
		settings["path_style_access"] = true
	}

	body, err := json.Marshal(map[string]any{"type": "s3", "settings": settings})
	if err != nil {
		return fmt.Errorf("encoding repository settings: %w", err)
	}

	_, _, err = c.do(ctx, http.MethodPut, "/_snapshot/"+url.PathEscape(name), body)
	return err
}

// CreateSnapshot starts the snapshot name of indices in repo. A snapshot
// that already exists under the same name is success, so a re-entrant caller
// never double-starts.
//
// indices must name at least one index. An empty list would send an empty
// pattern, which selects nothing rather than everything: the snapshot would
// succeed and hold no data.
func (c *Client) CreateSnapshot(ctx context.Context, repo, name string, indices []string) error {
	if len(indices) == 0 {
		return fmt.Errorf("snapshot %q of repository %q names no index", name, repo)
	}

	body, err := json.Marshal(map[string]any{
		"indices":              strings.Join(indices, ","),
		"include_global_state": false,
	})
	if err != nil {
		return fmt.Errorf("encoding snapshot request: %w", err)
	}

	_, status, err := c.do(ctx, http.MethodPut, snapshotPath(repo, name), body)
	if err != nil {
		// Elasticsearch rejects a duplicate name with
		// invalid_snapshot_name_exception or snapshot_name_already_in_use.
		// Re-entry is not a failure: when the snapshot exists, the start
		// already happened.
		if status == http.StatusBadRequest || status == http.StatusConflict {
			if state, statusErr := c.SnapshotStatus(ctx, repo, name); statusErr == nil && state != SnapshotMissing {
				return nil
			}
		}
		return err
	}

	return nil
}

// SnapshotStatus reports the state of the snapshot name in repo. A snapshot
// that does not exist is SnapshotMissing, not an error.
func (c *Client) SnapshotStatus(ctx context.Context, repo, name string) (SnapshotState, error) {
	payload, status, err := c.do(ctx, http.MethodGet, snapshotPath(repo, name), nil)
	if status == http.StatusNotFound {
		// A 404 is how Elasticsearch reports an absent snapshot, but also an
		// absent repository. Only the first is a state; a dropped repository
		// must never read as "the snapshot is gone".
		if errorType(payload) == "snapshot_missing_exception" {
			return SnapshotMissing, nil
		}
		return "", err
	}
	if err != nil {
		return "", err
	}

	var response struct {
		Snapshots []struct {
			State string `json:"state"`
		} `json:"snapshots"`
	}
	if err := json.Unmarshal(payload, &response); err != nil {
		return "", fmt.Errorf("decoding snapshot status: %w", err)
	}
	if len(response.Snapshots) == 0 {
		return SnapshotMissing, nil
	}

	return SnapshotState(response.Snapshots[0].State), nil
}

// DeleteSnapshot deletes the snapshot name from repo. A snapshot that does
// not exist is success, so a re-entrant finalizer can call it again. A
// repository that does not exist is an error: the artifacts of the backup may
// still sit in the bucket, and a finalizer that read that as success would
// release without cleaning up.
func (c *Client) DeleteSnapshot(ctx context.Context, repo, name string) error {
	payload, status, err := c.do(ctx, http.MethodDelete, snapshotPath(repo, name), nil)
	if status == http.StatusNotFound && errorType(payload) == "snapshot_missing_exception" {
		return nil
	}

	return err
}

// snapshotPath returns the URL path of one snapshot, with each name escaped:
// the repository name of a hand-written contract is user input, and a slash
// in it must not retarget the request.
func snapshotPath(repo, name string) string {
	return "/_snapshot/" + url.PathEscape(repo) + "/" + url.PathEscape(name)
}

// errorType extracts error.type from an Elasticsearch error body, or empty
// when the body has none.
func errorType(payload []byte) string {
	var body struct {
		Error struct {
			Type string `json:"type"`
		} `json:"error"`
	}
	if err := json.Unmarshal(payload, &body); err != nil {
		return ""
	}

	return body.Error.Type
}

// ReloadSecureSettings reloads the reloadable secure settings (S3
// credentials among them) on every node, so a rotated keystore value is
// picked up without a restart.
func (c *Client) ReloadSecureSettings(ctx context.Context) error {
	_, _, err := c.do(ctx, http.MethodPost, "/_nodes/reload_secure_settings", nil)
	return err
}

// MaxNodeFSTotalAndUsedBytes reports the largest node filesystem total and
// the largest node used bytes across every node that reports filesystem
// statistics. The two are the inputs of the effective restore size, and they
// can come from different nodes: each is the worst case of its own kind.
func (c *Client) MaxNodeFSTotalAndUsedBytes(ctx context.Context) (total, used int64, err error) {
	payload, _, err := c.do(ctx, http.MethodGet, "/_nodes/stats/fs", nil)
	if err != nil {
		return 0, 0, err
	}

	var response struct {
		Nodes map[string]struct {
			FS struct {
				Total struct {
					TotalInBytes     int64 `json:"total_in_bytes"`
					AvailableInBytes int64 `json:"available_in_bytes"`
				} `json:"total"`
			} `json:"fs"`
		} `json:"nodes"`
	}
	if err := json.Unmarshal(payload, &response); err != nil {
		return 0, 0, fmt.Errorf("decoding node fs stats: %w", err)
	}

	for _, node := range response.Nodes {
		nodeTotal := node.FS.Total.TotalInBytes
		nodeUsed := nodeTotal - node.FS.Total.AvailableInBytes
		total = max(total, nodeTotal)
		used = max(used, nodeUsed)
	}

	return total, used, nil
}

// do sends one request and returns the body and status code. A transport
// error wraps ErrUnreachable. A non-2xx status wraps ErrRejected with the
// response body in the message; the caller reads the returned status for the
// branches that are not failures.
func (c *Client) do(ctx context.Context, method, path string, body []byte) ([]byte, int, error) {
	var reader io.Reader
	if body != nil {
		reader = bytes.NewReader(body)
	}

	req, err := http.NewRequestWithContext(ctx, method, c.base+path, reader)
	if err != nil {
		return nil, 0, fmt.Errorf("building request: %w", err)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if c.user != "" || c.pass != "" {
		req.SetBasicAuth(c.user, c.pass)
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, 0, fmt.Errorf("%w: %s %s: %w", ErrUnreachable, method, path, err)
	}
	defer func() { _ = resp.Body.Close() }()

	payload, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return nil, resp.StatusCode, fmt.Errorf(
			"%w: reading response of %s %s: %w", ErrUnreachable, method, path, err,
		)
	}

	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return payload, resp.StatusCode, fmt.Errorf(
			"%w: %s %s returned %d: %s",
			ErrRejected, method, path, resp.StatusCode, errorBody(payload),
		)
	}

	return payload, resp.StatusCode, nil
}

// errorBodyLimit bounds how much of a rejected response body an error message
// carries. The full body is still read and returned to the caller. Only the
// message is bounded, so an error can land in a condition or an event without
// exceeding their limits.
const errorBodyLimit = 1 << 10

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
