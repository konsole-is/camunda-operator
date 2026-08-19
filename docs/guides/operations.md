# Operations

This guide covers the day-2 tasks of a running orchestration cluster: read its status, change its shape, suspend it, grow its storage, rotate its passwords, and delete it. It applies to `CamundaCluster`, and where it says so, to `ElasticsearchCluster` and `Database`.

## Read the status

The operator reports the state of a cluster in `status.conditions`. Read them with `kubectl describe`, or print the `Ready` condition alone:

```bash
kubectl describe camundacluster my-cluster -n my-cluster-ns
kubectl get camundacluster my-cluster -n my-cluster-ns -o jsonpath='{.status.conditions[?(@.type=="Ready")]}'
```

A `CamundaCluster` has one condition per process and one for each internal Secret: `ZeebeReady`, `GatewayReady`, `OperateReady`, `TasklistReady`, `AdminReady`, `ConnectorsReady`, `AdminSecretReady`, and `MirroredSecretsReady`. The `Ready` condition sums them up. It is `True` only when every condition that the cluster needs is `True`. Its reason and message name the condition that holds it back.

Every condition carries one of these reasons:

| Reason | Meaning | What to do |
| --- | --- | --- |
| `Healthy` | The workload runs with every replica ready. | Nothing. |
| `Creating` | The operator created the workload and waits for the first replicas. | Wait. |
| `Updating`, `Scaling` | The workload rolls out a new configuration or image, or changes its replica count. | Wait. |
| `Failing` | A replica does not become ready. | Read the pods of the workload. Look for restarts, failed probes, and resource limits. |
| `Degraded` | Some replicas are not ready after the grace period. | Read the pods of the workload. Look for restarts, failed probes, and resource limits. |
| `Down` | No replica is ready after the grace period. | Read the pods and their events. Make sure that secondary storage is reachable. |
| `Suspended` | `spec.suspend` is `true` and the workload is at zero replicas. | Nothing. Set `spec.suspend: false` to start the cluster again. |
| `Disabled` | The cluster does not need this component. | Nothing. This reason is not an error, and the condition stays out of `Ready`. |
| `InvalidReference` | A referenced resource does not exist, or the merged spec is not valid. The message names it. | Create the resource, or fix the field that the message names. |
| `MissingSecret` | A referenced Secret or one of its keys does not exist. The message names it. | Create the Secret with the keys that the reference names. |

`Disabled` shows on `GatewayReady` when the gateway is `Embedded`, on `OperateReady`, `TasklistReady`, and `AdminReady` when the web application is `Embedded`, on `ConnectorsReady` when connectors are off, on `AdminSecretReady` under OIDC, and on `MirroredSecretsReady` when every referenced Secret lives in the namespace of the cluster. `InvalidReference` and `MissingSecret` show on `Ready` only.

Three more status fields help you:

- `status.management` holds the address of the management API, for example `http://my-cluster-gateway.my-cluster-ns.svc:9600`, with the Camunda version and the number of partitions. The backup kinds read it. It is absent while the cluster is suspended.
- `status.volumes` lists every bound broker volume with its name and capacity.
- `status.observedGeneration` is the last generation of the spec that the operator reconciled. If it is lower than `metadata.generation`, the operator has not processed your last edit yet.

An `ElasticsearchCluster` uses the same reason vocabulary. Its `Ready` also reports `ConnectionFailed` when the snapshot repository cannot be registered. Read the [ElasticsearchCluster](../crds/elasticsearchcluster.md) page for its condition types.

## Change the topology

The gateway and each web application run `Standalone` or `Embedded`. You change the mode on the cluster, and the operator changes the workloads:

```bash
kubectl patch camundacluster my-cluster -n my-cluster-ns --type=merge -p '{"spec":{"operate":{"mode":"Standalone"}}}'
```

When you set `operate.mode: Standalone`, the Deployment and the Service `my-cluster-operate` appear, and `OperateReady` reports `Creating` and then `Healthy`. When you set it back to `Embedded`, the operator deletes both, `OperateReady` reads `Disabled`, and the gateway serves Operate again. The same applies to `tasklist`, `admin`, and `gateway`. An embedded web application runs inside the gateway when the gateway is standalone, otherwise inside the brokers.

The Services stay stable, so clients find the cluster this way:

| Service | Ports | Serves |
| --- | --- | --- |
| `<name>-gateway`, or `<name>-zeebe` when the gateway is `Embedded` | `26500` gRPC, `8080` HTTP | The gRPC API, the REST API under `/v2/`, and the embedded web applications under `/operate/`, `/tasklist/`, and `/admin/`. |
| `<name>-operate`, `<name>-tasklist`, `<name>-admin` | `8080` | The standalone web application. |
| `<name>-connectors` | `8080` | The connectors runtime. |
| Every Service of a unified process | `9600` | The management API: health, metrics, and backups. |

## Suspend and resume

Set `spec.suspend: true` to stop a cluster and keep its data:

```bash
kubectl patch camundacluster my-cluster -n my-cluster-ns --type=merge -p '{"spec":{"suspend":true}}'
```

The operator scales every workload to zero replicas and keeps the broker volumes. `Ready` stays `True` with reason `Suspended`, and `status.management` disappears. A backup of a suspended cluster waits.

Set `spec.suspend: false` to start the cluster again. `Ready` reads `Updating` while the pods start, then `Healthy`. The brokers attach to the same volumes, and the process instances from before the suspension are still there.

An `ElasticsearchCluster` suspends the same way. The operator deletes the ECK `Elasticsearch` resource and keeps the data volumes. `Ready` is `True` with reason `Suspended`, and `MetricsReady` reports `Suspended` too. On resume the operator recreates the resource, and ECK attaches the same volumes with the data intact.

`spec.pause: true` is different. The operator stops reconciling the resource. It writes nothing, not even status, and records one `Paused` event per reconcile. The workloads keep running as they are. Use `suspend` to save compute and keep the data. Use `pause` when you must stop the operator from touching the resource, for example while you inspect or repair a workload by hand.

## Grow storage

Increase `spec.zeebe.storageSize` to give the brokers more disk:

```bash
kubectl patch camundacluster my-cluster -n my-cluster-ns --type=merge -p '{"spec":{"zeebe":{"storageSize":"64Gi"}}}'
```

The operator expands every bound broker volume in place, without a restart. The storage class must allow volume expansion. If it does not, the API server rejects the expansion and the operator retries with backoff. A volume of a new replica is expanded after it binds. `status.volumes` shows the result per volume.

A smaller value is rejected at admission. If a preset lowers the size under a running cluster, the operator ignores it, keeps the current size, and records the Warning event `StorageShrinkIgnored` once per requested size. To get a smaller volume, delete and recreate the cluster.

`spec.storageSize` of an `ElasticsearchCluster` obeys the same rules: it grows in place, a smaller inline value is rejected, and a smaller preset value is ignored with `StorageShrinkIgnored`.

## Rotate passwords

The operator generates each password once and keeps it stable. To rotate one, delete its Secret. The operator generates a new password on the next reconcile and publishes it in a new Secret. The old password is never published again.

| Password | Secret to delete | What happens next |
| --- | --- | --- |
| The Elasticsearch user of an `ElasticsearchCluster` | `<name>-es-user` | ECK updates the user. Every `CamundaCluster` that references the `SecondaryStorageConfig` of this Elasticsearch rolls its pods. |
| A role of a `Database` | The application or backup credential Secret that the `Database` created | The operator sets the new password on the server before it publishes the Secret. Every `CamundaCluster` that references the `DatabaseConfig` rolls its pods. |
| The admin user of a basic-authentication cluster | None. Set `spec.auth.basic.passwordRotation` instead; see below. | The operator sets the new password on the `admin` user and rolls the connectors Deployment. |

```bash
kubectl delete secret my-cluster-es-es-user -n my-cluster-ns
```

> **Caution:** Between the deletion and the roll, a client with the old password is rejected. Plan the rotation outside of peak hours.

The admin user is different. The orchestration cluster creates the `admin` user once, at first start, and a new Secret alone does not change its password. Rotate it through the spec:

```yaml
spec:
  auth:
    basic:
      passwordRotation: "2026-08"
```

A changed value rotates once. The operator generates a new password, sets it on the `admin` user through the user API of the running cluster, publishes it in `<name>-camunda-admin`, and rolls the connectors Deployment. `status.adminPassword.rotation` shows the value when the rotation is complete. A failed rotation surfaces on the condition `AdminSecretReady` and retries; the [authentication guide](authentication.md#rotate-the-password) has the failure modes.

## Change configuration and referenced Secrets

You do not edit the cluster to roll a configuration change. A change to the `CamundaPlatformConfig`, the `CamundaClusterPreset`, the `SecondaryStorageConfig` and its `DatabaseConfig` or `DatabaseServerConfig`, an `ObjectStorageConfig`, or any referenced Secret reaches the pods on its own. The pod templates carry the annotation `camunda.io/config-hash`, and a new hash rolls the pods.

A referenced Secret in another namespace is copied into the namespace of the cluster as `<name>-camunda-<purpose>`, for example `my-cluster-camunda-license` or `my-cluster-camunda-oidc-client`. The pods read the copy. When the source Secret changes, the copy follows, and the pods roll. `MirroredSecretsReady` reports the copies.

To add your own environment variables, use `extraEnv` and `extraEnvFrom`. The operator renders its own configuration first, then the top-level `extraEnv`, then the `extraEnv` of the embedded components that the process hosts, then the `extraEnv` of the process itself. A later entry with the same name wins, and an entry replaces an operator entry with the same name. For example, to set the heap of the brokers:

```yaml
spec:
  zeebe:
    extraEnv:
      - name: JAVA_TOOL_OPTIONS
        value: "-XX:+ExitOnOutOfMemoryError -Xmx4g"
```

Keep `-XX:+ExitOnOutOfMemoryError` in the value. The operator sets it, and your entry replaces the whole value.

## Monitor

Set `spec.monitoring.serviceMonitor.enabled: true` to let a Prometheus operator scrape the cluster. The operator creates one ServiceMonitor per process, named like the workload. It scrapes `/actuator/prometheus` on port `9600` of a unified process and on port `8080` of connectors. On a Kubernetes cluster without the `ServiceMonitor` kind, the operator creates none and reports no error.

An `ElasticsearchCluster` with the same flag runs the Prometheus `elasticsearch_exporter` next to the cluster and a ServiceMonitor for it. The exporter reports its state in the `MetricsReady` condition. This condition is not part of `Ready`, so a broken exporter never marks the cluster not ready.

## Find the resources of a cluster

Every resource that the operator creates carries labels that name its owner and its role:

| Label | Value |
| --- | --- |
| `camunda.io/cluster` | The name of the owning `CamundaCluster`. |
| `camunda.io/elasticsearch-cluster` | The name of the owning `ElasticsearchCluster`. |
| `camunda.io/database` | The name of the owning `Database`. |
| `camunda.io/component` | The role: `zeebe`, `gateway`, `operate`, `tasklist`, `admin`, `connectors`, `elasticsearch`, or `elasticsearch-exporter`. |
| `app.kubernetes.io/managed-by` | `camunda-operator`. The pods and volume claims carry the two labels above, not this one. |

Two examples:

```bash
# Every pod of one cluster.
kubectl get pod -n my-cluster-ns -l camunda.io/cluster=my-cluster

# Every workload, Service, Secret, and ServiceMonitor of one cluster.
kubectl get all,secret,servicemonitor -n my-cluster-ns -l camunda.io/cluster=my-cluster

# The broker pods and their volumes.
kubectl get pod,pvc -n my-cluster-ns -l camunda.io/cluster=my-cluster,camunda.io/component=zeebe
```

## Delete a cluster

```bash
kubectl delete camundacluster my-cluster -n my-cluster-ns
```

Kubernetes garbage-collects everything that the cluster owns: the workloads, the Services, the ServiceAccount, the admin Secret, the copied Secrets, and the ServiceMonitors. The broker volumes follow `spec.zeebe.persistentVolumeClaimRetentionPolicy.whenDeleted`. With `Delete`, the default, they go with the cluster. With `Retain`, they stay, and a later cluster with the same name attaches them again.

The `ElasticsearchCluster` and the `Database` are separate resources with their own lifecycle. Deleting the `CamundaCluster` leaves them in place. Deleting an `ElasticsearchCluster` removes the ECK resource, its Secrets, and its `SecondaryStorageConfig`, and its data volumes follow its own `persistentVolumeClaimRetentionPolicy`. Deleting a `Database` removes its `DatabaseConfig`, its `SecondaryStorageConfig`, and its credential Secrets, but it never drops the logical database or the SQL roles. Data removal on the server is a manual act.

## Related

- [CamundaCluster](../crds/camundacluster.md): every field, condition, and validation rule of the orchestration cluster.
- [ElasticsearchCluster](../crds/elasticsearchcluster.md): the Elasticsearch secondary storage, its conditions, and its snapshot repository.
- [Database](../crds/database.md): the logical database and its credential Secrets.
- [CamundaClusterPreset](../crds/camundaclusterpreset.md): the baseline that a cluster inherits, and how a preset change reaches every cluster.
