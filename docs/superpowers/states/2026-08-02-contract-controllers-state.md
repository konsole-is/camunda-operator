---
feature: batch-a-contract-controllers
spec: docs/superpowers/specs/2026-08-02-contract-controllers-design.md
plan: docs/superpowers/plans/2026-08-02-contract-controllers-plan.md
tracking_issue: #17
feature_branch: feature/batch-a-contract-controllers
feature_worktree: .claude/worktrees/batch-a-contract-controllers
sub_pr_approval: autonomous
sub_pr_review_loop: on
sub_pr_target: feature-branch
integration_pr:
status: consumer-wave
---

# Contract-CRD validation controllers — orchestration state

## Phases

- **Phase 1 (foundational)** — `#18` (types, schema validation, shared helpers, wiring)
- **Phase 2 (consumers, parallel)** — `#19`, `#20`, `#21`, `#22`, `#23` (one validation controller each)
- **Phase 3 (integration)** — integration PR `feature/batch-a-contract-controllers` → `main`, `Closes #17`

## PRs / worktrees

| Issue | Branch | Worktree path | PR (→ base) | Status |
| --- | --- | --- | --- | --- |
| #18 | batch-a/foundations | .claude/worktrees/batch-a-contract-controllers--foundations | #24 → feature/batch-a-contract-controllers | self-merged |
| #19 | batch-a/databaseserverconfig | .claude/worktrees/batch-a-contract-controllers--databaseserverconfig | #26 → feature/batch-a-contract-controllers | ready |
| #20 | batch-a/databaseconfig | .claude/worktrees/batch-a-contract-controllers--databaseconfig | #29 → feature/batch-a-contract-controllers | ready |
| #21 | batch-a/secondarystorageconfig | .claude/worktrees/batch-a-contract-controllers--secondarystorageconfig | #28 → feature/batch-a-contract-controllers | ready |
| #22 | batch-a/objectstorageconfig | .claude/worktrees/batch-a-contract-controllers--objectstorageconfig | #25 → feature/batch-a-contract-controllers | ready |
| #23 | batch-a/managementauthconfig | .claude/worktrees/batch-a-contract-controllers--managementauthconfig | #27 → feature/batch-a-contract-controllers | ready |

## Contracts

All contracts are realized by the foundations PR (#18) merging into the feature branch before the five controller PRs branch off. Shapes are in the plan's `## Contracts` table.

| Name | Realization | Realized in | Status |
| --- | --- | --- | --- |
| shared-ref-types | foundations PR | #24 (merged 0615da3) | locked |
| contract-specs | foundations PR | #24 (merged 0615da3) | locked |
| conditions-api | foundations PR | #24 (merged 0615da3) | locked |
| secretref-api | foundations PR | #24 (merged 0615da3) | locked |
| refindex-api | foundations PR | #24 (merged 0615da3) | locked |
| suite-manager | foundations PR | #24 (merged 0615da3) | locked |

## Bubble-up log

- **2026-08-03 — coherence note (from #20 implementer):** IndexField errors are returned unwrapped across controllers to match the plan's Task 11 template (how-we-write-go prefers %w wrapping). Consistent across all five PRs; assess at the wave checkpoint whether to align on wrapping in a follow-up or record as deliberate.
- **2026-08-03 — role.yaml overlap (from #19 implementer):** config/rbac/role.yaml gains the core-group secrets rule; #20 and #23 regenerate the identical rule. Identical content merges cleanly; verify role.yaml sanity at the wave checkpoint. No agent action needed.
- **2026-08-02 — Copilot pushback on PR #24 (review-loop, fan-out mode):** Copilot flagged `new(int32(N))` in the schema tests as invalid syntax; rejected — Go 1.26's generalized `new(expr)` is valid, branch compiles and CI Tests passed on that commit. Replied and resolved. Relevant to #19–#23: Copilot may repeat this on any PR using `new(expr)`; same pushback applies.
- **2026-08-02 — CI break on PR #24 (E2E):** Dockerfile builder image `golang:1.25` vs go.mod `go 1.26.0` (raised by the ocf setup commit) broke `make docker-build` in the e2e job. Fixed on the feature branch (`8cb89b2`), merged forward into batch-a/foundations. No consumer action needed — Phase 2 branches will include the fix.
- **2026-08-02 — propagate to #19–#23 (from #18 implementer):** (1) the five `internal/controller/<kind>_controller_test.go` files already exist as minimal valid-fixture smoke tests — controller PRs replace their own file, never create blind; (2) the `valid<Kind>()` fixture helpers live in the sibling `<kind>_schema_test.go` files — reuse them in reconciliation specs; (3) `conditions.PatchReady` internally uses controller-runtime v0.24 `Status().Apply(...)` (the plan's `Status().Patch(..., client.Apply)` is deprecated and fails staticcheck) — exported signature unchanged; (4) testify is now a direct dependency; (5) `corev1` → `v1` import-alias rename is already done in the five controller files, `cmd/main.go`, and `suite_test.go`; (6) shell note: the permission guard misfires on `cd <worktree> && git ...` compound commands — use `git -C <worktree-path> ...`. Propagation path: baked into the Phase 2 dispatch prompts (no consumer agents running yet).

## Pending snapshot

1. Hand off to `feature-dev-workflow:developing-a-feature`: dispatch Phase 1 (#18, plan Tasks 1–10) on branch `batch-a/foundations` from the feature branch tip.
2. **Gate:** after the #18 sub-PR is ready, pause for user review before self-merging — it locks every convention the fan-out inherits (plan's review checkpoint).
3. After #18 self-merges into the feature branch, fan out Phase 2 in parallel: #19–#23 (plan Tasks 11–15), each in its own worktree/branch per the PR table.
4. Phase 3: integration PR to `main` (`Closes #17`), CI green, docs drift check (plan Task 16); after merge, delete the plan and this state file in the orchestrator's final commit.

Notes for resumers: the feature branch already carries two commits beyond `origin/main` — the ocf v0.18.1 tooling setup (go.mod tool directive + `.claude/settings.json` plugin pin) and the spec. The ocf Claude plugin and `go tool ocf` are set up for later batches; this batch's reconcilers deliberately do not import ocf (see spec "Framework posture"). The Test Chart CI workflow is known-red on main and owned by a separate session — ignore it.

## Resume checklist

For a fresh Claude session resuming this work: invoke `feature-dev-workflow:resuming-a-feature` — it executes the steps below, routes by the `status:` frontmatter, and works the `## Pending snapshot`. Fallback if that skill is unavailable:

1. Read this state file in full.
2. Read the plan at the path in the `plan:` frontmatter.
3. Read the spec at the path in the `spec:` frontmatter.
4. Verify each open PR's actual state via `gh pr view <num>`.
5. For each `in-progress` or `draft` row, `cd` to the worktree path and check `git status -sb` — an `ahead` count in the header means unpushed commits, and no upstream in the header means the branch was never pushed at all.
6. Re-dispatch subagents as needed per `feature-dev-workflow:developing-a-feature` (parallel waves still in flight; the orchestrator watch loop continues), or work the `## Pending snapshot` when development is past.
