import { expect, it } from "vitest";
import config from "../playwright.docs.config";
import { DOCS_PROJECT } from "./docs-images";

it("docs config bounds every wait and stops at the first failure", () => {
  expect(config.maxFailures).toBe(1);
  expect(config.retries).toBe(0);
  expect(config.globalTimeout).toBe(30 * 60_000);
  expect(config.use).toMatchObject({
    viewport: { width: 1440, height: 900 },
    colorScheme: "dark",
    locale: "en-US",
    timezoneId: "UTC",
    actionTimeout: 15_000,
    navigationTimeout: 30_000,
  });
  expect(config.projects).toEqual([
    expect.objectContaining({ name: DOCS_PROJECT }),
  ]);
});

it("docs config rasters on one thread, so the same page gives the same bytes", () => {
  expect(config.use?.launchOptions?.args).toEqual(
    expect.arrayContaining([
      "--num-raster-threads=1",
      "--disable-partial-raster",
    ]),
  );
});
