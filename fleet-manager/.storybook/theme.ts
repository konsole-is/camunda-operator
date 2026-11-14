import { create } from "storybook/theming";

// Keep these colors equal to the tokens of src/app/globals.css.
export const theme = create({
  base: "dark",
  brandTitle: "Fleet manager",
  fontBase: '"IBM Plex Sans", sans-serif',
  fontCode: '"JetBrains Mono", monospace',
  colorPrimary: "#ff6b1a",
  colorSecondary: "#ff6b1a",
  appBg: "#0a0f1e",
  appContentBg: "#0a0f1e",
  appPreviewBg: "#0a0f1e",
  appBorderColor: "#1f2a55",
  appBorderRadius: 8,
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
  inputBorderRadius: 8,
});
