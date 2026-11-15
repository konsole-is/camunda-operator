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

// The rest edge is under 3:1, so the fill must also mark a control at rest,
// and each other state must give an edge of 3:1 or more. A checkbox and a
// radio are too small for the rest edge, so they keep the strong edge at rest.
const STATES = [
  "bg-input/30",
  "focus-visible:border-ring",
  "aria-invalid:border-destructive",
];
const FIELD = [
  "border-control-edge",
  "hover:border-control-edge-strong",
  "disabled:border-control-edge",
  ...STATES,
];
const CONTROLS: Record<string, string[]> = {
  "checkbox.tsx": ["border-control-edge-strong", ...STATES],
  "radio.tsx": ["border-control-edge-strong", ...STATES],
  "input.tsx": FIELD,
  "native-select.tsx": FIELD,
  "select.tsx": FIELD,
  "textarea.tsx": FIELD,
};

describe("shadcn primitives", () => {
  it("has the thirteen primitives the pages need", () => {
    expect(primitives.sort()).toEqual(
      [
        "badge.tsx",
        "button.tsx",
        "checkbox.tsx",
        "dialog.tsx",
        "input.tsx",
        "native-select.tsx",
        "popover.tsx",
        "radio.tsx",
        "select.tsx",
        "table.tsx",
        "tabs.tsx",
        "textarea.tsx",
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

  it.each(primitives)("%s keeps the invalid border opaque", (file) => {
    const source = readFileSync(path.join(dir, file), "utf8");
    expect(source).not.toMatch(/aria-invalid:border-destructive\//);
  });

  // A variant stacked on hover, such as not-disabled, makes the hover edge
  // outrank the focus-visible and aria-invalid edges.
  it.each(Object.keys(CONTROLS))(
    "%s gives the hover edge no second variant",
    (file) => {
      const source = readFileSync(path.join(dir, file), "utf8");
      expect(source).not.toMatch(/hover:[\w-]+:border-/);
    },
  );

  it.each(Object.entries(CONTROLS))(
    "%s fills the control and gives each state other than rest a strong edge",
    (file, classes) => {
      const source = readFileSync(path.join(dir, file), "utf8");
      for (const name of classes) {
        expect(source, `${file} has ${name}`).toMatch(
          new RegExp(`(?<![\\w:-])${name}(?![\\w/-])`),
        );
      }
    },
  );
});
