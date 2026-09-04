# Operations

This guide covers the day-2 tasks of a running orchestration cluster: read its status, change its shape, suspend it, grow its storage, rotate its passwords, and delete it. It applies to `CamundaCluster`, and where it says so, to `ElasticsearchCluster` and `Database`.

## Read the status

`kubectl get` shows whether a cluster is ready, why, and the Camunda version it runs:

```bash
kubectl get camundacluster -n my-cluster-ns
```

```
NAME         READY   REASON    VERSION   AGE
my-cluster   True    Healthy   8.9.9     12m
```

The operator reports the state of each component in `status.conditions`. Read the status to see them:

```bash
kubectl get camundacluster my-cluster -n my-cluster-ns -o yaml
```

The status of a healthy cluster with the default topology looks like this:

```yaml
status:
  observedGeneration: 3
  serviceAccountName: my-cluster-camunda
  management:
    endpoint: http://my-cluster-zeebe.my-cluster-ns.svc:9600
    auth:
      method: none
    version: "8.9.9"
    partitions: 1
  volumes:
    - name: data-my-cluster-zeebe-0
      capacity: 10Gi
  conditions:
    - type: Ready
      status: "True"
      reason: Healthy
      message: "zeebe: All resources healthy."
    - type: ZeebeReady
      status: "True"
      reason: Healthy
      message: All resources healthy.
    - type: GatewayReady
      status: "True"
      reason: Disabled
      message: Component is disabled.
    - type: OperateReady
      status: "True"
      reason: Disabled
      message: Component is disabled.
    - type: TasklistReady
      status: "True"
      reason: Disabled
      message: Component is disabled.
    - type: AdminReady
      status: "True"
      reason: Disabled
      message: Component is disabled.
    - type: ConnectorsReady
      status: "True"
      reason: Disabled
      message: Component is disabled.
    - type: AdminSecretReady
      status: "True"
      reason: Healthy
      message: All resources healthy.
    - type: MirroredSecretsReady
      status: "True"
      reason: Disabled
      message: Component is disabled.
```

A `CamundaCluster` has one condition per process and one for each internal Secret. The `Ready` condition sums them up. It is `True` only when every condition that the cluster needs is `True`. Its reason and message name the condition that holds it back. While the brokers of this cluster start, `Ready` reads:

```yaml
    - type: Ready
      status: "False"
      reason: Creating
      message: "zeebe: Waiting for replicas: 0/1 ready"
    - type: ZeebeReady
      status: "False"
      reason: Creating
      message: "Waiting for replicas: 0/1 ready"
```

And when `storageRef` names a `SecondaryStorageConfig` that does not exist:

```yaml
    - type: Ready
      status: "False"
      reason: InvalidReference
      message: SecondaryStorageConfig "my-cluster-ns/my-storage-config" not found
```

To print one condition without the rest:

```bash
kubectl get camundacluster my-cluster -n my-cluster-ns -o jsonpath='{.status.conditions[?(@.type=="Ready")]}'
```

Every condition carries one of these reasons:

| Reason | Meaning | What to do |
| --- | --- | --- |
| `Healthy` | The workload runs with every replica ready. | Nothing. |
| `Creating` | The operator created the workload and waits for the first replicas. | Wait. |
| `Updating`, `Scaling` | The workload rolls out a new configuration or image, or changes its replica count. | Wait. |
| `Failing` | A replica does not become ready. | Read the pods of the workload. Look for restarts, failed probes, and resource limits. |
| `Degraded` | Some replicas are not ready after the grace period. | Read the pods of the workload. Look for restarts, failed probes, and resource limits. |
| `Down` | No replica is ready after the grace period. | Read the pods and their events. Make sure that secondary storage is reachable. |
| `Suspended` | `spec.suspend` is `true` and the workload is at zero replicas. | Find out what suspended the cluster before you start it again. See "Suspend and resume". |
| `Disabled` | The cluster does not need this component. | Nothing. This reason is not an error, and the condition stays out of `Ready`. |
| `InvalidReference` | A referenced resource does not exist, or the merged spec is not valid. The message names it. | Create the resource, or fix the field that the message names. |
| `MissingSecret` | A referenced Secret or one of its keys does not exist. The message names it. | Create the Secret with the keys that the reference names. |

`Disabled` shows on `GatewayReady` when the gateway is `Embedded`, on `OperateReady`, `TasklistReady`, and `AdminReady` when the web application is `Embedded`, on `ConnectorsReady` when connectors are off, on `AdminSecretReady` under OIDC, and on `MirroredSecretsReady` when every referenced Secret lives in the namespace of the cluster. `InvalidReference` and `MissingSecret` show on `Ready` only.

The other status fields in the example above:

- `management` holds the address of the management API, with the Camunda version and the number of partitions. The backup kinds read it. It is absent while the cluster is suspended.
- `volumes` lists every bound broker volume with its name and capacity.
- `serviceAccountName` is the ServiceAccount that the pods run under.
- `observedGeneration` is the last generation of the spec that the operator reconciled. If it is lower than `metadata.generation`, the operator has not processed your last edit yet.

An `ElasticsearchCluster` uses the same reason vocabulary. Its `Ready` also reports `ConnectionFailed` when the snapshot repository cannot be registered. Read the [ElasticsearchCluster](../crds/elasticsearchcluster.md) page for its condition types.

## Change the topology

The gateway and each web application run `Standalone` or `Embedded`. You change the mode on the cluster, and the operator changes the workloads. A cluster with a standalone gateway and a standalone Operate:

```yaml
apiVersion: core.camunda.io/v1
kind: CamundaCluster
metadata:
  name: my-cluster
  namespace: my-cluster-ns
spec:
  # ... version, platformConfigRef, storageRef, and the rest of your cluster
  gateway:
    mode: Standalone
    replicas: 2
  operate:
    mode: Standalone
    replicas: 2
```

When Operate becomes `Standalone`, the Deployment and the Service `my-cluster-operate` appear, and `OperateReady` reports `Creating` and then `Healthy`:

```yaml
    - type: OperateReady
      status: "False"
      reason: Creating
      message: "Waiting for replicas: 0/2 ready"
```

When you set it back to `Embedded`, the operator deletes both, `OperateReady` reads `Disabled`, and the gateway serves Operate again. The same applies to `tasklist`, `admin`, and `gateway`. An embedded web application runs inside the gateway when the gateway is standalone, otherwise inside the brokers.

The Services stay stable, so clients find the cluster this way:

| Service | Ports | Serves |
| --- | --- | --- |
| `<name>-gateway`, or `<name>-zeebe` when the gateway is `Embedded` | `26500` gRPC, `8080` HTTP | The gRPC API, the REST API under `/v2/`, and the embedded web applications under `/operate/`, `/tasklist/`, and `/admin/`. |
| `<name>-operate`, `<name>-tasklist`, `<name>-admin` | `8080` | The standalone web application. |
| `<name>-connectors` | `8080` | The connectors runtime. |
| Every Service of a unified process | `9600` | The management API: health, metrics, and backups. |

## Suspend and resume

To stop a cluster and keep its data, suspend it:

```yaml
apiVersion: core.camunda.io/v1
kind: CamundaCluster
metadata:
  name: my-cluster
  namespace: my-cluster-ns
spec:
  # ... the rest of your cluster
  suspend: true
```

Or, as a one-liner:

```bash
kubectl patch camundacluster my-cluster -n my-cluster-ns --type=merge -p '{"spec":{"suspend":true}}'
```

The operator scales every workload to zero replicas and keeps the broker volumes. `Ready` stays `True` with reason `Suspended`, and `management` disappears from the status. A backup of a suspended cluster waits.

```yaml
status:
  volumes:
    - name: data-my-cluster-zeebe-0
      capacity: 10Gi
  conditions:
    - type: Ready
      status: "True"
      reason: Suspended
      message: "zeebe: StatefulSet scaled to zero"
```

Set `suspend: false` to start the cluster again. `Ready` reads `Updating` while the pods start, then `Healthy`. The brokers attach to the same volumes, and the process instances from before the suspension are still there.

### A restore suspends the cluster too

A restore of the cluster suspends it and gives that suspension back when it reaches `Completed`. You do not suspend the cluster for a restore, and you do not start it again afterwards.

**Find out what suspended the cluster before you set `suspend: false`.** A restore that failed leaves the cluster suspended on purpose, and so does a restore that somebody deleted while it ran. Its broker volumes can be empty or half written. Brokers that start over such volumes are worse than a cluster that is down.

Two reads answer it. The restores say whether one of them suspended the cluster, and in which phase it stopped:

```bash
kubectl get pitr,lres,lrrdbms -n my-cluster-ns \
  -o custom-columns='NAME:.metadata.name,PHASE:.status.phase,SUSPENDED:.status.clusterSuspended'
```

The cluster says which manager owns the field:

```bash
kubectl get camundacluster my-cluster -n my-cluster-ns \
  -o jsonpath='{.metadata.managedFields[*].manager}'
```

`camunda-operator/restore-suspend` among those managers means that a restore applied the suspension. A restore that reaches `Completed` gives it back on its own, so wait for that. A restore that failed keeps it, and the cluster is yours to start again once you know what its volumes hold.

CAUTION: A merge patch of `spec.suspend` takes the field from the restore that owns it. A restore that is still running suspends the cluster again at once, and the patch achieves nothing. A restore that already failed leaves the cluster to you, and the volumes hold what the restore wrote before it stopped.

The restore pages hold the rules: [LogicalRestoreElasticsearch](../crds/logicalrestoreelasticsearch.md), [LogicalRestoreRDBMS](../crds/logicalrestorerdbms.md), and [PointInTimeRestore](../crds/pointintimerestore.md).

### A point-in-time restore restarts the cluster on a new server

A point-in-time restore on a [DatabaseServer](../crds/databaseserver.md) rolls the server back to the point you asked for. The rollback replaces the PostgreSQL server with a new one built from the archive, under a new name. The published `DatabaseServerConfig` moves to it, so the address of the database changes, and every `CamundaCluster` that reads that contract rolls its pods. Plan the restore as a restart of the whole cluster, not of the database alone.

Read the new address from the server and the contract when the restore is done:

```bash
kubectl get databaseserver my-db -n my-cluster-ns \
  -o custom-columns='CLUSTER:.status.cluster,IDENTIFIER:.status.systemIdentifier'
kubectl get databaseserverconfig my-db-server -n my-cluster-ns -o jsonpath='{.spec.host}'
```

The archive the rollback read stays in the bucket, next to the archive the new server writes. Nothing removes it. `retentionPeriodDays` applies only to the archive the server writes now, so the replaced archive stays in the bucket until you remove it. `status.archive.history` on the `DatabaseServer` keeps its record with an end time. A later restore reaches a point from before the rollback while that point lies within `retentionPeriodDays` of now. A restore of an older point holds with reason `PitrUnavailable`, whatever the bucket still holds. No restore can reach the window between the two archives, because the archive of the new server starts at its first base backup.

An `ElasticsearchCluster` suspends the same way. The operator deletes the ECK `Elasticsearch` resource and keeps the data volumes. `Ready` is `True` with reason `Suspended`, and `MetricsReady` reports `Suspended` too. On resume the operator recreates the resource, and ECK attaches the same volumes with the data intact.

A `DatabaseServer` suspends the same way. The instance pods go, the data volumes stay, and the instances come back on those volumes on resume. A suspended server refuses a rollback request, so unsuspend it before you create a point-in-time restore.

`pause: true` is different:

```yaml
apiVersion: core.camunda.io/v1
kind: CamundaCluster
metadata:
  name: my-cluster
  namespace: my-cluster-ns
spec:
  # ... the rest of your cluster
  pause: true
```

The operator changes nothing for this resource. It writes no status, and it records a `Paused` event each time it looks at the resource. The workloads keep running as they are. Use `suspend` to save compute and keep the data. Use `pause` when you must stop the operator from touching the resource, for example while you inspect or repair a workload by hand.

## Grow storage

Increase `zeebe.storageSize` to give the brokers more disk:

```yaml
apiVersion: core.camunda.io/v1
kind: CamundaCluster
metadata:
  name: my-cluster
  namespace: my-cluster-ns
spec:
  # ... the rest of your cluster
  zeebe:
    storageSize: 64Gi
```

The operator expands every bound broker volume in place, without a restart. The storage class must allow volume expansion. If it does not, the API server rejects the expansion, and the operator tries again. A volume of a new replica is expanded after it binds. `status.volumes` shows the result per volume:

```yaml
status:
  volumes:
    - name: data-my-cluster-zeebe-0
      capacity: 64Gi
    - name: data-my-cluster-zeebe-1
      capacity: 64Gi
    - name: data-my-cluster-zeebe-2
      capacity: 32Gi   # not expanded yet
```

The API server rejects a smaller value. If a preset lowers the size under a running cluster, the operator ignores it, keeps the current size, and records the Warning event `StorageShrinkIgnored` once per requested size. To get a smaller volume, delete and recreate the cluster.

`storageSize` of an `ElasticsearchCluster`, and `storageSize` and `walStorageSize` of a `DatabaseServer`, obey the same rules: they grow in place, a smaller inline value is rejected, and a smaller preset value is ignored with `StorageShrinkIgnored`.

## Rotate passwords

The operator generates each password once and keeps it stable. To rotate one, delete its Secret. The operator then generates a new password and publishes it in a new Secret. The old password is never published again. The admin user of a basic-authentication cluster is the exception: it rotates through the spec, and a deletion does not change it on the cluster.

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
apiVersion: core.camunda.io/v1
kind: CamundaCluster
metadata:
  name: my-cluster
  namespace: my-cluster-ns
spec:
  # ... the rest of your cluster
  auth:
    basic:
      passwordRotation: "2026-08"
```

A changed value rotates once. The operator generates a new password, sets it on the `admin` user through the user API of the running cluster, publishes it in `<name>-camunda-admin`, and rolls the connectors Deployment. `status.adminPassword.rotation` shows the value when the rotation is complete. A failed rotation shows on the condition `AdminSecretReady`, and the operator tries again. The [authentication guide](authentication.md#rotate-the-password) has the failure modes.

## Change configuration and referenced Secrets

You do not edit the cluster to roll a configuration change. A change to the `CamundaPlatformConfig`, the `CamundaClusterPreset`, the `CamundaRelease`, the `SecondaryStorageConfig` and its `DatabaseConfig` or `DatabaseServerConfig`, an `ObjectStorageConfig`, or any referenced Secret reaches the pods on its own. The pod templates carry the annotation `camunda.io/config-hash`, and a new hash rolls the pods.

The [CamundaPlatformConfig](../crds/camundaplatformconfig.md) is cluster-scoped, so the Secrets it names are copied into the namespace of the cluster as `<name>-camunda-<purpose>`, for example `my-cluster-camunda-license` or `my-cluster-camunda-oidc-client`. The pods read the copy. When the source Secret changes, the copy follows, and the pods roll. `MirroredSecretsReady` reports the copies. Every other Secret a cluster reads already lives in its namespace.

To add your own environment variables, use `extraEnv` and `extraEnvFrom`. The operator writes its own configuration first, then the top-level `extraEnv`, then the `extraEnv` of the embedded parts that the process hosts, then the `extraEnv` of the process itself. A later entry with the same name wins, and an entry replaces an operator entry with the same name. For example, to set the heap of the brokers:

```yaml
apiVersion: core.camunda.io/v1
kind: CamundaCluster
metadata:
  name: my-cluster
  namespace: my-cluster-ns
spec:
  # ... the rest of your cluster
  zeebe:
    extraEnv:
      - name: JAVA_TOOL_OPTIONS
        value: "-XX:+ExitOnOutOfMemoryError -Xmx4g"
```

Keep `-XX:+ExitOnOutOfMemoryError` in the value. The operator sets it, and your entry replaces the whole value.

## Monitor

To let a Prometheus operator scrape the cluster, turn on the ServiceMonitors:

```yaml
apiVersion: core.camunda.io/v1
kind: CamundaCluster
metadata:
  name: my-cluster
  namespace: my-cluster-ns
spec:
  # ... the rest of your cluster
  monitoring:
    serviceMonitor:
      enabled: true
```

The operator creates one ServiceMonitor per process, named like the workload. It scrapes `/actuator/prometheus` on port `9600` of a unified process and on port `8080` of connectors. On a Kubernetes cluster without the `ServiceMonitor` kind, the operator creates none and reports no error.

An `ElasticsearchCluster` with the same block runs the Prometheus `elasticsearch_exporter` next to the cluster and a ServiceMonitor for it. The exporter reports its state in the `MetricsReady` condition. This condition is not part of `Ready`, so a broken exporter never marks the cluster not ready.

```yaml
apiVersion: core.camunda.io/v1
kind: ElasticsearchCluster
metadata:
  name: my-cluster-es
  namespace: my-cluster-ns
spec:
  # ... version, replicas, storageSize, secondaryStorageConfig
  monitoring:
    serviceMonitor:
      enabled: true
```

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

Kubernetes garbage-collects everything that the cluster owns: the workloads, the Services, the ServiceAccount, the admin Secret, the copied Secrets, and the ServiceMonitors. The broker volumes follow the retention policy of the cluster. With `Delete`, the default, they go with the cluster. With `Retain`, they stay, and a later cluster with the same name attaches them again. Set the policy before you delete:

```yaml
apiVersion: core.camunda.io/v1
kind: CamundaCluster
metadata:
  name: my-cluster
  namespace: my-cluster-ns
spec:
  # ... the rest of your cluster
  zeebe:
    persistentVolumeClaimRetentionPolicy:
      whenDeleted: Retain
```

A restore of the cluster that reached `Failed` **after it started the restore application** keeps its per-broker Jobs, and the pods of those Jobs hold the broker volumes. The delete of the cluster then waits on a volume that never terminates. Delete that restore first. `status.primaryJobNames` on the restore tells you which case you are in: a restore that failed in an earlier phase names no Job there and holds nothing, and a restore that reached `Completed` already removed the Jobs it names, together with their pods.

The `ElasticsearchCluster`, the `DatabaseServer`, and the `Database` are separate resources with their own lifecycle. Deleting the `CamundaCluster` leaves them in place. Deleting an `ElasticsearchCluster` removes the ECK resource, its Secrets, and its `SecondaryStorageConfig`, and its data volumes follow its own `persistentVolumeClaimRetentionPolicy`. Deleting a `Database` removes its `DatabaseConfig`, its `SecondaryStorageConfig`, and its credential Secrets, but it never drops the logical database or the SQL roles. Data removal on the server is a manual act.

**CAUTION: Deleting a `DatabaseServer` removes its PostgreSQL instances and their data volumes.** The archive in the bucket stays, and nothing reads it once the server is gone. Take a [logical backup](./backup.md) before you delete a server whose data you still need.

## Related

- [CamundaCluster](../crds/camundacluster.md): every field, condition, and validation rule of the orchestration cluster.
- [ElasticsearchCluster](../crds/elasticsearchcluster.md): the Elasticsearch secondary storage, its conditions, and its snapshot repository.
- [DatabaseServer](../crds/databaseserver.md): the PostgreSQL server, its archive, and the rollback it performs.
- [Database](../crds/database.md): the logical database and its credential Secrets.
- [CamundaClusterPreset](../crds/camundaclusterpreset.md): the baseline that a cluster inherits, and how a preset change reaches every cluster.
