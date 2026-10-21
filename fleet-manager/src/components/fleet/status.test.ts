import { readFileSync } from "node:fs";
import { describe, expect, it } from "vitest";
import { HEALTHY_REASON, describeReady } from "./status";

// goConstant reads the value of a Go string constant from a file of the
// repository.
function goConstant(file: string, name: string): string | undefined {
  const source = readFileSync(
    new URL(`../../../../${file}`, import.meta.url),
    "utf8",
  );
  return new RegExp(`\\b${name}\\s*=\\s*"([^"]*)"`).exec(source)?.[1];
}

describe("HEALTHY_REASON", () => {
  it("matches ReasonHealthy of api/v1/conditions.go", () => {
    expect(HEALTHY_REASON).toBe(
      goConstant("api/v1/conditions.go", "ReasonHealthy"),
    );
  });
});

describe("describeReady", () => {
  it("hides the reason of a True Ready by default", () => {
    expect(describeReady({ status: "True", reason: "Suspended" })).toEqual({
      tone: "ok",
      label: "Ready",
    });
  });

  it("keeps the reason of a True Ready with trueReason", () => {
    expect(
      describeReady(
        { status: "True", reason: "Suspended" },
        { trueReason: true },
      ),
    ).toEqual({ tone: "ok", label: "Ready", reason: "Suspended" });
  });

  it.each([HEALTHY_REASON, undefined, ""])(
    "hides the reason %j of a True Ready with trueReason",
    (reason) => {
      expect(
        describeReady({ status: "True", reason }, { trueReason: true }),
      ).toEqual({ tone: "ok", label: "Ready" });
    },
  );

  it.each([
    ["False", "bad", "Not ready"],
    ["Unknown", "warn", "Unknown"],
  ] as const)("keeps the reason of a %s Ready", (status, tone, label) => {
    expect(describeReady({ status, reason: "Degraded" })).toEqual({
      tone,
      label,
      reason: "Degraded",
    });
  });

  it("says No status without a condition", () => {
    expect(describeReady(undefined)).toEqual({
      tone: "off",
      label: "No status",
    });
  });
});
