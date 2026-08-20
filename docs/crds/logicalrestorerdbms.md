# LogicalRestoreRDBMS

`LogicalRestoreRDBMS` restores one completed [LogicalBackupRDBMS](logicalbackuprdbms.md) into one suspended [CamundaCluster](camundacluster.md). You create it, or a recovery flow above the operator creates it for you.

The backup and the target both store their data in a relational database. The target must be the cluster the backup came from. A restore names a `CamundaCluster` with the same name, in the same namespace as the backup. Use this kind to undo a destructive operation, or to rebuild the cluster on new infrastructure under its own name.

One resource is one restore. The spec is immutable, and the restore runs once. `kubectl get lrrdbms` lists the restores with their phase, backup, and target.

Two things must be true before you create the resource:

- The backup reports `Completed`.
- The target cluster has `spec.suspend: true`, so no workload writes to primary or secondary storage.

The smallest restore names the backup and the target:

```yaml
apiVersion: core.camunda.io/v1
kind: LogicalRestoreRDBMS
metadata:
  name: my-cluster-restore
  namespace: my-cluster-ns
spec:
  backupRef:
    name: my-cluster-1748937221000
  targetClusterRef:
    name: my-cluster
```

```mermaid
graph LR
    LRR[LogicalRestoreRDBMS] -.->|backupRef| LBR[LogicalBackupRDBMS]
    LRR -.->|targetClusterRef| CC[CamundaCluster]
    CC -.->|storageRef| SSC[SecondaryStorageConfig]
    SSC -.->|databaseConfigRef| DBC[DatabaseConfig]
    DBC -.->|serverRef| DBS[DatabaseServerConfig]
    LBR -.->|"pinned bucket"| OSC[ObjectStorageConfig]
    LRR -->|pg_restore Job| PG["PostgreSQL (external)"]
    LRR -->|restore Job per broker| PVC[Broker data volumes]
```

## Suspend

The operator only reads `spec.suspend` of the target. It never writes it. You suspend the cluster before you create the restore, and you unsuspend it after the restore is `Completed`. A running cluster holds the restore in `Pending` with reason `ClusterNotSuspended`, and the operator touches no data.

Suspend is a standing condition, not a gate that admission passes once. A cluster that you unsuspend while the restore runs holds the restore in its current phase and fails it after ten minutes. Every phase after admission erases something of the target.

## One operation at a time

A cluster holds one backup or one restore at a time. The operator records the holder in a Lease next to the cluster. A restore takes that Lease when every rule of its admission holds, and it gives the Lease back when it reaches `Completed` or `Failed`.

A cluster that another backup or another restore holds keeps this restore in `Pending` with reason `ClusterClaimed`. The message names the holder. Nothing bounds this wait, and you change nothing: the restore starts on its own when the holder reaches a terminal phase.

## Phases

`status.phase` is the resume marker. A restore that re-enters after an operator restart continues at the recorded phase.

| Phase | What happens |
| --- | --- |
| `Pending` | The restore waits. The target runs, the backup does not exist or is not completed, the target does not exist, or another backup or restore holds the target. The operator touches nothing here. |
| `ValidatingCompatibility` | The operator compares the backup against the target. |
| `RestoringSecondaryStorage` | One Job downloads the dump from the backup bucket and runs `pg_restore` against the logical database of the target. |
| `RestoringPrimaryStorage` | The operator recreates the broker data volumes and runs the Camunda restore application on them. |
| `Completed` | The restore finished. You can unsuspend the target. |
| `Failed` | A phase failed. `status.failureMessage` names it. |

## Compatibility

The operator compares the backup against the target before the first destructive step. A breach fails the restore with reason `IncompatibleTarget`. No change to the restore resource resolves it, so you create a new restore against a target that fits.

| Rule | Why |
| --- | --- |
| The target is the cluster the backup came from. | The restore application reads the primary-storage backup under the prefix of the cluster it runs as. A different name points it at a prefix that holds no backup of this cluster. |
| The target stores its data in a relational database. | The dump holds relational data. An Elasticsearch target has nothing to read it with. |
| The target backs up through the same `ObjectStorageConfig` as the backup. | The `pg_restore` Job reads the bucket of the backup with the credentials that the `CamundaCluster` controller copies into the namespace, and it copies them for the bucket of the target alone. |
| The target runs the same Camunda minor as the backup, or one minor newer. | Camunda migrates its own schema one minor at a time. The patch level is free. |

The brokers write the backup prefix of their own cluster into their configuration. The restore Jobs copy that configuration from the live broker StatefulSet, so the restore always reads the prefix of the target.

The backup must record the Camunda version it was taken with, in `status.version`. A backup that recorded none fails the restore: nothing then proves that the target can read it.

The rules compare no partition count. A relational backup records none, and it needs none. The restore application reads the exporter position from the restored database and aligns the partitions itself.

The target facts come from the live broker StatefulSet, not from `status.management` of the cluster. A suspended cluster has no management binding.

## Secondary storage

The operator applies one Job that rebuilds the logical database of the target. An init container streams the dump from the backup bucket into a scratch volume through the `download` subcommand of `camunda-operator-cli`. The main container then runs `pg_restore --clean --if-exists --no-owner` from that file.

**The Job connects as the application role of the target**, the role that `DatabaseConfig.spec.credentialsSecretRef` names. `pg_restore --clean` drops each object before it recreates it, and PostgreSQL lets only the owner of an object drop it. The application role owns the database and every object in it. The backup role that wrote the dump owns nothing: it holds USAGE and CREATE on the schema and DML on the tables. A restore that connected as the backup role would fail every DROP with "must be owner of table" and would restore no data.

A credentials Secret outside the namespace of the target is read through the local copy that the `CamundaCluster` controller maintains. A Secret that is missing or lacks a key holds the restore with reason `MissingSecret` for the database credentials, or `MissingCredentials` for the bucket credentials.

The Job takes its pod settings and its postgres image from `spec.backup.dump` of the target cluster, through its preset when it names one. The Job runs under the ServiceAccount of the cluster, so the pod shape and the executable stay the choice of whoever owns the cluster. The restore resource carries no pod block of its own.

The `DatabaseServerConfig` of the target must publish `status.serverVersion`, which its controller writes once it reached the server as declared. The Job runs client tools of that major version. A server that was not probed for its current spec holds the restore with reason `InvalidReference`.

The operator records the Job in `status.secondaryJobName` and follows it to its end:

- A completed Job moves the restore to `RestoringPrimaryStorage`.
- A failed Job fails the restore, and the message names the Job. The logical database then holds a partial restore that only a new attempt repairs.
- A Job that disappears before it completes fails the restore, for the same reason.
- A Job under that name that carries the UID of another restore fails the restore. Its completion would let this restore continue without a restore of its own database.

## Primary storage

The Camunda restore application refuses a non-empty data directory, so the operator deletes the broker data volumes of the target and creates them again. The new volume takes the size that the backup recorded in `status.storageSizes.zeebe`, or the size of the claim template of the broker StatefulSet when the backup recorded none. The storage class, the access modes, and the labels always come from the claim template.

**The volumes belong to the broker StatefulSet, not to the restore.** They carry no owner reference to the restore resource, so deleting the restore never deletes a broker volume.

The operator refuses a Job that carries the name of one of its Jobs but no owner reference of this restore. Such a Job belongs to an earlier restore of the same name, and its result says nothing about this one. The restore fails, and the message names the Job.

The operator then runs the Camunda restore application once per broker, as a Job with **no arguments**. The continuous primary-storage backup of Zeebe carries the checkpoint, and the restore application reads the exporter position from the restored database and picks the backups itself.

The Jobs copy their configuration from the live broker StatefulSet, so the restore application always runs with the configuration the brokers run with, and the two cannot drift. A cluster whose broker StatefulSet was deleted cannot restore until its own controller applies it again.

## Deletion

When you delete the restore, the operator deletes the Jobs it created. It writes nothing to an external store, so it needs no finalizer and leaves no artifact behind. The recreated broker volumes stay.

## Status

| Type | Reason | Meaning | What to do |
| --- | --- | --- | --- |
| `Ready` | `Progressing` | A restore phase runs. | Wait. The message names the phase. |
| `Ready` | `Completed` | The restore finished. `Ready` is `True`. | Unsuspend the target. |
| `Ready` | `ClusterNotSuspended` | The target runs. | Set `spec.suspend: true` on the target. |
| `Ready` | `ClusterClaimed` | Another backup or restore holds the target. The message names it. | Wait. The restore starts when that operation finishes. |
| `Ready` | `IncompatibleTarget` | The target cannot hold the backup. See "Compatibility". | Create a new restore against a target that fits. |
| `Ready` | `InvalidReference` | The backup or the target does not exist, the backup is not `Completed`, a link in the storage chain is gone, or the database server was not probed. | Correct the reference that the message names. |
| `Ready` | `MissingSecret` | The database credentials Secret is missing or lacks a key. | Create the Secret that the message names. |
| `Ready` | `MissingCredentials` | The bucket credentials Secret is missing or lacks a key. | Create the Secret that the message names. |
| `Ready` | `Failed` | A phase failed. | Read `status.failureMessage`. Correct the cause and create a new restore. |

A restore that already started keeps a broken dependency for ten minutes. After that it fails, because a restore that rewrote a database or recreated a volume must not wait without an end. A restore that still waits in `Pending` has no such limit: it erased nothing.

The status also records what the restore pinned and what it did:

- `status.backupId` pins the backup id. A backup that is deleted and created again under one name carries another id, and the restore fails.
- `status.targetClusterUID` pins the identity of the target. A cluster that is deleted and created again under one name fails the restore.
- `status.secondaryJobName` is the `pg_restore` Job, while it exists.
- `status.brokers` is the broker count that the operator read off the broker StatefulSet.
- `status.recreatedClaims` names the broker data volumes that the operator deleted and created again.
- `status.primaryJobNames` names the per-broker restore Jobs, in broker order.
- `status.terminalReason` is the `Ready` reason of the terminal phase. The operator stages the condition again from it, so a write conflict cannot lose the reason.
- `status.completionTime` is when the restore reached `Completed` or `Failed`.
- `status.observedGeneration` is the last generation the operator reconciled.

## Spec reference

Every field, with its type and whether it is required:

```yaml
apiVersion: core.camunda.io/v1
kind: LogicalRestoreRDBMS
metadata:
  name: my-cluster-restore
  namespace: my-cluster-ns
spec:
  # object. Required. The backup to restore.
  backupRef:
    # string. Required. Name of the LogicalBackupRDBMS, in this namespace.
    name: my-cluster-1748937221000
  # object. Required. The cluster to restore into. It is the cluster the
  # backup came from.
  targetClusterRef:
    # string. Required. Name of the CamundaCluster, in this namespace.
    name: my-cluster
```

### Validation rules

- `spec` is immutable. A restore runs once, and you retry it with a new resource.
- `backupRef` and `targetClusterRef` name resources in the namespace of the restore. Neither crosses a namespace. The operator reads the Secrets of the target and runs Jobs in that namespace, so both references stay inside the RBAC boundary of the restore.
- `backupRef` carries a name alone. The kind of the restore says which backup kind it reads.
- The suspend state, the phase of the backup, and the compatibility rules depend on live state. The operator checks them at reconcile time.

## Related

- [LogicalBackupRDBMS](logicalbackuprdbms.md): referenced through `backupRef`. It must report `Completed`, and it records the bucket, the dump key, the Camunda version, and the broker volume size that this restore reads.
- [CamundaCluster](camundacluster.md): referenced through `targetClusterRef`. You suspend it for the whole restore, and its `spec.backup.dump` shapes the `pg_restore` Job.
- [SecondaryStorageConfig](secondarystorageconfig.md): resolved through the `storageRef` of the target. It must be `type: rdbms`.
- [DatabaseConfig](databaseconfig.md): resolved for the logical database. Its `credentialsSecretRef` holds the application role that `pg_restore` connects as.
- [DatabaseServerConfig](databaseserverconfig.md): resolved for the endpoint and the probed major version of the server.
- [ObjectStorageConfig](objectstorageconfig.md): the bucket that the backup wrote its dump to. The target must back up through the same one.
- [PointInTimeRestore](pointintimerestore.md): the alternative without a backup resource, for a database that you already restored to a point in time.
