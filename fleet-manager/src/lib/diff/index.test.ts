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
