# Storage Backend Controllers (Batch B) Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Implement the `ElasticsearchCluster` and `Database` controllers (plus `ElasticsearchClusterPreset` types) on the operator-component-framework, after re-scoping the two binding contract CRDs to namespaced.

**Architecture:** Thin reconcilers run reference/secret pre-checks and derive the documented CR-level `Ready`/`Suspended` conditions; ocf components own all managed resources (ECK `Elasticsearch` CR, credential Secrets, binding CRs) and report per-component conditions. `Database` additionally drives an idempotent SQL bootstrap layer that lives outside ocf.

**Tech Stack:** Go 1.26, controller-runtime 0.24.1, ocf v0.18.1 (`go tool ocf` generator), `github.com/elastic/cloud-on-k8s/v3` v3.5.0 (types only), `github.com/jackc/pgx/v5`, `github.com/testcontainers/testcontainers-go` (postgres module), Ginkgo/Gomega envtest, testify.

**Spec:** `docs/superpowers/specs/2026-08-13-storage-backend-controllers-design.md`

## Global Constraints

- SSA exclusively; component-managed resources carry ocf's derived field managers `<OwnerKind>/<component>` (e.g. `ElasticsearchCluster/elasticsearch`, `Database/bindings` — ocf hardcodes this, no override hook); controller status writes and the Batch A validation controllers keep `camunda-operator`.
- Docs bind the implementation: `docs/crds/<kind>.md` is the contract; if implementation reveals a doc gap, correct the doc in the same PR.
- Every exported symbol gets GoDoc; `make all` clean; no `t.Fatal` (testify asserts/requires); Ginkgo+gomega only for reconciliation-level specs.
- ocf guidelines apply to all components (baseline desired state, pure mutations, one component per logical condition, registration order = dependency order). Verify exact ocf/ECK signatures with `go doc` before use — plan snippets are shape, not gospel.
- Condition reasons at CR level are exactly the documented vocabulary: `Healthy`, `Progressing`, `InvalidReference`, `MissingSecret`, `ConnectionFailed`, `Suspended`.
- Commits reference their sub-issue: `feat(controller): ... (#37)`.
- Sub-PR base branch: `feat/storage-backend-controllers`. PR order: #35 → #36 → (#37 ∥ #38) → #39.

## Contracts

All code-shaped contracts are realized by the foundations PR (#36) merging into the feature branch before #37/#38 branch off (same model as Batch A).

| Name | Producer | Consumer | Shape | Realization |
| --- | --- | --- | --- | --- |
| `binding-scope` | #35 | #36, #37, #38 | `SecondaryStorageConfig` + `DatabaseConfig` namespaced; binding refs resolve in the consumer's namespace; `ElasticsearchStorage` gains optional `CASecretRef *SecretKeyRef` | pre-merge PR #35 |
| `eck-wrapper` | #36 | #37 | package `pkg/wrappers/eckelasticsearch`, generated `--variant workload`; builder `NewBuilder(obj *esv1.Elasticsearch)` per ocf template | foundations PR |
| `binding-wrappers` | #36 | #37, #38 | packages `pkg/wrappers/secondarystorageconfig`, `pkg/wrappers/databaseconfig`, generated `--variant static`; builders `NewBuilder(obj *corev1api.SecondaryStorageConfig)` etc. | foundations PR |
| `credentials-api` | #36 | #37, #38 | `pkg/credentials`: `func NewPassword() (string, error)` — 32-char `[a-zA-Z0-9]`, crypto/rand; `func Lookup(ctx, r client.Reader, key client.ObjectKey, field string) (string, bool, error)` — read existing Secret value for stable-once-created semantics | foundations PR |
| `ready-derivation` | #36 | #37, #38 | `pkg/conditions`: `type PreCheckFailure struct { Reason string; Message string }`; `func DeriveReady(pre *PreCheckFailure, componentConds []metav1.Condition, suspended bool) (reason, message string)` — pre-check failure wins, else `Suspended`, else any component not-True → `Progressing` (message names it), else `Healthy` | foundations PR |
| `api-types` | #36 | #37, #38 | `api/v1` types below (Tasks 3–4) — field names and Go types are frozen once #36 merges | foundations PR |
| `e2e-flows` | #37, #38 | #39 | the Verification bullets of #37/#38 (CR name/spec shapes from the CRD docs' minimal examples) | data-only |

## Conventions

- File layout per controller (mirrors Batch A): `internal/controller/<kind>_controller.go` (reconciler + pre-checks), `<kind>_components.go` (pure component-building functions), `<kind>_controller_test.go` (Ginkgo envtest), `<kind>_schema_test.go` (admission specs), plus feature-specific files (`elasticsearchcluster_presetmerge.go`, `database_collision.go`) with sibling `_test.go` testify tables.
- Component names: lowercase single words (`credentials`, `elasticsearch`, `storage-contract`, `bindings`). Condition types: `CredentialsReady`, `ElasticsearchReady`, `StorageContractReady`, `BindingsReady`.
- ocf mutation names: PascalCase descriptive (`SuspendNodeSet`, `SchedulingConstraints`) — golden manifests reference them.
- Golden manifests: `internal/controller/testdata/golden/<kind>/…` via ocf `pkg/testing/golden`.
- Generated wrappers live under `pkg/wrappers/<package>`; regenerate with `go tool ocf scaffold wrapper … --force`, never hand-edit generated files; hand-written status handlers sit next to them in a separate file.
- Error wrapping: `%w` everywhere except `IndexField` errors (Batch A recorded unwrapped as deliberate — stay consistent).
- Import alias: `corev1 "k8s.io/api/core/v1"` is aliased `v1` in Batch A controller files — follow the file you're editing; new files use `corev1`. ECK types: `esv1 "github.com/elastic/cloud-on-k8s/v3/pkg/apis/elasticsearch/v1"`.
- envtest loads ECK CRDs from the module dir (`go list -m -f '{{.Dir}}' github.com/elastic/cloud-on-k8s/v3` + `/config/crds`) — never vendored copies.
- The ES file-realm Secret is basic-auth style (`username`/`password`/`roles` keys — ECK hashes it); the Camunda user's role is `superuser` in this batch (snapshot-repo registration in Batch C needs cluster-manage; revisit narrowing then — noted in `elasticsearchcluster.md`).

---

## PR #35 — Contract scope rework

### Task 1: Re-scope the binding CRDs and add the ES CA reference

**Files:**
- Modify: `api/v1/secondarystorageconfig_types.go`, `api/v1/databaseconfig_types.go` (drop `+kubebuilder:resource:scope=Cluster`; GoDoc: bindings are namespaced, refs resolve in the consumer's namespace; add `CASecretRef *SecretKeyRef` with GoDoc to `ElasticsearchStorage`)
- Modify: `api/v1/common_types.go` (GoDoc on `SecretKeyRef`/`CredentialsSecretRef`: correct the "all consumers are cluster-scoped" rationale — namespace stays required for uniform explicit refs)
- Modify: `internal/controller/secondarystorageconfig_controller.go`, `databaseconfig_controller.go` (indexes/watch mappings become namespace-aware: index key `<namespace>/<name>`, enqueue uses the event object's namespace)
- Test: existing `internal/controller/{secondarystorageconfig,databaseconfig}_controller_test.go` and `_schema_test.go`

- [ ] **Step 1:** Update both `_controller_test.go` suites first: create the CRs in a namespace; add a spec proving a dangling `databaseConfigRef` in namespace A is **not** satisfied by a same-named `DatabaseConfig` in namespace B (expect `Ready: False`, `InvalidReference`); add a `caSecretRef` round-trip to the SSC schema specs. Run `go test ./internal/controller/...` — expect failures (types still cluster-scoped).
- [ ] **Step 2:** Apply the type changes, run `make manifests generate`, confirm `scope: Namespaced` for exactly the two kinds in `config/crd/bases/`.
- [ ] **Step 3:** Make the two controllers' `IndexField` extractors and `Enqueue` mappings namespace-aware. Reference resolution reads the binding in `req.Namespace`.
- [ ] **Step 4:** `make test` green. Commit: `feat(api)!: namespace the binding contract CRDs (#35)`.

### Task 2: Doc sweep for binding scope

**Files:**
- Modify: `docs/crds/secondarystorageconfig.md` (namespaced; `caSecretRef` documented in the API reference and the elasticsearch example), `databaseconfig.md`, `camundacluster.md` (`storageRef` = same-namespace name), `camundamanagementcluster.md` (`*DbRef` same-namespace), `elasticsearchcluster.md` (SSC in own namespace, owner-ref GC, no finalizer; superuser-role note), `database.md` (bindings land in `targetNamespace`), `pvcautoresize.md` (traversal wording)

- [ ] **Step 1:** Update all seven docs; every "cluster-scoped" claim about the two bindings removed; reference-resolution semantics stated once per consumer field.
- [ ] **Step 2:** `grep -ri 'cluster-scoped' docs/crds/{secondarystorageconfig,databaseconfig}.md` returns nothing. Commit: `docs: binding contracts are namespaced (#35)`.

---

## PR #36 — Foundations

### Task 3: ElasticsearchCluster + ElasticsearchClusterPreset API types

**Files:**
- Create: `api/v1/elasticsearchcluster_types.go` (replace stub), `api/v1/elasticsearchclusterpreset_types.go` (replace stub)
- Test: `internal/controller/elasticsearchcluster_schema_test.go`, `elasticsearchclusterpreset_schema_test.go`

**Interfaces (produces, frozen by this PR):**

```go
type ElasticsearchClusterSpec struct {
    PresetRef              string                       `json:"presetRef,omitempty"`
    Version                string                       `json:"version,omitempty"`      // pattern ^\d+\.\d+\.\d+$
    Replicas               *int32                       `json:"replicas,omitempty"`     // Minimum=1
    Resources              *corev1.ResourceRequirements `json:"resources,omitempty"`
    StorageSize            *resource.Quantity           `json:"storageSize,omitempty"`  // XValidation quantity no-shrink, optionalOldSelf
    StorageClassName       *string                      `json:"storageClassName,omitempty"`
    ServiceAccount         *ServiceAccountSpec          `json:"serviceAccount,omitempty"` // Annotations map[string]string
    ExtraEnv               []corev1.EnvVar              `json:"extraEnv,omitempty"`
    ExtraEnvFrom           []corev1.EnvFromSource       `json:"extraEnvFrom,omitempty"`
    PodLabels              map[string]string            `json:"podLabels,omitempty"`
    PodAnnotations         map[string]string            `json:"podAnnotations,omitempty"`
    Scheduling             *SchedulingSpec              `json:"scheduling,omitempty"` // NodeAffinity, PodAffinity, Tolerations
    SecondaryStorageConfig string                       `json:"secondaryStorageConfig"` // required, DNS-1123
    Monitoring             *MonitoringSpec              `json:"monitoring,omitempty"`   // ServiceMonitor{Enabled,Labels,Annotations}
    Suspend                bool                         `json:"suspend,omitempty"`
}
type ElasticsearchClusterPresetSpec struct {
    Cluster ElasticsearchClusterSpec `json:"cluster"` // CEL: !has presetRef/secondaryStorageConfig/suspend/monitoring
}
```

Status types follow the Batch A pattern (`Conditions`, `ObservedGeneration`). The 8.19+/9.2+ version floor is controller-side (Task 9's merge-completeness check), not CEL — the schema pins only the semver pattern.

- [ ] **Step 1:** Write both `_schema_test.go` suites first (Batch A style): valid minimal + realistic fixtures accepted; each Validation-doc rule rejected (preset with `presetRef` set inside `cluster`; `storageSize` shrink on update; bad `secondaryStorageConfig` name; bad version string). `valid<Kind>()` fixture helpers exported to the package for reuse by controller specs.
- [ ] **Step 2:** Implement types + markers, `make manifests generate`, run the suites green.
- [ ] **Step 3:** Commit: `feat(api): ElasticsearchCluster and preset types and schema validation (#36)`.

### Task 4: Database API types

**Files:**
- Create: `api/v1/database_types.go` (replace stub)
- Test: `internal/controller/database_schema_test.go`

**Interfaces (produces):**

```go
type DatabaseSpec struct {
    ServerRef              string           `json:"serverRef"`     // MinLength=1
    DatabaseName           string           `json:"databaseName"`  // pattern ^[a-z_][a-z0-9_]{0,62}$
    TargetNamespace        string           `json:"targetNamespace"` // required (recorded doc deviation): consumers resolve bindings in their own namespace, so no reachable default exists
    ApplicationCredentials *CredentialsSpec `json:"applicationCredentials,omitempty"` // SecretName, SecretNamespace
    BackupCredentials      *BackupCredentialsSpec `json:"backupCredentials,omitempty"` // + Disabled bool
    DatabaseConfig         string           `json:"databaseConfig,omitempty"`          // default: CR name
    SecondaryStorageConfig string           `json:"secondaryStorageConfig,omitempty"`  // optional
}
```

`Database` stays cluster-scoped (`+kubebuilder:resource:scope=Cluster`). Defaults that need the CR name are resolved in the controller, not the schema; GoDoc states each default. `targetNamespace` has no default — it is required (schema `MinLength=1`), and `database.md` records the deviation.

- [ ] **Step 1:** `database_schema_test.go` first: minimal + realistic accepted; bad `databaseName` (uppercase, leading digit, 64 chars) rejected; missing `serverRef` rejected.
- [ ] **Step 2:** Implement, regenerate, suites green. Commit: `feat(api): Database types and schema validation (#36)`.

### Task 5: ocf wrappers via the generator

**Files:**
- Create (generated): `pkg/wrappers/eckelasticsearch/`, `pkg/wrappers/secondarystorageconfig/`, `pkg/wrappers/databaseconfig/`
- Create (hand-written): status handler files per wrapper (`health.go`) where the variant requires one
- Test: `pkg/wrappers/<pkg>/health_test.go` for hand-written logic only

- [ ] **Step 1:** Generate:

```bash
go tool ocf scaffold wrapper --type github.com/elastic/cloud-on-k8s/v3/pkg/apis/elasticsearch/v1.Elasticsearch \
  --variant workload --group elasticsearch.k8s.elastic.co --package eckelasticsearch --out pkg/wrappers/eckelasticsearch
go tool ocf scaffold wrapper --type github.com/konsole-is/camunda-operator/api/v1.SecondaryStorageConfig \
  --variant static --group core.camunda.io --package secondarystorageconfig --out pkg/wrappers/secondarystorageconfig
go tool ocf scaffold wrapper --type github.com/konsole-is/camunda-operator/api/v1.DatabaseConfig \
  --variant static --group core.camunda.io --package databaseconfig --out pkg/wrappers/databaseconfig
```

- [ ] **Step 2:** Implement the ECK wrapper's `Alive`/`Suspendable` handlers per the ocf custom-resource docs (`/ocf:custom-resource-wrappers` skill): health maps `status.health` green→Healthy, yellow while converging→Creating/Updating, red→AliveFailing; suspend sets nodeSet `count: 0` and reports `Suspended` when `status.availableNodes == 0`. testify tables for both mappings.
- [ ] **Step 3:** `make test` green; add ECK scheme registration to `cmd/main.go` scheme init. Commit: `feat(pkg): ocf wrappers for ECK Elasticsearch and binding CRDs (#36)`.

### Task 6: credentials and Ready-derivation helpers

**Files:**
- Create: `pkg/credentials/credentials.go` + `credentials_test.go`
- Modify: `pkg/conditions/conditions.go` + `conditions_test.go` (add `PreCheckFailure`, `DeriveReady` per the Contracts table)

- [ ] **Step 1:** testify tables first: `NewPassword` length/charset/uniqueness; `Lookup` hit/miss/missing-key; `DeriveReady` — pre-check failure wins; suspended wins over converging; one False component → `Progressing` naming it; all True → `Healthy`.
- [ ] **Step 2:** Implement; `make test` green. Commit: `feat(pkg): credential generation and Ready derivation (#36)`.

### Task 7: controller scaffolds, RBAC, suite wiring

**Files:**
- Modify: `internal/controller/elasticsearchcluster_controller.go`, `database_controller.go` (real reconciler structs, empty reconcile bodies returning early; RBAC markers), `internal/controller/suite_test.go` (register both; load ECK CRDs from the module dir), `cmd/main.go`, `test/utils` if the ECK CRD path helper needs a home

RBAC markers: full verbs on `elasticsearches.elasticsearch.k8s.elastic.co`; `secrets` gains `create;update;patch;delete` for the two new controllers; `monitoring.coreos.com/servicemonitors` full verbs; get/list/watch on presets and `databaseserverconfigs`.

- [ ] **Step 1:** suite change: `testEnv.CRDDirectoryPaths` appends the ECK `config/crds` dir resolved from the module cache; assert the ECK CRD is queryable in a smoke spec.
- [ ] **Step 2:** `make manifests` regenerates `config/rbac/role.yaml`; `make test` green. Commit: `feat(controller): scaffolds, RBAC and ECK-aware envtest (#36)`.

---

## PR #37 — ElasticsearchCluster controller

### Task 8: preset merge

**Files:**
- Create: `internal/controller/elasticsearchcluster_presetmerge.go` + `_presetmerge_test.go`

**Interfaces (produces):** `func mergePreset(spec ElasticsearchClusterSpec, preset *ElasticsearchClusterPresetSpec) ElasticsearchClusterSpec` and `func validateMerged(spec ElasticsearchClusterSpec) error` (error message lists every missing required field and enforces the 8.19+/9.2+ version floor).

- [ ] **Step 1:** Exhaustive testify tables first: nil preset passthrough; each field inherited when unset; each field overridden wholesale when set inline; `scheduling` block replaced entirely (preset tolerations dropped when inline sets only nodeAffinity); merged-missing-fields message lists all of `version`, `replicas`, `storageSize`; version floor rejects 8.18.0 and 9.1.9, accepts 8.19.0 and 9.2.4.
- [ ] **Step 2:** Implement (pure functions, no client). Commit: `feat(controller): ElasticsearchCluster preset merge (#37)`.

### Task 9: components

**Files:**
- Create: `internal/controller/elasticsearchcluster_components.go` + `_components_test.go` (golden)

Three pure builder functions consuming the merged spec, assembled in the reconciler as ocf components in this registration order:

1. `credentials`: Secret `<name>-es-user` (keys `username: camunda`, `password` from `credentials.NewPassword()` unless `credentials.Lookup` found one, `roles: superuser`), built with the ocf secret primitive.
2. `elasticsearch`: `esv1.Elasticsearch` named `<name>`: version, one nodeSet `default` with `count: replicas`, resources, volumeClaimTemplates (`storageSize`/`storageClassName`), podTemplate labels `camunda.io/cluster: <name>` + `camunda.io/component: elasticsearch` (+ `podLabels`), annotations, SA annotations via podTemplate, `extraEnv`/`extraEnvFrom`, scheduling, `spec.auth.fileRealm: [{secretName: <name>-es-user}]`. Suspension via `Suspend(spec.Suspend)` on the builder + the wrapper's nodeSet-zero handler. ServiceMonitor resource behind `component.GatedBy` on `monitoring.serviceMonitor.enabled`.
3. `storage-contract`: `SecondaryStorageConfig` named `spec.secondaryStorageConfig` in the CR's namespace: `type: elasticsearch`, endpoint `https://<name>-es-http.<ns>.svc:9200`, `credentialsSecretRef` → the user Secret, `caSecretRef` → `{name: <name>-es-http-certs-public, namespace: <ns>, key: ca.crt}`; guarded on the credentials Secret existing.

- [ ] **Step 1:** Golden tests first (ocf `pkg/testing/golden`): rendered ECK CR and SSC pinned for a minimal and a realistic fixture (reuse `validElasticsearchCluster()`), plus a suspended variant.
- [ ] **Step 2:** Implement builders; goldens green. Commit: `feat(controller): ElasticsearchCluster components (#37)`.

### Task 10: reconciler

**Files:**
- Modify: `internal/controller/elasticsearchcluster_controller.go`

Flow: fetch CR (ignore not-found) → resolve preset via APIReader (`InvalidReference` on dangling) → `validateMerged` (`InvalidReference`, message from the merge validator) → build + `Reconcile` the three components with a `ReconcileContext` → deferred status flush writes component conditions plus `Ready` from `conditions.DeriveReady` and the `Suspended` condition, and `status.observedGeneration`, via SSA. Watches: owned Secret/ECK CR/SSC, plus preset→clusters via a field index on `spec.presetRef`. Detect preset-driven `storageSize` shrink by comparing against the applied ECK CR's claim size; report `Ready: False` (`InvalidReference`, message names the shrink) instead of applying.

- [ ] **Step 1:** Wire it; `make test` green (existing scaffold specs still pass). Commit: `feat(controller): ElasticsearchCluster reconciler (#37)`.

### Task 11: envtest specs

**Files:**
- Modify: `internal/controller/elasticsearchcluster_controller_test.go` (replace smoke test)

Specs (Ginkgo, driving ECK status by patching the ECK CR since no ECK operator runs): create → ECK CR + user Secret + SSC exist with expected wiring, `Ready: Progressing`; patch ECK status to green/available → `Ready: Healthy`; dangling preset → `InvalidReference`; incomplete merge → message names missing fields; `suspend: true` → nodeSet count 0, `Suspended` condition, `Ready: Suspended`; delete credentials Secret → new password appears and SSC still points at it; preset edit flows to the cluster (watch path — never touch the CR); `observedGeneration` tracks.

- [ ] **Step 1:** Write and pass the specs. `make all` clean. Commit: `test(controller): ElasticsearchCluster reconciliation specs (#37)`.

---

## PR #38 — Database controller

### Task 12: pgbootstrap package

**Files:**
- Create: `pkg/pgbootstrap/pgbootstrap.go`, `identifiers.go`, `pgbootstrap_test.go` (testcontainers; build tag `//go:build !no_docker`), `identifiers_test.go` (pure)

**Interfaces (produces):**

```go
type Connection struct{ Host string; Port int32; AdminUser, AdminPassword, SSLMode string }
type Bootstrapper interface {
    EnsureDatabase(ctx context.Context, name string) error
    EnsureUser(ctx context.Context, name, password string) error      // create or ALTER ROLE password
    GrantApplication(ctx context.Context, user, database string) error
    EnsureBackupUser(ctx context.Context, name, password, database string) error
    Ping(ctx context.Context) error
    Close()
}
func Connect(ctx context.Context, c Connection) (Bootstrapper, error)
```

Identifiers pass `^[a-z_][a-z0-9_]{0,62}$` and are quoted with `pgx.Identifier`; passwords go through query parameters or `quoteLiteral`, never string concatenation into DDL where parameters are unsupported.

- [ ] **Step 1:** `identifiers_test.go` pure tables first (accept/reject/quoting).
- [ ] **Step 2:** testcontainers suite: shared postgres:17 container per test binary; every `Ensure*` idempotent (run twice, no error, same result); app user can CRUD in its database, cannot connect to another; backup user reads all tables incl. ones created after the grant (`ALTER DEFAULT PRIVILEGES`), and holds restore rights; `EnsureUser` with a new password takes effect (reconnect with it).
- [ ] **Step 3:** Implement; `make test` green (Docker present). Commit: `feat(pkg): idempotent PostgreSQL bootstrap layer (#38)`.

### Task 13: pre-checks and collision index

**Files:**
- Create: `internal/controller/database_collision.go` + `_collision_test.go`
- Modify: `internal/controller/database_controller.go`

Field index on `Database` by `spec.serverRef + "/" + spec.databaseName`; collision resolution: oldest `creationTimestamp` (name as tiebreaker) wins; later CRs get `Ready: False` with a message naming the winner. Pre-checks: `serverRef` → `DatabaseServerConfig` via APIReader (`InvalidReference`); admin Secret keys via `secretref.CheckKeys` (`MissingSecret`); `pgbootstrap.Connect` + `Ping` (`ConnectionFailed`).

- [ ] **Step 1:** testify tables for the collision decision (single, two ordered by time, tie on time ordered by name).
- [ ] **Step 2:** Wire pre-checks; commit: `feat(controller): Database pre-checks and collision rule (#38)`.

### Task 14: bindings component and reconciler

**Files:**
- Create: `internal/controller/database_components.go` + `_components_test.go` (golden)
- Modify: `internal/controller/database_controller.go`

Flow after pre-checks: resolve defaults (`databaseConfig` → CR name, Secret names per doc defaults; `targetNamespace` is required, no default) → passwords via `credentials.Lookup`-else-`NewPassword` → SQL bootstrap (`EnsureDatabase`, `EnsureUser`+`GrantApplication`, `EnsureBackupUser` unless disabled) — always before Secret writes → one `bindings` component: app Secret, backup Secret (`GatedBy` not-disabled), `DatabaseConfig` (serverRef, databaseName, both credential refs), `SecondaryStorageConfig` type `rdbms` (`GatedBy` field set; `databaseConfigRef` → the DatabaseConfig, same namespace). All four children carry an owner reference to the `Database` (a namespaced dependent may name a cluster-scoped owner — legal, GC-honored) and are garbage-collected on CR deletion; no finalizer. SQL objects are never touched by deletion. Verify with `go doc` that the ocf/controllerutil owner-ref path accepts the cluster-scoped owner; if ocf declines to set it, set the owner reference in the resource baseline.

- [ ] **Step 1:** Golden tests for the rendered bindings (minimal + full fixtures).
- [ ] **Step 2:** Implement components + owner-referenced children + Ready derivation (same pattern as Task 10). Commit: `feat(controller): Database bindings component (#38)`.

### Task 15: envtest + testcontainers specs

**Files:**
- Modify: `internal/controller/database_controller_test.go` (replace smoke test)

Specs run against envtest plus the shared testcontainers postgres (a `DatabaseServerConfig` fixture points at the container): happy path → SQL objects exist, Secrets/DatabaseConfig/SSC exist in `targetNamespace`, `Ready: Healthy`; each pre-check failure → documented reason (stop the container's listener for `ConnectionFailed` via wrong port fixture); collision → loser reports the winner; every binding and Secret carries the owner reference to the `Database` (envtest runs no garbage collector, so cascade deletion itself is proven in e2e — here assert the ownerReferences and that deletion leaves database + users intact over SQL); backup disabled → no backup Secret and DatabaseConfig omits the ref; deleted app Secret → new password works against the server.

- [ ] **Step 1:** Write and pass. `make all` clean. Commit: `test(controller): Database reconciliation specs (#38)`.

---

## PR #39 — e2e

### Task 16: kind e2e flows

**Files:**
- Modify: `test/e2e/e2e_test.go` (+ helpers in `test/utils/`), `.github/workflows/` e2e job, `Makefile` (`test-e2e` prerequisites)

Setup installs ECK (pinned operator manifest version, documented in the Makefile variable) and a `postgres:17` Deployment+Service with a known admin Secret. Flow assertions per issue #39: `ElasticsearchCluster` (1 replica, small resources) reaches `Ready: Healthy`; SSC endpoint + generated credentials authenticate (curl from an in-cluster pod using the CA from `caSecretRef`); suspend → 0 ES pods → resume; `Database` reaches `Ready: Healthy`; both SQL users authenticate with documented privileges; CR deletion garbage-collects the bindings and Secrets (owner refs, real GC runs here) while the logical database survives.

- [ ] **Step 1:** Implement helpers + flows; `make test-e2e` green on a fresh kind cluster locally.
- [ ] **Step 2:** CI job updated (ECK install step, memory for one ES node). Commit: `test(e2e): storage backend flows against real ECK and PostgreSQL (#39)`.

---

## Integration

### Task 17: integration checkpoint

- [ ] **Step 1:** Docs drift check: every behavior doc claim for the three kinds matches implementation; README operator prerequisites gain ECK (external) and the testcontainers/Docker note for `make test`.
- [ ] **Step 2:** `make all` + full suite green on the feature branch; integration PR `feat/storage-backend-controllers` → `main` with `Closes #34`; leave ready for user review (never self-merge to main).
