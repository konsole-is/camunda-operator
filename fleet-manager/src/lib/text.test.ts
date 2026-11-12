import { describe, expect, it } from "vitest";
import { listOf } from "./text";

describe("listOf", () => {
  it.each([
    [[], ""],
    [["a"], "a"],
    [["a", "b"], "a and b"],
    [["a", "b", "c"], "a, b, and c"],
  ])("joins %j as %j", (items, text) => {
    expect(listOf(items)).toBe(text);
  });
});
