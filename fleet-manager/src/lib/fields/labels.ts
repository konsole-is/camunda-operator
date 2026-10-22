import type { FieldLock, FieldState } from "./types";

/** stateLabel returns the words that the pages show for state. */
export function stateLabel(state: FieldState): string {
  switch (state.kind) {
    case "filled":
      return `Filled by ${state.sectionLabel}`;
    case "setBy":
      return `Set by ${state.manager ?? "an unknown writer"}`;
    case "claim":
      return "Changed in the claim";
    case "release":
      return "From release";
    case "preset":
      return "From preset";
    case "admin":
      return "Set by your admin";
    case "environment":
      return "From environment";
  }
}

/** stateNote returns the line under the label of state, or undefined when it has none. */
export function stateNote(state: FieldState): string | undefined {
  return state.kind === "setBy" && state.takeOver
    ? "Setting it in the claim takes it over."
    : undefined;
}

/** lockLabel returns the words that the pages show for lock. */
export function lockLabel(lock: FieldLock): string {
  switch (lock.reason) {
    case "filled":
      return `Filled by ${lock.sectionLabel}`;
    case "release":
      return "Set by the release";
    case "claimController":
      return "Fixed by the claim controller";
    case "section":
      return "Set by a section";
    case "notManaged":
      return "Not managed by the fleet manager";
    case "admin":
      return "Set by your admin";
  }
}
