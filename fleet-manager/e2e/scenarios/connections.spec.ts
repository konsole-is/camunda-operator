import type { ObjectStorageConfig } from "../../src/generated/crds/objectstorageconfig";
import { kubeRequest } from "../../test/scenarios/kube";
import { expect, test } from "./fixture";
import { checkPage } from "./themes";

test.use({ scenarioName: "cluster-es-backups" });

// The tests of this file share one envtest, so they run in this order.

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

test("saves a change of a contract created by hand", async ({
  page,
  scenario,
}) => {
  await page.goto("/setup/connections/object-storage/team-a/backups");
  await page.getByLabel("Region").fill("eu-central-1");
  await expect(
    page.getByText("Delete is possible when no resource uses this contract."),
  ).toBeVisible();
  await page.getByRole("button", { name: "Review" }).click();
  const review = page.getByRole("region", { name: "Review the contract" });
  await expect(
    review.getByText(
      "CamundaCluster team-a/camundacluster-sample restarts its pods, all but Connectors.",
    ),
  ).toBeVisible();
  const save = review.getByRole("button", { name: "Save" });
  await expect(save).toBeEnabled();
  await checkPage(page, "connection-edit");
  await save.click();
  await expect(page.getByText("The contract backups is saved.")).toBeVisible();

  const saved = await kubeRequest<ObjectStorageConfig>(
    scenario.admin,
    "GET",
    "/apis/core.camunda.io/v1/namespaces/team-a/objectstorageconfigs/backups",
  );
  expect(saved.spec.s3?.region).toBe("eu-central-1");
  expect(
    saved.metadata.managedFields?.some((f) => f.manager === "fleet-manager"),
  ).toBe(true);
});
