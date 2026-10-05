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
	"cmp"
	"context"
	"encoding/json"
	"fmt"
	"maps"
	"net/http"
	"net/url"
	"slices"
	"strconv"
	"strings"

	"github.com/konsole-is/camunda-operator/pkg/adminhttp"
)

// recoveryStageDone is the stage of a shard recovery that is over. Every
// other stage of the Elasticsearch vocabulary (INIT, INDEX, VERIFY_INDEX,
// TRANSLOG, FINALIZE) is a recovery that still runs.
const recoveryStageDone = "DONE"

// The filter_path of each read keeps the fields that it uses. The unfiltered
// answer grows with the size of the cluster and can pass the 1 MiB that the
// client reads.
const (
	resolveFilter  = "indices.name,aliases.indices"
	recoveryFilter = "*.shards.stage"
	healthFilter   = "indices.*.shards.*.initializing_shards,indices.*.shards.*.unassigned_primary_shards"
	explainFilter  = "current_state,unassigned_info.reason,unassigned_info.last_allocation_status"
)

// The current states of a shard in the allocation explain API.
const (
	shardInitializing = "initializing"
	shardUnassigned   = "unassigned"
)

// noValidShardCopy is the last allocation status of a primary that has no
// valid copy on any node. Elasticsearch gives it only to a primary that
// recovers from its own store (PrimaryShardAllocator), never to one that a
// restore brings back from a snapshot.
const noValidShardCopy = "no_valid_shard_copy"

// waitingAllocations are the last allocation statuses of an unassigned primary
// that a node can still take, in the words of the allocation explain API.
var waitingAllocations = map[string]bool{
	// no is not here: Elasticsearch fails the restore of a primary for good
	// once the allocation deciders refuse it.
	"throttled":          true,
	"awaiting_info":      true,
	"allocation_delayed": true,
	"no_attempt":         true,
}

// allocationStatusNames maps a last allocation status of the allocation
// explain API back to the name that the shard's own allocation status has.
// Elasticsearch derives the first from the second in
// AllocationDecision.fromAllocationStatus.
var allocationStatusNames = map[string]string{
	"no":                 "deciders_no",
	"throttled":          "deciders_throttled",
	"awaiting_info":      "fetching_shard_data",
	"allocation_delayed": "delayed_allocation",
}

// RestoreState is how far the restore of a set of indices has come.
type RestoreState string

// The restore states that RestoreProgress reports.
const (
	RestoreInProgress RestoreState = "IN_PROGRESS"
	RestoreDone       RestoreState = "DONE"
	// RestoreStranded is a restore that nothing recovers any more, with a
	// restored primary that no node takes. The index of that primary stays
	// red until someone changes the cluster.
	RestoreStranded RestoreState = "STRANDED"
)

// Progress is the answer of RestoreProgress.
type Progress struct {
	// State is RestoreInProgress, RestoreStranded, or RestoreDone.
	State RestoreState
	// Stranded names each restored primary that no node takes, sorted by index
	// and shard. It is empty unless State is RestoreStranded.
	Stranded []StrandedShard
}

// StrandedShard is a restored primary that no node takes.
type StrandedShard struct {
	// Index is the name of the restored index that holds the primary.
	Index string
	// Shard is the shard number.
	Shard int
	// Reason is why the shard is unassigned, for example NEW_INDEX_RESTORED or
	// ALLOCATION_FAILED.
	Reason string
	// AllocationStatus is the last allocation status, for example deciders_no.
	AllocationStatus string
}

// ResolveIndices returns the concrete index names that patterns match,
// sorted, or nothing when they match no index. An alias that they match
// stands for the indices that it points to. An empty pattern list is nothing
// too, and sends no request: an empty target names every index.
//
// The query expands its wildcards to open and closed indices. The resolve
// API expands to open ones by default, while the delete expands to open and
// closed ones, so the default would leave every closed index out of a set
// that the delete is meant to clear.
func (c *Client) ResolveIndices(ctx context.Context, patterns []string) ([]string, error) {
	if len(patterns) == 0 {
		return nil, nil
	}

	payload, _, err := c.api.Do(ctx, adminhttp.Request{
		Method: http.MethodGet,
		Path: "/_resolve/index/" + indexTarget(patterns) +
			"?ignore_unavailable=true&allow_no_indices=true&expand_wildcards=open,closed&filter_path=" + resolveFilter,
	})
	if err != nil {
		return nil, err
	}

	var response struct {
		Indices []struct {
			Name string `json:"name"`
		} `json:"indices"`
		Aliases []struct {
			Indices []string `json:"indices"`
		} `json:"aliases"`
	}
	if err := json.Unmarshal(payload, &response); err != nil {
		return nil, fmt.Errorf("decoding index list: %w", err)
	}

	names := map[string]struct{}{}
	for _, index := range response.Indices {
		names[index.Name] = struct{}{}
	}
	for _, alias := range response.Aliases {
		for _, index := range alias.Indices {
			names[index] = struct{}{}
		}
	}

	return slices.Sorted(maps.Keys(names)), nil
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

// RestoreProgress reports how far the restore of the indices that patterns
// match has come. It is RestoreInProgress while a shard of them recovers or
// has yet to start, RestoreStranded when none does and a restored primary
// gets no node, and RestoreDone otherwise.
//
// A shard counts as in progress when it has a recovery that is not DONE,
// whatever the recovery type, or when it is INITIALIZING. An unassigned
// primary counts while a node can still take it, also while it waits for the
// retry of a failed recovery. A primary without a valid copy never counts,
// because it is not one that the restore brings back. An unassigned replica
// never counts: the restore does not wait for it, and it does not strand the
// restore.
//
// An empty pattern list is RestoreDone and sends no request, because an empty
// target asks about every index in the cluster.
func (c *Client) RestoreProgress(ctx context.Context, patterns []string) (Progress, error) {
	if len(patterns) == 0 {
		return Progress{State: RestoreDone}, nil
	}

	recovering, err := c.shardRecovering(ctx, patterns)
	if err != nil {
		return Progress{}, err
	}
	if recovering {
		return Progress{State: RestoreInProgress}, nil
	}

	// A shard of a restored index can be initializing before its recovery is
	// registered, and a primary can wait for allocation behind the
	// concurrent recoveries limit. Neither has a recovery entry yet.
	return c.shardProgress(ctx, patterns)
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
			"/_recovery?active_only=true&ignore_unavailable=true&allow_no_indices=true&filter_path=" + recoveryFilter,
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

// shardProgress reads the progress of the target from the shard health, and
// asks the allocation explain API about each unassigned primary.
func (c *Client) shardProgress(ctx context.Context, patterns []string) (Progress, error) {
	// With a target, the health API waits until every part of it names an
	// index, and answers 408 when its timeout ends. A timeout of 0s answers
	// at once, and the 408 still holds the health of the indices that exist.
	payload, _, err := c.api.Do(ctx, adminhttp.Request{
		Method: http.MethodGet,
		Path: "/_cluster/health/" + indexTarget(patterns) +
			"?level=shards&timeout=0s&filter_path=" + healthFilter,
		Accept: func(status int) bool {
			return status == http.StatusOK || status == http.StatusRequestTimeout
		},
	})
	if err != nil {
		return Progress{}, err
	}

	var response struct {
		Indices map[string]struct {
			Shards map[string]struct {
				InitializingShards      int `json:"initializing_shards"`
				UnassignedPrimaryShards int `json:"unassigned_primary_shards"`
			} `json:"shards"`
		} `json:"indices"`
	}
	if err := json.Unmarshal(payload, &response); err != nil {
		return Progress{}, fmt.Errorf("decoding shard health: %w", err)
	}

	var unassigned []StrandedShard
	for name, index := range response.Indices {
		for id, shard := range index.Shards {
			if shard.InitializingShards > 0 {
				return Progress{State: RestoreInProgress}, nil
			}
			if shard.UnassignedPrimaryShards == 0 {
				continue
			}
			number, err := strconv.Atoi(id)
			if err != nil {
				return Progress{}, fmt.Errorf("decoding shard health: shard number %q of index %s: %w", id, name, err)
			}
			unassigned = append(unassigned, StrandedShard{Index: name, Shard: number})
		}
	}
	slices.SortFunc(unassigned, func(a, b StrandedShard) int {
		return cmp.Or(strings.Compare(a.Index, b.Index), cmp.Compare(a.Shard, b.Shard))
	})

	var stranded []StrandedShard
	for _, primary := range unassigned {
		explanation, err := c.explainPrimary(ctx, primary.Index, primary.Shard)
		if err != nil {
			return Progress{}, err
		}

		status := explanation.UnassignedInfo.LastAllocationStatus
		switch {
		case explanation.CurrentState == shardInitializing || waitingAllocations[status]:
			return Progress{State: RestoreInProgress}, nil
		case explanation.CurrentState != shardUnassigned || status == noValidShardCopy:
			continue
		}

		primary.Reason = explanation.UnassignedInfo.Reason
		primary.AllocationStatus = cmp.Or(allocationStatusNames[status], status)
		stranded = append(stranded, primary)
	}
	if len(stranded) == 0 {
		return Progress{State: RestoreDone}, nil
	}

	return Progress{State: RestoreStranded, Stranded: stranded}, nil
}

// explanation is the part of an allocation explain answer that
// shardProgress reads.
type explanation struct {
	CurrentState   string `json:"current_state"`
	UnassignedInfo struct {
		Reason               string `json:"reason"`
		LastAllocationStatus string `json:"last_allocation_status"`
	} `json:"unassigned_info"`
}

// explainPrimary asks the allocation explain API about the primary of shard
// number of index.
func (c *Client) explainPrimary(ctx context.Context, index string, number int) (explanation, error) {
	body, err := json.Marshal(map[string]any{"index": index, "shard": number, "primary": true})
	if err != nil {
		return explanation{}, fmt.Errorf("encoding allocation explain request: %w", err)
	}

	payload, _, err := c.api.Do(ctx, adminhttp.Request{
		Method: http.MethodPost,
		Path:   "/_cluster/allocation/explain?filter_path=" + explainFilter,
		Body:   body,
	})
	if err != nil {
		return explanation{}, err
	}

	var answer explanation
	if err := json.Unmarshal(payload, &answer); err != nil {
		return explanation{}, fmt.Errorf(
			"decoding allocation explanation of shard %d of index %s: %w", number, index, err,
		)
	}

	return answer, nil
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
