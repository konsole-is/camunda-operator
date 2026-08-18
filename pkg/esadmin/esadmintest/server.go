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
// pkg/esadmin calls: snapshot repositories, snapshots, secure-settings
// reload, and node filesystem statistics.
package esadmintest

import (
	"encoding/json"
	"encoding/pem"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
)

// snapshotPath is the first path segment of every snapshot API call.
const snapshotPath = "_snapshot"

// Repository is the fake's record of one snapshot repository.
type Repository struct {
	// Type of the repository, s3 for everything this operator registers.
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
}

// Server fakes the Elasticsearch admin surface. Every exported method is
// safe for concurrent use.
type Server struct {
	mu sync.Mutex

	repos     map[string]*Repository
	snapshots map[string]*Snapshot

	repositoryPuts map[string]int

	snapshotCreates map[string]int
	reloadCalls     int
	statsCalls      int

	// nodeFS drives _nodes/stats/fs, keyed by node name.
	nodeFS map[string]nodeFS

	failures map[string]int

	server *httptest.Server
}

// nodeFS is the filesystem report of one node.
type nodeFS struct {
	total int64
	used  int64
}

// New starts the fake over plain HTTP. Close it with Close.
func New() *Server {
	s := newServer()
	s.server = httptest.NewServer(http.HandlerFunc(s.handle))

	return s
}

// NewTLS starts the fake over HTTPS with a self-signed certificate, the way
// ECK serves Elasticsearch. CertificatePEM returns the bundle that verifies
// it, so a test exercises the same CA path as production. Close it with
// Close.
func NewTLS() *Server {
	s := newServer()
	s.server = httptest.NewTLSServer(http.HandlerFunc(s.handle))

	return s
}

func newServer() *Server {
	return &Server{
		repos:           map[string]*Repository{},
		repositoryPuts:  map[string]int{},
		snapshots:       map[string]*Snapshot{},
		snapshotCreates: map[string]int{},
		nodeFS:          map[string]nodeFS{"node-0": {total: 100 << 30, used: 10 << 30}},
		failures:        map[string]int{},
	}
}

// CertificatePEM returns the PEM encoding of the certificate a NewTLS server
// serves, which is its own CA: the certificate is self-signed.
func (s *Server) CertificatePEM() []byte {
	cert := s.server.Certificate()
	if cert == nil {
		return nil
	}

	return pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: cert.Raw})
}

// URL is the base URL of the fake.
func (s *Server) URL() string { return s.server.URL }

// Close stops the fake.
func (s *Server) Close() { s.server.Close() }

// Repository returns the registered repository name, or nil.
func (s *Server) Repository(name string) *Repository {
	s.mu.Lock()
	defer s.mu.Unlock()
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
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.repositoryPuts[name]
}

// SetSnapshotState sets the state of the snapshot repo/name, creating it
// when absent.
func (s *Server) SetSnapshotState(repo, name, state string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.snapshots[repo+"/"+name] = &Snapshot{Repo: repo, Name: name, State: state}
}

// SnapshotExists reports whether the snapshot repo/name exists.
func (s *Server) SnapshotExists(repo, name string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	_, ok := s.snapshots[repo+"/"+name]
	return ok
}

// SnapshotCreates reports how often a create of repo/name was accepted.
func (s *Server) SnapshotCreates(repo, name string) int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.snapshotCreates[repo+"/"+name]
}

// ReloadCalls reports the number of secure-settings reloads.
func (s *Server) ReloadCalls() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.reloadCalls
}

// StatsCalls reports how often _nodes/stats/fs was queried, injected
// failures included. A test asserts with it that a caller does not probe the
// statistics at a moment it must not.
func (s *Server) StatsCalls() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.statsCalls
}

// SetNodeFS sets what _nodes/stats/fs reports for the node name, adding the
// node when absent. The fake starts with one node, node-0, so setting that
// name replaces the default and any other name adds a node beside it.
func (s *Server) SetNodeFS(name string, totalBytes, usedBytes int64) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.nodeFS[name] = nodeFS{total: totalBytes, used: usedBytes}
}

// FailNext makes the next n calls of op answer 500. op is one of
// "repository", "snapshotCreate", "snapshotStatus", "snapshotDelete",
// "reload", "stats".
func (s *Server) FailNext(op string, n int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.failures[op] = n
}

func (s *Server) failing(op string) bool {
	if s.failures[op] > 0 {
		s.failures[op]--
		return true
	}
	return false
}

func (s *Server) handle(w http.ResponseWriter, r *http.Request) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Split the escaped form, then unescape each segment: an escaped slash
	// inside a name must stay inside its segment, the way a real server
	// routes.
	path := strings.Trim(r.URL.EscapedPath(), "/")
	parts := strings.Split(path, "/")
	for i, part := range parts {
		if unescaped, err := url.PathUnescape(part); err == nil {
			parts[i] = unescaped
		}
	}
	path, _ = url.PathUnescape(path)

	switch {
	case r.Method == http.MethodPost && path == "_nodes/reload_secure_settings":
		if s.failing("reload") {
			errorBody(w, http.StatusInternalServerError, "injected reload failure")
			return
		}
		s.reloadCalls++
		writeJSON(w, http.StatusOK, map[string]any{"nodes": map[string]any{}})

	case r.Method == http.MethodGet && path == "_nodes/stats/fs":
		s.statsCalls++
		if s.failing("stats") {
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
		writeJSON(w, http.StatusOK, map[string]any{"nodes": nodes})

	case len(parts) == 2 && parts[0] == snapshotPath && r.Method == http.MethodPut:
		if s.failing("repository") {
			errorBody(w, http.StatusInternalServerError, "injected repository failure")
			return
		}
		var body struct {
			Type     string         `json:"type"`
			Settings map[string]any `json:"settings"`
		}
		_ = json.NewDecoder(r.Body).Decode(&body)
		s.repos[parts[1]] = &Repository{Type: body.Type, Settings: body.Settings}
		s.repositoryPuts[parts[1]]++
		writeJSON(w, http.StatusOK, map[string]any{"acknowledged": true})

	case len(parts) == 3 && parts[0] == snapshotPath:
		s.handleSnapshot(w, r, parts)

	default:
		errorBody(w, http.StatusNotFound, "unknown path "+r.URL.Path)
	}
}

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}

func errorBody(w http.ResponseWriter, status int, message string) {
	errorBodyTyped(w, status, "exception", message)
}

// errorBodyTyped writes the error shape of Elasticsearch:
// {"error":{"type":...,"reason":...},"status":N}.
func errorBodyTyped(w http.ResponseWriter, status int, errorType, reason string) {
	writeJSON(w, status, map[string]any{
		"error":  map[string]string{"type": errorType, "reason": reason},
		"status": status,
	})
}

// handleSnapshot routes the per-snapshot requests: create, status, delete.
// The caller holds the lock.
func (s *Server) handleSnapshot(w http.ResponseWriter, r *http.Request, parts []string) {
	switch r.Method {
	case http.MethodPut:
		if s.failing("snapshotCreate") {
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
			Indices string `json:"indices"`
		}
		_ = json.NewDecoder(r.Body).Decode(&body)
		s.snapshots[key] = &Snapshot{Repo: parts[1], Name: parts[2], State: "IN_PROGRESS", Indices: body.Indices}
		s.snapshotCreates[key]++
		writeJSON(w, http.StatusOK, map[string]any{"accepted": true})

	case http.MethodGet:
		if s.failing("snapshotStatus") {
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
		writeJSON(w, http.StatusOK, map[string]any{
			"snapshots": []map[string]any{{"snapshot": snapshot.Name, "state": snapshot.State}},
		})

	case http.MethodDelete:
		if s.failing("snapshotDelete") {
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
		writeJSON(w, http.StatusOK, map[string]any{"acknowledged": true})

	default:
		errorBody(w, http.StatusMethodNotAllowed, "unsupported method "+r.Method)
	}
}
