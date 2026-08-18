# LogicalBackupElasticsearch

A LogicalBackupElasticsearch is one backup of an Elasticsearch-backed [CamundaCluster](camundacluster.md). The backup is a coordinated set under one backup ID, taken hot. This page is a stub. The full page lands with the end-to-end suite of the backup epic.

## Purpose

The backup captures the web-application indices, the exported Zeebe record indices, and the Zeebe partitions of one cluster. A completed backup is a restore point, keyed by its backup ID and its recorded snapshot names. You create one by hand for a one-off backup.

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

`status.historySnapshots` names the web-application snapshots, so a restore can locate them after the cluster is gone. `status.repository` pins the snapshot repository that the set is written to. `status.storage` pins the storage contract and the Elasticsearch endpoint. Every step that writes into Elasticsearch verifies the pinned destination first, before the history backup starts and before its status is trusted, and before the record indices are snapshotted. A repository, contract, or endpoint that changed mid-run fails the backup instead of splitting the set, and the failure message names the pinned and the current destination. The finalizer holds the deletion while the contract points at another endpoint, so it never deletes against the wrong cluster. `status.storageSizes` records the effective restore sizes, best effort. They are computed at the start and backfilled only while exporting runs, before the pause and after the resume, so a slow Elasticsearch never delays a poll or a resume attempt.

`status.failureMessage` names the step that failed. `status.resumeFailureMessage` stands beside it when the procedure also failed to resume exporting. Both survive, so a backup that ends as `ResumeFailed` still says which step failed first.

The steps that call Elasticsearch run with exporting paused. An unreachable Elasticsearch endpoint is retried for ten minutes, then the step fails and the procedure resumes exporting. An unreachable management API at a backup step is retried without a bound, because nothing else can resume the cluster. The resume itself is bounded: the procedure retries it until the accumulated time of active attempts reaches the resume deadline, 30 minutes by default. The time inside each attempt counts, so attempts that hit the client timeout exhaust the deadline in about the deadline. Time in which the procedure is parked, for example while the cluster is suspended, does not count. After the deadline the backup ends as `Failed` with reason `ResumeFailed`, `status.resumeFailureMessage` says why, and exporting stays paused. The backup keeps its claim on the cluster and does not retry the resume on its own. Deleting it tries the resume again, and the finalizer releases only after the resume succeeded, so no sibling meets the paused cluster.

The runtime backup is requested in two reconciles. `status.runtimeRequestedTime` records the intent first, then the request is sent. A lost response or a restart finds the intent and does not request a second backup under a fresh ID. `status.runtimeAcceptedTime` records when the cluster accepted the request of this backup, as the operator observed it. It is the only evidence that the runtime backup under the ID is this backup's. The cluster registers the backup asynchronously, so an absent backup is polled through a two-minute registration grace that starts at the acceptance. Operator downtime before the request cannot consume it. An ID the cluster still does not hold after the grace fails the step.

A runtime backup that exists under the ID without a recorded acceptance is never adopted. It can be this backup's, after a response that was lost, or another actor's that won the ID; the cluster gives no token to tell them apart. The step fails through resume, the message names the ID and says that the backup was not adopted, and the finalizer leaves that runtime backup alone. A crash of the operator between the request and the write of the acceptance therefore fails the backup safely and can leave a runtime backup under that ID in the cluster, for the user to remove by hand with the management API. A conflict answer to the request is treated the same way and is never an acceptance.

The backups of one cluster run one at a time, across both backup kinds. The claim on a cluster is a `coordination.k8s.io` Lease in the namespace of the cluster, named `camunda-backup-<cluster>`, with the holder identity `<Kind>/<Name>/<UID>` of the backup. The API server creates the Lease atomically, so the first backup to claim wins under concurrent reconciles and across the two controllers. A backup that finds the Lease held waits as `Pending` with reason `BackupInProgress` and names the holder. Among the pending backups of one kind, the older one, then the smaller name, tries the claim first; that order is a fairness pre-filter and never decides who holds the cluster. The Lease goes back when the holder reaches a terminal phase and when a deleted holder releases its finalizer. A Lease whose holder is gone, was recreated under the same name, or is terminal is taken over by the next claimant. A Lease that another actor wrote blocks until that actor removes it.

The claim follows the pause, not the phase. A backup that ends as `ResumeFailed` left the cluster's exporting paused, and it keeps its Lease: a sibling must not back up a paused cluster. Such a holder is never taken over, and a waiting sibling says that the cluster is paused and needs the holder's deletion or repair. Deleting the holder resumes exporting in its finalizer, then releases the Lease, and only then does the sibling start.

The backup ID is the clock in milliseconds, raised past the highest ID of the other backups of this kind that name the same cluster. A clock that stepped back therefore cannot reuse an ID a visible sibling holds. The IDs of the other backup kind and of deleted resources stay with the cluster, which answers a repeated or lower ID with a conflict.

Deleting a backup while its history backup is still `IN_PROGRESS` waits until the web applications report a terminal state. Camunda 8.9 offers no call that cancels a history backup, and the snapshots it still creates would leak. The hold has no bound, like the hold on a runtime backup that is still in progress.

Every message that carries an error from the cluster or from Elasticsearch is bounded, so one oversized answer never makes the status unwritable.

The `Ready` condition carries the reasons `Progressing`, `Completed`, `Failed`, `ResumeFailed`, `ClusterSuspended`, `BackupInProgress`, `StorageTypeMismatch`, `InvalidReference`, `MissingSecret`, and `ConnectionFailed`.
