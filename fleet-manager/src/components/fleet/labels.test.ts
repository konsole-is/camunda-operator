import { describe, expect, it } from "vitest";
import { isLabelKey, isLabelValue } from "./labels";

describe("isLabelKey", () => {
  it.each(["team", "app.kubernetes.io/name", "a", "a_b.c-d"])(
    "accepts %j",
    (key) => {
      expect(isLabelKey(key)).toBe(true);
    },
  );
  it.each([
    "",
    "-team",
    "team-",
    "a/b/c",
    "UPPER.example.com/x",
    "x".repeat(64),
    "/name",
  ])("refuses %j", (key) => {
    expect(isLabelKey(key)).toBe(false);
  });
});

describe("isLabelValue", () => {
  it.each(["", "payments", "a.b_c-d", "x".repeat(63)])(
    "accepts %j",
    (value) => {
      expect(isLabelValue(value)).toBe(true);
    },
  );
  it.each(["-x", "x-", "a b", "x".repeat(64)])("refuses %j", (value) => {
    expect(isLabelValue(value)).toBe(false);
  });
});
