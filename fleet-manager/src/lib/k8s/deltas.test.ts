import { describe, expect, it } from "vitest";
import { applyDelta, deltaType } from "./deltas";

describe("deltaType", () => {
  it.each([
    ["add", false, true, "ADDED", true],
    ["update", true, true, "MODIFIED", true],
    ["update", false, true, "ADDED", true],
    ["update", true, false, "DELETED", false],
    ["update", false, false, undefined, false],
    ["delete", true, true, "DELETED", false],
    ["delete", false, false, undefined, false],
  ] as const)(
    "a %s that was seen=%s and matches=%s is %s",
    (change, wasSeen, matches, expected, seenAfter) => {
      const seen = new Set(wasSeen ? ["ns/a"] : []);

      expect(deltaType(seen, change, "ns/a", matches)).toBe(expected);
      expect(seen.has("ns/a")).toBe(seenAfter);
    },
  );
});

describe("applyDelta", () => {
  const obj = (name: string, v: string) => ({
    metadata: { namespace: "ns", name, resourceVersion: v },
  });

  it("adds, replaces in place, and removes by namespace and name", () => {
    let list = [obj("a", "1"), obj("b", "1")];
    list = applyDelta(list, {
      type: "ADDED",
      kind: "Pod",
      object: obj("c", "1"),
    });
    list = applyDelta(list, {
      type: "MODIFIED",
      kind: "Pod",
      object: obj("a", "2"),
    });
    list = applyDelta(list, {
      type: "DELETED",
      kind: "Pod",
      object: obj("b", "2"),
    });

    expect(list).toEqual([obj("a", "2"), obj("c", "1")]);
  });

  it("does not change the list it was given", () => {
    const list = [obj("a", "1")];
    applyDelta(list, { type: "DELETED", kind: "Pod", object: obj("a", "2") });
    expect(list).toEqual([obj("a", "1")]);
  });
});
