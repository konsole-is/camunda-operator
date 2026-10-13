import { readFileSync } from "node:fs";
import path from "node:path";
import { describe, expect, it } from "vitest";

// The values of the UI draft, as issue #611 lists them, with one change:
// --faint is lighter than the draft's #6B7591, which is under 4.5:1 as text.
const TOKENS: Record<string, string> = {
  "--bg": "#0A0F1E",
  "--panel": "#0F1630",
  "--panel-2": "#151D3B",
  "--border": "#1F2A55",
  "--divider": "#1C2650",
  "--text": "#E8ECF5",
  "--text-2": "#C9D2E6",
  "--muted": "#A3ADC2",
  "--faint": "#7D869E",
  "--accent": "#FF6B1A",
  "--accent-hover": "#FF8541",
  "--ok": "#3DD68C",
  "--warn": "#FFC14D",
  "--bad": "#FF6B81",
  "--info": "#5B9BFF",
  "--off": "#3C4C85",
};

// --off is a dot color only, never text, so it is not in this list.
const TEXT_TOKENS = [
  "--text",
  "--text-2",
  "--muted",
  "--faint",
  "--accent",
  "--ok",
  "--warn",
  "--bad",
  "--info",
];
const SURFACE_TOKENS = ["--bg", "--panel", "--panel-2"];

const css = readFileSync(path.join(import.meta.dirname, "globals.css"), "utf8");
const root = /:root\s*\{([^}]*)\}/.exec(css)?.[1] ?? "";

function tokenValue(name: string): string | undefined {
  return new RegExp(`${name}:\\s*(#[0-9A-Fa-f]{6})\\s*;`)
    .exec(root)?.[1]
    ?.toUpperCase();
}

// The relative luminance and contrast ratio of WCAG 2.
function luminance(hex: string): number {
  const [r, g, b] = [1, 3, 5].map((i) => {
    const channel = parseInt(hex.slice(i, i + 2), 16) / 255;
    return channel <= 0.03928
      ? channel / 12.92
      : ((channel + 0.055) / 1.055) ** 2.4;
  });
  return 0.2126 * r + 0.7152 * g + 0.0722 * b;
}

function contrast(a: string, b: string): number {
  const [light, dark] = [luminance(a), luminance(b)].sort((x, y) => y - x);
  return (light + 0.05) / (dark + 0.05);
}

describe("design tokens", () => {
  it.each(Object.entries(TOKENS))(
    "defines %s as %s on :root",
    (name, value) => {
      expect(tokenValue(name)).toBe(value);
    },
  );

  it("maps every token to a Tailwind color of the same name", () => {
    for (const name of Object.keys(TOKENS)) {
      expect(css).toContain(`--color-${name.slice(2)}: var(${name});`);
    }
  });

  it("loads no stylesheet or font from another host", () => {
    expect(css).not.toMatch(/https?:\/\//);
  });

  it.each(
    TEXT_TOKENS.flatMap((text) =>
      SURFACE_TOKENS.map((surface) => [text, surface]),
    ),
  )("gives %s on %s a contrast of at least 4.5:1", (text, surface) => {
    const foreground = tokenValue(text);
    const background = tokenValue(surface);
    expect(foreground).toBeDefined();
    expect(background).toBeDefined();
    expect(contrast(foreground!, background!)).toBeGreaterThanOrEqual(4.5);
  });

  it.each(SURFACE_TOKENS)(
    "gives the border of a form control on %s a contrast of at least 3:1",
    (surface) => {
      const input = /--color-input:\s*var\((--[\w-]+)\)/.exec(css)?.[1];
      expect(input).toBeDefined();
      const border = tokenValue(input!);
      expect(border).toBeDefined();
      expect(contrast(border!, tokenValue(surface)!)).toBeGreaterThanOrEqual(3);
    },
  );
});
