import { readdirSync, readFileSync } from "node:fs";
import path from "node:path";
import { describe, expect, it } from "vitest";

const dir = import.meta.dirname;
const primitives = readdirSync(dir).filter(
  (file) => file.endsWith(".tsx") && !file.endsWith(".stories.tsx"),
);

// shadcn color classes whose names the tokens use for something else, and
// white text, which is under the contrast floor on --accent and --bad.
const FOREIGN = [
  /(?<![\w-])bg-accent(?![\w-])/,
  /accent-foreground/,
  /(?<![\w-])bg-muted(?![\w-])/,
  /(?<![\w-])text-white(?![\w-])/,
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
