# One CamundaCluster per secondary storage backend

- Date: 2026-08-22
- Status: draft
- Issue: #166

## Problem

Two `CamundaCluster` resources that resolve to one Elasticsearch endpoint write the same indices
(`camunda-*`, `operate-*`, `tasklist-*`, `zeebe-record*`). They take backups into one snapshot
repository under one base path, and they allocate backup ids without knowledge of each other. A
restore of either one deletes the indices of both, because the delete patterns are fixed strings.
Nothing in the schema, the controllers, or the documentation prevents this, and
`docs/guides/backup.md:34` invites the reader to generalize the bucket rule ("two clusters can
share one bucket") to Elasticsearch, where it is false.

The relational path has the same gap at a smaller scale: two clusters that name one rdbms contract
share one logical database.

The operator enforces this class of rule everywhere it was considered: `Database.spec.databaseName`
is unique per server, `PointInTimeRestore` refuses a shared server, `rejectSharedAzureContainer`
refuses a second cluster on one Azure container, and `CamundaOptimize` refuses a second instance on
one cluster because the Optimize index prefix is fixed. The secondary storage backend is the one
shared resource without a guard.

## Goals

- One `CamundaCluster` per secondary storage backend, cluster-wide, for both storage types.
- The cluster that yields stops writing into the shared backend and keeps its data.
- The cluster that yields reports the holder and the backend in its `Ready` condition.
- The cluster that yields resumes on its own when the holder releases the backend.
- The rule is stated where a reader decides how to lay out storage: `docs/guides/secondary-storage.md`,
  `docs/guides/backup.md`, and `docs/crds/camundacluster.md`.

## Non-goals

- A per-cluster Elasticsearch index prefix. With one cluster per backend there is nothing to separate.
- Changes to the restore delete patterns, the backup id arbitration, or the snapshot repository
  naming. The rule makes each of them correct as they are. Their comments and docs change where
  they claim a scope that the code does not have.
- A guard on `CamundaOptimize` per Elasticsearch endpoint. One cluster per backend and one Optimize
  per cluster together give one Optimize per backend.
- The same-named `ElasticsearchCluster` in two namespaces that share one Elasticsearch, where a
  restore can repoint the repository of the target. That is a different resource and a different
  fix. It gets its own issue.
- A webhook or admission policy. The operator checks references at reconcile time, and CEL cannot
  express a rule across objects.

## Design

### The rule

A secondary storage backend is the thing the workloads connect to, not the contract object that
names it. Two contracts in two namespaces can name one endpoint, and a hand-written contract has no
producer resource behind it. The guard therefore keys on a resolved backend identity:

| Storage type | Identity | Source |
| --- | --- | --- |
| elasticsearch | normalized `endpoint` | `SecondaryStorageConfig.spec.elasticsearch.endpoint` |
| rdbms | `host:port/database` | `databaseConfigRef` → `DatabaseConfig` → `DatabaseServerConfig`, the chain `resolveRDBMSStorage` already walks |

Normalization of the Elasticsearch endpoint: lowercase scheme and host, an explicit port (the
default port of the scheme when the URL has none), no trailing slash, query and fragment dropped.
The path stays, because an Elasticsearch behind a path prefix is a different endpoint.
Credentials are not part of the identity. Two users still write one set of indices.

A pure function computes the identity from the resolved storage. A reader-backed variant resolves
a contract to its identity for the siblings. Both live in `pkg/wrappers/secondarystorageconfig`,
next to `ElasticsearchAdmin`, the one shared helper that turns a contract into something a consumer
uses.

### The guard

The guard is a pre-check step in `internal/controller/camundacluster/precheck.go`, placed right
after `resolveStorage`, in its own file beside `backup.go`. It lists every `CamundaCluster` through
the live `APIReader`, as the Azure guard and the `SharedServer` guard do, because a cached list can
miss a sibling that was created a moment ago. For each sibling it resolves the identity of the
sibling's backend and compares it with its own. A sibling whose contract does not resolve uses
nothing yet and is skipped. When its chain resolves it holds the backend if it is older, and this
cluster yields on its next reconcile.

When two clusters name one backend, the older one holds it. `olderThan` decides: creation
timestamp, then name. This is the rule of the Azure guard, the Optimize attachment, and the
Database collision. The guard runs on every reconcile, so a running holder yields when an older
cluster appears on its backend (amendment of 2026-08-23, see Handover).

The Azure guard and the new guard share one loop: list the siblings, find the oldest one that
matches a predicate. The loop moves into one helper that both call, so the two guards read as one
author's.

### The cluster that yields

The guard does not fail the pre-check. A failed pre-check returns before the components are built,
which leaves the workloads running and writes the stale per-process conditions back unchanged.
Instead the guard records the holder on the `Input`, and the reconcile continues with suspension
forced on. The existing suspend path scales every workload to zero and keeps the volumes. The
component conditions flush truthfully, for example `ZeebeReady=False/Suspended`.

After the components reconcile, `Ready` is overwritten with `False`, reason `StorageAlreadyAttached`,
and a message that names the holder and the backend:

```text
CamundaCluster "my-other-ns/my-other-cluster" already uses Elasticsearch "https://es.example.com:9200".
One CamundaCluster uses one backend, so this cluster stays suspended until that one releases it
```

The reason mirrors `ClusterAlreadyAttached` on `CamundaOptimize`, which exists for the same cause.
`spec.suspend` stays the user's field. The forced suspension is visible through the reason only.

### Handover

A cluster that yields must reconcile again when the holder is deleted, when the holder's
`storageRef` changes, or when the holder's contract names a different endpoint. A running holder
must reconcile again when an older cluster appears on its backend: a cluster created in the same
second with a name that sorts first, an older cluster whose contract chain becomes resolvable, or
an older cluster that repoints its `storageRef`. Only the first set can be keyed to the parked
clusters. The second set needs the holder, and a backend cannot be indexed without resolving a
contract chain.

*Amendment of 2026-08-23.* The first design enqueued only the parked clusters and missed the
second set: two holders could coexist. The watches now fan out to every cluster:

- A watch on `CamundaCluster` enqueues every other cluster.
- The watches on `SecondaryStorageConfig` and `DatabaseConfig` enqueue every cluster, as the
  `DatabaseServerConfig` watch already did.
- All three carry `predicate.GenerationChangedPredicate`, so a status write does not fan out. Create
  and delete events pass it.

No timer is needed. The cost is one reconcile of every cluster per spec change of any cluster or
contract, which is rare and human-driven.

### What the rule closes without code

With one cluster per backend:

- The restore's cluster-wide `DeleteIndices` deletes the indices of its target only, because its
  target is the only cluster on that endpoint.
- Per-cluster backup id arbitration is per-repository arbitration.
- Two `CamundaOptimize` instances on one Elasticsearch cannot exist.

The remaining work is words. The claim at `pkg/components/elasticsearchcluster/snapshotstorage.go:308-311`
("two clusters that reference the same contract never share a repository") becomes a statement about
`ElasticsearchCluster` resources, which is what it is true of. `docs/crds/logicalrestoreelasticsearch.md`
says why the deleted indices are the target's. `docs/guides/backup.md:34` stops at the bucket and
points to the secondary storage rule for Elasticsearch.

## Alternatives considered

**Key on the contract object (namespace and `storageRef`).** Cheap and indexable, like the Database
collision guard. Rejected: two clusters in different namespaces each own a contract, and both
contracts can name one endpoint. The bring-your-own flow in `docs/guides/secondary-storage.md`
produces exactly that layout.

**Park only, like the Azure guard.** Smallest change. Rejected: a running cluster that yields keeps
writing into the shared backend, which is the harm the rule exists to stop, and its stale
`ZeebeReady=True` conditions are written back.

**Release the workloads, like Optimize.** Stops the writes. Rejected: deletion of a broker
StatefulSet is heavier than scale-to-zero and gains nothing over the suspend path that already
exists.

**Holder sticks, then oldest.** A cluster that records the backend in its status keeps it, so a
spec change elsewhere can never depose a running cluster. Rejected for now: it costs a status field
and a second rule to explain. Oldest-wins is the rule of three existing guards, and repointing an
older cluster onto a live backend is an explicit act on that cluster's spec. The message on the
cluster that yields names the cause.

**A Lease per backend, like `pkg/clusterclaim`.** Atomic under concurrent reconciles. Rejected:
the Lease is namespaced and the backend is not, the lease name would hash the endpoint, and release
on cluster deletion would need a finalizer. The live list with a deterministic tie-break is the
accepted precedent, and the window between two simultaneous creates closes on the next reconcile
of either side.

## Risks

- **Cost per reconcile.** The guard resolves one contract per sibling cluster, and for rdbms two
  more objects. The number of clusters per operator is small, and the Azure and `SharedServer`
  guards already pay a live list per reconcile.
- **Clock granularity in envtest.** Two clusters created in one second tie on the timestamp and the
  name decides. Tests name the pair so that the winner is deterministic, or assert the XOR as the
  Azure test does.
- **A sibling under deletion.** It still counts until it is gone, as in the Optimize attachment.
  The cluster that yields takes over when the delete event arrives.
- **A holder whose own contract chain breaks keeps its pods.** A failed pre-check returns before the
  components are built, so a running holder whose contract or DatabaseConfig is deleted keeps
  writing, while a parked cluster on another contract sees no holder and resumes. The same-contract
  case is safe, because the parked cluster fails the same resolve. The fix is a broader rule, a
  failed pre-check of a running cluster suspends it, and has its own issue.

## Testing

- testify: endpoint normalization (case, default port, trailing slash, path kept, query dropped),
  rdbms identity, and the shared oldest-sibling helper.
- envtest, Ginkgo: two clusters in one namespace on one contract (exactly one parks, its
  StatefulSets scale to zero, its `Ready` names the holder); two clusters in two namespaces on two
  contracts with one endpoint (same outcome); two clusters on one rdbms database (same outcome);
  two clusters on two endpoints (both Ready); the holder is deleted and the parked cluster resumes.

## Implementation breakdown

One pull request against `main`. There is no second surface that ships value on its own.
