# Environments

An environment sets the rules for the clusters and management planes in its namespaces. It names the platform config, the presets, and the releases that they can use. It sets the storage and access options that they can pick, and the values of the parameters of these options. It also sets what happens to the resources on delete, and the rules for a new namespace. The [Environment](../crds/environment.md) reference lists the fields.

The page **Setup > Environments** lists the environments with their color, their namespaces, their clusters, their management planes, and their status. Select an environment to open its editor, or select **New environment**.

## Namespaces

A namespace belongs to the environment that its label `fleet.camunda.io/environment` names:

```yaml
apiVersion: v1
kind: Namespace
metadata:
  name: my-cluster-ns
  labels:
    fleet.camunda.io/environment: production
```

To add a namespace, pick it in **Add a namespace** and select **Add**. Then confirm with **Add the namespace**. The list offers only namespaces without an environment. A new environment has no namespaces yet, so save it first. The label goes straight to the Kubernetes API, also in Git mode.

!!! warning "A namespace without an environment"
    Once one environment exists, the operator refuses each claim in a namespace without the label. A new cluster there is not created, and an existing cluster takes no change. The list of environments shows a banner with each such namespace. Add the namespace to an environment.

## Platform config, presets, and releases

The parts **Platform config**, **Presets**, and **Releases** of the editor set these fields:

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

A cluster can use only the presets and the releases that its environment allows. The Elasticsearch and PostgreSQL presets are for the built-in options **New Elasticsearch cluster** and **New PostgreSQL server**. An empty list allows none. The default release must be one of the allowed releases.

A cluster that names no release uses the default release. A cluster that names no platform config uses the one of the environment. So a change of either default moves such clusters to the new value.

## Storage and access options

Each storage and access choice of a cluster, such as **Backup storage** or **Access**, has a row in the part **Sections** of the editor. The API calls such a choice a section. An option is a registration that serves the choice, for example `rustfs` for backups:

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

- **Allowed options**: the options that a cluster of the environment can pick. Without an allowed option, the choice offers only its empty option, for example **No backups**.
- **Default option**: the option that the create form picks first. It must be one of the allowed options.
- **Required**: a cluster must pick an allowed option. The create form then does not offer the empty option.

The row offers only the options that the operator accepted. An option that is not ready shows `Not ready:` and its message. You can allow it, and the create form offers it when it is ready.

Some options need a value from another choice, for example encrypted volumes need an **Encryption key**. Set a default in that other choice, so that the create form picks one.

## Settings of the options

The part **Settings of the options** lists the parameters of each allowed option:

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

A value of the environment replaces the default of the option. While the field is empty, the page shows the default under it, for example `From the registration: 3`.

**A claim can change it** lets each cluster of the environment set its own value. With it on, **Add a choice** builds a list of named values. A cluster can then pick only a name of the list, and only the option gets the value, for example the ARN of a policy.

A parameter that the option requires needs a value from the environment or from the option, or **A claim can change it**. Without one, the page shows `This parameter needs a value.`, and the create form refuses the option.

### Template parameters

Some parameters are templates, for example the inline policy of the Connectors role. The editor has a chip for each placeholder:

| Placeholder | Value |
| --- | --- |
| `{{ .namespace }}` | The namespace of the cluster. |
| `{{ .claim }}` | The name of the claim. |
| `{{ .request }}` | The name of the request of the claim for this option. |
| `{{ .serviceAccount }}` | The ServiceAccount that gets the permissions. |
| `{{ .parameters.<name> }}` | The value of another parameter of the option. |

The preview under the editor shows the template for a cluster of the environment that you pick.

### Limits set at install

An option can have limits that its Helm chart sets, for example a permissions boundary for the role of each cluster. The page shows them, for example as **Permissions boundary (limit set at install)**, but cannot change them.

## Namespace creation

In the part **Namespaces**, **The create flow can create a namespace** lets the create form make a new namespace in the environment. **Fixed labels of a new namespace** are labels that each new namespace gets. For each key of **Required label keys**, the create form asks for a value:

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

## When a claim is deleted

The part **When a claim is deleted** sets the default for new clusters and planes. The choices are **Delete the resources of the claim** (`Delete`) and **Keep the resources of the claim** (`Orphan`). The resources are those of the storage and access options, for example a bucket.

```yaml
apiVersion: fleet.camunda.io/v1alpha1
kind: Environment
metadata:
  name: production
spec:
  deletionPolicy: Orphan
  # ... the rest of your environment
```

## Save a change

**Review** shows the change before the save. In Git mode, the save pushes a commit, and the form stays locked until GitOps applies it.

A cluster keeps the rules that applied when it was created, so most changes reach only new clusters. A note above **Review** counts the clusters and planes that keep their rules:

> Changes apply to new claims only. 2 clusters and 1 management plane keep the policy they recorded at creation.

The note also names each cluster that the change moves, for example a cluster that follows the default release:

> orders in team-a names no release and moves to 8.9.23.

Such a cluster restarts its brokers one at a time.

## Delete an environment

The delete is possible only when no namespace carries the label of the environment. Remove the label from each namespace first:

```bash
kubectl label namespace my-cluster-ns fleet.camunda.io/environment-
```

Then type the name of the environment, select **Review the delete**, and select **Delete the environment**. In Git mode, the button is **Push the delete to Git**.

## Status

The editor shows the `Ready` condition of the environment. When the environment names an object that does not exist, the message names it:

```yaml
status:
  conditions:
    - type: Ready
      status: "False"
      reason: InvalidReference
      message: "The spec names objects that do not exist: CamundaRelease camunda-8-9-9"
  namespaces: 2
```

Create the object, or clear its name in the editor. A name without an object shows **Not found** in the editor.
