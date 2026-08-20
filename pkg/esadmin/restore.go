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

package esadmin

import (
	"context"
	"encoding/json"
	"fmt"
	"maps"
	"net/http"
	"net/url"
	"slices"
	"strings"

	"github.com/konsole-is/camunda-operator/pkg/adminhttp"
)

// recoveryStageDone is the stage of a shard recovery that is over. Every
// other stage of the Elasticsearch vocabulary (INIT, INDEX, VERIFY_INDEX,
// TRANSLOG, FINALIZE) is a recovery that still runs.
const recoveryStageDone = "DONE"

// RestoreState is how far the restore of a set of indices has come.
type RestoreState string

// The restore states that RestoreProgress reports.
const (
	RestoreInProgress RestoreState = "IN_PROGRESS"
	RestoreDone       RestoreState = "DONE"
)

// ResolveIndices returns the concrete index names that patterns match,
// sorted, or nothing when they match no index. An empty pattern list is
// nothing too, and sends no request: an empty target names every index.
//
// The query expands its wildcards to open and closed indices. The get-index
// API expands to open ones by default, while the delete expands to open and
// closed ones, so the default would leave every closed index out of a set
// that the delete is meant to clear.
func (c *Client) ResolveIndices(ctx context.Context, patterns []string) ([]string, error) {
	if len(patterns) == 0 {
		return nil, nil
	}

	payload, _, err := c.api.Do(ctx, adminhttp.Request{
		Method: http.MethodGet,
		Path: "/" + indexTarget(patterns) +
			"?ignore_unavailable=true&allow_no_indices=true&expand_wildcards=open,closed",
	})
	if err != nil {
		return nil, err
	}

	// The answer is one entry per index, keyed by name. Only the names are
	// read, so the settings and the mappings of the entry stay raw.
	var response map[string]json.RawMessage
	if err := json.Unmarshal(payload, &response); err != nil {
		return nil, fmt.Errorf("decoding index list: %w", err)
	}

	return slices.Sorted(maps.Keys(response)), nil
}

// DeleteIndices deletes every index that patterns match. It resolves the
// patterns to their concrete names first, and then deletes all of them in one
// request. Elasticsearch cannot restore an index that already exists, so the
// whole restore set goes in one call: a per-index loop that fails halfway
// leaves indices behind that the restore then collides with.
//
// The resolution is what makes the call work on a cluster that runs the
// Elasticsearch default of action.destructive_requires_name, which is true
// since 8.0 and refuses a wildcard delete. The caller therefore keeps naming
// patterns, and the exact names never leave this client.
//
// Patterns that match no index send no delete at all, because an empty target
// names every index.
func (c *Client) DeleteIndices(ctx context.Context, patterns []string) error {
	names, err := c.ResolveIndices(ctx, patterns)
	if err != nil {
		return err
	}
	if len(names) == 0 {
		return nil
	}

	// The names are concrete and were there a moment ago, but an index can be
	// gone between the two calls. ignore_unavailable tolerates that, and
	// allow_no_indices tolerates a target that ends up matching nothing.
	_, _, err = c.api.Do(ctx, adminhttp.Request{
		Method: http.MethodDelete,
		Path:   "/" + indexTarget(names) + "?ignore_unavailable=true&allow_no_indices=true",
	})

	return err
}

// RestoreSnapshot starts the restore of indices from the snapshot name in
// repo. It returns when Elasticsearch accepted the restore, not when the
// restore is over: wait_for_completion stays false, because a call that
// waited would hold the reconcile worker for the whole restore.
// RestoreProgress follows it from there.
//
// indices must name at least one index. An empty list would send an empty
// pattern, which selects nothing rather than everything: the restore would
// succeed and bring back no data.
func (c *Client) RestoreSnapshot(ctx context.Context, repo, name string, indices []string) error {
	if len(indices) == 0 {
		return fmt.Errorf("restore of snapshot %q of repository %q names no index", name, repo)
	}

	body, err := json.Marshal(map[string]any{
		"indices":              strings.Join(indices, ","),
		"include_global_state": false,
	})
	if err != nil {
		return fmt.Errorf("encoding restore request: %w", err)
	}

	_, _, err = c.api.Do(ctx, adminhttp.Request{
		Method: http.MethodPost,
		Path:   snapshotPath(repo, name) + "/_restore?wait_for_completion=false",
		Body:   body,
	})

	return err
}

// RestoreProgress reports whether a shard of the indices that patterns match
// still recovers. It is RestoreInProgress while one does, and RestoreDone
// when none does.
//
// A shard counts as active when its recovery stage is not DONE. The query
// carries active_only, so Elasticsearch already drops the recoveries it
// finished, and the stage is read on top of that: a finished recovery stays
// in the cluster state, and it must never read as a restore that still runs.
// The stage is read for every shard of the target, whatever the recovery
// type. A restore writes entries of type SNAPSHOT, whose source names the
// repository and the snapshot, but an index of the restore set that recovers
// for another reason is not ready either, and waiting is the safe direction.
//
// An empty pattern list is RestoreDone and sends no request, because an empty
// target asks about every index in the cluster.
func (c *Client) RestoreProgress(ctx context.Context, patterns []string) (RestoreState, error) {
	if len(patterns) == 0 {
		return RestoreDone, nil
	}

	// An index of the restore set that does not exist yet has no recovery,
	// and a target that matches nothing must read as such rather than fail
	// the poll.
	payload, _, err := c.api.Do(ctx, adminhttp.Request{
		Method: http.MethodGet,
		Path: "/" + indexTarget(patterns) +
			"/_recovery?active_only=true&ignore_unavailable=true&allow_no_indices=true",
	})
	if err != nil {
		return "", err
	}

	var response map[string]struct {
		Shards []struct {
			Stage string `json:"stage"`
		} `json:"shards"`
	}
	if err := json.Unmarshal(payload, &response); err != nil {
		return "", fmt.Errorf("decoding recovery status: %w", err)
	}

	for _, index := range response {
		for _, shard := range index.Shards {
			if shard.Stage != recoveryStageDone {
				return RestoreInProgress, nil
			}
		}
	}

	return RestoreDone, nil
}

// indexTarget joins patterns into the multi-target path segment of an index
// API call.
func indexTarget(patterns []string) string {
	escaped := make([]string, 0, len(patterns))
	for _, pattern := range patterns {
		escaped = append(escaped, escapePattern(pattern))
	}

	return strings.Join(escaped, ",")
}

// escapePattern escapes one index pattern for a URL path segment and keeps
// its wildcards. url.PathEscape encodes an asterisk as %2A, which
// Elasticsearch matches as a literal character and not as a wildcard, so the
// pattern is escaped between its wildcards and joined again. A slash in a
// pattern is still escaped, so a pattern from a hand-written contract cannot
// retarget the request to another API path.
func escapePattern(pattern string) string {
	parts := strings.Split(pattern, "*")
	for i, part := range parts {
		parts[i] = url.PathEscape(part)
	}

	return strings.Join(parts, "*")
}
