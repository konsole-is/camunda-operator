import { KubeError, kubeRequest } from "../../test/scenarios/kube";
import { expect, test } from "./fixture";
import { checkPage } from "./themes";

test.use({ scenarioName: "plane-create" });

// The tests of this file share one envtest, so they run in this order.

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

test("refuses the delete of a used contract and deletes an unused one", async ({
  page,
  scenario,
}) => {
  // Three database configs name the server.
  await page.goto("/setup/connections/database-servers/platform/postgres");
  await expect(
    page.getByText("Delete is possible when no resource uses this contract."),
  ).toBeVisible();
  await expect(page.getByLabel("Type postgres to confirm")).toHaveCount(0);

  // No plane exists in this scenario, so nothing uses modeler-db.
  await page.goto("/setup/connections/databases/platform/modeler-db");
  await page.getByLabel("Type modeler-db to confirm").fill("modeler-db");
  await page.getByRole("button", { name: "Review the delete" }).click();
  const review = page.getByRole("region", { name: "Review the delete" });
  const remove = review.getByRole("button", { name: "Delete the contract" });
  await expect(remove).toBeEnabled();
  await checkPage(page, "connection-delete");
  await remove.click();
  await expect(page).toHaveURL(/\/setup\/connections\?tab=databases$/);

  const read = kubeRequest(
    scenario.admin,
    "GET",
    "/apis/core.camunda.io/v1/namespaces/platform/databaseconfigs/modeler-db",
  );
  await expect(read).rejects.toBeInstanceOf(KubeError);
  await expect(read).rejects.toHaveProperty("status", 404);
});
