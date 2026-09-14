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
status: review
---

# Suspend only for shared backends — orchestration state

## Phases

- **Phase 1 (one wave, parallel)** — `#369`: the storage claim on a Lease keyed by the backend, the pod labels, the handover gate, and their docs. `#370`: no suspension on a failed pre-check, `Suspended()` narrowed, and their docs. Both branch off the feature branch. The second to land merges the feature branch forward and resolves `internal/controller/camundacluster/secondarystorage_test.go` and `internal/controller/camundaoptimize/controller_test.go`, and adds the contract-deleted Lease assertion (plan Task 7 Step 1).

## PRs / worktrees

| Issue | Branch | Worktree path | PR (→ base) | Status |
| --- | --- | --- | --- | --- |
| #369 | feat/suspend-only-for-shared-backends--storage-claim-on-a-lease | .claude/worktrees/suspend-only-for-shared-backends/.claude/worktrees/storage-claim-on-a-lease | #372 → fix/suspend-only-for-shared-backends | self-merged as a622870 (squash of d98ffe1) on 2026-09-14 05:24 UTC; #369 closed. Copilot gave no review at the final head after two waits, so the loop ended on its last round and the two orchestrator review passes stand. |
| #370 | fix/suspend-only-for-shared-backends--keep-workloads-on-precheck-failure | .claude/worktrees/suspend-only-for-shared-backends/.claude/worktrees/keep-workloads-on-precheck-failure | #371 → fix/suspend-only-for-shared-backends | self-merged as 51fa307 (squash of b77ed89) on 2026-09-14 08:55 UTC; #370 closed. Copilot loop ended clean at b77ed89: every finding applied or declined with a reason. |

## Contracts

| Name | Realization | Realized in | Status |
| --- | --- | --- | --- |
| `suspended-ready-reasons` | data-only | n/a | locked |

## Bubble-up log

- 2026-09-14, follow-ups filed at the user's request: #373 (read-once suspension state), #374 (Optimize Reconcile harness), #375 (schedule skips a not-ready cluster), #376 (restore invisible to the handover gate), #377 (docs sentence-length sweep). The user merged a minio fix to main for CI; merge origin/main into the feature branch after #371 lands, before the integration PR.
- 2026-09-14, review of the merged head d756a64: the create refusal was still a pre-check failure on the merged branch, so a running cluster could serve while `Suspended()` read true; it renders at zero now. The hold path is read-only and removes a stale suspension condition; a stop stops what it read. Spec amended; #370 implements before the merge.
- 2026-09-14, merge-forward of #371 onto a622870: seven files conflicted. Kept #372's `reportFailedPreCheck` structure with #370's behavior inside it; the old `suspendWorkloads` has no caller. On the merged branch a stop on the Optimize failure path can follow a claim wait as well as a suspension, so the note, the condition message, and the event name which (`stopReason`), the falsehood #372's `StorageClaimAwaited` exists to avoid. The contract-deleted spec now asserts the Lease survives the contract and a third cluster on the same address parks. The vocabulary sweep landed. The Optimize page and the two backup pages read one vocabulary.
- 2026-09-14 04:10 UTC, #372: Copilot produced no round-15 review at 80012e3 after two ten-minute waits (request at 03:36, re-trigger at 03:52). Loop stopped at this head per the review-loop rule. One fresh request follows the agent's next push; if that also yields nothing after two waits, the Copilot loop on #372 ends on its last clean-with-reasons round and the two orchestrator review passes stand as the review.
- 2026-09-14, final pass of #371 at eba7e57: the resume event tied to a clean apply was lost for good on a failed apply, so both suspension events now mark the decision; the kept-at-zero path re-reads a mid-drain workload; endpoints clear only for a process that is at zero. Spec amended; #370 implements. Not propagated to #369.
- 2026-09-14, last pass of #372 at 80012e3: the release keeps its own post-apply pod list on purpose; a pre-check list would miss a pod created between the pre-check and the apply, which is the window the post-apply release closes. The GoDoc now says so. Twelve over-limit sentences on the two reference pages predate this feature; listed by the agent, left for the docs sweep of the merge-forward or a follow-up.
- 2026-09-14, final pass of #372 at e7080fc: a pre-check failure before the claim step released the cluster's live backend once its pods were gone. Settled: no release unless the claim step ran on that pass; the refused-downgrade path releases; a suspended cluster skips the handover gate; `WaitingForHandover` covers the create refusal; the finalizer is added below the pause guard. Spec amended; #369 implements.
- 2026-09-14, final review of #372: three protocol changes and one reversed non-goal. A claimant creates a free Lease only when no pod of another cluster writes the backend (`TakeUnclaimed` rule), so a hand-deleted Lease returns to the running holder; the controller watches the claim Leases; a backend is released after the render was applied, not in the pre-check; and the cluster carries a finalizer that releases its Leases, because a Lease per deleted cluster is an unbounded leak. Spec amended; #369 implements. Not propagated to #370.
- 2026-09-14, Copilot round 11 on #372: the parked and repoint paths released the old backend's claim before the old pods were gone, so a claimant's one-time scan could miss a pod mid-recreation. Settled: a cluster releases a backend only when none of its pods still carries its claim, the same signal the handover gate uses, and requeues while it holds one back. Spec amended; #369 implements.
- 2026-09-13, orchestrator review of #372: the repository docs skill lists a Lease as content that stays off a user page, so the three Copilot rounds I declined on that point were right; #369 rewrites the two pages. Follow-up candidate, not in scope: the pods of a LogicalRestoreRDBMS Job write the backend but carry no storage-claim label, so a cluster deleted mid-restore lets its successor start beside the restore; the old gate had the same gap. File it after the integration PR merges.
- 2026-09-13, orchestrator review of #371: an Optimize whose own check fails kept importing while its cluster was suspended, which the Elasticsearch restore relies on not happening; and an Optimize whose cluster was deleted kept pods that block the backend's handover forever. Settled: Optimize follows the cluster's suspension on its own failed check, and releases its workloads when its cluster is gone. Explicit suspend on the cluster stages `Suspended` per process. Spec amended; #370 implements. Not propagated to #369.
- 2026-09-13, Copilot round 4 on #371: `spec.suspend` set while a reference check fails left the workloads running. Settled: an explicit suspend scales the owned workloads to zero on the failure branch, with no condition faking. Spec amended; #370 implements it.
- 2026-09-13, Copilot round 3 on #371: the parked-memory rule and the running-repoint nuance kept breeding contradictions (four findings). Settled: `WaitingForHandover` renders at zero like a parked cluster (#369 implements it), and a failed reference on a parked cluster reports the failure with no memory (#370 reverts four commits). The Optimize claim gate on #372 covers the parked case. Spec amended in both places. Earlier bubble-up entries on the two settled edges are superseded by this one.
- 2026-09-13, Copilot round 2 on #372: Optimize gated its importer on the cluster's `Suspended()` alone, which lags a repoint by one reconcile. Settled: Optimize also checks `Holds` on the storage claim and parks while its cluster does not hold the backend. Spec amended; #369 implements it. Three doc findings rejected: the Lease is a user-visible object, and the 30-second wait is a stated outcome.
- 2026-09-13, Copilot round 2 on #371: a running cluster that repoints into a handover keeps its workloads while `Suspended()` reads true. Settled as intended: `Suspended()` means the attachments hold. Spec amended; docs and GoDoc on #370 follow. Second finding: a parked cluster whose storage chain fails lost its parked reason. Settled: the standing `StorageAlreadyAttached` is kept with the failure appended. Spec amended. Not propagated to #369; its claim step is unchanged by either.
- 2026-09-13, #369 agent: the spec `waits for the pods of a deleted holder before it resumes` asserted the claim stays with the deleted holder until its pods go. That encoded the annotation mechanism. With `Take`, the parked cluster takes the Lease of a gone holder in the same pass and the pods hold only the render. The assertion now reads `expectClaimedBy(binding, parked)`; the zero-replica `Consistently` stays. Matches the spec; no propagation needed.
- 2026-09-13, #369 agent: the worktree shell hook refuses `flock ... go test` lines. Workaround: a scratch script that runs the flocked command. Record in memory for later dispatches.
- 2026-09-13, Copilot round 1 on #371: a parked cluster whose later reference fails reported `InvalidReference`, flipping `Suspended()` false while the workloads sat at zero, so an attached Optimize could start beside the holder. Resolution on #370: storage resolve and claim run first in `preCheck`, and a failure after a found holder keeps `StorageAlreadyAttached` with the failure in the message. Not propagated to #369; it does not touch that branch.
- 2026-09-13, #370 agent: two parallel agents share one scratchpad directory and one overwrote the other's helper script. Resolution: later dispatches name scratch files by issue number. Propagated to the #369 dispatch prompt only if it reports the same; no code impact.
- 2026-09-13, #370 agent: docs on that branch already say "storage claim of its backend" (the #369 vocabulary), and the cluster reason table has adjacent edits from both PRs. Resolution: expected, reconciled in the merge-forward of whichever PR lands second.

## Pending snapshot

1. Done: both sub-PRs self-merged (#372 as a622870, #371 as 51fa307), #369 and #370 closed, origin/main merged into the feature branch as af55f59 for the minio CI fix.
2. Running: every gate on the merged feature branch (`scratchpad/gates-feature.sh`). Then `feature-dev-workflow:reviewing-feature-progress` at the wave boundary, then the integration PR from `fix/suspend-only-for-shared-backends` to `main` with `Closes #368`; the Copilot auto-review fires on it, budget one fix round; teardown of the plan and this file once CI is green; the user merges. Superseded text of the earlier steps: #371 resolves the two shared test files and the pre-check failure branch of `internal/controller/camundacluster/controller.go` (#369 stages `WaitingForHandover` from the render path, #370 adds the explicit-suspend scale on the failure branch), adds the contract-deleted Lease assertion, fixes the `Suspended` GoDoc in `pkg/components/camundaoptimize/input.go:37` (it names one operator-driven state and says "storage contract"), and re-runs the gates before its self-merge. Follow-up candidates, not in scope: `processConditions` in `internal/controller/camundacluster/explicitsuspend.go` duplicates the component-to-condition pairing of `Process.ConditionType`; an exported accessor in `pkg/components/camundacluster` would remove the drift. The Optimize suspension events are carried by two predicates over conditions read by two paths (`wasSuspending`, `followsSuspendedCluster`); a small suspension-state value read once per reconcile would replace the pair if that code keeps moving; the #370 agent notes that gating the resume event on a clean apply can lose it rather than defer it once ocf components restage their own conditions, which that value would also close. A Reconcile-level harness for the Optimize controller that passes the whole pre-check over a fake client would make an apply failure testable; today only the early-failing pre-check is cheap to drive. A `BackupSchedule` now creates a backup for a cluster whose reference check fails, which parks in `Progressing` and blocks later triggers until a human fixes the reference; before #319 it did the same, and #319 hid it by accident. A skip while the cluster's `Ready` is False, with its own `TriggerSkipped` reason, is the follow-up.
3. `feature-dev-workflow:reviewing-feature-progress`, then the integration PR from `fix/suspend-only-for-shared-backends` to `main` with `Closes #368`. The user merges to main. Delete this file and the plan in the last commit before the merge; keep the spec.

## Resume checklist

For a fresh Claude session resuming this work: invoke `feature-dev-workflow:resuming-a-feature` — it executes the steps below, routes by the `status:` frontmatter, and works the `## Pending snapshot`. Fallback if that skill is unavailable:

1. Read this state file in full.
2. Read the plan at the path in the `plan:` frontmatter.
3. Read the spec at the path in the `spec:` frontmatter.
4. Verify each open PR's actual state via `gh pr view <num>`.
5. For each `in-progress` or `draft` row, `cd` to the worktree path and check `git status -sb` — an `ahead` count in the header means unpushed commits, and no upstream in the header means the branch was never pushed at all.
