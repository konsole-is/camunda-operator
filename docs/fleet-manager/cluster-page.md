# The cluster page

The cluster page shows one Camunda cluster: its processes and pods, their logs and events, its backups, its addresses, and its claim. To open it, select the name of the cluster in [the fleet view](fleet-view.md). The address of the page is `/fleet/clusters/<namespace>/<name>`, for example `/fleet/clusters/my-cluster-ns/my-cluster`.

## The tabs of the cluster

| Tab | What it shows |
| --- | --- |
| **Overview** | The **Topology** of the processes and their pods, a card for each component, and the **Conditions** of the cluster, of Optimize, and of the claim. |
| **Backups** | The backup storage, the schedules, the backups, and the restores, as [Back up and restore a cluster](backups.md) shows. |
| **Endpoints** | The address of each app: its public URL, its Service, its protocol, and the process that serves it. |
| **Claim** | The choices of the claim for the whole cluster, as [Change the claim in the Claim tab](change-a-cluster.md#change-the-claim-in-the-claim-tab) shows. |

A cluster that a claim made also has the button **Change topology** in the **Topology** panel, as [Change the topology](change-a-cluster.md#change-the-topology) shows.

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
- The `CamundaCluster`, or the `CamundaOptimize` on the page of Optimize.

Each event shows its age, its reason, the object that it is about, and its message. A Warning event has the label **Warning**. An event that repeated shows the count, for example "(4 times)". With no event, the tab shows "No events".

The tab reads the events of the namespace of the cluster only. Kubernetes keeps an event for one hour by default, so an older event does not show.

When a read fails, the tab shows "The list can miss events, because a read failed." Below it, a line names each read that failed, for example `Could not read PersistentVolumeClaim:` and the error. The other events still show.

## What the tabs need

The **Logs** and **Events** tabs need only the fleet manager.

The CPU and memory of the pods, on the **Overview** and **Metrics** tabs, come from metrics-server. Without it, the table of pods says "Usage is not available. The cluster has no metrics server, or the app could not read the usage." [Fleet manager settings](settings.md) shows if the fleet manager finds metrics-server.

The charts of the **Metrics** tab come from Prometheus at the address of `prometheus.url`. [Metrics of the charts](settings.md#metrics-of-the-charts) lists the metrics, and the messages of the tab when Prometheus does not answer.
