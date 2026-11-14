import { expect, test } from "./fixture";
import { checkPage } from "./themes";

test.use({ scenarioName: "claim-ready" });

test("the fleet page shows the cluster of a claim in its environment tab as ready", async ({
  page,
}) => {
  await page.goto("/fleet");
  await page.getByRole("tab", { name: /Production/ }).click();
  const row = page.getByRole("row", { name: /orders/ });
  await expect(row.getByRole("link", { name: "orders" })).toHaveAttribute(
    "href",
    "/fleet/clusters/team-orders/orders",
  );
  await expect(row.getByText("Ready", { exact: true })).toBeVisible();
  await expect(row.getByText("Not managed")).toHaveCount(0);
  await expect(
    row.getByRole("img", { name: /^Secondary storage/ }),
  ).toBeVisible();
  await checkPage(page, "fleet-claim-ready");
});
