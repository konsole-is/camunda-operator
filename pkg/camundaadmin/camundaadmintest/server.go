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

// Package camundaadmintest fakes the Camunda 8.9 management API for tests.
// It serves the endpoints that pkg/camundaadmin calls with the status codes
// and body shapes of the real API, tracks every call, and lets a test drive
// backup states and inject failures.
package camundaadmintest

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync"
)

// Backup is the fake's record of one backup on either endpoint.
type Backup struct {
	// ID of the backup.
	ID int64
	// State in the vocabulary of the real API, for example IN_PROGRESS.
	State string
	// FailureReason reported when State is FAILED.
	FailureReason string
}

// Server fakes the management API. Every exported method is safe for
// concurrent use.
type Server struct {
	mu sync.Mutex

	// Exporting is the exporting state: "running", "softPaused", or
	// "paused". It starts as "running".
	exporting string

	pauseCalls  int
	resumeCalls int
	// resumeAttempts counts every resume request, the injected failures
	// included; resumeCalls counts the ones that resumed.
	resumeAttempts int

	conflictRuntimeStarts int

	history map[int64]*Backup
	runtime map[int64]*Backup
	// deletedRuntime holds the ids of deleted runtime backups. An id is never
	// reusable, deleted or not, so the conflict check consults both.
	deletedRuntime map[int64]bool

	historyStarts map[int64]int
	runtimeStarts map[int64]int

	// hiddenRuntimeStatus is how many more runtime status queries answer 404
	// for a backup that the fake holds. It fakes the registration lag of the
	// real cluster after it accepted a start.
	hiddenRuntimeStatus int
	// hiddenHistoryStatus is the same for history status queries.
	hiddenHistoryStatus int

	nextGeneratedID int64

	failures map[string]int

	server *httptest.Server
}

// New starts the fake. Close it with Close.
func New() *Server {
	s := &Server{
		exporting:       "running",
		history:         map[int64]*Backup{},
		runtime:         map[int64]*Backup{},
		deletedRuntime:  map[int64]bool{},
		historyStarts:   map[int64]int{},
		runtimeStarts:   map[int64]int{},
		nextGeneratedID: 1,
		failures:        map[string]int{},
	}
	s.server = httptest.NewServer(http.HandlerFunc(s.handle))

	return s
}

// URL is the base URL of the fake, used as the binding endpoint.
func (s *Server) URL() string { return s.server.URL }

// Close stops the fake.
func (s *Server) Close() { s.server.Close() }

// Exporting reports the exporting state: "running", "softPaused", or
// "paused".
func (s *Server) Exporting() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.exporting
}

// PauseCalls and ResumeCalls report how often the exporting endpoints were
// called, so a test can assert that a re-entrant caller does not repeat a
// POST.
func (s *Server) PauseCalls() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.pauseCalls
}

// ResumeAttempts reports how often resume was requested, injected failures
// included. A test paces itself on the retries of a caller with it.
func (s *Server) ResumeAttempts() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.resumeAttempts
}

// ResumeCalls reports the number of resume calls that resumed exporting.
func (s *Server) ResumeCalls() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.resumeCalls
}

// HistoryStarts reports how often a history backup start was accepted for
// id.
func (s *Server) HistoryStarts(id int64) int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.historyStarts[id]
}

// RuntimeStarts reports how often a runtime backup start was accepted for
// id.
func (s *Server) RuntimeStarts(id int64) int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.runtimeStarts[id]
}

// HideRuntimeStatus makes the next n runtime status queries answer 404 for a
// backup that exists. The real cluster registers a runtime backup
// asynchronously and can report it absent for a moment after it accepted the
// start. A second start for the same id conflicts during that moment.
func (s *Server) HideRuntimeStatus(n int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.hiddenRuntimeStatus = n
}

// HideHistoryStatus makes the next n history status queries answer 404 for
// a backup that exists, the way the web applications report an accepted
// history backup absent while it registers.
func (s *Server) HideHistoryStatus(n int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.hiddenHistoryStatus = n
}

// SetHistoryState sets the state of the history backup id, creating it when
// absent.
func (s *Server) SetHistoryState(id int64, state, failureReason string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.history[id] = &Backup{ID: id, State: state, FailureReason: failureReason}
}

// SetRuntimeState sets the state of the runtime backup id, creating it when
// absent.
func (s *Server) SetRuntimeState(id int64, state, failureReason string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.runtime[id] = &Backup{ID: id, State: state, FailureReason: failureReason}
}

// RuntimeBackup returns the runtime backup id, or nil.
func (s *Server) RuntimeBackup(id int64) *Backup {
	s.mu.Lock()
	defer s.mu.Unlock()
	if b, ok := s.runtime[id]; ok {
		copied := *b
		return &copied
	}
	return nil
}

// ConflictNextRuntimeStart answers the next n runtime-backup starts with a
// conflict, the way a cluster does when a higher id landed between the
// generation and the registration of one.
func (s *Server) ConflictNextRuntimeStart(n int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.conflictRuntimeStarts = n
}

// FailNext makes the next n calls of op answer 500. op is one of "pause",
// "resume", "historyStart", "historyStatus", "runtimeStart",
// "runtimeStatus", "runtimeDelete".
func (s *Server) FailNext(op string, n int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.failures[op] = n
}

// failing consumes one injected failure of op.
func (s *Server) failing(op string) bool {
	if s.failures[op] > 0 {
		s.failures[op]--
		return true
	}
	return false
}

// handleExporting serves the pause and resume operations. Both answer the
// envelope that Camunda 8.9 answers: HTTP 200 with the outcome inside.
func (s *Server) handleExporting(w http.ResponseWriter, r *http.Request, operation string) {
	switch operation {
	case "pause":
		if s.failing("pause") {
			exportingEnvelope(w, http.StatusInternalServerError, "injected pause failure")
			return
		}
		s.pauseCalls++
		if r.URL.Query().Get("soft") == "true" {
			s.exporting = "softPaused"
		} else {
			s.exporting = "paused"
		}
		exportingEnvelope(w, http.StatusNoContent, "")

	case "resume":
		s.resumeAttempts++
		if s.failing("resume") {
			exportingEnvelope(w, http.StatusInternalServerError, "injected resume failure")
			return
		}
		s.resumeCalls++
		s.exporting = "running"
		exportingEnvelope(w, http.StatusNoContent, "")

	default:
		errorBody(w, http.StatusNotFound, "unknown exporting operation "+operation)
	}
}

func (s *Server) handle(w http.ResponseWriter, r *http.Request) {
	s.mu.Lock()
	defer s.mu.Unlock()

	path := strings.TrimPrefix(r.URL.Path, "/actuator")

	if strings.HasPrefix(path, "/exporting/") && r.Method == http.MethodPost {
		s.handleExporting(w, r, strings.TrimPrefix(path, "/exporting/"))
		return
	}

	switch {
	case r.Method == http.MethodPost && path == "/backupHistory":
		if s.failing("historyStart") {
			errorBody(w, http.StatusInternalServerError, "injected history start failure")
			return
		}
		id := decodeBackupID(r)
		if _, exists := s.history[id]; exists {
			errorBody(w, http.StatusBadRequest, "a backup with ID "+strconv.FormatInt(id, 10)+" already exists")
			return
		}
		s.history[id] = &Backup{ID: id, State: "IN_PROGRESS"}
		s.historyStarts[id]++
		writeJSON(w, http.StatusOK, map[string]any{"scheduledSnapshots": []string{
			"camunda_webapps_" + strconv.FormatInt(id, 10) + "_8.9_part_1_of_1",
		}})

	case r.Method == http.MethodGet && strings.HasPrefix(path, "/backupHistory/"):
		if s.failing("historyStatus") {
			errorBody(w, http.StatusInternalServerError, "injected history status failure")
			return
		}
		id, _ := strconv.ParseInt(strings.TrimPrefix(path, "/backupHistory/"), 10, 64)
		backup, ok := s.history[id]
		if ok && s.hiddenHistoryStatus > 0 {
			s.hiddenHistoryStatus--
			ok = false
		}
		if !ok {
			errorBody(w, http.StatusNotFound, "backup does not exist")
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"backupId":      backup.ID,
			"state":         backup.State,
			"failureReason": backup.FailureReason,
			"details": []map[string]any{{
				"snapshotName": "camunda_webapps_" + strconv.FormatInt(id, 10) + "_8.9_part_1_of_1",
				"state":        backup.State,
			}},
		})

	case r.Method == http.MethodPost && path == "/backupRuntime":
		s.handleRuntimeStart(w, r)

	case r.Method == http.MethodGet && strings.HasPrefix(path, "/backupRuntime/"):
		s.handleRuntimeStatus(w, path)

	case r.Method == http.MethodDelete && strings.HasPrefix(path, "/backupRuntime/"):
		if s.failing("runtimeDelete") {
			errorBody(w, http.StatusInternalServerError, "injected runtime delete failure")
			return
		}
		id, _ := strconv.ParseInt(strings.TrimPrefix(path, "/backupRuntime/"), 10, 64)
		backup, ok := s.runtime[id]
		if !ok {
			// The documented responses are 204, 400, 500, 502, and 504 —
			// there is no 404. Deleting an absent backup answers 204.
			w.WriteHeader(http.StatusNoContent)
			return
		}
		if backup.State == "IN_PROGRESS" {
			errorBody(w, http.StatusBadRequest, "backup is in progress and cannot be deleted")
			return
		}
		delete(s.runtime, id)
		s.deletedRuntime[id] = true
		w.WriteHeader(http.StatusNoContent)

	default:
		errorBody(w, http.StatusNotFound, "unknown path "+r.URL.Path)
	}
}

// handleRuntimeStart answers POST /backupRuntime: an explicit id conflicts
// with any same-or-higher (or deleted) id; a nil id gets a generated one.
func (s *Server) handleRuntimeStart(w http.ResponseWriter, r *http.Request) {
	if s.failing("runtimeStart") {
		errorBody(w, http.StatusInternalServerError, "injected runtime start failure")
		return
	}
	var id int64
	if r.ContentLength != 0 {
		id = decodeBackupID(r)
		conflicts := func(existing int64) bool { return existing >= id }
		conflicted := false
		for existing := range s.runtime {
			conflicted = conflicted || conflicts(existing)
		}
		// A deleted id conflicts too: ids are never reusable, even after
		// the backup they named is gone.
		for deleted := range s.deletedRuntime {
			conflicted = conflicted || conflicts(deleted)
		}
		if conflicted {
			conflictBody(w)
			return
		}
	} else {
		// The injected conflict targets only cluster-generated ids: a higher
		// id landing between the generation and the registration is the one
		// conflict an id-less request can hit.
		if s.conflictRuntimeStarts > 0 {
			s.conflictRuntimeStarts--
			conflictBody(w)
			return
		}
		id = s.nextGeneratedID
		s.nextGeneratedID++
	}
	s.runtime[id] = &Backup{ID: id, State: "IN_PROGRESS"}
	s.runtimeStarts[id]++

	// The cluster echoes an id of its own when one was supplied: the
	// documented example answers a request for 100 with 1772011199310.
	// The backup keys on the supplied id, so a client that trusts the
	// echo loses track of its own snapshots.
	echoed := id
	if r.ContentLength != 0 {
		echoed = s.nextGeneratedID
		s.nextGeneratedID++
	}
	writeJSON(w, http.StatusAccepted, map[string]any{
		"backupId": echoed,
		"message":  "A backup has been scheduled",
	})
}

// handleRuntimeStatus serves GET /backupRuntime/{id}. The caller holds the
// lock.
func (s *Server) handleRuntimeStatus(w http.ResponseWriter, path string) {
	if s.failing("runtimeStatus") {
		errorBody(w, http.StatusInternalServerError, "injected runtime status failure")
		return
	}
	id, _ := strconv.ParseInt(strings.TrimPrefix(path, "/backupRuntime/"), 10, 64)
	backup, ok := s.runtime[id]
	if ok && s.hiddenRuntimeStatus > 0 {
		s.hiddenRuntimeStatus--
		ok = false
	}
	if !ok {
		errorBody(w, http.StatusNotFound, "backup does not exist")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"backupId":      backup.ID,
		"state":         backup.State,
		"failureReason": backup.FailureReason,
		"details": []map[string]any{{
			"partitionId":   1,
			"state":         backup.State,
			"failureReason": backup.FailureReason,
		}},
	})
}

// decodeBackupID reads {"backupId": N} from the request body.
func decodeBackupID(r *http.Request) int64 {
	var body struct {
		BackupID int64 `json:"backupId"`
	}
	_ = json.NewDecoder(r.Body).Decode(&body)
	return body.BackupID
}

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}

func errorBody(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]string{"message": message})
}

// conflictBody answers the 409 a cluster answers when an id is not usable.
func conflictBody(w http.ResponseWriter) {
	errorBody(w, http.StatusConflict, "a backup with the same or higher ID already exists")
}

// exportingEnvelope answers an exporting call the way Camunda 8.9 does: the
// HTTP status is always 200, and the outcome is the status field of the body.
// A message produces the failure shape, which nests it under body.
func exportingEnvelope(w http.ResponseWriter, status int, message string) {
	envelope := map[string]any{"status": status, "contentType": nil, "body": nil}
	if message != "" {
		envelope["body"] = map[string]string{"message": message}
	}

	writeJSON(w, http.StatusOK, envelope)
}
