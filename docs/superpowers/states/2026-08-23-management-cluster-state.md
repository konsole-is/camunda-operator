---
feature: management-cluster
spec: docs/superpowers/specs/2026-08-23-management-cluster-design.md
plan: docs/superpowers/plans/2026-08-23-management-cluster-plan.md
tracking_issue: #185
feature_branch: feat/management-cluster
feature_worktree: .claude/worktrees/management-cluster
sub_pr_approval: autonomous
sub_pr_review_loop: on
sub_pr_target: feature-branch
integration_pr:
status: foundational-wave
---

# Management plane — orchestration state

The user granted full autonomy on 2026-08-23: file issues, write the plan, implement, review-loop every sub-PR until clean, self-merge, open the integration PR. The user is not available for questions; the orchestrator decides and records the decision here. The merge to main stays the user's.

## Phases

- **Wave 1 (foundational)** — `#186` API types and shared contracts
- **Wave 2 (foundational)** — `#187` Identity on oidc, contract output, cluster discovery and claim (depends on #186)
- **Wave 3 (consumers, parallel)** — `#188` Keycloak modes, `#189` Console + ping, `#190` Web Modeler + cluster list + user (depend on #187)
- **Wave 4 (parallel)** — `#191` e2e flows, `#192` docs (depend on wave 3)

## PRs / worktrees

| Issue | Branch | Worktree path | PR (→ base) | Status |
| --- | --- | --- | --- | --- |
| #186 | feat/management-cluster--api-types | .claude/worktrees/management-cluster/.claude/worktrees/api-types | #193 → feat/management-cluster | ready |
| #187 | feat/management-cluster--identity-oidc-contract | .claude/worktrees/management-cluster/.claude/worktrees/identity-oidc-contract | | not-started |
| #188 | feat/management-cluster--keycloak-modes | .claude/worktrees/management-cluster/.claude/worktrees/keycloak-modes | | not-started |
| #189 | feat/management-cluster--console-ping | .claude/worktrees/management-cluster/.claude/worktrees/console-ping | | not-started |
| #190 | feat/management-cluster--web-modeler | .claude/worktrees/management-cluster/.claude/worktrees/web-modeler | | not-started |
| #191 | test/management-cluster--e2e-flows | .claude/worktrees/management-cluster/.claude/worktrees/e2e-flows | | not-started |
| #192 | docs/management-cluster--user-docs | .claude/worktrees/management-cluster/.claude/worktrees/user-docs | | not-started |

## Contracts

| Name | Realization | Realized in | Status |
| --- | --- | --- | --- |
| `management-api-types` | pre-merge stub PR | #186 | pending |
| `platform-oidc-management` | pre-merge stub PR | #186 | pending |
| `platform-images` | pre-merge stub PR | #186 | pending |
| `cluster-gateway-status` | pre-merge stub PR | #186 | pending |
| `keycloak-cr-types` | pre-merge stub PR | #186 | pending |
| `management-render-core` | pre-merge stub PR | #187 | pending |
| `management-controller-core` | pre-merge stub PR | #187 | pending |
| `ping-and-list-env` | data-only | plan table | locked |
| `e2e-flow-names` | data-only | plan table | locked |

## Bubble-up log

- 2026-08-23 — Copilot on #193 flagged a confused-deputy risk: a namespaced kind selecting and patching CamundaClusters in every namespace. Decision: keep cross-namespace selection (one management plane serves a platform; creating the kind is a platform-administrator action, the RBAC boundary) but adopt LabelSelector semantics: unset selector = no cluster, `{}` = all. Propagated: spec Decision 6 and API section, plan Task 2.3 Step 0 (GoDoc fix + platform-admin note), #187 dispatch prompt. The #192 docs state it.
- 2026-08-23 — #193 Copilot loop reached the 3-round cap still producing "previously missed" findings on unchanged code. Decision: the correct round-3 items were applied in a final commit set (passwordSecretRef forbidden in oidc mode, repository-name pattern on `spec.images`, named `OIDCProviderType`, `hub-websockets` 8.10 default if the chart confirms it); the owner-label-with-namespace item was rejected (the contract carries `camunda.io/management-cluster-namespace` by design; #187 writes it). No round 4 requested; the orchestrator verified the final diff and merged. `reviewing-feature-progress` at the wave-2 boundary re-checks the api types.
- 2026-08-23 — Worktree subagents inherit the orchestrator's Bash isolation pin (`.claude/worktrees/management-cluster`) and cannot run commands in a sibling worktree. Decision: sub-worktrees live nested under the feature worktree at `.claude/worktrees/management-cluster/.claude/worktrees/<sub-name>` (`**/.claude/worktrees/` is gitignored). The #186 worktree was moved there; waves 2+ are created there. The PR-table paths below are the nested ones.
- 2026-08-23 — Copilot on #193: `MinLength=1` added on `IdentityAdminSpec.claimName/claimValue/username` (`has()` admits an explicit empty string). Pushed back on "GoDoc promises behaviour the scaffold does not implement": the kind's GoDoc is its contract and the controller lands on this feature branch before the integration PR.
- 2026-08-23 — Cluster discovery and the claim annotation moved from #189 (Console) into #187 (core) so that #189 and #190 can run in parallel against one attachment. Propagated: plan Task 2.3, issue bodies #187 and #189 updated.

## Pending snapshot

1. Dispatch wave 1: create worktree `.claude/worktrees/management-cluster--api-types` on `feat/management-cluster--api-types` off `feat/management-cluster`; dispatch a worktree subagent with plan Tasks 1.1–1.6, the Conventions block, the contract rows #186 produces (`feature-dev-workflow:fanning-out-with-worktrees` Step 2).
2. On PR open: copilot-review-loop → two-stage review → self-merge → close #186 → flip contract rows to locked.
3. Dispatch wave 2 (#187) off the updated feature branch; same ripening.
4. Dispatch wave 3 (#188, #189, #190) in parallel; resolve the one-line conflicts in `components.go`/`controller.go` at merge.
5. Dispatch wave 4 (#191, #192) in parallel.
6. `reviewing-feature-progress`, integration PR with `Closes #185`, review loop, teardown after CI green, report ready-to-merge.

## Resume checklist

For a fresh Claude session resuming this work: invoke `feature-dev-workflow:resuming-a-feature` — it executes the steps below, routes by the `status:` frontmatter, and works the `## Pending snapshot`. Fallback if that skill is unavailable:

1. Read this state file in full.
2. Read the plan at the path in the `plan:` frontmatter.
3. Read the spec at the path in the `spec:` frontmatter.
4. Verify each open PR's actual state via `gh pr view <num>`.
5. For each `in-progress` or `draft` row, `cd` to the worktree path and check `git status -sb` — an `ahead` count in the header means unpushed commits, and no upstream in the header means the branch was never pushed at all.
6. Re-dispatch subagents as needed per `feature-dev-workflow:developing-a-feature` (parallel waves still in flight; the orchestrator watch loop continues), or work the `## Pending snapshot` when development is past.
