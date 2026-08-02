# Contract-CRD validation controllers (Batch A)

**Status:** draft
**Date:** 2026-08-02
**Scope:** DatabaseServerConfig, DatabaseConfig, SecondaryStorageConfig, ObjectStorageConfig, ManagementAuthConfig

## Summary

Implement the five contract-CRD validation controllers — the first controller batch from the
[implementation order](../../crds/index.md#implementation-order). Each contract CRD gets real
`api/v1` types replacing the kubebuilder placeholder stubs, admission-time schema validation,
and a thin validation-only reconciler that continuously checks the contract's references and
reports the result as a `Ready` condition. Alongside them land the minimal shared foundations
every later batch inherits: shared secret-reference types, condition and secret-check helpers,
watch-mapping helpers, and the envtest/testify test conventions.

The authoritative behavioral contract is `docs/crds/<kind>.md` for each of the five kinds. This
spec does not restate those documents; it records the decisions about *how* the operator
implements them.

## Goals

- Real `api/v1` types for the five contract CRDs, field-for-field faithful to their design docs,
  with generated deepcopy and CRD manifests.
- Every rule in each doc's **Validation** section enforced at admission time by the CRD schema.
- A validation-only controller per kind: watches the CR and everything it references, re-runs
  validation on any change, and maintains the documented `Ready` condition
  (`Healthy` / `MissingSecret` / `InvalidReference`) plus `status.observedGeneration`.
- Shared foundations sized to what these five controllers need — nothing speculative.
- The batch is covered by CI as it ships (the existing Tests workflow runs `make test`).

## Non-goals

- No provisioning of any kind — these controllers never create, update, or delete anything
  except their own CR's status.
- No connectivity probing: validation checks that references resolve and Secrets have the
  required keys; it does not dial database servers, Elasticsearch endpoints, or OIDC issuers.
- No operator-component-framework components in these reconcilers (see "Framework posture").
- No admission webhooks — CRD schema (markers + CEL) covers all admission rules.
- No finalizers: deleting a contract CR needs no cleanup.
- No changes to the other 14 CRDs beyond what shared types require.
- No helm chart or Test Chart workflow repair (owned by separate ongoing work).

## Design

### Contract-first: the docs bind the implementation

`docs/crds/<kind>.md` is the API contract. Types, validation rules, condition reasons, and
watch behavior implement those docs exactly. If implementation reveals a doc to be wrong or
incomplete, the doc is corrected in the same PR — the two never diverge across a merge.

### Admission vs. runtime validation split

Static and cross-field rules are enforced by the CRD schema so invalid contracts never enter
the cluster; cross-resource checks (which can change truth value without the CR changing) are
the controller's job and surface as conditions:

| Enforcement | Rules |
| --- | --- |
| Schema markers | `engine` enum, `port` 1–65535, `type` enums, `provider` enums, required fields, defaults |
| CEL `XValidation` | `pitr.enabled` ⇒ `retentionPeriodDays ≥ 1`; exactly the block matching `spec.type` set (SecondaryStorageConfig); `provider` ↔ `type` pairing (ObjectStorageConfig); `isURL()` + scheme checks on all URL fields |
| Controller conditions | Referenced Secrets exist and contain the configured keys; referenced CRs (`serverRef`, `databaseConfigRef`) exist |

### Shared API types

Two reference types live in `api/v1` and are shared by all contract specs (and later batches):

- **`SecretKeyRef`** — `name`, `namespace`, `key`: a single value in a Secret
  (ManagementAuthConfig's `clientSecretRef`).
- **`CredentialsSecretRef`** — `name`, `namespace`, `usernameKey`, `passwordKey`: a
  username/password pair in a Secret (all `*credentialsSecretRef` fields).

`namespace` is required on both: every consumer here is cluster-scoped, so there is no default
namespace to fall back to. Cross-CR references (`serverRef`, `databaseConfigRef`) stay plain
`string` fields per the docs — they name cluster-scoped objects.

### Controller shape

Five thin, hand-written controller-runtime reconcilers with one common skeleton:

1. Fetch the CR; ignore not-found.
2. Run the kind's documented checks in the order its doc lists them; the first failure
   determines the condition (reason + a message naming the exact missing object or key).
3. Set `Ready` (with `observedGeneration` on the condition) and `status.observedGeneration`,
   then patch status via SSA with field owner `camunda-operator` — per the repo-wide
   SSA-exclusive rule. Skip the patch when nothing changed.
4. Return no requeue on success or clean failure (watches drive re-validation); return the
   error (default backoff) only on transient API failures.

Shared helpers live under `pkg/` (per the repo layout): condition construction/setting, the
Secret-keys check (returns the precise `MissingSecret` message or nil), and the watch-mapping
plumbing below. The five reconcilers stay small enough that each is readable on one screen.

### Watching referenced objects

The docs promise re-validation "whenever either changes". The CRs are cluster-scoped and may
reference Secrets in any namespace, so a default informer would cache every Secret in the
cluster. Instead:

- **Secrets are watched metadata-only** (`metav1.PartialObjectMetadata`): events fire on every
  Secret change, but only object metadata is cached, keeping memory flat regardless of cluster
  size. When a reconcile needs Secret *data* it reads that single Secret uncached via the
  manager's `APIReader`.
- **Field indexes map events to CRs**: each controller indexes its CRs by referenced Secret
  `namespace/name` (and by referenced CR name), and a shared `EnqueueRequestsFromMapFunc`
  handler looks up the index to enqueue exactly the referencing CRs.
- **CR-to-CR watches use normal typed informers** (our own CRDs are few and small):
  DatabaseConfig watches DatabaseServerConfig; SecondaryStorageConfig watches DatabaseConfig.
- RBAC: the manager needs cluster-wide `get;list;watch` on `secrets`. The operator can read any
  Secret's data on demand — an accepted consequence of validating user-named references; it
  never copies Secret contents anywhere.

An accepted, documented side effect: anyone permitted to create a contract CR can learn whether
an arbitrary Secret exists and whether it has given keys (an existence oracle). Contract CRDs
are cluster-scoped and intended for operators of the platform, so RBAC on the contract kinds is
the mitigation.

### Status and conditions

- Single condition type `Ready`, reasons exactly as documented: `Healthy`, `MissingSecret`,
  `InvalidReference` (ObjectStorageConfig only ever reports `Healthy`).
- `status.observedGeneration` records the last reconciled generation; conditions carry their
  own `observedGeneration` too.
- Condition messages name the failing object concretely
  (e.g. `Secret "camunda-system/creds" is missing key "password"`), so a `kubectl describe`
  answers "what do I fix" without reading operator logs.

### Testing

- **Reconciliation (Ginkgo + envtest):** per-controller specs run against a live manager with
  all five controllers registered. Core scenarios: CR with missing Secret → `Ready=False`,
  `MissingSecret`; creating the Secret flips it to `Healthy` without touching the CR (proves
  the watch path); deleting it flips it back; dangling `serverRef`/`databaseConfigRef` →
  `InvalidReference`; `observedGeneration` tracks spec updates.
- **Schema validation (envtest):** invalid manifests for every marker/CEL rule are rejected by
  the API server — the admission table above is tested, not assumed.
- **Unit (testify):** the check helpers, message construction, and mapping functions get pure
  Go table tests next to their implementation.
- Everything runs under `make test`, which the Tests workflow already executes on every push
  and PR.

### Framework posture (operator-component-framework)

The reconcilers in this batch do not use ocf components: the framework's condition reasons are
a closed enum (`Healthy`/`Blocked`/`OperationFailing`/…) that cannot express the documented
`MissingSecret`/`InvalidReference` reasons, it provides no watch/index helpers (the bulk of a
validation controller), and its resource-management machinery has nothing to manage here. ocf
remains the standard for controllers that *do* manage resources, starting with Batch B.

This feature does set the project up for that future: the ocf dependency is pinned at
`v0.18.1`, the `ocf` scaffolding CLI is pinned to the same version via a go.mod `tool`
directive (`go tool ocf`), and the ocf Claude Code plugin is enabled through
`.claude/settings.json` with its marketplace ref pinned to the same tag. The dependency bump
(controller-runtime 0.24.1, k8s.io 0.36.1, go 1.26) builds clean and passes the full existing
test suite.

## Risks

- **Metadata-only watches are the least-trodden path** in controller-runtime; wiring
  `PartialObjectMetadata` sources wrong typically fails silently (no events). Mitigated by the
  envtest scenarios that explicitly prove watch-triggered re-validation.
- **CEL cost budget:** per-field validation cost limits are generous for rules this small, but
  `isURL()` on six fields (ManagementAuthConfig) is verified against envtest's API server
  rather than assumed.
- **Transitive upgrade risk** from the ocf bump (controller-runtime minor, k8s.io minor):
  already validated — full suite green post-bump.

## Alternatives considered

- **ocf read-only components for validation** — rejected for this batch: closed reason
  vocabulary contradicts the published CRD contract, no watch helpers, and the imported
  mutation/SSA/suspension engine would be dead weight. Revisit if ocf grows free-form reasons
  or a validation-oriented shape.
- **Full Secret informer** — rejected: caches every Secret in the cluster inside the operator;
  memory and security-posture cost with no benefit over metadata-only + targeted reads.
- **No watch, periodic requeue** — rejected: violates the documented "re-runs validation
  whenever either changes" behavior and adds up to a full interval of reaction latency.
- **Admission webhooks for cross-field rules** — rejected: CEL covers every documented rule
  without a webhook server, certificates, or availability concerns.
