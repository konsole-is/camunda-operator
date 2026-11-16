import { expect, test } from "./fixture";
import { checkPage } from "./themes";

test.use({ scenarioName: "cluster-es-backups" });

test("lists a contract created by hand with its user", async ({ page }) => {
  await page.goto("/setup/connections");
  await expect(
    page.getByRole("heading", { level: 1, name: "Connections" }),
  ).toBeVisible();
  const row = page.getByRole("row").filter({
    has: page.getByRole("link", { name: "backups", exact: true }),
  });
  await expect(row.getByText("Created by hand")).toBeVisible();
  await expect(row.getByRole("cell", { name: "1", exact: true })).toBeVisible();
  await expect(row.getByText("Ready", { exact: true })).toBeVisible();
  await checkPage(page, "connections");

  await row.getByRole("link", { name: "backups", exact: true }).click();
  await expect(page).toHaveURL(
    /\/setup\/connections\/object-storage\/team-a\/backups$/,
  );
  const crumbs = page.getByRole("navigation", { name: "Breadcrumb" });
  await expect(crumbs.locator('[aria-current="page"]')).toHaveText(
    "team-a/backups",
  );
  const users = page.getByRole("region", { name: "Used by" });
  const user = users
    .getByRole("row")
    .filter({ hasText: "team-a/camundacluster-sample" });
  await expect(user.getByText("spec.backupStorageRef")).toBeVisible();
  await checkPage(page, "connection-detail");

  await page.goto("/setup/connections?tab=secondary-storage");
  await page
    .getByRole("link", { name: "camundacluster-sample-storage" })
    .click();
  await expect(
    page
      .getByRole("region", { name: "Used by" })
      .getByRole("row")
      .filter({ hasText: "team-a/camundacluster-sample" })
      .getByText("spec.storageRef"),
  ).toBeVisible();
});
