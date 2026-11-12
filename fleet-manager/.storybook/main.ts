import { globSync, readFileSync } from "node:fs";
import path from "node:path";
import type { StorybookConfig } from "@storybook/nextjs-vite";

const root = path.join(import.meta.dirname, "..");

// Storybook reads only the stories files in Component Story Format, which
// have a default export. The gallery reads the other stories files, which
// export a list of stories and no default.
const csfFiles = globSync("src/components/**/*.stories.tsx", {
  cwd: root,
}).filter((file) =>
  /^export default /m.test(readFileSync(path.join(root, file), "utf8")),
);

// The previews are public pages, so no search engine may list them.
const noIndex = (head: string) =>
  `${head}<meta name="robots" content="noindex, nofollow" />`;

const config: StorybookConfig = {
  stories: csfFiles.map((file) => `../${file}`),
  addons: ["@storybook/addon-a11y", "@storybook/addon-docs"],
  framework: { name: "@storybook/nextjs-vite", options: {} },
  core: { disableTelemetry: true },
  managerHead: noIndex,
  previewHead: noIndex,
};

export default config;
