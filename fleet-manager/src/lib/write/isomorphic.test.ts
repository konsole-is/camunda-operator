import { readFileSync } from "node:fs";
import { describe, expect, it } from "vitest";
import { srcPath, valueGraph, valueImports } from "../../../test/imports/scan";

// The rest of this folder is server-only. A client builds preconditions with
// refs.ts, so nothing that it imports may need the server.
describe("lib/write", () => {
  it("refs.ts stays isomorphic", () => {
    const graph = [...valueGraph(srcPath("lib/write/refs.ts"))];
    expect(graph).toContain(srcPath("lib/write/refs.ts"));
    for (const file of graph)
      expect(valueImports(readFileSync(file, "utf8"))).not.toContain(
        "server-only",
      );
    expect(graph).not.toContain(srcPath("lib/write/writer.ts"));
  });
});
