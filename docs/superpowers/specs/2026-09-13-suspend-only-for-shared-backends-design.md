# Suspend only for shared backends

**Status:** draft
**Date:** 2026-09-13
**Scope:** CamundaCluster storage claim, CamundaOptimize handover labels, the pre-check failure
branch of both controllers, `CamundaCluster.Suspended()`

## Summary

The operator stops a running workload for one reason only: the workload would write a backend
that another live cluster writes. Every other failure keeps the workloads on their last
configuration and reports on `Ready`.

Two changes carry this rule.

1. The storage claim moves from two annotations on the `SecondaryStorageConfig` to a Lease on
   `pkg/leaseclaim`, keyed by the backend the contract resolves to. Two contracts that name one
   Elasticsearch meet on one Lease. A holder whose contract is deleted keeps its Lease, so a
   peer that resolves the same backend parks instead of starting beside it.
2. The pre-check failure branch of `CamundaCluster` and `CamundaOptimize` no longer scales
   the workloads to zero. PR #319 added that on every failed check, and the only case it
   closed is the one the Lease now closes without an outage.

Issue #331, which asked for the same scale-to-zero on three more kinds, is closed as not
planned. Those kinds are the backend, not a writer to one.

## Goals

- A running cluster keeps running when a preset, a release, a platform config, a bucket, a
  ServiceAccount, or a Secret it references goes away, or when a spec edit fails validation.
  `Ready` reports the failure. The same holds for a `CamundaOptimize`.
- Two clusters never both run against one backend, whether they name one contract or two
  contracts that resolve to the same address, and whether or not the contract of the holder
  still exists.
- A backend moves from one cluster to the next without both writing it at once, across
  contracts as well as within one.
- One claim protocol in the repository. The storage claim runs on `pkg/leaseclaim` beside the
  Database claim and the realm claim.

## Non-goals

- No change to the Optimize attachment claim (`ClusterAlreadyAttached`) or to the Database
  and realm claims.
- No detection of two addresses that reach one server. The key compares what the contract
  says. A DNS alias for the same Elasticsearch is not caught, as the docs already state for
  two hand-written contracts.
- Reversed during review: the `CamundaCluster` carries a finalizer that releases its storage
  Leases on deletion, as the Database controller does for its claims. Without it every
  deleted cluster left one Lease behind for good, an unbounded leak in any CI that creates
  and deletes clusters. A holder that is gone without the finalizer having run is still taken
  over.
- No framework primitive for a detached scale-to-zero (sourcehawk/operator-component-framework#207).
  Nothing in this operator needs one after this change.
- No change to what `spec.suspend` does.

## The rule

A workload of the operator stops on its own in exactly two states, both on the storage claim:

| `Ready` reason | State |
| --- | --- |
| `StorageAlreadyAttached` | Another live cluster holds the Lease of the backend this cluster resolves. This cluster renders every workload at zero. |
| `WaitingForHandover` | Pods of another cluster still write the backend this cluster resolves, whether this cluster holds the Lease or waits to create it. This cluster renders every workload at zero until they are gone. |

`spec.suspend` is the user's own instruction and stands above this table: a cluster whose
reference check fails while `spec.suspend` is set still scales every workload it controls
to zero, directly, because a broken reference is exactly when a user reaches for suspend.
`Ready` reports the failure and names the suspension in its message, and the per-process
conditions of the scaled workloads report `Suspended`. A cluster that was at zero when its
check failed stays at zero until the check passes, because only the render restores replicas.

The same holds one level down. A `CamundaOptimize` whose own check fails still follows the
suspension of its cluster: it scales its webapp and importer to zero when the cluster is
suspended, because the logical restore of Elasticsearch relies on the importer stopping. It
keeps its workloads only for a failure while its cluster runs. An Optimize whose cluster is
deleted releases its workloads, as it does when another instance holds the cluster, so its
pods never block the handover of the backend to the next cluster.

Every other `Ready` reason leaves the workloads as the last successful reconcile rendered them.
`InvalidReference`, `MissingSecret`, `VersionMismatch`, `StorageTypeMismatch`,
`ExporterConflict`, and `VersionDowngradeRefused` are reports, not stops.

The Kubernetes posture is the model: a Deployment whose ConfigMap is deleted keeps running.
The operator adds a stop only where running would corrupt data that belongs to someone else.

## The storage claim on a Lease

### Schema

`pkg/components/camundacluster` declares `StorageClaimSchema() leaseclaim.Schema[*v1.CamundaCluster]`
with prefix `camunda-storage-`, noun `storage claim`, four annotation keys under
`camunda.io/storage-claim-*`, and labels `labels.Managed(labels.Cluster(name), "storage-claim")`.
The controller validates it in `SetupWithManager` and reads `ClaimNamespace` from the operator
namespace, as the Database and management controllers do. The Leases live in the operator
namespace, so two clusters in two namespaces that name one backend meet on one Lease.

### Key

`StorageClaimKey(storage components.Storage) string` returns the backend a resolved chain
addresses:

- Elasticsearch: `elasticsearch|<scheme>://<host>:<port>`, with the scheme and the host
  lowercased and the port explicit (80 or 443 when the URL names none). The path is not part
  of the key: Optimize renders host and port only, so two endpoints that differ in a path
  prefix reach one Elasticsearch for at least one writer.
- RDBMS: `rdbms|<host>:<port>/<database>`, with the host lowercased.

The key is what the contract resolves to, not the contract. Two contracts on one address
produce one key. One contract edited to another address produces another key, and the
cluster on it moves backends the way a repoint does.

### Take

`claimStorage` runs after `resolveStorage`, where it runs today, and calls `TakeUnclaimed`
with the key and one rule that runs only while no Lease holds it: no pod of another cluster
may still carry the claim. A free key with such pods is a handover in progress or a Lease
that was deleted by hand, and the cluster whose pods write the backend recreates the Lease
first, because its own pods are not "other". A parked cluster keeps waiting. The controller
also watches the claim Leases, enqueueing the holder the annotations name, so a deleted
Lease is noticed at once and not at the next unrelated event. Three outcomes:

- `Take` returns no blocker: this cluster holds the Lease. It then runs the handover gate.
  The release of every other storage Lease it holds (`Held`, then `Release` for each Lease
  whose name differs) happens in the controller after the render was applied, not in the
  pre-check, so a pass that fails between the claim and the apply never frees a backend the
  old pod template still writes. A Lease is released only when no pod of this cluster still
  carries its claim label, the same signal the handover gate reads; while one is held back,
  the cluster looks again on its retry interval. The same pod-gated release runs on the
  foreign-Lease and bad-key exits, where the cluster keeps running on its previous backend
  and gives it back once its pods are gone, and on the refused-downgrade path. A pre-check
  failure that comes before the claim step releases nothing: the pass does not know which
  backend the cluster resolves, and a cluster at zero must not give its live backend away
  over a missing preset.
- The blocker names a holder: `in.Storage.Holder` carries the holder and the key, and the
  controller renders the cluster suspended with `StorageAlreadyAttached`, as it does today.
  The message names the holder and the backend.
- The blocker is foreign (a Lease with the name and no holder annotations): `Ready` reports
  `InvalidReference` with a message that names the Lease and says to delete it if nothing
  uses it, as the Database claim does. No cluster holds the backend, so it is not a
  suspension.

`HolderKeeps` is the default `OwnerExists`: a holder keeps the Lease while a `CamundaCluster`
exists under the recorded UID. `Take` takes over the Lease of a holder that is gone in the
same pass. A paused holder keeps its Lease because it exists. A holder that repointed keeps
its old Lease until its own reconcile resolves the new backend and releases the old one. If
the new chain fails the pre-check, the old Lease stands and the old workloads keep running,
which is right: they still write the old backend.

The `staleHolder` rule goes. Its three cases are covered: a holder that does not exist is
taken over by `OwnerExists`, a later cluster of the same name has another UID, and a repoint
is a release by the holder itself.

### The dangling contract

A holder whose `resolveStorage` fails returns before `claimStorage`, so it neither takes nor
releases anything. Its Lease stands with its UID, its workloads keep running, and `Ready`
reports `InvalidReference` or `MissingSecret`. A peer that resolves the same backend through
any contract meets the Lease, finds the holder alive, and parks. When the reference returns,
the holder's next pass calls `Take`, finds its own Lease, and writes nothing.

This is the case #319 closed by scaling the holder to zero. The Lease closes it with the
holder still running.

### The handover gate

The pod label `camunda.io/storage-contract` becomes `camunda.io/storage-claim` and carries
the Lease name (`StorageClaimSchema().LeaseName(key)`, 56 characters, a valid label value).
`labels.StorageContractKey` becomes `labels.StorageClaimKey`. `StoragePodLabels` takes the
Lease name instead of the contract name. The label stays on the pod template only, never on
the selector, so a change of backend rolls the pods and the new ones carry the new value.

After `Take` succeeds, the cluster lists the pods in every namespace that carry the Lease name
and do not belong to it. The Lease is shared across namespaces, so the pods must be too. A pod belongs to it when its `camunda.io/cluster-uid` label names
this cluster's UID. The pod templates of the cluster and of Optimize gain that label; today
only the Web Modeler user Secrets of the management plane carry it. While any such pod
exists, the claim step records them on the input and the controller renders the cluster
suspended, every workload at zero and the volumes kept, with `Ready` False and reason
`WaitingForHandover` naming the pods. It looks again on its retry interval, as a parked
cluster does. This is a render, not a pre-check failure: a running cluster that repoints into
a handover stops, so its old pods leave the previous backend and that handover completes too.

The gate runs only at a takeover: when this cluster did not hold the claim at the start of
the pass, and none of its own pods carries the claim yet, and the cluster is not suspended
by its spec, because a suspended cluster writes nothing and needs no handover. Once its own pods write the
backend, no pod of another cluster can, because the render at zero was what held them back.
That signal cannot be lost the way a status write can, so a healthy cluster never lists pods
cluster-wide. One namespaced metadata list of the cluster's own pods serves this decision and
the release below.

A holder that moves to another backend keeps the old claim while any of its pods still
carries it, so a cluster parked on that backend stays `StorageAlreadyAttached` until those
pods are gone, and reaches `WaitingForHandover` only for a holder that is deleted. The claim,
not the one-time scan, carries the guarantee in the repoint case. This covers a
previous holder on the same contract, a previous holder on another contract to the same
address, a deleted cluster whose pods the garbage collector has not reached, and a later
cluster of the same name.

The Optimize pods carry the same label. `CamundaOptimize` resolves the cluster's chain
itself, so it computes the key and the Lease name from that chain, and `Input.StorageContract`
becomes the Lease name. Its importer writes the same backend, so it is gated the same way.

Optimize also reads the claim itself: it renders its importer only while its cluster holds
the storage claim of the backend it resolves (`Holds` on the Lease) and no pod of another
cluster carries that claim, the same two checks the cluster makes before it renders. The
cluster's `Ready` lags a repoint by one reconcile, so `Suspended()` alone would let the
importer start against a backend another cluster holds, or still writes, in that window. The
claim and the pods are the gate; the status is the report. One exported selector helper
serves both gates so they cannot drift.

### What goes

- `pkg/wrappers/secondarystorageconfig/claim.go` and its test: `Claim`, `HolderOf`, `Holder`,
  and the two annotation keys. The wrapper keeps its mutator and its Elasticsearch admin
  client.
- `staleHolder` and `holderPods` in `internal/controller/camundacluster/secondarystorage.go`.
  `waitForHandover` stays and lists by the Lease name.
- The `camunda.io/claim-holder` and `camunda.io/claim-holder-uid` annotations from the docs.
  No producer references them in code: server-side apply kept them because another field
  manager owned them, so nothing else changes there.

The event `StorageClaimed` stays and names the backend as well as the contract.

### RBAC

The controller gains the `coordination.k8s.io` `leases` marker the Database controller
carries. `config/rbac/role.yaml` already grants it through that controller, so the generated
role should not change. `make manifests` decides.

## No suspension on a failed pre-check

The pre-check failure branch of both controllers returns to the shape before #319: stage
`Failed` on `Ready`, requeue on a timer for an unwatched failure, return. That holds for a
parked cluster too: a failed reference on it reports the failure, its workloads stay at zero
because nothing renders, and `Suspended()` reads false. Optimize does not depend on that
reading for the parked case, because it gates its importer on the storage claim itself. A
backup of such a cluster is admitted and waits at `Pending` for the management endpoint,
which the parked render cleared, until the cluster recovers, and a schedule skips its later
triggers while that backup is not terminal. That is a doubly broken cluster, reported by
name on both objects, and the user fixes the reference.

What goes:

- `internal/controller/camundacluster/suspend.go` and `suspend_test.go`: `suspendWorkloads`,
  `scaleToZero`, `stageSuspension`, `processConditions`, `eventReasonWorkloadsSuspended`.
- `internal/controller/camundaoptimize/suspend.go` and `suspend_test.go`: the same three
  functions, `workloadConditions`, `eventReasonWorkloadsSuspended`. `recordSuspensionChange`
  and `wasSuspending` stay: they report the suspension that follows the cluster.
- The message suffix "The workloads are scaled to zero, with the volumes kept, until the
  pre-check passes" and its Optimize twin.
- The clearing of `status.gateway` and `status.management` on a failed pre-check. A running
  cluster keeps publishing the endpoints its workloads serve.
- The envtest specs that assert a scale to zero on a deleted contract or Secret. They become
  specs that assert the replicas stay and `Ready` reports the failure.

The Optimize `ClusterAlreadyAttached` path is untouched: a deposed holder still removes the
workloads that belong to the instance that holds the cluster.

### `Suspended()`

`suspendedReadyReasons` in `api/v1` narrows to `StorageAlreadyAttached` and
`WaitingForHandover`. `Suspended()` then answers true for `spec.suspend` and for the two
states in the table above, and nothing else. All three hold every workload at zero. Its
three readers change behavior without a code change:

- `CamundaOptimize` scales to zero with the cluster in those states only.
- A logical backup waits with `ClusterSuspended` in those states only. A cluster on a
  dangling reference is running, so a backup of it runs.
- A `BackupSchedule` skips triggers in those states only.

## Docs

- `docs/crds/camundacluster.md`: the Secondary storage section describes the backend the
  operator claims (the address the contract resolves to), the label, the cross-contract case,
  and the handover. It does not name the Lease or its name pattern: the repository docs skill
  keeps a Lease off a user page, and the Database page sets the precedent. Only the row for a
  foreign claim names the Lease the message names, because the user deletes that object. The claim annotations and the
  "recreated contract is a new claim" warning go, because a recreated contract resolves to
  the same key and the holder keeps its Lease. The reference-check section and the
  `InvalidReference` and `MissingSecret` rows say the workloads keep running. The Suspend
  section lists the two operator-driven suspensions.
- `docs/crds/secondarystorageconfig.md`: the claim paragraphs describe the claim on the
  backend, without the Lease. "The contract is the unit of the claim, not the endpoint"
  inverts: the backend is the unit.
- `docs/guides/secondary-storage.md`: "One cluster per contract" becomes one cluster per
  backend, and "The operator compares contracts, not endpoints" goes.
- `docs/crds/camundaoptimize.md`: the label paragraph, the Suspension section, and the
  reason table drop the scale-to-zero on a failed check. The VersionMismatch paragraph says
  the workloads keep running until `spec.version` follows the cluster.
- The two logical backup pages and `docs/guides/backup.md` follow the narrowed `Suspended()`.

## Risks

- **A Lease outlives a cluster deleted while the operator was down.** The finalizer releases
  the Leases of a cluster on an ordinary deletion. A cluster removed while nothing ran the
  finalizer leaves one Lease that blocks nothing: `OwnerExists` answers false for it, and the
  next claimant of that backend takes it.
- **Two clusters on one backend before this change both run.** After the upgrade, both
  reconcile, one takes the Lease, and the other parks. Which one is reconcile order. The
  pre-change annotation claim had the same property for one contract. This is a clean-slate
  project with no release, so no upgrade note is written.
- **A key change of the Elasticsearch normalization changes every Lease name.** The
  normalization is pinned by a unit test with the documented cases, so a later edit is a
  visible decision.
- **The handover gate reads pods once, before render.** It protects against pods the
  previous holder started before it lost the claim. A holder that points back at the backend
  meets the claim the waiter holds and parks, so it starts nothing beside the waiter.

## Alternatives considered

- **A backend record in `status.storage` with an age tie-break.** Rejected once the Lease
  package landed (#364). Two status writes on two objects cannot exclude each other; the
  Lease create does, and the protocol is already in the repository.
- **The identity check layered on the annotation claim.** Rejected: two mechanisms for one
  concept, and the same-contract flow keeps its recreated-contract race.
- **Narrow the #319 suspension to the storage-chain failures instead of removing it.**
  Rejected: with the Lease, a holder on a dangling contract stops no one from parking, so
  there is no failure left whose only remedy is an outage.
- **Keep `VersionMismatch` as a stop for Optimize.** Rejected under the rule. Nothing stops
  a Helm-installed Optimize of the previous minor during the same upgrade window, and the
  window closes with the second edit the docs already ask for.

## Testing

- `pkg/components/camundacluster`: `StorageClaimKey` over the two types, the normalization
  cases (case, default port, trailing slash), and the Schema's `Validate`. A golden Lease
  fixture as the Database claim has.
- `internal/controller/camundacluster` unit tests: the handover gate over a fake client with
  pods of this cluster, of a previous holder on the same contract, of a holder on another
  contract with the same key, and of a same-named later cluster.
- envtest, cluster: two contracts to one address leave the second cluster at
  `StorageAlreadyAttached` naming the holder and the backend; deleting the holder's contract
  keeps the holder's replicas and reports `InvalidReference`, and the second cluster stays
  parked; deleting the holder hands the backend over after its pods go; a repoint releases
  the old Lease; deleting the credentials Secret keeps the replicas and the published
  endpoints. The existing same-contract specs pass with the label rename.
- envtest, Optimize: a failed check keeps both Deployments; the importer is parked while its
  cluster waits for a handover, as today.
- `api/v1`: the `Suspended()` table drops the two reasons.
- `pkg/logicalbackup`: a cluster at `InvalidReference` is not `ClusterSuspended`.

## Implementation breakdown

Two PRs on `fix/suspend-only-for-shared-backends`, sequential:

1. The Lease claim, the key, the label rename, the Optimize label, and their docs.
2. The removal of the pre-check suspension, the narrowed `Suspended()`, and their docs.

The order keeps the two-writer guard closed at every commit: the Lease holds the deleted
contract case before the scale-to-zero that held it is removed.
