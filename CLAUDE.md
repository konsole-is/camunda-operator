# camunda-operator

Core Kubernetes operator for the Camunda platform. It manages the orchestration cluster lifecycle,
storage backends, backup and restore, Optimize, and the management plane.

Module: `github.com/konsole-is/camunda-operator`

This file is the entry point. It tells you which skill to load for each kind of work. The skills hold
the detail. Do not work from memory when a skill covers the task.

@AGENTS.md

---

## Skills: load before you act

| Before you ... | Load this skill |
| --- | --- |
| Write, change, or review any Go code | `how-we-write-go` |
| Write or edit the user docs: `docs/`, `README.md`, `dist/chart/README.md`, CRD field descriptions | `writing-operator-docs` |
| Write or edit other prose: GoDoc, comments, error and condition messages | `simple-english:simple-english` |
| Write or change Camunda application config (env vars, Spring properties) | `verifying-camunda-app-config` |
| Design or review how a controller or component is structured | `ocf:structuring-operators` |
| Create or change an ocf component: builder, lifecycle, conditions, status, `FlushStatus` | `ocf:building-components` |
| Create or edit resource primitives, mutations, feature gates | `ocf:using-primitives` |
| Wrap a custom resource as an ocf primitive with `pkg/generic` | `ocf:custom-resource-wrappers` |
| Write or update tests for a component: mutation tests, golden snapshots, version matrix | `ocf:testing-operators` |
| Start a feature, plan it, or split it into PRs | `feature-dev-workflow:planning-a-feature`, then `feature-dev-workflow:developing-a-feature` |
| Open or edit a pull request | `feature-dev-workflow:opening-a-pull-request` |
| Say that work is complete | `superpowers:verification-before-completion` |

The operator uses the operator component framework (ocf):
https://github.com/sourcehawk/operator-component-framework. The `ocf:*` skills come from that
repository through the plugin in `.claude/settings.json`. The `feature-dev-workflow:*` and
`simple-english` skills come from their plugins the same way. `writing-operator-docs` loads both.

Use the `camunda-docs` MCP server for every question about Camunda behavior. Do not answer from memory.

---

## Read before you write

Get context from the code and the docs before you propose or make a change. Do not reason from
assumptions.

- `README.md`: architecture overview and quick start
- `docs/architecture.md`: operator design, CRD ownership, extension model
- `docs/crds/`: design docs for each CRD
- `api/v1/`: CRD types and their GoDoc
- `internal/controller/`: reconcilers
- `pkg/`: shared utilities and clients

If a prompt is not clear in scope or intent, ask before you write code.

If a proposed design conflicts with the goals in `docs/architecture.md`, say so. Ask questions and
explain why the design does not fit. The design docs are guidelines. If the code finds a better shape,
change the doc in the same change.

---

## Architecture

This operator is the bottom layer of the operator stack. It has no knowledge of
`camunda-cloud-operator`.

```
CloudCamundaCluster (camunda-cloud-operator)
└─ CamundaCluster (this operator)
   └─ Workloads (Deployments, StatefulSets, Services)
```

Core principle: features attach to workloads. Workloads do not know about features. `CamundaCluster`
creates labeled workloads. Extensions find them and attach through `clusterRef` or label selectors.
`CamundaCluster` never imports or calls an extension controller.

---

## Rules that are not in a skill

### Resources and status

- Apply every managed resource with server-side apply (SSA).
- Write the status of a CR once per reconcile through the ocf `FlushStatus`. Never write status with SSA.

### Tests

- Write top-level controller tests with Ginkgo and Gomega. They cover the reconciliation and the
  high-level behavior of the operator.
- Write low-level tests next to the file that holds the feature. Cover each edge case and each expected
  result. Prefer pure Go unit tests with testify.
- Do not use `t.Fatal`. Use `assert` and `require`.
- Tests encode intent, not implementation. Never change a test only to make it pass.

### GoDoc and docs

- Each exported symbol has a GoDoc comment. Update it when you change the behavior, the signature, or
  the meaning.
- Update the docs in the same response as the code change:

| Code area changed | Docs to update |
| --- | --- |
| CRD types, spec fields, status model | `api/v1/` GoDoc and `docs/crds/` |
| Operator setup, deployment, quick start | `README.md` |
| Behavior described in a comment | The comment, wherever it is |

### Hard boundaries

- This is a clean-slate project. There is no migration, no legacy compatibility layer, and no
  ZeebeCluster. Do not add any of these.
- Never create cloud infrastructure resources (IAM, KMS, buckets). That work belongs to
  `camunda-cloud-operator`.

---

## Commands

Run every gate below before you open a pull request. Each one catches something the
others do not.

```bash
make setup-envtest          # writes KUBEBUILDER_ASSETS; every envtest suite fails without it
go test ./...               # the root module only
go -C api test ./...        # ./... never crosses a module boundary
make lint                   # both modules, expect 0 issues
make lint-renovate          # renovate.json5 against the validator of RENOVATE_VERSION; needs npx
make manifests generate     # then `git status --porcelain config api` prints nothing
go vet -tags=e2e ./test/e2e/  # go test ./... never compiles this package
mkdocs build --strict       # catches a broken link or a missing nav entry
```

Two traps that cost time before:

- `make all` builds. It does not lint. Run `make lint` yourself.
- `test/e2e` sits behind the `e2e` build tag, so `go test ./...` never compiles it. A moved
  or renamed symbol breaks CI while every other gate stays green. `go vet -tags=e2e` is the
  cheap check.
