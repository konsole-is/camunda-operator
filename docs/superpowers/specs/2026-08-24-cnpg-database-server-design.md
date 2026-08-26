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
- The same check blocks a `DatabaseServer` created again under an old name. The objects of the
  first one stay in the bucket, and the rollbacks of the new one number from zero again, so every
  path collides. The directory of a server in the bucket therefore carries eight hex characters of
  the SHA-256 of its UID (`ArchiveSegment`), and `ArchiveLocation` carries the same directory, so a
  server created again is a new location in `status.archive.history`.

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

CloudNativePG parses that schedule with the `cron.Parse` of `robfig/cron` v1 and its seconds
field, so the six-field form is the one it takes, together with the `@yearly` to `@hourly`
descriptors and `@every`. A schema pattern holds the field to that form and bounds each field to
the values that parser takes there, so an hour of 24 or a weekday of 7 is refused at admission
rather than at read time. The pattern is written in ECMA 262 with explicit case alternatives for
the names, because that is the dialect an OpenAPI `pattern` is read in. It cannot compare the two
ends of a range, so `FRI-MON` stays a CloudNativePG rejection.

Admission also rejects the five-field cron of a Kubernetes CronJob. CloudNativePG reads it
seconds first and runs it at a different time from the one its author meant. Without the pattern a
malformed schedule reaches the `ScheduledBackup`, CloudNativePG takes no base backup at all, and
the server keeps publishing an archive nobody refreshes.

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
holds is refused with `PitrUnavailable`. An interval says which archive wrote a point, not that the
objects are still there, so a target after now, or older than `retentionPeriodDays`, is refused the
same way before the pick. Old archives stay in the bucket; the docs say the user
prunes them.

The plugin prunes by the retention period in force when it runs, so a raised `retentionPeriodDays`
brings back nothing that a shorter one already dropped. `status.archive.reachableFrom` carries the
highest floor any past period pruned to: every reconcile that writes an archive raises it to
`now - retentionPeriodDays` and never lowers it. A target before it is refused with `Unavailable`
even when the current period reaches further, and the floor ages out once
`now - retentionPeriodDays` passes it. A record written before that field existed carries none, and
the retention period alone bounds it.

`location`, the canonical location of the bucket contract narrowed to the prefix of the server
(`ObjectStorageConfig.LocationOf`), is what makes an interval readable. `objectStorageRef` names it
for the reader. It is the canonical location rather than the URL the plugin is given, because the
endpoint and the region select the service that answers and neither reaches that URL. A record
written before the field carries only the contract, and it is adopted into the current location only
under that same contract: a record of another contract moved since, and nothing says where its
objects went, so `SelectArchive` refuses it as unplaceable. A spec that moves the archive to another location closes
the open record at that moment and opens a record of the new location at its first base backup, the
same way removing and restoring `spec.archive` does. With no record open, which is where removing
and re-adding `spec.archive` leaves the server, the move is recorded as `status.archive.boundary`
instead and cleared when the next record opens. `archiveBoundary` returns the later of the recorded
closes and that boundary, and a reconcile that finds a move reads no backups at all: the
`ObjectStore` it is about to apply is what moves the archive, so every backup that exists by then
began before the move. The clock for the move is read after `reconcileComponents` returns, so a
backup that started while the old `ObjectStore` still stood is behind the boundary. That window is
one apply wide and status timestamps carry whole seconds, so no envtest can place a backup inside
it. The unit test pins that the instant is used as given.

`archiveMoved(server, ref, location)` is the one comparison, and the guard on the archive component
and the history both read it. A move that the reconcile does not see at all leaves the guard reading
backups against the recorded closes, and a base backup of the location the server leaves then
reports the new archive ready in the same status write that closes the record.

A backup is placed against that boundary by `status.startedAt`, never by `status.stoppedAt`: one
that was already running when the interval closed keeps the destination the plugin gave it, so its
object lands in the bucket the server left while its end falls after the close. The field is
`+optional`, and a backup that carries no start is skipped once a boundary exists, because nothing
places it on either side of one. Before the first boundary there is nothing to place it against and
every completed backup counts, by its end. The skip is a guard rather than a path a supported stack
takes: the Barman Cloud plugin reports the start of every backup it completes
(`internal/cnpgi/instance/backup.go`, `StartedAt` from `executedBackupInfo.BeginTime`) and
CloudNativePG copies a non-zero value into `status.startedAt`
(`pkg/management/postgres/webserver/plugin_backup.go` in `release-1.30`). CloudNativePG core does
not guarantee it: `BackupStatus.SetAsStarted` sets `reconciliationStartedAt`, and only the volume
snapshot path of `internal/controller/backup_controller.go` copies that into `startedAt`.

The recovered cluster is given one `ObjectStore`,
the one of the location the spec resolves to now, so a target inside a record of an earlier
location is refused with `result: Unavailable` and a message that names the bucket contract and
the location of each. `SelectArchive` compares the location, and `status.recovery.archive` pins it,
so an `ObjectStorageConfig` edited under a running recovery does not move it. `recoveryHoldsSpec`
pins the contract by name, and `recoveryHoldsLocation` covers the edit that keeps the name: while
the resolved location differs from `status.recovery.archive.location`, `Ready` reports
`InvalidReference` and the archive component does not reconcile, so the one `ObjectStore` keeps
describing the archive the recovery asked for. That object names no workload identity of its own, so
`status.recovery.archive.identity` records the ServiceAccount annotations and the pod labels of the
bucket at the start, and `ArchiveStorage.HeldIdentity` puts them on the running cluster and on the
recovering one while the hold is on. The history is held with it: nothing applies the
location the spec resolves to then, so move detection and every record update are skipped, and the
move is decided on the reconcile after the hold lifts against the location that is applied by then. A second `ObjectStore`
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
      retentionPeriodDays: 30      # the archive settings every edit is held against
      baseBackupSchedule: "0 0 2 * * *"
    result: Completed              # unset while the recovery runs
    message: ""
    completedAt: "2026-08-20T15:02:11Z"
  volumes: []
  conditions: []
```

Printcolumns: `Ready`, `Reason`, `Version`, `Age`. The image is
`ghcr.io/cloudnative-pg/postgresql:<version>` by default, resolved through `pkg/images` so the
platform config can override the repository.

Validation: `metadata.name` must be a DNS-1035 label of at most 46 characters, through a CEL rule
on the root type. The name is used verbatim as the CloudNativePG cluster name, CloudNativePG takes
at most 50, and `recoveryName` appends `-r<n>`, so 50 minus the four of `-r99` is the bound: a name
at it reaches a recovery cluster whole while `n` stays below 100, and is shortened to a head and a
hash above that. `n` is `len(status.archive.history)`, so a rollback, an archive re-enable, and a
bucket change each advance it.
`recoveryName` shortens against the same 50 and needs no budget of its own for the Services
CloudNativePG derives: 50 plus `-any` is well inside a DNS label of 63. The rule is create-only
(`optionalOldSelf: true`, `oldSelf.hasValue() || ...`): a name never changes on update, and a rule
that runs there rejects an edit of another field on an object that predates it, and nothing more.

Validation: `databaseServerConfig` required; `storageSize` and `walStorageSize` may not shrink,
one CEL rule each; `archive` requires `retentionPeriodDays` from 1 to 36500 (a hundred years, under the
106751 days that overflow the `time.Duration` the reachable window is counted in); `version` matches `^\d+$` and is
floored at the oldest major Camunda 8.9 supports (verified with the `camunda-docs` MCP during
implementation).

The CEL rules bind the spec of the `DatabaseServer` only, so a lowered preset baseline reaches the
merged spec unchecked. The controller therefore raises each merged size back to the largest volume
that exists, taken from the data and write-ahead log claims and from the sizes the applied
CloudNativePG cluster asks for, and records the `StorageShrinkIgnored` Warning event. This mirrors
`keepAppliedStorageSize` of `ElasticsearchCluster`.

The same clamp keeps a write-ahead log volume that the merged spec no longer asks for.
CloudNativePG refuses a cluster that removes `spec.walStorage` once it applied it
(`walStorage cannot be disabled once configured`), and it accepts one that adds it, so the CEL
rule stays as it is. A cleared `walStorageSize`, inline or from a preset, keeps the applied size
and records the `WALStorageKept` Warning event.

`monitoring.podMonitor` carries `labels` and `annotations` beside `enabled` and `interval`. The
labels are what a Prometheus selects the monitor by. The annotations carry metadata for other
tools.

The archive credentials are not referenced where the user keeps them. The operator mirrors what
the `ObjectStorageConfig` resolves into an operator-owned Secret `<server>-archive` in the
server's namespace, and the `ObjectStore` points every credential at that Secret.

Conditions and components, one component per condition:

| Condition | Component owns | True when |
| --- | --- | --- |
| `ClusterReady` | `Cluster` | the Cluster the contract points at is Healthy. `ClusterTaken` when a Cluster of that name exists that this server does not control |
| `ArchiveReady` | `ObjectStore`, `ScheduledBackup` | absent `archive` block, or the first base backup of the current archive completed |
| `ContractReady` | `DatabaseServerConfig` | the contract is published and the superuser Secret exists (the contract's own Ready is the probe's business; a hibernated server keeps a published contract). `ContractTaken` when a contract of that name exists that this server did not publish |
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

1. Refuses a `targetTime` after now, older than the retention period of the archive, or before
   `status.archive.reachableFrom`, with `result: Unavailable` and a message that names the bound it
   broke. The retention is the one
   `status.recovery.archive` pins while a recovery runs, and the merged spec otherwise. Then picks
   the archive from `status.archive.history[]` whose interval holds `targetTime`. None:
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
   that previous cluster, and no other, at whichever comes first: the cutover, when the contract
   moves to the new cluster, or the first base backup of the new cluster. The old archive's WAL
   ends when the old cluster goes away, so the record states what the archive holds. Points
   between that close and the new archive's first base backup are honestly unavailable.
   The new record opens at that first base backup, which CloudNativePG can
   take before the cutover finishes, so the record of the new cluster is sometimes open already
   and must stay open. Applies a `ScheduledBackup` for the new cluster with `immediate: true`.
5. Records `status.recovery {requestID, contract, requestedBy, targetTime, cluster,
   previousCluster, archive{serverName, objectStorageRef}, result, message, completedAt}`.

Each step is idempotent and keyed on what exists, so a restart in the middle resumes. The
previous `Cluster` is deleted only after the contract points at the new one, so a failure before
step 4 leaves the old server intact and the restore reports `Failed` without data loss.
The ownership of the recovery `Cluster` is tested again on the live object at the cutover, because
the derived name comes back once the number of archives comes back. A cluster of that name that
this server does not own abandons the rollback with `Failed`, and the server runs from the previous
cluster again.

Every object the server applies under a derived name is registered with ocf
`BlockOnForeignController`; the two read-only registrations, the superuser Secret and the
recoverable cluster, are never applied and cannot carry it. ocf reads the live object before each apply and blocks the resource
while another owner controls it, so a name the server derives is never enough to write on somebody
else's object. The block covers the delete and the suspension too: a foreign object is neither
scaled down nor removed when the server withdraws its own.

One contract name belongs to one server. The block is what keeps it: the first server to publish a
name keeps it, and the second one publishes nothing until the owner and its contract are gone. The
reconcile also reads the `DatabaseServerConfig` that the merged spec names, and `ContractReady`
reports `ContractTaken` with the owner in the message. It stays because the contract is registered
behind the superuser Secret, and a blocked
resource stops every resource after it, so a server still waiting for that Secret would report the
wait and never the holder.

That read is also the protection for the case ocf does not cover. A `DatabaseServerConfig` with no
controller is refused rather than adopted: it is the bring-your-own-server API, so a person wrote it
for a PostgreSQL server the operator does not run. Adopting it rewrites that endpoint and those
credentials, and the owner reference the apply leaves behind takes the contract with the server when
the server is deleted. A guard on the contract carries the same message the condition does. The
feature gate that withdraws the contract while the cluster name is held stands down for it as well,
because a contract the server never published is not the server's to withdraw.

The name of the `Cluster` carries a guard of its own as well, for the case ocf does not cover: a
`Cluster` with no controller at all is refused rather than adopted, because it holds a database
this server did not build. Withholding the apply is not enough on its own, because a server that
owned the cluster and lost it has already published objects that name it. The contract and the monitoring components gate
themselves off while the name is held, so ocf removes the `DatabaseServerConfig` and the
`PodMonitor`, and the archive component withdraws the `ScheduledBackup` and keeps the `ObjectStore`.
`status.archive.history` and `status.recovery` are untouched, so the server comes back whole once
the name is free. `ClusterReady` reports `ClusterTaken` with the holder in the message.

`spec.databaseServerConfig` is mutable. The reconcile sweeps every `DatabaseServerConfig` it owns
under the `camunda.io/database-server` label of the server and deletes the ones it no longer
publishes, so a rename leaves no contract behind that still declares `pitr.recovery: operator` and
that nothing answers. The sweep is skipped while a recovery is unanswered, because the answer goes
on the contract the record names.

While a request is unanswered, the server holds the two things the recovery reads. A spec that
moves `spec.databaseServerConfig` or `spec.archive.objectStorageRef`, or that removes
`spec.archive` altogether, puts `Ready=False/InvalidReference` on the server, and the merged spec
stays pinned to the contract name and the archive that `status.recovery` recorded. The components
keep running on those recorded values, because a recovery that never gets its contract republished
never finishes, and one that loses its archive has nothing left to read.

`status.recovery.archive` therefore carries `retentionPeriodDays` and `baseBackupSchedule`
beside the bucket it names. `heldArchive` builds the merged archive block out of those three, and
it is also what decides that the spec moved one of them, so the block the server renders and the
edit it reports never disagree. A preset carries these fields as readily as an inline block, and
the merge runs before the hold, so a baseline edit is caught the same way. The `ObjectStore`, the
`ScheduledBackup`, and the published `pitr` therefore stay what they were, and the edit applies on
the reconcile after the answer goes out. A shorter `retentionPeriodDays` is what makes this more
than tidiness: it becomes the retention policy of the bucket and prunes the base backup the
rollback starts from.

A record written before those two fields existed carries neither. The spec fills what it still
has, and a removal is not held against such a record at all: rendering it would put `0d` on the
`ObjectStore`, which that CRD refuses, and publish a retention that the CEL rule on
`PITRCapability` refuses, so every apply would fail. Letting the removal apply refuses the
rollback and takes nothing out of the bucket.

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
- A rollback that neither converges nor fails has no deadline, and the hold widens what it stops
  while it runs: the contract name and every setting of `spec.archive` stay where the rollback
  needs them, and a preset edit that touches those fields is held with them. Both are
  pre-existing, and a rollback that hangs therefore parks more of the spec than it used to.
  Mitigation is the same as before: `status.recovery` and `Ready` name the rollback and what it
  holds, so an operator can read why an edit has not applied.
- The endpoint changes on every recovery. `CamundaCluster` rolls its pods; the docs state that a
  point-in-time restore restarts the orchestration cluster, which the existing suspend already
  implies.
- Major version upgrades of PostgreSQL are the user's operation. A `spec.version` whose major is
  not the one `status.pgDataImageInfo.majorVersion` reports on the applied cluster is refused, in
  either direction, until a later epic defines the path. The controller pins `merged.Version` to
  the running major, the way `keepAppliedStorageSize` pins the volume sizes, and stages
  `Ready=False` with reason `VersionChangeRefused` in place of the aggregate, plus one Warning
  event of that name. Everything else reconciles on the pinned version, so a recovery in flight
  finishes and the contract and the archive stay maintained. The guard reads the running cluster
  through `status.cluster` first and falls back to the contracts the server owns, via
  `recoveredClusterOf`: a rollback whose status write was lost leaves `status.cluster` on the
  cluster the rollback removed, and a guard that read that as a server with no cluster lets the new
  major reach the recovered one. Every owned contract is read, not the one the spec names, because
  a rename that lands before the repair leaves that name unpublished. The contract a rollback
  answers on is read first, because it names the cluster the rollback moved to. The guard does not fail open on an
  old CloudNativePG: `status.pgDataImageInfo` arrived in the api module at `v1.26.0` and is absent
  at `v1.25.1`, and 1.26 is the floor `docs/installation.md` names, so an absent field means only
  that the data directory is not written yet. There is no annotation escape hatch:
  CloudNativePG
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
