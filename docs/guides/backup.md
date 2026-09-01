# Backup

A backup of an orchestration cluster is one consistent set: the secondary storage data and the Zeebe partitions, taken together. The operator writes the set to a bucket that you provide. There is one backup kind per storage backend: `LogicalBackupElasticsearch` for Elasticsearch and `LogicalBackupRDBMS` for PostgreSQL. They are called logical backups because they back up the data through the Camunda and storage APIs, not the volumes. One resource is one backup. You create it, it runs once, and its status records what was written.

A completed relational backup is restored with a [LogicalRestoreRDBMS](../crds/logicalrestorerdbms.md). A completed Elasticsearch backup is restored with a [LogicalRestoreElasticsearch](../crds/logicalrestoreelasticsearch.md). Each kind restores a cluster from its own backup. The ids and names that a completed backup records in its status are what a restore needs.

## The base backups of a DatabaseServer are not backups of a cluster

A [DatabaseServer](../crds/databaseserver.md) with `spec.archive` writes base backups of the whole PostgreSQL server into a bucket, beside the write-ahead log. Those two together are what a [PointInTimeRestore](../crds/pointintimerestore.md) replays to reach a timestamp. They are not part of the backup model on this page. A base backup produces no `LogicalBackupRDBMS`, appears in no backup list, and no restore kind of this page reads one.

Run both on a cluster that has an archived server. The logical backups give you a restore of the Camunda data from a named backup. The archive gives you a rollback of the server to any timestamp inside its retention period.

## Set up once

### The bucket

Create an `ObjectStorageConfig` in the namespace of the cluster. It describes the bucket and how the cluster authenticates to it. The operator never creates the bucket. You create it, or another tool creates it for you.

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

`basePath` is a key prefix inside the bucket, without leading or trailing slashes. Every backup of a cluster lands under `<basePath>/<namespace>/<cluster>/`. Two clusters can share one bucket and never share one prefix. A cluster reads the contract in its own namespace, so a cluster in another namespace needs a contract of its own. Azure Blob is the exception: the Zeebe backup store writes into the whole container. On Azure, create one container and one `ObjectStorageConfig` per cluster. The secondary storage contract belongs to one cluster, see [one cluster per contract](./secondary-storage.md#one-cluster-per-contract).

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

This reference causes three things:

- The operator configures the Zeebe backup store with the bucket. Zeebe writes its partition backups there.
- On a PostgreSQL cluster, continuous primary-storage backups are on by default. Zeebe takes a backup every `PT1H`, writes a checkpoint every `PT15M`, and keeps backups for `P7D`. The schedule and the checkpoint interval together set the granularity of a [point-in-time restore](../crds/pointintimerestore.md#what-goes-in-spectimestamp). You change these values on the cluster:

    ```yaml
    spec:
      backupStorageRef: my-backup-bucket
      backup:
        primaryStorage:
          schedule: "PT30M"
          checkpointInterval: "PT5M"
          retention:
            window: "P14D"
    ```

- If the bucket carries a workload identity, the operator writes the matching annotation on the ServiceAccount of the cluster. The ServiceAccount is named `<cluster>-camunda` by default. With a bucket that carries no annotation (for example EKS Pod Identity), bind the principal `system:serviceaccount:my-cluster-ns:my-cluster-camunda` on the cloud side.

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

The operator registers the repository `my-cluster-es` in Elasticsearch, under the same prefix layout as every other backup object. It then publishes the name in the `SecondaryStorageConfig` as `snapshotRepository`, and the `CamundaCluster` configures its components with it.

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
    backupRepository: my-cluster-es
    version: "8.9.9"
    partitions: 3
```

A `CamundaCluster` with a `backupStorageRef` and no repository name reports `Ready: InvalidReference` until the repository exists.

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

A `LogicalBackupRDBMS` can replace the pod settings of this block for one backup with its own `dump` block. The image always comes from the cluster.

```yaml
apiVersion: core.camunda.io/v1
kind: LogicalBackupRDBMS
metadata:
  name: my-cluster-backup-large
  namespace: my-cluster-ns
spec:
  clusterRef:
    name: my-cluster
  dump:
    scratchVolume:
      sizeLimit: 200Gi
      storageClassName: standard
```

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

Both kinds print the columns `Phase`, `Step`, and `Backup ID`:

```text
NAME                PHASE     STEP              BACKUP ID       AGE
my-cluster-backup   Running   SnapshotRecords   1755640800000   2m
```

`phase` moves from `Pending` to `Running` to `Completed` or `Failed`. The last two are final. `step` names the part of the procedure that runs now.

`Completed` means that every part of the set is written and that the cluster exports again. The `Ready` condition is `True` with reason `Completed`. The status of a completed Elasticsearch backup:

```yaml
status:
  phase: Completed
  backupId: 1755640800000
  partitionsCount: 3
  repository: my-cluster-es
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

The fields that a restore needs:

| Kind | Field | Meaning |
| --- | --- | --- |
| both | `status.backupId` | The id of the backup. The Elasticsearch kind passes it to the cluster. The PostgreSQL kind uses it in the object key. |
| `LogicalBackupElasticsearch` | `status.historySnapshots` | The names of the Elasticsearch snapshots of the web application indices. |
| `LogicalBackupElasticsearch` | `status.repository` | The snapshot repository that holds the snapshots. The snapshot of the Zeebe record indices is named `camunda_zeebe_records_backup_<backupId>`. |
| `LogicalBackupElasticsearch` | `status.history`, `status.records`, `status.runtime` | The state of the three parts. Each is `Completed` on a completed backup. |
| `LogicalBackupRDBMS` | `status.objectKey` | The key of the dump in the bucket: `<basePath>/<namespace>/<cluster>/<backupId>/<uid>/camunda.dump`. |
| `LogicalBackupRDBMS` | `status.zeebeBackupId` | The id of the Zeebe backup that pairs with the dump. The cluster generates it. |
| both | `status.completionTime` | When the backup reached its final phase. |

To look at a backup in full:

```bash
kubectl get lbes my-cluster-backup -n my-cluster-ns -o yaml
```

If the phase is `Failed`, `failureMessage` names the step that failed and why:

```yaml
status:
  phase: Failed
  step: BackupRuntime
  failureMessage: "Step BackupRuntime failed: the cluster is unreachable: Get \"http://my-cluster-zeebe.my-cluster-ns.svc:9600/actuator/backupRuntime/1755640800000\": dial tcp: i/o timeout"
  completionTime: "2026-08-19T22:30:02Z"
  conditions:
    - type: Ready
      status: "False"
      reason: Failed
```

See [When a backup fails](#when-a-backup-fails).

## What happens during a backup

These are the effects that you notice while a backup runs.

- On the Elasticsearch path, the operator pauses exporting on the cluster first. Zeebe keeps processing. The operator resumes exporting when the set is written, and also when a step fails.
- On the PostgreSQL path, a Job named `<backup>-dump` runs in the namespace of the cluster. It runs under the ServiceAccount of the cluster. When the dump is uploaded, the operator requests one Zeebe backup and waits for it. A Job that succeeded is removed. A Job that failed stays, so that you can read its logs.
- The operator runs one backup of a cluster at a time, across both kinds. A second backup waits as `Pending` with reason `BackupInProgress` and starts when the first one ends.
- If the cluster is suspended, the backup waits with reason `ClusterSuspended`. The management API of a suspended cluster is not reachable.

## What an upgrade does to the backups you hold

A backup records the Camunda version it was taken with, in `status.version`. Every restore compares that version against the version the cluster runs.

The two storage paths differ:

- **Elasticsearch.** A `LogicalRestoreElasticsearch` needs the exact version. Elasticsearch carries that version in the name of every snapshot, so a cluster one patch release newer cannot read a snapshot of the older one.
- **PostgreSQL.** A `LogicalRestoreRDBMS` accepts the same Camunda minor as the backup, or one minor newer. Camunda migrates its own schema one minor at a time.

**The restore carries the cluster back to the version of the backup.** You do not lower `spec.version` by hand, and you do not suspend the cluster by hand. Create the restore against the cluster as it is.

The operator refuses a downgrade that you do by hand on a running cluster, outside a restore. The cluster reports `VersionDowngradeRefused`. The [CamundaCluster page](../crds/camundacluster.md#version) states the rule.

Two things follow that are worth knowing:

- On the PostgreSQL path, a cluster that the version rule would already accept is still moved back to the version of the backup. The cluster comes back one minor behind where it was, and you upgrade it forward again after the restore.
- The restore keeps `spec.version`. Declare the version you want the cluster to run once the restore is `Completed`. A manifest that omits the field does not take it back, because server-side apply removes a field only from the manager that declared it. A cluster that took its version from a release needs the field removed by hand. If the release is below the version the brokers run, remove the field and set `camunda.io/allow-version-downgrade` in one command. The CRD page of each restore kind shows that command.

**Take a backup before every upgrade.** The most common reason to restore is an upgrade that went wrong, and the backup you want is the one from just before it.

## Delete a backup

Deleting a backup resource removes what the backup wrote. A finalizer holds the resource until the artifacts are gone.

- On the Elasticsearch path, the operator deletes the snapshots that this backup created and the Zeebe backup under its id.
- On the PostgreSQL path, the operator deletes the dump object. It never deletes Zeebe backups. They belong to the continuous range that Zeebe keeps under `spec.backup.primaryStorage.retention`.

If you delete a backup that is still running, the operator stops it. On the Elasticsearch path it resumes exporting first. The resource is gone only when the cluster exports again.

On a bucket with workload identity, the cluster ServiceAccount holds the identity, not the operator. The PostgreSQL path then runs a cleanup Job named `<backup>-cleanup` under that ServiceAccount. If the cleanup Job fails, the deletion waits, and an event on the backup names the Job. Read the logs of the Job, correct the cause, and delete the Job. The operator creates it again and tries once more.

## When a backup fails

The `Ready` condition of the backup carries the reason. Before the backup starts, every reason except `Failed` and `ResumeFailed` means that the backup waits in `Pending`. It starts when the cause is gone. During a run, a dependency that goes away holds the backup for 10 minutes with the same reason, then fails it. A backup that ended as `Failed` does not run again. To retry, create a new resource with a new name. The spec of a backup is immutable.

| Reason | What it means | What to do |
| --- | --- | --- |
| `Progressing` | The backup runs, or it waits for the cluster to publish its management API or to finish a rollout. | Wait. |
| `Failed` | A step failed. `status.failureMessage` names the step and the error. | Read the message and the events on the resource. On the PostgreSQL path, read the logs of the dump Job. Correct the cause, then create a new backup. |
| `ResumeFailed` (Elasticsearch) | A step failed, and the resume of exporting did not succeed within 30 minutes. Exporting on the cluster stays paused. No other backup of this cluster starts. | Make sure that the management API of the cluster is reachable. Then delete this backup. The deletion resumes exporting and releases the cluster. |
| `InvalidReference` | The cluster, its `SecondaryStorageConfig`, the `ObjectStorageConfig`, or a dependency of the dump does not exist. On the Elasticsearch path, the cluster publishes no snapshot repository. On the PostgreSQL path, the dump image does not pull, or the `DatabaseServerConfig` has no `status.serverVersion` yet. | Read the message. Create the missing resource, or wait until the snapshot repository or the server version is published. |
| `MissingSecret` | A Secret or a key in it does not exist: the Elasticsearch credentials, the management credentials, or the database backup credentials. | Create the Secret, or set `backupCredentialsSecretRef` on the `DatabaseConfig`. |
| `MissingCredentials` (PostgreSQL) | The static credentials Secret of the bucket does not resolve. | Make sure that the Secret that the `ObjectStorageConfig` names exists and holds the configured keys. |
| `ConnectionFailed` | The management API of the cluster, or Elasticsearch, is not reachable or rejected the call. | Make sure that the cluster is healthy and that a network policy does not block the operator. The backup retries for a bounded time, then fails. |
| `StorageTypeMismatch` | The cluster stores its data in the other backend. | Use the other backup kind. |
| `ClusterSuspended` | The cluster is suspended. The backup waits. | Set `spec.suspend: false` on the cluster, or wait. |
| `BackupInProgress` | Another backup of the cluster runs. The message names it. The backup waits. | Wait, or delete the other backup. |

A `Failed` backup holds no artifacts that a restore can use. Delete it to remove what it wrote.

## Related

- [LogicalBackupElasticsearch](../crds/logicalbackupelasticsearch.md): the backup kind of an Elasticsearch cluster, with every status field.
- [LogicalBackupRDBMS](../crds/logicalbackuprdbms.md): the backup kind of a PostgreSQL cluster, with `spec.dump` and every status field.
- [ObjectStorageConfig](../crds/objectstorageconfig.md): the bucket contract, with examples for S3, GCS, Azure Blob, and static credentials.
- [ElasticsearchCluster](../crds/elasticsearchcluster.md): `spec.snapshotStorageRef` and the snapshot repository.
- [CamundaCluster](../crds/camundacluster.md): `spec.backupStorageRef`, `spec.backup`, and `status.management`.
- [Secondary storage](./secondary-storage.md): how a cluster gets its Elasticsearch or PostgreSQL backend.
- [DatabaseServer](../crds/databaseserver.md): the continuous archive of a PostgreSQL server, which is separate from the backups on this page.
