# CamundaOptimize Controller Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Implement the `CamundaOptimize` CRD and controller: two Deployments (webapp + importer) attached to one `CamundaCluster`, an SSA patch that turns on the legacy Zeebe Elasticsearch exporter, and a full data-flow e2e test.

**Architecture:** Extension pattern. The controller resolves `clusterRef` → `CamundaCluster` → `storageRef` → `SecondaryStorageConfig` (must be `elasticsearch`) and `managementAuthRef` → `ManagementAuthConfig`. It patches only the exporter env vars on `spec.zeebe.extraEnv` with its own field manager, deploys the workloads through ocf components, and flushes status once per reconcile.

**Tech Stack:** Go 1.26, Kubebuilder, controller-runtime, ocf v0.19.1 (`github.com/sourcehawk/operator-component-framework`), Ginkgo/Gomega (envtest, e2e), testify (unit).

**Spec:** `docs/superpowers/specs/2026-08-20-optimize-controller-design.md`

**Tracking:** epic #114; PR 1 → #115, PR 2 → #116, PR 3 → #117. Commit subjects carry the sub-issue ref, for example `feat(api): add the CamundaOptimize types (#115)`.

## Global Constraints

- Load skills before you act: `how-we-write-go` (any Go), `simple-english:simple-english` (any prose: GoDoc, CRD field descriptions, condition messages, docs), `verifying-camunda-app-config` (any Camunda env var), `ocf:building-components` / `ocf:using-primitives` / `ocf:testing-operators` (component work). Verify every Camunda config key against the `camunda-docs` MCP server; never from memory.
- Apply managed resources with SSA. Write status once per reconcile through ocf `FlushStatus`. Never write status with SSA.
- Never hand-edit `config/crd/bases/*`, `config/rbac/role.yaml`, `zz_generated.*`, `PROJECT`. After type changes: `make manifests generate`. After Go changes: `make lint-fix && go test ./...` (two modules: `.` and `./api`).
- `CamundaCluster` code never imports the Optimize packages. The dependency is one-way.
- No `t.Fatal`; use testify `assert`/`require`. Ginkgo+Gomega only for envtest/e2e suites.
- The api module (`api/go.mod`) may import only `k8s.io/api`, `k8s.io/apimachinery`, and the standard library (`api/v1/module_test.go` enforces this).
- Pointer literals use Go 1.26 `new(expr)` (for example `new(int32(1))`), matching the codebase.
- Camunda 8.9 unified config only: exporter keys use `CAMUNDA_DATA_EXPORTERS_ELASTICSEARCH_*`. Never emit legacy `ZEEBE_BROKER_EXPORTERS_*` keys — mixing the two prevents the broker from starting.
- Update the docs listed per PR in the same PR as the code.

## Contracts

None. The three PRs are strictly sequential (PR 2 needs PR 1's types; PR 3 needs PR 2's controller). Each PR targets the feature branch `feat/optimize-controller`; the integration PR to `main` collects all three.

## Conventions

| Dimension | Decision |
| --- | --- |
| API file | `api/v1/camundaoptimize_types.go` (replace scaffold in place) |
| Component package | `pkg/components/camundaoptimize` (mirror `pkg/components/camundacluster` layout: `doc.go`, `input.go`, `names.go`, `components.go`, `mutations.go`, `exporter.go`) |
| Controller package | `internal/controller/camundaoptimize` (delete the root-package stub `internal/controller/camundaoptimize_controller.go` + its test, like earlier batches did for their kinds) |
| Component names / labels | `optimize-webapp`, `optimize-importer` (constants `ComponentWebapp`, `ComponentImporter` in `names.go`). Labels via `labels.Managed(labels.Cluster(<referenced cluster name>), component)` / `labels.Discovery(...)` — `camunda.io/cluster` carries the **referenced cluster's** name |
| Workload names | `<cr-name>-webapp`, `<cr-name>-importer` (`WorkloadName(o *v1.CamundaOptimize, component string)`) — derived from the CamundaOptimize CR name, so two CRs cannot collide |
| Ocf condition types | `WebappReady` (component `optimize-webapp`), `ImporterReady` (component `optimize-importer`); `Ready` aggregates |
| Exporter patch field manager | `camunda-operator/camundaoptimize` — explicit `client.FieldOwner` + `client.ForceOwnership` on a minimal apply object. NOT an ocf component (ocf would use `CamundaOptimize/<component>` and own the whole cluster object) |
| Fixed index prefix | `zeebe-record` — constant `ZeebeRecordPrefix` in `pkg/components/camundaoptimize/names.go`; no CRD field |
| Image | `camunda/optimize` (`OptimizeImage` constant) + `:` + `spec.version`, prefixed by `Platform.ImageRegistry` like `camundacluster.Image` |
| Ports | container `http` 8090, `management` 8092; Service exposes both by name; ServiceMonitor endpoint port `management`, path `/actuator/prometheus` |
| Watch index fields | `"camundaoptimize.spec.clusterRef"`, `"camundaoptimize.spec.managementAuthRef"` (naming convention `<kind lowercase>.spec.<field>`) |
| Finalizer | `core.camunda.io/camundaoptimize-exporter` constant in the controller package |
| E2E namespace / names | `camunda-optimize-e2e`; cluster-scoped fixtures flow-prefixed (`camunda-optimize-e2e-*`) |

---

## PR 1 — API types and extraEnv map lists (#115)

Branch: `pr/optimize-api` off `feat/optimize-controller`; PR targets `feat/optimize-controller`.

### Task 1: CamundaOptimize API types

**Files:**
- Modify: `api/v1/camundaoptimize_types.go` (full replace of scaffold content between the license header and `init()`)
- Modify: `docs/crds/camundaoptimize.md` (align API section; keep the "Not implemented yet" banner)

**Interfaces:**
- Produces: `v1.CamundaOptimizeSpec{Version, ManagementAuthRef, ClusterRef, Webapp, Importer, Monitoring}`, `v1.OptimizeMonitoringSpec`, condition/reason constants `ConditionWebappReady`, `ConditionImporterReady`, `ReasonVersionMismatch`, the shared `ReasonStorageTypeMismatch` promoted into `api/v1/conditions.go`, and the ocf owner methods on `*CamundaOptimize`.

- [ ] **Step 1: Write the types.** Shape (GoDoc per exported symbol, simple-english, omitted here for brevity — write it):

```go
type CamundaOptimizeSpec struct {
	// +kubebuilder:validation:Pattern=`^\d+\.\d+\.\d+$`
	Version string `json:"version"`
	// +kubebuilder:validation:MinLength=1
	ManagementAuthRef string `json:"managementAuthRef"`
	ClusterRef ClusterRef `json:"clusterRef"`
	// +optional
	Webapp *WorkloadSpec `json:"webapp,omitempty"`
	// +optional
	// +kubebuilder:validation:XValidation:rule="!has(self.replicas) || self.replicas == 1",message="importer.replicas must be 1: Optimize supports one active importer"
	Importer *WorkloadSpec `json:"importer,omitempty"`
	// +optional
	Monitoring *OptimizeMonitoringSpec `json:"monitoring,omitempty"`
}

type OptimizeMonitoringSpec struct {
	// +optional
	ServiceMonitor *ServiceMonitorSpec `json:"serviceMonitor,omitempty"`
}
```

Note the CEL rule sits on the `Importer` field (a `WorkloadSpec`), so `self` is the importer block. Status: `Conditions` (`listType=map`, `listMapKey=type`) + `ObservedGeneration`. Add `GetStatusConditions() *[]metav1.Condition`, `GetKind() string`, `SetObservedGeneration(int64)` (copy the pattern from `api/v1/managementauthconfig_types.go:97-108`). Add printcolumns matching the other kinds. Constants next to the types:

```go
const (
	ConditionWebappReady   = "WebappReady"
	ConditionImporterReady = "ImporterReady"
	ReasonVersionMismatch  = "VersionMismatch"
)
```

The RDBMS rejection reuses `ReasonStorageTypeMismatch`, which the backup kinds already report. Move that constant from `api/v1/logicalbackup_shared.go` into `api/v1/conditions.go` and generalize its GoDoc, rather than adding a second spelling of the same concept.

- [ ] **Step 2:** `make manifests generate` (runs per module). Inspect `config/crd/bases/core.camunda.io_camundaoptimizes.yaml` for the CEL rule and required fields.
- [ ] **Step 3:** Update the API reference section of `docs/crds/camundaoptimize.md`: add `version`, drop `clusterRef.namespace`, align `webapp`/`importer` fields with `WorkloadSpec` (they gain `podAnnotations`; field list must match exactly), note reused types. Keep the warning banner.
- [ ] **Step 4:** `go test ./api/... ./...` — expect PASS (module_test guards imports).
- [ ] **Step 5:** Commit: `feat(api): add the CamundaOptimize spec and status types (#115)`.

### Task 2: SSA map-list semantics for extraEnv

**Files:**
- Modify: `api/v1/camundacluster_types.go` — `WorkloadSpec.ExtraEnv` (~line 88) and the top-level `CamundaClusterSpec.ExtraEnv` (~line 322)
- Modify: `docs/crds/camundacluster.md` — describe per-entry SSA ownership and duplicate-name rejection
- Test: `internal/controller/camundacluster/schema_test.go`

**Interfaces:**
- Produces: `spec.zeebe.extraEnv` (and every other `extraEnv`) as an SSA map list keyed by `name` — the contract PR 2's exporter patch relies on.

- [ ] **Step 1: Failing schema test.** In `schema_test.go`, add to the existing `Describe`: create a minimal cluster whose `spec.zeebe.extraEnv` holds two entries with the same `name` — expect the API server to reject it. Second spec: two SSA appliers (`client.Apply` with two different `client.FieldOwner`s) each apply a distinct entry to the same list; expect both entries present afterwards.
- [ ] **Step 2:** Run: `go test ./internal/controller/camundacluster/ -run TestCamundaClusterController -v` — the duplicate-name case FAILS (atomic list accepts duplicates today).
- [ ] **Step 3: Add the markers** on both `ExtraEnv` fields:

```go
	// +listType=map
	// +listMapKey=name
	// +optional
	ExtraEnv []corev1.EnvVar `json:"extraEnv,omitempty"`
```

Extend the GoDoc: entries are merged by name under server-side apply, and one list cannot hold two entries with the same name. Then `make manifests generate`.
- [ ] **Step 4:** Re-run the suite — both new specs PASS, existing specs stay green.
- [ ] **Step 5:** Update `docs/crds/camundacluster.md` (the `extraEnv` field descriptions). Commit: `feat(api): merge extraEnv entries by name under server-side apply (#115)`.

### Task 3: Sample, lint, PR

- [ ] **Step 1:** Rewrite `config/samples/core_v1_camundaoptimize.yaml` to the realistic manifest from the docs page (with `version`). Do NOT add it to `implementedKindSamples` in `internal/controller/samples_schema_test.go` yet — that happens in PR 2 when the kind is real.
- [ ] **Step 2:** `make all` and `go test ./...` in both modules — PASS.
- [ ] **Step 3:** Commit, push, open PR 1 targeting `feat/optimize-controller` via `feature-dev-workflow:opening-a-pull-request`. Body uses `Towards #115`. Run the review loop (`feature-dev-workflow:copilot-review-loop`), self-merge, close #115.

---

## PR 2 — Components and controller (#116)

Branch: `pr/optimize-controller` off `feat/optimize-controller` (after PR 1 merges).

### Task 4: Component package — input, names, env rendering

**Files:**
- Create: `pkg/components/camundaoptimize/doc.go`, `input.go`, `names.go`, `render.go`
- Test: `pkg/components/camundaoptimize/render_test.go`

**Interfaces:**
- Produces:
  - `type Input struct { Optimize *v1.CamundaOptimize; ClusterName string; Partitions int32; Platform v1.CamundaPlatformConfigSpec; Storage v1.ElasticsearchStorage; StorageNamespace string; Auth *v1.ManagementAuthConfig; ServiceMonitorSupported bool }`
  - `func Image(in Input) string` — `camunda/optimize:<spec.version>`, registry prefix like `camundacluster.Image`
  - `func WorkloadName(o *v1.CamundaOptimize, component string) string` — `o.Name + "-" + shortName` (`webapp`/`importer`)
  - Constants: `ComponentWebapp = "optimize-webapp"`, `ComponentImporter = "optimize-importer"`, `OptimizeImage = "camunda/optimize"`, `ZeebeRecordPrefix = "zeebe-record"`, `PortHTTP = 8090`, `PortManagement = 8092`
  - `func baseEnv(in Input, importEnabled bool) []corev1.EnvVar`

- [ ] **Step 1: Load `verifying-camunda-app-config` and verify every key below against the camunda-docs MCP before writing them.** Already-verified set (8.9):

| Key | Value |
| --- | --- |
| `SPRING_PROFILES_ACTIVE` | `ccsm` |
| `CAMUNDA_OPTIMIZE_DATABASE` | `elasticsearch` |
| `OPTIMIZE_ELASTICSEARCH_HOST` | host from `Storage.Endpoint` |
| `OPTIMIZE_ELASTICSEARCH_HTTP_PORT` | port from `Storage.Endpoint` |
| `CAMUNDA_OPTIMIZE_ZEEBE_ENABLED` | `true` importer / `false` webapp fallback (see below) |
| `CAMUNDA_OPTIMIZE_ZEEBE_NAME` | `zeebe-record` |
| `CAMUNDA_OPTIMIZE_ZEEBE_PARTITION_COUNT` | `in.Partitions` (verify exact env spelling for `zeebe.partitionCount`) |
| `CAMUNDA_OPTIMIZE_IDENTITY_ISSUER_URL` | `Auth.Spec.IssuerURL` |
| `CAMUNDA_OPTIMIZE_IDENTITY_ISSUER_BACKEND_URL` | `Auth.Spec.IssuerBackendURL`, default `IssuerURL` |
| `CAMUNDA_OPTIMIZE_IDENTITY_BASE_URL` | `Auth.Spec.BaseURL` |
| `CAMUNDA_OPTIMIZE_IDENTITY_CLIENTID` | `Auth.Spec.ClientID` |
| `CAMUNDA_OPTIMIZE_IDENTITY_CLIENTSECRET` | `valueFrom` → `Auth.Spec.ClientSecretRef` |
| `CAMUNDA_OPTIMIZE_IDENTITY_AUDIENCE` | `Auth.Spec.Audience` |

To verify by search (decision rules included, so this is not open-ended): (a) ES credentials — expected `CAMUNDA_OPTIMIZE_ELASTICSEARCH_SECURITY_USERNAME` / `..._PASSWORD` (`es.security.*`); password via `valueFrom` on `Storage.CredentialsSecretRef`. (b) ES TLS — expected `es.security.ssl.enabled` + certificate path keys; mount `Storage.CASecretRef` as a volume and point the certificate key at it (mirror how `pkg/components/camundacluster/render.go` mounts the ES CA). (c) Import split — expected `CAMUNDA_OPTIMIZE_IMPORT_ENABLED=false` on the webapp; if the 8.9 docs do not confirm that key, use `CAMUNDA_OPTIMIZE_ZEEBE_ENABLED=false` on the webapp instead (verified to gate Zeebe import). The importer always sets import on.
- [ ] **Step 2:** Write `render_test.go` first (testify): `TestBaseEnvImporterEnablesImport`, `TestBaseEnvWebappDisablesImport`, `TestImageUsesRegistryPrefix`, `TestIssuerBackendURLDefaultsToIssuer`. Run — FAIL (functions missing).
- [ ] **Step 3:** Implement `input.go`, `names.go`, `render.go`. License env: when `in.Platform.LicenseSecretRef` is set, add the license env the same way `camundacluster` does (find the key in its `render.go` and reuse the same mechanism). Run tests — PASS.
- [ ] **Step 4:** Commit: `feat(camundaoptimize): add the component input and env rendering (#116)`.

### Task 5: Component package — deployments, services, monitors, mutations

**Files:**
- Create: `pkg/components/camundaoptimize/components.go`, `mutations.go`, `fixtures_test.go`
- Test: `pkg/components/camundaoptimize/components_test.go`, `mutations_test.go`, `testdata/golden/...`

**Interfaces:**
- Produces: `func Build(in Input) ([]*component.Component, error)` — two components (`optimize-webapp`, `optimize-importer`), each with a Deployment, a Service, and an `IncludeWhen(in.ServiceMonitorSupported, ...)` ServiceMonitor gated by `monitoringGate(in)`.

- [ ] **Step 1:** Follow `pkg/components/camundacluster/components.go:187-214` exactly: `deployment.NewBuilder(...).WithMutation(deploymentMutations(in, c)...).Build()`, `service.NewBuilder`, the repo `pkg/wrappers/servicemonitor` wrapper, `component.NewComponentBuilder().WithName(c).WithConditionType(...).WithResource(...)`. Webapp replicas default 1 from `spec.webapp.replicas`; importer replicas fixed 1. Selector labels via `labels.Discovery(labels.Cluster(in.ClusterName), component)`; object labels `labels.Managed(...)`. Readiness probe: HTTP GET `/api/readyz` on port `http`; liveness the same path. ServiceMonitor endpoint: port `management`, path `/actuator/prometheus`.
- [ ] **Step 2:** `mutations.go` mirrors `camundacluster/mutations.go`: one gated mutation per `WorkloadSpec` surface (`resources`, `scheduling`, pod metadata, `extraEnv`, `extraEnvFrom`) using `feature.Mutation[primitives.WorkloadMutator]` + `deployment.LiftMutation`. Operator labels win on merge.
- [ ] **Step 3:** Tests, in this order: `mutations_test.go` (pin `RegisteredMutations()` order; `FiringSet()` empty without overrides, exact set with overrides — copy the camundacluster test shape), then golden tests: `assertGoldens`-style helper with `golden.AssertComponentYAML`, fixtures `minimal` and `realistic` in `fixtures_test.go` with deterministic values. Generate goldens with `go test ./pkg/components/camundaoptimize/ -run Golden -update-golden`, then read every golden YAML and check it by eye (image tag, env, labels, probe, ports).
- [ ] **Step 4:** `go test ./pkg/components/camundaoptimize/` — PASS. Commit: `feat(camundaoptimize): build the webapp and importer components (#116)`.

### Task 6: Exporter patch helper

**Files:**
- Create: `pkg/components/camundaoptimize/exporter.go`
- Test: `pkg/components/camundaoptimize/exporter_test.go`

**Interfaces:**
- Produces: `const ExporterFieldManager = "camunda-operator/camundaoptimize"`; `func ExporterEnv(storage v1.ElasticsearchStorage) []corev1.EnvVar`; `func ExporterPatch(cluster types.NamespacedName, env []corev1.EnvVar) *v1.CamundaCluster` (a minimal typed object carrying only TypeMeta, name/namespace, and `spec.zeebe.extraEnv`).

- [ ] **Step 1:** Verify the exporter arg keys with camunda-docs MCP, then write the env set (unified config only):

```go
{Name: "CAMUNDA_DATA_EXPORTERS_ELASTICSEARCH_CLASSNAME", Value: "io.camunda.zeebe.exporter.ElasticsearchExporter"},
{Name: "CAMUNDA_DATA_EXPORTERS_ELASTICSEARCH_ARGS_URL", Value: storage.Endpoint},
{Name: "CAMUNDA_DATA_EXPORTERS_ELASTICSEARCH_ARGS_INDEX_PREFIX", Value: ZeebeRecordPrefix},
```

plus authentication args (expected `..._ARGS_AUTHENTICATION_USERNAME` / `..._ARGS_AUTHENTICATION_PASSWORD`; password by `valueFrom` on `storage.CredentialsSecretRef` — verify the exact arg names in the exporter's authentication section). TLS trust: inspect how the broker already trusts the ES CA for secondary storage in `pkg/components/camundacluster/render.go`; the exporter runs inside the same JVM, so if trust is JVM-level nothing more is needed — if it is client-level, add the exporter's TLS arg pointing at the same mounted CA path. Record the finding in the code's GoDoc.
- [ ] **Step 2:** Unit tests (testify): env set is exactly the expected list; `ExporterPatch` carries only name, namespace, TypeMeta, and the env — assert nothing else is set (marshal to JSON and compare keys).
- [ ] **Step 3:** Commit: `feat(camundaoptimize): render the exporter enablement patch (#116)`.

### Task 7: Controller — resolution, patch, components, status

**Files:**
- Create: `internal/controller/camundaoptimize/controller.go`, `precheck.go`, `finalizer.go`
- Delete: `internal/controller/camundaoptimize_controller.go`, `internal/controller/camundaoptimize_controller_test.go`
- Modify: `cmd/main.go` (replace the stub registration ~line 355 with the new package)

**Interfaces:**
- Consumes: everything Tasks 4–6 produce.
- Produces: `camundaoptimize.Reconciler` with `SetupWithManager(mgr ctrl.Manager) error`.

- [ ] **Step 1: Reconciler skeleton** modeled on `internal/controller/camundacluster/controller.go:132-209`: get CR via `APIReader`, deletion → `finalize`, add finalizer before first side effect (pattern `internal/controller/logicalbackupelasticsearch/controller.go:160-176`), build `component.ReconcileContext` with an **uncached** `componentClient` (see `watches.go:239` — never the manager cache), `defer FlushStatus`, precheck → `conditions.Stage(cr, conditions.Failed(...))` on `*conditions.PreCheckFailure`, components → `Reconcile` each → `conditions.Stage(cr, conditions.Aggregate(cr, comps...))`.
- [ ] **Step 2: precheck.go** — ordered resolution, each failure a `conditions.PreCheckFailure`:
  1. `clusterRef` → cluster in the CR's namespace (`ReasonInvalidReference`).
  2. Cluster's `storageRef` → `SecondaryStorageConfig`; `Type != elasticsearch` → `ReasonStorageTypeMismatch`; missing → `ReasonInvalidReference`.
  3. Version gate: major.minor of `spec.version` vs the referenced cluster's effective version. Resolve the cluster's effective spec the same way its own controller does: resolve `presetRef` (if set) + `components.MergePreset`; compare with `strings.Cut` on the two dot positions. Mismatch → `ReasonVersionMismatch`. Partitions come from `camundacluster.NewEffective(merged).Partitions()`.
  4. `managementAuthRef` → cluster-scoped `ManagementAuthConfig` (`ReasonInvalidReference`); its `ClientSecretRef` via `pkg/secretref.Get` (`ReasonMissingSecret`).
  5. Cluster's `platformConfigRef` → `CamundaPlatformConfig` for `ImageRegistry` + `LicenseSecretRef` (absent ref is fine — empty platform spec).
  6. ES credentials secret exists (`ReasonMissingSecret`).
  Return the populated `camundaoptimize.Input`.
- [ ] **Step 3: Exporter patch.** After precheck, before components:

```go
patch := camundaoptimize.ExporterPatch(clusterKey, camundaoptimize.ExporterEnv(in.Storage))
err := r.Client.Patch(ctx, patch, client.Apply,
	client.FieldOwner(camundaoptimize.ExporterFieldManager), client.ForceOwnership)
```

- [ ] **Step 4: finalizer.go.** On deletion: apply the same minimal object with an **empty** `spec.zeebe.extraEnv` under the same field manager (removes only owned entries), tolerate NotFound, then `RemoveFinalizer` + `Update` (pattern `logicalbackupelasticsearch/finalizer.go:52-80`).
- [ ] **Step 5: Watches.** `For(&v1.CamundaOptimize{})`, `Owns(Deployment)`, `Owns(Service)`, field indexes `"camundaoptimize.spec.clusterRef"` (namespaced key) and `"camundaoptimize.spec.managementAuthRef"` (name key), then `Watches(&v1.CamundaCluster{}, refindex.Enqueue(...))`, `Watches(&v1.ManagementAuthConfig{}, refindex.Enqueue(..., refindex.ObjectName))`, `Watches(&v1.SecondaryStorageConfig{}, <two-hop handler>)` — the two-hop handler lists clusters by the existing `"camundacluster.spec.storageRef"` index, then optimizes by clusterRef, and `Watches(&corev1.Secret{}, ..., builder.OnlyMetadata)` for the client/ES secrets (model on `camundacluster/watches.go` `enqueueForSecret`). ServiceMonitor support via the `RESTMapping` check (`camundacluster/controller.go:295`) — no watch on ServiceMonitors.
- [ ] **Step 6: RBAC markers** on the reconciler: camundaoptimizes (+status,+finalizers), camundaclusters `get;list;watch;patch`, camundaclusterpresets/camundaplatformconfigs/managementauthconfigs/secondarystorageconfigs `get;list;watch`, secrets `get;list;watch`, deployments/services full CRUD, servicemonitors full CRUD, events `create;patch`. `make manifests` regenerates the role.
- [ ] **Step 7:** Replace the `cmd/main.go` registration block with the new package; delete the two stub files. Build: `go build ./...`.
- [ ] **Step 8:** Commit: `feat(camundaoptimize): reconcile the Optimize workloads and exporter patch (#116)`.

### Task 8: Envtest suite + schema tests

**Files:**
- Create: `internal/controller/camundaoptimize/suite_test.go`, `controller_test.go`, `schema_test.go`
- Modify: `internal/controller/samples_schema_test.go` (add `core_v1_camundaoptimize.yaml` to `implementedKindSamples`)

- [ ] **Step 1:** `suite_test.go` from the `internal/testenv.Start` pattern (`internal/controller/camundacluster/suite_test.go`); register only the camundaoptimize reconciler.
- [ ] **Step 2:** `schema_test.go`: minimal + realistic fixtures matching the docs page examples; CEL specs — `importer.replicas: 2` rejected, missing `version` rejected, bad semver rejected.
- [ ] **Step 3:** `controller_test.go` Ginkgo specs (create the referenced fixtures directly with the uncached client — no other reconcilers needed):
  - Happy path: fixture `CamundaCluster` (version `8.9.9`, `storageRef`) + `SecondaryStorageConfig` (elasticsearch) + ES credentials secret + `ManagementAuthConfig` + client secret → CR becomes `Ready=True/Healthy`; wait for Deployments to exist with correct image/env/labels; `spec.zeebe.extraEnv` on the cluster gains exactly the exporter entries.
  - Co-ownership: pre-set a user entry in `spec.zeebe.extraEnv` (different field manager), reconcile, assert both the user entry and the exporter entries are present; delete the CR, assert the exporter entries vanish and the user entry survives, finalizer gone.
  - Each failure reason from its broken input: missing cluster → `InvalidReference`; RDBMS storage → `StorageTypeMismatch`; missing client secret → `MissingSecret`; version `8.8.1` vs cluster `8.9.9` → `VersionMismatch`.
- [ ] **Step 4:** `go test ./internal/controller/camundaoptimize/ -v` — PASS. Then full `go test ./...` + `make all`.
- [ ] **Step 5:** Commit: `test(camundaoptimize): cover reconciliation, patch ownership, and schema (#116)`. Open PR 2 (`Towards #116`), review loop, self-merge, close #116.

---

## PR 3 — Data-flow e2e and docs finalization (#117)

Branch: `pr/optimize-e2e` off `feat/optimize-controller` (after PR 2 merges).

### Task 9: E2E test

**Files:**
- Create: `test/e2e/camundaoptimize_test.go`
- Modify: `test/e2e/helpers_test.go` (`customResourceKinds` + resource dump list gain `camundaoptimizes` and `managementauthconfigs`), `Makefile` (`E2E_TIMEOUT ?= 60m` → `75m`)

- [ ] **Step 1: Pick the Optimize version.** `docker manifest inspect camunda/optimize:<tag>` for the newest 8.9.x tag; pin it as `optimizeVersion` next to `ccVersion` (8.9.9). The minors must match; the patch may differ.
- [ ] **Step 2: Fixture setup** (`Describe("CamundaOptimize", Ordered)`, ns `camunda-optimize-e2e`), reusing the existing helpers exactly:
  - Apply `testdata/keycloak.yaml` into the ns; `kubectl rollout status deployment/keycloak` (6m).
  - `ElasticsearchCluster` literal publishing a `SecondaryStorageConfig` (copy `camundacluster_oidc_test.go:129-139`); wait Ready 15m.
  - Basic-auth `CamundaPlatformConfig` (`basicPlatform` pattern) + `newCluster`-style `CamundaCluster` with one **pre-existing user entry** in `spec.zeebe.extraEnv` (for example `E2E_USER_MARKER=keep-me`); wait Ready 15m.
  - Client secret Secret in the ns + cluster-scoped `ManagementAuthConfig` literal named `camunda-optimize-e2e-auth`: `baseUrl`/`issuerUrl` = `http://keycloak.<ns>.svc:8080/realms/camunda`, `authUrl`/`tokenUrl`/`jwksUrl` = issuer + `/protocol/openid-connect/{auth,token,certs}`, `clientId: camunda`, `audience: camunda`, `clientSecretRef` → the Secret (explicit namespace).
  - `CamundaOptimize` CR: `version: optimizeVersion`, `clusterRef`, `managementAuthRef`.
- [ ] **Step 3: Assertions**, in order:
  1. CR `Ready=True` (`expectReady`, 15m).
  2. Both Deployments available; labels `camunda.io/component` = `optimize-webapp`/`optimize-importer`.
  3. Cluster's `spec.zeebe.extraEnv` holds the exporter entries AND `E2E_USER_MARKER` — co-ownership proven end to end.
  4. Deploy `testdata/process.bpmn` + start an instance via `camundaREST` with basic auth (copy `camundacluster_test.go:474-507`).
  5. `Eventually` 10m: `curlElasticsearch(&contract, "zeebe-indices", "/_cat/indices/zeebe-record*?format=json")` returns a non-empty JSON array.
  6. `Eventually` 10m: `/_cat/indices/*optimize*?format=json` non-empty — the importer wrote its analytics indices.
  7. Delete the `CamundaOptimize` CR; `Eventually`: exporter entries gone from the cluster, `E2E_USER_MARKER` still present.
- [ ] **Step 4:** `AfterAll` teardown + `AfterEach { dumpDiagnostics(...) }` per suite convention. Run the focused flow locally against a dedicated Kind cluster only if feasible; otherwise rely on CI (repo convention: never run the whole suite locally).
- [ ] **Step 5:** Commit: `test(e2e): prove Optimize imports data from a live cluster (#117)`.

### Task 10: Docs finalization

**Files:**
- Modify: `docs/crds/camundaoptimize.md`, `docs/crds/index.md`, `docs/crds/managementauthconfig.md` (it now has a consumer), `docs/architecture.md` (remove "no consumer yet" note if present)

- [ ] **Step 1:** Load `feature-dev-workflow:writing-docs` + `simple-english`. Remove the "Not implemented yet" banner; align every section with shipped behavior (reconcile steps, `version` + `VersionMismatch`, exact status table, validation, examples that match `config/samples`). Follow the docs conventions (outcomes for users, show-don't-tell fragments).
- [ ] **Step 2:** `make all`, `go test ./...` — PASS. Commit: `docs(camundaoptimize): document the shipped Optimize controller (#117)`. Open PR 3 (`Towards #117`), review loop, self-merge, close #117.

---

## Integration

- [ ] Verify on `feat/optimize-controller`: `make all`, `go test ./...` (both modules) green; e2e green in CI on the last sub-PR.
- [ ] Run `feature-dev-workflow:reviewing-feature-progress`, then open the integration PR `feat/optimize-controller` → `main` with `Closes #114`; final review loop; delete the plan + state file in the last commit per the state-file lifecycle.
