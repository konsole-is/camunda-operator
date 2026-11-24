# Back up and restore a cluster from the fleet manager

The **Backups** tab of a cluster page shows the backup storage, the schedules, the backups, and the restores of the cluster. A cluster that shows **Not managed** has no actions here.

Backups and restores go straight to the Kubernetes API, also in Git mode. They are not in the Git repository. A restore changes `spec.suspend` and `spec.version` of the `CamundaCluster`. If your GitOps tool owns the cluster, read [A GitOps tool that owns the CamundaCluster](../guides/operations.md#a-gitops-tool-that-owns-the-camundacluster) before the restore.

## Back up now

1. Select **Back up now** at the top of the **Backups** tab.
2. Keep the **Name**, or type another one. The default is the cluster name and the time in UTC, such as `my-cluster-manual-20261110-143005`.
3. Select **Start backup**.

The fleet manager creates the backup kind that the secondary storage needs. A cluster on Elasticsearch gets a [LogicalBackupElasticsearch](../crds/logicalbackupelasticsearch.md), and a cluster on PostgreSQL a [LogicalBackupRDBMS](../crds/logicalbackuprdbms.md):

```yaml
apiVersion: core.camunda.io/v1
kind: LogicalBackupElasticsearch
metadata:
  name: my-cluster-manual-20261110-143005
  namespace: my-cluster-ns
spec:
  clusterRef:
    name: my-cluster
```

The new backup shows in the table of backups, where you follow its phase.

**Back up now** is off when the cluster cannot take a backup, and the reason shows under it. The common reasons are:

| Reason on the page | What to do |
| --- | --- |
| "This cluster has no backup storage." | The backup storage of a cluster is fixed at creation. Create a cluster with a bucket in **Backup storage**. |
| "The cluster is suspended. A backup waits until it runs again, so resume the cluster first." | Resume the cluster, as [Suspend and resume](../guides/operations.md#suspend-and-resume) shows. |
| A message about a suspension hold or about the secondary storage | The cluster is held suspended. Do what the message says. |

A cluster takes one backup or one restore at a time. While another backup or restore runs, the new backup waits in `Pending` and then starts on its own.

## Delete a backup

Select **Delete** on the row of the backup, then **Delete backup**.

| Backup | What goes | What stays |
| --- | --- | --- |
| Elasticsearch | The backup, with its Elasticsearch snapshots and its Zeebe backup in the bucket | Nothing |
| PostgreSQL | The backup, with its database dump in the bucket | The Zeebe backups. Zeebe removes them after the retention window of the cluster. |

A backup of a schedule shows **Scheduled** in the column **Origin**. The schedule deletes its oldest backups beyond the counts that it keeps. **Delete** is off while a restore reads the backup.

## Restore from a backup

1. Select **Restore** on the row of a `Completed` backup.
2. Read the confirm. The restore loses everything that the cluster wrote after the backup.
3. Type the name of the cluster.
4. Select **Restore**.

The confirm lists what the restore does:

- It sets the Camunda version of the cluster to the version of the backup.
- It replaces the data in Elasticsearch or in the database, and the broker volumes, with the data of the backup.
- It suspends a running cluster while it runs, and starts it again when it completes. A cluster that you suspended yourself stays suspended.

!!! warning "A failed restore"
    A failed restore can leave the data and the broker volumes empty or half written. The cluster then stays suspended. Do not resume it. Correct the cause and create a new restore, as [When the cluster starts again](../guides/operations.md#when-the-cluster-starts-again) shows.

The fleet manager creates a [LogicalRestoreElasticsearch](../crds/logicalrestoreelasticsearch.md) or a [LogicalRestoreRDBMS](../crds/logicalrestorerdbms.md):

```yaml
apiVersion: core.camunda.io/v1
kind: LogicalRestoreRDBMS
metadata:
  name: my-cluster-restore-20261110-143005
  namespace: my-cluster-ns
spec:
  backupRef:
    name: my-cluster-manual-20261110-120000
  targetClusterRef:
    name: my-cluster
```

A backup that is more than one minor newer than the brokers of the cluster cannot restore. The restore then fails at once with `IncompatibleTarget`, and the cluster stays as it was. [Operations: Restore a cluster](../guides/operations.md#restore-a-cluster) says more about a restore.

## Restore to a point in time

A cluster on PostgreSQL has **Restore to a point in time**. `pitr.recovery` on the `DatabaseServerConfig` of the cluster says who rolls the database server back. See [PointInTimeRestore](../crds/pointintimerestore.md#who-rolls-the-database-back).

```yaml
apiVersion: core.camunda.io/v1
kind: DatabaseServerConfig
metadata:
  name: my-db-server
  namespace: my-cluster-ns
spec:
  pitr:
    enabled: true
    retentionPeriodDays: 7
    recovery: external
  # ... the rest of your server config
```

The confirm shows the restore window in UTC. With `operator`, the operator rolls the server back, and the window comes from the archive of the server. With `external`, the default, you roll the server back to the point before you confirm. The window then reaches back as far as the cluster keeps its Zeebe backups, and not more than `retentionPeriodDays`.

**Restore to a point in time** is off in these cases, and the reason shows under it:

- With `external`, `pitr.enabled` is false.
- No [Database](../crds/database.md) resource created the database of the cluster. The operator restores to a point in time only a database that a `Database` resource created.
- Another `Database` resource uses the same server. A rollback of the server rolls that database back too.

CAUTION: Choose a point at least one period of `spec.backup.primaryStorage.schedule` before the cluster stopped writing. The default period is one hour. If no Zeebe backup covers the point, the restore erases the broker volumes and then fails. [Choose the point to restore to](../crds/pointintimerestore.md#choose-the-point-to-restore-to) shows how to read the real window.

1. With `external`, select **Suspend the cluster** in **Basics** on the **Claim** tab. See [Change the claim in the Claim tab](change-a-cluster.md#change-the-claim-in-the-claim-tab). A cluster that runs writes to the database after the rollback.
2. Select **Restore to a point in time**.
3. Enter the time in **Time (UTC)**. The field refuses a time outside the restore window.
4. With `external`, roll the database server back to that time. Roll back only a server that holds no other database. The fleet manager sees only the `Database` resources of the namespace. If the provider creates a new instance for the rollback, set `host` on the `DatabaseServerConfig` to that instance. Amazon RDS is one such provider.
5. Read the confirm. The restore loses everything that the cluster wrote after that time.
6. With `external`, select **I rolled the database server back to this time**.
7. Type the name of the cluster, and select **Restore**.
8. With `external`, clear **Suspend the cluster** on the **Claim** tab when the restore is `Completed`. A cluster that you suspended stays suspended after the restore.

The fleet manager creates a [PointInTimeRestore](../crds/pointintimerestore.md):

```yaml
apiVersion: core.camunda.io/v1
kind: PointInTimeRestore
metadata:
  name: my-cluster-pitr-20261110-143005
  namespace: my-cluster-ns
spec:
  clusterRef:
    name: my-cluster
  timestamp: "2026-11-09T09:30:15Z"
```

## Change the schedule

**Change schedule** opens the attachments on the **Claim** tab of the cluster. The schedule is a five-field cron expression in UTC:

```yaml
apiVersion: fleet.camunda.io/v1alpha1
kind: CamundaClusterClaim
metadata:
  name: my-cluster
  namespace: my-cluster-ns
spec:
  attachments:
    backupSchedule:
      schedule: "0 2 * * *"
  # ... the rest of your claim
```

A change of the schedule is a change of the cluster, so in Git mode it goes to the repository as a commit.

## Related

- [Backup](../guides/backup.md): set up the bucket and take a backup with `kubectl`.
- [Operations: Restore a cluster](../guides/operations.md#restore-a-cluster): what a restore does to the cluster, and a failed restore.
