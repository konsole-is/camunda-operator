# PluginSection

A `PluginSection` is one step of the create flow of a claim in the fleet manager, for example "Backup storage" or "Access". The [PluginRegistrations](pluginregistration.md) that name the section are its options. The operator writes the core sections. A plugin chart writes a section that it brings.

`PluginSection` is cluster-scoped. It is an alpha API in the group `plugins.camunda.io/v1alpha1`, and its fields can change in a later release.

The smallest section:

```yaml
apiVersion: plugins.camunda.io/v1alpha1
kind: PluginSection
metadata:
  name: dns-zone
spec:
  label: DNS zone
  selection: One
  appliesTo:
    - CamundaClusterClaim
```

```mermaid
graph LR
    PR[PluginRegistration] -.->|sections| PS[PluginSection]
```

## Slots

A claim generates a `CamundaCluster` or a `CamundaManagementCluster`. A slot is the place in that generated resource that the outputs of the selected option fill. For example, the core section `backup-storage` has the slot `backupStorage`. A claim selects an option of `backup-storage`, and the plugin writes the output `objectStorageConfig`. The value of this output becomes `spec.backupStorageRef` of the generated `CamundaCluster`.

| Slot | Outputs | What the outputs fill |
| --- | --- | --- |
| `workloadIdentity` | `principal`, `serviceAccountAnnotations` | `serviceAccountAnnotations` fills the ServiceAccount annotations of the `CamundaCluster`. `principal` fills no field. |
| `volumeEncryption` | `storageClassName` | The StorageClass of the Zeebe volumes. |
| `backupStorage` | `objectStorageConfig` | `spec.backupStorageRef` of the `CamundaCluster`. |
| `secondaryStorage` | `secondaryStorageConfig` | `spec.storageRef` of the `CamundaCluster`. |
| `externalAccess` | `externalUrl`, `grpcHost`, `optimizeUrl`, `managementHost` | `externalUrl` fills the external URL of the `CamundaCluster`, and `optimizeUrl` the external URL of its `CamundaOptimize`. `managementHost` fills the five URLs of the `CamundaManagementCluster`: `https://<host>/auth`, `/identity`, `/console`, `/modeler`, and `/modeler-ws`. `grpcHost` fills no field. |
| `managementDatabases` | `keycloakDatabaseConfig`, `identityDatabaseConfig`, `webModelerDatabaseConfig` | The database references of Keycloak, Management Identity, and Web Modeler in the `CamundaManagementCluster`. |
| `connectorsIdentity` | `principal`, `serviceAccountAnnotations` | `serviceAccountAnnotations` fills the ServiceAccount annotations of the Connectors. `principal` fills no field. |

An output that fills no field, such as `principal` or `grpcHost`, is still in `status.outputs` of the request. Another option can read it as an input, for example `cloud-identity/principal`.

The set of slots is closed, and only core sections use them. A section that a plugin brings has no slot. Its outputs fill nothing in the generated resource. They are only inputs of other registrations, as in [A section that a plugin brings](#a-section-that-a-plugin-brings). The core section `encryption-key` has no slot either. Other sections read its key as an input.

A slot takes the outputs of one option only, so a section with a slot must have `selection: One`. A section without a slot can have `selection: One` or `selection: Many`.

## Claim kinds

`appliesTo` lists the claim kinds that can select an option of the section: `CamundaClusterClaim`, `CamundaManagementClusterClaim`, or both. The create flow of the fleet manager offers no option of the section to a claim of another kind.

A claim of another kind that selects an option of the section is refused with `NotAllowed`. The message is `section "<name>" does not apply to a <kind>`. For that kind, the section is never required, also when the `Environment` marks it required. Thus one `Environment` can require `backup-storage` for its clusters and `management-database` for its planes.

## Core sections

The operator writes eight core sections:

| Name | Label | Slot | Empty option | Claim kinds | Outputs |
| --- | --- | --- | --- | --- | --- |
| `cloud-identity` | Cloud identity | `workloadIdentity` | No cloud identity | `CamundaClusterClaim` | `principal`, `serviceAccountAnnotations` |
| `encryption-key` | Encryption key | none | No key | `CamundaClusterClaim` | `keyRef`, `format` |
| `volume-encryption` | Volume encryption | `volumeEncryption` | Unencrypted volumes | `CamundaClusterClaim` | `storageClassName` |
| `backup-storage` | Backup storage | `backupStorage` | No backups | `CamundaClusterClaim` | `objectStorageConfig` |
| `secondary-storage` | Secondary storage | `secondaryStorage` | none | `CamundaClusterClaim` | `secondaryStorageConfig` |
| `access` | Access | `externalAccess` | No access from outside | `CamundaClusterClaim`, `CamundaManagementClusterClaim` | `externalUrl`, `grpcHost`, `optimizeUrl`, `managementHost` |
| `connectors-permissions` | Connectors permissions | `connectorsIdentity` | No cloud permissions | `CamundaClusterClaim` | `principal`, `serviceAccountAnnotations` |
| `management-database` | Databases | `managementDatabases` | none | `CamundaManagementClusterClaim` | `keycloakDatabaseConfig`, `identityDatabaseConfig`, `webModelerDatabaseConfig` |

Every core section has `selection: One`. Their `order` values are 10 to 80, in steps of 10, in the order of the table. A plugin section with an `order` between two of these values goes between them in the create flow.

The operator writes the core sections each time the manager starts:

- A core section that you delete comes back at the next start.
- A field of a core section that you change goes back to its value at the next start.
- A field that the operator does not write stays, for example a label that you add to `metadata.labels`.
- The operator never deletes a section, also when a later release no longer writes it. A claim can still name it.

The operator writes these fields with the field manager `camunda-operator-builtins`. A GitOps tool that writes the same fields of a core section shows this name in its conflict message.

## Status

`PluginSection` has no status.

## Fields

The [API reference](api-reference.md#pluginsection) lists every field of `PluginSection`, with its type, whether it is required, its default, and its validation. The `label`, `description`, `order`, and `emptyOptionLabel` fields change only what the fleet manager shows.

## Validation rules

The API server refuses a section when:

- It has a slot and `selection` is not `One`: `a section with a slot must have selection One, because a slot fills one field`.
- `emptyOptionLabel` is longer than 64 characters: `emptyOptionLabel must be at most 64 characters`.
- `appliesTo` is empty: `appliesTo must list at least one claim kind`.
- The name is longer than 63 characters: `metadata.name must be at most 63 characters, because it is used as a label value`.
- `label` is empty, or two outputs have the same `key`.

## A section that a plugin brings

A DNS plugin adds a section without a slot. Its output is an input of other registrations, for example of an option of `access` that writes the DNS records of the cluster:

```yaml
apiVersion: plugins.camunda.io/v1alpha1
kind: PluginSection
metadata:
  name: dns-zone
spec:
  label: DNS zone
  description: The DNS zone that holds the records of the cluster.
  order: 65
  selection: One
  emptyOptionLabel: No DNS records
  appliesTo:
    - CamundaClusterClaim
  outputs:
    - key: zoneId
      description: The ID of the DNS zone.
      format: aws-route53-zone-id
```

An option of `access` reads the zone as an input, in the form `<section>/<output>`:

```yaml
apiVersion: plugins.camunda.io/v1alpha1
kind: PluginRegistration
metadata:
  name: route53-ingress
spec:
  sections:
    - access
  inputs:
    - name: zoneId
      from: dns-zone/zoneId
      required: true
  # ... the rest of your registration
```

The output `zoneId` fills no field of the generated `CamundaCluster`. Only the option `route53-ingress` reads it.

## Related

- [PluginRegistration](pluginregistration.md): the options of a section.
- [PluginRequest](pluginrequest.md): the request that a claim writes for the option that it selects.
- [API reference](api-reference.md#pluginsection): every field of the kind.
