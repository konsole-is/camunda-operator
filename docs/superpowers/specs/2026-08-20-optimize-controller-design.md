# CamundaOptimize controller design

- Date: 2026-08-20
- Status: draft, waiting for review
- Branch: `feat/optimize-controller`

## Purpose

Implement the `CamundaOptimize` CRD and its controller. The CR deploys one Optimize instance for one
`CamundaCluster` and turns on the export pipeline that Optimize reads from. The design page
`docs/crds/camundaoptimize.md` was a rough draft. This spec is the reviewed design. Where the two
differ, this spec wins, and the docs page changes in the same epic.

## Goals

- Real API types for `CamundaOptimize` in `api/v1`, in place of the scaffold.
- A controller that deploys the Optimize webapp and the Optimize importer as separate Deployments.
- The controller turns on the legacy Zeebe Elasticsearch exporter on the referenced cluster with a
  narrow SSA patch, and removes the patch when the CR is deleted.
- Status conditions that follow the repository's condition conventions.
- Mutation tests, golden snapshots, envtest controller tests, and one full data-flow e2e test.

## Non-goals

- No management cluster work. `CamundaManagementCluster` is a separate epic. Optimize consumes the
  `ManagementAuthConfig` contract, which already exists.
- No OpenSearch support beyond what the `SecondaryStorageConfig` contract already models. A cluster
  on RDBMS secondary storage is rejected at reconcile time.
- No Ingress. The platform routes traffic outside this operator.
- No backup or restore changes. `LogicalBackupElasticsearch` covers the `zeebe-record` indices that
  Optimize reads, so a restored cluster can import again. It does not cover the Optimize analytics
  indices, which sit behind a backup API of Optimize that no controller calls. That gap is real and
  it stays out of this epic.

## API

The CRD follows the draft docs page with these changes:

### `spec.version` is new and required

Optimize patch releases are not in lockstep with the platform. Camunda publishes separate
`X.Y.Z-optimize` releases on their own cadence, so a cluster at 8.9.3 gives no guarantee that
`camunda/optimize:8.9.3` exists. The CR carries its own full semantic version. At reconcile time the
controller compares major.minor with the referenced cluster's effective version. On a difference,
the controller sets `Ready=False` with reason `VersionMismatch`, because Camunda requires the two
minors to match.

### Reused types

- `clusterRef` uses the existing `ClusterRef` type: a name in the CR's own namespace. The draft's
  optional `namespace` field is dropped. All extensions in this operator attach in the same
  namespace, and a cross-namespace reference adds RBAC and watch complexity with no user.
- `webapp` and `importer` use the existing `WorkloadSpec` type, so the API reads like the
  per-process sections of `CamundaCluster` (`replicas`, `resources`, `extraEnv`, `extraEnvFrom`,
  `podLabels`, `podAnnotations`, `scheduling`).
- `monitoring` uses a thin `OptimizeMonitoringSpec` wrapper that holds the existing
  `ServiceMonitorSpec`, the same way `ClusterMonitoringSpec` does. The Elasticsearch
  `MonitoringSpec` is not reused, because it also carries an `Exporter` field that only an
  `ElasticsearchCluster` has. ServiceMonitors are created only when the Kubernetes cluster serves
  the kind, which is the existing pattern.
- `managementAuthRef` stays a string that names the cluster-scoped `ManagementAuthConfig`.

### No `platformConfigRef`

Optimize is strictly per-cluster, so platform defaults arrive through the referenced cluster's
`platformConfigRef`. The controller takes the image registry prefix and the license secret from
that platform config. A separate reference on this CR would let the two disagree.

### One Optimize per cluster

One `CamundaCluster` carries one `CamundaOptimize`. The Optimize index prefix is fixed, so two
instances write the same analytics indices of the same Elasticsearch. Their pods would also carry
identical discovery labels, which makes each Service route to the pods of both, and they would share
one field manager on the exporter patch, so deleting either would strip the settings the other still
needs.

Nothing in the schema can express the rule, because it spans resources. The controller resolves it
instead: it lists the `CamundaOptimize` resources of the namespace that name the same cluster and
picks the oldest one, with the name breaking a tie. That one holds the attachment and does the work.
Every other one reports `Ready=False` with reason `ClusterAlreadyAttached`, applies no exporter
patch, and renders no workload. A resource that does not hold the attachment also withdraws nothing
when it is deleted.

`spec.clusterRef` is immutable. A repoint would apply the exporter settings to the new cluster while
the old cluster kept the settings this operator applied, and it would change the pod selectors of the
Deployments, which Kubernetes does not allow. To attach Optimize to another cluster, delete the
resource and create a new one.

### Validation

- CEL rejects `spec.importer.replicas` other than 0 or 1. Optimize supports one active importer, and
  0 is the shared suspend value of `WorkloadSpec`: it stops the import during a restore or an index
  rewrite while the webapp keeps serving.
- CEL rejects a change to `spec.clusterRef`.
- `spec.version`, `spec.managementAuthRef`, and `spec.clusterRef.name` are required and non-empty.

## Exporter attach

The controller SSA-patches `spec.zeebe.extraEnv` on the referenced `CamundaCluster` with the field
manager `camunda-operator/camundaoptimize`. The patch holds only the environment variables that turn
on the legacy Zeebe Elasticsearch exporter with the fixed index prefix `zeebe-record`. The
controller never touches anything else on the cluster.

This needs a schema change on `CamundaCluster`: the `extraEnv` fields become SSA map lists
(`+listType=map`, `+listMapKey=name`). Today the lists are atomic, so a second field manager would
take the whole list and fight the user or a GitOps tool. With map lists, each manager owns only its
own entries. All `extraEnv` fields on the cluster CRD change together for consistency. A side
effect: the API server now rejects duplicate names inside one list. That is a validation
improvement. `extraEnvFrom` stays atomic, because `EnvFromSource` has no merge key.

A finalizer on the `CamundaOptimize` CR removes the patch on deletion: the controller applies an
empty entry set with the same field manager, so the exporter turns off when nothing consumes the
indices. Only the resource that holds the attachment withdraws, because every resource applies under
the same field manager.

A map list merges per field inside an entry, so two field managers that apply the same name do not
collide: one can own `value` while the other owns `valueFrom`. The result is one entry with both,
which a container rejects, and the rollout of the cluster stalls while the CR still looks healthy. A
CEL rule on every `extraEnv` refuses to store that combination. The Optimize controller also reports
`ExporterConflict` before it applies, so the user reads a condition instead of an admission error.

## Controller

The controller follows the extension pattern and the ocf conventions of the existing controllers:

1. Resolve `clusterRef` to the `CamundaCluster`. Read its `storageRef` to find the
   `SecondaryStorageConfig`. A storage type other than `elasticsearch` sets `Ready=False` with
   reason `StorageTypeMismatch`.
2. Resolve `managementAuthRef` to the `ManagementAuthConfig` and make sure that the client secret
   exists.
3. Compare `spec.version` with the cluster's effective version (major.minor), as described above.
4. Apply the exporter patch.
5. Build the components: webapp Deployment, importer Deployment, one Service per Deployment, and
   optional ServiceMonitors. Workloads carry `camunda.io/cluster` (the referenced cluster's name)
   and `camunda.io/component` (`optimize-webapp` or `optimize-importer`).
6. The importer runs with Zeebe data import on and reads all partitions of the cluster. Webapp
   replicas run with import off.
7. Write status once per reconcile through `FlushStatus`: `Ready`, `WebappReady`, `ImporterReady`,
   `MirroredSecretsReady`, and `status.observedGeneration`. Reasons follow the draft docs page, plus
   `VersionMismatch`, `ClusterAlreadyAttached`, and `ExporterConflict`.

A pod reads a Secret of its own namespace only, and `ManagementAuthConfig` is cluster-scoped, so its
client secret reference names one namespace for every consumer. The controller therefore copies every
referenced Secret that lives elsewhere into the `CamundaOptimize` namespace, the way `CamundaCluster`
already does, and reports the shared `MirroredSecretsReady` condition. The pod templates carry a
config hash of the rendered environment and of the resource versions of the referenced Secrets, so a
rotated credential rolls the pods.

Watches: the referenced cluster, both contract CRs, the referenced secrets, and the owned
workloads.

All Camunda application configuration (OIDC settings from the contract, the Elasticsearch
connection from the storage contract, the `CAMUNDA_OPTIMIZE_ZEEBE_*` importer settings) is verified
against the Camunda documentation during implementation. Nothing is written from memory.

## Testing

- Mutation tests and golden snapshots for every component, per the ocf testing conventions.
- Envtest controller tests: reference resolution, each failure reason, patch ownership next to
  user-owned `extraEnv` entries, and finalizer cleanup.
- One full data-flow e2e test: a real Elasticsearch, a `ManagementAuthConfig` written against the
  suite's Keycloak, a seeded process instance, then two waits — first for the `zeebe-record`
  indices, then for the Optimize analytics indices.

## Documentation

- `docs/crds/camundaoptimize.md`: remove the "not implemented" banner, add `spec.version` and the
  `VersionMismatch` reason, drop `clusterRef.namespace`, and align the field list with the reused
  types.
- `docs/crds/camundacluster.md`: describe the map-list semantics of `extraEnv` and the duplicate
  name rejection.

## Risks

- The map-list change on `extraEnv` changes apply semantics for existing users of the cluster CRD.
  Manifests with duplicate names inside one list now fail. This is a clean-slate project, so the
  risk is acceptable.
- The exporter patch turns on export for the whole cluster. Export continues while any
  `CamundaOptimize` attaches to it. The finalizer removes the patch only when the CR is deleted.
- The data-flow e2e test adds minutes of polling to a long suite. It runs in CI. Local runs focus
  one flow, per the repository's e2e conventions.

## Alternatives considered

- **Inherit the version from the cluster.** Rejected: Optimize patch tags are released on their own
  cadence, so an inherited tag can point at an image that does not exist.
- **A dedicated exporter field on `CamundaCluster`.** Rejected: it adds API surface whose only
  client is this patch, and the map-list change keeps the documented extension pattern intact.
- **Cross-namespace `clusterRef`.** Rejected: no extension supports it today, and it adds RBAC and
  watch complexity with no user.

## Implementation breakdown

Multiple PRs on the feature branch `feat/optimize-controller`: the API and schema changes, the
components and controller, and the e2e test. The plan document holds the exact breakdown.
