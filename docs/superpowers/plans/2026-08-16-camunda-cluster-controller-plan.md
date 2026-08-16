# CamundaCluster controller (Batch C) implementation plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Turn a `CamundaCluster` into the workloads of a Camunda 8.9 orchestration cluster, configured through verified unified-configuration environment variables, with per-process conditions, watches on every reference, and kind e2e proof on both Batch B storage backends.

**Architecture:** Batch B shape. `internal/controller/camundacluster` does the I/O (pre-checks, watches, storage lifecycle, one `FlushStatus`). `pkg/components/camundacluster` is pure (preset merge, topology resolver, env renderer, one ocf component per process, goldens). `pkg/camundaconfig` declares every configuration key with a source pointer. `CamundaPlatformConfig` gets real types and a Batch A style validation controller.

**Tech Stack:** Go 1.26, kubebuilder v4, controller-runtime, operator-component-framework v0.18.1 (`statefulset`, `deployment`, `service`, `secret` primitives), envtest through `internal/testenv`, Ginkgo/Gomega, testify, kind e2e.

**Spec:** `docs/superpowers/specs/2026-08-16-camunda-cluster-controller-design.md` (read it first; every configuration decision in this plan comes from its "Verified configuration" table).

**Tracking:** epic #47; sub-issues #48 (platform config), #49 (cluster API types), #50 (components), #51 (controller), #52 (e2e and docs).

## Global constraints

- Camunda 8.9 only. Every key, profile, image, port, and endpoint comes from the spec's "Verified configuration" table, verified against `~/Documents/camunda/camunda` at tag `8.9.9` (worktree it if missing: `git -C ~/Documents/camunda/camunda worktree add <dir> 8.9.9`) or the Camunda Helm chart 14.8.3 for the connectors runtime. Do not add a key that is not in the table without adding its source pointer to `pkg/camundaconfig` and to the spec table in the same commit. Load the project skill `verifying-camunda-app-config` when you touch env rendering.
- Prose (GoDoc, comments, docs, condition messages) obeys `simple-english`; Go obeys `.claude/skills/how-we-write-go/SKILL.md`; `make fmt` and `make lint` must pass (golines, `hack/callsplit`).
- No new CRDs (the count stays 19). Generated files (`config/crd/bases`, `zz_generated.deepcopy.go`) are always regenerated with `make manifests generate` and committed.
- Feature branch: `feat/camunda-cluster-controller`. Sub-PR branches: `batch-c/<sub-name>`, worktrees `.claude/worktrees/camunda-cluster-controller--<sub-name>`, PRs target the feature branch.
- Docs change in the same PR as the code they describe.

---

## PR breakdown and order

| PR | Sub-name | Issue | Label | Depends on | Content |
| --- | --- | --- | --- | --- | --- |
| A | `platform-config` | #48 | task | — | `CamundaPlatformConfig` types, CEL, sample, validation controller in `internal/controller/camundaplatformconfig`, envtest |
| B | `cluster-api-types` | #49 | task | — | `CamundaCluster` and `CamundaClusterPreset` types, CEL, samples, schema envtests, condition constants, `labels.Cluster`, `MergePreset`, `ValidateMerged` |
| C | `cluster-components` | #50 | feature | A, B | `pkg/camundaconfig`, `Resolve`, renderer, ocf components, admin Secret component, config hash, goldens |
| D | `cluster-controller` | #51 | feature | C | reconciler, pre-checks, watches, storage lifecycle, `cmd/main.go`, envtest |
| E | `cluster-e2e` | #52 | task | D | kind e2e flows (ES, RDBMS), CRD docs, README/index touch-ups |

Waves: **1** = A ∥ B (independent files, no shared symbol). **2** = C. **3** = D. **4** = E. C and D are sequential on purpose: the controller's envtest value (conditions, rollout, StatefulSet recreation) needs real components, and a stubbed `Build` would give the D worker nothing to test against.

## Contracts

A and B are the only parallel pair. They share no wire shape: A owns `api/v1/camundaplatformconfig_types.go` and `internal/controller/camundaplatformconfig/`; B owns `api/v1/camundacluster_types.go`, `api/v1/camundaclusterpreset_types.go`, `pkg/labels`, `pkg/components/camundacluster/presetmerge*.go`. Both regenerate `zz_generated.deepcopy.go` and `config/crd/bases` — the second one to merge rebases and regenerates. Both add a sample to `internal/controller/samples_schema_test.go`'s allowlist (one line each; the merge is trivial).

| Name | Producer | Consumer | Shape | Realization |
| --- | --- | --- | --- | --- |
| `samples-allowlist` | A, B | — | one string per sample file appended to `implementedKindSamples` | data-only |
| `secondary-storage-chain` | Batch B (shipped) | D | `SecondaryStorageConfig.spec.rdbms.databaseConfigRef` → `DatabaseConfig` (same ns) → `spec.serverRef` → `DatabaseServerConfig` (`spec.host`, `spec.port`); credentials `DatabaseConfig.spec.credentialsSecretRef`; database `DatabaseConfig.spec.databaseName` | data-only |

The interfaces between the sequential PRs are pinned in each task's **Interfaces** block; the later PR consumes exactly those names.

## Conventions

Every sub-PR inherits these. A worker that needs a convention not listed here asks the orchestrator instead of inventing one.

- **Layout.** `internal/controller/<kind>/controller.go` (+ `suite_test.go`, `controller_test.go`, `schema_test.go`), reconciler `<Kind>Reconciler` with fields `client.Client`, `APIReader client.Reader`, `Scheme *runtime.Scheme`, `Recorder record.EventRecorder` (when it records events), unexported `componentClient client.Client`, `restMapper meta.RESTMapper` (when it runs components). Pure code in `pkg/components/camundacluster` imported as `components`. Shared envtest bootstrap through `internal/testenv`. Cross-package fixtures in `internal/fixtures`.
- **Status.** `conditions.Stage(owner, cond)` for `Ready`; `conditions.Failed` for pre-check failures; `conditions.Aggregate(owner, comps...)` once components exist; one deferred `component.FlushStatus`. `Suspended` is `Ready=True`. No `Progressing`, no separate `Suspended` condition.
- **Condition vocabulary.** Shared reasons in `api/v1/conditions.go`. Per-process condition types are `const` strings in `api/v1/camundacluster_types.go`: `ConditionZeebeReady = "ZeebeReady"`, `ConditionGatewayReady = "GatewayReady"`, `ConditionOperateReady = "OperateReady"`, `ConditionTasklistReady = "TasklistReady"`, `ConditionIdentityReady = "IdentityReady"`, `ConditionConnectorsReady = "ConnectorsReady"`. Components use `component.ConditionType(v1.ConditionZeebeReady)`.
- **Labels.** `pkg/labels` only: `labels.Cluster(name)` owner (`camunda.io/cluster`), component values `zeebe`, `gateway`, `operate`, `tasklist`, `identity`, `connectors` (constants `components.ComponentZeebe` … in `pkg/components/camundacluster/names.go`). `labels.Managed` on objects, `labels.Discovery` on pod templates, volume claim templates, and selectors, `labels.Merge(user, operator)` for `podLabels`.
- **Names** (all in `pkg/components/camundacluster/names.go`, exported functions taking the `*v1.CamundaCluster`): workloads and their Services `<name>-zeebe`, `<name>-gateway`, `<name>-operate`, `<name>-tasklist`, `<name>-identity`, `<name>-connectors` (`WorkloadName(cluster, component)`); the zeebe Service is headless; admin Secret `<name>-camunda-admin` (`AdminSecretName(cluster)`, keys `username`, `password`, username `admin`); ServiceAccount `<name>-camunda` (`ServiceAccountName(cluster)`, only when `spec.serviceAccount` is set); ServiceMonitor `<name>-<component>` (same name as the workload); annotation `camunda.io/config-hash` (`ConfigHashAnnotation`); the unified container is named `camunda`, the connectors container `connectors`; the broker volume claim template is `data`, mounted at `/usr/local/camunda/data`; the CA mount is `/etc/camunda/es-ca`.
- **Env rendering.** Only `pkg/camundaconfig` keys reach a container; `camundaconfig.Var(key, value)` and `camundaconfig.VarFrom(key, source)` build the `corev1.EnvVar`. Layer order and win-by-name as in the spec ("Configuration rendering"). `extraEnv` from users is appended last, then de-duplicated by name (last wins).
- **Events.** Typed constants in the controller package: `eventReasonPaused = "Paused"`, `eventReasonStorageShrinkIgnored = "StorageShrinkIgnored"`, `eventReasonStatefulSetRecreated = "StatefulSetRecreated"`; always `corev1.EventTypeNormal` / `corev1.EventTypeWarning`.
- **Errors.** `fmt.Errorf("<what>: %w", err)`; pre-check failures are `*conditions.PreCheckFailure`.
- **Tests.** Testify for pure code; goldens under `pkg/components/camundacluster/testdata/golden/<fixture>/<component>.yaml` with `-update-golden`; Ginkgo envtest per controller package; fixtures named after the state they encode (`minimal`, `default`, `all-in-one`, `separated`, `rdbms`, `oidc`, `preset`, `suspended`), never after a wave or PR.
- **Vocabulary.** "process" = one workload of the unified binary or the connectors runtime; "component" = the ocf component of a process; "binding" = the `SecondaryStorageConfig`; "platform config" = `CamundaPlatformConfig`; "preset" = `CamundaClusterPreset`; "admin Secret" = `<name>-camunda-admin`. Say "unified configuration", not "Spring config".
- **Verification pointers.** Every constant in `pkg/camundaconfig` carries a `// <file>:<line>` comment naming the 8.9.9 source (or `helm chart 14.8.3 <path>:<line>`).

---

## Wave 1, PR A — `batch-c/platform-config` (#48)

### Task A1: CamundaPlatformConfig types

**Files:**
- Modify: `api/v1/camundaplatformconfig_types.go` (replace the scaffold body)
- Regenerate: `api/v1/zz_generated.deepcopy.go`, `config/crd/bases/core.camunda.io_camundaplatformconfigs.yaml`
- Modify: `config/samples/core_v1_camundaplatformconfig.yaml`
- Modify: `internal/controller/samples_schema_test.go` (append `"core_v1_camundaplatformconfig.yaml"` to `implementedKindSamples`)
- Modify: `docs/crds/camundaplatformconfig.md` (drop `issuerBackendUrl` with a deviation note)

**Interfaces:**
- Produces (consumed by C and D):

```go
// api/v1/camundaplatformconfig_types.go
type AuthenticationMethod string

const (
	AuthenticationMethodBasic AuthenticationMethod = "basic"
	AuthenticationMethodOIDC  AuthenticationMethod = "oidc"
)

// OIDCSpec is the identity provider connection of a platform config.
type OIDCSpec struct {
	// +kubebuilder:validation:MinLength=1
	IssuerURL string `json:"issuerUrl"`
	// +optional
	JWKSURL string `json:"jwksUrl,omitempty"`
	// +optional
	TokenURL string `json:"tokenUrl,omitempty"`
	// +optional
	AuthURL string `json:"authUrl,omitempty"`
	// +kubebuilder:validation:MinLength=1
	ClientID string `json:"clientId"`
	// +optional
	Audience string `json:"audience,omitempty"`
	ClientSecretRef SecretKeyRef `json:"clientSecretRef"`
}

// PlatformAuthSpec selects the authentication method of every orchestration
// cluster that references the platform config.
// +kubebuilder:validation:XValidation:rule="(self.method == 'oidc') == has(self.oidc)",message="oidc is required when method is oidc and must not be set when method is basic"
type PlatformAuthSpec struct {
	// +kubebuilder:validation:Enum=basic;oidc
	// +kubebuilder:default=basic
	// +optional
	Method AuthenticationMethod `json:"method,omitempty"`
	// +optional
	OIDC *OIDCSpec `json:"oidc,omitempty"`
}

type CamundaPlatformConfigSpec struct {
	// +optional
	Auth *PlatformAuthSpec `json:"auth,omitempty"`
	// +optional
	LicenseSecretRef *SecretKeyRef `json:"licenseSecretRef,omitempty"`
	// +optional
	ImageRegistry string `json:"imageRegistry,omitempty"`
}

type CamundaPlatformConfigStatus struct {
	// +optional
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`
	// +listType=map
	// +listMapKey=type
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

// Method returns the effective authentication method: basic when auth is unset.
func (in *CamundaPlatformConfigSpec) Method() AuthenticationMethod

func (in *CamundaPlatformConfig) GetStatusConditions() *[]metav1.Condition
func (in *CamundaPlatformConfig) GetKind() string // "CamundaPlatformConfig"
func (in *CamundaPlatformConfig) SetObservedGeneration(generation int64)
```

- [ ] **Step 1: Write the types.** Replace the scaffold spec/status with the block above, GoDoc every exported symbol from the doc's field descriptions (simple-english). Keep the `+kubebuilder:resource:scope=Cluster` and `+kubebuilder:subresource:status` markers on the root type. Note in the `PlatformAuthSpec.Method` GoDoc that an unset auth block means basic. Since `Method` has a default of `basic`, the CEL rule sees `basic` when omitted.

- [ ] **Step 2: Regenerate and check.**

Run: `make manifests generate && git status --short`
Expected: the CRD yaml and deepcopy changed; `go build ./...` passes.

- [ ] **Step 3: Sample and allowlist.** Rewrite `config/samples/core_v1_camundaplatformconfig.yaml` as the doc's realistic manifest minus `issuerBackendUrl` (name `camundaplatformconfig-sample`). Append `"core_v1_camundaplatformconfig.yaml"` to `implementedKindSamples`.

- [ ] **Step 4: Doc.** In `docs/crds/camundaplatformconfig.md` remove the `issuerBackendUrl` line from the API reference and the realistic example, and add under "How it works" a `!!! note "Deviation from the original proposal"` block: Camunda 8.9 has no property for a backend issuer URL (`camunda.security.authentication.oidc.*` carries `issuer-uri`, `jwk-set-uri`, `token-uri`, `authorization-uri`, `redirect-uri` — `OidcAuthenticationConfiguration.java:33-61`), so the field is dropped; split-horizon setups use `jwksUrl` and `tokenUrl`.

- [ ] **Step 5: Run the sample schema test.**

Run: `make test 2>&1 | tail -20` (or `go test ./internal/controller/ -run TestControllers` after `make manifests`)
Expected: PASS.

- [ ] **Step 6: Commit** — `feat(api): real CamundaPlatformConfig types and schema validation`.

### Task A2: CamundaPlatformConfig validation controller

**Files:**
- Create: `internal/controller/camundaplatformconfig/controller.go`
- Create: `internal/controller/camundaplatformconfig/suite_test.go`, `controller_test.go`, `schema_test.go`
- Delete: `internal/controller/camundaplatformconfig_controller.go`, `internal/controller/camundaplatformconfig_controller_test.go`
- Modify: `cmd/main.go` (register `camundaplatformconfig.CamundaPlatformConfigReconciler{Client, APIReader, Scheme}` in place of the scaffold)

**Interfaces:**
- Consumes: `secretref.CheckKeys(ctx, reader, types.NamespacedName, keys...) (string, error)`, `conditions.Ready`, `conditions.Stage`, `component.FlushStatus`, `refindex.Enqueue`, `refindex.NamespacedKey`, `refindex.ObjectNamespacedName`.
- Produces: `type CamundaPlatformConfigReconciler struct { client.Client; APIReader client.Reader; Scheme *runtime.Scheme }`, `Reconcile`, `SetupWithManager`, and the exported index field constant `SecretRefsField = "camundaplatformconfig.spec.secretRefs"` (D's Secret watch looks platform configs up by it).

- [ ] **Step 1: Write the failing envtest.** `suite_test.go` copies `internal/controller/secondarystorageconfig/suite_test.go` with the reconciler swapped. `controller_test.go`:

```go
var _ = Describe("CamundaPlatformConfig controller", func() {
	It("reports Healthy for a basic config without secrets", func() {
		cfg := &v1.CamundaPlatformConfig{
			ObjectMeta: metav1.ObjectMeta{Name: "pfc-" + utilrand.String(8)},
			Spec: v1.CamundaPlatformConfigSpec{Auth: &v1.PlatformAuthSpec{Method: v1.AuthenticationMethodBasic}},
		}
		Expect(k8sClient.Create(ctx, cfg)).To(Succeed())
		expectReady(cfg, metav1.ConditionTrue, v1.ReasonHealthy)
	})

	It("reports MissingSecret naming the license reference, then Healthy once the Secret exists", func() {
		cfg := ... // basic auth, LicenseSecretRef {Name: "lic-"+rand, Namespace: "default", Key: "license-key"}
		Expect(k8sClient.Create(ctx, cfg)).To(Succeed())
		expectReadyMessage(cfg, metav1.ConditionFalse, v1.ReasonMissingSecret, ContainSubstring(cfg.Spec.LicenseSecretRef.Name))
		Expect(k8sClient.Create(ctx, &corev1.Secret{ObjectMeta: ..., StringData: map[string]string{"license-key": "x"}})).To(Succeed())
		expectReady(cfg, metav1.ConditionTrue, v1.ReasonHealthy)
	})

	It("reports MissingSecret when the oidc client secret key is missing", func() { ... Secret exists with a different key ... })
})
```

`expectReady` polls with `Eventually(..., timeout, interval)` reading the condition through `meta.FindStatusCondition(latest.Status.Conditions, v1.ConditionReady)`, asserting `Status`, `Reason`, and `ObservedGeneration == latest.Generation`.

`schema_test.go`: `Describe("CamundaPlatformConfig schema")` — rejects `method: oidc` without `oidc`; rejects `method: basic` with `oidc`; rejects an `oidc` block without `issuerUrl`; accepts the doc's minimal and realistic manifests.

- [ ] **Step 2: Run to see it fail.** `go test ./internal/controller/camundaplatformconfig/ 2>&1 | tail -5` — fails to compile (no package).

- [ ] **Step 3: Write the controller.** Model: `internal/controller/secondarystorageconfig/controller.go`. `validate` checks `Auth.OIDC.ClientSecretRef` when `Method() == oidc` and `LicenseSecretRef` when set, each through `secretref.CheckKeys(ctx, r.APIReader, types.NamespacedName{...}, ref.Key)`; the first non-empty message becomes `conditions.Ready(metav1.ConditionFalse, v1.ReasonMissingSecret, msg, cfg.Generation)`; otherwise `conditions.Ready(metav1.ConditionTrue, v1.ReasonHealthy, "All checks passed", cfg.Generation)`. `Reconcile` stages and flushes exactly like the model. RBAC markers: `camundaplatformconfigs` get/list/watch, `/status` get/update/patch, core `secrets` get/list/watch. `SetupWithManager` indexes both Secret refs under `SecretRefsField` with `refindex.NamespacedKey(ref.Namespace, ref.Name)` and watches Secrets metadata-only through `refindex.Enqueue(..., &v1.CamundaPlatformConfigList{}, SecretRefsField, refindex.ObjectNamespacedName)`; `Named("camundaplatformconfig")`.

- [ ] **Step 4: Wire `cmd/main.go`, delete the scaffold files, run.**

Run: `make test 2>&1 | tail -20`
Expected: PASS, including the new suite.

- [ ] **Step 5: Lint.** `make lint`.

- [ ] **Step 6: Commit** — `feat(controller): CamundaPlatformConfig validation controller`.

- [ ] **Step 7: Open PR A** against `feat/camunda-cluster-controller` per `feature-dev-workflow:opening-a-pull-request`; run the Copilot loop.

---

## Wave 1, PR B — `batch-c/cluster-api-types` (#49)

### Task B1: CamundaCluster and CamundaClusterPreset types

**Files:**
- Modify: `api/v1/camundacluster_types.go`, `api/v1/camundaclusterpreset_types.go` (replace scaffolds)
- Modify: `pkg/labels/labels.go` (add `Cluster`)
- Regenerate: deepcopy, `config/crd/bases/core.camunda.io_camundaclusters.yaml`, `core.camunda.io_camundaclusterpresets.yaml`
- Modify: `config/samples/core_v1_camundacluster.yaml`, `core_v1_camundaclusterpreset.yaml`; allowlist in `internal/controller/samples_schema_test.go`
- Modify: `docs/crds/camundacluster.md`, `docs/crds/camundaclusterpreset.md` (fields added: `connectors.version`, `zeebe.persistentVolumeClaimRetentionPolicy`, `status.storageSize`; `auth.clientSecretRef.namespace` required)

**Interfaces:**
- Produces (consumed by C, D, E):

```go
// api/v1/camundacluster_types.go
const (
	ConditionZeebeReady      = "ZeebeReady"
	ConditionGatewayReady    = "GatewayReady"
	ConditionOperateReady    = "OperateReady"
	ConditionTasklistReady   = "TasklistReady"
	ConditionIdentityReady   = "IdentityReady"
	ConditionConnectorsReady = "ConnectorsReady"
)

// +kubebuilder:validation:Enum=Standalone;Embedded
type ComponentMode string

const (
	ComponentModeStandalone ComponentMode = "Standalone"
	ComponentModeEmbedded   ComponentMode = "Embedded"
)

// WorkloadSpec is the per-process override surface shared by every component block.
type WorkloadSpec struct {
	// +kubebuilder:validation:Minimum=0
	// +optional
	Replicas *int32 `json:"replicas,omitempty"`
	// +optional
	Resources *corev1.ResourceRequirements `json:"resources,omitempty"`
	// +optional
	ExtraEnv []corev1.EnvVar `json:"extraEnv,omitempty"`
	// +optional
	ExtraEnvFrom []corev1.EnvFromSource `json:"extraEnvFrom,omitempty"`
	// +optional
	PodLabels map[string]string `json:"podLabels,omitempty"`
	// +optional
	PodAnnotations map[string]string `json:"podAnnotations,omitempty"`
	// +optional
	Scheduling *SchedulingSpec `json:"scheduling,omitempty"`
}

type ZeebeSpec struct {
	WorkloadSpec `json:",inline"`
	// +kubebuilder:validation:Minimum=1
	// +optional
	Partitions *int32 `json:"partitions,omitempty"`
	// +kubebuilder:validation:Minimum=1
	// +optional
	ReplicationFactor *int32 `json:"replicationFactor,omitempty"`
	// +optional
	StorageClassName *string `json:"storageClassName,omitempty"`
	// +optional
	StorageSize *resource.Quantity `json:"storageSize,omitempty"`
	// +optional
	PersistentVolumeClaimRetentionPolicy *PersistentVolumeClaimRetentionPolicy `json:"persistentVolumeClaimRetentionPolicy,omitempty"`
}

type GatewaySpec struct {
	// +optional
	Mode ComponentMode `json:"mode,omitempty"` // default Standalone (applied by Effective helpers, not by the schema)
	WorkloadSpec `json:",inline"`
}

type WebAppSpec struct {
	// +optional
	Mode ComponentMode `json:"mode,omitempty"` // default Embedded
	WorkloadSpec `json:",inline"`
}

type ConnectorsSpec struct {
	// +optional
	Enabled *bool `json:"enabled,omitempty"`
	// +kubebuilder:validation:Pattern=`^\d+\.\d+\.\d+$`
	// +optional
	Version string `json:"version,omitempty"`
	WorkloadSpec `json:",inline"`
}

type ClusterAuthSpec struct {
	// +optional
	ClientID string `json:"clientId,omitempty"`
	// +optional
	Audience string `json:"audience,omitempty"`
	// +optional
	ClientSecretRef *SecretKeyRef `json:"clientSecretRef,omitempty"`
}

type ClusterMonitoringSpec struct {
	// +optional
	ServiceMonitor *ServiceMonitorSpec `json:"serviceMonitor,omitempty"`
}

// +kubebuilder:validation:XValidation:rule="!has(oldSelf.zeebe) || !has(oldSelf.zeebe.partitions) || !has(self.zeebe) || !has(self.zeebe.partitions) || self.zeebe.partitions >= oldSelf.zeebe.partitions",message="zeebe.partitions may not be decreased"
// +kubebuilder:validation:XValidation:rule="!has(oldSelf.zeebe) || !has(oldSelf.zeebe.storageClassName) || (has(self.zeebe) && has(self.zeebe.storageClassName) && self.zeebe.storageClassName == oldSelf.zeebe.storageClassName)",message="zeebe.storageClassName is immutable"
// +kubebuilder:validation:XValidation:rule="!has(oldSelf.zeebe) || !has(oldSelf.zeebe.storageSize) || !has(self.zeebe) || !has(self.zeebe.storageSize) || !quantity(string(self.zeebe.storageSize)).isLessThan(quantity(string(oldSelf.zeebe.storageSize)))",message="zeebe.storageSize may not be shrunk"
// +kubebuilder:validation:XValidation:rule="!has(self.zeebe) || !has(self.zeebe.replicas) || !has(self.zeebe.replicationFactor) || self.zeebe.replicationFactor <= self.zeebe.replicas",message="zeebe.replicationFactor must not exceed zeebe.replicas"
type CamundaClusterSpec struct {
	// +kubebuilder:validation:Pattern=`^[a-z0-9]([-a-z0-9]*[a-z0-9])?$`
	// +kubebuilder:validation:MaxLength=253
	// +optional
	PlatformConfigRef string `json:"platformConfigRef,omitempty"` // required on a cluster (CEL on CamundaCluster), forbidden in a preset
	// +optional
	PresetRef string `json:"presetRef,omitempty"`
	// +kubebuilder:validation:Pattern=`^\d+\.\d+\.\d+$`
	// +optional
	Version string `json:"version,omitempty"`
	// +optional
	ExternalURL string `json:"externalUrl,omitempty"`
	// +optional
	ServiceAccount *ServiceAccountSpec `json:"serviceAccount,omitempty"`
	// +optional
	Auth *ClusterAuthSpec `json:"auth,omitempty"`
	// +optional
	Zeebe *ZeebeSpec `json:"zeebe,omitempty"`
	// +optional
	Gateway *GatewaySpec `json:"gateway,omitempty"`
	// +optional
	Operate *WebAppSpec `json:"operate,omitempty"`
	// +optional
	Tasklist *WebAppSpec `json:"tasklist,omitempty"`
	// +optional
	Identity *WebAppSpec `json:"identity,omitempty"`
	// +optional
	Connectors *ConnectorsSpec `json:"connectors,omitempty"`
	// +optional
	ExtraEnv []corev1.EnvVar `json:"extraEnv,omitempty"`
	// +optional
	ExtraEnvFrom []corev1.EnvFromSource `json:"extraEnvFrom,omitempty"`
	// +optional
	PodLabels map[string]string `json:"podLabels,omitempty"`
	// +optional
	PodAnnotations map[string]string `json:"podAnnotations,omitempty"`
	// +optional
	Scheduling *SchedulingSpec `json:"scheduling,omitempty"`
	// +optional
	StorageRef string `json:"storageRef,omitempty"` // required on a cluster (CEL), forbidden in a preset
	// +optional
	BackupStorageRef string `json:"backupStorageRef,omitempty"`
	// +optional
	DocumentStorageRef string `json:"documentStorageRef,omitempty"`
	// +optional
	Monitoring *ClusterMonitoringSpec `json:"monitoring,omitempty"`
	// +optional
	Suspend bool `json:"suspend,omitempty"`
	// +optional
	Pause bool `json:"pause,omitempty"`
}

type CamundaClusterStatus struct {
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`
	StorageSize *resource.Quantity `json:"storageSize,omitempty"`
	// +listType=map
	// +listMapKey=type
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

// On the CamundaCluster root type, the CEL that a preset does not carry:
// +kubebuilder:validation:XValidation:rule="has(self.spec.storageRef) && self.spec.storageRef != ''",message="spec.storageRef is required"
// +kubebuilder:validation:XValidation:rule="has(self.spec.platformConfigRef) && self.spec.platformConfigRef != ''",message="spec.platformConfigRef is required"

func (in *CamundaCluster) GetStatusConditions() *[]metav1.Condition
func (in *CamundaCluster) GetKind() string // "CamundaCluster"
func (in *CamundaCluster) SetObservedGeneration(generation int64)

// api/v1/camundaclusterpreset_types.go (no status, no subresource, scope=Cluster)
type CamundaClusterPresetSpec struct {
	// +kubebuilder:validation:XValidation:rule="(!has(self.platformConfigRef) || self.platformConfigRef == '') && (!has(self.presetRef) || self.presetRef == '') && (!has(self.externalUrl) || self.externalUrl == '') && !has(self.serviceAccount) && (!has(self.storageRef) || self.storageRef == '') && (!has(self.backupStorageRef) || self.backupStorageRef == '') && (!has(self.documentStorageRef) || self.documentStorageRef == '') && !has(self.monitoring) && (!has(self.suspend) || !self.suspend) && (!has(self.pause) || !self.pause)",message="instance-bound fields (platformConfigRef, presetRef, externalUrl, serviceAccount, storageRef, backupStorageRef, documentStorageRef, monitoring, suspend, pause) must not be set in a preset"
	// +required
	Cluster CamundaClusterSpec `json:"cluster"`
}

// pkg/labels
func Cluster(name string) Owner // Owner{Key: ClusterKey, Name: name}
```

Defaults that the schema does not apply (they are resolved after the preset merge so a preset can set them): `zeebe.replicas=1`, `partitions=1`, `replicationFactor=1`, `storageSize=10Gi`, `gateway.mode=Standalone`, `operate/tasklist/identity.mode=Embedded`, `connectors.enabled=false`, every other `replicas=1`. B ships them as pure helpers on the merged spec (Task B2 `Effective`).

- [ ] **Step 1: Write the types** with GoDoc from the docs (simple-english). Keep `+kubebuilder:object:root=true`, `+kubebuilder:subresource:status` on `CamundaCluster`; on `CamundaClusterPreset` keep `+kubebuilder:resource:scope=Cluster` and remove `Status` and the subresource marker (passive kind, same as `ElasticsearchClusterPreset`). Add `labels.Cluster` next to `labels.ElasticsearchCluster` with the same GoDoc pattern.

- [ ] **Step 2: Regenerate.** `make manifests generate`; `go build ./...`.

- [ ] **Step 3: Samples.** `core_v1_camundacluster.yaml` = the doc's realistic manifest (name `camundacluster-sample`, `storageRef: camundacluster-sample-storage`, `platformConfigRef: camundaplatformconfig-sample`, `presetRef: medium`, add `connectors: {enabled: true, version: "8.9.7"}`); `core_v1_camundaclusterpreset.yaml` = the doc's realistic preset (`medium`) with `connectors.version: "8.9.7"`. Append both file names to `implementedKindSamples`.

- [ ] **Step 4: Schema envtest.** `internal/controller/camundacluster/suite_test.go` (testenv, registers nothing yet: `testenv.Start(func(mgr ctrl.Manager) error { return nil })`) and `schema_test.go` with one `It` per rule: missing `storageRef`; missing `platformConfigRef`; `partitions` decrease on update; `storageClassName` change; `storageSize` shrink; `replicationFactor > replicas`; `connectors.version` not `x.y.z`; `mode: Sideways`; preset with `storageRef`, with `suspend: true`, with `monitoring`; accepts the doc's minimal cluster and both preset examples. Use `utilrand.String(8)` names in namespace `fixtures.SchemaTestNamespace`.

- [ ] **Step 5: Docs.** `camundacluster.md`: add `zeebe.persistentVolumeClaimRetentionPolicy.whenDeleted` (Delete default, Retain), `connectors.version` (required when enabled), `status.storageSize`; change `auth.clientSecretRef.namespace` to "Required" with the uniform-reference sentence used by the contract docs; add the validation bullets for `connectors.version` and the retention policy. `camundaclusterpreset.md`: add `connectors.version` to the preset-legal fields and the example. Do not touch the status table or the topology note yet (D and E own those).

- [ ] **Step 6: Run.** `make test 2>&1 | tail -20` — PASS. `make lint`.

- [ ] **Step 7: Commit** — `feat(api): real CamundaCluster and CamundaClusterPreset types with schema validation`.

### Task B2: preset merge and merged validation

**Files:**
- Create: `pkg/components/camundacluster/doc.go` (package doc: pure, spec in, resources out), `presetmerge.go`, `presetmerge_test.go`, `effective.go`, `effective_test.go`

**Interfaces:**
- Produces (consumed by C, D):

```go
package camundacluster

// MergePreset applies the preset baseline under the CamundaClusterPreset merge rules
// and returns the effective spec. A nil preset returns spec unchanged.
func MergePreset(spec v1.CamundaClusterSpec, preset *v1.CamundaClusterPresetSpec) v1.CamundaClusterSpec

// ValidateMerged reports every problem of an effective spec in one error, joined by "; ".
func ValidateMerged(spec v1.CamundaClusterSpec) error

// Effective is the merged spec with the documented defaults applied. Every reader of the
// topology uses it, so a default lives in exactly one place.
type Effective struct {
	v1.CamundaClusterSpec
}

func NewEffective(merged v1.CamundaClusterSpec) Effective
func (e Effective) ZeebeReplicas() int32          // default 1
func (e Effective) Partitions() int32             // default 1
func (e Effective) ReplicationFactor() int32      // default 1
func (e Effective) StorageSize() resource.Quantity // default 10Gi
func (e Effective) GatewayMode() v1.ComponentMode  // default Standalone
func (e Effective) OperateMode() v1.ComponentMode  // default Embedded
func (e Effective) TasklistMode() v1.ComponentMode
func (e Effective) IdentityMode() v1.ComponentMode
func (e Effective) ConnectorsEnabled() bool
func (e Effective) Replicas(component string) int32 // gateway/operate/tasklist/identity/connectors, default 1
func (e Effective) Workload(component string) v1.WorkloadSpec // the per-component block (zero value when unset)
func (e Effective) VolumeRetention() v1.PersistentVolumeClaimRetentionPolicyType // default Delete
```

- [ ] **Step 1: Write the failing tests.** `presetmerge_test.go` as a testify table, one row per rule from `camundaclusterpreset.md`:

```go
func TestMergePreset(t *testing.T) {
	t.Parallel()
	preset := &v1.CamundaClusterPresetSpec{Cluster: v1.CamundaClusterSpec{
		Version: "8.9.0",
		Auth:    &v1.ClusterAuthSpec{ClientID: "preset-client", ClientSecretRef: &v1.SecretKeyRef{Name: "p", Namespace: "camunda-system", Key: "s"}},
		ExtraEnv: []corev1.EnvVar{{Name: "TZ", Value: "UTC"}, {Name: "KEEP", Value: "preset"}},
		ExtraEnvFrom: []corev1.EnvFromSource{{ConfigMapRef: &corev1.ConfigMapEnvSource{LocalObjectReference: corev1.LocalObjectReference{Name: "preset-cm"}}}},
		PodLabels: map[string]string{"team": "preset", "shared": "preset"},
		Scheduling: &v1.SchedulingSpec{Tolerations: []corev1.Toleration{{Key: "preset"}}},
		Zeebe: &v1.ZeebeSpec{
			WorkloadSpec: v1.WorkloadSpec{Replicas: new(int32(3)), Resources: &corev1.ResourceRequirements{
				Requests: corev1.ResourceList{corev1.ResourceCPU: resource.MustParse("1"), corev1.ResourceMemory: resource.MustParse("2Gi")},
			}},
			Partitions: new(int32(3)), StorageSize: new(resource.MustParse("32Gi")),
		},
		Gateway:    &v1.GatewaySpec{Mode: v1.ComponentModeStandalone, WorkloadSpec: v1.WorkloadSpec{Replicas: new(int32(2))}},
		Connectors: &v1.ConnectorsSpec{Enabled: new(true), Version: "8.9.7"},
	}}

	tests := []struct {
		name string
		spec v1.CamundaClusterSpec
		want func(t *testing.T, got v1.CamundaClusterSpec)
	}{
		{"scalar override: version", v1.CamundaClusterSpec{Version: "8.9.1"}, func(t *testing.T, got v1.CamundaClusterSpec) { assert.Equal(t, "8.9.1", got.Version) }},
		{"scalar inherit: version", v1.CamundaClusterSpec{}, func(t *testing.T, got v1.CamundaClusterSpec) { assert.Equal(t, "8.9.0", got.Version) }},
		{"auth fields override individually", v1.CamundaClusterSpec{Auth: &v1.ClusterAuthSpec{ClientID: "mine"}}, func(t *testing.T, got v1.CamundaClusterSpec) {
			assert.Equal(t, "mine", got.Auth.ClientID); assert.Equal(t, "p", got.Auth.ClientSecretRef.Name)
		}},
		{"pointer override: zeebe.replicas", ..., replicas 5 wins, partitions 3 inherited},
		{"resources merge per entry", spec sets memory request 4Gi -> memory 4Gi, cpu 1 kept},
		{"extraEnv by name, cluster wins, preset first", spec sets KEEP=cluster and NEW=x -> order TZ, KEEP(cluster), NEW},
		{"extraEnvFrom concatenates preset first", ...},
		{"podLabels merge by key, cluster wins", ...},
		{"scheduling replaced entirely at top level", spec sets Scheduling{NodeAffinity} -> tolerations gone},
		{"scheduling at component level replaced only there", spec sets zeebe.scheduling -> top-level scheduling still preset's},
		{"connectors.enabled pointer override", spec Enabled=false -> false; Version inherited},
		{"instance-bound fields come from the cluster", spec sets StorageRef, PlatformConfigRef, Suspend -> present in merged},
		{"nil preset returns spec unchanged", ...},
	}
	...
}
```

`effective_test.go`: defaults when nothing is set; `Replicas("gateway")` reads `Gateway.Replicas`; `Workload("zeebe")` returns the zeebe block; `VolumeRetention()` default `Delete`.

`ValidateMerged` table: missing version → `"missing required fields after preset merge: version"`; version `8.8.0` → contains `"version 8.8.0 is below the supported floor 8.9.0"`; `replicationFactor 3 > replicas 1` → contains `"zeebe.replicationFactor 3 exceeds zeebe.replicas 1"`; connectors enabled without version → contains `"connectors.version is required when connectors are enabled"`; a valid spec → nil.

- [ ] **Step 2: Run to see failure.** `go test ./pkg/components/camundacluster/` — compile error.

- [ ] **Step 3: Implement.** `MergePreset`: `merged := *preset.Cluster.DeepCopy()`; then per field the rule; a helper `mergeWorkload(base, over v1.WorkloadSpec) v1.WorkloadSpec` for the shared block (replicas pointer, resources per entry via `mergeResources`, env by name via `mergeEnv`, envFrom concat, maps via `mergeMap`, scheduling wholesale) reused for every component; typed wrappers for zeebe (adds partitions, replicationFactor, storageClassName, storageSize, retention policy), gateway/webapp (adds mode), connectors (adds enabled, version); auth per field; finally the instance-bound fields copied from `spec` unconditionally. `mergeEnv(base, over []corev1.EnvVar) []corev1.EnvVar` keeps base order, replaces by name, appends new. Version floor check: parse three segments, compare against `8.9.0`. Keep every message as a package-level format string constant so tests and docs agree.

- [ ] **Step 4: Run tests.** PASS. `make lint`.

- [ ] **Step 5: Commit** — `feat(components): field-level preset merge and merged validation for CamundaCluster`.

- [ ] **Step 6: Open PR B**, Copilot loop.

---

## Wave 2, PR C — `batch-c/cluster-components` (#50)

Branch from the feature branch after A and B merged.

### Task C1: `pkg/camundaconfig` — declared keys and relaxed binding

**Files:**
- Create: `pkg/camundaconfig/doc.go`, `keys.go`, `env.go`, `env_test.go`, `keys_test.go`

**Interfaces:**
- Produces:

```go
package camundaconfig

// Key is a unified-configuration property in dotted form, for example
// "camunda.data.secondary-storage.type".
type Key string

// Env returns the environment variable name under Spring relaxed binding:
// upper case, dots to underscores, dashes removed, "[N]" to "_N_".
func (k Key) Env() string

// Index returns the key with a list index appended to the given segment, for
// example Index("camunda.security.initialization.users", 0, "username") ==
// "camunda.security.initialization.users[0].username".
func Index(list Key, i int, rest string) Key

// Var and VarFrom build the container environment entry of a key.
func Var(k Key, value string) corev1.EnvVar
func VarFrom(k Key, source *corev1.EnvVarSource) corev1.EnvVar

// Declared returns every key the operator may set. The renderer test asserts
// that every CAMUNDA_, ZEEBE_, and SPRING_ variable it emits maps to one.
func Declared() []Key
func IsDeclared(envName string) bool // matches Env() of any Declared key, index digits normalised

// Profiles and non-camunda variables (also carry source pointers).
const (
	EnvSpringProfilesActive = "SPRING_PROFILES_ACTIVE" // Profile.java, StandaloneCamunda.java:44-52
	EnvJavaToolOptions      = "JAVA_TOOL_OPTIONS"      // helm chart 14.8.3 templates/orchestration/statefulset.yaml:84-91
	EnvLicenseKeyConnectors = "CAMUNDA_LICENSE_KEY"    // helm chart 14.8.3 templates/connectors/deployment.yaml:48-51

	ProfileBroker           = "broker"            // Profile.java
	ProfileGateway          = "gateway"
	ProfileOperate          = "operate"
	ProfileTasklist         = "tasklist"
	ProfileAdmin            = "admin"             // Profile.java:24-25 (identity is the legacy name)
	ProfileConsolidatedAuth = "consolidated-auth" // WebSecurityConfig.java:148, StandaloneCamunda.java:52
)
```

Keys to declare (each with its `// <source>` comment; every one is in the spec table): `camunda.cluster.name`, `.size`, `.partition-count`, `.replication-factor`, `.node-id`, `.initial-contact-points`, `.gateway-id`; `camunda.api.grpc.address`, `.port`; `server.port`, `management.server.port`; `zeebe.broker.gateway.enable`; `camunda.data.secondary-storage.type`, `.elasticsearch.url`, `.username`, `.password`, `.security.enabled`, `.security.certificate-path`, `.security.verify-hostname`, `.security.self-signed`, `.rdbms.url`, `.rdbms.username`, `.rdbms.password`, `.rdbms.database-vendor-id`; `camunda.security.authentication.method`; `camunda.security.authentication.oidc.issuer-uri`, `.client-id`, `.client-secret`, `.audiences`, `.jwk-set-uri`, `.token-uri`, `.authorization-uri`, `.redirect-uri`; `camunda.security.initialization.users` (list; fields `username`, `password`, `name`, `email`), `camunda.security.initialization.default-roles.admin.users` (list); `camunda.license.key`; connectors: `camunda.client.mode`, `.grpc-address`, `.rest-address`, `.auth.method`, `.auth.username`, `.auth.password`, `.auth.client-id`, `.auth.client-secret`, `.auth.issuer-url`, `.auth.audience` (source `helm chart 14.8.3 templates/connectors/files/_application.yaml:43-71`; before committing, confirm the env var names of these `camunda.client.*` properties with `mcp__camunda-docs__search_camunda_knowledge_sources` — query "connectors runtime configuration camunda.client environment variables self-managed" — and record the doc URL in the comment).

- [ ] **Step 1: Failing tests.** `env_test.go`:

```go
func TestKeyEnv(t *testing.T) {
	t.Parallel()
	tests := map[Key]string{
		"camunda.data.secondary-storage.type":              "CAMUNDA_DATA_SECONDARYSTORAGE_TYPE",
		"camunda.security.authentication.oidc.jwk-set-uri": "CAMUNDA_SECURITY_AUTHENTICATION_OIDC_JWKSETURI",
		"camunda.security.initialization.users[0].username": "CAMUNDA_SECURITY_INITIALIZATION_USERS_0_USERNAME",
		"zeebe.broker.gateway.enable":                       "ZEEBE_BROKER_GATEWAY_ENABLE",
		"server.port":                                       "SERVER_PORT",
	}
	for k, want := range tests { assert.Equal(t, want, k.Env(), k) }
}
func TestIndex(t *testing.T) { assert.Equal(t, Key("camunda.security.initialization.users[1].email"), Index(KeyInitializationUsers, 1, "email")) }
func TestIsDeclared(t *testing.T) { assert.True(t, IsDeclared("CAMUNDA_SECURITY_INITIALIZATION_USERS_0_USERNAME")); assert.False(t, IsDeclared("CAMUNDA_MODE")) }
```

`keys_test.go`: every declared key has a non-empty comment source — enforce by a `sources map[Key]string` (the same table that `Declared()` reads) and assert no empty value.

- [ ] **Step 2: Implement.** `keys.go` holds `var sources = map[Key]string{ KeyClusterName: "Cluster.java, defaults.yaml:73", ... }` next to the `const` block, `Declared()` returns sorted keys, `IsDeclared` normalises `_<digits>_` to `_N_` before matching. `env.go` holds `Env`, `Index`, `Var`, `VarFrom`.

- [ ] **Step 3: Run, lint, commit** — `feat(camundaconfig): declared unified-configuration keys with source pointers`.

### Task C2: names, topology resolver

**Files:**
- Create: `pkg/components/camundacluster/names.go`, `topology.go`, `topology_test.go`

**Interfaces:**
- Produces:

```go
const (
	ComponentZeebe      = "zeebe"
	ComponentGateway    = "gateway"
	ComponentOperate    = "operate"
	ComponentTasklist   = "tasklist"
	ComponentIdentity   = "identity"
	ComponentConnectors = "connectors"

	ConfigHashAnnotation = "camunda.io/config-hash"
	AdminUsername        = "admin"
	AdminUsernameKey     = "username"
	AdminPasswordKey     = "password"
	DataVolumeName       = "data"
	DataMountPath        = "/usr/local/camunda/data"
	CAMountPath          = "/etc/camunda/es-ca"
	CamundaImage         = "camunda/camunda"
	ConnectorsImage      = "camunda/connectors-bundle"

	PortGRPC       int32 = 26500
	PortHTTP       int32 = 8080
	PortManagement int32 = 9600
	PortCommand    int32 = 26501
	PortInternal   int32 = 26502
)

func WorkloadName(cluster *v1.CamundaCluster, component string) string // cluster.Name + "-" + component
func AdminSecretName(cluster *v1.CamundaCluster) string                // cluster.Name + "-camunda-admin"
func ServiceAccountName(cluster *v1.CamundaCluster) string             // cluster.Name + "-camunda"

type ProcessKind string
const (
	ProcessStatefulSet ProcessKind = "StatefulSet"
	ProcessDeployment  ProcessKind = "Deployment"
)

// Process is one workload of the cluster and the role it plays.
type Process struct {
	Component       string      // label value and name suffix
	Kind            ProcessKind
	Replicas        int32
	Profiles        []string    // SPRING_PROFILES_ACTIVE, sorted, always ends with consolidated-auth; empty for connectors
	EmbeddedGateway bool        // zeebe only: ZEEBE_BROKER_GATEWAY_ENABLE
	ServesHTTP      bool        // exposes 8080 (gateway, web apps, zeebe with embedded gateway, connectors)
	ServesGRPC      bool        // exposes 26500 (gateway, zeebe with embedded gateway)
	ConditionType   string      // v1.Condition<X>Ready
}

// Resolve maps the effective spec to its processes in a stable order:
// zeebe, gateway, operate, tasklist, identity, connectors.
func Resolve(e Effective) []Process

// GatewayHost returns the Service name that clients (connectors) call: the gateway
// Service, or the zeebe Service when the gateway is embedded.
func GatewayHost(cluster *v1.CamundaCluster, e Effective) string
```

- [ ] **Step 1: Failing tests** — one per topology row of the spec:

```go
func TestResolveDefaultTopology(t *testing.T) { // gateway Standalone, apps Embedded
	got := Resolve(NewEffective(v1.CamundaClusterSpec{}))
	require.Len(t, got, 2)
	assert.Equal(t, []string{"broker", "consolidated-auth"}, got[0].Profiles)
	assert.False(t, got[0].EmbeddedGateway)
	assert.Equal(t, []string{"admin", "consolidated-auth", "gateway", "operate", "tasklist"}, got[1].Profiles)
	assert.True(t, got[1].ServesGRPC)
}
func TestResolveAllInOne(t *testing.T) { gateway Embedded -> one process: broker,admin,operate,tasklist,consolidated-auth; EmbeddedGateway true; ServesGRPC true }
func TestResolveStandaloneWebApp(t *testing.T) { operate Standalone -> gateway profiles drop operate; a third process operate: gateway,operate,consolidated-auth, ServesGRPC false, ServesHTTP true }
func TestResolveConnectors(t *testing.T) { enabled -> last process connectors, no profiles, Deployment, ServesHTTP }
func TestGatewayHost(t *testing.T) { default -> "<name>-gateway"; embedded -> "<name>-zeebe" }
```

- [ ] **Step 2: Implement** `Resolve` with `hostFor(app)` logic: an `Embedded` app rides on the gateway when the gateway is Standalone, else on zeebe. Profiles are built as a set, then sorted, so tests are stable.

- [ ] **Step 3: Run, lint, commit** — `feat(components): topology resolver for CamundaCluster processes`.

### Task C3: environment renderer

**Files:**
- Create: `pkg/components/camundacluster/input.go`, `render.go`, `render_test.go`, `confighash.go`, `confighash_test.go`

**Interfaces:**
- Produces:

```go
// Storage is the resolved secondary storage binding, filled by the controller.
type Storage struct {
	Type          v1.SecondaryStorageType
	Elasticsearch *v1.ElasticsearchStorage // when Type == elasticsearch
	RDBMS         *RDBMSStorage            // when Type == rdbms
}

// RDBMSStorage is the DatabaseConfig and DatabaseServerConfig chain flattened.
type RDBMSStorage struct {
	Host        string
	Port        int32
	Database    string
	Credentials v1.CredentialsSecretRef
}

// Input is everything the pure package needs to render one cluster.
type Input struct {
	Cluster   *v1.CamundaCluster
	Effective Effective
	Platform  v1.CamundaPlatformConfigSpec
	Storage   Storage
	// HashInputs are the resource versions and generations of the referenced Secrets and CRs,
	// as "kind/namespace/name=version" strings; the controller sorts them.
	HashInputs []string
	// ServiceMonitorSupported gates the ServiceMonitor resources.
	ServiceMonitorSupported bool
}

// EffectiveAuth is the auth source after the platform config → preset auth → cluster auth layering.
type EffectiveAuth struct {
	Method          v1.AuthenticationMethod
	OIDC            *v1.OIDCSpec // issuer and endpoint fields from the platform config; client id, audience, secret ref after layering
}
func ResolveAuth(in Input) EffectiveAuth

// rendered is the env, volumes and mounts of one process (unexported; tests are in-package).
type rendered struct {
	env     []corev1.EnvVar
	envFrom []corev1.EnvFromSource
	volumes []corev1.Volume
	mounts  []corev1.VolumeMount
	command []string // zeebe: the node-id wrapper; nil otherwise
}
func render(in Input, p Process) rendered

// ConfigHash hashes the rendered env of every process (names, values, secretKeyRef names and keys — never
// secret data) together with in.HashInputs. Stable across reconciles for the same input.
func ConfigHash(in Input) string

// AdminSecretReferences returns the Secrets a basic-auth cluster reads its admin credentials from.
func Image(in Input, p Process) string // registry prefix + camunda/camunda:<version> or connectors-bundle:<connectors.version>
```

- [ ] **Step 1: Failing tests** (`render_test.go`), using a `newInput(t, mutate func(*Input))` helper with the doc's minimal cluster (`my-cluster` in `my-cluster-ns`, version `8.9.9`, ES binding `https://es-http.my-cluster-ns.svc:9200` with credentials Secret `es-user` keys `username`/`password`, basic auth):

```go
func TestRenderZeebeIdentity(t *testing.T) {
	in := newInput(t, func(in *Input) { in.Effective = NewEffective(v1.CamundaClusterSpec{Zeebe: &v1.ZeebeSpec{WorkloadSpec: v1.WorkloadSpec{Replicas: new(int32(3))}, Partitions: new(int32(3)), ReplicationFactor: new(int32(3))}}) })
	r := render(in, Resolve(in.Effective)[0])
	assertEnv(t, r.env, "CAMUNDA_CLUSTER_NAME", "my-cluster")
	assertEnv(t, r.env, "CAMUNDA_CLUSTER_SIZE", "3")
	assertEnv(t, r.env, "CAMUNDA_CLUSTER_PARTITIONCOUNT", "3")
	assertEnv(t, r.env, "CAMUNDA_CLUSTER_REPLICATIONFACTOR", "3")
	assertEnv(t, r.env, "CAMUNDA_CLUSTER_INITIALCONTACTPOINTS",
		"my-cluster-zeebe-0.my-cluster-zeebe.my-cluster-ns.svc:26502,my-cluster-zeebe-1.my-cluster-zeebe.my-cluster-ns.svc:26502,my-cluster-zeebe-2.my-cluster-zeebe.my-cluster-ns.svc:26502")
	assertEnv(t, r.env, "SPRING_PROFILES_ACTIVE", "broker,consolidated-auth")
	assertEnv(t, r.env, "ZEEBE_BROKER_GATEWAY_ENABLE", "false")
	assert.Equal(t, []string{"bash", "-c", "export CAMUNDA_CLUSTER_NODEID=${HOSTNAME##*-}; exec /usr/local/camunda/bin/camunda"}, r.command)
	assertNoEnv(t, r.env, "CAMUNDA_MODE")
}
func TestRenderGatewayIdentity(t *testing.T) { gateway: CAMUNDA_CLUSTER_GATEWAYID from fieldRef metadata.name; contact points present; no node id; no command }
func TestRenderElasticsearch(t *testing.T) { CAMUNDA_DATA_SECONDARYSTORAGE_TYPE=elasticsearch; ..._ELASTICSEARCH_URL; USERNAME/PASSWORD are secretKeyRef {es-user, username/password}; with caSecretRef: SECURITY_ENABLED=true, CERTIFICATEPATH=/etc/camunda/es-ca/<key>, VERIFYHOSTNAME=true, SELFSIGNED=false, a volume "es-ca" from the Secret and a read-only mount }
func TestRenderRDBMS(t *testing.T) { TYPE=rdbms; RDBMS_URL=jdbc:postgresql://pg.ns.svc:5432/camunda; USERNAME/PASSWORD secretKeyRef from Credentials; DATABASEVENDORID=postgresql }
func TestRenderBasicAuthSeedsAdmin(t *testing.T) { METHOD=basic; INITIALIZATION_USERS_0_USERNAME=admin; _PASSWORD secretKeyRef {my-cluster-camunda-admin, password}; _NAME=admin; _EMAIL=admin@localhost; DEFAULTROLES_ADMIN_USERS_0=admin }
func TestRenderOIDC(t *testing.T) { platform oidc + externalUrl: METHOD=oidc; ISSUERURI; CLIENTID; CLIENTSECRET secretKeyRef; AUDIENCES=client id when audience empty; JWKSETURI/TOKENURI/AUTHORIZATIONURI only when set; REDIRECTURI=https://my-cluster.example.com/sso-callback; no initialization users }
func TestRenderOIDCClusterAuthWins(t *testing.T) { cluster auth clientId overrides preset and platform }
func TestRenderLicenseAndRegistry(t *testing.T) { CAMUNDA_LICENSE_KEY secretKeyRef; Image() == "registry.example.com/camunda/camunda:8.9.9" }
func TestRenderExtraEnvWinsByName(t *testing.T) { global extraEnv sets FOO=global, component sets FOO=zeebe -> one FOO=zeebe; user CAMUNDA_CLUSTER_NAME=x replaces the operator value }
func TestRenderJavaToolOptions(t *testing.T) { JAVA_TOOL_OPTIONS=-XX:+ExitOnOutOfMemoryError on zeebe and gateway; absent on connectors }
func TestRenderConnectors(t *testing.T) { CAMUNDA_CLIENT_MODE=selfManaged; GRPCADDRESS=http://my-cluster-gateway.my-cluster-ns.svc:26500; RESTADDRESS=http://my-cluster-gateway.my-cluster-ns.svc:8080; AUTH_METHOD=basic; AUTH_USERNAME=admin; AUTH_PASSWORD secretKeyRef admin secret; CAMUNDA_LICENSE_KEY when license set; oidc variant: AUTH_METHOD=oidc, CLIENTID, CLIENTSECRET, ISSUERURL, AUDIENCE }
func TestRenderOnlyDeclaredKeys(t *testing.T) { for every fixture in goldens and every process: each env whose name starts with CAMUNDA_, ZEEBE_, SPRING_ satisfies camundaconfig.IsDeclared }
func TestConfigHashStableAndSensitive(t *testing.T) { same input twice -> equal; changing a HashInputs entry, or the ES URL, or a Secret name -> different; the hash is 16 hex chars (sha256 truncated) }
```

- [ ] **Step 2: Implement `render`** in layers as the spec's "Configuration rendering" lists them (identity, storage, auth and platform, role, user overrides), each layer a small function returning `[]corev1.EnvVar` and appended in order; `dedupeEnv` keeps the last occurrence per name while preserving first-seen order. `ResolveAuth` picks `Method` from `in.Platform.Method()`; for OIDC copies the platform `OIDCSpec` and overlays preset auth (already merged into `in.Effective.Auth`) then cluster auth — since `MergePreset` already merged the preset into the effective spec, `ResolveAuth` overlays only `in.Effective.Auth`. Basic auth adds no OIDC keys. `Image` prefixes `in.Platform.ImageRegistry` (trim trailing `/`) when set.

- [ ] **Step 3: Implement `ConfigHash`**: build a deterministic string — for each process, `component=` then each env as `NAME=value` or `NAME=secretKeyRef:<name>/<key>` or `NAME=fieldRef:<path>`, then `envFrom` names, then `in.HashInputs` sorted — sha256, first 16 hex chars.

- [ ] **Step 4: Run, lint, commit** — `feat(components): layered unified-configuration renderer and config hash`.

### Task C4: ocf components, admin Secret, goldens

**Files:**
- Create: `pkg/components/camundacluster/components.go` (`Build`, per-process builders), `admin.go` (`AdminSecretComponent`), `components_test.go` (goldens), `testdata/golden/<fixture>/<component>.yaml`

**Interfaces:**
- Produces (consumed by D):

```go
// Build returns one component per process, in Resolve order. Every one of them
// takes part in Ready. The admin Secret component is returned separately by
// AdminSecretComponent because it exists only for basic auth and needs the password.
func Build(in Input) ([]*component.Component, error)

// AdminSecretComponent renders the admin credentials Secret of a basic-auth cluster.
// The controller reads or generates the password with pkg/credentials.
func AdminSecretComponent(cluster *v1.CamundaCluster, password string) (*component.Component, error)

// BrokerClaimSelector is the label selector of the broker volume claims.
func BrokerClaimSelector(cluster *v1.CamundaCluster) map[string]string // labels.Discovery(labels.Cluster(name), ComponentZeebe)

// StatefulSetName is the zeebe StatefulSet name (WorkloadName(cluster, ComponentZeebe)).
```

Component shapes (all labels through `pkg/labels`; owner `labels.Cluster(cluster.Name)`):
- **zeebe**: `statefulset.NewBuilder(zeebeStatefulSet(...))` with `serviceName = WorkloadName(zeebe)`, `podManagementPolicy: Parallel`, `updateStrategy: RollingUpdate`, `persistentVolumeClaimRetentionPolicy{WhenDeleted: e.VolumeRetention(), WhenScaled: Retain}`, one volume claim template `data` (`storageClassName`, `storageSize`, `ReadWriteOnce`), container `camunda` with the image, `command` from `render`, env, envFrom, ports (`command 26501`, `internal 26502`, `management 9600`, plus `grpc 26500` and `http 8080` when `EmbeddedGateway`), probes on 9600 (`startup` failureThreshold 60 period 5, `readiness` period 10, `liveness` period 30), `securityContext{runAsUser: 1001, runAsGroup: 1001, fsGroup: 1001, runAsNonRoot: true}`, resources, scheduling (nodeAffinity, podAffinity, tolerations), pod labels `labels.Merge(user, labels.Discovery(...))` plus the config-hash annotation, `serviceAccountName` when `spec.serviceAccount` is set; plus a headless `service.NewBuilder` (`clusterIP: None`, `publishNotReadyAddresses: true`, ports as above); plus `IncludeWhen(in.ServiceMonitorSupported && monitoring enabled, servicemonitor ...)`; `Suspend(e.Suspend)`; condition `component.ConditionType(v1.ConditionZeebeReady)`.
- **gateway / operate / tasklist / identity**: `deployment.NewBuilder` (container `camunda`, ports `http 8080`, `management 9600`, plus `grpc 26500` for the gateway), same probes/security/labels/annotation, `strategy: RollingUpdate`; Service with the same ports; ServiceMonitor gated the same way; `Suspend`; condition per component.
- **connectors**: Deployment (container `connectors`, image `Image(in, p)`, port `http 8080`, readiness `/actuator/health/readiness` and liveness `/actuator/health/liveness` on 8080, security context `runAsNonRoot: true` and `runAsUser: 1001` — check the chart's `connectors.podSecurityContext`/`containerSecurityContext` in values.yaml and copy the uid it uses; note the value and its source in the code comment), Service on 8080, `Suspend`, condition `ConnectorsReady`.
- **ServiceAccount**: when `spec.serviceAccount` is set, the zeebe component carries `serviceaccount.NewBuilder(...)` with the annotations (first component in order, so it exists before pods reference it), name `ServiceAccountName(cluster)`, and every pod template sets `serviceAccountName`.
- **admin Secret**: `secret.NewBuilder` with `StringData{username: admin, password: <password>}`, `Managed` labels, name `AdminSecretName(cluster)`, condition type `component.ConditionType("AdminSecretReady")` — declared as `v1.ConditionAdminSecretReady = "AdminSecretReady"` in the types file (D adds it to the doc's status table as an internal condition, present only under basic auth). It stays out of `Ready`? No: it takes part in `Ready` — a cluster without its admin Secret is not ready.

- [ ] **Step 1: Golden fixtures** (`components_test.go`, `-update-golden` flag, `golden.AssertComponentYAML` per component as in `pkg/components/elasticsearchcluster/components_test.go`). Fixture builders return `Input`:
  - `minimal`: doc minimal cluster; ES binding without CA; basic auth; platform config without license/registry → components zeebe (StatefulSet + headless Service), gateway (Deployment + Service), admin secret.
  - `default`: realistic doc cluster (3 brokers, resources, extraEnv, preset merged already through `MergePreset` with the doc's `medium` preset, connectors enabled 8.9.7, monitoring enabled with `ServiceMonitorSupported: true`, serviceAccount annotations, license Secret, registry `registry.example.com/camunda`).
  - `all-in-one`: gateway Embedded.
  - `separated`: operate, tasklist, identity Standalone; connectors enabled.
  - `rdbms`: RDBMS binding.
  - `oidc`: platform OIDC with jwks/token/auth URLs; cluster `auth` overriding client id; `externalUrl`.
  - `preset`: cluster that sets only refs and version, preset provides everything (proves the merge in the golden).
  - `suspended`: `suspend: true` (replicas 0 through ocf's suspend mutation — check what `Preview()` shows and pin it).
  Password for goldens: `golden-test-password`.

- [ ] **Step 2: Implement `Build`, `AdminSecretComponent`** with small per-process helper functions (`zeebeComponent(in, p)`, `deploymentComponent(in, p)`, `connectorsComponent(in, p)`, `serviceFor(in, p)`, `serviceMonitorFor(in, p)`, `podTemplate(in, p, r rendered)`), each returning builder errors wrapped as `fmt.Errorf("building %s component: %w", p.Component, err)`.

- [ ] **Step 3: Regenerate goldens** — `go test ./pkg/components/camundacluster/ -run Golden -update-golden`, then read every golden YAML once, end to end, and check it against the spec table (this is the review gate for configuration drift). Then `go test ./pkg/components/...` PASS with the flag off.

- [ ] **Step 4: Unit tests beyond goldens**: `TestBuildOrderAndConditions` (components in Resolve order, condition types match), `TestPodLabelsDoNotOverrideDiscoveryLabels`, `TestConfigHashAnnotationOnEveryPodTemplate`, `TestServiceMonitorOmittedWhenUnsupported` (Preview types).

- [ ] **Step 5: Docs** — `docs/crds/camundacluster.md`: correct the env examples in "How it works" step 3 to `SPRING_PROFILES_ACTIVE`, `ZEEBE_BROKER_GATEWAY_ENABLE`, `CAMUNDA_SECURITY_AUTHENTICATION_METHOD`, `CAMUNDA_DATA_SECONDARYSTORAGE_*`; rewrite the topology deviation note to the profiles explanation (copy the reasoning from the spec's "Why profiles and not `camunda.mode`" in three sentences); document the admin Secret `<name>-camunda-admin` under a new "Basic authentication" sub-heading; document `JAVA_TOOL_OPTIONS`.

- [ ] **Step 6: Lint, commit** — `feat(components): ocf components and goldens for every CamundaCluster process`.

- [ ] **Step 7: Open PR C**, Copilot loop.

---

## Wave 3, PR D — `batch-c/cluster-controller` (#51)

### Task D1: reconciler with pre-checks and admin credentials

**Files:**
- Create: `internal/controller/camundacluster/controller.go`, `precheck.go`, `controller_test.go` (envtest), extend `suite_test.go` from B to register the reconciler
- Delete: `internal/controller/camundacluster_controller.go`, `internal/controller/camundacluster_controller_test.go`
- Modify: `cmd/main.go` (register `camundacluster.CamundaClusterReconciler{Client, APIReader, Scheme}`)
- Modify: `internal/fixtures/fixtures.go` (add `SecondaryStorageConfigElasticsearch(namespace) *v1.SecondaryStorageConfig`, `CamundaPlatformConfigBasic() *v1.CamundaPlatformConfig`, used by D and E)

**Interfaces:**
- Consumes: everything in C's Interfaces blocks; `credentials.LookupOrNew`, `secretref.CheckKeys`, `conditions.*`, `refindex.*`.
- Produces:

```go
type CamundaClusterReconciler struct {
	client.Client
	APIReader client.Reader
	Scheme    *runtime.Scheme
	Recorder  record.EventRecorder

	componentClient client.Client
	restMapper      meta.RESTMapper
}

// preCheck resolves every reference and returns the render input, or a
// *conditions.PreCheckFailure (InvalidReference, MissingSecret).
func (r *CamundaClusterReconciler) preCheck(ctx context.Context, cluster *v1.CamundaCluster) (components.Input, error)
```

`Reconcile` order: Get; `if cluster.Spec.Pause { r.Recorder.Event(&cluster, corev1.EventTypeNormal, eventReasonPaused, "reconcile paused by spec.pause"); return ctrl.Result{}, nil }` **before** the `FlushStatus` defer is armed (pause writes nothing); ReconcileContext + deferred `FlushStatus`; `preCheck` → `conditions.Failed` on `PreCheckFailure`; storage lifecycle (`keepAppliedStorageSize`, `recreateStatefulSetOnClaimChange`, Task D2); `buildComponents` (admin Secret component first when basic auth, then `components.Build(in)`); `reconcileComponents` (continue past failures, return the first error); `conditions.Stage(&cluster, conditions.Aggregate(&cluster, comps...))`; `cluster.Status.StorageSize = sizes.smallest()`.

`preCheck` steps, each producing `InvalidReference` with the reference named or `MissingSecret` with `secretref.CheckKeys`' message:
1. preset (`APIReader.Get` cluster-scoped) → `MergePreset` → `ValidateMerged` (`InvalidReference`, message `"invalid effective spec: " + err`).
2. platform config (cluster-scoped) → `MissingSecret` for `auth.oidc.clientSecretRef` (when method oidc; the effective auth after layering decides the Secret) and `licenseSecretRef`.
3. `storageRef` binding in the cluster's namespace; ES: `CheckKeys` on credentials (`usernameKey`, `passwordKey`) and CA (`key`); RDBMS: `DatabaseConfig` (same namespace as the binding) → `DatabaseServerConfig` (cluster-scoped) → `RDBMSStorage{Host, Port, Database, Credentials}`; `CheckKeys` on the DatabaseConfig credentials.
4. `backupStorageRef`, `documentStorageRef` existence (`ObjectStorageConfig`, cluster-scoped).
5. cluster `auth.clientSecretRef` when set (`CheckKeys`).
6. `HashInputs`: `"<Kind>/<ns>/<name>=<resourceVersion>"` for every Secret read above (from the metadata the uncached read returned) and `"<Kind>/<name>=<generation>"` for preset, platform config, binding, DatabaseConfig, DatabaseServerConfig; sorted.

Admin credentials: when `ResolveAuth(in).Method == basic`, `password, err := credentials.LookupOrNew(ctx, r.APIReader, client.ObjectKey{Namespace, Name: components.AdminSecretName(&cluster)}, components.AdminPasswordKey)`; the admin Secret component goes first in the component list.

- [ ] **Step 1: Failing envtest** (`controller_test.go`), helpers `createPlatformConfig`, `createBinding` (fixtures), `createCluster`, `expectReady(cluster, status, reason)`, `expectCondition(cluster, type, reason)`, `stampStatefulSetReady(name)` / `stampDeploymentReady(name)` (set `status.replicas/readyReplicas/observedGeneration/updatedReplicas` through `Status().Update`):

```go
It("renders the default topology, mirrors workload readiness onto per-process conditions and Ready", ...)
  // creates: platform config (basic), ES binding + credentials Secret, cluster (version 8.9.9)
  // expects: StatefulSet <name>-zeebe (owner ref, labels, 1 replica, container camunda, env SPRING_PROFILES_ACTIVE=broker,consolidated-auth, annotation config-hash), Deployment <name>-gateway, Services, Secret <name>-camunda-admin with username admin and a 32-char password
  // ZeebeReady/GatewayReady not Healthy until stamped; after stamping both: Ready True Healthy
It("reports InvalidReference for a missing preset, platform config, binding, DatabaseConfig, DatabaseServerConfig, ObjectStorageConfig", ...) // table inside the It, each naming the reference in the message
It("reports InvalidReference with the field names when the merged spec is invalid", ...)
It("reports MissingSecret for a missing storage credentials Secret and recovers when it appears", ...)
It("keeps the admin password stable across reconciles and regenerates it when the Secret is deleted", ...)
It("does nothing while paused, not even status", ...) // pause: true from creation; Consistently no conditions, no workloads; unpause -> workloads appear
It("suspends every workload to zero replicas and reports Ready True Suspended, then resumes", ...)
```

- [ ] **Step 2: Implement** `controller.go` and `precheck.go`. RBAC markers: `camundaclusters{,/status,/finalizers}`, `camundaclusterpresets`, `camundaplatformconfigs`, `secondarystorageconfigs`, `databaseconfigs`, `databaseserverconfigs`, `objectstorageconfigs` (get;list;watch), core `secrets`, `services`, `serviceaccounts`, `persistentvolumeclaims` (get;list;watch;create;update;patch;delete as needed), `events` (create;patch), `apps/statefulsets`, `apps/deployments`, `monitoring.coreos.com/servicemonitors`.

- [ ] **Step 3: Run** `go test ./internal/controller/camundacluster/ 2>&1 | tail -30`; iterate to green.

- [ ] **Step 4: Commit** — `feat(controller): CamundaCluster reconciler with pre-checks and admin credentials`.

### Task D2: watches, indexes, rollout on reference change

**Files:**
- Create: `internal/controller/camundacluster/watches.go` (index constants, index functions, `SetupWithManager`)
- Extend: `controller_test.go`

Index fields: `camundacluster.spec.presetRef`, `camundacluster.spec.platformConfigRef`, `camundacluster.spec.storageRef` (namespaced key), `camundacluster.spec.objectStorageRefs` (both refs), `camundacluster.spec.secretRefs` (the cluster's own `auth.clientSecretRef`, namespaced key).

Watch decision (made here, not in the branch). A validation controller that re-checks a Secret and finds the same condition writes nothing, so a status bump cannot be relied on to fan out. The cluster therefore watches:
- `CamundaPlatformConfig`, `CamundaClusterPreset`, `SecondaryStorageConfig`, `ObjectStorageConfig` through `refindex.Enqueue` on the four index fields above.
- `Secret` (metadata only) through one `handler.EnqueueRequestsFromMapFunc` that returns: every `CamundaCluster` in the Secret's namespace (this covers the binding's credentials and CA, the `DatabaseConfig` credentials, and same-namespace auth Secrets — bindings live in the cluster's namespace by the Batch B rule), plus every cluster whose `camundacluster.spec.secretRefs` index matches (a cross-namespace auth Secret), plus every cluster whose `platformConfigRef` names a platform config that references the Secret (list `CamundaPlatformConfigList` by the A index `camundaplatformconfig.spec.secretRefs`, then `CamundaClusterList` by `camundacluster.spec.platformConfigRef`). Reads go through the cached `mgr.GetClient()`; the metadata-only Secret posture holds.
- `DatabaseConfig`: every cluster in its namespace (RDBMS bindings and their `DatabaseConfig` share the namespace). `DatabaseServerConfig`: every cluster (rare change).
- Owned: StatefulSet, Deployment, Service, ServiceAccount, Secret (metadata), and the PVC watch by the `camunda.io/cluster` label.

`HashInputs` carry the resource versions of the Secrets and the generations of the CRs the pre-checks read, so any of these events changes the config hash and rolls the pods. Say this in the `SetupWithManager` doc comment. Do not re-register index field names that other controllers already own; the platform config index name is a constant exported by A (`camundaplatformconfig.SecretRefsField`) — A exports it for this purpose.

`SetupWithManager`: `Owns` StatefulSet, Deployment, Service, ServiceAccount, Secret (`builder.OnlyMetadata`); `Watches` PVC with `enqueueForBrokerClaim()` (label `camunda.io/cluster` → request); the four `refindex.Enqueue` watches; `Named("camundacluster")`; nil-guarded `Recorder` (`mgr.GetEventRecorderFor("camundacluster") //nolint:staticcheck`) and `componentClient` (plain `client.New`, as Database does).

- [ ] **Step 1: Failing envtest**: `It("rolls the workloads when the platform config, the preset, the binding, or a referenced Secret changes")` — read the config-hash annotation, edit each reference in turn (platform config `imageRegistry`; preset `zeebe.resources`; binding endpoint; the binding's credentials Secret data — its status write must land first, so `Eventually`), assert the annotation changes each time and the StatefulSet's env reflects the edit where visible (image prefix, resources).

- [ ] **Step 2: Implement `watches.go`, run, commit** — `feat(controller): watch every CamundaCluster reference and roll on change`.

### Task D3: Zeebe storage lifecycle

**Files:**
- Create: `internal/controller/camundacluster/storage.go`, extend `controller_test.go`

**Interfaces:**

```go
// keepAppliedStorageSize clamps a preset-driven decrease to the largest bound broker claim
// (StorageShrinkIgnored event) and returns the sizes of the bound claims.
func (r *CamundaClusterReconciler) keepAppliedStorageSize(ctx context.Context, cluster *v1.CamundaCluster, in *components.Input) (claimSizes, error)

// recreateStatefulSetOnClaimChange deletes the zeebe StatefulSet with orphan propagation when its
// applied volume claim template differs from the rendered one, so ocf re-applies it and the new
// StatefulSet adopts the pods and claims. Records StatefulSetRecreated.
func (r *CamundaClusterReconciler) recreateStatefulSetOnClaimChange(ctx context.Context, cluster *v1.CamundaCluster, in components.Input) error

// growBrokerClaims patches every bound broker claim below the rendered size up to it.
func (r *CamundaClusterReconciler) growBrokerClaims(ctx context.Context, cluster *v1.CamundaCluster, size resource.Quantity) error

type claimSizes struct{ claims []resource.Quantity }
func (s claimSizes) smallest() *resource.Quantity
```

- [ ] **Step 1: Failing envtest**: create the cluster; create two bound PVCs `data-<name>-zeebe-0/1` with the broker labels and 10Gi; grow `storageSize` to 20Gi → both PVCs request 20Gi (patched), the StatefulSet is re-created (new UID, same name, orphan delete → the test asserts the UID changed and the volume claim template shows 20Gi), event `StatefulSetRecreated`; `status.storageSize` reports the smallest bound claim; a preset decrease to 5Gi is clamped with the Warning event.

- [ ] **Step 2: Implement**, run, `make lint`, commit — `feat(controller): Zeebe storage growth, StatefulSet recreation, storage status`.

- [ ] **Step 3: Docs** — `docs/crds/camundacluster.md`: rewrite the status table (drop `Progressing` and the `Suspended` condition, add `AdminSecretReady`, say `Ready` mirrors the highest-priority process condition and `Suspended` is `Ready=True`), the reconciliation steps 5 (field manager `CamundaCluster/<process>` — ocf derives it from `GetKind`), 8-9 marked "lands with Batch D", 10 (suspend scales to zero, pause writes nothing), and a "Zeebe storage" paragraph (growth, recreation, retention, `status.storageSize`).

- [ ] **Step 4: Full run** — `make test && make lint`; **Open PR D**, Copilot loop.

---

## Wave 4, PR E — `batch-c/cluster-e2e` (#52)

### Task E1: kind e2e flows

**Files:**
- Create: `test/e2e/camundacluster_test.go`, `test/e2e/testdata/process.bpmn` (a one-task process, `id="e2e-process"`, a service task with `zeebe:taskDefinition type="e2e"`)
- Extend: `test/utils/utils.go` (`CamundaREST(namespace, method, url, user, password, body string) (string, error)` through `RunPod` with `curlimages/curl` — reuse the existing `curlImage` constant by exporting it or by adding a helper next to `RunPod`)
- Modify: `.github/workflows/test-e2e.yml` only if the runner cannot host both flows (split into two jobs with a Ginkgo label filter `--ginkgo.label-filter`)

Flow (`Describe("CamundaCluster", Ordered)`, namespace `camunda-e2e`):
1. `BeforeAll`: `ElasticsearchCluster` (Batch B minimal, 1 node) → wait for its `Ready`; `CamundaPlatformConfig` basic; `CamundaCluster` `version: 8.9.9`, `connectors: {enabled: true, version: "8.9.7"}`, `storageRef` = the ES binding, small resources (`zeebe` 1 CPU / 1.5Gi request; gateway 512Mi; connectors 512Mi).
2. `Ready: Healthy` within 10 minutes (`expectReady`).
3. Read `admin` password from `<name>-camunda-admin`; `GET http://<name>-gateway.camunda-e2e.svc:8080/v2/topology` with basic auth → JSON contains `"brokers"` with one entry and `partitionsCount: 1`.
4. `GET /operate/`, `/tasklist/`, `/admin/` on the gateway return 200 (follow redirects with `-L`; assert final HTTP code).
5. `POST /v2/deployments` (multipart `resources=@process.bpmn`) → 200; `POST /v2/process-instances` `{"processDefinitionId":"e2e-process"}` → 200 with `processInstanceKey`; then `POST /v2/process-instances/search` `{"filter":{"processDefinitionId":"e2e-process"}}` eventually returns the instance (export to secondary storage works).
6. `ConnectorsReady: Healthy`.
7. `suspend: true` → workloads at 0 replicas, `Ready: True` reason `Suspended`; `suspend: false` → `Healthy` again and the process search still returns the deployed definition.
8. `AfterAll`: delete the cluster; workloads gone; broker PVC gone (default `whenDeleted: Delete`); `dumpDiagnostics` on failure.
9. Second `Describe("CamundaCluster on RDBMS", Ordered)`: `Database` (Batch B, on the suite's postgres) → its `SecondaryStorageConfig` → the same cluster shape without connectors → steps 2-5.

- [ ] **Step 1: Write the flows**, run locally: `make test-e2e` (needs Docker, kind, `vm.max_map_count>=262144`). Budget: if the ES flow plus RDBMS flow exceed `E2E_TIMEOUT` or the runner's memory, add a Ginkgo label `rdbms` and a second workflow job with `--ginkgo.label-filter=rdbms` (and `!rdbms` on the first).

- [ ] **Step 2: Iterate to green**; commit — `test(e2e): CamundaCluster on Elasticsearch and RDBMS`.

### Task E2: docs reconciliation

**Files:** `docs/crds/camundacluster.md`, `camundaclusterpreset.md`, `camundaplatformconfig.md`, `docs/crds/index.md` (implementation-order note: Batch C shipped), `README.md` (only if a status list exists — none today; leave it).

- [ ] **Step 1:** Read all three docs against the shipped types and behaviour (goldens, envtest, e2e). Fix any sentence a test contradicts. Every "Deviation" note carries the source pointer.
- [ ] **Step 2:** `make lint`; commit — `docs(crds): reconcile the CamundaCluster docs with the shipped controller`.
- [ ] **Step 3:** Open PR E, Copilot loop.

---

## Integration

After E is self-merged: `feature-dev-workflow:reviewing-feature-progress`, then the integration PR `feat/camunda-cluster-controller` → `main` with `Closes #47`, Copilot loop, then stop for the user's review. If the integration diff exceeds Copilot's 20,000-line cap (generated CRD YAML counts), note it in the PR body and rely on the sub-PR reviews.

## Self-review

- Spec coverage: types (B, A), config vocabulary (C1), topology (C2), rendering layers and JVM (C3), images and connectors (C3, C4), health/conditions/suspend/pause (C4, D1), storage lifecycle (D3), preset merge (B2), watches (D2), tests at three levels (B, C, D, E), doc deviations (A1, B1, C4 step 5, D3 step 3, E2), backup wiring out of scope (nothing renders `backupStorageRef` beyond existence — D1 step 4). Platform config controller (A2).
- Placeholders: none — every "verify with the docs MCP" item names the query and where the answer is recorded.
- Type consistency: `Effective`, `Process`, `Input`, `Storage`, `RDBMSStorage`, `EffectiveAuth`, `Build`, `AdminSecretComponent`, `ConfigHash`, `Image`, `GatewayHost`, `WorkloadName`, `AdminSecretName`, `BrokerClaimSelector` are used with the same names in C and D.
