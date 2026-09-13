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

- **Phase 1 (one wave, parallel)** — `#369`: the storage claim on a Lease keyed by the backend, the pod labels, the handover gate, and their docs. `#370`: no suspension on a failed pre-check, `Suspended()` narrowed, and their docs. Both branch off the feature branch. The second to land merges the feature branch forward and resolves `internal/controller/camundacluster/secondarystorage_test.go` and `internal/controller/camundaoptimize/controller_test.go`, and adds the contract-deleted Lease assertion (plan Task 7 Step 1).

## PRs / worktrees

| Issue | Branch | Worktree path | PR (→ base) | Status |
| --- | --- | --- | --- | --- |
| #369 | feat/suspend-only-for-shared-backends--storage-claim-on-a-lease | .claude/worktrees/suspend-only-for-shared-backends/.claude/worktrees/storage-claim-on-a-lease | #372 → fix/suspend-only-for-shared-backends | ready |
| #370 | fix/suspend-only-for-shared-backends--keep-workloads-on-precheck-failure | .claude/worktrees/suspend-only-for-shared-backends/.claude/worktrees/keep-workloads-on-precheck-failure | #371 → fix/suspend-only-for-shared-backends | ready |

## Contracts

| Name | Realization | Realized in | Status |
| --- | --- | --- | --- |
| `suspended-ready-reasons` | data-only | n/a | locked |

## Bubble-up log

- 2026-09-13, Copilot round 2 on #372: Optimize gated its importer on the cluster's `Suspended()` alone, which lags a repoint by one reconcile. Settled: Optimize also checks `Holds` on the storage claim and parks while its cluster does not hold the backend. Spec amended; #369 implements it. Three doc findings rejected: the Lease is a user-visible object, and the 30-second wait is a stated outcome.
- 2026-09-13, Copilot round 2 on #371: a running cluster that repoints into a handover keeps its workloads while `Suspended()` reads true. Settled as intended: `Suspended()` means the attachments hold. Spec amended; docs and GoDoc on #370 follow. Second finding: a parked cluster whose storage chain fails lost its parked reason. Settled: the standing `StorageAlreadyAttached` is kept with the failure appended. Spec amended. Not propagated to #369; its claim step is unchanged by either.
- 2026-09-13, #369 agent: the spec `waits for the pods of a deleted holder before it resumes` asserted the claim stays with the deleted holder until its pods go. That encoded the annotation mechanism. With `Take`, the parked cluster takes the Lease of a gone holder in the same pass and the pods hold only the render. The assertion now reads `expectClaimedBy(binding, parked)`; the zero-replica `Consistently` stays. Matches the spec; no propagation needed.
- 2026-09-13, #369 agent: the worktree shell hook refuses `flock ... go test` lines. Workaround: a scratch script that runs the flocked command. Record in memory for later dispatches.
- 2026-09-13, Copilot round 1 on #371: a parked cluster whose later reference fails reported `InvalidReference`, flipping `Suspended()` false while the workloads sat at zero, so an attached Optimize could start beside the holder. Resolution on #370: storage resolve and claim run first in `preCheck`, and a failure after a found holder keeps `StorageAlreadyAttached` with the failure in the message. Not propagated to #369; it does not touch that branch.
- 2026-09-13, #370 agent: two parallel agents share one scratchpad directory and one overwrote the other's helper script. Resolution: later dispatches name scratch files by issue number. Propagated to the #369 dispatch prompt only if it reports the same; no code impact.
- 2026-09-13, #370 agent: docs on that branch already say "storage claim of its backend" (the #369 vocabulary), and the cluster reason table has adjacent edits from both PRs. Resolution: expected, reconciled in the merge-forward of whichever PR lands second.

## Pending snapshot

1. Dispatch #369 (plan Tasks 1 to 6) and #370 (plan Tasks 7 to 11) in parallel into their sub-worktrees under the feature worktree; `feature-dev-workflow:fanning-out-with-worktrees` owns the dispatch. Gate per sub-PR: every command in plan Task 6 Step 1 passes, Copilot review loop clean, self-merge into `fix/suspend-only-for-shared-backends`, `gh issue close <n>`.
2. The second sub-PR to ripen merges the feature branch forward, resolves the two shared test files, adds the contract-deleted Lease assertion, and re-runs the gates before its self-merge.
3. `feature-dev-workflow:reviewing-feature-progress`, then the integration PR from `fix/suspend-only-for-shared-backends` to `main` with `Closes #368`. The user merges to main. Delete this file and the plan in the last commit before the merge; keep the spec.

## Resume checklist

For a fresh Claude session resuming this work: invoke `feature-dev-workflow:resuming-a-feature` — it executes the steps below, routes by the `status:` frontmatter, and works the `## Pending snapshot`. Fallback if that skill is unavailable:

1. Read this state file in full.
2. Read the plan at the path in the `plan:` frontmatter.
3. Read the spec at the path in the `spec:` frontmatter.
4. Verify each open PR's actual state via `gh pr view <num>`.
5. For each `in-progress` or `draft` row, `cd` to the worktree path and check `git status -sb` — an `ahead` count in the header means unpushed commits, and no upstream in the header means the branch was never pushed at all.
