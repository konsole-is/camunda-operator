import { describe, expect, it } from "vitest";
import { RELEASE_TABS, releaseTabOf } from "./release-tabs";

describe("releaseTabOf", () => {
  it("selects the tab of the query and the Settings tab for an unknown or missing value", () => {
    expect(releaseTabOf("settings")).toBe("settings");
    expect(releaseTabOf(["settings", "x"])).toBe("settings");
    expect(releaseTabOf("nope")).toBe("settings");
    expect(releaseTabOf(undefined)).toBe("settings");
  });

  it("selects only a tab that the page lists", () => {
    expect(RELEASE_TABS).toEqual([{ value: "settings", label: "Settings" }]);
    expect(releaseTabOf("rollout")).toBe("settings");
  });
});
