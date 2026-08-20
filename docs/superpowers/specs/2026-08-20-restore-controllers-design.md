# Restore controllers: LogicalRestoreElasticsearch, LogicalRestoreRDBMS, and PointInTimeRestore

- Date: 2026-08-20
- Status: draft
- Detailed design record: `docs/crds/logicalrestoreelasticsearch.md`,
  `docs/crds/logicalrestorerdbms.md`, and `docs/crds/pointintimerestore.md`

## Problem

The operator takes backups but cannot restore them. The restore kinds are empty kubebuilder stubs:
the API types hold a `Foo` field and the reconcilers return immediately.
The backup epic (#64) stated its own limit: it proves that backup artifacts exist, and it defers the
proof that a backup restores to this work. Until a restore path exists, every backup the operator
takes is unverified.

## Goals

- Implement one logical restore kind for each secondary storage type, `LogicalRestoreElasticsearch`
  and `LogicalRestoreRDBMS`, as their two pages under `docs/crds/` describe.
- Implement the `PointInTimeRestore` API and controller, as `docs/crds/pointintimerestore.md`
  describes, plus the database-state precheck below.
- Prove restorability end to end: an e2e test that takes a backup, wipes the cluster state, restores
  it, and verifies the data, on both secondary storage paths.
- Verify the safety properties before every destructive step. The controllers delete Zeebe PVCs, so
  each destructive phase must sit behind explicit validation phases.

## Non-goals

- Restore of the database server itself. PostgreSQL point-in-time recovery needs host-level access
  or a provider API. That belongs to the composition layer above. This decision is recorded in
  `docs/crds/pointintimerestore.md`.
- Writing `spec.suspend` on the target `CamundaCluster`. The restore controllers only read the
  suspend state. You or the composition layer suspend the cluster before a restore and unsuspend it
  after.
- Camunda's `--allow-version-mismatch` escape hatch. The API does not expose it.
- A real WAL-replay PITR e2e test in kind. See "e2e scope" below.
- `BackupSchedule` (#70). Another session owns it, and it blocks nothing here.

## Design overview

The CRD design docs are the detailed design record. All are verified against Camunda 8.9 and record
their deviations from the original proposal. This spec adds the mechanisms on top of them and fixes
the scope of the tests.

`LogicalRestoreElasticsearch` and `LogicalRestoreRDBMS` restore one completed logical backup into a
suspended target cluster, on the same cluster or a different one. Each kind validates the target and
the backup reference, checks compatibility (storage type, partition count, version rule), restores
secondary storage, recreates the Zeebe PVCs, and runs Camunda's standalone restore application once
per broker as a Job. The Elasticsearch kind restores secondary storage through the snapshot API. The
relational kind runs a `pg_restore` Job.

`PointInTimeRestore` aligns an RDBMS-backed cluster's primary storage with a database that was
already restored to a timestamp, in place. The controller validates the suspend state, the storage
chain, the PITR capability declaration, and the dedicated-server rule. Then it recreates the Zeebe
PVCs and runs the restore application with `--to=<spec.timestamp>` once per broker.

## Decision: one restore kind for each secondary storage type

`LogicalRestore` was one kind for both secondary storage types. It is now two kinds,
`LogicalRestoreElasticsearch` and `LogicalRestoreRDBMS`. Epic #64 split the backups on the same
criterion, and these two mirror that pair.

The single kind carried a union. Three status fields were live on one path and unset on the other:
`repository` and `restoredSnapshots` on the Elasticsearch path, `secondaryJobName` on the relational
path. Two more fields existed only to say which path ran, `spec.backupRef.kind` and
`status.storageType`. The version rule also differs per path. An Elasticsearch backup needs the
exact Camunda version of the target, and a relational backup accepts the same minor or one minor
newer. The two procedures share no code.

The kind now says which path runs, so both discriminator fields are gone. `spec.backupRef` keeps
only a name, like `spec.targetClusterRef`. Each status holds the fields of its own path alone.

`PointInTimeRestore` stays one kind. It reads no backup, and it runs on the relational path only.

## Decision: the shared restore machinery lives in pkg/restore

The three restore kinds share their admission, their pinning, their mid-run grace, their terminal
transitions, and the whole primary-storage phase. Only the secondary-storage phase differs, and
`PointInTimeRestore` has none.

`pkg/restore` holds this machinery, in the role that `pkg/logicalbackup` has for the backup pair. It
renders and applies as before, and it now also drives the phases that every restore kind shares. Its
package comment records the wider scope.

The driver takes a request struct and returns an outcome, like `logicalbackup.PreCheck`. It needs no
type parameters, because the status fields that every kind shares live in one embedded struct:

```go
// api/v1/restore_shared.go
type RestoreProgress struct { ... }  // embedded with json:",inline" in all three statuses
```

The driver reads and writes `*v1.RestoreProgress` in place. The JSON form of each status does not
change, because controller-gen flattens an inline embedded struct.

The driver never writes `status.phase`. Each kind owns its own phase vocabulary, so the driver
returns an outcome and the controller maps it:

```go
type Outcome struct {
    Wait    time.Duration  // zero means that the watches carry the wake-up
    Done    bool           // the phase finished
    Failure *Failure       // terminal: a reason and a message
}
```

`PointInTimeRestore` and `LogicalRestore` grew as separate controllers, and their copies of the
shared machinery diverged. The unification resolves each divergence in one direction:

| Behavior | Kept from | Reason |
| --- | --- | --- |
| Record `primaryJobNames` before the Jobs are applied | `PointInTimeRestore` | The names are durable before a Job exists, so the record covers what the next look applies |
| Refuse an ordinal past the live broker count | `PointInTimeRestore` | It names the real cause. The render error underneath reports only counts |
| Fail when a recorded Job is gone | `LogicalRestore` | A second Job runs the restore application on a volume that the first one wrote |
| One event for each broker that starts | `PointInTimeRestore` | It is the only per-broker signal that the user gets |
| `fail` takes a reason | `LogicalRestore` | The logical kinds report `IncompatibleTarget`. `PointInTimeRestore` passes `Failed` |
| The field name `targetClusterUID` | `LogicalRestore` | `PointInTimeRestore` called it `clusterUID`. One thing has one name |

## Decision: a restore claims its cluster

`pkg/clusterclaim` holds one Lease for each cluster. Both backup kinds take it. No restore kind took
it, so two restores of one cluster can run together and erase the volumes of each other.

Every restore kind now takes the claim when admission passes, and gives it back at the terminal
transition. Completed and Failed both give it back. The claim point is the same for all three kinds,
and it comes before every phase that touches storage. Two restores of one cluster can therefore
never both pass validation.

A cluster that another holder claims holds the restore in `Pending` with the reason
`ClusterClaimed`. Nothing bounds this hold, and the restore starts on its own when the holder
reaches a terminal phase. The holder can be a backup or another restore, so the reason names no
kind. The backup pair reports `BackupInProgress`, which names one.

## Decision: the database-state precheck for PointInTimeRestore

### The gap

As first designed, the controller deletes the Zeebe PVCs and then runs the restore Jobs. Camunda's
restore application rejects a `--to` timestamp that lies before the restored state of the database.
That check is real (verified in the 8.9 restore docs), but it fires inside the Jobs — after the
operator has already erased primary storage. A skipped or wrong database restore then leaves the
cluster with no local Zeebe state.

### The mechanism

The controller gains a `ValidatingDatabaseState` phase that runs before any destructive step:

1. Connect to the logical database with the cluster's application credentials, resolved through the
   existing storage chain (`storageRef` → `SecondaryStorageConfig` → `DatabaseConfig`).
2. Read `LAST_UPDATED` for every partition from the `EXPORTER_POSITION` table.
3. If a partition row is missing, or if any `LAST_UPDATED` is after `spec.timestamp` plus a fixed
   slack for clock skew (one minute), the database is ahead of the requested point. The restore holds in `Pending` with
   `Ready: DatabaseNotRestored` and never touches the PVCs.

The operator already speaks PostgreSQL (`pkg/pgbootstrap`), so the precheck adds no new dependency.

### The limits, stated in the docs

The precheck proves that the database is not ahead of `spec.timestamp`. It cannot prove that the
database was restored to exactly that timestamp. A database restored to an earlier point passes the
precheck, and that is safe: Zeebe re-exports the difference after the restore. The restore
application's own check stays the authoritative gate. The precheck only moves the common failure
before the PVC deletion.

`docs/crds/pointintimerestore.md` gains this phase and these limits in the same change.

### Alternative considered: a probe restore Job

A restore-application Job against a throwaway `emptyDir` volume would exercise the authoritative
check itself before the PVC deletion. Rejected: the probe downloads and restores a full backup only
to discard it, which is slow and expensive on every restore. The SQL precheck is one query.

## Decision: restore Jobs mirror the live broker StatefulSet

The restore-application Jobs do not re-render the broker configuration. The controller reads the
target's live broker StatefulSet — it still exists while the cluster is suspended — and copies the
broker container's environment, volume mounts, and resources into the Job pods. The restore
application then always runs with the configuration the brokers themselves use, and the two cannot
drift.

The Job pods also copy the broker pods' labels and topology spread constraints. With
`WaitForFirstConsumer` storage classes, the first pod that binds a PVC pins its zone. Because the
restore Jobs spread like broker pods, the recreated PVCs land in zones that the brokers can
schedule into after the restore.

The copy of a spread constraint keeps the broker's topology key, skew, and policies, but its
selector points at the restore's own pods. An operator label always wins over a copied one, so a
restore pod carries `camunda.io/component: restore`. A constraint that still selected the broker
component would count no pod at all, and every restore pod could land in one zone.

This decision adapts the restore-mode mechanism of Camunda's SaaS operator, which mirrors the
broker container into an init container on the StatefulSet itself. That operator owns the cluster
spec, so it can flip the StatefulSet into a restore mode and roll it back. This operator does not
own the cluster spec, so it copies from the StatefulSet instead of rewriting it. The copy buys the
same two properties (configuration mirroring and scheduler-correct PVC zones) without a restore
mode on `CamundaCluster` and without the quorum-sensitive exit that a restore-mode StatefulSet
needs.

## Layering note: suspend orchestration belongs to the composition layer

Camunda's SaaS operator wraps the restore in a spec-writing pipeline: it suspends the cluster,
locks the spec against concurrent writers, records the prior state for its abort path, and resumes
the cluster afterward. In this stack, that whole role belongs to `camunda-cloud-operator`'s future
recovery flow, because that layer owns the `CamundaCluster` spec. The restore CRs in this operator
are the mechanism that such a pipeline drives. They stay safe without one: the suspend validation
holds the restore in `Pending` until the owner has suspended the cluster.

## Decision: e2e scope

- `LogicalRestoreElasticsearch` and `LogicalRestoreRDBMS`: a full round trip for each kind, in the
  existing kind + MinIO harness. Seed data into a cluster, take a backup, wipe the state, restore,
  unsuspend, and verify that the seeded data is visible again. This closes the verification gap that
  the backup epic left open.
- `PointInTimeRestore`: the operator's side only, with no WAL replay in kind. The test treats the
  live database as "already restored". It creates one restore with a timestamp before the database
  state and verifies the `DatabaseNotRestored` refusal. It then creates one restore with a valid
  timestamp and verifies that the restore Jobs run and the cluster converges.

A real WAL-replay PITR e2e needs archive-mode PostgreSQL and a recovery harness inside kind. That
cost is not justified now. If it becomes justified, it is a follow-up issue, not part of this batch.

## Consequences

- The backup story closes: backups are proven restorable by CI, not assumed.
- `PointInTimeRestore` fails fast and non-destructively when the database was not restored, which is
  the most likely operator error in the flow.
- Each kind writes under its own field manager: `camunda-operator/logicalrestoreelasticsearch`,
  `camunda-operator/logicalrestorerdbms`, and `camunda-operator/pointintimerestore`. The recreated
  broker volumes go through SSA. The Jobs do not. A Job is created once, as an identity claim, and
  the API server decides who owns the name. A read and a forced apply after it are two calls, and a
  Job of an earlier restore can land between them.
- One restore of a cluster runs at a time, across all three restore kinds and both backup kinds.

## Risks

- The `LAST_UPDATED` comparison trusts the database clock relative to `spec.timestamp`. A large
  clock skew between the exporter's writes and the caller's timestamp source can pass or hold a
  restore incorrectly. The slack bounds this, and the restore application still catches the unsafe
  direction.
- `LogicalRestoreElasticsearch` deletes the target's Camunda indices before the snapshot restore. A
  restore that fails between the delete and the snapshot restore leaves secondary storage empty
  until a retry succeeds. The backup itself stays intact, so the retry path is safe.
- The unification rewrites the destructive primary-storage phase of `PointInTimeRestore` after that
  controller merged. Its envtest suite is the safety net, and it stays as it is. A test that the
  unification changes names the resolved divergence that made the change necessary.
- Camunda's restore application is a one-shot app that this operator has not run before. The Job
  pods copy their configuration from the live broker StatefulSet, which removes the drift risk, but
  the copy itself is new code. The version matrix tests and the e2e round trip carry this risk.
- The copy from the live StatefulSet makes the restore depend on the StatefulSet's presence. A
  cluster whose StatefulSet was deleted cannot restore until the CamundaCluster controller applies
  it again. The suspend semantics keep the StatefulSet in place, so this is an edge, not a flow.
