# Management plane implementation plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking. In this feature the tasks are grouped by sub-PR and dispatched by `feature-dev-workflow:fanning-out-with-worktrees`; a worktree subagent owns one sub-PR and works its tasks in order.

**Goal:** `CamundaManagementCluster` deploys Management Identity, an identity provider, Console, and Web Modeler, writes the `ManagementAuthConfig`, and attaches to the orchestration clusters it serves.

**Architecture:** One namespaced CR rendered by a pure package (`pkg/components/camundamanagementcluster`) into ocf components, reconciled by a thin controller (`internal/controller/camundamanagementcluster`) that holds the pre-checks, the cluster attachment, the contract write, and the finalizer. Keycloak is an external-operator CR behind a `pkg/generic` wrapper, the ECK shape. OIDC facts live on `CamundaPlatformConfig`; images resolve through one `pkg/images`.

**Tech Stack:** Go 1.26, controller-runtime, ocf (`github.com/sourcehawk/operator-component-framework`), envtest through `internal/testenv`, Ginkgo/Gomega for controller tests, testify for unit tests, ocf golden snapshots, kind e2e behind the `e2e` tag.

**Spec:** `docs/superpowers/specs/2026-08-23-management-cluster-design.md`

## Global constraints

- Camunda 8.9 and later only. Version floors: `8.9.0` for Identity, Console, Web Modeler; `26.0.0` for Keycloak.
- The api module (`api/`) imports nothing from `pkg/`, `internal/`, ocf, or controller-runtime (`api/v1/module_test.go`).
- Every managed resource is applied with SSA. Status is written once per reconcile through `component.FlushStatus`.
- Every label comes from `pkg/labels`. Every condition type and reason is a constant in `api/v1`.
- Every env var name, port, health path, and image name the operator renders is verified against the docs pages listed in the spec or the 8.9 Helm chart before it is written, and declared as a constant with a comment naming its source (the `pkg/components/camundaoptimize/render.go` shape).
- Prose follows `simple-english:simple-english` pragmatic mode; user docs follow `writing-operator-docs`; Go follows `how-we-write-go`.
- Commits: `<type>(<area>): <imperative summary> (#<sub-issue>)`. Sub-PRs target `feat/management-cluster` with `Towards #<sub-issue>`.
- Gates before a PR is ready: `make setup-envtest`, `go test ./...`, `go -C api test ./...`, `make lint`, `make manifests generate` then `git status --porcelain config api` empty, `go vet -tags=e2e ./test/e2e/`, `mkdocs build --strict`.

---

## Contracts

| Name | Producer | Consumers | Shape | Realization |
| --- | --- | --- | --- | --- |
| `management-api-types` | #186 | #187, #188, #189, #190, #191, #192 | `api/v1/camundamanagementcluster_types.go` as written in Task 1.1 below (spec, status, conditions, reasons) | pre-merge stub PR (#186 merges before #187 starts) |
| `platform-oidc-management` | #186 | #187, #188, #189, #190, #192 | `OIDCSpec.ProviderType`, `OIDCSpec.Management.Clients.{Identity,Optimize,WebModeler,WebModelerAPI,Console}` as in Task 1.2 | pre-merge stub PR (#186) |
| `platform-images` | #186 | #187, #188, #189, #190 | `pkg/images`: `type Image string`, constants, `func Resolve(p *v1.CamundaPlatformConfigSpec, img Image, version string) string` | pre-merge stub PR (#186) |
| `cluster-gateway-status` | #186 | #190, #191 | `CamundaCluster.status.gateway {grpcEndpoint, restEndpoint}` | pre-merge stub PR (#186) |
| `keycloak-cr-types` | #186 | #188 | `pkg/wrappers/keycloak/types.go`: `Keycloak`, `KeycloakSpec`, `KeycloakList`, `KeycloakStatus` | pre-merge stub PR (#186) |
| `management-render-core` | #187 | #188, #189, #190 | `pkg/components/camundamanagementcluster`: `Input` (+ `Mirrors`), `AttachedCluster` (`AuthMethod v1.AuthenticationMethod`), `IdentityProvider` (+ `SpringProfile`), `ResolveIdentityProvider`, `Build(in) (Built, error)`, `Built`, `ManagementAuthSpec`, `var builders = []func(Input) ([]*component.Component, error){secretsComponents, identityComponents, keycloakComponents, consoleComponents, webModelerComponents}` in `components.go`, one stub file per later component (Task 2.1). As shipped by #200. | pre-merge stub PR (#187 merges before wave 3 starts) |
| `management-controller-core` | #187 | #188, #189, #190 | `internal/controller/camundamanagementcluster`: `Reconciler` (`New(client, apiReader, scheme)`, field `keycloakServed`), `resolved{Input components.Input; ContractName string}` from `preCheck`, `applyContract`/`withdrawContract`/`checkContractOwner` in `contract.go`, `attachedClusters`/`withdrawClaims`/`ClaimValue` in `attachment.go` (Task 2.3); reconcile order: finalizer → preCheck → attachedClusters → Build → reconcile components → applyContract → aggregate. As shipped by #200. | pre-merge stub PR (#187) |
| `ping-and-list-env` | #189 (ping), #190 (list) | #191 | env names `CAMUNDA_CONSOLE_PING_*` / `CAMUNDA_HUB_PING_*`, `CAMUNDA_MODELER_CLUSTERS_<n>_*` | data-only (this table) |
| `e2e-flow-names` | #191 | #192 | Ginkgo labels `management-keycloak`, `management-oidc` | data-only |

## Conventions

Layout
- `api/v1/camundamanagementcluster_types.go` holds the CR types, its conditions, and its reasons. Shared reasons stay in `api/v1/conditions.go`.
- `pkg/components/camundamanagementcluster/`: `doc.go`, `input.go` (Input, AttachedCluster, IdentityProvider, ResolveIdentityProvider, accessors), `names.go` (component values, Secret names, Service names, ports, container names), `components.go` (`Build`, the builder list, `Built`), one file per component: `identity.go`, `keycloak.go`, `console.go`, `webmodeler.go`, plus `contract.go` (`ManagementAuthSpec`), `ping.go` (`PingEnv`), `clusters.go` (`ClustersEnv`), `secrets.go` (generated Secrets component), `validate.go` (`ValidateSpec`, version floors), `mutations.go` (the five `WorkloadSpec` override mutations, the Optimize shape), `testdata/golden/<fixture>/`.
- `internal/controller/camundamanagementcluster/`: `controller.go` (Reconcile, SetupWithManager), `precheck.go` (reference and Secret resolution into `resolved`), `contract.go` (apply and withdraw `ManagementAuthConfig`), `attachment.go` (discovery, claim, withdraw, `status.clusters`), `ping.go` (apply and withdraw ping env), `webmodeleruser.go` (the dedicated user), `finalizer.go`, `watches.go`, `suite_test.go`, `*_test.go`.
- `pkg/wrappers/keycloak/`: `types.go`, `builder.go`, `mutator.go`, `resource.go`, `health.go`, `applyclient.go`, `builder_test.go`.
- `pkg/images/images.go` + `images_test.go`.
- Docs: `docs/crds/camundamanagementcluster.md`, `docs/guides/management-plane.md`.

Naming
- Owner label key `camunda.io/management-cluster` (`labels.ManagementCluster(name)`); component values `keycloak`, `management-identity`, `console`, `web-modeler-restapi`, `web-modeler-websockets`.
- Resource names: `<name>-keycloak` (Keycloak CR; its Service is `<name>-keycloak-service`, written by the Keycloak Operator), `<name>-identity`, `<name>-console`, `<name>-web-modeler-restapi`, `<name>-web-modeler-websockets` for Deployments and Services; all through `labels.BoundedName`.
- Generated Secrets: `<name>-identity-client`, `<name>-optimize-client`, `<name>-identity-admin`, `<name>-web-modeler-pusher`, `<name>-web-modeler-cluster-<cluster uid first 8>`; keys `client-secret`, `password`, `app-id`/`app-key`/`app-secret`.
- Field managers: `camunda-operator/camundamanagementcluster` for the CR's own resources and the contract, `camunda-operator/camundamanagementcluster-attachment` for the claim annotation and the ping entries on a `CamundaCluster`.
- Claim annotation key `camunda.io/management-cluster` with value `<namespace>/<name>`.
- Condition types: `KeycloakReady`, `IdentityReady`, `ConsoleReady`, `WebModelerReady`, `ManagementAuthReady`, `SecretsReady`. Reasons new to this kind: `KeycloakOperatorNotInstalled`, `Conflict`, `UnsupportedVersion`, `ClaimedElsewhere`, `NotReady`, `ImmutableAfterStart`, `BasicAuthUserFailed`.
- Env var constants are named after the variable in CamelCase with a prefix per app: `identityEnvKeycloakURL = "KEYCLOAK_URL"`, `consoleEnvDiscoveryMode = "CAMUNDA_CONSOLE_EXPERIMENTAL_DISCOVERY_MODE"`, `webModelerEnvServerURL = "RESTAPI_SERVER_URL"`.
- Golden fixtures: `managed-keycloak`, `external-keycloak`, `oidc`, each `minimal` and `realistic`. No organizing labels in any identifier.

Interfaces and idiom
- Render packages are pure: spec in, resources out, no API calls. Controllers hold I/O.
- Every component is built with `component.NewComponentBuilder().WithName(...).WithConditionType(...).WithResource(...)...Suspend(in.Suspended)`; Deployments through ocf `primitives/deployment`, Services through `primitives/service`, Secrets through `primitives/secret`.
- `ResolveIdentityProvider` returns one value struct for all three modes; no per-mode switches downstream (`resolve-once` rule). Renderers read `in.Provider`.
- `Ready` is `conditions.Stage(&mc, conditions.Aggregate(&mc, built.Ready...))` or `conditions.Failed(&mc, failure)`. Never `meta.SetStatusCondition` for `Ready` by hand.
- Pre-check failures are `conditions.PreCheckFailure{Reason, Message}` returned from `precheck.go`, one function per reference kind.
- SSA on another kind (the `CamundaCluster` claim and ping) carries the target UID as precondition and withdraws by applying the empty set; the Optimize `patchExporter`/`withdrawExporter` shape.
- Generated credentials go through `pkg/credentials` (`LookupOrNew`, rotate by deletion).
- Errors: `fmt.Errorf("<verb-ing> <what> %q: %w", ...)`. Events through `mgr.GetEventRecorder("camundamanagementcluster")` with typed reason and action constants.
- Tests: unit with testify (`assert`/`require`, no `t.Fatal`); controller with Ginkgo/Gomega; golden through `pkg/testing/golden`; mutation tests per ocf `testing-operators`.
- Vocabulary: "identity provider" (never IdP in docs), "management cluster", "orchestration cluster", "attached cluster", "claim", "contract" for `ManagementAuthConfig`, "initial admin".

---

## Wave 1 — #186 Management plane API types and shared contracts

Branch `feat/management-cluster--api-types`, worktree `.claude/worktrees/management-cluster--api-types`.

### Task 1.1: CamundaManagementCluster types

**Files:**
- Modify: `api/v1/camundamanagementcluster_types.go` (replace the scaffold)
- Modify: `api/v1/conditions.go` (only if a reason is shared; otherwise nothing)
- Test: `api/v1/camundamanagementcluster_types_test.go` (CEL rules through the CRD schema, the `schema_test.go` shape of other kinds), `api/v1/module_test.go` unchanged

**Produces:** the Go types below. Later tasks use these exact names.

- [ ] **Step 1: Write the schema test** in the shape of `internal/controller/elasticsearchcluster/schema_test.go` (load the CRD from `config/crd/bases`, validate sample objects): a CR with both `keycloak` and `oidc` set is rejected with "exactly one identity provider"; Web Modeler enabled without `mail.fromAddress` is rejected; `identity.externalUrl` without scheme is rejected; a valid oidc sample is accepted.
- [ ] **Step 2: Replace the types**

```go
// CamundaManagementClusterSpec describes one management plane: Management Identity,
// its identity provider, and optionally Console and Web Modeler.
type CamundaManagementClusterSpec struct {
	// PlatformConfigRef names the cluster-scoped CamundaPlatformConfig that carries the license,
	// the image settings, and in oidc mode the identity provider and every client.
	// +kubebuilder:validation:MinLength=1
	PlatformConfigRef string `json:"platformConfigRef"`
	// Suspend scales every component to zero. The contract, the claims, and the ping entries stay.
	// +optional
	Suspend bool `json:"suspend,omitempty"`
	// ClusterSelector selects the CamundaClusters, in every namespace, that Console and Web
	// Modeler serve. An empty selector selects every cluster.
	// +optional
	ClusterSelector *metav1.LabelSelector `json:"clusterSelector,omitempty"`
	// ManagementAuthConfigName is the name of the cluster-scoped ManagementAuthConfig this
	// management cluster writes. Defaults to the name of this resource.
	// +optional
	ManagementAuthConfigName string `json:"managementAuthConfigName,omitempty"`
	// IdentityProvider selects where users authenticate. Exactly one of keycloak,
	// externalKeycloak, or oidc is set.
	// +kubebuilder:validation:XValidation:rule="[has(self.keycloak), has(self.externalKeycloak), has(self.oidc)].filter(x, x).size() == 1",message="exactly one identity provider: keycloak, externalKeycloak, or oidc"
	IdentityProvider IdentityProviderSpec `json:"identityProvider"`
	// Identity configures Management Identity.
	Identity IdentitySpec `json:"identity"`
	// Console configures Console. Disabled when unset.
	// +optional
	Console *ConsoleSpec `json:"console,omitempty"`
	// WebModeler configures Web Modeler. Disabled when unset.
	// +optional
	WebModeler *WebModelerSpec `json:"webModeler,omitempty"`
}

// IdentityProviderSpec holds one of the three identity-provider modes.
type IdentityProviderSpec struct {
	// Keycloak runs Keycloak through the Keycloak Operator.
	// +optional
	Keycloak *ManagedKeycloakSpec `json:"keycloak,omitempty"`
	// ExternalKeycloak connects Management Identity to a Keycloak that you run.
	// +optional
	ExternalKeycloak *ExternalKeycloakSpec `json:"externalKeycloak,omitempty"`
	// OIDC connects Management Identity to the provider of the platform config.
	// +optional
	OIDC *ManagementOIDCSpec `json:"oidc,omitempty"`
}

// ManagedKeycloakSpec configures the Keycloak that the operator runs.
type ManagedKeycloakSpec struct {
	// Version is the Keycloak version, as a full semantic version. The image is
	// camunda/keycloak:quay-optimized-<version> unless the platform config overrides it.
	// +kubebuilder:validation:Pattern=`^\d+\.\d+\.\d+$`
	Version string `json:"version"`
	// ExternalURL is the URL users reach Keycloak at, including the /auth path.
	// +kubebuilder:validation:XValidation:rule="self.startsWith('http://') || self.startsWith('https://')",message="externalUrl must be an http or https URL"
	ExternalURL string `json:"externalUrl"`
	// DatabaseConfigRef names the DatabaseConfig of the Keycloak database, in this namespace.
	// +kubebuilder:validation:MinLength=1
	DatabaseConfigRef string `json:"databaseConfigRef"`
	// Replicas is the number of Keycloak instances. Defaults to 1.
	// +kubebuilder:validation:Minimum=0
	// +optional
	Replicas *int32 `json:"replicas,omitempty"`
	// Resources are the CPU and memory of the Keycloak container.
	// +optional
	Resources *corev1.ResourceRequirements `json:"resources,omitempty"`
}

// ExternalKeycloakSpec connects to a Keycloak that you run.
type ExternalKeycloakSpec struct {
	// URL is the URL of Keycloak, including the /auth path when it has one.
	// +kubebuilder:validation:XValidation:rule="self.startsWith('http://') || self.startsWith('https://')",message="url must be an http or https URL"
	URL string `json:"url"`
	// Realm is the realm Management Identity uses and bootstraps. Defaults to camunda-platform.
	// +optional
	Realm string `json:"realm,omitempty"`
	// AdminCredentialsSecretRef names the Secret with the Keycloak admin credentials that
	// Management Identity uses to bootstrap the realm.
	AdminCredentialsSecretRef CredentialsSecretRef `json:"adminCredentialsSecretRef"`
}

// ManagementOIDCSpec selects the provider of the platform config. It has no fields today.
type ManagementOIDCSpec struct{}

// IdentitySpec configures Management Identity.
type IdentitySpec struct {
	// +kubebuilder:validation:Pattern=`^\d+\.\d+\.\d+$`
	Version string `json:"version"`
	// +kubebuilder:validation:XValidation:rule="self.startsWith('http://') || self.startsWith('https://')",message="externalUrl must be an http or https URL"
	ExternalURL string `json:"externalUrl"`
	// +kubebuilder:validation:MinLength=1
	DatabaseConfigRef string `json:"databaseConfigRef"`
	// Admin names the initial administrator of Management Identity.
	Admin IdentityAdminSpec `json:"admin"`
	WorkloadSpec `json:",inline"`
}

// IdentityAdminSpec names the initial administrator. In oidc mode it is a claim of the
// provider's tokens, and it cannot change after the first start. In the Keycloak modes it is
// the first Keycloak user.
// +kubebuilder:validation:XValidation:rule="(has(self.claimName) && has(self.claimValue)) != has(self.username)",message="set claimName and claimValue (oidc mode) or username (keycloak modes)"
type IdentityAdminSpec struct {
	// +optional
	ClaimName string `json:"claimName,omitempty"`
	// +optional
	ClaimValue string `json:"claimValue,omitempty"`
	// +optional
	Username string `json:"username,omitempty"`
	// PasswordSecretRef names the Secret key with the password of the first Keycloak user.
	// The operator generates one when unset.
	// +optional
	PasswordSecretRef *SecretKeyRef `json:"passwordSecretRef,omitempty"`
}

// ConsoleSpec configures Console.
type ConsoleSpec struct {
	// +kubebuilder:validation:Pattern=`^\d+\.\d+\.\d+$`
	Version string `json:"version"`
	// +kubebuilder:validation:XValidation:rule="self.startsWith('http://') || self.startsWith('https://')",message="externalUrl must be an http or https URL"
	ExternalURL string `json:"externalUrl"`
	WorkloadSpec `json:",inline"`
}

// WebModelerSpec configures Web Modeler: the restapi and the websockets processes.
type WebModelerSpec struct {
	// +kubebuilder:validation:Pattern=`^\d+\.\d+\.\d+$`
	Version string `json:"version"`
	// +kubebuilder:validation:XValidation:rule="self.startsWith('http://') || self.startsWith('https://')",message="externalUrl must be an http or https URL"
	ExternalURL string `json:"externalUrl"`
	// WebsocketsExternalURL is the URL browsers reach the websockets process at.
	// +kubebuilder:validation:XValidation:rule="self.startsWith('http://') || self.startsWith('https://')",message="websocketsExternalUrl must be an http or https URL"
	WebsocketsExternalURL string `json:"websocketsExternalUrl"`
	// +kubebuilder:validation:MinLength=1
	DatabaseConfigRef string `json:"databaseConfigRef"`
	// Mail configures the SMTP server Web Modeler sends notifications through.
	Mail WebModelerMailSpec `json:"mail"`
	// +optional
	Restapi *WorkloadSpec `json:"restapi,omitempty"`
	// +optional
	Websockets *WorkloadSpec `json:"websockets,omitempty"`
}

// WebModelerMailSpec configures SMTP.
type WebModelerMailSpec struct {
	// +kubebuilder:validation:MinLength=1
	SMTPHost string `json:"smtpHost"`
	// +kubebuilder:default=587
	// +optional
	SMTPPort int32 `json:"smtpPort,omitempty"`
	// +kubebuilder:validation:MinLength=3
	FromAddress string `json:"fromAddress"`
	// +optional
	FromName string `json:"fromName,omitempty"`
	// TLS enables STARTTLS. Defaults to true.
	// +optional
	TLS *bool `json:"tls,omitempty"`
	// CredentialsSecretRef names the Secret with the SMTP user and password.
	// +optional
	CredentialsSecretRef *CredentialsSecretRef `json:"credentialsSecretRef,omitempty"`
}

// CamundaManagementClusterStatus is the observed state.
type CamundaManagementClusterStatus struct {
	// +optional
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`
	// ManagementAuthConfig is the name of the contract this management cluster writes.
	// +optional
	ManagementAuthConfig string `json:"managementAuthConfig,omitempty"`
	// Clusters lists every CamundaCluster the selector matched and whether it is attached.
	// +listType=map
	// +listMapKey=namespace
	// +listMapKey=name
	// +optional
	Clusters []AttachedClusterStatus `json:"clusters,omitempty"`
	// +listType=map
	// +listMapKey=type
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

// AttachedClusterStatus is one selected CamundaCluster.
type AttachedClusterStatus struct {
	Name      string `json:"name"`
	Namespace string `json:"namespace"`
	Attached  bool   `json:"attached"`
	// +optional
	Reason string `json:"reason,omitempty"`
	// +optional
	Message string `json:"message,omitempty"`
}

const (
	ConditionKeycloakReady       = "KeycloakReady"
	ConditionIdentityReady       = "IdentityReady"
	ConditionConsoleReady        = "ConsoleReady"
	ConditionWebModelerReady     = "WebModelerReady"
	ConditionManagementAuthReady = "ManagementAuthReady"
	ConditionSecretsReady        = "SecretsReady"

	ReasonKeycloakOperatorNotInstalled = "KeycloakOperatorNotInstalled"
	ReasonConflict                     = "Conflict"
	ReasonUnsupportedVersion           = "UnsupportedVersion"
	ReasonClaimedElsewhere             = "ClaimedElsewhere"
	ReasonNotReady                     = "NotReady"
	ReasonImmutableAfterStart          = "ImmutableAfterStart"
	ReasonBasicAuthUserFailed          = "BasicAuthUserFailed"
)
```

Each field gets a full GoDoc in the `api/v1` voice (the snippets above are abbreviated on purpose; the implementer writes the contract sentence per field, `writing-operator-docs` applies). Keep `+kubebuilder:resource:scope=Namespaced` (remove the `Cluster` marker), add printcolumns `Ready` (status and reason) and `Age`, and the ocf owner methods `GetStatusConditions`, `GetKind`, `SetObservedGeneration` the other kinds carry. `CredentialsSecretRef` and `SecretKeyRef` already exist in `common_types.go`.

- [ ] **Step 3: `make manifests generate`**, commit the CRD base and `zz_generated`.
- [ ] **Step 4: Run the schema test**, expect PASS.
- [ ] **Step 5: Update `config/samples/core_v1_camundamanagementcluster.yaml`** to a valid oidc-mode sample and make `internal/controller/samples_schema_test.go` pass.
- [ ] **Step 6: Commit** `feat(api): give CamundaManagementCluster its spec and status (#186)`.

### Task 1.2: Platform config additions

**Files:**
- Modify: `api/v1/camundaplatformconfig_types.go`
- Modify: `internal/controller/camundaplatformconfig/controller.go` (Secret checks), its tests
- Modify: `docs/crds/camundaplatformconfig.md` only in the spec reference (full docs are #192)

**Produces:**

```go
// on OIDCSpec
// ProviderType names the kind of provider. Management Identity reads it. generic (default) or microsoft.
// +kubebuilder:validation:Enum=generic;microsoft
// +optional
ProviderType string `json:"providerType,omitempty"`
// Management holds the clients of the management plane at this provider.
// +optional
Management *ManagementOIDCClientsSpec `json:"management,omitempty"`

type ManagementOIDCClientsSpec struct {
	Clients ManagementClients `json:"clients"`
}

type ManagementClients struct {
	// +optional
	Identity *ConfidentialClientSpec `json:"identity,omitempty"`
	// +optional
	Optimize *ConfidentialClientSpec `json:"optimize,omitempty"`
	// +optional
	WebModeler *PublicClientSpec `json:"webModeler,omitempty"`
	// +optional
	WebModelerAPI *WebModelerAPIClientSpec `json:"webModelerApi,omitempty"`
	// +optional
	Console *PublicClientSpec `json:"console,omitempty"`
}

type ConfidentialClientSpec struct {
	// +kubebuilder:validation:MinLength=1
	ClientID string `json:"clientId"`
	// +optional  (defaults to clientId)
	Audience string `json:"audience,omitempty"`
	ClientSecretRef SecretKeyRef `json:"clientSecretRef"`
}

type PublicClientSpec struct {
	// +kubebuilder:validation:MinLength=1
	ClientID string `json:"clientId"`
	// +optional
	Audience string `json:"audience,omitempty"`
}

type WebModelerAPIClientSpec struct {
	ConfidentialClientSpec `json:",inline"`
	// PublicAPIAudience is the audience of the public API. Defaults to web-modeler-public-api.
	// +optional
	PublicAPIAudience string `json:"publicApiAudience,omitempty"`
}

// on CamundaPlatformConfigSpec
// Images overrides the repository of one image. The tag always comes from the version of the
// resource that runs it. An override wins over imageRegistry for that image.
// +optional
Images *ImagesSpec `json:"images,omitempty"`

type ImagesSpec struct {
	// +optional
	Camunda string `json:"camunda,omitempty"`
	// +optional
	Connectors string `json:"connectors,omitempty"`
	// +optional
	Optimize string `json:"optimize,omitempty"`
	// +optional
	Identity string `json:"identity,omitempty"`
	// +optional
	Console string `json:"console,omitempty"`
	// +optional
	WebModelerRestapi string `json:"webModelerRestapi,omitempty"`
	// +optional
	WebModelerWebsockets string `json:"webModelerWebsockets,omitempty"`
	// +optional
	Keycloak string `json:"keycloak,omitempty"`
}
```

- [ ] **Step 1: Write the failing controller test**: a platform config whose `management.clients.identity.clientSecretRef` names a missing Secret reports `Ready=False/MissingSecret` with the message naming `spec.auth.oidc.management.clients.identity.clientSecretRef`; present Secret → `Healthy`.
- [ ] **Step 2: Add the types**, `make manifests generate`.
- [ ] **Step 3: Extend the validation controller** to iterate every `clientSecretRef` under `management.clients` with `secretref.CheckKeys`, and extend its Secret index the same way it indexes the orchestration client Secret today.
- [ ] **Step 4: Run the tests**, expect PASS. **Commit** `feat(api): add provider type, management clients, and image overrides to the platform config (#186)`.

### Task 1.3: `pkg/images`

**Files:**
- Create: `pkg/images/images.go`, `pkg/images/images_test.go`
- Modify: `pkg/components/camundacluster/input.go` (`Image` delegates), `pkg/components/camundaoptimize/input.go` (`Image` delegates), every other hard-coded `camunda/...` repository under `pkg/` (grep `"camunda/"`), goldens refreshed.

**Produces:**

```go
package images

// Image names one container image the operator pulls.
type Image string

const (
	Camunda              Image = "camunda"
	Connectors           Image = "connectors"
	Optimize             Image = "optimize"
	Identity             Image = "identity"
	Console              Image = "console"
	WebModelerRestapi    Image = "web-modeler-restapi"
	WebModelerWebsockets Image = "web-modeler-websockets"
	Keycloak             Image = "keycloak"
)

// Resolve returns the image reference: the platform override of img when set, else the
// platform registry in front of the default repository, else the default repository; always
// tagged with version (Keycloak: "quay-optimized-" + version).
func Resolve(p *v1.CamundaPlatformConfigSpec, img Image, version string) string
```

Default repositories: `camunda/camunda`, `camunda/connectors-bundle`, `camunda/optimize`, `camunda/identity`, `camunda/console`, `camunda/web-modeler-restapi`, `camunda/web-modeler-websockets`, `camunda/keycloak`. `p` may be nil.

- [ ] **Step 1: Table test** for each image × {no platform, registry only, override only, both}, plus the Keycloak tag prefix.
- [ ] **Step 2: Implement**, run, PASS.
- [ ] **Step 3: Delegate the two existing `Image()` functions** to `images.Resolve` without changing their signatures; run `go test ./pkg/...`; goldens unchanged.
- [ ] **Step 4: Commit** `refactor(images): resolve every image through pkg/images (#186)`.

### Task 1.4: `CamundaCluster.status.gateway`

**Files:**
- Modify: `api/v1/camundacluster_types.go` (`GatewayBinding`, `Status.Gateway`), `internal/controller/camundacluster/controller.go` where `status.management` is set, its tests, `docs/crds/camundacluster.md` status block.

**Produces:**

```go
// GatewayBinding is the published in-cluster address of the gateway.
type GatewayBinding struct {
	// GRPCEndpoint is host:port of the gRPC API, for example my-cluster-gateway.my-cluster-ns.svc:26500.
	GRPCEndpoint string `json:"grpcEndpoint"`
	// RESTEndpoint is the base URL of the REST API, for example http://my-cluster-gateway.my-cluster-ns.svc:8080.
	RESTEndpoint string `json:"restEndpoint"`
}
// Gateway is the published address of the gateway, unset while suspended.
// +optional
Gateway *GatewayBinding `json:"gateway,omitempty"`
```

- [ ] **Step 1: Failing envtest** next to the `status.management` test: the fields equal the gateway Service name and ports from `pkg/components/camundacluster/names.go`; cleared on suspend.
- [ ] **Step 2: Implement** in the same place `Management` is computed. **Step 3: PASS, commit** `feat(camundacluster): publish the gateway endpoints in status (#186)`.

### Task 1.5: Keycloak CR types, labels, RBAC

**Files:**
- Create: `pkg/wrappers/keycloak/types.go`
- Modify: `pkg/labels/labels.go` (`ManagementCluster(name string) Owner` next to the other owner keys, key `camunda.io/management-cluster`), `internal/controller/camundamanagementcluster_controller.go` (RBAC markers only; the scaffold stays no-op in this PR)

**Produces:** plain Go types with `json` tags and DeepCopy, registered in a `SchemeBuilder` for group `k8s.keycloak.org`, version `v2alpha1`:

```go
type Keycloak struct { metav1.TypeMeta; metav1.ObjectMeta; Spec KeycloakSpec; Status KeycloakStatus }
type KeycloakSpec struct {
	Instances         *int32                      `json:"instances,omitempty"`
	Image             string                      `json:"image,omitempty"`
	DB                *KeycloakDBSpec             `json:"db,omitempty"`
	HTTP              *KeycloakHTTPSpec           `json:"http,omitempty"`
	Hostname          *KeycloakHostnameSpec       `json:"hostname,omitempty"`
	Ingress           *KeycloakIngressSpec        `json:"ingress,omitempty"`
	AdditionalOptions []KeycloakValueOrSecret     `json:"additionalOptions,omitempty"`
	Unsupported       *KeycloakUnsupportedSpec    `json:"unsupported,omitempty"` // podTemplate for labels and resources
	Resources         *corev1.ResourceRequirements `json:"resources,omitempty"`
}
type KeycloakDBSpec struct { Vendor, Host, Database string; Port *int32; UsernameSecret, PasswordSecret *corev1.SecretKeySelector }
type KeycloakHTTPSpec struct { HTTPEnabled *bool `json:"httpEnabled,omitempty"`; HTTPPort *int32 `json:"httpPort,omitempty"` }
type KeycloakHostnameSpec struct { Hostname string; Strict *bool }
type KeycloakIngressSpec struct { Enabled *bool }
type KeycloakValueOrSecret struct { Name, Value string }
type KeycloakUnsupportedSpec struct { PodTemplate *corev1.PodTemplateSpec }
type KeycloakStatus struct { Conditions []KeycloakCondition }
type KeycloakCondition struct { Type string; Status string; Message string }   // status is "True"/"False"/"Unknown" as a string in v2alpha1
```

Field names and shapes are verified against the Keycloak Operator CRD at the pinned version (vendor the CRD YAML into `internal/testenv/crds/keycloak/` in this task; the wrapper in #188 uses it).

- [ ] **Step 1: Vendor the CRD** (`keycloaks.k8s.keycloak.org`) from `keycloak/keycloak-k8s-resources` at the 26.x tag the spec names; record the version in a `VERSION` file next to it.
- [ ] **Step 2: Write a round-trip test**: marshal a `Keycloak` with every field set, unmarshal into `unstructured`, validate against the vendored CRD schema with `apiextensions` validation (the `schema_test.go` helper) — no unknown fields.
- [ ] **Step 3: Implement types + DeepCopy** (`controller-gen object` runs on `pkg/`, the Makefile already does). **PASS.**
- [ ] **Step 4: Add `labels.ManagementCluster`** with a unit test, and the RBAC markers on the scaffold controller: `camundamanagementclusters` (all verbs, status, finalizers), `camundaclusters` get/list/watch/patch, `camundaclusters/status` get, `keycloaks` all verbs, `managementauthconfigs` all verbs, `databaseconfigs` get/list/watch, `camundaplatformconfigs` get/list/watch, `secrets`/`deployments`/`services`/`events` as the other controllers declare. `make manifests`.
- [ ] **Step 5: Commit** `feat(api): add the Keycloak CR types, the management owner label, and RBAC (#186)`.

### Task 1.6: Gates and PR

- [ ] Run every gate in Global constraints. Open the PR with `feature-dev-workflow:opening-a-pull-request`: `--base feat/management-cluster`, title `feat(api): add the management plane types and shared contracts`, body `Towards #186`.

---

## Wave 2 — #187 Management Identity on external OIDC, the contract, and cluster discovery

Branch `feat/management-cluster--identity-oidc-contract`. Starts after #186 is self-merged.

### Task 2.1: Render package core

**Files:**
- Create: `pkg/components/camundamanagementcluster/{doc.go,input.go,names.go,components.go,identity.go,contract.go,secrets.go,validate.go,mutations.go}` and tests, `testdata/golden/oidc/{minimal,realistic}/`

**Produces (contract `management-render-core`):**

```go
package camundamanagementcluster

// Input is everything the render needs. The controller fills it; the package never reads the API.
type Input struct {
	Cluster    *v1.CamundaManagementCluster
	Platform   *v1.CamundaPlatformConfigSpec
	Provider   IdentityProvider          // from ResolveIdentityProvider
	Databases  Databases                 // resolved DatabaseConfig + credentials Secret names per database
	Secrets    GeneratedSecrets          // names of the generated Secrets, see names.go
	Clusters   []AttachedCluster         // attached clusters, ordered by namespace/name
	Suspended  bool
	HashInputs []string                  // generations and resourceVersions folded into the config hash
	KeycloakCRDServed bool
}

// Mode of the identity provider.
type ProviderMode string
const (
	ModeKeycloak         ProviderMode = "keycloak"
	ModeExternalKeycloak ProviderMode = "externalKeycloak"
	ModeOIDC             ProviderMode = "oidc"
)

// IdentityProvider is the one value struct every renderer reads. No per-mode switch downstream.
type IdentityProvider struct {
	Mode              ProviderMode
	Type              string // KEYCLOAK | GENERIC | MICROSOFT (CAMUNDA_IDENTITY_TYPE)
	IssuerURL         string // browser-facing issuer
	IssuerBackendURL  string // in-cluster issuer
	AuthURL, TokenURL, JwksURL string
	KeycloakURL       string // in-cluster Keycloak URL incl. /auth; empty in oidc mode
	KeycloakPublicURL string // externalUrl / url
	Realm             string
	UsernameClaim     string
	Clients           ProviderClients // per component: ID, Audience, SecretRef (name+key) — filled from platform clients (oidc) or the bootstrapped ids + generated Secrets (keycloak modes)
}

type ProviderClients struct {
	Identity, Optimize, WebModelerAPI Client
	WebModeler, Console               PublicClient
}
type Client struct { ID, Audience string; SecretRef *v1.SecretKeyRef }
type PublicClient struct { ID, Audience string }

type AttachedCluster struct {
	Name, Namespace string
	UID             types.UID
	Version         string
	ExternalURL     string
	GRPCEndpoint, RESTEndpoint string
	AuthMethod      v1.AuthMethod   // basic | oidc
	BasicUserSecret string          // name of the Secret with the web-modeler user password; empty for oidc
}

// ResolveIdentityProvider builds the value struct. This PR fills the oidc branch; #188 fills the Keycloak branches.
func ResolveIdentityProvider(in Input) (IdentityProvider, error)

// Built is what Build returns.
type Built struct {
	Components []*component.Component // every component, for FlushStatus
	Ready      []*component.Component // the ones Ready aggregates over
}
// Build renders the components. The builder list in components.go is the extension point:
//   builders = []func(Input) []*component.Component{secretsComponents, identityComponents, keycloakComponents, consoleComponents, webModelerComponents}
// #187 implements secrets and identity and declares the other three returning nil.
func Build(in Input) Built

// ManagementAuthSpec derives the contract spec. All modes read in.Provider only.
func ManagementAuthSpec(in Input) v1.ManagementAuthConfigSpec

// ValidateSpec checks version floors and mode-dependent rules; returns a PreCheckFailure.
func ValidateSpec(mc *v1.CamundaManagementCluster) *conditions.PreCheckFailure
```

`names.go` exports: `ComponentIdentity = "management-identity"`, `ComponentKeycloak = "keycloak"`, `ComponentConsole = "console"`, `ComponentWebModelerRestapi`, `ComponentWebModelerWebsockets`, `IdentityName(mc)`, `KeycloakName(mc)`, `ConsoleName(mc)`, `WebModelerRestapiName(mc)`, `WebModelerWebsocketsName(mc)`, `IdentityClientSecretName(mc)`, `OptimizeClientSecretName(mc)`, `IdentityAdminSecretName(mc)`, `PusherSecretName(mc)`, `WebModelerClusterUserSecretName(mc, uid)`, `ContractName(mc)`, ports, `ConfigHashAnnotation`, `ClaimAnnotation = "camunda.io/management-cluster"`, `FieldManager`, `AttachmentFieldManager`.

- [ ] **Step 1: Unit tests first** (`identity_test.go`, `contract_test.go`, `validate_test.go`, `input_test.go`): Identity env in oidc mode equals the table in the spec (every key present with the right value, `SPRING_PROFILES_ACTIVE=oidc`, `CAMUNDA_IDENTITY_AUDIENCE` always set); contract spec in oidc mode copies platform issuer fields and `clients.optimize`; `ValidateSpec` rejects `8.8.0`, accepts `8.9.0`; `ResolveIdentityProvider` errors when platform is basic or a required client is missing (the error carries the field path).
- [ ] **Step 2: Implement** the files. Identity component: Deployment (`images.Resolve(..., images.Identity, v)`, container `identity`, ports 8080 and 8082, readiness `/actuator/health/readiness:8082`, config-hash annotation, the five override mutations), Service 80→8080 and 82→8082. `secrets.go`: Secrets component `management-secrets` with `SecretsReady`, feature-gated on having something to generate (empty in oidc mode).
- [ ] **Step 3: Golden fixtures** `oidc/minimal` and `oidc/realistic` with `goldengen`; commit goldens.
- [ ] **Step 4: PASS, commit** `feat(management): render Management Identity on an external provider (#187)`.

### Task 2.2: Controller: reconcile, pre-checks, contract

**Files:**
- Create: `internal/controller/camundamanagementcluster/{controller.go,precheck.go,contract.go,finalizer.go,watches.go,suite_test.go,controller_test.go,precheck_test.go,contract_test.go}`
- Delete: `internal/controller/camundamanagementcluster_controller.go` and `_test.go` (move RBAC markers into the new package), update `cmd/main.go`.

**Produces (contract `management-controller-core`):** `type Reconciler struct{ client.Client; APIReader client.Reader; Scheme; EventRecorder; componentClient; keycloakServed bool }`, `New(...)`, `Reconcile`, `SetupWithManager`, `type resolved struct{ platform, databases, secrets, provider, clusters, contractName }`, `precheck(ctx, mc) (resolved, *conditions.PreCheckFailure, error)`, `applyContract(ctx, mc, spec) error`, `withdrawContract(ctx, mc) error`. The reconcile order in `controller.go`: get → finalizer → precheck → attach clusters (Task 2.4) → build → reconcile components → apply contract → aggregate → flush (deferred). Hook points are plain function calls; wave-2 PRs add theirs.

- [ ] **Step 1: Ginkgo tests** (envtest through `internal/testenv`): the acceptance list of #187 — `IdentityReady=True` after marking the Deployment available, contract exists with the expected spec, `InvalidReference` on a basic platform config and on a missing client, `MissingSecret`, `Conflict` when the contract exists with other owner labels, contract deleted with the CR, `ImmutableAfterStart` when `admin.claimValue` changes after `IdentityReady` was once True (the first value is recorded in the CR's annotation `camunda.io/identity-initial-claim`).
- [ ] **Step 2: Implement** following `internal/controller/camundaoptimize/controller.go` for the shape (deferred `FlushStatus`, `conditions.Failed`/`Aggregate`, finalizer, `refindex` watches on platform config, DatabaseConfig, Secrets, and `ManagementAuthConfig`).
- [ ] **Step 3: PASS, commit** `feat(management): reconcile the management cluster and write the contract (#187)`.

### Task 2.3: Cluster discovery and claim

**Files:**
- Create: `internal/controller/camundamanagementcluster/attachment.go`, `attachment_test.go`

**Produces:** `attachedClusters(ctx, mc) ([]components.AttachedCluster, []v1.AttachedClusterStatus, error)`: lists `CamundaCluster` in all namespaces, filters by `clusterSelector` with the LabelSelector convention (nil selector selects nothing, `{}` selects all; `metav1.LabelSelectorAsSelector(nil)` already returns `labels.Nothing()`), claims unclaimed ones (SSA of the annotation under `AttachmentFieldManager`, UID precondition), reports `ClaimedElsewhere` / `NotReady` (no `status.gateway`), withdraws the claim from clusters no longer selected (remembered through the annotation value), and on finalization withdraws every claim. Watches `CamundaCluster` with an enqueue that maps a cluster to the management clusters whose selector matches it (label-selector evaluation in the handler).

- [ ] **Step 0: Correct the `ClusterSelector` GoDoc** shipped by #186 ("An empty selector selects every cluster") to the convention above, and state that creating this kind is a platform-administrator action because it reaches clusters in other namespaces; `make manifests generate`.
- [ ] **Step 1: Ginkgo tests**: two clusters, selector matches one → annotation set, unset selector → none attached, `status.clusters` has both rows with the right `attached`; a cluster already annotated with another value → `ClaimedElsewhere` and untouched; relabel the cluster out → annotation withdrawn; delete the CR → annotations withdrawn.
- [ ] **Step 2: Implement. PASS. Commit** `feat(management): select and claim the orchestration clusters (#187)`.

### Task 2.4: Gates and PR

- [ ] Gates; PR `feat(management): deploy Management Identity and write the contract` with `Towards #187`.

---

## Wave 3 — parallel: #188 Keycloak, #189 Console + ping, #190 Web Modeler + cluster list + user

All three branch from `feat/management-cluster` after #187 is self-merged. Each adds its own file in the render package and its own file in the controller package; each adds one line to the builder list in `components.go` and one hook call in `controller.go`. The orchestrator resolves those one-line conflicts at merge.

### #188 — branch `feat/management-cluster--keycloak-modes`

#### Task 3.1: Keycloak wrapper

**Files:** `pkg/wrappers/keycloak/{builder.go,mutator.go,resource.go,health.go,applyclient.go,builder_test.go}` (scaffold with `ocf scaffold wrapper` per `ocf:custom-resource-wrappers`, then fill).

- [ ] Tests: builder renders the CR with `instances`, image, `db` (vendor `postgres`, host/port/database from the DatabaseConfig's server, `usernameSecret`/`passwordSecret` keyed on the credentials Secret), `http.httpEnabled=true`, `additionalOptions` `http-relative-path=/auth`, `proxy-headers=xforwarded`, `hostname.hostname`, `ingress.enabled=false`, pod template labels from `labels.Discovery`; health handler maps the CR's `Ready` condition; suspend handler sets `instances: 0`; apply client strips fields not in the vendored CRD.
- [ ] Implement; PASS; commit `feat(wrappers): wrap the Keycloak CR of the Keycloak Operator (#188)`.

#### Task 3.2: Keycloak modes in the render package

**Files:** `pkg/components/camundamanagementcluster/keycloak.go`, `input.go` (the two branches of `ResolveIdentityProvider`), `identity.go` (Keycloak-mode env), `secrets.go` (generated Secrets), `contract.go` (realm URLs), goldens `managed-keycloak/*`, `external-keycloak/*`.

- [ ] Unit tests: `ResolveIdentityProvider` in `keycloak` mode yields `KeycloakURL=http://<name>-keycloak-service.<ns>.svc:8080/auth`, `IssuerURL=<externalUrl>/realms/camunda-platform`, backend = in-cluster realm URL, `Clients` with bootstrapped ids and the generated Secret refs; `externalKeycloak` mode yields issuer = backend = `<url>/realms/<realm>`; Identity env table for both modes (every `KEYCLOAK_*`, `IDENTITY_*`, `KEYCLOAK_INIT_*`, `KEYCLOAK_CLIENTS_0_*` for Console); contract spec per mode.
- [ ] Implement; goldens; PASS; commit `feat(management): run Identity on a managed or an existing Keycloak (#188)`.

#### Task 3.3: Controller hooks

**Files:** `internal/controller/camundamanagementcluster/{controller.go (hook), keycloak.go, keycloak_test.go, precheck.go (Keycloak DatabaseConfig + adminCredentialsSecretRef + initial-admin Secret)}`.

- [ ] Ginkgo: `keycloak` mode creates the Keycloak CR; `KeycloakReady` follows the CR condition (set it in the test); suspend sets `instances: 0`; the RESTMapper probe false → `KeycloakOperatorNotInstalled` and no `Owns` (unit test the probe with a fake RESTMapper, the ECK test shape); `externalKeycloak` reads the admin Secret and creates no CR; deleting a generated Secret re-generates it.
- [ ] Implement; PASS; commit. Verify in implementation whether Identity updates an existing client's redirect URIs on restart (run Identity against a Keycloak locally or read the Identity source); record the answer in the PR body and in the spec's risk; if yes, declare `optimize` through `KEYCLOAK_CLIENTS_*` with one redirect URI per `CamundaOptimize` the controller lists (`List` of `CamundaOptimize` in all namespaces, their `spec.externalUrl`), else document the wildcard.
- [ ] Gates; PR `feat(management): run Keycloak through the Keycloak Operator or connect to an existing one`, `Towards #188`.

### #189 — branch `feat/management-cluster--console-ping`

#### Task 3.4: Console component

**Files:** `pkg/components/camundamanagementcluster/console.go`, `ping.go`, tests, goldens updated to include Console.

**Produces:** `consoleComponents(in) []*component.Component`; `PingEnv(consoleURL, clusterName, clusterVersion string) []corev1.EnvVar` (8.9 keys `CAMUNDA_CONSOLE_PING_ENABLED|ENDPOINT|CLUSTERNAME|PINGPERIOD`; from 8.10 `CAMUNDA_HUB_PING_*`).

- [ ] Unit tests: env table per mode; `PingEnv` key set by version; disabled Console → no components.
- [ ] Implement (Deployment `images.Console`, readiness `/health/readiness:9100`, Service 80→8080 — verify ports against the chart); PASS; commit `feat(management): render Console (#189)`.

#### Task 3.5: Ping attachment

**Files:** `internal/controller/camundamanagementcluster/ping.go`, `ping_test.go`, `controller.go` (hook after attachment), `finalizer.go` (withdraw).

- [ ] Ginkgo: attached cluster gets the entries in top-level `spec.extraEnv` under `AttachmentFieldManager`; deselect → withdrawn; Console disabled → withdrawn, claim kept; CR deleted → withdrawn; an 8.10 cluster gets `CAMUNDA_HUB_PING_*`.
- [ ] Implement with the `applyExporterPatch` shape (UID precondition, `ForceOwnership`); PASS; commit `feat(management): point every attached cluster at Console (#189)`.
- [ ] Gates; PR `feat(management): deploy Console and point every selected cluster at it`, `Towards #189`.

### #190 — branch `feat/management-cluster--web-modeler`

#### Task 3.6: Web Modeler components and cluster list

**Files:** `pkg/components/camundamanagementcluster/webmodeler.go`, `clusters.go`, `secrets.go` (pusher Secret), tests, goldens updated.

**Produces:** `webModelerComponents(in) []*component.Component`; `ClustersEnv(clusters []AttachedCluster) []corev1.EnvVar` (the `CAMUNDA_MODELER_CLUSTERS_<n>_*` block per attached cluster, `BEARER_TOKEN` for oidc, `BASIC` with `_BASIC_USERNAME=web-modeler` and `_BASIC_PASSWORD` from the user Secret for basic; verify the exact basic-auth key names against the Web Modeler docs page before writing them).

- [ ] Unit tests: restapi env table (datasource, mail, server URL, OAuth, audiences, pusher pairing, `CLIENT_PUSHER_*` from `websocketsExternalUrl`, license); websockets env; `ClustersEnv` for one oidc and one basic cluster; a cluster without endpoints omitted.
- [ ] Implement; goldens; PASS; commit `feat(management): render Web Modeler with the attached clusters (#190)`.

#### Task 3.7: The dedicated user and the admin client calls

**Files:** `pkg/camundaadmin/roles.go`, `roles_test.go` (`AssignRole(ctx, roleID, username)`, `CreateAuthorization(ctx, Authorization)`), `internal/controller/camundamanagementcluster/webmodeleruser.go`, `_test.go`, `controller.go` (hook), `finalizer.go` (remove users).

- [ ] Tests against `camundaadmintest`: the two calls; controller: for an attached basic cluster a Secret `<name>-web-modeler-cluster-<uid8>` exists and the user PUT was made with its password; failure → `status.clusters` row `BasicAuthUserFailed`; CR deletion removes the user.
- [ ] Verify in the Camunda docs which authorizations Web Modeler needs (`camunda-docs` search: "Web Modeler deploy authorizations", "authorizations resource types"); encode them in `webmodeleruser.go` as constants with the source URL; fall back to assigning the `admin` role and say so in the PR body.
- [ ] Implement; PASS; commit `feat(management): give Web Modeler its own user on basic-auth clusters (#190)`.
- [ ] Gates; PR `feat(management): deploy Web Modeler with the selected clusters wired in`, `Towards #190`.

---

## Wave 4 — parallel: #191 e2e, #192 docs

### #191 — branch `test/management-cluster--e2e-flows`

- [ ] `test/e2e/e2e_suite_test.go`: `setupKeycloakOperator()` mirroring `setupECK()` (`KEYCLOAK_OPERATOR_INSTALL_SKIP`, `KEYCLOAK_OPERATOR_VERSION`, install CRDs + operator manifests from `keycloak/keycloak-k8s-resources` at the version vendored in `internal/testenv/crds/keycloak/VERSION`, `utils.IsKeycloakOperatorInstalled`, teardown when installed by the suite).
- [ ] `test/e2e/management_test.go`: Flow A (label `management-keycloak`) and Flow B (label `management-oidc`) as the spec states, with the waits in the style of the existing flows; `testdata/management-*.yaml` manifests.
- [ ] `test/e2e/matrix/<minor>.env`: `IDENTITY_IMAGE`, `CONSOLE_IMAGE`, `WEB_MODELER_RESTAPI_IMAGE`, `WEB_MODELER_WEBSOCKETS_IMAGE`, `KEYCLOAK_IMAGE`; `renovate.json5` rules if the existing ones do not match.
- [ ] Run both flows on a kind cluster (`KIND_CLUSTER`, detached with `setsid nohup`, Monitor), fix what fails in the right PR (fix in the branch that owns the code, merge forward), paste the passing output in the PR body.
- [ ] PR `test(e2e): prove the management plane with the Keycloak Operator in kind`, `Towards #191`.

### #192 — branch `docs/management-cluster--user-docs`

- [ ] Invoke `writing-operator-docs` (and the skills it loads). Rewrite `docs/crds/camundamanagementcluster.md` from `TEMPLATE.md`; update `managementauthconfig.md`, `camundaplatformconfig.md`, `camundacluster.md`, `camundaoptimize.md`, `index.md`, `architecture.md`, `installation.md`; write `docs/guides/management-plane.md`; `mkdocs.yml` nav.
- [ ] Every condition message quoted from the code; every field from `api/v1`; every Camunda link from a `camunda-docs` search.
- [ ] Fresh-reader pass (`feature-dev-workflow:writing-docs`); `mkdocs build --strict`.
- [ ] PR `docs(crds): document the management plane`, `Towards #192`.

---

## Integration

- [ ] `feature-dev-workflow:reviewing-feature-progress` at each wave boundary and before the integration PR.
- [ ] Integration PR `feat/management-cluster` → `main`, title `feat(management): run the Camunda management plane from one CamundaManagementCluster`, body `Closes #185`; run `feature-dev-workflow:copilot-review-loop` on it; teardown of plan and state after CI is green; the spec stays (no ADR convention in this repo). The merge to main is the user's.
