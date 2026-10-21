import { defineConfig } from "@playwright/test";

const GALLERY = "http://127.0.0.1:3611";
const PRODUCTION = "http://127.0.0.1:3612";

// make fleet-gallery builds .next (production) and .next-gallery
// (FLEET_DEV_GALLERY=1) first. next.config.ts picks the directory from the
// same variable, so each server serves its own build.
export default defineConfig({
  testDir: "./e2e/gallery",
  outputDir: "./test-results",
  timeout: 300_000,
  workers: 1,
  forbidOnly: !!process.env.CI,
  reporter: [["list"]],
  use: { browserName: "chromium" },
  webServer: [
    {
      command:
        "./node_modules/.bin/next start --hostname 127.0.0.1 --port 3611",
      url: `${GALLERY}/dev/gallery`,
      env: { FLEET_DEV_GALLERY: "1" },
      reuseExistingServer: false,
      timeout: 60_000,
    },
    {
      command:
        "./node_modules/.bin/next start --hostname 127.0.0.1 --port 3612",
      url: `${PRODUCTION}/fleet`,
      env: { FLEET_DEV_GALLERY: "0" },
      reuseExistingServer: false,
      timeout: 60_000,
    },
  ],
  projects: [
    {
      name: "gallery",
      testMatch: "gallery.spec.ts",
      use: { baseURL: GALLERY },
      teardown: "index",
    },
    { name: "index", testMatch: "index.teardown.ts" },
    {
      name: "production",
      testMatch: "production.spec.ts",
      use: { baseURL: PRODUCTION },
    },
    // Playwright starts webServer before globalSetup, so a server cannot get
    // the kubeconfig of an envtest from the global setup. The spec starts
    // its own envtest and its own server of the gallery build.
    {
      name: "watch",
      testDir: "./e2e/watch",
      testMatch: "shared-watch.spec.ts",
    },
  ],
});
