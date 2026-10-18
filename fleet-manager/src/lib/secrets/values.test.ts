import { readFileSync, readdirSync } from "node:fs";
import path from "node:path";
import { describe, expect, it } from "vitest";
import { srcPath, valueImports } from "../../../test/imports/scan";

const VALUES = srcPath("lib/secrets/values");

function sourceFilesUnder(dir: string): string[] {
  return readdirSync(dir, { withFileTypes: true, recursive: true })
    .filter(
      (e) =>
        e.isFile() && /\.tsx?$/.test(e.name) && !/\.test\.tsx?$/.test(e.name),
    )
    .map((e) => path.join(e.parentPath, e.name));
}

function importsValues(file: string): boolean {
  return valueImports(readFileSync(file, "utf8")).some((specifier) => {
    const target = specifier.startsWith("@/")
      ? srcPath(specifier.slice(2))
      : specifier.startsWith(".")
        ? path.resolve(path.dirname(file), specifier)
        : specifier;
    return target.replace(/\.ts$/, "") === VALUES;
  });
}

describe("the Secret value reader", () => {
  it("no module outside src/lib imports it", () => {
    const outside = sourceFilesUnder(srcPath(".")).filter(
      (file) => !file.startsWith(srcPath("lib") + path.sep),
    );
    expect(outside.length).toBeGreaterThan(0);
    expect(outside.filter(importsValues)).toEqual([]);
  });

  it("the check finds an import by alias and by relative path", () => {
    expect(
      valueImports('import { readSecretValues } from "@/lib/secrets/values";'),
    ).toEqual(["@/lib/secrets/values"]);
    expect(importsValues(srcPath("lib/secrets/index.ts"))).toBe(true);
  });
});
