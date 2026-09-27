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
3. **Classify each finding** by what it says is wrong:

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

    It lists every comment the round added or changed, and it exits 1 on a flagged (✗) one:
    - **GREW**: a doc comment is longer than at the round base. A rename counts as the same declaration. It is allowed one extra line for each parameter or result the signature gained, except a `context.Context`.
    - **LONGER than its body**: a doc of three or more lines over a shorter body.

6. **Resolve every ✗ by cutting.** Shorten the doc to the contract (what it returns, its preconditions, the trap a caller hits), then run the gate again. Repeat until it exits 0. There is no other way to clear a ✗: no exemption, no note in the PR, no "this one is justified".
7. **Give every listed comment a verdict,** including the ones without ✗. For each, write one line in your notes: KEEP with the `how-we-write-go` rule it satisfies, or CUT. A comment you cannot name a rule for is cut.
8. **Reply to the threads.** For a finding about prose, say what you cut ("cut the doc to the contract; the conditions are in the body"). Do not say "clarified" or "expanded".
9. **Push.**

After the last round, before you report the PR as clean, run the tool once more against the PR base (`go run ./hack/commentdiff origin/main`) and give a step 7 verdict to every comment the whole PR adds. Against the PR base, a ✗ can be a contract the PR changed on purpose, so here each ✗ gets a verdict: KEEP names the new contract fact each added line states, and every other line is cut. The hard rule of step 6 applies to review rounds, where the contract is already set.

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
| "The signature changed, so the doc is new." | The gate pairs renames and allows one line per new parameter or result. The rest is growth. |
| "This growth is justified." | A ✗ is cleared by cutting. There is no other path. |
| "I read how-we-write-go at the start." | Hours and rounds ago. Step 2 exists because of that. |
| "It is a test helper, the rules are looser." | Test comments follow the same rules. |

## Red flags

- You are about to edit a comment and no code line changes in the same hunk.
- Your thread reply says "clarified", "expanded", "now states", or "documented".
- A godoc now names the branches of its body: "both kinds…", "X also needs…", "instead…".
- A comment carries "so that…" rationale for a decision the PR body already records.
- You are about to push without running the gate in this round.

Each one means: stop, go back to step 3.
