import { defineConfig } from "@playwright/test";
import { DOCS_PROJECT, docShotFiles } from "./e2e/docs-images";
import scenarios from "./playwright.scenarios.config";

// The guide images have their own config: it runs each scenario file that
// calls docShot, at the width of the guide images. Each wait is bounded and
// the run stops at the first failure, so a page that stalls fails the run
// with the name of the image instead of a hang.
export default defineConfig({
  ...scenarios,
  outputDir: "./test-results/docs",
  maxFailures: 1,
  retries: 0,
  globalTimeout: 30 * 60_000,
  use: {
    ...scenarios.use,
    viewport: { width: 1440, height: 900 },
    locale: "en-US",
    timezoneId: "UTC",
    actionTimeout: 15_000,
    navigationTimeout: 30_000,
    // With more than one raster thread, Chromium draws the edge of a rounded
    // box in one of two ways, so the same page gives other bytes.
    launchOptions: {
      args: ["--num-raster-threads=1", "--disable-partial-raster"],
    },
  },
  projects: [
    {
      name: DOCS_PROJECT,
      testMatch: docShotFiles("e2e/scenarios").map((f) => `**/${f}`),
    },
  ],
});
