import { expect, test } from "./fixture";
import { checkPage } from "./themes";

test.describe("a cluster with backup storage", () => {
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
    await expect(
      row.getByRole("cell", { name: "1", exact: true }),
    ).toBeVisible();
    await expect(row.getByText("Ready", { exact: true })).toBeVisible();
    await checkPage(page, "connections");

    await row.getByRole("link", { name: "backups", exact: true }).click();
    await expect(page).toHaveURL(
      /\/setup\/connections\/object-storage\/team-a\/backups$/,
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
});

test.describe("a management plane with its databases", () => {
  test.use({ scenarioName: "plane-create" });

  test("lists the users of a database server config", async ({ page }) => {
    await page.goto("/setup/connections?tab=database-servers");
    const row = page.getByRole("row").filter({
      has: page.getByRole("link", { name: "postgres", exact: true }),
    });
    await expect(
      row.getByRole("cell", { name: "3", exact: true }),
    ).toBeVisible();

    await row.getByRole("link", { name: "postgres", exact: true }).click();
    const users = page.getByRole("region", { name: "Used by" });
    for (const name of ["identity-db", "keycloak-db", "modeler-db"]) {
      await expect(
        users
          .getByRole("row")
          .filter({ hasText: `platform/${name}` })
          .getByText("spec.serverRef"),
      ).toBeVisible();
    }
    // The generator records the Ready of the contract in objects.yaml:
    // envtest runs no PostgreSQL, so the probe of 127.0.0.1:1 fails.
    const status = page.getByRole("region", { name: "Status" });
    await expect(status.getByText("ConnectionFailed")).toBeVisible();
    await expect(
      status.getByText(/^Connecting to 127\.0\.0\.1:1: /),
    ).toBeVisible();
    await checkPage(page, "connection-database-server");
  });
});
