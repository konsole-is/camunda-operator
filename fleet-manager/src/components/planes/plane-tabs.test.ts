import { describe, expect, it } from "vitest";
import { PLANE_TABS, planeTabOf } from "./plane-tabs";

describe("PLANE_TABS", () => {
  it("lists Overview as the first tab", () => {
    expect(PLANE_TABS[0]).toEqual({ id: "overview", label: "Overview" });
  });

  it("lists the read tabs after Overview, in their order", () => {
    expect(PLANE_TABS.slice(1, 5)).toEqual([
      { id: "clusters", label: "Clusters served" },
      { id: "optimize", label: "Optimize" },
      { id: "contract", label: "Contract" },
      { id: "endpoints", label: "Endpoints" },
    ]);
  });

  it("lists the Claim tab last", () => {
    expect(PLANE_TABS.at(-1)).toEqual({ id: "claim", label: "Claim" });
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
