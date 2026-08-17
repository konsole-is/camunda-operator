# Backup Controllers Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use feature-dev-workflow:developing-a-feature to drive this plan PR by PR on the feature branch, dispatching per-PR workers via feature-dev-workflow:fanning-out-with-worktrees where the graph fans out. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Ship the backup epic (#64): `LogicalBackupElasticsearch`, `LogicalBackupRDBMS`, `BackupSchedule`, the storage wiring, and the MinIO e2e suite.

**Architecture:** Seven sub-PRs on the long-lived `feat/backup-controllers` branch, each a real GitHub PR targeting the feature branch; a final integration PR to `main`. Ownership follows produce/consume contracts (management binding, `snapshotRepository`, the bucket contract); external protocols live in pure packages outside ocf.

**Tech Stack:** Go, kubebuilder/controller-runtime, ocf v0.19.1, gocloud.dev/blob, pgx (existing), ECK v3.5, Ginkgo/Gomega (controller tests), testify (unit), kind + MinIO (e2e).

**Spec:** `docs/superpowers/specs/2026-08-16-backup-controllers-design.md` — the design authority. Every task below implements a spec section; read the spec section before the task.

**Tracking:** Epic #64. Sub-issues: PR1 #65, PR2 #66, PR3 #67, PR4 #68, PR5 #69, PR6 #70, PR7 #71.

## Global Constraints

- Camunda 8.9 only; every config key verified per `verifying-camunda-app-config` (docs MCP / source), never from memory.
- CLAUDE.md rules: SSA for managed resources, status once per reconcile via ocf `FlushStatus`, Ginkgo top-level + testify low-level, no `t.Fatal`, GoDoc on every exported symbol, docs updated in the same PR as the code.
- Load `how-we-write-go` before writing Go; `simple-english` before prose; `ocf:*` skills per their table.
- `make all`, `go test ./...`, and `make helm-verify` must pass at every PR head.
- Commits reference the sub-issue (`feat(backup): ... (#68)`); sub-PR bodies use `Towards #<sub-issue>`; the integration PR uses `Closes #64`.
- Never alter `main`; everything lands via PRs.

## Merge policy

Every PR runs the review loop (`feature-dev-workflow:copilot-review-loop`) until clean — no round cap; keep going while findings remain. PRs 1, 2, 3, 6, 7 are then self-merged into the feature branch. **PRs 4 and 5 and the integration PR stop after the loop is clean and wait for the user's own review; do not merge them without it.**

## PR graph

```
PR1 foundation ── PR2 ES side ──┐
              └── PR3 CC wiring ─┼── PR4 LBES (user review) ─┐
                                 ├── PR5 LBRDBMS (user review) ─┼── PR6 schedule ── PR7 e2e+docs ── integration PR (user review)
```

PR2 and PR3 fan out after PR1. PR4 and PR5 fan out after PR2+PR3. PR6 needs the types from PR4+PR5. PR7 closes.

## Contracts

| Name | Producer | Consumer | Shape | Realization |
| --- | --- | --- | --- | --- |
| `bucket-contract` | #65 | #66, #67, #69 | `ObjectStorageConfigSpec`: `Type` + `S3/GCS/AzureBlob` blocks, `auth.type: workloadIdentity\|credentials` (spec §ObjectStorageConfig) | merged in PR1 |
| `objectstore-api` | #65 | #69 | `objectstore.Open(ctx, cfg *v1.ObjectStorageConfig, creds *Credentials) (*Bucket, error)`; `Bucket.Upload(ctx, key string, r io.Reader) error` (aborts the write on a partial read — no truncated object is ever committed), `Delete(ctx, key) error`, **`Walk(ctx, prefix string, fn func(key string) error) error`** (streaming; the unbounded `List` was removed so a large bucket cannot exhaust the manager), `Close()`. Secret→credential mapping is `objectstore.CredentialsFrom(cfg, data)`; `api/v1` carries `BasePath()` and `CredentialsSecret()` (api/v1 must not import objectstore — the dependency runs the other way) | merged in PR1 |
| `esadmin-api` | #65 | #66, #68 | **`esadmin.New(endpoint string, user, pass string, ca []byte) (*Client, error)`** (errors on an unusable CA bundle instead of silently building an empty trust pool); `EnsureSnapshotRepository(ctx, name string, cfg S3RepositoryConfig) error`, `CreateSnapshot(ctx, repo, name string, indices []string) error`, `SnapshotStatus(ctx, repo, name) (SnapshotState, error)`, `DeleteSnapshot(ctx, repo, name) error`, `DeleteSnapshotsByPrefix` is FORBIDDEN (finalizer deletes by exact name), `MaxNodeFSTotalAndUsedBytes(ctx) (total, used int64, err error)` | merged in PR1 |
| `camundaadmin-api` | #65 | #68, #69 | `camundaadmin.New(binding Binding) (*Client, error)` where `Binding{Endpoint, Version string; Auth Auth}`; the seven methods of spec §pkg/camundaadmin. Sentinels are `ErrUnreachable`, `ErrRejected`, **`ErrConflict`**. Verified 8.9 protocol facts baked into the client — do not "simplify" them away: `/actuator/exporting/pause|resume` always answer **HTTP 200** with the real outcome in the body's `status` (204 success, 500 failure, and a 500 can mean a PARTIAL pause); `POST /actuator/backupHistory` answers **200** (not 202) and **400** when the id exists; `POST /actuator/backupRuntime` answers 202 and **409 for same-or-higher id**, surfaced as `ErrConflict` with NO client-side adoption because the caller alone knows whether it is re-entering its own backup. A supplied id is authoritative; the response id is authoritative only when none was supplied (the RDBMS path) | merged in PR1 |
| `management-binding` | #67 | #68, #69 | **AS SHIPPED in #77** — `v1.ManagementBinding{Endpoint string; Auth ManagementAuth{Method string; CredentialsSecretRef *CredentialsSecretRef}; Version string; Partitions int32; BackupRepository string}` at `CamundaCluster.status.management`; nil while the cluster is SUSPENDED (workloads scaled to zero, endpoint genuinely unreachable). It deliberately persists through other not-ready states (`InvalidReference`, `MissingSecret`, ...) because the workloads keep running and the management endpoint stays reachable; consumers treat `camundaadmin.ErrUnreachable` as the authoritative reachability signal, never the cluster's Ready condition. `Auth.Method` is `none` on 8.9 (the management port is unauthenticated — PR1 verified `/actuator/**` installs no auth filter); the credentials ref exists for a user who fronts 9600 themselves, and it is a `CredentialsSecretRef` because `camundaadmin.Auth` needs a username AND a password (no bare `SecretRef` type exists in api/v1). Consumers must treat `none` as the normal case, not an error | merged in PR3 |
| `snapshot-repository-field` | #66 | #67 | `SecondaryStorageConfig.spec.elasticsearch.snapshotRepository string` (optional) | merged in PR2 |
| `logicalbackup-skeleton` | #65 | #68, #69, #70 | **AS SHIPPED in #74** — status vocabulary in `api/v1`: `v1.LogicalBackupPhase` (`LogicalBackupPending/Running/Completed/Failed`), shared `v1.Reason*` constants, `v1.ClusterRef`, `v1.LogicalBackupStorageSizes{Elasticsearch, Zeebe *resource.Quantity}`. Logic in `pkg/logicalbackup`: `PreCheck(ctx, PreCheckRequest{Reader client.Reader, Ref v1.ClusterRef, Namespace string, StorageType v1.SecondaryStorageType, InProgress func(ctx) (string, error)}) (*PreCheckResult, error)` returning `*conditions.PreCheckFailure`, `Waiting(err) bool`, `AllocateBackupID`, `ObjectKeyPrefix`, `Finalizer`, `ZeebeSize`/`ElasticsearchSize`/`RecordStorageSizes`. Schedule label is `labels.BackupScheduleKey` (NOT `logicalbackup.ScheduleLabel` — removed). Per-kind reasons are NOT here: PR4 declares `ResumeFailed`, PR5 declares `MissingCredentials`. | merged in PR1 |
| `backup-kind-types` | #68, #69 | #70 | `v1.LogicalBackupElasticsearch` / `v1.LogicalBackupRDBMS` full types incl. `Status.Phase`, `Status.CompletionTime`; plurals `logicalbackupelasticsearches`/`logicalbackuprdbmses`, short names `lbes`/`lbrdbms` | PR6 branches after PR4+PR5 merge |

The `logicalbackup-skeleton` row overrides the sketch in #65's issue text: the skeleton ships **complete in PR1** (pure functions, table-tested), so PR4/PR5 import a merged real package and no stub choreography is needed. Its row above records the shape as actually shipped and reviewed in PR #74 — three deviations from the original sketch were accepted (status vocabulary belongs in `api/v1` because it is CRD status surface needing deepcopy and enum markers; `PreCheck` takes a request struct with an injected `InProgress` because the backup kinds do not exist until PR4/PR5, so only a kind can list itself; per-kind reasons live next to their kind). **`InProgress` must list BOTH backup kinds** — spec §Shared skeleton serializes on "another backup of either kind", and that half is the caller's responsibility, so PR4 and PR5 each implement and test it.

## Conventions

- **Naming firewall:** PR numbers and "PR N" labels never appear in code, fixtures, or test names.
- **Kinds:** `LogicalBackupElasticsearch`, `LogicalBackupRDBMS` everywhere — never `ESBackup`, `RDBMSBackup`, `Backup`.
- **Packages:** `pkg/camundaadmin`, `pkg/esadmin`, `pkg/objectstore`, `pkg/logicalbackup`, `pkg/components/logicalbackuprdbms` (Job builder), controllers under `internal/controller/{logicalbackupelasticsearch,logicalbackuprdbms,backupschedule}` — one directory per CRD, matching Batch B/C layout.
- **Labels:** owner keys via `pkg/labels` — `labels.LogicalBackupElasticsearchKey/LogicalBackupRDBMSKey/BackupScheduleKey`, shipped in PR1. Schedule linkage uses `labels.BackupScheduleKey`; `pkg/labels` is the single owner of label strings, so no package declares its own copy.
- **Field managers:** ocf defaults for components; `camunda-operator/backup` for the dump Job SSA.
- **Conditions:** `conditions.Aggregate` derives `Ready`; per-kind reasons live next to the kind's types; shared reasons come from `api/v1/conditions.go` — never redeclare.
- **Job helper images come from ONE flag, one CLI:** every Job the operator runs helper code in (dump upload today; ES helpers later) uses the `camunda-operator-cli` image, named by the manager flag `--camunda-operator-cli-image` (env `CAMUNDA_OPERATOR_CLI_IMAGE`), chart-defaulted next to `manager.image`. Never resolve the operator's own image at runtime; never put subcommands in the manager binary. (User decision, review of #85.)
- **Facts about external systems are observed, not declared:** a server's version is probed by the controller that already holds a connection and published in status; it is not a spec field a human or an upstream producer must keep true. (User decision, review of #85.)
- **Exclude, never gate, possibly-foreign resources:** an ocf resource that may pre-exist and be user-owned (a `create: false` ServiceAccount, an adopted Secret) is EXCLUDED from the component (`IncludeWhen`) when unmanaged — a gated-off resource is a deletion target, and the operator must never delete what it does not own. Established red-first in #77/#80; applies to every PR4/PR5 resource with a user-supplied name.
- **Preset legality of `serviceAccount`:** an ElasticsearchClusterPreset MAY carry `serviceAccount` (a fleet-standard account name/identity is a legitimate baseline); a CamundaClusterPreset MUST NOT (CEL-enforced — the CamundaCluster account carries per-cluster bucket identity). Decided at the Phase 2→3 checkpoint; the asymmetry is deliberate.
- **Suspension pre-check stance differs by CR, deliberately:** a suspended ElasticsearchCluster skips bucket/SA pre-checks (its workloads are deleted; nothing consumes them). A suspended CamundaCluster still resolves buckets and mirrors credentials — it keeps applying workload specs at zero replicas, and those specs reference the mirrored Secret. Both recorded; do not "align" one to the other.
- **Errors:** clients return wrapped sentinel errors (`camundaadmin.ErrUnreachable`, `ErrRejected`); controllers map them to `ConnectionFailed` / `Failed`. Message prefixes follow the ocf 0.19 Aggregate conventions.
- **Tests:** golden snapshots under `pkg/components/<pkg>/testdata/golden/<case>`; envtest suites per controller dir via `internal/testenv`; fake HTTP servers live in the client packages as exported test helpers (`camundaadmin/camundaadmintest`, `esadmin/esadmintest`) so controller tests reuse them.
- **Docs:** the PR that changes a CRD's schema owns that CRD's `docs/crds/` page and rewrites it in the same PR (CLAUDE.md rule). PR7 owns only the backup-page split/removal, the cross-page prose sweep, and `mkdocs.yml` nav. A page must never contradict its own shipped CRD on the feature branch.
- **CEL and gofmt:** `make fmt` normalizes `''` into typographic quotes inside **declaration** doc comments, silently corrupting `+kubebuilder:validation:XValidation` rules (committed CRD YAML keeps the old rule, so tests stay green). Prefer `size() > 0` and quote-free CEL on declaration comments; after adding CEL, run `make manifests` and confirm the regenerated YAML still holds the rule you wrote. Field-level markers are unaffected.
- **YAML in docs/goldens:** field names exactly as in the spec API sections; no invented synonyms.

---

### PR1 — Foundation (#65, branch `feat/backup-controllers--foundation`)

Implements spec §ObjectStorageConfig (API + design decision), §pkg/camundaadmin, the client/package layer, the scaffold removals, and the label keys.

**Files:**
- Rewrite: `api/v1/objectstorageconfig_types.go` (+ regenerate `zz_generated.deepcopy.go`, CRD base)
- Create: `pkg/objectstore/{objectstore.go,objectstore_test.go}`
- Create: `pkg/esadmin/{client.go,client_test.go,esadmintest/server.go}`
- Create: `pkg/camundaadmin/{client.go,client_test.go,camundaadmintest/server.go}`
- Create: `pkg/logicalbackup/{phase.go,precheck.go,keys.go,sizes.go}` + tests (complete, per Contracts)
- Create: `api/v1/logicalbackup_shared.go` (`ClusterRef`, shared reason constants)
- Modify: `pkg/labels/labels.go` (+3 owner keys), `internal/controller/objectstorageconfig/controller.go` (`MissingSecret`)
- Delete: `api/v1/backup_types.go`, `api/v1/backupretention_types.go`, `internal/controller/backup*`, `internal/controller/backupretention*`, their CRD bases, 6 RBAC roles, 2 samples, `PROJECT` entries, `cmd/main.go` wiring, `docs/crds/{backup,backupretention}.md` (references updated where compilation/docs-build requires; the full doc pass is PR7)
- Modify: `go.mod` (gocloud.dev)

**Steps:**
- [ ] Verify management-port auth on Camunda 8.9 (docs MCP + `CAMUNDA_SOURCE_DIR` scan); record the answer in the PR body and shape `camundaadmin.Auth` accordingly
- [ ] TDD the `ObjectStorageConfig` types + CEL (schema tests first: block/type match both levels, S3 region rule, credentials shapes)
- [ ] TDD `pkg/objectstore` against `fileblob`
- [ ] TDD `pkg/esadmin` and `pkg/camundaadmin` against `httptest` (idempotent branches, both error classes, unknown-version constructor error)
- [ ] TDD `pkg/logicalbackup` (pre-check table, ID allocation, key layout, size rule)
- [ ] Remove the scaffold kinds; `make manifests generate && make all && make helm-verify`
- [ ] Validation controller `MissingSecret` (envtest)
- [ ] Open PR (`Towards #65`), review-loop to clean, self-merge, close #65

### PR2 — ElasticsearchCluster side (#66, branch `feat/backup-controllers--es-snapshot-repository`)

Implements spec §ElasticsearchCluster (API), §ElasticsearchCluster owns the Elasticsearch side, §SecondaryStorageConfig, and the SA `name`/`create` on the shared type.

**Files:**
- Modify: `api/v1/elasticsearchcluster_types.go` (`snapshotStorageRef`, `secureSettings`, `ServiceAccountSpec.name/create`), `api/v1/secondarystorageconfig_types.go` (`snapshotRepository`)
- Modify: `pkg/wrappers/eckelasticsearch/` (secureSettings passthrough), `pkg/components/elasticsearchcluster/` (keystore Secret component, derived SA annotation, role narrowing, contract publication)
- Modify: `internal/controller/elasticsearchcluster/` (repository registration component via `pkg/esadmin`, `SnapshotRepositoryReady`, bucket + Secret watches)

**Steps:**
- [ ] TDD types + schema (SA `create:false` semantics, secureSettings shape)
- [ ] TDD annotation derivation per storage type (pure, in the components package)
- [ ] TDD keystore Secret + secureSettings render (goldens)
- [ ] Elasticsearch `gcs`/`azure` **repository** settings are Elastic's surface, not Camunda's (PR1 verified only the Camunda side and deliberately did not claim them) — verify them against Elasticsearch/ECK documentation before rendering, the same way the `s3` repository settings are verified
- [ ] Narrow the `camunda` role (snapshot perms on the repo + `monitor`); prove in e2e later, envtest now asserts the rendered role string
- [ ] envtest: registration idempotence, `SnapshotRepositoryReady`, published `snapshotRepository`, `InvalidReference` on missing pre-existing SA
- [ ] Update `docs/crds/{elasticsearchcluster,secondarystorageconfig}.md`
- [ ] Open PR (`Towards #66`), review-loop to clean, self-merge, close #66

### PR3 — CamundaCluster wiring (#67, branch `feat/backup-controllers--cluster-backup-wiring`)

Implements spec §CamundaCluster and CamundaClusterPreset (API), §Backup policy on the cluster, §CamundaCluster wiring, §management binding, and the derived annotations on the CC side.

**Files:**
- Modify: `api/v1/camundacluster_types.go` (`spec.backup`, `status.management`, SA fields via shared type)
- Modify: `pkg/components/camundacluster/` (presetmerge for `backup`, backup-store env render per path, repository name from contract, SA name/create + derived annotation, checksum-workaround env)
- Modify: `internal/controller/camundacluster/` (binding publication, precheck: missing `snapshotRepository`, two-identity rejection, bucket Secret mirror + watches)

**Steps:**
- [ ] Verify every rendered key with the camunda-docs MCP before writing it into `pkg/camundaconfig` (backup store `s3/gcs/azure` blocks, `repository-name`, continuous/schedule/checkpoint/retention)
- [ ] **Three GCS/Azure mapping rules verified in PR1 against `camunda/camunda` 8.9.16 — do NOT re-infer them from field names:** (a) Azure's `base-path` **is the container name** (the azure block has no container field; `AzureBackupConfig.containerName` maps to `basePath`), so render `azure.base-path: <container>` and never concatenate our `basePath` into it; (b) Azure's `endpoint` is effectively required without a connection string — derive `https://<accountName>.blob.core.windows.net` when unset; (c) GCS accepts **no key as configuration** (`gcs.auth` is only `auto | none`; credentials resolve through `GoogleCredentials.getApplicationDefault()`), so a static GCS key is mounted as a file with `GOOGLE_APPLICATION_CREDENTIALS` set and `auth: auto` — never rendered as a property
- [ ] TDD presetmerge for `backup` (incl. `continuous` `*bool` three-state)
- [ ] TDD render goldens: ES-path and RDBMS-path backup variants (backup store env, repository name, `WHEN_REQUIRED`, static keys vs none)
- [ ] TDD binding publication + clear-on-suspend (envtest), precheck rejections
- [ ] Update `docs/crds/{camundacluster,camundaclusterpreset}.md` (`objectstorageconfig.md` is owned by PR1, which changed its schema)
- [ ] Open PR (`Towards #67`), review-loop to clean, self-merge, close #67

### PR4 — LogicalBackupElasticsearch (#68, branch `feat/backup-controllers--lbes-controller`) — **user reviews before merge**

Implements spec §LogicalBackupElasticsearch (API + state machine + finalizer + storage sizes).

**Files:**
- Create: `api/v1/logicalbackupelasticsearch_types.go` (plural + `lbes` short name markers)
- Create: `internal/controller/logicalbackupelasticsearch/{controller.go,statemachine.go,finalizer.go,controller_test.go,schema_test.go,suite_test.go}`
- Modify: `cmd/main.go`, RBAC markers, `config/samples/`

**Steps:**
- [ ] TDD types + schema (immutable `clusterRef`, step enum, per-part status; reuse `v1.LogicalBackupPhase` and `v1.LogicalBackupStorageSizes` from PR1 — do not redeclare). Declare `ResumeFailed` here, next to this kind
- [ ] Implement `PreCheckRequest.InProgress` listing **both** backup kinds (spec serializes on "another backup of either kind") and test the cross-kind case
- [ ] TDD the state machine against `camundaadmintest`/`esadmintest`: happy path; crash re-entry per step (no duplicate POST); failure-in-each-step → `ResumeExporting` → terminal; resume deadline → `ResumeFailed`; `BackupInProgress` serialization; pre-check reasons; empty binding holds `Pending`
- [ ] TDD `RecordStorageSizes` wiring (cluster volumes + fake `_nodes/stats`)
- [ ] TDD finalizer (delete by exact backupId; release when cluster gone)
- [ ] Sample + CRD doc stub (full page in PR7)
- [ ] Open PR (`Towards #68`), review-loop to clean, **stop: request user review**; after approval merge and close #68

### PR5 — LogicalBackupRDBMS (#69, branch `feat/backup-controllers--lbrdbms-controller`) — **user reviews before merge**

Implements spec §LogicalBackupRDBMS (API + state machine + Job + upload subcommand + finalizer).

**Files:**
- Create: `api/v1/logicalbackuprdbms_types.go` (plural + `lbrdbms` short name markers)
- Create: `pkg/components/logicalbackuprdbms/{job.go,job_test.go,testdata/golden/...}`
- Create: `cmd/upload/upload.go` (subcommand: env-driven, streams file → `pkg/objectstore`)
- Create: `internal/controller/logicalbackuprdbms/{controller.go,finalizer.go,controller_test.go,schema_test.go,suite_test.go}`
- Modify: `cmd/main.go` (controller + subcommand dispatch), RBAC (Jobs), samples

**Steps:**
- [ ] TDD types + schema (immutable `clusterRef`, `dump` whole-block override; reuse `v1.LogicalBackupPhase` / `v1.LogicalBackupStorageSizes`). Declare `MissingCredentials` here, next to this kind
- [ ] Implement `PreCheckRequest.InProgress` listing **both** backup kinds and test the cross-kind case
- [ ] TDD the Job builder goldens: initContainer `postgres:<major>` w/ `pg_dump -Fc`, upload main container, both auth shapes, scratch `emptyDir`/PVC, effective dump block (scheduling/resources/annotations), cluster SA
- [ ] TDD `upload` subcommand against `fileblob`
- [ ] TDD controller: Job SSA + tracking, `PrimaryBackup` via binding (records generated ID), `MissingSecret`/`MissingCredentials` pre-checks, `storageSizes.zeebe`
- [ ] TDD finalizer (running Job deleted first; dump object removed; primary-storage backups untouched)
- [ ] Sample + CRD doc stub
- [ ] Open PR (`Towards #69`), review-loop to clean, **stop: request user review**; after approval merge and close #69

### PR6 — BackupSchedule (#70, branch `feat/backup-controllers--backup-schedule`)

Implements spec §BackupSchedule (API + controller).

**Files:**
- Rewrite: `api/v1/backupschedule_types.go`
- Create: `internal/controller/backupschedule/{controller.go,retention.go,controller_test.go,schema_test.go,suite_test.go}` (replacing the scaffold file layout)
- Modify: `cmd/main.go`, RBAC, samples

**Steps:**
- [ ] TDD types + schema (cron validation, `retained` bounds/defaults)
- [ ] TDD trigger logic (next-from-lastScheduleTime, creation-time first trigger; kind selection by storage type; naming + labels; no owner ref)
- [ ] TDD skip paths (suspend, overlap) as events
- [ ] TDD retention (both bounds; manual + non-terminal untouched; phase-change-driven prune via label index; schedule deletion leaves backups)
- [ ] TDD the retention-window warning event (RDBMS path)
- [ ] Open PR (`Towards #70`), review-loop to clean, self-merge, close #70

### PR7 — e2e + docs (#71, branch `test/backup-controllers--e2e-minio`)

Implements spec §Testing (e2e) and §Doc deviations in full.

**Files:**
- Create: `test/utils/minio.go`, `test/e2e/testdata/minio.yaml`, `test/e2e/backup_test.go`
- Rewrite/create/remove doc pages per spec §Doc deviations; `mkdocs.yml`
- Modify: CI workflow if the suite needs a split job (spec §Risks)

**Steps:**
- [ ] MinIO utility + manifest (deployment, service, bucket bootstrap, credentials Secret)
- [ ] ES-path e2e (assertions per #71)
- [ ] RDBMS-path e2e (assertions per #71)
- [ ] Schedule e2e (one-minute cron, overlap skip)
- [ ] Artifact deletion e2e (finalizers against real stores)
- [ ] Full docs pass; `mkdocs build`
- [ ] Open PR (`Towards #71`), review-loop to clean, self-merge, close #71

### Integration PR — `feat/backup-controllers` → `main` — **user reviews**

- [ ] `feature-dev-workflow:reviewing-feature-progress` checkpoint over the merged feature branch
- [ ] Open the integration PR (`Closes #64`), review-loop to clean, **stop: request user review**; after approval the user (or I, on their word) merges
- [ ] Delete the plan + state file in the orchestrator's last commit; update memory
