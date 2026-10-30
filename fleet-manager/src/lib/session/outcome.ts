import { pathFrom } from "@/lib/fields";
import { child, type FieldPath } from "@/lib/merge";
import type { Preview, WriteResult } from "@/lib/write/types";
import type { FieldError } from "./cluster";
import { claimAfter, type Session } from "./session";
import { indexSteps } from "./steps";

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
    targets.set(path, path);
    const steps = indexSteps(after, entry.steps);
    if (steps) targets.set(pathFrom("", steps), path);
  }
  return targets;
}

function pendingPath(
  path: FieldPath,
  targets: Map<FieldPath, FieldPath>,
): FieldPath | undefined {
  for (const p of [path, child("spec", path)])
    for (const [target, pending] of targets)
      if (
        p === target ||
        p.startsWith(`${target}.`) ||
        p.startsWith(`${target}[`)
      )
        return pending;
  return undefined;
}
