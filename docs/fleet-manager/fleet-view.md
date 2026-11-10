# The fleet view

The **Fleet** page of the fleet manager lists each Camunda cluster of this Kubernetes cluster, grouped by environment. Above the clusters, it lists the management planes that serve them. The button **New cluster** opens the create flow.

The page updates live. A new cluster, a change of status, or a new backup shows without a reload.

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
kubectl label namespace team-orders fleet.camunda.io/environment=production
```

When no `Environment` exists, the page has no tabs. It shows one table of every cluster.

## Management planes

The table **Management planes** lists the management planes of the selected tab:

- On **All**, and on a page without tabs, it lists every management plane.
- On the tab of an environment, it lists the planes that serve that environment. A plane serves the environments that its claim names, as [CamundaManagementClusterClaim](../crds/camundamanagementclusterclaim.md) shows.
- On **No environment**, it lists the planes that serve a namespace without the label.

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

A cluster shows **Not managed** when you applied the `CamundaCluster` with `kubectl` or a GitOps tool, and not through a `CamundaClusterClaim`. You can read such a cluster in the fleet manager, but you cannot change it there.

A claim that has no `CamundaCluster` yet shows as a row too, with the `Ready` condition of the claim. For example, a claim that the environment does not allow shows `Not ready · NotAllowed`. The row has no link and no component dots until the cluster exists. The message of the condition says what to change, as [CamundaClusterClaim](../crds/camundaclusterclaim.md) shows.

### Component dots

The dots show the components of the cluster in this order:

1. The Camunda processes that run in their own pods: Brokers, Gateway, Operate, Tasklist, Admin, and Connectors. A process that runs embedded in the gateway or in the brokers has no dot of its own.
2. Optimize, when a `CamundaOptimize` names the cluster.
3. For a cluster of a claim, one dot for each section in which the claim selects an option. An example is `Secondary storage (New Elasticsearch cluster)`.
4. For a cluster that is not managed, one dot for the secondary storage that `spec.storageRef` names, such as `Secondary storage (Elasticsearch)`.

The color of a dot is the `Ready` state of its component. Green is ready, red is not ready, and yellow is unknown. Disabled Connectors, and Optimize for a cluster without a `CamundaOptimize`, show a grey dot. A component that has no status yet also shows a grey dot. Hold the pointer on the dots, or move the keyboard focus to them, to see a list of the components. The list gives the state of each component and the message of its condition.

## Filters

Three filters apply to every table of clusters on the tab:

- **Search clusters** keeps the clusters whose name holds the text. Upper case and lower case are the same.
- **Namespace** keeps the clusters of one namespace.
- **Status** with **Not ready** keeps the clusters whose `Ready` condition is not `True`, also a cluster that has no status yet.

The filters do not apply to the management planes. When no cluster passes the filters, the page says "No cluster matches the search and the filters."

## Waiting for GitOps

In Git mode, a new cluster goes to the Git repository first, as [Fleet manager settings](settings.md#git-mode) shows. Until your GitOps tool applies the claim, the page shows it in the table **Waiting for GitOps**. This table is above the tables of clusters, on every tab. The row shows the file in the repository and the time since the push. The row goes away when the claim exists in the cluster, and the claim then shows in the table of its environment.

Only the browser tab that pushed the claim shows its row. The browser tab keeps the list of its pushes, and a reload of the page clears it. The fleet manager reads the clusters from the Kubernetes API. So another browser tab or another user does not see a claim that is only in Git.
