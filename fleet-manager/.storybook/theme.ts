import { create } from "storybook/theming";

const shared = {
  brandTitle: "Fleet manager",
  fontBase: '"IBM Plex Sans", sans-serif',
  fontCode: '"JetBrains Mono", monospace',
  appBorderRadius: 8,
  inputBorderRadius: 8,
};

// Keep the colors of each theme equal to the tokens of src/app/globals.css:
// the light value of light-dark() for light, the dark value for dark.
const light = create({
  ...shared,
  base: "light",
  colorPrimary: "#b5440b",
  colorSecondary: "#b5440b",
  appBg: "#f4f6fa",
  appContentBg: "#f4f6fa",
  appPreviewBg: "#f4f6fa",
  appBorderColor: "#d3d9e8",
  textColor: "#0d1430",
  textInverseColor: "#f4f6fa",
  textMutedColor: "#4a5470",
  barBg: "#ffffff",
  barTextColor: "#4a5470",
  barHoverColor: "#963808",
  barSelectedColor: "#b5440b",
  inputBg: "#ffffff",
  inputBorder: "#d3d9e8",
  inputTextColor: "#0d1430",
});

const dark = create({
  ...shared,
  base: "dark",
  colorPrimary: "#ff6b1a",
  colorSecondary: "#ff6b1a",
  appBg: "#0a0f1e",
  appContentBg: "#0a0f1e",
  appPreviewBg: "#0a0f1e",
  appBorderColor: "#1f2a55",
  textColor: "#e8ecf5",
  textInverseColor: "#0a0f1e",
  textMutedColor: "#a3adc2",
  barBg: "#0f1630",
  barTextColor: "#a3adc2",
  barHoverColor: "#ff8541",
  barSelectedColor: "#ff6b1a",
  inputBg: "#0f1630",
  inputBorder: "#1f2a55",
  inputTextColor: "#e8ecf5",
});

/** themes holds the Storybook theme of each value of the theme global. */
export const themes = { light, dark };

/** StorybookTheme is a value of the theme global. */
export type StorybookTheme = keyof typeof themes;

/** storybookTheme returns the value of the theme global, or dark for any other value. */
export function storybookTheme(value: unknown): StorybookTheme {
  return value === "light" ? "light" : "dark";
}
