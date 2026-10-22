import { describe, expect, it } from "vitest";
import { lockLabel, stateLabel, stateNote } from "./labels";
import type { FieldLock, FieldState } from "./types";

describe("stateLabel", () => {
  it.each<[FieldState, string]>([
    [
      {
        kind: "filled",
        section: "volume-encryption",
        sectionLabel: "Volume encryption",
      },
      "Filled by Volume encryption",
    ],
    [
      { kind: "setBy", manager: "pvc-autoresizer", takeOver: true },
      "Set by pvc-autoresizer",
    ],
    [{ kind: "setBy", takeOver: false }, "Set by an unknown writer"],
    [{ kind: "claim" }, "Changed in the claim"],
    [{ kind: "release" }, "From release"],
    [{ kind: "preset" }, "From preset"],
    [{ kind: "admin" }, "Set by your admin"],
    [{ kind: "environment" }, "From environment"],
    [{ kind: "default" }, "Default value"],
  ])("%o reads %s", (state, want) => {
    expect(stateLabel(state)).toBe(want);
  });
});

describe("stateNote", () => {
  it("says that the claim takes over a field that another writer sets", () => {
    expect(stateNote({ kind: "setBy", manager: "x", takeOver: true })).toBe(
      "Setting it in the claim takes it over.",
    );
  });

  it.each<FieldState>([
    { kind: "setBy", manager: "x", takeOver: false },
    { kind: "claim" },
    { kind: "preset" },
    { kind: "default" },
    { kind: "filled", section: "access", sectionLabel: "Access" },
  ])("has no note for %o", (state) => {
    expect(stateNote(state)).toBeUndefined();
  });
});

describe("lockLabel", () => {
  it.each<[FieldLock, string]>([
    [
      { reason: "filled", section: "access", sectionLabel: "Access" },
      "Filled by Access",
    ],
    [{ reason: "release" }, "Set by the release"],
    [{ reason: "claimController" }, "Fixed by the claim controller"],
    [{ reason: "section" }, "Set by a section"],
    [{ reason: "notManaged" }, "Not managed by the fleet manager"],
    [{ reason: "admin" }, "Set by your admin"],
  ])("%o reads %s", (lock, want) => {
    expect(lockLabel(lock)).toBe(want);
  });
});
