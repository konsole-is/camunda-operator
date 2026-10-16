import { describe, expect, it } from "vitest";
import { srcPath, valueGraph } from "../../../test/imports/scan";

// FieldHelp is a client component, so each module that it imports as a value
// goes to the browser. The field docs map is several hundred KB.
describe("FieldHelp", () => {
  it("does not import the field docs map as a value", () => {
    const graph = valueGraph(srcPath("components/fleet/field-help.tsx"));
    expect(graph.size).toBeGreaterThan(1);
    expect([...graph]).not.toContain(srcPath("generated/fielddocs.ts"));
    expect([...graph]).not.toContain(srcPath("lib/fielddocs/index.ts"));
  });
});
