import { describe, expect, it } from "vitest";
import { clusterTabOf } from "./tabs";

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
