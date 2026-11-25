import { docShot } from "./docs-shot";
import { expect, test } from "./fixture";
import { checkPage } from "./themes";

test.use({ scenarioName: "claim-ready" });

test("the section card shows the summary headline of its request", async ({
  page,
}) => {
  await page.goto("/fleet/clusters/team-orders/orders");
  const card = page.getByRole("article", { name: "Secondary storage" });
  await expect(card.getByText("orders-storage, Ready")).toBeVisible();
  await expect(
    card.getByRole("img", { name: "Secondary storage: Ready" }),
  ).toBeVisible();
  await checkPage(page, "cluster-section-cards");
  await docShot(page, "cluster-page-overview", { ready: card });
});

test("an existing secondary storage shows as connected", async ({ page }) => {
  await page.goto("/fleet/clusters/team-orders/orders");
  await page
    .getByRole("article", { name: "Secondary storage" })
    .getByRole("link", { name: "Secondary storage" })
    .click();
  await expect(page).toHaveURL(/component=section%3Asecondary-storage/);
  await expect(
    page.getByRole("heading", { level: 2, name: "Secondary storage" }),
  ).toBeVisible();
  const connected = page.getByRole("region", { name: "Connected" });
  await expect(
    connected.getByRole("row", {
      name: "SecondaryStorageConfig orders-storage Ready",
    }),
  ).toBeVisible();
  await expect(page.getByText("This component runs no pods.")).toBeVisible();
  await expect(page.getByRole("region", { name: "Resources" })).toHaveCount(0);
  await checkPage(page, "cluster-section-connected");
  await docShot(page, "cluster-option-connected", { ready: connected });
});
