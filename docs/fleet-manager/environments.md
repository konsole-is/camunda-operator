# Environments

An environment holds the policy for the claims in its namespaces. A claim asks for a Camunda cluster or a management plane. The policy names the platform config, the presets, and the releases that a claim can use. It sets the options that a claim can pick in each section, and the values of their parameters. It also sets the deletion default and the rules for a new namespace. The page **Setup > Environments** of the fleet manager lists the environments and edits each one. The [Environment](../crds/environment.md) reference lists the fields.

The list shows each environment with its color, the number of its namespaces, its clusters, its management planes, and its status. Select an environment to open its editor. Select **New environment** to create one.

If the fleet manager cannot read its settings, the editor shows `Changes are off: the fleet manager could not read its settings.` and accepts no change. Reload the page when the settings can be read again.

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

When an environment exists, the operator refuses each claim in a namespace without the label. A new cluster is not created there, and an existing claim is not ready and takes no change. The list of environments then shows a banner that names each namespace that holds clusters and has no environment. It also names each namespace whose label names an environment that does not exist.

| Reason in the banner | Meaning | What to do |
| --- | --- | --- |
| `NoEnvironment` | The namespace has no label `fleet.camunda.io/environment`. Its claims take no change. | Add the namespace to an environment. |
| `UnknownEnvironment` | The label names an environment that does not exist. Its claims take no change. | Create that environment, or change the label. |

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
    elasticsearchCluster:
      - es-small
    databaseServer:
      - pg-small
  releases:
    allowed:
      - camunda-8-9-22
      - camunda-8-9-23
    default: camunda-8-9-23
  # ... the rest of your environment
```

A claim can use only the presets and the releases that the environment allows. The Elasticsearch presets and the PostgreSQL presets are the presets of the built-in Elasticsearch and PostgreSQL options of a claim. An empty list allows none, and the page says so under the list. The default release is one of the allowed releases. The page refuses a save with another default, and it shows `The default release must be one of the allowed releases.`

A claim that names no release uses the default release. A claim that names no platform config uses the platform config of the environment. These two defaults are not part of the recorded policy of a claim (see [Changes apply to new claims only](#changes-apply-to-new-claims-only)). So a change of either default moves such a claim to the new value.

A list can hold a name that no object has. The editor shows that name checked, with **Not found**. Clear the box to remove the name.

## Sections and options

A section is one choice of a claim, for example **Backup storage** or **Access**. An option is a registration that serves the section, for example RustFS for backups. The section **Sections** of the editor has a row for each section, in one group for clusters and one for management planes. The section **Access** is in both groups, and both rows edit one policy.

```yaml
apiVersion: fleet.camunda.io/v1alpha1
kind: Environment
metadata:
  name: production
spec:
  sections:
    backup-storage:
      required: true
      allowed:
        - rustfs
      default: rustfs
  # ... the rest of your environment
```

Each row has three settings:

- **Allowed options** are the options that a claim of the environment can pick. A section without an allowed option offers only its empty option, for example **No backups**.
- **Default option** is the option that the create flow selects first. It must be one of the allowed options, and the page refuses a save with another default.
- **Required** makes a claim pick an allowed option. The create flow then does not offer the empty option.

The row offers only the options that the operator accepted. An option that is not ready shows its reason, for example `Not ready: AccessDenied on the key.` The create flow offers it only when it is ready. An option can need a value from another section, for example an encryption key for encrypted volumes. If that section has no default, the option shows `Needs an option of Encryption key. Set a default in Encryption key.` A claim that picks the option without an encryption key is refused with `MissingInput`. An allowed name that no accepted option has shows checked, with **Not offered**. Clear the box to remove the name.

## Settings of the options

The section **Settings of the options** has an entry for each option that a section allows. Each entry lists the parameters of the option. A field follows the type of the parameter: a number field shows its range, and a parameter with a fixed set of values is a select.

```yaml
apiVersion: fleet.camunda.io/v1alpha1
kind: Environment
metadata:
  name: production
spec:
  parameters:
    aws-connectors-role:
      managedPolicyArns:
        value:
          - arn:aws:iam::aws:policy/AmazonSQSReadOnlyAccess
      extraPolicyArns:
        claimEditable: true
        allowed:
          - name: sqs
            label: SQS access
            value: arn:aws:iam::aws:policy/AmazonSQSFullAccess
  # ... the rest of your environment
```

A value of the environment replaces the value of the option. While the field is empty, the page shows the value of the option under it, for example `From the registration: 3`. **A claim can change it** lets a claim of the environment set its own value. Without it, each claim gets the value of the environment, or of the option.

When **A claim can change it** is on, the parameter can get a list of choices. Select **Add a choice** and give each choice a name, a label, and a value. A claim stores the name, and the create flow shows the label. Only the request of the claim gets the value, for example the ARN of a policy. Each name and each label is used once. A parameter can have 32 choices at most. When you turn **A claim can change it** off, the choices go too.

A host pattern of an access option shows the host that it gives the first claim of the environment, for example `orders in team-a gets orders.team-a.example.com`. The preset of a built-in Elasticsearch or PostgreSQL option offers only the presets that the section **Presets** allows.

## Template parameters

Some parameters are templates, for example the inline policy of the Connectors role. Such a parameter gets an editor with a chip for each placeholder. Select a chip to put the placeholder at the cursor.

| Placeholder | Value |
| --- | --- |
| `{{ .namespace }}` | The namespace of the claim. |
| `{{ .claim }}` | The name of the claim. |
| `{{ .request }}` | The name of the request of the claim for this option. |
| `{{ .serviceAccount }}` | The ServiceAccount that the request gives the permissions to. |
| `{{ .parameters.<name> }}` | The value of another parameter of the option. |

A template parameter cannot read `.outputs`. The editor shows `the placeholder .outputs.bucket is not allowed: a template parameter cannot read the outputs` at the field, and an unknown placeholder gets the same kind of error. The page refuses a save while a template has an error.

The preview under the editor renders the template for a claim of the environment, which you pick. A parameter that holds an object renders as YAML. If the environment has no claim yet, the preview uses the example claim `example` in the first namespace. If the environment has no namespace, the preview asks you to add one first.

## Limits set at install

An option can have limits that its install sets, for example a permissions boundary for the role of each cluster. The page shows each limit by its label, as **Permissions boundary (limit set at install)**, with its value. The page cannot change a limit. The chart of the option sets it.

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
| `orders in team-a uses release 8.9.22, which this change no longer allows.` | The cluster keeps running. A later change of its release must pick an allowed release. The same note exists for a cluster preset, an Elasticsearch preset, and a PostgreSQL preset. |
| `orders in team-a names no release and moves to 8.9.23.` | The claim follows the default release, so its cluster moves to the new release. Its brokers restart one at a time. |
| `orders in team-a names no release, and its cluster refuses the move to 8.9.0 (VersionDowngradeRefused).` | The new default release is older than the version that the cluster reports, or skips a minor after that version (`VersionSkipRefused`). A downgrade that the claim or the cluster approves gets no such note, see [Downgrade](../crds/camundaclusterclaim.md#downgrade). If the cluster reports no version, the note compares the new default with the old default. The cluster keeps running and takes no further change. |
| `orders in team-a names no platform config and moves to shared.` | The claim follows the default platform config. |
| `This is the first environment. The claims in legacy have no environment label, …` | Once an environment exists, the claims of a namespace without the label take no change. Add those namespaces to an environment. |
| `orders in team-a names no release, and this change removes the default release.` | The cluster keeps running, but the claim is not ready and its cluster takes no further change. The same note exists for the platform config. A management plane without a platform config keeps running, and its claim reports `MissingInput` and takes no further change. |

**Review** shows the change to the environment before the save. A field manager is a tool that writes the object, such as `kubectl apply` or a GitOps tool. If another field manager set a field that the change leaves out, the field stays. To remove such a field, remove it with the tool that set it. The review names each one: `spec.sections.backup-storage stays, because another field manager set it.`

If the environment changed after you opened the page, the save writes nothing. It shows `The Environment production changed since you opened it.` Select **Discard** to load the current environment, then make your change again.

In Git mode, the save pushes a commit, and the page waits until GitOps applied it. The form stays locked while it waits.

## Delete an environment

The section **Delete the environment** offers the delete only when no namespace carries the label of the environment. The fleet manager must also be able to read the namespaces. The delete is also off while the review of a new namespace is open. Remove the label from each namespace first, for example with `kubectl label namespace my-cluster-ns fleet.camunda.io/environment-`. The page does not remove a label. Then type the name of the environment and select **Review the delete**. The review shows the dry run of the delete, and **Delete the environment** deletes it.

In Git mode the delete is a commit that removes the file of the environment, and the button reads **Push the delete to Git**. GitOps must prune the Environment to delete it from the cluster.

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
