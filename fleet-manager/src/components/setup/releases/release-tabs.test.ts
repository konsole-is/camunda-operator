import { describe, expect, it } from "vitest";
import { RELEASE_TABS, releaseTabOf } from "./release-tabs";

describe("releaseTabOf", () => {
  it("selects the tab of the query and the Settings tab for an unknown or missing value", () => {
    expect(releaseTabOf("settings")).toBe("settings");
    expect(releaseTabOf("rollout")).toBe("rollout");
    expect(releaseTabOf(["settings", "x"])).toBe("settings");
    expect(releaseTabOf("nope")).toBe("settings");
    expect(releaseTabOf(undefined)).toBe("settings");
  });

  it("lists the Settings tab and the Rollout tab, in that order", () => {
    expect(RELEASE_TABS).toEqual([
      { value: "settings", label: "Settings" },
      { value: "rollout", label: "Rollout" },
    ]);
  });
});
