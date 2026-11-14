import type { StorybookConfig } from "@storybook/nextjs-vite";

// The previews are public pages, so no search engine may list them.
const noIndex = (head?: string) =>
  `${head ?? ""}<meta name="robots" content="noindex, nofollow" />`;

const config: StorybookConfig = {
  stories: ["../src/components/**/*.stories.tsx"],
  addons: ["@storybook/addon-a11y", "@storybook/addon-docs"],
  framework: { name: "@storybook/nextjs-vite", options: {} },
  core: { disableTelemetry: true },
  managerHead: noIndex,
  previewHead: noIndex,
};

export default config;
