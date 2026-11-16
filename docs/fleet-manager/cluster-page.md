# The cluster page

The cluster page shows one Camunda cluster: its processes and pods, their logs and events, its backups, its addresses, and its claim. To open it, select the name of the cluster in [the fleet view](fleet-view.md). The address of the page is `/fleet/clusters/<namespace>/<name>`, for example `/fleet/clusters/my-cluster-ns/my-cluster`.

## The tabs of the cluster

| Tab | What it shows |
| --- | --- |
| **Overview** | The **Topology** of the processes and their pods, and a card for each component and for each selection of the claim. Then the **Conditions** of the cluster, of Optimize, and of the claim. |
| **Backups** | The backup storage, the schedules, the backups, and the restores, as [Back up and restore a cluster](backups.md) shows. |
| **Endpoints** | The address of each app: its public URL, its Service, its protocol, and the process that serves it. |
| **Claim** | The choices of the claim for the whole cluster, as [Change the claim in the Claim tab](change-a-cluster.md#change-the-claim-in-the-claim-tab) shows. |

A cluster that a claim made also has the button **Change topology** in the **Topology** panel, as [Change the topology](change-a-cluster.md#change-the-topology) shows. It also has the button **Delete** in its header, as [Delete the cluster](#delete-the-cluster) shows.

## The tabs of a component

On the **Overview** tab, select a card in **Components** to open that component, for example **Brokers**. A card also lists the apps that run inside it, and you can select each app. Select **All components** to go back to the cards.

A component has five tabs:

| Tab | What it shows |
| --- | --- |
| **Overview** | The conditions of the component, and a table of its pods. The table shows the ready containers, the phase, the node, the restarts, the age, and the CPU and memory of each pod. The brokers also show the health of each partition. |
| **Metrics** | The live CPU and memory of each pod, then charts over the last hour, 6 hours, or 24 hours, as [Metrics of the charts](settings.md#metrics-of-the-charts) shows. |
| **Logs** | The logs of the pods, as [Logs](#logs) shows. |
| **Events** | The Kubernetes events of the component, as [Events](#events) shows. |
| **Settings** | Each field of the component with its value and where the value comes from, as [Change the settings of a component](change-a-cluster.md#change-the-settings-of-a-component) shows. |

An app that runs inside another process has no pods of its own. Its page says, for example, **Embedded in Gateway**. Its tabs show the pods, the logs, and the events of that process.

## Storage, security, and access

A cluster that a claim made also has a card for each selection of the claim, for example **Backup storage** or **Access**. These cards come after the cards of the Camunda processes, in the order of the sections. Each card has the label of its section.

A card shows the first item of the summary of the request of its section, and the number of resources of the request. A Warning or an Error item shows in the color of its severity, with a mark in front. A screen reader reads the severity before the value. The dot of the card shows the `Ready` condition of the request, for example **Not ready (GrantRejected)**. When the request does not exist yet, the card shows **Request: Not made yet**. When the read of the requests fails, the card shows "Request: Could not read" in place of the summary. While the list of the requests is out of date, for example after the page lost its connection, the card shows **Request: Not known**. With either of these two texts, the dot of the card shows no state.

Select the card to open the request. Its tabs show:

| Tab | What it shows |
| --- | --- |
| **Overview** | The conditions of the request, a table of its pods, and what the request made, as [The Overview of a section](#the-overview-of-a-section) shows. |
| **Metrics** | The live CPU and memory of the pods of the request. Then the charts of those pods, and the charts that the registration adds. |
| **Logs** | The logs of the pods of the request, and the lines about the request in the logs of its controller. The group **Controller** of the list **Pod** holds the pods of the controller. |
| **Events** | The events of the `PluginRequest` and of its pods. |
| **Settings** | The parameters of the selection, as [The Settings tab of a section](#the-settings-tab-of-a-section) shows. |

A request that runs no pods says "This component runs no pods." on its **Overview** tab and on its **Metrics** tab. Its **Events** tab lists only the events of the `PluginRequest`. When the app cannot read the requests, the **Overview** tab says `Could not read PluginRequest:` and the error. When the request does not exist yet, the **Overview** tab says "The claim controller has not made the request of this section yet."

The plugin of a section writes the pod selector of its request. When that pod selector is not valid, the **Overview** tab says "The pod selector of the request is not valid", with the reason, and the detail shows no pods.

### The Overview of a section

Below the conditions, the **Overview** tab of a section shows a box for each part of the request that has something to show:

| Box | What it shows |
| --- | --- |
| **Summary** | The summary items of the request, in the order that the plugin wrote them. A Warning or an Error item shows in the color of its severity. "Updated 4m ago" says when the plugin last wrote the summary. |
| **Connected** | The contracts that an existing option connects. See [Existing contracts](#existing-contracts). |
| **Principal** | For **Connectors permissions** only: the cloud role of the Connectors from the output `principal`, and their ServiceAccount, for example `my-cluster-connectors`. |
| **Grants** | The Grants of the request. See [Grants](#grants). |
| **Outputs** | Each output of the request, its value, and the field that it fills, for example `CamundaCluster spec.zeebe.storageClassName`. An output that fills no field shows **No field**. |
| **Inputs** | Each input of the option, the section and the output that it comes from, for example **Encryption key / keyRef**, and its value. The value shows **Waiting** until the claim controller gives it. |
| **Used by** | The sections whose options take an output of this section as an input. For example, **Encryption key** shows **Volume encryption** and **Backup storage** when both take its key. |
| **Links** | The links of the request, for example to an app of the cluster. |
| **Resources** | Each resource that the plugin made: its kind, name, ID, location, and role. The last column shows what happens to it when the claim is deleted. |

The column **After the claim is deleted** shows **Kept** or **Deleted**. The line above the table names the deletion policy of the claim. With **Delete the resources**, each resource shows **Deleted**. With **Keep the resources**, each resource shows what its plugin reports, because a plugin can delete a resource in both cases, for example a route. After the delete of the claim starts, a request keeps the policy that it had then. A request that the claim does not control shows "Not controlled by this claim" in this column, for example a request that an older claim of the same name made. [Keep or delete the resources](#keep-or-delete-the-resources) shows how to choose the policy.

A value of the wrong type in the status of a request shows as an error on its row, for example "Summary item 2 is not a valid summary item." The other rows still show.

### Grants

A Grant asks the cloud identity of the cluster for one permission. The **Grants** box shows each Grant of the request with its template, its statement as JSON, and the conditions `Applied`, `MatchesTemplate`, and `Ready` with their reasons. The section **Cloud identity** shows every Grant of the cluster, with the request that asked for each.

A line under the name of a Grant says what its conditions mean:

| Line | What it means | What to do |
| --- | --- | --- |
| None | `Ready` is `True`: the identity has the permission. | Nothing. |
| **Refused. The conditions say why.** | `Applied` is `False`, for example with the reason `OverCeiling`, `OverSizeLimit`, or `Rejected`. The identity does not have the permission. | Read the message of `Applied`. For `OverCeiling` or `OverSizeLimit`, pick an option that asks for less, or ask the admin of the identity for a larger ceiling. For `Rejected`, the message says what the cloud refused. |
| **Applied with the old template.** | `MatchesTemplate` is `False` with the reason `TemplateChanged`. The identity still has the permission of the old template. | Wait. The controller of the option writes the Grant again from the new template. |
| **Not applied yet.** | `Ready` has no value yet. | Wait for the identity to apply the Grant. |

While a Grant of the request has `Ready` `False`, the request shows `Ready` `False` with the reason `GrantRejected`, and its card shows **Not ready (GrantRejected)**. This includes a Grant that is applied with the old template. Open the section: the **Grants** box of its **Overview** tab shows which Grant and why.

### Existing contracts

An option that connects a contract that you made makes no resources. The options **Existing secondary storage**, **Existing bucket**, and **Existing databases** show the box **Connected** in place of **Resources**. It lists the kind and the name of each contract that the selection names, for example `SecondaryStorageConfig` `my-storage`, and its `Ready`. A contract that is not in the namespace of the cluster shows **Not in the namespace**.

### Built-in storage

The options **New Elasticsearch cluster** and **New PostgreSQL server** of **Secondary storage** make an `ElasticsearchCluster` or a `DatabaseServer`. The detail of the section shows that object in a box of its own, with what its status holds:

- For an `ElasticsearchCluster`: the version, the snapshot repository, the volumes with their capacity, and the conditions.
- For a `DatabaseServer`: the version, the start of the archive, the archive boundary, the recovery request with its target and start, the volumes, and the conditions.

A field that the status does not hold shows **Not reported yet**. The box shows no health, node, disk, or connection numbers. The pods of the storage show in the **Pods** table, and their CPU and memory on the **Metrics** tab.

### The Settings tab of a section

The **Settings** tab of a section lists the parameters of the selection, with their values and where each value comes from. On a cluster that a claim made, select **Edit settings** to open the same editor as the section on the **Claim** tab. You can change the parameters that the claim may set. The option is locked: it stays as you chose it when you made the cluster. [Change the claim in the Claim tab](change-a-cluster.md#change-the-claim-in-the-claim-tab) shows how to write the change.

### The Logs tab of a section

The **Logs** tab of a section shows the lines of its controller about its request, in the group **Controller** of the list **Pod**. For example, when the request of a built-in option becomes ready, the controller writes the line `Set Ready of PluginRequest`. Select the arrow of the line to see its other keys, for example `status` `True` and `reason` `Healthy`. Each plugin writes its own lines, so the lines of another option can differ.

### When the page cannot read a list

While the page cannot read a list, or its list is out of date, for example after the page lost its connection, the boxes that need it show "Not known: the page could not read" and the kind. For example, without the Grants, the **Grants** box shows "Not known: the page could not read Grant." and the conditions of each Grant show **Not known**. Without the claim, each resource shows **Not known** in place of **Kept** or **Deleted**. Without the requests, the card of a section shows **Request: Not known**, and its detail shows no state of the request. The page reads each list again by itself.

## Logs

The **Logs** tab follows the logs of the pods of the component. It shows the newest lines at the bottom and adds new lines as the pods write them.

The list **Pod** chooses the logs:

- **All pods** follows the pods of the component. The number after it counts the pods. It follows the first 10 pods of the list. With more pods, the tab says, for example, "Showing the logs of 10 of 12 pods. Pick a pod to see the others."
- The name of a pod follows only that pod.

Each log starts with its last 200 lines. The tab keeps the newest 1000 lines of all its pods together. A reload of the page starts again from the last 200 lines of each log.

The tab shows the log of the first container of each pod, as it runs now. It does not show the log of a container before its last restart. To read that log, use `kubectl logs --previous`.

The tab reads each line that is a JSON object as a structured line. It shows the time, the level, the logger, and the message in their own columns. A line with more keys has an arrow that opens the other keys. The tab shows any other line as the pod wrote it. The pod column shows when the lines come from more than one pod.

The times show as hours, minutes, and seconds in UTC, as the note **Time (UTC)** below the lines says. A time without a time zone, and a time that the tab cannot read, show as the line wrote them.

To read the lines:

1. Select **Pause** to stop the lines from moving. The tab counts the new lines, for example "3 new lines".
2. Select **Follow** to show the new lines again.
3. To show only the lines of one level, select it in **Level**. The list offers the levels of the lines, in lower case. A line without a level does not show while a level is selected.
4. To show only the lines that hold a text, type it in **Filter**. Upper case and lower case are the same.

A component without pods shows "This component has no pods, so it has no logs."

### When a log cannot be read

The tab names each pod whose log it cannot read, below the lines, for example:

| The tab shows | What it means | What to do |
| --- | --- | --- |
| `my-cluster-zeebe-0: The container zeebe of pod my-cluster-ns/my-cluster-zeebe-0 is not running` | The container has not started yet. | Wait until the pod is running, then reload the page. |
| `my-cluster-zeebe-0: The pod my-cluster-ns/my-cluster-zeebe-0 does not exist` | The pod was deleted, for example by a rollout. | Reload the page to follow the new pods. |
| `my-cluster-zeebe-0: The API server did not give the log of pod my-cluster-ns/my-cluster-zeebe-0` | Kubernetes refused the read. | Make sure that the API server and the node of the pod answer, then reload the page. |
| `my-cluster-zeebe-0: The log stream of pod my-cluster-zeebe-0 broke` | The read stopped after it started. | Reload the page. |
| `my-cluster-zeebe-0: The connection to the log stream broke` | The browser lost the connection to the fleet manager. | Reload the page. |

The tab does not connect again by itself. The lines that it already has stay. When the fleet manager cannot read the pods of the component, the tab shows `Could not read Pod:` and the error. Reload the page when the API server answers again.

## Events

The **Events** tab lists the Kubernetes events of the component, newest first. It includes the events of:

- The pods of the component.
- Its Services and its PersistentVolumeClaims.
- Its StatefulSet or Deployment, and the ReplicaSets of the Deployment.
- The `CamundaCluster`, or the `CamundaOptimize` on the page of Optimize, or the `PluginRequest` on the page of a section.

Each event shows its age, its reason, the object that it is about, and its message. A Warning event has the label **Warning**. An event that repeated shows the count, for example "(4 times)". With no event, the tab shows "No events".

The tab reads the events of the namespace of the cluster only. Kubernetes keeps an event for one hour by default, so an older event does not show.

When a read fails, the tab shows "The list can miss events, because a read failed." Below it, a line names each read that failed, for example `Could not read PersistentVolumeClaim:` and the error. The other events still show.

## Delete the cluster

Select **Delete** in the header of the cluster. The delete page opens at `/fleet/clusters/<namespace>/<name>/delete`. Only a cluster that a claim made has **Delete**, because the page deletes the claim. The page stays open while the cluster goes.

### Keep or delete the resources

The page asks what happens to the resources of the storage and access options when the cluster is deleted:

- **Keep the resources**: the resources stay after the cluster is gone.
- **Delete the resources**: the resources go with the cluster.

The choice starts at the deletion policy of the claim. A claim without a deletion policy starts at the policy that it recorded when it was made. Without that, the choice starts at **Delete the resources**. The label **Default** marks the policy that the claim recorded, or **Delete the resources** when it recorded none.

Some options always delete what they made. When the claim selects such an option, **Keep the resources** is off, and the option shows with **Always deleted**. While **Keep the resources** is still picked, the page shows no review. It names the registration of the option, for example `PluginRegistration "gateway" of section "access" cannot keep the resources. Choose Delete the resources.` Choose **Delete the resources** to delete the cluster.

The page checks this again before it sends the delete, and again after the wait for the deletion policy. If a registration stops supporting Keep before that check, the page sends no delete and shows the same message. If a registration stops supporting Keep after the page sent the delete, the operator refuses the delete, and the claim stays. The page then says what to do. Set the deletion policy of the claim to Delete. Or add `Orphan`, the policy of Keep, back to the deletion policies of the registration.

### The resources of each option

The page lists each resource that an option made, under the name of its section. Each row shows the kind, the name, the ID, and what happens to the resource: **Kept** or **Deleted**. Under **Keep the resources**, an option can still delete some of its resources, for example a route. An option that made no resources says "This option made no resources."

### What always goes

The delete always removes these objects, whatever you choose:

- The `CamundaCluster`, when the claim made it.
- The `CamundaOptimize` `<name>-optimize` and the `BackupSchedule` `<name>-backups`, when the claim made them.
- The claim.

A `CamundaCluster`, a `CamundaOptimize`, or a `BackupSchedule` of the cluster that the claim did not make stays. The page lists it under **Kept**, with "Kept: the claim did not make it". When the `CamundaCluster` stays, the page ends with "The claim my-cluster is deleted. The cluster my-cluster stays: the claim did not make it."

By default, the broker volumes go with the `CamundaCluster`, and the page says "Volumes deleted" next to it. The choice between keeping and deleting the resources does not change this. To keep the volumes for a later cluster of the same name, set `whenDeleted: Retain` in the claim. Apply this change to the claim before you open the delete page. In Git mode, wait until GitOps applied it:

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

The page then says "Volumes kept for a cluster of the same name" next to the `CamundaCluster`. It says this, or "Volumes deleted", only after the operator applied the current spec of the `CamundaCluster`. Until then, the volumes keep the earlier setting of the cluster. If that setting is Delete, a delete now removes the volumes. In that time, the page says one of these:

- "Volumes not known: the operator has not applied the current spec of the cluster yet". Wait until the note changes before you delete the cluster.
- "Volumes not known: the Ready condition of the cluster reports a problem that can block its spec". Correct the cause that the `Ready` condition of the `CamundaCluster` names, for example a missing `CamundaRelease` or preset.
- "Volumes not known: the cluster takes its volume policy from its preset, and the page cannot tell if the operator applied it". With a preset, the page knows the policy only when the claim sets it. Set `whenDeleted: Retain` in the claim, as above.
- "Volumes not known: the page could not read what decides them". A read of the `CamundaCluster`, its preset, or its release failed.

### Backups

The backups of the cluster stay, and their artifacts stay in the bucket. When the backup storage option deletes its bucket under your choice, the backups have nowhere to stay. The page then warns "You cannot restore my-cluster after this: the backup bucket goes too."

### Confirm the delete

1. Type the name of the cluster.
2. Select **Delete the cluster**. In Git mode, the button is **Push the delete to Git**.

You cannot undo the delete. In direct mode, the fleet manager first writes your choice to the claim when the claim has another deletion policy. The options read the choice from there. The page shows "Waiting for the operator to read the deletion policy." Then the fleet manager deletes the claim. The page lists each object that goes, with **Waiting**, **Deleting**, or **Deleted**, until the claim is gone. Then it says "The cluster my-cluster is deleted."

The wait ends only when the operator reads the deletion policy. If the operator does not run, select **Stop waiting**. The fleet manager then sends no delete. The deletion policy that the fleet manager wrote stays in the claim. The page shows the deletion policy that the claim has, also after another change.

If the claim changed after the page read it, the delete fails, and the page shows the claim again for a new review. This also applies when someone else changes the claim during the wait. When no delete follows the write of your choice, the page shows the deletion policy that the claim has now. For example, it says "The cluster is not deleted. The claim my-cluster has the deletion policy Keep."

### When the page cannot read a list

The page reads the requests, the registrations, and the other objects of the delete. When a read fails, the page says, for example, "Could not read PluginRequest:" and the error. What happens to the resources then shows as **Not known**, and the page shows no review. It says "Delete is possible when the fleet manager can read PluginRequest." The page reads each list again by itself, and the review comes back when the fleet manager can read every list.

When the page cannot read the `CamundaCluster`, the `CamundaOptimize`, or the `BackupSchedule` objects, it lists each of them under **Not known**. This includes an object that the claim did not make.

While the delete runs, an object of a kind that the page cannot read shows **Not known** in place of **Waiting**, **Deleting**, or **Deleted**.

### Delete in Git mode

When your choice is the deletion policy of the claim, the delete is one commit, which removes the claim file. When your choice differs, the delete is two commits. The first commit sets the deletion policy. The page shows "Waiting for GitOps to apply the deletion policy." When the claim in the cluster carries that commit, the fleet manager pushes the second commit, which removes the claim file.

To stop the wait without a delete, select **Stop waiting**. The first commit stays in the repository. The page shows the choice again. Until GitOps applies the commit, the page says "The cluster is not deleted. The repository holds the commit that sets the deletion policy of my-cluster to Keep, and GitOps has not applied it yet. The claim my-cluster has the deletion policy Delete." After GitOps applies the commit, the page says "The cluster is not deleted. The claim my-cluster has the deletion policy Keep."

After the delete commit, the page shows "Waiting for GitOps to delete my-cluster" until the cluster starts to delete the claim. Your GitOps tool must prune: it deletes the claim only when it removes the objects of deleted files. [Waiting for GitOps](settings.md#waiting-for-gitops) shows how the fleet manager follows a push.

If the claim has no file in the repository, the review refuses the delete before the first commit, with either choice. For example, with an empty `git.path`, it shows `my-cluster-ns/camundaclusterclaims/my-cluster.yaml is not in the repository. GitOps does not manage CamundaClusterClaim my-cluster.` Commit the claim to the folder of `git.path`, then delete it again. Or delete the claim with `kubectl delete camundaclusterclaim my-cluster -n my-cluster-ns`.

## What the tabs need

The **Logs** and **Events** tabs need only the fleet manager.

The CPU and memory of the pods, on the **Overview** and **Metrics** tabs, come from metrics-server. Without it, the table of pods says "Usage is not available. The cluster has no metrics server, or the app could not read the usage." [Fleet manager settings](settings.md) shows if the fleet manager finds metrics-server.

The charts of the **Metrics** tab come from Prometheus at the address of `prometheus.url`. [Metrics of the charts](settings.md#metrics-of-the-charts) lists the metrics, and the messages of the tab when Prometheus does not answer.
