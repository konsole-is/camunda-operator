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
status: foundational-wave
---

# Contract-CRD validation controllers — orchestration state

## Phases

- **Phase 1 (foundational)** — `#18` (types, schema validation, shared helpers, wiring)
- **Phase 2 (consumers, parallel)** — `#19`, `#20`, `#21`, `#22`, `#23` (one validation controller each)
- **Phase 3 (integration)** — integration PR `feature/batch-a-contract-controllers` → `main`, `Closes #17`

## PRs / worktrees

| Issue | Branch | Worktree path | PR (→ base) | Status |
| --- | --- | --- | --- | --- |
| #18 | batch-a/foundations | .claude/worktrees/batch-a-contract-controllers--foundations | → feature/batch-a-contract-controllers | in-progress |
| #19 | batch-a/databaseserverconfig | .claude/worktrees/batch-a-contract-controllers--databaseserverconfig | → feature/batch-a-contract-controllers | not-started |
| #20 | batch-a/databaseconfig | .claude/worktrees/batch-a-contract-controllers--databaseconfig | → feature/batch-a-contract-controllers | not-started |
| #21 | batch-a/secondarystorageconfig | .claude/worktrees/batch-a-contract-controllers--secondarystorageconfig | → feature/batch-a-contract-controllers | not-started |
| #22 | batch-a/objectstorageconfig | .claude/worktrees/batch-a-contract-controllers--objectstorageconfig | → feature/batch-a-contract-controllers | not-started |
| #23 | batch-a/managementauthconfig | .claude/worktrees/batch-a-contract-controllers--managementauthconfig | → feature/batch-a-contract-controllers | not-started |

## Contracts

All contracts are realized by the foundations PR (#18) merging into the feature branch before the five controller PRs branch off. Shapes are in the plan's `## Contracts` table.

| Name | Realization | Realized in | Status |
| --- | --- | --- | --- |
| shared-ref-types | foundations PR | #18 PR | pending |
| contract-specs | foundations PR | #18 PR | pending |
| conditions-api | foundations PR | #18 PR | pending |
| secretref-api | foundations PR | #18 PR | pending |
| refindex-api | foundations PR | #18 PR | pending |
| suite-manager | foundations PR | #18 PR | pending |

## Bubble-up log

- _No concerns yet._

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
