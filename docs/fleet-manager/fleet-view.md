# The fleet view

The **Fleet** page lists each Camunda cluster of this Kubernetes cluster, grouped by environment. Above the clusters, it lists the management planes that serve them. **New cluster** opens the create form, as [Create a cluster](create-a-cluster.md) shows. The page updates live, without a reload.

When the fleet manager cannot read a list, the page shows `Could not read <kind>:` and the error. When its list is not current, the page shows `Not known: the fleet manager has no current list of <kind>.` The page then does not say that there is no cluster or no plane. The other pages of the fleet manager show the same lines.

## Needs attention

The panel **Needs attention** at the top of the page lists what is wrong now. It shows the same list on every tab, and the filters do not apply to it:

| Entry | When it shows |
| --- | --- |
| Cluster | The `Ready` condition of the cluster is `False`. The entry links to the cluster page. |
| Backup | A backup failed in the last 24 hours. The entry links to the **Backups** tab of its cluster. |
| Option | An option is not accepted or not ready. See [Registrations](registrations.md). |
| Management plane | The `Ready` condition of the plane is `False`. |

Each entry shows the reason and the message. When nothing is wrong, the panel says "Nothing needs attention."

## Environments and tabs

The page has one tab for each environment, and the tab **All** with every cluster. A namespace belongs to the environment that its label `fleet.camunda.io/environment` names. A tab takes its color from `spec.color` of the `Environment`, a hex color such as `#2E7D32`.

The tab **No environment** holds the clusters whose namespace belongs to no environment. To move a cluster into an environment, label its namespace:

```bash
kubectl label --overwrite namespace team-orders fleet.camunda.io/environment=production
```

When no `Environment` exists, the page has no tabs. It shows one table of every cluster.

## Management planes

The table **Management planes** lists the planes that serve the environment of the tab. On **All**, it lists every plane. [Management planes](management-planes.md) shows how to create and change a plane.

## Clusters

Each table of clusters has these columns:

| Column | What it shows |
| --- | --- |
| Cluster | The name, which opens [the cluster page](cluster-page.md), and the namespace. A cluster that you applied without the fleet manager shows **Not managed**. |
| Status | The `Ready` condition, such as `Ready` or `Not ready · Degraded`. Hold the pointer on it to see the message. |
| Version | The Camunda version that the cluster runs, and its preset. |
| Components | One dot for each part of the cluster, as the next section shows. |
| Last backup | The phase and the start time of the newest backup, or **No backups**. |
| Warnings | One chip for each warning, errors first, as [Warnings](#warnings) shows. |

A new cluster shows before the operator has built it. Its row shows the `Ready` condition of the claim, for example `Not ready · NotAllowed` when the environment does not allow a choice. The message says what to change.

### Component dots

The dots show the Camunda processes that run in their own pods, Optimize, and each storage and access option of the cluster. Green is ready, red is not ready, yellow is unknown, and grey is off or has no status yet. Hold the pointer on the dots to see the state and the message of each part.

## Warnings

A warning is a fact that needs your attention, also while the cluster is ready. Hold the pointer on a chip to see the details.

| Warning | When it shows |
| --- | --- |
| Out of memory | A container stopped with `OOMKilled` in the last hour. |
| Restarts | A container restarted more than 3 times, and its last restart was in the last hour. |
| Near memory limit, near CPU limit | A pod uses 75 % or more of its limit. At 90 % the chip is an error. |
| Backpressure | The brokers dropped more than 1 % of the requests in the last 5 minutes. |
| Backup failed | The newest backup of the cluster failed. |
| `<choice>` not ready, such as `Backup storage not ready` | The storage or access option of that choice is not ready. |
| The label that an option gives | A storage or access option reports a warning or an error. |

The near limit warnings need metrics-server. The Backpressure warning needs Prometheus at **Prometheus URL** of the [settings](settings.md), and a cluster on Camunda 8.9. When one of them does not answer, the page says once which warnings do not show.

## Filters

**Search clusters**, **Namespace**, and **Status** filter the tables of clusters on the tab. **Status** has two values: **Not ready**, and **With warnings**. The filters do not apply to the management planes.

## Waiting for GitOps

In Git mode, a new cluster goes to the Git repository first, as [Git mode](settings.md#git-mode) shows. Until your GitOps tool applies it, the table **Waiting for GitOps** shows the file and the time since the push. Only the browser tab that pushed the cluster shows this row, and a reload clears it.
