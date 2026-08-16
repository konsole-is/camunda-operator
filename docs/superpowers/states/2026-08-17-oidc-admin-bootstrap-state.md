---
feature: oidc-admin-bootstrap
spec: docs/superpowers/specs/2026-08-17-oidc-admin-bootstrap-design.md
plan: docs/superpowers/plans/2026-08-17-oidc-admin-bootstrap-plan.md
tracking_issue: #72
feature_branch: feat/oidc-admin-bootstrap
feature_worktree: .claude/worktrees/oidc-admin-bootstrap
sub_pr_approval: autonomous
sub_pr_review_loop: on
sub_pr_target: feature-branch
integration_pr:
status: foundational-wave
---

# OIDC admin bootstrap — orchestration state

## Phases

The two sub-issues are strictly sequential. #61 cannot go green until #73 has merged into the feature branch, because the e2e asserts a process deployment that only the admin grant makes possible. There is no parallel wave.

- **Phase 1 (foundational)** — `#73` operator: claim names, admin bootstrap block, render, preset merge, goldens, CRD docs
- **Phase 2 (consumer)** — `#61` e2e: Keycloak realm in kind, request-helper auth modes, the OIDC flow

## PRs / worktrees

| Issue | Branch | Worktree path | PR (→ base) | Status |
| --- | --- | --- | --- | --- |
| #73 | feat/oidc-admin-bootstrap--admin-config | .claude/worktrees/oidc-admin-bootstrap--admin-config | #75 → feat/oidc-admin-bootstrap | self-merged |
| #61 | feat/oidc-admin-bootstrap--keycloak-e2e | .claude/worktrees/oidc-admin-bootstrap--keycloak-e2e | → feat/oidc-admin-bootstrap | in-progress |

## Contracts

| Name | Realization | Realized in | Status |
| --- | --- | --- | --- |
| `oidc-admin-crd-surface` | sequential — #61 branches from the feature branch after #73 merges into it | #75 (merged 2026-08-17) | locked |

## Bubble-up log

- **Copilot review effort level is the repository default (2026-08-17).** #75 is a substantive change — new CRD surface plus a render path — and a balanced review would suit it better than the lite default. No API sets the level, so the autonomous fan-out cannot raise it. Raise it in **Settings → Copilot → Code review** if later sub-PRs deserve deeper analysis.
- **Keycloak realm verified ahead of Phase 2 (2026-08-17).** The realm in the plan's Task 9 was probed against the kind cluster before any Go was written, because the two protocol mappers were the highest-risk part. A client-credentials token carries `aud: ["camunda", "account"]`, `client_id: "camunda"`, and `iss` equal to the Service URL, so `clientIdClaim: client_id` resolves it to a client. The realm also emits `preferred_username: service-account-camunda`, so the token holds both claims and the client id claim wins, which is the documented order. Manifest is parked at the session scratchpad until the Phase 2 worktree exists.

## Pending snapshot

1. Commit and push the planning artifacts on `feat/oidc-admin-bootstrap`, then invoke `feature-dev-workflow:developing-a-feature`.
2. Phase 1: work `#73` through Tasks 1 to 8 of the plan in `.claude/worktrees/oidc-admin-bootstrap--admin-config`. Gate: `make all` and `go test ./...` green, and the source scan green with `CAMUNDA_SOURCE_DIR` set to a camunda/camunda checkout at 8.9.9.
3. Self-merge `#73` into `feat/oidc-admin-bootstrap`, then close the issue with `gh issue close 73`.
4. Phase 2: work `#61` through Tasks 9 to 12 in `.claude/worktrees/oidc-admin-bootstrap--keycloak-e2e`, branched after step 3. Gate: `make test-e2e` green and the suite inside `E2E_TIMEOUT`.
5. Self-merge `#61`, close it, then open the integration PR to `main` with `Closes #72`. Delete the plan and this state file in the last commit of the feature branch. The user reviews that PR — it is the one gate they kept.

## Resume checklist

For a fresh Claude session resuming this work: invoke `feature-dev-workflow:resuming-a-feature` — it executes the steps below, routes by the `status:` frontmatter, and works the `## Pending snapshot`. Fallback if that skill is unavailable:

1. Read this state file in full.
2. Read the plan at the path in the `plan:` frontmatter.
3. Read the spec at the path in the `spec:` frontmatter.
4. Verify each open PR's actual state via `gh pr view <num>`.
5. For each `in-progress` or `draft` row, `cd` to the worktree path and check `git status -sb` — an `ahead` count in the header means unpushed commits, and no upstream in the header means the branch was never pushed at all.
6. Re-dispatch subagents as needed per `feature-dev-workflow:developing-a-feature`, or work the `## Pending snapshot` when development is past.
