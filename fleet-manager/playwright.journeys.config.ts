import { defineConfig } from "@playwright/test";

const baseURL = process.env.FLEET_JOURNEYS_BASE_URL;
if (!baseURL) {
  throw new Error(
    "Set FLEET_JOURNEYS_BASE_URL to the port-forward address of the fleet manager",
  );
}

// The journeys run against the fleet manager on the kind harness, so this
// config starts no server. playwright.config.ts starts its webServer entries
// for every project, so the journeys cannot be a project of it.
export default defineConfig({
  testDir: "./e2e/journeys",
  outputDir: "./test-results",
  timeout: 120_000,
  workers: 1,
  forbidOnly: !!process.env.CI,
  reporter: [["list"]],
  use: { browserName: "chromium", baseURL },
  projects: [
    { name: "journeys", testMatch: "*.spec.ts", teardown: "journeys-index" },
    { name: "journeys-index", testMatch: "index.teardown.ts" },
  ],
});
