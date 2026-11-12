import { blockingViolations } from "../axe";
import { PAGES_DIR, shot } from "../shot";
import { expect, test } from "./fixture";

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
  expect(await blockingViolations(page)).toEqual([]);
  await shot(page, "fleet-claim-ready", { dir: PAGES_DIR });
});
