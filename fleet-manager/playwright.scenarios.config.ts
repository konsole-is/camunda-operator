import { defineConfig } from "@playwright/test";

// The page scenarios have their own config, because Playwright starts each
// webServer entry of playwright.config.ts for any project that runs. A
// scenario file starts its own envtest and app server in a worker fixture.
export default defineConfig({
  testDir: "./e2e/scenarios",
  outputDir: "./test-results/scenarios",
  timeout: 60_000,
  workers: 1,
  forbidOnly: !!process.env.CI,
  reporter: [["list"]],
  use: { browserName: "chromium" },
  projects: [
    { name: "scenarios", testMatch: "*.spec.ts", teardown: "pages-index" },
    { name: "pages-index", testMatch: "index.teardown.ts" },
  ],
});
