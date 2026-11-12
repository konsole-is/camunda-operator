# Back up and restore a cluster from the fleet manager

The **Backups** tab of a cluster page shows the backup storage, the schedules, the backups, and the restores of the cluster. On a cluster that a `CamundaClusterClaim` made, the tab also has the actions on this page. A cluster that shows **Not managed** in [the fleet view](fleet-view.md) has no actions. Its backups are read only in the fleet manager.

## Back up now

**Back up now** is at the top of the **Backups** tab. It opens a form with one field, **Name**. The field holds a name of the cluster and the current time in UTC, such as `my-cluster-manual-20261110-143005`. You can change the name. The name must be a DNS subdomain name, and no other backup of the namespace can have it. **Start backup** stays off until the name is correct.

**Start backup** creates one backup of the kind that the secondary storage of the cluster needs. A cluster on Elasticsearch gets a [LogicalBackupElasticsearch](../crds/logicalbackupelasticsearch.md). A cluster on PostgreSQL gets a [LogicalBackupRDBMS](../crds/logicalbackuprdbms.md). The backup names the cluster and nothing more:

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

The page says "Backup my-cluster-manual-20261110-143005 created." when the backup exists. The new backup then shows in the table of backups, where you follow its phase. If the name is taken by then, the backup is not created, and the page shows the error.

**Back up now** is off, with the reason under it, in these cases:

| Reason on the page | What to do |
| --- | --- |
| "This cluster has no backup storage." | The claim chose no bucket in its **Backup storage** section when it was created. The sections of a claim are fixed after creation. To back up from the fleet manager, create a cluster with a bucket in that section. |
| "`SecondaryStorageConfig <name>` not found, so the kind of backup is not known." | Create the `SecondaryStorageConfig` that `spec.storageRef` of the cluster names. |
| "The cluster is suspended. A backup waits until it runs again, so resume the cluster first." | `spec.suspend` of the cluster is `true`. Resume the cluster, as [Suspend and resume](../guides/operations.md#suspend-and-resume) shows. |
| "A suspension hold keeps the cluster suspended, and a backup waits until the cluster runs again. A failed restore keeps its hold until you delete that restore." | Find the failed restore in the table of restores, and read its failure. Then delete the restore, as [Suspension holds](../crds/camundacluster.md#suspension-holds) shows. |
| "Another cluster holds the secondary storage of this cluster, so the operator keeps this cluster suspended, and a backup waits until it runs again. Give this cluster storage of its own, or delete the cluster that holds the storage." | Do one of the two, as [A second cluster on a held backend](../crds/camundacluster.md#a-second-cluster-on-a-held-backend) shows. |
| "Something else still writes the secondary storage of this cluster, so the operator keeps this cluster suspended, and a backup waits until it runs again. Wait, or remove what the Ready condition of the cluster names." | Read the message of the `Ready` condition of the `CamundaCluster`, as [Waiting for a handover](../crds/camundacluster.md#waiting-for-a-handover) shows. |
| "Changes are off: the fleet manager could not read its settings." | Make sure that the fleet manager can read the ConfigMap `fleet-manager-settings`, as [Fleet manager settings](settings.md) shows. |

A cluster holds one backup or one restore at a time, as [One backup or restore at a time](../guides/operations.md#one-backup-or-restore-at-a-time) shows. While a backup or a restore of the cluster runs, **Back up now** stays on. The form then says, for example, "Restore my-cluster-restore-1 is running. The new backup waits until it ends." The new backup waits in `Pending` and starts on its own. A restore also suspends the cluster while it runs. This suspension does not turn **Back up now** off, because it ends with the restore. Any other suspension turns it off.

An Elasticsearch backup that ended as `Failed` with the reason `ResumeFailed` also holds the cluster, until you delete it. The form then says "Backup my-cluster-manual-20261110-143005 ended as ResumeFailed and still holds the cluster. The new backup waits until you delete that backup."

## Delete a backup

Each row of the table of backups has **Delete**. It opens a confirm in the row that lists what goes and what stays. **Delete backup** deletes the backup resource. The page then says "Backup my-cluster-manual-20261110-143005 deleted."

| Backup | What goes | What stays |
| --- | --- | --- |
| Elasticsearch | The backup, with its Elasticsearch snapshots and its Zeebe partition backup in the bucket | Nothing |
| PostgreSQL | The backup, with its database dump in the bucket | The Zeebe backups. Zeebe removes them after the retention window of the cluster. |

When you delete a backup that is still `Pending` or `Running`, its run stops too. On Elasticsearch, the operator resumes exporting on the cluster if the backup paused it. On PostgreSQL, the operator deletes the dump Job of the backup.

A backup of a schedule shows **Scheduled** in the column **Origin**. When you delete such a backup, the other backups of the schedule stay. The schedule deletes the oldest of them beyond the counts that it keeps, as [BackupSchedule](../crds/backupschedule.md) shows.

**Delete** is off while a restore that has not ended reads the backup. The reason under it names the restore:

> Restore my-cluster-restore-1 reads this backup. Delete the backup after the restore ends.

If such a restore starts after the page showed the backup, **Delete backup** deletes nothing. The page then shows the reason:

> Restore my-cluster-restore-1 reads the backup my-cluster-manual-20261110-143005. Delete the backup after the restore ends.

A delete during the restore can leave the secondary storage of the cluster empty, as [LogicalRestoreElasticsearch](../crds/logicalrestoreelasticsearch.md) shows. When the page cannot read its settings, **Delete** is off with the same reason as **Back up now**.

The operator removes the artifacts before the resource goes. When it cannot reach them, the artifacts stay in the bucket, and an event on the backup names them. The Deletion section of [LogicalBackupElasticsearch](../crds/logicalbackupelasticsearch.md#deletion) and [LogicalBackupRDBMS](../crds/logicalbackuprdbms.md#deletion) names the cases.

## Restore from a backup

Each row of a `Completed` backup has **Restore**. A backup in another phase has no **Restore**. **Restore** opens a confirm. Its first line says that the restore loses everything that the cluster wrote after the backup. Under it, the confirm lists these facts:

- The Camunda version of the backup and the Camunda version of the cluster. The restore sets the version of the cluster to the version of the backup.
- What the restore deletes. A restore of an Elasticsearch backup deletes the Camunda indices in Elasticsearch and restores the snapshots of the backup. A restore of a PostgreSQL backup runs `pg_restore` over the database of the cluster. `pg_restore` drops each table of the dump and creates it again. Both kinds delete the broker volumes and create them again from the Zeebe data of the backup.
- Whether the cluster starts again. The restore suspends a running cluster while it runs, and starts it again when it completes. A cluster that you suspended yourself stays suspended after the restore, and you resume it. A cluster that a suspension hold or its secondary storage keeps suspended stays suspended until that suspension ends.
- What a failed restore leaves. The Camunda indices or the database, and the broker volumes, can be empty or half written. Then the cluster has no usable data, and it stays suspended. Do not resume it. Correct the cause and create a new restore, as [When the cluster starts again](../guides/operations.md#when-the-cluster-starts-again) shows.

Type the name of the cluster, then select **Restore**. The fleet manager creates one restore of the kind that the backup needs. A `LogicalBackupElasticsearch` gets a [LogicalRestoreElasticsearch](../crds/logicalrestoreelasticsearch.md). A `LogicalBackupRDBMS` gets a [LogicalRestoreRDBMS](../crds/logicalrestorerdbms.md). The name is the cluster, `restore`, and the current time in UTC:

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

If a restore of the namespace has that name, the fleet manager adds `-2`, `-3`, and so on. The page then says "Restore my-cluster-restore-20261110-143005 created." with the name that it used, and the restore shows in the table of restores. What the restore does to the cluster, and what to do when it fails, is in [Operations: Restore a cluster](../guides/operations.md#restore-a-cluster).

**Restore** is off, with the reason under it, in these cases:

| Reason on the page | What to do |
| --- | --- |
| "Restore my-cluster-restore-1 of this cluster has not ended. Start a new restore after it ends." | Wait until the restore is `Completed` or `Failed`. |
| "Backup my-cluster-manual-1 recorded no Camunda version, so it cannot be restored." | For both kinds. Choose another backup. A backup without a version of the form `x.y.z` does not restore. |
| "Backup my-cluster-manual-1 has Camunda 8.8.4. It restores only into Camunda 8.8 or 8.9, and the cluster has 8.10.1." | Only for PostgreSQL. Choose a backup whose Camunda minor is the minor of the cluster, or one minor below it. |
| "The cluster has no Camunda version yet, so the version rule cannot be checked." | Only for PostgreSQL. Wait until the cluster reports its version. |
| "Changes are off: the fleet manager could not read its settings." | Make sure that the fleet manager can read the ConfigMap `fleet-manager-settings`, as [Fleet manager settings](settings.md) shows. |

An Elasticsearch backup has no minor rule. The restore sets the exact version of the backup on the cluster, as [LogicalRestoreElasticsearch: Compatibility](../crds/logicalrestoreelasticsearch.md#compatibility) shows. A PostgreSQL backup restores only into the same Camunda minor or one minor newer, as [LogicalRestoreRDBMS: Compatibility](../crds/logicalrestorerdbms.md#compatibility) shows. A cluster that ran the newer minor comes back on the minor of the backup. [The version after a logical restore](../guides/operations.md#the-version-after-a-logical-restore) shows how to upgrade it again.

While a backup of the cluster runs, **Restore** stays on, and the confirm says that the new restore waits. For example:

> Backup my-cluster-manual-1 is running. The new restore waits until it ends.

A cluster holds one backup or one restore at a time, as [One backup or restore at a time](../guides/operations.md#one-backup-or-restore-at-a-time) shows.

When you select **Restore**, the fleet manager reads the backup and the restores of the namespace again. The page can be older than these objects. So the fleet manager creates nothing, and the page shows the reason, in these cases:

- After the page chose the name, another restore took that name.
- A restore of the cluster has not ended.
- The backup does not exist, it is not `Completed`, or it is being deleted.
- The backup is a backup of another cluster.

## Restore to a point in time

**Restore to a point in time** is at the top of the **Backups** tab of a cluster on PostgreSQL. It shows only when the operator rolls the database server back, that is when the `DatabaseServerConfig` of the cluster has `pitr.recovery: operator`. A server with `pitr.recovery: external` gets no point-in-time restore in the fleet manager. [PointInTimeRestore: Who rolls the database back](../crds/pointintimerestore.md#who-rolls-the-database-back) shows the two values.

The confirm lists the restore window. The window holds the times that are inside both of these:

- The archive window of the database server, as the **Backups** tab shows it.
- The time in which Zeebe keeps its primary-storage backups. `spec.backup.primaryStorage.retention.window` of the cluster sets it, and the default is `P7D`.

Each interval of the window shows its start and its end in UTC. An interval that ends where the uploads of the archive started to fail says "Uploads are failing after this point."

Enter a time in UTC in **Time (UTC)**. A time outside the window, or a time in the future, shows an error. A time inside the window shows the confirm. Its first line says that the restore loses everything that the cluster wrote after that time. Under it, the confirm lists these facts:

- The database server rolls back to the time. The restore deletes the broker volumes and creates them again from the Zeebe backup that covers the point.
- Whether the cluster starts again, as for **Restore**.
- If no Zeebe backup covers the point, the restore fails after it erased the broker volumes.
- What a failed restore leaves, as for **Restore**. The broker volumes can be empty or half written.

Type the name of the cluster, then select **Restore**. The fleet manager creates a [PointInTimeRestore](../crds/pointintimerestore.md). The name is the cluster, `pitr`, and the current time in UTC:

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

The window shows where a restore can reach. The fleet manager cannot see the Zeebe backups, so it cannot prove that a backup covers the point.

CAUTION: Choose a point at least one period of `spec.backup.primaryStorage.schedule` before the cluster stopped writing. The default period is one hour. If no Zeebe backup covers the point, the restore erases the broker volumes and then fails. [PointInTimeRestore: Choose the point to restore to](../crds/pointintimerestore.md#choose-the-point-to-restore-to) shows how to read the real window.

A point-in-time restore writes no version on the cluster. While a backup of the cluster runs, **Restore to a point in time** stays on, and the confirm says that the new restore waits.

**Restore to a point in time** is off, with the reason under it, in these cases:

| Reason on the page | What to do |
| --- | --- |
| "The archive has no restore point yet." | Wait until the database server archives its first base backup. |
| "No point is inside both the archive window and the P7D in which Zeebe keeps its backups." | The archive holds no point that Zeebe still covers. `P7D` is the `retention.window` of the cluster. Use **Restore** with a backup instead. |
| "Restore my-cluster-restore-1 of this cluster has not ended. Start a new restore after it ends." | Wait until the restore is `Completed` or `Failed`. |
| "Changes are off: the fleet manager could not read its settings." | Make sure that the fleet manager can read the ConfigMap `fleet-manager-settings`, as [Fleet manager settings](settings.md) shows. |

## Change the schedule

**Change schedule** opens the attachments on the **Claim** tab of the cluster. The claim holds the schedule of its cluster in `spec.attachments.backupSchedule`, so you change the schedule there:

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

The schedule is a five-field cron expression in UTC. The claim then writes the `BackupSchedule` `my-cluster-backups`, as [CamundaClusterClaim](../crds/camundaclusterclaim.md) shows. A change of the schedule is a change of the claim, so in Git mode it goes to the repository as a commit.

## Git mode

The top of the tab says that backups and restores go straight to the cluster, also in Git mode. In Git mode, the fleet manager commits each claim change to the repository and waits for your GitOps tool to apply it. A backup, a restore, and the delete of a backup do not go to Git. The fleet manager writes them to the Kubernetes API at once, and the page does not wait for GitOps. These backups and restores are not in the repository, so your GitOps tool does not see them. A restore writes `spec.suspend` and `spec.version` on the `CamundaCluster`. If your GitOps tool owns the cluster, read [A GitOps tool that owns the CamundaCluster](../guides/operations.md#a-gitops-tool-that-owns-the-camundacluster) before the restore.

## Related

- [Backup](../guides/backup.md): set up the bucket and take a backup with `kubectl`.
- [Operations: Restore a cluster](../guides/operations.md#restore-a-cluster): what a restore does to the cluster, and a failed restore.
- [The fleet view](fleet-view.md): the list of clusters and their last backup.
