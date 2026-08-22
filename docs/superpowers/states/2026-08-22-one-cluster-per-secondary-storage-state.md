---
feature: one-cluster-per-secondary-storage
spec: docs/superpowers/specs/2026-08-22-one-cluster-per-secondary-storage-design.md
plan: docs/superpowers/plans/2026-08-22-one-cluster-per-secondary-storage-plan.md
tracking_issue: #166
status: foundational-wave
---

# One CamundaCluster per secondary storage backend — orchestration state

## Phases

- **Phase 1 (single PR)** — `#166`: Tasks 1 to 6 of the plan, in order, on `fix/one-cluster-per-secondary-storage`.

## PRs / worktrees

| Issue | Branch                                  | Worktree path                                          | PR (→ base)   | Status      |
| ----- | --------------------------------------- | ------------------------------------------------------ | ------------- | ----------- |
| #166  | fix/one-cluster-per-secondary-storage   | .claude/worktrees/one-cluster-per-secondary-storage    | (none) → main | in-progress |

## Contracts

None. Single PR, sequential tasks.

## Bubble-up log

- 2026-08-23 — Task 3 review: the parked-only enqueue of Task 4 could not reach a running holder that must yield (older sibling's chain resolves, same-second tie, older sibling repoints storageRef), so two holders could coexist. Ruling: every event that can move the holder enqueues every cluster (CamundaCluster self-watch → all others; SecondaryStorageConfig and DatabaseConfig watches → all), filtered by `predicate.GenerationChangedPredicate`. Propagated: Task 4 fix round (commit cb34b1d, two extra specs), spec amended (Handover), docs state the other-direction rule. Issue #166 body stays true (its criteria still hold).

## Pending snapshot

1. Execute the plan task by task in the worktree (`superpowers:subagent-driven-development`, or `superpowers:executing-plans` inline). Task 1 (API reason) first; Tasks 3 and 4 carry the envtest specs and need `make setup-envtest`.
2. After Task 6's gates pass, open the PR with `feature-dev-workflow:opening-a-pull-request`: title `fix(camundacluster): one cluster per secondary storage backend`, body `Fixes #166`, name the corrected statements and the out-of-scope item (same-named `ElasticsearchCluster` in two namespaces, to be filed as its own issue).
3. Run the Copilot review loop (`feature-dev-workflow:copilot-review-loop`) until clean; balanced reviewer.
4. On merge: delete this state file and the plan in the last commit; keep the spec. File the out-of-scope issue.

## Resume checklist

For a fresh Claude session resuming this work: invoke `feature-dev-workflow:resuming-a-feature` — it executes the steps below, routes by the `status:` frontmatter, and works the `## Pending snapshot`. Fallback if that skill is unavailable:

1. Read this state file in full.
2. Read the plan at the path in the `plan:` frontmatter.
3. Read the spec at the path in the `spec:` frontmatter.
4. Verify each open PR's actual state via `gh pr view <num>`.
5. For each `in-progress` or `draft` row, `cd` to the worktree path and check `git status -sb` — an `ahead` count in the header means unpushed commits, and no upstream in the header means the branch was never pushed at all.
6. Re-dispatch subagents as needed per `feature-dev-workflow:developing-a-feature`, or work the `## Pending snapshot` when development is past.
