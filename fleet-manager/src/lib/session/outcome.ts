import { pathFrom, type Step } from "@/lib/fields";
import { child, type FieldPath } from "@/lib/merge";
import type { Preview, WriteResult } from "@/lib/write/types";
import type { FieldError } from "./cluster";
import { claimAfter, type PendingChange, type Session } from "./session";
import { equalJSON } from "@/lib/policy";
import { indexSteps, valueAt, withChanges } from "./steps";

/** STALE_CLAIM_MESSAGE is what a session shows when its claim changed since the form read it. */
export const STALE_CLAIM_MESSAGE =
  "The claim changed since you opened it. Reload to see the new version.";

/**
 * WriteOutcome is what a session shows for a write or a preview. fields are
 * the errors of pending fields, and messages the other errors.
 */
export type WriteOutcome =
  | { ok: true }
  | { ok: false; stale: boolean; fields: FieldError[]; messages: string[] };

/**
 * outcomeOf maps a result to what the session shows. A Stale error gives
 * STALE_CLAIM_MESSAGE and nothing else. An error whose path names a pending
 * field or a field below it, with or without the leading spec, goes to that
 * pending field. It does not change s.
 */
export function outcomeOf(
  result: WriteResult | Preview,
  s: Session,
): WriteOutcome {
  if ("ok" in result && result.ok) return { ok: true };
  const { errors } = result as { errors: Preview["errors"] };
  if (errors.length === 0) return { ok: true };
  if (errors.some((e) => e.code === "Stale"))
    return {
      ok: false,
      stale: true,
      fields: [],
      messages: [STALE_CLAIM_MESSAGE],
    };
  const fields: FieldError[] = [];
  const messages: string[] = [];
  const targets = targetsOf(s);
  for (const error of errors) {
    const path = error.path && pendingPath(error.path, targets);
    if (path) fields.push({ path, message: error.message });
    else messages.push(error.message);
  }
  return { ok: false, stale: false, fields, messages };
}

// targetsOf maps the path of each pending field, keyed and by index in the
// claim after the change, to its pending path. The API server names a list
// item by its index.
function targetsOf(s: Session): Map<FieldPath, FieldPath> {
  const after = claimAfter(s);
  const targets = new Map<FieldPath, FieldPath>();
  for (const [path, entry] of s.pending) {
    // The path of an index names an item of the claim, which a reset can
    // move.
    const indexed = entry.steps.some((step) => "index" in step);
    if (!indexed) targets.set(path, path);
    if ("reset" in entry) continue;
    const steps = indexSteps(after, shifted(s, entry.steps));
    if (steps) targets.set(pathFrom("", steps), path);
  }
  return targets;
}

// shifted moves each index of steps, which names an item of the claim, to
// the place of that item after the changes of the session removed earlier
// items.
function shifted(s: Session, steps: readonly Step[]): Step[] {
  return steps.map((step, j) => {
    if (!("index" in step)) return step;
    const list = steps.slice(0, j);
    const below = new Map<number, PendingChange[]>();
    for (const entry of s.pending.values()) {
      const at = entry.steps[j];
      if (
        !at ||
        !("index" in at) ||
        at.index >= step.index ||
        !equalJSON(entry.steps.slice(0, j), list)
      )
        continue;
      below.set(at.index, [...(below.get(at.index) ?? []), entry]);
    }
    const length = (items: unknown) =>
      Array.isArray(items) ? items.length : 0;
    let removed = 0;
    for (const changes of below.values())
      if (
        length(valueAt(withChanges(s.claim, changes), list)) <
        length(valueAt(s.claim, list))
      )
        removed++;
    return { index: step.index - removed };
  });
}

function pendingPath(
  path: FieldPath,
  targets: Map<FieldPath, FieldPath>,
): FieldPath | undefined {
  let best: [FieldPath, FieldPath] | undefined;
  for (const p of [path, child("spec", path)])
    for (const [target, pending] of targets)
      if (
        (p === target ||
          p.startsWith(`${target}.`) ||
          p.startsWith(`${target}[`)) &&
        (!best || target.length > best[0].length)
      )
        best = [target, pending];
  return best?.[1];
}
