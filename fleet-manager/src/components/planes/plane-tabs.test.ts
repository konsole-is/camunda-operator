import { describe, expect, it } from "vitest";
import { PLANE_TABS, planeTabOf } from "./plane-tabs";

describe("PLANE_TABS", () => {
  it("lists Overview as the first tab", () => {
    expect(PLANE_TABS[0]).toEqual({ id: "overview", label: "Overview" });
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
  ])(
    "an unknown, missing, or array value gives overview, and the first item of an array counts: %j",
    (value, tab) => {
      expect(planeTabOf(value)).toBe(tab);
    },
  );
});
