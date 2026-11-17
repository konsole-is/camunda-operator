import { describe, expect, it } from "vitest";
import { listOf, messageOf } from "./text";

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

describe("messageOf", () => {
  it("gives the message of an Error and any other value as text", () => {
    expect(messageOf(new Error("refused"))).toBe("refused");
    expect(messageOf("timeout")).toBe("timeout");
    expect(messageOf(503)).toBe("503");
  });
});
