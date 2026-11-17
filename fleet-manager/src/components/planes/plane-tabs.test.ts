import { describe, expect, it } from "vitest";
import { PLANE_TABS, planeTabOf } from "./plane-tabs";

describe("PLANE_TABS", () => {
  it("lists Overview as the first tab", () => {
    expect(PLANE_TABS[0]).toEqual({ value: "overview", label: "Overview" });
  });

  it("lists the read tabs after Overview, in their order", () => {
    expect(PLANE_TABS.slice(1, 5)).toEqual([
      { value: "clusters", label: "Clusters served" },
      { value: "optimize", label: "Optimize" },
      { value: "contract", label: "Contract" },
      { value: "endpoints", label: "Endpoints" },
    ]);
  });

  it("lists the Claim tab last", () => {
    expect(PLANE_TABS.at(-1)).toEqual({ value: "claim", label: "Claim" });
  });
});

describe("planeTabOf", () => {
  it.each([
    [undefined, "overview"],
    ["nope", "overview"],
    ["overview", "overview"],
    [[], "overview"],
    [["overview", "x"], "overview"],
    [["nope", "overview"], "overview"],
    ["endpoints", "endpoints"],
    ["claim", "claim"],
  ])(
    "an unknown, missing, or array value gives overview, and the first item of an array counts: %j",
    (value, tab) => {
      expect(planeTabOf(value)).toBe(tab);
    },
  );
});
