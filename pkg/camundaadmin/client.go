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

// Package camundaadmin is the management API client of the orchestration
// cluster: pure HTTP against the management port, no Kubernetes types. It is
// constructed from the management binding that a CamundaCluster publishes,
// and it covers exactly the surface that backups need.
//
// The Camunda version of the binding selects the endpoint set at
// construction. One set exists today, 8.9: the unified management API
// (/actuator/exporting/*, /actuator/backupHistory, /actuator/backupRuntime).
// An unknown version is a constructor error, never a guess.
//
// The exporting calls and the deletes are idempotent from the caller's
// view: the backup state machine re-enters after a crash, so "already done"
// is success and never an error. The two backup starts are the exception.
// A duplicate under the same or a higher id is ErrConflict, never success.
// A duplicate is only "already done" when the existing backup is the
// caller's own, and the client cannot know that. The caller decides from
// its own recorded state. Errors distinguish an unreachable endpoint
// (ErrUnreachable) from a rejected call (ErrRejected, with the response
// body in the message).
package camundaadmin

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/konsole-is/camunda-operator/pkg/adminhttp"
)

// ErrUnreachable, ErrRejected, and ErrConflict classify every failure of the
// client. ErrUnreachable means the endpoint did not answer (connection,
// timeout); the caller maps it to ConnectionFailed and retries. ErrRejected
// means the endpoint answered with an error; the message carries the response
// body.
//
// ErrConflict is the one failure a caller must interpret itself: a runtime
// backup with the same or a higher id already exists. Only the caller knows
// whether that id is its own. A backup re-entering on the id in its status
// may poll that backup, while one that just allocated an id must fail rather
// than adopt the artifacts of another backup.
var (
	ErrUnreachable = errors.New("management API unreachable")
	ErrRejected    = errors.New("management API rejected the call")
	ErrConflict    = errors.New("a backup with the same or a higher id already exists")
)

// State is the aggregated state of a backup as the management API reports
// it.
type State string

// The states of the 8.9 backup endpoints. The history endpoint reports no
// DoesNotExist; a missing history backup is a 404, which the client maps to
// StateDoesNotExist.
const (
	StateDoesNotExist State = "DOES_NOT_EXIST"
	StateInProgress   State = "IN_PROGRESS"
	StateCompleted    State = "COMPLETED"
	StateFailed       State = "FAILED"
	StateIncomplete   State = "INCOMPLETE"
	StateIncompatible State = "INCOMPATIBLE"
	StateDeleted      State = "DELETED"
)

// Detail is the state of one part of a backup: a partition on the runtime
// endpoint, a snapshot on the history endpoint.
type Detail struct {
	// Name identifies the part: the partition ID or the snapshot name.
	Name string
	// State of the part, as the endpoint reports it.
	State string
	// Reason for failure, when the part failed.
	Reason string
}

// BackupStatus is the status of one backup: the aggregated state and the
// per-part details for the failure message.
type BackupStatus struct {
	// ID of the backup.
	ID int64
	// State is the aggregated state.
	State State
	// FailureReason is set when State is StateFailed.
	FailureReason string
	// Details lists the per-part states.
	Details []Detail
}

// Auth authenticates the client against the management port. The zero value
// is no authentication, the Camunda 8.9 default: the management port is not
// secured unless the user adds their own Spring Security configuration.
type Auth = adminhttp.BasicAuth

// Binding locates and authenticates the management API of one cluster. It
// mirrors the management binding that a CamundaCluster publishes in
// status.management.
type Binding struct {
	// Endpoint is the base URL of the management port on the workload
	// Service that hosts the gateway, for example
	// http://my-cluster-zeebe.ns.svc:9600 (or the gateway Service when the
	// topology runs one standalone).
	Endpoint string
	// Version is the Camunda version of the cluster, for example 8.9.9. Its
	// minor selects the endpoint set.
	Version string
	// Auth authenticates the calls.
	Auth Auth
}

// Client calls the management API of one cluster.
type Client struct {
	api *adminhttp.Client
}

// New builds a client for the cluster that binding describes. It returns an
// error when the endpoint is empty or the Camunda version is not one the
// client knows.
func New(binding Binding) (*Client, error) {
	if binding.Endpoint == "" {
		return nil, errors.New("management binding has no endpoint")
	}

	if !strings.HasPrefix(binding.Version, "8.9.") && binding.Version != "8.9" {
		return nil, fmt.Errorf("unsupported Camunda version %q: this client knows 8.9 only", binding.Version)
	}

	api, err := adminhttp.New(adminhttp.Config{
		Endpoint:    binding.Endpoint,
		Auth:        binding.Auth,
		Unreachable: ErrUnreachable,
		Rejected:    ErrRejected,
	})
	if err != nil {
		return nil, err
	}

	return &Client{api: api}, nil
}

// PauseExporting pauses exporting on every partition. With soft, records
// keep exporting but log compaction stops, which makes a hot backup
// possible. Pausing an already paused cluster is success.
//
// A failure can be partial: some partitions may have paused. The caller must
// read the error as "exporting is in an unknown state" and resume, which is
// what a backup does before it writes a terminal phase.
func (c *Client) PauseExporting(ctx context.Context, soft bool) error {
	path := "/actuator/exporting/pause"
	if soft {
		path += "?soft=true"
	}

	return c.exporting(ctx, path)
}

// ResumeExporting resumes exporting on every partition. Resuming a cluster
// that already exports is success. A failure can be partial here too, and the
// caller retries until it succeeds: a cluster left paused cannot compact its
// log and eventually fills its disks.
func (c *Client) ResumeExporting(ctx context.Context) error {
	return c.exporting(ctx, "/actuator/exporting/resume")
}

// exporting posts to an exporting endpoint and reads the outcome from the
// body. These endpoints always answer HTTP 200 and carry the real status in
// an envelope: 204 succeeded, and 500 failed with the reason under
// body.message. Reading the HTTP status alone would report every call, and
// every partial pause, as a success.
func (c *Client) exporting(ctx context.Context, path string) error {
	payload, _, err := c.api.Do(ctx, adminhttp.Request{
		Method: http.MethodPost,
		Path:   path,
		Accept: adminhttp.Status(http.StatusOK),
	})
	if err != nil {
		return err
	}

	var envelope struct {
		Status int `json:"status"`
		Body   struct {
			Message string `json:"message"`
		} `json:"body"`
	}
	if err := json.Unmarshal(payload, &envelope); err != nil {
		return fmt.Errorf("decoding exporting response of %s: %w", path, err)
	}

	if envelope.Status == http.StatusNoContent {
		return nil
	}

	message := envelope.Body.Message
	if message == "" {
		message = strings.TrimSpace(string(payload))
	}

	return fmt.Errorf("%w: POST %s reported status %d: %s", ErrRejected, path, envelope.Status, message)
}

// StartHistoryBackup starts the backup of the web-application indices under
// id. It returns the names of the snapshots that the cluster scheduled. The
// documentation requires the caller to persist the names with the backup id,
// because a restore locates the snapshots by them. A backup that already
// exists under the same id returns ErrConflict, like StartRuntimeBackup.
// The client cannot know whether that backup is the caller's own from an
// earlier request or another actor's under a reused id. If the client
// resolved it as success, it adopted a backup without ownership evidence.
// The caller therefore decides from its own recorded state.
func (c *Client) StartHistoryBackup(ctx context.Context, id int64) ([]string, error) {
	body, err := json.Marshal(map[string]int64{"backupId": id})
	if err != nil {
		return nil, fmt.Errorf("encoding request: %w", err)
	}

	// The endpoint answers 200 with the scheduled snapshot names and creates
	// the snapshots asynchronously. The caller polls HistoryBackupStatus for
	// the outcome. A 200 is an acceptance whatever its body holds. A body
	// that does not decode yields no names, and the status poll reports the
	// names later. If the client returned an error here, the caller lost an
	// acceptance that happened.
	payload, status, err := c.api.Do(ctx, adminhttp.Request{
		Method: http.MethodPost,
		Path:   "/actuator/backupHistory",
		Body:   body,
		Accept: adminhttp.Status(http.StatusOK),
	})
	if err == nil {
		var scheduled struct {
			ScheduledSnapshots []string `json:"scheduledSnapshots"`
		}
		if json.Unmarshal(payload, &scheduled) != nil {
			return nil, nil
		}
		return scheduled.ScheduledSnapshots, nil
	}

	// The endpoint answers 400 when the id already exists. 409 is treated
	// the same because it is the conventional code for the condition. Only
	// an id that the cluster really holds is a conflict. Any other 400
	// stays the rejection it is.
	if status == http.StatusBadRequest || status == http.StatusConflict {
		if existing, statusErr := c.HistoryBackupStatus(
			ctx,
			id,
		); statusErr == nil &&
			existing.State != StateDoesNotExist {
			return nil, fmt.Errorf("%w: %v", ErrConflict, err)
		}
	}

	return nil, err
}

// HistoryBackupStatus reports the status of the history backup id. A backup
// that does not exist is StateDoesNotExist, not an error.
func (c *Client) HistoryBackupStatus(ctx context.Context, id int64) (BackupStatus, error) {
	body, status, err := c.api.Do(ctx, adminhttp.Request{
		Method: http.MethodGet,
		Path:   "/actuator/backupHistory/" + strconv.FormatInt(id, 10),
		Accept: adminhttp.Status(http.StatusOK),
	})
	if status == http.StatusNotFound {
		return BackupStatus{ID: id, State: StateDoesNotExist}, nil
	}
	if err != nil {
		return BackupStatus{}, err
	}

	var info struct {
		BackupID      json.Number `json:"backupId"`
		State         string      `json:"state"`
		FailureReason string      `json:"failureReason"`
		Details       []struct {
			SnapshotName string   `json:"snapshotName"`
			State        string   `json:"state"`
			Failures     []string `json:"failures"`
		} `json:"details"`
	}
	if err := json.Unmarshal(body, &info); err != nil {
		return BackupStatus{}, fmt.Errorf("decoding history backup status: %w", err)
	}

	result := BackupStatus{ID: id, State: State(info.State), FailureReason: info.FailureReason}
	for _, d := range info.Details {
		result.Details = append(result.Details, Detail{
			Name:   d.SnapshotName,
			State:  d.State,
			Reason: strings.Join(d.Failures, "; "),
		})
	}

	return result, nil
}

// StartRuntimeBackup starts the backup of the Zeebe partitions and returns
// the id the backup is keyed by.
//
// Which id that is depends on who chose it. With an explicit id, that id is
// authoritative: the partition backups and every later status query key on
// it, and the cluster echoes an id of its own that the caller must ignore.
// With a nil id the cluster generates one and the returned id is the only
// one there is; that is the RDBMS path, where continuous backups require the
// parameter to be omitted.
//
// An id that the cluster already holds, or that is lower than one it holds,
// returns ErrConflict. The client does not resolve it, because a conflict on
// an id the caller did not choose is indistinguishable here from re-entry on
// its own; see ErrConflict.
func (c *Client) StartRuntimeBackup(ctx context.Context, id *int64) (int64, error) {
	var request []byte
	if id != nil {
		var err error
		request, err = json.Marshal(map[string]int64{"backupId": *id})
		if err != nil {
			return 0, fmt.Errorf("encoding request: %w", err)
		}
	}

	body, status, err := c.api.Do(ctx, adminhttp.Request{
		Method: http.MethodPost,
		Path:   "/actuator/backupRuntime",
		Body:   request,
		Accept: adminhttp.Status(http.StatusAccepted),
	})
	if err != nil {
		if status == http.StatusConflict {
			return 0, fmt.Errorf("%w: %v", ErrConflict, err)
		}

		return 0, err
	}

	if id != nil {
		return *id, nil
	}

	var response struct {
		BackupID int64 `json:"backupId"`
	}
	if err := json.Unmarshal(body, &response); err != nil {
		return 0, fmt.Errorf("decoding take backup response: %w", err)
	}

	return response.BackupID, nil
}

// RuntimeBackupStatus reports the status of the runtime backup id. A backup
// that does not exist is StateDoesNotExist, not an error.
func (c *Client) RuntimeBackupStatus(ctx context.Context, id int64) (BackupStatus, error) {
	body, status, err := c.api.Do(ctx, adminhttp.Request{
		Method: http.MethodGet,
		Path:   "/actuator/backupRuntime/" + strconv.FormatInt(id, 10),
		Accept: adminhttp.Status(http.StatusOK),
	})
	if status == http.StatusNotFound {
		return BackupStatus{ID: id, State: StateDoesNotExist}, nil
	}
	if err != nil {
		return BackupStatus{}, err
	}

	var info struct {
		BackupID      int64  `json:"backupId"`
		State         string `json:"state"`
		FailureReason string `json:"failureReason"`
		Details       []struct {
			PartitionID   int32  `json:"partitionId"`
			State         string `json:"state"`
			FailureReason string `json:"failureReason"`
		} `json:"details"`
	}
	if err := json.Unmarshal(body, &info); err != nil {
		return BackupStatus{}, fmt.Errorf("decoding runtime backup status: %w", err)
	}

	result := BackupStatus{ID: id, State: State(info.State), FailureReason: info.FailureReason}
	for _, d := range info.Details {
		result.Details = append(result.Details, Detail{
			Name:   strconv.FormatInt(int64(d.PartitionID), 10),
			State:  d.State,
			Reason: d.FailureReason,
		})
	}

	return result, nil
}

// DeleteRuntimeBackup deletes the runtime backup id from the backup store. A
// backup that does not exist is success, so a re-entrant finalizer can call
// it again.
func (c *Client) DeleteRuntimeBackup(ctx context.Context, id int64) error {
	_, status, err := c.api.Do(ctx, adminhttp.Request{
		Method: http.MethodDelete,
		Path:   "/actuator/backupRuntime/" + strconv.FormatInt(id, 10),
		Accept: adminhttp.Status(http.StatusNoContent),
	})
	if status == http.StatusNotFound {
		return nil
	}

	return err
}
