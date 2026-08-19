# Restore Controllers Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use feature-dev-workflow:developing-a-feature to drive this plan PR by PR on the feature branch, dispatching per-PR workers via feature-dev-workflow:fanning-out-with-worktrees where the graph fans out. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Ship the restore epic (#109): the `LogicalRestore` and `PointInTimeRestore` APIs and controllers, plus an e2e round trip that proves a backup restores.

**Architecture:** Four sub-PRs on the long-lived `feat/restore-controllers` branch, each a real GitHub PR that targets the feature branch, then one integration PR to `main`. PR1 ships both API types and one shared package, `pkg/restore`. PR2 and PR3 consume it in parallel, one controller each. PR4 proves the whole path in kind. The restore Jobs copy their configuration from the live broker StatefulSet, so the restore application always runs with the configuration the brokers run with.

**Tech Stack:** Go, kubebuilder/controller-runtime, ocf v0.19.1, pgx v5 (through `pkg/pgbootstrap`), `pkg/esadmin` over `pkg/adminhttp`, Ginkgo/Gomega (controller tests through `internal/testenv`), testify (unit tests), kind + MinIO + ECK + PostgreSQL (e2e).

**Spec:** `docs/superpowers/specs/2026-08-20-restore-controllers-design.md` — the design authority. The detailed design record is `docs/crds/logicalrestore.md` and `docs/crds/pointintimerestore.md`. Read the matching section before you start a task.

**Tracking:** Epic #109. Sub-issues: PR1 #110, PR2 #111, PR3 #112, PR4 #113. Orchestration state: `docs/superpowers/states/2026-08-20-restore-controllers-state.md`.

## Global Constraints

- Camunda 8.9 only. Verify every configuration key and every Camunda behaviour with the `camunda-docs` MCP server or the Camunda source, per `verifying-camunda-app-config`. Never answer from memory.
- CLAUDE.md rules hold: server-side apply for every managed resource, one status write per reconcile through the ocf `FlushStatus`, Ginkgo at controller level and testify next to the file, no `t.Fatal`, GoDoc on every exported symbol, docs updated in the same PR as the code.
- Load `how-we-write-go` before you write Go. Load `simple-english:simple-english` before you write prose. Load the `ocf:*` skills per the CLAUDE.md table.
- `go test ./...`, `make all`, and `make manifests generate` with no diff must pass at every PR head.
- The restore controllers only read `CamundaCluster.spec.suspend`. They never write it. Suspend orchestration belongs to `camunda-cloud-operator`.
- The API does not expose Camunda's `--allow-version-mismatch`. The operator never restores a database server.
- Commits reference the sub-issue, for example `feat(logicalrestore): validate the target before it deletes a volume (#111)`. Sub-PR bodies say `Towards #<sub-issue>`. The integration PR says `Closes #109`.
- Never push to `main`. Everything lands through PRs.

## Merge policy

Every PR runs `feature-dev-workflow:copilot-review-loop` until the loop returns nothing new. PR1 (#110) and PR4 (#113) are then self-merged into the feature branch. **PR2 (#111) and PR3 (#112) stop after the loop is clean and wait for the user's own review. Do not merge them without it.** The integration PR also waits for the user.

## PR graph

```
PR1 #110 API + pkg/restore ──┬── PR2 #111 LogicalRestore ──┐
                             └── PR3 #112 PointInTimeRestore ─┴── PR4 #113 e2e ── integration PR (user review)
```

PR2 and PR3 fan out in parallel after PR1 merges. PR4 starts after both merge.

## Contracts

| Name | Producer issue | Consumer issues | Shape | Realization |
| --- | --- | --- | --- | --- |
| `restore-api-types` | #110 | #111, #112, #113 | `v1.LogicalRestoreSpec{BackupRef LogicalBackupRef, TargetClusterRef ClusterRef}`, `v1.LogicalBackupRef{Kind LogicalBackupKind, Name string}`, `v1.LogicalRestorePhase` (`Pending`, `ValidatingCompatibility`, `RestoringSecondaryStorage`, `RestoringPrimaryStorage`, `Completed`, `Failed`), `v1.LogicalRestoreStatus` (fields in PR1 Task 1), `v1.PointInTimeRestoreSpec{ClusterRef ClusterRef, Timestamp metav1.Time}`, `v1.PointInTimeRestorePhase` (`Pending`, `ValidatingDatabaseState`, `RestoringPrimaryStorage`, `Completed`, `Failed`), `v1.PointInTimeRestoreStatus` and `v1.PartitionPosition{PartitionID int32, LastUpdated metav1.Time}`. Both kinds implement `GetStatusConditions`, `GetKind`, `SetObservedGeneration`, `Terminal`. New reasons: `v1.ReasonClusterNotSuspended` (shared, `api/v1/restore_shared.go`), `v1.ReasonIncompatibleTarget` (LogicalRestore), `v1.ReasonPitrUnavailable`, `v1.ReasonSharedServer`, `v1.ReasonDatabaseNotRestored` (PointInTimeRestore) | merged code (PR1 lands first) |
| `restore-shared-package` | #110 | #111, #112, #113 | `pkg/restore`: `const FieldManagerLogicalRestore client.FieldOwner = "camunda-operator/logicalrestore"`, `const FieldManagerPointInTimeRestore client.FieldOwner = "camunda-operator/pointintimerestore"`, `const RestoreEntrypoint = "/usr/local/camunda/bin/restore"`, `const ComponentRestore = "restore"`; `type Target struct{ StatefulSet *appsv1.StatefulSet; Broker *corev1.Container; Brokers, Partitions int32; Version string; ClaimTemplate *corev1.PersistentVolumeClaim }`; `func ReadTarget(ctx context.Context, reader client.Reader, cluster *v1.CamundaCluster) (*Target, error)`; `func (t *Target) ClaimNames() []string`; `func (t *Target) ClaimSize(recorded *resource.Quantity) resource.Quantity`; `func (t *Target) BuildClaim(ordinal int32, size resource.Quantity) *corev1.PersistentVolumeClaim`; `type Progress struct{ Done bool; Message string; Recreated []string }`; `type ClaimInput struct{ Target *Target; Size resource.Quantity; Recreated []string; FieldManager client.FieldOwner }`; `func RecreateClaims(ctx context.Context, c client.Client, reader client.Reader, in ClaimInput) (Progress, error)`; `type JobInput struct{ Target *Target; Owner client.Object; OwnerLabel labels.Owner; Ordinal int32; Args []string }`; `func JobName(owner client.Object, ordinal int32) string`; `func BuildJob(in JobInput) (*batchv1.Job, error)`; `func Apply(ctx context.Context, c client.Client, obj client.Object, manager client.FieldOwner) error` | merged code (PR1 lands first) |
| `pod-stuck-helper` | #110 | #111, #112 | `pkg/podstate`: `func Stuck(ctx context.Context, reader client.Reader, namespace string, selector map[string]string, what string) (*conditions.PreCheckFailure, error)` — the first pod under the selector that cannot start, reported as a `MissingSecret`, `InvalidReference`, or `Progressing` failure that names the pod, the container, and the waiting reason. Lifted out of `internal/controller/logicalbackuprdbms`, which then consumes it | merged code (PR1 lands first) |
| `backup-version-field` | #110 | #111 | `v1.LogicalBackupElasticsearchStatus.Version string` and `v1.LogicalBackupRDBMSStatus.Version string`, written at admission from `cluster.status.management.version`. It is the only place a restore can read the Camunda version a backup was taken with, because `status.management` is nil while a cluster is suspended | merged code (PR1 lands first) |
| `backup-artifact-naming` | #110 | #111, #113 | `pkg/logicalbackup`: `func RecordsSnapshotName(id int64) string` (moved out of `internal/controller/logicalbackupelasticsearch`), `const ZeebeRecordIndices = "zeebe-record*"`, `func CamundaIndexPatterns(withOptimize bool) []string`, `func HasOptimizeSnapshot(names []string) bool` | merged code (PR1 lands first) |
| `pg-open-seam` | #110 | #112 | `pkg/pgbootstrap`: `func Open(ctx context.Context, c Connection, database string) (*pgx.Conn, error)` — the one place that builds a PostgreSQL DSN, now reachable by a caller that runs its own SQL inside a logical database. `Connection.AdminUser` and `Connection.AdminPassword` are renamed to `User` and `Password`, because the type now carries whatever role the caller holds | merged code (PR1 lands first) |
| `esadmin-restore-api` | #111 | #113 | `pkg/esadmin`: `func (c *Client) DeleteIndices(ctx context.Context, patterns []string) error`, `func (c *Client) RestoreSnapshot(ctx context.Context, repo, name string, indices []string) error`, `type RestoreState string` with `RestoreInProgress`/`RestoreDone`, `func (c *Client) RestoreProgress(ctx context.Context, patterns []string) (RestoreState, error)`. `esadmintest` gains the routes and the operation names `"indexDelete"`, `"snapshotRestore"`, `"recovery"` | merged in PR2, consumed by PR4 only through the controller |

`restore-shared-package` and `restore-api-types` are the main contract. PR1 lands them complete, table-tested, with golden Job snapshots. PR2 and PR3 then import merged code, so neither has to stub the other's surface.

## Conventions

- **Naming firewall:** PR numbers and "PR N" labels never appear in code, fixtures, or test names.
- **Kinds:** `LogicalRestore` and `PointInTimeRestore` everywhere. Never `Restore`, `PITR`, or `PitrRestore`. The Go identifier for point-in-time restore is `PointInTimeRestore`, and the abbreviation `PITR` appears only inside prose and inside the existing `v1.PITRCapability` type.
- **Shared package:** `pkg/restore`, package name `restore`. It is pure where it can be: `BuildJob`, `BuildClaim`, `JobName`, `ClaimNames`, and `ClaimSize` take values and return values. `ReadTarget`, `RecreateClaims`, and `Apply` take a client because they must talk to the API server. The package never reads a restore CR's spec, so both controllers use the same functions.
- **Controller packages:** `internal/controller/logicalrestore` and `internal/controller/pointintimerestore`, one directory per CRD, matching the backup layout. The flat scaffold files `internal/controller/logicalrestore_controller.go`, `internal/controller/pointintimerestore_controller.go`, and their two test files are deleted in PR1.
- **Reconciler shape:** `New(c client.Client, reader client.Reader, scheme *runtime.Scheme, options Options) *Reconciler` plus `SetupWithManager(mgr ctrl.Manager) error`, matching `internal/controller/logicalbackupelasticsearch`. Not the struct-literal plus options-argument shape of `logicalbackuprdbms`.
- **Phases:** the phase is the resume marker. Neither kind gets a separate `step` field. A phase is persisted before the side effect it names.
- **Condition reasons:** reuse `api/v1/conditions.go` and `api/v1/logicalbackup_shared.go` wherever a reason already exists — `ReasonProgressing`, `ReasonCompleted`, `ReasonFailed`, `ReasonInvalidReference`, `ReasonMissingSecret`, `ReasonConnectionFailed`. Declare `ReasonClusterNotSuspended` once, in a new `api/v1/restore_shared.go`, because both restore kinds report it. Declare a reason that one kind alone reports next to that kind's types, the way `ReasonResumeFailed` and `ReasonMissingCredentials` are declared today.
- **SSA field managers:** `camunda-operator/logicalrestore` and `camunda-operator/pointintimerestore`, as `client.FieldOwner` constants in `pkg/restore`. Every Job and every recreated PVC is applied with `client.Apply`, the calling controller's field manager, and `client.ForceOwnership`. Status is never written with SSA. It goes through `component.FlushStatus`.
- **PVC naming, sizing, ownership:** the name is what the StatefulSet expects, `data-<cluster>-zeebe-<ordinal>`, built from `components.DataVolumeName`, `components.WorkloadName(cluster, components.ComponentZeebe)`, and the ordinal. The count is `CAMUNDA_CLUSTER_SIZE` read from the live broker container, not `spec.replicas`, because a suspended StatefulSet runs at zero replicas. The size is the backup's recorded `status.storageSizes.zeebe` when the backup recorded one, and the StatefulSet's own claim template request when it did not. The storage class, the access modes, and the claim labels come from the claim template. **The recreated PVC carries no owner reference.** The StatefulSet owns those claims, and an owner reference to the restore CR deletes a live broker volume as soon as the restore CR is deleted.
- **Job labels:** every restore Job and its pods carry `labels.Managed(owner, restore.ComponentRestore)` — that is `camunda.io/logical-restore` or `camunda.io/point-in-time-restore` with the CR name, `camunda.io/component: restore`, and `app.kubernetes.io/managed-by: camunda-operator` — merged with `labels.ClusterKey: <target cluster name>` and with the broker pod labels that `ReadTarget` copied. The operator labels win over the copied ones. `pkg/labels` gains `LogicalRestoreKey = "camunda.io/logical-restore"`, `PointInTimeRestoreKey = "camunda.io/point-in-time-restore"`, `func LogicalRestore(name string) Owner`, and `func PointInTimeRestore(name string) Owner`. No package declares a label string of its own.
- **Job ownership:** every restore Job gets a controller reference to its restore CR, so deleting the CR removes the Jobs. Neither controller needs a finalizer: a restore writes no artifact to an external store.
- **Test layout:** Ginkgo plus envtest at controller level, one `suite_test.go` per controller directory booting `internal/testenv`. Pure Go tests with testify next to the file that holds the feature. Rendered Kubernetes objects get golden snapshots under `pkg/restore/testdata/golden/<case>.yaml`, matching `pkg/components/logicalbackuprdbms`. CRD schema tests live in the controller directory as `schema_test.go`, matching the backup kinds.
- **Requeue cadence:** `defaultPollInterval = 5 * time.Second` paces a running phase. `defaultRetryInterval = 30 * time.Second` paces a hold that no watch resolves. `defaultMidRunGrace = 10 * time.Minute` bounds how long a started restore waits on a dependency that stopped resolving before the restore fails. `shortly = time.Second` re-enters after a staged status is persisted. Every value is an `Options` field so a test can shrink it, exactly like the backup controllers.
- **Docs:** PR1 owns both `docs/crds/` pages, because PR1 owns the schema. PR2 and PR3 correct their own page when the code finds a better shape. The pages lose their "Not implemented yet" warning in the PR that makes the kind real.
- **Commit messages:** `feat(logicalrestore): ... (#111)`, `feat(pointintimerestore): ... (#112)`, `feat(api): ... (#110)`, `test(e2e): ... (#113)`. One issue reference per commit.

## Facts this plan is built on

Read these before you argue with a task. Each was verified in the worktree.

- `CamundaCluster.status.management` is **nil while the cluster is suspended** (`internal/controller/camundacluster/binding.go`). A restore runs against a suspended cluster, so neither controller can read the partition count or the Camunda version from the management binding. Both read them from the live broker StatefulSet instead.
- The broker StatefulSet is `<cluster>-zeebe` (`components.WorkloadName(cluster, components.ComponentZeebe)`). Its only claim template is `data` (`components.DataVolumeName`). The generated PVC name is `data-<cluster>-zeebe-<ordinal>`.
- The broker container is named `camunda`. Its command is `["bash", "-c", "export CAMUNDA_CLUSTER_NODEID=${HOSTNAME##*-}; exec /usr/local/camunda/bin/camunda"]` (`pkg/components/camundacluster/render.go`). A Job pod has a random host name, so a restore Job sets `CAMUNDA_CLUSTER_NODEID` to the ordinal as a plain environment entry and runs the restore entrypoint directly.
- The broker container already carries `CAMUNDA_CLUSTER_SIZE`, `CAMUNDA_CLUSTER_PARTITIONCOUNT`, `CAMUNDA_CLUSTER_REPLICATIONFACTOR`, the whole secondary-storage block, and the primary-storage backup block. Copying that environment is what makes the restore application read the same database with the same credentials the brokers use, which is exactly what `docs/crds/pointintimerestore.md` promises.
- The Elasticsearch snapshot repository is registered by the `ElasticsearchCluster` controller under `base_path = logicalbackup.ClusterPrefix(bucketBasePath, elasticsearchClusterNamespace, elasticsearchClusterName)`, and its name is the `ElasticsearchCluster` name (`pkg/components/elasticsearchcluster`). A backup records that name in `status.repository` and pins its bucket in `status.storage.BucketRef`.
- `pkg/esadmin` has no index deletion and no snapshot restore today. PR2 adds both.
- `pkg/pgbootstrap` builds every DSN in one unexported `dial` function and exposes no way to run caller SQL inside a logical database. PR1 adds `Open`.
- The e2e suite already seeds real data: `itRunsTheOrchestrationCluster` deploys `testdata/process.bpmn`, starts an instance, and proves it through `expectInstanceSearchable(cluster)`, which reads through secondary storage. The e2e suite talks to services from in-cluster helper pods through `utils.RunPod`. There is no port forwarding anywhere, and PR4 adds none.

---

## PR1 — API types and the shared restore package (#110, branch `feat/restore-api-and-machinery`, worktree `.claude/worktrees/restore-controllers--api`)

Implements the API sections of both CRD design docs, the spec's "restore Jobs mirror the live broker StatefulSet" decision, and the new `ValidatingDatabaseState` phase in the PointInTimeRestore documentation.

**Files:**
- Rewrite: `api/v1/logicalrestore_types.go`, `api/v1/pointintimerestore_types.go`
- Create: `api/v1/restore_shared.go`
- Modify: `api/v1/logicalbackupelasticsearch_types.go`, `api/v1/logicalbackuprdbms_types.go` (record `status.version`), the two backup controllers' admission steps and tests, and `docs/crds/logicalbackupelasticsearch.md`, `docs/crds/logicalbackuprdbms.md`
- Modify: `api/v1/zz_generated.deepcopy.go` (generated), `config/crd/bases/*logicalrestores.yaml`, `config/crd/bases/*pointintimerestores.yaml`, `config/rbac/role.yaml` (all generated)
- Create: `pkg/restore/{doc.go,target.go,target_test.go,claims.go,claims_test.go,job.go,job_test.go,apply.go}` and `pkg/restore/testdata/golden/`
- Modify: `pkg/logicalbackup/keys.go`, `pkg/logicalbackup/keys_test.go`
- Modify: `internal/controller/logicalbackupelasticsearch/statemachine.go` (use the moved `RecordsSnapshotName`), and its tests
- Modify: `pkg/pgbootstrap/pgbootstrap.go`, `pkg/pgbootstrap/pgbootstrap_test.go`, `internal/controller/database/controller.go`, `internal/controller/databaseserverconfig/controller.go`
- Modify: `pkg/labels/labels.go`, `pkg/labels/labels_test.go`
- Create: `pkg/podstate/{podstate.go,podstate_test.go}`; modify `internal/controller/logicalbackuprdbms/dump.go` and its tests to consume it
- Delete: `internal/controller/logicalrestore_controller.go`, `internal/controller/logicalrestore_controller_test.go`, `internal/controller/pointintimerestore_controller.go`, `internal/controller/pointintimerestore_controller_test.go`
- Modify: `cmd/main.go` (drop the two stub registrations, leave the scaffold markers in place)
- Modify: `config/samples/core_v1_logicalrestore.yaml`, `config/samples/core_v1_pointintimerestore.yaml`
- Rewrite: `docs/crds/logicalrestore.md`, `docs/crds/pointintimerestore.md`

### Task 1: The two API types

**Produces:** everything in the `restore-api-types` contract row.

- [ ] **Step 1: Write the failing schema test**

Create `api/v1/restore_types_test.go` with pure Go tests that do not need an API server, covering the phase enums and the helper methods.

```go
func TestLogicalRestoreTerminal(t *testing.T) {
	for _, tc := range []struct {
		phase LogicalRestorePhase
		want  bool
	}{
		{LogicalRestorePending, false},
		{LogicalRestoreValidatingCompatibility, false},
		{LogicalRestoreRestoringSecondaryStorage, false},
		{LogicalRestoreRestoringPrimaryStorage, false},
		{LogicalRestoreCompleted, true},
		{LogicalRestoreFailed, true},
	} {
		restore := &LogicalRestore{Status: LogicalRestoreStatus{Phase: tc.phase}}
		assert.Equal(t, tc.want, restore.Terminal(), "phase %s", tc.phase)
	}
}

func TestPointInTimeRestoreTerminal(t *testing.T) {
	for _, tc := range []struct {
		phase PointInTimeRestorePhase
		want  bool
	}{
		{PointInTimeRestorePending, false},
		{PointInTimeRestoreValidatingDatabaseState, false},
		{PointInTimeRestoreRestoringPrimaryStorage, false},
		{PointInTimeRestoreCompleted, true},
		{PointInTimeRestoreFailed, true},
	} {
		restore := &PointInTimeRestore{Status: PointInTimeRestoreStatus{Phase: tc.phase}}
		assert.Equal(t, tc.want, restore.Terminal(), "phase %s", tc.phase)
	}
}

func TestRestoreKindsReportTheirKind(t *testing.T) {
	assert.Equal(t, "LogicalRestore", (&LogicalRestore{}).GetKind())
	assert.Equal(t, "PointInTimeRestore", (&PointInTimeRestore{}).GetKind())
}
```

- [ ] **Step 2: Run the test and confirm it fails**

Run: `go test ./api/... -run 'TestLogicalRestoreTerminal|TestPointInTimeRestoreTerminal|TestRestoreKindsReportTheirKind' -v`
Expected: FAIL, compile error, the identifiers do not exist.

- [ ] **Step 3: Write `api/v1/restore_shared.go`**

```go
package v1

// The condition vocabulary that both restore kinds report. A reason that only
// one restore kind reports is declared next to that kind, in its types file.
const (
	// ReasonClusterNotSuspended means that the target cluster still runs. A
	// restore rewrites primary storage, so it waits in Pending until the
	// owner of the cluster suspends it. The restore controllers never write
	// spec.suspend.
	ReasonClusterNotSuspended = "ClusterNotSuspended"
)
```

- [ ] **Step 4: Rewrite `api/v1/logicalrestore_types.go`**

Drop the scaffold. Write, with GoDoc on every exported symbol in the house prose style:

```go
// ReasonIncompatibleTarget means that the target cluster cannot hold the
// backup: the secondary storage types differ, the partition counts differ,
// the backup bucket differs, or the Camunda versions break the version rule.
// Only a LogicalRestore reports it.
const ReasonIncompatibleTarget = "IncompatibleTarget"

// LogicalBackupKind names one of the two logical backup kinds.
// +kubebuilder:validation:Enum=LogicalBackupElasticsearch;LogicalBackupRDBMS
type LogicalBackupKind string

const (
	LogicalBackupKindElasticsearch LogicalBackupKind = "LogicalBackupElasticsearch"
	LogicalBackupKindRDBMS         LogicalBackupKind = "LogicalBackupRDBMS"
)

// LogicalBackupRef references a completed logical backup in the namespace of
// the restore. The reference never crosses a namespace.
type LogicalBackupRef struct {
	// Kind of the backup.
	// +required
	Kind LogicalBackupKind `json:"kind"`
	// Name of the backup, in the namespace of this restore.
	// +required
	// +kubebuilder:validation:MinLength=1
	Name string `json:"name"`
}

// LogicalRestorePhase tracks the one-shot restore. Completed and Failed are
// terminal. A retry is a new resource.
// +kubebuilder:validation:Enum=Pending;ValidatingCompatibility;RestoringSecondaryStorage;RestoringPrimaryStorage;Completed;Failed
type LogicalRestorePhase string

const (
	LogicalRestorePending                   LogicalRestorePhase = "Pending"
	LogicalRestoreValidatingCompatibility   LogicalRestorePhase = "ValidatingCompatibility"
	LogicalRestoreRestoringSecondaryStorage LogicalRestorePhase = "RestoringSecondaryStorage"
	LogicalRestoreRestoringPrimaryStorage   LogicalRestorePhase = "RestoringPrimaryStorage"
	LogicalRestoreCompleted                 LogicalRestorePhase = "Completed"
	LogicalRestoreFailed                    LogicalRestorePhase = "Failed"
)

// LogicalRestoreSpec names the backup to restore and the cluster to restore
// into. The whole spec is immutable: a restore is one operation, retried by
// creating a new resource.
type LogicalRestoreSpec struct {
	// BackupRef references the completed backup to restore from.
	// +required
	BackupRef LogicalBackupRef `json:"backupRef"`
	// TargetClusterRef references the CamundaCluster to restore into. It can
	// differ from the cluster the backup was taken from. The cluster must be
	// suspended for the whole restore.
	// +required
	TargetClusterRef ClusterRef `json:"targetClusterRef"`
}

// LogicalRestoreStatus tracks the restore to a terminal phase.
type LogicalRestoreStatus struct {
	// Phase of the restore. It is the resume marker: a reconcile that
	// re-enters after a crash continues at the recorded phase.
	// +optional
	Phase LogicalRestorePhase `json:"phase,omitempty"`
	// BackupID is the backup id that the restore reads, pinned when the
	// restore starts. The backup can be deleted afterwards without moving the
	// restore to another set of artifacts.
	// +optional
	BackupID int64 `json:"backupId,omitempty"`
	// StorageType is the secondary storage type of the backup, pinned with
	// the backup id. It decides which restore procedure runs.
	// +optional
	StorageType SecondaryStorageType `json:"storageType,omitempty"`
	// TargetClusterUID pins the identity of the target cluster. A cluster
	// that is deleted and created again under the same name is another
	// cluster, and this restore is not its restore.
	// +optional
	TargetClusterUID types.UID `json:"targetClusterUID,omitempty"`
	// Brokers is the broker count read from the live broker StatefulSet when
	// the restore entered RestoringPrimaryStorage. It fixes how many volumes
	// are recreated and how many Jobs run.
	// +optional
	Brokers int32 `json:"brokers,omitempty"`
	// Repository is the Elasticsearch snapshot repository the restore reads
	// from, registered on the target's Elasticsearch. It is unset on the
	// relational path.
	// +optional
	Repository string `json:"repository,omitempty"`
	// RestoredSnapshots names every snapshot the restore asked Elasticsearch
	// to restore. It is unset on the relational path.
	// +optional
	RestoredSnapshots []string `json:"restoredSnapshots,omitempty"`
	// SecondaryJobName is the Job that runs pg_restore on the relational
	// path, while it exists. It is unset on the Elasticsearch path.
	// +optional
	SecondaryJobName string `json:"secondaryJobName,omitempty"`
	// PrimaryJobNames are the per-broker restore-application Jobs, in broker
	// order.
	// +optional
	PrimaryJobNames []string `json:"primaryJobNames,omitempty"`
	// RecreatedClaims names the broker data claims that the restore deleted
	// and created again. A reconcile that re-enters does not delete a claim
	// twice.
	// +optional
	RecreatedClaims []string `json:"recreatedClaims,omitempty"`
	// FirstFailedAt is when a dependency of the running restore first stopped
	// resolving. The operator measures the mid-run grace from it, and clears
	// it when the restore recovers.
	// +optional
	FirstFailedAt *metav1.Time `json:"firstFailedAt,omitempty"`
	// FailureMessage names the failing phase and its error. The Ready
	// condition carries the same message, and the operator stages the
	// condition again from this field, so a write conflict cannot lose it.
	// +optional
	FailureMessage string `json:"failureMessage,omitempty"`
	// CompletionTime is when the restore reached a terminal phase.
	// +optional
	CompletionTime *metav1.Time `json:"completionTime,omitempty"`
	// ObservedGeneration is the last generation reconciled by the operator.
	// +optional
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`
	// Conditions represent the current state. The Ready condition carries the
	// reasons Progressing, Completed, Failed, ClusterNotSuspended,
	// InvalidReference, IncompatibleTarget, MissingSecret, and
	// ConnectionFailed.
	// +listType=map
	// +listMapKey=type
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}
```

Markers on the root type:

```go
// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:path=logicalrestores,shortName=lr
// +kubebuilder:printcolumn:name="Phase",type=string,JSONPath=`.status.phase`
// +kubebuilder:printcolumn:name="Backup",type=string,JSONPath=`.spec.backupRef.name`
// +kubebuilder:printcolumn:name="Target",type=string,JSONPath=`.spec.targetClusterRef.name`
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`
```

The `Spec` field carries the immutability rule, exactly as `LogicalBackupElasticsearch` does:

```go
	// +required
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="spec is immutable: a restore is one-shot, retried by creating a new resource"
	Spec LogicalRestoreSpec `json:"spec"`
```

Add `GetStatusConditions`, `GetKind`, `SetObservedGeneration`, and `Terminal` with the same bodies the backup kinds use.

- [ ] **Step 5: Rewrite `api/v1/pointintimerestore_types.go`**

Same shape. The reasons this kind alone reports:

```go
// ReasonPitrUnavailable means that the database server does not declare
// point-in-time recovery, or that spec.timestamp lies outside its retention
// period.
const ReasonPitrUnavailable = "PitrUnavailable"

// ReasonSharedServer means that more than one Database references the
// database server. Engine-level point-in-time recovery rolls back the whole
// server, so a shared server rolls back unrelated databases too.
const ReasonSharedServer = "SharedServer"

// ReasonDatabaseNotRestored means that the database is ahead of
// spec.timestamp, or that it reports no exporter position for a partition.
// The restore holds in Pending and touches no volume.
const ReasonDatabaseNotRestored = "DatabaseNotRestored"
```

The phase enum, the spec, and the status:

```go
// +kubebuilder:validation:Enum=Pending;ValidatingDatabaseState;RestoringPrimaryStorage;Completed;Failed
type PointInTimeRestorePhase string

const (
	PointInTimeRestorePending                 PointInTimeRestorePhase = "Pending"
	PointInTimeRestoreValidatingDatabaseState PointInTimeRestorePhase = "ValidatingDatabaseState"
	PointInTimeRestoreRestoringPrimaryStorage PointInTimeRestorePhase = "RestoringPrimaryStorage"
	PointInTimeRestoreCompleted               PointInTimeRestorePhase = "Completed"
	PointInTimeRestoreFailed                  PointInTimeRestorePhase = "Failed"
)

// PointInTimeRestoreSpec names the cluster to roll back and the point it was
// rolled back to. The whole spec is immutable.
type PointInTimeRestoreSpec struct {
	// ClusterRef references the CamundaCluster to align, in the namespace of
	// this restore. Its secondary storage must be a relational database.
	// +required
	ClusterRef ClusterRef `json:"clusterRef"`
	// Timestamp is the point the database was already restored to. It must
	// not lie in the future, and it must lie within the retention period the
	// database server declares.
	// +required
	Timestamp metav1.Time `json:"timestamp"`
}

// PartitionPosition is the exporter position of one partition, as the
// pre-check read it from the restored database.
type PartitionPosition struct {
	// PartitionID is the Zeebe partition.
	PartitionID int32 `json:"partitionId"`
	// LastUpdated is the LAST_UPDATED value of the partition's row in the
	// EXPORTER_POSITION table.
	LastUpdated metav1.Time `json:"lastUpdated"`
}

// PointInTimeRestoreStatus tracks the restore to a terminal phase.
type PointInTimeRestoreStatus struct {
	// Phase of the restore. It is the resume marker.
	// +optional
	Phase PointInTimeRestorePhase `json:"phase,omitempty"`
	// ClusterUID pins the identity of the cluster.
	// +optional
	ClusterUID types.UID `json:"clusterUID,omitempty"`
	// Brokers is the broker count read from the live broker StatefulSet.
	// +optional
	Brokers int32 `json:"brokers,omitempty"`
	// ObservedPositions are the exporter positions the pre-check read, in
	// partition order. They record what the operator saw when it let the
	// restore past the database-state check, or what held it.
	// +optional
	// +listType=map
	// +listMapKey=partitionId
	ObservedPositions []PartitionPosition `json:"observedPositions,omitempty"`
	// PrimaryJobNames are the per-broker restore-application Jobs, in broker
	// order.
	// +optional
	PrimaryJobNames []string `json:"primaryJobNames,omitempty"`
	// RecreatedClaims names the broker data claims that the restore deleted
	// and created again.
	// +optional
	RecreatedClaims []string `json:"recreatedClaims,omitempty"`
	// FirstFailedAt is when a dependency of the running restore first stopped
	// resolving.
	// +optional
	FirstFailedAt *metav1.Time `json:"firstFailedAt,omitempty"`
	// FailureMessage names the failing phase and its error.
	// +optional
	FailureMessage string `json:"failureMessage,omitempty"`
	// CompletionTime is when the restore reached a terminal phase.
	// +optional
	CompletionTime *metav1.Time `json:"completionTime,omitempty"`
	// ObservedGeneration is the last generation reconciled by the operator.
	// +optional
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`
	// Conditions represent the current state. The Ready condition carries the
	// reasons Progressing, Completed, Failed, ClusterNotSuspended,
	// InvalidReference, PitrUnavailable, SharedServer, DatabaseNotRestored,
	// MissingSecret, and ConnectionFailed.
	// +listType=map
	// +listMapKey=type
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}
```

Markers:

```go
// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:path=pointintimerestores,shortName=pitr
// +kubebuilder:printcolumn:name="Phase",type=string,JSONPath=`.status.phase`
// +kubebuilder:printcolumn:name="Cluster",type=string,JSONPath=`.spec.clusterRef.name`
// +kubebuilder:printcolumn:name="Timestamp",type=string,JSONPath=`.spec.timestamp`
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`
```

The `Spec` field carries the immutability rule and the not-in-the-future rule:

```go
	// +required
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="spec is immutable: a restore is one-shot, retried by creating a new resource"
	Spec PointInTimeRestoreSpec `json:"spec"`
```

The "not in the future" rule cannot be expressed in CEL without a clock. The controller checks it at reconcile time and reports `PitrUnavailable`. Record that in the GoDoc of `Timestamp` and on the doc page.

`spec.clusterRef` uses `v1.ClusterRef`, which has **no** namespace field. `docs/crds/pointintimerestore.md` currently shows a `namespace` key in its examples. Task 6 removes it. The same-namespace rule is the operator's established boundary: the controller reads the cluster's Secrets and runs Jobs in its namespace.

- [ ] **Step 6: Run the test and confirm it passes**

Run: `go test ./api/... -run 'TestLogicalRestoreTerminal|TestPointInTimeRestoreTerminal|TestRestoreKindsReportTheirKind' -v`
Expected: PASS.

- [ ] **Step 6b: Record the Camunda version on a backup**

A restore has to compare the Camunda version the backup was taken with against the version the target runs. Neither backup kind records one today, and `status.management` is nil while a cluster is suspended, so a restore cannot read the source version anywhere. Add one field to both backup statuses:

```go
	// Version is the Camunda version of the cluster when the backup started,
	// as the management binding reported it. A restore compares it against
	// the version of its target: an Elasticsearch backup restores only with
	// the exact same version, and a relational backup restores with the same
	// version or one minor newer.
	// +optional
	Version string `json:"version,omitempty"`
```

Write it in each backup's admission step, from `cluster.Status.Management.Version`, next to where the backup already pins its cluster UID. Extend the existing admission tests of both backup controllers to assert it. Add the field to `docs/crds/logicalbackupelasticsearch.md` and `docs/crds/logicalbackuprdbms.md`.

This lands in PR1, not in PR2, because PR1 owns the API types and PR2 and PR3 fan out in parallel over the same `api/v1` directory.

- [ ] **Step 7: Regenerate and verify**

```bash
make manifests generate
git diff --stat config/crd/bases api/v1/zz_generated.deepcopy.go
go build ./... && go test ./api/...
```

Expected: the two restore CRD bases carry the real schema, and `zz_generated.deepcopy.go` carries deepcopy functions for `LogicalBackupRef`, `PartitionPosition`, and both statuses.

- [ ] **Step 8: Commit**

```bash
git add api config
git commit -m "feat(api): give the restore kinds a real spec, status, and phases (#110)"
```

### Task 2: Label keys and the moved backup artifact names

**Produces:** the `backup-artifact-naming` contract row and the two new label owners.

- [ ] **Step 1: Write the failing tests**

Add to `pkg/labels/labels_test.go`:

```go
func TestRestoreOwners(t *testing.T) {
	assert.Equal(t, Owner{Key: LogicalRestoreKey, Name: "r"}, LogicalRestore("r"))
	assert.Equal(t, Owner{Key: PointInTimeRestoreKey, Name: "p"}, PointInTimeRestore("p"))
	assert.Equal(t, "camunda.io/logical-restore", LogicalRestoreKey)
	assert.Equal(t, "camunda.io/point-in-time-restore", PointInTimeRestoreKey)
}
```

Add to `pkg/logicalbackup/keys_test.go`:

```go
func TestRecordsSnapshotName(t *testing.T) {
	assert.Equal(t, "camunda_zeebe_records_backup_42", RecordsSnapshotName(42))
}

func TestCamundaIndexPatternsExcludeOptimizeUnlessAsked(t *testing.T) {
	without := CamundaIndexPatterns(false)
	assert.NotContains(t, without, optimizeIndices)
	assert.Contains(t, without, ZeebeRecordIndices)

	with := CamundaIndexPatterns(true)
	assert.Contains(t, with, optimizeIndices)
	assert.Subset(t, with, without)
}

func TestHasOptimizeSnapshot(t *testing.T) {
	assert.False(t, HasOptimizeSnapshot(nil))
	assert.False(t, HasOptimizeSnapshot([]string{"camunda_operate_8.9.9_part_1_of_6"}))
	assert.True(t, HasOptimizeSnapshot([]string{"camunda_optimize_8.9.9_part_1_of_2"}))
}
```

- [ ] **Step 2: Run the tests and confirm they fail**

Run: `go test ./pkg/labels/... ./pkg/logicalbackup/... -v`
Expected: FAIL, the identifiers do not exist.

- [ ] **Step 3: Verify the index and snapshot naming against Camunda 8.9**

Use the `camunda-docs` MCP server. Confirm the index prefixes that Camunda 8.9 creates in Elasticsearch, and the snapshot names that `POST /actuator/backupHistory` schedules. Record the answers in the PR body, exactly as the backup epic recorded its protocol facts. Only then write the patterns. Do not answer from memory.

- [ ] **Step 4: Add the label owners**

In `pkg/labels/labels.go`, next to the existing owner keys:

```go
	// LogicalRestoreKey names the LogicalRestore that owns a resource.
	LogicalRestoreKey = "camunda.io/logical-restore"
	// PointInTimeRestoreKey names the PointInTimeRestore that owns a
	// resource.
	PointInTimeRestoreKey = "camunda.io/point-in-time-restore"
```

```go
// LogicalRestore returns the owner labels of a LogicalRestore.
func LogicalRestore(name string) Owner { return Owner{Key: LogicalRestoreKey, Name: name} }

// PointInTimeRestore returns the owner labels of a PointInTimeRestore.
func PointInTimeRestore(name string) Owner {
	return Owner{Key: PointInTimeRestoreKey, Name: name}
}
```

- [ ] **Step 5: Move the artifact names into `pkg/logicalbackup/keys.go`**

```go
// ZeebeRecordIndices is the index pattern of the exported Zeebe record
// indices. It is the default prefix of the exporter, and the operator
// configures no other prefix.
const ZeebeRecordIndices = "zeebe-record*"

// optimizeIndices is the index pattern of the Optimize indices. They are
// restored only when the backup holds Optimize snapshots.
const optimizeIndices = "camunda-optimize*"

// RecordsSnapshotName returns the name of the Elasticsearch snapshot that
// holds the exported Zeebe record indices of a backup id. The backup writes
// it, and a restore locates it by the same rule.
func RecordsSnapshotName(id int64) string {
	return "camunda_zeebe_records_backup_" + strconv.FormatInt(id, 10)
}

// CamundaIndexPatterns returns the index patterns that a restore deletes from
// the target before it restores the snapshots. It includes the Optimize
// indices only when the backup holds Optimize snapshots. A backup without
// them cannot restore them, and deleting them erases Optimize data that the
// restore cannot put back.
func CamundaIndexPatterns(withOptimize bool) []string {
	patterns := slices.Clone(camundaIndices)
	if withOptimize {
		patterns = append(patterns, optimizeIndices)
	}

	return patterns
}

// HasOptimizeSnapshot reports whether a backup's recorded snapshot names hold
// an Optimize snapshot.
func HasOptimizeSnapshot(names []string) bool {
	for _, name := range names {
		if strings.Contains(name, optimizeSnapshotMarker) {
			return true
		}
	}

	return false
}
```

`camundaIndices` and `optimizeSnapshotMarker` are the two values Step 3 verified. Write them as unexported package variables with a comment that names the documentation page the verification came from, the way `pkg/camundaconfig/keys.go` names the Camunda class behind every key:

```go
// camundaIndices are the index patterns of a Camunda 8.9 cluster that a
// restore replaces, without the Optimize indices. <name the verified source>
var camundaIndices = []string{ /* the verified patterns, plus ZeebeRecordIndices */ }

// optimizeSnapshotMarker is the fragment that appears in the name of every
// Optimize snapshot and in no other Camunda snapshot name.
const optimizeSnapshotMarker = "_optimize_"
```

Delete `RecordsSnapshotName` and `zeebeRecordIndices` from `internal/controller/logicalbackupelasticsearch/statemachine.go` and use the `pkg/logicalbackup` versions there. The restore controllers must never import a controller package.

- [ ] **Step 6: Run the tests and confirm they pass**

Run: `go test ./pkg/... ./internal/controller/logicalbackupelasticsearch/... -v`
Expected: PASS. The Elasticsearch backup suite still passes with the moved names.

- [ ] **Step 7: Lift the stuck-pod check into `pkg/podstate`**

Both restore controllers must report a pod that cannot start, exactly as `internal/controller/logicalbackuprdbms` does today. They fan out in parallel, so the shared helper has to exist before either starts.

Move `stuckPod`, `stuckWaitingReasons`, and `podsOf` out of `internal/controller/logicalbackuprdbms/dump.go` into a new package:

```go
// Package podstate reports a pod that cannot start on its own. The kubelet
// retries such a pod without end, the Job that owns it stays active, and the
// Job consumes no backoff. A controller that waits on a Job therefore never
// learns about it from the Job alone.
package podstate

// Stuck reports the first pod under selector that cannot start, as a
// pre-check failure that names the pod, the container, and the waiting
// reason. what names the work the pod does, for example "the dump Job", and
// it goes into the message. Stuck returns nil when every pod progresses.
//
// The reader must be uncached. A pod that just entered a waiting state is
// the reason to call this at all.
func Stuck(
	ctx context.Context, reader client.Reader, namespace string,
	selector map[string]string, what string,
) (*conditions.PreCheckFailure, error)
```

Write `pkg/podstate/podstate_test.go` first, as a table test with a fake client, and carry over every case the RDBMS backup controller's tests already cover: `CreateContainerConfigError`, `CreateContainerError`, `ErrImagePull`, `ImagePullBackOff`, `InvalidImageName`, an unschedulable pod, an init container in a waiting state, and a pod where everything progresses. Then change `internal/controller/logicalbackuprdbms/dump.go` to call it with `client.MatchingLabels{components.BackupUIDLabel: string(backup.UID)}` and keep its own tests green.

- [ ] **Step 8: Run every affected suite**

Run: `go test ./pkg/... ./internal/controller/logicalbackuprdbms/... ./internal/controller/logicalbackupelasticsearch/... -v`
Expected: PASS.

- [ ] **Step 9: Commit**

```bash
git add pkg/labels pkg/logicalbackup pkg/podstate internal/controller
git commit -m "refactor(logicalbackup): let a restore reach the names and pod checks it must reuse (#110)"
```

### Task 3: `pkg/restore` reads the live broker StatefulSet

**Consumes:** `pkg/components/camundacluster` names and constants.
**Produces:** `restore.Target` and `restore.ReadTarget`.

- [ ] **Step 1: Write the failing test**

Create `pkg/restore/target_test.go`. Build a StatefulSet fixture that matches what `pkg/components/camundacluster` renders, load it through a fake client, and assert the extracted facts.

```go
func brokerStatefulSet() *appsv1.StatefulSet {
	replicas := int32(0) // suspended
	return &appsv1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{Name: "my-cluster-zeebe", Namespace: "ns"},
		Spec: appsv1.StatefulSetSpec{
			Replicas: &replicas,
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{
					"camunda.io/cluster": "my-cluster", "camunda.io/component": "zeebe",
				}},
				Spec: corev1.PodSpec{
					ServiceAccountName: "my-cluster",
					Containers: []corev1.Container{{
						Name:  "camunda",
						Image: "camunda/camunda:8.9.9",
						Env: []corev1.EnvVar{
							{Name: "CAMUNDA_CLUSTER_SIZE", Value: "3"},
							{Name: "CAMUNDA_CLUSTER_PARTITIONCOUNT", Value: "6"},
						},
						VolumeMounts: []corev1.VolumeMount{
							{Name: "data", MountPath: "/usr/local/camunda/data"},
						},
					}},
				},
			},
			VolumeClaimTemplates: []corev1.PersistentVolumeClaim{{
				ObjectMeta: metav1.ObjectMeta{Name: "data"},
				Spec: corev1.PersistentVolumeClaimSpec{
					AccessModes: []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce},
					Resources: corev1.VolumeResourceRequirements{
						Requests: corev1.ResourceList{corev1.ResourceStorage: resource.MustParse("10Gi")},
					},
				},
			}},
		},
	}
}

func TestReadTargetReadsTheFactsOffTheLiveBroker(t *testing.T) {
	c := fake.NewClientBuilder().WithScheme(testScheme(t)).WithObjects(brokerStatefulSet()).Build()
	cluster := &v1.CamundaCluster{ObjectMeta: metav1.ObjectMeta{Name: "my-cluster", Namespace: "ns"}}

	target, err := ReadTarget(context.Background(), c, cluster)
	require.NoError(t, err)
	assert.Equal(t, int32(3), target.Brokers)
	assert.Equal(t, int32(6), target.Partitions)
	assert.Equal(t, "8.9.9", target.Version)
	assert.Equal(t, "camunda", target.Broker.Name)
	assert.Equal(t, "data", target.ClaimTemplate.Name)
}

func TestReadTargetReportsAMissingStatefulSetAsAnInvalidReference(t *testing.T) {
	c := fake.NewClientBuilder().WithScheme(testScheme(t)).Build()
	cluster := &v1.CamundaCluster{ObjectMeta: metav1.ObjectMeta{Name: "my-cluster", Namespace: "ns"}}

	_, err := ReadTarget(context.Background(), c, cluster)
	var failure *conditions.PreCheckFailure
	require.ErrorAs(t, err, &failure)
	assert.Equal(t, v1.ReasonInvalidReference, failure.Reason)
}
```

Add table cases for: no container named `camunda`, a missing `CAMUNDA_CLUSTER_SIZE`, a non-numeric `CAMUNDA_CLUSTER_SIZE`, a broker image with no tag, and no claim template named `data`. Each returns a `*conditions.PreCheckFailure` with `ReasonInvalidReference` and a message that names what is missing.

- [ ] **Step 2: Run the test and confirm it fails**

Run: `go test ./pkg/restore/... -v`
Expected: FAIL, the package does not exist.

- [ ] **Step 3: Write `pkg/restore/doc.go` and `pkg/restore/target.go`**

The package doc says what the package is and what it refuses to do:

```go
// Package restore is the shared machinery of the two restore kinds,
// LogicalRestore and PointInTimeRestore: the facts they read off the live
// broker StatefulSet, the broker data volumes they delete and create again,
// and the restore-application Job they run once per broker.
//
// The restore Jobs never re-render the broker configuration. They copy it
// from the StatefulSet that the CamundaCluster controller applied, which
// still exists while the cluster is suspended. The restore application then
// always runs with the configuration the brokers run with, and the two
// cannot drift.
//
// The package holds no knowledge of either restore CR's spec. It renders and
// applies. The controllers decide.
package restore
```

`ReadTarget` reads `<cluster>-zeebe` in the cluster's namespace, finds the container named `camunda`, parses `CAMUNDA_CLUSTER_SIZE` and `CAMUNDA_CLUSTER_PARTITIONCOUNT` from its environment, takes the version from the image tag, and finds the claim template named `data`. Every failure is a `*conditions.PreCheckFailure` with `v1.ReasonInvalidReference`. A transport error is a plain wrapped error.

The environment lookup must ignore an entry with `ValueFrom`, because a value that comes from a Secret or a field is not readable here. Such an entry is a failure with a message that names the variable.

- [ ] **Step 4: Run the test and confirm it passes**

Run: `go test ./pkg/restore/... -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add pkg/restore
git commit -m "feat(restore): read the restore facts off the live broker StatefulSet (#110)"
```

### Task 4: `pkg/restore` recreates the broker data volumes

**Consumes:** `restore.Target`.
**Produces:** `ClaimNames`, `ClaimSize`, `BuildClaim`, `ClaimInput`, `RecreateClaims`, `Progress`, `Apply`.

- [ ] **Step 1: Write the failing pure tests**

Create `pkg/restore/claims_test.go`:

```go
func TestClaimNamesFollowTheStatefulSet(t *testing.T) {
	target := &Target{StatefulSet: brokerStatefulSet(), Brokers: 3}
	assert.Equal(t,
		[]string{"data-my-cluster-zeebe-0", "data-my-cluster-zeebe-1", "data-my-cluster-zeebe-2"},
		target.ClaimNames(),
	)
}

func TestClaimSizePrefersTheRecordedRestoreSize(t *testing.T) {
	target := &Target{ClaimTemplate: &brokerStatefulSet().Spec.VolumeClaimTemplates[0]}

	recorded := resource.MustParse("30Gi")
	assert.Equal(t, recorded, target.ClaimSize(&recorded))
	assert.Equal(t, resource.MustParse("10Gi"), target.ClaimSize(nil))
}

func TestBuildClaimCarriesNoOwnerReference(t *testing.T) {
	target := &Target{StatefulSet: brokerStatefulSet(), Brokers: 1,
		ClaimTemplate: &brokerStatefulSet().Spec.VolumeClaimTemplates[0]}

	claim := target.BuildClaim(0, resource.MustParse("10Gi"))
	assert.Empty(t, claim.OwnerReferences)
	assert.Equal(t, "data-my-cluster-zeebe-0", claim.Name)
	assert.Equal(t, "my-cluster", claim.Labels["camunda.io/cluster"])
	assert.Equal(t, "zeebe", claim.Labels["camunda.io/component"])
}
```

The claim labels are the claim template's labels, so the StatefulSet's own selectors and the `PVCAutoResize` controller keep working. `BuildClaim` therefore takes no owner: the restore owner label is deliberately **not** on the claim, because the claim outlives the restore.

- [ ] **Step 2: Write the failing behaviour test**

Add an envtest-free test for `RecreateClaims` with a fake client and an interceptor, covering:
- every claim is deleted and applied again, and the applied claim asks for the size passed in
- a claim already listed in `Progress` handling is not deleted twice, so re-entry is safe
- while a deleted claim still exists because a pod holds it, `RecreateClaims` returns `Progress{Done: false}` with a message that names the claim, and no error

Re-entry safety comes from the caller: the controller records `status.recreatedClaims` before the delete and skips a recorded name on re-entry. `RecreateClaims` takes the recorded list as `ClaimInput.Recreated` and returns the grown list as `Progress.Recreated`. The controller writes `Progress.Recreated` into `status.recreatedClaims` and flushes it before it acts again.

- [ ] **Step 3: Run and confirm both fail**

Run: `go test ./pkg/restore/... -v`
Expected: FAIL.

- [ ] **Step 4: Write `pkg/restore/claims.go` and `pkg/restore/apply.go`**

`Apply` is three lines and one comment:

```go
// Apply server-side applies obj under manager, forcing ownership of every
// field the operator sets. Every resource a restore manages goes through it,
// so the field manager of a restore is one string in one place.
func Apply(ctx context.Context, c client.Client, obj client.Object, manager client.FieldOwner) error {
	return c.Patch(ctx, obj, client.Apply, manager, client.ForceOwnership)
}
```

`RecreateClaims` deletes each claim that is not yet recorded, then applies the claim the StatefulSet expects. It returns `Progress{Done: true}` only when every claim exists again and is not terminating.

- [ ] **Step 5: Run the tests and confirm they pass**

Run: `go test ./pkg/restore/... -v`
Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add pkg/restore
git commit -m "feat(restore): give the brokers empty data volumes the StatefulSet expects (#110)"
```

### Task 5: `pkg/restore` renders the per-broker restore Job

**Consumes:** `restore.Target`.
**Produces:** `JobInput`, `JobName`, `BuildJob`, `RestoreEntrypoint`, `ComponentRestore`, the two field manager constants.

- [ ] **Step 1: Verify the restore application against Camunda 8.9**

Use the `camunda-docs` MCP server. Confirm the path of the standalone restore application inside the `camunda/camunda` image, its command-line flags (`--backupId`, `--to`), and what it requires of the data directory. Record the answers in the PR body. The plan assumes `/usr/local/camunda/bin/restore`, next to the `/usr/local/camunda/bin/camunda` entrypoint the operator already uses. Correct the constant if the docs say otherwise.

- [ ] **Step 2: Write the failing golden test**

Create `pkg/restore/job_test.go` with a golden snapshot per case, under `pkg/restore/testdata/golden/`:

- `elasticsearch-broker-0.yaml` — `Args: []string{"--backupId=42"}`, ordinal 0
- `elasticsearch-broker-2.yaml` — ordinal 2, to prove the node id and the claim name follow the ordinal
- `rdbms-no-args.yaml` — `Args: nil`
- `pitr-to-timestamp.yaml` — `Args: []string{"--to=2026-07-30T14:30:00Z"}`, owner label `camunda.io/point-in-time-restore`

Assert in plain Go, beside the golden compare, the properties that must never regress:

```go
func TestBuildJobMirrorsTheBrokerAndPinsTheNodeID(t *testing.T) {
	target := readTargetFixture(t)
	job, err := BuildJob(JobInput{
		Target: target, Owner: restoreCR(), OwnerLabel: labels.LogicalRestore("r"),
		Ordinal: 2, Args: []string{"--backupId=42"},
	})
	require.NoError(t, err)

	container := job.Spec.Template.Spec.Containers[0]
	assert.Equal(t, target.Broker.Image, container.Image)
	assert.Equal(t, []string{RestoreEntrypoint}, container.Command)
	assert.Equal(t, []string{"--backupId=42"}, container.Args)
	assert.Equal(t, target.Broker.Resources, container.Resources)
	assert.Equal(t, "2", envValue(container.Env, "CAMUNDA_CLUSTER_NODEID"))
	assert.Equal(t, "3", envValue(container.Env, "CAMUNDA_CLUSTER_SIZE"))
	assert.Equal(t, target.Broker.VolumeMounts, container.VolumeMounts)

	assert.Equal(t, "data-my-cluster-zeebe-2", dataClaimName(job))
	assert.Equal(t,
		target.StatefulSet.Spec.Template.Spec.TopologySpreadConstraints,
		job.Spec.Template.Spec.TopologySpreadConstraints,
	)
	assert.Equal(t, "restore", job.Labels["camunda.io/component"])
	assert.Equal(t, "r", job.Labels["camunda.io/logical-restore"])
	assert.Equal(t, "my-cluster", job.Labels["camunda.io/cluster"])
	assert.Equal(t, corev1.RestartPolicyNever, job.Spec.Template.Spec.RestartPolicy)
}
```

- [ ] **Step 3: Run and confirm it fails**

Run: `go test ./pkg/restore/... -run TestBuildJob -v`
Expected: FAIL.

- [ ] **Step 4: Write `pkg/restore/job.go`**

What `BuildJob` copies from the broker StatefulSet, and why each is needed:

| Copied | Why |
| --- | --- |
| container image | the restore application ships in the same image, so the version can never drift |
| container environment | the restore application must see the same secondary storage, credentials, and primary-storage backup store |
| container volume mounts | the data directory and the Elasticsearch CA must be at the same paths |
| container resources | the restore reads and writes the same data the broker does |
| container security context | the data directory is owned by the broker's user |
| pod volumes | a volume mount without its volume is not a valid pod |
| pod service account | a workload-identity bucket is reachable only under the broker's account |
| pod security context and image pull secrets | the pod must be admissible and the image pullable where the broker's is |
| pod labels | the pods look like broker pods to a topology spread constraint |
| topology spread constraints | with a `WaitForFirstConsumer` storage class, the pod that binds a recreated claim pins its zone, and the brokers must be able to schedule into it afterwards |

What `BuildJob` changes:

- the `data` volume becomes a `persistentVolumeClaim` on `data-<cluster>-zeebe-<ordinal>` instead of a claim template
- the command becomes `[]string{RestoreEntrypoint}` with `Args` from the input, replacing the broker's node-id shell wrapper
- `CAMUNDA_CLUSTER_NODEID` is set to the ordinal as a plain value, because a Job pod's host name carries no ordinal
- `RestartPolicy` is `Never`, `BackoffLimit` is `0`, and there is no readiness or liveness probe

`BackoffLimit` is zero because the restore application refuses a non-empty data directory. A second pod finds the directory the first one wrote and fails for the wrong reason. A failed restore is retried by creating a new restore resource, which recreates the volume first.

`JobName` follows the `pkg/components/logicalbackuprdbms` rule: `boundedName(owner.GetName(), limit) + "-restore-" + ordinal`, truncated deterministically with a hash so a long CR name still yields a DNS label. Copy `boundedName` into `pkg/restore` rather than exporting it from a components package, and test the truncation.

- [ ] **Step 5: Run and confirm it passes**

Run: `go test ./pkg/restore/... -v`
Expected: PASS, and the four goldens exist.

- [ ] **Step 6: Commit**

```bash
git add pkg/restore
git commit -m "feat(restore): run the restore application with the broker's own configuration (#110)"
```

### Task 6: The `pgbootstrap` seam, the scaffold removal, the samples, and the docs

- [ ] **Step 1: Write the failing test for `pgbootstrap.Open`**

Add to `pkg/pgbootstrap/pgbootstrap_test.go`, against the existing testcontainer:

```go
func TestOpenConnectsToANamedDatabase(t *testing.T) {
	ctx := context.Background()
	b := connect(t)
	require.NoError(t, b.EnsureDatabase(ctx, "opened"))

	conn, err := Open(ctx, adminConn, "opened")
	require.NoError(t, err)
	defer closeQuietly(conn)

	var name string
	require.NoError(t, conn.QueryRow(ctx, "SELECT current_database()").Scan(&name))
	assert.Equal(t, "opened", name)
}
```

- [ ] **Step 2: Run and confirm it fails**

Run: `go test ./pkg/pgbootstrap/... -run TestOpen -v`
Expected: FAIL, `Open` is undefined.

- [ ] **Step 3: Add `Open` and rename the credential fields**

```go
// Open dials database on the server that c describes and returns the live
// connection. It is how a caller that must run its own SQL inside a logical
// database reaches it, so the DSN of this operator is built in one place and
// one place only. The caller closes the connection.
func Open(ctx context.Context, c Connection, database string) (*pgx.Conn, error) {
	return dial(ctx, c, database)
}
```

Rename `Connection.AdminUser` to `User` and `Connection.AdminPassword` to `Password`, and update the GoDoc: the type carries the credentials of whichever role the caller holds, not only an administrator. Update `internal/controller/database/controller.go`, `internal/controller/databaseserverconfig/controller.go`, and the package tests.

- [ ] **Step 4: Run and confirm it passes**

Run: `go test ./pkg/pgbootstrap/... ./internal/controller/database/... ./internal/controller/databaseserverconfig/... -v`
Expected: PASS.

- [ ] **Step 5: Remove the scaffold controllers**

Delete `internal/controller/logicalrestore_controller.go`, `internal/controller/pointintimerestore_controller.go`, and their two test files. Remove the two registrations from `cmd/main.go`. Leave every `// +kubebuilder:scaffold:` marker in place. Both controllers are registered again in PR2 and PR3.

- [ ] **Step 6: Rewrite the two samples**

`config/samples/core_v1_logicalrestore.yaml`:

```yaml
apiVersion: core.camunda.io/v1
kind: LogicalRestore
metadata:
  name: logicalrestore-sample
  namespace: default
spec:
  backupRef:
    kind: LogicalBackupElasticsearch
    name: logicalbackupelasticsearch-sample
  targetClusterRef:
    name: camundacluster-sample
```

`config/samples/core_v1_pointintimerestore.yaml`:

```yaml
apiVersion: core.camunda.io/v1
kind: PointInTimeRestore
metadata:
  name: pointintimerestore-sample
  namespace: default
spec:
  clusterRef:
    name: camundacluster-sample
  timestamp: "2026-07-30T14:30:00Z"
```

`internal/controller/samples_schema_test.go` validates the samples against the CRDs, so this step is covered by the existing suite.

- [ ] **Step 7: Rewrite both CRD doc pages**

Load `simple-english:simple-english` first.

`docs/crds/logicalrestore.md`:
- keep the "Not implemented yet" warning, because PR1 ships no controller
- keep the phase list unchanged
- add the compatibility rule that the code enforces and the page does not yet state: **the target's `spec.backupStorageRef` must name the same `ObjectStorageConfig` the backup wrote to.** The restore reads the backup's artifacts through the target's bucket, so a different bucket cannot hold them. It fails with `IncompatibleTarget`.
- state that the Elasticsearch path registers its own snapshot repository on the target's Elasticsearch, derived from the backup's pinned bucket and its recorded repository prefix, and that this is what makes a restore into a second cluster work
- state that the restore Jobs copy the broker configuration from the live broker StatefulSet, and that a cluster whose broker StatefulSet was deleted cannot restore until its controller applies it again
- state the PVC rule: the operator deletes and creates the broker data volumes again, sized from the backup's recorded restore size, and the volumes belong to the StatefulSet, not to the restore

`docs/crds/pointintimerestore.md`:
- keep the "Not implemented yet" warning
- change the phase list to `Pending | ValidatingDatabaseState | RestoringPrimaryStorage | Completed | Failed`
- insert `ValidatingDatabaseState` as step 5 of "How it works", before the destructive step: the operator connects to the logical database with the cluster's application credentials, reads `LAST_UPDATED` for every partition from `EXPORTER_POSITION`, and holds the restore in `Pending` with `Ready: DatabaseNotRestored` when a partition row is missing or when any `LAST_UPDATED` is later than `spec.timestamp` plus one minute of slack. It touches no volume while it holds.
- add a "Limits of the database-state check" note: the check proves that the database is not ahead of the requested point. It cannot prove that the database was restored to exactly that point. A database restored to an earlier point passes, and that is safe, because Zeebe re-exports the difference after the restore. The restore application's own check stays the authoritative gate. The check only moves the common failure before the volume deletion.
- add the `DatabaseNotRestored` row to the status table
- add the one-minute slack to the status table row and say why it exists: the database clock and the caller's timestamp source are not the same clock
- **remove the `namespace` key from both `clusterRef` examples and from the API reference block.** `ClusterRef` names a cluster in the namespace of the restore and never crosses a namespace.
- state that `spec.timestamp` must not lie in the future, and that the controller checks it at reconcile time with `PitrUnavailable`, because a CEL rule has no clock

- [ ] **Step 8: Verify the whole PR**

```bash
make manifests generate
git status --porcelain config api   # must be empty
go test ./...
make all
mkdocs build --strict
```

- [ ] **Step 9: Commit and open the PR**

```bash
git add -A
git commit -m "docs(crds): record the restore contract the controllers will implement (#110)"
```

Open the PR with `Towards #110`. Run `feature-dev-workflow:copilot-review-loop` until it returns nothing new. Self-merge into `feat/restore-controllers` and close #110.

### Review checkpoint after PR1

Before PR2 and PR3 fan out, run `feature-dev-workflow:reviewing-feature-progress` over the merged feature branch. Confirm that `pkg/restore` reads as one package, that no controller package is imported by another, and that the two doc pages agree with the shipped schema. Record the answers to the two Camunda verification steps in the state file, so PR2 and PR3 do not repeat them.

---

## PR2 — The LogicalRestore controller (#111, branch `feat/logicalrestore-controller`, worktree `.claude/worktrees/restore-controllers--logical`)

Implements `docs/crds/logicalrestore.md` in full, on both secondary storage paths.

**Files:**
- Create: `pkg/esadmin/restore.go`, `pkg/esadmin/restore_test.go`; modify `pkg/esadmin/esadmintest/server.go`
- Create: `pkg/components/logicalrestore/{job.go,job_test.go,testdata/golden/}` (the `pg_restore` Job)
- Create: `internal/controller/logicalrestore/{controller.go,admit.go,compatibility.go,secondary_elasticsearch.go,secondary_rdbms.go,primary.go,suite_test.go,controller_test.go,schema_test.go}` plus the unit tests beside each file
- Modify: `cmd/main.go` (register the controller with `Options{CLIImage: cliImage}`)
- Modify: `config/rbac/role.yaml` (generated from the new RBAC markers)
- Modify: `docs/crds/logicalrestore.md` (remove the "Not implemented yet" warning, correct anything the code found)

### Task 1: Elasticsearch index deletion and snapshot restore

**Produces:** the `esadmin-restore-api` contract row.

- [ ] **Step 1: Verify the Elasticsearch API surface**

The Elasticsearch snapshot restore API and the index delete API are Elastic's surface, not Camunda's. Verify them against the Elasticsearch documentation for the version the operator runs (`esVersion` in the e2e suite is 9.2.4), the same way the backup epic verified the repository settings. Confirm the request shapes, the query parameters that make a delete tolerate a missing index, and how to tell that a restore finished. Record the answers in the PR body.

- [ ] **Step 2: Write the failing tests against `esadmintest`**

Create `pkg/esadmin/restore_test.go`. Cover, one test each:
- `DeleteIndices` sends one request that names every pattern, and tolerates a pattern that matches nothing
- `DeleteIndices` maps a transport failure to `ErrUnreachable` and a rejection to `ErrRejected`
- `RestoreSnapshot` posts the restore of one snapshot with the given indices and does not wait for completion
- `RestoreSnapshot` maps both error classes
- `RestoreProgress` reports `RestoreInProgress` while a recovery is active and `RestoreDone` when none is
- `RestoreProgress` maps both error classes

Extend `pkg/esadmin/esadmintest/server.go` with the routes and the operation names `"indexDelete"`, `"snapshotRestore"`, and `"recovery"`, so `FailNext` and `DropNext` reach them.

- [ ] **Step 3: Run and confirm they fail**

Run: `go test ./pkg/esadmin/... -v`
Expected: FAIL.

- [ ] **Step 4: Write `pkg/esadmin/restore.go`**

Follow the existing file exactly: `c.api.Do` with an `adminhttp.Request`, `adminhttp.Status` for the accepted codes, `url.PathEscape` on every path segment that carries a name, and the two sentinels for the two error classes.

- [ ] **Step 5: Run and confirm they pass**

Run: `go test ./pkg/esadmin/... -v`
Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add pkg/esadmin
git commit -m "feat(esadmin): delete indices and restore snapshots (#111)"
```

### Task 2: The `pg_restore` Job builder

- [ ] **Step 1: Write the failing golden test**

Create `pkg/components/logicalrestore/job_test.go`. The Job mirrors the dump Job in `pkg/components/logicalbackuprdbms`, run backwards:

- an init container from the `camunda-operator-cli` image downloads the dump object into a scratch volume, through a `download` subcommand that mirrors the existing `upload` subcommand's environment contract
- the main container runs `postgres:<major>` and runs `pg_restore --clean --if-exists --no-owner --dbname=<database> /scratch/camunda.dump`
- the pod uses the cluster's ServiceAccount, the same scratch volume rules, the same security context, and the same active deadline default as the dump Job

Golden cases: static bucket credentials, workload identity, and a scratch volume with a storage class.

Assert in plain Go that the connection environment is the target's `DatabaseConfig` and its **backup** credentials, and that the Job carries the restore's owner labels and its UID label.

- [ ] **Step 2: Run and confirm it fails, then write the builder and the `download` subcommand**

The `download` subcommand goes next to `cmd/upload`, reads the same `UPLOAD_*` style contract under `DOWNLOAD_*` names, and streams the object from `pkg/objectstore` into a file. Give it its own test against `fileblob`, exactly as the `upload` subcommand has.

- [ ] **Step 3: Run and confirm it passes**

Run: `go test ./pkg/components/logicalrestore/... ./cmd/... -v`
Expected: PASS.

- [ ] **Step 4: Commit**

```bash
git add pkg/components/logicalrestore cmd
git commit -m "feat(logicalrestore): restore a dump into the target database (#111)"
```

### Task 3: Admission and the Pending holds

- [ ] **Step 1: Write the failing envtest specs**

Create `internal/controller/logicalrestore/suite_test.go`, copying `internal/controller/logicalbackupelasticsearch/suite_test.go` and passing short `Options`. Create `controller_test.go` with a `Describe("LogicalRestore admission")` covering:

- a target that is not suspended holds the restore in `Pending` with `Ready=False`, reason `ClusterNotSuspended`, and creates nothing
- a `backupRef` that names no resource holds the restore in `Pending` with reason `InvalidReference`
- a backup that is not `Completed` holds the restore in `Pending` with reason `InvalidReference`
- a `targetClusterRef` that names no cluster holds the restore in `Pending` with reason `InvalidReference`
- a suspended target and a completed backup move the restore to `ValidatingCompatibility` and pin `status.backupId`, `status.storageType`, and `status.targetClusterUID`
- flipping `spec.suspend` to `true` on the target wakes the waiting restore through the watch, without waiting for the retry interval

- [ ] **Step 2: Run and confirm they fail**

Run: `go test ./internal/controller/logicalrestore/... -v`
Expected: FAIL.

- [ ] **Step 3: Write `controller.go` and `admit.go`**

`controller.go` holds `Options`, `Reconciler`, `New`, `Reconcile`, the terminal transitions (`complete`, `fail`, `stageTerminal`), `progressing`, and `SetupWithManager`. Copy the structure of `internal/controller/logicalbackupelasticsearch/controller.go`: read the restore live through `APIReader`, defer one `component.FlushStatus`, re-stage the terminal condition on every look, and switch on `status.phase`.

RBAC markers:

```go
// +kubebuilder:rbac:groups=core.camunda.io,resources=logicalrestores,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=core.camunda.io,resources=logicalrestores/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=core.camunda.io,resources=logicalrestores/finalizers,verbs=update
// +kubebuilder:rbac:groups=core.camunda.io,resources=camundaclusters;secondarystorageconfigs;databaseconfigs;databaseserverconfigs;objectstorageconfigs;logicalbackupelasticsearches;logicalbackuprdbmses,verbs=get;list;watch
// +kubebuilder:rbac:groups=apps,resources=statefulsets,verbs=get;list;watch
// +kubebuilder:rbac:groups=batch,resources=jobs,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="",resources=persistentvolumeclaims,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="",resources=secrets,verbs=get;list;watch
// +kubebuilder:rbac:groups="",resources=pods,verbs=list
// +kubebuilder:rbac:groups=events.k8s.io,resources=events,verbs=create;patch
```

`SetupWithManager` indexes restores by the namespace and name of their `targetClusterRef`, watches `CamundaCluster` for a suspend flip, watches both backup kinds for a phase change to `Completed`, owns Jobs, and is `Named("logicalrestore")`.

The controller **never** writes `spec.suspend`. Add that as a comment on the suspend check, so a later reader does not add it.

- [ ] **Step 4: Run and confirm they pass, then commit**

```bash
git add internal/controller/logicalrestore cmd config
git commit -m "feat(logicalrestore): hold a restore until its target is suspended (#111)"
```

### Task 4: `ValidatingCompatibility`

- [ ] **Step 1: Write the failing unit tests**

Create `internal/controller/logicalrestore/compatibility_test.go` as a pure table test over a `check(...)` function that takes the resolved inputs and returns `*conditions.PreCheckFailure` or nil. Cases, one row each:

| Case | Result |
| --- | --- |
| Elasticsearch backup, Elasticsearch target, same partitions, same version, same bucket | nil |
| Elasticsearch backup, relational target | `IncompatibleTarget`, message names both types |
| relational backup, Elasticsearch target | `IncompatibleTarget` |
| partition counts differ | `IncompatibleTarget`, message names both counts |
| target bucket differs from the backup's pinned bucket | `IncompatibleTarget`, message names both buckets |
| Elasticsearch, backup 8.9.9, target 8.9.9 | nil |
| Elasticsearch, backup 8.9.9, target 8.9.10 | `IncompatibleTarget`, message states that Elasticsearch needs the exact version |
| Elasticsearch, backup 8.9.9, target 8.10.0 | `IncompatibleTarget` |
| relational, backup 8.9.9, target 8.9.9 | nil |
| relational, backup 8.9.9, target 8.9.12 | nil, the patch level is free |
| relational, backup 8.9.9, target 8.10.0 | nil, one minor newer is allowed |
| relational, backup 8.9.9, target 8.11.0 | `IncompatibleTarget` |
| relational, backup 8.10.0, target 8.9.9 | `IncompatibleTarget`, older is never allowed |
| either version is not `x.y.z` | `IncompatibleTarget`, message names the unreadable version |

The source version comes from `status.version` on the backup, which PR1 added to both backup kinds. The target version comes from `restore.Target.Version`. A backup that recorded no version fails with `IncompatibleTarget`, and the message says that the backup did not record its Camunda version. Add that row to the table too.

- [ ] **Step 2: Run, confirm they fail, write `compatibility.go`, run again**

The target's version comes from `restore.Target.Version`, which is the tag of the live broker image. The target's partition count comes from `restore.Target.Partitions`. Neither is readable from `status.management` while the cluster is suspended, which is the whole reason `restore.ReadTarget` exists.

- [ ] **Step 3: Add the envtest spec**

One spec that walks a compatible pair from `ValidatingCompatibility` into `RestoringSecondaryStorage`, and one that walks an incompatible pair into `Failed` with reason `IncompatibleTarget` and a `status.failureMessage` that names the mismatch.

- [ ] **Step 4: Commit**

```bash
git add internal
git commit -m "feat(logicalrestore): refuse a target that cannot hold the backup (#111)"
```

### Task 5: `RestoringSecondaryStorage`

- [ ] **Step 1: Write the failing envtest specs for the Elasticsearch path**

Drive `esadmintest` from the suite. Specs:

- the restore ensures a snapshot repository on the target's Elasticsearch, derived from the backup's pinned `BucketRef` and the prefix `logicalbackup.ClusterPrefix(bucketBasePath, backupNamespace, backup.status.repository)`, and records its name in `status.repository`
- the restore deletes the Camunda indices before it restores anything, and the delete does **not** name `camunda-optimize*` when the backup's `status.historySnapshots` holds no Optimize snapshot
- the delete **does** name `camunda-optimize*` when the backup's snapshots hold one
- the restore then restores every history snapshot and the records snapshot named by `logicalbackup.RecordsSnapshotName(status.backupId)`, and records them in `status.restoredSnapshots`
- while a recovery is active the restore stays in `RestoringSecondaryStorage` with `Ready=False`, reason `Progressing`
- when no recovery is active the restore moves to `RestoringPrimaryStorage`
- an unreachable Elasticsearch holds the restore with reason `ConnectionFailed` for the mid-run grace, then fails it
- a re-entered reconcile does not delete the indices a second time, because the phase advanced past the delete

The delete-then-restore ordering is the risk the spec names: a failure between the delete and the restore leaves the target's secondary storage empty. Record that in the doc page, and prove with a spec that a retry re-issues the restore and converges.

- [ ] **Step 2: Write the failing envtest specs for the relational path**

- the restore applies exactly one `pg_restore` Job, records it in `status.secondaryJobName`, and tracks it with the same `ocfjob.DefaultConvergingStatusHandler` the dump step uses
- a completed Job moves the restore to `RestoringPrimaryStorage`
- a failed Job fails the restore with a message that names the Job
- a pod that cannot start, for example on a missing Secret, reports `MissingSecret` through the mid-run grace and then fails, through `podstate.Stuck`, which PR1 already shipped

- [ ] **Step 3: Run, confirm they fail, write `secondary_elasticsearch.go` and `secondary_rdbms.go`, run again**

- [ ] **Step 4: Commit**

```bash
git add pkg internal
git commit -m "feat(logicalrestore): rebuild secondary storage on both paths (#111)"
```

### Task 6: `RestoringPrimaryStorage` and completion

- [ ] **Step 1: Write the failing envtest specs**

- the restore records `status.brokers` from `restore.ReadTarget` before it deletes anything
- the restore deletes and creates every broker data claim, records them in `status.recreatedClaims`, and sizes them from the backup's `status.storageSizes.zeebe`
- a backup that recorded no Zeebe size gives claims the size of the StatefulSet's claim template
- the recreated claims carry **no** owner reference to the restore
- the restore then applies one Job per broker, with `--backupId=<status.backupId>` on the Elasticsearch path and no arguments on the relational path, and records them in `status.primaryJobNames`
- every Job carries `camunda.io/component: restore` and `camunda.io/logical-restore: <name>`
- all Jobs complete, and the restore reaches `Completed` with `Ready=True`, reason `Completed`, and a `status.completionTime`
- one failing Job fails the restore with a message that names the broker
- deleting the restore removes its Jobs and leaves the claims in place

The last spec is the one that protects the ownership rule. Write it first.

- [ ] **Step 2: Run, confirm they fail, write `primary.go`, run again**

The phase is persisted before the delete. `status.recreatedClaims` is persisted before each delete, so a crash between the delete and the apply re-enters and does not delete a fresh claim.

- [ ] **Step 3: Update the CRD doc page**

Remove the "Not implemented yet" warning. Correct anything the code found. Load `simple-english:simple-english` first.

- [ ] **Step 4: Verify the whole PR**

```bash
make manifests generate
git status --porcelain config api   # must be empty
go test ./...
make all
mkdocs build --strict
```

- [ ] **Step 5: Commit and open the PR**

```bash
git add -A
git commit -m "feat(logicalrestore): restore the Zeebe partitions and report completion (#111)"
```

Open the PR with `Towards #111`. Run the review loop until clean. **Stop. Request the user's review. Do not merge.**

---

## PR3 — The PointInTimeRestore controller (#112, branch `feat/pointintimerestore-controller`, worktree `.claude/worktrees/restore-controllers--pitr`)

Implements `docs/crds/pointintimerestore.md` in full, including the `ValidatingDatabaseState` phase the spec added.

**Files:**
- Create: `internal/controller/pointintimerestore/{controller.go,admit.go,dbstate.go,dbstate_test.go,primary.go,suite_test.go,controller_test.go,schema_test.go}`
- Modify: `cmd/main.go`, `config/rbac/role.yaml` (generated)
- Modify: `docs/crds/pointintimerestore.md` (remove the "Not implemented yet" warning, correct anything the code found)

### Task 1: Admission, the storage chain, and the two capability rules

- [ ] **Step 1: Write the failing envtest specs**

Create `suite_test.go` and `controller_test.go`. `Describe("PointInTimeRestore admission")`:

- a cluster that is not suspended holds the restore in `Pending` with reason `ClusterNotSuspended`, and touches nothing
- a `clusterRef` that names no cluster holds it with reason `InvalidReference`
- a cluster with an empty `spec.storageRef` holds it with reason `InvalidReference`
- a `SecondaryStorageConfig` of type `elasticsearch` holds it with reason `InvalidReference` and a message that states that point-in-time restore does not exist for an Elasticsearch cluster
- a `DatabaseConfig` or `DatabaseServerConfig` that does not resolve holds it with reason `InvalidReference`
- a server with `pitr` unset, or `pitr.enabled: false`, fails the restore with reason `PitrUnavailable`
- a `spec.timestamp` older than `pitr.retentionPeriodDays` fails with reason `PitrUnavailable` and a message that names the retention period
- a `spec.timestamp` in the future fails with reason `PitrUnavailable`
- a server that two `Database` resources reference fails with reason `SharedServer` and a message that names both
- a server that exactly one `Database` references passes and moves the restore to `ValidatingDatabaseState`
- flipping `spec.suspend` to `true` wakes the waiting restore

The dedicated-server rule needs a list over `Database` resources filtered by `spec.serverRef`. Add a field index in `SetupWithManager` for it, so the check is one indexed list and not a full scan.

`Database` is namespaced and `DatabaseServerConfig` is cluster-scoped, so the rule counts `Database` resources across every namespace. Give the controller `list` on `databases` cluster-wide in its RBAC markers and say why in a comment.

- [ ] **Step 2: Run, confirm they fail, write `controller.go` and `admit.go`, run again**

Copy the reconciler shape and the flush pattern from PR2. RBAC markers mirror PR2's, minus the backup kinds and the object storage, plus `databases`:

```go
// +kubebuilder:rbac:groups=core.camunda.io,resources=pointintimerestores,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=core.camunda.io,resources=pointintimerestores/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=core.camunda.io,resources=pointintimerestores/finalizers,verbs=update
// +kubebuilder:rbac:groups=core.camunda.io,resources=camundaclusters;secondarystorageconfigs;databaseconfigs;databaseserverconfigs;databases,verbs=get;list;watch
// +kubebuilder:rbac:groups=apps,resources=statefulsets,verbs=get;list;watch
// +kubebuilder:rbac:groups=batch,resources=jobs,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="",resources=persistentvolumeclaims,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="",resources=secrets,verbs=get;list;watch
// +kubebuilder:rbac:groups="",resources=pods,verbs=list
// +kubebuilder:rbac:groups=events.k8s.io,resources=events,verbs=create;patch
```

- [ ] **Step 3: Commit**

```bash
git add internal cmd config
git commit -m "feat(pointintimerestore): validate the storage chain and the server rules (#112)"
```

### Task 2: `ValidatingDatabaseState`

**Consumes:** `pgbootstrap.Open` and `pgbootstrap.Connection` from PR1.

- [ ] **Step 1: Write the failing unit test for the decision**

Create `internal/controller/pointintimerestore/dbstate_test.go`. Split the decision from the input and out over a pure function, so the table test needs no database:

```go
// decide reports why the database is not ready for the requested point, or
// nil when it is.
func decide(
	positions []v1.PartitionPosition, partitions int32, want time.Time, slack time.Duration,
) *conditions.PreCheckFailure
```

Table cases:

| Case | Result |
| --- | --- |
| a row for every partition, every `LAST_UPDATED` before `want` | nil |
| a row for every partition, one `LAST_UPDATED` exactly at `want` | nil |
| one `LAST_UPDATED` at `want` plus 30 seconds, slack one minute | nil, inside the slack |
| one `LAST_UPDATED` at `want` plus 90 seconds, slack one minute | `DatabaseNotRestored`, the message names the partition and both times |
| a partition has no row | `DatabaseNotRestored`, the message names the missing partition |
| no rows at all | `DatabaseNotRestored`, the message states that the table holds no position |
| a row for a partition the cluster does not have | nil, an extra row is not the operator's business |

Pin the slack as an exported package constant with a comment that says why it exists:

```go
// clockSlack is how far the database's exporter clock is allowed to run ahead
// of spec.timestamp before the restore refuses to start. The database writes
// LAST_UPDATED with its own clock, and the caller's timestamp comes from
// another one. One minute bounds ordinary skew without hiding a database that
// was never rolled back.
const clockSlack = time.Minute
```

- [ ] **Step 2: Run, confirm it fails, write `decide`, run again**

- [ ] **Step 3: Write the failing envtest specs for the phase**

Inject the reader through `Options`, so the suite needs no PostgreSQL:

```go
// ReadPositions reads the exporter position of every partition from the
// logical database. Nil means the production reader, which connects with
// pgbootstrap. The tests point it at a fake.
ReadPositions func(
	ctx context.Context, conn pgbootstrap.Connection, database string,
) ([]v1.PartitionPosition, error)
```

Specs:

- a database ahead of `spec.timestamp` holds the restore in `Pending` with `Ready=False`, reason `DatabaseNotRestored`, records what it read in `status.observedPositions`, and **creates no Job and deletes no claim**
- the same restore recovers on its own once the reader reports positions behind the timestamp, and moves to `RestoringPrimaryStorage`
- a database that rejects the credentials reports `ConnectionFailed` and holds through the mid-run grace
- a missing credentials Secret reports `MissingSecret`
- the phase records `status.observedPositions` on the passing path too, so the operator can see what the check saw

The "creates no Job and deletes no claim" assertion is the whole point of the phase. Write it first, and assert on the live claim `creationTimestamp` and on an empty Job list.

- [ ] **Step 4: Write `dbstate.go`**

The production reader resolves the application credentials through `storageRef` to `SecondaryStorageConfig` to `DatabaseConfig.credentialsSecretRef`, exactly as `docs/crds/pointintimerestore.md` says, reads them with `secretref.Get` on the uncached reader, builds a `pgbootstrap.Connection` from the `DatabaseServerConfig`'s host and port, opens `DatabaseConfig.spec.databaseName` with `pgbootstrap.Open`, and runs one query.

The table name is upper case, so it must be quoted. `pgbootstrap.quoteIdentifier` rejects an upper-case name by design, so write the SQL literally and take no identifier from a variable:

```go
const exporterPositionQuery = `SELECT "PARTITION_ID", "LAST_UPDATED" FROM "EXPORTER_POSITION"`
```

Verify the exact table name and column names against Camunda 8.9 with the `camunda-docs` MCP server before you write the query. Record the answer in the PR body. A table that does not exist is a `DatabaseNotRestored` hold, not a transient error: an empty database is exactly the state the check must catch.

- [ ] **Step 5: Run, confirm they pass, commit**

```bash
git add internal
git commit -m "feat(pointintimerestore): refuse a database that is ahead of the requested point (#112)"
```

### Task 3: `RestoringPrimaryStorage` and completion

- [ ] **Step 1: Write the failing envtest specs**

The same set PR2's Task 6 covers, with two differences:

- the Jobs carry `--to=<spec.timestamp>` formatted as RFC 3339, and `camunda.io/point-in-time-restore: <name>`
- there is no recorded backup size, so the claims always take the size of the StatefulSet's claim template

Add one spec that proves the arguments: `Args` is exactly `[]string{"--to=2026-07-30T14:30:00Z"}` for `spec.timestamp: "2026-07-30T14:30:00Z"`.

- [ ] **Step 2: Run, confirm they fail, write `primary.go`, run again**

`primary.go` is a thin wrapper over `pkg/restore`. If it turns out to be nearly identical to PR2's `primary.go`, move the shared body into `pkg/restore` as a small step function and let both controllers call it. That is the point of the shared package. Do not copy the body.

- [ ] **Step 3: Update the CRD doc page**

Remove the "Not implemented yet" warning. Correct anything the code found. Load `simple-english:simple-english` first.

- [ ] **Step 4: Verify the whole PR**

```bash
make manifests generate
git status --porcelain config api   # must be empty
go test ./...
make all
mkdocs build --strict
```

- [ ] **Step 5: Commit and open the PR**

```bash
git add -A
git commit -m "feat(pointintimerestore): align primary storage with the restored database (#112)"
```

Open the PR with `Towards #112`. Run the review loop until clean. **Stop. Request the user's review. Do not merge.**

### Review checkpoint after PR2 and PR3

Run `feature-dev-workflow:reviewing-feature-progress` over the feature branch once both merge. The two controllers must read as one author's work: same reconciler shape, same `Options` fields, same hold-and-grace vocabulary, same phase-is-the-resume-marker rule. Fix any drift before PR4 starts, in a small follow-up commit on the feature branch.

---

## PR4 — The restore e2e suite (#113, branch `test/restore-e2e`, worktree `.claude/worktrees/restore-controllers--e2e`)

Implements the spec's "e2e scope" decision.

**Files:**
- Create: `test/e2e/restore_test.go`
- Modify: `test/e2e/camundacluster_test.go` (register the restore specs, generalise the `psql` helper, add a suspend and unsuspend helper)
- Modify: `test/e2e/database_test.go` (move `psql` into `helpers_test.go` and give it a namespace parameter)
- Modify: `test/e2e/helpers_test.go`

The suite registers the restore specs from the existing `Ordered` containers, exactly as `backup_test.go` registers the backup specs, so the restore runs against a cluster that is already healthy and already holds a process instance. It stands up no new cluster. The whole suite shares one 60 minute budget.

### Task 1: Suspend, unsuspend, and the wipe helpers

- [ ] **Step 1: Write the helpers**

In `helpers_test.go`:

```go
// suspend sets spec.suspend on the cluster and waits until every workload is
// scaled to zero.
func suspend(cluster *v1.CamundaCluster)

// unsuspend clears spec.suspend and waits until the cluster reports Ready
// with reason Healthy again.
func unsuspend(cluster *v1.CamundaCluster)

// brokerClaims returns the broker data claims of a cluster, keyed by name.
func brokerClaims(cluster *v1.CamundaCluster) map[string]corev1.PersistentVolumeClaim
```

`suspend` reuses `expectScaledToZero`, which `camundacluster_test.go` already has. Move `psql` from `database_test.go` into `helpers_test.go` and give it a `namespace` parameter, because the restore specs run against `camunda-rdbms-e2e` and not `database-e2e`. Update the existing call sites.

- [ ] **Step 2: Run the existing suite and confirm nothing regressed**

Run: `make test-e2e` and focus the existing Database container.
```bash
KIND_CLUSTER=camunda-operator-test-e2e go test -tags=e2e ./test/e2e/ -v -ginkgo.v \
  -ginkgo.focus 'Database' -timeout 30m
```
Expected: PASS.

- [ ] **Step 3: Commit**

```bash
git add test/e2e
git commit -m "test(e2e): share the suspend and SQL helpers across namespaces (#113)"
```

### Task 2: The Elasticsearch round trip

- [ ] **Step 1: Write the spec function**

Create `test/e2e/restore_test.go` with:

```go
// itRestoresTheElasticsearchCluster registers the LogicalRestore round trip
// of an Elasticsearch-backed cluster. It takes its own backup, so it never
// depends on the ordering of the backup specs.
func itRestoresTheElasticsearchCluster(cluster *v1.CamundaCluster, elasticsearch, storageConfig string)
```

Ordered specs inside it:

1. *takes the backup the restore will read* — applies `LogicalBackupElasticsearch` named `camunda-es-restore-source`, waits `Ready/Completed` within `backupTimeout`, and records `status.backupId`.
2. *suspends the cluster* — `suspend(cluster)`.
3. *wipes secondary storage* — deletes every Camunda index through `curlElasticsearch(contract, "wipe-indices", "/<patterns>?ignore_unavailable=true", "-XDELETE")`, using the same patterns `logicalbackup.CamundaIndexPatterns` produces.
4. *restores the backup* — applies `LogicalRestore{BackupRef:{Kind: LogicalBackupElasticsearch, Name: "camunda-es-restore-source"}, TargetClusterRef:{Name: cluster.Name}}` named `camunda-es-restore`, waits `Ready/Completed` within `restoreTimeout`, and asserts `status.phase == v1.LogicalRestoreCompleted`, a non-empty `status.restoredSnapshots`, and one entry in `status.primaryJobNames` per broker.
5. *unsuspends the cluster and finds the seeded instance again* — `unsuspend(cluster)` then `expectInstanceSearchable(cluster)`.

Step 3 destroys secondary storage. The restore itself destroys primary storage, because it deletes and recreates the broker data volumes. So step 5 passing proves that both halves came back from the backup and not from surviving state. Add an assertion between steps 3 and 4 that a search for the process instance returns no item, so a wipe that silently did nothing cannot make the test pass.

- [ ] **Step 2: Register it**

In `camundacluster_test.go`, inside `Describe("CamundaCluster", Ordered)`, call `itRestoresTheElasticsearchCluster(cluster, esName, storageConfig)` **after** `itBacksUpTheElasticsearchCluster(...)`. The backup specs end by deleting their own backup and its snapshots, and the restore takes its own backup under another name, so the two do not collide.

- [ ] **Step 3: Run it**

```bash
KIND_CLUSTER=camunda-operator-test-e2e go test -tags=e2e ./test/e2e/ -v -ginkgo.v \
  -ginkgo.focus 'CamundaCluster$' -timeout 60m
```
Expected: PASS. If the run is killed locally for time, focus the container and trust CI, per the repository's e2e conventions.

- [ ] **Step 4: Commit**

```bash
git add test/e2e
git commit -m "test(e2e): prove an Elasticsearch backup restores end to end (#113)"
```

### Task 3: The relational round trip

- [ ] **Step 1: Write the spec function**

```go
// itRestoresTheRelationalCluster registers the LogicalRestore round trip of
// a relational cluster.
func itRestoresTheRelationalCluster(cluster *v1.CamundaCluster)
```

Ordered specs:

1. *takes the backup the restore will read* — applies `LogicalBackupRDBMS` named `camunda-rdbms-restore-source`, waits `Ready/Completed`, records `status.objectKey` and `status.zeebeBackupId`.
2. *suspends the cluster* — `suspend(cluster)`.
3. *wipes the logical database* — through `psql(rdbmsNamespace, "wipe", adminRef, "DROP SCHEMA public CASCADE; CREATE SCHEMA public;")` as the server administrator, then re-grants the application user with `GRANT ALL ON SCHEMA public TO camunda;`. The restore Job runs `pg_restore --clean --if-exists`, which recreates every object the dump holds.
4. *restores the backup* — applies `LogicalRestore{BackupRef:{Kind: LogicalBackupRDBMS, ...}}`, waits `Ready/Completed`, asserts a non-empty `status.secondaryJobName` and one entry in `status.primaryJobNames` per broker.
5. *unsuspends the cluster and finds the seeded instance again* — `unsuspend(cluster)` then `expectInstanceSearchable(cluster)`.

Add the same "the wipe really wiped" assertion between steps 3 and 4: a `psql` count over the process-instance table returns zero, or the table does not exist.

- [ ] **Step 2: Register it in `Describe("CamundaCluster on RDBMS", Ordered)`, after `itBacksUpTheRelationalCluster(cluster)`**

- [ ] **Step 3: Run and commit**

```bash
git add test/e2e
git commit -m "test(e2e): prove a relational backup restores end to end (#113)"
```

### Task 4: The PointInTimeRestore specs

The database is never rolled back in kind. The suite treats the live database as "already restored", which is exactly what the spec's e2e decision allows.

- [ ] **Step 1: Enable the capability on the relational fixture**

In `camundacluster_test.go`, give the RDBMS container's `DatabaseServerConfig` a `pitr` block:

```go
	PITR: &v1.PITRCapability{Enabled: true, RetentionPeriodDays: new(int32(7))},
```

Confirm that exactly one `Database` references that server. The `DatabaseServerConfig` is cluster-scoped, so it must carry a name that `database_test.go` does not use.

- [ ] **Step 2: Write the refusal spec**

```go
// itRefusesAPointInTimeRestoreOfAnUnrestoredDatabase registers the
// non-destructive refusal of a timestamp that lies before the database.
func itRefusesAPointInTimeRestoreOfAnUnrestoredDatabase(cluster *v1.CamundaCluster)
```

1. *records the broker volumes* — `brokerClaims(cluster)`, keeping every `creationTimestamp`.
2. *refuses a timestamp before the database state* — applies `PointInTimeRestore{ClusterRef:{Name: cluster.Name}, Timestamp: metav1.NewTime(time.Now().Add(-time.Hour))}`, then `Consistently` for 30 seconds asserts `Ready=False` with reason `DatabaseNotRestored` and `status.phase == v1.PointInTimeRestorePending`.
3. *left the broker volumes untouched* — `brokerClaims(cluster)` again, and every `creationTimestamp` equals the recorded one. Also assert that no Job carries `camunda.io/point-in-time-restore`.

The refusal spec is the one that protects the whole design decision. It runs before the accepting spec, so a bug that deletes volumes early is caught before anything else can hide it.

- [ ] **Step 3: Write the accepting spec**

```go
// itRunsAPointInTimeRestoreAtTheCurrentDatabaseState registers the
// operator-side path of a valid point-in-time restore.
func itRunsAPointInTimeRestoreAtTheCurrentDatabaseState(cluster *v1.CamundaCluster)
```

1. *accepts a timestamp at the current database state* — applies a second `PointInTimeRestore` with `Timestamp: metav1.Now()`, and waits until `status.phase` leaves `ValidatingDatabaseState`. That is the proof that the pre-check passed.
2. *runs one restore Job per broker* — lists Jobs by `camunda.io/point-in-time-restore` and asserts one per broker, each with `--to=` in its container arguments.
3. *converges* — waits for `Ready` with reason `Completed` within `restoreTimeout`, then `unsuspend(cluster)` and `expectInstanceSearchable(cluster)`.

Spec 3 depends on Zeebe's continuous primary-storage backups holding a checkpoint at or before the timestamp. The `CamundaCluster` controller enables continuous mode for every relational cluster with a `backupStorageRef`, and the container has already taken a Zeebe backup through `itBacksUpTheRelationalCluster`. If the restore application still finds no checkpoint in CI, keep specs 1 and 2 and turn spec 3 into an assertion that the restore reaches a terminal phase with a message that names the missing checkpoint. Do not weaken specs 1 and 2. Record the choice in the PR body. This is listed under Open questions.

- [ ] **Step 4: Register both in `Describe("CamundaCluster on RDBMS", Ordered)`, after `itRestoresTheRelationalCluster(cluster)`**

Both must run last in that container. They suspend the cluster and rewrite its primary storage, and nothing after them can assume the old state.

- [ ] **Step 5: Run, verify, commit, and open the PR**

```bash
KIND_CLUSTER=camunda-operator-test-e2e go test -tags=e2e ./test/e2e/ -v -ginkgo.v \
  -ginkgo.focus 'CamundaCluster on RDBMS' -timeout 60m
go test ./...
make all
```

```bash
git add test/e2e
git commit -m "test(e2e): prove the point-in-time restore refuses an unrestored database (#113)"
```

Open the PR with `Towards #113`. Run the review loop until clean. Self-merge into `feat/restore-controllers` and close #113.

---

## Integration PR — `feat/restore-controllers` into `main` — the user reviews

- [ ] Run the `feature-dev-workflow:reviewing-feature-progress` checkpoint over the merged feature branch
- [ ] Confirm `go test ./...`, `make all`, `make manifests generate` with no diff, and `mkdocs build --strict`
- [ ] Confirm the full e2e suite passes in CI, not only the focused containers
- [ ] Open the integration PR with `Closes #109`, run the review loop until clean, **stop, and request the user's review**
- [ ] After the user approves, merge, then delete the plan and the state file in the last commit and update memory

## Verification commands

Run these at every task boundary, and all of them before every PR opens.

```bash
go test ./...                       # both modules, through the Makefile MODULES loop
make all                            # lint and format, both modules
make manifests generate             # must leave the tree clean
git status --porcelain config api   # must print nothing
mkdocs build --strict               # the docs must build with no warning
```

The e2e suite is not part of that loop. It runs with:

```bash
make test-e2e                       # creates and deletes its own kind cluster
```

## Resolved questions

The orchestrator resolved these on 2026-08-20 with the user AFK and full autonomy granted. Each resolution takes the plan's own safe direction. Raise a bubble-up entry if implementation contradicts one.

- **Cross-cluster Elasticsearch restore requires the same backup bucket.** Confirmed. `docs/crds/logicalrestore.md` already states that the target's `spec.backupStorageRef` must point at a bucket that contains the source backup's artifacts. A target with a different bucket is out of scope for this epic.
- **A backup with no recorded version fails with `IncompatibleTarget`.** Confirmed. This is a clean-slate project, so the restore fails closed instead of warning.
- **The accepting PointInTimeRestore e2e spec uses the fallback assertion** when no checkpoint exists at or before the timestamp. Confirmed. The e2e stays short.
- **`DROP SCHEMA public CASCADE` is the relational wipe, and the spec re-grants the application user by hand.** Confirmed.
