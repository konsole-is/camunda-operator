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
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/konsole-is/camunda-operator/pkg/adminhttp"
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
	api *adminhttp.Client
}

// New builds a client for the cluster at endpoint, authenticated with basic
// auth. ca verifies the TLS certificate of the endpoint; nil means the system
// pool.
//
// A ca that is present but holds no certificate is an error, not a fallback:
// the system pool would hide a wrongly read Secret key as an unexplained TLS
// failure on every call. Only a caller that deliberately passes nil gets the
// system pool.
func New(endpoint, user, pass string, ca []byte) (*Client, error) {
	api, err := adminhttp.New(adminhttp.Config{
		Endpoint:    endpoint,
		Auth:        adminhttp.BasicAuth{Username: user, Password: pass},
		CABundle:    ca,
		Unreachable: ErrUnreachable,
		Rejected:    ErrRejected,
	})
	if err != nil {
		return nil, err
	}

	return &Client{api: api}, nil
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

	_, _, err = c.api.Do(ctx, adminhttp.Request{
		Method: http.MethodPut,
		Path:   "/_snapshot/" + url.PathEscape(name),
		Body:   body,
	})
	return err
}

// Snapshot is what SnapshotStatus reports about one snapshot: its state and
// the user metadata that its creation carried. The metadata is how a caller
// tells its own snapshot from one that another actor created under the same
// name.
type Snapshot struct {
	// State of the snapshot. SnapshotMissing means that no snapshot with
	// the name exists; every other field is empty then.
	State SnapshotState
	// Metadata is the user metadata of the snapshot, as the creation
	// request carried it. Elasticsearch stores any JSON value verbatim and
	// returns it with the snapshot info, so the values are decoded as
	// encoding/json decodes into any: a string, a float64, a bool, nil, a
	// []any, or a map[string]any. A snapshot that this client created
	// carries strings only; one that another actor created can carry
	// anything. MetadataString reads one entry as a string.
	Metadata map[string]any
}

// MetadataString returns the value under key in the metadata of the
// snapshot when it is a string. It reports false for an absent key and for a
// value of any other JSON type.
func (s Snapshot) MetadataString(key string) (string, bool) {
	value, ok := s.Metadata[key].(string)
	return value, ok
}

// sameMetadata reports whether existing holds exactly the entries of want:
// the same keys, each with the string value that want carries. Any entry
// of another JSON type is a difference.
func sameMetadata(existing map[string]any, want map[string]string) bool {
	if len(existing) != len(want) {
		return false
	}
	for key, value := range want {
		if got, ok := existing[key].(string); !ok || got != value {
			return false
		}
	}

	return true
}

// CreateSnapshot starts the snapshot name of indices in repo, carrying
// metadata as the user metadata of the snapshot. A snapshot that already
// exists under the same name AND the same metadata is success, so a
// re-entrant caller never double-starts. An existing snapshot with other
// metadata is another actor's, and the duplicate rejection comes back as an
// error. The metadata is the ownership evidence of that rule. A caller that
// carries none gets every duplicate rejection back as an error, because a
// metadata-free snapshot under the name can be anyone's.
//
// indices must name at least one index. An empty list would send an empty
// pattern, which selects nothing rather than everything: the snapshot would
// succeed and hold no data.
func (c *Client) CreateSnapshot(
	ctx context.Context,
	repo, name string,
	indices []string,
	metadata map[string]string,
) error {
	if len(indices) == 0 {
		return fmt.Errorf("snapshot %q of repository %q names no index", name, repo)
	}

	request := map[string]any{
		"indices":              strings.Join(indices, ","),
		"include_global_state": false,
	}
	if len(metadata) > 0 {
		request["metadata"] = metadata
	}
	body, err := json.Marshal(request)
	if err != nil {
		return fmt.Errorf("encoding snapshot request: %w", err)
	}

	_, status, err := c.api.Do(ctx, adminhttp.Request{
		Method: http.MethodPut,
		Path:   snapshotPath(repo, name),
		Body:   body,
	})
	if err != nil {
		// Elasticsearch rejects a duplicate name with
		// invalid_snapshot_name_exception or snapshot_name_already_in_use.
		// Re-entry is not a failure, but only the caller's own earlier
		// snapshot counts: the metadata must match, and there must be
		// metadata to match. Without it, sameMetadata(nil, nil) adopted any
		// metadata-free snapshot under the name.
		if (status == http.StatusBadRequest || status == http.StatusConflict) && len(metadata) > 0 {
			if existing, statusErr := c.SnapshotStatus(ctx, repo, name); statusErr == nil &&
				existing.State != SnapshotMissing &&
				sameMetadata(existing.Metadata, metadata) {
				return nil
			}
		}
		return err
	}

	return nil
}

// SnapshotStatus reports the snapshot name in repo: its state and its user
// metadata. A snapshot that does not exist is SnapshotMissing, not an
// error.
func (c *Client) SnapshotStatus(ctx context.Context, repo, name string) (Snapshot, error) {
	payload, status, err := c.api.Do(ctx, adminhttp.Request{Method: http.MethodGet, Path: snapshotPath(repo, name)})
	if status == http.StatusNotFound {
		// A 404 is how Elasticsearch reports an absent snapshot, but also an
		// absent repository. Only the first is a state; a dropped repository
		// must never read as "the snapshot is gone".
		if errorType(payload) == "snapshot_missing_exception" {
			return Snapshot{State: SnapshotMissing}, nil
		}
		return Snapshot{}, err
	}
	if err != nil {
		return Snapshot{}, err
	}

	var response struct {
		Snapshots []struct {
			State    string         `json:"state"`
			Metadata map[string]any `json:"metadata"`
		} `json:"snapshots"`
	}
	if err := json.Unmarshal(payload, &response); err != nil {
		return Snapshot{}, fmt.Errorf("decoding snapshot status: %w", err)
	}
	if len(response.Snapshots) == 0 {
		return Snapshot{State: SnapshotMissing}, nil
	}

	return Snapshot{
		State:    SnapshotState(response.Snapshots[0].State),
		Metadata: response.Snapshots[0].Metadata,
	}, nil
}

// DeleteSnapshot deletes the snapshot name from repo. A snapshot that does
// not exist is success, so a re-entrant finalizer can call it again. A
// repository that does not exist is an error: the artifacts of the backup may
// still sit in the bucket, and a finalizer that read that as success would
// release without cleaning up.
func (c *Client) DeleteSnapshot(ctx context.Context, repo, name string) error {
	payload, status, err := c.api.Do(ctx, adminhttp.Request{Method: http.MethodDelete, Path: snapshotPath(repo, name)})
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
	_, _, err := c.api.Do(ctx, adminhttp.Request{Method: http.MethodPost, Path: "/_nodes/reload_secure_settings"})
	return err
}

// MaxNodeFSTotalAndUsedBytes reports the largest node filesystem total and
// the largest node used bytes across every node that reports filesystem
// statistics. The two are the inputs of the effective restore size, and they
// can come from different nodes: each is the worst case of its own kind.
func (c *Client) MaxNodeFSTotalAndUsedBytes(ctx context.Context) (total, used int64, err error) {
	payload, _, err := c.api.Do(ctx, adminhttp.Request{Method: http.MethodGet, Path: "/_nodes/stats/fs"})
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
