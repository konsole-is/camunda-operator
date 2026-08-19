---
feature: optimize-controller
spec: docs/superpowers/specs/2026-08-20-optimize-controller-design.md
plan: docs/superpowers/plans/2026-08-20-optimize-controller-plan.md
tracking_issue: #114
feature_branch: feat/optimize-controller
feature_worktree: .claude/worktrees/optimize-controller
sub_pr_approval: autonomous
sub_pr_review_loop: on
sub_pr_target: feature-branch
integration_pr:
status: foundational-wave
---

# CamundaOptimize controller — orchestration state

The user granted full autonomy on 2026-08-20: run review loops on the sub-PRs, self-merge them into the feature branch, and finish with one clean integration PR against main.

## Phases

Strictly sequential; each PR needs the one before it.

- **Phase 1** — `#115` (API types + extraEnv map lists)
- **Phase 2** — `#116` (components + controller)
- **Phase 3** — `#117` (data-flow e2e + docs finalization)

## PRs / worktrees

| Issue | Branch                                   | Worktree path                                      | PR (→ base)                | Status      |
| ----- | ---------------------------------------- | -------------------------------------------------- | -------------------------- | ----------- |
| #115  | feat/optimize-controller--api-types      | .claude/worktrees/optimize-controller--api-types   | #119 → feat/optimize-controller | self-merged |
| #116  | feat/optimize-controller--reconciler     | .claude/worktrees/optimize-controller--reconciler  | → feat/optimize-controller | not-started |
| #117  | test/optimize-controller--data-flow-e2e  | .claude/worktrees/optimize-controller--data-flow-e2e | → feat/optimize-controller | not-started |

## Contracts

None — the PRs are sequential; each consumes the merged result of the one before it.

## Bubble-up log

- 2026-08-20 — PR 1 quality review: `ReasonUnsupportedStorageType` duplicated the backup kinds' `ReasonStorageTypeMismatch`. Resolution: reuse `ReasonStorageTypeMismatch`, promoted to `api/v1/conditions.go`; spec, plan, and docs renamed in PR 1. Propagation: wave-2 dispatch prompt must use `StorageTypeMismatch`; issues #114/#116 bodies reconciled after the fix commit lands.
- 2026-08-20 — PR 1 quality review: docs pages must use the ocf component reason vocabulary (Healthy/Creating/Updating/...), never `Progressing`. Applies to the PR 3 docs finalization too.
- 2026-08-20 — PR 1 implementer notes: fresh worktrees need `make setup-envtest` (and `chmod -R u+w bin` before worktree removal); `make lint` re-runs the golangci-lint install each time and can hit a transient sum.golang.org 404 — `GOSUMDB=off` works around it. Propagate to wave-2/3 dispatch prompts.
- 2026-08-20 — PR 1 deviation accepted: no printcolumns on CamundaOptimize (no long-lived kind has them); the known printcolumns follow-up covers all kinds at once. `spec.backup.dump.extraEnv` (DumpSpec) deliberately stays atomic.

## Pending snapshot

1. Phase 1 done (#119 self-merged as 4f7a302, #115 closed). Start Phase 2 (#116) per the plan (Tasks 4-8); gate its self-merge on the sub-PR's CI checks. Watch the feature-branch CI for 4f7a302; if red, root-cause and fix forward before the wave-2 merge.
2. Token note: the user runs close to the Fable limit — dispatch every implementation subagent with an explicit model override (`opus`; `sonnet`/`haiku` for mechanical work), never a fork.

## Resume checklist

For a fresh Claude session resuming this work: invoke `feature-dev-workflow:resuming-a-feature` — it executes the steps below, routes by the `status:` frontmatter, and works the `## Pending snapshot`. Fallback if that skill is unavailable:

1. Read this state file in full.
2. Read the plan at the path in the `plan:` frontmatter.
3. Read the spec at the path in the `spec:` frontmatter.
4. Verify each open PR's actual state via `gh pr view <num>`.
5. For each `in-progress` or `draft` row, check the worktree with `git -C <path> status -sb`.
6. Re-dispatch subagents as needed per `feature-dev-workflow:developing-a-feature`, or work the `## Pending snapshot`.
