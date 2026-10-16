// Test-only: reads the import statements of the app modules, for the tests
// that keep a module out of the browser bundle or keep it isomorphic. A type
// import is erased, so these helpers skip it.
import { existsSync, readFileSync, readdirSync } from "node:fs";
import path from "node:path";
import { fileURLToPath } from "node:url";
import { describe, expect, it } from "vitest";

const src = fileURLToPath(new URL("../../src/", import.meta.url));
const imports =
  /^(?:import|export)\s+(type\s+)?[\w\s{},*]*?from\s*["']([^"']+)["']|^import\s*["']([^"']+)["']/gm;

/**
 * valueImports returns the specifier of each import and re-export in source
 * that is not type-only, in source order. An inline type specifier, as in
 * `import { a, type B }`, still counts as a value import.
 */
export function valueImports(source: string): string[] {
  return [...source.matchAll(imports)]
    .filter((match) => match[1] === undefined)
    .map((match) => match[2] ?? match[3]);
}

/**
 * valueGraph returns the absolute path of entry and of each app module that
 * it reaches through value imports. It follows relative and "@/" specifiers
 * to a .ts or .tsx file or an index file, and skips packages.
 */
export function valueGraph(entry: string): Set<string> {
  const seen = new Set<string>();
  const queue = [entry];
  while (queue.length) {
    const file = queue.pop()!;
    if (seen.has(file)) continue;
    seen.add(file);
    for (const specifier of valueImports(readFileSync(file, "utf8"))) {
      const next = resolve(file, specifier);
      if (next) queue.push(next);
    }
  }
  return seen;
}

/** srcPath returns the absolute path of a file under src/. */
export function srcPath(relative: string): string {
  return path.join(src, relative);
}

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

/**
 * describeIsomorphic adds the tests that keep the modules of dir isomorphic:
 * each non-test .ts file imports values only from dir and from the given
 * packages. So no module of dir can import "server-only", a Node.js
 * builtin, or another module of the app, which can need the server.
 */
export function describeIsomorphic(
  dir: URL,
  packages: readonly string[] = [],
): void {
  const name = path.relative(
    path.dirname(src),
    fileURLToPath(dir).replace(/\/$/, ""),
  );
  describe(`${name} is isomorphic`, () => {
    const files = readdirSync(dir).filter(
      (f) => f.endsWith(".ts") && !f.endsWith(".test.ts"),
    );

    it("has modules", () => {
      expect(files.length).toBeGreaterThan(0);
    });

    it.each(files)(
      `%s imports values only from ${name}${packages.length ? ` and ${packages.join(", ")}` : ""}`,
      (file) => {
        const source = readFileSync(new URL(file, dir), "utf8");
        expect(
          valueImports(source).filter(
            (s) => !s.startsWith("./") && !packages.includes(s),
          ),
        ).toEqual([]);
      },
    );
  });
}
