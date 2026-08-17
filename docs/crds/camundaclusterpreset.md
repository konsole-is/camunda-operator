# CamundaClusterPreset

Cluster-scoped, passive preset that defines a standardized orchestration cluster configuration for [CamundaCluster](camundacluster.md) resources to inherit.

## Purpose

`CamundaClusterPreset` lets platform teams define standardized cluster shapes — sizing, topology, and defaults — once, so individual clusters stay small and consistent.
You create presets (or a composition layer above ships a catalog of them, e.g. `small`, `medium`, `large`), and each [CamundaCluster](camundacluster.md) opts in by name via `presetRef`.
It is a passive data CRD: there is no preset controller, and creating a preset by itself does nothing.

## How it works

There is no controller for this kind; consumers resolve and merge presets themselves.
The CamundaCluster controller resolves `presetRef` on each reconcile, so editing a preset propagates to every referencing cluster on their next reconcile.

`spec.cluster` reuses the preset-legal subset of the CamundaCluster spec type as a full baseline, and the effective spec of a referencing cluster is computed with these merge rules:

1. Start from the preset's `spec.cluster` as the baseline.
2. Instance-bound fields are cluster-only and rejected in a preset (see Validation): the baseline carries only configuration that is meaningful for any number of clusters.
3. Scalar and pointer fields override individually: a value set on the CamundaCluster replaces the preset's value, and an absent field inherits it. This covers `version`, the `auth` client-credential fields, per-component `mode`, `replicas`, `partitions`, `replicationFactor`, `storageClassName`, `storageSize`, `persistentVolumeClaimRetentionPolicy`, `connectors.enabled`, and `connectors.version`.
4. `resources` merges per entry: a request or limit value set on the CamundaCluster replaces the preset's matching entry, and unset entries inherit.
5. `extraEnv` lists merge by variable name: preset entries come first, and a CamundaCluster entry with the same name replaces the preset's entry.
6. `extraEnvFrom` lists concatenate: preset entries first, then CamundaCluster entries.
7. `podLabels` and `podAnnotations` maps merge by key, with the CamundaCluster winning on conflicts.
8. `scheduling` is the exception and never merges: if the CamundaCluster sets `scheduling` at some level (top-level or per component), it replaces the preset's `scheduling` at that level entirely, because partial scheduling merges are error-prone.
9. `backup` merges per field. `backup.primaryStorage` overrides field by field, so a cluster can change the schedule and keep the retention of the preset. `backup.dump` follows the component rules above (rules 4 to 8), and `backup.dump.scratchVolume` replaces as a whole block, like `scheduling`. `backup.primaryStorage.continuous` is a pointer, so a preset can turn continuous backups on for a fleet and one cluster can still set it to `false`.

The override surface is deliberately small — sizing, env vars, and metadata that commonly vary per cluster.
Clusters that fit no preset skip `presetRef` and configure everything inline.

```mermaid
graph LR
    CC[CamundaCluster] -.->|presetRef| CCP[CamundaClusterPreset]
```

!!! note "Deviation from the original proposal"
    The proposal had a preset controller that created `PVCAutoResize` CRs from an `autoResize` block on the preset; that design is dropped.
    Presets carry no `autoResize` fields, and `PVCAutoResize` CRs are always created explicitly by you or a composition layer above.

## API reference

```yaml
apiVersion: core.camunda.io/v1
kind: CamundaClusterPreset
metadata:
  # Cluster-scoped: no namespace. Presets are conventionally named for their size or role.
  name: medium
spec:
  # object. Required. Preset-legal subset of the CamundaCluster spec as a baseline; see the CamundaCluster API reference for field details and Validation below for the fields a preset must not set.
  cluster:
    # string. Optional. Camunda version applied to clusters that do not set their own.
    version: "8.9.0"
    # object. Optional. Per-cluster OIDC client-credential defaults for referencing clusters; same shape as the CamundaCluster auth block, sits between the platform config's defaults and a cluster's own auth override.
    auth:
      # string. Optional. Default OIDC client ID for referencing clusters.
      clientId: "medium-clusters"
      # string. Optional, default: the clientId. Audience validated in access tokens.
      audience: "medium-clusters"
      # object. Optional. Secret holding the default client secret; the namespace is required, because presets are cluster-scoped and have no namespace to default to.
      clientSecretRef:
        name: "medium-clusters-oidc-secret"
        namespace: "camunda-system"
        key: "client-secret"
    # list. Optional. Env vars applied to ALL workloads of referencing clusters; merged by name, with cluster entries winning.
    extraEnv:
      - name: TZ
        value: "UTC"
    # list. Optional. Bulk env from ConfigMaps/Secrets applied to ALL workloads; preset entries first, then cluster entries.
    extraEnvFrom: []
    # map[string]string. Optional. Labels merged into all workload pods of referencing clusters.
    podLabels:
      company.com/team: "automation-ops"
    # map[string]string. Optional. Annotations merged into all workload pods of referencing clusters.
    podAnnotations:
      company.com/cluster-preset: "medium"
    # object. Optional. Scheduling baseline for all workloads; a cluster that sets its own scheduling replaces this entirely (no merge).
    scheduling: {}
    # object. Optional. Backup policy baseline for all clusters that reference this preset.
    # See the CamundaCluster doc for the full block. backupStorageRef is instance-bound and stays on the cluster.
    backup:
      primaryStorage:
        continuous: true
        schedule: "PT1H"
        checkpointInterval: "PT15M"
        retention:
          window: "P7D"
          cleanupSchedule: "PT1H"
    # object. Optional. Zeebe baseline; zeebe is always a standalone StatefulSet.
    zeebe:
      # integer. Optional. Broker replica baseline.
      replicas: 3
      # integer. Optional. Partition count baseline.
      partitions: 3
      # integer. Optional. Replication factor baseline.
      replicationFactor: 3
      # object. Optional. Compute resources baseline.
      resources:
        requests: { cpu: "1", memory: "2Gi" }
      # string. Optional. StorageClass for broker volumes.
      storageClassName: "ssd"
      # quantity. Optional. Broker volume size. A cluster that applied a larger size keeps it and records a StorageShrinkIgnored event.
      storageSize: "32Gi"
      # object. Optional. What happens to the broker volumes when a referencing cluster is deleted; same shape as on the CamundaCluster.
      persistentVolumeClaimRetentionPolicy:
        # string (Retain | Delete). Optional, default: Delete.
        whenDeleted: Delete
      # list. Optional. Env var baseline, merged by name with cluster-level entries; an entry replaces an operator entry with the same name.
      extraEnv:
        - name: JAVA_TOOL_OPTIONS
          value: "-XX:+ExitOnOutOfMemoryError -Xmx4g"
    # object. Optional. Gateway baseline.
    gateway:
      # string. Optional. Standalone | Embedded.
      mode: Standalone
      # integer. Optional. Gateway replica baseline.
      replicas: 2
      # object. Optional. Compute resources baseline.
      resources:
        requests: { cpu: "500m", memory: "1Gi" }
    # object. Optional. Operate baseline.
    operate:
      # string. Optional. Standalone | Embedded.
      mode: Embedded
    # object. Optional. Tasklist baseline.
    tasklist:
      # string. Optional. Standalone | Embedded.
      mode: Embedded
    # object. Optional. Admin baseline. Identity was renamed to Admin in Camunda 8.9; the profile is `admin`.
    admin:
      # string. Optional. Standalone | Embedded.
      mode: Embedded
    # object. Optional. Connectors baseline; connectors are standalone-only.
    connectors:
      # boolean. Optional. Whether referencing clusters run the connectors runtime.
      enabled: true
      # string. Optional. Connectors bundle version applied to clusters that do not set their own.
      version: "8.9.7"
      # integer. Optional. Connectors replica baseline.
      replicas: 1
      # object. Optional. Compute resources baseline.
      resources:
        requests: { cpu: "250m", memory: "512Mi" }
```

## Status

This is a passive data CRD: no controller reconciles it and it reports no status.
Reference errors surface on the consumer instead — a [CamundaCluster](camundacluster.md) pointing at a missing preset reports `Ready: False` with reason `InvalidReference`.

## Validation

- `spec.cluster` must be present and must conform to the preset-legal subset of the CamundaCluster spec schema.
- Instance-bound CamundaCluster fields are rejected at admission inside `spec.cluster`: `platformConfigRef`, `presetRef` (no preset chaining), `externalUrl`, `serviceAccount`, `storageRef`, `backupStorageRef`, `documentStorageRef`, `monitoring`, `suspend`, and `pause`.
- Preset-legal fields are everything else: `version`, `auth`, the per-component blocks (`zeebe`, `gateway`, `operate`, `tasklist`, `admin`, `connectors`), `backup`, and the top-level `extraEnv`, `extraEnvFrom`, `podLabels`, `podAnnotations`, and `scheduling`. `backup` is policy and belongs in a preset; `backupStorageRef`, which says where backups go, is instance-bound.
- There is no cross-resource validation: preset resolution problems are reported by the consuming controller.

## Relationships

- [CamundaCluster](camundacluster.md) — references this CR via `presetRef` and inherits its baseline under the merge rules above.
- [CamundaPlatformConfig](camundaplatformconfig.md) — a preset's `auth` baseline sits between the platform config's environment defaults and a cluster's own `auth` override.
- [PVCAutoResize](pvcautoresize.md) — never created by presets; create it explicitly per cluster (deviation from the original proposal, which had preset-driven auto-resize).

A composition layer above may ship a standard catalog of presets for its clusters to reference.

## Examples

A minimal manifest:

```yaml
apiVersion: core.camunda.io/v1
kind: CamundaClusterPreset
metadata:
  name: small
spec:
  cluster:
    version: "8.9.0"
    zeebe:
      replicas: 1
      partitions: 1
      replicationFactor: 1
      storageSize: "10Gi"
```

A realistic manifest:

```yaml
apiVersion: core.camunda.io/v1
kind: CamundaClusterPreset
metadata:
  name: medium
spec:
  cluster:
    version: "8.9.0"
    podLabels:
      company.com/team: "automation-ops"
    podAnnotations:
      company.com/cluster-preset: "medium"
    zeebe:
      replicas: 3
      partitions: 3
      replicationFactor: 3
      resources:
        requests: { cpu: "1", memory: "2Gi" }
        limits: { cpu: "2", memory: "4Gi" }
      storageClassName: "ssd"
      storageSize: "32Gi"
      extraEnv:
        - name: JAVA_TOOL_OPTIONS
          value: "-XX:+ExitOnOutOfMemoryError -Xmx4g"
    gateway:
      mode: Standalone
      replicas: 2
      resources:
        requests: { cpu: "500m", memory: "1Gi" }
    operate:
      mode: Embedded
    tasklist:
      mode: Embedded
    admin:
      mode: Embedded
    connectors:
      enabled: true
      version: "8.9.7"
      replicas: 1
      resources:
        requests: { cpu: "250m", memory: "512Mi" }
```
