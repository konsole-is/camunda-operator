import { defineConfig } from "@playwright/test";

const PRODUCTION = "http://127.0.0.1:3612";

// make fleet-app-test builds .next (production) and .next-envtest
// (FLEET_ENVTEST_BUILD=1) first. next.config.ts picks the directory from the
// same variable, so each server serves its own build.
export default defineConfig({
  testDir: "./e2e/app",
  outputDir: "./test-results/app",
  timeout: 300_000,
  workers: 1,
  forbidOnly: !!process.env.CI,
  reporter: [["list"]],
  use: { browserName: "chromium" },
  webServer: [
    {
      command:
        "./node_modules/.bin/next start --hostname 127.0.0.1 --port 3612",
      url: `${PRODUCTION}/fleet`,
      env: { FLEET_ENVTEST_BUILD: "0" },
      reuseExistingServer: false,
      timeout: 60_000,
    },
  ],
  projects: [
    {
      name: "production",
      testMatch: "production.spec.ts",
      use: { baseURL: PRODUCTION },
    },
    // Playwright starts webServer before globalSetup, so a server cannot get
    // the kubeconfig of an envtest from the global setup. The spec starts
    // its own envtest and its own server of the envtest build.
    { name: "watch", testMatch: "shared-watch.spec.ts" },
  ],
});
