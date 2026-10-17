# ElasticsearchCluster

`ElasticsearchCluster` runs an Elasticsearch cluster for secondary storage through the ECK operator (Elastic Cloud on Kubernetes). You create it, or another tool creates it for you.

The operator does not run Elasticsearch itself. It creates an ECK `Elasticsearch` resource named `<name>`, and the ECK operator runs the nodes. Install ECK before you start the operator. The operator looks for the ECK CRDs only when it starts, so if you install ECK later, restart the operator.

Use this kind when you want the operator to own the Elasticsearch cluster, its credentials, and its snapshot repository. If you want PostgreSQL as secondary storage, use [Database](database.md) instead. An `ElasticsearchCluster` never references a `CamundaCluster`. The two meet only through the `SecondaryStorageConfig` that `spec.secondaryStorageConfig` names. The operator creates that [contract](index.md#contracts) in the same namespace, and it carries:

- The HTTPS endpoint `https://<name>-es-http.<namespace>.svc:9200`.
- The user `camunda`, in the Secret `<name>-es-user` (keys `username` and `password`).
- The CA of the self-signed certificate, in the Secret `<name>-es-http-certs-public` (key `ca.crt`).
- The node count, as `nodeCount`. Consumers take their default index replica count from it, so a one-node cluster stays green. [Node count](secondarystorageconfig.md#node-count) has the rule.
- `snapshotRepository`, once a repository is registered.

The Elasticsearch pods and their data volumes carry the labels `camunda.io/elasticsearch-cluster: <name>` and `camunda.io/component: elasticsearch`.

The smallest cluster names a preset, a release, and the storage contract to create:

```yaml
apiVersion: core.camunda.io/v1
kind: ElasticsearchCluster
metadata:
  name: my-cluster-es
  namespace: my-cluster-ns
spec:
  presetRef: "standard"
  releaseRef: "camunda-8-9-4"
  secondaryStorageConfig: "my-storage-config"
```

```mermaid
graph LR
    ESC[ElasticsearchCluster] -.->|presetRef| ESCP[ElasticsearchClusterPreset]
    ESC -.->|releaseRef| CR[CamundaRelease]
    ESC -.->|snapshotStorageRef| OSC[ObjectStorageConfig]
    ESC -->|creates| ECK["Elasticsearch (ECK operator, external)"]
    ESC -->|creates| SEC["Secret <name>-es-user"]
    ESC -->|creates| SSC[SecondaryStorageConfig]
    CC[CamundaCluster] -.->|storageRef| SSC
```

## Preset and release

Three layers make the configuration of a cluster. Each later layer wins over the one before it:

1. The [ElasticsearchClusterPreset](elasticsearchclusterpreset.md) that `spec.presetRef` names holds the shape: the node count, the volume size, the resources.
2. The [CamundaRelease](camundarelease.md) that `spec.releaseRef` names holds the version, in `spec.elasticsearch.version`.
3. The `ElasticsearchCluster` itself holds what belongs to this one cluster, and it overrides both.

A field set on the `ElasticsearchCluster` replaces the value of the layer below for that field. An object, a list, or a map is replaced as a whole. [Merge rules](elasticsearchclusterpreset.md#merge-rules) has the details. An edit of the preset or the release reaches every cluster that references it.

A preset rejects `version`. A cluster that follows a fleet version leaves `spec.version` unset and names a release. A cluster that must move before the fleet does sets `spec.version`, which wins over the release.

## Version

`spec.version` is the Elasticsearch version, as three segments. Camunda 8.9 supports Elasticsearch 8.x from 8.19, and 9.x from 9.2. A merged version below 8.19, or 9.0 or 9.1, gives `Ready: False` with reason `InvalidReference`.

```yaml
apiVersion: core.camunda.io/v1
kind: ElasticsearchCluster
metadata:
  name: my-cluster-es
  namespace: my-cluster-ns
spec:
  version: "9.2.8"
  # ... the rest of your cluster
```

## Storage

You can increase `spec.storageSize` at any time. The data volume never shrinks. The API server rejects a smaller inline value that replaces an inline value. A preset can still lower the size under a running cluster, and a first inline value can be smaller than the current size. The operator then keeps the current size and records a Warning event with reason `StorageShrinkIgnored`. To get a smaller volume, delete and recreate the cluster. With `whenDeleted: Retain`, also delete the kept volumes before you recreate it, because a cluster with the same name reattaches them.

The storage class of the data volumes cannot change while the ECK resource of the cluster exists. A new class can still come from the preset or from `spec.storageClassName`. The operator then keeps the class of the ECK resource, or no class when the ECK resource has none. It records the Warning event `StorageClassChangeIgnored` when it ignores a new requested class, also when the preset removes its class. A suspended cluster has no ECK resource, so it takes the new class when it resumes. New volumes get the new class, and the volumes that the suspension kept keep their old class. Volumes that `whenDeleted: Retain` kept from a deleted cluster of the same name keep their class.

## Snapshot repository

Set `spec.snapshotStorageRef` to an `ObjectStorageConfig` to take part in backups. Use the bucket that the `CamundaCluster` references in its `backupStorageRef`.

```yaml
apiVersion: core.camunda.io/v1
kind: ElasticsearchCluster
metadata:
  name: my-cluster-es
  namespace: my-cluster-ns
spec:
  snapshotStorageRef: "my-backup-bucket"
  # ... the rest of your cluster
```

The operator registers the snapshot repository `<namespace>.<name>` in Elasticsearch. The snapshots go under `<basePath>/<namespace>/<name>` in the bucket, where `<basePath>` comes from the `ObjectStorageConfig`. Because the name carries the namespace, two clusters of one name in two namespaces never share a repository. The operator gives the nodes the credentials or the workload identity of the bucket.

`SnapshotRepositoryReady` reports the registration. After it succeeds, `status.snapshotRepository` and the `snapshotRepository` field of the `SecondaryStorageConfig` carry the name. Until then, both are empty, and the `CamundaCluster` cannot take backups.

For an `AzureBlob` bucket, the endpoint must have the form `https://<account>.blob.<suffix>`, without a port or a path. Otherwise `Ready` reports `InvalidReference`.

## ServiceAccount

When `spec.serviceAccount` is set, or when the snapshot bucket uses workload identity, the Elasticsearch pods run under the ServiceAccount `spec.serviceAccount.name`, or `<name>-es` by default. Otherwise they run under the ServiceAccount `default` of the namespace. Unless `spec.serviceAccount.create` is `false`, the operator creates the named account. The account carries the annotations of the bucket identity, see [ObjectStorageConfig](objectstorageconfig.md#authentication). An annotation in `spec.serviceAccount.annotations` wins over the derived one.

```yaml
apiVersion: core.camunda.io/v1
kind: ElasticsearchCluster
metadata:
  name: my-cluster-es
  namespace: my-cluster-ns
spec:
  serviceAccount:
    annotations:
      eks.amazonaws.com/role-arn: "arn:aws:iam::123456789012:role/my-es-snapshot-role"
  # ... the rest of your cluster
```

While neither the cluster nor its preset sets `spec.serviceAccount` and the snapshot bucket uses no workload identity, the pods need no account. The operator then deletes `<name>-es` only when the cluster owns it, because the operator created it or took it over. A ServiceAccount `<name>-es` that you created stays.

When the cluster needs `<name>-es` and you created a ServiceAccount of that name, the operator takes it over. It adds its owner reference and its annotations. From then on, the operator deletes the account when the cluster no longer needs it, and when you delete the cluster. To keep an account of your own, give it another name, or set `create: false`.

The cloud reads the annotations of the ServiceAccount when a pod starts. So when the annotations that the operator puts on the account change, the Elasticsearch pods restart. This holds for an edit of `spec.serviceAccount.annotations`, and for a snapshot bucket that names another identity. The pod template then carries a new value of the annotation `camunda.io/service-account-hash`. Other changes of the spec, such as `spec.replicas`, keep this value.

With `serviceAccount.create: false`, the operator does not create or change the ServiceAccount, and it does not read its annotations. A change to the annotations of that account restarts no pod. After such a change, restart the pods yourself. Add or change a value in `spec.podAnnotations`, and ECK restarts the pods:

```yaml
apiVersion: core.camunda.io/v1
kind: ElasticsearchCluster
metadata:
  name: my-cluster-es
  namespace: my-cluster-ns
spec:
  serviceAccount:
    name: "platform-es"
    create: false
  podAnnotations:
    example.com/restarted-at: "2026-10-12T10:00:00Z"
  # ... the rest of your cluster
```

## Credentials

The operator generates the password of the user `camunda` once and keeps it. To rotate it, delete the Secret `<name>-es-user`. The operator then generates a new password and publishes it in a new Secret. The `CamundaCluster` that uses the contract restarts its pods with the new password.

## Monitoring

Elasticsearch serves no Prometheus endpoint itself. With monitoring on, the operator runs the Prometheus `elasticsearch_exporter` in the Deployment `<name>-es-exporter`. It also creates the ServiceMonitor `<name>-es-metrics` when the Kubernetes cluster serves that kind. `MetricsReady` reports the exporter, and it is not part of `Ready`.

```yaml
apiVersion: core.camunda.io/v1
kind: ElasticsearchCluster
metadata:
  name: my-cluster-es
  namespace: my-cluster-ns
spec:
  monitoring:
    serviceMonitor:
      enabled: true
      labels:
        release: prometheus
  # ... the rest of your cluster
```

## Suspend

With `spec.suspend: true`, the operator deletes the ECK resource and keeps the data volumes. `Ready` is `True` with reason `Suspended`. The exporter stops as well, and `MetricsReady` reports `Suspended`. When you set `spec.suspend` back to `false`, the operator recreates the resource and ECK reattaches the volumes.

## Deletion

Deletion removes everything the operator created: the ECK resource, the Secrets, the `SecondaryStorageConfig`, the exporter, and the ServiceAccount. A ServiceAccount with `create: false` stays, also when the operator created it before the switch. The data volumes obey `spec.persistentVolumeClaimRetentionPolicy.whenDeleted`. With `Delete`, ECK removes them with the cluster. With `Retain`, the volumes stay and a later cluster with the same name reattaches them.

## Status

`kubectl get elasticsearchcluster` shows `Ready`, its reason, the Elasticsearch version, and the age.

| Type | Reason | Meaning | What to do |
| --- | --- | --- | --- |
| `Ready` | `ECKNotInstalled` | The ECK CRDs were not installed when the operator started. The operator does not create the ECK resource, the Secrets, or the `SecondaryStorageConfig`. | Install ECK, then restart the operator. |
| `Ready` | `InvalidReference` | `spec.presetRef`, `spec.releaseRef`, or `spec.snapshotStorageRef` names a resource that does not exist, or the merged spec lacks `version`, `replicas`, or `storageSize`. Or the version is below the floor, the bucket has settings that Elasticsearch cannot use, or a ServiceAccount with `create: false` does not exist. | Read the message. Create the missing resource, or fix the field it names. |
| `Ready` | `MissingSecret` | A Secret or a key does not exist. The message names it. It is a Secret that the bucket of `spec.snapshotStorageRef` names, or a Secret that ECK creates with the cluster: `<name>-es-elastic-user` or `<name>-es-http-certs-public`. | For a Secret of the bucket, create it with the keys that the `ObjectStorageConfig` names. For a Secret of ECK, wait. ECK creates it when the cluster starts. |
| `Ready` | `Suspended` | `Ready` is `True`. The cluster is suspended by `spec.suspend: true`. The data volumes stay. | Nothing. To serve again, set `spec.suspend: false`. To wait for a serving cluster, require `Ready=True` and a reason other than `Suspended`. |
| `Ready` | `ConnectionFailed` | The components are healthy, but the snapshot repository is not registered. See `SnapshotRepositoryReady`. | Read the message of `SnapshotRepositoryReady`. Make sure that the bucket and its credentials are correct. The operator retries on its own. |
| `Ready` | component status | `Ready` is `True` only when every component is `True`. The reason comes from the component that is not ready, for example `Creating` or `Updating` (also yellow health), `Failing` (also red health), or `Error`. The [grace period](../architecture.md#status-conventions) of the cluster is 30 minutes by default. After it, a cluster that is not ready reports `Degraded` on yellow health, and `Down` on red or no health. The message names the component. | Wait while the reason is `Creating` or `Updating`. For other reasons, read the component condition and the ECK resource `<name>`. If yellow health stays, find the Warning event `IndexReplicasExceedNodes` on the `CamundaCluster` or `CamundaOptimize` that writes here. That consumer asks for more index replicas than the nodes can hold. |
| `CredentialsReady`, `KeystoreReady`, `ElasticsearchReady`, `StorageContractReady` | component status | The detail of each component that makes up `Ready`. `KeystoreReady` is `Disabled` unless the bucket needs keystore entries. | Read the message of the component that is not `True`. |
| `SnapshotRepositoryReady` | `Healthy` | The snapshot repository `<namespace>.<name>` is registered. The condition is absent when `spec.snapshotStorageRef` is unset. | Nothing. |
| `SnapshotRepositoryReady` | `ConnectionFailed` | Elasticsearch did not answer, or it rejected the registration. `Ready` is `False` while this holds. | Make sure that the bucket, its credentials, and the identity of the pods are correct. |
| `SnapshotRepositoryReady` | `MissingSecret` | The `elastic` user Secret or the CA Secret of ECK does not exist yet. | Wait. ECK creates them with the cluster. |
| `MetricsReady` | component status | The exporter. It is not part of `Ready`. It is `Disabled` while monitoring is off and `Suspended` while the cluster is suspended. When the exporter is still not ready at the end of its grace period, 30 minutes by default, the reason is `Degraded` or `Down`. | Read the exporter Deployment `<name>-es-exporter` when it is `Failing`, `Degraded`, or `Down`. |

```yaml
status:
  version: "9.2.4"
  snapshotRepository: my-cluster-ns.my-cluster-es
  volumes:
    - name: elasticsearch-data-my-cluster-es-es-default-0
      capacity: 64Gi
  conditions:
    - type: Ready
      status: "True"
      reason: Healthy
    - type: SnapshotRepositoryReady
      status: "True"
      reason: Healthy
      message: snapshot repository "my-cluster-ns.my-cluster-es" is registered
```

`status.version` is the Elasticsearch version that runs, from the cluster, the release, or the preset. It is empty until the operator resolves the references of the cluster.

`status.snapshotRepository` is the registered snapshot repository. It is empty without `spec.snapshotStorageRef`, and until the first registration succeeds.

`status.volumes` lists the bound data PersistentVolumeClaims, sorted by name, each with `name` and `capacity`. The claims can differ in size when a claim was resized outside the spec.

`status.observedGeneration` is the last generation that the operator reconciled.

## Fields

The [API reference](api-reference.md#elasticsearchcluster) lists every field of `ElasticsearchCluster`, with its type, whether it is required, its default, and its validation.

## Validation rules

- `spec.secondaryStorageConfig` is required.
- `spec.storageSize` cannot shrink. The API server rejects a value that is lower than the previous inline value.
- `spec.version` must have three segments (`9.2.4`, not `9.2`). The operator then requires Elasticsearch 8.19+ or 9.2+ on the merged spec.
- `spec.replicas` must be at least 1.
- `version`, `replicas`, and `storageSize` must be present after the merge. Set them inline, or take `replicas` and `storageSize` from a preset and `version` from a release. The operator enforces this rule, not the API server.
- `spec.secondaryStorageConfig`, `spec.snapshotStorageRef`, and `spec.serviceAccount.name` must be valid resource names.
- `spec.persistentVolumeClaimRetentionPolicy.whenDeleted` must be `Retain` or `Delete`.

## A production-shaped example

```yaml
apiVersion: core.camunda.io/v1
kind: ElasticsearchCluster
metadata:
  name: my-cluster-es
  namespace: my-cluster-ns
spec:
  version: "9.2.4"
  replicas: 3
  resources:
    requests: { cpu: "2", memory: "4Gi" }
    limits: { memory: "4Gi" }
  storageSize: "128Gi"
  storageClassName: "ssd"
  snapshotStorageRef: "my-backup-bucket"
  podLabels:
    team: platform
  scheduling:
    tolerations:
      - key: dedicated
        operator: Equal
        value: elasticsearch
        effect: NoSchedule
  secondaryStorageConfig: "my-storage-config"
  monitoring:
    serviceMonitor:
      enabled: true
      labels:
        release: prometheus
  persistentVolumeClaimRetentionPolicy:
    whenDeleted: Retain
```

## Related

- [ElasticsearchClusterPreset](elasticsearchclusterpreset.md): the baseline that `spec.presetRef` names.
- [CamundaRelease](camundarelease.md): the version that `spec.releaseRef` names.
- [ObjectStorageConfig](objectstorageconfig.md): the snapshot bucket that `spec.snapshotStorageRef` names.
- [SecondaryStorageConfig](secondarystorageconfig.md): the contract that this kind creates under `spec.secondaryStorageConfig`.
- [CamundaCluster](camundacluster.md): references the `SecondaryStorageConfig` through `storageRef`. It must reference the same `ObjectStorageConfig` through `backupStorageRef`.
- [Database](database.md): the other secondary storage kind. An orchestration cluster uses one or the other.
- [Secondary storage guide](../guides/secondary-storage.md): how to choose and connect secondary storage.
- [Backup guide](../guides/backup.md): how the snapshot repository takes part in backups.
- [Operations guide](../guides/operations.md): suspend, resize, and rotate credentials.
- [Getting started](../getting-started.md): the first cluster, end to end.
