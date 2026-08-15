# camunda-operator

Core Kubernetes operator for the Camunda platform. Manages the orchestration cluster lifecycle,
storage backends, backup/restore, Optimize, and the management plane.

Module: `github.com/konsole-is/camunda-operator`

---

## Operator implementation guidelines

- Use the operator component framework for resource and lifecycle management: https://github.com/sourcehawk/operator-component-framework
- Apply every managed resource with SSA. A CR's own status is written once per reconcile through the ocf `FlushStatus`, never with SSA.
- Write top level controller tests (reconciliation) using ginkgo and gomega, verifying high level concerns of operator logic
- Write low level tests of features of a controller close to the method / file that implements it, covering all edge cases and expected outcomes, preferring
  pure go unit tests using testify.
- If the proposed architecture conflicts with our goals, challenge the idea with followup questions and explanations of why it does not fit.
- The architecture is high level and not all details may have been covered. If you see missing or inaccurate definitions, either ask or implement if clearly obvious.
- Use the `camunda-docs` mcp server rigorously to gather context on subjects required for implementation.
- Follow kubernetes best practices and go conventions

## Before Making Changes

**Read before writing.** Always gather context from the actual source code and documentation before
proposing or making changes. Do not reason from assumptions.

### Documentation to read

- `README.md` — architecture overview and quick start
- `docs/architecture.md` — operator design, CRD ownership, extension model

### Source to read

- `api/v1/` — CRD types and their GoDoc
- `internal/controller/` — reconciler implementations
- `pkg/` — shared utilities and clients

---

## Architecture

This is the bottom layer of a three-operator stack. It has zero knowledge of
`camunda-cloud-operator` or `camunda-saas-operator`.

```
CloudCamundaCluster (camunda-cloud-operator)
└─ CamundaCluster (this operator)
└─ Workloads (Deployments, StatefulSets, Services)
```

**Core principle:** features attach to workloads — workloads don't know about features.
`CamundaCluster` creates labeled workloads. Extensions discover and attach via `clusterRef`
or label selectors. `CamundaCluster` never imports or calls extension controllers.

---

## Rules for Code Changes

### Clarify before implementing

If a prompt is ambiguous in scope, intent, or missing context that would materially affect the
approach — ask before writing any code.

### GoDoc

Every exported symbol has a GoDoc comment. Update it whenever you change the associated
behaviour, signature, or semantics.

### Writing prose: always load `simple-english`

Before you write or edit prose, invoke the `simple-english:simple-english` skill. Then obey it.
Prose includes GoDoc, inline comments, `docs/`, `README.md`, CRD field descriptions, and error and
condition messages. This rule applies to a one-line docstring and to a full document.

### Documentation

Update documentation in the **same response** as the code change — never leave them out of sync.

| Code area changed                       | Documentation to update |
|-----------------------------------------|-------------------------|
| CRD types, spec fields, status model    | `api/v1/` GoDoc         |
| Operator setup, deployment, quick start | `README.md`             |
| Comments in docstrings, methods etc     | entire project          |

### Tests

Uses Ginkgo/Gomega and testify. Do not use `t.Fatal` — use asserts and requires.

Run with:

```bash
go test ./...
```

Lint and format with:

```bash
make all
```

**Tests encode intent, not implementation.** Never modify a test purely to make it pass.

### Hard boundaries — never cross these

- This is a clean slate project. There is no migration, no legacy compatibility layer,
  and no ZeebeCluster. Do not introduce any of these concepts.
- Never create cloud infrastructure resources (IAM, KMS, buckets). That belongs in
  `camunda-cloud-operator`.
