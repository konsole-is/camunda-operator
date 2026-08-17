# LogicalBackupRDBMS

A LogicalBackupRDBMS is one backup of a relational orchestration cluster: a dump of the entire logical database, uploaded to the backup bucket, paired with one cluster-generated primary-storage backup.

!!! note "Stub"
    The backup epic writes the full page when the end-to-end suite lands. This stub states the shipped schema and behavior.

## Purpose

A LogicalBackupRDBMS captures a complete restore point of a `CamundaCluster` that stores its data in a relational database. The dump holds the exporter position, and the primary-storage backup taken right after it holds the matching Zeebe state; a restore reads the position from the restored dump and picks the primary-storage backups that pair with it. You create one by hand for a one-off backup; a [BackupSchedule](backupschedule.md) creates them on a cron schedule.

## How it works

1. The operator resolves `clusterRef` and checks: the cluster exists, has a `backupStorageRef`, is not suspended, stores its data in a relational database, and no other backup of it runs — a running backup blocks all others, and among pending ones the oldest starts first. The `DatabaseConfig` must name a `backupCredentialsSecretRef`, and the `DatabaseServerConfig` must state its `version`, because the dump runs client tools of at least the server's major version. These checks run only until the backup starts; a backup that fails them parks in `Pending` with the reason on `Ready`.
2. It allocates the backup id (a millisecond timestamp), pins the bucket it writes through (`status.bucketRef`), and records the object key `<basePath>/<namespace>/<cluster>/<id>/camunda.dump`.
3. It applies a Job in the cluster namespace: a `postgres:<version>` initContainer runs `pg_dump --format=custom` of the entire logical database into a scratch volume, then the operator image's `upload` subcommand streams the archive to the bucket. The operator resolves its own image from its running pod; `--operator-image` overrides it. The pod runs under the cluster's ServiceAccount for workload identity, or with the bucket's static credentials — the source Secret when it lives in the cluster namespace, the local copy the cluster controller keeps otherwise. The pod settings come from the cluster's `spec.backup.dump`, or from this CR's `spec.dump` replacing that block as a whole.
4. When the Job succeeds, the operator requests one primary-storage backup through the cluster's management binding, without an id — the cluster generates it — and records it as `status.primaryBackupId`, polling until it completes. Partitions register their parts asynchronously, so a backup the cluster does not report yet is polled for a short grace before it counts as failed.
5. A dependency that fails mid-run — a deleted reference, a broken binding, a management API that stops answering — holds the backup for a bounded grace, measured from when the dependency first failed and reset when it recovers, then fails it. A `Running` backup either finishes or terminalizes; it never parks forever, because it would block every later backup of the cluster.
6. Deleting the CR deletes a still-running Job and the dump object in the pinned bucket — never the primary-storage backups, which belong to the continuous range that a point-in-time restore consumes. When the cluster, the pinned bucket, or its credentials are genuinely gone, the finalizer releases and an event records what was left behind.

## API reference

```yaml
apiVersion: core.camunda.io/v1
kind: LogicalBackupRDBMS
metadata:
  name: my-cluster-backup
  namespace: my-cluster-ns
spec:
  # object. Required. Immutable. The CamundaCluster to back up.
  clusterRef:
    # string. Required. Name of the CamundaCluster.
    name: my-cluster
    # string. Optional, default: this CR's namespace.
    namespace: my-cluster-ns
  # object. Optional. Replaces the cluster's spec.backup.dump block as a
  # whole for this backup. The two never merge.
  dump: {}
```

The spec is immutable: a backup is one operation, and a retry is a new CR.

## Status

`status.phase` tracks the one-shot operation (`Pending | Running | Completed | Failed`); `status.step` is the resume marker (`Dumping | PrimaryBackup`). `status.backupId` identifies the dump object, `status.objectKey` its full key, `status.jobName` the Job, and `status.primaryBackupId` the cluster-generated primary-storage backup, requested at `status.primaryBackupRequestedAt`. `status.bucketRef` and `status.bucketGeneration` pin the `ObjectStorageConfig` the dump was written through, so a later retarget of the cluster cannot orphan the object. `status.firstFailedAt` is when a dependency of the running backup first stopped resolving; the mid-run grace is measured from it, and it clears on recovery. `status.failureMessage` states why a `Failed` backup failed. `status.storageSizes.zeebe` records the effective restore size of one broker volume, best effort.

| Type | Reason | Meaning |
| --- | --- | --- |
| `Ready` | `Progressing` | The backup runs, or waits on the management binding. |
| `Ready` | `Completed` | The backup finished and is restorable. |
| `Ready` | `Failed` | The backup failed; the message names the failing step. |
| `Ready` | `ClusterSuspended` | The cluster is suspended; the backup waits. |
| `Ready` | `BackupInProgress` | Another backup of the cluster runs; this one waits. |
| `Ready` | `StorageTypeMismatch` | The cluster does not store its data in a relational database. |
| `Ready` | `InvalidReference` | A referenced object does not exist, or the server states no version. |
| `Ready` | `MissingSecret` | The database names no usable backup credentials. |
| `Ready` | `MissingCredentials` | The bucket's static credentials do not resolve. |
| `Ready` | `ConnectionFailed` | The management API of the cluster is not reachable. |

## Relationships

- [CamundaCluster](camundacluster.md) — referenced via `clusterRef`; its management binding and `spec.backup.dump` drive the backup.
- [DatabaseConfig](databaseconfig.md) / [DatabaseServerConfig](databaseserverconfig.md) — locate the database and the credentials of the dump.
- [ObjectStorageConfig](objectstorageconfig.md) — the bucket the dump is uploaded to, via the cluster's `backupStorageRef`.
- [BackupSchedule](backupschedule.md) — creates these CRs on a cron schedule.
