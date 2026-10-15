import { readdirSync, readFileSync } from "node:fs";
import { fileURLToPath } from "node:url";
import { describe, expect, it } from "vitest";

// The merge module runs in the browser and on the server. A module of the app
// can need the server: most modules of src/lib/k8s start with import
// "server-only". A type import is erased.
const dir = fileURLToPath(new URL(".", import.meta.url));
const imports =
  /^(?:import|export)\s+(type\s+)?[\w\s{},*]*?from\s*["']([^"']+)["']|^import\s*["']([^"']+)["']/gm;

function valueImportsFromOutside(source: string): string[] {
  return [...source.matchAll(imports)]
    .filter((match) => match[1] === undefined)
    .map((match) => match[2] ?? match[3])
    .filter((specifier) => !specifier.startsWith("./"));
}

describe("valueImportsFromOutside", () => {
  it.each([
    ['import { readFileSync } from "node:fs";', "node:fs"],
    ['export { readFileSync } from "node:fs";', "node:fs"],
    ['export * from "@/lib/k8s/read";', "@/lib/k8s/read"],
    ['import "server-only";', "server-only"],
    [
      'import {\n  watch,\n  type Kind,\n} from "@/lib/k8s/cache";',
      "@/lib/k8s/cache",
    ],
  ])("reports %j", (line, specifier) => {
    expect(valueImportsFromOutside(line)).toEqual([specifier]);
  });

  it.each([
    'import type { ImagesTable } from "@/startup/bundled-data";',
    'export type { FieldPath } from "./path";',
    'import { child } from "./path";',
    'export { mergeCluster } from "./cluster";',
  ])("accepts %j", (line) => {
    expect(valueImportsFromOutside(line)).toEqual([]);
  });
});

describe("src/lib/merge", () => {
  const files = readdirSync(dir).filter(
    (f) => f.endsWith(".ts") && !f.endsWith(".test.ts"),
  );

  it("has modules", () => {
    expect(files.length).toBeGreaterThan(0);
  });

  it.each(files)("%s imports values only from this directory", (file) => {
    const source = readFileSync(new URL(file, import.meta.url), "utf8");

    expect(valueImportsFromOutside(source)).toEqual([]);
  });
});
