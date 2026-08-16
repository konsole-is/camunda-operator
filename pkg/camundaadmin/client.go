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
// Every method is idempotent from the caller's view: the backup state
// machine re-enters after a crash, so "already done" is success and never an
// error. Errors distinguish an unreachable endpoint (ErrUnreachable) from a
// rejected call (ErrRejected, with the response body in the message).
package camundaadmin

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// ErrUnreachable and ErrRejected classify every failure of the client.
// ErrUnreachable means the endpoint did not answer (connection, timeout);
// the caller maps it to ConnectionFailed and retries. ErrRejected means the
// endpoint answered with an error; the message carries the response body.
var (
	ErrUnreachable = errors.New("management API unreachable")
	ErrRejected    = errors.New("management API rejected the call")
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
type Auth struct {
	// Username and Password authenticate with HTTP basic auth when both are
	// set.
	Username string
	Password string
}

// Binding locates and authenticates the management API of one cluster. It
// mirrors the management binding that a CamundaCluster publishes in
// status.management.
type Binding struct {
	// Endpoint is the base URL of the management port, for example
	// http://my-cluster-camunda-management.ns.svc:9600.
	Endpoint string
	// Version is the Camunda version of the cluster, for example 8.9.9. Its
	// minor selects the endpoint set.
	Version string
	// Auth authenticates the calls.
	Auth Auth
}

// Client calls the management API of one cluster.
type Client struct {
	base string
	auth Auth
	http *http.Client
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

	return &Client{
		base: strings.TrimRight(binding.Endpoint, "/"),
		auth: binding.Auth,
		http: &http.Client{Timeout: 30 * time.Second},
	}, nil
}

// PauseExporting pauses exporting on every partition. With soft, records
// keep exporting but log compaction stops, which makes a hot backup
// possible. Pausing an already paused cluster is success.
func (c *Client) PauseExporting(ctx context.Context, soft bool) error {
	path := "/actuator/exporting/pause"
	if soft {
		path += "?soft=true"
	}

	_, _, err := c.do(ctx, http.MethodPost, path, nil, http.StatusNoContent)
	return err
}

// ResumeExporting resumes exporting on every partition. Resuming a cluster
// that already exports is success.
func (c *Client) ResumeExporting(ctx context.Context) error {
	_, _, err := c.do(ctx, http.MethodPost, "/actuator/exporting/resume", nil, http.StatusNoContent)
	return err
}

// StartHistoryBackup starts the backup of the web-application indices under
// id. A backup that already exists under the same id is success, so a
// re-entrant caller never double-starts.
func (c *Client) StartHistoryBackup(ctx context.Context, id int64) error {
	body, err := json.Marshal(map[string]int64{"backupId": id})
	if err != nil {
		return fmt.Errorf("encoding request: %w", err)
	}

	_, status, err := c.do(ctx, http.MethodPost, "/actuator/backupHistory", body, http.StatusAccepted)
	if err == nil {
		return nil
	}

	// The endpoint rejects a repeated id. Re-entry is not a failure: when
	// the backup exists, the start already happened.
	if status == http.StatusBadRequest || status == http.StatusConflict {
		if existing, statusErr := c.HistoryBackupStatus(
			ctx,
			id,
		); statusErr == nil &&
			existing.State != StateDoesNotExist {
			return nil
		}
	}

	return err
}

// HistoryBackupStatus reports the status of the history backup id. A backup
// that does not exist is StateDoesNotExist, not an error.
func (c *Client) HistoryBackupStatus(ctx context.Context, id int64) (BackupStatus, error) {
	body, status, err := c.do(
		ctx, http.MethodGet, "/actuator/backupHistory/"+strconv.FormatInt(id, 10), nil, http.StatusOK,
	)
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
// the backup ID. With a nil id the cluster generates one, which is the RDBMS
// path with continuous backups enabled. With an explicit id, a backup that
// already exists under the same id is success, so a re-entrant caller never
// double-starts.
func (c *Client) StartRuntimeBackup(ctx context.Context, id *int64) (int64, error) {
	var request []byte
	if id != nil {
		var err error
		request, err = json.Marshal(map[string]int64{"backupId": *id})
		if err != nil {
			return 0, fmt.Errorf("encoding request: %w", err)
		}
	}

	body, status, err := c.do(ctx, http.MethodPost, "/actuator/backupRuntime", request, http.StatusAccepted)
	if err != nil {
		// The endpoint answers 409 when a backup with the same or a higher
		// id exists. Re-entry with the same id is not a failure.
		if id != nil && status == http.StatusConflict {
			if existing, statusErr := c.RuntimeBackupStatus(
				ctx,
				*id,
			); statusErr == nil &&
				existing.State != StateDoesNotExist {
				return *id, nil
			}
		}
		return 0, err
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
	body, status, err := c.do(
		ctx, http.MethodGet, "/actuator/backupRuntime/"+strconv.FormatInt(id, 10), nil, http.StatusOK,
	)
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
	_, status, err := c.do(
		ctx, http.MethodDelete, "/actuator/backupRuntime/"+strconv.FormatInt(id, 10), nil, http.StatusNoContent,
	)
	if status == http.StatusNotFound {
		return nil
	}

	return err
}

// do sends one request and returns the body and status code. A transport
// error wraps ErrUnreachable. A status other than want wraps ErrRejected
// with the response body in the message; the caller reads the returned
// status for the branches that are not failures.
func (c *Client) do(
	ctx context.Context,
	method string,
	path string,
	body []byte,
	want int,
) ([]byte, int, error) {
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
	if c.auth.Username != "" || c.auth.Password != "" {
		req.SetBasicAuth(c.auth.Username, c.auth.Password)
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, 0, fmt.Errorf("%w: %s %s: %v", ErrUnreachable, method, path, err)
	}
	defer func() { _ = resp.Body.Close() }()

	payload, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return nil, resp.StatusCode, fmt.Errorf("%w: reading response of %s %s: %v", ErrUnreachable, method, path, err)
	}

	if resp.StatusCode != want {
		return payload, resp.StatusCode, fmt.Errorf(
			"%w: %s %s returned %d: %s",
			ErrRejected, method, path, resp.StatusCode, strings.TrimSpace(string(payload)),
		)
	}

	return payload, resp.StatusCode, nil
}
