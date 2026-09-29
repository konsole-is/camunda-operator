# LogicalRestoreElasticsearch

`LogicalRestoreElasticsearch` restores one completed [LogicalBackupElasticsearch](logicalbackupelasticsearch.md) into one `CamundaCluster`. You create it, or an automated recovery flow creates it for you.

The target is the cluster the backup was taken from. The operator rebuilds both halves of the cluster state. It puts the web-application indices and the Zeebe record indices back into the Elasticsearch of the target. It also gives the brokers new data volumes that the Camunda restore application fills from the partition backup.

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

`status.phase` records how far the restore got. A restore continues at that phase after the operator restarts.

| Phase | What happens |
| --- | --- |
| `Pending` | The restore waits. A reference does not resolve, the backup is not completed, another operation holds the cluster, or the operator is still preparing the target. Nothing of the target is erased here. Preparation does write `spec.suspend` and `spec.version` on the target, which [The restore prepares the target](#the-restore-prepares-the-target) describes. |
| `ValidatingCompatibility` | The operator compares the backup against the target. |
| `RestoringSecondaryStorage` | The operator deletes the Camunda indices of the target and restores every snapshot of the backup. |
| `RestoringPrimaryStorage` | The operator deletes and creates the broker data volumes, and runs the Camunda restore application once per broker. |
| `Completed` | The restore finished. The restore unsuspended the target, unless you suspended it yourself. |
| `Failed` | The restore ended. `status.failureMessage` says why. |

`Completed` and `Failed` are terminal.

## Compatibility

Before the operator deletes anything, it compares the backup against the target. A breach fails the restore with the reason `IncompatibleTarget`, and the message names both values. Five rules apply:

- The target stores its data in Elasticsearch. Use a `LogicalRestoreRDBMS` for a relational cluster.
- The target is the cluster that the backup was taken from, which the backup names in `spec.clusterRef`. The restore application reads the partition backup under the prefix of the cluster it runs as. No other cluster reads that prefix.
- The partition count of the target is the partition count that the backup recorded in `status.partitionsCount`.
- The `spec.backupStorageRef` of the target names the same `ObjectStorageConfig` that the backup wrote to. The operator reads the artifacts through the bucket of the target, with the credentials that its contract names in the namespace of the target.
- The target runs the exact Camunda version that the backup recorded in `status.version`. An Elasticsearch backup carries that version in the name of every snapshot, so a target one patch release newer cannot read it. The restore moves the target to that version before this rule runs, so a version that differs is a wait and not a failure. Only a backup that recorded no version, or that recorded a value which is not of the form `x.y.z`, fails this rule. The operator cannot write such a value.

## The restore prepares the target

The operator brings the target to the state that the restore needs. You do not suspend the cluster by hand, and you do not set its Camunda version by hand.

The restore suspends the target. It also sets `spec.version` to the Camunda version that the backup recorded in `status.version`. It sets that version every time, even when the compatibility rule of this kind accepts the target as it is.

The restore stays in `Pending` while it does this, and nothing bounds the wait. It erases nothing before it leaves that phase. `Ready` reports `Progressing`, and its message names what the operator waits for.

The operator writes nothing else on the target. It writes no credential, and no reference to one.

### What the operator writes, and what it keeps

Each write is a server-side apply of one field or annotation, under a field manager of its own:

| Field | Field manager | What happens at the end |
| --- | --- | --- |
| `spec.suspend` | `camunda-operator/restore-suspend` | The restore withdraws it when it reaches `Completed`. |
| The annotation `suspension-hold.camunda.io/<restore UID>` | `camunda-operator/suspension-hold-<restore UID>` | The restore removes it when it reaches `Completed`. When you delete the restore, it removes the hold once its Jobs and their pods are gone. A failed restore keeps it. |
| `spec.version` and the annotation `camunda.io/allow-version-downgrade` | `camunda-operator/restore-version` | The restore keeps `spec.version`. The operator removes the annotation once the brokers carry the version, and as soon as it names another version. |

These names are published. A GitOps tool reads them in a conflict message, and they tell a write of a restore from a write of a user.

The annotation sanctions the move. A [CamundaCluster](camundacluster.md#version) refuses a version below the one its brokers run, unless the annotation names it. The restore writes the version and the annotation together.

The restore keeps `spec.version` on purpose. The cluster runs the version of the backup from then on, which is the point of writing it.

To move the cluster off that version, declare the version you want:

- A client-side `kubectl apply` that sets `spec.version` takes the field back. It writes the field, and the API server gives ownership to the manager that wrote it.
- A server-side apply, which is what Argo CD and Flux use, reports a conflict on the field. Force the conflict, and the tool owns `spec.version` again.

CAUTION: A manifest that omits `spec.version` does not take the field back. Server-side apply removes a field only from the manager that declared it, and `camunda-operator/restore-version` still declares this one. Watch for this on a cluster that took its version from a release. An explicit `spec.version` always wins over the release. So the value the restore wrote governs the cluster until somebody removes the field. Remove it by hand to give the release control again.

If the version of the release is below the one the brokers run, the operator refuses that removal. Set the annotation `camunda.io/allow-version-downgrade` to the version of the release in the same edit that removes the field. Setting the annotation first does not work. The operator removes an annotation that does not name the version the cluster is asked to run. Remove the field first and set the annotation after the refusal, or do both in the one command shown.

```bash
kubectl patch camundacluster my-cluster -n my-cluster-ns --type=merge -p '{
  "metadata": {"annotations": {"camunda.io/allow-version-downgrade": "8.9.0"}},
  "spec": {"version": null}
}'
```

A merge patch keeps the other annotations of the cluster, and it writes the annotations map when the cluster has none. The value `null` removes `spec.version`.

### Why the downgrade is safe here

Camunda does not support a downgrade of a running cluster. A restore is not that. Nothing runs while the version changes, and the broker volumes are erased before any broker of the older version starts.

The operator refuses a downgrade that you do by hand on a running cluster, outside a restore. The cluster reports `VersionDowngradeRefused`. The [CamundaCluster page](camundacluster.md#downgrade-on-purpose) explains how to downgrade on purpose, and what it costs.

### When the restore unsuspends the target

The restore withdraws its suspension when it reaches `Completed`, and only when `status.clusterSuspended` is `true`.

- **A target that you suspended yourself stays suspended.** The restore recorded no suspension of its own, so it withdraws none.
- **A failed restore leaves the target suspended.** Its broker volumes can be empty or half written. Brokers that start over such volumes are worse than a cluster that is down. Read `status.failureMessage` and correct the cause. Delete the failed restore, then create a new one. The failed restore keeps its suspension hold until you delete it. While `status.recoveryHeld` is `true`, the delete waits until that hold ends, as [After a failure or a delete](#after-a-failure-or-a-delete) describes. Until then, neither `spec.suspend: false` nor a new restore starts the target.
- **A restore that you delete while it runs leaves the target suspended.** A delete never unsuspends the target. It removes the suspension hold of the restore, and `spec.suspend` decides from then on. While `status.recoveryHeld` is `true`, the delete waits until that hold ends. Brokers that start over volumes the restore already erased are worse than a cluster that is down. Unsuspend the cluster yourself once you know what its volumes hold.

### A GitOps tool that owns the CamundaCluster

The operator declares the two fields above under its own names, the same way [CamundaOptimize](camundaoptimize.md) does for `spec.zeebe.extraEnv`. A tool that manages the `CamundaCluster` with server-side apply keeps every field that it declares. The operator keeps the fields that it declares.

A tool that also declares one of these fields fights the operator for it. Argo CD or Flux reverts the write of the restore, the restore writes it again, and the restore stalls in `Pending`. If you drive the `CamundaCluster` from Git:

- Remove `spec.suspend` and `spec.version` from the manifest for the time of the restore. Or mark both fields as an ignored difference.
- Put `spec.version` back after the restore, with the version that you want the cluster to run.
- A tool that prunes annotations it does not declare removes the sanction, and the cluster then refuses the version write. Exclude `camunda.io/allow-version-downgrade` from pruning for the time of the restore.
- Such a tool also removes the suspension hold of the restore. Exclude the annotations that start with `suspension-hold.camunda.io/` from pruning.

## One operation at a time

A cluster holds one backup or one restore at a time. This restore holds the target from the moment it starts to prepare it, which it reports as `Pending`. It gives the hold back when it reaches a terminal phase. A failed restore that keeps the backend for a recovery also keeps the target until its `status.recoveryHeld` is `false`.

A restore whose target another operation holds waits in `Pending` with the reason `ClusterClaimed`, and the message names the holder. Nothing bounds this wait, and you change nothing. The restore starts on its own a short time after the holder reaches a terminal phase and its `status.recoveryHeld` is not `true`.

## The backend

A restore writes into the backend of its target. That is the Elasticsearch that the `SecondaryStorageConfig` of the target resolves to. The restore writes it only while the target holds that backend. When the restore leaves `Pending`, it records the backend in `status.backend`.

From then until the restore reaches `Completed` or `Failed`, and after a failure while `status.recoveryHeld` is `true`, no other `CamundaCluster` starts on that backend. This also holds when you delete the target during the restore, or point it at another backend. The next cluster on the backend reports `WaitingForHandover`, and the message names this restore. The hold lasts even when the restore stops making progress. To free the backend from a restore that does not move, delete the restore. A deleted restore gives the backend back at once, unless `status.recoveryHeld` is `true`. [CamundaCluster: Secondary storage](camundacluster.md#secondary-storage) has the rule for the cluster.

The restore itself waits in `Pending` while the target does not hold its backend:

- `StorageAlreadyAttached` means that another cluster holds the backend. The message names that cluster. The restore writes nothing into a backend that another cluster holds.
- `WaitingForHandover` means that the target does not hold the backend yet, or that pods still write it. In the first case the message names the target and the backend. In the second case it names those pods. They can be pods of another cluster, or pods of the target that have not stopped yet, such as its Optimize importer.
- `InvalidReference` can name a Lease that claims the backend and names no `CamundaCluster`. The target cannot take the backend while it exists. Delete the Lease if nothing uses it.

After the restore left `Pending`, these two reasons hold it for 10 minutes, and then it fails. A target that now resolves to another backend than `status.backend` holds it with reason `InvalidReference` for the same time.

### After a failure or a delete

Elasticsearch recovers the snapshots that it accepted, even when the restore fails or you delete it. The restore keeps the backend until no index that it replaces recovers any more. Until then, `status.recoveryHeld` is `true`. A deleted restore stays until then too, and so does its suspension hold on the target. Another restore of the target waits in `Pending` with the reason `ClusterClaimed`. A backup of the target waits with the reason `ClusterSuspended`.

```yaml
status:
  phase: Failed
  recoveryHeld: true
```

The event `RecoveryHeld` marks the start of this hold. The event `RecoveryEnded` marks its end when Elasticsearch finishes the recovery.

If the restore cannot read the recovery for 10 minutes, it gives the backend back and records the Warning event `RecoveryUnknown`. That happens when the Elasticsearch does not answer, when the target is gone, or when the target now points at another Elasticsearch. `status.recoveryHeld` is then `false`, the same as after `RecoveryEnded`. While the restore is held, `status.recoveryUnknownSince` shows since when it cannot read the recovery. The field is gone once the hold ends. After a `RecoveryUnknown` event, make sure that no index recovery is active before you start another cluster on this Elasticsearch.

## The snapshot repository

The restore reads the snapshots from the repository that the backup recorded in `status.repository`, on the Elasticsearch of the target. If that repository is absent, the operator registers it over the bucket that the backup pinned and the prefix that the source cluster wrote under. The snapshots lie under that prefix, whichever Elasticsearch server the target reads through.

An `ElasticsearchCluster` names its repository `<namespace>.<name>`, and that name gives the prefix. The registration that a restore leaves therefore belongs to the source alone. No `ElasticsearchCluster` of the target points it somewhere else.

The prefix is read against the `basePath` that the `ObjectStorageConfig` carries now. Keep `basePath` unchanged for as long as you keep backups. A bucket whose `basePath` moved after a backup holds that backup under the old prefix, and the restore looks under the new one.

If the repository is already registered, the operator uses it as it is. It never points an existing registration at another bucket or another prefix. The Elasticsearch of a target can be a cluster that this operator does not manage, where an administrator registered the repository by hand. A registration that points elsewhere makes the restore fail on a snapshot that is missing, and the message names the repository.

A hand-chosen repository name gives no prefix. If the target holds no registration under such a name, `Ready` reports `InvalidReference` and the message names the repository and the bucket. Register the repository on the Elasticsearch of the target, over the prefix that holds the snapshots of the backup. The restore continues on its own.

## Secondary storage

The operator deletes the Camunda indices of the target first, then asks Elasticsearch to restore every snapshot of the backup. It names the Optimize indices only when the backup holds an Optimize snapshot. A backup without one cannot put those indices back, so the operator keeps them. One `CamundaCluster` holds one contract, see [Secondary storage](camundacluster.md#secondary-storage), so the Camunda indices on the Elasticsearch of the target belong to the target. A second contract that names the same Elasticsearch puts another cluster's data in reach of this delete. Give one contract to one backend.

The restore of a snapshot is asynchronous. The operator waits until the restored indices exist and no shard recovers any more, then it moves on.

`status.restoredSnapshots` names every snapshot the restore already asked for. A restore that names one deletes no index a second time.

CAUTION: A failure between the delete and the restore leaves the secondary storage of the target empty. The backup itself stays whole. The restore then deletes what is there and asks for the snapshots again, so it converges. Do not delete the backup while the restore runs.

### An Optimize attached to the target

A [CamundaOptimize](camundaoptimize.md) whose `clusterRef` names the target follows the suspension of that cluster. So its webapp and its importer go to zero with the cluster. The restore waits in `Pending` until the importer pod is gone, so no import runs when this phase deletes the indices. You do not have to stop the import by hand.

This matters because the Optimize importer reads Elasticsearch directly, not through the orchestration cluster. An importer that keeps running reads indices that are half restored and writes analytics from them. It also holds an import position that disagrees with the restored data. Both workloads start again when you unsuspend the cluster, and the importer reads the restored indices.

## Primary storage

The operator deletes the data volume of every broker and creates it again, then runs the Camunda restore application once per broker with `--backupId=<status.backupId>`.

The volumes belong to the `StatefulSet` of the cluster, not to the restore. Deleting the restore leaves the brokers with their data. Each volume keeps the storage class, the access modes, and the labels of the claim template of the `StatefulSet`. Its size is the size that the backup recorded in `status.storageSizes.zeebe`, and the request of the claim template when the backup recorded none.

The restore Jobs run with the broker configuration, so the restore application reads the same storage with the same credentials as the brokers. A cluster whose broker `StatefulSet` is gone cannot restore until the cluster brings it back. Suspending a cluster keeps the `StatefulSet` in place.

Every Job carries the labels `camunda.io/component: restore`, `camunda.io/logical-restore-elasticsearch: <restore name>`, and `camunda.io/cluster: <target name>`. Deleting the restore removes its Jobs.

## Time limits

A restore that has started waits 10 minutes on a dependency that stops resolving, then it fails. The 10 minutes run from the first outage. Once an index or a volume is gone, a dependency that resolves again starts no second wait. The wait covers an Elasticsearch that does not answer, a reference that breaks, a pod that cannot start, and a target that somebody starts mid-run. A restore that already deleted an index or a volume must reach a terminal phase. That way, whoever owns the cluster learns that it has to act.

A restore in `Pending` waits without a bound, because it deleted nothing yet.

## Identity

The restore pins the backup ID and the identity of the target when it starts. A backup that somebody deletes and creates again under the same name holds other artifacts. A cluster that somebody deletes and creates again under the same name is another cluster. Both end the restore. Create a new restore for the resources as they are now.

## The restore Jobs

The restore runs the Camunda restore application once per broker, as a Job. Each Job pod uses the data volume of its broker. A pod that finished still holds that volume, so the volume cannot terminate while the pod exists.

| Terminal phase | What happens to the Jobs |
| --- | --- |
| `Completed` | The operator deletes them, together with their pods. Kubernetes removes the pods first and the Job last, so the delete takes a moment. The broker data volumes are free once the last pod is gone. |
| `Failed` | The operator keeps them. The logs of a failed Job name the cause, and only the pod keeps them readable. |

**A restore that failed after it started the restore application holds the broker data volumes.** `status.primaryJobNames` tells you which case you are in. A restore that failed in an earlier phase names no Job there and holds nothing.

When it does name Jobs, you read their logs, and then you delete the restore. The delete takes the Jobs and their pods with it, and the volumes are free once the last pod is gone. Until you do that, a second restore of the cluster and the deletion of the cluster both wait on a volume that never terminates. The waiting restore reports the pod that holds the volume and names the resource that runs it.

```bash
# The Jobs that the restore still holds. status.primaryJobNames lists the same names.
kubectl get job -n my-cluster-ns -l camunda.io/logical-restore-elasticsearch=my-cluster-restore

# The log of the Job of broker 0, named the way the command above lists it.
kubectl logs -n my-cluster-ns job/my-cluster-restore-lres-0
```

## Deletion

Deleting the restore removes its Jobs. A restore that completed already removed them. A restore that failed still has them, and this is how you remove them. The recreated broker volumes stay, and so does everything the restore wrote into Elasticsearch. The restore stays until the last pod of its Jobs is gone. While `status.recoveryHeld` is `true`, it also stays until that hold ends, as [After a failure or a delete](#after-a-failure-or-a-delete) describes.

The delete removes the suspension hold of the restore from the target. A target that the restore suspended through `spec.suspend` stays suspended. That is deliberate. Brokers that start over volumes the restore already erased are worse than a cluster that is down. Unsuspend the cluster yourself once you know what its volumes hold.

## Status

| Type | Reason | Meaning | What to do |
| --- | --- | --- | --- |
| `Ready` | `Progressing` | A phase of the restore runs. | Wait. The message names the work. |
| `Ready` | `Completed` | The restore finished, and it gives back the suspension it applied, so the target starts again a moment later, unless another hold remains on it. `Ready` is `True`. | Nothing. Unsuspend the target yourself only when you suspended it yourself. |
| `Ready` | `Failed` | The restore ended. | Read `status.failureMessage`. Correct the cause. Delete the failed restore, then create a new one. |
| `Ready` | `ClusterNotSuspended` | Somebody removed the suspension hold of the restore from the target and cleared `spec.suspend`. | Suspend the cluster again. A restore that already erased something fails 10 minutes after the first outage. |
| `Ready` | `ClusterClaimed` | Another backup or restore holds the cluster. | Wait. The restore starts when the holder reaches a terminal phase. A failed restore that keeps the backend for a recovery holds the cluster until its `status.recoveryHeld` is `false`. |
| `Ready` | `StorageAlreadyAttached` | Another cluster holds the Elasticsearch of the target. The message names it. | Read "The backend" above. The restore starts when the target holds the Elasticsearch. |
| `Ready` | `WaitingForHandover` | The target does not hold its Elasticsearch yet, or pods still write it. | Wait. If the target does not hold the backend yet, the message names the target and the backend. The restore starts once the target takes it. If pods still write the backend, the message names them. The restore starts when they are gone. |
| `Ready` | `IncompatibleTarget` | The target cannot hold the backup. The message names both values. | Read "Compatibility" above. A backup restores into the cluster it was taken from alone. |
| `Ready` | `InvalidReference` | A referenced resource does not exist, or the backup is not completed. Or the snapshot repository of the backup is absent from the target under a name the operator cannot place. | Read the message. Create the resource, wait for the backup, or register the repository on the target. |
| `Ready` | `ConnectionFailed` | The Elasticsearch of the target does not answer, or it refuses the credentials. | Make sure that the endpoint answers and that the credentials of the `SecondaryStorageConfig` are valid. |
| `Ready` | `MissingSecret` | A pod of a restore Job cannot start, because a Secret it needs does not exist. | Create the Secret that the message names. |

These status fields report what the restore did:

- `status.backupId` is the backup that the restore reads.
- `status.backend` is the Elasticsearch that the restore writes, as the scheme, the host, and the port.
- `status.repository` is the snapshot repository on the Elasticsearch of the target.
- `status.restoredSnapshots` names every snapshot that the operator asked Elasticsearch to restore.
- `status.recoveryHeld` is `true` while a restore that failed, or that you deleted, keeps the backend for the recovery of its snapshots.
- `status.recoveryUnknownSince` is the time since which a held restore cannot read that recovery.
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
- The API server accepts a restore that breaks the rules below, because they depend on live state. The restore reports the breach on `Ready` instead: the suspend state of the target, the state of the backup, and the compatibility rules.

## Related

- [LogicalBackupElasticsearch](logicalbackupelasticsearch.md): the backup that this restore reads.
- [CamundaCluster](camundacluster.md): referenced through `targetClusterRef`. You suspend it for the whole restore.
- [SecondaryStorageConfig](secondarystorageconfig.md): resolved through the `storageRef` of the target. It must be `type: elasticsearch`, and it carries the endpoint and the credentials.
- [ObjectStorageConfig](objectstorageconfig.md): resolved through the `backupStorageRef` of the target. It holds the snapshots and the partition backup.
- [CamundaOptimize](camundaoptimize.md): an Optimize attached to the target suspends with it, so its import stops for the whole restore.
- [PointInTimeRestore](pointintimerestore.md): the restore kind that rolls a relational cluster back to a point in time.
- [Backup guide](../guides/backup.md): how to set up backup storage and take a backup.
