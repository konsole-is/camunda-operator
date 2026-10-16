import { readFileSync } from "node:fs";
import { describe, expect, it } from "vitest";
import { ENVIRONMENT_LABEL, isLabelKey, isLabelValue } from ".";

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

describe("ENVIRONMENT_LABEL", () => {
  // The Go constant is the source. A change on one side alone fails here.
  it("matches LabelEnvironment of api/fleet/v1alpha1", () => {
    const goSource = readFileSync(
      new URL(
        "../../../../api/fleet/v1alpha1/shared_types.go",
        import.meta.url,
      ),
      "utf8",
    );
    expect(ENVIRONMENT_LABEL).toBe(
      /LabelEnvironment\s*=\s*"([^"]*)"/.exec(goSource)?.[1],
    );
  });
});
