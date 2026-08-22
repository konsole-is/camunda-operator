---
name: writing-operator-docs
description: Use when about to create, edit, or review a page that a user of the operator reads - anything under docs/ except docs/superpowers/, README.md, dist/chart/README.md, or a spec or status field description in api/v1 - before the first edit, and again before you call the docs done.
---

# Writing operator docs

## Overview

The docs tell a person how to use the operator and what happens to their cluster when they do. They are not a record of how the operator is built. A reader opens a page with a task: create a resource, understand what it did, act on a condition. Every sentence serves one of those tasks, or it goes.

## Before you write

Do these in order, before the first sentence goes on a page. Not after the draft. Not "in spirit".

1. Invoke `simple-english:simple-english` with the Skill tool. It holds the sentence rules: 20 or 25 words, active voice, one word for one thing, no semicolons, no em-dashes, no contractions. Use pragmatic mode. Every sentence you write after this point follows it, and its self-check runs before you call the page done.
2. Invoke `feature-dev-workflow:writing-docs` with the Skill tool. It decides which surface a change belongs on, names the reader personas, and runs the fresh-reader RED and GREEN loop that decides when the docs are done.
3. Read the rest of this skill. It adds what is specific to this operator's docs.

A draft written before step 1 is a draft you rewrite. Do not patch it.

## Where the docs live

| Surface | What goes there |
| --- | --- |
| `docs/crds/<kind>.md` | The reference for one kind. Follow `docs/crds/TEMPLATE.md` and the rules in its comment block. |
| `docs/guides/*.md` | Tasks that span kinds, and narrative: presets, storage, auth, backup, operations. |
| `docs/index.md`, `getting-started.md`, `installation.md`, `architecture.md`, `go-api.md` | Entry, first cluster, install, the rules the operator follows, the Go module. |
| `README.md` | What the operator is, install, one taste, pointers into `docs/`. No depth. |
| `dist/chart/README.md` | The Helm values. Hand-maintained, the plugin preserves it. |
| GoDoc on spec and status fields in `api/v1` | Shows in `kubectl explain` and in the CRD. Same reader, same rules. |

`docs/superpowers/` is internal working material. mkdocs excludes it. This skill does not apply there.

A kind the operator does not act on yet gets no new page. `docs/crds/index.md` lists such kinds under "Planned kinds".

## The shape of a section

Write one section per behavior a user relies on, named after the topic the reader searches for: Endpoints, Authentication, Storage, Deletion, Suspend, Rotation. Never "How it works". Inside a section, write one paragraph per reader question, in this order:

1. What the reader sets, with the CR fragment next to it.
2. What the reader gets, named by the one handle they use for it: the Service they connect to, the Secret they read the password from, the condition they wait on. Most outcomes need one name or none. "The cluster trusts the authority" is complete without the init container, the volume, and the mount that make it so.
3. What happens without the setting, on failure, and on deletion, and the step the reader takes.

Do not inventory what the operator created. A resource is named only when the reader connects to it, reads it, selects on it, or waits on it. Keep a name on the page when the reader types it, selects on it, or reads it back in `kubectl get`, an event, or a condition message. A path or a password stays only where the reader has to type it.

## What stays off the page

The mechanism you read in the code tells you the outcome to write. The mechanism itself stays in the code. These do not belong on a user page:

- The order of reconcile steps, a state machine, a step that "runs first", what is written "before" the next call.
- Polling intervals, retries, requeues, what the controller watches, which client it uses.
- Field managers, server-side apply, mutations, components, guards, ocf vocabulary.
- Init containers, mounts, volumes, how an ID or a name is generated, what the code asserts or pins.
- A Lease, a lock, an "admission" phase, what a Job copies from a pod. Say what the reader sees: "a cluster holds one backup or one restore at a time", "the restore waits in `Pending`".
- A finalizer. Say what is removed and what is kept on deletion instead.
- Why the code is built the way it is. A reason the reader acts on stays: "a failed restore leaves the cluster suspended, because its volumes can be half written". A reason about the code goes to the commit message and the design spec.
- The operators above this one. `camunda-cloud-operator` and `CloudCamundaCluster` never appear in these docs.

One fact about mechanism stays when the reader meets it: a field manager name that appears in a conflict message for a GitOps tool, a label the reader selects on, an event name.

Before (from an operator that explains itself):

> The operator posts the backup ID to `/actuator/backupHistory`. Operate and Tasklist write their indices into the snapshot repository. The operator records the names in `status.historySnapshots`, then polls `/actuator/backupHistory/<backupId>` every 5 seconds until the state is complete.

After (what the reader uses):

> `status.historySnapshots` lists the Elasticsearch snapshots that hold the Operate and Tasklist data of this backup. A restore needs these names. Camunda documents what the snapshots contain in [Backup and restore](https://docs.camunda.io/docs/self-managed/operational-guides/backup-restore/backup-and-restore/).

## Camunda facts: link, do not restate

The operator docs explain the operator. Camunda explains Camunda. When a sentence states how Camunda itself behaves, for example what a backup API does, what a pause of exporting means, what a configuration key does, or what Optimize needs, then:

1. Search with the `camunda-docs` MCP server and take the URL the search returns. Never write a `docs.camunda.io` URL from memory.
2. Link that page and keep one sentence on what it means for a cluster the operator runs.
3. Restate only the one fact the reader needs at that moment, such as a default or a limit, with the link next to it.

A paraphrase of a Camunda page is wrong twice: it is longer than a link, and it rots when Camunda changes the page.

## Show, do not tell

These conventions bind every page. Mirror the nearest existing block before you write a new one.

- Never say "set `spec.x.y`" in prose alone. Put a CR fragment next to it, with `# ... the rest of your cluster` for elided fields.
- Never describe a status field in prose alone. Show the `status:` block as `kubectl get -o yaml` prints it, with the real condition messages from the code.
- Status tables keep the "What to do" column.
- Diagrams are Mermaid. Resource maps are `graph LR`.
- Inside a numbered list, continuation lines and fenced blocks use 4-space indentation. A blank line follows a nested fence before a sibling bullet.
- Names in examples: cluster `my-cluster` in namespace `my-cluster-ns`, and the derived names listed in `docs/crds/TEMPLATE.md`.

## Verify every fact

Check every field, default, condition, reason, event, and derived name against `api/v1` and the controller or package that produces it before you write it. The docs are read by people who type what they read. A wrong name costs them an hour.

## Done means

- The self-check of `simple-english:simple-english` ran on every sentence you added or changed.
- The fresh-reader pass of `feature-dev-workflow:writing-docs` passed: a subagent given only the page answered the reader's questions.
- `mkdocs build --strict` passes.
- You scanned the diff for `reconcil`, `controller`, `state machine`, `poll`, `field manager`, `server-side apply`, `init container`, `mount`, `emptyDir`, `mutation`, `component`, `watch`, `requeue`, `finalizer`. Every hit names something the reader types or sees, or it is gone.
- Every `docs.camunda.io` link came from a `camunda-docs` search result.

## Red flags

| Thought | Reality |
| --- | --- |
| "The reader asked how it works" | They asked what happens to their cluster. Write the outcomes in the order the reader sees them, not the steps the operator takes. |
| "This detail shows the operator is careful" | The reader cannot act on it. The code review is where carefulness is judged. |
| "The reader should know what was created" | They should know what they can use. The Service they connect to, yes. The ConfigMap the process reads, no. |
| "I just built it, so I know what to write" | You know the mechanism. Start from the reader's questions, and verify each fact against the code like a stranger would. |
| "Camunda's page explains it well, I will paraphrase" | Link it. Keep the one sentence that says what it means here. |
| "A How it works section makes the page complete" | Name sections after the reader's topics. A step list of the controller is not a topic. |
| "The reader needs the reason to believe the rule" | State the rule and what the reader sees. A reason that names a mechanism ("the Job copies the broker image, so") is the commit message. |
| "The page next to it already says it this way" | Mirror its shape, not its leaks. A sentence that fails the test above is wrong on the old page too. Fix it while you are there. |
| "It reads clearly to me" | You hold the feature in your head. Run the fresh-reader pass. |
