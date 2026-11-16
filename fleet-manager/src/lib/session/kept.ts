import {
  APP_FIELD_MANAGER,
  child,
  type FieldPath,
  fieldManagers,
  type ManagedObject,
} from "@/lib/merge";
import { claimAfter, type ClaimObject, type Session } from "./session";

/** KeptField is a field that a change leaves out and that another field manager keeps. */
export type KeptField = { path: FieldPath; manager: string };

/**
 * keptFields returns each field of the spec of the claim of s that the
 * change of s leaves out while a field manager other than the fleet
 * manager owns it. The apply keeps such a field, because the fleet manager
 * does not take a field over. A list counts as one field. The result is in
 * the order of the claim, and empty when the claim holds no managedFields.
 */
export function keptFields<C extends ClaimObject>(s: Session<C>): KeptField[] {
  const owners = fieldManagers(s.claim as ManagedObject);
  const kept: KeptField[] = [];
  // above holds the paths above path, the nearest first.
  const visit = (
    before: unknown,
    after: unknown,
    path: FieldPath,
    above: FieldPath[],
  ) => {
    // A removed object names the fields below it unless one manager owns
    // it as a whole.
    const gone = after === undefined;
    const descend =
      isObject(before) &&
      Object.keys(before).length > 0 &&
      !(gone && Object.hasOwn(owners, path));
    if (gone && !descend) {
      const owner = [path, ...above].find((p) => Object.hasOwn(owners, p));
      const manager = owner === undefined ? undefined : owners[owner];
      if (manager !== undefined && manager !== APP_FIELD_MANAGER)
        kept.push({ path, manager });
      return;
    }
    if (!descend || !(gone || isObject(after))) return;
    const below = gone ? {} : (after as Record<string, unknown>);
    for (const [key, value] of Object.entries(before as object))
      visit(value, below[key], child(path, key), [path, ...above]);
  };
  visit(s.claim.spec, claimAfter(s).spec, "spec", []);
  return kept;
}

function isObject(value: unknown): value is Record<string, unknown> {
  return typeof value === "object" && value !== null && !Array.isArray(value);
}
