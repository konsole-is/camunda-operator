import { readFileSync } from "node:fs";
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

  // A server component can call a function only from a module without
  // "use client". The cluster page calls clusterTabOf on the server.
  it("is in a module that the server can call", () => {
    const source = readFileSync(new URL("./tabs.ts", import.meta.url), "utf8");
    expect(source).not.toMatch(/^\s*["']use client["']/);
    const page = readFileSync(
      new URL(
        "../../app/fleet/clusters/[namespace]/[name]/page.tsx",
        import.meta.url,
      ),
      "utf8",
    );
    expect(page).toMatch(
      /import \{[^}]*\bclusterTabOf\b[^}]*\} from "@\/components\/cluster\/tabs"/,
    );
  });
});
