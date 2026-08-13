---
feature: storage-backend-controllers
spec: docs/superpowers/specs/2026-08-13-storage-backend-controllers-design.md
plan: docs/superpowers/plans/2026-08-13-storage-backend-controllers-plan.md
tracking_issue: #34
feature_branch: feat/storage-backend-controllers
feature_worktree: .claude/worktrees/storage-backend-controllers
sub_pr_approval: autonomous
sub_pr_review_loop: on
sub_pr_target: feature-branch
integration_pr:
status: foundational-wave
---

# Storage backend controllers (Batch B) — orchestration state

## Phases

- **Phase 1 (rework)** — `#35` (binding CRDs to namespaced + `caSecretRef`, plan Tasks 1–2)
- **Phase 2 (foundational)** — `#36` (API types, ocf wrappers, shared helpers, scaffolds; plan Tasks 3–7)
- **Phase 3 (consumers, parallel)** — `#37` (ElasticsearchCluster, Tasks 8–11) ∥ `#38` (Database, Tasks 12–15)
- **Phase 4 (e2e)** — `#39` (Task 16)
- **Phase 5 (integration)** — integration PR `feat/storage-backend-controllers` → `main`, `Closes #34` (Task 17)

## PRs / worktrees

| Issue | Branch | Worktree path | PR (→ base) | Status |
| --- | --- | --- | --- | --- |
| #35 | batch-b/binding-scope | .claude/worktrees/storage-backend-controllers/.claude/worktrees/storage-backend-controllers--binding-scope | #40 → feat/storage-backend-controllers | ready |
| #36 | batch-b/foundations | .claude/worktrees/storage-backend-controllers--foundations | — | not-started |
| #37 | batch-b/elasticsearchcluster | .claude/worktrees/storage-backend-controllers--elasticsearchcluster | — | not-started |
| #38 | batch-b/database | .claude/worktrees/storage-backend-controllers--database | — | not-started |
| #39 | batch-b/e2e | .claude/worktrees/storage-backend-controllers--e2e | — | not-started |

## Contracts

| Name | Realization | Realized in | Status |
| --- | --- | --- | --- |
| `binding-scope` | pre-merge PR #35 | — | pending |
| `eck-wrapper` | foundations PR (#36) | — | pending |
| `binding-wrappers` | foundations PR (#36) | — | pending |
| `credentials-api` | foundations PR (#36) | — | pending |
| `ready-derivation` | foundations PR (#36) | — | pending |
| `api-types` | foundations PR (#36) | — | pending |
| `e2e-flows` | data-only (issue #37/#38 Verification bullets) | n/a | locked |

## Bubble-up log

- **2026-08-13 — worktree path drift (from #35 implementer):** sub-worktrees created with relative paths from the feature worktree nest under `.claude/worktrees/storage-backend-controllers/.claude/worktrees/…`. Row paths record reality; orchestrator creates future sub-worktrees from the main repo root with absolute paths (dispatch prompts must state the verified path from `git worktree list`).
- **2026-08-13 — `CamundaManagementCluster` `*DbRef` anchor (from #35 implementer):** the namespaced-binding refs on that cluster-scoped CR are documented as resolving in its `spec.targetNamespace` (default `<name>-camunda`) — the only coherent anchor. Doc-only decision; no code exists for that controller until Batch D. Revisit there if a different anchor is wanted.
- **2026-08-13 — propagate to #36–#38 (from #35 implementer):** (1) `refindex.SecretKey` renamed to `refindex.NamespacedKey` (keys non-Secret referents too); (2) `valid<Kind>()` fixtures now default `Namespace: "default"`, controller specs override with per-spec random namespaces, `expect*Ready` helpers take `types.NamespacedName`; (3) `elasticsearchcluster.md` doc claims #37 must honor: SSC + credentials Secret owner-ref GC'd (no finalizer), SSC `caSecretRef` → `<name>-es-http-certs-public`/`ca.crt`, file-realm user role `superuser`; (4) `database.md`: bindings land in `spec.targetNamespace` (#38). Propagation path: baked into Phase 2/3 dispatch prompts (no sibling agents running yet).

## Pending snapshot

1. Hand off to `feature-dev-workflow:developing-a-feature`: dispatch Phase 1 (#35, plan Tasks 1–2) on branch `batch-b/binding-scope` from the feature branch tip.
2. After #35 self-merges, dispatch Phase 2 (#36, Tasks 3–7) on `batch-b/foundations` — it locks every API type, wrapper, and helper the consumer wave imports.
3. After #36 self-merges, fan out Phase 3 in parallel: #37 (Tasks 8–11) and #38 (Tasks 12–15), each in its own worktree/branch per the PR table.
4. Phase 4: #39 (Task 16) after both consumers self-merge.
5. Phase 5: integration checkpoint (Task 17), integration PR to `main` (`Closes #34`) left ready for the user — never self-merged; after merge, delete the plan and this state file in the orchestrator's final commit.

Notes for resumers: user directives for this feature — proceed autonomously, review loop on for all sub-PRs, sub-PR self-merge autonomous, integration PR to main is user-merged. ocf skills (`ocf:building-components`, `ocf:custom-resource-wrappers`, `ocf:using-primitives`, `ocf:testing-operators`) and `how-we-write-go` are expected reading for implementers; `go doc` is ground truth over plan snippets. ECK CRDs for envtest come from the module cache, never vendored. The permission sandbox misfires on `cd <worktree> && git ...` compounds — use `git -C <path>`.

## Resume checklist

For a fresh Claude session resuming this work: invoke `feature-dev-workflow:resuming-a-feature` — it executes the steps below, routes by the `status:` frontmatter, and works the `## Pending snapshot`. Fallback if that skill is unavailable:

1. Read this state file in full.
2. Read the plan at the path in the `plan:` frontmatter.
3. Read the spec at the path in the `spec:` frontmatter.
4. Verify each open PR's actual state via `gh pr view <num>`.
5. For each `in-progress` or `draft` row, `cd` to the worktree path and check `git status -sb` — an `ahead` count in the header means unpushed commits, and no upstream in the header means the branch was never pushed at all.
6. Re-dispatch subagents as needed per `feature-dev-workflow:developing-a-feature` (parallel waves still in flight; the orchestrator watch loop continues), or work the `## Pending snapshot` when development is past.
