import { describe, expect, it } from "vitest";
import { REGISTRATION_TABS, registrationTabOf } from "./registration-tabs";

describe("registrationTabOf", () => {
  it.each([
    [undefined, "about"],
    ["grants", "grants"],
    [["usedBy", "health"], "usedBy"],
    ["retain", "about"],
  ])("reads %o as %s", (value, tab) => {
    expect(registrationTabOf(value)).toBe(tab);
  });
});

describe("REGISTRATION_TABS", () => {
  it("labels the six tabs in order", () => {
    expect(REGISTRATION_TABS.map((t) => t.label)).toEqual([
      "About",
      "Contract",
      "Parameters",
      "Grants",
      "Health",
      "Used by",
    ]);
  });
});
