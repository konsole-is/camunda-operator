# LogicalBackupElasticsearch

A LogicalBackupElasticsearch is one backup of an Elasticsearch-backed [CamundaCluster](camundacluster.md). The backup is a coordinated set under one backup ID, taken hot. This page is a stub. The full page lands with the end-to-end suite of the backup epic.

## Purpose

The backup captures the web-application indices, the exported Zeebe record indices, and the Zeebe partitions of one cluster. A LogicalRestore can bring the cluster back to this point. You create one by hand for a one-off backup. A [BackupSchedule](backupschedule.md) creates them on a cron schedule.

When you delete the resource, a finalizer deletes the stored artifacts. If the backup was still running, the finalizer resumes exporting on the cluster first. The deletion releases without that resume only when nothing is addressable anymore. That is the case when the cluster is gone, or when a client can no longer be built.

## API reference

```yaml
apiVersion: core.camunda.io/v1
kind: LogicalBackupElasticsearch
metadata:
  name: my-cluster-backup
  namespace: my-cluster-ns
spec:
  # object. Required, immutable. The CamundaCluster to back up. Its secondary
  # storage must be Elasticsearch.
  clusterRef:
    # string. Required. Name of the CamundaCluster.
    name: my-cluster
    # string. Optional, default: this resource's namespace.
    namespace: my-cluster-ns
```

The whole spec is immutable. A backup is one-shot. To retry, create a new resource. `kubectl get lbes` lists the resources with their phase, step, and backup ID.

## Status

`status.phase` is `Pending`, `Running`, `Completed`, or `Failed`. The last two are terminal. `status.step` is the resume marker of the running procedure. `status.backupId` keys every stored artifact. `status.history`, `status.records`, and `status.runtime` track the three parts of the set.

`status.historySnapshots` names the web-application snapshots, so a restore can locate them after the cluster is gone. `status.repository` pins the snapshot repository that the set is written to. A repository that changes mid-run fails the backup instead of splitting the set. `status.storageSizes` records the effective restore sizes, best effort.

The `Ready` condition carries the reasons `Progressing`, `Completed`, `Failed`, `ResumeFailed`, `ClusterSuspended`, `BackupInProgress`, `StorageTypeMismatch`, `InvalidReference`, `MissingSecret`, and `ConnectionFailed`.
