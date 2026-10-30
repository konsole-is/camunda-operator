import { child, type FieldPath } from "@/lib/merge";
import type { Preview, WriteResult } from "@/lib/write/types";
import type { FieldError } from "./cluster";
import type { Session } from "./session";

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
 * field, with or without the leading spec, goes to that field. It does not
 * change s.
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
  for (const error of errors) {
    const path = error.path && pendingPath(error.path, s);
    if (path) fields.push({ path, message: error.message });
    else messages.push(error.message);
  }
  return { ok: false, stale: false, fields, messages };
}

function pendingPath(path: FieldPath, s: Session): FieldPath | undefined {
  return [path, child("spec", path)].find((p) => s.pending.has(p));
}
