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

## Save or discard the changes

The bar at the bottom of the page shows the number of unsaved changes, with **Discard** and **Review**:

1. Select **Review** to see the change of the claim.
2. Select **Save** to write the claim. In Git mode, the button is **Push to Git**.

**Discard** removes every unsaved change. The editors then show the live values again.

A value with an error does not count as a change, and the editor shows the error. Kubernetes checks some rules of a field only when you review the change, for example the highest number of Optimize importers. When the change fails such a rule, the review shows the message at the field.

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
