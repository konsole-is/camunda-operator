# ElasticsearchClusterPreset

Cluster-scoped, passive baseline configuration for `ElasticsearchCluster` resources.

## Purpose

`ElasticsearchClusterPreset` captures a standardized Elasticsearch sizing — version, node count, resources, storage, scheduling — as reusable data, so that individual [ElasticsearchCluster](elasticsearchcluster.md) resources stay small and consistent.
It is a passive data CRD: no controller reconciles it and it provisions nothing.
You create presets directly, or a composition layer above may install a standard set of sizes (for example `small`, `standard`, `large`).

## How it works

`ElasticsearchClusterPreset` has no controller; consumers resolve and merge it instead.

1. An `ElasticsearchCluster` names a preset through its `spec.presetRef` (a plain string, since presets are cluster-scoped).
2. The `ElasticsearchCluster` controller reads `spec.cluster` from the preset as the full configuration baseline.
3. Any field set inline on the `ElasticsearchCluster` overrides the preset's value for that field wholesale; fields left unset inherit from the preset. An empty list or map (`extraEnv`, `extraEnvFrom`, `podLabels`, `podAnnotations`) counts as unset, because the API drops it: to remove a list that the preset provides, override it with the list you want, or reference a preset without it.
4. `scheduling` is the explicit exception spelled out for emphasis: an inline `scheduling` block replaces the preset's entire scheduling block, it is never merged field by field.
5. Editing a preset flows to every `ElasticsearchCluster` that references it on their next reconciliation.

```mermaid
graph LR
    ESC[ElasticsearchCluster] -.->|presetRef| ESCP[ElasticsearchClusterPreset]
```

!!! note "Deviation from the original proposal"
    The proposal let presets carry an `autoResize` block from which the preset machinery would create `PVCAutoResize` resources.
    Presets are passive data with no controller, so the `autoResize` field does not exist; create a `PVCAutoResize` explicitly when you need volume auto-resizing.

## API reference

`spec.cluster` reuses the `ElasticsearchCluster` spec type directly, so the two never drift apart.
The instance-bound fields of that type — `presetRef`, `secondaryStorageConfig`, and `suspend` — must be left unset inside a preset. Every other field is inheritable, `serviceAccount`, `snapshotStorageRef`, `secureSettings`, and `monitoring` included. A preset alone can make the operator create the `<name>-es` ServiceAccount. A preset alone can also put a whole fleet on one snapshot bucket. `monitoring` is a baseline like any other field: a preset can enable scraping and pin the exporter image and resources for every cluster that references it, and a cluster that sets its own `monitoring` block replaces the preset's wholesale.

```yaml
apiVersion: core.camunda.io/v1
kind: ElasticsearchClusterPreset
metadata:
  name: standard
spec:
  # object (ElasticsearchCluster spec type). Required. The full configuration baseline consumers inherit.
  cluster:
    # string. Optional. Elasticsearch version to deploy; Camunda 8.9 supports 8.19+ and 9.2+.
    version: "9.2.4"
    # integer. Optional. Number of Elasticsearch nodes.
    replicas: 3
    # object (corev1.ResourceRequirements). Optional. CPU and memory for each Elasticsearch node.
    resources:
      requests: { cpu: "1", memory: "2Gi" }
      limits: { memory: "2Gi" }
    # string (resource quantity). Optional. Size of each node's data volume.
    storageSize: "64Gi"
    # string. Optional, default: the cluster's default StorageClass. StorageClass for the data volumes.
    storageClassName: "ssd"
    # string. Optional. Name of a cluster-scoped ObjectStorageConfig holding the bucket of the snapshot repository; see ElasticsearchCluster.
    snapshotStorageRef: "my-backup-config"
    # list of objects. Optional. Secrets that ECK loads into the keystore of every node; see ElasticsearchCluster.
    secureSettings: []
    # list (corev1.EnvVar). Optional. Extra environment variables for every Elasticsearch node.
    extraEnv: []
    # list (corev1.EnvFromSource). Optional. Extra environment sources (ConfigMaps, Secrets) for every node.
    extraEnvFrom: []
    # map[string]string. Optional. Extra labels applied to the Elasticsearch pods.
    podLabels: {}
    # map[string]string. Optional. Extra annotations applied to the Elasticsearch pods.
    podAnnotations: {}
    # object. Optional. Scheduling constraints; replaced entirely when the referencing ElasticsearchCluster sets its own scheduling block.
    scheduling:
      # object (corev1.NodeAffinity). Optional. Node affinity rules.
      nodeAffinity: {}
      # object (corev1.PodAffinity). Optional. Pod affinity rules.
      podAffinity: {}
      # list (corev1.Toleration). Optional. Tolerations for the pods.
      tolerations: []
    # object. Optional. Dedicated ServiceAccount for the nodes; see ElasticsearchCluster.
    serviceAccount:
      annotations: {}
    # object. Optional. Metrics baseline; a cluster that sets its own monitoring block replaces it wholesale.
    monitoring:
      serviceMonitor:
        enabled: false
      exporter:
        image: ""
        resources: {}
    # object. Optional. Data volume retention on cluster deletion; see ElasticsearchCluster.
    persistentVolumeClaimRetentionPolicy:
      whenDeleted: Delete
```

## Status

`ElasticsearchClusterPreset` is passive data: no controller reconciles it, so it reports no conditions and no `status.observedGeneration`.
Problems with a preset (a dangling `presetRef`, or a merge that still lacks required fields) surface on the referencing `ElasticsearchCluster`'s conditions.

## Validation

- `spec.cluster` must not set the instance-bound fields `presetRef` (presets cannot chain), `secondaryStorageConfig`, or `suspend`. Explicit zero values — an empty `presetRef` or `suspend: false` — count as unset, so templated YAML that renders unset fields as zero values still applies. An empty-string `secondaryStorageConfig` is rejected by the resource-name pattern; omit the field instead.
- No other rules beyond schema validation; completeness of the merged configuration is validated on the consuming `ElasticsearchCluster`. In particular the `ElasticsearchCluster` no-shrink rule for `storageSize` does not bind a preset. Its baseline can be resized freely. A referencing cluster that already applied a larger size keeps that size and records a `StorageShrinkIgnored` event; a new cluster uses the new baseline.

## Relationships

- [ElasticsearchCluster](elasticsearchcluster.md) — references this preset via `presetRef` and uses `spec.cluster` as its configuration baseline.

A composition layer above may install and maintain the standard preset catalog.

## Examples

A minimal manifest:

```yaml
apiVersion: core.camunda.io/v1
kind: ElasticsearchClusterPreset
metadata:
  name: standard
spec:
  cluster:
    version: "9.2.4"
    replicas: 3
    storageSize: "64Gi"
```

A realistic manifest:

```yaml
apiVersion: core.camunda.io/v1
kind: ElasticsearchClusterPreset
metadata:
  name: large
spec:
  cluster:
    version: "9.2.4"
    replicas: 5
    resources:
      requests: { cpu: "4", memory: "8Gi" }
      limits: { memory: "8Gi" }
    storageSize: "256Gi"
    storageClassName: "ssd"
    scheduling:
      tolerations:
        - key: dedicated
          operator: Equal
          value: elasticsearch
          effect: NoSchedule
```
