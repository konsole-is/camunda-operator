import { existsSync, readFileSync } from "node:fs";
import path from "node:path";
import { fileURLToPath } from "node:url";
import { describe, expect, it } from "vitest";

// FieldHelp is a client component, so each module that it imports as a value
// goes to the browser. The field docs map is several hundred KB. A type
// import is erased.
const src = fileURLToPath(new URL("../../", import.meta.url));
const imports =
  /^(?:import|export)\s+(type\s+)?[\w\s{},*]*?from\s*["']([^"']+)["']|^import\s*["']([^"']+)["']/gm;

function resolve(from: string, specifier: string): string | undefined {
  const base = specifier.startsWith("@/")
    ? path.join(src, specifier.slice(2))
    : specifier.startsWith(".")
      ? path.resolve(path.dirname(from), specifier)
      : undefined;
  if (!base) return undefined;
  return [".ts", ".tsx", "/index.ts", "/index.tsx"]
    .map((ext) => base + ext)
    .find((file) => existsSync(file));
}

function valueGraph(entry: string): Set<string> {
  const seen = new Set<string>();
  const queue = [entry];
  while (queue.length) {
    const file = queue.pop()!;
    if (seen.has(file)) continue;
    seen.add(file);
    for (const match of readFileSync(file, "utf8").matchAll(imports)) {
      if (match[1] !== undefined) continue;
      const next = resolve(file, match[2] ?? match[3]);
      if (next) queue.push(next);
    }
  }
  return seen;
}

describe("FieldHelp", () => {
  it("does not import the field docs map as a value", () => {
    const graph = valueGraph(path.join(src, "components/fleet/field-help.tsx"));
    expect(graph.size).toBeGreaterThan(1);
    expect([...graph]).not.toContain(path.join(src, "generated/fielddocs.ts"));
  });
});
