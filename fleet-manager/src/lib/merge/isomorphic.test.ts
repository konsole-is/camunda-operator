import { readdirSync, readFileSync } from "node:fs";
import { fileURLToPath } from "node:url";
import { describe, expect, it } from "vitest";

// The merge module runs in the browser and on the server, so it imports no
// Node.js module and no server-only module. A type import is erased.
const dir = fileURLToPath(new URL(".", import.meta.url));
const forbidden =
  /^import\s+(?!type\b)[^;]*?["'](node:[^"']*|server-only|@\/startup\/bundled-data)["']/m;

describe("src/lib/merge", () => {
  const files = readdirSync(dir).filter(
    (f) => f.endsWith(".ts") && !f.endsWith(".test.ts"),
  );

  it("has modules", () => {
    expect(files.length).toBeGreaterThan(0);
  });

  it.each(files)("%s imports nothing that needs the server", (file) => {
    expect(readFileSync(new URL(file, import.meta.url), "utf8")).not.toMatch(
      forbidden,
    );
  });
});
