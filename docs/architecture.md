# Architecture

This page gives the rules that shape the API of the operator.
Read it to learn why a feature is its own resource. Read it also before you write a controller that extends the operator.

## One small core, many attachments

`CamundaCluster` describes the orchestration cluster and nothing else.
Every other capability is its own kind. A backup, for example, is a `LogicalBackupElasticsearch` that names the cluster in `clusterRef`. The cluster does not know that the backup exists.

This is the core rule of the operator: **features attach to workloads. Workloads do not know about features.**

The rule has three consequences that you can rely on:

- You do not edit the cluster spec to attach a feature. There is no plugin to enable and no field to set.
- A failure in an attached feature does not stop the operator from managing the cluster.
- You can create, change, or delete an attached feature without an edit to the cluster spec.

## How a feature finds its workloads

The operator labels every resource that it creates with the owner, the component, and itself:

| Label | Value |
| --- | --- |
| `camunda.io/<owner-kind>` | the name of the owning custom resource. The key names the kind, for example `camunda.io/cluster` for a `CamundaCluster`. |
| `camunda.io/component` | the role of the resource, for example `zeebe`, `gateway`, `elasticsearch` |
| `app.kubernetes.io/managed-by` | `camunda-operator` |

The owner keys are `camunda.io/cluster`, `camunda.io/elasticsearch-cluster`, `camunda.io/database-server`, `camunda.io/database`, `camunda.io/management-cluster`, `camunda.io/backup-schedule`, `camunda.io/logical-backup-elasticsearch`, `camunda.io/logical-backup-rdbms`, `camunda.io/logical-restore-elasticsearch`, `camunda.io/logical-restore-rdbms`, and `camunda.io/point-in-time-restore`.
A label value holds at most 63 characters. For a longer name, the value is the start of the name followed by a hash.

ECK and CloudNativePG run pods and volumes from a template of this operator. These pods and volumes carry the owner and component labels, but not the `managed-by` label.

A feature finds the workloads of a cluster through these labels, or it reads the cluster through `clusterRef`.
To call the cluster, a feature reads the status of the `CamundaCluster`. `status.management` holds the address of the management API. `status.gateway` holds the gRPC and REST addresses of the client APIs. Both are empty while the cluster is suspended.

## How a feature acts on a cluster

A feature that must stop a cluster, such as a restore, adds an annotation with the prefix `suspension-hold.camunda.io/` to the `CamundaCluster`. The value says why. The cluster stays suspended while it has at least one of these annotations, and its `Ready` condition reports `SuspensionHeld`. When the feature removes its annotation, the cluster starts again, unless `spec.suspend` is true. See [Suspension holds](crds/camundacluster.md#suspension-holds).

## Contracts carry connection details

The kinds pass connection details to each other through contract kinds, never through direct references. The [CRD reference](crds/index.md#contracts) lists the contract kinds.

The resource that provides a backend writes the contract. The resource that uses the backend reads the contract by name.
An `ElasticsearchCluster` writes a `SecondaryStorageConfig`. A `Database` writes a `DatabaseConfig`. A `CamundaManagementCluster` writes a `ManagementAuthConfig`. A `CamundaCluster` reads the `SecondaryStorageConfig` in `storageRef` and does not know who wrote it.

You can also write a contract by hand, or let another tool write it. An Elasticsearch cluster that the operator does not manage, or a bucket that another tool provisions, looks the same to the consumer.

The operator validates every contract and reports the result on `Ready`. It checks that the referenced Secrets and resources exist. For a `DatabaseServerConfig`, it also connects to the server. It provisions nothing from a contract.

[How the kinds relate](crds/index.md#how-the-kinds-relate) shows which kind writes and reads each contract.

## References and namespaces

A reference by name points into the namespace of the resource that holds it. A `CamundaCluster` in `my-cluster-ns` reads the `SecondaryStorageConfig` of `storageRef` and the `ObjectStorageConfig` of `backupStorageRef` in `my-cluster-ns`. A reference to a cluster-scoped kind carries a plain name. `CamundaPlatformConfig`, `ManagementAuthConfig`, `CamundaRelease`, and the three preset kinds are cluster-scoped.

The same rule holds for Secrets. A namespaced kind reads its Secrets from its own namespace, so its Secret references name a Secret and a key, and never a namespace. `CamundaPlatformConfig` and `ManagementAuthConfig` are cluster-scoped, so their Secret references also name a namespace. The operator copies those Secrets into each namespace that reads them. A preset names no namespace. A Secret reference on a preset resolves in the namespace of each cluster that inherits it.

The management plane is the one place where a resource reaches across namespaces. A `CamundaManagementCluster` selects `CamundaClusters` with `clusterSelector`, in the namespaces that `namespaceSelector` admits, and annotates the clusters that it serves. A `CamundaCluster` never references a management plane.

## Some settings are fixed at creation

A few fields cannot change after the resource exists. The API server rejects the change with the message of the rule:

| Field | Message |
| --- | --- |
| `spec.zeebe.storageClassName` of a `CamundaCluster` | `zeebe.storageClassName is immutable` |
| `spec.clusterRef` of a `CamundaOptimize` | `clusterRef is immutable: delete this CamundaOptimize and create a new one to attach it to another cluster` |
| The whole spec of a logical backup, a logical restore, or a `PointInTimeRestore` | Starts with `spec is immutable`. To try again, create a new resource. |

## Status conventions

Every kind that the operator acts on reports conditions, and every one has an aggregate `Ready` condition. `status.observedGeneration` records the last generation that the operator processed.

A kind that runs workloads also reports one condition per component, named `<Component>Ready`, for example `ZeebeReady`. `Ready` takes its reason and message from the component that needs attention. When every component is healthy, the reason is `Healthy`.

The reasons that you see most:

| Reason | Meaning |
| --- | --- |
| `Healthy` | Everything the resource needs is in place. |
| `Creating`, `Updating`, `Scaling` | The workloads roll out. |
| `Failing` | A workload does not reach the desired state. |
| `Degraded` | A workload is still not ready at the end of its grace period. It reports a partial state, for example some ready replicas or yellow Elasticsearch health. |
| `Down` | A workload is still not ready at the end of its grace period. It reports a critical state, for example no ready replica or red Elasticsearch health. |
| `Suspended` | `spec.suspend` is true. `Ready` is `True`, because the resource is in its desired state. |
| `SuspensionHeld` | A `CamundaCluster` carries a suspension hold. `Ready` is `False`, because another resource keeps the cluster stopped, not your spec. The message names each hold. |
| `Disabled` | The component is not part of the current topology. This is not an error. |
| `InvalidReference` | A referenced resource does not exist, or the merged configuration is invalid. The message names it. |
| `MissingSecret` | A referenced Secret or key does not exist. The message names it. |
| `ConnectionFailed` | An external system did not answer, or it refused the credentials. |
| `Error` | The operator hit an error. The message carries it. |

The page of each kind in the [CRD reference](crds/index.md) lists all its reasons and the step for each.

A workload gets a grace period to become ready. The grace period starts when the condition first reports a not-ready reason, such as `Creating` or `Blocked`, or when it changes from `True` to `False`. It starts again when the workload leaves `PrerequisiteNotMet`, `Disabled`, a suspension, or a name that another owner holds. A change from `Creating` to `Updating` does not start it again. Until the grace period ends, the condition reports its progress: `Creating`, `Updating`, `Scaling`, or `Failing`. After it ends, the condition reports `Degraded` or `Down` until the workload is ready again. By default, every grace period is 30 minutes. To change the values, see [Grace periods](installation.md#grace-periods).

The API server accepts a resource that names something you did not create yet, so you can create resources in any order. A resource waits with `InvalidReference` or `MissingSecret` until its references exist.

## How the operator writes

The operator applies the resources that it keeps in shape, such as the workloads, Services, and Secrets of a cluster, with Server-Side Apply. It owns only the fields that it sets. A field that you set by hand on such a resource stays until the operator sets that field. The Jobs of a backup or a restore are different: the operator creates each one once and does not update it.

## The api module

To use the custom resources from Go, see [Use the API types from Go](go-api.md).

## Support policy

- **Camunda 8.9 and later.** The operator runs the orchestration cluster of Camunda 8.9 and later. A `CamundaCluster` with a lower version reports `InvalidReference`, and the message contains `is below the supported floor 8.9.0`.
- **Minor releases are the test matrix.** Each release of the operator is tested against the newest patch of each supported Camunda minor.
- **Elasticsearch through ECK, PostgreSQL through CloudNativePG.** `ElasticsearchCluster` needs the ECK operator. `DatabaseServer` needs the CloudNativePG operator, and its archive also needs the Barman Cloud plugin and cert-manager. `Database` creates a logical database on any PostgreSQL server, whether a `DatabaseServer` runs it or you do. See [Installation](installation.md#requirements).
