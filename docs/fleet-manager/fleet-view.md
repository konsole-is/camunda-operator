# The fleet view

The **Fleet** page of the fleet manager lists each Camunda cluster of this Kubernetes cluster, grouped by environment. Above the clusters, it lists the management planes that serve them. The button **New cluster** opens the create flow.

The page updates live. A new cluster, a change of status, or a new backup shows without a reload. The warnings of the pods update every 15 seconds.

## Needs attention

The panel **Needs attention** is at the top of the page, above the tabs. It shows the same list on every tab, and the filters do not apply to it. It lists, in this order:

| Entry | When it shows | Link |
| --- | --- | --- |
| Cluster | The `Ready` condition of the cluster, or of a claim without a cluster, is `False`. | The cluster page. A claim without a cluster has no link. |
| Backup | A backup has the phase `Failed`, and it finished in the last 24 hours. A backup without a completion time counts from the time it was created. | The **Backups** tab of its cluster. |
| Option | The `Accepted` or the `Ready` condition of a `PluginRegistration` is `False`. The entry shows the label of the option, or its name. | No link. |
| Management plane | The `Ready` condition of the plane is `False`. | No link. |

Each entry shows the reason and the message of the condition. A backup shows its failure message. When nothing needs attention, the panel says "Nothing needs attention." When the page cannot read one of these lists, the panel says "Some lists could not be read, so this list can miss entries."

## Environments and tabs

A namespace belongs to the `Environment` that its label `fleet.camunda.io/environment` names, as [Environment](../crds/environment.md) shows. The page has one tab for each `Environment`, in the order of their names. Each tab shows the display name of the environment, its color, and the number of its clusters. The tab **All** comes first and shows every cluster, with one table for each environment.

The color comes from `spec.color` of the `Environment`:

```yaml
apiVersion: fleet.camunda.io/v1alpha1
kind: Environment
metadata:
  name: production
spec:
  displayName: Production
  color: "#2E7D32"
  # ... the rest of your environment
```

The page shows the color only when it is a hex color with three or six digits, such as `#2E7D32` or `#2E7`. An environment without such a color has a tab with no color.

The tab **No environment** holds the clusters in a namespace that belongs to no environment. The page has this tab only while such a cluster exists. A namespace without the label belongs to no environment. A namespace whose label names an `Environment` that does not exist also belongs to no environment. To move a cluster into an environment, give its namespace the label with the name of the `Environment`:

```bash
kubectl label --overwrite namespace team-orders fleet.camunda.io/environment=production
```

`--overwrite` replaces a label that names another environment.

When no `Environment` exists, the page has no tabs. It shows one table of every cluster.

## Management planes

The table **Management planes** lists the management planes of the selected tab:

- On **All**, and on a page without tabs, it lists every management plane.
- On the tab of an environment, it lists the planes that serve that environment. A plane serves the environments that its claim names, as [CamundaManagementClusterClaim](../crds/camundamanagementclusterclaim.md) shows.
- On **No environment**, it lists the planes that serve a namespace that belongs to no environment.

When the tab has no plane, the page says "No management plane serves this environment." When no plane exists at all, it says "No management plane yet." on **All**.

## Clusters

Each table of clusters has these columns:

| Column | What it shows |
| --- | --- |
| Cluster | The name, which opens the cluster page, and the namespace below it. A cluster that no claim made shows **Not managed**. |
| Status | The `Ready` condition of the cluster, such as `Ready` or `Not ready · Degraded`. A ready cluster shows its reason too when the reason is not `Healthy`, such as `Ready · Suspended`. The message of the condition shows when you hold the pointer on the status. |
| Version | The Camunda version that the cluster runs, from `status.management.version`, and the preset below it. A suspended cluster shows `-`. |
| Components | One dot for each component of the cluster, as the next section shows. |
| Last backup | The phase and the start time of the newest backup of the cluster, or **No backups**. |
| Warnings | One chip for each warning of the cluster, errors first, as [Warnings](#warnings) shows. A cluster without a warning shows `-`. |

A cluster shows **Not managed** when you applied the `CamundaCluster` with `kubectl` or a GitOps tool, and not through a `CamundaClusterClaim`. You can read such a cluster in the fleet manager, but you cannot change it there.

A claim that has no `CamundaCluster` yet shows as a row too, with the `Ready` condition of the claim. For example, a claim that the environment does not allow shows `Not ready · NotAllowed`. The row has no link and no component dots until the cluster exists. The message of the condition says what to change, as [CamundaClusterClaim](../crds/camundaclusterclaim.md) shows.

### Component dots

The dots show the components of the cluster in this order:

1. The Camunda processes that run in their own pods: Brokers, Gateway, Operate, Tasklist, Admin, and Connectors. A process that runs embedded in the gateway or in the brokers has no dot of its own.
2. Optimize, when a `CamundaOptimize` names the cluster.
3. For a cluster of a claim, one dot for each section in which the claim selects an option. An example is `Secondary storage (New Elasticsearch cluster)`.
4. For a cluster that is not managed, one dot for the secondary storage that `spec.storageRef` names, such as `Secondary storage (Elasticsearch)`.

The color of a dot is the `Ready` state of its component. Green is ready, red is not ready, and yellow is unknown. Disabled Connectors, and Optimize for a cluster without a `CamundaOptimize`, show a grey dot. A component that has no status yet also shows a grey dot. Hold the pointer on the dots, or move the keyboard focus to them, to see a list of the components. The list gives the state of each component and the message of its condition.

## Warnings

A warning is a fact that needs your attention, also while the cluster is ready. The column **Warnings** shows one chip for each warning. Hold the pointer on a chip, or move the keyboard focus to it, to see the details. The details say what happened and, when the fact has a time, how long ago it happened.

| Warning | When it shows | Severity | Source |
| --- | --- | --- | --- |
| Out of memory | A container or an init container of the cluster stopped with the reason `OOMKilled` in the last hour. | Error | The [state of the container](https://kubernetes.io/docs/concepts/workloads/pods/pod-lifecycle/#container-states) in the pod status |
| Restarts | A container or an init container restarted more than 3 times, and its last restart was in the last hour. | Warning | The restart count and the last state of the container in the pod status |
| Near memory limit, near CPU limit | A pod uses 75 % or more of its memory or CPU limit. At 90 % or more the chip is an error. A pod without a limit has no such warning. | Warning or Error | metrics-server, through the [resource metrics pipeline](https://kubernetes.io/docs/tasks/debug/debug-cluster/resource-metrics-pipeline/) |
| Backpressure | The brokers dropped more than 1 % of the requests in the last 5 minutes. | Warning | Prometheus |
| Backup failed | The newest backup of the cluster has the phase `Failed`. The chip stays until a newer backup of the cluster exists. | Warning | The backup |
| An item of a request summary | An option reports an item with the severity `Warning` or `Error` in the summary of its request. The chip shows the label of the item. | The severity of the item | The `PluginRequest` |
| `<section>` not ready | The `Ready` condition of the request of a section is `False`. The tooltip shows its reason and its message. | Error | The `PluginRequest` |

The restart count of a container counts every restart since the pod started. So the Restarts warning uses the time of the last restart, and it goes away one hour after the last restart.

The backpressure share is `zeebe_dropped_request_count_total` divided by `zeebe_received_request_count_total` of the brokers of the cluster. Camunda documents both metrics in [Backpressure tuning](https://docs.camunda.io/docs/self-managed/components/orchestration-cluster/zeebe/operations/backpressure/#backpressure-tuning). Prometheus must keep the labels `namespace` and `pod` on these metrics, as [Metrics of the charts](settings.md#metrics-of-the-charts) shows. The fleet manager reads the backpressure share for each Camunda 8.9 patch release. A cluster that reports another version, or no version, has no Backpressure warning.

The near limit warnings need metrics-server. The Backpressure warning needs Prometheus at the address that **Prometheus URL** (`prometheus.url`) of [Fleet manager settings](settings.md) gives. When one of them does not answer, its warnings do not show, and the page says so once, above the clusters:

- `metrics-server is not found, so the near memory and CPU limit warnings do not show.`
- `Prometheus is not set or does not answer, so the backpressure warning does not show.`

The other warnings still show. When the fleet manager cannot read the pods, the page says `The warnings of the pods do not show, because the fleet manager could not read them.` The warnings of the backups and of the requests still show then.

## Filters

Three filters apply to every table of clusters on the tab:

- **Search clusters** keeps the clusters whose name holds the text. Upper case and lower case are the same.
- **Namespace** keeps the clusters of one namespace.
- **Status** with **Not ready** keeps the clusters whose `Ready` condition is not `True`, also a cluster that has no status yet.
- **Status** with **With warnings** keeps the clusters that have at least one warning.

The filters do not apply to the management planes. When no cluster passes the filters, the page says "No cluster matches the search and the filters."

## Waiting for GitOps

In Git mode, a new cluster goes to the Git repository first, as [Fleet manager settings](settings.md#git-mode) shows. Until your GitOps tool applies the claim, the page shows it in the table **Waiting for GitOps**. This table is above the tables of clusters, on every tab. The row shows the file in the repository and the time since the push. The row goes away when the claim exists in the cluster, and the claim then shows in the table of its environment.

Only the browser tab that pushed the claim shows its row. The browser tab keeps the list of its pushes, and a reload of the page clears it. The fleet manager reads the clusters from the Kubernetes API. So another browser tab or another user does not see a claim that is only in Git.
