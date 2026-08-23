---
feature: version-downgrade-guard
spec: docs/superpowers/specs/2026-08-22-version-downgrade-guard-design.md
plan: docs/superpowers/plans/2026-08-22-version-downgrade-guard-plan.md
tracking_issue: #168
integration_pr: #184
status: review
---

# Version downgrade guard — orchestration state

## Phases

- **Phase 1 (single PR)** — `#168`, seven sequential tasks in the plan: api reason, pure rule, controller guard, restore annotation, docs, e2e spec, gates and PR.

## PRs / worktrees

| Issue | Branch                         | Worktree path                                  | PR (→ base)   | Status      |
| ----- | ------------------------------ | ---------------------------------------------- | ------------- | ----------- |
| #168  | feat/version-downgrade-guard   | .claude/worktrees/version-downgrade-guard-168  | #184 → main | ready |

## Contracts

| Name | Realization | Realized in | Status |
| ---- | ----------- | ----------- | ------ |
| none | single PR, sequential tasks | n/a | n/a |

## Bubble-up log

- _No concerns yet._

## Pending snapshot

- Tasks 1–6 implemented and reviewed (SDD ledger in the worktree's `.superpowers/sdd/`); final whole-branch review clean after one fix wave; all gates green on `9c57674`. #168 reconciled (decision comment + body).
- Open the PR from `feat/version-downgrade-guard` to `main` with `feature-dev-workflow:opening-a-pull-request`, then run `feature-dev-workflow:copilot-review-loop` to clean (interactive context; the user asked for autonomy through the loop).
- Tear down plan and state after CI is green on the PR; keep the spec (the repo has no `docs/adrs/`). The merge to main is the user's.

## Resume checklist

For a fresh Claude session resuming this work: invoke `feature-dev-workflow:resuming-a-feature` — it executes the steps below, routes by the `status:` frontmatter, and works the `## Pending snapshot`. Fallback if that skill is unavailable:

1. Read this state file in full.
2. Read the plan at the path in the `plan:` frontmatter.
3. Read the spec at the path in the `spec:` frontmatter.
4. Verify each open PR's actual state via `gh pr view <num>`.
5. For each `in-progress` or `draft` row, `cd` to the worktree path and check `git status -sb` — an `ahead` count in the header means unpushed commits, and no upstream in the header means the branch was never pushed at all.
6. Work the `## Pending snapshot`.
