# LogicalBackupElasticsearch

A LogicalBackupElasticsearch is one backup of an Elasticsearch-backed [CamundaCluster](camundacluster.md). The backup is a coordinated set under one backup ID, taken hot. This page is a stub. The full page lands with the end-to-end suite of the backup epic.

## Purpose

The backup captures the web-application indices, the exported Zeebe record indices, and the Zeebe partitions of one cluster. A completed backup is a restore point, keyed by its backup ID and its recorded snapshot names. You create one by hand for a one-off backup. A [BackupSchedule](backupschedule.md) creates them on a cron schedule.

When you delete the resource, a finalizer deletes the stored artifacts. If the backup was still running, the finalizer resumes exporting on the cluster first. The deletion releases without that resume only when nothing is addressable anymore. That is the case when the cluster is gone, or when a client can no longer be built.

## API reference

```yaml
apiVersion: core.camunda.io/v1
kind: LogicalBackupElasticsearch
metadata:
  name: my-cluster-backup
  namespace: my-cluster-ns
spec:
  # object. Required, immutable. The CamundaCluster to back up, in the
  # namespace of this resource. Its secondary storage must be Elasticsearch.
  clusterRef:
    # string. Required. Name of the CamundaCluster.
    name: my-cluster
```

The reference cannot cross namespaces. The operator reads the Secrets of the cluster and calls its management API on behalf of the backup, so the backup must live in the namespace of the cluster.

The whole spec is immutable. A backup is one-shot. To retry, create a new resource. `kubectl get lbes` lists the resources with their phase, step, and backup ID.

## Status

`status.phase` is `Pending`, `Running`, `Completed`, or `Failed`. The last two are terminal. `status.step` is the resume marker of the running procedure. `status.backupId` keys every stored artifact. `status.history`, `status.records`, and `status.runtime` track the three parts of the set.

`status.historySnapshots` names the web-application snapshots, so a restore can locate them after the cluster is gone. `status.repository` pins the snapshot repository that the set is written to. `status.storage` pins the storage contract and the Elasticsearch endpoint. A repository, contract, or endpoint that changes mid-run fails the backup instead of splitting the set. The finalizer holds the deletion while the contract points at another endpoint, so it never deletes against the wrong cluster. `status.storageSizes` records the effective restore sizes, best effort.

`status.failureMessage` names the step that failed. `status.resumeFailureMessage` stands beside it when the procedure also failed to resume exporting. Both survive, so a backup that ends as `ResumeFailed` still says which step failed first.

The steps that call Elasticsearch run with exporting paused. An unreachable Elasticsearch endpoint is retried for ten minutes, then the step fails and the procedure resumes exporting. An unreachable management API at a backup step is retried without a bound, because nothing else can resume the cluster. The resume itself is bounded: the procedure retries it until the accumulated time of active attempts reaches the resume deadline, 30 minutes by default. Time in which the procedure is parked, for example while the cluster is suspended, does not count. After the deadline the backup ends as `Failed` with reason `ResumeFailed`, `status.resumeFailureMessage` says why, and exporting stays paused. Deleting such a backup tries the resume again before the finalizer releases.

The runtime backup is requested in two reconciles. `status.runtimeRequestedTime` records the intent first, then the request is sent. A lost response or a restart finds the intent and does not request a second backup under a fresh ID. `status.runtimeAcceptedTime` records when the cluster first acknowledged the ID: it accepted the request, or it reported the ID as held. The cluster registers the backup asynchronously, so an absent backup is polled through a two-minute registration grace that starts at the acceptance. Operator downtime before the request cannot consume it. An ID the cluster still does not hold after the grace fails the step: the request was lost, or another actor holds the ID.

The backup ID is the clock in milliseconds, raised past the highest ID of the other backups of this kind that name the same cluster. A clock that stepped back therefore cannot reuse an ID a visible sibling holds. The IDs of the other backup kind and of deleted resources stay with the cluster, which answers a repeated or lower ID with a conflict.

Deleting a backup while its history backup is still `IN_PROGRESS` waits until the web applications report a terminal state. Camunda 8.9 offers no call that cancels a history backup, and the snapshots it still creates would leak. The hold has no bound, like the hold on a runtime backup that is still in progress.

Every message that carries an error from the cluster or from Elasticsearch is bounded, so one oversized answer never makes the status unwritable.

The `Ready` condition carries the reasons `Progressing`, `Completed`, `Failed`, `ResumeFailed`, `ClusterSuspended`, `BackupInProgress`, `StorageTypeMismatch`, `InvalidReference`, `MissingSecret`, and `ConnectionFailed`.
