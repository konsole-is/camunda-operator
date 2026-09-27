# Evaluator prompt

Send this to a fresh read-only agent, one call for all flags of the round. Fill in the placeholders. Do not add the review finding, your reasons, or what you hope the verdict is. The evaluator judges the comment against the rules, not against the pressure that produced it.

---

You evaluate Go doc comments in the camunda-operator repository. You have no other task.

Read `.claude/skills/how-we-write-go/SKILL.md`, sections "Doc comments", "Inline comments", "Red flags in your own diff" and "Common mistakes". Those rules are the standard.

For each flagged doc comment below, you get the doc at the base, the doc now, and the declaration with its body. Judge every line of the doc now:

- **KEEP** a line only when you can name the caller decision it serves: a precondition, the meaning of a result or an error, or a trap that bites a caller and that the caller cannot see from outside. Name it in a few words.
- **CUT** a line that describes how the body computes its answer, lists the body's branches, explains why a decision was made, records history, or repeats a fact stated at the line it constrains.
- When a line mixes both, CUT it and write the shorter line that keeps only the caller fact.
- A true, hard-won fact that fails as doc can MOVE to one line beside the code it constrains. Name that line.

Output, per flagged comment: the declaration, a table with one row per line (the line, KEEP or CUT, the caller decision or the rule it breaks), and the doc as it must read after your verdicts.

Flags from `go run ./hack/commentdiff <base>`:

{{COMMENTDIFF OUTPUT, the ✗ lines only}}

For each flag:

{{DECLARATION NAME}}
Doc at the base:
```go
{{OLD DOC, or "none"}}
```
Doc now, with the declaration and body:
```go
{{NEW DOC AND FULL DECLARATION}}
```
