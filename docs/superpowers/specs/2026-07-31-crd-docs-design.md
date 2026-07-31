# CRD Documentation Foundation Design

**Date:** 2026-07-31
**Status:** Draft

---

## Overview

Author the authoritative API design for all 19 core-operator CRDs as user-facing documentation,
before any Go types are written. Each `docs/crds/<kind>.md` is the design record its implementation
must satisfy — reviewable as prose now, and the per-controller contract that implementation work is
later fanned out against.

This is a **docs-first** feature: the kubebuilder bootstrap (2026-04-11) produced all 19 CRD kinds
with empty placeholder specs and 17 TODO-stub controllers. The API surface itself — spec fields,
status conditions, validation rules, controller behavior — has not been designed anywhere except
the original architecture proposal, which mixes core, cloud, and SaaS concerns and predates the
project's scope decisions.

## Goals

- One `docs/crds/<kind>.md` per CRD (19 files), each a complete API-level design: purpose,
  controller behavior, annotated spec/status reference, validation rules, relationships, examples.
- `docs/crds/index.md`: CRD inventory table, reconciler dependency graph, and an
  implementation-order section that groups controllers into batches by what-consumes-what —
  the artifact that drives implementation fan-out later.
- Rewrite `docs/architecture.md` scoped to this operator only.
- Publishable docs site via mkdocs-material, mirroring the operator-component-framework setup,
  with `mkdocs build --strict` as a CI-enforceable link/structure gate.

## Non-Goals

- No Go type implementation. Types stay placeholder stubs; implementing them against these docs
  is the next feature.
- No controller logic, no webhook implementation.
- No cloud-operator or saas-operator documentation. Their CRDs appear only as external actors in
  relationship diagrams.
- No versioned docs, no publishing pipeline (GitHub Pages etc.) — buildable locally is enough for
  now.

## Scope Decisions

These bind every CRD doc. Deviations from the original architecture proposal are deliberate and
recorded here.

1. **Supported versions: Camunda 8.9+ only.** The unified orchestration-cluster architecture is
   assumed. No pre-8.9 rendering modes, no per-component legacy images, no version-conditional
   topology. Testing targets minor releases only; features landed in patches are treated as part
   of the next minor.
2. **Clean-slate rewrite.** No migration or adoption path from the existing SaaS operator: no
   `statefulSetName` / `eckResourceName` adoption overrides, no ZeebeCluster concepts.
3. **API group `core.camunda.io/v1`** (flat, single group) — reaffirming the bootstrap decision.
   All examples use it.
4. **Presets are passive data.** `CamundaClusterPreset` and `ElasticsearchClusterPreset` have no
   controllers. The proposal's preset-driven PVCAutoResize creation is dropped: `PVCAutoResize`
   CRs are always created explicitly (by users or a composition layer above). Preset `autoResize`
   fields do not exist. Revisit only if real demand appears.
5. **Contract CRDs keep lightweight controllers** that validate references/secrets and surface
   status conditions. They never provision anything.
6. **Controller inventory is complete**: 17 active controllers + 2 passive kinds. This feature
   adds no controllers.

## Doc Template

Every `docs/crds/<kind>.md` follows the same skeleton:

1. **Purpose** — what it is, one paragraph; who creates it (user, peer controller, composition
   layer above, control plane).
2. **How it works** — controller behavior as numbered reconciliation steps plus a mermaid
   relationship diagram. Passive CRDs describe how consumers resolve and merge them instead.
3. **API reference** — full annotated `core.camunda.io/v1` YAML: every spec field with type,
   required/optional, default, constraints.
4. **Status** — conditions table (type / reason semantics), phases where applicable.
5. **Validation** — admission rules (e.g. PointInTimeRestore's dedicated-server rule, Database's
   name-collision rule).
6. **Relationships** — links to referenced / referencing CRD docs.
7. **Examples** — one minimal and one realistic manifest.

## Architecture Doc Rewrite

`docs/architecture.md` is currently a verbatim copy of the cross-operator proposal (3,668 lines,
including migration strategy, pre-8.9 rendering, and all cloud/SaaS CRD designs). It is replaced
with a core-operator-scoped document:

- Extension model: features attach to workloads; workloads don't know about features.
- The three connection mechanisms: explicit inputs before creation, `clusterRef` + SSA patching
  after creation, contract CRDs for data passing.
- CRD overview graph (this operator's 19 kinds and their relationships).
- Deployment context: this operator is the bottom layer; cloud/saas operators exist above it
  (one paragraph + link, no design detail).
- Support policy: 8.9+, minor versions, no migration path.

The original proposal remains available in git history and in the camunda repo's proposal branch.

## Docs Tooling

Mirror operator-component-framework:

- `mkdocs.yml` — mkdocs-material theme, mermaid via `pymdownx.superfences`, admonitions, tabs,
  `exclude_docs: superpowers/`.
- `requirements-docs.txt` — pinned `mkdocs-material`.
- Makefile targets `docs-serve` (live reload) and `docs-build` (`mkdocs build --strict`).
- Section indexes use `index.md` (mkdocs `navigation.indexes` convention); the CRD index is
  `docs/crds/index.md`.
- Nav: Home (`index.md`) → Architecture (`architecture.md`) → CRDs (one entry per kind, grouped
  by area: Cluster, Storage, Contracts, Backup & Restore, Management & Extensions).

## Verification

Docs are the deliverable, so "done" is defined as:

- `mkdocs build --strict` passes (resolves links, validates nav and structure).
- Template conformance: every CRD doc has all seven sections.
- Every proposal behavior for a CRD either appears in its doc or is covered by a Scope Decision
  above.
- Claims about Camunda 8.9 behavior (unified binary / Spring profiles, backup API surface,
  exporter behavior, management-plane components) are verified against both the `camunda-docs`
  MCP server and the camunda/camunda source checkout at `~/Documents/camunda/camunda` — not
  assumed from the proposal.
- Coherence review before integration: consistent vocabulary, field-naming conventions, and
  condition semantics across all 19 docs.

## Implementation Breakdown

Ships as a multi-PR feature on `feature/crd-docs`: a conventions PR (tooling + architecture
rewrite + CRD index + template), then five parallel docs PRs — Contracts; Core cluster; Storage
backends; Backup & restore; Management & extensions.
