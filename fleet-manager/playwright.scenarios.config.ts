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
  // The pages follow the theme of the system, so a test starts dark and
  // switches to light with inBothThemes of e2e/scenarios/themes.ts.
  use: { browserName: "chromium", colorScheme: "dark" },
  projects: [
    { name: "scenarios", testMatch: "*.spec.ts", teardown: "pages-index" },
    { name: "pages-index", testMatch: "index.teardown.ts" },
  ],
});
