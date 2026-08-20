# Restore controllers: LogicalRestore and PointInTimeRestore

- Date: 2026-08-20
- Status: draft
- Detailed design record: `docs/crds/logicalrestore.md` and `docs/crds/pointintimerestore.md`

## Problem

The operator takes backups but cannot restore them. `LogicalRestore` and `PointInTimeRestore` are
empty kubebuilder stubs: the API types hold a `Foo` field and the reconcilers return immediately.
The backup epic (#64) stated its own limit: it proves that backup artifacts exist, and it defers the
proof that a backup restores to this work. Until a restore path exists, every backup the operator
takes is unverified.

## Goals

- Implement the `LogicalRestore` API and controller for both secondary storage types, as
  `docs/crds/logicalrestore.md` describes.
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

The two CRD design docs are the detailed design record. Both are verified against Camunda 8.9 and
record their deviations from the original proposal. This spec adds one mechanism on top of them and
fixes the scope of the tests.

`LogicalRestore` restores a completed `LogicalBackupElasticsearch` or `LogicalBackupRDBMS` into a
suspended target cluster, on the same cluster or a different one. The controller validates the
target and the backup reference, checks compatibility (storage type, partition count, version rule),
restores secondary storage (Elasticsearch snapshot API, or a `pg_restore` Job), recreates the Zeebe
PVCs, and runs Camunda's standalone restore application once per broker as a Job.

`PointInTimeRestore` aligns an RDBMS-backed cluster's primary storage with a database that was
already restored to a timestamp, in place. The controller validates the suspend state, the storage
chain, the PITR capability declaration, and the dedicated-server rule. Then it recreates the Zeebe
PVCs and runs the restore application with `--to=<spec.timestamp>` once per broker.

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

- `LogicalRestore`: a full round trip on both secondary storage paths, in the existing kind + MinIO
  harness. Seed data into a cluster, take a backup, wipe the state, restore, unsuspend, and verify
  that the seeded data is visible again. This closes the verification gap that the backup epic left
  open.
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
- Both controllers apply their Jobs with SSA under their own field managers
  (`camunda-operator/logicalrestore`, `camunda-operator/pointintimerestore`), consistent with the
  rest of the operator.

## Risks

- The `LAST_UPDATED` comparison trusts the database clock relative to `spec.timestamp`. A large
  clock skew between the exporter's writes and the caller's timestamp source can pass or hold a
  restore incorrectly. The slack bounds this, and the restore application still catches the unsafe
  direction.
- The Elasticsearch path deletes the target's Camunda indices before the snapshot restore. A restore
  that fails between the delete and the snapshot restore leaves secondary storage empty until a
  retry succeeds. The backup itself stays intact, so the retry path is safe.
- Camunda's restore application is a one-shot app that this operator has not run before. The Job
  pods copy their configuration from the live broker StatefulSet, which removes the drift risk, but
  the copy itself is new code. The version matrix tests and the e2e round trip carry this risk.
- The copy from the live StatefulSet makes the restore depend on the StatefulSet's presence. A
  cluster whose StatefulSet was deleted cannot restore until the CamundaCluster controller applies
  it again. The suspend semantics keep the StatefulSet in place, so this is an edge, not a flow.
