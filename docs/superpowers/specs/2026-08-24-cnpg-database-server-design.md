# DatabaseServer: PostgreSQL through CloudNativePG, and point-in-time restore on it

**Status:** draft for review
**Date:** 2026-08-24
**Epic:** #127, with #128 as its first sub-issue
**Scope:** a `DatabaseServer` kind and its `DatabaseServerPreset`, a CloudNativePG `Cluster`
wrapper and a Barman Cloud `ObjectStore` wrapper, `DatabaseServerConfig` and `Database` moved to
namespace scope, server identity by PostgreSQL system identifier, `PointInTimeRestore` driving
the database recovery when a `DatabaseServer` runs the server

## Summary

The operator runs Elasticsearch through ECK but cannot run PostgreSQL. A user who wants an
RDBMS-backed orchestration cluster brings a server, and point-in-time restore stops at the
database: `PointInTimeRestore` requires that the database is already recovered to the target
time before the CR exists, because the base backups and the WAL archive belong to whoever runs
the server.

`DatabaseServer` closes both gaps. It is a namespaced kind that creates a CloudNativePG
`Cluster`, archives its WAL and base backups to an `ObjectStorageConfig` through the Barman
Cloud plugin, and publishes a `DatabaseServerConfig` whose `pitr` block states what the server
can do. Because the operator controls the archive, `PointInTimeRestore` can ask the
`DatabaseServer` to recover the server to the target time, and then continue with the primary
storage restore it already performs.

The RDBMS chain becomes namespace-local: `DatabaseServerConfig` and `Database` move to namespace
scope and `Database.spec.targetNamespace` goes away. A server is identified by the system
identifier PostgreSQL reports, not by the name of a contract, so the rules that depend on server
identity hold when two namespaces describe one server.

The work ships as one epic with sub-PRs on `feat/cnpg-database-server`. The PR breakdown and the
contracts between PRs live in the plan, not here.

## Verified facts

The CloudNativePG facts below were read on 2026-08-24 from the pages named. Implementation
verifies every field name against the API reference of the pinned CloudNativePG version before it
writes it.

### Backup and the Barman Cloud plugin

- In-tree `spec.backup.barmanObjectStore` is deprecated since 1.26. The release notes for 1.30.0
  (2026-06-29, the latest minor) say the in-tree Barman Cloud support "will now be removed in
  CloudNativePG 1.31.0". https://github.com/cloudnative-pg/cloudnative-pg/releases
- The replacement is the Barman Cloud CNPG-I plugin, version 0.14.0. It needs CloudNativePG 1.26
  or newer (1.27 recommended), it needs cert-manager, and it installs into the namespace of the
  CloudNativePG operator (`cnpg-system`).
  https://cloudnative-pg.io/plugin-barman-cloud/docs/installation/
- The plugin serves `barmancloud.cnpg.io/v1 ObjectStore`, a namespaced kind. Its spec carries
  `configuration.destinationPath`, `configuration.endpointURL`, one credentials block
  (`s3Credentials`, `azureCredentials`, `googleCredentials`), `configuration.wal.compression`,
  `configuration.data.compression`, and `spec.retentionPolicy` (the retention policy sits beside `configuration`, not inside it). A `Cluster` archives
  through `spec.plugins[]` with `name: barman-cloud.cloudnative-pg.io`, `isWALArchiver: true`,
  `parameters.barmanObjectName`, and `parameters.serverName`. `Backup` and `ScheduledBackup` use
  `method: plugin` with `pluginConfiguration.name`.
  https://cloudnative-pg.io/plugin-barman-cloud/docs/usage/
- WAL archiving alone does not give point-in-time recovery. Recovery starts from a base backup
  and replays WAL from there, so the archive needs base backups on a schedule.
  https://cloudnative-pg.io/docs/devel/backup

### Recovery

- Recovery is never in place. It bootstraps a new `Cluster` from a backup through
  `bootstrap.recovery.source`, which names an entry of `externalClusters[]`. With the plugin the
  entry carries `plugin.name` and `plugin.parameters` (`barmanObjectName`, `serverName`).
- `bootstrap.recovery.recoveryTarget.targetTime` holds the target. A timestamp without a timezone
  is read as UTC; the docs say to always give an explicit timezone.
- A recovered cluster must archive under its own `serverName`. Reusing the `serverName` of the
  source overwrites the source archive. A safety check in the plugin refuses to archive into a
  path that another server already uses.
  https://cloudnative-pg.io/docs/devel/recovery

### Identity and monitoring

- `Cluster.status.systemID` holds "the latest detected PostgreSQL SystemID". Over SQL the same
  value is `SELECT system_identifier FROM pg_control_system()`.
  https://cloudnative-pg.io/docs/devel/cloudnative-pg.v1
- Metrics are served on port 9187 at `/metrics`. `spec.monitoring.enablePodMonitor` is
  deprecated; the docs say to create a `PodMonitor` yourself.
  https://cloudnative-pg.io/docs/devel/monitoring
- The CloudNativePG Go types are published as their own module,
  `github.com/cloudnative-pg/api`, without the operator's dependencies.

### This repository, as it stood before #128

- `ElasticsearchCluster` is the template for a kind that runs a third-party operator's CR:
  `pkg/wrappers/eckelasticsearch` wraps the ECK CR through `pkg/generic`, the controller decides
  once at start whether ECK is installed (`eckInstalled`, commit a4e6a02), and `presetRef` merges
  a cluster-scoped preset into the spec.
- `DatabaseServerConfig` has a status-only controller that probes the server every ten minutes
  through `pkg/pgbootstrap` and publishes `status.serverVersion`. The probe is the one place the
  operator opens an admin connection to read facts about the server.
- `Database` rejects a duplicate claim through `CollisionKey` in
  `pkg/components/database/collision.go`, keyed on `serverRef` and `databaseName`.
- `PointInTimeRestore` pins the resolved chain in `status.storage` (`PointInTimeRestoreStorage`,
  with the `Endpoint` it reached) and re-checks it on every look. Its `dedicatedServer` rule
  lists `Database` objects across all namespaces and compares `spec.serverRef` to the contract
  name. It writes the `CamundaCluster` suspend through server-side apply under its own field
  manager.
- `api/` is its own Go module with three dependencies. Nothing in `api/v1` may import
  CloudNativePG types.

## Goals

- Run a PostgreSQL server for an orchestration cluster from one namespaced CR, with the sizing,
  storage, scheduling, and monitoring shape that `ElasticsearchCluster` has.
- Publish a `DatabaseServerConfig` whose `pitr` block is true because the operator owns the
  archive.
- Let `PointInTimeRestore` perform the database recovery when a `DatabaseServer` runs the server,
  and keep the current prerequisite for every other server.
- Make the RDBMS chain namespace-local so owner references clean it up and admin credentials
  never leave the namespace.
- Identify a server by what it reports, so the `Database` collision rule and the
  `PointInTimeRestore` dedicated-server rule hold across namespaces.
- Start the operator when the CloudNativePG CRDs are absent.

## Non-goals

- Managed PostgreSQL. A hand-written `DatabaseServerConfig` keeps working, and
  `PointInTimeRestore` keeps its prerequisite for it.
- One server shared by several orchestration clusters. It stays possible with a hand-written
  contract in each namespace, and it gives up point-in-time restore.
- Changing how `Database` creates a logical database. It keeps its SQL path and never uses the
  CloudNativePG `Database` kind.
- Replacing `LogicalBackupRDBMS`. Logical dumps and the WAL archive solve different problems.
- Engines other than PostgreSQL.
- Pruning old archives in the bucket after a recovery. The user removes them.
- A migration or conversion for the scope change. This is a clean-slate project.

## Decisions

### The archive goes through the Barman Cloud plugin, not the in-tree field

The in-tree field is removed in 1.31.0. A component that starts on a path with a removal date
inherits a rewrite. The plugin is the only path forward, so `DatabaseServer` creates an
`ObjectStore` next to the `Cluster` and wires the `Cluster` to it through `spec.plugins[]`.

The cost is two more things the user installs: cert-manager and the plugin, both into
`cnpg-system`. The installation docs list them next to CloudNativePG itself. The e2e suite
installs all three.

### The operator creates base backups, not only the WAL archive

The epic names continuous WAL archiving. Recovery needs a base backup to start from and replays
WAL forward from it, so an archive that holds WAL but no base backup cannot be recovered to any
point. With the plugin, a base backup is taken only through a CloudNativePG `Backup` or
`ScheduledBackup` with `method: plugin`. `DatabaseServer` therefore owns one `ScheduledBackup`.
The schedule comes from `spec.archive.baseBackupSchedule` (default daily at 02:00 UTC) and the
first backup runs at once (`immediate: true`). `ArchiveReady` is `True` only after the first
base backup completed.

The base backups are part of the archive, not part of the operator's backup model. The
operator's backup model is `BackupSchedule` creating `LogicalBackupRDBMS` (a `pg_dump`) and
`LogicalBackupElasticsearch`, coordinated with the Camunda backup API and restorable by
`LogicalRestore*`. The CloudNativePG base backups are physical, invisible to `BackupSchedule`,
produce no `LogicalBackup*` object, and only a `PointInTimeRestore` uses them, indirectly,
through a recovery request. Both run on one cluster.

The Elasticsearch side has the same split at a different line. `ElasticsearchCluster` registers
the snapshot repository from the `ObjectStorageConfig` but takes no snapshots, because a
snapshot is the backup model there. `DatabaseServer` registers the archive and takes base
backups, because a base backup is not the backup model here. The field is named
`baseBackupSchedule` and the docs say "base backup" throughout so nobody looks for these backups
in the backup list or confuses them with `BackupSchedule`.

The published `pitr.retentionPeriodDays` equals `spec.archive.retentionPeriodDays`, which also
sets `spec.retentionPolicy` on the `ObjectStore`. The value the operator declares is the
value it enforces.

### PointInTimeRestore asks through the contract, the producer acts

`PointInTimeRestore` consumes a `DatabaseServerConfig`. It never learns who wrote it. The
contract already declares what the server can do (`spec.pitr.enabled`,
`spec.pitr.retentionPeriodDays`); it grows to carry who performs a recovery, the request, and
the outcome:

- `spec.pitr.recovery: operator | external` (default `external`), a producer declaration.
  `DatabaseServer` publishes `operator`. A hand-written contract keeps `external`, and the
  restore keeps its prerequisite that the database is already recovered.
- `spec.recovery {requestedBy, requestID, targetTime}`, written by the consumer; `requestID` is the restore's UID, so a restore recreated under the same name is a new request. `PointInTimeRestore`
  applies it through server-side apply under its own field manager, the way it writes the
  `CamundaCluster` suspend. The producer's apply of the contract never carries this field, so
  the two writers never conflict.
- `spec.pitr.lastRecovery {requestedBy, requestID, targetTime, completedAt, result, message}`, written by
  the producer when a request is done. `result` is `Completed`, `Failed`, or `Unavailable`.

`DatabaseServer` owns the contract it publishes, so it already receives its events. A request
newer than `lastRecovery` triggers the recovery. `PointInTimeRestore` writes a request only when
the contract says `operator`, waits until `lastRecovery` matches its request, and reads `result`.
The contract's status stays with its own status-only controller, which keeps publishing
`systemIdentifier`.

The producer moves the endpoint and answers the request in two applies, under two field managers.
The contract component republishes `host` from `status.cluster` under its own manager. The
producer applies `lastRecovery` under `RecoveryFieldManager` only after the contract reports its
own generation as observed, together with a `systemIdentifier` for the new endpoint. The two
writers declare disjoint fields, so neither takes ownership from the other. The re-probe between
the applies is what proves the answer describes the server the contract now names.

`lastRecovery` is a producer-declared fact in spec next to a consumer-written request. It has
the shape of `pitr` itself, a declaration; the alternative, an outcome in the contract's status,
needs a second writer of that status and breaks the one-writer rule.

A second producer can honour the same request. A contract for a managed server that a cloud
operator produces can declare `operator` and recover through the provider API, and the restore
does not change.

The alternative, where `PointInTimeRestore` resolves the contract's owner and talks to the
`DatabaseServer`, ties the consumer to one producer. The alternative where it builds the
recovery `Cluster` itself puts two controllers on one set of CloudNativePG objects. Both
rejected.

### A recovery creates a new Cluster under a new name and removes the old one

The recovered cluster is `<name>-r<N>`, where `N` is the recovery sequence number from status.
It archives under its own `serverName`, equal to its `Cluster` name. When it is Healthy the
operator repoints `host` on the published contract to the new read-write Service and deletes the
previous `Cluster`. Its volumes go with it under the PVC retention policy of the spec.

The same-name shape keeps the endpoint stable but has nothing to roll back to between the delete
and the recovery, and it still needs a new `serverName`. Keeping the old `Cluster` after a
successful recovery doubles the storage for a server nobody connects to. The endpoint change is
absorbed by the contract: `CamundaCluster` reads `host` from the contract on reconcile and rolls
its pods.

### A restore can reach any point in retention, across recoveries

Each recovery starts a new archive. `status.archive.history[]` records every archive the server
has written: `serverName`, `objectStorageRef`, `from`, and `to` (`to` is unset for the current
archive). A recovery picks the source whose interval holds `targetTime`. A target that no interval
holds is refused with `PitrUnavailable`. Old archives stay in the bucket; the docs say the user
prunes them.

`objectStorageRef` is what makes an interval readable. A spec that names another bucket closes the
open record at that moment and opens a record of the new bucket at its first base backup, the same
way removing and restoring `spec.archive` does. The recovered cluster is given one `ObjectStore`,
the one of the bucket the spec names now, so a target inside a record of an earlier bucket is
refused with `result: Unavailable` and a message that names both buckets. A second `ObjectStore`
for the source would reach it and is a follow-up.

The alternative, only the current archive, is simpler but cannot correct a recovery to the wrong
point with a second restore further back.

### The RDBMS chain becomes namespace-local

`DatabaseServerConfig` and `Database` move to namespace scope. `Database.spec.targetNamespace`
is removed; the bindings land in the `Database`'s own namespace. `serverRef` on `Database` and
`DatabaseConfig` resolves in the CR's namespace. `adminCredentialsSecretRef` on
`DatabaseServerConfig` names a Secret in the contract's namespace and loses its namespace field.

A platform team that owned `Database` objects centrally writes `DatabaseConfig` and
`SecondaryStorageConfig` directly in the target namespace instead. `DatabaseServerPreset` and
`ObjectStorageConfig` stay cluster-scoped: they are catalogs many namespaces read.

### A server is identified by its system identifier

`pkg/pgbootstrap` gains `SystemIdentifier`, read from `pg_control_system()`. The
`DatabaseServerConfig` probe publishes it to `status.systemIdentifier` next to `serverVersion`.

`Database` keys its collision index on `systemIdentifier` and `databaseName`. A `Database` whose
contract has no identifier yet waits with `Ready=False` and a message that says so. The winner
rule stays: oldest `creationTimestamp`, then the smaller `namespace/name`.

`PointInTimeRestore` counts the `Database` objects across all namespaces whose contract reports
the same identifier. `PointInTimeRestoreStorage` pins `systemIdentifier` next to `Endpoint`, and
the pin check fails the restore when either changes. The operator's own recovery is the one
exception, and only for `Endpoint`: a physical recovery restores the `pg_control` of the base
backup, so the recovered instance reports the identity the restore pinned, and the identifier goes
on binding. A contract that has not published an identity yet, which is how it reads between the
repoint and the probe that follows, states nothing and cannot disagree with the pin.

This is #128. It ships first because every later PR builds on the namespaced kinds and the
identifier.

### CloudNativePG types come from `github.com/cloudnative-pg/api`

The wrapper in `pkg/wrappers/cnpgcluster` imports the published API module, the way
`eckelasticsearch` imports the ECK module. Hand-written types (the Keycloak precedent) exist to
avoid a heavy dependency; the API module is light, so the precedent does not apply. The
`ObjectStore` type of the plugin is small and has no published module, so
`pkg/wrappers/barmanobjectstore` carries hand-written types with generated deep-copy, the
Keycloak way. `api/v1` imports neither.

### The operator starts without the CloudNativePG CRDs

Commit a4e6a02 is copied: `cnpgInstalled` is decided once in `SetupWithManager` through the
REST mapper, `Owns` on the `Cluster` is conditional, a `DatabaseServer` on a cluster without the
CRDs reports `Ready=False` with `ReasonCNPGNotInstalled`, and the message says to install
CloudNativePG and restart the operator. `internal/testenv` gains `Options{WithoutCNPG}` and a
`withoutcnpg` suite. The plugin CRD is checked the same way; a `DatabaseServer` with an `archive`
block on a cluster without the plugin reports `ReasonBarmanPluginNotInstalled`.

### Admin credentials are the CloudNativePG superuser Secret

The `Cluster` sets `enableSuperuserAccess: true`. CloudNativePG writes `<name>-superuser` with
`username` and `password`. The published contract points at it. No password passes through the
operator, and `Database` connects the way it does today.

## API

### DatabaseServer

```yaml
apiVersion: core.camunda.io/v1
kind: DatabaseServer
metadata:
  name: camunda
  namespace: camunda
spec:
  presetRef: production            # DatabaseServerPreset, cluster-scoped, optional
  version: "17"                    # PostgreSQL major, required; image tag is resolved from it
  instances: 2                     # 1..N, default 1
  resources: {}
  storageSize: 20Gi                # may not shrink (CEL)
  storageClassName: ""
  walStorageSize: 5Gi              # optional separate WAL volume
  serviceAccount:
    annotations: {}                # CloudNativePG names the ServiceAccount; only annotations pass through
  scheduling: {}
  podLabels: {}
  podAnnotations: {}
  monitoring:
    podMonitor:
      enabled: true
  platformConfigRef: platform      # CamundaPlatformConfig, for the image repository
  databaseServerConfig: camunda    # name of the contract this server publishes, required
  archive:                         # optional; without it the contract says pitr.enabled=false
    objectStorageRef: backups      # ObjectStorageConfig, cluster-scoped
    retentionPeriodDays: 30
    baseBackupSchedule: "0 0 2 * * *"   # CloudNativePG six-field cron, default daily 02:00 UTC
  suspend: false
status:
  observedGeneration: 3
  cluster: camunda-r1              # the CloudNativePG Cluster the contract points at
  systemIdentifier: "7370..."
  archive:
    history:
      - serverName: camunda
        objectStorageRef: camunda-backups
        from: "2026-08-01T10:00:00Z"
        to: "2026-08-20T14:30:00Z"
      - serverName: camunda-r1
        objectStorageRef: camunda-backups
        from: "2026-08-20T14:30:00Z"
  recovery:                        # the last request this server acted on
    requestID: 6f1c…               # the restore's UID, as the contract carried it
    contract: camunda              # the contract that carried the request
    requestedBy: camunda/pitr-1
    targetTime: "2026-08-20T14:30:00Z"
    cluster: camunda-r1            # the cluster the recovery builds
    previousCluster: camunda       # where the contract goes back to if it fails
    archive:                       # pinned when the request is recorded
      serverName: camunda
      objectStorageRef: backups
    result: Completed              # unset while the recovery runs
    message: ""
    completedAt: "2026-08-20T15:02:11Z"
  volumes: []
  conditions: []
```

Printcolumns: `Ready`, `Reason`, `Version`, `Age`. The image is
`ghcr.io/cloudnative-pg/postgresql:<version>` by default, resolved through `pkg/images` so the
platform config can override the repository.

Validation: `databaseServerConfig` required; `storageSize` and `walStorageSize` may not shrink,
one CEL rule each; `archive` requires `retentionPeriodDays >= 1`; `version` matches `^\d+$` and is
floored at the oldest major Camunda 8.9 supports (verified with the `camunda-docs` MCP during
implementation).

The CEL rules bind the spec of the `DatabaseServer` only, so a lowered preset baseline reaches the
merged spec unchecked. The controller therefore raises each merged size back to the largest volume
that exists, taken from the data and write-ahead log claims and from the sizes the applied
CloudNativePG cluster asks for, and records the `StorageShrinkIgnored` Warning event. This mirrors
`keepAppliedStorageSize` of `ElasticsearchCluster`.

`monitoring.podMonitor` carries `labels` and `annotations` beside `enabled` and `interval`. The
labels are what a Prometheus selects the monitor by. The annotations carry metadata for other
tools.

The archive credentials are not referenced where the user keeps them. The operator mirrors what
the `ObjectStorageConfig` resolves into an operator-owned Secret `<server>-archive` in the
server's namespace, and the `ObjectStore` points every credential at that Secret.

Conditions and components, one component per condition:

| Condition | Component owns | True when |
| --- | --- | --- |
| `ClusterReady` | `Cluster` | the Cluster the contract points at is Healthy |
| `ArchiveReady` | `ObjectStore`, `ScheduledBackup` | absent `archive` block, or the first base backup of the current archive completed |
| `ContractReady` | `DatabaseServerConfig` | the contract is published and the superuser Secret exists (the contract's own Ready is the probe's business; a hibernated server keeps a published contract) |
| `MonitoringReady` | `PodMonitor` | monitoring disabled, or the PodMonitor is applied |

`Ready` on the owner aggregates `ClusterReady`, `ContractReady`, and, on a server that asks for an
archive, `ArchiveReady`. `MonitoringReady` always stands on its own, and so does `ArchiveReady` on
a server with no `archive` block, so `Ready` never reports `Disabled`. `ReasonCNPGNotInstalled` and
`ReasonBarmanPluginNotInstalled` are pre-check failures on the owner, before any component runs.

Suspend hibernates the `Cluster`: the wrapper's suspend mutation sets the annotation
`cnpg.io/hibernation: "on"`, CloudNativePG removes the pods and keeps the volumes, and the
suspension status reads the `cnpg.io/hibernation` condition. `spec.instances` cannot be zero
(the CRD declares `minimum: 1`), so scale-to-zero is not available on this kind.

### DatabaseServerPreset

Cluster-scoped. `spec.server DatabaseServerSpec` with a CEL rule that forbids `presetRef`,
`databaseServerConfig`, and `suspend` inside a preset, the shape of
`ElasticsearchClusterPreset`. `presetRef` on the server merges the preset under the server's own
fields the way `ElasticsearchCluster` does.

### DatabaseServerConfig

Namespaced. `adminCredentialsSecretRef` becomes `{name, key...}` in the contract's namespace.
`status.systemIdentifier` is added. The `pitr` block gains `recovery` and `lastRecovery`, and
the spec gains `recovery`, as decided above:

```yaml
spec:
  engine: postgres
  host: camunda-r1-rw.camunda.svc
  port: 5432
  adminCredentialsSecretRef:
    name: camunda-superuser
  pitr:
    enabled: true
    retentionPeriodDays: 30
    recovery: operator             # operator | external (default); producer-declared
    lastRecovery:                  # producer-written when a request is done
      requestedBy: camunda/pitr-1
      requestID: 6f1c…             # the restore's UID, echoed from the request
      targetTime: "2026-08-20T14:30:00Z"
      completedAt: "2026-08-20T15:02:11Z"
      result: Completed            # Completed | Failed | Unavailable
      message: ""
  recovery:                        # consumer-written under its own field manager
    requestedBy: camunda/pitr-1
    requestID: 6f1c…               # the restore's UID; a retry is a new restore
    targetTime: "2026-08-20T14:30:00Z"
```

Validation: `recovery.targetTime` and `lastRecovery.targetTime` are RFC 3339 with an explicit
zone; `pitr.recovery: operator` requires `pitr.enabled: true`. A `DatabaseServer` owns the
contract it publishes through an owner reference; the contract controller does not care who
wrote it, and no consumer reads the owner.

### Database

Namespaced. `targetNamespace` is removed. `applicationCredentials.secretNamespace` and
`backupCredentials.secretNamespace` are removed too; a Secret the user names lives in the
`Database`'s namespace. The collision key changes as decided above.

### PointInTimeRestore

Phase `RestoringDatabase` is added between `Pending` and `ValidatingDatabaseState`:

```
Pending → RestoringDatabase → ValidatingDatabaseState → RestoringPrimaryStorage → Completed
                                                                                → Failed
```

`admit` enters `RestoringDatabase` when the pinned `DatabaseServerConfig` declares
`pitr.recovery: operator`; otherwise it enters `ValidatingDatabaseState` as today. In
`RestoringDatabase` the restore:

1. Applies `spec.recovery {requestID, requestedBy, targetTime}` on the contract under its own
   field manager. `requestID` is the restore's UID. `requestedBy` is the restore's
   `namespace/name`. `targetTime` is `spec.timestamp` rendered in RFC 3339 UTC.
2. Polls the contract until `pitr.lastRecovery` matches the request and the contract's own
   `Ready` is `True`, then refreshes the pinned chain from the contract's status and moves on.
   The endpoint is what that refresh replaces. The identifier comes back unchanged from a
   recovery that read this server's own archive.
3. Fails with `PitrUnavailable` when `lastRecovery.result` is `Unavailable`, and with `Failed`
   when it is `Failed`; `message` is copied into the restore's condition.

A request that a producer never answers holds the restore in `RestoringDatabase`; the message
names the contract and the request. The pin on the contract UID still fails the restore if the
contract is replaced.

`status.storage` gains `systemIdentifier`. `ValidatingDatabaseState` then finds the database at
the target and continues unchanged.

## Recovery in DatabaseServer

The reconcile reads the contract it owns and sees `spec.recovery` that differs from
`pitr.lastRecovery` and from `status.recovery`, and:

1. Picks the archive from `status.archive.history[]` whose interval holds `targetTime`. None:
   applies `lastRecovery` with `result: Unavailable` and a message that names the covered
   intervals, and stops. One that holds it in a bucket the spec no longer names: the same result,
   with a message that names both buckets.
2. Applies `Cluster <name>-r<N>` with `bootstrap.recovery.source: source`,
   `recoveryTarget.targetTime`, an `externalClusters` entry that names the `ObjectStore` and the
   source `serverName`, and its own `plugins[]` entry with `serverName: <name>-r<N>`.
3. Waits for the new `Cluster` to be Healthy, then puts `status.cluster` on it. The contract
   component republishes `host` from `status.cluster` under its own field manager. Once the
   contract reports its own generation as observed, with a `systemIdentifier` for the new
   endpoint, the producer applies `lastRecovery {result: Completed}` under
   `RecoveryFieldManager`. That is two applies under two field managers, sequenced by the
   re-probe. A `Cluster` that fails to bootstrap (the recovery Job fails, or the target is past the archive)
   is deleted and `lastRecovery` is applied with `result: Failed` and the CloudNativePG message.
4. Deletes the previous `Cluster` and its base-backup schedule. Closes the archive interval of
   that previous cluster, and no other, at the cutover, when the contract moves to the new
   cluster: the old archive's WAL ends when the old cluster goes away, so the record states what
   the archive holds. Points between the cutover and the new archive's first base backup are
   honestly unavailable. The new record opens at that first base backup, which CloudNativePG can
   take before the cutover finishes, so the record of the new cluster is sometimes open already
   and must stay open. Applies a `ScheduledBackup` for the new cluster with `immediate: true`.
5. Records `status.recovery {requestID, contract, requestedBy, targetTime, cluster,
   previousCluster, archive{serverName, objectStorageRef}, result, message, completedAt}`.

Each step is idempotent and keyed on what exists, so a restart in the middle resumes. The
previous `Cluster` is deleted only after the contract points at the new one, so a failure before
step 4 leaves the old server intact and the restore reports `Failed` without data loss.

`spec.databaseServerConfig` is mutable. The reconcile sweeps every `DatabaseServerConfig` it owns
under the `camunda.io/database-server` label of the server and deletes the ones it no longer
publishes, so a rename leaves no contract behind that still declares `pitr.recovery: operator` and
that nothing answers. The sweep is skipped while a recovery is unanswered, because the answer goes
on the contract the record names.

While a request is unanswered, the server holds the two things the recovery reads. A spec that
moves `spec.databaseServerConfig` or `spec.archive.objectStorageRef` puts
`Ready=False/InvalidReference` on the server, and the merged spec stays pinned to the contract
name and the bucket that `status.recovery` recorded. The components keep running on those
recorded values, because a recovery that never gets its contract republished never finishes.

A candidate that fails after the contract already moved to it rolls the contract back. The server
puts `status.cluster` back on `status.recovery.previousCluster`, which still holds the data, and
answers the request with `result: Failed`. A record that names no cluster to go back to is an
error instead. The sweep of the next look reads `status.cluster`, and it would take the cluster
that holds the data with it.

`targetTime` is rendered in RFC 3339 UTC and keeps the fraction the restore asked for. A time
truncated to the second renders the same either way.

A request while `suspend` is true is answered with `result: Failed` and a message that says
so; the restore fails and the user retries after unsuspending.

## Watches and indexes

- `DatabaseServer` owns `Cluster`, `ObjectStore`, `ScheduledBackup`, `PodMonitor`, and
  `DatabaseServerConfig`. It watches the `ObjectStorageConfig` it names and its credentials
  Secret through the existing `refindex` helpers, and the `DatabaseServerPreset` it names.
- `Database` watches `DatabaseServerConfig` in its namespace and keeps a cluster-wide index on
  the collision key. The `PointInTimeRestore` dedicated-server rule keeps its live, unindexed
  list, filtered on the identifier the contract of each `Database` reports.
- `PointInTimeRestore` writes the contract and reads it back; it has no RBAC on
  `DatabaseServer`.
- No controller lists or watches a sibling `DatabaseServer`. A recovery request is a claim on
  one object; nothing coordinates across servers.

## Testing

- Unit: mutation tests and golden snapshots for the two wrappers and each component
  (`ocf:testing-operators`); the archive selection rule; the RFC 3339 rendering; the collision
  key; `SystemIdentifier` against a testcontainers PostgreSQL, next to the `ServerVersion` test.
- envtest: the `DatabaseServer` controller against the CloudNativePG and plugin CRDs read from
  the copies vendored under `internal/testenv/crds/cnpg` and `internal/testenv/crds/barmancloud` (the published `github.com/cloudnative-pg/api` module ships types without CRDs), with a fake Healthy status written by the
  test; the `withoutcnpg` suite; the `Database` collision through two contracts that report one
  identifier; `PointInTimeRestore` entering `RestoringDatabase`, writing the request on the
  contract, and reading `lastRecovery` that the test writes as the producer.
- e2e (CI only): `test/utils/cnpg.go`, which already holds the CRD path helpers, grows the install helpers for CloudNativePG and the plugin from pinned versions
  (`CNPG_VERSION`, `BARMAN_PLUGIN_VERSION` in `test/e2e/matrix/8.9.env` with renovate markers;
  cert-manager is already installed). A `databaseserver` flow brings a server up, sees the
  contract published with `pitr.enabled: true`, sees the first base backup in MinIO, and runs a
  `Database` on it. The RDBMS cluster flow gains the case where a `PointInTimeRestore` recovers
  the server: write, note the time, write more, restore, and read only the first write.

Four behaviors stay in envtest on purpose and never reach the e2e suite. Each e2e flow pays for
a CloudNativePG bootstrap on every pull request, and none of the four needs a real PostgreSQL to
be proved:

- Suspension: `It("hibernates the cluster while spec.suspend is true")` in
  `internal/controller/databaseserver/controller_test.go`.
- The removal of what an answered recovery replaced, under the volume retention policy:
  `It("keeps removing what an answered recovery replaced")` in
  `internal/controller/databaseserver/recovery_test.go`.
- Closing the archive record and opening a new one:
  `It("closes the archive record when the archive block is removed")`,
  `It("starts a new archive record when the archive comes back")`, and
  `It("starts a new archive record when the bucket changes")` in
  `internal/controller/databaseserver/controller_test.go`.
- The shared-server refusal:
  `It("holds a server that a Database of another namespace reaches through another contract")` in
  `internal/controller/pointintimerestore/controller_test.go`.

## Docs

- New `docs/crds/databaseserver.md` and `docs/crds/databaseserverpreset.md`, from `TEMPLATE.md`.
- `docs/crds/pointintimerestore.md`: the claim that the operator never restores the server
  becomes conditional; the new phase; the dedicated-server rule in terms of the identifier.
- `docs/crds/databaseserverconfig.md` and `docs/crds/database.md`: namespaced scope, the Secret
  in the same namespace, the identifier, the removed fields, the stale line about `pitr`.
- `docs/guides/secondary-storage.md`: one server per orchestration cluster as the recommended
  topology and why; the shared-server topology and what it gives up.
- `docs/installation.md`, `docs/getting-started.md`, `README.md`: CloudNativePG, cert-manager,
  and the plugin as optional installs, next to ECK.
- `docs/architecture.md` and `docs/crds/index.md`: the new kinds and the scope change.

## Risks

- The Barman Cloud plugin is below 1.0 and sits on the durability path. Mitigation: the e2e
  flow exercises a real recovery on every PR, and the version is pinned and bumped by renovate.
- A `DatabaseServer` with `instances: 1` has no failover. The preset docs say so and the
  production preset sets 2.
- The endpoint changes on every recovery. `CamundaCluster` rolls its pods; the docs state that a
  point-in-time restore restarts the orchestration cluster, which the existing suspend already
  implies.
- Major version upgrades of PostgreSQL are the user's operation. A `spec.version` whose major is
  not the one `status.pgDataImageInfo.majorVersion` reports on the applied cluster is refused, in
  either direction, until a later epic defines the path. The controller stages `Ready=False` with
  reason `VersionChangeRefused`, records one Warning event of that name, and applies nothing that
  reconcile, so the cluster keeps its image. There is no annotation escape hatch: CloudNativePG
  performs an offline in-place `pg_upgrade`, PITR does not cross a major, and the Barman plugin
  needs a new `serverName` per major, which the operator does not give it.
- CloudNativePG serves its own `Database` kind in `postgresql.cnpg.io`. The docs name the group
  whenever both could be meant.

## Implementation breakdown

Sub-PRs on `feat/cnpg-database-server`, in this order; the plan holds the contracts:

1. Namespaced `DatabaseServerConfig` and `Database`, the system identifier, the collision key,
   and the `PointInTimeRestore` rules on it (#128).
2. The `cnpgcluster` and `barmanobjectstore` wrappers with the CloudNativePG dependency.
3. `DatabaseServer` types, preset, controller, components, without-CRD start, and CRD docs.
4. The contract's recovery fields, the `PointInTimeRestore` recovery phase, and the
   `DatabaseServer` recovery.
5. e2e flows, installation and guide docs.
