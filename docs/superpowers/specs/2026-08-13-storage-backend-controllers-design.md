# Storage backend controllers (Batch B)

**Status:** draft
**Date:** 2026-08-13
**Scope:** ElasticsearchCluster, Database, ElasticsearchClusterPreset (types only), contract-scope rework of SecondaryStorageConfig and DatabaseConfig

## Summary

Implement the two storage backend controllers — the second controller batch from the
[implementation order](../../crds/index.md#implementation-order). `ElasticsearchCluster`
provisions an Elasticsearch cluster through the external ECK operator and publishes a
`SecondaryStorageConfig`; `Database` bootstraps a logical database and users on an existing
PostgreSQL server over plain SQL and publishes a `DatabaseConfig` (and optionally a
`SecondaryStorageConfig`). `ElasticsearchClusterPreset` ships alongside as a passive data CRD
(types + schema validation, no controller). This is the first batch built on the
operator-component-framework (ocf), per the framework posture recorded in the
[Batch A spec](2026-08-02-contract-controllers-design.md).

The batch also carries one deliberate piece of rework: `SecondaryStorageConfig` and
`DatabaseConfig` change from cluster-scoped to namespaced (see "Contract scope").

The authoritative behavioral contract is `docs/crds/<kind>.md` for each kind. This spec does
not restate those documents; it records the decisions about *how* the operator implements
them.

## Goals

- Real `api/v1` types for `ElasticsearchCluster`, `Database`, and `ElasticsearchClusterPreset`,
  field-for-field faithful to their design docs, with schema/CEL admission validation per each
  doc's Validation section.
- An `ElasticsearchCluster` controller that renders and converges an ECK `Elasticsearch` CR,
  provisions a file-realm Camunda user with generated credentials, publishes the
  `SecondaryStorageConfig` binding, reflects ECK health, and supports suspend (scale to zero).
- A `Database` controller that idempotently bootstraps the logical database, application user,
  and backup user over SQL, and publishes the credential Secrets, `DatabaseConfig`, and
  optional `SecondaryStorageConfig`.
- `SecondaryStorageConfig` and `DatabaseConfig` re-scoped to namespaced, with the Batch A
  validation controllers, tests, and docs updated in the same batch.
- Both controllers built on ocf components, with the documented `Ready`/`Suspended` conditions
  preserved as the CR-level contract.
- Test coverage at three levels: unit, envtest (with real ECK CRDs and a real PostgreSQL), and
  kind-based e2e (real ECK operator + Elasticsearch + PostgreSQL).

## Non-goals

- No OpenSearch and no non-PostgreSQL engines — `ElasticsearchCluster` manages Elasticsearch
  only; `Database` bootstraps PostgreSQL-compatible servers only (recorded deviations in the
  CRD docs).
- No deployment of Elasticsearch workloads by this operator: the ECK operator is an external
  prerequisite; we manage only the ECK CR.
- No snapshot repository registration in Elasticsearch — that is the `CamundaCluster`
  controller's job (Batch C); this batch only carries the workload identity through
  `serviceAccount.annotations`.
- No explicit credential-rotation trigger (annotation, field, or schedule) in this batch;
  rotation is delete-the-Secret (see "Generated credentials").
- No `PVCAutoResize` integration beyond applying the documented `camunda.io/cluster` and
  `camunda.io/component` labels to pods and data PVCs.
- No changes to `DatabaseServerConfig`, `ObjectStorageConfig`, or `ManagementAuthConfig`
  beyond doc-comment corrections from the scope rework.
- No dropping of logical databases or SQL users on `Database` deletion — data removal stays a
  deliberate, manual act (per the CRD doc).

## Design

### Contract scope: bindings become namespaced

Batch A shipped all five contract CRDs cluster-scoped. Implementation of the producers exposed
a principled split the original uniform design missed:

- **Shared infra descriptors** — `DatabaseServerConfig`, `ObjectStorageConfig`,
  `ManagementAuthConfig` — describe infrastructure genuinely shared across namespaces and
  clusters. They stay cluster-scoped.
- **Per-consumer bindings** — `SecondaryStorageConfig`, `DatabaseConfig` — bind one storage
  backend to (typically) one consumer. They become namespaced.

What this buys: `ElasticsearchCluster` (namespaced) can own its `SecondaryStorageConfig` with
a normal owner reference — no finalizer, free garbage collection; binding names are unique per
namespace instead of user-chosen global names that collide across tenants; RBAC on bindings is
namespace-local. What it costs: a cross-namespace topology (storage produced in namespace A,
consumed in namespace B) is no longer expressible — the binding is created in the consumer's
namespace instead, which is the better isolation posture anyway.

Reference semantics after the rework: consumer refs to bindings (`storageRef`,
`databaseConfigRef`, `keycloakDbRef`, ...) stay plain strings and resolve **in the consumer's
own namespace**. Refs to cluster-scoped kinds (`serverRef`, `presetRef`) are unchanged.
`SecretKeyRef`/`CredentialsSecretRef` keep `namespace` required everywhere — explicit refs
stay uniform across all five contracts; only the Batch A GoDoc rationale is corrected.

The rework lands first in the batch: scope markers, regenerated CRD manifests, the two
validation controllers' watches/indexes made namespace-aware, envtest updates, and the
affected docs (`secondarystorageconfig.md`, `databaseconfig.md`, `camundacluster.md`,
`camundamanagementcluster.md`, `elasticsearchcluster.md`, `database.md`, `pvcautoresize.md`)
in the same PR.

### Framework posture: ocf components with a hybrid condition model

Both controllers are built on ocf (pinned `v0.18.1`), as Batch A's framework posture
prescribed for controllers that manage resources. Custom-resource primitives (the ECK
`Elasticsearch` CR, our own `SecondaryStorageConfig` and `DatabaseConfig`) are scaffolded with
the `go tool ocf` generator; Secrets use the built-in primitive. Components follow the ocf
guidelines: desired state in the baseline object, pure mutations, one component per logical
condition, thin controllers, registration order = dependency order.

The condition model is layered, matching what ocf is designed for:

- **ocf component conditions** (`CredentialsReady`, `ElasticsearchReady`,
  `StorageContractReady`, `BindingsReady`) appear on each CR's status with ocf's own reason
  vocabulary — per-component operational detail, staged by the components and persisted in one
  status write (`FlushStatus` merges by condition type, so coexisting writers are a supported
  pattern, not a workaround).
- **The documented CR-level conditions** — `Ready` with reasons `Healthy` / `Progressing` /
  `InvalidReference` / `MissingSecret` / `ConnectionFailed` / `Suspended`, plus `Suspended`
  for `ElasticsearchCluster` — are owned by the controller itself, derived from its pre-checks
  and the component conditions, written with Batch A's `pkg/conditions`. The CRD docs remain
  the binding contract; each doc's Status section gains a note that component conditions also
  appear.

Pre-checks run before component reconciliation, exactly like a Batch A validation pass:
unresolved references map to `InvalidReference`, missing/incomplete Secrets to
`MissingSecret`, unreachable or rejecting servers to `ConnectionFailed`. A failed pre-check
short-circuits component reconciliation for that cycle.

### ElasticsearchCluster controller

ECK API types are imported from `github.com/elastic/cloud-on-k8s/v3` (v3.5.0 pins align with
ours: controller-runtime 0.24.1, k8s.io 0.36.x, Go 1.26) — compile-time-safe rendering,
wrapped as an ocf custom-resource primitive.

Preset resolution happens before component building: resolve `spec.presetRef` (dangling →
`InvalidReference`), overlay inline pointer fields wholesale (an inline `scheduling` block
replaces the preset's entirely — never merged field-by-field), then check merged completeness
(`version`, `replicas`, `storageSize` present — a cross-resource rule the schema cannot
enforce; failure → `InvalidReference` with a message naming the missing fields). The merge is
a pure function, unit-tested exhaustively. `ElasticsearchClusterPreset.spec.cluster` reuses
the `ElasticsearchCluster` spec type; CEL forbids the instance-bound fields (`presetRef`,
`secondaryStorageConfig`, `suspend`, `monitoring`) inside a preset.

Three components, in dependency order:

1. **`credentials`** (`CredentialsReady`) — the file-realm user Secret (basic-auth style,
   consumed by ECK's `spec.auth.fileRealm`) carrying the generated Camunda user credentials
   and role grant.
2. **`elasticsearch`** (`ElasticsearchReady`) — the ECK `Elasticsearch` CR: version, one
   nodeSet (replicas, resources, volumeClaimTemplates with `storageSize`/`storageClassName`),
   podTemplate carrying the `camunda.io/cluster` and `camunda.io/component: elasticsearch`
   labels on pods and data PVCs, `podLabels`/`podAnnotations`, service-account annotations
   (workload identity for the snapshot bucket), `extraEnv`/`extraEnvFrom`, scheduling
   constraints, and the fileRealm auth block. ECK's reported health
   (`status.health`/`phase`) drives this component's condition. `spec.suspend` maps to ocf
   suspension scaling the nodeSet to zero. The optional ServiceMonitor is a resource in this
   component behind a resource-level gate on `monitoring.serviceMonitor.enabled`.
3. **`storage-contract`** (`StorageContractReady`) — the namespaced `SecondaryStorageConfig`
   (`type: elasticsearch`, the ECK service's in-cluster HTTPS endpoint, credentials Secret
   ref), owner-referenced, guarded on the credentials Secret existing.

All applies are SSA under field manager `camunda-operator/elasticsearchcluster`. The
`storageSize` no-shrink rule is CEL on the CRD (`self >= oldSelf` on the quantity) for the
inline field; shrink-via-preset-edit is caught by the controller comparing against the applied
ECK CR and reported as a `Ready: False` condition rather than silently applied.

### Database controller

The SQL layer lives outside ocf in its own package (`pkg/pgbootstrap`): a small pgx-based
bootstrap with idempotent operations — ensure database exists, ensure role with password,
grant application privileges, ensure backup role (read access on all tables plus the DDL and
write rights restore needs), ensure ownership/grants converged. Identifiers are validated and
quoted defensively; every operation is safe to re-run. The package exposes a narrow interface
and is tested against a real PostgreSQL via testcontainers.

Reconcile flow:

1. Pre-checks: `spec.serverRef` resolves to a `DatabaseServerConfig` (else
   `InvalidReference`); its admin credentials Secret exists with the expected keys (else
   `MissingSecret`); the same-`serverRef`-same-`databaseName` collision rule is checked via a
   field index (first creation wins; later collider reports `Ready: False`) — a cross-CR rule
   the schema cannot enforce.
2. Connect with the admin credentials; failure or rejection → `ConnectionFailed`.
3. Run the idempotent SQL bootstrap for the database, application user, and (unless disabled)
   backup user.
4. Reconcile one ocf component, **`bindings`** (`BindingsReady`): the application-credentials
   Secret, the backup-credentials Secret (resource-level gate on
   `backupCredentials.disabled`), the `DatabaseConfig`, and the `SecondaryStorageConfig`
   (`type: rdbms`; resource-level gate on `spec.secondaryStorageConfig` being set) — all in
   `targetNamespace`. One component: none of these has useful readiness independent of the
   others.

`spec.targetNamespace` is **required** (a recorded deviation from the original doc's
"optional, defaults to the operator namespace"): consumers resolve bindings by name in their
own namespace, so an operator-namespace default would place bindings where no consumer can
ever find them.

`Database` is cluster-scoped and its children are namespaced. Kubernetes owner references
permit a namespaced dependent to name a cluster-scoped owner, so the bindings and credential
Secrets carry a normal owner reference to the `Database` and are garbage-collected on CR
deletion — no finalizer. The logical database and SQL users are never touched by deletion.
All applies are SSA under field manager `camunda-operator/database`.

(An earlier draft of this spec claimed the opposite — that owner references cannot cross this
boundary — and prescribed a finalizer. That premise was wrong: only the reverse direction, a
namespaced owner of a cluster-scoped dependent, is disallowed.)

### Generated credentials

Both controllers generate passwords (crypto/rand) and treat them as stable once created: on
every reconcile the existing Secret is read (uncached, via the manager's `APIReader`, matching
the Batch A pattern) and reused. Deleting a credentials Secret is the rotation mechanism: a
new password is generated and converged backend-first — `ALTER ROLE` before the Secret write
for PostgreSQL; the fileRealm Secret update for Elasticsearch (ECK propagates it) — so a
published Secret never names a password the backend does not know. Secrets are SSA-owned by
the controllers; manual edits are reverted on the next reconcile.

### Watches

The Batch A watch machinery (`pkg/refindex` field indexes, metadata-only Secret watches with
targeted `APIReader` reads) extends to the new controllers: `ElasticsearchCluster` watches its
owned resources (via ocf/owner refs), `ElasticsearchClusterPreset` (spec changes flow to
referencing clusters), and the ECK CR's status; `Database` watches `DatabaseServerConfig`, the
admin credentials Secret (metadata-only), and its owner-referenced children. RBAC additions:
full management of `elasticsearch.k8s.elastic.co/elasticsearches`, create/update on `secrets`
(previously read-only), and the ServiceMonitor kind gated on its CRD being present.

### Testing

- **Unit (testify):** the preset merge, ECK CR rendering (golden manifests per ocf's
  `pkg/testing/golden` conventions), SQL statement construction and identifier quoting,
  condition derivation, collision-index logic.
- **envtest (Ginkgo):** both controllers under a live manager. ECK CRDs are vendored into the
  envtest setup so the rendered `Elasticsearch` CR actually applies against the API server —
  ECK itself does not run; specs assert the rendered CR and drive its status to simulate
  health transitions. `Database` envtest specs run against a testcontainers PostgreSQL, so
  reconciliation is tested against real SQL. Watch-driven re-reconciliation is proven the
  Batch A way (change the referenced object, never the CR).
- **e2e (kind):** the existing suite gains the ECK operator, a single-node Elasticsearch, and
  a PostgreSQL pod; asserts an `ElasticsearchCluster` and a `Database` reach
  `Ready: Healthy` end-to-end with usable published bindings (the generated credentials
  authenticate against the real backends).
- Everything below e2e runs under `make test` (testcontainers needs Docker, already present in
  CI); e2e stays in its own job.

## Risks

- **ECK behavioral surface** — rendering the ECK CR is typed and golden-tested, but ECK's
  *runtime* reaction (fileRealm propagation, nodeSet scale-to-zero on suspend, health
  reporting during upgrades) is only proven in e2e. Mitigation: the e2e suite lands with the
  controller, not after it.
- **Scope rework ripples** — re-scoping two shipped CRDs touches Batch A controllers, tests,
  and seven docs; a missed reference surfaces as a confusing resolution failure. Mitigation:
  the rework is an isolated first PR with the full test suite as the net.
- **testcontainers in CI** — a new CI dependency (Docker-in-CI for `make test`); flakiness
  here blocks every PR. Mitigation: container reuse across specs, and the SQL suite is
  skippable via build tag in constrained environments while remaining mandatory in CI.
- **ocf first adoption** — first real use of components/primitives/generator in this repo;
  conventions set here propagate to Batches C and D. Mitigation: the `ocf:review` checklist
  runs on every controller PR.

## Alternatives considered

- **Keep all contracts cluster-scoped** — rejected: for binding-shaped contracts it forces
  finalizers over owner references, invites global name collisions, and models a
  cross-namespace topology nobody asked for. The shared-infra contracts keep cluster scope.
- **Extend ocf with free-form condition reasons** — rejected: unnecessary. `FlushStatus`
  merges conditions by type, so the controller owning the documented `Ready` alongside ocf's
  component conditions is the framework's intended layering, not a workaround.
- **Minimal local mirror or raw unstructured for the ECK CR** — rejected: ECK v3.5.0's
  dependency pins align with ours, so the official types cost nothing and buy compile-time
  safety for the largest rendered object in the operator so far.
- **SQL behind an interface with a fake, real PostgreSQL only in e2e** — rejected: the SQL
  layer's actual behavior (idempotency, quoting, grants) is the risk; testcontainers tests it
  where it lives, cheaply.
- **Presets deferred to a later batch** — rejected: `presetRef` resolution is core
  `ElasticsearchCluster` behavior and its absence would ship a knowingly broken field.
