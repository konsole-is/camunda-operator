# camunda-operator

Core Kubernetes operator for the Camunda platform. Manages the orchestration cluster lifecycle,
storage backends, backup/restore, Optimize, and the management plane.

Module: `github.com/konsole-is/camunda-operator`

---

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
