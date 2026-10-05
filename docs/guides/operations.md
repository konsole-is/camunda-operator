# Operations

This guide covers the day-2 tasks of a running orchestration cluster. It shows how to read its status, change its shape, suspend it, restore it, grow its storage, rotate its passwords, and delete it. It applies to `CamundaCluster`, and where it says so, to `ElasticsearchCluster` and `Database`.

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

The status of a healthy cluster with the default topology and basic authentication looks like this:

```yaml
status:
  observedGeneration: 3
  management:
    endpoint: http://my-cluster-gateway.my-cluster-ns.svc:9600
    auth:
      method: none
    version: "8.9.9"
    partitions: 1
  gateway:
    grpcEndpoint: my-cluster-gateway.my-cluster-ns.svc:26500
    restEndpoint: http://my-cluster-gateway.my-cluster-ns.svc:8080
  volumes:
    - name: data-my-cluster-zeebe-0
      capacity: 10Gi
  conditions:
    - type: Ready
      status: "True"
      reason: Healthy
      message: "admin-secret: Component is healthy."
    - type: ZeebeReady
      status: "True"
      reason: Healthy
      message: Component is healthy.
    - type: GatewayReady
      status: "True"
      reason: Healthy
      message: Component is healthy.
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
      message: Component is healthy.
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

The reasons that you see most often:

| Reason | Meaning | What to do |
| --- | --- | --- |
| `Healthy` | The workload runs with every replica ready. | Nothing. |
| `Creating` | The operator created the workload and waits for the first replicas. | Wait. |
| `Updating`, `Scaling` | The workload rolls out a new configuration or image, or changes its replica count. | Wait. If the reason stays, read the pods and their events. Make sure that secondary storage is reachable. |
| `Failing` | A replica does not become ready. | Read the pods of the workload. Look for restarts, failed probes, and resource limits. |
| `Degraded` | The workload is still not ready at the end of its [grace period](../architecture.md#status-conventions), but some replicas are ready. | Read the pods of the workload and their events. |
| `Down` | The workload is still not ready at the end of its grace period, and no replica is ready. | Read the pods of the workload and their events. Make sure that secondary storage is reachable. |
| `Suspended` | The workload is at zero replicas, because of `spec.suspend: true`, a suspension hold, or a wait for the backend. | Find out what suspended the cluster before you start it again. See [Suspend and resume](#suspend-and-resume). |
| `Disabled` | The cluster does not need this component. | Nothing. This reason is not an error, and the condition stays out of `Ready`. |
| `InvalidReference` | A referenced resource does not exist, or the merged spec is not valid. The message names it. | Create the resource, or fix the field that the message names. |
| `MissingSecret` | A referenced Secret or one of its keys does not exist. The message names it. | Create the Secret with the keys that the reference names. |

`Ready` can also report `SuspensionHeld`, `StorageAlreadyAttached`, `WaitingForHandover`, or `VersionDowngradeRefused`. The [CamundaCluster](../crds/camundacluster.md#status) page lists every reason.

`Disabled` shows on these conditions:

- `GatewayReady`, when the gateway is `Embedded`.
- `OperateReady`, `TasklistReady`, and `AdminReady`, when the web application is `Embedded`. This is the default.
- `ConnectorsReady`, when connectors are off.
- `AdminSecretReady`, under OIDC.
- `MirroredSecretsReady`, when every referenced Secret lives in the namespace of the cluster.

The other status fields in the example above:

- `management` holds the address of the management API, with the Camunda version and the number of partitions. The backup kinds read it.
- `gateway` holds the addresses of the gRPC API and the REST API.
- `management` and `gateway` are absent while the cluster is suspended, and the `VERSION` column is then empty.
- `adminPassword.rotation` records the last admin password rotation, under basic authentication.
- `volumes` lists every bound broker volume with its name and capacity.
- `observedGeneration` is the last generation of the spec that the operator processed. If it is lower than `metadata.generation`, the operator has not processed your last edit yet.

When you set `spec.serviceAccount`, or a bucket uses workload identity, `serviceAccountName` names the ServiceAccount that the pods run under.

An `ElasticsearchCluster` uses the same reason vocabulary. Read the [ElasticsearchCluster](../crds/elasticsearchcluster.md) page for its condition types.

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

The operator scales every workload to zero replicas and keeps the broker volumes. `Ready` stays `True` with reason `Suspended`, and `management` and `gateway` disappear from the status. A backup of a suspended cluster waits.

```yaml
status:
  volumes:
    - name: data-my-cluster-zeebe-0
      capacity: 10Gi
  conditions:
    - type: Ready
      status: "True"
      reason: Suspended
      message: "zeebe: All resources are suspended."
```

Set `suspend: false` to start the cluster again. `Ready` reads `Updating` while the pods start, then `Healthy`. The brokers attach to the same volumes, and the process instances from before the suspension are still there.

An `ElasticsearchCluster` suspends the same way. The operator deletes the ECK `Elasticsearch` resource and keeps the data volumes. `Ready` is `True` with reason `Suspended`. When monitoring is on, `MetricsReady` reports `Suspended` too. On resume the operator recreates the resource, and ECK attaches the same volumes with the data intact.

A `DatabaseServer` suspends the same way. The instance pods go, the data volumes stay, and the instances come back on those volumes on resume. A suspended server refuses a rollback request, so unsuspend it before you create a point-in-time restore.

A restore suspends the cluster too. Before you set `suspend: false`, read [Find out what suspended the cluster](#find-out-what-suspended-the-cluster).

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

The operator changes nothing for this resource. It writes no status, and it records a `Paused` event each time it looks at the resource. A delete still goes through. The workloads keep running as they are. Use `suspend` to save compute and keep the data. Use `pause` when you must stop the operator from touching the resource. For example, pause it while you inspect or repair a workload by hand.

## Restore a cluster

Three kinds restore a cluster. Each resource is one restore. Its spec is immutable, and it runs once. To retry, create a new resource.

| Kind | Short name | Restores from | Secondary storage of the cluster |
| --- | --- | --- | --- |
| [LogicalRestoreElasticsearch](../crds/logicalrestoreelasticsearch.md) | `lres` | A completed `LogicalBackupElasticsearch` of the same cluster | Elasticsearch |
| [LogicalRestoreRDBMS](../crds/logicalrestorerdbms.md) | `lrrdbms` | A completed `LogicalBackupRDBMS` of the same cluster | PostgreSQL |
| [PointInTimeRestore](../crds/pointintimerestore.md) | `pitr` | The continuous backups of the cluster, at a timestamp | PostgreSQL |

A logical restore writes into the cluster that its backup was taken from, and into no other cluster. This section holds what the three kinds share. The page of each kind holds its own rules, phases, and conditions.

### What a restore writes on the cluster

You do not suspend the cluster before a restore. The restore does it:

- It puts a [suspension hold](../crds/camundacluster.md#suspension-holds) on the cluster and sets `spec.suspend: true`. The cluster stays suspended while the hold exists, whatever `spec.suspend` says.
- A logical restore also sets `spec.version` to the Camunda version that the backup recorded. See [The version after a logical restore](#the-version-after-a-logical-restore).

The restore waits in `Pending` until the brokers are stopped and the cluster carries that version. It erases nothing in `Pending`. Then it rebuilds the secondary storage (logical kinds only) and gives every broker a new data volume.

Each write carries a field manager of its own. A GitOps tool shows these names in a conflict message:

| Field | Field manager | At the end of the restore |
| --- | --- | --- |
| `spec.suspend` | `camunda-operator/restore-suspend` | Withdrawn when the restore reaches `Completed`. |
| The annotation `suspension-hold.camunda.io/<restore UID>` | `camunda-operator/suspension-hold-<restore UID>` | Removed when the restore reaches `Completed`, or when you delete the restore. A failed restore keeps it. |
| `spec.version` and the annotation `camunda.io/allow-version-downgrade` | `camunda-operator/restore-version` | `spec.version` stays. The operator removes the annotation once the brokers run that version. |

### When the cluster starts again

- **The restore reaches `Completed`.** It removes its hold and withdraws `spec.suspend`, and the cluster starts a moment later. A cluster that you suspended yourself stays suspended. `status.clusterSuspended` on the restore is `true` only when the restore applied the suspension.
- **The restore fails.** The cluster stays suspended, and the restore keeps its hold until you delete it. The broker volumes can be empty or half written. Brokers that start over such volumes do more damage than a cluster that is down. Read `status.failureMessage`, correct the cause, delete the failed restore, and create a new one.
- **You delete a restore that runs.** The restore removes its hold when its work stops, and `spec.suspend: true` stays. Start the cluster yourself when you know what its volumes hold.

### Find out what suspended the cluster

Do this before you set `suspend: false` on a suspended cluster. The restores say whether one of them suspended it, and in which phase it stopped:

```bash
kubectl get pitr,lres,lrrdbms -n my-cluster-ns \
  -o custom-columns='NAME:.metadata.name,PHASE:.status.phase,SUSPENDED:.status.clusterSuspended'
```

The cluster names the managers of its fields:

```bash
kubectl get camundacluster my-cluster -n my-cluster-ns \
  -o jsonpath='{.metadata.managedFields[*].manager}'
```

If `camunda-operator/restore-suspend` is in the list, a restore applied the suspension. A restore that reaches `Completed` gives it back. A failed restore keeps it until you delete the restore.

CAUTION: While a restore holds the cluster, `spec.suspend: false` starts nothing. When that restore goes, the value you wrote applies, and the brokers start over the volumes as the restore left them. Do not start the cluster over the volumes of a failed restore. Create a new restore instead.

To start a cluster that no restore holds, and whose volumes you trust:

```bash
kubectl patch camundacluster my-cluster -n my-cluster-ns --type=merge -p '{"spec":{"suspend":false}}'
```

### One backup or restore at a time

A cluster holds one backup or one restore at a time. A restore takes the cluster when it starts to prepare it, in `Pending`, and gives it back at `Completed` or `Failed`. A restore that finds the cluster held waits in `Pending` with the reason `ClusterClaimed`, and the message names the holder:

```yaml
status:
  phase: Pending
  conditions:
    - type: Ready
      status: "False"
      reason: ClusterClaimed
      message: >-
        LogicalBackupRDBMS/my-cluster-1748937221000 holds
        CamundaCluster my-cluster-ns/my-cluster. Only one backup or restore of
        a cluster runs at a time, so this restore starts when that operation
        no longer holds the cluster
```

Nothing limits this wait, and usually you do not act. If the message names a claim Lease to delete, no backup or restore of the operator holds it. Delete that Lease, and the restore starts. A failed `LogicalRestoreElasticsearch` also keeps the cluster while its `status.recoveryHeld` is `true`.

### A failed restore holds the broker volumes

A restore runs the Camunda restore application once per broker, as a Job named `<restore>-<short name>-<broker>`. Each Job pod uses the data volume of its broker. When the restore reaches `Completed`, the operator deletes the Jobs and their pods. When it reaches `Failed`, it keeps them, because the pod logs name the cause.

So a restore that failed while its Jobs ran holds the broker volumes. `status.primaryJobNames` lists those Jobs. If it is empty, the restore failed earlier and holds no volume. If it lists Jobs, read their logs, then delete the restore:

```bash
kubectl get job -n my-cluster-ns -l camunda.io/logical-restore-rdbms=my-cluster-restore
kubectl logs -n my-cluster-ns job/my-cluster-restore-lrrdbms-0
```

The label key is `camunda.io/logical-restore-elasticsearch`, `camunda.io/logical-restore-rdbms`, or `camunda.io/point-in-time-restore`. Until you delete the restore, a second restore of the cluster and the deletion of the cluster both wait on the volumes.

### Delete a restore

Deleting a restore removes its Jobs and their pods. The broker volumes stay, because they belong to the cluster, and so does the data that the restore wrote. The restore removes its hold from the cluster when its work has stopped, and `spec.suspend` stays as it is. Some kinds wait longer before they go:

- A `LogicalRestoreElasticsearch` waits while Elasticsearch still recovers its snapshots. See [After a failure or a delete](../crds/logicalrestoreelasticsearch.md#after-a-failure-or-a-delete).
- A `PointInTimeRestore` waits while its database server rolls back. See [Who rolls the database back](../crds/pointintimerestore.md#who-rolls-the-database-back).

### The version after a logical restore

A logical restore sets `spec.version` of the cluster to the version of the backup, and it keeps that value. The cluster runs the version of the backup from then on. The restore also writes `camunda.io/allow-version-downgrade`, so that the cluster accepts a version below the one its brokers run. [CamundaCluster: Version](../crds/camundacluster.md#version) states the rule. A `PointInTimeRestore` writes no version.

To run another version after the restore, declare it:

- A client-side `kubectl apply` or `kubectl edit` that sets `spec.version` takes the field over.
- A server-side apply, as Argo CD and Flux use it, reports a conflict on the field. Force the conflict to take the field over.

CAUTION: A manifest that omits `spec.version` does not take the field back. The value that the restore wrote stays, and it wins over the version of a [CamundaRelease](../crds/camundarelease.md). To give the release control again, remove the field by hand.

If the release carries a version below the one the brokers run, the cluster refuses that removal. Remove the field and set the annotation to the version of the release in one command. The annotation alone does not work, because the operator removes an annotation that does not name the version the cluster is asked to run.

```bash
kubectl patch camundacluster my-cluster -n my-cluster-ns --type=merge -p '{
  "metadata": {"annotations": {"camunda.io/allow-version-downgrade": "8.9.0"}},
  "spec": {"version": null}
}'
```

### A GitOps tool that owns the CamundaCluster

A tool that declares `spec.suspend` or `spec.version` fights the restore for the field. The tool reverts the write of the restore, the restore writes it again, and the restore stays in `Pending`. If you drive the `CamundaCluster` from Git:

- For the time of the restore, remove `spec.suspend` and `spec.version` from the manifest, or mark both as an ignored difference.
- After the restore, put `spec.version` back with the version that you want the cluster to run.
- Exclude the annotations that start with `suspension-hold.camunda.io/`, and `camunda.io/allow-version-downgrade`, from pruning. A tool that prunes them removes the hold of the restore and the sanction of its version.

### A point-in-time restore restarts the cluster on a new server

A point-in-time restore on a [DatabaseServer](../crds/databaseserver.md) rolls the server back to the point you asked for. The rollback replaces the PostgreSQL server with a new one built from the archive, under a new name. The published `DatabaseServerConfig` moves to it, so the address of the database changes, and every `CamundaCluster` that reads that contract rolls its pods. Plan the restore as a restart of the whole cluster, not of the database alone.

Read the name of the new server and the address of the contract when the restore is done. The system identifier does not change, because the new server recovers the identity of the old one.

```bash
kubectl get databaseserver my-db -n my-cluster-ns \
  -o custom-columns='CLUSTER:.status.cluster'
kubectl get databaseserverconfig my-db-server -n my-cluster-ns -o jsonpath='{.spec.host}'
```

The archive the rollback read stays in the bucket, next to the archive the new server writes. Nothing removes it, and `retentionPeriodDays` applies only to the archive the server writes now. `status.archive.history` on the `DatabaseServer` keeps its record with an end time. A later restore reaches a point from before the rollback while that point lies within `retentionPeriodDays` of now. A restore of an older point holds with reason `PitrUnavailable`, whatever the bucket still holds. No restore can reach the time between the two archives, because the archive of the new server starts at its first base backup.

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

The operator expands every bound broker volume in place. The storage class must allow volume expansion. If it does not, the API server rejects the expansion. The operator then applies no other change to the cluster, and it tries again. A volume of a new replica is expanded after it binds. `status.volumes` shows the result per volume:

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

The API server rejects a smaller value. If a preset lowers the size under a running cluster, the operator ignores it and keeps the current size. It records the Warning event `StorageShrinkIgnored` once per requested size. To get a smaller volume, delete and recreate the cluster.

`storageSize` of an `ElasticsearchCluster`, and `storageSize` and `walStorageSize` of a `DatabaseServer`, obey the same rules. They grow in place, and a smaller inline value is rejected. A smaller preset value is ignored, with one `StorageShrinkIgnored` event for each requested size. An `ElasticsearchCluster` records the event again after a resume.

## Rotate passwords

The operator generates each password once and keeps it stable. To rotate one, delete its Secret. The operator then generates a new password and publishes it in a new Secret. The old password is never published again. The admin user of a basic-authentication cluster is the exception: it rotates through the spec, and a deletion does not change it on the cluster.

| Password | Secret to delete | What happens next |
| --- | --- | --- |
| The Elasticsearch user of an `ElasticsearchCluster` | `<name>-es-user` | ECK updates the user. Every `CamundaCluster` that references the `SecondaryStorageConfig` of this Elasticsearch rolls its pods. |
| A role of a `Database` | The application or backup credential Secret that the `Database` created | The operator sets the new password on the server before it publishes the Secret. For the application role, every `CamundaCluster` that references the `DatabaseConfig` rolls its pods. |
| The admin user of a basic-authentication cluster | None. Set `spec.auth.basic.passwordRotation` instead. See below. | The operator sets the new password on the `admin` user and rolls the connectors Deployment. |

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

A changed value rotates once. The operator generates a new password and sets it on the `admin` user through the user API of the running cluster. Then it publishes the password in `<name>-camunda-admin` and rolls the connectors Deployment. `status.adminPassword.rotation` shows the value when the rotation is complete. A failed rotation shows on the condition `AdminSecretReady`, and the operator tries again. The [authentication guide](authentication.md#rotate-the-password) has the failure modes.

## Change configuration and referenced Secrets

You do not edit the cluster to roll a configuration change. A change to one of these resources reaches the pods on its own:

- The `CamundaPlatformConfig`, the `CamundaClusterPreset`, or the `CamundaRelease`.
- The `SecondaryStorageConfig` and its `DatabaseConfig` or `DatabaseServerConfig`.
- An `ObjectStorageConfig`.
- A key that the cluster reads from a referenced Secret. The backup credentials of a `DatabaseConfig` are not among them.

The pod templates carry the annotation `camunda.io/config-hash`, and a new hash rolls the pods. A change to the labels or annotations of a Secret, or to a key that the cluster does not read, rolls nothing.

The [CamundaPlatformConfig](../crds/camundaplatformconfig.md) is cluster-scoped. A Secret that it names in another namespace is copied into the namespace of the cluster as `<name>-camunda-<purpose>`, for example `my-cluster-camunda-license` or `my-cluster-camunda-oidc-client`. The pods read the copy. When the source Secret changes, the copy follows. When a key that the pods read changes, the pods roll. `MirroredSecretsReady` reports the copies. Every other Secret a cluster reads already lives in its namespace.

To add your own environment variables, use `extraEnv` and `extraEnvFrom`. The operator writes its own configuration first, then the top-level `extraEnv`. Then it writes the `extraEnv` of the embedded parts that the process hosts, then the `extraEnv` of the process itself. A later entry with the same name wins, and an entry replaces an operator entry with the same name. For example, to set the heap of the brokers:

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

Keep `-XX:+ExitOnOutOfMemoryError` in the value. The operator sets it on every process except connectors, and your entry replaces the value of the operator. When the cluster trusts a certificate authority, the operator adds its trust store options after your value.

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
| `camunda.io/database-server` | The name of the owning `DatabaseServer`. |
| `camunda.io/component` | The role, for example `zeebe`, `gateway`, `operate`, `tasklist`, `admin`, `connectors`, `elasticsearch`, `elasticsearch-exporter`, or `postgres`. |
| `app.kubernetes.io/managed-by` | `camunda-operator`. The pods and volume claims carry the two labels above, not this one. |

Three examples:

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

Kubernetes garbage-collects everything that the cluster owns: the workloads, the Services, the admin Secret, the copied Secrets, and the ServiceMonitors. A ServiceAccount that the operator created goes too. The broker volumes follow the retention policy of the cluster. With `Delete`, the default, they go with the cluster. With `Retain`, they stay, and a later cluster with the same name attaches them again. Set the policy before you delete:

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

A failed restore can hold the broker volumes, and the delete of the cluster then waits on them. Delete that restore first, as [A failed restore holds the broker volumes](#a-failed-restore-holds-the-broker-volumes) describes.

The `ElasticsearchCluster`, the `DatabaseServer`, and the `Database` are separate resources with their own lifecycle. Deleting the `CamundaCluster` leaves them in place. Deleting an `ElasticsearchCluster` removes the ECK resource, its Secrets, and its `SecondaryStorageConfig`, and its data volumes follow its own `persistentVolumeClaimRetentionPolicy`. Deleting a `Database` removes its `DatabaseConfig`, its `SecondaryStorageConfig`, and its credential Secrets, but it never drops the logical database or the SQL roles. Data removal on the server is a manual act.

**CAUTION: Deleting a `DatabaseServer` removes its PostgreSQL instances and their data volumes.** The archive in the bucket stays, and nothing reads it once the server is gone. Take a [logical backup](./backup.md) before you delete a server whose data you still need.

## Related

- [CamundaCluster](../crds/camundacluster.md): every field, condition, and validation rule of the orchestration cluster.
- [ElasticsearchCluster](../crds/elasticsearchcluster.md): the Elasticsearch secondary storage, its conditions, and its snapshot repository.
- [DatabaseServer](../crds/databaseserver.md): the PostgreSQL server, its archive, and the rollback it performs.
- [Database](../crds/database.md): the logical database and its credential Secrets.
- [CamundaClusterPreset](../crds/camundaclusterpreset.md): the baseline that a cluster inherits, and how a preset change reaches every cluster.
