# DatabaseServer on CloudNativePG Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Run PostgreSQL for an orchestration cluster from a `DatabaseServer` that creates a CloudNativePG `Cluster`, archives to an `ObjectStorageConfig` through the Barman Cloud plugin, publishes a `DatabaseServerConfig`, and answers a recovery request that `PointInTimeRestore` writes on that contract.

**Architecture:** Five sub-PRs on `feat/cnpg-database-server`. PR 1 makes the RDBMS chain namespace-local and keys server identity on the PostgreSQL system identifier. PR 2 wraps the two third-party kinds as ocf primitives. PR 3 adds the `DatabaseServer` kind and controller. PR 4 adds the recovery request to the contract, the `RestoringDatabase` phase to `PointInTimeRestore`, and the recovery in `DatabaseServer`. PR 5 proves it in e2e and writes the installation and guide docs. PR 1 and PR 2 are independent and run in parallel; PR 3 needs both; PR 4 needs PR 3; PR 5 needs PR 4.

**Tech Stack:** Go 1.26, controller-runtime 0.24, ocf 0.19.1, `github.com/cloudnative-pg/api` v1.30.0, Barman Cloud plugin 0.14.0 (`barmancloud.cnpg.io/v1`), pgx v5, envtest, Ginkgo/Gomega, testify, kind in CI.

**Spec:** `docs/superpowers/specs/2026-08-24-cnpg-database-server-design.md`

## Global Constraints

- `api/` is its own Go module. Nothing under `api/v1` imports CloudNativePG or the plugin types.
- Every managed resource is applied with server-side apply. Status is written once per reconcile through ocf `FlushStatus`.
- Clean slate: no conversion, no migration, no compatibility shim for the scope change.
- The in-tree `spec.backup.barmanObjectStore` is never used. The archive goes through `spec.plugins[]` with `name: barman-cloud.cloudnative-pg.io`.
- Base backups are named "base backup" in every field, doc, and message. The words "scheduled backup" and "backup schedule" are reserved for the operator's own `BackupSchedule`.
- No controller lists or watches a sibling `DatabaseServer`. `PointInTimeRestore` reads and writes the contract only and has no RBAC on `DatabaseServer`.
- Load `how-we-write-go` before any Go change, `ocf:building-components` / `ocf:using-primitives` / `ocf:custom-resource-wrappers` / `ocf:testing-operators` for the ocf parts, `writing-operator-docs` for every page under `docs/` and every CRD field description, `simple-english:simple-english` for GoDoc and messages.
- Gates before every sub-PR opens: `make setup-envtest`, `go test ./...`, `go -C api test ./...`, `make lint`, `make lint-renovate`, `make manifests generate` with `git status --porcelain config api` empty, `go vet -tags=e2e ./test/e2e/`, `mkdocs build --strict`.
- Commit subjects carry the sub-issue: `<type>(<area>): <summary> (#<issue>)`. Sub-PR bodies say `Towards #<issue>`.
- e2e runs in CI on the PR only. Never run the kind suite locally.

---

## Contracts

| Name | Producer (issue) | Consumer (issue) | Shape | Realization |
| --- | --- | --- | --- | --- |
| `namespaced-rdbms-kinds` | #128 | #235, #236 | `DatabaseServerConfig` and `Database` carry no `scope=Cluster` marker; `Database.spec.targetNamespace` is gone; `DatabaseServerConfigSpec.AdminCredentialsSecretRef` is `LocalCredentialsSecretRef{Name, UsernameKey, PasswordKey}`; `DatabaseServerConfigStatus.SystemIdentifier string \`json:"systemIdentifier,omitempty"\`` | stub-on-producer-branch: PR 3 branches from the feature branch after PR 1 merges |
| `cnpg-wrappers` | #234 | #235, #236 | `pkg/wrappers/cnpgcluster.NewBuilder(*cnpgv1.Cluster) *Builder`, `.WithMutation(...Mutation)`, `.Build() (*Resource, error)`; `pkg/wrappers/barmanobjectstore.NewBuilder(*barmanobjectstore.ObjectStore) *Builder`; `pkg/wrappers/cnpgscheduledbackup.NewBuilder(*cnpgv1.ScheduledBackup) *Builder`; `pkg/wrappers/podmonitor.NewBuilder(*monitoringv1.PodMonitor) *Builder`; `test/utils.CNPGCRDPath() string`, `test/utils.BarmanCRDPath() string` | stub-on-producer-branch: PR 3 branches after PR 2 merges |
| `contract-recovery-fields` | #236 | #236 (both sides in one PR) | `PITRCapability.Recovery RecoveryMode` (`operator`/`external`), `PITRCapability.LastRecovery *RecoveryOutcome`, `DatabaseServerConfigSpec.Recovery *RecoveryRequest` | data-only: both writers ship in PR 4 |

PR 1 and PR 2 share no symbol, so they need no contract between them.

## Conventions

- **Layout.** One package per wrapper under `pkg/wrappers/<kind>`; one package per owner kind under `pkg/components/<owner>`; one controller package under `internal/controller/<owner>` with `suite_test.go`, `controller.go`, `controller_test.go`, `schema_test.go`; the without-CRD suite in `internal/controller/<owner>/without<dep>/`.
- **Names.** `DatabaseServer` is the kind, `databaseserver` the package and path. Components are named for their condition: `ClusterComponent` owns `ClusterReady`, `ArchiveComponent` owns `ArchiveReady`, `ContractComponent` owns `ContractReady`, `MonitoringComponent` owns `MonitoringReady`. Reasons are `ReasonCNPGNotInstalled = "CNPGNotInstalled"`, `ReasonBarmanPluginNotInstalled = "BarmanPluginNotInstalled"`. The CloudNativePG `Cluster` of a server is named `<server>` at birth and `<server>-r<N>` after the N-th recovery; `serverName` in the archive equals the `Cluster` name. Labels: `camunda.io/database-server: <name>` on every owned object.
- **Vocabulary.** "archive" is the WAL archive plus base backups; "base backup" never "scheduled backup"; "recovery" is the CloudNativePG bootstrap, "restore" is the `PointInTimeRestore` operation; "contract" is the `DatabaseServerConfig`; "producer" writes a contract, "consumer" reads it.
- **Interfaces.** Every owner kind implements `GetStatusConditions`, `GetKind`, `SetObservedGeneration`. Every wrapper follows `eckelasticsearch`: `builder.go`, `mutator.go`, `resource.go`, `health.go`, `applyclient.go` if the CRD schema rejects a bare apply. Every component takes `(owner, merged spec, resolved inputs)` and returns `component.Component`.
- **Idiom.** Pre-check failures through `conditions.PreCheckFailure`; owner Ready through `conditions.Aggregate`; every contract resolved once into a value struct (the `activeBlock` precedent), never re-read per method. Time is RFC 3339 UTC with a `Z` suffix on the wire. Tests use testify `assert`/`require` for pure Go, Ginkgo for controllers, golden snapshots for every component.
- **Docs.** Every new page starts from `docs/crds/TEMPLATE.md`. Every page states outcomes, not reconcile steps.

---

## PR 1: namespaced RDBMS chain and server identity (#128)

Branch `fix/cnpg-database-server--namespaced-chain` off `feat/cnpg-database-server`.

### Task 1.1: `SystemIdentifier` in `pkg/pgbootstrap`

**Files:**
- Modify: `pkg/pgbootstrap/pgbootstrap.go` (the `Bootstrapper` interface at `:59-89`, `ServerVersion` at `:384`)
- Test: `pkg/pgbootstrap/pgbootstrap_test.go` (next to the existing `ServerVersion` testcontainers test)

**Interfaces:**
- Produces: `SystemIdentifier(ctx context.Context) (string, error)` on `Bootstrapper`, returning the decimal string of `pg_control_system().system_identifier`.

- [ ] **Step 1: Write the failing test** next to the `ServerVersion` test: connect to the testcontainers PostgreSQL, call `SystemIdentifier`, assert it matches `^\d{15,20}$` and equals the value of `SELECT system_identifier::text FROM pg_control_system()` read through a second connection.
- [ ] **Step 2: Run** `go test ./pkg/pgbootstrap/ -run TestSystemIdentifier -v` and see it fail to compile.
- [ ] **Step 3: Implement** `SystemIdentifier` on the concrete bootstrapper: `SELECT system_identifier::text FROM pg_control_system()` scanned into a string. Add it to the interface and to every fake that implements the interface (`grep -rn "ServerVersion(ctx" --include=*_test.go`).
- [ ] **Step 4: Run** the package tests and `make lint`.
- [ ] **Step 5: Commit** `feat(pgbootstrap): read the PostgreSQL system identifier (#128)`.

### Task 1.2: namespaced `DatabaseServerConfig` with `status.systemIdentifier`

**Files:**
- Modify: `api/v1/databaseserverconfig_types.go`, `api/v1/database_types.go`, `api/v1/databaseconfig_types.go` (GoDoc on `ServerRef`), `api/v1/zz_generated.deepcopy.go` (generated), `config/crd/bases/*`, `config/rbac/role.yaml` (generated), `config/samples/core_v1_databaseserverconfig.yaml`, `config/samples/core_v1_database.yaml`
- Modify: `internal/controller/databaseserverconfig/controller.go` (`probeWithin` at `:220-241`, the Secret lookup, `SetupWithManager` index)
- Test: `internal/controller/databaseserverconfig/controller_test.go`, `internal/controller/samples_schema_test.go`

**Interfaces:**
- Produces:

```go
// LocalCredentialsSecretRef names a Secret in the namespace of the object that holds the reference.
type LocalCredentialsSecretRef struct {
	// +kubebuilder:validation:MinLength=1
	Name string `json:"name"`
	// +kubebuilder:default=username
	UsernameKey string `json:"usernameKey,omitempty"`
	// +kubebuilder:default=password
	PasswordKey string `json:"passwordKey,omitempty"`
}

type DatabaseServerConfigSpec struct {
	Engine                    DatabaseEngine            `json:"engine"`
	Host                      string                    `json:"host"`
	Port                      int32                     `json:"port"`
	AdminCredentialsSecretRef LocalCredentialsSecretRef `json:"adminCredentialsSecretRef"`
	PITR                      *PITRCapability           `json:"pitr,omitempty"`
}

type DatabaseServerConfigStatus struct {
	ObservedGeneration  int64              `json:"observedGeneration,omitempty"`
	ServerVersion       string             `json:"serverVersion,omitempty"`
	SystemIdentifier    string             `json:"systemIdentifier,omitempty"`
	ProbedAt            *metav1.Time       `json:"probedAt,omitempty"`
	ProbedSecretVersion string             `json:"probedSecretVersion,omitempty"`
	Conditions          []metav1.Condition `json:"conditions,omitempty"`
}
```

  `DatabaseSpec` loses `TargetNamespace`; `CredentialsSpec` and `BackupCredentialsSpec` lose `SecretNamespace`. Check whether `CredentialsSecretRef` (used by `DatabaseConfig`, `SecondaryStorageConfig`) already has this shape minus the namespace; if it is `{Name, Namespace, ...}` add `LocalCredentialsSecretRef` beside it, do not change the other contracts in this PR.
- Printcolumns on both kinds: `Ready` (`.status.conditions[?(@.type=='Ready')].status`), `Reason`, `Age`. `DatabaseServerConfig` also `Version` (`.status.serverVersion`).

- [ ] **Step 1: Types.** Remove `+kubebuilder:resource:scope=Cluster` from both kinds; add `+kubebuilder:resource:scope=Namespaced` and the printcolumns. Apply the struct changes above. Rewrite every GoDoc that says "cluster-scoped" or "any namespace" (`grep -rn "cluster-scoped\|any namespace\|targetNamespace" api/v1/`).
- [ ] **Step 2: Generate.** `make manifests generate`; fix the samples so `internal/controller/samples_schema_test.go` passes (`metadata.namespace: default`, no `targetNamespace`, the admin Secret in the same namespace).
- [ ] **Step 3: Controller test first.** In `controller_test.go` add: a contract whose Secret is in the contract's namespace probes Healthy and publishes `status.systemIdentifier` equal to the value the injected `probe` seam returns; a contract whose Secret is missing reports `MissingSecret`. Extend the `probe` seam type to return `(version, systemIdentifier string, err error)`.
- [ ] **Step 4: Controller.** `probeWithin` calls `SystemIdentifier` after `ServerVersion`, `validate` sets `status.SystemIdentifier`. The Secret lookup uses `req.Namespace`. The `refindex.NamespacedKey` index keys on the contract's own namespace.
- [ ] **Step 5: Run** `go test ./internal/controller/databaseserverconfig/... ./api/...`, `go -C api test ./...`, `make lint`.
- [ ] **Step 6: Commit** `feat(databaseserverconfig): make the contract namespaced and publish the system identifier (#128)`.

### Task 1.3: namespaced `Database` keyed on the system identifier

**Files:**
- Modify: `pkg/components/database/collision.go` (`CollisionKey` at `:26`), `pkg/components/database/collision_test.go`
- Modify: `internal/controller/database/controller.go` (`resolveServer` `:242`, `checkCollision` `:287`, `enqueueForAdminSecret` `:341`, `SetupWithManager` `:375`, the `database.spec.serverDatabase` index at `:59`), `internal/controller/database/controller_test.go`
- Modify: `pkg/components/database/bindings.go` (`ResolveBindings` uses `db.Namespace`, not `TargetNamespace`)

**Interfaces:**
- Produces: `CollisionKey(systemIdentifier, databaseName string) string` returning `systemIdentifier + "/" + databaseName`; the field index name `database.status.collisionKey` on `Database.status.collisionKey`.
- `DatabaseStatus` gains `CollisionKey string \`json:"collisionKey,omitempty"\`` so the index is served from what the controller resolved, not from spec.

- [ ] **Step 1: Unit test** for `CollisionKey`: two calls with different contract names and one identifier produce one key; empty identifier is rejected by the caller (tested in the controller).
- [ ] **Step 2: Implement** `CollisionKey`. Keep `CollisionWinner`, with the tiebreak on `namespace/name`.
- [ ] **Step 3: Controller tests** (envtest, `controller_test.go`): (a) two contracts in two namespaces, both with `status.systemIdentifier: "7000000000000000001"` written by the test, two `Database` objects with `databaseName: camunda`; the older is Ready and runs SQL through the injected bootstrapper, the newer reports `InvalidReference` naming the winner and runs no SQL. (b) A contract without `systemIdentifier` holds its `Database` at `Ready=False` with reason `ServerIdentityUnknown` and message `DatabaseServerConfig <ns>/<name> has not published its system identifier yet`. (c) Bindings land in the `Database`'s own namespace.
- [ ] **Step 4: Controller.** `resolveServer` gets `{Namespace: db.Namespace, Name: db.Spec.ServerRef}`. `preCheck` order becomes resolveServer → identity (`ServerIdentityUnknown` when empty) → adminCredentials → checkCollision (on the new key, listing through the cluster-wide index) → connect. Write `status.collisionKey` in the same flush. `enqueueForAdminSecret` lists contracts in the Secret's namespace only. `SetupWithManager` watches `DatabaseServerConfig` by namespace/name.
- [ ] **Step 5: Run** `go test ./internal/controller/database/... ./pkg/components/database/...`, `make lint`.
- [ ] **Step 6: Commit** `fix(database): key the collision rule on the server identity and make the kind namespaced (#128)`.

### Task 1.4: `PointInTimeRestore` counts by identifier and pins it

**Files:**
- Modify: `api/v1/pointintimerestore_types.go` (`PointInTimeRestoreStorage` `:120-144`: add `SystemIdentifier string \`json:"systemIdentifier"\``)
- Modify: `internal/controller/pointintimerestore/admit.go` (`resolve` `:171`, `pinnedChain` `:307`, `pinnedChainCurrent` `:333`, `dedicatedServer` `:428-478`)
- Test: `internal/controller/pointintimerestore/admit_test.go` and the controller suite

- [ ] **Step 1: Tests.** `dedicatedServer`: two contracts in two namespaces with one identifier, one `Database` each → `SharedServer` listing both `ns/name`; one contract, one `Database` → pass; contract without identifier → `InvalidReference` with the `ServerIdentityUnknown` message. `pinnedChainCurrent`: identifier changed → `errChainChanged`.
- [ ] **Step 2: Implement.** `resolve` reads `DatabaseServerConfig` in the cluster's namespace and refuses (`InvalidReference`) when `status.systemIdentifier` is empty. `dedicatedServer` lists every `Database` (unindexed, APIReader), resolves each one's contract in its own namespace, and counts those whose identifier equals the pinned one. `pinnedChain` stores the identifier; `pinnedChainCurrent` compares it.
- [ ] **Step 3: Run** the package tests, `make manifests generate`, `make lint`.
- [ ] **Step 4: Commit** `fix(pointintimerestore): count the dedicated-server rule by system identifier (#128)`.

### Task 1.5: e2e, samples, and docs for the namespaced chain

**Files:**
- Modify: `test/e2e/database_test.go` (`:96-118`, namespaced contract and no `targetNamespace`), `test/e2e/camundacluster_test.go:500+`, `test/e2e/restore_test.go` (contract creation in the helpers), `test/e2e/testdata/postgres.yaml` if the admin Secret namespace changes
- Modify: `docs/crds/database.md`, `docs/crds/databaseserverconfig.md`, `docs/crds/pointintimerestore.md` (`:105-107`), `docs/crds/databaseconfig.md` (`serverRef` in this namespace), `docs/guides/secondary-storage.md:107`, `docs/guides/backup.md:191`, `docs/guides/operations.md:300`, `docs/architecture.md` (the scope table)

- [ ] **Step 1:** Update the e2e flows; `go vet -tags=e2e ./test/e2e/`.
- [ ] **Step 2:** Docs with `writing-operator-docs` loaded: scope, the same-namespace Secret (drop the "any namespace" note on these two pages only), the uniqueness rule in terms of the server, the dedicated-server rule across namespaces, and delete the stale "No kind in the operator reads it yet" line in `databaseserverconfig.md:58`.
- [ ] **Step 3:** `mkdocs build --strict`; all gates; commit `docs(rdbms): describe the namespaced chain and server identity (#128)`.
- [ ] **Step 4:** Open the sub-PR `fix(rdbms): make the RDBMS chain namespaced and key server identity on the system identifier` with base `feat/cnpg-database-server`, body `Towards #128`. Run `feature-dev-workflow:copilot-review-loop` to clean. CI's Camunda 8.9 job must be green (it runs the `database` and `camundacluster-rdbms` flows).

---

## PR 2: CloudNativePG and Barman Cloud wrappers (#234)

Branch `chore/cnpg-database-server--cnpg-wrappers` off `feat/cnpg-database-server`. Independent of PR 1.

### Task 2.1: the `Cluster` wrapper

**Files:**
- Modify: `go.mod` (`github.com/cloudnative-pg/api v1.30.0`), `go.sum`
- Create: `pkg/wrappers/cnpgcluster/{builder.go,mutator.go,resource.go,health.go,builder_test.go,health_test.go,component_smoke_test.go}` via `ocf scaffold wrapper` (load `ocf:custom-resource-wrappers`; the type is `cnpgv1.Cluster` from `github.com/cloudnative-pg/api/pkg/api/v1`, category workload)
- Create: `pkg/wrappers/cnpgcluster/applyclient.go` only if the envtest apply is rejected with "field not declared in schema"
- Create: `test/utils/cnpg.go` with `CNPGCRDPath() string` resolving `crds` from the `github.com/cloudnative-pg/api` module cache the way `ECKCRDPath` does (the CRDs ship under `config/crd/bases` in that module; verify with `go list -m -f '{{.Dir}}' github.com/cloudnative-pg/api` and `find`)

**Interfaces:**
- Produces: `cnpgcluster.NewBuilder(obj *cnpgv1.Cluster) *Builder`, `(*Builder).WithMutation(...Mutation)`, `(*Builder).Build() (*Resource, error)`, `type Mutation = feature.Mutation[*Mutator]`, `DefaultConvergingStatusHandler`, and the identity string `postgresql.cnpg.io/v1/Cluster/<ns>/<name>`.
- Health: `status.phase == "Cluster in healthy state"` and `status.readyInstances == spec.instances` → Healthy; phase contains `"Failed"` or `"unrecoverable"` → Failing; first apply → Creating; else Updating. Read the phase constants from `cnpgv1` (`PhaseHealthy`, `PhaseFailed`, ...) instead of string literals.
- Suspend: the suspend mutation sets the annotation `cnpg.io/hibernation: "on"` (declarative hibernation); CloudNativePG removes the pods and keeps the PVCs. `spec.instances` has `minimum: 1` in the CRD, so scale-to-zero is impossible. The suspension status reads the `cnpg.io/hibernation` condition, never `status.instances` (that counts PVC groups). The delete-on-suspend decision is never.

- [ ] **Step 1:** `go get github.com/cloudnative-pg/api@v1.30.0`; `go mod tidy`; confirm `api/go.mod` is untouched.
- [ ] **Step 2:** Scaffold; write `health_test.go` for the five cases above; run to fail; implement `health.go`; run to pass.
- [ ] **Step 3:** `builder_test.go`: a builder with one mutation that sets `spec.instances` applies it; a missing name fails `Build`. `component_smoke_test.go`: register the resource in a component and preview it against the golden testdata.
- [ ] **Step 4:** envtest in `pkg/wrappers/cnpgcluster/envtest_test.go`: start envtest with `CNPGCRDPath()` and apply a minimal `Cluster` through the wrapper. If the apply is rejected on schema, add `applyclient.go` the way `eckelasticsearch/applyclient.go` does.
- [ ] **Step 5:** Register the scheme in `internal/testenv/testenv.go` (`cnpgv1.AddToScheme`) and in `cmd/main.go`; load the CRD path in `testenv.Start` (PR 3 adds the `WithoutCNPG` option; here it is always loaded).
- [ ] **Step 6:** Gates; commit `chore(wrappers): wrap the CloudNativePG Cluster as an ocf primitive (#234)`.

### Task 2.2: the `ObjectStore` wrapper with hand-written types

**Files:**
- Create: `pkg/wrappers/barmanobjectstore/{types.go,zz_generated.deepcopy.go,doc.go,builder.go,mutator.go,resource.go,builder_test.go}`
- Create: `test/utils/crds/barmancloud.cnpg.io_objectstores.yaml` (copied from the plugin's `v0.14.0` release manifest; record the source URL in a comment at the top) and `test/utils.BarmanCRDPath() string`

**Interfaces:**
- Produces:

```go
// Package barmanobjectstore wraps the ObjectStore kind of the Barman Cloud plugin
// (barmancloud.cnpg.io/v1). The plugin publishes no Go module, so the types here
// are the subset this operator writes.
type ObjectStore struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`
	Spec              ObjectStoreSpec   `json:"spec"`
	Status            ObjectStoreStatus `json:"status,omitempty"`
}
type ObjectStoreSpec struct {
	Configuration                BarmanObjectStoreConfiguration `json:"configuration"`
	RetentionPolicy              string                         `json:"retentionPolicy,omitempty"` // e.g. "30d"
	InstanceSidecarConfiguration *InstanceSidecarConfiguration  `json:"instanceSidecarConfiguration,omitempty"`
}
type BarmanObjectStoreConfiguration struct {
	DestinationPath   string                 `json:"destinationPath"`
	EndpointURL       string                 `json:"endpointURL,omitempty"`
	EndpointCA        *SecretKeySelector     `json:"endpointCA,omitempty"`
	S3Credentials     *S3Credentials         `json:"s3Credentials,omitempty"`
	AzureCredentials  *AzureCredentials      `json:"azureCredentials,omitempty"`
	GoogleCredentials *GoogleCredentials     `json:"googleCredentials,omitempty"`
	Wal               *WalBackupConfiguration  `json:"wal,omitempty"`
	Data              *DataBackupConfiguration `json:"data,omitempty"`
}
```

  Copy the exact field names and JSON tags from the plugin's CRD (`spec.versions[0].schema`), including `S3Credentials{AccessKeyID, SecretAccessKey *SecretKeySelector; InheritFromIAMRole bool}`, `AzureCredentials{StorageAccount, StorageKey, StorageSasToken *SecretKeySelector; InheritFromAzureAD bool}`, `GoogleCredentials{ApplicationCredentials *SecretKeySelector; GKEEnvironment bool}`. Where the CRD says `retentionPolicy` lives (top level of `spec` vs `spec.configuration`), follow the CRD; the plan's guess is not authoritative.
- Category: static resource (`generic.StaticResource`, the `servicemonitor` precedent). Identity `barmancloud.cnpg.io/v1/ObjectStore/<ns>/<name>`.

- [ ] **Step 1:** Fetch the CRD from `https://github.com/cloudnative-pg/plugin-barman-cloud/releases/download/v0.14.0/manifest.yaml`, extract the `objectstores.barmancloud.cnpg.io` document into the testdata file.
- [ ] **Step 2:** Write `types.go` from the schema; `controller-gen object paths=./pkg/wrappers/barmanobjectstore/...` for deep-copy (add the package to the `generate` target the way `keycloak` is).
- [ ] **Step 3:** `builder_test.go`: build and preview an S3 store with static credentials; assert the JSON round-trips through the CRD schema in envtest (apply against `BarmanCRDPath()`).
- [ ] **Step 4:** Register the scheme in testenv and `cmd/main.go`. Gates; commit `chore(wrappers): wrap the Barman Cloud ObjectStore as an ocf primitive (#234)`.

### Task 2.3: `ScheduledBackup` and `PodMonitor` wrappers

**Files:**
- Create: `pkg/wrappers/cnpgscheduledbackup/{builder.go,mutator.go,resource.go,builder_test.go}` (static resource over `cnpgv1.ScheduledBackup`)
- Create: `pkg/wrappers/podmonitor/{builder.go,mutator.go,resource.go,builder_test.go}` (static resource over `monitoringv1.PodMonitor`, mirror of `pkg/wrappers/servicemonitor`)

- [ ] **Step 1:** Scaffold both; builder tests that preview a `ScheduledBackup{Spec: {Schedule, Immediate: ptr(true), Method: cnpgv1.BackupMethodPlugin, PluginConfiguration: {Name: "barman-cloud.cloudnative-pg.io"}, Cluster: {Name}}}` and a `PodMonitor` selecting `cnpg.io/cluster: <name>` on port `metrics`.
- [ ] **Step 2:** Gates; commit `chore(wrappers): wrap ScheduledBackup and PodMonitor as ocf primitives (#234)`.
- [ ] **Step 3:** Open the sub-PR `chore(wrappers): wrap the CloudNativePG kinds as ocf primitives`, base `feat/cnpg-database-server`, `Towards #234`; review loop to clean.

---

## PR 3: the `DatabaseServer` kind (#235)

Branch `feat/cnpg-database-server--database-server` off `feat/cnpg-database-server` after PR 1 and PR 2 merged.

### Task 3.1: types, preset, samples

**Files:**
- Create via `kubebuilder create api --group core --version v1 --kind DatabaseServer` and `--kind DatabaseServerPreset --controller=false`: `api/v1/databaseserver_types.go`, `api/v1/databaseserverpreset_types.go`, `internal/controller/databaseserver/controller.go`, `config/samples/core_v1_databaseserver.yaml`, `config/samples/core_v1_databaseserverpreset.yaml`, entries in `PROJECT`, `config/crd/kustomization.yaml`, `config/rbac/`, `config/samples/kustomization.yaml`
- Test: `api/v1/databaseserver_types_test.go` (CEL through `internal/controller/databaseserver/schema_test.go` and `preset_schema_test.go`, the `elasticsearchcluster` precedent)

**Interfaces:**
- Produces:

```go
type DatabaseServerSpec struct {
	PresetRef        string                                `json:"presetRef,omitempty"`
	// +kubebuilder:validation:Pattern=`^\d+$`
	Version          string                                `json:"version,omitempty"`
	// +kubebuilder:validation:Minimum=1
	Instances        *int32                                `json:"instances,omitempty"`
	Resources        *corev1.ResourceRequirements          `json:"resources,omitempty"`
	StorageSize      *resource.Quantity                    `json:"storageSize,omitempty"`
	StorageClassName *string                               `json:"storageClassName,omitempty"`
	WALStorageSize   *resource.Quantity                    `json:"walStorageSize,omitempty"`
	ServiceAccount   *ServiceAccountSpec                   `json:"serviceAccount,omitempty"`
	Scheduling       *SchedulingSpec                       `json:"scheduling,omitempty"`
	PodLabels        map[string]string                     `json:"podLabels,omitempty"`
	PodAnnotations   map[string]string                     `json:"podAnnotations,omitempty"`
	Monitoring       *DatabaseServerMonitoringSpec         `json:"monitoring,omitempty"`
	PersistentVolumeClaimRetentionPolicy *PersistentVolumeClaimRetentionPolicy `json:"persistentVolumeClaimRetentionPolicy,omitempty"`
	DatabaseServerConfig string                            `json:"databaseServerConfig,omitempty"`
	Archive          *DatabaseServerArchiveSpec            `json:"archive,omitempty"`
	Suspend          bool                                  `json:"suspend,omitempty"`
}
type DatabaseServerMonitoringSpec struct {
	PodMonitor *PodMonitorSpec `json:"podMonitor,omitempty"` // {Enabled bool, Labels map[string]string, Interval string}
}
type DatabaseServerArchiveSpec struct {
	ObjectStorageRef    string `json:"objectStorageRef"`
	// +kubebuilder:validation:Minimum=1
	RetentionPeriodDays int32  `json:"retentionPeriodDays"`
	// +kubebuilder:default="0 0 2 * * *"
	BaseBackupSchedule  string `json:"baseBackupSchedule,omitempty"`
}
type ArchiveRecord struct {
	ServerName string       `json:"serverName"`
	From       metav1.Time  `json:"from"`
	To         *metav1.Time `json:"to,omitempty"`
}
type DatabaseServerArchiveStatus struct {
	History []ArchiveRecord `json:"history,omitempty"`
}
type DatabaseServerRecoveryStatus struct { // filled in PR 4
	RequestedBy string       `json:"requestedBy,omitempty"`
	TargetTime  string       `json:"targetTime,omitempty"`
	Cluster     string       `json:"cluster,omitempty"`
	CompletedAt *metav1.Time `json:"completedAt,omitempty"`
}
type DatabaseServerStatus struct {
	ObservedGeneration int64                         `json:"observedGeneration,omitempty"`
	Cluster            string                        `json:"cluster,omitempty"`
	SystemIdentifier   string                        `json:"systemIdentifier,omitempty"`
	Archive            *DatabaseServerArchiveStatus  `json:"archive,omitempty"`
	Recovery           *DatabaseServerRecoveryStatus `json:"recovery,omitempty"`
	Volumes            []VolumeStatus                `json:"volumes,omitempty"`
	Conditions         []metav1.Condition            `json:"conditions,omitempty"`
}
const (
	ReasonCNPGNotInstalled         = "CNPGNotInstalled"
	ReasonBarmanPluginNotInstalled = "BarmanPluginNotInstalled"
)
```

  `DatabaseServerPresetSpec{Server DatabaseServerSpec}` with the CEL that forbids `presetRef`, `databaseServerConfig`, `suspend`. Type-level CEL on `DatabaseServer`: `databaseServerConfig` required after merge is a controller check (a preset cannot supply it, so the CEL requires it on the instance: `has(self.databaseServerConfig) && self.databaseServerConfig != ''`); `storageSize` may not shrink (`!has(oldSelf.storageSize) || !has(self.storageSize) || quantity(self.storageSize).compareTo(quantity(oldSelf.storageSize)) >= 0`, the ES precedent).
  Printcolumns: `Ready`, `Reason`, `Version` (`.spec.version`), `Age`. `+kubebuilder:resource:scope=Namespaced`.

- [ ] **Step 1:** Scaffold both kinds; write the types; `make manifests generate`.
- [ ] **Step 2:** `schema_test.go`: a server without `databaseServerConfig` is rejected; shrinking `storageSize` is rejected; `retentionPeriodDays: 0` is rejected; `preset_schema_test.go`: a preset with `suspend: true` is rejected.
- [ ] **Step 3:** Samples: `core_v1_databaseserver.yaml` (version `17`, one instance, `databaseServerConfig: databaseserver-sample`, no archive) and `core_v1_databaseserverpreset.yaml` (two instances, 20Gi, podMonitor enabled). `samples_schema_test.go` passes.
- [ ] **Step 4:** Gates; commit `feat(databaseserver): add the DatabaseServer and DatabaseServerPreset kinds (#235)`.

### Task 3.2: components

**Files:**
- Create: `pkg/components/databaseserver/{components.go,presetmerge.go,presetmerge_test.go,archive.go,archive_test.go,contract.go,contract_test.go,monitoring.go,monitoring_test.go,components_test.go,testdata/}`

**Interfaces:**
- Produces:

```go
const (
	ConditionCluster    = "ClusterReady"
	ConditionArchive    = "ArchiveReady"
	ConditionContract   = "ContractReady"
	ConditionMonitoring = "MonitoringReady"
	LabelDatabaseServer = "camunda.io/database-server"
	BarmanPluginName    = "barman-cloud.cloudnative-pg.io"
)
func MergePreset(server *v1.DatabaseServer, preset *v1.DatabaseServerPreset) (v1.DatabaseServerSpec, error)
func ValidateMerged(spec v1.DatabaseServerSpec) error // version floor, databaseServerConfig set
func ClusterName(server *v1.DatabaseServer) string     // status.cluster, or server.Name when empty
func ReadWriteHost(server *v1.DatabaseServer) string   // "<ClusterName>-rw.<ns>.svc"
func SuperuserSecretName(server *v1.DatabaseServer) string // "<ClusterName>-superuser"
func ObjectStoreName(server *v1.DatabaseServer) string // server.Name
// ArchiveStorage is the ObjectStorageConfig resolved once (the activeBlock precedent).
type ArchiveStorage struct { DestinationPath, EndpointURL string; Credentials *corev1.Secret; WorkloadIdentity bool; Provider v1.ObjectStorageType }
func ResolveArchiveStorage(ctx context.Context, c client.Reader, spec v1.DatabaseServerArchiveSpec) (*ArchiveStorage, error)
func ClusterComponent(server *v1.DatabaseServer, merged v1.DatabaseServerSpec, archive *ArchiveStorage, images images.Resolver) component.Component
func ArchiveComponent(server *v1.DatabaseServer, merged v1.DatabaseServerSpec, archive *ArchiveStorage) component.Component
func ContractComponent(server *v1.DatabaseServer, merged v1.DatabaseServerSpec) component.Component
func MonitoringComponent(server *v1.DatabaseServer, merged v1.DatabaseServerSpec, podMonitorSupported bool) component.Component
```

- The `Cluster` baseline: `instances`, `imageName` from `pkg/images` (`ghcr.io/cloudnative-pg/postgresql:<version>`), `storage{size, storageClass}`, `walStorage` when set, `resources`, `affinity` from `Scheduling`, `enableSuperuserAccess: true`, `inheritedMetadata{labels: podLabels + LabelDatabaseServer, annotations: podAnnotations}`, `serviceAccountTemplate` with the workload-identity annotations of the `ObjectStorageConfig` when `archive` uses workload identity, and `plugins: [{name: BarmanPluginName, isWALArchiver: true, parameters: {barmanObjectName: ObjectStoreName, serverName: ClusterName}}]` when `archive` is set.
- The `ObjectStore` baseline: `destinationPath` = the `ObjectStorageConfig` base path + `/databaseserver/<ns>/<name>/`, `endpointURL` for S3-compatible stores, the credentials block by provider (`s3Credentials` with `accessKeyId`/`secretAccessKey` from the resolved Secret, or `inheritFromIAMRole: true`; `azureCredentials`/`googleCredentials` likewise), `retentionPolicy: "<days>d"`, `wal.compression: gzip`, `data.compression: gzip`.
- The `ScheduledBackup` baseline as in Task 2.3, `schedule` from `baseBackupSchedule`, `cluster.name: ClusterName`.
- The contract baseline: `engine: postgres`, `host: ReadWriteHost`, `port: 5432`, `adminCredentialsSecretRef{name: SuperuserSecretName}`, `pitr{enabled: archive != nil, retentionPeriodDays, recovery: external}` (PR 4 flips it to `operator`); owner reference to the server; label `LabelDatabaseServer`.
- `ArchiveReady` reads the `ScheduledBackup`'s last `Backup` (`status.lastScheduleTime` and the `Backup` list labelled `cnpg.io/cluster`) and is `True` when at least one `Backup` has `status.phase: completed` for the current `ClusterName`. Use a declared-data cell on the `ScheduledBackup` resource for the observation, the `ocf:building-components` way.

- [ ] **Step 1:** `presetmerge_test.go` (mirror the ES test table) → `presetmerge.go`.
- [ ] **Step 2:** `archive_test.go`: `ResolveArchiveStorage` against a fake reader for S3 static, S3 workload identity, GCS, Azure; the `ObjectStore` and `Cluster` plugin block golden snapshots for each. `contract_test.go`: the published contract golden snapshot with and without archive. `monitoring_test.go`: PodMonitor present only when enabled and supported.
- [ ] **Step 3:** Implement each component; `components_test.go` runs the golden suite over `testdata/`.
- [ ] **Step 4:** Gates; commit `feat(databaseserver): build the server, archive, contract, and monitoring components (#235)`.

### Task 3.3: controller, without-CRD start, docs

**Files:**
- Modify: `internal/controller/databaseserver/controller.go` (scaffolded), `cmd/main.go` (register, pass `--camunda-operator-cli-image` not needed here), `internal/testenv/testenv.go` (`Options{WithoutECK, WithoutCNPG bool}`; CRD paths include `CNPGCRDPath()` and `BarmanCRDPath()` unless `WithoutCNPG`)
- Create: `internal/controller/databaseserver/{suite_test.go,controller_test.go,withoutcnpg/suite_test.go,withoutcnpg/cnpg_test.go}`
- Create: `docs/crds/databaseserver.md`, `docs/crds/databaseserverpreset.md`; modify `docs/crds/index.md`, `mkdocs.yml` nav (Storage group), `docs/crds/databaseserverconfig.md` (producers table: `DatabaseServer`)

**Interfaces:**
- Produces: `databaseserver.Reconciler{Client, Scheme, componentClient, restMapper, cnpgInstalled, barmanInstalled bool, retryInterval}`; RBAC markers for `postgresql.cnpg.io/clusters;scheduledbackups;backups`, `barmancloud.cnpg.io/objectstores`, `monitoring.coreos.com/podmonitors`, `core.camunda.io/databaseservers;databaseserverpresets;databaseserverconfigs;objectstorageconfigs`, Secrets get/list/watch.
- Reconcile: `preCheck` (cnpgInstalled → presetRef → merge → validate → archive resolve with `barmanInstalled` when `archive` set) → build components → `Reconcile` each → `conditions.Aggregate` → `FlushStatus`. `status.cluster` is set to `server.Name` on first reconcile. `status.systemIdentifier` is copied from the `Cluster`'s `status.systemID` through a declared-data cell.

- [ ] **Step 1:** Controller tests (envtest): (a) a server without a preset reaches `ClusterReady=True` once the test writes `status.phase: "Cluster in healthy state"` and `readyInstances` on the `Cluster`, and the contract exists with `host: <name>-rw.<ns>.svc` and `pitr.enabled: false`; (b) with an archive on an S3 `ObjectStorageConfig`, the `ObjectStore` and `ScheduledBackup` exist, `ArchiveReady=False` until the test creates a completed `Backup`, then `True`, and the contract says `pitr.enabled: true`; (c) `presetRef` to a missing preset → `InvalidReference`; (d) `suspend: true` sets the `cnpg.io/hibernation: "on"` annotation on the `Cluster`; (e) `status.systemIdentifier` mirrors the `Cluster`'s `status.systemID`.
- [ ] **Step 2:** Implement the controller from the `elasticsearchcluster` controller shape.
- [ ] **Step 3:** `withoutcnpg` suite: `testenv.StartWith(Options{WithoutCNPG: true}, ...)`; a `DatabaseServer` reports `Ready=False`, reason `CNPGNotInstalled`, message `CloudNativePG is not installed on this cluster. Install it, then restart the operator`.
- [ ] **Step 4:** Docs from `TEMPLATE.md` with `writing-operator-docs`: what the server gives you, the archive and base backups paragraph (from the spec's "base backups are not the backup model"), presets, monitoring, suspend, status, spec reference, validation rules, examples.
- [ ] **Step 5:** All gates; commit `feat(databaseserver): reconcile a CloudNativePG Cluster and publish its contract (#235)`; open the sub-PR `feat(databaseserver): run PostgreSQL from a DatabaseServer`, `Towards #235`; review loop to clean.

---

## PR 4: recovery through the contract (#236)

Branch `feat/cnpg-database-server--contract-recovery` off `feat/cnpg-database-server` after PR 3 merged.

### Task 4.1: the contract's recovery fields

**Files:**
- Modify: `api/v1/databaseserverconfig_types.go`, generated files, `config/samples/core_v1_databaseserverconfig.yaml` (unchanged, `external` is the default), `docs/crds/databaseserverconfig.md`

**Interfaces:**
- Produces:

```go
// +kubebuilder:validation:Enum=operator;external
type RecoveryMode string
const (
	RecoveryModeOperator RecoveryMode = "operator"
	RecoveryModeExternal RecoveryMode = "external"
)
// +kubebuilder:validation:Enum=Completed;Failed;Unavailable
type RecoveryResult string
const (
	RecoveryResultCompleted   RecoveryResult = "Completed"
	RecoveryResultFailed      RecoveryResult = "Failed"
	RecoveryResultUnavailable RecoveryResult = "Unavailable"
)
type RecoveryRequest struct {
	RequestedBy string `json:"requestedBy"` // "<namespace>/<name>" of the PointInTimeRestore
	// +kubebuilder:validation:Format=date-time
	TargetTime  string `json:"targetTime"`  // RFC 3339 with an explicit zone
}
type RecoveryOutcome struct {
	RequestedBy string         `json:"requestedBy"`
	TargetTime  string         `json:"targetTime"`
	CompletedAt metav1.Time    `json:"completedAt"`
	Result      RecoveryResult `json:"result"`
	Message     string         `json:"message,omitempty"`
}
type PITRCapability struct {
	Enabled             bool             `json:"enabled,omitempty"`
	RetentionPeriodDays *int32           `json:"retentionPeriodDays,omitempty"`
	// +kubebuilder:default=external
	Recovery            RecoveryMode     `json:"recovery,omitempty"`
	LastRecovery        *RecoveryOutcome `json:"lastRecovery,omitempty"`
}
// on DatabaseServerConfigSpec:
Recovery *RecoveryRequest `json:"recovery,omitempty"`
```

  CEL on `PITRCapability`: `self.recovery != 'operator' || self.enabled` with message `recovery: operator requires enabled: true`. Helper `func (r RecoveryRequest) Matches(o *RecoveryOutcome) bool` (same `RequestedBy` and `TargetTime`).

- [ ] **Step 1:** Types, CEL schema test (`recovery: operator` with `enabled: false` rejected; a bad `targetTime` rejected), generate, docs section "Recovery request" on the contract page.
- [ ] **Step 2:** Commit `feat(databaseserverconfig): carry a recovery request and its outcome (#236)`.

### Task 4.2: `PointInTimeRestore` phase `RestoringDatabase`

**Files:**
- Modify: `api/v1/pointintimerestore_types.go` (phase enum `:49`, phase docs `:53-78`, kind doc `:178-194`)
- Create: `internal/controller/pointintimerestore/dbrecovery.go`, `dbrecovery_test.go`
- Modify: `internal/controller/pointintimerestore/controller.go` (phase switch `:189-256`), `admit.go` (`admit` `:72` routes to the new phase; `pinnedChainCurrent` tolerates the identifier change while the phase is `RestoringDatabase`), RBAC marker: `databaseserverconfigs` gains `patch`
- Modify: `docs/crds/pointintimerestore.md` (`:7`, `:85` phases, `:97` chain, `:139-163` "choosing the point")

**Interfaces:**
- Produces: `PhaseRestoringDatabase PointInTimeRestorePhase = "RestoringDatabase"`; `const recoveryFieldManager = "pointintimerestore.core.camunda.io/recovery"`; `func recoveryRequest(pitr *v1.PointInTimeRestore) v1.RecoveryRequest` rendering `spec.timestamp.UTC().Format(time.RFC3339)`.

- [ ] **Step 1:** Tests (envtest in the existing suite): a pinned contract with `pitr.recovery: operator` → phase `RestoringDatabase` and `spec.recovery` written on the contract with the manager above; the test writes `lastRecovery` as the producer: `Completed` → phase `ValidatingDatabaseState` and `status.storage.systemIdentifier` refreshed; `Unavailable` → `Failed` with reason `PitrUnavailable` and the message; `Failed` → `Failed`; `external` → straight to `ValidatingDatabaseState` as today; an unanswered request stays in `RestoringDatabase` with message `Waiting for DatabaseServerConfig <ns>/<name> to answer the recovery request from <ns>/<pitr>`.
- [ ] **Step 2:** Implement `dbrecovery.go`: `enterDatabaseRecovery` applies the request through `client.Apply` with `client.ForceOwnership` and the field manager, then polls every `defaultPollInterval`.
- [ ] **Step 3:** Docs; gates; commit `feat(pointintimerestore): ask the contract to recover the database before the primary restore (#236)`.

### Task 4.3: recovery in `DatabaseServer`

**Files:**
- Create: `pkg/components/databaseserver/recovery.go`, `recovery_test.go`
- Modify: `pkg/components/databaseserver/components.go` (`ClusterName` reads `status.cluster`; the contract baseline sets `pitr.recovery: operator` when `archive` is set and copies `lastRecovery` from status), `internal/controller/databaseserver/controller.go`, `controller_test.go`, `docs/crds/databaseserver.md` ("Recovery" section)

**Interfaces:**
- Produces:

```go
// SelectArchive returns the archive whose interval holds target, or an error naming the covered intervals.
func SelectArchive(history []v1.ArchiveRecord, target time.Time) (v1.ArchiveRecord, error)
func RecoveryClusterName(server *v1.DatabaseServer) string // "<name>-r<N>", N = len(history)
func RecoveryCluster(server *v1.DatabaseServer, merged v1.DatabaseServerSpec, archive *ArchiveStorage, source v1.ArchiveRecord, target string) *cnpgv1.Cluster
```

  `RecoveryCluster` copies the baseline and sets `bootstrap.recovery{source: "source", recoveryTarget{targetTime: target}}`, `externalClusters: [{name: "source", plugin{name: BarmanPluginName, parameters{barmanObjectName: ObjectStoreName, serverName: source.ServerName}}}]`, `plugins[0].parameters.serverName = RecoveryClusterName`.
- Controller: a `recovery` on the owned contract that does not match `pitr.lastRecovery` and does not match `status.recovery` starts the steps of the spec section "Recovery in DatabaseServer". Each step keys on what exists: the recovery `Cluster` absent → create; present and not Healthy → wait (or `Failed` when its phase is failed: delete it, write `lastRecovery{Failed}`); Healthy and contract host is old → apply contract with new host + `lastRecovery{Completed}` and set `status.cluster`; old `Cluster` present → delete; history not closed → close and append; `status.recovery` unset → set. A request while `suspend` is true → `lastRecovery{Failed, "server is suspended"}`.

- [ ] **Step 1:** `recovery_test.go`: `SelectArchive` for a target inside the first, inside the current (open) interval, before all, after all, on a boundary (belongs to the later interval); `RecoveryCluster` golden snapshot.
- [ ] **Step 2:** Controller tests: full happy path with the test moving the recovery `Cluster` to Healthy (assert contract host, `lastRecovery`, old `Cluster` gone, history closed and appended, new `ScheduledBackup`); `Unavailable` for a target before history; `Failed` for a failed phase and for suspend; a restart mid-way (delete and recreate the reconciler between steps) resumes.
- [ ] **Step 3:** Implement; docs section; gates; commit `feat(databaseserver): recover the server to a requested point in time (#236)`.
- [ ] **Step 4:** Open the sub-PR `feat(restore): recover a DatabaseServer through its contract before a point-in-time restore`, `Towards #236`; review loop to clean.

---

## PR 5: e2e and installation docs (#237)

Branch `ci/cnpg-database-server--e2e` off `feat/cnpg-database-server` after PR 4 merged.

### Task 5.1: install CloudNativePG and the plugin in e2e

**Files:**
- Create: `test/utils/cnpginstall.go` (`CNPGVersion()`, `BarmanPluginVersion()`, `IsCNPGInstalled()`, `InstallCNPG()`, `InstallBarmanPlugin()`, `UninstallCNPG()`, mirror of `test/utils/eck.go`; manifests `https://raw.githubusercontent.com/cloudnative-pg/cloudnative-pg/release-<major.minor>/releases/cnpg-<version>.yaml` and `https://github.com/cloudnative-pg/plugin-barman-cloud/releases/download/v<version>/manifest.yaml`; rollout wait on `cnpg-system/cnpg-controller-manager` and `cnpg-system/barman-cloud`)
- Modify: `test/e2e/e2e_suite_test.go` (install after cert-manager, before ECK), `test/e2e/matrix/8.9.env` (`CNPG_VERSION=1.30.0 # renovate: datasource=github-releases depName=cloudnative-pg/cloudnative-pg`, `BARMAN_PLUGIN_VERSION=0.14.0 # renovate: datasource=github-releases depName=cloudnative-pg/plugin-barman-cloud`), `test/e2e/matrix_test.go:27-49` (required vars), `Makefile:100-157` (`CNPG_INSTALL_SKIP`, pass both vars), `renovate.json5` (minor bounds and the `separateMinorPatch` group, the `renovate-dry-run-technique` memory)

- [ ] **Step 1:** Helpers, suite wiring, matrix vars; `make lint-renovate`; `go vet -tags=e2e ./test/e2e/`.
- [ ] **Step 2:** Commit `ci(e2e): install CloudNativePG and the Barman Cloud plugin (#237)`.

### Task 5.2: the `databaseserver` flow and the operator-driven restore

**Files:**
- Create: `test/e2e/databaseserver_test.go` (`Describe("DatabaseServer", Ordered, Label(labelDatabaseServer))`)
- Modify: `test/e2e/matrix_test.go:54-79` (`labelDatabaseServer = "databaseserver"` in `allLabels`), `test/e2e/restore_test.go` (new helper `itRunsAPointInTimeRestoreThroughTheDatabaseServer`), `test/e2e/camundacluster_test.go:500+` (the RDBMS flow provisions its server through a `DatabaseServer` with an archive on MinIO and calls the new helper), `.github/workflows/*` if the flow label filter is per job

- [ ] **Step 1:** `databaseserver_test.go`: create an `ObjectStorageConfig` on MinIO (`testdata/minio.yaml`), a `DatabaseServer` with `archive` and one instance; expect `Ready=True` within 10 minutes; expect the contract with `pitr.enabled: true`, `recovery: operator`, `status.systemIdentifier` set; list the bucket prefix and expect a `base/` entry; create a `Database` on it and expect its bindings.
- [ ] **Step 2:** The restore helper: deploy a process, note `t1`, deploy another, create a `PointInTimeRestore` at `t1`; expect phases `RestoringDatabase` → `ValidatingDatabaseState` → `RestoringPrimaryStorage` → `Completed`; the contract `host` names `<server>-r1-rw`; only the first process is visible after the cluster unsuspends.
- [ ] **Step 3:** `go vet -tags=e2e ./test/e2e/`; commit `test(e2e): prove DatabaseServer and the operator-driven point-in-time restore (#237)`.

### Task 5.3: installation and guide docs

**Files:**
- Modify: `docs/installation.md`, `docs/getting-started.md`, `README.md`, `docs/architecture.md`, `docs/crds/index.md`, `docs/guides/secondary-storage.md`, `docs/guides/operations.md` (recovery restarts the orchestration cluster; old archives stay in the bucket)

- [ ] **Step 1:** With `writing-operator-docs`: CloudNativePG + cert-manager + plugin as optional installs next to ECK, with the version floor (1.26, plugin 0.14); the topology recommendation and its reason; what a shared server gives up; the base-backup paragraph in the backup guide.
- [ ] **Step 2:** `mkdocs build --strict`; commit `docs: install CloudNativePG for DatabaseServer and recommend one server per cluster (#237)`.
- [ ] **Step 3:** Open the sub-PR `ci(e2e): prove DatabaseServer and operator-driven point-in-time restore`, `Towards #237`; review loop to clean; CI green including the new flow.

---

## Checkpoint and hand-off

After PR 5 merges into `feat/cnpg-database-server`: `feature-dev-workflow:reviewing-feature-progress` on the feature worktree, all gates on the merged branch, then the integration PR `feat/cnpg-database-server` → `main` with `Closes #127` is the user's to open and merge. The plan and the state file are deleted in the teardown commit; the spec stays.
