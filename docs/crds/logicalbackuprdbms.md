# LogicalBackupRDBMS

A LogicalBackupRDBMS is one backup of a relational orchestration cluster: a dump of the entire logical database, uploaded to the backup bucket, paired with one cluster-generated primary-storage backup.

!!! note "Stub"
    The backup epic writes the full page when the end-to-end suite lands. This stub states the shipped schema and behavior.

## Purpose

A LogicalBackupRDBMS captures a complete restore point of a `CamundaCluster` that stores its data in a relational database. The dump holds the exporter position, and the primary-storage backup taken right after it holds the matching Zeebe state; a restore reads the position from the restored dump and picks the primary-storage backups that pair with it. You create one by hand for a one-off backup; a [BackupSchedule](backupschedule.md) creates them on a cron schedule.

## How it works

1. The operator resolves `clusterRef` and checks: the cluster exists, has a `backupStorageRef`, is not suspended, stores its data in a relational database, and no other backup of it runs. The `DatabaseConfig` must name a `backupCredentialsSecretRef`, and the `DatabaseServerConfig` must state its `version`, because the dump runs client tools of at least the server's major version.
2. It allocates the backup id (a millisecond timestamp) and records the object key `<basePath>/<namespace>/<cluster>/<id>/camunda.dump`.
3. It applies a Job in the cluster namespace: a `postgres:<major>` initContainer runs `pg_dump --format=custom` of the entire logical database into a scratch volume, then the operator image's `upload` subcommand streams the archive to the bucket. The pod runs under the cluster's ServiceAccount for workload identity, or with the bucket's static credentials copied by the cluster controller. The pod settings come from the cluster's `spec.backup.dump`, or from this CR's `spec.dump` replacing that block as a whole.
4. When the Job succeeds, the operator requests one primary-storage backup through the cluster's management binding, without an id — the cluster generates it — and records it as `status.primaryBackupId`, polling until it completes.
5. Deleting the CR deletes a still-running Job, the credentials copy, and the dump object — never the primary-storage backups, which belong to the continuous range that a point-in-time restore consumes.

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

`status.phase` tracks the one-shot operation (`Pending | Running | Completed | Failed`); `status.step` is the resume marker (`Dumping | PrimaryBackup`). `status.backupId` identifies the dump object, `status.objectKey` its full key, `status.jobName` the Job, and `status.primaryBackupId` the cluster-generated primary-storage backup. `status.storageSizes.zeebe` records the effective restore size of one broker volume, best effort.

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
| `Ready` | `MissingCredentials` | The bucket's static credentials copy does not resolve. |

## Relationships

- [CamundaCluster](camundacluster.md) — referenced via `clusterRef`; its management binding and `spec.backup.dump` drive the backup.
- [DatabaseConfig](databaseconfig.md) / [DatabaseServerConfig](databaseserverconfig.md) — locate the database and the credentials of the dump.
- [ObjectStorageConfig](objectstorageconfig.md) — the bucket the dump is uploaded to, via the cluster's `backupStorageRef`.
- [BackupSchedule](backupschedule.md) — creates these CRs on a cron schedule.
