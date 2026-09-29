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

// The shard states of the routing table that RestoreProgress reads.
const (
	shardInitializing = "INITIALIZING"
	shardUnassigned   = "UNASSIGNED"
)

var restoredReasons = map[string]bool{
	"NEW_INDEX_RESTORED":      true,
	"EXISTING_INDEX_RESTORED": true,
}

// waitingAllocations are the allocation statuses of an unassigned shard that a
// node can still take.
var waitingAllocations = map[string]bool{
	"deciders_throttled":  true,
	"fetching_shard_data": true,
	"delayed_allocation":  true,
	"no_attempt":          true,
}

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

// MaxDeletePathBytes bounds the path of one delete request. Elasticsearch
// reads a request line of http.max_initial_line_length at most, which
// defaults to 4kb and answers a longer one with a 400. A cluster with real
// history holds one zeebe-record index per day, and every name of the set
// goes into the path, so a set of a few months passes that bound. The budget
// stays under the default, because the line also carries the method, the
// query, and the protocol.
const MaxDeletePathBytes = 3 << 10

// DeleteIndices deletes every index that patterns match. It resolves the
// patterns to their concrete names first, and deletes the names in as few
// requests as the path budget allows: one when they fit, and several when the
// set is large. See MaxDeletePathBytes.
//
// A batch that fails leaves the indices of the later batches in place, and
// the caller re-enters and deletes them: the restore records what it asked
// Elasticsearch for only after every restore request went out, so a delete
// that stopped halfway is repeated whole before any snapshot is restored.
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
	for _, batch := range deleteBatches(names) {
		// The names are concrete and were there a moment ago, but an index can
		// be gone between the two calls. ignore_unavailable tolerates that,
		// and allow_no_indices tolerates a target that ends up matching
		// nothing.
		_, _, err = c.api.Do(ctx, adminhttp.Request{
			Method: http.MethodDelete,
			Path:   "/" + batch + "?ignore_unavailable=true&allow_no_indices=true",
		})
		if err != nil {
			return err
		}
	}

	return nil
}

// deleteBatches groups names into the path segments of as few delete requests
// as MaxDeletePathBytes allows. A single name that passes the budget on its
// own still gets its own request: Elasticsearch answers it, or it does not,
// and splitting a name is not an option.
func deleteBatches(names []string) []string {
	batches := make([]string, 0, 1)

	var batch strings.Builder
	for _, name := range names {
		escaped := escapePattern(name)
		// The separating comma joins this name to the ones before it.
		if batch.Len() > 0 && batch.Len()+1+len(escaped) > MaxDeletePathBytes {
			batches = append(batches, batch.String())
			batch.Reset()
		}
		if batch.Len() > 0 {
			batch.WriteByte(',')
		}
		batch.WriteString(escaped)
	}
	if batch.Len() > 0 {
		batches = append(batches, batch.String())
	}

	return batches
}

// SnapshotRepositoryExists reports whether the repository name is registered.
// A repository that does not exist is false, not an error: a caller asks
// before it decides to register one.
//
// The answer is what keeps a restore from overwriting a registration that it
// does not own. An operator can register a repository by hand on an
// Elasticsearch that this operator does not manage, and a blind PUT would
// point it at another prefix of another bucket.
func (c *Client) SnapshotRepositoryExists(ctx context.Context, name string) (bool, error) {
	payload, status, err := c.api.Do(ctx, adminhttp.Request{
		Method: http.MethodGet,
		Path:   "/_snapshot/" + url.PathEscape(name),
	})
	if status == http.StatusNotFound && errorType(payload) == "repository_missing_exception" {
		return false, nil
	}
	if err != nil {
		return false, err
	}

	return true, nil
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
// still recovers or has yet to start. It is RestoreInProgress while one does,
// and RestoreDone when none does.
//
// A shard counts when it has a recovery that is not DONE, whatever the
// recovery type, or when it is INITIALIZING. A primary counts while it is
// UNASSIGNED and waits for the allocation of its restore. An unassigned
// replica does not count, and neither does a primary that Elasticsearch
// failed to allocate or that no node will take: no recovery comes for them
// without a change to the cluster.
//
// An empty pattern list is RestoreDone and sends no request, because an empty
// target asks about every index in the cluster.
func (c *Client) RestoreProgress(ctx context.Context, patterns []string) (RestoreState, error) {
	if len(patterns) == 0 {
		return RestoreDone, nil
	}

	recovering, err := c.shardRecovering(ctx, patterns)
	if err != nil {
		return "", err
	}
	if recovering {
		return RestoreInProgress, nil
	}

	// A shard of a restored index can be initializing before its recovery is
	// registered, and a primary can wait for allocation behind the
	// concurrent recoveries limit. Neither has a recovery entry yet.
	pending, err := c.shardPending(ctx, patterns)
	if err != nil {
		return "", err
	}
	if pending {
		return RestoreInProgress, nil
	}

	return RestoreDone, nil
}

// shardRecovering reports whether a shard of the target has a recovery that
// is not DONE. The query carries active_only, but a finished recovery can
// stay in the cluster state, so the stage is read too.
func (c *Client) shardRecovering(ctx context.Context, patterns []string) (bool, error) {
	// An index of the restore set that does not exist yet has no recovery,
	// and a target that matches nothing must read as such rather than fail
	// the poll.
	payload, _, err := c.api.Do(ctx, adminhttp.Request{
		Method: http.MethodGet,
		Path: "/" + indexTarget(patterns) +
			"/_recovery?active_only=true&ignore_unavailable=true&allow_no_indices=true",
	})
	if err != nil {
		return false, err
	}

	var response map[string]struct {
		Shards []struct {
			Stage string `json:"stage"`
		} `json:"shards"`
	}
	if err := json.Unmarshal(payload, &response); err != nil {
		return false, fmt.Errorf("decoding recovery status: %w", err)
	}

	for _, index := range response {
		for _, shard := range index.Shards {
			if shard.Stage != recoveryStageDone {
				return true, nil
			}
		}
	}

	return false, nil
}

// shardPending reports whether the routing table holds a shard of the target
// that is INITIALIZING, or a primary that is UNASSIGNED since its restore and
// that a node can still take.
func (c *Client) shardPending(ctx context.Context, patterns []string) (bool, error) {
	payload, _, err := c.api.Do(ctx, adminhttp.Request{
		Method: http.MethodGet,
		Path: "/_cluster/state/routing_table/" + indexTarget(patterns) +
			"?ignore_unavailable=true&allow_no_indices=true",
	})
	if err != nil {
		return false, err
	}

	var response struct {
		RoutingTable struct {
			Indices map[string]struct {
				Shards map[string][]struct {
					State          string `json:"state"`
					Primary        bool   `json:"primary"`
					UnassignedInfo struct {
						Reason           string `json:"reason"`
						AllocationStatus string `json:"allocation_status"`
					} `json:"unassigned_info"`
				} `json:"shards"`
			} `json:"indices"`
		} `json:"routing_table"`
	}
	if err := json.Unmarshal(payload, &response); err != nil {
		return false, fmt.Errorf("decoding routing table: %w", err)
	}

	for _, index := range response.RoutingTable.Indices {
		for _, copies := range index.Shards {
			for _, shard := range copies {
				if shard.State == shardInitializing {
					return true, nil
				}
				if shard.Primary && shard.State == shardUnassigned &&
					restoredReasons[shard.UnassignedInfo.Reason] &&
					waitingAllocations[shard.UnassignedInfo.AllocationStatus] {
					return true, nil
				}
			}
		}
	}

	return false, nil
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
