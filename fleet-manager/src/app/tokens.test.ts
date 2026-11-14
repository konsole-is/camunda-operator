import { readFileSync } from "node:fs";
import path from "node:path";
import { describe, expect, it } from "vitest";
import { themes } from "../../.storybook/theme";

type Theme = "light" | "dark";
const THEMES: Theme[] = ["light", "dark"];

// The dark values are those of the UI draft, as issue #611 lists them, with
// one change: --faint is lighter than the draft's #6B7591, which is under
// 4.5:1 as text.
const TOKENS: Record<string, [light: string, dark: string]> = {
  "--bg": ["#F4F6FA", "#0A0F1E"],
  "--panel": ["#FFFFFF", "#0F1630"],
  "--panel-2": ["#EBEFF6", "#151D3B"],
  "--border": ["#D3D9E8", "#1F2A55"],
  "--divider": ["#E1E6F0", "#1C2650"],
  "--text": ["#0D1430", "#E8ECF5"],
  "--text-2": ["#2A3454", "#C9D2E6"],
  "--muted": ["#4A5470", "#A3ADC2"],
  "--faint": ["#59637D", "#7D869E"],
  "--accent": ["#B5440B", "#FF6B1A"],
  "--accent-hover": ["#963808", "#FF8541"],
  "--ok": ["#0B6C41", "#3DD68C"],
  "--warn": ["#7A4F00", "#FFC14D"],
  "--bad": ["#AB1A37", "#FF6B81"],
  "--info": ["#1A55BF", "#5B9BFF"],
  "--off": ["#AAB4CF", "#3C4C85"],
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
const STATUS_TOKENS = ["--ok", "--warn", "--bad", "--info"];

const css = readFileSync(path.join(import.meta.dirname, "globals.css"), "utf8");
const root = /:root\s*\{([^}]*)\}/.exec(css)?.[1] ?? "";

function tokenValues(name: string): [string, string] | undefined {
  const match = new RegExp(
    `${name}:\\s*light-dark\\((#[0-9A-Fa-f]{6}),\\s*(#[0-9A-Fa-f]{6})\\)\\s*;`,
  ).exec(root);
  return match ? [match[1].toUpperCase(), match[2].toUpperCase()] : undefined;
}

function tokenValue(name: string, theme: Theme): string | undefined {
  return tokenValues(name)?.[theme === "light" ? 0 : 1];
}

// Each test of a pair runs once for each theme.
const inThemes = <T extends unknown[]>(rows: T[]): [Theme, ...T][] =>
  THEMES.flatMap((theme) =>
    rows.map((row) => [theme, ...row] as [Theme, ...T]),
  );

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

// The color that a fill of `top` at `alpha` gives over `bottom`.
function over(top: string, alpha: number, bottom: string): string {
  return `#${[1, 3, 5]
    .map((i) => {
      const t = parseInt(top.slice(i, i + 2), 16);
      const b = parseInt(bottom.slice(i, i + 2), 16);
      return Math.round(t * alpha + b * (1 - alpha))
        .toString(16)
        .padStart(2, "0");
    })
    .join("")}`.toUpperCase();
}

function mappedToken(color: string): string | undefined {
  return new RegExp(`--color-${color}:\\s*var\\((--[\\w-]+)\\)`).exec(css)?.[1];
}

describe("design tokens", () => {
  it.each(Object.entries(TOKENS))(
    "defines %s as light-dark(%s) on :root",
    (name, values) => {
      expect(tokenValues(name)).toEqual(values);
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
    inThemes(
      TEXT_TOKENS.flatMap((text) =>
        SURFACE_TOKENS.map((surface) => [text, surface]),
      ),
    ),
  )(
    "%s: gives %s on %s a contrast of at least 4.5:1",
    (theme, text, surface) => {
      const foreground = tokenValue(text, theme);
      const background = tokenValue(surface, theme);
      expect(foreground).toBeDefined();
      expect(background).toBeDefined();
      expect(contrast(foreground!, background!)).toBeGreaterThanOrEqual(4.5);
    },
  );

  // A chip or a diff line shows a status color on a 10 or 15 percent fill of
  // the same color.
  it.each(
    inThemes(
      STATUS_TOKENS.flatMap((status) =>
        SURFACE_TOKENS.map((surface) => [status, surface]),
      ),
    ),
  )(
    "%s: gives %s text on its own tint on %s a contrast of at least 4.5:1",
    (theme, status, surface) => {
      const color = tokenValue(status, theme)!;
      for (const alpha of [0.1, 0.15]) {
        const fill = over(color, alpha, tokenValue(surface, theme)!);
        expect(
          contrast(color, fill),
          `fill at ${alpha}`,
        ).toBeGreaterThanOrEqual(4.5);
      }
    },
  );

  // A primary button and a checked checkbox put --bg on --accent, and on
  // --accent-hover under the pointer.
  it.each(inThemes([["--accent"], ["--accent-hover"]]))(
    "%s: gives the primary text on %s a contrast of at least 4.5:1",
    (theme, fill) => {
      expect(mappedToken("primary-foreground")).toBe("--bg");
      expect(
        contrast(tokenValue("--bg", theme)!, tokenValue(fill, theme)!),
      ).toBeGreaterThanOrEqual(4.5);
    },
  );

  it.each(inThemes(SURFACE_TOKENS.map((surface) => [surface])))(
    "%s: gives the border of a form control on %s a contrast of at least 3:1",
    (theme, surface) => {
      const edge = mappedToken("control-edge");
      expect(edge).toBeDefined();
      const border = tokenValue(edge!, theme);
      expect(border).toBeDefined();
      expect(
        contrast(border!, tokenValue(surface, theme)!),
      ).toBeGreaterThanOrEqual(3);
    },
  );

  // The primitives fill a hovered control with input at 50%.
  it.each(inThemes(SURFACE_TOKENS.map((surface) => [surface])))(
    "%s: gives --muted text on a hovered control on %s a contrast of at least 4.5:1",
    (theme, surface) => {
      const input = mappedToken("input");
      expect(input).toBeDefined();
      const fill = over(
        tokenValue(input!, theme)!,
        0.5,
        tokenValue(surface, theme)!,
      );
      expect(
        contrast(tokenValue("--muted", theme)!, fill),
      ).toBeGreaterThanOrEqual(4.5);
    },
  );

  it.each(
    inThemes(
      ["badge.tsx", "button.tsx"].flatMap((file) =>
        SURFACE_TOKENS.map((surface) => [file, surface]),
      ),
    ),
  )(
    "%s: gives --bad text on each destructive fill of %s on %s a contrast of at least 4.5:1",
    (theme, file, surface) => {
      const source = readFileSync(
        path.join(import.meta.dirname, "..", "components", "ui", file),
        "utf8",
      );
      const alphas = [...source.matchAll(/bg-destructive\/(\d+)/g)].map(
        (match) => Number(match[1]) / 100,
      );
      expect(alphas).not.toHaveLength(0);
      const bad = tokenValue("--bad", theme)!;
      for (const alpha of alphas) {
        const fill = over(bad, alpha, tokenValue(surface, theme)!);
        expect(contrast(bad, fill), `fill at ${alpha}`).toBeGreaterThanOrEqual(
          4.5,
        );
      }
    },
  );

  it.each(THEMES)(
    "gives the %s Storybook theme the colors of the tokens",
    (theme) => {
      const value = (name: string) => tokenValue(name, theme)?.toLowerCase();
      expect(themes[theme]).toMatchObject({
        base: theme,
        colorPrimary: value("--accent"),
        appBg: value("--bg"),
        appContentBg: value("--bg"),
        appPreviewBg: value("--bg"),
        appBorderColor: value("--border"),
        textColor: value("--text"),
        textMutedColor: value("--muted"),
        barBg: value("--panel"),
        barSelectedColor: value("--accent"),
        barHoverColor: value("--accent-hover"),
        inputBg: value("--panel"),
        inputBorder: value("--border"),
        inputTextColor: value("--text"),
      });
    },
  );
});
