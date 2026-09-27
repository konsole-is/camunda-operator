---
name: addressing-review-feedback
description: Use when about to act on code review feedback in this repository - a Copilot or human pull request review, an inline review thread, a review summary, a suppressed-comments block, a /code-review finding, or any round of a review loop - before the first edit made in response, and again before every push of a review fix.
---

# Addressing review feedback

## Overview

A review round is where comments grow. Each finding is a small push to make prose more precise. After five rounds, a godoc lists the branches of the body and reads like a second spec. It is the drift that `how-we-write-go` warns about ("A comment that restates the code becomes a second spec, and it drifts"). This skill is the procedure that keeps a review round from producing it.

**REQUIRED BACKGROUND:** `how-we-write-go`, sections "Doc comments", "Inline comments", "Red flags in your own diff" and "Common mistakes". This skill applies them at the moment they are easiest to forget. It does not replace them.

**Violating the letter of these steps is violating their spirit.** The steps are not a summary to follow in your own words.

## The procedure, every round

Run it for each round, even when the previous round passed it.

1. **Record the round base.** Before you edit, write down the current head: `BASE=$(git rev-parse HEAD)`.
2. **Re-read the comment rules.** Open `.claude/skills/how-we-write-go/SKILL.md` and read the four sections named above again. What you read at the start of the session has faded by now. That is how the rules get lost.
3. **Classify each finding** by what it says is wrong. The table is for Go code comments: godocs, inline comments, and test comments. A user-facing text is different: an `api/v1` field or type description (it becomes the CRD description), a page under `docs/`, or an error or condition message. For those, accuracy comes first. When a finding shows that one of them is wrong, correct it under `writing-operator-docs`, even when the correct version is longer.

    | The finding says | The fix | What happens to the comments |
    | --- | --- | --- |
    | The behavior is wrong or incomplete | Change the code, with a test that fails without the change | Nothing grows. A comment that the new code makes false is cut to the contract, not extended |
    | A comment or godoc is wrong, stale, or does not describe a case | Cut the claim to the contract, or delete it | It gets shorter or stays the same length. It never gets longer or more precise |
    | A comment or godoc should say more ("does not state X", "a reader cannot tell Y") | Reject it in the thread: the code states X, and a second account drifts | Unchanged |
    | Rationale is missing ("why does ES differ?") | Put it in the PR body's Decisions section | Unchanged. If the next reader of the code would be caught by it, add one line at the line it constrains, not in the godoc |

4. **Write the fix.**
5. **Run the gate** from the repository root:

    ```bash
    go run ./hack/commentdiff "$BASE"
    ```

    It lists every comment the round added or changed. It flags (✗) two shapes, and exits 1 when it flags one:
    - **GREW**: a doc comment is longer than at the round base. A rename counts as the same declaration.
    - **LONGER than its body**: a doc of three or more lines over a shorter body.

    A flag is not a verdict. It means the comment must be evaluated before you push.

6. **Get every ✗ evaluated by a fresh agent, not by you.** You are the one under pressure from the finding, so your own judgment of the growth does not count. Dispatch one read-only agent for all the flags of the round, with the prompt in `evaluator-prompt.md` next to this file. Give it the rules, the flagged docs, and the declarations. Do not give it the finding or your reasons.
    - Apply its verdicts as it gives them: cut every CUT line, use its shortened doc where it wrote one, and put a fact it moved on the line it named. You do not overrule a CUT. If you think a CUT is wrong, the line stays cut and you raise it with the user in your report.
    - **If your harness gives you no way to dispatch an agent** (a fork, for example), re-read the four `how-we-write-go` sections named above first, then evaluate with the same prompt yourself. Mark every verdict "self-evaluated", so the user can see that it was not independent.
7. **Give every other listed comment a verdict yourself,** the ones without ✗. For each, write one line in your notes: KEEP with the `how-we-write-go` rule it satisfies, or CUT. A comment you cannot name a rule for is cut.
8. **Post the verdicts.** In the round's PR comment, add a table of every ✗: the declaration, the lines kept with the caller decision each serves, the lines cut, and who evaluated it (fresh agent or self-evaluated).
9. **Reply to the threads.** For a finding about prose, say what you cut ("cut the doc to the contract; the conditions are in the body"). Do not say "clarified" or "expanded".
10. **Push.**

After the last round, before you report the PR as clean, run the gate once more against the PR base (`go run ./hack/commentdiff origin/main`). Evaluate its flags the same way (step 6), and give a step 7 verdict to every other comment the whole PR adds. Against the PR base, a flag is often a contract that the PR changed on purpose. The evaluator keeps each line that states a new caller fact.

## Example

The finding: "The doc comment does not state the two-part gate (binding for both kinds, plus Ready for RDBMS or backupRepository for Elasticsearch)."

What the rounds produced without this procedure:

```go
// cannotStart describes why a backup of the storage type cannot start on
// the cluster, as the note of the skip event after the trigger time. It
// returns "" when the backup can start. Both kinds need a management binding
// that the backup controllers accept. An RDBMS backup also needs Ready True.
// An Elasticsearch backup needs the backup repository of the binding
// instead, so a degraded cluster that still serves its management API is
// backed up.
```

What the procedure produces. The thread reply rejects the finding, because the switch below the doc is the account of the two-part gate:

```go
// cannotStart returns the note of the skip event when a backup of storageType
// cannot start on the cluster, or "" when it can. An error means the check
// itself failed.
```

## Rationalizations

| Thought | Reality |
| --- | --- |
| "The reviewer says the comment is inaccurate, so I make it accurate." | An accurate, longer comment is the drift. Make it shorter, until what is left is true. |
| "The reviewer asked for the doc to state the conditions." | The conditions are in the code. A second account is the thing `how-we-write-go` forbids. Reject in the thread. |
| "Documenting the decision stops reviewers raising it again." | The PR body holds decisions. The code comment is read long after the review is over. |
| "The signature changed, so the doc is new." | The gate pairs renames. The evaluator keeps each line that states a new caller fact, and cuts the rest. |
| "This growth is justified, I can see it is." | Then the evaluator keeps it. You do not decide on your own growth. |
| "Dispatching an evaluator for one line is overkill." | One flag is one short call. The grown comment on #391 grew one line at a time. |
| "I read how-we-write-go at the start." | Hours and rounds ago. Step 2 exists because of that. |
| "It is a test helper, the rules are looser." | Test comments follow the same rules. |
| "The CRD description is wrong, but the fix would make it longer." | A user-facing text that is false gets corrected. The cut rule is for code comments that restate code. |

## Red flags

- You are about to edit a comment and no code line changes in the same hunk.
- Your thread reply says "clarified", "expanded", "now states", or "documented".
- A godoc now names the branches of its body: "both kinds…", "X also needs…", "instead…".
- A comment carries "so that…" rationale for a decision the PR body already records.
- You are about to push without running the gate in this round.
- You are about to push a ✗ that no evaluator has judged, or to keep a line it cut.

Each one means: stop, go back to step 3.
