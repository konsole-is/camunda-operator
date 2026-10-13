import { readdirSync, readFileSync } from "node:fs";
import path from "node:path";
import { describe, expect, it } from "vitest";

const dir = import.meta.dirname;
const primitives = readdirSync(dir).filter(
  (file) =>
    file.endsWith(".tsx") &&
    !file.endsWith(".stories.tsx") &&
    !file.endsWith(".test.tsx"),
);

// shadcn color classes whose names the tokens use for something else, white
// text, which is under the contrast floor on --accent and --bad, and the
// input edge, which is under 3:1.
const FOREIGN = [
  /(?<![\w-])bg-accent(?![\w-])/,
  /accent-foreground/,
  /(?<![\w-])bg-muted(?![\w-])/,
  /(?<![\w-])text-white(?![\w-])/,
  /(?<![\w-])border-input(?![\w-])/,
];

describe("shadcn primitives", () => {
  it("has the ten primitives the pages need", () => {
    expect(primitives.sort()).toEqual(
      [
        "badge.tsx",
        "button.tsx",
        "checkbox.tsx",
        "dialog.tsx",
        "input.tsx",
        "popover.tsx",
        "select.tsx",
        "table.tsx",
        "tabs.tsx",
        "tooltip.tsx",
      ].sort(),
    );
  });

  it.each(primitives)("%s hovers the accent with accent-hover", (file) => {
    const source = readFileSync(path.join(dir, file), "utf8");
    expect(source).not.toMatch(/hover:bg-primary\//);
  });

  it.each(primitives)("%s reads only CSS variables that exist", (file) => {
    const css = readFileSync(
      path.join(dir, "..", "..", "app", "globals.css"),
      "utf8",
    );
    const source = readFileSync(path.join(dir, file), "utf8");
    for (const [, name] of source.matchAll(/var\((--[\w-]+)/g)) {
      if (name.startsWith("--radix-")) continue;
      expect(css, `${file} reads ${name}`).toMatch(new RegExp(`${name}:`));
    }
  });

  it.each(primitives)(
    "%s uses no color name that a token gives another meaning",
    (file) => {
      const source = readFileSync(path.join(dir, file), "utf8");
      for (const pattern of FOREIGN) {
        expect(source, `${file} matches ${pattern}`).not.toMatch(pattern);
      }
    },
  );
});
