import { expect, test } from "./fixture";

test.use({ scenarioName: "cluster-ready" });

// Playwright runs this file after fleet.spec.ts, which advances the same
// scenario, and can reuse its worker.
test("a second file of a scenario starts in the first state of the scenario", async ({
  page,
}) => {
  await page.goto("/fleet/clusters/team-a/camundacluster-sample");
  const header = page
    .locator("header")
    .filter({ has: page.getByRole("heading", { level: 1 }) });
  await expect(header.getByText("Ready", { exact: true })).toBeVisible();
});
