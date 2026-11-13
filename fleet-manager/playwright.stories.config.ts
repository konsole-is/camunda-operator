import { defineConfig } from "@playwright/test";

// GitHub Pages serves the Storybook of a pull request under this path, so
// every file of the build must load from below it.
const PREFIX = "/camunda-operator/pr-preview/pr-0/";
const PORT = 3613;
const BASE_URL = `http://127.0.0.1:${PORT}${PREFIX}`;

// make fleet-stories-test builds storybook-static first.
export default defineConfig({
  testDir: "./e2e/stories",
  outputDir: "./test-results/stories",
  timeout: 300_000,
  workers: 1,
  forbidOnly: !!process.env.CI,
  reporter: [["list"]],
  use: { browserName: "chromium", baseURL: BASE_URL },
  webServer: {
    command: `node e2e/stories/serve.mts storybook-static ${PREFIX} ${PORT}`,
    url: `${BASE_URL}index.json`,
    reuseExistingServer: false,
    timeout: 30_000,
  },
  projects: [
    { name: "stories", testMatch: "stories.spec.ts", teardown: "index" },
    { name: "index", testMatch: "index.teardown.ts" },
  ],
});
