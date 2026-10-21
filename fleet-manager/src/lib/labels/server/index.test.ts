import { readFileSync } from "node:fs";
import { describe, expect, it } from "vitest";
import { ownerName } from ".";

// pkg/labels tests OwnerName against the same file, so the two rules stay
// the same.
const fixture = JSON.parse(
  readFileSync(
    new URL(
      "../../../../../test/fixtures/labels/owner-names.json",
      import.meta.url,
    ),
    "utf8",
  ),
) as { schemaVersion: number; names: { name: string; value: string }[] };

describe("ownerName", () => {
  it("reads a fixture of schema version 1", () => {
    expect(fixture.schemaVersion).toBe(1);
    expect(fixture.names.length).toBeGreaterThan(0);
  });

  it.each(fixture.names.map((c) => [c.name.length, c.name, c.value]))(
    "matches OwnerName of pkg/labels for a name of %i characters",
    (_, name, value) => {
      expect(ownerName(name as string)).toBe(value);
    },
  );
});
