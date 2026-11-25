import { docShot } from "./docs-shot";
import { expect, test } from "./fixture";
import { checkPage } from "./themes";

test.use({ scenarioName: "claim-waiting" });

// The tests of this file share the scenario and run in order, so the test
// that advances it runs last.

test("the page of a waiting claim opens and names the request it waits for", async ({
  page,
}) => {
  const response = await page.goto("/fleet/clusters/team-orders/orders");
  expect(response?.status()).toBe(200);
  await expect(
    page.getByRole("heading", { level: 1, name: "orders" }),
  ).toBeVisible();
  const block = page.getByRole("region", {
    name: "The cluster does not exist yet",
  });
  await expect(block.getByRole("listitem")).toHaveText([
    "Secondary storage: orders-secondary-storage",
  ]);
  await expect(
    page
      .getByRole("region", { name: "Conditions" })
      .getByRole("rowheader", { name: "RequestsReady" }),
  ).toBeVisible();
  await checkPage(page, "cluster-claim-waiting");
  await docShot(page, "cluster-page-waiting-for-options", { ready: block });
});

test("the Claim tab of a waiting claim opens with its editors", async ({
  page,
}) => {
  await page.goto("/fleet/clusters/team-orders/orders?tab=claim");
  await expect(page.getByRole("button", { name: "Edit Basics" })).toBeVisible();
  await checkPage(page, "cluster-claim-waiting-claim-tab");
});

test("a name with neither a claim nor a cluster answers 404", async ({
  page,
}) => {
  const response = await page.goto("/fleet/clusters/team-orders/payments");
  expect(response?.status()).toBe(404);
});

test("the registration links the waiting request to its cluster page", async ({
  page,
}) => {
  await page.goto("/setup/plugins/existing-secondary-storage?tab=used-by");
  await page
    .getByRole("region", { name: "team-orders/orders-secondary-storage" })
    .getByRole("link", { name: "orders" })
    .click();
  await expect(page).toHaveURL(/\/fleet\/clusters\/team-orders\/orders$/);
  await expect(
    page.getByRole("region", { name: "The cluster does not exist yet" }),
  ).toBeVisible();
});

test("the page shows the cluster once the claim makes it, without a reload", async ({
  page,
  scenario,
}) => {
  await page.goto("/fleet/clusters/team-orders/orders");
  await expect(
    page.getByRole("region", { name: "The cluster does not exist yet" }),
  ).toBeVisible();

  await scenario.advance("storage-ready");

  await expect(page.getByRole("region", { name: "Topology" })).toBeVisible({
    timeout: 15_000,
  });
  await expect(
    page.getByRole("region", { name: "The cluster does not exist yet" }),
  ).toHaveCount(0);
});
