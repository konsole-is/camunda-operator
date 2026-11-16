import { describe, expect, it } from "vitest";
import { condition } from "./fixtures";
import { conditionsOf, listOf, readyOf } from "./values";

describe("listOf", () => {
  it("gives an empty list for no value", () => {
    expect(listOf(undefined)).toEqual([]);
    expect(listOf(null)).toEqual([]);
  });

  it("gives a list as it is", () => {
    expect(listOf([1, "a"])).toEqual([1, "a"]);
  });

  it("gives undefined for a value that is not a list", () => {
    expect(listOf({ 0: "a" })).toBeUndefined();
    expect(listOf("a")).toBeUndefined();
  });
});

describe("conditionsOf", () => {
  it("gives the conditions of a valid list", () => {
    const ready = condition("Ready", "True", "Healthy", "All good");
    expect(conditionsOf([ready])).toEqual({ conditions: [ready] });
  });

  it("gives no conditions for no value", () => {
    expect(conditionsOf(undefined)).toEqual({ conditions: [] });
  });

  it("gives an error for a value that is not a list", () => {
    expect(conditionsOf({ type: "Ready" })).toEqual({
      error: "The conditions are not a list.",
    });
  });

  it.each([
    ["a string", "Ready"],
    ["no type", { status: "True" }],
    [
      "a status that is not True, False, or Unknown",
      { type: "Ready", status: "Yes" },
    ],
    [
      "a message that is not text",
      { type: "Ready", status: "True", message: { a: 1 } },
    ],
    ["a reason that is not text", { type: "Ready", status: "True", reason: 3 }],
  ])("gives an error for a condition with %s", (_, item) => {
    expect(
      conditionsOf([condition("Accepted", "True", "Healthy"), item]),
    ).toEqual({
      error: "Condition 2 is not a valid condition.",
    });
  });
});

describe("readyOf", () => {
  it("gives the Ready condition, and nothing for conditions that conditionsOf refuses", () => {
    const ready = condition("Ready", "False", "Unreachable");
    expect(readyOf([condition("Other", "True", "Healthy"), ready])).toBe(ready);
    expect(readyOf("Ready")).toBeUndefined();
    expect(
      readyOf([ready, { type: "Other", status: "Maybe" }]),
    ).toBeUndefined();
  });
});
