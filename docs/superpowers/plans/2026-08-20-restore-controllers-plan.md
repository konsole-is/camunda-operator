# Restore Controllers Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use feature-dev-workflow:developing-a-feature to drive this plan PR by PR on the feature branch, dispatching per-PR workers via feature-dev-workflow:fanning-out-with-worktrees where the graph fans out. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Ship the restore epic (#109): the `LogicalRestoreElasticsearch`, `LogicalRestoreRDBMS`, and `PointInTimeRestore` APIs and controllers, plus an e2e round trip that proves a backup restores.

**Architecture:** Sub-PRs on the long-lived `feat/restore-controllers` branch, each a real GitHub PR that targets the feature branch, then one integration PR to `main`. `pkg/restore` is the shared machinery of all three restore kinds, in the role `pkg/logicalbackup` plays for the backup pair. Each controller owns its own admission detail and its own secondary-storage phase, and calls the shared driver for everything else. The restore Jobs copy their configuration from the live broker StatefulSet, so the restore application always runs with the configuration the brokers run with.

**Tech Stack:** Go, kubebuilder/controller-runtime, ocf v0.19.1, pgx v5 (through `pkg/pgbootstrap`), `pkg/esadmin` over `pkg/adminhttp`, Ginkgo/Gomega (controller tests through `internal/testenv`), testify (unit tests), kind + MinIO + ECK + PostgreSQL (e2e).

**Spec:** `docs/superpowers/specs/2026-08-20-restore-controllers-design.md` — the design authority, amended on 2026-08-20 with three decisions that this plan realizes. The detailed design record is `docs/crds/logicalrestoreelasticsearch.md`, `docs/crds/logicalrestorerdbms.md`, and `docs/crds/pointintimerestore.md`. Read the matching section before you start a task.

**Tracking:** Epic #109. Sub-issues: PR1 #110 (merged), PR3 #112 (merged), PR-B #111, PR-D #113. PR-A and PR-C need a new sub-issue each, filed under #109 before the work starts. Orchestration state: `docs/superpowers/states/2026-08-20-restore-controllers-state.md`.

## Amendment of 2026-08-20: the logical restore kind splits in two

The spec now carries three decisions that this plan did not have when it was written:

1. `LogicalRestore` becomes two kinds, `LogicalRestoreElasticsearch` and `LogicalRestoreRDBMS`. They mirror the `LogicalBackupElasticsearch` and `LogicalBackupRDBMS` pair.
2. `pkg/restore` widens from a renderer to the shared machinery of all three restore kinds.
3. Every restore kind takes the per-cluster claim of `pkg/clusterclaim` at admission and gives it back at the terminal transition.

The PR breakdown changes with them. The state of the old breakdown:

| Old PR | Issue | Outcome |
| --- | --- | --- |
| PR1 — API types and `pkg/restore` | #110 | **Merged** as PR #120. Its section below stays as the record of what shipped. |
| PR2 — the `LogicalRestore` controller | #111 | **Obsolete as written.** Its PR #123 is closed. Its code is raw material on the branch `feat/restore-controllers--logicalrestore`, head `14a1545`. |
| PR3 — the `PointInTimeRestore` controller | #112 | **Merged** as PR #122, commit `075746a`. PR-A refactors it onto the shared driver. |
| PR4 — the e2e suite | #113 | Not started. It now covers both new kinds. |

The new breakdown, all on the base branch `feat/restore-controllers`:

| New PR | Issue | What it ships |
| --- | --- | --- |
| PR-A | new sub-issue under #109 | The shared driver, `v1.RestoreProgress`, the cluster claim, and the removal of the dead `LogicalRestore` kind. |
| PR-B | #111 | `LogicalRestoreElasticsearch`. |
| PR-C | new sub-issue under #109 | `LogicalRestoreRDBMS`. |
| PR-D | #113 | The e2e round trip of both new kinds, plus the `PointInTimeRestore` specs. |
| PR-E | — | The integration PR to `main` with `Closes #109`. |

Issue #111 still carries the wording of the single kind. Reconcile its body to `LogicalRestoreElasticsearch` before PR-B starts.

## Global Constraints

- Camunda 8.9 only. Verify every configuration key and every Camunda behaviour with the `camunda-docs` MCP server or the Camunda source, per `verifying-camunda-app-config`. Never answer from memory.
- CLAUDE.md rules hold: server-side apply for every managed resource, one status write per reconcile through the ocf `FlushStatus`, Ginkgo at controller level and testify next to the file, no `t.Fatal`, GoDoc on every exported symbol, docs updated in the same PR as the code.
- Load `how-we-write-go` before you write Go. Load `simple-english:simple-english` before you write prose. Load the `ocf:*` skills per the CLAUDE.md table.
- The five verification gates of the "Verification commands" section pass at every PR head. `make all` is `build` alone in this repository. It does not lint.
- The restore controllers only read `CamundaCluster.spec.suspend`. They never write it. Suspend orchestration belongs to `camunda-cloud-operator`.
- The API does not expose Camunda's `--allow-version-mismatch`. The operator never restores a database server.
- Every new kind is scaffolded with `kubebuilder create api`. Never write the files by hand. Never edit `PROJECT` to add a kind. AGENTS.md holds this rule.
- Commits reference the sub-issue, for example `feat(logicalrestoreelasticsearch): validate the target before it deletes a volume (#111)`. Sub-PR bodies say `Towards #<sub-issue>`. The integration PR says `Closes #109`.
- Never push to `main`. Everything lands through PRs.

## Merge policy

Every PR runs `feature-dev-workflow:copilot-review-loop` until the loop returns nothing new. Set the review level to balanced at least once on each PR. The lite level found only nits on the earlier rounds.

PR-A and PR-D are then self-merged into the feature branch. **PR-B (#111) and PR-C stop after the loop is clean and wait for the user's own review. Do not merge them without it.** PR-E also waits for the user.

## PR graph

```
(merged) PR1 #110 API + pkg/restore
(merged) PR3 #112 PointInTimeRestore
                    │
                    ▼
   PR-A unify the restore machinery ──┬── PR-B #111 LogicalRestoreElasticsearch ──┐
                                      └── PR-C LogicalRestoreRDBMS ───────────────┴── PR-D #113 e2e ── PR-E integration (user review)
```

PR-B and PR-C fan out in parallel after PR-A merges. They touch disjoint files, so neither waits for the other. PR-D starts after both merge.

The one file that both PR-B and PR-C touch is `api/v1/zz_generated.deepcopy.go`, which is generated. Whichever merges second runs `make generate` again and takes the result.

## Contracts

| Name | Producer issue | Consumer issues | Shape | Realization |
| --- | --- | --- | --- | --- |
| `restore-api-types` | #110 | #111, #112, #113 | `v1.LogicalRestoreSpec{BackupRef LogicalBackupRef, TargetClusterRef ClusterRef}`, `v1.LogicalBackupRef{Kind LogicalBackupKind, Name string}`, `v1.LogicalRestorePhase` (`Pending`, `ValidatingCompatibility`, `RestoringSecondaryStorage`, `RestoringPrimaryStorage`, `Completed`, `Failed`), `v1.LogicalRestoreStatus` (fields in PR1 Task 1), `v1.PointInTimeRestoreSpec{ClusterRef ClusterRef, Timestamp metav1.Time}`, `v1.PointInTimeRestorePhase` (`Pending`, `ValidatingDatabaseState`, `RestoringPrimaryStorage`, `Completed`, `Failed`), `v1.PointInTimeRestoreStatus` and `v1.PartitionPosition{PartitionID int32, LastUpdated metav1.Time}`. Both kinds implement `GetStatusConditions`, `GetKind`, `SetObservedGeneration`, `Terminal`. New reasons: `v1.ReasonClusterNotSuspended` (shared, `api/v1/restore_shared.go`), `v1.ReasonIncompatibleTarget` (LogicalRestore), `v1.ReasonPitrUnavailable`, `v1.ReasonSharedServer`, `v1.ReasonDatabaseNotRestored` (PointInTimeRestore) | **realized in #120.** PR-A deletes the `LogicalRestore` half of it and moves `LogicalRestorePhase`, `LogicalBackupRef`, and `ReasonIncompatibleTarget` into `api/v1/restore_shared.go` |
| `restore-shared-package` | #110 | #111, #112, #113 | `pkg/restore`: `const FieldManagerLogicalRestore client.FieldOwner = "camunda-operator/logicalrestore"`, `const FieldManagerPointInTimeRestore client.FieldOwner = "camunda-operator/pointintimerestore"`, `const RestoreEntrypoint = "/usr/local/camunda/bin/restore"`, `const ComponentRestore = "restore"`; `type Target struct{ StatefulSet *appsv1.StatefulSet; Broker *corev1.Container; Brokers, Partitions int32; Version string; ClaimTemplate *corev1.PersistentVolumeClaim }`; `func ReadTarget(ctx context.Context, reader client.Reader, cluster *v1.CamundaCluster) (*Target, error)`; `func (t *Target) ClaimNames() []string`; `func (t *Target) ClaimSize(recorded *resource.Quantity) resource.Quantity`; `func (t *Target) BuildClaim(ordinal int32, size resource.Quantity) *corev1.PersistentVolumeClaim`; `type Progress struct{ Done bool; Message string; Recreated []string }`; `type ClaimInput struct{ Target *Target; Size resource.Quantity; Recreated []string; FieldManager client.FieldOwner }`; `func RecreateClaims(ctx context.Context, c client.Client, reader client.Reader, in ClaimInput) (Progress, error)`; `type JobInput struct{ Target *Target; Owner client.Object; OwnerLabel labels.Owner; Ordinal int32; Args []string }`; `func JobName(owner client.Object, ordinal int32) string`; `func BuildJob(in JobInput) (*batchv1.Job, error)`; `func Apply(ctx context.Context, c client.Client, obj client.Object, manager client.FieldOwner) error` | **realized in #120.** PR-A replaces `FieldManagerLogicalRestore` with one field manager per new kind and rewrites `jobKindInfixes` |
| `restore-progress` | PR-A | #111, PR-C, #113 | `api/v1/restore_shared.go`: `type RestoreProgress struct{ TargetClusterUID types.UID; Brokers int32; PrimaryJobNames []string; RecreatedClaims []string; FirstFailedAt *metav1.Time; TerminalReason string; FailureMessage string; CompletionTime *metav1.Time; ObservedGeneration int64; Conditions []metav1.Condition }`, embedded with `json:",inline"` in all three restore statuses. Also `type LogicalRestorePhase string` with `Pending`, `ValidatingCompatibility`, `RestoringSecondaryStorage`, `RestoringPrimaryStorage`, `Completed`, `Failed`, shared by the two logical kinds; `type LogicalBackupRef struct{ Name string }`; `const ReasonIncompatibleTarget`; `const ReasonClusterClaimed = "ClusterClaimed"` | merged code (PR-A lands first) |
| `restore-driver` | PR-A | #111, PR-C | `pkg/restore`: `type Outcome struct{ Wait time.Duration; Done bool; Failure *conditions.PreCheckFailure }`; `func Recovered(p *v1.RestoreProgress)`; `func HoldRunning(p *v1.RestoreProgress, failure *conditions.PreCheckFailure, now metav1.Time, grace, poll time.Duration) Outcome`; `func Complete(p *v1.RestoreProgress, now metav1.Time)`; `func Fail(p *v1.RestoreProgress, reason, message string, now metav1.Time)`; `func StageTerminal(owner conditions.Owner, p *v1.RestoreProgress)`; `type PrimaryInput struct{ Owner client.Object; OwnerLabel labels.Owner; Target *Target; Size resource.Quantity; FieldManager client.FieldOwner; Recorder record.EventRecorder }`; `func Primary(ctx context.Context, c client.Client, reader client.Reader, scheme *runtime.Scheme, p *v1.RestoreProgress, in PrimaryInput) (Outcome, error)`. `Primary` drives the whole primary-storage phase: it records `Brokers`, recreates the claims, and runs one restore-application Job per broker. It never writes `status.phase` | merged code (PR-A lands first) |
| `restore-cluster-claim` | PR-A | #111, PR-C | `pkg/restore`: `func Take(ctx context.Context, c client.Client, reader client.Reader, namespace, cluster string, self clusterclaim.Claimant) (Outcome, error)` and `func Give(ctx context.Context, c client.Client, reader client.Reader, namespace, cluster string, self clusterclaim.Claimant) error`, over `clusterclaim.Claim` and `clusterclaim.Release`. A cluster that another holder claims returns an `Outcome` whose `Failure` carries `v1.ReasonClusterClaimed` | merged code (PR-A lands first) |
| `logicalrestoreelasticsearch-api` | #111 | #113 | `v1.LogicalRestoreElasticsearch` at path `logicalrestoreelasticsearches`, shortName `lres`. `v1.LogicalRestoreElasticsearchSpec{BackupRef LogicalBackupRef, TargetClusterRef ClusterRef}`. `v1.LogicalRestoreElasticsearchStatus{Phase LogicalRestorePhase, BackupID int64, Repository string, RestoredSnapshots []string, RestoreProgress}`. Field manager `camunda-operator/logicalrestoreelasticsearch`. Label key `camunda.io/logical-restore-elasticsearch` | merged in PR-B, consumed by PR-D |
| `logicalrestorerdbms-api` | PR-C | #113 | `v1.LogicalRestoreRDBMS` at path `logicalrestorerdbmses`, shortName `lrrdbms`. `v1.LogicalRestoreRDBMSSpec{BackupRef LogicalBackupRef, TargetClusterRef ClusterRef}`. `v1.LogicalRestoreRDBMSStatus{Phase LogicalRestorePhase, BackupID int64, SecondaryJobName string, RestoreProgress}`. Field manager `camunda-operator/logicalrestorerdbms`. Label key `camunda.io/logical-restore-rdbms` | merged in PR-C, consumed by PR-D |
| `pod-stuck-helper` | #110 | #111, #112 | `pkg/podstate`: `func Stuck(ctx context.Context, reader client.Reader, namespace string, selector map[string]string, what string) (*conditions.PreCheckFailure, error)` — the first pod under the selector that cannot start, reported as a `MissingSecret`, `InvalidReference`, or `Progressing` failure that names the pod, the container, and the waiting reason. Lifted out of `internal/controller/logicalbackuprdbms`, which then consumes it | merged code (PR1 lands first) |
| `backup-version-field` | #110 | #111 | `v1.LogicalBackupElasticsearchStatus.Version string` and `v1.LogicalBackupRDBMSStatus.Version string`, written at admission from `cluster.status.management.version`. It is the only place a restore can read the Camunda version a backup was taken with, because `status.management` is nil while a cluster is suspended | merged code (PR1 lands first) |
| `backup-artifact-naming` | #110 | #111, #113 | `pkg/logicalbackup`: `func RecordsSnapshotName(id int64) string` (moved out of `internal/controller/logicalbackupelasticsearch`), `const ZeebeRecordIndices = "zeebe-record*"`, `func CamundaIndexPatterns(withOptimize bool) []string`, `func HasOptimizeSnapshot(names []string) bool` | merged code (PR1 lands first) |
| `pg-open-seam` | #110 | #112 | `pkg/pgbootstrap`: `func Open(ctx context.Context, c Connection, database string) (*pgx.Conn, error)` — the one place that builds a PostgreSQL DSN, now reachable by a caller that runs its own SQL inside a logical database. `Connection.AdminUser` and `Connection.AdminPassword` are renamed to `User` and `Password`, because the type now carries whatever role the caller holds | merged code (PR1 lands first) |
| `esadmin-restore-api` | #111 | #113 | `pkg/esadmin`: `func (c *Client) ResolveIndices(ctx context.Context, patterns []string) ([]string, error)`, `func (c *Client) DeleteIndices(ctx context.Context, patterns []string) error`, `const MaxDeletePathBytes = 3 << 10`, `func (c *Client) SnapshotRepositoryExists(ctx context.Context, name string) (bool, error)`, `func (c *Client) RestoreSnapshot(ctx context.Context, repo, name string, indices []string) error`, `type RestoreState string` with `RestoreInProgress`/`RestoreDone`, `func (c *Client) RestoreProgress(ctx context.Context, patterns []string) (RestoreState, error)`. `esadmintest` gains the routes and the operation names `"indexDelete"`, `"snapshotRestore"`, `"recovery"` | merged in PR-B, consumed by PR-D only through the controller |
| `logicalrestorerdbms-job` | PR-C | #113 | `pkg/components/logicalrestorerdbms`: `type JobInput struct{ ... }`, `func BuildJob(in JobInput) (*batchv1.Job, error)`, `func JobBelongsTo(job *batchv1.Job, restore client.Object) bool`, `const RestoreUIDLabel`. Plus `internal/cli/download` and its `download` subcommand on `camunda-operator-cli`, and `func (b *Bucket) Download(ctx context.Context, key string, w io.Writer) error` in `pkg/objectstore` | merged in PR-C, consumed by PR-D only through the controller |

`restore-progress`, `restore-driver`, and `restore-cluster-claim` are the main contract of the new wave. PR-A lands them complete and table-tested, and it proves them by moving the merged `PointInTimeRestore` controller onto them. PR-B and PR-C then import merged code, so neither has to stub the other's surface.

## Conventions

- **Naming firewall:** PR numbers and "PR N" labels never appear in code, fixtures, or test names.
- **Kinds:** `LogicalRestoreElasticsearch`, `LogicalRestoreRDBMS`, and `PointInTimeRestore` everywhere. Never `Restore`, `LogicalRestore`, `PITR`, or `PitrRestore`. The Go identifier for point-in-time restore is `PointInTimeRestore`, and the abbreviation `PITR` appears only inside prose and inside the existing `v1.PITRCapability` type.
- **Naming mirrors the backup pair.** `LogicalBackupElasticsearch` is `logicalbackupelasticsearches` with the shortName `lbes`, and `LogicalBackupRDBMS` is `logicalbackuprdbmses` with `lbrdbms`. The restore pair follows: `LogicalRestoreElasticsearch` is `logicalrestoreelasticsearches` with `lres`, and `LogicalRestoreRDBMS` is `logicalrestorerdbmses` with `lrrdbms`.
- **Scaffolding:** a new kind is created with `kubebuilder create api --group core --version v1 --kind <Kind>`. The CLI writes the types file, the controller stub, the `PROJECT` entry, the kustomization entries, and the sample. Hand-written files and hand-written `PROJECT` entries are both against AGENTS.md.
- **Shared package:** `pkg/restore`, package name `restore`. It is the shared machinery of all three restore kinds, in the role `pkg/logicalbackup` plays for the backup pair. It is pure where it can be: `BuildJob`, `BuildClaim`, `JobName`, `ClaimNames`, and `ClaimSize` take values and return values. `ReadTarget`, `RecreateClaims`, `Apply`, `Primary`, `Take`, and `Give` take a client because they must talk to the API server. The package never reads a restore CR's spec. It reads and writes `*v1.RestoreProgress` in place, and it never writes `status.phase`.
- **The driver never owns the phase.** Each kind owns its own phase vocabulary. A driver function returns an `Outcome`, and the controller maps that outcome onto its own phase. The two logical kinds share `v1.LogicalRestorePhase`, because their phase values are identical. `PointInTimeRestore` keeps `v1.PointInTimeRestorePhase`, because its values differ.
- **Controller packages:** `internal/controller/logicalrestoreelasticsearch`, `internal/controller/logicalrestorerdbms`, and `internal/controller/pointintimerestore`, one directory per CRD, matching the backup layout. The flat scaffold files that `kubebuilder create api` writes under `internal/controller/` are moved into the per-kind directory in the same PR.
- **Reconciler shape:** `New(c client.Client, reader client.Reader, scheme *runtime.Scheme, options Options) *Reconciler` plus `SetupWithManager(mgr ctrl.Manager) error`, matching `internal/controller/logicalbackupelasticsearch`. Not the struct-literal plus options-argument shape of `logicalbackuprdbms`.
- **Phases:** the phase is the resume marker. Neither kind gets a separate `step` field. A phase is persisted before the side effect it names.
- **Condition reasons:** reuse `api/v1/conditions.go` and `api/v1/logicalbackup_shared.go` wherever a reason already exists — `ReasonProgressing`, `ReasonCompleted`, `ReasonFailed`, `ReasonInvalidReference`, `ReasonMissingSecret`, `ReasonMissingCredentials`, `ReasonConnectionFailed`. Declare a reason that every restore kind reports in `api/v1/restore_shared.go`: `ReasonClusterNotSuspended`, `ReasonClusterClaimed`, and `ReasonIncompatibleTarget`. Declare a reason that one kind alone reports next to that kind's types, the way `ReasonResumeFailed` is declared today.
- **SSA field managers:** `camunda-operator/logicalrestoreelasticsearch`, `camunda-operator/logicalrestorerdbms`, and `camunda-operator/pointintimerestore`, as `client.FieldOwner` constants in `pkg/restore`. Every Job and every recreated PVC is applied with `client.Apply`, the calling controller's field manager, and `client.ForceOwnership`. Status is never written with SSA. It goes through `component.FlushStatus`.
- **The claim comes before every destructive step.** A restore takes the per-cluster Lease of `pkg/clusterclaim` when its admission passes, and gives it back at the terminal transition. Completed and Failed both give it back. A cluster that another holder claims holds the restore in `Pending` with the reason `ClusterClaimed`. Nothing bounds that hold. The restore starts on its own when the holder reaches a terminal phase. The reason names no kind, because the holder can be a backup or another restore.
- **PVC naming, sizing, ownership:** the name is what the StatefulSet expects, `data-<cluster>-zeebe-<ordinal>`, built from `components.DataVolumeName`, `components.WorkloadName(cluster, components.ComponentZeebe)`, and the ordinal. The count is `CAMUNDA_CLUSTER_SIZE` read from the live broker container, not `spec.replicas`, because a suspended StatefulSet runs at zero replicas. The size is the backup's recorded `status.storageSizes.zeebe` when the backup recorded one, and the StatefulSet's own claim template request when it did not. The storage class, the access modes, and the claim labels come from the claim template. **The recreated PVC carries no owner reference.** The StatefulSet owns those claims, and an owner reference to the restore CR deletes a live broker volume as soon as the restore CR is deleted.
- **Job labels:** every restore Job and its pods carry `labels.Managed(owner, restore.ComponentRestore)`. That is the owner key of the kind with the CR name, `camunda.io/component: restore`, and `app.kubernetes.io/managed-by: camunda-operator`. Those labels merge with `labels.ClusterKey: <target cluster name>` and with the broker pod labels that `ReadTarget` copied. The operator labels win over the copied ones. `pkg/labels` holds `PointInTimeRestoreKey = "camunda.io/point-in-time-restore"` today. PR-B adds `LogicalRestoreElasticsearchKey = "camunda.io/logical-restore-elasticsearch"` with `func LogicalRestoreElasticsearch(name string) Owner`. PR-C adds `LogicalRestoreRDBMSKey = "camunda.io/logical-restore-rdbms"` with `func LogicalRestoreRDBMS(name string) Owner`. No package declares a label string of its own.
- **Job name infixes:** `pkg/restore/job.go` holds `jobKindInfixes`. The map puts the kind into a Job name, so two restores of one name in one namespace never collide. The value is the shortName of the CRD. PR-A leaves the map with `pitr` alone. PR-B adds `lres` and PR-C adds `lrrdbms`.
- **Job ownership:** every restore Job gets a controller reference to its restore CR, so deleting the CR removes the Jobs. Neither controller needs a finalizer: a restore writes no artifact to an external store.
- **Test layout:** Ginkgo plus envtest at controller level, one `suite_test.go` per controller directory booting `internal/testenv`. Pure Go tests with testify next to the file that holds the feature. Rendered Kubernetes objects get golden snapshots under `pkg/restore/testdata/golden/<case>.yaml`, matching `pkg/components/logicalbackuprdbms`. CRD schema tests live in the controller directory as `schema_test.go`, matching the backup kinds.
- **Requeue cadence:** `defaultPollInterval = 5 * time.Second` paces a running phase. `defaultRetryInterval = 30 * time.Second` paces a hold that no watch resolves. `defaultMidRunGrace = 10 * time.Minute` bounds how long a started restore waits on a dependency that stopped resolving before the restore fails. `shortly = time.Second` re-enters after a staged status is persisted. Every value is an `Options` field so a test can shrink it, exactly like the backup controllers.
- **Docs:** each PR owns the `docs/crds/` page of the kind it ships, together with the `mkdocs.yml` nav entry and the `docs/crds/index.md` row. PR-A deletes `docs/crds/logicalrestore.md` and every link into it. PR-B creates `docs/crds/logicalrestoreelasticsearch.md` and PR-C creates `docs/crds/logicalrestorerdbms.md`, each without a "Not implemented yet" warning, because each ships its controller.
- **Interim links:** `mkdocs build --strict` fails on a link to a page that does not exist. Between PR-A and PR-B, no logical restore page exists. PR-A therefore rewrites the sentences that link to `logicalrestore.md` to name the two kinds in plain text, with no link. PR-B and PR-C put the link back for their own page.
- **Commit messages:** `feat(restore): ... (#<PR-A issue>)`, `feat(logicalrestoreelasticsearch): ... (#111)`, `feat(logicalrestorerdbms): ... (#<PR-C issue>)`, `feat(pointintimerestore): ...`, `test(e2e): ... (#113)`. One issue reference per commit.

## Facts this plan is built on

Read these before you argue with a task. Each was verified in the worktree.

- `CamundaCluster.status.management` is **nil while the cluster is suspended** (`internal/controller/camundacluster/binding.go`). A restore runs against a suspended cluster, so neither controller can read the partition count or the Camunda version from the management binding. Both read them from the live broker StatefulSet instead.
- The broker StatefulSet is `<cluster>-zeebe` (`components.WorkloadName(cluster, components.ComponentZeebe)`). Its only claim template is `data` (`components.DataVolumeName`). The generated PVC name is `data-<cluster>-zeebe-<ordinal>`.
- The broker container is named `camunda`. Its command is `["bash", "-c", "export CAMUNDA_CLUSTER_NODEID=${HOSTNAME##*-}; exec /usr/local/camunda/bin/camunda"]` (`pkg/components/camundacluster/render.go`). A Job pod has a random host name, so a restore Job sets `CAMUNDA_CLUSTER_NODEID` to the ordinal as a plain environment entry and runs the restore entrypoint directly.
- The broker container already carries `CAMUNDA_CLUSTER_SIZE`, `CAMUNDA_CLUSTER_PARTITIONCOUNT`, `CAMUNDA_CLUSTER_REPLICATIONFACTOR`, the whole secondary-storage block, and the primary-storage backup block. Copying that environment is what makes the restore application read the same database with the same credentials the brokers use, which is exactly what `docs/crds/pointintimerestore.md` promises.
- The Elasticsearch snapshot repository is registered by the `ElasticsearchCluster` controller under `base_path = logicalbackup.ClusterPrefix(bucketBasePath, elasticsearchClusterNamespace, elasticsearchClusterName)`, and its name is the `ElasticsearchCluster` name (`pkg/components/elasticsearchcluster`). A backup records that name in `status.repository` and pins its bucket in `status.storage.BucketRef`.
- `pkg/esadmin` has no index deletion and no snapshot restore today. PR2 adds both.
- `pkg/pgbootstrap` builds every DSN in one unexported `dial` function and exposes no way to run caller SQL inside a logical database. PR1 adds `Open`.
- The e2e suite already seeds real data: `itRunsTheOrchestrationCluster` deploys `testdata/process.bpmn`, starts an instance, and proves it through `expectInstanceSearchable(cluster)`, which reads through secondary storage. The e2e suite talks to services from in-cluster helper pods through `utils.RunPod`. There is no port forwarding anywhere, and PR-D adds none.

Verified again on 2026-08-20, for the amended breakdown:

- `pkg/conditions.PreCheckFailure` is exactly `{Reason, Message string}` with an `Error()` method. The spec sketches an `Outcome.Failure *Failure`. This plan realizes that field as `*conditions.PreCheckFailure` and declares no second type. The repository already passes that type through every pre-check path.
- `pkg/clusterclaim` exposes `Claim(ctx, c, reader, namespace, cluster, self) (string, error)` and `Release(ctx, c, reader, namespace, cluster, self) error`. `Claim` returns the current holder identity when another claimant holds the Lease, and the empty string when the caller holds it. `holderKinds` in `pkg/clusterclaim/claim.go` maps a kind name to an empty resource, and a kind that is absent from the map blocks every takeover.
- `v1.PointInTimeRestoreStatus` already carries `TerminalReason`, which #122 added. `v1.LogicalRestoreStatus` on the base branch does not. `RestoreProgress` carries it for all three kinds.
- `api/v1/restore_shared.go` exists and holds `ReasonClusterNotSuspended` alone.
- `api/v1/logicalrestore_types.go` holds `ReasonIncompatibleTarget`, `LogicalBackupKind`, `LogicalBackupRef`, `LogicalRestorePhase`, `LogicalRestoreSpec`, and `LogicalRestoreStatus`. PR-A keeps the parts that survive by moving them into `restore_shared.go`, and deletes the file.
- Both backup controllers carry the RBAC marker `+kubebuilder:rbac:groups=core.camunda.io,resources=logicalrestores;pointintimerestores,verbs=get`. That marker is how a backup reads the restore that holds the cluster claim. It is in `internal/controller/logicalbackupelasticsearch/controller.go` and `internal/controller/logicalbackuprdbms/controller.go`, and it names resources that PR-A removes.
- The closed branch `feat/restore-controllers--logicalrestore`, head `14a1545`, holds the working code of the old single kind. It is 7039 added lines over 35 files. PR-B and PR-C lift from it. The two secondary-storage files are `internal/controller/logicalrestore/secondary_elasticsearch.go` (274 lines) and `internal/controller/logicalrestore/secondary_rdbms.go` (517 lines).

---

## PR1 — API types and the shared restore package (#110) — DONE

**Merged as PR #120.** This section is the record of what shipped. Do not work it again. PR-A rewrites the `LogicalRestore` half of it and widens `pkg/restore`.

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
	// TargetClusterRef references the CamundaCluster to restore into. It must
	// name the cluster the backup was taken from: the restore application
	// reads the primary-storage backup under the prefix of the cluster it
	// runs as. Issue #140 tracks a restore into a differently-named cluster.
	// The cluster must be suspended for the whole restore.
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
- state that the Elasticsearch path registers its own snapshot repository on the target's Elasticsearch, derived from the backup's pinned bucket and the prefix the source cluster wrote under, so the snapshots are readable whichever Elasticsearch server the target reads through. This does not make a restore into a second cluster work: the primary-storage half still derives its base path from the target, so the target must carry the name of the source cluster. Issue #140 tracks lifting that.
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

### Review checkpoint after PR1 — DONE

Ran over the merged feature branch. The answers to the two Camunda verification steps are in the state file.

---

## PR2 — The LogicalRestore controller (#111) — OBSOLETE

**Superseded by PR-A, PR-B, and PR-C below.** Its PR #123 is closed. The spec now splits the kind by secondary storage type, so a single controller over both paths no longer matches the design.

The work of this section did not go to waste. It is merge-ready code on the closed branch `feat/restore-controllers--logicalrestore`, head `14a1545`, and PR-B and PR-C lift from it file by file. The section that told an implementer how to write that code from nothing is gone, because the code exists. What replaces it is a map from each file on that branch to the PR that takes it.

---

## PR3 — The PointInTimeRestore controller (#112) — DONE

**Merged as PR #122, commit `075746a`.** This section is the record of what shipped. Do not work it again. PR-A moves the controller onto the shared driver and adds the cluster claim to it.

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

### Review checkpoint after PR3 — DONE

Ran over the feature branch after #122 merged. The drift it found is what PR-A resolves.

---

## PR-A — Unify the restore machinery (new sub-issue under #109, branch `feat/restore-controllers--unify`, worktree `.claude/worktrees/restore-controllers--unify`)

Implements the spec's "the shared restore machinery lives in pkg/restore" decision and its "a restore claims its cluster" decision. It also removes the `LogicalRestore` kind, which the split leaves dead.

PR-A ships no new kind. It is the last change that touches `internal/controller/pointintimerestore`, and it is the base that PR-B and PR-C fan out from.

**File the sub-issue first.** Title it "Unify the restore machinery in pkg/restore". Body: the three spec decisions above, the removal surface of Task 5, and `Towards #109`.

**Files:**
- Modify: `api/v1/restore_shared.go` (add `RestoreProgress`, `LogicalRestorePhase`, `LogicalBackupRef`, `ReasonIncompatibleTarget`, `ReasonClusterClaimed`)
- Modify: `api/v1/pointintimerestore_types.go` (embed `RestoreProgress`, drop the moved fields, rename `ClusterUID`)
- Rewrite: `api/v1/restore_types_test.go`
- Create: `pkg/restore/{outcome.go,outcome_test.go,progress.go,progress_test.go,primary.go,primary_test.go,claim.go,claim_test.go}`
- Modify: `pkg/restore/{doc.go,apply.go,apply_test.go,job.go,job_test.go,claims_test.go}` and `pkg/restore/testdata/golden/`
- Modify: `internal/controller/pointintimerestore/{controller.go,admit.go,primary.go,controller_test.go}`
- Delete: `api/v1/logicalrestore_types.go`, `config/crd/bases/core.camunda.io_logicalrestores.yaml`, `config/rbac/logicalrestore_admin_role.yaml`, `config/rbac/logicalrestore_editor_role.yaml`, `config/rbac/logicalrestore_viewer_role.yaml`, `config/samples/core_v1_logicalrestore.yaml`, `docs/crds/logicalrestore.md`
- Modify: `config/crd/kustomization.yaml`, `config/rbac/kustomization.yaml`, `config/samples/kustomization.yaml`, `PROJECT`, `mkdocs.yml`
- Modify: `pkg/labels/labels.go`, `pkg/labels/labels_test.go`, `pkg/clusterclaim/claim.go`, `pkg/clusterclaim/claim_test.go`
- Modify: `internal/controller/samples_schema_test.go`, `internal/controller/logicalbackupelasticsearch/controller.go`, `internal/controller/logicalbackuprdbms/controller.go`
- Modify: `docs/crds/index.md`, `docs/crds/camundaoptimize.md`, `docs/crds/pointintimerestore.md`
- Modify (generated): `api/v1/zz_generated.deepcopy.go`, `config/rbac/role.yaml`, `config/crd/bases/core.camunda.io_pointintimerestores.yaml`

### Task 1: `v1.RestoreProgress` and the shared restore vocabulary

**Produces:** the `restore-progress` contract row.

- [ ] **Step 1: Write the failing test**

Rewrite `api/v1/restore_types_test.go`. Drop every `LogicalRestore` case. Add the one property that the whole embedding rests on:

```go
func TestRestoreProgressStaysInlineInTheStatus(t *testing.T) {
	status := v1.PointInTimeRestoreStatus{
		Phase: v1.PointInTimeRestoreCompleted,
		RestoreProgress: v1.RestoreProgress{
			Brokers:            3,
			PrimaryJobNames:    []string{"r-pitr-0", "r-pitr-1", "r-pitr-2"},
			ObservedGeneration: 7,
		},
	}

	payload, err := json.Marshal(status)
	require.NoError(t, err)

	var raw map[string]any
	require.NoError(t, json.Unmarshal(payload, &raw))
	assert.Equal(t, float64(3), raw["brokers"])
	assert.Equal(t, float64(7), raw["observedGeneration"])
	assert.NotContains(t, raw, "restoreProgress")
}

func TestPointInTimeRestoreTerminal(t *testing.T) {
	for _, tc := range []struct {
		phase v1.PointInTimeRestorePhase
		want  bool
	}{
		{v1.PointInTimeRestorePending, false},
		{v1.PointInTimeRestoreValidatingDatabaseState, false},
		{v1.PointInTimeRestoreRestoringPrimaryStorage, false},
		{v1.PointInTimeRestoreCompleted, true},
		{v1.PointInTimeRestoreFailed, true},
	} {
		restore := &v1.PointInTimeRestore{Status: v1.PointInTimeRestoreStatus{Phase: tc.phase}}
		assert.Equal(t, tc.want, restore.Terminal(), "phase %s", tc.phase)
	}
}

func TestSharedRestoreReasons(t *testing.T) {
	assert.Equal(t, "ClusterNotSuspended", v1.ReasonClusterNotSuspended)
	assert.Equal(t, "ClusterClaimed", v1.ReasonClusterClaimed)
	assert.Equal(t, "IncompatibleTarget", v1.ReasonIncompatibleTarget)
}
```

- [ ] **Step 2: Run the test and confirm it fails**

Run: `go test ./api/... -run 'TestRestoreProgress|TestSharedRestoreReasons' -v`
Expected: FAIL, compile error, the identifiers do not exist.

- [ ] **Step 3: Write the shared types in `api/v1/restore_shared.go`**

Keep the existing `ReasonClusterNotSuspended`. Add:

```go
// ReasonClusterClaimed means that another backup or another restore holds
// the cluster. The restore waits in Pending until that holder reaches a
// terminal phase. Nothing bounds the wait, and the reason names no kind,
// because the holder can be either.
const ReasonClusterClaimed = "ClusterClaimed"

// ReasonIncompatibleTarget means that the target cluster cannot hold the
// backup: the secondary storage types differ, the partition counts differ,
// the backup bucket differs, or the Camunda versions break the version rule.
// Only a logical restore reports it.
const ReasonIncompatibleTarget = "IncompatibleTarget"

// LogicalBackupRef references a completed logical backup in the namespace of
// the restore. The reference never crosses a namespace. The kind of the
// restore says which backup kind it reads, so the reference carries a name
// alone.
type LogicalBackupRef struct {
	// Name of the backup, in the namespace of this restore.
	// +required
	// +kubebuilder:validation:MinLength=1
	Name string `json:"name"`
}

// LogicalRestorePhase tracks a one-shot logical restore. Completed and Failed
// are terminal. A retry is a new resource. Both logical restore kinds use it,
// because their phase values are the same.
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

// RestoreProgress is the part of a restore status that every restore kind
// has. It is embedded with json:",inline", so each status keeps the field
// names it had before and the CRD schema does not change. controller-gen
// flattens an inline embedded struct the same way encoding/json does.
//
// pkg/restore reads and writes this struct in place. The driver owns every
// field here. Each kind owns its own phase and the fields of its own
// procedure.
type RestoreProgress struct {
	// TargetClusterUID pins the identity of the target cluster. A cluster
	// that is deleted and created again under the same name is another
	// cluster, and this restore is not its restore.
	// +optional
	TargetClusterUID types.UID `json:"targetClusterUID,omitempty"`
	// Brokers is the broker count read from the live broker StatefulSet when
	// the restore entered the primary-storage phase. It fixes how many
	// volumes are recreated and how many Jobs run.
	// +optional
	Brokers int32 `json:"brokers,omitempty"`
	// PrimaryJobNames are the per-broker restore-application Jobs, in broker
	// order. The operator records them before it applies the Jobs, so the
	// record covers every Job that the next look finds.
	// +optional
	PrimaryJobNames []string `json:"primaryJobNames,omitempty"`
	// RecreatedClaims names the broker data claims that the restore deleted
	// and created again. A reconcile that re-enters does not delete a claim
	// twice.
	// +optional
	RecreatedClaims []string `json:"recreatedClaims,omitempty"`
	// FirstFailedAt is when a dependency of the running restore first stopped
	// resolving. The operator measures the mid-run grace from it. It stays
	// set once the restore starts, because a dependency that flaps must not
	// reset the grace.
	// +optional
	FirstFailedAt *metav1.Time `json:"firstFailedAt,omitempty"`
	// TerminalReason is the Ready reason recorded at the terminal transition.
	// The operator stages the terminal condition again from this field, so a
	// write conflict cannot replace the reason with a weaker one.
	// +optional
	TerminalReason string `json:"terminalReason,omitempty"`
	// FailureMessage names the failing phase and its error. The Ready
	// condition carries the same message.
	// +optional
	FailureMessage string `json:"failureMessage,omitempty"`
	// CompletionTime is when the restore reached a terminal phase.
	// +optional
	CompletionTime *metav1.Time `json:"completionTime,omitempty"`
	// ObservedGeneration is the last generation reconciled by the operator.
	// +optional
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`
	// Conditions represent the current state of the restore.
	// +listType=map
	// +listMapKey=type
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}
```

- [ ] **Step 4: Embed `RestoreProgress` in `v1.PointInTimeRestoreStatus`**

Delete the nine fields that moved: `ClusterUID`, `Brokers`, `PrimaryJobNames`, `RecreatedClaims`, `FirstFailedAt`, `TerminalReason`, `FailureMessage`, `CompletionTime`, `ObservedGeneration`, and `Conditions`. Add one line:

```go
	// RestoreProgress is the part of the status that every restore kind has.
	RestoreProgress `json:",inline"`
```

`Phase`, `Storage`, and `ObservedPositions` stay. They are this kind's own.

The field `ClusterUID` becomes `TargetClusterUID`. The spec resolves that divergence in favour of the name that the logical kind used. One thing has one name. This renames a JSON key in the `PointInTimeRestore` CRD. The kind is unreleased, so no conversion is needed. State the rename in the PR body.

`GetStatusConditions` and `SetObservedGeneration` reach the promoted fields without a change, because Go promotes an embedded struct's fields.

- [ ] **Step 5: Regenerate and confirm the schema is unchanged apart from the rename**

```bash
make manifests generate
git diff config/crd/bases/core.camunda.io_pointintimerestores.yaml
```

Expected: the only property difference is `clusterUID` becoming `targetClusterUID`. Every other property keeps its place. A diff that nests the fields under a `restoreProgress` key means the `json:",inline"` tag is missing.

- [ ] **Step 6: Run the tests and confirm they pass**

Run: `go test ./api/... ./internal/controller/pointintimerestore/... -v`
Expected: PASS.

- [ ] **Step 7: Commit**

```bash
git add api config
git commit -m "refactor(api): give every restore kind one progress struct (#<PR-A issue>)"
```

### Task 2: The driver in `pkg/restore`

**Consumes:** `v1.RestoreProgress`, `restore.Target`, `restore.RecreateClaims`, `restore.BuildJob`.
**Produces:** the `restore-driver` contract row.

The driver is a move, not a rewrite. Every function it holds runs in production today inside `internal/controller/pointintimerestore`. The move is what makes the two logical kinds reuse it instead of copying it.

- [ ] **Step 1: Write `pkg/restore/outcome.go`**

```go
// Outcome is what a driver step reports back to a controller. The driver
// never writes status.phase, because each restore kind owns its own phase
// vocabulary. The controller maps an outcome onto its own phase.
type Outcome struct {
	// Wait is how long before the next look. Zero means that the watches
	// carry the wake-up, and the controller settles.
	Wait time.Duration
	// Done reports that the step finished and the controller advances.
	Done bool
	// Failure is set when the restore reached a terminal failure. It carries
	// the Ready reason and the message.
	Failure *conditions.PreCheckFailure
}
```

The spec sketches `Failure *Failure`. This plan realizes that field as `*conditions.PreCheckFailure`, which is exactly `{Reason, Message string}` and is what every pre-check in this repository already returns. A second type of the same shape earns nothing.

- [ ] **Step 2: Write the failing tests for the progress helpers**

Create `pkg/restore/progress_test.go` as a pure table test over `*v1.RestoreProgress`. Cover, one case each:

| Case | Expected |
| --- | --- |
| `HoldRunning` on a progress with no `FirstFailedAt` | records `now`, returns `Outcome{Wait: poll}` |
| `HoldRunning` on a progress whose `FirstFailedAt` is inside the grace | leaves `FirstFailedAt`, returns `Outcome{Wait: poll}` |
| `HoldRunning` on a progress whose `FirstFailedAt` is past the grace | returns `Outcome{Failure: the passed failure}` |
| `Recovered` on a progress that never failed | no change |
| `Recovered` on a running progress with `FirstFailedAt` set | leaves `FirstFailedAt` set |
| `Complete` | sets `CompletionTime`, `TerminalReason: Completed` |
| `Fail` | sets `CompletionTime`, `TerminalReason`, `FailureMessage` |

The fifth row is the rule that #122 found the hard way: a running restore never clears `FirstFailedAt`. A dependency that flaps in and out resets the grace otherwise, and the restore holds without end. `Recovered` therefore clears nothing on a started restore, and it exists to keep the call sites of the two controllers identical.

- [ ] **Step 3: Write `pkg/restore/progress.go`**

Move the bodies of `recovered`, `waiting`, `holdStarted`, `complete`, `fail`, and `stageTerminal` out of `internal/controller/pointintimerestore/controller.go`. Change each to take `*v1.RestoreProgress` and a clock value instead of the `*v1.PointInTimeRestore`, and to return an `Outcome` instead of the package-local `hold`.

```go
// HoldRunning holds a started restore on a dependency that stopped
// resolving. It records when the failure started, and it fails the restore
// once the grace is over. A restore that has not started yet holds without a
// bound instead, through the Pending path of its controller.
func HoldRunning(
	p *v1.RestoreProgress,
	failure *conditions.PreCheckFailure,
	now metav1.Time,
	grace, poll time.Duration,
) Outcome

// Recovered marks the restore as making progress again. It clears nothing on
// a started restore: a dependency that flaps must not reset the mid-run
// grace.
func Recovered(p *v1.RestoreProgress)

// Complete records the terminal success.
func Complete(p *v1.RestoreProgress, now metav1.Time)

// Fail records the terminal failure with its Ready reason and message.
func Fail(p *v1.RestoreProgress, reason, message string, now metav1.Time)

// StageTerminal stages the Ready condition of a terminal restore again, from
// the recorded reason and message. Every look does it, so a write conflict
// cannot leave a weaker condition behind.
func StageTerminal(owner conditions.Owner, p *v1.RestoreProgress)
```

- [ ] **Step 4: Run the tests and confirm they pass**

Run: `go test ./pkg/restore/... -run 'TestHoldRunning|TestRecovered|TestComplete|TestFail|TestStageTerminal' -v`
Expected: PASS.

- [ ] **Step 5: Write the failing tests for the primary-storage phase**

Create `pkg/restore/primary_test.go` with a fake client and an interceptor. Cover:

- `Primary` records `Brokers` from the target before it deletes anything
- `Primary` records every claim name in `RecreatedClaims` before it deletes that claim
- `Primary` records every Job name in `PrimaryJobNames` before it applies a Job
- `Primary` refuses an ordinal past the live broker count, with a message that names both counts
- `Primary` fails when a Job that `PrimaryJobNames` records is gone
- `Primary` emits one event for each broker whose Job starts
- `Primary` returns `Outcome{Done: true}` only when every Job completed
- a recreated claim carries no owner reference to the restore

The last case is the ownership rule. Write it first. The `StatefulSet` owns the broker claims, and an owner reference to a restore deletes a live broker volume as soon as the restore is deleted.

- [ ] **Step 6: Write `pkg/restore/primary.go`**

Move `restorePrimaryStorage`, `recreateClaims`, `jobNames`, `runJobs`, `ensureJob`, `claimJob`, `ownedBy`, and `errUnrecoverable` out of `internal/controller/pointintimerestore/primary.go`. The exported entry point is `Primary`.

```go
// PrimaryInput is what the primary-storage phase renders and applies from.
type PrimaryInput struct {
	// Owner is the restore resource. Every Job gets a controller reference
	// to it.
	Owner client.Object
	// OwnerLabel is the owner label of the restore kind, from pkg/labels.
	OwnerLabel labels.Owner
	// Target is the live broker StatefulSet and the facts read off it.
	Target *Target
	// Size is the requested size of each recreated claim.
	Size resource.Quantity
	// FieldManager is the field manager of the calling restore kind.
	FieldManager client.FieldOwner
	// Recorder emits one event for each broker whose Job starts.
	Recorder record.EventRecorder
	// Args are the arguments of the restore application, for example
	// --backupId=42 or --to=2026-07-30T14:30:00Z.
	Args []string
}

// Primary drives the whole primary-storage phase: it records the broker
// count, deletes and creates the broker data volumes, and runs the restore
// application once per broker. It reports Done when every Job completed.
//
// It never writes status.phase. The caller persists the phase before it
// calls Primary the first time, because the phase is the resume marker of
// the destructive step.
func Primary(
	ctx context.Context,
	c client.Client,
	reader client.Reader,
	scheme *runtime.Scheme,
	p *v1.RestoreProgress,
	in PrimaryInput,
) (Outcome, error)
```

The spec's unification table decides each divergence between the two copies. Realize every row:

| Behavior | Kept from | Reason |
| --- | --- | --- |
| Record `PrimaryJobNames` before the Jobs are applied | `PointInTimeRestore` | The names are durable before a Job exists, so the record covers what the next look applies |
| Refuse an ordinal past the live broker count | `PointInTimeRestore` | It names the real cause. The render error underneath reports only counts |
| Fail when a recorded Job is gone | `LogicalRestore` | A second Job runs the restore application on a volume that the first one wrote |
| One event for each broker that starts | `PointInTimeRestore` | It is the only per-broker signal that the user gets |
| `Fail` takes a reason | `LogicalRestore` | The logical kinds report `IncompatibleTarget`. `PointInTimeRestore` passes `Failed` |
| The field name `targetClusterUID` | `LogicalRestore` | `PointInTimeRestore` called it `clusterUID`. One thing has one name |

The third row is the one change of behaviour, not just of shape. `PointInTimeRestore` iterated the recorded names and tolerated a missing Job. It now fails. Name that row in the test that changes, so a reader of the diff sees the resolved divergence.

`Primary` also carries the two-call contract of `RecreateClaims`, which #110 shipped: the caller flushes `RecreatedClaims` between calls and stops calling once `Progress.Done`. Fold that loop inside `Primary`, so no controller repeats it.

- [ ] **Step 7: Run the tests and confirm they pass**

Run: `go test ./pkg/restore/... -v`
Expected: PASS.

- [ ] **Step 8: Commit**

```bash
git add pkg/restore
git commit -m "feat(restore): drive the shared restore phases from one package (#<PR-A issue>)"
```

### Task 3: The cluster claim inside the driver

**Produces:** the `restore-cluster-claim` contract row.

- [ ] **Step 1: Write the failing test**

Create `pkg/restore/claim_test.go` against a fake client. Cover:

- an unclaimed cluster returns `Outcome{Done: true}` and the Lease names the caller
- a cluster that a live backup claims returns an `Outcome` whose `Failure.Reason` is `v1.ReasonClusterClaimed`, with a message that names the holder
- a cluster that a terminal holder claims is taken over, and the call returns `Outcome{Done: true}`
- `Give` on a Lease that the caller holds removes it
- `Give` on a Lease that another claimant holds removes nothing and returns no error

- [ ] **Step 2: Run it and confirm it fails**

Run: `go test ./pkg/restore/... -run TestTake -v`
Expected: FAIL, `Take` is undefined.

- [ ] **Step 3: Write `pkg/restore/claim.go`**

```go
// Take claims the cluster for self. A cluster that no live holder claims is
// claimed, and Take reports Done. A cluster that another live holder claims
// is not, and Take reports a failure with v1.ReasonClusterClaimed that names
// the holder.
//
// Nothing bounds the hold. The restore starts on its own when the holder
// reaches a terminal phase, because the next look takes the claim over. The
// holder can be a backup or another restore, so the reason names no kind.
func Take(
	ctx context.Context,
	c client.Client,
	reader client.Reader,
	namespace, cluster string,
	self clusterclaim.Claimant,
) (Outcome, error)

// Give returns the claim on the cluster. It is safe on every look of a
// terminal restore and inside a finalizer: a Lease that another claimant
// holds is left alone.
func Give(
	ctx context.Context,
	c client.Client,
	reader client.Reader,
	namespace, cluster string,
	self clusterclaim.Claimant,
) error
```

`Take` wraps `clusterclaim.Claim` and `Give` wraps `clusterclaim.Release`. Both already read through an uncached reader, which is what makes the exclusion real.

- [ ] **Step 4: Run it and confirm it passes, then commit**

```bash
git add pkg/restore
git commit -m "feat(restore): let a restore claim the cluster it rewrites (#<PR-A issue>)"
```

### Task 4: Move `PointInTimeRestore` onto the driver and give it the claim

The merged controller keeps its envtest suite. That suite is the safety net of this refactor, so change a test only where the resolved divergence changed the intent.

- [ ] **Step 1: Add the failing claim specs**

In `internal/controller/pointintimerestore/controller_test.go`, add to the admission Describe:

- a cluster whose claim Lease a `LogicalBackupRDBMS` holds keeps the restore in `Pending` with `Ready=False`, reason `ClusterClaimed`, and creates no Job and deletes no claim
- the same restore starts on its own once the holder reaches `Completed`, without a spec change
- a restore that reaches `Completed` removes the claim Lease
- a restore that reaches `Failed` removes the claim Lease

The "creates no Job and deletes no claim" assertion is what the claim exists for. Write it first.

- [ ] **Step 2: Run and confirm they fail**

Run: `go test ./internal/controller/pointintimerestore/... -v`
Expected: FAIL.

- [ ] **Step 3: Rewire the controller**

- `controller.go` drops `hold`, `settle`, `shortly`, `complete`, `fail`, `stageTerminal`, `waiting`, `holdStarted`, and `recovered`, and calls the `pkg/restore` versions. It keeps `progressing`, which stages a phase message in this kind's own words.
- `admit.go` calls `restore.Take` after the last admission check passes, and maps a claimed cluster onto a `Pending` hold with `ReasonClusterClaimed`.
- `controller.go` calls `restore.Give` at both terminal transitions.
- `primary.go` shrinks to a thin wrapper that builds a `restore.PrimaryInput` and maps the `Outcome` onto `v1.PointInTimeRestorePhase`.

The claim point is the same for all three restore kinds, and it comes before every phase that touches storage. Two restores of one cluster therefore never both pass validation. Put that sentence in the GoDoc of the admission step.

- [ ] **Step 4: Run every affected suite and confirm they pass**

Run: `go test ./api/... ./pkg/... ./internal/controller/pointintimerestore/... -v`
Expected: PASS.

- [ ] **Step 5: Update `docs/crds/pointintimerestore.md`**

Load `simple-english:simple-english` first. Add the claim to the page: the restore waits in `Pending` with the reason `ClusterClaimed` while another backup or restore holds the cluster, and it starts on its own when that holder finishes. Add the `ClusterClaimed` row to the status table. Rename `clusterUID` to `targetClusterUID` in the API reference block.

- [ ] **Step 6: Commit**

```bash
git add internal docs
git commit -m "refactor(pointintimerestore): run on the shared driver and claim the cluster (#<PR-A issue>)"
```

### Task 5: Remove the `LogicalRestore` kind

The kind is dead the moment the spec splits it. Every reference goes in this task, so PR-B and PR-C start from a tree with no half-removed kind in it.

Nothing in this task is optional. A leftover reference breaks one of the five verification gates, and the gate that catches it is not always the obvious one.

- [ ] **Step 1: Delete the Go and config surface**

```bash
git rm api/v1/logicalrestore_types.go
git rm config/crd/bases/core.camunda.io_logicalrestores.yaml
git rm config/rbac/logicalrestore_admin_role.yaml
git rm config/rbac/logicalrestore_editor_role.yaml
git rm config/rbac/logicalrestore_viewer_role.yaml
git rm config/samples/core_v1_logicalrestore.yaml
git rm docs/crds/logicalrestore.md
```

Task 1 already moved `ReasonIncompatibleTarget`, `LogicalBackupRef`, and `LogicalRestorePhase` out of the deleted types file. The deleted file also holds `LogicalBackupKind`, `LogicalBackupKindElasticsearch`, and `LogicalBackupKindRDBMS`. Those go for good: the kind of the restore now says which backup kind it reads, so the discriminator has no reader.

- [ ] **Step 2: Remove every kustomization and `PROJECT` entry**

- `config/crd/kustomization.yaml`: remove `- bases/core.camunda.io_logicalrestores.yaml`
- `config/rbac/kustomization.yaml`: remove the three `logicalrestore_*_role.yaml` lines
- `config/samples/kustomization.yaml`: remove `- core_v1_logicalrestore.yaml`
- `PROJECT`: remove the whole resource block whose `kind` is `LogicalRestore`

AGENTS.md says never to hand-edit `PROJECT`, and `kubebuilder` has no verb that removes an API. The hand edit is therefore the only path. Say so in the PR body, and remove nothing else from the file.

- [ ] **Step 3: Remove every Go reference**

| File | Reference | Change |
| --- | --- | --- |
| `pkg/labels/labels.go` | `LogicalRestoreKey` and `func LogicalRestore` | delete both. PR-B and PR-C add one owner each |
| `pkg/labels/labels_test.go` | the assertions on both | delete them |
| `pkg/restore/apply.go` | `FieldManagerLogicalRestore` | delete it. PR-B and PR-C add one field manager each |
| `pkg/restore/apply_test.go` | the assertion on it and its use in the apply test | retarget both at `FieldManagerPointInTimeRestore` |
| `pkg/restore/claims_test.go` | `const testManager = client.FieldOwner("camunda-operator/logicalrestore")` | retarget at `camunda-operator/pointintimerestore` |
| `pkg/restore/job.go` | `jobKindInfixes[labels.LogicalRestoreKey] = "lr"` | delete the entry. The map keeps `pitr` alone |
| `pkg/restore/job_test.go` | the `logicalRestore()` fixture and every case built on it | rebuild them on `v1.PointInTimeRestore` |
| `pkg/restore/doc.go` | the package comment names the two old kinds | rewrite it for the wider scope of Task 2 |
| `pkg/clusterclaim/claim.go` | `holderKinds["LogicalRestore"]` | delete the entry |
| `pkg/clusterclaim/claim_test.go` | the `v1.LogicalRestore` holder fixture and the kind list at the end | rebuild the fixture on a backup kind, and drop `"LogicalRestore"` from the list |
| `api/v1/restore_types_test.go` | every `LogicalRestore` case | Task 1 already rewrote the file |
| `internal/controller/samples_schema_test.go` | `"core_v1_logicalrestore.yaml"` | delete the entry |
| `internal/controller/logicalbackupelasticsearch/controller.go` | `+kubebuilder:rbac:...resources=logicalrestores;pointintimerestores,verbs=get` | drop `logicalrestores`. PR-B and PR-C each add their own plural back |
| `internal/controller/logicalbackuprdbms/controller.go` | the same marker | the same change |
| `internal/controller/pointintimerestore/admit.go` | the message "Use a LogicalRestore instead" | name the two new kinds in plain text |
| `internal/controller/pointintimerestore/primary.go` | the comment "The pg_restore Job of a LogicalRestore claims its name the same way" | name `LogicalRestoreRDBMS` |

The two backup-controller RBAC markers are easy to miss. They are how a backup reads the restore that holds the cluster claim, and they name a resource that the API server stops serving.

- [ ] **Step 4: Regenerate the golden Job snapshots**

`pkg/restore/testdata/golden/elasticsearch-broker-0.yaml`, `elasticsearch-broker-2.yaml`, and `rdbms-no-args.yaml` carry `camunda.io/logical-restore` labels and `-lr-` Job names. Retarget those three cases at `PointInTimeRestore` and regenerate. PR-B and PR-C add a golden case for their own kind.

- [ ] **Step 5: Remove every documentation reference**

| File | Reference | Change |
| --- | --- | --- |
| `mkdocs.yml` | `- LogicalRestore: crds/logicalrestore.md` under `Planned` | delete the line |
| `docs/crds/index.md` | the `LogicalRestore` row of the "Planned kinds" table | replace it with one row for each new kind, still under "Planned kinds" |
| `docs/crds/camundaoptimize.md` | the link to `logicalrestore.md` | name `LogicalRestoreElasticsearch` in plain text, with no link |
| `docs/crds/pointintimerestore.md` | two links to `logicalrestore.md` | name the two new kinds in plain text, with no link |

`mkdocs build --strict` fails on a link to a page that does not exist. Neither new page exists until PR-B and PR-C, so the links go now and come back then.

- [ ] **Step 6: Prove that nothing is left**

```bash
grep -rniI --exclude-dir=.git --exclude-dir=superpowers 'logicalrestore\|logical-restore\|LogicalBackupKind' .
```

Expected: no line outside `docs/superpowers/`, where the plan, the spec, and the state file record the history on purpose.

- [ ] **Step 7: Commit**

```bash
git add -A
git commit -m "refactor(api): remove the LogicalRestore kind that the split replaces (#<PR-A issue>)"
```

### Task 6: Verify the whole PR and open it

- [ ] **Step 1: Run all five gates**

```bash
go test ./...
make lint
make manifests generate && git status --porcelain config api
go vet -tags=e2e ./test/e2e/
mkdocs build --strict
```

Expected: tests pass, lint prints 0 issues in both modules, `git status --porcelain config api` prints nothing, the e2e vet compiles, and the docs build with no warning.

- [ ] **Step 2: Open the PR**

Open it with `Towards #<PR-A issue>`. Record in the body: the `clusterUID` rename, the `PROJECT` hand edit, the resolved-divergence table, and the removal surface. Run `feature-dev-workflow:copilot-review-loop` at the balanced level until it returns nothing new. Self-merge into `feat/restore-controllers`.

### Review checkpoint after PR-A

Before PR-B and PR-C fan out, run `feature-dev-workflow:reviewing-feature-progress` over the merged feature branch. Confirm three things. `pkg/restore` reads as one package with one vocabulary. No controller package is imported by another. The `PointInTimeRestore` controller holds no copy of a driver function.

Record in the state file the exact driver signatures that PR-B and PR-C consume, so neither worker has to read `pkg/restore` to find them.

---

## PR-B — LogicalRestoreElasticsearch (#111, branch `feat/restore-controllers--lres`, worktree `.claude/worktrees/restore-controllers--lres`)

Implements the Elasticsearch half of the old `LogicalRestore`, as its own kind.

**Reconcile issue #111 first.** Its body still describes one kind over both storage types. Retitle it "LogicalRestoreElasticsearch" and cut the relational half out of the body.

**Most of this PR is a lift.** The code ran, passed three lite Copilot rounds, a balanced round, and two orchestrator review passes on the closed branch `feat/restore-controllers--logicalrestore`, head `14a1545`. Read each source file before you adapt it. Do not rewrite from the description.

**Files:**
- Create (scaffold): `api/v1/logicalrestoreelasticsearch_types.go`, `internal/controller/logicalrestoreelasticsearch/`, `config/samples/core_v1_logicalrestoreelasticsearch.yaml`, `config/crd/bases/core.camunda.io_logicalrestoreelasticsearches.yaml`, the three `config/rbac/logicalrestoreelasticsearch_*_role.yaml` files
- Create: `pkg/esadmin/restore.go`, `pkg/esadmin/restore_test.go`
- Modify: `pkg/esadmin/client.go`, `pkg/esadmin/esadmintest/server.go`
- Modify: `pkg/components/elasticsearchcluster/snapshotstorage.go` (add `RepositoryConfigAt`)
- Create: `internal/controller/logicalrestoreelasticsearch/{controller.go,admit.go,compatibility.go,secondary.go,primary.go,suite_test.go,controller_test.go,world_test.go,schema_test.go}` plus the unit tests beside each file
- Modify: `pkg/labels/labels.go`, `pkg/labels/labels_test.go`, `pkg/restore/apply.go`, `pkg/restore/job.go` and their tests
- Modify: `pkg/clusterclaim/claim.go`, `pkg/clusterclaim/claim_test.go`
- Modify: `cmd/main.go`, `internal/controller/samples_schema_test.go`
- Modify: `internal/controller/logicalbackupelasticsearch/controller.go`, `internal/controller/logicalbackuprdbms/controller.go` (RBAC markers)
- Create: `docs/crds/logicalrestoreelasticsearch.md`
- Modify: `mkdocs.yml`, `docs/crds/index.md`, `docs/crds/camundaoptimize.md`, `docs/crds/pointintimerestore.md`

### Task 1: Scaffold the kind and write its API types

**Produces:** the `logicalrestoreelasticsearch-api` contract row.

- [ ] **Step 1: Scaffold**

```bash
kubebuilder create api --group core --version v1 --kind LogicalRestoreElasticsearch \
  --resource --controller
```

The CLI writes the types file, a controller stub under `internal/controller/`, the `PROJECT` entry, the kustomization entries, the sample, and the three role files. Never write any of them by hand. Move the controller stub into `internal/controller/logicalrestoreelasticsearch/` and leave every `// +kubebuilder:scaffold:` marker in place.

- [ ] **Step 2: Write the failing schema test**

Create `internal/controller/logicalrestoreelasticsearch/schema_test.go`, copying the shape of `internal/controller/pointintimerestore/schema_test.go`. Assert that the CRD rejects a spec change after creation, and that it rejects an empty `backupRef.name`.

- [ ] **Step 3: Write the types**

```go
// LogicalRestoreElasticsearchSpec names the backup to restore and the cluster
// to restore into. The whole spec is immutable: a restore is one operation,
// retried by creating a new resource.
type LogicalRestoreElasticsearchSpec struct {
	// BackupRef references the completed LogicalBackupElasticsearch to
	// restore from.
	// +required
	BackupRef LogicalBackupRef `json:"backupRef"`
	// TargetClusterRef references the CamundaCluster to restore into. It must
	// name the cluster the backup was taken from: the restore application
	// reads the primary-storage backup under the prefix of the cluster it
	// runs as. Issue #140 tracks a restore into a differently-named cluster.
	// The cluster must be suspended for the whole restore.
	// +required
	TargetClusterRef ClusterRef `json:"targetClusterRef"`
}

// LogicalRestoreElasticsearchStatus tracks the restore to a terminal phase.
type LogicalRestoreElasticsearchStatus struct {
	// Phase of the restore. It is the resume marker: a reconcile that
	// re-enters after a crash continues at the recorded phase.
	// +optional
	Phase LogicalRestorePhase `json:"phase,omitempty"`
	// BackupID is the backup id that the restore reads, pinned when the
	// restore starts. The backup can be deleted afterwards without moving
	// the restore to another set of artifacts.
	// +optional
	BackupID int64 `json:"backupId,omitempty"`
	// Repository is the Elasticsearch snapshot repository the restore reads
	// from, registered on the Elasticsearch of the target.
	// +optional
	Repository string `json:"repository,omitempty"`
	// RestoredSnapshots names every snapshot the restore asked Elasticsearch
	// to restore.
	// +optional
	RestoredSnapshots []string `json:"restoredSnapshots,omitempty"`
	// RestoreProgress is the part of the status that every restore kind has.
	RestoreProgress `json:",inline"`
}
```

Markers on the root type:

```go
// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:path=logicalrestoreelasticsearches,shortName=lres
// +kubebuilder:printcolumn:name="Phase",type=string,JSONPath=`.status.phase`
// +kubebuilder:printcolumn:name="Backup",type=string,JSONPath=`.spec.backupRef.name`
// +kubebuilder:printcolumn:name="Target",type=string,JSONPath=`.spec.targetClusterRef.name`
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`
```

The `Spec` field carries the immutability rule, as every one-shot kind in this API does:

```go
	// +required
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="spec is immutable: a restore is one-shot, retried by creating a new resource"
	Spec LogicalRestoreElasticsearchSpec `json:"spec"`
```

Add `GetStatusConditions`, `GetKind`, `SetObservedGeneration`, and `Terminal` with the same bodies the backup kinds use. `Terminal` reads `Phase` and reports true for `LogicalRestoreCompleted` and `LogicalRestoreFailed`.

- [ ] **Step 4: Register the kind everywhere a kind is registered**

| Where | What |
| --- | --- |
| `pkg/labels/labels.go` | `LogicalRestoreElasticsearchKey = "camunda.io/logical-restore-elasticsearch"` and `func LogicalRestoreElasticsearch(name string) Owner` |
| `pkg/restore/apply.go` | `FieldManagerLogicalRestoreElasticsearch client.FieldOwner = "camunda-operator/logicalrestoreelasticsearch"` |
| `pkg/restore/job.go` | `jobKindInfixes[labels.LogicalRestoreElasticsearchKey] = "lres"` |
| `pkg/clusterclaim/claim.go` | `holderKinds["LogicalRestoreElasticsearch"]` |
| `cmd/main.go` | the controller registration, with `Options{}` |
| `internal/controller/logicalbackupelasticsearch/controller.go` | add `logicalrestoreelasticsearches` to the claim-holder read marker |
| `internal/controller/logicalbackuprdbms/controller.go` | the same |
| `internal/controller/samples_schema_test.go` | `"core_v1_logicalrestoreelasticsearch.yaml"` |
| `mkdocs.yml` and `docs/crds/index.md` | the nav entry and the table row |

Extend `pkg/labels/labels_test.go`, `pkg/restore/apply_test.go`, `pkg/restore/job_test.go`, and `pkg/clusterclaim/claim_test.go` with the new kind. Add a golden Job case at `pkg/restore/testdata/golden/lres-broker-0.yaml`.

- [ ] **Step 5: Regenerate, run, and commit**

```bash
make manifests generate
go test ./api/... ./pkg/... ./internal/controller/... -v
git add -A
git commit -m "feat(api): add the LogicalRestoreElasticsearch kind (#111)"
```

### Task 2: Lift the Elasticsearch restore surface into `pkg/esadmin`

**Produces:** the `esadmin-restore-api` contract row.

**Source:** `pkg/esadmin/restore.go` (283 lines), `pkg/esadmin/restore_test.go` (317 lines), and the `pkg/esadmin/esadmintest/server.go` and `pkg/esadmin/client.go` changes, all at `14a1545`.

- [ ] **Step 1: Lift the files**

```bash
git checkout feat/restore-controllers--logicalrestore -- \
  pkg/esadmin/restore.go pkg/esadmin/restore_test.go \
  pkg/esadmin/esadmintest/server.go pkg/esadmin/client.go
```

Read every lifted file before the next step. Reconcile each against the base branch, which moved on after `14a1545`.

- [ ] **Step 2: Confirm the two hard-won properties survive**

These two cost a full review round each. A rewrite loses them, and the tests that prove them are in the lifted `restore_test.go`.

1. **`DeleteIndices` resolves patterns to concrete names before it deletes.** Elasticsearch defaults `action.destructive_requires_name` to true since 8.0, and refuses a wildcard delete. `ResolveIndices` therefore expands the patterns first, over open and closed indices, and the exact names never leave the client. The caller keeps naming patterns.
2. **The delete batches under a path budget.** `MaxDeletePathBytes` is `3 << 10`. Elasticsearch reads a request line of `http.max_initial_line_length` at most, which defaults to 4kb, and answers a longer one with a 400. A cluster with real history holds one `zeebe-record` index per day, so a set of a few months passes that bound. `deleteBatches` groups the names into as few requests as the budget allows.

- [ ] **Step 3: Run and commit**

Run: `go test ./pkg/esadmin/... -v`
Expected: PASS.

```bash
git add pkg/esadmin
git commit -m "feat(esadmin): delete indices and restore snapshots (#111)"
```

### Task 3: Admission, the cluster claim, and `ValidatingCompatibility`

**Source:** `internal/controller/logicalrestore/{controller.go,admit.go,compatibility.go,compatibility_test.go,suite_test.go,world_test.go}` at `14a1545`.

- [ ] **Step 1: Lift and adapt the controller and the admission**

Lift the four files into `internal/controller/logicalrestoreelasticsearch/`. Then adapt them:

- the resource type is `*v1.LogicalRestoreElasticsearch`
- every driver call goes to `pkg/restore`, not to a package-local copy. PR-A shipped `Outcome`, `HoldRunning`, `Recovered`, `Complete`, `Fail`, `StageTerminal`, `Primary`, `Take`, and `Give`
- `spec.backupRef` carries a name alone, so the admission reads a `LogicalBackupElasticsearch` and never switches on a kind
- the admission calls `restore.Take` after its last check passes, and maps a claimed cluster onto a `Pending` hold with `v1.ReasonClusterClaimed`
- both terminal transitions call `restore.Give`

- [ ] **Step 2: Cut the compatibility table down to the Elasticsearch rule**

`compatibility_test.go` is a pure table test. Keep the rows that apply and delete the relational rows:

| Case | Result |
| --- | --- |
| Elasticsearch backup, Elasticsearch target, same partitions, same version, same bucket | nil |
| the target's secondary storage is relational | `IncompatibleTarget`, the message names both types |
| partition counts differ | `IncompatibleTarget`, the message names both counts |
| the target bucket differs from the backup's pinned bucket | `IncompatibleTarget`, the message names both buckets |
| backup 8.9.9, target 8.9.9 | nil |
| backup 8.9.9, target 8.9.10 | `IncompatibleTarget`, the message states that Elasticsearch needs the exact version |
| backup 8.9.9, target 8.10.0 | `IncompatibleTarget` |
| either version is not `x.y.z` | `IncompatibleTarget`, the message names the unreadable version |
| the backup recorded no version | `IncompatibleTarget`, the message says that the backup recorded no Camunda version |

The source version comes from `status.version` on the backup, which #110 added. The target version comes from `restore.Target.Version`, which is the tag of the live broker image. Neither is readable from `status.management` while the cluster is suspended.

- [ ] **Step 3: Write the RBAC markers**

The markers of the old kind went with the scaffold controllers in #110. This PR carries its own:

```go
// +kubebuilder:rbac:groups=core.camunda.io,resources=logicalrestoreelasticsearches,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=core.camunda.io,resources=logicalrestoreelasticsearches/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=core.camunda.io,resources=logicalrestoreelasticsearches/finalizers,verbs=update
// +kubebuilder:rbac:groups=core.camunda.io,resources=camundaclusters;camundaclusterpresets;secondarystorageconfigs;elasticsearchclusters;objectstorageconfigs;logicalbackupelasticsearches,verbs=get;list;watch
// +kubebuilder:rbac:groups=apps,resources=statefulsets,verbs=get;list;watch
// +kubebuilder:rbac:groups=batch,resources=jobs,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="",resources=persistentvolumeclaims,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="",resources=secrets,verbs=get;list;watch
// +kubebuilder:rbac:groups="",resources=pods,verbs=list
// +kubebuilder:rbac:groups=coordination.k8s.io,resources=leases,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=events.k8s.io,resources=events,verbs=create;patch
```

The Lease verbs are what the cluster claim needs. The backup kinds carry the same block.

`SetupWithManager` indexes restores by the namespace and name of their `targetClusterRef`, watches `CamundaCluster` for a suspend flip, watches `LogicalBackupElasticsearch` for a phase change to `Completed`, owns Jobs, and is `Named("logicalrestoreelasticsearch")`. Prefix every field index name, because index names are manager-global.

- [ ] **Step 4: Run and commit**

Run: `go test ./internal/controller/logicalrestoreelasticsearch/... -v`
Expected: PASS.

```bash
git add internal cmd config
git commit -m "feat(logicalrestoreelasticsearch): admit a restore and refuse a target that cannot hold it (#111)"
```

### Task 4: `RestoringSecondaryStorage`

**Source:** `internal/controller/logicalrestore/secondary_elasticsearch.go` (274 lines) and `secondary_elasticsearch_test.go` (177 lines) at `14a1545`, plus `pkg/components/elasticsearchcluster/snapshotstorage.go`.

- [ ] **Step 1: Lift the files**

```bash
git checkout feat/restore-controllers--logicalrestore -- \
  internal/controller/logicalrestore/secondary_elasticsearch.go \
  internal/controller/logicalrestore/secondary_elasticsearch_test.go \
  pkg/components/elasticsearchcluster/snapshotstorage.go
git mv internal/controller/logicalrestore/secondary_elasticsearch.go \
  internal/controller/logicalrestoreelasticsearch/secondary.go
git mv internal/controller/logicalrestore/secondary_elasticsearch_test.go \
  internal/controller/logicalrestoreelasticsearch/secondary_test.go
rmdir internal/controller/logicalrestore
```

Rename the package to `logicalrestoreelasticsearch` and retype the receiver on `*v1.LogicalRestoreElasticsearch`.

- [ ] **Step 2: Confirm the two hard-won properties survive**

1. **The snapshot repository is registered only when it is absent, and it is never repointed.** `ensureRepository` calls `SnapshotRepositoryExists` first. A repository that is already registered is used as it is. The Elasticsearch of a target can be a cluster that this operator does not manage, where an operator registered the repository by hand. A blind PUT points that registration at another prefix of another bucket. A registration that points elsewhere makes the restore fail on a missing snapshot, which names the repository and which a human corrects.
2. **The restore waits for the indices to exist before it reads the recovery.** Right after Elasticsearch accepts the restore requests, the indices do not exist yet, and a recovery of nothing reads as a recovery that finished. `trackElasticsearchRestore` therefore calls `ResolveIndices` first, and only reads `RestoreProgress` once at least one index exists.

- [ ] **Step 3: Keep the envtest specs green**

The lifted `world_test.go` drives `esadmintest` from the suite. The specs it holds:

- the restore registers a snapshot repository derived from the backup's pinned `BucketRef` and prefix, and records its name in `status.repository`
- the restore deletes the Camunda indices before it restores anything
- the delete does not name the Optimize indices when the backup's `status.historySnapshots` holds no Optimize snapshot
- the delete does name them when the backup's snapshots hold one
- the restore then restores every history snapshot and the records snapshot that `logicalbackup.RecordsSnapshotName(status.backupId)` names, and records them in `status.restoredSnapshots`
- the restore stays in `RestoringSecondaryStorage` with reason `Progressing` while a recovery is active
- the restore moves to `RestoringPrimaryStorage` when the recovery is over
- an unreachable Elasticsearch holds the restore with reason `ConnectionFailed` for the mid-run grace, then fails it
- a re-entered reconcile does not delete the indices a second time, because `status.restoredSnapshots` is the resume marker

The delete-then-restore ordering is the risk the spec names. A failure between the two leaves the secondary storage of the target empty. The backup itself stays whole, so the retry converges. Keep the spec that proves the retry, and state the risk on the doc page.

- [ ] **Step 4: Run and commit**

```bash
git add pkg internal
git commit -m "feat(logicalrestoreelasticsearch): rebuild secondary storage from the snapshots (#111)"
```

### Task 5: `RestoringPrimaryStorage`, the docs, and the PR

- [ ] **Step 1: Write `primary.go` over the driver**

`primary.go` is a thin wrapper. It reads the target, builds a `restore.PrimaryInput` with `Args: []string{"--backupId=" + strconv.FormatInt(status.BackupID, 10)}`, calls `restore.Primary`, and maps the `Outcome` onto `v1.LogicalRestorePhase`. It holds no loop and no Job logic of its own. PR-A owns all of that.

The lifted `internal/controller/logicalrestore/primary.go` at `14a1545` is 407 lines. After the move onto the driver, the file is under 100. That reduction is the point of PR-A. If the wrapper grows past that, the shared piece belongs in `pkg/restore`, not here.

- [ ] **Step 2: Keep the envtest specs of the phase green**

- the restore records `status.brokers` before it deletes anything
- the restore deletes and creates every broker data claim, records them in `status.recreatedClaims`, and sizes them from the backup's `status.storageSizes.zeebe`
- a backup that recorded no Zeebe size gives the claims the size of the StatefulSet's claim template
- the recreated claims carry no owner reference to the restore
- the restore applies one Job per broker with `--backupId=<status.backupId>` and records them in `status.primaryJobNames`
- every Job carries `camunda.io/component: restore` and `camunda.io/logical-restore-elasticsearch: <name>`
- all Jobs complete, and the restore reaches `Completed` with `Ready=True`, reason `Completed`, and a `status.completionTime`
- one failing Job fails the restore with a message that names the broker
- deleting the restore removes its Jobs and leaves the claims in place
- a completed restore removes its claim Lease

The ninth spec protects the ownership rule and the tenth protects the claim. Write both first.

- [ ] **Step 3: Write `docs/crds/logicalrestoreelasticsearch.md`**

Load `simple-english:simple-english` first. Follow `docs/crds/TEMPLATE.md` and the conventions of `docs/crds/logicalbackupelasticsearch.md`. The page carries no "Not implemented yet" warning, because this PR ships the controller. It states:

- the phase list `Pending | ValidatingCompatibility | RestoringSecondaryStorage | RestoringPrimaryStorage | Completed | Failed`
- the compatibility rules: same secondary storage type, same partition count, same backup bucket, and the exact Camunda version
- that the target's `spec.backupStorageRef` must name the same `ObjectStorageConfig` the backup wrote to, because the restore reads the artifacts through the bucket of the target
- that the restore registers its own snapshot repository on the Elasticsearch of the target when none exists, and never repoints one that does
- that the restore Jobs copy the broker configuration from the live broker StatefulSet, and that a cluster whose broker StatefulSet was deleted cannot restore until its controller applies it again
- the PVC rule: the operator deletes and creates the broker data volumes again, sized from the backup's recorded restore size, and the volumes belong to the StatefulSet
- the `ClusterClaimed` hold and the `ClusterNotSuspended` hold
- the risk that a failure between the index delete and the snapshot restore leaves secondary storage empty until a retry succeeds

Add the nav entry to `mkdocs.yml` and the row to `docs/crds/index.md`, both under the shipped kinds and not under "Planned". Put the link back into `docs/crds/camundaoptimize.md` and `docs/crds/pointintimerestore.md`, which PR-A turned into plain text.

- [ ] **Step 4: Run all five gates**

```bash
go test ./...
make lint
make manifests generate && git status --porcelain config api
go vet -tags=e2e ./test/e2e/
mkdocs build --strict
```

- [ ] **Step 5: Commit and open the PR**

```bash
git add -A
git commit -m "feat(logicalrestoreelasticsearch): restore the Zeebe partitions and report completion (#111)"
```

Open the PR with `Towards #111`. Record in the body which files were lifted from `14a1545` and what changed in each. Run the review loop at the balanced level until clean. **Stop. Request the user's review. Do not merge.**

---

## PR-C — LogicalRestoreRDBMS (new sub-issue under #109, branch `feat/restore-controllers--lrrdbms`, worktree `.claude/worktrees/restore-controllers--lrrdbms`)

Implements the relational half of the old `LogicalRestore`, as its own kind. It runs in parallel with PR-B. The two touch disjoint files.

**File the sub-issue first.** Title it "LogicalRestoreRDBMS". Body: the relational half of the old #111, plus `Towards #109`.

**Most of this PR is a lift**, from the same closed branch and the same head, `14a1545`.

**Files:**
- Create (scaffold): `api/v1/logicalrestorerdbms_types.go`, `internal/controller/logicalrestorerdbms/`, `config/samples/core_v1_logicalrestorerdbms.yaml`, `config/crd/bases/core.camunda.io_logicalrestorerdbmses.yaml`, the three `config/rbac/logicalrestorerdbms_*_role.yaml` files
- Create: `internal/cli/download/{download.go,download_test.go}`
- Modify: `cmd/camunda-operator-cli/main.go`
- Modify: `pkg/objectstore/objectstore.go`, `pkg/objectstore/objectstore_test.go` (add `Bucket.Download`)
- Create: `pkg/components/logicalrestorerdbms/{job.go,job_test.go,testdata/golden/}`
- Create: `internal/controller/logicalrestorerdbms/{controller.go,admit.go,compatibility.go,secondary.go,primary.go,suite_test.go,controller_test.go,world_test.go,schema_test.go}` plus the unit tests beside each file
- Modify: `pkg/labels/labels.go`, `pkg/labels/labels_test.go`, `pkg/restore/apply.go`, `pkg/restore/job.go` and their tests
- Modify: `pkg/clusterclaim/claim.go`, `pkg/clusterclaim/claim_test.go`
- Modify: `cmd/main.go`, `internal/controller/samples_schema_test.go`
- Modify: `internal/controller/logicalbackupelasticsearch/controller.go`, `internal/controller/logicalbackuprdbms/controller.go` (RBAC markers)
- Create: `docs/crds/logicalrestorerdbms.md`
- Modify: `mkdocs.yml`, `docs/crds/index.md`

### Task 1: Scaffold the kind and write its API types

**Produces:** the `logicalrestorerdbms-api` contract row.

- [ ] **Step 1: Scaffold**

```bash
kubebuilder create api --group core --version v1 --kind LogicalRestoreRDBMS \
  --resource --controller
```

Move the controller stub into `internal/controller/logicalrestorerdbms/` and leave every scaffold marker in place.

- [ ] **Step 2: Write the failing schema test**

Create `internal/controller/logicalrestorerdbms/schema_test.go`, the same shape as PR-B's. Assert that the CRD rejects a spec change after creation, and that it rejects an empty `backupRef.name`.

- [ ] **Step 3: Write the types**

```go
// LogicalRestoreRDBMSSpec names the backup to restore and the cluster to
// restore into. The whole spec is immutable: a restore is one operation,
// retried by creating a new resource.
type LogicalRestoreRDBMSSpec struct {
	// BackupRef references the completed LogicalBackupRDBMS to restore from.
	// +required
	BackupRef LogicalBackupRef `json:"backupRef"`
	// TargetClusterRef references the CamundaCluster to restore into. It must
	// name the cluster the backup was taken from: the restore application
	// reads the primary-storage backup under the prefix of the cluster it
	// runs as. Issue #140 tracks a restore into a differently-named cluster.
	// The cluster must be suspended for the whole restore.
	// +required
	TargetClusterRef ClusterRef `json:"targetClusterRef"`
}

// LogicalRestoreRDBMSStatus tracks the restore to a terminal phase.
type LogicalRestoreRDBMSStatus struct {
	// Phase of the restore. It is the resume marker: a reconcile that
	// re-enters after a crash continues at the recorded phase.
	// +optional
	Phase LogicalRestorePhase `json:"phase,omitempty"`
	// BackupID is the Zeebe backup id that the restore reads, pinned when
	// the restore starts.
	// +optional
	BackupID int64 `json:"backupId,omitempty"`
	// SecondaryJobName is the Job that runs pg_restore, while it exists.
	// +optional
	SecondaryJobName string `json:"secondaryJobName,omitempty"`
	// RestoreProgress is the part of the status that every restore kind has.
	RestoreProgress `json:",inline"`
}
```

Markers:

```go
// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:path=logicalrestorerdbmses,shortName=lrrdbms
// +kubebuilder:printcolumn:name="Phase",type=string,JSONPath=`.status.phase`
// +kubebuilder:printcolumn:name="Backup",type=string,JSONPath=`.spec.backupRef.name`
// +kubebuilder:printcolumn:name="Target",type=string,JSONPath=`.spec.targetClusterRef.name`
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`
```

The `Spec` field carries the same immutability rule PR-B's does. Add `GetStatusConditions`, `GetKind`, `SetObservedGeneration`, and `Terminal`.

- [ ] **Step 4: Register the kind everywhere a kind is registered**

| Where | What |
| --- | --- |
| `pkg/labels/labels.go` | `LogicalRestoreRDBMSKey = "camunda.io/logical-restore-rdbms"` and `func LogicalRestoreRDBMS(name string) Owner` |
| `pkg/restore/apply.go` | `FieldManagerLogicalRestoreRDBMS client.FieldOwner = "camunda-operator/logicalrestorerdbms"` |
| `pkg/restore/job.go` | `jobKindInfixes[labels.LogicalRestoreRDBMSKey] = "lrrdbms"` |
| `pkg/clusterclaim/claim.go` | `holderKinds["LogicalRestoreRDBMS"]` |
| `cmd/main.go` | the controller registration, with `Options{CLIImage: cliImage}` |
| `internal/controller/logicalbackupelasticsearch/controller.go` | add `logicalrestorerdbmses` to the claim-holder read marker |
| `internal/controller/logicalbackuprdbms/controller.go` | the same |
| `internal/controller/samples_schema_test.go` | `"core_v1_logicalrestorerdbms.yaml"` |
| `mkdocs.yml` and `docs/crds/index.md` | the nav entry and the table row |

Extend `pkg/labels/labels_test.go`, `pkg/restore/apply_test.go`, `pkg/restore/job_test.go`, and `pkg/clusterclaim/claim_test.go`. Add a golden Job case at `pkg/restore/testdata/golden/lrrdbms-broker-0.yaml` with `Args: nil`.

- [ ] **Step 5: Regenerate, run, and commit**

```bash
make manifests generate
go test ./api/... ./pkg/... ./internal/controller/... -v
git add -A
git commit -m "feat(api): add the LogicalRestoreRDBMS kind (#<PR-C issue>)"
```

### Task 2: Lift the download subcommand and `Bucket.Download`

**Source:** `internal/cli/download/download.go` (109 lines), `download_test.go` (186 lines), the `cmd/camunda-operator-cli/main.go` change, and the `pkg/objectstore` change, all at `14a1545`.

- [ ] **Step 1: Lift the files**

```bash
git checkout feat/restore-controllers--logicalrestore -- \
  internal/cli/download cmd/camunda-operator-cli/main.go \
  pkg/objectstore/objectstore.go pkg/objectstore/objectstore_test.go
```

Read each file before you adapt it. The `download` subcommand reuses the `UPLOAD_STORAGE_*` environment contract of the existing `upload` subcommand, so the two read one contract and not two.

- [ ] **Step 2: Confirm the hard-won property survives**

**`Bucket.Download` propagates the Close error.** The close is part of the answer, not cleanup. A blob reader closes the transfer of its driver, and a driver that finds the transfer incomplete reports it there. `gocloud` returns the error of the driver from `blob.Reader.Close`. A caller that drops it reads a truncated archive as a whole one and hands it to `pg_restore`. The helper `drain` copies, then closes, then reports the first failure of either.

- [ ] **Step 3: Run and commit**

Run: `go test ./internal/cli/... ./pkg/objectstore/... -v`
Expected: PASS.

```bash
git add internal/cli cmd pkg/objectstore
git commit -m "feat(cli): download a backup object from a bucket (#<PR-C issue>)"
```

### Task 3: Lift the `pg_restore` Job builder

**Source:** `pkg/components/logicalrestore/job.go` (461 lines), `job_test.go` (449 lines), and the three golden cases at `14a1545`.

- [ ] **Step 1: Lift the package and rename it**

```bash
git checkout feat/restore-controllers--logicalrestore -- pkg/components/logicalrestore
git mv pkg/components/logicalrestore pkg/components/logicalrestorerdbms
```

Rename the package to `logicalrestorerdbms`. Retype `JobInput.Restore` on `*v1.LogicalRestoreRDBMS`. The three golden cases are `s3-credentials`, `s3-workload-identity`, and `scratch-pvc`. Regenerate them after the rename.

- [ ] **Step 2: Confirm the hard-won property survives**

**`pg_restore` connects as the application role, with `--no-owner`.** The application role owns the database and every object in it. `pg_restore --clean` drops each object before it recreates it, and PostgreSQL lets only the owner of an object drop it. The backup role that wrote the dump owns nothing: it holds USAGE and CREATE on the schema and DML on the tables, which `pkg/pgbootstrap.EnsureBackupUser` grants. A restore that connected as the backup role fails every DROP with "must be owner of table" and restores no data. This was proven against `postgres:17`, not reasoned about.

The credential contract that follows: the Job mounts `DatabaseConfig.spec.credentialsSecretRef`, and never the backup credentials. A Secret outside the namespace of the cluster is read through the local copy that the `CamundaCluster` controller maintains, under the mirror purpose `db-credentials` (`camundacluster.MirrorPurposeDBCredentials`).

The Job also uses the component name `pg-restore`, not `restore`, so the broker-Job selector of the primary phase stays broker-Jobs-only.

- [ ] **Step 3: Run and commit**

Run: `go test ./pkg/components/logicalrestorerdbms/... -v`
Expected: PASS, and the three goldens match.

```bash
git add pkg/components
git commit -m "feat(logicalrestorerdbms): restore a dump into the target database (#<PR-C issue>)"
```

### Task 4: Admission, the cluster claim, and `ValidatingCompatibility`

**Source:** the same four controller files at `14a1545` that PR-B lifts.

- [ ] **Step 1: Lift and adapt the controller and the admission**

The adaptation is PR-B's, with the relational type. The resource is `*v1.LogicalRestoreRDBMS`, the admission reads a `LogicalBackupRDBMS`, and every driver call goes to `pkg/restore`.

- [ ] **Step 2: Cut the compatibility table down to the relational rule**

| Case | Result |
| --- | --- |
| relational backup, relational target, same version, same bucket | nil |
| the target's secondary storage is Elasticsearch | `IncompatibleTarget`, the message names both types |
| the target bucket differs from the backup's pinned bucket | `IncompatibleTarget`, the message names both buckets |

A relational backup records no partition count, so the rules compare none. Only
`LogicalBackupElasticsearchStatus` carries `partitionsCount`. The restore application
reads the exporter position from the restored database and aligns the partitions itself.
| backup 8.9.9, target 8.9.9 | nil |
| backup 8.9.9, target 8.9.12 | nil, the patch level is free |
| backup 8.9.9, target 8.10.0 | nil, one minor newer is allowed |
| backup 8.9.9, target 8.11.0 | `IncompatibleTarget` |
| backup 8.10.0, target 8.9.9 | `IncompatibleTarget`, older is never allowed |
| either version is not `x.y.z` | `IncompatibleTarget`, the message names the unreadable version |
| the backup recorded no version | `IncompatibleTarget`, the message says that the backup recorded no Camunda version |

The relational rule is the one that differs from PR-B's. An Elasticsearch backup needs the exact Camunda version of the target. A relational backup accepts the same minor or one minor newer.

- [ ] **Step 3: Write the RBAC markers**

```go
// +kubebuilder:rbac:groups=core.camunda.io,resources=logicalrestorerdbmses,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=core.camunda.io,resources=logicalrestorerdbmses/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=core.camunda.io,resources=logicalrestorerdbmses/finalizers,verbs=update
// +kubebuilder:rbac:groups=core.camunda.io,resources=camundaclusters;camundaclusterpresets;secondarystorageconfigs;databaseconfigs;databaseserverconfigs;objectstorageconfigs;logicalbackuprdbmses,verbs=get;list;watch
// +kubebuilder:rbac:groups=apps,resources=statefulsets,verbs=get;list;watch
// +kubebuilder:rbac:groups=batch,resources=jobs,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="",resources=persistentvolumeclaims,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="",resources=secrets,verbs=get;list;watch
// +kubebuilder:rbac:groups="",resources=pods,verbs=list
// +kubebuilder:rbac:groups=coordination.k8s.io,resources=leases,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=events.k8s.io,resources=events,verbs=create;patch
```

`SetupWithManager` watches `LogicalBackupRDBMS` for a phase change to `Completed`, and is `Named("logicalrestorerdbms")`. Prefix every field index name.

- [ ] **Step 4: Run and commit**

```bash
git add internal cmd config
git commit -m "feat(logicalrestorerdbms): admit a restore and refuse a target that cannot hold it (#<PR-C issue>)"
```

### Task 5: `RestoringSecondaryStorage`

**Source:** `internal/controller/logicalrestore/secondary_rdbms.go` (517 lines) at `14a1545`.

- [ ] **Step 1: Lift the file**

```bash
git checkout feat/restore-controllers--logicalrestore -- \
  internal/controller/logicalrestore/secondary_rdbms.go
git mv internal/controller/logicalrestore/secondary_rdbms.go \
  internal/controller/logicalrestorerdbms/secondary.go
rmdir internal/controller/logicalrestore
```

Rename the package and retype the receiver on `*v1.LogicalRestoreRDBMS`.

- [ ] **Step 2: Confirm the hard-won properties survive**

1. **The Job is created, never force-applied.** A forced apply after a NotFound is not atomic. It overwrites the UID label and the owner reference of a same-named Job that lands in between, before the adoption check looks at them. A `Create` is atomic, so the API server decides who owns the name. The dump Job of a relational backup follows the same reasoning.
2. **A Job under this restore's name that carries another UID fails the restore.** Its completion lets the restore advance without a restore of its own database. `JobBelongsTo` is the check, and `RestoreUIDLabel` is what it reads.
3. **A recorded Job that is gone fails the restore.** The logical database then holds a partial restore that only a new attempt repairs.
4. **Every look resolves the target again**, the look that tracks a running Job included. The Job rewrites the database while it runs, so a cluster that is unsuspended under it must reach the restore. The mid-run grace bounds how long that holds it.

- [ ] **Step 3: Keep the envtest specs green**

- the restore applies exactly one `pg_restore` Job and records it in `status.secondaryJobName`
- a completed Job moves the restore to `RestoringPrimaryStorage`
- a failed Job fails the restore with a message that names the Job
- a pod that cannot start, for example on a missing Secret, reports `MissingSecret` through the mid-run grace and then fails, through `podstate.Stuck`
- a Job under the same name with another UID fails the restore

- [ ] **Step 4: Run and commit**

```bash
git add internal
git commit -m "feat(logicalrestorerdbms): rebuild the logical database from the dump (#<PR-C issue>)"
```

### Task 6: `RestoringPrimaryStorage`, the docs, and the PR

- [ ] **Step 1: Write `primary.go` over the driver**

The same thin wrapper PR-B writes, with `Args: nil`. The relational path runs the restore application with no arguments, because Zeebe's continuous primary-storage backup carries the checkpoint.

- [ ] **Step 2: Keep the envtest specs of the phase green**

The set is PR-B's, with two differences. The Jobs carry `camunda.io/logical-restore-rdbms: <name>` and no arguments. The claims size from the backup's `status.storageSizes.zeebe` the same way.

- [ ] **Step 3: Write `docs/crds/logicalrestorerdbms.md`**

Load `simple-english:simple-english` first. Follow `docs/crds/TEMPLATE.md` and the conventions of `docs/crds/logicalbackuprdbms.md`. The page carries no "Not implemented yet" warning. It states:

- the phase list `Pending | ValidatingCompatibility | RestoringSecondaryStorage | RestoringPrimaryStorage | Completed | Failed`
- the compatibility rules: same secondary storage type, same backup bucket, and the same Camunda minor or one minor newer. There is no partition-count rule, because a relational backup records no partition count
- that the `pg_restore` Job connects as the application role of the cluster, which is the role that owns every object it drops and recreates
- that the restore Jobs copy the broker configuration from the live broker StatefulSet
- the PVC rule and the volume ownership rule
- the `ClusterClaimed` hold and the `ClusterNotSuspended` hold

Add the nav entry and the index row, both under the shipped kinds.

- [ ] **Step 4: Run all five gates**

```bash
go test ./...
make lint
make manifests generate && git status --porcelain config api
go vet -tags=e2e ./test/e2e/
mkdocs build --strict
```

- [ ] **Step 5: Commit and open the PR**

```bash
git add -A
git commit -m "feat(logicalrestorerdbms): restore the Zeebe partitions and report completion (#<PR-C issue>)"
```

Open the PR with `Towards #<PR-C issue>`. Record in the body which files were lifted from `14a1545` and what changed in each. Run the review loop at the balanced level until clean. **Stop. Request the user's review. Do not merge.**

### Review checkpoint after PR-B and PR-C

Run `feature-dev-workflow:reviewing-feature-progress` over the feature branch once both merge. The three restore controllers must read as one author's work: same reconciler shape, same `Options` fields, same hold-and-grace vocabulary, same phase-is-the-resume-marker rule, and the same thin wrapper over `restore.Primary`. Fix any drift before PR-D starts, in a small follow-up commit on the feature branch.

Regenerate `api/v1/zz_generated.deepcopy.go` after the second merge. It is the one file both PRs touch.

---

## PR-D — The restore e2e suite (#113, branch `test/restore-controllers--e2e`, worktree `.claude/worktrees/restore-controllers--e2e`)

Implements the spec's "e2e scope" decision, over both new logical restore kinds.

The spec asks for a full round trip for each kind. Task 2 covers `LogicalRestoreElasticsearch` and Task 3 covers `LogicalRestoreRDBMS`. Task 4 covers the operator side of `PointInTimeRestore`, with no WAL replay in kind.

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
// itRestoresTheElasticsearchCluster registers the LogicalRestoreElasticsearch
// round trip of an Elasticsearch-backed cluster. It takes its own backup, so
// it never depends on the ordering of the backup specs.
func itRestoresTheElasticsearchCluster(cluster *v1.CamundaCluster, elasticsearch, storageConfig string)
```

Ordered specs inside it:

1. *takes the backup the restore will read* — applies `LogicalBackupElasticsearch` named `camunda-es-restore-source`, waits `Ready/Completed` within `backupTimeout`, and records `status.backupId`.
2. *suspends the cluster* — `suspend(cluster)`.
3. *wipes secondary storage* — resolves the Camunda index patterns to concrete names first, then deletes the names. Elasticsearch defaults `action.destructive_requires_name` to true since 8.0, so a wildcard delete from the suite is refused. The patterns are the ones `logicalbackup.CamundaIndexPatterns` produces. Read the index list with `curlElasticsearch(contract, "list-indices", "/<patterns>?expand_wildcards=open,closed", "-XGET")`, then delete the names in one call.
4. *restores the backup* — applies `LogicalRestoreElasticsearch{BackupRef:{Name: "camunda-es-restore-source"}, TargetClusterRef:{Name: cluster.Name}}` named `camunda-es-restore`, waits `Ready/Completed` within `restoreTimeout`, and asserts `status.phase == v1.LogicalRestoreCompleted`, a non-empty `status.restoredSnapshots`, and one entry in `status.primaryJobNames` per broker.
5. *unsuspends the cluster and finds the seeded instance again* — `unsuspend(cluster)` then `expectInstanceSearchable(cluster)`.

`spec.backupRef` carries a name alone. The kind of the restore says which backup kind it reads.

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
// itRestoresTheRelationalCluster registers the LogicalRestoreRDBMS round trip
// of a relational cluster.
func itRestoresTheRelationalCluster(cluster *v1.CamundaCluster)
```

Ordered specs:

1. *takes the backup the restore will read* — applies `LogicalBackupRDBMS` named `camunda-rdbms-restore-source`, waits `Ready/Completed`, records `status.objectKey` and `status.zeebeBackupId`.
2. *suspends the cluster* — `suspend(cluster)`.
3. *wipes the logical database* — through `psql(rdbmsNamespace, "wipe", adminRef, "DROP SCHEMA public CASCADE; CREATE SCHEMA public;")` as the server administrator, then re-grants the application user with `GRANT ALL ON SCHEMA public TO camunda;`. The restore Job runs `pg_restore --clean --if-exists --no-owner` as the application role, which recreates every object the dump holds.
4. *restores the backup* — applies `LogicalRestoreRDBMS{BackupRef:{Name: "camunda-rdbms-restore-source"}, TargetClusterRef:{Name: cluster.Name}}`, waits `Ready/Completed`, asserts a non-empty `status.secondaryJobName` and one entry in `status.primaryJobNames` per broker.
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

1. *accepts a timestamp at the current database state* — applies a second `PointInTimeRestore` with `Timestamp: metav1.Now()`, and waits until `status.phase` reaches `RestoringPrimaryStorage` or `Completed`. That is the proof that the pre-check passed. Do not wait for the phase to leave `ValidatingDatabaseState`. #122 records that the phase is visible only while the database is unreachable, because the admission falls through in one reconcile.
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
make lint
go vet -tags=e2e ./test/e2e/
```

```bash
git add test/e2e
git commit -m "test(e2e): prove the point-in-time restore refuses an unrestored database (#113)"
```

Open the PR with `Towards #113`. Run the review loop at the balanced level until clean. Self-merge into `feat/restore-controllers` and close #113.

---

## PR-E — Integration: `feat/restore-controllers` into `main` — the user reviews

- [ ] Run the `feature-dev-workflow:reviewing-feature-progress` checkpoint over the merged feature branch
- [ ] Run all five verification gates below
- [ ] Confirm the full e2e suite passes in CI, not only the focused containers
- [ ] Confirm that `grep -rniI --exclude-dir=.git --exclude-dir=superpowers 'logicalrestore\b\|LogicalRestore\b' .` finds no line. The kind is gone, and the two new kinds carry their own names
- [ ] Open the integration PR with `Closes #109`, run the review loop until clean, **stop, and request the user's review**
- [ ] After the user approves, merge, then delete the plan and the state file in the last commit and update memory

## Verification commands

Run these at every task boundary, and all five before every PR opens. All five are required. Each one catches a class the others miss.

```bash
go test ./...                       # both modules, through the Makefile MODULES loop
make lint                           # must print 0 issues, both modules
make manifests generate             # must leave the tree clean
git status --porcelain config api   # must print nothing
go vet -tags=e2e ./test/e2e/        # the e2e package is behind a build tag
mkdocs build --strict               # the docs must build with no warning
```

Two of them exist because CI caught what the local loop did not:

- **`make all` does not lint.** The Makefile has `all: build`. The CLAUDE.md comment claims otherwise and is wrong. Run `make lint` and read its output. A PR that claims a clean lint from `make all` claims nothing.
- **`go test ./...` never compiles `test/e2e`.** That package sits behind the `e2e` build tag. A symbol that moves or is renamed breaks CI while every local gate stays green. #120 moved `RecordsSnapshotName` and broke the base branch of two open PRs this way. `go vet -tags=e2e ./test/e2e/` is the cheap check that catches it.

The e2e suite is not part of that loop. It runs with:

```bash
make test-e2e                       # creates and deletes its own kind cluster
```

## Resolved questions

The orchestrator resolved these on 2026-08-20 with the user AFK and full autonomy granted. Each resolution takes the plan's own safe direction. Raise a bubble-up entry if implementation contradicts one.

- **Cross-cluster Elasticsearch restore requires the same backup bucket.** Confirmed. The target's `spec.backupStorageRef` must point at a bucket that contains the source backup's artifacts. A target with a different bucket is out of scope for this epic. `docs/crds/logicalrestoreelasticsearch.md` carries the rule.
- **A backup with no recorded version fails with `IncompatibleTarget`.** Confirmed. This is a clean-slate project, so the restore fails closed instead of warning.
- **The accepting PointInTimeRestore e2e spec uses the fallback assertion** when no checkpoint exists at or before the timestamp. Confirmed. The e2e stays short.
- **`DROP SCHEMA public CASCADE` is the relational wipe, and the spec re-grants the application user by hand.** Confirmed.

Resolved on 2026-08-20, for the amended breakdown:

- **`Outcome.Failure` is a `*conditions.PreCheckFailure`.** The spec sketches a `*Failure` type of the same two fields. The repository already returns `*conditions.PreCheckFailure` from every pre-check, so a second type of the same shape earns nothing. Recorded as a deviation in PR-A's body.
- **`LogicalBackupRef` keeps a name alone and lives in `api/v1/restore_shared.go`.** The kind of the restore says which backup kind it reads, so `Kind` has no reader. Both logical restore specs use the same ref type, as both use the same `ClusterRef`.
- **The two logical kinds share `v1.LogicalRestorePhase`.** Their phase values are identical, and the backup pair shares `LogicalBackupPhase` on the same reasoning. `PointInTimeRestore` keeps its own phase type, because its values differ.
- **`PointInTimeRestoreStatus.ClusterUID` is renamed to `TargetClusterUID`.** The spec's unification table resolves the divergence in favour of the logical kind's name. The kind is unreleased, so no conversion is needed.
- **PR-B and PR-C lift their code from `14a1545` instead of writing it again.** That code passed four Copilot rounds and two orchestrator review passes. A rewrite risks losing the four properties that each cost a review round to find. Each lift task names them.

## Open questions

- ~~**`PROJECT` has no supported way to remove a kind.**~~ RESOLVED 2026-08-20 (1734224). AGENTS.md now carries a "One exception: removing a kind" block under *Never Edit These*. It names the hand edit, the files that go with it, and the duty to say so in the PR body. PR-A follows that rule instead of asking again.
- **The spec header names two CRD pages that do not exist yet.** `docs/crds/logicalrestoreelasticsearch.md` and `docs/crds/logicalrestorerdbms.md` arrive in PR-B and PR-C. Between PR-A and those PRs, no logical restore page exists, and `mkdocs build --strict` fails on any link into one. PR-A therefore removes the links and PR-B and PR-C put them back. An alternative is for PR-A to ship both pages as stubs. The plan takes the first path, because the task breakdown puts each page in the PR that ships its kind.
- **The `pkg/restore` package name is now wider than "restore rendering".** PR-A rewrites `pkg/restore/doc.go` to say so. If the package grows past one screen of files, split the driver into `pkg/restore/driver` in a follow-up. Do not split it inside this epic.
