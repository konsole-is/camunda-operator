# LogicalBackupRDBMS

`LogicalBackupRDBMS` is one backup of a `CamundaCluster` that stores its data in a relational database. You create it, or another tool creates it for you.

An orchestration cluster on a relational database holds its data in two places. The Zeebe log and snapshots are the primary storage. The exported relational data is the secondary storage. One `LogicalBackupRDBMS` writes one `pg_dump` of the whole logical database to the backup bucket. Then it takes one Zeebe backup of the primary storage. Together they are one restore point for a [LogicalRestoreRDBMS](logicalrestorerdbms.md).

One resource is one backup. The spec is immutable, and the backup runs once. To take another backup, or to retry a failed one, create a new resource. `kubectl get lbrdbms` lists the backups with their phase, step, and backup ID.

Before you create a backup, make sure that:

- The `CamundaCluster` has `spec.backupStorageRef` and is `Ready` for its current generation. It is not suspended.
- The `DatabaseConfig` of the cluster has `backupCredentialsSecretRef`.
- The `DatabaseServerConfig` is `Ready` and has `status.serverVersion`.
- The backup lives in the namespace of the cluster.

A `LogicalBackupRDBMS` named `<name>` runs the Job `<name>-dump` in its namespace, under the ServiceAccount that the cluster publishes in `status.serviceAccountName`. The Job uploads the dump to the bucket of `spec.backupStorageRef` of the cluster, and `status.objectKey` records the key. Then the operator requests the Zeebe backup, which Camunda writes to the same bucket. `status.zeebeBackupId` records its ID.

The Job and its pod carry the label `camunda.io/cluster: <cluster>`. The operator deletes the Job when the dump is uploaded. A Job that failed stays, so that you can read its logs, until you delete the backup.

The smallest backup names the cluster:

```yaml
apiVersion: core.camunda.io/v1
kind: LogicalBackupRDBMS
metadata:
  name: my-cluster-backup
  namespace: my-cluster-ns
spec:
  clusterRef:
    name: my-cluster
```

```mermaid
graph LR
    LB[LogicalBackupRDBMS] -.->|clusterRef| CC[CamundaCluster]
    CC -.->|storageRef| SSC[SecondaryStorageConfig]
    SSC -.->|databaseConfigRef| DBC[DatabaseConfig]
    DBC -.->|serverRef| DBS[DatabaseServerConfig]
    CC -.->|backupStorageRef| OSC[ObjectStorageConfig]
    LB -->|creates| JOB["Job <name>-dump"]
    JOB -->|uploads| BUCKET["Bucket (external)"]
    LB -->|requests Zeebe backup| CC
    OSC -.-> BUCKET
```

## Pod settings

The dump pod takes its settings from `spec.backup.dump` of the cluster. If this resource sets `spec.dump`, that block replaces the cluster block as a whole. The two never merge. The image that runs `pg_dump` always comes from the cluster: `spec.backup.dump.postgresImage`, or `postgres:<serverVersion>` by default. The `extraEnv` and `extraEnvFrom` of this resource reach the container that runs `pg_dump` only, never the container that uploads.

## One operation at a time

A cluster holds one backup or one restore at a time. A backup that finds another backup or a restore on the cluster waits in `Pending` with reason `BackupInProgress`, and the message names the holder. A restore also suspends the cluster, so the backup can wait with reason `ClusterSuspended` instead. The backup starts on its own when the holder gives the cluster back.

## Time limits

The dump Job fails after `activeDeadlineSeconds`, 24 hours by default. A dependency can stop resolving during the run. Examples are a deleted Secret, an image that does not pull, and a management API that does not answer. The backup then waits 10 minutes for it to recover. After that, the backup fails.

## Changes

Do not change the backup storage of the cluster, or roll the cluster, while a backup runs. The backup waits 10 minutes for the change to be reverted, then fails. A dump and a Zeebe backup taken under different configurations do not form one restore point. A backup on a cluster that is still rolling out waits with reason `Progressing` before it starts. If you delete and recreate the cluster under the same name during the run, the backup fails at once.

## Deletion

When you delete the backup, the operator deletes a Job that still runs, waits until its pods are gone, and then deletes the dump object. It never deletes Zeebe backups. If the bucket uses workload identity, the cleanup runs as the Job `<name>-cleanup` under the ServiceAccount of the cluster. A failed cleanup Job holds the deletion and records an event that names the Job. Read its logs, correct the cause, and delete the Job to retry. If the cluster, the bucket, or its credentials are gone, or the bucket now points at another location, the operator cannot reach the dump. The dump object then stays in the bucket, and the `LogicalBackupRDBMS` is deleted. The Warning event `ArtifactCleanupFailed` names the object that stays.

## Status

| Type | Reason | Meaning | What to do |
| --- | --- | --- | --- |
| `Ready` | `Progressing` | The backup runs, or it waits for the cluster to finish a rollout or to publish its management API in `status.management`. | Wait. The message names the step or the wait. |
| `Ready` | `Completed` | The backup finished. `Ready` is `True`. | Nothing. Record `status.backupId` and `status.zeebeBackupId` for a restore. |
| `Ready` | `Failed` | The backup failed. | Read `status.failureMessage`. Correct the cause and create a new backup. |
| `Ready` | `ClusterSuspended` | The cluster is suspended: by `spec.suspend`, by a restore, or by the operator to keep two clusters off one backend. The backup waits. | Read the `Ready` condition of the cluster for the cause. |
| `Ready` | `BackupInProgress` | Another backup or a restore holds the cluster, or a claim Lease that no backup or restore holds. This one waits. The message names the holder or the claim Lease. | Wait for the named holder to give the cluster back. If the message names a claim Lease to delete, delete it. |
| `Ready` | `StorageTypeMismatch` | The cluster does not store its data in a relational database. | Use `LogicalBackupElasticsearch` for an Elasticsearch cluster. |
| `Ready` | `InvalidReference` | The cluster, its `SecondaryStorageConfig`, `DatabaseConfig`, `DatabaseServerConfig`, or `ObjectStorageConfig` does not exist. Or the server has no current `status.serverVersion`, the dump pod cannot pull its image, or `spec.dump.extraEnv` names a reserved variable. | Read the message. Create the resource, or wait for the `DatabaseServerConfig` to become `Ready`. For a reserved variable, create a new backup without it. |
| `Ready` | `MissingSecret` | The `DatabaseConfig` has no `backupCredentialsSecretRef`, that Secret does not exist, or the dump pod cannot start for a missing Secret. | Set `backupCredentialsSecretRef` on the `DatabaseConfig` and create the Secret. |
| `Ready` | `MissingCredentials` | The static credentials of the bucket do not resolve. | Create the Secret that the `ObjectStorageConfig` names, with all of its keys. |
| `Ready` | `ConnectionFailed` | The management API is unreachable or rejects the call. | Make sure that the cluster answers on its management port. After 10 minutes the backup fails. |

`status.phase` is `Pending`, `Running`, `Completed`, or `Failed`. `Completed` and `Failed` are terminal. `status.step` is `Dumping` or `ZeebeBackup`.

A restore needs these fields:

- `status.backupId` identifies the dump.
- `status.objectKey` is the full key of the dump in the bucket.
- `status.zeebeBackupId` is the Zeebe backup that pairs with the dump.
- `status.version` is the Camunda version of the cluster when the backup started. A restore of this backup needs a cluster on the same Camunda minor, or one minor newer.
- `status.storageSizes.zeebe` is the recorded size of one Zeebe data volume, when the operator can compute it.

`status.jobName` names the dump Job while it exists. `status.bucketRef` and `status.bucketLocation` record the bucket that holds the dump. `status.failureMessage` says why a `Failed` backup failed. `status.completionTime` is when the backup ended. `status.observedGeneration` is the last generation that the operator reconciled.

## Spec reference

Every field, with its type, whether it is required, and its default:

```yaml
apiVersion: core.camunda.io/v1
kind: LogicalBackupRDBMS
metadata:
  name: my-cluster-backup
  namespace: my-cluster-ns
spec:
  # object. Required. The CamundaCluster to back up, in the namespace of this resource. It must store its data in a relational database and have a backupStorageRef.
  clusterRef:
    # string. Required. Name of the CamundaCluster.
    name: my-cluster
  # object. Optional. Replaces the spec.backup.dump block of the cluster as a whole for this backup. The image is not part of it.
  dump:
    # object. Optional. CPU and memory of the dump pod.
    resources:
      requests:
        cpu: "500m"
        memory: "1Gi"
    # list. Optional. Extra environment variables of the dump container. A name that starts with PG or UPLOAD_ keeps the backup in Pending.
    extraEnv: []
    # list. Optional, max 8. Extra environment sources of the dump container. Each source needs a prefix that cannot spell a PG* or UPLOAD_* name.
    extraEnvFrom: []
    # map. Optional. Extra labels of the dump pod.
    podLabels: {}
    # map. Optional. Extra annotations of the dump pod. Set the injection annotation of a service mesh to false here.
    podAnnotations: {}
    # object. Optional. Scheduling constraints of the dump pod.
    scheduling: {}
    # object. Optional. The volume that holds the dump before the upload. Unset is an emptyDir that the node bounds.
    scratchVolume:
      # quantity. Optional. Size of the scratch volume.
      sizeLimit: "20Gi"
      # string. Optional. Storage class of a PersistentVolumeClaim that replaces the emptyDir. Needs sizeLimit.
      storageClassName: "standard"
    # integer. Optional, default: 86400. Seconds that the Job can run before it fails. Minimum 1.
    activeDeadlineSeconds: 86400
```

### Validation rules

- The whole `spec` is immutable. To retry, create a new resource.
- `spec.clusterRef.name` is required and must not be empty. The cluster must live in the namespace of the backup.
- Every source in `spec.dump.extraEnvFrom` needs a `prefix`. The prefix must not start a `PG*` or `UPLOAD_*` name. At most 8 sources are allowed.
- `spec.dump.scratchVolume.storageClassName` needs `sizeLimit`.
- `spec.dump.activeDeadlineSeconds` must be 1 or more.
- The API server accepts a `spec.dump.extraEnv` name that starts with `PG` or `UPLOAD_`. The backup then stays in `Pending` with reason `InvalidReference`, because the spec cannot change. Create a new backup without that name.

### A production-shaped example

A backup with a larger scratch volume and a shorter deadline for this run:

```yaml
apiVersion: core.camunda.io/v1
kind: LogicalBackupRDBMS
metadata:
  name: my-cluster-backup-20260819
  namespace: my-cluster-ns
spec:
  clusterRef:
    name: my-cluster
  dump:
    resources:
      requests:
        cpu: "1"
        memory: "2Gi"
    scratchVolume:
      sizeLimit: "50Gi"
      storageClassName: "standard"
    podAnnotations:
      sidecar.istio.io/inject: "false"
    activeDeadlineSeconds: 14400
```

After the backup completes, `kubectl get lbrdbms -n my-cluster-ns` shows the phase `Completed` and the backup ID.

## Related

- [CamundaCluster](camundacluster.md): the cluster that the backup references. Its `backupStorageRef` names the bucket, and its `spec.backup.dump` shapes the dump pod.
- [DatabaseConfig](databaseconfig.md): names the database and the `backupCredentialsSecretRef` that the dump uses.
- [DatabaseServerConfig](databaseserverconfig.md): names the server. Its `status.serverVersion` picks the `pg_dump` version.
- [ObjectStorageConfig](objectstorageconfig.md): the bucket that holds the dump and the Zeebe backups.
- [LogicalBackupElasticsearch](logicalbackupelasticsearch.md): the backup kind for an Elasticsearch cluster.
- [BackupSchedule](backupschedule.md): creates backups of this kind on a cron schedule.
- [LogicalRestoreRDBMS](logicalrestorerdbms.md): restores a cluster from a completed backup of this kind.
- [Backup guide](../guides/backup.md): how to set up backup storage and take a backup.
- [Secondary storage guide](../guides/secondary-storage.md): how to choose and connect the secondary storage.
