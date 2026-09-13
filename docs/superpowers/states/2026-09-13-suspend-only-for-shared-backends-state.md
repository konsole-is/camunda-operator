---
feature: suspend-only-for-shared-backends
spec: docs/superpowers/specs/2026-09-13-suspend-only-for-shared-backends-design.md
plan: docs/superpowers/plans/2026-09-13-suspend-only-for-shared-backends-plan.md
tracking_issue: #368
feature_branch: fix/suspend-only-for-shared-backends
feature_worktree: .claude/worktrees/suspend-only-for-shared-backends
sub_pr_approval: autonomous
sub_pr_review_loop: on
sub_pr_target: feature-branch
integration_pr:
status: foundational-wave
---

# Suspend only for shared backends — orchestration state

## Phases

- **Phase 1 (foundational)** — `#369`: the storage claim on a Lease keyed by the backend, the pod labels, the handover gate, and their docs.
- **Phase 2 (consumer)** — `#370`: no suspension on a failed pre-check, `Suspended()` narrowed, and their docs. Branches off the feature branch after #369 is self-merged, so the two-writer guard is closed at every commit.

## PRs / worktrees

| Issue | Branch | Worktree path | PR (→ base) | Status |
| --- | --- | --- | --- | --- |
| #369 | feat/storage-claim-on-a-lease | .claude/worktrees/suspend-only-for-shared-backends/.claude/worktrees/storage-claim-on-a-lease | — → fix/suspend-only-for-shared-backends | not-started |
| #370 | fix/keep-workloads-on-precheck-failure | .claude/worktrees/suspend-only-for-shared-backends/.claude/worktrees/keep-workloads-on-precheck-failure | — → fix/suspend-only-for-shared-backends | not-started |

## Contracts

| Name | Realization | Realized in | Status |
| --- | --- | --- | --- |
| `suspended-ready-reasons` | data-only | n/a | locked |

## Bubble-up log

- _No concerns yet._

## Pending snapshot

1. Dispatch #369 (plan Tasks 1 to 6) into its sub-worktree under the feature worktree; `feature-dev-workflow:developing-a-feature` owns the dispatch. Gate: every command in plan Task 6 Step 1 passes, Copilot review loop clean, self-merge into `fix/suspend-only-for-shared-backends`, `gh issue close 369`.
2. Dispatch #370 (plan Tasks 7 to 11) into its sub-worktree branched from the feature branch after step 1. Same gates, self-merge, `gh issue close 370`.
3. `feature-dev-workflow:reviewing-feature-progress`, then the integration PR from `fix/suspend-only-for-shared-backends` to `main` with `Closes #368`. The user merges to main. Delete this file and the plan in the last commit before the merge; keep the spec.

## Resume checklist

For a fresh Claude session resuming this work: invoke `feature-dev-workflow:resuming-a-feature` — it executes the steps below, routes by the `status:` frontmatter, and works the `## Pending snapshot`. Fallback if that skill is unavailable:

1. Read this state file in full.
2. Read the plan at the path in the `plan:` frontmatter.
3. Read the spec at the path in the `spec:` frontmatter.
4. Verify each open PR's actual state via `gh pr view <num>`.
5. For each `in-progress` or `draft` row, `cd` to the worktree path and check `git status -sb` — an `ahead` count in the header means unpushed commits, and no upstream in the header means the branch was never pushed at all.
