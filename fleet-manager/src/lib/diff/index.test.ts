import { describe, expect, it } from "vitest";
import { lineDiff, objectDiff } from "./index";

describe("lineDiff", () => {
  it("marks added and removed lines and keeps the rest", () => {
    expect(lineDiff("a\nb\nc\n", "a\nx\nc\n")).toEqual([
      { op: "same", text: "a" },
      { op: "removed", text: "b" },
      { op: "added", text: "x" },
      { op: "same", text: "c" },
    ]);
  });

  it("adds every line of a new text", () => {
    expect(lineDiff("", "a\nb")).toEqual([
      { op: "added", text: "a" },
      { op: "added", text: "b" },
    ]);
  });

  it("gives no line for two empty texts", () => {
    expect(lineDiff("", "")).toEqual([]);
  });
});

describe("objectDiff", () => {
  it("shows no change for the same object with another key order", () => {
    const diff = objectDiff(
      { spec: { b: 2, a: 1 }, kind: "X" },
      { kind: "X", spec: { a: 1, b: 2 } },
    );
    expect(diff.every((line) => line.op === "same")).toBe(true);
  });

  it("treats a missing before as a new object", () => {
    expect(objectDiff(undefined, { a: 1 })).toEqual([
      { op: "added", text: "a: 1" },
    ]);
  });

  it("treats a missing after as a deleted object", () => {
    expect(objectDiff({ a: 1 }, undefined)).toEqual([
      { op: "removed", text: "a: 1" },
    ]);
  });

  it("gives no line when both objects are missing", () => {
    expect(objectDiff(undefined, undefined)).toEqual([]);
  });

  it("shows a changed field as one removed and one added line", () => {
    expect(
      objectDiff({ spec: { replicas: 1 } }, { spec: { replicas: 3 } }),
    ).toEqual([
      { op: "same", text: "spec:" },
      { op: "removed", text: "  replicas: 1" },
      { op: "added", text: "  replicas: 3" },
    ]);
  });
});

describe("lineDiff on large texts", () => {
  it("finds one change in a long text that is the same around it", () => {
    const lines = Array.from({ length: 20_000 }, (_, i) => `line ${i}`);
    const after = [...lines];
    after[10_000] = "changed";
    const diff = lineDiff(lines.join("\n"), after.join("\n"));
    expect(diff.filter((line) => line.op !== "same")).toEqual([
      { op: "removed", text: "line 10000" },
      { op: "added", text: "changed" },
    ]);
    expect(diff).toHaveLength(20_001);
  });

  it("shows a change too large to compare as removed lines, then added lines", () => {
    // "shared" would stay unchanged in a line by line compare.
    const before = Array.from({ length: 2_100 }, (_, i) =>
      i === 1_000 ? "shared" : `old ${i}`,
    );
    const after = Array.from({ length: 2_100 }, (_, i) =>
      i === 1_000 ? "shared" : `new ${i}`,
    );
    const diff = lineDiff(before.join("\n"), after.join("\n"));
    expect(diff).toEqual([
      ...before.map((text) => ({ op: "removed", text })),
      ...after.map((text) => ({ op: "added", text })),
    ]);
  });
});
