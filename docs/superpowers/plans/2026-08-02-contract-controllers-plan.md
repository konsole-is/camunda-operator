# Contract-CRD Validation Controllers Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking. Every implementer MUST invoke the `how-we-write-go` skill before writing any Go.

**Goal:** Implement the five contract-CRD validation controllers (DatabaseServerConfig, DatabaseConfig, SecondaryStorageConfig, ObjectStorageConfig, ManagementAuthConfig) plus the shared foundations they need.

**Architecture:** Real `api/v1` types with admission-time validation (kubebuilder markers + CEL) replace the placeholder stubs; five thin plain controller-runtime reconcilers share `pkg/` helpers for conditions/SSA status patching, Secret-key checks, and reference-index watch mapping. Secrets are watched metadata-only with uncached single-Secret reads. Spec: `docs/superpowers/specs/2026-08-02-contract-controllers-design.md`; behavioral contract: `docs/crds/<kind>.md` per kind.

**Tech Stack:** Go 1.26, controller-runtime v0.24, k8s.io v0.36, Ginkgo/Gomega + envtest, testify. No new dependencies.

## Global Constraints

- The docs bind the implementation: types, rules, reasons, and watch behavior follow `docs/crds/<kind>.md` exactly; any doc error found is fixed in the same PR.
- All API writes use SSA with field owner `camunda-operator` (`conditions.FieldOwner`). No other patch/update types.
- Condition reasons are exactly `Healthy`, `MissingSecret`, `InvalidReference`. No finalizers, no provisioning, no connectivity probing, no requeue on success (`ctrl.Result{}`), transient API errors returned for backoff.
- Every exported symbol gets a GoDoc contract comment (see `how-we-write-go`).
- `make all` and `make test` green before every PR.
- operator-component-framework is NOT imported by any code in this feature (tooling-only this batch).

## Orchestration

Feature-branch model on `feature/batch-a-contract-controllers` (exists, pushed from planning). PR 1 (foundations, #18) merges into the feature branch first; PRs 2–6 (#19–#23) then branch off the updated feature branch and run **in parallel**; a final integration PR closes #17 into `main`.

- Sub-branch names: `batch-a/foundations`, `batch-a/databaseserverconfig`, `batch-a/databaseconfig`, `batch-a/secondarystorageconfig`, `batch-a/objectstorageconfig`, `batch-a/managementauthconfig`.
- Sub-PRs target `feature/batch-a-contract-controllers`, body `Towards #<sub-issue>`; integration PR targets `main`, body `Closes #17`.
- Commit subjects carry the sub-issue ref: `feat(api): DatabaseServerConfig types (#18)`.
- **Review checkpoint:** after PR 1 merges, pause for user review — it locks every convention the fan-out inherits.

## Contracts

All contracts are realized by the foundations PR (#18) merging into the feature branch before the consumers branch off — the "pre-merge stub PR" pattern where the stub PR is the real foundations PR.

| Name | Producer | Consumers | Shape | Realization |
| --- | --- | --- | --- | --- |
| shared-ref-types | #18 | #19–#23 | `v1.CredentialsSecretRef{Name, Namespace, UsernameKey, PasswordKey string}`, `v1.SecretKeyRef{Name, Namespace, Key string}` | foundations PR |
| contract-specs | #18 | #19–#23 | Full spec/status types for all five kinds (Tasks 2–6) | foundations PR |
| conditions-api | #18 | #19–#23 | `conditions.Ready(status, reason, message string, observedGeneration int64) metav1.Condition`; `conditions.PatchReady(ctx, c client.Client, obj conditions.Object, cond metav1.Condition) error`; consts `TypeReady`, `ReasonHealthy`, `ReasonMissingSecret`, `ReasonInvalidReference`, `FieldOwner` | foundations PR |
| secretref-api | #18 | #19, #20, #21, #23 | `secretref.CheckKeys(ctx, reader client.Reader, ref types.NamespacedName, keys ...string) (string, error)` | foundations PR |
| refindex-api | #18 | #19, #20, #21, #23 | `refindex.SecretKey(namespace, name string) string`; `refindex.ObjectNamespacedName(o client.Object) string`; `refindex.ObjectName(o client.Object) string`; `refindex.Enqueue(c client.Client, list client.ObjectList, field string, keyOf func(client.Object) string) handler.EventHandler` | foundations PR |
| suite-manager | #18 | #19–#23 | `suite_test.go` starts a manager with all five reconcilers registered as `{Client: mgr.GetClient(), APIReader: mgr.GetAPIReader(), Scheme: mgr.GetScheme()}`; consts `timeout = 10 * time.Second`, `interval = 250 * time.Millisecond` | foundations PR |

## Conventions

- **Package layout:** `pkg/conditions` (condition vocabulary + SSA status patch), `pkg/secretref` (Secret key checks), `pkg/refindex` (field-index keys + enqueue mapping). One responsibility each; no `util` packages.
- **Import aliases:** our API group imports as `v1 "github.com/konsole-is/camunda-operator/api/v1"`; core Kubernetes as `corev1 "k8s.io/api/core/v1"`. The kubebuilder scaffold aliased our API as `corev1` — every controller PR renames that alias in the files it touches; the foundations PR renames it in `suite_test.go` and `cmd/main.go` if present.
- **Reconciler struct:** `client.Client`, `APIReader client.Reader`, `Scheme *runtime.Scheme` — in that order, for all five.
- **Index field names:** unexported per-controller consts in `internal/controller`, prefixed by kind because the package is shared: `databaseServerConfigSecretRefsField = "databaseserverconfig.spec.secretRefs"`, `databaseConfigSecretRefsField`, `databaseConfigServerRefField = "databaseconfig.spec.serverRef"`, `secondaryStorageConfigSecretRefsField`, `secondaryStorageConfigDatabaseConfigRefField = "secondarystorageconfig.spec.rdbms.databaseConfigRef"`, `managementAuthConfigSecretRefsField`.
- **Condition messages (exact templates):** `Secret "<ns>/<name>" not found`; `Secret "<ns>/<name>" is missing key "<key>"`; `DatabaseServerConfig "<name>" not found`; `DatabaseConfig "<name>" not found`; healthy: `All checks passed`.
- **Secret watches:** `Watches(&corev1.Secret{}, refindex.Enqueue(...), builder.OnlyMetadata)` — metadata-only projection; Secret *data* is read via `r.APIReader` (uncached) inside `secretref.CheckKeys` only.
- **Cross-CR watches:** typed informers (`Watches(&v1.DatabaseServerConfig{}, ...)`), cached reads via `r.Client`.
- **RBAC markers:** each controller carries its own kind's markers (already scaffolded) plus `+kubebuilder:rbac:groups="",resources=secrets,verbs=get;list;watch` when it checks Secrets, and get;list;watch on any cross-referenced CR kind.
- **Error split:** validation outcomes become conditions and return `nil`; only transient API failures return an error.
- **Tests:** reconciliation specs `Describe("<Kind> controller", ...)` in `internal/controller/<kind>_controller_test.go`; schema specs `Describe("<Kind> schema", ...)` in `internal/controller/<kind>_schema_test.go` (foundations); pure logic gets testify table tests in `_test.go` files next to the code (`require`/`assert`, never `t.Fatal`). Envtest assertions poll with `Eventually(..., timeout, interval)`. Unique CR names via `"<prefix>-" + utilrand.String(8)` (`k8s.io/apimachinery/pkg/util/rand`) with `DeferCleanup` deletion.
- **Naming firewall:** "Batch A" appears only in branch names, issue titles, and this plan — never in Go identifiers, file names, fixtures, or test names.
- **Skills:** `how-we-write-go` before any Go; `verifying-camunda-app-config` is NOT needed this batch (no Camunda application config keys are produced).
- **Deliberate (recorded at the wave checkpoint):** `IndexField` errors in `SetupWithManager` are returned unwrapped, uniformly across all five controllers — setup-time errors abort manager startup where the call site is unambiguous, so the wrapping that `how-we-write-go` prefers for runtime paths adds no information here.

---

## PR 1 — Foundations (#18), branch `batch-a/foundations`

### Task 1: Shared secret-reference types

**Files:**
- Create: `api/v1/common_types.go`

**Interfaces:**
- Produces: `v1.CredentialsSecretRef`, `v1.SecretKeyRef` (consumed by every spec in Tasks 2–6).

- [ ] **Step 1: Write the types**

```go
package v1

// CredentialsSecretRef references a username/password pair stored in a Secret.
// Namespace is required: every referencing kind is cluster-scoped, so there is
// no namespace to default to.
type CredentialsSecretRef struct {
	// Name of the Secret holding the credentials.
	// +kubebuilder:validation:MinLength=1
	Name string `json:"name"`
	// Namespace of the Secret.
	// +kubebuilder:validation:MinLength=1
	Namespace string `json:"namespace"`
	// UsernameKey is the key in the Secret holding the plaintext username.
	// +kubebuilder:validation:MinLength=1
	UsernameKey string `json:"usernameKey"`
	// PasswordKey is the key in the Secret holding the plaintext password.
	// +kubebuilder:validation:MinLength=1
	PasswordKey string `json:"passwordKey"`
}

// SecretKeyRef references a single value inside a Secret.
// Namespace is required: every referencing kind is cluster-scoped, so there is
// no namespace to default to.
type SecretKeyRef struct {
	// Name of the Secret holding the value.
	// +kubebuilder:validation:MinLength=1
	Name string `json:"name"`
	// Namespace of the Secret.
	// +kubebuilder:validation:MinLength=1
	Namespace string `json:"namespace"`
	// Key in the Secret holding the value.
	// +kubebuilder:validation:MinLength=1
	Key string `json:"key"`
}
```

- [ ] **Step 2: Regenerate and verify**

Run: `make manifests generate && go build ./...`
Expected: builds; `zz_generated.deepcopy.go` gains DeepCopy for both types.

- [ ] **Step 3: Commit**

```bash
git add api/v1 && git commit -m "feat(api): shared secret-reference types (#18)"
```

### Task 2: DatabaseServerConfig types + schema tests

**Files:**
- Modify: `api/v1/databaseserverconfig_types.go` (replace placeholder spec/status)
- Create: `internal/controller/databaseserverconfig_schema_test.go`

**Interfaces:**
- Consumes: `CredentialsSecretRef` (Task 1).
- Produces: `v1.DatabaseServerConfigSpec`, `v1.DatabaseServerConfigStatus`, accessors `GetConditions() []metav1.Condition` / `GetObservedGeneration() int64`.

- [ ] **Step 1: Replace the placeholder types** (fields per `docs/crds/databaseserverconfig.md`)

```go
// DatabaseEngine identifies the database engine of a server.
// +kubebuilder:validation:Enum=postgres
type DatabaseEngine string

// DatabaseEnginePostgres is the PostgreSQL engine, currently the only engine
// the Database controller can bootstrap against.
const DatabaseEnginePostgres DatabaseEngine = "postgres"

// PITRCapability declares a server's point-in-time-recovery capability: that it
// performs continuous WAL archiving with the given retention.
// +kubebuilder:validation:XValidation:rule="!self.enabled || (has(self.retentionPeriodDays) && self.retentionPeriodDays >= 1)",message="retentionPeriodDays of at least 1 is required when enabled is true"
type PITRCapability struct {
	// Enabled reports whether the server performs continuous WAL archiving.
	// +kubebuilder:default=false
	// +optional
	Enabled bool `json:"enabled,omitempty"`
	// RetentionPeriodDays is how many days into the past a point-in-time
	// restore can target. Required when enabled is true.
	// +optional
	RetentionPeriodDays *int32 `json:"retentionPeriodDays,omitempty"`
}

// DatabaseServerConfigSpec describes a database server: engine, endpoint,
// admin credentials, and point-in-time-recovery capability.
type DatabaseServerConfigSpec struct {
	// Engine is the database engine of the server.
	Engine DatabaseEngine `json:"engine"`
	// Host the server is reachable at.
	// +kubebuilder:validation:MinLength=1
	Host string `json:"host"`
	// Port the server listens on.
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:validation:Maximum=65535
	Port int32 `json:"port"`
	// AdminCredentialsSecretRef names an admin user with permission to create
	// databases and roles; used by the Database controller to bootstrap.
	AdminCredentialsSecretRef CredentialsSecretRef `json:"adminCredentialsSecretRef"`
	// PITR declares the server's point-in-time-recovery capability.
	// +optional
	PITR *PITRCapability `json:"pitr,omitempty"`
}

// DatabaseServerConfigStatus is the observed validation state of the contract.
type DatabaseServerConfigStatus struct {
	// ObservedGeneration is the last generation reconciled by the operator.
	// +optional
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`
	// Conditions represent the current validation state; the Ready condition
	// carries reasons Healthy or MissingSecret.
	// +listType=map
	// +listMapKey=type
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

// GetConditions returns the resource's status conditions.
func (in *DatabaseServerConfig) GetConditions() []metav1.Condition { return in.Status.Conditions }

// GetObservedGeneration returns the last reconciled generation recorded in status.
func (in *DatabaseServerConfig) GetObservedGeneration() int64 { return in.Status.ObservedGeneration }
```

Keep the existing `DatabaseServerConfig`/`DatabaseServerConfigList` roots and `+kubebuilder:resource:scope=Cluster`; delete the `Foo` placeholder and scaffolding comments. GoDoc the root type as the contract CRD (one sentence from the doc's intro).

- [ ] **Step 2: Regenerate**

Run: `make manifests generate && go build ./...`
Expected: CRD base `config/crd/bases/core.camunda.io_databaseserverconfigs.yaml` gains the schema incl. the CEL rule.

- [ ] **Step 3: Write the schema tests (failing only if schema wrong)**

```go
package controller

// imports: v1 "github.com/konsole-is/camunda-operator/api/v1", utilrand "k8s.io/apimachinery/pkg/util/rand", ginkgo/gomega dot-imports, metav1, ptr "k8s.io/utils/ptr"

func validDatabaseServerConfig() *v1.DatabaseServerConfig {
	return &v1.DatabaseServerConfig{
		ObjectMeta: metav1.ObjectMeta{Name: "dbsc-" + utilrand.String(8)},
		Spec: v1.DatabaseServerConfigSpec{
			Engine: v1.DatabaseEnginePostgres,
			Host:   "postgres.camunda-system.svc.cluster.local",
			Port:   5432,
			AdminCredentialsSecretRef: v1.CredentialsSecretRef{
				Name: "admin-creds", Namespace: "camunda-system",
				UsernameKey: "username", PasswordKey: "password",
			},
		},
	}
}

var _ = Describe("DatabaseServerConfig schema", func() {
	DescribeTable("admission",
		func(mutate func(*v1.DatabaseServerConfig), wantErr string) {
			obj := validDatabaseServerConfig()
			mutate(obj)
			err := k8sClient.Create(ctx, obj)
			if wantErr == "" {
				Expect(err).NotTo(HaveOccurred())
				DeferCleanup(func() { _ = k8sClient.Delete(ctx, obj) })
			} else {
				Expect(err).To(MatchError(ContainSubstring(wantErr)))
			}
		},
		Entry("accepts the minimal doc example", func(*v1.DatabaseServerConfig) {}, ""),
		Entry("accepts pitr with retention", func(o *v1.DatabaseServerConfig) {
			o.Spec.PITR = &v1.PITRCapability{Enabled: true, RetentionPeriodDays: ptr.To(int32(7))}
		}, ""),
		Entry("rejects unknown engine", func(o *v1.DatabaseServerConfig) { o.Spec.Engine = "mysql" }, "spec.engine"),
		Entry("rejects port 0", func(o *v1.DatabaseServerConfig) { o.Spec.Port = 0 }, "spec.port"),
		Entry("rejects port above 65535", func(o *v1.DatabaseServerConfig) { o.Spec.Port = 70000 }, "spec.port"),
		Entry("rejects empty host", func(o *v1.DatabaseServerConfig) { o.Spec.Host = "" }, "spec.host"),
		Entry("rejects missing secret namespace", func(o *v1.DatabaseServerConfig) { o.Spec.AdminCredentialsSecretRef.Namespace = "" }, "namespace"),
		Entry("rejects pitr enabled without retention", func(o *v1.DatabaseServerConfig) {
			o.Spec.PITR = &v1.PITRCapability{Enabled: true}
		}, "retentionPeriodDays"),
		Entry("rejects pitr retention 0", func(o *v1.DatabaseServerConfig) {
			o.Spec.PITR = &v1.PITRCapability{Enabled: true, RetentionPeriodDays: ptr.To(int32(0))}
		}, "retentionPeriodDays"),
	)
})
```

- [ ] **Step 4: Run and verify**

Run: `make test`
Expected: PASS (schema enforces every entry).

- [ ] **Step 5: Commit**

```bash
git add api/v1 config internal/controller && git commit -m "feat(api): DatabaseServerConfig types and schema validation (#18)"
```

### Task 3: DatabaseConfig types + schema tests

**Files:**
- Modify: `api/v1/databaseconfig_types.go`
- Create: `internal/controller/databaseconfig_schema_test.go`

**Interfaces:**
- Consumes: `CredentialsSecretRef` (Task 1).
- Produces: `v1.DatabaseConfigSpec`, status + accessors (same shape as Task 2).

- [ ] **Step 1: Replace the placeholder types** (fields per `docs/crds/databaseconfig.md`)

```go
// DatabaseConfigSpec describes one logical database: its server, name, and
// application credentials.
type DatabaseConfigSpec struct {
	// ServerRef names the DatabaseServerConfig describing the server hosting
	// this database.
	// +kubebuilder:validation:MinLength=1
	ServerRef string `json:"serverRef"`
	// DatabaseName is the name of the logical database on the server.
	// +kubebuilder:validation:MinLength=1
	DatabaseName string `json:"databaseName"`
	// CredentialsSecretRef names an application user with read/write access to
	// the database.
	CredentialsSecretRef CredentialsSecretRef `json:"credentialsSecretRef"`
	// BackupCredentialsSecretRef names a separate user with dump/restore
	// privileges, used by the backup and restore controllers.
	// +optional
	BackupCredentialsSecretRef *CredentialsSecretRef `json:"backupCredentialsSecretRef,omitempty"`
}
```

Status + `GetConditions`/`GetObservedGeneration` accessors exactly as in Task 2 (status GoDoc names reasons `Healthy`, `InvalidReference`, `MissingSecret`).

- [ ] **Step 2: Regenerate** — `make manifests generate && go build ./...`

- [ ] **Step 3: Schema tests** — same table pattern as Task 2 with `validDatabaseConfig()` (minimal doc example: `serverRef: my-db-server`, `databaseName: camunda`, credentials ref). Entries: accepts minimal doc example; accepts with backup credentials ref; rejects empty `serverRef` (`"spec.serverRef"`); rejects empty `databaseName` (`"spec.databaseName"`); rejects missing credentials namespace (`"namespace"`); rejects backup ref with empty `usernameKey` (`"usernameKey"`).

- [ ] **Step 4: Run** — `make test` → PASS.

- [ ] **Step 5: Commit** — `git commit -m "feat(api): DatabaseConfig types and schema validation (#18)"`

### Task 4: SecondaryStorageConfig types + schema tests

**Files:**
- Modify: `api/v1/secondarystorageconfig_types.go`
- Create: `internal/controller/secondarystorageconfig_schema_test.go`

**Interfaces:**
- Produces: `v1.SecondaryStorageConfigSpec` with `Type`, `Elasticsearch *ElasticsearchStorage`, `RDBMS *RDBMSStorage`; constants `SecondaryStorageTypeElasticsearch`, `SecondaryStorageTypeRDBMS`; status + accessors.

- [ ] **Step 1: Replace the placeholder types** (fields per `docs/crds/secondarystorageconfig.md`)

```go
// SecondaryStorageType identifies which secondary storage backend a contract
// describes.
// +kubebuilder:validation:Enum=elasticsearch;rdbms
type SecondaryStorageType string

const (
	// SecondaryStorageTypeElasticsearch selects an Elasticsearch backend.
	SecondaryStorageTypeElasticsearch SecondaryStorageType = "elasticsearch"
	// SecondaryStorageTypeRDBMS selects a relational database backend.
	SecondaryStorageTypeRDBMS SecondaryStorageType = "rdbms"
)

// ElasticsearchStorage holds Elasticsearch connection details.
type ElasticsearchStorage struct {
	// Endpoint is the HTTP(S) endpoint of the Elasticsearch cluster.
	// +kubebuilder:validation:XValidation:rule="isURL(self) && (url(self).getScheme() == 'http' || url(self).getScheme() == 'https')",message="endpoint must be a valid http or https URL"
	Endpoint string `json:"endpoint"`
	// CredentialsSecretRef names a basic-auth user with read/write access to
	// the Camunda indices.
	CredentialsSecretRef CredentialsSecretRef `json:"credentialsSecretRef"`
}

// RDBMSStorage holds relational database backend details.
type RDBMSStorage struct {
	// DatabaseConfigRef names the DatabaseConfig describing the logical
	// database to use.
	// +kubebuilder:validation:MinLength=1
	DatabaseConfigRef string `json:"databaseConfigRef"`
}

// SecondaryStorageConfigSpec tells an orchestration cluster where its
// secondary storage lives and how to authenticate against it.
// +kubebuilder:validation:XValidation:rule="(self.type == 'elasticsearch') == has(self.elasticsearch) && (self.type == 'rdbms') == has(self.rdbms)",message="exactly the block matching spec.type must be set"
type SecondaryStorageConfigSpec struct {
	// Type selects which secondary storage backend this contract describes.
	Type SecondaryStorageType `json:"type"`
	// Elasticsearch connection details. Required when type is elasticsearch,
	// forbidden otherwise.
	// +optional
	Elasticsearch *ElasticsearchStorage `json:"elasticsearch,omitempty"`
	// RDBMS backend details. Required when type is rdbms, forbidden otherwise.
	// +optional
	RDBMS *RDBMSStorage `json:"rdbms,omitempty"`
}
```

Status + accessors as in Task 2.

- [ ] **Step 2: Regenerate** — `make manifests generate && go build ./...`

- [ ] **Step 3: Schema tests** — `validSecondaryStorageConfigES()` (doc minimal ES example) and `validSecondaryStorageConfigRDBMS()` (doc rdbms example). Entries: accepts ES doc example; accepts RDBMS doc example; rejects type elasticsearch without elasticsearch block (`"matching spec.type"`); rejects type rdbms with elasticsearch block set (`"matching spec.type"`); rejects both blocks set (`"matching spec.type"`); rejects unknown type (`"spec.type"`); rejects non-URL endpoint `"not a url"` (`"endpoint"`); rejects `ftp://x:9200` endpoint (`"endpoint"`); rejects empty `databaseConfigRef` (`"databaseConfigRef"`).

- [ ] **Step 4: Run** — `make test` → PASS.

- [ ] **Step 5: Commit** — `git commit -m "feat(api): SecondaryStorageConfig types and schema validation (#18)"`

### Task 5: ObjectStorageConfig types + schema tests

**Files:**
- Modify: `api/v1/objectstorageconfig_types.go`
- Create: `internal/controller/objectstorageconfig_schema_test.go`

**Interfaces:**
- Produces: `v1.ObjectStorageConfigSpec`; constants for providers (`ObjectStorageProviderAWS/GCP/Azure`) and types (`ObjectStorageTypeS3/GCS/AzureBlob`); status + accessors.

- [ ] **Step 1: Replace the placeholder types** (fields per `docs/crds/objectstorageconfig.md`)

```go
// ObjectStorageProvider identifies the cloud provider hosting a bucket.
// +kubebuilder:validation:Enum=aws;gcp;azure
type ObjectStorageProvider string

// ObjectStorageProviderAWS, ObjectStorageProviderGCP, and
// ObjectStorageProviderAzure are the supported bucket providers.
const (
	ObjectStorageProviderAWS   ObjectStorageProvider = "aws"
	ObjectStorageProviderGCP   ObjectStorageProvider = "gcp"
	ObjectStorageProviderAzure ObjectStorageProvider = "azure"
)

// ObjectStorageType identifies the storage API of a bucket.
// +kubebuilder:validation:Enum=S3;GCS;AzureBlob
type ObjectStorageType string

// ObjectStorageTypeS3, ObjectStorageTypeGCS, and ObjectStorageTypeAzureBlob
// are the supported storage APIs; each pairs with exactly one provider.
const (
	ObjectStorageTypeS3        ObjectStorageType = "S3"
	ObjectStorageTypeGCS       ObjectStorageType = "GCS"
	ObjectStorageTypeAzureBlob ObjectStorageType = "AzureBlob"
)

// ObjectStorageConfigSpec describes a cloud bucket and the workload identity
// trusted to access it. Access is granted through workload identity, so the
// contract references no Secrets.
// +kubebuilder:validation:XValidation:rule="(self.provider == 'aws' && self.type == 'S3') || (self.provider == 'gcp' && self.type == 'GCS') || (self.provider == 'azure' && self.type == 'AzureBlob')",message="spec.type must match spec.provider: aws pairs with S3, gcp with GCS, azure with AzureBlob"
type ObjectStorageConfigSpec struct {
	// Provider is the cloud provider hosting the bucket; it determines the
	// workload-identity mechanism.
	Provider ObjectStorageProvider `json:"provider"`
	// Type is the storage API of the bucket; it must match the provider.
	Type ObjectStorageType `json:"type"`
	// BucketID is the provider-specific unique identifier of the bucket, for
	// example an ARN on AWS.
	// +kubebuilder:validation:MinLength=1
	BucketID string `json:"bucketId"`
	// BucketName is the bucket name as used by storage client SDKs.
	// +kubebuilder:validation:MinLength=1
	BucketName string `json:"bucketName"`
	// BasePath is the key prefix under which consumers write objects. Empty
	// means the bucket root.
	// +optional
	BasePath string `json:"basePath,omitempty"`
	// AccountID is the workload identity the bucket trusts: an IAM role ARN
	// (aws), a service account email (gcp), or a managed identity client ID
	// (azure).
	// +kubebuilder:validation:MinLength=1
	AccountID string `json:"accountId"`
}
```

Status + accessors as in Task 2 (status GoDoc: only reason is `Healthy`).

- [ ] **Step 2: Regenerate** — `make manifests generate && go build ./...`

- [ ] **Step 3: Schema tests** — `validObjectStorageConfig()` (aws/S3 doc example). Entries: accepts aws/S3 doc example; accepts gcp/GCS doc example with basePath; accepts azure/AzureBlob; rejects aws+GCS (`"must match spec.provider"`); rejects gcp+S3 (`"must match spec.provider"`); rejects unknown provider (`"spec.provider"`); rejects empty bucketId (`"bucketId"`); rejects empty accountId (`"accountId"`).

- [ ] **Step 4: Run** — `make test` → PASS.

- [ ] **Step 5: Commit** — `git commit -m "feat(api): ObjectStorageConfig types and schema validation (#18)"`

### Task 6: ManagementAuthConfig types + schema tests

**Files:**
- Modify: `api/v1/managementauthconfig_types.go`
- Create: `internal/controller/managementauthconfig_schema_test.go`

**Interfaces:**
- Consumes: `SecretKeyRef` (Task 1).
- Produces: `v1.ManagementAuthConfigSpec`; status + accessors.

- [ ] **Step 1: Replace the placeholder types** (fields per `docs/crds/managementauthconfig.md`; the URL CEL rule is identical on every URL field)

```go
// ManagementAuthConfigSpec carries the Management Identity OIDC configuration:
// endpoints, machine-to-machine client credentials, and audience.
type ManagementAuthConfigSpec struct {
	// BaseURL is the base URL of the Management Identity service.
	// +kubebuilder:validation:XValidation:rule="isURL(self) && (url(self).getScheme() == 'http' || url(self).getScheme() == 'https')",message="baseUrl must be a valid http or https URL"
	BaseURL string `json:"baseUrl"`
	// IssuerURL is the OIDC issuer URL used to validate tokens.
	// +kubebuilder:validation:XValidation:rule="isURL(self) && (url(self).getScheme() == 'http' || url(self).getScheme() == 'https')",message="issuerUrl must be a valid http or https URL"
	IssuerURL string `json:"issuerUrl"`
	// IssuerBackendURL is the issuer URL for in-cluster container-to-container
	// communication. Consumers default it to IssuerURL when empty.
	// +kubebuilder:validation:XValidation:rule="isURL(self) && (url(self).getScheme() == 'http' || url(self).getScheme() == 'https')",message="issuerBackendUrl must be a valid http or https URL"
	// +optional
	IssuerBackendURL string `json:"issuerBackendUrl,omitempty"`
	// AuthURL is the OIDC authorization endpoint used for browser login
	// redirects.
	// +kubebuilder:validation:XValidation:rule="isURL(self) && (url(self).getScheme() == 'http' || url(self).getScheme() == 'https')",message="authUrl must be a valid http or https URL"
	AuthURL string `json:"authUrl"`
	// TokenURL is the OIDC token endpoint used to acquire machine-to-machine
	// tokens.
	// +kubebuilder:validation:XValidation:rule="isURL(self) && (url(self).getScheme() == 'http' || url(self).getScheme() == 'https')",message="tokenUrl must be a valid http or https URL"
	TokenURL string `json:"tokenUrl"`
	// JwksURL is the JWKS endpoint used to fetch token signing keys.
	// +kubebuilder:validation:XValidation:rule="isURL(self) && (url(self).getScheme() == 'http' || url(self).getScheme() == 'https')",message="jwksUrl must be a valid http or https URL"
	JwksURL string `json:"jwksUrl"`
	// ClientID is the default machine-to-machine client ID.
	// +kubebuilder:validation:MinLength=1
	ClientID string `json:"clientId"`
	// Audience expected in access tokens issued for this client.
	// +kubebuilder:validation:MinLength=1
	Audience string `json:"audience"`
	// ClientSecretRef names the Secret key holding the client secret for the
	// machine-to-machine client.
	ClientSecretRef SecretKeyRef `json:"clientSecretRef"`
}
```

Status + accessors as in Task 2 (reasons `Healthy`, `MissingSecret`).

- [ ] **Step 2: Regenerate** — `make manifests generate && go build ./...`

- [ ] **Step 3: Schema tests** — `validManagementAuthConfig()` (minimal doc example). Entries: accepts minimal doc example; accepts with issuerBackendUrl; rejects non-URL baseUrl (`"baseUrl"`); rejects `ftp://` tokenUrl (`"tokenUrl"`); rejects non-URL jwksUrl (`"jwksUrl"`); rejects empty clientId (`"clientId"`); rejects clientSecretRef with empty key (`"key"`).

- [ ] **Step 4: Run** — `make test` → PASS.

- [ ] **Step 5: Commit** — `git commit -m "feat(api): ManagementAuthConfig types and schema validation (#18)"`

### Task 7: pkg/conditions

**Files:**
- Create: `pkg/conditions/conditions.go`, `pkg/conditions/conditions_test.go`

**Interfaces:**
- Produces: consts `TypeReady`, `ReasonHealthy`, `ReasonMissingSecret`, `ReasonInvalidReference`, `FieldOwner`; `type Object interface { client.Object; GetConditions() []metav1.Condition; GetObservedGeneration() int64 }`; `Ready(status metav1.ConditionStatus, reason, message string, observedGeneration int64) metav1.Condition`; `PatchReady(ctx context.Context, c client.Client, obj Object, cond metav1.Condition) error`.

- [ ] **Step 1: Write failing testify tests** for `Ready` (fields land verbatim) and the unexported `needsPatch(obj Object, cond metav1.Condition) bool` decision: true when no Ready condition exists, when status/reason/message differ, or when `status.observedGeneration` lags; false when everything matches.

- [ ] **Step 2: Run** — `go test ./pkg/conditions/` → FAIL (package missing).

- [ ] **Step 3: Implement**

```go
// Package conditions provides the Ready-condition vocabulary and SSA status
// patching shared by the contract validation controllers.
package conditions

// imports: context, fmt; metav1, meta "k8s.io/apimachinery/pkg/api/meta",
// runtime "k8s.io/apimachinery/pkg/runtime", unstructured; client,
// apiutil "sigs.k8s.io/controller-runtime/pkg/client/apiutil"

const (
	// TypeReady is the condition type every contract CRD reports.
	TypeReady = "Ready"
	// ReasonHealthy indicates all validation checks passed.
	ReasonHealthy = "Healthy"
	// ReasonMissingSecret indicates a referenced Secret is missing or lacks a
	// configured key.
	ReasonMissingSecret = "MissingSecret"
	// ReasonInvalidReference indicates a referenced custom resource does not
	// exist.
	ReasonInvalidReference = "InvalidReference"
)

// FieldOwner is the server-side-apply field manager for all camunda-operator
// writes.
const FieldOwner = client.FieldOwner("camunda-operator")

// Object is implemented by contract CRs whose Ready condition the validation
// controllers maintain.
type Object interface {
	client.Object
	// GetConditions returns the resource's status conditions.
	GetConditions() []metav1.Condition
	// GetObservedGeneration returns the last reconciled generation recorded in
	// status.
	GetObservedGeneration() int64
}

// Ready builds a Ready condition observed at the given generation. The caller
// supplies LastTransitionTime handling via PatchReady.
func Ready(status metav1.ConditionStatus, reason, message string, observedGeneration int64) metav1.Condition {
	return metav1.Condition{
		Type: TypeReady, Status: status, Reason: reason,
		Message: message, ObservedGeneration: observedGeneration,
	}
}

// needsPatch reports whether obj's persisted status differs from cond.
func needsPatch(obj Object, cond metav1.Condition) bool {
	current := meta.FindStatusCondition(obj.GetConditions(), TypeReady)
	if current == nil || obj.GetObservedGeneration() != cond.ObservedGeneration {
		return true
	}
	return current.Status != cond.Status || current.Reason != cond.Reason ||
		current.Message != cond.Message || current.ObservedGeneration != cond.ObservedGeneration
}

// PatchReady server-side-applies cond and status.observedGeneration
// (cond.ObservedGeneration) to obj's status subresource under FieldOwner. It
// preserves LastTransitionTime when the condition status is unchanged and
// skips the API call entirely when the persisted status already matches.
func PatchReady(ctx context.Context, c client.Client, obj Object, cond metav1.Condition) error {
	if !needsPatch(obj, cond) {
		return nil
	}
	if current := meta.FindStatusCondition(obj.GetConditions(), TypeReady); current != nil && current.Status == cond.Status {
		cond.LastTransitionTime = current.LastTransitionTime
	}
	if cond.LastTransitionTime.IsZero() {
		cond.LastTransitionTime = metav1.Now()
	}
	gvk, err := apiutil.GVKForObject(obj, c.Scheme())
	if err != nil {
		return fmt.Errorf("resolving GVK: %w", err)
	}
	condMap, err := runtime.DefaultUnstructuredConverter.ToUnstructured(&cond)
	if err != nil {
		return fmt.Errorf("converting condition: %w", err)
	}
	u := &unstructured.Unstructured{}
	u.SetGroupVersionKind(gvk)
	u.SetName(obj.GetName())
	if err := unstructured.SetNestedField(u.Object, map[string]any{
		"observedGeneration": cond.ObservedGeneration,
		"conditions":         []any{condMap},
	}, "status"); err != nil {
		return fmt.Errorf("building status patch: %w", err)
	}
	return c.Status().Patch(ctx, u, client.Apply, FieldOwner, client.ForceOwnership)
}
```

- [ ] **Step 4: Run** — `go test ./pkg/conditions/` → PASS. (`PatchReady`'s apply path is proven by every controller's envtest specs.)

- [ ] **Step 5: Commit** — `git commit -m "feat(pkg): Ready-condition vocabulary and SSA status patching (#18)"`

### Task 8: pkg/secretref

**Files:**
- Create: `pkg/secretref/secretref.go`, `pkg/secretref/secretref_test.go`

**Interfaces:**
- Produces: `CheckKeys(ctx context.Context, reader client.Reader, ref types.NamespacedName, keys ...string) (string, error)`.

- [ ] **Step 1: Write failing testify tests** using `fake.NewClientBuilder().WithObjects(...)`: Secret absent → message `Secret "ns/name" not found`, nil error; Secret present, key absent → `Secret "ns/name" is missing key "password"`; all keys present → `("", nil)`; multiple keys with first missing → message names the first missing key.

- [ ] **Step 2: Run** — `go test ./pkg/secretref/` → FAIL.

- [ ] **Step 3: Implement**

```go
// Package secretref checks the Secret references named by contract CRDs.
package secretref

// imports: context, fmt; corev1 "k8s.io/api/core/v1",
// apierrors "k8s.io/apimachinery/pkg/api/errors", types; client

// CheckKeys reports whether the Secret at ref exists and contains every key.
// It returns a condition-ready failure message when the Secret is missing or
// lacks a key, an error only for transient API failures, and ("", nil) when
// all keys are present. Pass an uncached reader: callers watch Secrets
// metadata-only, so data must be read live.
func CheckKeys(ctx context.Context, reader client.Reader, ref types.NamespacedName, keys ...string) (string, error) {
	var secret corev1.Secret
	if err := reader.Get(ctx, ref, &secret); err != nil {
		if apierrors.IsNotFound(err) {
			return fmt.Sprintf("Secret %q not found", ref), nil
		}
		return "", err
	}
	for _, key := range keys {
		if _, ok := secret.Data[key]; !ok {
			return fmt.Sprintf("Secret %q is missing key %q", ref, key), nil
		}
	}
	return "", nil
}
```

- [ ] **Step 4: Run** — `go test ./pkg/secretref/` → PASS.

- [ ] **Step 5: Commit** — `git commit -m "feat(pkg): Secret reference key checking (#18)"`

### Task 9: pkg/refindex

**Files:**
- Create: `pkg/refindex/refindex.go`, `pkg/refindex/refindex_test.go`

**Interfaces:**
- Produces: `SecretKey(namespace, name string) string`; `ObjectNamespacedName(o client.Object) string`; `ObjectName(o client.Object) string`; `Enqueue(c client.Client, list client.ObjectList, field string, keyOf func(client.Object) string) handler.EventHandler`.

- [ ] **Step 1: Write failing testify tests**: `SecretKey("ns", "s")` == `"ns/s"` == `ObjectNamespacedName` of a Secret in ns; `Enqueue` against `fake.NewClientBuilder().WithIndex(&v1.DatabaseServerConfig{}, "databaseserverconfig.spec.secretRefs", extractor).WithObjects(two CRs referencing "ns/s", one referencing "other/x")` maps a `PartialObjectMetadata` Secret event for "ns/s" to exactly the two matching reconcile requests, and an unreferenced Secret to none.

- [ ] **Step 2: Run** — `go test ./pkg/refindex/` → FAIL.

- [ ] **Step 3: Implement**

```go
// Package refindex maps watch events on referenced objects back to the
// contract CRs naming them, via controller-runtime field indexes.
package refindex

// imports: context; meta "k8s.io/apimachinery/pkg/api/meta", types; client,
// handler, reconcile

// SecretKey returns the index key under which a CR's Secret reference is
// stored: "<namespace>/<name>".
func SecretKey(namespace, name string) string { return namespace + "/" + name }

// ObjectNamespacedName keys an event object by "<namespace>/<name>". Use it as
// keyOf for namespaced referents such as Secrets.
func ObjectNamespacedName(o client.Object) string {
	return o.GetNamespace() + "/" + o.GetName()
}

// ObjectName keys an event object by name. Use it as keyOf for cluster-scoped
// referents.
func ObjectName(o client.Object) string { return o.GetName() }

// Enqueue returns an event handler that enqueues a reconcile request for every
// object in list whose index field matches the event object's key computed by
// keyOf. List failures drop the event; the periodic informer resync recovers
// missed transitions.
func Enqueue(c client.Client, list client.ObjectList, field string, keyOf func(client.Object) string) handler.EventHandler {
	return handler.EnqueueRequestsFromMapFunc(func(ctx context.Context, o client.Object) []reconcile.Request {
		l := list.DeepCopyObject().(client.ObjectList)
		if err := c.List(ctx, l, client.MatchingFields{field: keyOf(o)}); err != nil {
			return nil
		}
		items, err := meta.ExtractList(l)
		if err != nil {
			return nil
		}
		reqs := make([]reconcile.Request, 0, len(items))
		for _, item := range items {
			om, err := meta.Accessor(item)
			if err != nil {
				continue
			}
			reqs = append(reqs, reconcile.Request{NamespacedName: types.NamespacedName{
				Name: om.GetName(), Namespace: om.GetNamespace(),
			}})
		}
		return reqs
	})
}
```

- [ ] **Step 4: Run** — `go test ./pkg/refindex/` → PASS.

- [ ] **Step 5: Commit** — `git commit -m "feat(pkg): reference field-index watch mapping (#18)"`

### Task 10: Reconciler wiring — APIReader fields, main.go, envtest manager

**Files:**
- Modify: `internal/controller/databaseserverconfig_controller.go`, `databaseconfig_controller.go`, `secondarystorageconfig_controller.go`, `objectstorageconfig_controller.go`, `managementauthconfig_controller.go` (add field only)
- Modify: `cmd/main.go` (five constructor sites)
- Modify: `internal/controller/suite_test.go`

**Interfaces:**
- Produces: reconciler structs `{ client.Client; APIReader client.Reader; Scheme *runtime.Scheme }` for the five kinds; suite consts `timeout`, `interval`; a running manager in the suite.

- [ ] **Step 1: Add the field** to each of the five reconciler structs:

```go
// APIReader reads directly from the API server, bypassing the cache; used for
// Secret data because Secrets are watched metadata-only.
APIReader client.Reader
```

- [ ] **Step 2: Wire main.go** — each of the five constructor sites gains `APIReader: mgr.GetAPIReader(),` between Client and Scheme.

- [ ] **Step 3: Extend suite_test.go** — after `k8sClient` creation in `BeforeSuite`, start a manager and register the five reconcilers; add the polling consts:

```go
const (
	timeout  = 10 * time.Second
	interval = 250 * time.Millisecond
)

// in BeforeSuite:
mgr, err := ctrl.NewManager(cfg, ctrl.Options{
	Scheme:  scheme.Scheme,
	Metrics: metricsserver.Options{BindAddress: "0"},
})
Expect(err).NotTo(HaveOccurred())
for kind, setup := range map[string]func(ctrl.Manager) error{
	"DatabaseServerConfig":   (&DatabaseServerConfigReconciler{Client: mgr.GetClient(), APIReader: mgr.GetAPIReader(), Scheme: mgr.GetScheme()}).SetupWithManager,
	"DatabaseConfig":         (&DatabaseConfigReconciler{Client: mgr.GetClient(), APIReader: mgr.GetAPIReader(), Scheme: mgr.GetScheme()}).SetupWithManager,
	"SecondaryStorageConfig": (&SecondaryStorageConfigReconciler{Client: mgr.GetClient(), APIReader: mgr.GetAPIReader(), Scheme: mgr.GetScheme()}).SetupWithManager,
	"ObjectStorageConfig":    (&ObjectStorageConfigReconciler{Client: mgr.GetClient(), APIReader: mgr.GetAPIReader(), Scheme: mgr.GetScheme()}).SetupWithManager,
	"ManagementAuthConfig":   (&ManagementAuthConfigReconciler{Client: mgr.GetClient(), APIReader: mgr.GetAPIReader(), Scheme: mgr.GetScheme()}).SetupWithManager,
} {
	Expect(setup(mgr)).To(Succeed(), "setting up %s controller", kind)
}
go func() {
	defer GinkgoRecover()
	Expect(mgr.Start(ctx)).To(Succeed())
}()
```

- [ ] **Step 4: Run** — `make all && make test` → PASS (stub reconcilers no-op under the manager).

- [ ] **Step 5: Commit and open PR 1**

```bash
git commit -am "feat(controller): reconciler wiring and envtest manager startup (#18)"
git push -u origin batch-a/foundations
gh pr create --base feature/batch-a-contract-controllers --title "Contract CRD foundations: types, schema validation, shared helpers" --body "Towards #18 ..."
```

**Review checkpoint: user reviews PR 1 before the controller fan-out.**

---

## PRs 2–6 — the five controllers (parallel after PR 1)

Each PR follows the same TDD cycle; the code below is complete per controller. Common Reconcile shape (repeated in each controller file, not shared — it is 12 lines):

```go
// Reconcile validates the contract's references and maintains its Ready
// condition; it never creates or mutates other resources.
func (r *XReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	var cfg v1.X
	if err := r.Get(ctx, req.NamespacedName, &cfg); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}
	cond, err := r.validate(ctx, &cfg)
	if err != nil {
		return ctrl.Result{}, err
	}
	return ctrl.Result{}, conditions.PatchReady(ctx, r.Client, &cfg, cond)
}
```

### Task 11 (PR 2, #19): DatabaseServerConfig controller — branch `batch-a/databaseserverconfig`

**Files:**
- Modify: `internal/controller/databaseserverconfig_controller.go`
- Create/Modify: `internal/controller/databaseserverconfig_controller_test.go`

**Interfaces:**
- Consumes: `conditions`, `secretref`, `refindex` (PR 1), `v1.DatabaseServerConfig`.

- [ ] **Step 1: Write failing envtest specs** in `Describe("DatabaseServerConfig controller")`: helper creates a valid CR (unique name, `DeferCleanup`) referencing Secret `admin-creds` in a per-spec namespace. Scenarios:
  - CR without the Secret → `Eventually` Ready is `False`/`MissingSecret`, message `Secret "<ns>/admin-creds" not found`.
  - Create the Secret with both keys → flips to `True`/`Healthy`/`All checks passed` (no CR touch — proves the Secret watch).
  - Delete the Secret → flips back to `MissingSecret`.
  - Secret exists but lacks `passwordKey` → `MissingSecret` message names the key.
  - Spec update (change `host`) → `status.observedGeneration` catches up to `metadata.generation`.

- [ ] **Step 2: Run** — `make test` → new specs FAIL (stub sets no condition).

- [ ] **Step 3: Implement**

```go
const databaseServerConfigSecretRefsField = "databaseserverconfig.spec.secretRefs"

// validate runs the contract's documented checks and returns the resulting
// Ready condition, or an error for transient API failures.
func (r *DatabaseServerConfigReconciler) validate(ctx context.Context, cfg *v1.DatabaseServerConfig) (metav1.Condition, error) {
	ref := cfg.Spec.AdminCredentialsSecretRef
	msg, err := secretref.CheckKeys(ctx, r.APIReader,
		types.NamespacedName{Namespace: ref.Namespace, Name: ref.Name},
		ref.UsernameKey, ref.PasswordKey)
	if err != nil {
		return metav1.Condition{}, err
	}
	if msg != "" {
		return conditions.Ready(metav1.ConditionFalse, conditions.ReasonMissingSecret, msg, cfg.Generation), nil
	}
	return conditions.Ready(metav1.ConditionTrue, conditions.ReasonHealthy, "All checks passed", cfg.Generation), nil
}

// SetupWithManager registers the controller, an index of CRs by referenced
// Secret, and a metadata-only Secret watch.
func (r *DatabaseServerConfigReconciler) SetupWithManager(mgr ctrl.Manager) error {
	if err := mgr.GetFieldIndexer().IndexField(context.Background(), &v1.DatabaseServerConfig{},
		databaseServerConfigSecretRefsField, func(o client.Object) []string {
			ref := o.(*v1.DatabaseServerConfig).Spec.AdminCredentialsSecretRef
			return []string{refindex.SecretKey(ref.Namespace, ref.Name)}
		}); err != nil {
		return err
	}
	return ctrl.NewControllerManagedBy(mgr).
		For(&v1.DatabaseServerConfig{}).
		Watches(&corev1.Secret{},
			refindex.Enqueue(mgr.GetClient(), &v1.DatabaseServerConfigList{},
				databaseServerConfigSecretRefsField, refindex.ObjectNamespacedName),
			builder.OnlyMetadata).
		Named("databaseserverconfig").
		Complete(r)
}
```

Add RBAC marker `+kubebuilder:rbac:groups="",resources=secrets,verbs=get;list;watch`; run `make manifests`. Reconcile as the common shape above.

- [ ] **Step 4: Run** — `make all && make test` → PASS.

- [ ] **Step 5: Commit, push, PR** — `feat(controller): DatabaseServerConfig validation controller (#19)`; PR base `feature/batch-a-contract-controllers`, body `Towards #19`.

### Task 12 (PR 3, #20): DatabaseConfig controller — branch `batch-a/databaseconfig`

**Files:**
- Modify: `internal/controller/databaseconfig_controller.go`
- Create/Modify: `internal/controller/databaseconfig_controller_test.go`

**Interfaces:**
- Consumes: PR 1 contracts; `v1.DatabaseConfig`, `v1.DatabaseServerConfig`.

- [ ] **Step 1: Write failing envtest specs**: scenarios from issue #20 — dangling `serverRef` → `InvalidReference` with message `DatabaseServerConfig "<name>" not found`; creating the referenced DatabaseServerConfig moves validation to the Secret checks (`MissingSecret`); creating both Secrets (app + backup when set) → `Healthy`; deleting the DatabaseServerConfig flips a Healthy CR to `InvalidReference` (proves the CR watch); Secret missing key → `MissingSecret` naming secret and key; `observedGeneration` follows spec updates.

- [ ] **Step 2: Run** — `make test` → FAIL.

- [ ] **Step 3: Implement**

```go
const (
	databaseConfigSecretRefsField = "databaseconfig.spec.secretRefs"
	databaseConfigServerRefField  = "databaseconfig.spec.serverRef"
)

// validate runs the contract's documented checks in order — server reference
// first, then each credentials Secret — and returns the first failure as the
// Ready condition.
func (r *DatabaseConfigReconciler) validate(ctx context.Context, cfg *v1.DatabaseConfig) (metav1.Condition, error) {
	var server v1.DatabaseServerConfig
	if err := r.Get(ctx, types.NamespacedName{Name: cfg.Spec.ServerRef}, &server); err != nil {
		if apierrors.IsNotFound(err) {
			msg := fmt.Sprintf("DatabaseServerConfig %q not found", cfg.Spec.ServerRef)
			return conditions.Ready(metav1.ConditionFalse, conditions.ReasonInvalidReference, msg, cfg.Generation), nil
		}
		return metav1.Condition{}, err
	}
	refs := []v1.CredentialsSecretRef{cfg.Spec.CredentialsSecretRef}
	if cfg.Spec.BackupCredentialsSecretRef != nil {
		refs = append(refs, *cfg.Spec.BackupCredentialsSecretRef)
	}
	for _, ref := range refs {
		msg, err := secretref.CheckKeys(ctx, r.APIReader,
			types.NamespacedName{Namespace: ref.Namespace, Name: ref.Name},
			ref.UsernameKey, ref.PasswordKey)
		if err != nil {
			return metav1.Condition{}, err
		}
		if msg != "" {
			return conditions.Ready(metav1.ConditionFalse, conditions.ReasonMissingSecret, msg, cfg.Generation), nil
		}
	}
	return conditions.Ready(metav1.ConditionTrue, conditions.ReasonHealthy, "All checks passed", cfg.Generation), nil
}
```

SetupWithManager: index `databaseConfigSecretRefsField` extracting both Secret refs (backup ref only when non-nil), index `databaseConfigServerRefField` extracting `[]string{spec.serverRef}`; watches: Secrets metadata-only via `refindex.Enqueue(..., databaseConfigSecretRefsField, refindex.ObjectNamespacedName)`, plus `Watches(&v1.DatabaseServerConfig{}, refindex.Enqueue(mgr.GetClient(), &v1.DatabaseConfigList{}, databaseConfigServerRefField, refindex.ObjectName))`. RBAC markers: secrets get;list;watch + databaseserverconfigs get;list;watch. Testify unit tests for `validate`'s ordering using a fake client (server missing beats secret missing; backup ref absent vs broken).

- [ ] **Step 4: Run** — `make all && make test` → PASS.

- [ ] **Step 5: Commit, push, PR** — `feat(controller): DatabaseConfig validation controller (#20)`; body `Towards #20`.

### Task 13 (PR 4, #21): SecondaryStorageConfig controller — branch `batch-a/secondarystorageconfig`

**Files:**
- Modify: `internal/controller/secondarystorageconfig_controller.go`
- Create/Modify: `internal/controller/secondarystorageconfig_controller_test.go`

**Interfaces:**
- Consumes: PR 1 contracts; `v1.SecondaryStorageConfig`, `v1.DatabaseConfig`.

- [ ] **Step 1: Write failing envtest specs**: ES type — missing Secret → `MissingSecret`; creating it flips to `Healthy`; deleting flips back. RDBMS type — dangling `databaseConfigRef` → `InvalidReference` message `DatabaseConfig "<name>" not found`; creating the DatabaseConfig flips to `Healthy`; deleting flips back. `observedGeneration` follows spec updates.

- [ ] **Step 2: Run** — `make test` → FAIL.

- [ ] **Step 3: Implement**

```go
const (
	secondaryStorageConfigSecretRefsField        = "secondarystorageconfig.spec.secretRefs"
	secondaryStorageConfigDatabaseConfigRefField = "secondarystorageconfig.spec.rdbms.databaseConfigRef"
)

// validate branches on the contract's storage type: elasticsearch contracts
// check their credentials Secret, rdbms contracts check the referenced
// DatabaseConfig exists. The schema guarantees exactly the matching block is
// set.
func (r *SecondaryStorageConfigReconciler) validate(ctx context.Context, cfg *v1.SecondaryStorageConfig) (metav1.Condition, error) {
	switch cfg.Spec.Type {
	case v1.SecondaryStorageTypeElasticsearch:
		ref := cfg.Spec.Elasticsearch.CredentialsSecretRef
		msg, err := secretref.CheckKeys(ctx, r.APIReader,
			types.NamespacedName{Namespace: ref.Namespace, Name: ref.Name},
			ref.UsernameKey, ref.PasswordKey)
		if err != nil {
			return metav1.Condition{}, err
		}
		if msg != "" {
			return conditions.Ready(metav1.ConditionFalse, conditions.ReasonMissingSecret, msg, cfg.Generation), nil
		}
	case v1.SecondaryStorageTypeRDBMS:
		name := cfg.Spec.RDBMS.DatabaseConfigRef
		var db v1.DatabaseConfig
		if err := r.Get(ctx, types.NamespacedName{Name: name}, &db); err != nil {
			if apierrors.IsNotFound(err) {
				msg := fmt.Sprintf("DatabaseConfig %q not found", name)
				return conditions.Ready(metav1.ConditionFalse, conditions.ReasonInvalidReference, msg, cfg.Generation), nil
			}
			return metav1.Condition{}, err
		}
	}
	return conditions.Ready(metav1.ConditionTrue, conditions.ReasonHealthy, "All checks passed", cfg.Generation), nil
}
```

SetupWithManager: index `secondaryStorageConfigSecretRefsField` (Secret ref only when `Elasticsearch != nil`), index `secondaryStorageConfigDatabaseConfigRefField` (ref only when `RDBMS != nil`); watches: Secrets metadata-only + `Watches(&v1.DatabaseConfig{}, refindex.Enqueue(..., refindex.ObjectName))`. RBAC: secrets + databaseconfigs get;list;watch. Testify unit tests for the type branch.

- [ ] **Step 4: Run** — `make all && make test` → PASS.

- [ ] **Step 5: Commit, push, PR** — `feat(controller): SecondaryStorageConfig validation controller (#21)`; body `Towards #21`.

### Task 14 (PR 5, #22): ObjectStorageConfig controller — branch `batch-a/objectstorageconfig`

**Files:**
- Modify: `internal/controller/objectstorageconfig_controller.go`
- Create/Modify: `internal/controller/objectstorageconfig_controller_test.go`

**Interfaces:**
- Consumes: `conditions` (PR 1); `v1.ObjectStorageConfig`. No secretref/refindex — the contract references nothing.

- [ ] **Step 1: Write failing envtest specs**: an admitted CR reports `Ready=True`/`Healthy`/`All checks passed` with `status.observedGeneration == metadata.generation`; a spec update (change `basePath`) re-stamps `observedGeneration`.

- [ ] **Step 2: Run** — `make test` → FAIL.

- [ ] **Step 3: Implement** — Reconcile is the common shape with `validate` inlined to a single line (all rules are admission-time):

```go
// validate always reports Healthy: every ObjectStorageConfig rule is enforced
// by the CRD schema at admission, and the contract references no other
// objects.
func (r *ObjectStorageConfigReconciler) validate(_ context.Context, cfg *v1.ObjectStorageConfig) (metav1.Condition, error) {
	return conditions.Ready(metav1.ConditionTrue, conditions.ReasonHealthy, "All checks passed", cfg.Generation), nil
}
```

SetupWithManager stays `For(&v1.ObjectStorageConfig{}).Named("objectstorageconfig")` — no extra watches, no indexes, no new RBAC.

- [ ] **Step 4: Run** — `make all && make test` → PASS.

- [ ] **Step 5: Commit, push, PR** — `feat(controller): ObjectStorageConfig validation controller (#22)`; body `Towards #22`.

### Task 15 (PR 6, #23): ManagementAuthConfig controller — branch `batch-a/managementauthconfig`

**Files:**
- Modify: `internal/controller/managementauthconfig_controller.go`
- Create/Modify: `internal/controller/managementauthconfig_controller_test.go`

**Interfaces:**
- Consumes: PR 1 contracts; `v1.ManagementAuthConfig`.

- [ ] **Step 1: Write failing envtest specs**: CR without the Secret → `MissingSecret` message `Secret "<ns>/<name>" not found`; creating the Secret with the configured key flips to `Healthy`; removing the key (update Secret) flips back with message naming the key; `observedGeneration` follows spec updates.

- [ ] **Step 2: Run** — `make test` → FAIL.

- [ ] **Step 3: Implement**

```go
const managementAuthConfigSecretRefsField = "managementauthconfig.spec.secretRefs"

// validate checks the machine-to-machine client secret reference; endpoint
// URLs are validated by the CRD schema and never probed.
func (r *ManagementAuthConfigReconciler) validate(ctx context.Context, cfg *v1.ManagementAuthConfig) (metav1.Condition, error) {
	ref := cfg.Spec.ClientSecretRef
	msg, err := secretref.CheckKeys(ctx, r.APIReader,
		types.NamespacedName{Namespace: ref.Namespace, Name: ref.Name}, ref.Key)
	if err != nil {
		return metav1.Condition{}, err
	}
	if msg != "" {
		return conditions.Ready(metav1.ConditionFalse, conditions.ReasonMissingSecret, msg, cfg.Generation), nil
	}
	return conditions.Ready(metav1.ConditionTrue, conditions.ReasonHealthy, "All checks passed", cfg.Generation), nil
}
```

SetupWithManager: index `managementAuthConfigSecretRefsField` extracting `refindex.SecretKey(ref.Namespace, ref.Name)`; metadata-only Secret watch via `refindex.Enqueue`. RBAC: secrets get;list;watch.

- [ ] **Step 4: Run** — `make all && make test` → PASS.

- [ ] **Step 5: Commit, push, PR** — `feat(controller): ManagementAuthConfig validation controller (#23)`; body `Towards #23`.

---

## Integration

### Task 16: Integration PR to main

- [ ] **Step 1:** All sub-PRs self-merged into `feature/batch-a-contract-controllers`, sub-issues #18–#23 closed (`gh issue close <n>`, bodyless).
- [ ] **Step 2:** On the feature branch: `make all && make test` green; `make manifests generate` produces no diff; the ten doc example manifests (minimal + realistic per kind) apply against envtest via the schema suites.
- [ ] **Step 3:** Cross-check each `docs/crds/<kind>.md` against the shipped behavior; fix any drift found in a final commit on the feature branch.
- [ ] **Step 4:** Open the integration PR: base `main`, title `Batch A: validation controllers for the five contract CRDs`, body `Closes #17` + summary. CI (Tests, Lint, E2E) must be green on the PR.
- [ ] **Step 5:** After merge, the orchestrator deletes the plan and state files in its final commit per `feature-dev-workflow:developing-a-feature`.
