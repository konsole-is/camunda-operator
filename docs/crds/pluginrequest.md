# PluginRequest

A `PluginRequest` asks a plugin for the resources of one section of a claim. A claim writes one request for each option that it selects, for example the request `orders-backup-storage` of the claim `orders`. The plugin that serves the [PluginRegistration](pluginregistration.md) of the option writes the status: its outputs, its health, and the resources that it made.

`PluginRequest` is namespaced. It lives in the namespace of its claim. It is an alpha API in the group `plugins.camunda.io/v1alpha1`, and its fields can change in a later release.

The smallest request:

```yaml
apiVersion: plugins.camunda.io/v1alpha1
kind: PluginRequest
metadata:
  name: orders-backup-storage
  namespace: my-cluster-ns
spec:
  section: backup-storage
  registration: s3-backup-bucket
  claimRef:
    kind: CamundaClusterClaim
    name: orders
  target:
    kind: CamundaCluster
    name: orders
  deletionPolicy: Delete
```

```mermaid
graph LR
    PQ[PluginRequest] -.->|section| PS[PluginSection]
    PQ -.->|registration| PR[PluginRegistration]
    PQ -.->|target| CC[CamundaCluster]
    PQ -->|"creates (by the plugin)"| G[Grant]
```

## Labels

Each request carries three labels that you can select on:

| Label | Value |
| --- | --- |
| `plugins.camunda.io/section` | The name of the section. |
| `plugins.camunda.io/registration` | The name of the registration. |
| `fleet.camunda.io/claim` | The name of the claim. |

To list the requests of one claim:

```bash
kubectl get pluginrequests -n my-cluster-ns -l fleet.camunda.io/claim=orders
```

Each resource that a plugin writes for a request carries the labels `plugins.camunda.io/registration` and `plugins.camunda.io/request`.

A cluster-scoped resource, for example a StorageClass, also carries the label `plugins.camunda.io/request-namespace` with the namespace of the request. Two claims with the same name in two namespaces each get their own cluster-scoped resources, and this label tells them apart.

To list the StorageClasses of one request:

```bash
kubectl get storageclasses -l plugins.camunda.io/request=orders-backup-storage,plugins.camunda.io/request-namespace=my-cluster-ns
```

## Outputs

`status.outputs` holds the values that the plugin publishes for the request, by output key. Read them only when the condition `OutputsReady` is `True` and its `observedGeneration` equals `metadata.generation`. Before that, the outputs can belong to an older spec.

## Deletion

`spec.deletionPolicy` says what the plugin does with the resources of the request when the request is deleted:

- `Delete`: the plugin deletes the resources that it wrote for the request.
- `Orphan`: the plugin keeps the resources. A namespaced resource loses its owner reference to the request and keeps its labels. `status.resources[].onOrphan` says, for each resource, whether it is `Kept` or `Deleted` with this policy.

With both policies, the plugin deletes a route to a Service that no longer exists. The request goes away only after the plugin applies the policy. The claim copies its own deletion policy to the request before it deletes the request.

## Built-in requests

The operator serves the requests of three [built-in registrations](pluginregistration.md#built-in-registrations). Each one points a section at a contract that is already in the namespace of the claim:

| Registration | Parameters | Outputs |
| --- | --- | --- |
| `existing-object-storage` | `name` (required): an `ObjectStorageConfig` | `objectStorageConfig` |
| `existing-secondary-storage` | `name` (required): a `SecondaryStorageConfig` | `secondaryStorageConfig` |
| `existing-database-configs` | `keycloak`, `identity`, `webModeler`: `DatabaseConfig` names | `keycloakDatabaseConfig`, `identityDatabaseConfig`, `webModelerDatabaseConfig`, one for each name that you set |

The claim writes the request. A request for the bucket `my-backup-bucket`:

```yaml
apiVersion: plugins.camunda.io/v1alpha1
kind: PluginRequest
metadata:
  name: orders-backup-storage
  namespace: my-cluster-ns
  labels:
    plugins.camunda.io/section: backup-storage
    plugins.camunda.io/registration: existing-object-storage
    fleet.camunda.io/claim: orders
spec:
  section: backup-storage
  registration: existing-object-storage
  parameters:
    name: my-backup-bucket
  # ... the rest of your request
```

A built-in request waits until its registration is `Ready` at the current generation of the registration. Until then, the operator does not change the status of the request. A deletion does not wait.

The outputs hold the names of the contracts. `OutputsReady` turns `True` when every named contract exists, and `Ready` follows `OutputsReady`. The first item of `summary` names the contract and says if its `Ready` condition is `True`:

```yaml
status:
  observedGeneration: 1
  conditions:
    - type: OutputsReady
      status: "True"
      reason: Healthy
      message: Component is healthy.
      observedGeneration: 1
    - type: Ready
      status: "True"
      reason: Healthy
      message: "outputs: Component is healthy."
      observedGeneration: 1
  outputs:
    objectStorageConfig: my-backup-bucket
  summary:
    - label: ObjectStorageConfig
      value: my-backup-bucket, Ready
      severity: Info
  summaryTime: "2026-10-19T09:00:00Z"
```

If the contract exists and its `Ready` condition is not `True`, the value is `my-backup-bucket, not Ready` with the `Warning` severity. If the contract has a `Ready` reason, the value adds it, for example `my-backup-bucket, not Ready (MissingSecret)`.

While a named contract does not exist, `OutputsReady` and `Ready` are `False` with reason `Blocked`, and the message names the contract. The summary value ends in `waiting`. Create the contract, or correct the name in the claim. If a contract is deleted later, the request removes its outputs and reports `Blocked` again until the contract is back.

```yaml
status:
  conditions:
    - type: OutputsReady
      status: "False"
      reason: Blocked
      message: waiting for core.camunda.io/v1/ObjectStorageConfig/my-cluster-ns/my-backup-bucket
      observedGeneration: 1
    - type: Ready
      status: "False"
      reason: Blocked
      message: "outputs: waiting for core.camunda.io/v1/ObjectStorageConfig/my-cluster-ns/my-backup-bucket"
      observedGeneration: 1
  summary:
    - label: ObjectStorageConfig
      value: my-backup-bucket, waiting
      severity: Warning
```

If the parameters do not pass the schema of the registration, for example without the required `name`, `OutputsReady` and `Ready` are `False` with reason `InvalidParameters`. The message names each field that fails. `existing-database-configs` also refuses two fields with the same name, and the message names both fields.

When the claim or you delete the request, the contracts stay, with both deletion policies. The request goes away at once.

## Status

| Type | Reason | Meaning | What to do |
| --- | --- | --- | --- |
| `OutputsReady` | set by the plugin | `True` when the plugin wrote the outputs for the current generation. | If it stays `False`, read the message and the events of the request. |
| `OutputsReady` | `InvalidParameters` | The parameters do not pass the schema or the rules of the registration. | Correct the parameters in the claim. |
| `OutputsReady` | `Blocked` | A contract that the request names does not exist. The message names the contract. | Create the contract, or correct the name in the claim. |
| `Ready` | `Healthy` | The resources of the request are ready. | Nothing. |
| `Ready` | `GrantRejected` | A `Grant` of the request has `Ready=False`. The outputs stay usable. | Read the status of the grant. See [Grant](grant.md#status). |
| `Ready` | `WaitingForGrants` | A `Grant` of the request does not have `Ready=True` yet. The cloud permissions of the cluster are not in place. The outputs stay usable. | Wait for the identity extension to apply the grant. If it stays, read the status of the grant and the logs of the identity extension. See [Grant](grant.md#status). |
| `Ready` | `WaitingForEndpoints` | An access request waits for the endpoints in the status of its target. | Wait for the target to report its endpoints. |
| `Ready` | `InvalidParameters` | The parameters do not pass the schema or the rules of the registration. | Correct the parameters in the claim. |
| `Ready` | `Blocked` | A contract that the request names does not exist. The message names the contract. | Create the contract, or correct the name in the claim. |
| `Ready` | set by the plugin | Another state of the plugin. | Read the message, and the logs of the plugin controller. |

The plugin records an event on the request for each external action, for example a bucket that it created.

The rest of the status is for display:

```yaml
status:
  observedGeneration: 1
  conditions:
    - type: OutputsReady
      status: "True"
      reason: Healthy
      observedGeneration: 1
    - type: Ready
      status: "True"
      reason: Healthy
      observedGeneration: 1
  outputs:
    objectStorageConfig: orders-backup-storage
  summary:
    - label: Bucket
      value: orders-backup-storage
      severity: Info
  summaryTime: "2026-10-15T09:00:00Z"
  links:
    - name: Bucket console
      url: https://s3.console.aws.amazon.com/s3/buckets/orders-backup-storage
  podSelector:
    matchLabels:
      plugins.camunda.io/request: orders-backup-storage
  resources:
    - kind: S3Bucket
      name: orders-backup-storage
      id: arn:aws:s3:::orders-backup-storage
      location: eu-west-1
      role: Backup bucket
      onOrphan: Kept
```

`summary` lists facts about the request, and the first item is the headline. `severity` is `Info`, `Warning`, or `Error`. `links` are named URLs about the request. `podSelector` selects the pods that the plugin runs for the request. `resources` lists what the plugin made, in the cluster or in a cloud.

## Fields

The [API reference](api-reference.md#pluginrequest) lists every field of `PluginRequest`, with its type, whether it is required, its default, and its validation.

## Validation rules

The API server refuses a change of these fields after the request is created:

- `section`: `section is immutable: delete the request and create a new one`.
- `registration`: `registration is immutable: delete the request and create a new one`.
- `claimRef`: `claimRef is immutable: delete the request and create a new one`.
- `target.kind`: `target.kind is immutable: delete the request and create a new one`.
- `target.name`: `target.name is immutable: delete the request and create a new one`.

It also refuses a name longer than 63 characters: `metadata.name must be at most 63 characters, because it is used as a label value`.

## A request with inputs and parameters

The claim `orders` writes this request for the option `s3-backup-bucket`. It carries the key of the section `encryption-key` as the input `kmsKey`, and the retention that the claim selected:

```yaml
apiVersion: plugins.camunda.io/v1alpha1
kind: PluginRequest
metadata:
  name: orders-backup-storage
  namespace: my-cluster-ns
spec:
  section: backup-storage
  registration: s3-backup-bucket
  claimRef:
    kind: CamundaClusterClaim
    name: orders
  target:
    kind: CamundaCluster
    name: orders
    serviceAccountName: orders-camunda
    additionalServiceAccounts:
      - orders-es
  inputs:
    kmsKey: arn:aws:kms:eu-west-1:111122223333:key/example
  parameters:
    retentionDays: 30
  deletionPolicy: Delete
```

## Related

- [PluginRegistration](pluginregistration.md): the option that serves the request.
- [PluginSection](pluginsection.md): the section that the request fills.
- [Grant](grant.md): the permissions that the plugin asks for on behalf of the request.
- [API reference](api-reference.md#pluginrequest): every field of the kind.
