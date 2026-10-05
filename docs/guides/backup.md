# Backup

A backup of an orchestration cluster is one consistent set: the secondary storage data and the Zeebe partitions, taken together. The operator writes the set to a bucket that you provide. There is one backup kind per storage backend: `LogicalBackupElasticsearch` for Elasticsearch and `LogicalBackupRDBMS` for PostgreSQL. They are called logical backups because they back up the data through the Camunda and storage APIs, not the volumes. One resource is one backup. You create it, it runs once, and its status records what was written.

A completed Elasticsearch backup is restored with a [LogicalRestoreElasticsearch](../crds/logicalrestoreelasticsearch.md). A completed relational backup is restored with a [LogicalRestoreRDBMS](../crds/logicalrestorerdbms.md).

## The base backups of a DatabaseServer are not backups of a cluster

The archive of a [DatabaseServer](../crds/databaseserver.md#the-archive) rolls the whole PostgreSQL server back to a timestamp, through a [PointInTimeRestore](../crds/pointintimerestore.md). No kind on this page writes or reads that archive, so run both on a cluster with an archived server.

## Set up once

### The bucket

Create an `ObjectStorageConfig` in the namespace of the cluster. This [contract](../crds/index.md#contracts) describes the bucket and how the cluster authenticates to it. The operator never creates the bucket. You create it, or another tool creates it for you.

An S3 bucket with workload identity:

```yaml
apiVersion: core.camunda.io/v1
kind: ObjectStorageConfig
metadata:
  name: my-backup-bucket
  namespace: my-cluster-ns
spec:
  type: S3
  s3:
    bucketName: my-backup-bucket
    basePath: backups
    region: eu-west-1
    auth:
      type: workloadIdentity
      workloadIdentity:
        roleArn: "arn:aws:iam::123456789012:role/my-cluster-backup-role"
```

The [ObjectStorageConfig reference](../crds/objectstorageconfig.md) has examples for GCS, Azure Blob, and static credentials (MinIO, Ceph).

`basePath` is a key prefix inside the bucket, without leading or trailing slashes. On S3 and GCS, every backup of a cluster lands under `<basePath>/<namespace>/<cluster>/`, so two clusters can share one bucket. A cluster reads the contract in its own namespace, so a cluster in another namespace needs a contract of its own. On Azure, the Zeebe backup store writes into the whole container. Create one container and one `ObjectStorageConfig` per cluster there.

### Point the cluster at it

Set `spec.backupStorageRef` on the `CamundaCluster` to the name of the `ObjectStorageConfig`:

```yaml
apiVersion: core.camunda.io/v1
kind: CamundaCluster
metadata:
  name: my-cluster
  namespace: my-cluster-ns
spec:
  # ... version, platformConfigRef, storageRef, and the rest of your cluster
  backupStorageRef: my-backup-bucket
```

This reference has three results:

- Zeebe writes its partition backups to the bucket.
- On a PostgreSQL cluster, continuous primary-storage backups are on by default. Zeebe takes a backup every `PT1H`, writes a checkpoint every `PT15M`, and keeps backups for `P7D`. The schedule and the checkpoint interval together set the granularity of a [point-in-time restore](../crds/pointintimerestore.md). You change these values on the cluster:

    ```yaml
    apiVersion: core.camunda.io/v1
    kind: CamundaCluster
    metadata:
      name: my-cluster
      namespace: my-cluster-ns
    spec:
      backupStorageRef: my-backup-bucket
      backup:
        primaryStorage:
          schedule: "PT30M"
          checkpointInterval: "PT5M"
          retention:
            window: "P14D"
      # ... the rest of your cluster
    ```

- If the bucket carries a workload identity, the operator writes the matching annotation on the ServiceAccount of the cluster. The ServiceAccount is named `<cluster>-camunda` by default. For a bucket that carries no annotation (for example EKS Pod Identity), bind the principal `system:serviceaccount:my-cluster-ns:my-cluster-camunda` on the cloud side.

### Elasticsearch: the snapshot repository

Camunda writes the Elasticsearch part of a backup into a snapshot repository. The `ElasticsearchCluster` registers it. Set `spec.snapshotStorageRef` to the same `ObjectStorageConfig` that the cluster references:

```yaml
apiVersion: core.camunda.io/v1
kind: ElasticsearchCluster
metadata:
  name: my-cluster-es
  namespace: my-cluster-ns
spec:
  # ... version, replicas, storageSize, secondaryStorageConfig
  snapshotStorageRef: my-backup-bucket
```

The operator registers the repository `my-cluster-ns.my-cluster-es` in Elasticsearch and publishes its name in the `SecondaryStorageConfig` as `snapshotRepository`. The `CamundaCluster` points Camunda at it.

Make sure that the repository is ready before you take a backup. The `ElasticsearchCluster` reports it:

```yaml
status:
  conditions:
    - type: SnapshotRepositoryReady
      status: "True"
      reason: Healthy
```

And the `CamundaCluster` publishes the name in its management binding:

```yaml
status:
  management:
    endpoint: http://my-cluster-zeebe.my-cluster-ns.svc:9600
    backupRepository: my-cluster-ns.my-cluster-es
    version: "8.9.9"
    partitions: 3
```

A `CamundaCluster` on Elasticsearch with a `backupStorageRef` and no repository name reports `Ready: InvalidReference` until the repository exists.

### PostgreSQL: backup credentials

The dump Job connects to the database with a separate backup user. A `Database` resource creates that user by default, as the SQL role `<databaseName>_backup`. It writes the credentials to the Secret `<Database name>-backup-credentials` and names it in the `DatabaseConfig` as `backupCredentialsSecretRef`. If you write the `DatabaseConfig` by hand, name the backup credentials yourself. Without them the backup reports `MissingSecret`.

```yaml
apiVersion: core.camunda.io/v1
kind: DatabaseConfig
metadata:
  name: my-camunda-db
  namespace: my-cluster-ns
spec:
  # ... serverRef, databaseName, credentialsSecretRef
  backupCredentialsSecretRef:
    name: my-camunda-db-backup-credentials
    usernameKey: username
    passwordKey: password
```

The dump runs the PostgreSQL client tools of the major version of your server. The operator reads that version from the `DatabaseServerConfig`. Make sure that the `DatabaseServerConfig` is `Ready` and reports the version:

```yaml
status:
  serverVersion: "17"
  conditions:
    - type: Ready
      status: "True"
      reason: Healthy
```

`spec.backup.dump` on the `CamundaCluster` shapes the Job. Every field is optional:

```yaml
apiVersion: core.camunda.io/v1
kind: CamundaCluster
metadata:
  name: my-cluster
  namespace: my-cluster-ns
spec:
  # ... the rest of your cluster
  backupStorageRef: my-backup-bucket
  backup:
    dump:
      # CPU and memory of the dump pod. The dump and the upload run in turn in the same pod.
      resources:
        requests:
          cpu: 500m
          memory: 1Gi
      # Where the dump is written before the upload. Unset is an emptyDir that the node bounds.
      # Set a storage class for a dump that is larger than the ephemeral storage of a node.
      scratchVolume:
        sizeLimit: 50Gi
        storageClassName: standard
      # Image of the dump container. Default: postgres:<major of the server>. Set it in an air-gapped installation.
      postgresImage: "registry.example.com/postgres:17"
      # Seconds before the Job fails. Default: 86400 (24 hours).
      activeDeadlineSeconds: 86400
      # Extra annotations of the dump pod.
      podAnnotations:
        # A service-mesh sidecar that keeps running stops the Job from completing. Turn it off for this pod.
        sidecar.istio.io/inject: "false"
```

One backup can replace these pod settings with its own `spec.dump` block, for example a larger scratch volume. The image always comes from the cluster. The [LogicalBackupRDBMS reference](../crds/logicalbackuprdbms.md) shows the block.

## Take a backup

Create the backup in the namespace of the cluster. A backup cannot reference a cluster in another namespace.

### Elasticsearch

```yaml
apiVersion: core.camunda.io/v1
kind: LogicalBackupElasticsearch
metadata:
  name: my-cluster-backup
  namespace: my-cluster-ns
spec:
  clusterRef:
    name: my-cluster
```

Watch it:

```bash
kubectl get lbes my-cluster-backup -n my-cluster-ns -w
```

### PostgreSQL

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

Watch it:

```bash
kubectl get lbrdbms my-cluster-backup -n my-cluster-ns -w
```

### How to tell it worked

Both kinds print the columns `Phase`, `Step`, and `Backup ID`. An Elasticsearch backup that runs:

```text
NAME                PHASE     STEP              BACKUP ID       AGE
my-cluster-backup   Running   SnapshotRecords   1755640800000   2m
```

`phase` moves from `Pending` to `Running` to `Completed` or `Failed`. The last two are final. `step` names the part of the backup that runs now. A PostgreSQL backup shows the steps `Dumping` and then `ZeebeBackup`.

`Completed` means that every part of the set is written. The `Ready` condition is `True` with reason `Completed`. The status of a completed Elasticsearch backup:

```yaml
status:
  phase: Completed
  backupId: 1755640800000
  partitionsCount: 3
  repository: my-cluster-ns.my-cluster-es
  historySnapshots:
    - camunda_webapps_1755640800000_8.9.9_part_1_of_6
    - camunda_webapps_1755640800000_8.9.9_part_2_of_6
    # ...
  history:
    state: Completed
  records:
    state: Completed
  runtime:
    state: Completed
  completionTime: "2026-08-19T22:07:41Z"
  conditions:
    - type: Ready
      status: "True"
      reason: Completed
```

And of a completed PostgreSQL backup:

```yaml
status:
  phase: Completed
  backupId: 1755640800000
  zeebeBackupId: 1755640812345
  objectKey: backups/my-cluster-ns/my-cluster/1755640800000/3f9c2a7e-2b1d-4f0e-9c1a-7d2f4b8e6a10/camunda.dump
  bucketRef: my-backup-bucket
  completionTime: "2026-08-19T22:09:12Z"
  conditions:
    - type: Ready
      status: "True"
      reason: Completed
```

A restore reads what it needs from this status. The reference page of each kind lists the fields.

If the phase is `Failed`, `failureMessage` names the step that failed and why. A failed Elasticsearch backup:

```yaml
status:
  phase: Failed
  step: ResumeExporting
  failureMessage: 'Step BackupRuntime failed: the endpoint stayed unreachable for 10m0s: the cluster is unreachable: GET /actuator/backupRuntime/1755640800000: ... dial tcp: i/o timeout'
  completionTime: "2026-08-19T22:30:02Z"
  conditions:
    - type: Ready
      status: "False"
      reason: Failed
```

See [When a backup fails](#when-a-backup-fails).

## Take backups on a schedule

A [BackupSchedule](../crds/backupschedule.md) creates a backup of the right kind for the cluster on a cron schedule, in UTC. It also deletes its own old backups beyond `spec.retained`:

```yaml
apiVersion: core.camunda.io/v1
kind: BackupSchedule
metadata:
  name: my-cluster-schedule
  namespace: my-cluster-ns
spec:
  clusterRef:
    name: my-cluster
  schedule: "0 2 * * *"
  retained:
    completed: 14
    failed: 1
```

On a PostgreSQL cluster, keep the retained dumps inside the primary-storage retention window of the cluster. A dump older than that window can no longer be restored. The schedule warns you with the event `RetentionWindowExceeded`.

## What happens during a backup

- On the Elasticsearch path, the operator soft pauses exporting on the cluster. Zeebe keeps processing, and broker disk usage grows while the backup runs. Camunda describes the mode in [Management API](https://docs.camunda.io/docs/self-managed/components/orchestration-cluster/zeebe/operations/management-api/). The operator resumes exporting when the set is written, and also when a step fails.
- On the PostgreSQL path, a Job named `<backup>-dump` runs in the namespace of the cluster, under the ServiceAccount of the cluster. The operator deletes the Job when the dump is uploaded. A Job that failed stays, so that you can read its logs.
- A cluster holds one backup or one restore at a time. A second backup waits in `Pending` with reason `BackupInProgress`, and starts when the holder gives the cluster back. If the message names a claim Lease to delete, delete it.
- If the cluster is suspended, a backup that has not started waits with reason `ClusterSuspended`.

## What an upgrade does to the backups you hold

A backup records the Camunda version of the cluster in `status.version`. A restore compares it against the version the cluster runs:

- A `LogicalRestoreElasticsearch` needs the exact version of the backup.
- A `LogicalRestoreRDBMS` needs the same Camunda minor as the backup, or one minor newer. Camunda migrates its own schema one minor at a time, as [Version compatibility checks](https://docs.camunda.io/docs/self-managed/components/orchestration-cluster/core-settings/concepts/version-compatibility/) states.

You do not lower `spec.version` by hand for a restore. The restore sets the cluster to the version of the backup. [The version after a logical restore](./operations.md#the-version-after-a-logical-restore) says what the restore writes on the cluster and what you do after it.

**Take a backup before every upgrade.** The most common reason to restore is an upgrade that went wrong, and the backup you want is the one from just before it.

## Delete a backup

When you delete a backup resource, the operator removes what the backup wrote. The resource stays until the artifacts are gone. If the cluster or a storage contract of the backup is gone, the operator cannot reach the artifacts. The resource then goes, the artifacts stay in the bucket, and a Warning event on the backup names what stays. The Deletion section of [LogicalBackupElasticsearch](../crds/logicalbackupelasticsearch.md#deletion) and [LogicalBackupRDBMS](../crds/logicalbackuprdbms.md#deletion) names the cases.

- On the Elasticsearch path, the operator deletes the snapshots that this backup created and the Zeebe backup under its id. If the backup still runs, the operator resumes exporting first.
- On the PostgreSQL path, the operator stops a running dump Job and deletes the dump object. It never deletes Zeebe backups. Zeebe keeps them under `spec.backup.primaryStorage.retention` of the cluster.

On a bucket with workload identity, the PostgreSQL path runs a cleanup Job named `<backup>-cleanup` under the ServiceAccount of the cluster. If the cleanup Job fails, the deletion waits, and the event `ArtifactCleanupFailed` on the backup names the Job. Read the logs of the Job and correct the cause. Then delete the Job, and the operator tries again with a new one.

## When a backup fails

The `Ready` condition of the backup carries the reason, and its message names the cause. Before the backup starts, it waits in `Pending`, and the reason names what it waits for. It starts when the cause is gone. During a run, a dependency that goes away holds a PostgreSQL backup for 10 minutes, then fails it. An Elasticsearch backup retries an unreachable endpoint for 10 minutes. A change or loss of the storage contract, the backup bucket, or a credentials Secret fails an Elasticsearch backup at once. A roll of Zeebe to another configuration also fails it at once. If the cluster is suspended, an Elasticsearch backup waits in its step.

A `Failed` backup does not run again. Read `status.failureMessage` and the events on the resource. On the PostgreSQL path, also read the logs of the dump Job. Correct the cause, then create a new backup with a new name. A `Failed` backup holds nothing that a restore can use. Delete it to remove what it wrote.

`ResumeFailed` is the one reason that needs you at once. It appears only on the Elasticsearch path: exporting did not resume within 30 minutes, and it stays paused. No other backup of the cluster starts. Make sure that the management API of the cluster is reachable. Then delete the backup: the deletion resumes exporting and releases the cluster.

The Status section of [LogicalBackupElasticsearch](../crds/logicalbackupelasticsearch.md#status) and [LogicalBackupRDBMS](../crds/logicalbackuprdbms.md#status) lists every reason and what to do.

## Related

- [LogicalBackupElasticsearch](../crds/logicalbackupelasticsearch.md): the backup kind of an Elasticsearch cluster, with every status field.
- [LogicalBackupRDBMS](../crds/logicalbackuprdbms.md): the backup kind of a PostgreSQL cluster, with `spec.dump` and every status field.
- [BackupSchedule](../crds/backupschedule.md): backups on a cron schedule, with retention.
- [ObjectStorageConfig](../crds/objectstorageconfig.md): the bucket contract, with examples for S3, GCS, Azure Blob, and static credentials.
- [ElasticsearchCluster](../crds/elasticsearchcluster.md): `spec.snapshotStorageRef` and the snapshot repository.
- [CamundaCluster](../crds/camundacluster.md): `spec.backupStorageRef`, `spec.backup`, and `status.management`.
- [Secondary storage](./secondary-storage.md): how a cluster gets its Elasticsearch or PostgreSQL backend.
- [DatabaseServer](../crds/databaseserver.md): the continuous archive of a PostgreSQL server, which is separate from the backups on this page.
