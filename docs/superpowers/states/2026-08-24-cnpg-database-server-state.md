---
feature: cnpg-database-server
spec: docs/superpowers/specs/2026-08-24-cnpg-database-server-design.md
plan: docs/superpowers/plans/2026-08-24-cnpg-database-server-plan.md
tracking_issue: #127
feature_branch: feat/cnpg-database-server
feature_worktree: .claude/worktrees/cnpg-database-server
sub_pr_approval: autonomous
sub_pr_review_loop: on
sub_pr_target: feature-branch
integration_pr:
status: foundational-wave
---

# DatabaseServer on CloudNativePG — orchestration state

The user's standing instruction for this feature (2026-08-24): full autonomy through every sub-PR and its review loop; the integration PR to `main` is the user's to open and merge.

## Phases

- **Phase 1 (foundational, parallel)** — `#128` (namespaced chain + system identifier), `#234` (wrappers)
- **Phase 2** — `#235` (DatabaseServer kind)
- **Phase 3** — `#236` (recovery through the contract)
- **Phase 4** — `#237` (e2e + installation docs)

## PRs / worktrees

| Issue | Branch | Worktree path | PR (→ base) | Status |
| --- | --- | --- | --- | --- |
| #128 | fix/cnpg-database-server--namespaced-chain | .claude/worktrees/cnpg-database-server/.claude/worktrees/namespaced-chain | #239 → feat/cnpg-database-server | ready |
| #234 | chore/cnpg-database-server--cnpg-wrappers | .claude/worktrees/cnpg-database-server/.claude/worktrees/cnpg-wrappers | #238 → feat/cnpg-database-server | ready |
| #235 | feat/cnpg-database-server--database-server | .claude/worktrees/cnpg-database-server/.claude/worktrees/database-server | → feat/cnpg-database-server | not-started |
| #236 | feat/cnpg-database-server--contract-recovery | .claude/worktrees/cnpg-database-server/.claude/worktrees/contract-recovery | → feat/cnpg-database-server | not-started |
| #237 | ci/cnpg-database-server--e2e | .claude/worktrees/cnpg-database-server/.claude/worktrees/e2e | → feat/cnpg-database-server | not-started |

## Contracts

| Name | Realization | Realized in | Status |
| --- | --- | --- | --- |
| `namespaced-rdbms-kinds` | stub-on-producer-branch (PR 3 branches after PR 1 merges) | #128 sub-PR | pending |
| `cnpg-wrappers` | stub-on-producer-branch (PR 3 branches after PR 2 merges) | #234 sub-PR | pending |
| `contract-recovery-fields` | data-only (both writers in PR 4) | n/a | locked |

## Bubble-up log

- 2026-08-24 — #234 spec review: the CNPG CRD declares `minimum: 1` on `spec.instances`, so the scale-to-zero suspend in the spec can never apply. Resolution: suspend through declarative hibernation (`cnpg.io/hibernation: "on"` annotation, status from the `cnpg.io/hibernation` condition). Propagated: spec and plan amended; fix routed to the #234 implementer; the #235 dispatch will carry it.

## Pending snapshot

1. Create the two Phase 1 sub-worktrees off `feat/cnpg-database-server` (nested under the feature worktree, see the `nested-sub-worktrees-under-pin` memory) and dispatch #128 and #234 in parallel (`feature-dev-workflow:fanning-out-with-worktrees`).
2. At each ripening: `feature-dev-workflow:copilot-review-loop` to clean, `review` pass, self-merge into `feat/cnpg-database-server`, `gh issue close <n>`, update this table.
3. Phase 2 → 3 → 4 sequentially, each branched from the feature branch after the previous merge.
4. `feature-dev-workflow:reviewing-feature-progress`, then report ready for the user's integration PR. Do not open or merge the integration PR.

## Resume checklist

For a fresh Claude session resuming this work: invoke `feature-dev-workflow:resuming-a-feature` — it executes the steps below, routes by the `status:` frontmatter, and works the `## Pending snapshot`. Fallback if that skill is unavailable:

1. Read this state file in full.
2. Read the plan at the path in the `plan:` frontmatter.
3. Read the spec at the path in the `spec:` frontmatter.
4. Verify each open PR's actual state via `gh pr view <num>`.
5. For each `in-progress` or `draft` row, `cd` to the worktree path and check `git status -sb` — an `ahead` count in the header means unpushed commits, and no upstream in the header means the branch was never pushed at all.
6. Re-dispatch subagents as needed per `feature-dev-workflow:developing-a-feature` (parallel waves still in flight; the orchestrator watch loop continues), or work the `## Pending snapshot` when development is past.
