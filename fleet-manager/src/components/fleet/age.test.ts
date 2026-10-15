import { describe, expect, it } from "vitest";
import { age } from "./age";

const now = "2026-10-15T12:00:00Z";

describe("age", () => {
  it.each([
    ["2026-10-15T11:59:15Z", "45s"],
    ["2026-10-15T11:48:00Z", "12m"],
    ["2026-10-15T09:00:00Z", "3h"],
    ["2026-10-10T12:00:00Z", "5d"],
  ])("gives %s as %s", (from, expected) => {
    expect(age(from, now)).toBe(expected);
  });

  it("gives 0s for a time after now", () => {
    expect(age("2026-10-15T12:00:05Z", now)).toBe("0s");
  });

  it.each([[undefined], ["not a time"]])("gives - for %j", (from) => {
    expect(age(from, now)).toBe("-");
  });
});
