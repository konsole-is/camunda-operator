import { describe, expect, it } from "vitest";
import { claimPanelHref, claimPanelOf, clusterTabOf } from "./tabs";

describe("clusterTabOf", () => {
  it.each([
    [undefined, "overview"],
    ["nope", "overview"],
    ["backups", "backups"],
    [["claim", "x"], "claim"],
  ])("maps %j to %s", (value, tab) => {
    expect(clusterTabOf(value)).toBe(tab);
  });
});

describe("claimPanelHref", () => {
  it("opens the Claim tab with the panel in edit", () => {
    expect(claimPanelHref("attachments")).toBe(
      "?tab=claim&section=attachments",
    );
    expect(claimPanelHref("section:access")).toBe(
      "?tab=claim&section=section%3Aaccess",
    );
  });
});

describe("claimPanelOf", () => {
  it.each(["basics", "release", "attachments", "deletion", "section:access"])(
    "reads the known panel %s",
    (value) => {
      expect(claimPanelOf(value)).toBe(value);
    },
  );

  it.each([null, "", "section:", "zeebe"])("refuses %j", (value) => {
    expect(claimPanelOf(value)).toBeUndefined();
  });
});
