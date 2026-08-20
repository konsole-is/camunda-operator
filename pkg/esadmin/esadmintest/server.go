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

// Package esadmintest fakes the Elasticsearch administration APIs that
// pkg/esadmin calls: snapshot repositories, snapshots, snapshot restores,
// index resolution and deletion, index recovery, secure-settings reload, and
// node filesystem statistics.
package esadmintest

import (
	"encoding/json"
	"net/http"
	"net/url"
	"path"
	"slices"
	"strconv"
	"strings"

	"github.com/konsole-is/camunda-operator/pkg/adminhttp/adminhttptest"
)

// resolveWildcards is the expand_wildcards that a resolution must carry. The
// get-index API expands to open indices alone by default, while the delete
// expands to open and closed ones, so a resolution that takes the default
// leaves every closed index behind for the restore to collide with.
const resolveWildcards = "open,closed"

// The path segments that name an API of the fake.
const (
	snapshotPath = "_snapshot"
	restorePath  = "_restore"
	recoveryPath = "_recovery"
)

// The recovery stages that the fake reports. DONE is a recovery that is over;
// INDEX is one that still copies files.
const (
	recoveryStageDone   = "DONE"
	recoveryStageActive = "INDEX"
)

// Repository is the fake's record of one snapshot repository.
type Repository struct {
	// Type of the repository: s3, gcs, or azure.
	Type string
	// Settings as registered.
	Settings map[string]any
}

// Snapshot is the fake's record of one snapshot.
type Snapshot struct {
	// Repo that holds the snapshot.
	Repo string
	// Name of the snapshot.
	Name string
	// State in the Elasticsearch vocabulary, for example IN_PROGRESS.
	State string
	// Indices requested for the snapshot.
	Indices string
	// Metadata is the user metadata that the creation carried, decoded as
	// encoding/json decodes into any. A snapshot seeded with
	// SetSnapshotState has none, like one that another actor created;
	// SetSnapshotMetadata gives it any, strings or not.
	Metadata map[string]any
}

// RestoreRequest is the fake's record of one snapshot restore.
type RestoreRequest struct {
	// Repo that holds the snapshot.
	Repo string
	// Name of the snapshot.
	Name string
	// Indices requested for the restore, as the request body named them.
	Indices []string
	// WaitForCompletion is the wait_for_completion of the request. A restore
	// that the operator starts must never wait, because the call would hold
	// the reconcile worker until the restore is over.
	WaitForCompletion bool
}

// Server fakes the Elasticsearch admin surface. Every exported method is safe
// for concurrent use, and so is the handler: adminhttptest.Fake serves one
// request at a time under the same lock that every accessor here takes, so
// the handler reads and writes the state below directly and must never take
// that lock again.
//
// The operations that the inherited FailNext and DropNext name are
// "repository", "snapshotCreate", "snapshotStatus", "snapshotDelete",
// "repositoryGet", "snapshotRestore", "indexResolve", "indexDelete",
// "recovery", "reload", and
// "stats". A failing operation answers 500; a dropped one closes the
// connection.
//
// SetIndices seeds the indices that the fake holds. A resolution and a delete
// both match their target against that set with the multi-target syntax of
// Elasticsearch, and a delete removes what it matched.
//
// The fake pins the shape of both index requests: it accepts one only when it
// tolerates a target that matches nothing, and it accepts a resolution only
// when the request expands its wildcards to open and closed indices.
type Server struct {
	adminhttptest.Fake

	repos     map[string]*Repository
	snapshots map[string]*Snapshot

	repositoryPuts map[string]int

	snapshotCreates map[string]int
	reloadCalls     int
	statsCalls      int

	restores []RestoreRequest

	// indices is the set of indices that the fake holds.
	indices map[string]struct{}

	deletedIndices   []string
	indexDeletePaths []string
	indexDeleteCalls int

	// recoveryActive drives the stage that _recovery reports for every
	// queried index.
	recoveryActive bool

	// nodeFS drives _nodes/stats/fs, keyed by node name.
	nodeFS map[string]nodeFS
}

// nodeFS is the filesystem report of one node.
type nodeFS struct {
	total int64
	used  int64
}

// New starts the fake over plain HTTP. Close it with Close.
func New() *Server {
	s := newServer()
	s.Start(s.handle)

	return s
}

// NewTLS starts the fake over HTTPS with a self-signed certificate, the way
// ECK serves Elasticsearch. CertificatePEM returns the bundle that verifies
// it, so a test exercises the same CA path as production. Close it with
// Close.
func NewTLS() *Server {
	s := newServer()
	s.StartTLS(s.handle)

	return s
}

func newServer() *Server {
	return &Server{
		repos:           map[string]*Repository{},
		repositoryPuts:  map[string]int{},
		snapshots:       map[string]*Snapshot{},
		snapshotCreates: map[string]int{},
		indices:         map[string]struct{}{},
		nodeFS:          map[string]nodeFS{"node-0": {total: 100 << 30, used: 10 << 30}},
	}
}

// Repository returns the registered repository name, or nil.
func (s *Server) Repository(name string) *Repository {
	s.Lock()
	defer s.Unlock()
	if r, ok := s.repos[name]; ok {
		copied := *r
		return &copied
	}
	return nil
}

// RepositoryPuts reports how often the repository name was registered. A
// converging controller registers on every reconcile, so the count grows with
// the reconciles while Repository stays the same: that is what idempotence
// looks like from the fake.
func (s *Server) RepositoryPuts(name string) int {
	s.Lock()
	defer s.Unlock()
	return s.repositoryPuts[name]
}

// SetSnapshotState sets the state of the snapshot repo/name, creating it
// when absent. An existing snapshot keeps its metadata: the knob drives the
// state of a snapshot, whoever created it.
func (s *Server) SetSnapshotState(repo, name, state string) {
	s.Lock()
	defer s.Unlock()
	if snapshot, ok := s.snapshots[repo+"/"+name]; ok {
		snapshot.State = state
		return
	}
	s.snapshots[repo+"/"+name] = &Snapshot{Repo: repo, Name: name, State: state}
}

// SetSnapshotMetadata sets the user metadata of the snapshot repo/name,
// creating it in state SUCCESS when absent. The values can be any JSON
// value, so a test can seed a snapshot that another actor created with
// metadata this operator never writes: numbers, lists, or objects.
func (s *Server) SetSnapshotMetadata(repo, name string, metadata map[string]any) {
	s.Lock()
	defer s.Unlock()
	snapshot, ok := s.snapshots[repo+"/"+name]
	if !ok {
		snapshot = &Snapshot{Repo: repo, Name: name, State: "SUCCESS"}
		s.snapshots[repo+"/"+name] = snapshot
	}
	snapshot.Metadata = metadata
}

// SnapshotExists reports whether the snapshot repo/name exists.
func (s *Server) SnapshotExists(repo, name string) bool {
	s.Lock()
	defer s.Unlock()
	_, ok := s.snapshots[repo+"/"+name]
	return ok
}

// SnapshotCreates reports how often a create of repo/name was accepted.
func (s *Server) SnapshotCreates(repo, name string) int {
	s.Lock()
	defer s.Unlock()
	return s.snapshotCreates[repo+"/"+name]
}

// RestoreRequests returns the restores that the fake accepted, in the order
// they arrived.
func (s *Server) RestoreRequests() []RestoreRequest {
	s.Lock()
	defer s.Unlock()
	copied := slices.Clone(s.restores)
	for i := range copied {
		copied[i].Indices = slices.Clone(copied[i].Indices)
	}

	return copied
}

// SetIndices replaces the indices that the fake holds. A resolution reports
// the ones its target matches, and a delete removes them.
func (s *Server) SetIndices(names ...string) {
	s.Lock()
	defer s.Unlock()
	s.indices = make(map[string]struct{}, len(names))
	for _, name := range names {
		s.indices[name] = struct{}{}
	}
}

// Indices returns the indices that the fake holds, sorted. A test reads it
// after a delete to see what survived.
func (s *Server) Indices() []string {
	s.Lock()
	defer s.Unlock()
	return sortedKeys(s.indices)
}

// DeletedIndices returns every index name that a delete named, in the order
// the calls arrived. One call that names three indices adds three entries, so
// a caller that wants the number of calls reads IndexDeleteCalls.
func (s *Server) DeletedIndices() []string {
	s.Lock()
	defer s.Unlock()
	return slices.Clone(s.deletedIndices)
}

// IndexDeletePaths returns the escaped path of every delete request that the
// fake accepted, in the order they arrived. A test reads it to bound the
// request line that a client builds: Elasticsearch refuses one that passes
// http.max_initial_line_length.
func (s *Server) IndexDeletePaths() []string {
	s.Lock()
	defer s.Unlock()
	return slices.Clone(s.indexDeletePaths)
}

// IndexDeleteCalls reports how many delete requests the fake accepted. The
// whole restore set belongs in one request, so a test that resolves three
// indices and reads three calls has found a client that deletes one index at
// a time.
func (s *Server) IndexDeleteCalls() int {
	s.Lock()
	defer s.Unlock()
	return s.indexDeleteCalls
}

// SetRecoveryActive drives what _recovery reports for every queried index.
// true answers one shard in stage INDEX with a source of type SNAPSHOT, the
// way a running restore reads. false answers the same shard in stage DONE:
// Elasticsearch keeps a finished recovery in the cluster state, so a client
// must read the stage and not only the presence of an entry.
func (s *Server) SetRecoveryActive(active bool) {
	s.Lock()
	defer s.Unlock()
	s.recoveryActive = active
}

// ReloadCalls reports the number of secure-settings reloads.
func (s *Server) ReloadCalls() int {
	s.Lock()
	defer s.Unlock()
	return s.reloadCalls
}

// StatsCalls reports how often _nodes/stats/fs was queried, injected
// failures included. A test asserts with it that a caller does not probe the
// statistics at a moment it must not.
func (s *Server) StatsCalls() int {
	s.Lock()
	defer s.Unlock()
	return s.statsCalls
}

// SetNodeFS sets what _nodes/stats/fs reports for the node name, adding the
// node when absent. The fake starts with one node, node-0, so setting that
// name replaces the default and any other name adds a node beside it.
func (s *Server) SetNodeFS(name string, totalBytes, usedBytes int64) {
	s.Lock()
	defer s.Unlock()
	s.nodeFS[name] = nodeFS{total: totalBytes, used: usedBytes}
}

func (s *Server) handle(w http.ResponseWriter, r *http.Request) {
	// Split the escaped form, then unescape each segment: an escaped slash
	// inside a name must stay inside its segment, the way a real server
	// routes.
	route := strings.Trim(r.URL.EscapedPath(), "/")
	parts := strings.Split(route, "/")
	for i, part := range parts {
		if unescaped, err := url.PathUnescape(part); err == nil {
			parts[i] = unescaped
		}
	}
	route, _ = url.PathUnescape(route)

	switch {
	case r.Method == http.MethodPost && route == "_nodes/reload_secure_settings":
		if s.Dropping(w, "reload") {
			return
		}
		if s.Failing("reload") {
			errorBody(w, http.StatusInternalServerError, "injected reload failure")
			return
		}
		s.reloadCalls++
		adminhttptest.WriteJSON(w, http.StatusOK, map[string]any{"nodes": map[string]any{}})

	case r.Method == http.MethodGet && route == "_nodes/stats/fs":
		s.statsCalls++
		if s.Dropping(w, "stats") {
			return
		}
		if s.Failing("stats") {
			errorBody(w, http.StatusInternalServerError, "injected stats failure")
			return
		}
		nodes := map[string]any{}
		for name, fs := range s.nodeFS {
			nodes[name] = map[string]any{
				"fs": map[string]any{
					"total": map[string]any{
						"total_in_bytes":     fs.total,
						"available_in_bytes": fs.total - fs.used,
					},
				},
			}
		}
		adminhttptest.WriteJSON(w, http.StatusOK, map[string]any{"nodes": nodes})

	case len(parts) == 2 && parts[0] == snapshotPath:
		s.handleRepository(w, r, parts[1])

	case len(parts) == 3 && parts[0] == snapshotPath:
		s.handleSnapshot(w, r, parts)

	case len(parts) == 4 && parts[0] == snapshotPath && parts[3] == restorePath && r.Method == http.MethodPost:
		s.handleRestore(w, r, parts)

	case len(parts) == 2 && parts[1] == recoveryPath && r.Method == http.MethodGet:
		s.handleRecovery(w, parts[0])

	case len(parts) == 1 && parts[0] != "":
		s.handleIndex(w, r, parts[0])

	default:
		errorBody(w, http.StatusNotFound, "unknown path "+r.URL.Path)
	}
}

func errorBody(w http.ResponseWriter, status int, message string) {
	errorBodyTyped(w, status, "exception", message)
}

// errorBodyTyped writes the error shape of Elasticsearch:
// {"error":{"type":...,"reason":...},"status":N}.
func errorBodyTyped(w http.ResponseWriter, status int, errorType, reason string) {
	adminhttptest.WriteJSON(w, status, map[string]any{
		"error":  map[string]string{"type": errorType, "reason": reason},
		"status": status,
	})
}

// handleRepository serves the registration and the read of one snapshot
// repository: PUT registers or updates it, and GET answers it or reports it
// missing.
func (s *Server) handleRepository(w http.ResponseWriter, r *http.Request, name string) {
	switch r.Method {
	case http.MethodGet:
		if s.Dropping(w, "repositoryGet") {
			return
		}
		if s.Failing("repositoryGet") {
			errorBody(w, http.StatusInternalServerError, "injected repository read failure")
			return
		}
		repo, registered := s.repos[name]
		if !registered {
			errorBodyTyped(
				w, http.StatusNotFound, "repository_missing_exception", "["+name+"] missing",
			)
			return
		}
		adminhttptest.WriteJSON(w, http.StatusOK, map[string]any{
			name: map[string]any{"type": repo.Type, "settings": repo.Settings},
		})

	case http.MethodPut:
		if s.Dropping(w, "repository") {
			return
		}
		if s.Failing("repository") {
			errorBody(w, http.StatusInternalServerError, "injected repository failure")
			return
		}
		var body struct {
			Type     string         `json:"type"`
			Settings map[string]any `json:"settings"`
		}
		_ = json.NewDecoder(r.Body).Decode(&body)
		s.repos[name] = &Repository{Type: body.Type, Settings: body.Settings}
		s.repositoryPuts[name]++
		adminhttptest.WriteJSON(w, http.StatusOK, map[string]any{"acknowledged": true})

	default:
		errorBody(w, http.StatusMethodNotAllowed, "unsupported method "+r.Method)
	}
}

// handleRestore serves POST /_snapshot/<repo>/<snapshot>/_restore. The
// repository and the snapshot must both exist, the way they must on a real
// cluster, so a restore of an artifact that is gone reads as a rejection and
// not as a start.
func (s *Server) handleRestore(w http.ResponseWriter, r *http.Request, parts []string) {
	if s.Dropping(w, "snapshotRestore") {
		return
	}
	if s.Failing("snapshotRestore") {
		errorBody(w, http.StatusInternalServerError, "injected snapshot restore failure")
		return
	}
	if _, ok := s.repos[parts[1]]; !ok {
		errorBodyTyped(
			w, http.StatusNotFound,
			"repository_missing_exception", "["+parts[1]+"] missing",
		)
		return
	}
	if _, ok := s.snapshots[parts[1]+"/"+parts[2]]; !ok {
		errorBodyTyped(
			w, http.StatusNotFound,
			"snapshot_missing_exception", "["+parts[1]+":"+parts[2]+"] is missing",
		)
		return
	}

	var body struct {
		Indices string `json:"indices"`
	}
	_ = json.NewDecoder(r.Body).Decode(&body)

	var indices []string
	if body.Indices != "" {
		indices = strings.Split(body.Indices, ",")
	}
	waitForCompletion, _ := strconv.ParseBool(r.URL.Query().Get("wait_for_completion"))
	s.restores = append(s.restores, RestoreRequest{
		Repo:              parts[1],
		Name:              parts[2],
		Indices:           indices,
		WaitForCompletion: waitForCompletion,
	})
	adminhttptest.WriteJSON(w, http.StatusOK, map[string]any{"accepted": true})
}

// handleRecovery serves GET /<target>/_recovery. It answers one shard entry
// per named target, with the source of the last accepted restore, so a client
// reads the shape that a restored index has.
func (s *Server) handleRecovery(w http.ResponseWriter, target string) {
	if s.Dropping(w, "recovery") {
		return
	}
	if s.Failing("recovery") {
		errorBody(w, http.StatusInternalServerError, "injected recovery failure")
		return
	}

	stage := recoveryStageDone
	if s.recoveryActive {
		stage = recoveryStageActive
	}

	var repo, snapshot string
	if last := len(s.restores) - 1; last >= 0 {
		repo, snapshot = s.restores[last].Repo, s.restores[last].Name
	}

	indices := map[string]any{}
	for name := range strings.SplitSeq(target, ",") {
		indices[name] = map[string]any{"shards": []map[string]any{{
			"id":      0,
			"type":    "SNAPSHOT",
			"stage":   stage,
			"primary": true,
			"source": map[string]any{
				"repository": repo,
				"snapshot":   snapshot,
				"index":      name,
			},
		}}}
	}
	adminhttptest.WriteJSON(w, http.StatusOK, indices)
}

// handleIndex routes the requests that name indices: the resolution and the
// deletion.
func (s *Server) handleIndex(w http.ResponseWriter, r *http.Request, target string) {
	switch r.Method {
	case http.MethodGet:
		s.handleIndexResolve(w, r, target)
	case http.MethodDelete:
		s.handleIndexDelete(w, r, target)
	default:
		errorBody(w, http.StatusMethodNotAllowed, "unsupported method "+r.Method)
	}
}

// handleIndexResolve serves GET /<target>, the get-index API. It answers the
// object of the seeded indices that the target matches, keyed by index name.
func (s *Server) handleIndexResolve(w http.ResponseWriter, r *http.Request, target string) {
	if s.Dropping(w, "indexResolve") {
		return
	}
	if s.Failing("indexResolve") {
		errorBody(w, http.StatusInternalServerError, "injected index resolve failure")
		return
	}

	query := r.URL.Query()
	if !tolerates(query) {
		errorBodyTyped(
			w, http.StatusNotFound,
			"index_not_found_exception", "no such index ["+target+"]",
		)
		return
	}
	if query.Get("expand_wildcards") != resolveWildcards {
		errorBodyTyped(
			w, http.StatusBadRequest, "illegal_argument_exception",
			"expand_wildcards must be "+resolveWildcards+", or a closed index is never resolved",
		)
		return
	}

	response := map[string]any{}
	for _, name := range s.matching(target) {
		response[name] = map[string]any{
			"aliases":  map[string]any{},
			"mappings": map[string]any{},
			"settings": map[string]any{"index": map[string]any{"provided_name": name}},
		}
	}
	adminhttptest.WriteJSON(w, http.StatusOK, response)
}

// handleIndexDelete serves DELETE /<target>. It removes every seeded index
// that the target matches and records what the request named.
func (s *Server) handleIndexDelete(w http.ResponseWriter, r *http.Request, target string) {
	if s.Dropping(w, "indexDelete") {
		return
	}
	if s.Failing("indexDelete") {
		errorBody(w, http.StatusInternalServerError, "injected index delete failure")
		return
	}

	if !tolerates(r.URL.Query()) {
		errorBodyTyped(
			w, http.StatusNotFound,
			"index_not_found_exception", "no such index ["+target+"]",
		)
		return
	}

	for _, name := range s.matching(target) {
		delete(s.indices, name)
	}
	s.deletedIndices = append(s.deletedIndices, strings.Split(target, ",")...)
	s.indexDeletePaths = append(s.indexDeletePaths, r.URL.EscapedPath())
	s.indexDeleteCalls++
	adminhttptest.WriteJSON(w, http.StatusOK, map[string]any{"acknowledged": true})
}

// tolerates reports whether a request tolerates a target that matches
// nothing. An index can be gone between the resolution and the delete, and a
// pattern of the caller can match nothing at all, so both requests must
// carry it.
func tolerates(query url.Values) bool {
	ignoreUnavailable, _ := strconv.ParseBool(query.Get("ignore_unavailable"))
	allowNoIndices, _ := strconv.ParseBool(query.Get("allow_no_indices"))

	return ignoreUnavailable && allowNoIndices
}

// matching returns the seeded indices that target matches, sorted. target is
// a comma-separated list of index names and wildcard patterns, the
// multi-target syntax of Elasticsearch.
func (s *Server) matching(target string) []string {
	matched := map[string]struct{}{}
	for pattern := range strings.SplitSeq(target, ",") {
		for name := range s.indices {
			if ok, err := path.Match(pattern, name); err == nil && ok {
				matched[name] = struct{}{}
			}
		}
	}

	return sortedKeys(matched)
}

// sortedKeys returns the keys of a set, sorted, so an answer of the fake is
// deterministic.
func sortedKeys(set map[string]struct{}) []string {
	names := make([]string, 0, len(set))
	for name := range set {
		names = append(names, name)
	}
	slices.Sort(names)

	return names
}

// handleSnapshot routes the per-snapshot requests: create, status, delete.
func (s *Server) handleSnapshot(w http.ResponseWriter, r *http.Request, parts []string) {
	switch r.Method {
	case http.MethodPut:
		if s.Dropping(w, "snapshotCreate") {
			return
		}
		if s.Failing("snapshotCreate") {
			errorBody(w, http.StatusInternalServerError, "injected snapshot create failure")
			return
		}
		if _, ok := s.repos[parts[1]]; !ok {
			errorBodyTyped(
				w, http.StatusNotFound,
				"repository_missing_exception", "["+parts[1]+"] missing",
			)
			return
		}
		key := parts[1] + "/" + parts[2]
		if _, exists := s.snapshots[key]; exists {
			errorBody(
				w, http.StatusBadRequest,
				"invalid_snapshot_name_exception: snapshot with the same name already exists",
			)
			return
		}
		var body struct {
			Indices  string         `json:"indices"`
			Metadata map[string]any `json:"metadata"`
		}
		_ = json.NewDecoder(r.Body).Decode(&body)
		s.snapshots[key] = &Snapshot{
			Repo: parts[1], Name: parts[2], State: "IN_PROGRESS",
			Indices: body.Indices, Metadata: body.Metadata,
		}
		s.snapshotCreates[key]++
		adminhttptest.WriteJSON(w, http.StatusOK, map[string]any{"accepted": true})

	case http.MethodGet:
		if s.Dropping(w, "snapshotStatus") {
			return
		}
		if s.Failing("snapshotStatus") {
			errorBody(w, http.StatusInternalServerError, "injected snapshot status failure")
			return
		}
		if _, ok := s.repos[parts[1]]; !ok {
			errorBodyTyped(
				w, http.StatusNotFound,
				"repository_missing_exception", "["+parts[1]+"] missing",
			)
			return
		}
		snapshot, ok := s.snapshots[parts[1]+"/"+parts[2]]
		if !ok {
			errorBodyTyped(
				w, http.StatusNotFound,
				"snapshot_missing_exception", "["+parts[1]+":"+parts[2]+"] is missing",
			)
			return
		}
		info := map[string]any{"snapshot": snapshot.Name, "state": snapshot.State}
		if len(snapshot.Metadata) > 0 {
			info["metadata"] = snapshot.Metadata
		}
		adminhttptest.WriteJSON(w, http.StatusOK, map[string]any{"snapshots": []map[string]any{info}})

	case http.MethodDelete:
		if s.Dropping(w, "snapshotDelete") {
			return
		}
		if s.Failing("snapshotDelete") {
			errorBody(w, http.StatusInternalServerError, "injected snapshot delete failure")
			return
		}
		if _, ok := s.repos[parts[1]]; !ok {
			errorBodyTyped(
				w, http.StatusNotFound,
				"repository_missing_exception", "["+parts[1]+"] missing",
			)
			return
		}
		key := parts[1] + "/" + parts[2]
		if _, ok := s.snapshots[key]; !ok {
			errorBodyTyped(
				w, http.StatusNotFound,
				"snapshot_missing_exception", "["+parts[1]+":"+parts[2]+"] is missing",
			)
			return
		}
		delete(s.snapshots, key)
		adminhttptest.WriteJSON(w, http.StatusOK, map[string]any{"acknowledged": true})

	default:
		errorBody(w, http.StatusMethodNotAllowed, "unsupported method "+r.Method)
	}
}
