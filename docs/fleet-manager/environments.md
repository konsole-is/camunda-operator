# Environments

An environment holds the policy for the claims in its namespaces. A claim asks for a Camunda cluster or a management plane. The policy names the platform config, the presets, and the releases that a claim can use. It also sets the deletion default and the rules for a new namespace. The page **Setup > Environments** of the fleet manager lists the environments and edits each one. The [Environment](../crds/environment.md) reference lists the fields.

The list shows each environment with its color, the number of its namespaces, its clusters, its management planes, and its status. Select an environment to open its editor. Select **New environment** to create one.

## Membership

A namespace belongs to the environment that its label `fleet.camunda.io/environment` names. A namespace belongs to one environment at most.

```yaml
apiVersion: v1
kind: Namespace
metadata:
  name: my-cluster-ns
  labels:
    fleet.camunda.io/environment: production
```

In the editor, the section **Namespaces** lists the namespaces of the environment and their claims. To add a namespace, pick it in **Add a namespace** and select **Add**. The list offers only namespaces without an environment label. The review shows the label change, and **Add the namespace** writes it. The review also names each claim of the namespace that names no release or no platform config. Such a claim moves to the default of the environment.

The label goes straight to the cluster, also in Git mode. It is not in the Git repository, and the review says so. A new environment has no namespaces yet. Save it first, then add namespaces.

When an environment exists, the operator refuses a new claim in a namespace without the label. The list of environments then shows a banner that names each namespace that holds clusters and has no environment. It also names each namespace whose label names an environment that does not exist.

| Reason in the banner | Meaning | What to do |
| --- | --- | --- |
| `NoEnvironment` | The namespace has no label `fleet.camunda.io/environment`. | Add the namespace to an environment. |
| `UnknownEnvironment` | The label names an environment that does not exist. | Create that environment, or change the label. |

## Platform config, presets, and releases

The sections **Platform config**, **Presets**, and **Releases** of the editor set these fields:

```yaml
apiVersion: fleet.camunda.io/v1alpha1
kind: Environment
metadata:
  name: production
spec:
  platformConfigRef: shared
  presets:
    camundaCluster:
      - small
      - large
  releases:
    allowed:
      - camunda-8-9-22
      - camunda-8-9-23
    default: camunda-8-9-23
  # ... the rest of your environment
```

A claim can use only the presets and the releases that the environment allows. The default release is one of the allowed releases. The page refuses a save with another default, and it shows `The default release must be one of the allowed releases.`

A claim that names no release uses the default release. A claim that names no platform config uses the platform config of the environment. These two defaults are not part of the recorded policy of a claim (see [Changes apply to new claims only](#changes-apply-to-new-claims-only)). So a change of either default moves such a claim to the new value.

A list can hold a name that no object has. The editor shows that name checked, with **Not found**. Clear the box to remove the name.

## Namespace creation

In the section **Namespaces**, **The create flow can create a namespace** lets the create flow make a new namespace in the environment. **Fixed labels of a new namespace** are labels that each new namespace gets. **Required label keys** are keys, for example `team`. The create flow asks for a value of each key.

```yaml
apiVersion: fleet.camunda.io/v1alpha1
kind: Environment
metadata:
  name: production
spec:
  namespaceCreation:
    allowed: true
    labels:
      team: payments
    requiredLabelKeys:
      - cost-center
  # ... the rest of your environment
```

The create flow sets the label `fleet.camunda.io/environment` itself. Neither list can hold it, and the editor shows `The fleet manager sets fleet.camunda.io/environment itself.` when you type it. An environment can have 16 required label keys at most.

## When a claim is deleted

The section **When a claim is deleted** sets the default for new claims. **Delete the resources of the claim** is `Delete`, and **Keep the resources of the claim** is `Orphan`. A claim that sets its own policy keeps it.

```yaml
apiVersion: fleet.camunda.io/v1alpha1
kind: Environment
metadata:
  name: production
spec:
  deletionPolicy: Orphan
  # ... the rest of your environment
```

## Changes apply to new claims only

A claim records the policy of its environment when it is created. A later change of the environment does not change that recorded policy. While the editor has changes, a note above **Review** counts the claims that keep the policy they recorded:

> Changes apply to new claims only. 2 clusters and 1 management plane keep the policy they recorded at creation.

The note also names each claim that the change touches:

| Note | Meaning |
| --- | --- |
| `orders in team-a uses release 8.9.22, which this change no longer allows.` | The cluster keeps running. A later change of its release must pick an allowed release. The same note exists for a cluster preset. |
| `orders in team-a names no release and moves to 8.9.23.` | The claim follows the default release, so its cluster moves to the new release. Its brokers restart one at a time. |
| `orders in team-a names no platform config and moves to shared.` | The claim follows the default platform config. |
| `orders in team-a names no release, and this change removes the default release.` | The claim follows the default release, so its cluster then names no release. The same note exists for the platform config. A management plane without a platform config keeps running, and its claim reports `MissingInput` until it names one. |

**Review** shows the change to the environment before the save. A field manager is a tool that writes the object, such as `kubectl apply` or a GitOps tool. If another field manager set a field that the change leaves out, the field stays. To remove such a field, remove it with the tool that set it. The review names each one: `spec.sections.backup-storage stays, because another field manager set it.`

If the environment changed after you opened the page, the save writes nothing. It shows `The Environment production changed since you opened it.` Select **Discard** to load the current environment, then make your change again.

In Git mode, the save pushes a commit, and the page waits until GitOps applied it. The form stays locked while it waits.

## Delete an environment

The button **Delete the environment** shows only when no namespace carries the label of the environment, and the fleet manager can read the namespaces. Remove the label from each namespace first, for example with `kubectl label namespace my-cluster-ns fleet.camunda.io/environment-`. The page does not remove a label. Then type the name of the environment and select **Delete the environment**.

## Status

The editor shows the `Ready` condition of the environment. When a name in the spec has no object, the editor shows the message of the condition:

```yaml
status:
  conditions:
    - type: Ready
      status: "False"
      reason: InvalidReference
      message: "The spec names objects that do not exist: CamundaRelease camunda-8-9-9"
  namespaces: 2
```

| Type | Reason | Meaning | What to do |
| --- | --- | --- | --- |
| `Ready` | `Healthy` | Each object that the spec names exists. | Nothing. |
| `Ready` | `InvalidReference` | The spec names an object that does not exist. The message names each one. | Create the object, or clear its name in the editor. |

A new environment has no status until the operator counts its namespaces. Until then, the list of environments shows the number of namespaces with the label.
