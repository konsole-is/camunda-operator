import { describe, expect, it } from "vitest";
import { tabOf } from "./tabs";

const TABS = [{ value: "one" }, { value: "two-words" }] as const;

describe("tabOf", () => {
  it.each([
    ["two-words", "two-words"],
    [["two-words", "one"], "two-words"],
    [["nope", "one"], "one"],
    ["nope", "one"],
    ["", "one"],
    [[], "one"],
    [undefined, "one"],
  ] as const)("reads %j as %s", (value, tab) => {
    expect(tabOf(TABS, value as string | string[] | undefined, "one")).toBe(
      tab,
    );
  });
});
