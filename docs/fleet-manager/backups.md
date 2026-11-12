# Back up a cluster from the fleet manager

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
| "The cluster is suspended. A backup waits until it runs again, so resume the cluster first." | If you suspended the cluster, resume it, as [Suspend and resume](../guides/operations.md#suspend-and-resume) shows. Otherwise, read the reason of the `Ready` condition of the `CamundaCluster`, and find it in [Why the cluster is at zero replicas](../crds/camundacluster.md#why-the-cluster-is-at-zero-replicas). For example, `StorageAlreadyAttached` means that another cluster holds the secondary storage. |
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

The top of the tab says that backups and restores go straight to the cluster, also in Git mode. In Git mode, the fleet manager commits each claim change to the repository and waits for your GitOps tool to apply it. A backup and the delete of a backup do not go to Git. The fleet manager writes them to the Kubernetes API at once, and the page does not wait for GitOps. These backups are not in the repository, so your GitOps tool does not see them.

## Related

- [Backup](../guides/backup.md): set up the bucket and take a backup with `kubectl`.
- [The fleet view](fleet-view.md): the list of clusters and their last backup.
