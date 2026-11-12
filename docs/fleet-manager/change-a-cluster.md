# Change a cluster

The cluster page of the fleet manager changes a cluster through its `CamundaClusterClaim`. Each change edits the claim. The fleet manager does not write the `CamundaCluster` itself. A cluster that no claim made shows **Not managed** and has no editors.

## Change the settings of a component

Open the cluster page, then select the card of a component on the **Overview** tab. The **Settings** tab of the component shows each field with its value and where the value comes from. Below the table, the button **Edit settings** opens one editor for each field.

A component also offers the fields that neither the preset nor the claim sets, for example `replicas` of Tasklist. The type of each field decides its editor:

| Field | Editor |
| --- | --- |
| A string, such as `storageClassName` of the brokers | A text input. |
| A number, such as `replicas` | A number input. A value outside the range of the field shows an error. |
| `true` or `false`, such as `enabled` | A checkbox. |
| A field with a fixed list of values, such as `mode` | A list of the values, and **Not set**. |
| `resources` | The CPU and the memory, each as a request and a limit with a unit. |
| `storageSize` | An amount and a unit. |
| `extraEnv` | One row for each variable: a name, then a value or **From a Secret**. |
| `podLabels`, `podAnnotations`, and the other maps | One row for each key and value. |
| `scheduling.tolerations` | One row for each toleration. |
| `scheduling.nodeAffinity`, `scheduling.podAffinity`, `extraEnvFrom`, and the other lists | YAML, checked against the schema of the field. |

**From a Secret** shows the names of the Secrets in the namespace and the names of their keys. The fleet manager never shows the value of a Secret.

The preset and the release can add their own `extraEnvFrom` sources. In the pods, these sources come before the sources of the claim. For this reason, the editor of `extraEnvFrom` shows only the sources of the claim. It also shows how many sources the preset and the release add.

## What a change writes

A change of a field writes the field at its path in `spec.cluster` of the claim. The block of the component comes first in the path, for example `spec.cluster.zeebe.replicas` for the replicas of the brokers. When you set `replicas` of Tasklist to 2, the claim gets this value:

```yaml
apiVersion: fleet.camunda.io/v1alpha1
kind: CamundaClusterClaim
metadata:
  name: my-cluster
  namespace: my-cluster-ns
spec:
  cluster:
    tasklist:
      replicas: 2
  # ... the rest of your claim
```

The settings of Optimize go into `spec.attachments.optimize` of the claim.

The preset can set a block such as `scheduling` as one value. A claim value of the block then replaces the whole block of the preset. For this reason, a change of one part writes the whole block into the claim, with the other parts as the preset sets them. For example, the preset sets `scheduling.nodeAffinity`, and you add a toleration. The claim then gets `scheduling` with the node affinity of the preset and your toleration.

When you empty a field, for example when you remove the last label, the claim no longer sets the field. The value of the preset then applies again.

## Change the claim in the Claim tab

The **Claim** tab shows the choices for the whole cluster in panels. Each panel that you can change has an **Edit** button. One panel is open at a time. **Done** closes the panel and keeps its changes as unsaved changes.

**Basics** shows the name, the namespace, and the environment as **Set at creation**. You cannot change them. **Suspend the cluster** sets `spec.suspend` of the claim. Then every workload goes to zero, and the volumes stay.

**Release and preset** has three lists:

- **Release** lists the releases that the environment allows, newest first. The default release of the environment has the word **Default**. The current release stays in the list also when the environment no longer allows it.
- **Preset** lists the presets that the environment allows. The review of a new preset shows each field of the cluster that changes.
- **Platform config** lists the `CamundaPlatformConfig` objects. The first choice removes `platformConfigRef` from the claim, so that the platform config of the environment applies. Its name is **Default of the environment** when the environment names one.

The fleet manager checks a new release against the version that the cluster runs:

| The list shows | What it means |
| --- | --- |
| Skips 8.*n*. Move to an 8.*n* release first. | The release is one minor too far. Move to a release of the minor between them first. |
| downgrade | The release has a lower version. The review asks you to confirm the downgrade with **Downgrade**, or **Push downgrade** in Git mode. |
| The version checks run when the cluster resumes. | The cluster is suspended, so it reports no version. No release shows as a downgrade or a skip. |
| The version checks run when the cluster reports its version. | The cluster reports no version yet. |

A downgrade writes the release and `allowVersionDowngrade` with the target version:

```yaml
apiVersion: fleet.camunda.io/v1alpha1
kind: CamundaClusterClaim
metadata:
  name: my-cluster
  namespace: my-cluster-ns
spec:
  releaseRef: camunda-8-9-1
  allowVersionDowngrade: "8.9.1"
  # ... the rest of your claim
```

The operator removes the annotation `camunda.io/allow-version-downgrade` from the `CamundaCluster` when the downgrade is complete. After that, the next save also removes `allowVersionDowngrade` from the claim. The review shows this removal as a line of the change. With no other unsaved change, the panel shows **Clear**, which removes the field and opens the review. The fleet manager never writes the claim when you only open the page.

Each storage and access option has its own panel. The option shows **Chosen at creation**, because you cannot change it here. The panel has an editor for each parameter that the environment lets the claim change. A parameter with a list of choices shows the names of the choices. The other parameters show their value and the reason. A change writes only the parameter, in `spec.plugins` of the claim.

**Attachments** has two parts:

- **Attach Optimize** adds `spec.attachments.optimize` with the first `ManagementAuthConfig` by name. Then you can pick another **ManagementAuthConfig** and enter the **External URL**. The URL shows **Filled by Access** when an access option gives it. **Open Optimize** opens Optimize on the **Overview** tab. Its **Settings** tab holds the other settings of Optimize. Optimize needs Elasticsearch as secondary storage and a `ManagementAuthConfig`. Without them, the checkbox is disabled and says which one is missing.
- **Back up on a schedule** adds `spec.attachments.backupSchedule` with the cron `0 2 * * *`. Then you can change the **Schedule (cron, UTC)**, **Completed backups to keep**, and **Failed backups to keep**. **Completed backups to keep** is 1 or more, and **Failed backups to keep** is 0 or more. An empty number uses the default of the schedule. Without a backup storage option, the checkbox is disabled.

**Change schedule** on the **Backups** tab opens the **Attachments** panel.

**Deletion policy** decides if the resources of the storage and access options stay when the claim is deleted. **Keep the resources** writes `Orphan`, and **Delete the resources** writes `Delete`. The policy that the claim gets without a choice has the word **Default**. When an option always deletes what it made, **Keep the resources** is disabled. The panel shows that option with **Always deleted**.

## Save or discard the changes

The bar at the bottom of the page shows the number of unsaved changes, with **Discard** and **Review**:

1. Select **Review** to see the change of the claim.
2. Select **Save** to write the claim. In Git mode, the button is **Push to Git**.

**Discard** removes every unsaved change. The editors then show the live values again.

A value with an error does not count as a change, and the editor shows the error. While an editor shows an error, **Review** is disabled, and the bar names the field and the error. Correct the value, or close the editor, to enable **Review** again. Kubernetes checks some rules of a field only when you review the change, for example the highest number of Optimize importers. When the change fails such a rule, the review shows the message at the field.

## The state of a field

Below each editor, the page says where the value comes from, for example **From preset** or **Changed in the claim**. A field that the claim sets has the link **Reset to preset**. The link removes the value from the claim, so that the value of the preset applies again. The reset is an unsaved change, as each other change is.

When the claim sets a block such as `scheduling` as one value, each field of the block has the link **Reset scheduling to preset**. This link removes the whole block from the claim. Every `scheduling` field of the claim then goes back to the preset, also `scheduling.nodeAffinity`.

A field that you cannot change shows its value and the reason, with no editor:

| Reason | What sets the field |
| --- | --- |
| Filled by *section* | A selection of that section, for example **Filled by Volume encryption** for `storageClassName` of the brokers. Change the selection of the section in the claim. |
| Set by the release | The `CamundaRelease` of the claim, such as `version`. Change the release of the claim. |
| Fixed by the claim controller | The operator, such as `serviceAccount.name`. |

## Embedded apps

Operate, Tasklist, and Admin run inside the gateway unless their `mode` is `Standalone`. When the gateway `mode` is `Embedded`, the gateway and these apps run inside the brokers. The **Settings** tab of the host shows the fields of each app that runs inside it, below the fields of the host. A note above them says that these fields apply only when the app runs standalone. The note also names the two exceptions, `extraEnv` and `extraEnvFrom`.

A change of an embedded app writes to the block of the app, for example `spec.cluster.tasklist` for Tasklist. While the app runs inside its host, these values have no effect, except `extraEnv` and `extraEnvFrom`. When the `mode` of the app changes to `Standalone`, the app starts with them. You can prepare the settings of an app before it runs standalone.

The `extraEnv` and `extraEnvFrom` of an embedded app apply to its host. For example, an `extraEnv` variable of an embedded Tasklist goes to the pods of the gateway. When the gateway is embedded too, it goes to the brokers.

## Storage size

A volume cannot become smaller. When you enter a `storageSize` below the current size, the editor shows "Volumes cannot shrink. The operator keeps the current size." The claim gets the value, and the volumes keep their current size.
