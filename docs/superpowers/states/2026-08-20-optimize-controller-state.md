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
| #115  | feat/optimize-controller--api-types      | .claude/worktrees/optimize-controller--api-types   | → feat/optimize-controller | in-progress |
| #116  | feat/optimize-controller--reconciler     | .claude/worktrees/optimize-controller--reconciler  | → feat/optimize-controller | not-started |
| #117  | test/optimize-controller--data-flow-e2e  | .claude/worktrees/optimize-controller--data-flow-e2e | → feat/optimize-controller | not-started |

## Contracts

None — the PRs are sequential; each consumes the merged result of the one before it.

## Bubble-up log

- _No concerns yet._

## Pending snapshot

1. Start Phase 1 (#115) per the plan (Tasks 1-3), branch `pr/optimize-api` off `feat/optimize-controller`; skill: `feature-dev-workflow:developing-a-feature`.
2. Token note: the user runs close to the Fable limit — dispatch every implementation subagent with an explicit model override (`opus`; `sonnet`/`haiku` for mechanical work), never a fork.

## Resume checklist

For a fresh Claude session resuming this work: invoke `feature-dev-workflow:resuming-a-feature` — it executes the steps below, routes by the `status:` frontmatter, and works the `## Pending snapshot`. Fallback if that skill is unavailable:

1. Read this state file in full.
2. Read the plan at the path in the `plan:` frontmatter.
3. Read the spec at the path in the `spec:` frontmatter.
4. Verify each open PR's actual state via `gh pr view <num>`.
5. For each `in-progress` or `draft` row, check the worktree with `git -C <path> status -sb`.
6. Re-dispatch subagents as needed per `feature-dev-workflow:developing-a-feature`, or work the `## Pending snapshot`.
