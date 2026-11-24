# The cluster page

The cluster page shows one Camunda cluster: its processes and pods, their logs and events, its backups, its addresses, and its [claim](install.md#what-the-fleet-manager-writes). To open it, select the name of the cluster in [the fleet view](fleet-view.md).

## The tabs of the cluster

| Tab | What it shows |
| --- | --- |
| **Overview** | The **Topology** of the processes, a card for each component and for each storage and access option, and the **Conditions** of the cluster. |
| **Backups** | The backup storage, the schedules, the backups, and the restores, as [Back up and restore a cluster](backups.md) shows. |
| **Endpoints** | The address of each app: its public URL, its Service, and its protocol. |
| **Claim** | The choices for the whole cluster, as [Change the claim in the Claim tab](change-a-cluster.md#change-the-claim-in-the-claim-tab) shows. |

A cluster that has a claim also has **Change topology**, as [Change the topology](change-a-cluster.md#change-the-topology) shows, and **Delete** in its header.

## Before the cluster exists

The operator makes the cluster when each storage and access option of the claim gives its values, as the **Outputs** box of the option shows. For a built-in option, this takes seconds. For a cloud option, it can take minutes.

Until then, the page shows the claim and its options. The box **The cluster does not exist yet** names each option that the claim waits for, with the name of its request. The **Overview** tab shows a card for each option and the conditions of the claim. Select a card to see why its option waits.

The **Backups** and **Endpoints** tabs say that the cluster does not exist yet. The **Claim** tab and **Delete** in the header work as for a running cluster. When the cluster appears, the page shows it. You do not reload the page.

When the operator refuses the claim, the page shows the reason and the message above the tabs. The refusal shows also on a cluster that runs and is ready. Select **Open the Claim tab**, and change the value that the message names.

## The tabs of a component

On the **Overview** tab, select a card, for example **Brokers**, to open that component. **All components** goes back to the cards. A component has five tabs:

| Tab | What it shows |
| --- | --- |
| **Overview** | The conditions and the pods of the component, with the restarts, the node, and the CPU and memory of each pod. The brokers also show the health of each partition. |
| **Metrics** | The live CPU and memory of each pod, and charts over the last hour, 6 hours, or 24 hours, as [Metrics of the charts](settings.md#metrics-of-the-charts) shows. |
| **Logs** | The logs of the pods, as [Logs](#logs) shows. |
| **Events** | The Kubernetes events of the component, as [Events](#events) shows. |
| **Settings** | Each setting with its value and where it comes from, as [Change the settings of a component](change-a-cluster.md#change-the-settings-of-a-component) shows. |

An app that runs inside another process, such as Operate inside the gateway, shows **Embedded in Gateway**. Its tabs show the pods, the logs, and the events of that process.

The CPU and memory of the pods need metrics-server. The charts need Prometheus at `prometheus.url` of the [settings](settings.md).

## Storage and access cards

The cluster has a card for each storage and access option, for example **Backup storage** or **Access**. A card shows the first line that the option reports, and a dot with the state of the option, for example **Not ready (GrantRejected)**. A warning or an error shows in its color.

Select the card to open the option. Its tabs are the same as for a component. The **Overview** tab shows what the option reports:

| Box | What it shows |
| --- | --- |
| **Summary** | The facts that the option reports about itself. |
| **Connected** | For an option such as **Existing bucket**: the object that it connects, and its state. |
| **Grants** | The cloud permissions of the option, as [Grants](#grants) shows. |
| **Outputs** | Each value that the option gives, and the setting of the cluster that it fills. |
| **Inputs** | Each value that the option takes from another option, for example a key from **Encryption key**. |
| **Resources** | Each resource that the option made, and if it is **Kept** or **Deleted** when the cluster is deleted. |

The **Settings** tab lists the parameters of the option. **Edit settings** opens the same editor as on the **Claim** tab.

The options **New Elasticsearch cluster** and **New PostgreSQL server** also show the version, the volumes, and the conditions of the storage that they made.

### Grants

A Grant asks the cloud identity of the cluster for one permission, for example access to a bucket. The **Grants** box shows each Grant with its state. While a Grant is refused, the card of the option shows **Not ready (GrantRejected)**. Read the message of the Grant. For `OverCeiling` or `OverSizeLimit`, pick an option that asks for less, or ask the admin of the identity for a larger ceiling. A Grant that shows **Not applied yet** needs no action.

## Logs

The **Logs** tab follows the logs of the pods of the component, with the newest lines at the bottom. **Pod** picks one pod, or **All pods** for the first 10 pods. Each log starts with its last 200 lines.

The tab shows the time, the level, the logger, and the message of a JSON line in their own columns. Times are in UTC.

- Select **Pause** to stop the lines from moving, and **Follow** to go on.
- Select a level in **Level** to show only the lines of that level.
- Type a text in **Filter** to show only the lines that hold it.

The tab shows the log of the container as it runs now. For the log before the last restart, use `kubectl logs --previous`. When the tab cannot read a log, it names the pod and the cause below the lines. Reload the page to read it again.

## Events

The **Events** tab lists the Kubernetes events of the component, newest first. These are the events of its pods, Services, volumes, and workloads, and of the cluster itself. Kubernetes keeps an event for one hour by default, so an older event does not show.

## Delete the cluster

Select **Delete** in the header of the cluster. Only a cluster that has a claim has **Delete**.

### Keep or delete the resources

The page asks what happens to the resources of the storage and access options, for example a bucket or a database server:

- **Keep the resources**: the resources stay after the cluster is gone.
- **Delete the resources**: the resources go with the cluster.

The choice starts at the deletion policy of the cluster. The page lists each resource with **Kept** or **Deleted**. Some options always delete what they made. For such an option, **Keep the resources** is off, and the option shows **Always deleted**.

The delete always removes the claim, and the `CamundaCluster`, Optimize, and backup schedule that the claim made.

### Volumes and backups

By default, the broker volumes go with the cluster, and the page says "Volumes deleted". To keep them for a later cluster of the same name, set `whenDeleted: Retain` in the claim. Do this before you open the delete page. In Git mode, change the claim file in Git and wait until GitOps applied it:

```yaml
apiVersion: fleet.camunda.io/v1alpha1
kind: CamundaClusterClaim
metadata:
  name: my-cluster
  namespace: my-cluster-ns
spec:
  cluster:
    zeebe:
      persistentVolumeClaimRetentionPolicy:
        whenDeleted: Retain
  # ... the rest of your claim
```

Wait until the page says "Volumes kept for a cluster of the same name". Until the operator applied the change, the page says "Volumes not known", and a delete still removes the volumes.

The backups of the cluster stay. When the backup bucket goes with the cluster, the page warns "You cannot restore my-cluster after this: the backup bucket goes too."

### Confirm the delete

1. Type the name of the cluster.
2. Select **Delete the cluster**. In Git mode, the button is **Push the delete to Git**.

You cannot undo the delete. The page lists each object that goes, with **Waiting**, **Deleting**, or **Deleted**, until the cluster is gone. Then it says "The cluster my-cluster is deleted."

If another tool set the deletion policy, the page asks you to take the field over before it deletes with another policy. [Fields that another tool set](settings.md#fields-that-another-tool-set) says what that means.

### Delete in Git mode

When your choice is the policy that the cluster has, the delete is one commit that removes the file of the claim. Otherwise, the delete is two commits. The first commit sets the policy, and the page shows "Waiting for GitOps to apply the deletion policy." Then the second commit removes the file.

Your GitOps tool must prune, or the cluster stays. **Stop waiting** stops the delete before the second commit, and the first commit stays in the repository. If the claim has no file in the repository, delete it with `kubectl delete camundaclusterclaim my-cluster -n my-cluster-ns`.
