import { describe, expect, it } from "vitest";
import { srcPath, valueGraph, valueImports } from "./scan";

describe("valueImports", () => {
  it.each([
    ['import { readFileSync } from "node:fs";', "node:fs"],
    ['export { readFileSync } from "node:fs";', "node:fs"],
    ['export * from "@/lib/k8s/read";', "@/lib/k8s/read"],
    ['import "server-only";', "server-only"],
    [
      'import {\n  watch,\n  type Kind,\n} from "@/lib/k8s/cache";',
      "@/lib/k8s/cache",
    ],
    ['import Ajv, { type ErrorObject } from "ajv";', "ajv"],
    ['import { child } from "./path";', "./path"],
  ])("reports %j", (line, specifier) => {
    expect(valueImports(line)).toEqual([specifier]);
  });

  it.each([
    'import type { ImagesTable } from "@/startup/bundled-data";',
    'export type { FieldPath } from "./path";',
    "// import { child } from './path';",
  ])("skips %j", (line) => {
    expect(valueImports(line)).toEqual([]);
  });
});

describe("valueGraph", () => {
  it("follows value imports into the modules of the app", () => {
    const graph = valueGraph(srcPath("lib/merge/index.ts"));
    expect(graph).toContain(srcPath("lib/merge/index.ts"));
    expect(graph).toContain(srcPath("lib/merge/path.ts"));
  });

  it("does not follow a type import", () => {
    // images.ts imports the ImagesTable type of the startup module.
    const graph = valueGraph(srcPath("lib/merge/images.ts"));
    expect(graph).not.toContain(srcPath("startup/bundled-data.ts"));
  });
});
