# LogicalRestoreElasticsearch

`LogicalRestoreElasticsearch` restores one completed [LogicalBackupElasticsearch](logicalbackupelasticsearch.md) into one `CamundaCluster`. You create it, or a recovery flow above the operator creates it for you.

The target is the cluster the backup was taken from. The operator rebuilds both halves of the cluster state: it puts the web-application indices and the Zeebe record indices back into the Elasticsearch of the target, and it gives the brokers new data volumes that the Camunda restore application fills from the partition backup.

One resource is one restore. The spec is immutable, and the restore runs once. To retry, create a new resource. `kubectl get lres` lists the restores with their phase, backup, and target.

One thing must be true before you create the resource: the backup reports `status.phase: Completed`.

You do not suspend the target first, and you do not change its Camunda version first. The restore does both. Read "The restore prepares the target" below.

The smallest restore names the backup and the target:

```yaml
apiVersion: core.camunda.io/v1
kind: LogicalRestoreElasticsearch
metadata:
  name: my-cluster-lres
  namespace: my-cluster-ns
spec:
  backupRef:
    name: my-cluster-backup
  targetClusterRef:
    name: my-cluster
```

```mermaid
graph LR
    LR[LogicalRestoreElasticsearch] -.->|backupRef| LB[LogicalBackupElasticsearch]
    LR -.->|targetClusterRef| CC[CamundaCluster]
    CC -.->|storageRef| SSC[SecondaryStorageConfig]
    CC -.->|backupStorageRef| OSC[ObjectStorageConfig]
    SSC -.-> ES["Elasticsearch (external)"]
    LR -->|restores snapshots| ES
    LR -->|creates| PVC["Broker data volumes"]
    LR -->|creates| JOB["One restore Job per broker"]
```

## Phases

`status.phase` is the resume marker of the restore. The operator records it before the work it names, so a restore that re-enters after a restart continues where it stopped.

| Phase | What happens |
| --- | --- |
| `Pending` | The restore waits. A reference does not resolve, the backup is not completed, another operation holds the cluster, or the operator is still preparing the target. Nothing is deleted. |
| `ValidatingCompatibility` | The operator compares the backup against the target. |
| `RestoringSecondaryStorage` | The operator deletes the Camunda indices of the target and restores every snapshot of the backup. |
| `RestoringPrimaryStorage` | The operator deletes and creates the broker data volumes, and runs the Camunda restore application once per broker. |
| `Completed` | The restore finished. You can unsuspend the cluster. |
| `Failed` | The restore ended. `status.failureMessage` says why. |

`Completed` and `Failed` are terminal.

## Compatibility

Before the operator deletes anything, it compares the backup against the target. A breach fails the restore with the reason `IncompatibleTarget`, and the message names both values. Five rules apply:

- The target stores its data in Elasticsearch. Use a `LogicalRestoreRDBMS` for a relational cluster.
- The target is the cluster that the backup was taken from, which the backup names in `spec.clusterRef`. The restore application reads the partition backup under the prefix of the cluster it runs as. No other cluster reads that prefix.
- The partition count of the target is the partition count that the backup recorded in `status.partitionsCount`.
- The `spec.backupStorageRef` of the target names the same `ObjectStorageConfig` that the backup wrote to. The operator reads the artifacts through the bucket of the target, and the `CamundaCluster` controller copies the credentials of that bucket alone into the namespace.
- The target runs the exact Camunda version that the backup recorded in `status.version`. An Elasticsearch backup carries that version in the name of every snapshot, so a target one patch release newer cannot read it. The restore moves the target to that version before this rule runs, so a version that differs is a wait and not a failure. Only a backup that recorded no version, or that recorded a value which is not of the form `x.y.z`, fails this rule: the operator cannot write such a value.

## The restore prepares the target

The operator carries the target to the state that the restore needs. You do not suspend the cluster by hand, and you do not change its Camunda version by hand.

When the restore is admitted, the operator does this, in this order:

1. It takes the cluster claim, so that no other backup or restore of that cluster runs.
2. It records in `status.clusterSuspended` that it is about to suspend the target.
3. It applies `spec.suspend: true` on the target.
4. It waits until the broker `StatefulSet` reports zero replicas. `spec.suspend` says what was asked for. The `StatefulSet` says what happened.
5. It applies `spec.version` with the Camunda version that the backup recorded in `status.version`. It does this every time, even when the version rule of this kind would already accept the target.
6. It waits until the tag of the broker image is that version.

The restore stays in `Pending` for all of it. It erased nothing yet, so nothing bounds this wait. `Ready` reports `Progressing`, and the message says what the operator waits for.

The operator writes nothing else on the target. It writes no credential, and no reference to one.

### What the operator writes, and what it keeps

Each write is a server-side apply of one field, under a field manager of its own:

| Field | Field manager | What happens at the end |
| --- | --- | --- |
| `spec.suspend` | `camunda-operator/restore-suspend` | The restore withdraws it when it reaches `Completed`. |
| `spec.version` | `camunda-operator/restore-version` | The restore keeps it. |

The restore keeps `spec.version` on purpose. The cluster runs the version of the backup from then on, which is the point of writing it. Your next `kubectl apply` or GitOps sync of the `CamundaCluster` takes the field back, and that is correct: it is your declaration of the version again.

### Why the downgrade is safe here

Camunda does not support a running cluster that moves to an older version. A broker compares the version in its data directory against its own binary at startup, reports a downgrade, and applies no migration.

No broker does that comparison here. The order above is what makes it safe: nothing runs while the version changes, and the broker volumes are erased before a broker of the older version starts. The first broker that starts again finds the state of the backup at the version of the backup.

CAUTION: A downgrade that you do by hand on a running cluster, outside a restore, is still unsupported. The operator accepts the change to `spec.version`, and the brokers then report themselves unhealthy.

### When the restore unsuspends the target

The restore withdraws its suspension when it reaches `Completed`, and only when `status.clusterSuspended` is `true`.

- **A target that you suspended yourself stays suspended.** The restore recorded no suspension of its own, so it withdraws none.
- **A failed restore leaves the target suspended.** Its broker volumes can be empty or half written. Brokers that start over such volumes are worse than a cluster that is down. Read `status.failureMessage`, correct the cause, and create a new restore.
- **A restore that you delete while it runs leaves the target suspended.** The restore writes nothing outside the cluster, so it needs no finalizer, and it gets none for this either. A finalizer that unsuspended the target on delete would start brokers over volumes that the restore already erased. Suspended is the safe state to be left in. Unsuspend the cluster yourself once you know what its volumes hold.

The withdrawal is a server-side apply of an object without `spec.suspend`. Kubernetes removes the field that `camunda-operator/restore-suspend` owns, and it leaves the value of every other field manager in place.

### A GitOps tool that owns the CamundaCluster

The operator uses its own field managers, the same way [CamundaOptimize](camundaoptimize.md) does for `spec.zeebe.extraEnv`. A tool that manages the `CamundaCluster` with server-side apply keeps every field that it declares, and the operator keeps the fields that it declares.

A tool that also declares one of these fields fights the operator for it. Argo CD or Flux reverts the write of the restore, the restore reads the old value on its next look and writes again, and the restore stalls in `Pending`. If you drive the `CamundaCluster` from Git:

- Remove `spec.suspend` and `spec.version` from the manifest for the time of the restore, or mark `spec.suspend` and `spec.version` as an ignored difference.
- Put `spec.version` back after the restore, with the version that you want the cluster to run.

### The layer above

This operator is the bottom layer of a stack. A layer above it, for example a `CloudCamundaCluster` of `camunda-cloud-operator`, can consider itself the author of the fields above. While a restore runs, it is not. That layer keys on the field manager names above to tell a write of a restore from a write of a user.

## One operation at a time

The operator lets one backup or one restore of a cluster run at a time. A restore whose target another operation holds waits in `Pending` with the reason `ClusterClaimed`, and the message names the holder. Nothing bounds this wait. The restore starts on its own when the holder reaches a terminal phase. No watch wakes this hold. The controller wakes it on its retry timer alone, so the restore can start up to one retry interval after the holder finished. A restore takes the cluster for itself before it writes anything on that cluster and before every phase that deletes something. It therefore holds the cluster while it prepares it, which it reports as `Pending`.

## The snapshot repository

The restore reads the snapshots from the repository that the backup recorded in `status.repository`, on the Elasticsearch of the target. If that repository is absent, the operator registers it over the bucket that the backup pinned and the prefix that the source cluster wrote under. The snapshots lie under that prefix, whichever Elasticsearch server the target reads through.

If the repository is already registered, the operator uses it as it is. It never points an existing registration at another bucket or another prefix. The Elasticsearch of a target can be a cluster that this operator does not manage, where an administrator registered the repository by hand. A registration that points elsewhere makes the restore fail on a snapshot that is missing, and the message names the repository.

## Secondary storage

The operator deletes the Camunda indices of the target first, then asks Elasticsearch to restore every snapshot of the backup. It names the Optimize indices only when the backup holds an Optimize snapshot. A backup without one cannot put those indices back, so the operator keeps them.

The restore of a snapshot is asynchronous. The operator waits until the restored indices exist and no shard recovers any more, then it moves on.

`status.restoredSnapshots` is the resume marker of this phase. A look that finds it never deletes an index again.

CAUTION: A failure between the delete and the restore leaves the secondary storage of the target empty. The backup itself stays whole. The next look deletes what is there and asks for the restore again, so a retry converges. Do not delete the backup while the restore runs.

## Primary storage

The operator deletes the data volume of every broker and creates it again, then runs the Camunda restore application once per broker with `--backupId=<status.backupId>`.

The volumes belong to the `StatefulSet` of the cluster, not to the restore. They carry no owner reference to the restore, so deleting the restore leaves the brokers with their data. Each volume keeps the storage class, the access modes, and the labels of the claim template of the `StatefulSet`. Its size is the size that the backup recorded in `status.storageSizes.zeebe`, and the request of the claim template when the backup recorded none.

The restore Jobs copy the broker configuration from the live broker `StatefulSet`, so the restore application reads the same storage with the same credentials as the brokers. A cluster whose broker `StatefulSet` was deleted cannot restore until its controller applies the workload again. Suspending a cluster keeps the `StatefulSet` in place.

Every Job carries the labels `camunda.io/component: restore`, `camunda.io/logical-restore-elasticsearch: <restore name>`, and `camunda.io/cluster: <target name>`. Each Job has a controller reference to the restore, so deleting the restore removes its Jobs.

## Time limits

A restore that has started waits 10 minutes on a dependency that stops resolving, then it fails. The 10 minutes run from the first outage. Once an index or a volume is gone, a dependency that resolves again starts no second wait. The wait covers an Elasticsearch that does not answer, a reference that breaks, a pod that cannot start, and a target that somebody unsuspends mid-run. A restore that already deleted an index or a volume must reach a terminal phase, so that whoever owns the cluster learns that it has to act.

A restore in `Pending` waits without a bound, because it deleted nothing yet.

## Identity

The restore pins the backup ID and the identity of the target when it starts. A backup that somebody deletes and creates again under the same name holds other artifacts, and a cluster that somebody deletes and creates again under the same name is another cluster. Both end the restore. Create a new restore for the resources as they are now.

## Deletion

Deleting the restore removes its Jobs. The recreated broker volumes stay, and so does everything the restore wrote into Elasticsearch. The operator writes nothing outside the cluster, so the restore needs no finalizer.

A target that the restore suspended stays suspended. That is deliberate. Unsuspending it here would start brokers over volumes that the restore already erased. Unsuspend the cluster yourself once you know what its volumes hold.

## Status

| Type | Reason | Meaning | What to do |
| --- | --- | --- | --- |
| `Ready` | `Progressing` | A phase of the restore runs. | Wait. The message names the work. |
| `Ready` | `Completed` | The restore finished. `Ready` is `True`. | Unsuspend the cluster. |
| `Ready` | `Failed` | The restore ended. | Read `status.failureMessage`. Correct the cause and create a new restore. |
| `Ready` | `ClusterNotSuspended` | The target started running again while the restore ran. | Suspend the cluster again. A restore that already erased something fails 10 minutes after the first outage. |
| `Ready` | `ClusterClaimed` | Another backup or restore holds the cluster. | Wait. The restore starts when the holder reaches a terminal phase. |
| `Ready` | `IncompatibleTarget` | The target cannot hold the backup. The message names both values. | Read "Compatibility" above. A backup restores into the cluster it was taken from alone. |
| `Ready` | `InvalidReference` | A referenced resource does not exist, or the backup is not completed. | Read the message. Create the resource, or wait for the backup. |
| `Ready` | `ConnectionFailed` | The Elasticsearch of the target does not answer, or it refuses the credentials. | Make sure that the endpoint answers and that the credentials of the `SecondaryStorageConfig` are valid. |
| `Ready` | `MissingSecret` | A pod of a restore Job cannot start, because a Secret it mounts does not exist. | Create the Secret that the message names. |

These status fields report what the restore did:

- `status.backupId` is the backup that the restore reads.
- `status.repository` is the snapshot repository on the Elasticsearch of the target.
- `status.restoredSnapshots` names every snapshot that the operator asked Elasticsearch to restore.
- `status.clusterSuspended` records that this restore suspended the target. The restore withdraws that suspension when it completes.
- `status.brokers` is the broker count that the restore recorded before it deleted a volume.
- `status.recreatedClaims` names the broker data volumes that the restore deleted and created again.
- `status.primaryJobNames` names the restore Job of every broker, in broker order.
- `status.failureMessage` says why a failed restore ended.
- `status.completionTime` is when the restore reached a terminal phase.
- `status.observedGeneration` is the last generation that the operator reconciled.

## Spec reference

Every field, with its type, whether it is required, and its default:

```yaml
apiVersion: core.camunda.io/v1
kind: LogicalRestoreElasticsearch
metadata:
  name: my-cluster-lres
  namespace: my-cluster-ns
spec:
  # object. Required. The completed LogicalBackupElasticsearch to restore.
  backupRef:
    # string. Required. Name of the backup, in the namespace of this resource.
    name: my-cluster-backup
  # object. Required. The CamundaCluster to restore into.
  targetClusterRef:
    # string. Required. Name of the cluster, in the namespace of this resource.
    name: my-cluster
```

### Validation rules

- The whole `spec` is immutable. To retry, create a new resource.
- `spec.backupRef.name` and `spec.targetClusterRef.name` are required and must not be empty.
- Neither reference crosses a namespace. The backup and the cluster live in the namespace of the restore.
- The suspend state of the target, the state of the backup, and the compatibility rules all depend on live state. The operator checks them at reconcile time.

## Related

- [LogicalBackupElasticsearch](logicalbackupelasticsearch.md): the backup that this restore reads.
- [CamundaCluster](camundacluster.md): referenced through `targetClusterRef`. You suspend it for the whole restore.
- [SecondaryStorageConfig](secondarystorageconfig.md): resolved through the `storageRef` of the target. It must be `type: elasticsearch`, and it carries the endpoint and the credentials.
- [ObjectStorageConfig](objectstorageconfig.md): resolved through the `backupStorageRef` of the target. It holds the snapshots and the partition backup.
- [PointInTimeRestore](pointintimerestore.md): the restore kind for a relational cluster that a database administrator already restored to a point in time.
- [Backup guide](../guides/backup.md): how to set up backup storage and take a backup.
