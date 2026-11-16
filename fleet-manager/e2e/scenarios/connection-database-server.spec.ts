import { expect, test } from "./fixture";
import { checkPage } from "./themes";

test.use({ scenarioName: "plane-create" });

test("lists the users of a database server config", async ({ page }) => {
  await page.goto("/setup/connections?tab=database-servers");
  const row = page.getByRole("row").filter({
    has: page.getByRole("link", { name: "postgres", exact: true }),
  });
  await expect(row.getByRole("cell", { name: "3", exact: true })).toBeVisible();

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
