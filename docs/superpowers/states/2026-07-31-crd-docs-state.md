---
feature: crd-docs
spec: docs/superpowers/specs/2026-07-31-crd-docs-design.md
plan: docs/superpowers/plans/2026-07-31-crd-docs-plan.md
tracking_issue: #1
feature_branch: feature/crd-docs
feature_worktree: .claude/worktrees/crd-docs
sub_pr_approval: autonomous
sub_pr_review_loop: off
sub_pr_target: feature-branch
integration_pr:
status: foundational-wave
---

# CRD documentation foundation — orchestration state

## Phases

- **Phase 1 (foundational)** — `#2` (docs tooling, template, architecture rewrite, CRD index — the conventions everything inherits)
- **Phase 2 (parallel docs batches)** — `#3` (contracts), `#4` (core cluster), `#5` (storage backends), `#6` (backup & restore), `#7` (management & extensions)
- **Phase 3 (review + integration)** — coherence review, deviation audit, integration PR → main (plan Task 7)

## PRs / worktrees

| Issue | Branch | Worktree path | PR (→ base) | Status |
| --- | --- | --- | --- | --- |
| #2 | | | → feature/crd-docs | not-started |
| #3 | | | → feature/crd-docs | not-started |
| #4 | | | → feature/crd-docs | not-started |
| #5 | | | → feature/crd-docs | not-started |
| #6 | | | → feature/crd-docs | not-started |
| #7 | | | → feature/crd-docs | not-started |

## Contracts

| Name | Realization | Realized in | Status |
| --- | --- | --- | --- |
| `crd-doc-template` | pre-merge PR (#2's PR) | | pending |
| `crd-file-paths` | data-only (plan Task 1 inventory table) | n/a | locked |
| `mkdocs-nav-groups` | pre-merge PR (#2's PR) | | pending |
| `api-vocabulary` | data-only (plan Conventions section) | n/a | locked |

## Bubble-up log

- _No concerns yet._

## Pending snapshot

1. Push `feature/crd-docs` to origin (done in planning if this line is followed by a pushed branch — verify with `git status -sb`).
2. Invoke `feature-dev-workflow:developing-a-feature` from the integration worktree `.claude/worktrees/crd-docs`.
3. Phase 1: dispatch issue #2 (plan Task 1) as the first sub-PR; it must self-merge into `feature/crd-docs` before Phase 2 starts.
4. Phase 2: fan out issues #3–#7 (plan Tasks 2–6) in parallel worktrees per `feature-dev-workflow:fanning-out-with-worktrees`; each verifies Camunda 8.9 claims against the camunda-docs MCP and `~/Documents/camunda/camunda`.
5. Phase 3: plan Task 7 — coherence review, deviation audit, integration PR `feature/crd-docs` → `main` with `Closes #1`.

## Resume checklist

For a fresh Claude session resuming this work: invoke `feature-dev-workflow:resuming-a-feature` — it executes the steps below, routes by the `status:` frontmatter, and works the `## Pending snapshot`. Fallback if that skill is unavailable:

1. Read this state file in full.
2. Read the plan at the path in the `plan:` frontmatter.
3. Read the spec at the path in the `spec:` frontmatter.
4. Verify each open PR's actual state via `gh pr view <num>`.
5. For each `in-progress` or `draft` row, `cd` to the worktree path and check `git status -sb` — an `ahead` count in the header means unpushed commits, and no upstream in the header means the branch was never pushed at all.
6. Re-dispatch subagents as needed per `feature-dev-workflow:developing-a-feature` (parallel waves still in flight; the orchestrator watch loop continues), or work the `## Pending snapshot` when development is past.
