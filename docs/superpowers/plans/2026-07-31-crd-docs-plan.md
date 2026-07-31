# CRD Documentation Foundation Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking. Orchestration across PRs uses feature-dev-workflow:developing-a-feature + fanning-out-with-worktrees.

**Goal:** Author the authoritative API design for all 19 core-operator CRDs as user-facing mkdocs documentation, so controller implementation can later be fanned out per-CRD.

**Architecture:** Docs-first. PR 1 (issue #2) lands tooling + conventions + the rewritten architecture doc; five docs PRs (issues #3–#7) then run in parallel, each adding a batch of `docs/crds/<kind>.md` pages plus their nav entries. Everything targets `feature/crd-docs`; an integration PR to `main` collects the feature after a coherence review.

**Tech Stack:** mkdocs-material (pinned via `requirements-docs.txt`), mermaid via pymdownx.superfences, GNU Make targets, `gh` for PRs.

**Spec:** `docs/superpowers/specs/2026-07-31-crd-docs-design.md` (approved). Tracking: epic konsole-is/camunda-operator#1, sub-issues #2–#7.

**Source material for every docs task:** the original cross-operator proposal is the pre-rewrite `docs/architecture.md`. After Task 1 merges it is gone from the branch — read it from git instead: `git show origin/main:docs/architecture.md`. Verify all Camunda 8.9 behavior claims against the `camunda-docs` MCP server (`search_camunda_knowledge_sources`) AND the camunda/camunda source checkout at `~/Documents/camunda/camunda`.

## Global Constraints

- API group in every example: `apiVersion: core.camunda.io/v1` — never `camunda.io/v1`.
- Camunda 8.9+ only. No version-conditional rendering, no pre-8.9 topology, no migration/adoption fields (`statefulSetName`, `eckResourceName` do not exist).
- Clean slate: no ZeebeCluster concepts anywhere.
- Presets are passive data CRDs: no controllers, no `autoResize` fields. `PVCAutoResize` is always created explicitly.
- Contract CRDs (`SecondaryStorageConfig`, `ObjectStorageConfig`, `DatabaseServerConfig`, `DatabaseConfig`, `ManagementAuthConfig`) have lightweight validation-only controllers — they never provision anything.
- Controller inventory is final: 17 active + 2 passive. Docs must not invent new CRDs or controllers.
- No cloud/saas operator design content: `CloudCamundaCluster`, `EncryptedVolume`, `CloudObjectStorage` etc. may appear only as external actors in Purpose/Relationships prose ("a composition layer above may create this CR").
- Markdown prose: one line per paragraph and per list item, no hard-wrapping.
- `make docs-build` (strict) must pass at the end of every task.

## Contracts

| Name | Producer | Consumers | Shape | Realization |
| --- | --- | --- | --- | --- |
| `crd-doc-template` | #2 | #3–#7 | Seven H2 sections in this exact order: `## Purpose`, `## How it works`, `## API reference`, `## Status`, `## Validation`, `## Relationships`, `## Examples` (template exemplar lands in Task 1) | pre-merge PR (#2 merges before batches branch) |
| `crd-file-paths` | — | #3–#7 | `docs/crds/<kind lowercase>.md`, exact names listed in Task 1's inventory table (e.g. `docs/crds/camundacluster.md`, `docs/crds/secondarystorageconfig.md`) | data-only |
| `mkdocs-nav-groups` | #2 | #3–#7 | Fixed nav groups under `CRDs`: `Cluster`, `Storage`, `Contracts`, `Backup & Restore`, `Management & Extensions`. Each batch appends only its own pages to its group; nav insertion order within a group = the order the kinds appear in that task | pre-merge PR (#2) |
| `api-vocabulary` | — | #3–#7 | Reference-field shapes pinned in Conventions below (namespaced object refs vs cluster-scoped string refs vs secret refs); condition naming | data-only |

## Conventions

Every docs task inherits these. They exist so five parallel authors produce one coherent API.

**Reference fields (the core API vocabulary):**

- Reference to a namespaced CR: object with `name` (required) and `namespace` (optional, defaults to the referencing CR's namespace). Field name is `<thing>Ref`, e.g. `clusterRef: {name: my-cluster}`, `backupRef`, `targetClusterRef`.
- Reference to a cluster-scoped CR (all contract CRDs, presets, platform config): plain string holding the target's name, e.g. `storageRef: "my-storage-config"`, `presetRef: "medium"`, `serverRef: "my-db-server"`.
- Secret reference (single value): `{name, namespace, key}`, field named `<thing>SecretRef` (e.g. `clientSecretRef`, `licenseSecretRef`).
- Credentials secret reference (username+password): `{name, namespace, usernameKey, passwordKey}`, field named `credentialsSecretRef` / `adminCredentialsSecretRef` / `backupCredentialsSecretRef`.
- Output-name fields (CR the controller creates): plain string named after the created kind, e.g. `secondaryStorageConfig: "my-storage-config"` on ElasticsearchCluster, `databaseConfig: "my-camunda-db"` on Database.

**Status and conditions:**

- Conditions are the primary status mechanism; every active CRD has an aggregate `Ready` condition.
- Per-component conditions are PascalCase `<Component>Ready` (e.g. `ZeebeReady`, `GatewayReady`, `KeycloakReady`).
- Suspendable CRDs report a `Suspended` condition when scaled down.
- Reasons are PascalCase single words or short phrases: `Healthy`, `Suspended`, `InvalidReference`, `MissingSecret`, `Progressing`.
- Long-running operations (Backup, restores) additionally use `status.phase` with the enum values given in their task.
- Every status documents `observedGeneration`.

**Prose and examples:**

- Audience is the platform operator ("you"); controller behavior is narrated as "the operator ..." in third person.
- Shared vocabulary (use exactly these terms): "orchestration cluster" (the Camunda workload set), "secondary storage" (ES or RDBMS behind `storageRef`), "contract CRD", "management plane", "composition layer" (the thing above that may create CRs).
- Example resource names: cluster `my-cluster` in namespace `my-cluster-ns`; derived names prefixed with it (`my-cluster-es`, `my-cluster-backup-001`); cluster-scoped config CRs named for their role (`my-storage-config`, `my-db-server`).
- API reference is one fenced YAML block with every field present, each annotated with a comment line above it: `# <type>. <Required|Optional>[, default: <value>]. <one-line meaning>`.
- Mermaid diagrams: `graph LR` or `graph TD`; solid arrows `-->` mean "creates/provisions"; dotted arrows `-.->` mean "reads/references/patches"; suffix external systems with `(external)` in the node label; use real kind names as node labels.
- Workload labels are always written as `camunda.io/cluster` and `camunda.io/component` (the label domain is not the API group).
- SSA is spelled "Server-Side Apply (SSA)" on first use per page; every SSA-patching controller names its field manager.

**Conventions added during fan-out (wave 2, from the storage batch; propagated to all batches):**

- SSA field-manager naming scheme: `camunda-operator/<kind lowercase>` (e.g. `camunda-operator/pvcautoresize`).
- Deviation notes use the mkdocs-material admonition `!!! note "Deviation from the original proposal"`.
- Preset pages document which spec fields are preset-legal: `spec.cluster` rejects instance-bound fields (`presetRef` chaining, output-name fields, `suspend`, `monitoring`); passive kinds report no status.
- Verified 8.9 facts every page may cite: Elasticsearch floor 8.19+ (9.2+ recommended); RDBMS secondary storage GA (postgresql/mysql/oracle/mariadb/mssql per support policy); the Database CRD bootstrap is deliberately postgres-only; Optimize requires ES/OS secondary storage.

**Git/PR mechanics:**

- Commits reference the sub-issue: `docs: add contract CRD pages (#3)`.
- Sub-PRs target `feature/crd-docs`, body uses `Towards #<sub-issue>`. Integration PR targets `main`, body uses `Closes #1`.
- Every commit message ends with `Co-Authored-By: Claude Fable 5 <noreply@anthropic.com>`.

---

### Task 1: Docs tooling, template, architecture rewrite, CRD index (issue #2, PR into feature/crd-docs)

**Files:**
- Create: `mkdocs.yml`, `requirements-docs.txt`, `docs/index.md`, `docs/crds/index.md`, `docs/crds/TEMPLATE.md`
- Modify: `Makefile` (append docs targets), `docs/architecture.md` (full rewrite)

**Interfaces:**
- Consumes: original proposal via `git show origin/main:docs/architecture.md`; mkdocs setup reference at `~/Documents/personal/operator-component-framework/{mkdocs.yml,requirements-docs.txt,Makefile}`.
- Produces: the `crd-doc-template` (TEMPLATE.md), `mkdocs-nav-groups` (mkdocs.yml nav), and the inventory/index every batch links back to.

- [ ] **Step 1: Add mkdocs tooling**

`requirements-docs.txt`:

```
mkdocs-material==9.7.6
```

`mkdocs.yml` — adapt the operator-component-framework config: `site_name: Camunda Operator`, `site_description: Core Kubernetes operator for the Camunda platform`, `repo_url: https://github.com/konsole-is/camunda-operator`, `repo_name: konsole-is/camunda-operator`, `edit_uri: edit/main/docs/`, keep `exclude_docs: superpowers/`, keep the theme/palette/markdown_extensions/plugins blocks verbatim (mermaid superfences included), drop `extra_css`. Nav skeleton:

```yaml
nav:
  - Home: index.md
  - Architecture: architecture.md
  - CRDs:
      - Overview: crds/index.md
      - Cluster: []
      - Storage: []
      - Contracts: []
      - Backup & Restore: []
      - Management & Extensions: []
```

(Empty groups are placeholders; each batch task replaces its group's `[]` with its page list. If `mkdocs build --strict` rejects empty lists, put the group's pages in directly from this task's knowledge of the file paths — pages may be listed in nav only once they exist, so in that case each batch task adds its own group lines instead, and this task leaves only `Overview` under CRDs.)

Makefile — append:

```makefile
.PHONY: docs-serve
docs-serve: ## Serve the documentation site locally with live reload.
	mkdocs serve

.PHONY: docs-build
docs-build: ## Build the documentation site in strict mode.
	mkdocs build --strict
```

- [ ] **Step 2: Write `docs/index.md`** — site home: one-paragraph project description (core operator, bottom layer of the three-operator stack), quick links to Architecture and CRD Overview, install pointer to README. Keep under a screen.

- [ ] **Step 3: Rewrite `docs/architecture.md`** core-only. Structure:

```
# Architecture
## The extension model            (features attach to workloads; workloads don't know about features; cert-manager/HPA analogy)
## How features connect           (3 mechanisms: explicit inputs before creation → storageClassName example; clusterRef + SSA patching after creation → field managers, watch + field indexer pattern; contract CRDs for data passing → producer/consumer decoupling)
## CRD overview                   (mermaid graph of all 19 kinds and their creates/references edges — adapt the proposal's camunda-operator diagram, core kinds only)
## Deployment context             (one paragraph: cloud/saas operators exist above, they create CRs of this operator, link to their repos when public; this operator has zero knowledge of them)
## Support policy                 (Camunda 8.9+ unified architecture only; minor releases are the test matrix; features from patches are picked up in the next minor; clean-slate — no ZeebeCluster migration)
```

Adapt prose from the proposal's "The Proposed Direction" and "How Features Connect to the Core" sections; drop everything else (problem statement, path counting, benefits, cloud/saas CRD designs, SaaS control-plane flow, migration strategy).

- [ ] **Step 4: Write `docs/crds/TEMPLATE.md`** — the seven-section exemplar with guidance comments per section (what belongs there, per the spec's Doc Template section and the Conventions above). Exclude it from nav; add an `<!-- This is the authoring template; copy it to docs/crds/<kind>.md -->` header comment.

- [ ] **Step 5: Write `docs/crds/index.md`** with three parts:

Part 1 — inventory table (19 rows, exactly these files):

| Kind | File | Scope | Controller | Purpose |
| --- | --- | --- | --- | --- |
| CamundaCluster | `camundacluster.md` | Namespaced | Active | Core orchestration cluster |
| CamundaPlatformConfig | `camundaplatformconfig.md` | Cluster | Active | Shared OIDC, license, image registry |
| CamundaClusterPreset | `camundaclusterpreset.md` | Cluster | Passive | Standardized cluster sizing |
| ElasticsearchCluster | `elasticsearchcluster.md` | Namespaced | Active | Elasticsearch lifecycle via ECK |
| ElasticsearchClusterPreset | `elasticsearchclusterpreset.md` | Cluster | Passive | Standardized ES sizing |
| Database | `database.md` | Cluster | Active | Logical database and user bootstrapping |
| DatabaseServerConfig | `databaseserverconfig.md` | Cluster | Active (validation) | Contract: database server connection |
| DatabaseConfig | `databaseconfig.md` | Cluster | Active (validation) | Contract: logical database connection |
| SecondaryStorageConfig | `secondarystorageconfig.md` | Cluster | Active (validation) | Contract: secondary storage backend |
| ObjectStorageConfig | `objectstorageconfig.md` | Cluster | Active (validation) | Contract: bucket storage |
| ManagementAuthConfig | `managementauthconfig.md` | Cluster | Active (validation) | Contract: Management Identity OIDC |
| Backup | `backup.md` | Namespaced | Active | One backup operation |
| BackupSchedule | `backupschedule.md` | Namespaced | Active | Cron-driven Backup creation |
| BackupRetention | `backupretention.md` | Namespaced | Active | Old-backup deletion |
| PointInTimeRestore | `pointintimerestore.md` | Namespaced | Active | RDBMS in-place PITR |
| LogicalRestore | `logicalrestore.md` | Namespaced | Active | Cross-cluster restore from a Backup |
| CamundaOptimize | `camundaoptimize.md` | Namespaced | Active | Optimize deployment per cluster |
| CamundaManagementCluster | `camundamanagementcluster.md` | Cluster | Active | Management plane (Console, Web Modeler, Identity) |
| PVCAutoResize | `pvcautoresize.md` | Namespaced | Active | topolvm auto-resize annotations |

Part 2 — reconciler dependency graph: mermaid `graph TD` with produces/consumes edges (ElasticsearchCluster → SecondaryStorageConfig; Database → DatabaseConfig + SecondaryStorageConfig; DatabaseConfig -.-> DatabaseServerConfig; CamundaCluster -.-> SecondaryStorageConfig/ObjectStorageConfig/CamundaPlatformConfig/CamundaClusterPreset; Backup/BackupSchedule/BackupRetention/restores/CamundaOptimize/PVCAutoResize -.-> CamundaCluster; CamundaOptimize -.-> ManagementAuthConfig; CamundaManagementCluster → ManagementAuthConfig, -.-> DatabaseConfig ×3).

Part 3 — implementation order (for the future controller-implementation epic), derived from the graph:

```
Batch A (no dependencies): contract CRD validation controllers — DatabaseServerConfig, DatabaseConfig, SecondaryStorageConfig, ObjectStorageConfig, ManagementAuthConfig
Batch B (produce contracts): ElasticsearchCluster, Database
Batch C (consume contracts): CamundaCluster, CamundaPlatformConfig handling
Batch D (attach to clusters): Backup, BackupSchedule, BackupRetention, LogicalRestore, PointInTimeRestore, CamundaOptimize, CamundaManagementCluster, PVCAutoResize
```

- [ ] **Step 6: Verify** — `pip install -r requirements-docs.txt` (or confirm mkdocs-material present) then `make docs-build`; expect strict pass. `make docs-serve` spot-check mermaid rendering.

- [ ] **Step 7: Commit and open PR** — commit `docs: add docs tooling, CRD index, and core-only architecture (#2)`; PR into `feature/crd-docs` titled after issue #2 with body `Towards #2`, per feature-dev-workflow:opening-a-pull-request.

---

### Task 2: Contract CRD docs (issue #3, PR into feature/crd-docs)

**Files:**
- Create: `docs/crds/secondarystorageconfig.md`, `docs/crds/objectstorageconfig.md`, `docs/crds/databaseserverconfig.md`, `docs/crds/databaseconfig.md`, `docs/crds/managementauthconfig.md`
- Modify: `mkdocs.yml` (add the five pages under the `Contracts` nav group)

**Interfaces:**
- Consumes: TEMPLATE.md structure; proposal §2 "Contract CRDs" via `git show origin/main:docs/architecture.md`.
- Produces: the contract vocabulary pages that all other CRD docs cross-link.

- [ ] **Step 1: Write the five docs.** Required content per page (template sections implied):
  - Common to all five: producer/consumer table in Purpose (who creates it: which peer controller, a composition layer above, or the user by hand); validation-only controller behavior in How it works (validate referenced secrets/CRs exist and have required keys → conditions `Ready`/`InvalidReference`/`MissingSecret`); note that the consuming controller reads the contract by name, never caring who produced it.
  - `secondarystorageconfig.md`: `spec.type: elasticsearch | rdbms`; `elasticsearch: {endpoint, credentialsSecretRef{name,namespace,usernameKey,passwordKey}}`; `rdbms: {databaseConfigRef: <string>}`; produced by ElasticsearchCluster or Database, consumed by CamundaCluster (`storageRef`) and the backup/restore controllers.
  - `objectstorageconfig.md`: `provider: aws|gcp|azure`, `type: S3|GCS|AzureBlob`, `bucketId`, `bucketName`, `basePath`, `accountId` (workload identity the bucket trusts); consumed via `backupStorageRef`/`documentStorageRef` on CamundaCluster; produced by a composition layer above or manually.
  - `databaseserverconfig.md`: `engine: postgres` (document the enum as postgres-only for now if 8.9 verification doesn't support oracle/mariadb — record the deviation), `host`, `port`, `adminCredentialsSecretRef`, `pitr: {enabled, retentionPeriodDays}`; consumed by Database (`serverRef`) and PointInTimeRestore validation.
  - `databaseconfig.md`: `serverRef` (string → DatabaseServerConfig), `databaseName`, `credentialsSecretRef`, optional `backupCredentialsSecretRef`; produced by Database or manually; consumed by SecondaryStorageConfig (rdbms) and CamundaManagementCluster.
  - `managementauthconfig.md`: `baseUrl`, OIDC endpoints (`issuerUrl`, `issuerBackendUrl`, `tokenUrl`, `jwksUrl`), `clientId`, `audience`, `clientSecretRef`; produced by CamundaManagementCluster (or shipped directly in SaaS); consumed by CamundaOptimize via `managementAuthRef`.
- [ ] **Step 2: Verify claims** — ES endpoint/credential expectations and RDBMS support status against camunda-docs MCP + `~/Documents/camunda/camunda` (`db/`, `configuration/` trees). Record deviations in the doc where found.
- [ ] **Step 3: Add nav entries, run `make docs-build`** — strict pass required.
- [ ] **Step 4: Template conformance self-check** — all seven H2 sections present per page, in order.
- [ ] **Step 5: Commit and open PR** — `docs: add contract CRD pages (#3)`, PR body `Towards #3`.

---

### Task 3: Core cluster CRD docs (issue #4, PR into feature/crd-docs)

**Files:**
- Create: `docs/crds/camundacluster.md`, `docs/crds/camundaplatformconfig.md`, `docs/crds/camundaclusterpreset.md`
- Modify: `mkdocs.yml` (add the three pages under the `Cluster` nav group)

**Interfaces:**
- Consumes: TEMPLATE.md; proposal §1 and §3 via `git show origin/main:docs/architecture.md`.
- Produces: the topology and preset-merge vocabulary that storage/backup/extension docs reference.

- [ ] **Step 1: Write `camundacluster.md`.** Required content: component topology (zeebe always standalone StatefulSet with `replicas`/`partitions`/`replicationFactor`/`storageClassName`/`storageSize`; gateway/operate/tasklist/identity each `mode: Standalone | Embedded`, embedded apps run inside the nearest standalone app up the chain — gateway first, else zeebe; connectors standalone-only when enabled with `replicas`); per-component + top-level `resources`, `extraEnv`, `extraEnvFrom`, `podLabels`, `podAnnotations`, `scheduling` (nodeAffinity/tolerations/podAffinity; cluster-level `scheduling` replaces preset entirely, no merge); `platformConfigRef` (string), `presetRef` (string) + override semantics (pointer sizing fields override preset, absent fields inherit); optional per-cluster `auth` override; `externalUrl` (deterministic, set before creation, used for OIDC redirects; the operator creates no Ingress); `serviceAccount.annotations` for workload identity; required `storageRef`, optional `backupStorageRef`/`documentStorageRef`; optional `monitoring.serviceMonitor {enabled, labels, annotations}`; `suspend` (scale down workloads) and `pause` (halt reconciliation); workload labels `camunda.io/cluster` + `camunda.io/component`; status = conditions only (`<Component>Ready` per standalone component + `Ready`), `observedGeneration`.
- [ ] **Step 2: Write `camundaplatformconfig.md`.** Required content: cluster-scoped, one per environment; `auth.method: basic | oidc`; `auth.oidc` endpoints (issuerUrl, issuerBackendUrl, jwksUrl, tokenUrl, authUrl) + default client credentials (`clientId`, `audience`, `clientSecretRef`) overridable per preset/cluster; `licenseSecretRef`; `imageRegistry`; runtime propagation (operator watches it, changes roll out to referencing clusters); validation controller checks secrets exist.
- [ ] **Step 3: Write `camundaclusterpreset.md`.** Required content: passive data CRD, no controller; `spec.cluster` reuses the CamundaCluster spec type as a full baseline; consumers resolve preset + apply pointer-field overrides; `scheduling` replace-not-merge exception; explicitly note the deviation: no autoResize in presets, create PVCAutoResize directly.
- [ ] **Step 4: Verify claims** — unified binary topology, Spring-profile-driven embedded/standalone modes, 8.9 identity setup against camunda-docs MCP + camunda/camunda source (`dist/`, `configuration/`, `authentication/` trees). Record deviations.
- [ ] **Step 5: Nav entries + `make docs-build`** — strict pass.
- [ ] **Step 6: Template conformance self-check.**
- [ ] **Step 7: Commit and open PR** — `docs: add core cluster CRD pages (#4)`, body `Towards #4`.

---

### Task 4: Storage backend CRD docs (issue #5, PR into feature/crd-docs)

**Files:**
- Create: `docs/crds/elasticsearchcluster.md`, `docs/crds/elasticsearchclusterpreset.md`, `docs/crds/database.md`
- Modify: `mkdocs.yml` (add the three pages under the `Storage` nav group)

**Interfaces:**
- Consumes: TEMPLATE.md; proposal §4 and §16 via `git show origin/main:docs/architecture.md`.
- Produces: pages referenced by camundacluster.md (storageRef chain) and the backup/restore docs.

- [ ] **Step 1: Write `elasticsearchcluster.md`.** Required content: deploys an ECK `Elasticsearch` CR (ECK operator is an external prerequisite — document that clearly); spec: `version`, `replicas`, `resources`, `storageSize`, `storageClassName`, `extraEnv`/`extraEnvFrom`, pod labels/annotations, `scheduling`, `presetRef` + pointer overrides, `secondaryStorageConfig` (name of the SecondaryStorageConfig it creates with endpoint + generated credentials), optional `monitoring.serviceMonitor`, `suspend` (scales ECK replicas to zero → `Suspended` condition); reconciliation steps: render/apply ECK CR → watch ECK health into conditions → create/refresh SecondaryStorageConfig → handle suspend; independent lifecycle (no CamundaCluster references).
- [ ] **Step 2: Write `elasticsearchclusterpreset.md`.** Passive data CRD mirroring camundaclusterpreset.md's structure: `spec.cluster` reuses the ElasticsearchCluster spec; same override semantics; same no-autoResize deviation note.
- [ ] **Step 3: Write `database.md`.** Required content: cluster-scoped; bootstraps a logical database + users on an existing PostgreSQL server via SQL (`CREATE DATABASE`, `CREATE USER`) using `serverRef` → DatabaseServerConfig admin credentials; `targetNamespace` for created Secrets; `applicationCredentials {secretName, secretNamespace}` always created; `backupCredentials {disabled, secretName, secretNamespace}` opt-out, granted dump/restore privileges; `databaseConfig` (name of DatabaseConfig it creates, default CR name); optional `secondaryStorageConfig` (creates a type-rdbms SecondaryStorageConfig wired to the DatabaseConfig + backup credentials); validation: reject a second Database using the same `databaseName` on the same `serverRef`; reconciliation steps 1–7 as in the proposal.
- [ ] **Step 4: Verify claims** — supported ES versions for 8.9, RDBMS secondary-storage status (which engines 8.9/8.10 actually support — constrain the docs to reality), against camunda-docs MCP + camunda/camunda source (`db/` tree). Record deviations.
- [ ] **Step 5: Nav entries + `make docs-build`** — strict pass.
- [ ] **Step 6: Template conformance self-check.**
- [ ] **Step 7: Commit and open PR** — `docs: add storage backend CRD pages (#5)`, body `Towards #5`.

---

### Task 5: Backup and restore CRD docs (issue #6, PR into feature/crd-docs)

**Files:**
- Create: `docs/crds/backup.md`, `docs/crds/backupschedule.md`, `docs/crds/backupretention.md`, `docs/crds/logicalrestore.md`, `docs/crds/pointintimerestore.md`
- Modify: `mkdocs.yml` (add the five pages under the `Backup & Restore` nav group)

**Interfaces:**
- Consumes: TEMPLATE.md; proposal §6 and §7 via `git show origin/main:docs/architecture.md`.
- Produces: the procedure docs the backup/restore controller implementations will be coded against.

- [ ] **Step 1: Write `backup.md`.** Required content: `clusterRef {name, namespace?}`; controller reads the cluster's version/topology/`storageRef`; ES path = version-aware snapshot APIs on zeebe/gateway; RDBMS path = resolve `storageRef → SecondaryStorageConfig → DatabaseConfig`, create a Job running pg_dump uploading to the cluster's `backupStorageRef` bucket, fail with condition if backup credentials missing; `status.phase: Pending | Running | Completed | Failed` + conditions; document what a "backup" contains per storage type.
- [ ] **Step 2: Write `backupschedule.md`.** `clusterRef`, `schedule` (cron); creates Backup CRs on schedule; naming pattern for created Backups; suspension semantics when the cluster is suspended.
- [ ] **Step 3: Write `backupretention.md`.** `clusterRef`, `retainedCount`; lists completed Backups for the cluster, deletes oldest beyond count; never touches running backups.
- [ ] **Step 4: Write `logicalrestore.md`.** `backupRef {name, namespace}`, `targetClusterRef {name, namespace?}`; prerequisites: target suspended (`suspend: true`) — the restore controller validates but never owns the suspend field; validation: backup exists + completed, target compatible (version/topology); ES = restore via snapshot APIs; RDBMS = Job downloading dump + pg_restore with target's backup credentials; then primary storage restore; `status.phase: Pending | ValidatingCompatibility | RestoringSecondaryStorage | RestoringPrimaryStorage | Completed | Failed`.
- [ ] **Step 5: Write `pointintimerestore.md`.** `clusterRef`, `timestamp` (RFC 3339); RDBMS-only, in-place; prerequisites: cluster suspended; DatabaseServerConfig (resolved via storageRef → SecondaryStorageConfig → DatabaseConfig → serverRef) has `pitr.enabled: true` and timestamp within retention; validation: dedicated server required — reject when the DatabaseServerConfig is referenced by more than one Database; procedure: DB PITR via recovery_target_time + WAL replay → read exporter positions → restore primary storage to match; `status.phase: Pending | RestoringSecondaryStorage | RestoringPrimaryStorage | Completed | Failed`; note the CamundaCluster-side implication (PITR-enabled storage auto-enables continuous primary-storage backup) and cross-link camundacluster.md.
- [ ] **Step 6: Verify claims** — the actual 8.9 backup API surface (management/actuator endpoints, snapshot semantics, exporter positions) against camunda-docs MCP + camunda/camunda source (`zeebe/`, backup-related modules). This task has the highest drift risk from the proposal; record every deviation found.
- [ ] **Step 7: Nav entries + `make docs-build`** — strict pass.
- [ ] **Step 8: Template conformance self-check.**
- [ ] **Step 9: Commit and open PR** — `docs: add backup and restore CRD pages (#6)`, body `Towards #6`.

---

### Task 6: Management and extension CRD docs (issue #7, PR into feature/crd-docs)

**Files:**
- Create: `docs/crds/camundaoptimize.md`, `docs/crds/camundamanagementcluster.md`, `docs/crds/pvcautoresize.md`
- Modify: `mkdocs.yml` (add the three pages under the `Management & Extensions` nav group)

**Interfaces:**
- Consumes: TEMPLATE.md; proposal §5, §8, §9 via `git show origin/main:docs/architecture.md`.
- Produces: the extension-pattern exemplar pages (SSA field managers, clusterRef discovery).

- [ ] **Step 1: Write `camundaoptimize.md`.** Required content: `managementAuthRef` (string → ManagementAuthConfig), `clusterRef`; `webapp {replicas, resources, ...}` and `importer {replicas, resources, ...}` deployments; optional `monitoring.serviceMonitor`; behavior: SSA-patches `spec.zeebe.extraEnv` on the referenced CamundaCluster to enable the legacy Zeebe exporter with fixed prefix `zeebe-record` (own field manager; verify the 8.9 exporter default state), resolves ES via the cluster's `storageRef`, configures importer with matching prefix, deploys webapp + importer with `camunda.io/cluster` labels; reads ES directly, never the cluster API; conditions `Ready` etc.
- [ ] **Step 2: Write `camundamanagementcluster.md`.** Required content: cluster-scoped, once per platform; `targetNamespace` (default `<name>-camunda`); `keycloakDbRef`/`identityDbRef`/`webModelerDbRef` (strings → DatabaseConfig); `keycloak {replicas, resources}` — creates a `Keycloak` CR reconciled by the external Keycloak Operator (external prerequisite, like ECK); `identity`, `console {enabled, ...}`, `webModeler {enabled, ..., mail {fromAddress, smtpHost}}`; optional `auth` override of platform defaults; outputs a ManagementAuthConfig; Console discovers clusters via self-registration (no cluster refs here); conditions `KeycloakReady`/`IdentityReady`/`ConsoleReady`/`WebModelerReady`/`Ready`.
- [ ] **Step 3: Write `pvcautoresize.md`.** Required content: `clusterRef`; `zeebe {storageLimit, threshold, increase}` and `elasticsearch {storageLimit, threshold, increase}`; behavior: discovers PVCs by `camunda.io/cluster` labels, SSA-patches `resize.topolvm.io/storage_limit|threshold|increase` annotations with its own field manager (StatefulSet PVC templates are immutable — live PVC patching is the mechanism); external prerequisite: topolvm pvc-autoresizer; deviation note: always created explicitly, never by presets.
- [ ] **Step 4: Verify claims** — Optimize 8.9 importer/exporter reality, management-plane component set and their database needs (Keycloak, Identity, Web Modeler versions for 8.9) against camunda-docs MCP + camunda/camunda source (`optimize/`, `identity/` trees). Record deviations.
- [ ] **Step 5: Nav entries + `make docs-build`** — strict pass.
- [ ] **Step 6: Template conformance self-check.**
- [ ] **Step 7: Commit and open PR** — `docs: add management and extension CRD pages (#7)`, body `Towards #7`.

---

### Task 7: Coherence review and integration PR (orchestrator, after all sub-PRs merged)

**Files:**
- Modify (as review findings require): any `docs/crds/*.md`, `docs/architecture.md`, `mkdocs.yml`
- Delete (final commit before integration merge): `docs/superpowers/plans/2026-07-31-crd-docs-plan.md`, the state file (spec stays)

**Interfaces:**
- Consumes: all merged sub-PRs on `feature/crd-docs`.
- Produces: the integration PR `feature/crd-docs` → `main`.

- [ ] **Step 1: Full-set coherence review** per feature-dev-workflow:maintaining-architectural-coherence + reviewing-feature-progress: reference-field shapes match the Conventions table across all 19 pages; condition names/reasons consistent; shared vocabulary used; every cross-link resolves; each page's Relationships section is bidirectional (if A links B, B links A).
- [ ] **Step 2: Deviation audit** — grep the proposal's core sections (`git show origin/main:docs/architecture.md`) per CRD; confirm every behavior is either documented or listed as a recorded deviation in the page or the spec's Scope Decisions.
- [ ] **Step 3: `make docs-build` strict + `make docs-serve`** visual pass over nav, diagrams, tables.
- [ ] **Step 4: Fix findings, commit** directly on `feature/crd-docs` (`docs: coherence fixes across CRD pages (#1)`).
- [ ] **Step 5: Open integration PR** `feature/crd-docs` → `main`, body `Closes #1`, summarizing the doc set and the six recorded scope decisions; run the review loop per feature-dev-workflow:copilot-review-loop if enabled.
